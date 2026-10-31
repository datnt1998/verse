// Package backdrop exposes persisted owner edits through the shared job-edit seam.
package backdrop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	pipelineBackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
)

type Service struct {
	Edits *jobedit.Service
	Deps  pipelineBackdrop.Deps
}

type EvidenceView = pipelineBackdrop.EvidenceView

type EditRequest struct {
	Op       string `json:"op"`
	ItemID   string `json:"itemId,omitempty"`
	Revision string `json:"revision,omitempty"`
	pipelineBackdrop.VideoChoice
}

type Result struct {
	Accepted    bool               `json:"accepted"`
	JobID       string             `json:"jobId"`
	Revision    string             `json:"revision"`
	Invalidated []domain.CloneStep `json:"invalidated"`
	RunID       *string            `json:"runId"`
}

func videoSec(snapshot jobedit.Snapshot) *float64 {
	for _, step := range snapshot.Steps {
		if step.Step == domain.CloneDownload && step.Status == domain.CloneSucceeded && step.Output != nil {
			return pipelineBackdrop.DownloadedVideoSec(step.Output.Data)
		}
	}
	return nil
}

func checkpoint(snapshot jobedit.Snapshot) (*pipelineBackdrop.Checkpoint, error) {
	if len(snapshot.Manifest) == 0 {
		return nil, nil
	}
	value, err := pipelineBackdrop.ParseCheckpoint(snapshot.Manifest)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Service) snapshot(ctx context.Context, id string) (jobedit.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return jobedit.Snapshot{}, err
	}
	if s.Edits == nil || s.Edits.Store == nil || s.Deps.Store == nil {
		return jobedit.Snapshot{}, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Dịch vụ nền video chưa sẵn sàng"}
	}
	return s.Edits.Read(ctx, id, domain.CloneBackdrop)
}

// Read uses only the job's pinned template, download output and cached catalogue.
// Before the producer runs, its draft revision is also the first edit's CAS revision.
func (s *Service) Read(ctx context.Context, id string) (*EvidenceView, error) {
	snapshot, err := s.snapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	stored, err := checkpoint(snapshot)
	if err != nil {
		return nil, err
	}
	input := pipelineBackdrop.EvidenceInput{TemplateSnapshot: snapshot.Job.TemplateSnapshot, VideoSec: videoSec(snapshot), Busy: jobedit.Busy(snapshot, domain.CloneBackdrop)}
	if stored != nil {
		input.Manifest = &stored.Manifest
		view, err := pipelineBackdrop.Evidence(ctx, s.Deps.Store, input)
		if err == nil && view != nil {
			decorateEvidence(view, snapshot)
		}
		return view, err
	}
	view, err := pipelineBackdrop.Evidence(ctx, s.Deps.Store, input)
	if err != nil || view == nil {
		return view, err
	}
	template, err := pipelineBackdrop.ResolveTemplate(ctx, snapshot.Job.TemplateSnapshot, s.Deps.Store)
	if err != nil {
		return nil, err
	}
	initial, _ := pipelineBackdrop.InitialManifest(template, input.VideoSec)
	view.Revision = initial.Revision
	view.WantedCount = initial.WantedCount
	decorateEvidence(view, snapshot)
	return view, nil
}

func decorateEvidence(view *EvidenceView, snapshot jobedit.Snapshot) {
	for _, step := range snapshot.Steps {
		if step.Step != domain.CloneBuild {
			continue
		}
		view.Rebuilding = step.Status == domain.ClonePending || step.Status == domain.CloneStale || step.Status == domain.CloneRunning
		if step.Output != nil {
			if revision, ok := step.Output.Data["projectRevision"].(string); ok && revision != "" {
				view.ProjectRevision = &revision
			}
		}
	}
	for i := range view.Items {
		item := &view.Items[i]
		if item.MediaID == nil {
			continue
		}
		for _, artifact := range snapshot.Artifacts {
			if (artifact.Path == *item.MediaID || strings.HasSuffix(artifact.Path, "/"+*item.MediaID)) && len(artifact.SHA256) >= 32 {
				item.EditorMediaID = "media-" + artifact.SHA256[:32]
				break
			}
		}
	}
}

// Edit computes catalogue selection outside the transaction. Commit compares the
// original checkpoint and job generation again before publishing the new choice.
func (s *Service) Edit(ctx context.Context, id string, input EditRequest) (Result, error) {
	snapshot, err := s.snapshot(ctx, id)
	if err != nil {
		return Result{}, err
	}
	if snapshot.Job.SupersededAt != nil {
		return Result{}, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Video đã được thay thế bởi lượt làm mới"}
	}
	if jobedit.Busy(snapshot, domain.CloneBackdrop) {
		return Result{}, jobedit.Conflict("Video đang xử lý nền hoặc dựng video")
	}
	stored, err := checkpoint(snapshot)
	if err != nil {
		return Result{}, err
	}
	template, err := pipelineBackdrop.ResolveTemplate(ctx, snapshot.Job.TemplateSnapshot, s.Deps.Store)
	if err != nil {
		return Result{}, err
	}
	state := pipelineBackdrop.EditState{JobID: snapshot.Job.ID, Template: template, VideoSec: videoSec(snapshot)}
	if stored != nil {
		state.Manifest = &stored.Manifest
	}
	edited, err := (pipelineBackdrop.Editor{Deps: s.Deps}).Edit(ctx, state, pipelineBackdrop.EditRequest{Action: input.Op, ItemID: input.ItemID, Revision: input.Revision, VideoChoice: input.VideoChoice})
	if err != nil {
		return Result{}, err
	}
	// Preserve the producer's staged files across edits of an interrupted run.
	// The published read model remains a bare manifest, not the retry envelope.
	var next any = edited.Manifest // The seam accepts both producer-owned checkpoint shapes.
	var artifacts []media.Artifact
	var work string
	if stored != nil && len(stored.Artifacts) > 0 {
		next = pipelineBackdrop.Checkpoint{Manifest: edited.Manifest, Artifacts: stored.Artifacts}
		artifacts, work, err = manifestArtifact(edited.Manifest)
		if err != nil {
			return Result{}, err
		}
		defer os.RemoveAll(work)
	}
	rerun := edited.InvalidateFrom == domain.CloneBackdrop
	if err := s.Edits.Commit(ctx, snapshot, next, edited.InvalidateFrom, rerun, artifacts); err != nil {
		return Result{}, err
	}
	invalidated := make([]domain.CloneStep, 0, 4)
	invalidated = append(invalidated, edited.InvalidateFrom)
	invalidated = append(invalidated, domain.DependentsOf(edited.InvalidateFrom)...)
	return Result{Accepted: true, JobID: id, Revision: edited.Manifest.Revision, Invalidated: invalidated}, nil
}

func manifestArtifact(manifest pipelineBackdrop.Manifest) ([]media.Artifact, string, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	work, err := os.MkdirTemp("", "viderse-backdrop-edit-")
	if err != nil {
		return nil, "", err
	}
	file := filepath.Join(work, "manifest.json")
	if err := os.WriteFile(file, append(raw, '\n'), 0600); err != nil {
		return nil, "", errors.Join(err, os.RemoveAll(work))
	}
	return []media.Artifact{{Path: "backdrop/manifest.json", File: file, ContentType: "application/json", Filename: "manifest.json"}}, work, nil
}
