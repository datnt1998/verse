package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type StartJobInput struct {
	ChannelID        string
	SourceKey        string
	Source           domain.CloneSourceSnapshot
	TemplateID       string
	TemplateSnapshot json.RawMessage
	ModelRefs        json.RawMessage
	NoImages         bool
	Priority         domain.ClonePriority
	Origin           domain.CloneOrigin
	Replaces         string
	QueueItemID      string
	QueueOverrideRevision int64
	ImageCheckpoint json.RawMessage
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
	RequestCancel(context.Context, string) error
	DeleteJob(context.Context, string) error
	ListArtifacts(context.Context, string) ([]domain.CloneArtifact, error)
	GetArtifact(context.Context, string, string) (*domain.CloneArtifact, error)
	InvalidateSteps(context.Context, string, []domain.CloneStep) error
}

type QueueReadModel interface {
	BoardItems(context.Context, *string) ([]domain.CloneQueueItem, error)
}

// TemplateResolver snapshots the selected recipe and models at admission.
type TemplateResolver func(context.Context, domain.CloneQueueItem) (templateID string, recipe json.RawMessage, models json.RawMessage, err error)

type Service struct {
	Store           ServiceStore
	Worker          *Worker
	Media           *media.Service
	Publish         Publisher
	ResolveTemplate TemplateResolver
	ResolveQueuedImageOverrides func(context.Context, domain.CloneQueueItem) (json.RawMessage, error)
	Queue           QueueReadModel
	Now             func() time.Time
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
	id, recipe, models, err := s.ResolveTemplate(ctx, item)
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
	result, err := s.Store.StartJob(ctx, StartJobInput{ChannelID: item.ChannelID, SourceKey: item.SourceKey, Source: item.Source, TemplateID: id, TemplateSnapshot: recipe, ModelRefs: models, NoImages: item.NoImages, Priority: item.Priority, Origin: item.Origin, QueueItemID: item.ID, QueueOverrideRevision: item.OverrideRevision, ImageCheckpoint: checkpoint})
	if err != nil {
		return nil, err
	}
	if result.Created {
		s.event(domain.CloneEventCreated, result.Job.ID)
	}
	s.Wake()
	return result.Job, nil
}

// Restart creates a new generation while retaining the superseded job's evidence.
func (s *Service) Restart(ctx context.Context, id string) (*StartJobResult, error) {
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
	if s.ResolveTemplate == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có template cho kênh"}
	}
	item := domain.CloneQueueItem{ChannelID: job.ChannelID, SourceVideoID: job.Source.ID, SourceKey: job.SourceKey, Source: job.Source, NoImages: job.NoImages}
	templateID, recipe, models, err := s.ResolveTemplate(ctx, item)
	if err != nil {
		return nil, err
	}
	result, err := s.Store.StartJob(ctx, StartJobInput{ChannelID: job.ChannelID, SourceKey: job.SourceKey, Source: job.Source, TemplateID: templateID, TemplateSnapshot: recipe, ModelRefs: models, NoImages: job.NoImages, Priority: domain.ClonePick, Origin: domain.OriginManual, Replaces: id})
	if err != nil {
		return nil, err
	}
	s.event(domain.CloneEventCreated, result.Job.ID)
	s.event(domain.CloneEventUpdated, id)
	s.Wake()
	return result, nil
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
	return new(s.view(*job, steps)), nil
}
func (s *Service) view(job domain.CloneJob, steps []domain.CloneStepRecord) domain.CloneJobView {
	job.StageGroup = domain.StageGroupOf(job, steps)
	return domain.CloneJobView{CloneJob: job, Steps: steps, AgentView: domain.AgentViews(job, steps), Subline: domain.Subline(job, steps, s.now()), ExportBytes: domain.CloneExportBytes(steps)}
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
			result.Jobs = append(result.Jobs, domain.CloneJobView{CloneJob: domain.CloneJob{ID: item.ID, ChannelID: item.ChannelID, Source: item.Source, NoImages: item.NoImages, StageGroup: domain.StageQueued, Status: domain.ClonePending, Priority: item.Priority, Origin: item.Origin, CreatedAt: item.CreatedAt, UpdatedAt: item.CreatedAt}, QueueItem: new(item), Steps: []domain.CloneStepRecord{}, AgentView: []domain.CloneAgentView{}})
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
	if err := s.Store.RequestCancel(ctx, id); err != nil {
		return nil, err
	}
	if err := s.Store.SetJobStatus(ctx, id, domain.CloneCancelled); err != nil {
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
	if job.Status == domain.CloneRunning || job.Status == domain.ClonePending {
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
