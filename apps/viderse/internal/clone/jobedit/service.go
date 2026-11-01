// Package jobedit publishes owner edits under the same transaction as their
// checkpoint CAS and pipeline invalidation. Paid work remains in the worker.
package jobedit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

type Snapshot struct {
	Job       domain.CloneJob
	Steps     []domain.CloneStepRecord
	Step      domain.CloneStepRecord
	Artifacts []domain.CloneArtifact
	// Manifest is hydrated from the checkpoint or the committed producer output.
	// Step.Checkpoint stays untouched as the compare-and-swap token.
	Manifest json.RawMessage
}
type Change struct {
	Checkpoint     json.RawMessage
	InvalidateFrom domain.CloneStep
	Rerun          bool
	// ImageRedraw: vẽ lại ảnh của một video đã dựng — chỉ bước images chạy
	// lại, bước sau giữ nguyên (media của clip được thay tại chỗ).
	ImageRedraw   bool
	Artifacts     []domain.CloneArtifact
	ApproveExport bool
	ImageReview   bool
	// ImageRebind: lượt thay media của clip trong project đã dựng (07 §26) —
	// ghi manifest bước build được phép trong lúc job lease còn sống vì lượt
	// vẽ lại ảnh vẫn đang chạy.
	ImageRebind   bool
	ImageIDs      []string
	ImageOverride bool
}
type Store interface {
	Read(context.Context, string, domain.CloneStep) (Snapshot, error)
	Commit(context.Context, Snapshot, Change) error
}
type Service struct {
	Store   Store
	Media   pipeline.Media
	Publish pipeline.Publisher
	Wake    func()
}

func Conflict(message string) error {
	return &domain.CloneError{Code: domain.CloneErrConflict, Message: message}
}
func Busy(snapshot Snapshot, producer domain.CloneStep) bool {
	guarded := map[domain.CloneStep]bool{producer: true}
	for _, step := range domain.DependentsOf(producer) {
		guarded[step] = true
	}
	if guarded[snapshot.Step.Step] && snapshot.Step.Status == domain.CloneRunning {
		return true
	}
	for _, step := range snapshot.Steps {
		if guarded[step.Step] && step.Status == domain.CloneRunning {
			return true
		}
	}
	return false
}
func hasManifest(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != "{}"
}
func (s *Service) Read(ctx context.Context, id string, step domain.CloneStep) (Snapshot, error) {
	snapshot, err := s.Store.Read(ctx, id, step)
	if err != nil {
		return Snapshot{}, err
	}
	if hasManifest(snapshot.Manifest) {
		return snapshot, nil
	}
	if hasManifest(snapshot.Step.Checkpoint) {
		snapshot.Manifest = append(json.RawMessage(nil), snapshot.Step.Checkpoint...)
		return snapshot, nil
	}
	if snapshot.Step.Output != nil {
		if manifest, ok := snapshot.Step.Output.Data["manifest"]; ok {
			if _, path := manifest.(string); !path {
				snapshot.Manifest, err = json.Marshal(manifest)
				return snapshot, err
			}
		}
	}
	path := string(step) + "/manifest.json"
	if !Published(snapshot, path) {
		return snapshot, nil
	}
	file, err := s.Acquire(ctx, snapshot, path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Release()
	snapshot.Manifest, err = os.ReadFile(file.Path)
	if err != nil {
		return Snapshot{}, err
	}
	if !json.Valid(snapshot.Manifest) {
		return Snapshot{}, fmt.Errorf("invalid published %s manifest", step)
	}
	return snapshot, nil
}
func matches(artifact domain.CloneArtifact, path string) bool {
	return artifact.Path == path || !strings.HasPrefix(path, "clone/") && strings.HasSuffix(artifact.Path, "/"+path)
}
func selected(snapshot Snapshot, path string) (string, error) {
	// The producer's output identifies the current binding when earlier versions
	// of an immutable artifact have the same recipe-relative name.
	if snapshot.Step.Output != nil {
		for _, artifact := range snapshot.Step.Output.Artifacts {
			if matches(artifact, path) {
				return artifact.Path, nil
			}
		}
	}
	selected := ""
	for _, artifact := range snapshot.Artifacts {
		if !matches(artifact, path) {
			continue
		}
		if selected != "" && selected != artifact.Path {
			return "", Conflict("Tệp đã có nhiều phiên bản — tải lại video")
		}
		selected = artifact.Path
	}
	if selected == "" {
		return "", &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy tệp đã công bố"}
	}
	return selected, nil
}
func Published(snapshot Snapshot, path string) bool {
	_, err := selected(snapshot, path)
	return err == nil
}
func (s *Service) Acquire(ctx context.Context, snapshot Snapshot, path string) (*media.Handle, error) {
	if media.ValidatePath(path) != nil {
		return nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Đường dẫn tệp không hợp lệ"}
	}
	logical, err := selected(snapshot, path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(logical, "clone/"+snapshot.Job.ID+"/") {
		return nil, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Tệp không thuộc video này"}
	}
	if s.Media == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Kho tệp chưa sẵn sàng"}
	}
	return s.Media.Acquire(ctx, logical)
}

func (s *Service) Commit(ctx context.Context, before Snapshot, checkpoint any, from domain.CloneStep, rerun bool, artifacts []media.Artifact) error {
	return s.commit(ctx, before, checkpoint, from, rerun, artifacts, false, nil, false)
}

// CommitBuildMedia ghi manifest bước build sau khi thay media của clip: lượt
// vẽ lại ảnh (bước images) vẫn giữ job lease nên commit phải đi qua — CAS của
// bước build vẫn chặn mọi thay đổi khác.
func (s *Service) CommitBuildMedia(ctx context.Context, before Snapshot, checkpoint any, artifacts []media.Artifact) error {
	return s.commit(ctx, before, checkpoint, "", false, artifacts, false, nil, true)
}
func (s *Service) CommitOwnerApproval(ctx context.Context, before Snapshot, checkpoint any) error {
	return s.commit(ctx, before, checkpoint, domain.CloneBuild, false, nil, true, nil, false)
}
func (s *Service) CommitImageReview(ctx context.Context, before Snapshot, checkpoint any, from domain.CloneStep, rerun bool, imageIDs []string) error {
	if before.Step.Status == domain.CloneRunning {
		raw, err := json.Marshal(checkpoint)
		if err != nil {
			return err
		}
		if err := s.Store.Commit(ctx, before, Change{Checkpoint: raw, InvalidateFrom: from, Rerun: rerun, ImageReview: true, ImageIDs: imageIDs}); err != nil {
			return err
		}
		if s.Publish != nil {
			s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": before.Job.ID, "step": before.Step.Step})
		}
		if from != "" && s.Wake != nil {
			s.Wake()
		}
		return nil
	}
	return s.commit(ctx, before, checkpoint, from, rerun, nil, false, imageIDs, false)
}

// CommitImageRedraw ghi yêu cầu vẽ lại ảnh và chỉ đánh dấu bước images chạy
// lại: video đã dựng giữ nguyên project, media của clip được thay tại chỗ.
func (s *Service) CommitImageRedraw(ctx context.Context, before Snapshot, checkpoint any, imageIDs []string) error {
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	if before.Step.Status == domain.CloneRunning {
		if err := s.Store.Commit(ctx, before, Change{Checkpoint: raw, Rerun: true, ImageRedraw: true, ImageIDs: imageIDs}); err != nil {
			return err
		}
		if s.Publish != nil {
			s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": before.Job.ID, "step": before.Step.Step})
		}
		if s.Wake != nil {
			s.Wake()
		}
		return nil
	}
	// Step đã đỗ (video đang chờ duyệt): yêu cầu vẽ lại phải xếp lại đúng bước
	// images và đánh thức worker — nếu không bản mới chỉ nằm trong manifest mà
	// không ai vẽ. Chỉ bước images chạy lại; project và bước sau giữ nguyên.
	if err := s.Store.Commit(ctx, before, Change{Checkpoint: raw, InvalidateFrom: domain.CloneImages, Rerun: true, ImageRedraw: true, ImageIDs: imageIDs}); err != nil {
		return err
	}
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": before.Job.ID, "step": before.Step.Step})
	}
	if s.Wake != nil {
		s.Wake()
	}
	return nil
}

func (s *Service) CommitImageOverride(ctx context.Context, before Snapshot, checkpoint any) error {
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	if err := s.Store.Commit(ctx, before, Change{Checkpoint: raw, InvalidateFrom: domain.CloneImages, Rerun: true, ImageOverride: true}); err != nil {
		return err
	}
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": before.Job.ID, "step": before.Step.Step})
	}
	if s.Wake != nil {
		s.Wake()
	}
	return nil
}
func (s *Service) commit(ctx context.Context, before Snapshot, checkpoint any, from domain.CloneStep, rerun bool, artifacts []media.Artifact, approveExport bool, imageIDs []string, imageRebind bool) error {
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	if s.Media == nil {
		return &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Kho tệp chưa sẵn sàng"}
	}
	work, err := os.MkdirTemp("", "viderse-job-edit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	manifestPath := string(before.Step.Step) + "/manifest.json"
	artifacts = append([]media.Artifact(nil), artifacts...)
	has := false
	for _, artifact := range artifacts {
		if artifact.Path == manifestPath {
			has = true
		}
	}
	if !has {
		file := filepath.Join(work, "manifest.json")
		if err := os.WriteFile(file, append(raw, '\n'), 0600); err != nil {
			return err
		}
		artifacts = append(artifacts, media.Artifact{Path: manifestPath, File: file, ContentType: "application/json"})
	}
	published := make([]media.Artifact, 0, len(artifacts))
	entries := make([]domain.CloneArtifact, 0, len(artifacts))
	seen := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		if err := media.ValidatePath(artifact.Path); err != nil {
			return err
		}
		if strings.HasPrefix(artifact.Path, "clone/") || seen[artifact.Path] {
			return fmt.Errorf("invalid or repeated relative edit artifact")
		}
		seen[artifact.Path] = true
		file, err := os.Open(artifact.File)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		artifact.Path = "clone/" + before.Job.ID + "/" + digest + "/" + artifact.Path
		entries = append(entries, domain.CloneArtifact{ID: pipeline.InputFingerprint(artifact.Path), Path: artifact.Path, SHA256: digest, Bytes: size, Step: before.Step.Step, ContentType: artifact.ContentType, Filename: artifact.Filename})
		published = append(published, artifact)
	}
	err = s.Media.Publish(ctx, published, func(txctx context.Context) error {
		return s.Store.Commit(txctx, before, Change{Checkpoint: raw, InvalidateFrom: from, Rerun: rerun, Artifacts: entries, ApproveExport: approveExport, ImageReview: imageIDs != nil, ImageRebind: imageRebind, ImageIDs: imageIDs})
	})
	if err != nil {
		return err
	}
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": before.Job.ID, "step": before.Step.Step})
	}
	if from != "" && s.Wake != nil {
		s.Wake()
	}
	return nil
}
