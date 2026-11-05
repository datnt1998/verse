package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// ErrAdmissionTemplateChanged means the prepared snapshot lost its default or
// revision fence before admission. Nothing was persisted; re-resolve once.
var ErrAdmissionTemplateChanged = &domain.CloneError{Code: domain.CloneErrStale, Message: "Mẫu video vừa đổi; xin thử lại."}

type StartJobInput struct {
	ChannelID  string
	SourceKey  string
	Source     domain.CloneSourceSnapshot
	TemplateID string
	// TemplateExplicit: template đến từ override của mục hàng chờ (giữ nguyên
	// dù default kênh đổi), không phải default suy ra lúc lập kế hoạch.
	TemplateExplicit bool
	TemplateSnapshot json.RawMessage
	// SnapshotRevision là templates.revision đã build snapshot; 0 = không kiểm
	// (đường cũ/test trực tiếp). Admission kiểm dưới KEY SHARE.
	SnapshotRevision      int64
	ModelRefs             json.RawMessage
	NoImages              bool
	Priority              domain.ClonePriority
	Origin                domain.CloneOrigin
	Replaces              string
	QueueItemID           string
	QueueOverrideRevision int64
	ImageCheckpoint       json.RawMessage
}
type StartJobResult struct {
	Job     *domain.CloneJob
	Created bool
}

type ServiceStore interface {
	Store
	StartJob(context.Context, StartJobInput) (*StartJobResult, error)
	GetJobBySourceKey(context.Context, string) (*domain.CloneJob, error)
	ListBoardJobs(context.Context, *string, int) ([]domain.CloneJob, error)
	ListJobSteps(context.Context, []string) (map[string][]domain.CloneStepRecord, error)
	CountBoardGroups(context.Context) (map[[2]string]int64, error)
	CountBoardGroupsToday(context.Context, time.Time) (map[string]int64, error)
	ResumeJob(context.Context, string) (*domain.CloneJob, error)
	SetPaused(context.Context, string, bool) error
	// CancelJob yêu cầu huỷ, đóng step mở, bỏ tạm dừng và ghi trạng thái
	// cancelled trong một transaction; Delete dựa vào state đã commit đó.
	CancelJob(context.Context, string) error
	DeleteJob(context.Context, string) error
	ListArtifacts(context.Context, string) ([]domain.CloneArtifact, error)
	GetArtifact(context.Context, string, string) (*domain.CloneArtifact, error)
	InvalidateSteps(context.Context, string, []domain.CloneStep) error
}

type QueueReadModel interface {
	BoardItems(context.Context, *string) ([]domain.CloneQueueItem, error)
}

// TemplateResolver snapshots the selected recipe and models at admission.
type TemplateResolver func(context.Context, domain.CloneQueueItem) (templateID string, recipe json.RawMessage, models json.RawMessage, revision int64, err error)

type Service struct {
	Store                       ServiceStore
	Worker                      *Worker
	Media                       *media.Service
	Publish                     Publisher
	ResolveTemplate             TemplateResolver
	PlanRestart                 func(context.Context, *domain.CloneJob) (*domain.CloneRestartPlan, error)
	EnqueueRestart              func(context.Context, *domain.CloneJob) (*domain.CloneQueueItem, error)
	ResolveQueuedImageOverrides func(context.Context, domain.CloneQueueItem) (json.RawMessage, error)
	Queue                       QueueReadModel
	Now                         func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func (s *Service) Wake() {
	if s.Worker != nil {
		s.Worker.Wake()
	}
}
func (s *Service) event(name, id string) {
	if s.Publish != nil {
		s.Publish(name, map[string]any{"jobId": id})
	}
}

func (s *Service) Start(ctx context.Context, item domain.CloneQueueItem) (*domain.CloneJob, error) {
	if s.ResolveTemplate == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có template cho kênh"}
	}
	for attempt := 0; ; attempt++ {
		id, recipe, models, revision, err := s.ResolveTemplate(ctx, item)
		if err != nil {
			return nil, err
		}
		var checkpoint json.RawMessage
		if s.ResolveQueuedImageOverrides != nil {
			checkpoint, err = s.ResolveQueuedImageOverrides(ctx, item)
			if err != nil {
				return nil, err
			}
		}
		result, err := s.Store.StartJob(ctx, StartJobInput{ChannelID: item.ChannelID, SourceKey: item.SourceKey, Source: item.Source, TemplateID: id, TemplateExplicit: item.TemplateID != nil, TemplateSnapshot: recipe, SnapshotRevision: revision, ModelRefs: models, NoImages: item.NoImages, Priority: item.Priority, Origin: item.Origin, QueueItemID: item.ID, QueueOverrideRevision: item.OverrideRevision, ImageCheckpoint: checkpoint})
		if errors.Is(err, ErrAdmissionTemplateChanged) && attempt == 0 {
			continue
		}
		if err != nil {
			return nil, err
		}
		if result.Created {
			s.event(domain.CloneEventCreated, result.Job.ID)
		}
		s.Wake()
		return result.Job, nil
	}
}

// Restart places a new generation at the head of the normal queue.
func (s *Service) Restart(ctx context.Context, id string) (*domain.CloneQueueItem, error) {
	job, err := s.Store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrNotFound
	}
	if job.SupersededAt != nil || job.Status == domain.ClonePending || job.Status == domain.CloneRunning {
		return nil, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video vẫn đang làm hoặc đã có bản làm lại"}
	}
	if job.ChannelID == "" || job.Source.ID == "" {
		return nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Không xác định được kênh hoặc video nguồn để làm lại"}
	}
	if s.EnqueueRestart == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Hàng chờ chưa sẵn sàng"}
	}
	return s.EnqueueRestart(ctx, job)
}

func (s *Service) RestartPlan(ctx context.Context, id string) (*domain.CloneRestartPlan, error) {
	job, err := s.Store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrNotFound
	}
	if job.SupersededAt != nil || job.Status == domain.ClonePending || job.Status == domain.CloneRunning {
		return nil, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video vẫn đang làm hoặc đã có bản làm lại"}
	}
	if s.PlanRestart == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Chưa có kế hoạch làm lại"}
	}
	return s.PlanRestart(ctx, job)
}
func (s *Service) Get(ctx context.Context, id string) (*domain.CloneJobView, error) {
	job, err := s.Store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	steps, err := s.Store.ListSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	steps = domain.InChainOrder(steps)
	if err := s.enrichSource(ctx, job, steps); err != nil {
		return nil, err
	}
	view := s.view(*job, steps)
	if previous, ok := s.Store.(interface {
		PreviousCloneJobID(context.Context, string, int) (*string, error)
	}); ok {
		view.PreviousJobID, err = previous.PreviousCloneJobID(ctx, job.SourceKey, job.Generation)
		if err != nil {
			return nil, err
		}
	}
	return &view, nil
}
func (s *Service) view(job domain.CloneJob, steps []domain.CloneStepRecord) domain.CloneJobView {
	job.StageGroup = domain.StageGroupOf(job, steps)
	return domain.CloneJobView{CloneJob: job, Steps: steps, AgentView: domain.AgentViews(job, steps), Subline: domain.Subline(job, steps, s.now()), ExportBytes: domain.CloneExportBytes(steps)}
}

// PreviewSourceJob là hàng nhẹ cho bộ chọn nguồn xem thử: job đã xong, còn
// hiệu lực và đã publish đủ artifact nguồn. Không nạp step/checkpoint của cả
// bảng như List (trang template hỏi lại mỗi 1,5 giây).
type PreviewSourceJob struct {
	ID          string
	Title       string
	ChannelName string
	DurationSec float64
	// NoImages: video nguồn đã chọn "không vẽ ảnh" — preview cần ảnh cảnh của
	// recipe sẽ không dựng được từ video này.
	NoImages bool
}

// PreviewSourceArtifacts là các artifact bắt buộc để một job làm nguồn xem thử.
var PreviewSourceArtifacts = []string{"scenes/manifest.json", "images/manifest.json", "transcript/captions.json", "build/source-audio.m4a"}

// FinishedPreviewSources trả nguồn xem thử bằng truy vấn nhẹ (không step).
// Store không hỗ trợ thì lùi về List để test cũ vẫn chạy.
func (s *Service) FinishedPreviewSources(ctx context.Context) ([]PreviewSourceJob, error) {
	if store, ok := s.Store.(interface {
		ListPreviewSourceJobs(context.Context) ([]PreviewSourceJob, error)
	}); ok {
		return store.ListPreviewSourceJobs(ctx)
	}
	list, err := s.List(ctx, nil, 10000)
	if err != nil {
		return nil, err
	}
	out := make([]PreviewSourceJob, 0)
	for _, job := range list.Jobs {
		if job.Status != domain.CloneSucceeded || job.SupersededAt != nil || job.QueueItem != nil || !hasSourceArtifacts(job.Steps) {
			continue
		}
		source := PreviewSourceJob{ID: job.ID, Title: job.Source.Title, NoImages: job.NoImages}
		if job.Source.ChannelTitle != nil {
			source.ChannelName = *job.Source.ChannelTitle
		}
		if job.Source.DurationSec != nil {
			source.DurationSec = *job.Source.DurationSec
		}
		out = append(out, source)
	}
	return out, nil
}

func hasSourceArtifacts(steps []domain.CloneStepRecord) bool {
	found := make(map[string]bool, len(PreviewSourceArtifacts))
	for _, step := range steps {
		if step.Output == nil {
			continue
		}
		for _, artifact := range step.Output.Artifacts {
			for _, want := range PreviewSourceArtifacts {
				if artifact.Path == want || strings.HasSuffix(artifact.Path, "/"+want) {
					found[want] = true
				}
			}
		}
	}
	for _, want := range PreviewSourceArtifacts {
		if !found[want] {
			return false
		}
	}
	return true
}
func (s *Service) List(ctx context.Context, channelID *string, limit int) (domain.CloneJobList, error) {
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}
	jobs, err := s.Store.ListBoardJobs(ctx, channelID, limit)
	if err != nil {
		return domain.CloneJobList{}, err
	}
	result := domain.CloneJobList{Jobs: make([]domain.CloneJobView, 0, len(jobs))}
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	byJob, err := s.Store.ListJobSteps(ctx, ids)
	if err != nil {
		return result, err
	}
	for _, job := range jobs {
		result.Jobs = append(result.Jobs, s.view(job, domain.InChainOrder(byJob[job.ID])))
	}
	if s.Queue != nil {
		items, err := s.Queue.BoardItems(ctx, channelID)
		if err != nil {
			return result, err
		}
		result.QueuedTotal = len(items)
		for _, item := range items {
			result.Jobs = append(result.Jobs, domain.CloneJobView{CloneJob: domain.CloneJob{ID: item.ID, ChannelID: item.ChannelID, SourceKey: item.SourceKey, Source: item.Source, NoImages: item.NoImages, StageGroup: domain.StageQueued, Status: domain.ClonePending, Priority: item.Priority, Origin: item.Origin, CreatedAt: item.CreatedAt, UpdatedAt: item.CreatedAt}, QueueItem: new(item), PreviousJobID: item.RestartOf, Steps: []domain.CloneStepRecord{}, AgentView: []domain.CloneAgentView{}})
		}
	}
	return result, nil
}
func (s *Service) Counts(ctx context.Context) (domain.CloneCounts, error) {
	counts := domain.CloneCounts{ByOwner: map[string]domain.CloneBucketCounts{}}
	if counter, ok := s.Store.(interface {
		ApprovedToday(context.Context, string) (int, error)
	}); ok {
		count, err := counter.ApprovedToday(ctx, "")
		if err != nil {
			return counts, err
		}
		counts.ApprovedToday = &count
	}
	groups, err := s.Store.CountBoardGroups(ctx)
	if err != nil {
		return counts, err
	}
	for key, n := range groups {
		owner := key[0]
		if owner == "" {
			owner = "none"
		}
		c := counts.ByOwner[owner]
		addCount(&c, key[1], int(n))
		counts.ByOwner[owner] = c
		addCount(&counts.Totals, key[1], int(n))
	}
	now := s.now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today, err := s.Store.CountBoardGroupsToday(ctx, midnight)
	if err != nil {
		return counts, err
	}
	for owner, n := range today {
		if owner == "" {
			owner = "none"
		}
		c := counts.ByOwner[owner]
		c.DoneToday = int(n)
		counts.ByOwner[owner] = c
		counts.Totals.DoneToday += int(n)
	}
	if s.Queue != nil {
		items, err := s.Queue.BoardItems(ctx, nil)
		if err != nil {
			return counts, err
		}
		for _, item := range items {
			c := counts.ByOwner[item.ChannelID]
			c.Queued++
			counts.ByOwner[item.ChannelID] = c
			counts.Totals.Queued++
		}
	}
	return counts, nil
}
func addCount(c *domain.CloneBucketCounts, g string, n int) {
	switch g {
	case domain.StageQueued:
		c.Queued += n
	case domain.StageWorking:
		c.Working += n
	case domain.StageReview:
		c.Review += n
	case domain.StageAttention:
		c.Attention += n
	case domain.StagePaused:
		c.Paused += n
	case domain.StageReady:
		c.Ready += n
	case domain.StagePosted:
		c.Posted += n
	}
}
func (s *Service) Run(ctx context.Context, id string) (*domain.CloneJob, error) {
	return s.Resume(ctx, id)
}
func (s *Service) Pause(ctx context.Context, id string) (*domain.CloneJob, error) {
	if _, err := s.Store.GetJob(ctx, id); err != nil {
		return nil, err
	}
	if err := s.Store.SetPaused(ctx, id, true); err != nil {
		return nil, err
	}
	s.event(domain.CloneEventUpdated, id)
	return s.Store.GetJob(ctx, id)
}
func (s *Service) Resume(ctx context.Context, id string) (*domain.CloneJob, error) {
	job, err := s.Store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.SupersededAt != nil {
		return nil, ErrConflict
	}
	job, err = s.Store.ResumeJob(ctx, id)
	if err != nil {
		return nil, err
	}
	s.event(domain.CloneEventUpdated, id)
	s.Wake()
	return job, nil
}
func (s *Service) Cancel(ctx context.Context, id string) (*domain.CloneJob, error) {
	if _, err := s.Store.GetJob(ctx, id); err != nil {
		return nil, err
	}
	// Huỷ là một bước nguyên tử: request + đóng step + bỏ tạm dừng + trạng thái
	// cancelled cùng commit, nên Delete gọi ngay sau đó thấy state đã dừng hẳn.
	if err := s.Store.CancelJob(ctx, id); err != nil {
		return nil, err
	}
	s.event(domain.CloneEventUpdated, id)
	return s.Store.GetJob(ctx, id)
}
func (s *Service) Delete(ctx context.Context, id string) error {
	job, err := s.Store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	// Tạm dừng không phải một CloneStatus (cột paused_at + stage paused), nên
	// phải chặn riêng: video còn dừng giữ step/lease, xoá lúc đó sẽ cắt ngang
	// phần đang giữ. UI đã khoá nút và yêu cầu dừng hẳn trước.
	if job.PausedAt != nil || job.Status == domain.CloneRunning || job.Status == domain.ClonePending {
		return ErrConflict
	}
	artifacts, err := s.Store.ListArtifacts(ctx, id)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		paths = append(paths, a.Path)
	}
	if s.Media == nil {
		return errors.New("media service unavailable")
	}
	var cutouts []string
	err = s.Media.Unbind(ctx, paths, func(txctx context.Context) error {
		if store, ok := s.Store.(interface {
			DeleteJobWithCutouts(context.Context, string) ([]string, error)
		}); ok {
			var err error
			cutouts, err = store.DeleteJobWithCutouts(txctx, id)
			return err
		}
		return s.Store.DeleteJob(txctx, id)
	})
	if err == nil {
		retireLocalCutouts(ctx, s.Media, cutouts)
		s.event(domain.CloneEventDeleted, id)
	}
	return err
}
func (s *Service) Step(ctx context.Context, id string, step domain.CloneStep) (*domain.CloneStepRecord, error) {
	if !domain.IsCloneStep(string(step)) {
		return nil, ErrNotFound
	}
	return s.Store.GetStep(ctx, id, step)
}
func (s *Service) Artifact(ctx context.Context, id, artifactID string) (*domain.CloneArtifact, error) {
	return s.Store.GetArtifact(ctx, id, artifactID)
}

// Edit invalidates the edited step and all transitive dependents atomically.
func (s *Service) Edit(ctx context.Context, id string, step domain.CloneStep) error {
	if !domain.IsCloneStep(string(step)) {
		return ErrNotFound
	}
	steps := append([]domain.CloneStep{step}, domain.DependentsOf(step)...)
	if err := s.Store.InvalidateSteps(ctx, id, steps); err != nil {
		return err
	}
	s.event(domain.CloneEventUpdated, id)
	s.Wake()
	return nil
}
