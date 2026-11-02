package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/blobstore"
)

type JobRepository interface {
	GetJob(context.Context, string) (*domain.CloneJob, error)
	ListSteps(context.Context, string) ([]domain.CloneStepRecord, error)
}
type Service struct {
	Store         *Store
	Supervisor    *Supervisor
	Editor        *editor.Service
	Jobs          JobRepository
	Media         pipeline.Media
	Runner        tools.Runner
	ResolveFFmpeg func(context.Context) (string, string, error)
	Resume        func(context.Context, string) error
	ChannelName   func(context.Context, string) (string, error)
	requestMu     sync.Mutex
	mu            sync.Mutex
}
type QualityState struct {
	Score      *quality.Score `json:"score"`
	Project    editor.Project `json:"project,omitempty"`
	AcceptedAt *time.Time     `json:"acceptedAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

func (s *Service) statePath(jobID string) (string, error) {
	if err := editor.ValidateID(jobID); err != nil {
		return "", err
	}
	return filepath.Join(s.Store.Root, "quality", jobID+".json"), nil
}
func (s *Service) State(jobID string) (QualityState, error) {
	path, err := s.statePath(jobID)
	if err != nil {
		return QualityState{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return QualityState{}, nil
	}
	if err != nil {
		return QualityState{}, err
	}
	var state QualityState
	err = json.Unmarshal(raw, &state)
	if err == nil && state.Score != nil {
		err = state.Score.Validate()
	}
	return state, err
}
func (s *Service) saveState(jobID string, fn func(*QualityState)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.State(jobID)
	if err != nil {
		return err
	}
	fn(&state)
	state.UpdatedAt = time.Now().UTC()
	path, err := s.statePath(jobID)
	if err != nil {
		return err
	}
	return steps.WriteJSONAtomic(path, state)
}
func (s *Service) Accept(jobID string, accept bool) error {
	return s.saveState(jobID, func(state *QualityState) {
		state.AcceptedAt = nil
		if accept {
			now := time.Now().UTC()
			state.AcceptedAt = &now
		}
	})
}
func (s *Service) Inputs(ctx context.Context, jobID string) (domain.CloneJob, map[domain.CloneStep]domain.CloneStepOutput, string, pipeline.Media, error) {
	job, err := s.Jobs.GetJob(ctx, jobID)
	if err != nil {
		return domain.CloneJob{}, nil, "", nil, err
	}
	if job == nil {
		return domain.CloneJob{}, nil, "", nil, pipeline.ErrNotFound
	}
	records, err := s.Jobs.ListSteps(ctx, jobID)
	if err != nil {
		return *job, nil, "", nil, err
	}
	upstream := map[domain.CloneStep]domain.CloneStepOutput{}
	projectID := ""
	for _, r := range records {
		if r.Output != nil {
			upstream[r.Step] = *r.Output
			if r.Step == domain.CloneBuild {
				projectID, _ = r.Output.Data["projectId"].(string)
			}
		}
	}
	if projectID == "" {
		return *job, upstream, "", nil, renderError(domain.CloneErrInvalidRequest, "Video chưa có bản dựng — chạy bước dựng trước")
	}
	var resources editor.Repository
	if s.Editor != nil {
		resources = s.Editor.Repository
		if resources == nil {
			resources = s.Editor.Store.Repository
		}
	}
	return *job, upstream, projectID, artifactMedia{Base: s.Media, JobID: jobID, Upstream: upstream, Resources: resources}, nil
}

type artifactMedia struct {
	Base      pipeline.Media
	JobID     string
	Upstream  map[domain.CloneStep]domain.CloneStepOutput
	Resources editor.Repository
}

func (m artifactMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	logical := ""
	for _, out := range m.Upstream {
		for _, a := range out.Artifacts {
			if a.Path == path || strings.HasSuffix(a.Path, "/"+path) {
				if logical != "" && logical != a.Path {
					return nil, fmt.Errorf("ambiguous artifact %s", path)
				}
				logical = a.Path
			}
		}
	}
	if logical != "" {
		return m.Base.Acquire(ctx, logical)
	}
	if strings.HasPrefix(path, "clone/") && !strings.HasPrefix(path, "clone/"+m.JobID+"/") {
		return nil, pipeline.ErrNotFound
	}
	if m.Resources != nil && strings.HasPrefix(path, "editor/media/") && strings.HasSuffix(path, "/metadata.json") {
		parts := strings.Split(path, "/")
		if len(parts) != 4 || editor.ValidateID(parts[2]) != nil {
			return nil, pipeline.ErrNotFound
		}
		record, err := m.Resources.Get(ctx, "media/"+parts[2])
		if err != nil {
			return nil, err
		}
		return m.Base.Acquire(ctx, record.Path)
	}
	if strings.HasPrefix(path, "editor/media/") {
		for _, out := range m.Upstream {
			for _, a := range out.Artifacts {
				if strings.HasSuffix(a.Path, "/"+path) {
					return m.Base.Acquire(ctx, a.Path)
				}
			}
		}
	}
	return m.Base.Acquire(ctx, path)
}
func (m artifactMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	return m.Base.Publish(ctx, artifacts, commit)
}
func (s *Service) CurrentBinding(ctx context.Context, jobID string) (quality.Binding, error) {
	_, upstream, pid, files, err := s.Inputs(ctx, jobID)
	if err != nil {
		return quality.Binding{}, err
	}
	project, revision, err := s.Editor.LoadProject(ctx, pid)
	if err != nil {
		return quality.Binding{}, err
	}
	return DeriveBinding(ctx, files, upstream, project, revision)
}
func (s *Service) Gate(ctx context.Context, jobID string) (quality.GateDecision, error) {
	state, err := s.State(jobID)
	if err != nil {
		return quality.GateDecision{}, err
	}
	var binding *quality.Binding
	if state.Score != nil {
		b, e := s.CurrentBinding(ctx, jobID)
		if e == nil {
			binding = &b
		}
	}
	return quality.EvaluateGate(state.Score, binding, state.AcceptedAt != nil), nil
}
func (s *Service) CheckGate(ctx context.Context, projectID string, accept bool) (quality.GateDecision, error) {
	jobID, err := s.Editor.ExportOwner(ctx, projectID)
	if err != nil {
		return quality.GateDecision{}, err
	}
	if jobID == "" {
		return quality.GateDecision{Allowed: true, Fresh: true, Reasons: []string{}}, nil
	}
	if accept {
		if err = s.Accept(jobID, true); err != nil {
			return quality.GateDecision{}, err
		}
	}
	return s.Gate(ctx, jobID)
}
func (s *Service) Score(ctx context.Context, jobID string, lufs *float64, measurementID *string, output *OutputEvidence) (*quality.Score, error) {
	job, upstream, pid, files, err := s.Inputs(ctx, jobID)
	if err != nil {
		return nil, err
	}
	project, revision, err := s.Editor.LoadProject(ctx, pid)
	if err != nil {
		return nil, err
	}
	var snapshot library.Snapshot
	var template editor.Project
	if err = json.Unmarshal(job.TemplateSnapshot, &snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.Project) > 0 {
		template, err = editor.Parse(snapshot.Project)
		if err != nil {
			return nil, err
		}
	}
	score, err := ComputeScore(ctx, ScoreInput{Project: project, Revision: revision, Job: job, Upstream: upstream, LUFS: lufs, MeasurementID: measurementID, Template: template, Media: files, Output: output})
	if err != nil {
		return nil, err
	}
	if measurementID != nil {
		measured, err := s.Store.GetMeasurement(*measurementID)
		if err != nil {
			return nil, err
		}
		if measured.Status != "measured" || measured.Binding != score.Binding {
			return nil, renderError(domain.CloneErrStale, "Project đã đổi trong lúc đo — đo lại")
		}
	}
	if err = s.saveState(jobID, func(state *QualityState) {
		state.Score = score
		state.Project = editor.Clone(project)
	}); err != nil {
		return nil, err
	}
	return score, nil
}
func (s *Service) Measure(ctx context.Context, jobID string) (MeasurementRequest, *quality.Score, error) {
	_, _, pid, _, err := s.Inputs(ctx, jobID)
	if err != nil {
		return MeasurementRequest{}, nil, err
	}
	binding, err := s.CurrentBinding(ctx, jobID)
	if err != nil {
		return MeasurementRequest{}, nil, err
	}
	request, err := s.PendingMeasurement(jobID, pid, binding)
	if err != nil {
		return request, nil, err
	}
	request, err = s.Supervisor.LaunchMeasurement(ctx, request)
	if err != nil {
		return request, nil, err
	}
	state, err := s.State(jobID)
	return request, state.Score, err
}
func (s *Service) CompleteMeasurement(ctx context.Context, id, fence string) (MeasurementRequest, *quality.Score, error) {
	ffmpeg, _, err := s.ResolveFFmpeg(ctx)
	if err != nil {
		return MeasurementRequest{}, nil, err
	}
	r, err := s.Store.CompleteMeasurement(ctx, id, fence, func(ctx context.Context, file string, rate, channels int) (float64, error) {
		return MeasurePCMLUFS(ctx, s.Runner, ffmpeg, file, rate, channels)
	})
	if err != nil {
		_, _ = s.Store.FailMeasurement(id, fence, RequestError{Code: "measure", Message: err.Error()})
		return r, nil, err
	}
	binding, err := s.CurrentBinding(ctx, r.JobID)
	if err == nil && binding != r.Binding {
		err = renderError(domain.CloneErrStale, "Project đã đổi trong lúc đo — đo lại")
	}
	if err != nil {
		_, _ = s.Store.StaleMeasurement(id, fence, err.Error())
		return r, nil, err
	}
	score, err := s.Score(ctx, r.JobID, &r.Result.LUFS, &r.ID, nil)
	if err != nil {
		_, _ = s.Store.StaleMeasurement(id, fence, err.Error())
		return r, nil, err
	}
	return r, score, nil
}
func (s *Service) Quality(ctx context.Context, projectID string) (editor.Object, error) {
	jobID, err := s.Editor.ExportOwner(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if jobID == "" {
		return editor.Object{"score": nil, "checks": []any{}, "ranges": []any{}, "measuring": false, "items": []any{}}, nil
	}
	state, err := s.State(jobID)
	if err != nil {
		return nil, err
	}
	gate, err := s.Gate(ctx, jobID)
	if err != nil {
		return nil, err
	}
	project, _, err := s.Editor.LoadProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ranges := ScoreRanges(state.Score, project)
	measures, err := s.Store.ListMeasurements(jobID)
	if err != nil {
		return nil, err
	}
	measuring := false
	for _, r := range measures {
		if !r.Terminal() {
			measuring = true
			break
		}
	}
	job, _, _, _, err := s.Inputs(ctx, jobID)
	if err != nil {
		return nil, err
	}
	var snapshot library.Snapshot
	if err = json.Unmarshal(job.TemplateSnapshot, &snapshot); err != nil {
		return nil, err
	}
	channelName := ""
	if s.ChannelName != nil {
		channelName, err = s.ChannelName(ctx, job.ChannelID)
		if err != nil {
			return nil, err
		}
	}
	out := editor.Object{"jobId": jobID, "channelName": channelName, "templateName": snapshot.TemplateName, "score": state.Score, "total": nil, "gate": gate, "ranges": ranges, "measuring": measuring, "changedSinceMeasure": !gate.Fresh, "items": []any{}, "checks": []any{}}
	if state.Score != nil {
		baseline := state.Project
		if !gate.Fresh && baseline != nil {
			binding, err := s.CurrentBinding(ctx, jobID)
			if err != nil {
				return nil, err
			}
			if binding.SettingsHash != state.Score.Binding.SettingsHash || binding.ArtifactHash != state.Score.Binding.ArtifactHash {
				baseline = nil
			}
		}
		stale := qualityStaleItems(state.Score, baseline, project, gate.Fresh)
		items := []editor.Object{}
		for _, item := range state.Score.Items {
			itemRanges := []editor.Object{}
			for _, r := range ranges {
				if r.CheckID == item.ID {
					itemRanges = append(itemRanges, editor.Object{"startSec": r.Start, "endSec": r.End, "cueId": r.CueID, "itemId": r.ItemID, "text": r.Text, "offsetSec": r.OffsetSec})
				}
			}
			items = append(items, editor.Object{"id": item.ID, "status": item.Status, "label": quality.Labels[item.ID], "evidence": item.Evidence, "ranges": itemRanges, "stale": stale[item.ID]})
		}
		out["items"] = items
		out["checks"] = state.Score.Items
		out["total"] = state.Score.Total
		out["measuredAt"] = state.Score.MeasuredAt
	}
	return out, nil
}
func (s *Service) Begin(ctx context.Context, pid, revision string) (editor.Object, error) {
	jobID, err := s.Editor.ExportOwner(ctx, pid)
	if err != nil {
		return nil, err
	}
	if jobID == "" {
		return nil, nil
	}
	if err = s.CheckTemplate(ctx, jobID); err != nil {
		return nil, err
	}
	gate, err := s.Gate(ctx, jobID)
	if err != nil {
		return nil, err
	}
	binding, err := s.CurrentBinding(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if revision != "" && revision != binding.ProjectRevision {
		return nil, &editor.Error{Code: "conflict", Message: "Project đã sửa sau lần đo — đo lại Độ hoàn thiện", Extra: editor.Object{"cloneGate": "revision_mismatch"}}
	}
	session := editor.Object{"cloneJobId": jobID, "jobId": jobID, "binding": binding, "accepted": gate.Accepted, "score": gate.Score, "projectRevision": binding.ProjectRevision}
	var score editor.Object
	if gate.Score != nil {
		raw, _ := json.Marshal(gate.Score)
		_ = json.Unmarshal(raw, &score)
	}
	return editor.EvaluateCloneBeginPolicy(editor.CloneBeginPolicy{Session: session, Score: score, Fresh: gate.Fresh, Accepted: gate.Accepted, StaleReason: reasonString(gate.Reason), Reasons: gate.Reasons})
}
func reasonString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func (s *Service) Commit(ctx context.Context, pid string, session editor.Object, file string, metadata editor.Object) (editor.Object, error) {
	jobID, _ := session["cloneJobId"].(string)
	if jobID == "" {
		return nil, nil
	}
	accepted, _ := session["accepted"].(bool)
	if err := s.CheckTemplate(ctx, jobID); err != nil {
		return nil, err
	}
	actualOwner, err := s.Editor.ExportOwner(ctx, pid)
	if err != nil {
		return nil, err
	}
	if actualOwner != jobID {
		return nil, &editor.Error{Code: "conflict", Message: "Project không còn thuộc job clone", Extra: editor.Object{"cloneGate": "final_check_failed"}}
	}
	raw, _ := json.Marshal(session["binding"])
	var expected quality.Binding
	if err = json.Unmarshal(raw, &expected); err != nil {
		return nil, err
	}
	current, err := s.CurrentBinding(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if current.ProjectRevision != expected.ProjectRevision || current.SettingsHash != expected.SettingsHash || (!accepted && current.ArtifactHash != expected.ArtifactHash) {
		return nil, &editor.Error{Code: "conflict", Message: "Project hoặc ảnh/phụ đề/âm thanh đã thay đổi sau lần đo", Extra: editor.Object{"cloneGate": "final_check_failed"}}
	}
	ffmpeg, ffprobe, err := s.ResolveFFmpeg(ctx)
	if err != nil {
		return nil, err
	}
	facts, err := ProbeMedia(ctx, s.Runner, ffprobe, file)
	if err != nil {
		return nil, err
	}
	if facts.AudioChannels == nil {
		return nil, renderError(domain.CloneErrInvalidRequest, "Bản xuất thiếu âm thanh")
	}
	lufs, err := MeasureLayoutAwareLUFS(ctx, s.Runner, ffmpeg, file, *facts.AudioChannels)
	if err != nil {
		return nil, err
	}
	score, err := s.Score(ctx, jobID, &lufs, nil, &OutputEvidence{Facts: facts, LUFS: &lufs})
	if err != nil {
		return nil, err
	}
	var scoreObject editor.Object
	raw, _ = json.Marshal(score)
	_ = json.Unmarshal(raw, &scoreObject)
	var probed editor.Object
	raw, _ = json.Marshal(facts)
	_ = json.Unmarshal(raw, &probed)
	return editor.EvaluateCloneCommitPolicy(editor.CloneCommitPolicy{JobPresent: true, Accepted: accepted, Probed: probed, Score: scoreObject, ScoreReasons: quality.GateReasons(score)})
}
func Missing(err error) bool {
	return errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}
