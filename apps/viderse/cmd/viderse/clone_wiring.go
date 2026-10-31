package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
	stepbackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/build"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/download"
	stepexport "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/export"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	stepscore "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/score"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/platform/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

type cloneHandler func(context.Context, *pipeline.StepContext) (pipeline.StepResult, error)

func (f cloneHandler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	return f(ctx, s)
}

type cloneSettings struct{ store *postgres.SettingsStore }

func (s cloneSettings) read(key string, def int) int {
	var n int
	found, err := s.store.Get(context.Background(), key, &n)
	if err != nil || !found {
		return def
	}
	return n
}
func (s cloneSettings) JobConcurrency() int {
	return s.read(domain.KeyJobConcurrency, domain.JobConcurrencyDefault)
}
func (s cloneSettings) ImageConcurrency() int {
	return s.read(domain.KeyImageConcurrency, domain.ImageConcurrencyDefault)
}

type cloneStatusSource struct{ queue *queue.Service }

func (s cloneStatusSource) Report(ctx context.Context) (status.Report, error) {
	counts, err := s.queue.Counts(ctx)
	if err != nil {
		return status.Report{}, err
	}
	return status.Report{Running: counts.Totals.Working, Queued: counts.Totals.Queued, ReviewCount: counts.Totals.Review}, nil
}

type cloneQueueBoard struct{ store *postgres.QueueStore }

func (q cloneQueueBoard) BoardItems(ctx context.Context, channelID *string) ([]domain.CloneQueueItem, error) {
	items, err := q.store.ListQueuedItems(ctx)
	if err != nil {
		return nil, err
	}
	if channelID == nil || *channelID == "" || *channelID == "all" {
		return items, nil
	}
	out := make([]domain.CloneQueueItem, 0)
	for _, item := range items {
		if item.ChannelID == *channelID {
			out = append(out, item)
		}
	}
	return out, nil
}

type cloneText struct{ models []*llm.ClientModel }

func (g cloneText) Generate(ctx context.Context, m []llm.Message, t []llm.ToolSpec, o ...llm.CallOption) (*llm.LLMResponse, error) {
	var last error
	for _, model := range g.models {
		result, err := model.Generate(ctx, m, t, o...)
		if err == nil {
			return result, nil
		}
		last = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if last == nil {
		last = &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Chưa chọn model Đạo diễn"}
	}
	return nil, last
}

func mountClonePipeline(ctx context.Context, g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, statusSvc *status.Service, cfgDir string, toolKit *tools.Tools, toolDeps tools.Deps, ai *aiWiring, librarySvc *library.Service, backdropSvc *backdrop.Service, files *media.Service, scans *scan.Service, origin string) (func() error, error) {
	store := postgres.NewCloneStore(pool)
	queueStore := postgres.NewQueueStore(pool)
	rules := cloneSettings{settings}
	publish := func(name string, data any) { hub.Publish(events.Event{Type: name, Data: data}); statusSvc.Bump() }
	service := &pipeline.Service{Store: store, Media: files, Publish: publish, Queue: cloneQueueBoard{queueStore}}
	edits := cloneOwnerEdits(store, files, publish, service.Wake)
	service.ResolveTemplate = func(ctx context.Context, item domain.CloneQueueItem) (string, json.RawMessage, json.RawMessage, error) {
		owner, err := queueStore.Owner(ctx, item.ChannelID)
		if err != nil {
			return "", nil, nil, err
		}
		if owner == nil {
			return "", nil, nil, pipeline.ErrNotFound
		}
		templates, err := queueStore.Templates(ctx, item.ChannelID)
		if err != nil {
			return "", nil, nil, err
		}
		if len(templates) == 0 {
			return "", nil, nil, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Kênh chưa có template"}
		}
		cursor := owner.TemplateCursor % len(templates)
		if cursor < 0 {
			cursor = 0
		}
		id := templates[cursor].ID
		if item.TemplateID != nil {
			id = *item.TemplateID
		}
		snap, err := librarySvc.SnapshotTemplate(ctx, id)
		if err != nil {
			return "", nil, nil, err
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			return "", nil, nil, err
		}
		models, err := ai.Service.Agents(ctx)
		if err != nil {
			return "", nil, nil, err
		}
		modelJSON, err := json.Marshal(models)
		return id, raw, modelJSON, err
	}
	generator := func(s *pipeline.StepContext) (cloneText, error) {
		var refs domain.AgentModels
		if err := json.Unmarshal(s.Job.ModelRefs, &refs); err != nil {
			return cloneText{}, err
		}
		chain := append([]domain.ModelRef{refs.Director.Model}, refs.Director.Fallback...)
		if s.Step.Step == domain.CloneImages {
			chain = []domain.ModelRef{refs.Artist.PromptModel}
		}
		return cloneText{ai.Executors.textModels(ctx, chain)}, nil
	}
	ff := func(ctx context.Context) (*tools.FfmpegResolved, error) {
		r := toolKit.Ffmpeg.Resolve(ctx)
		if r == nil {
			return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Thiếu FFmpeg"}
		}
		return r, nil
	}
	editorService := newEditorService(pool, files, cfgDir, toolKit, toolDeps)
	editorService.Store.Templates = librarySvc
	activityService := activity.New(postgres.NewActivityStore(pool), web.Notifier(hub, "activity"))
	renderRuntime := mountRender(ctx, g, store, files, editorService, cfgDir, origin, toolKit, toolDeps, activityService)
	renderRuntime.ChannelName = func(ctx context.Context, id string) (string, error) {
		owner, err := queueStore.Owner(ctx, id)
		if err != nil || owner == nil {
			return "", err
		}
		return owner.Name, nil
	}
	tr := tools.NewTranscription(toolDeps, toolKit.Uv, nil)
	metadataRun := func(ctx context.Context, s *pipeline.StepContext, parts []string, direction string) (pipeline.StepResult, error) {
		model, err := generator(s)
		if err != nil { return pipeline.StepResult{}, err }
		return (metadata.Handler{Deps: metadata.Deps{Model: model, Selected: parts, Direction: direction, OwnerChannel: func(ctx context.Context, job domain.CloneJob) (*metadata.ChannelView, error) {
			owner, err := queueStore.Owner(ctx, job.ChannelID)
			if err != nil || owner == nil { return nil, err }
			note, footer := "", ""
			if owner.StyleNote != nil { note = *owner.StyleNote }
			if owner.DescriptionFooter != nil { footer = *owner.DescriptionFooter }
			return &metadata.ChannelView{ID: owner.ID, Name: owner.Name, StyleNote: note, Language: owner.Language, Footer: footer, AlwaysTags: owner.AlwaysTags}, nil
		}}}).Run(ctx, s)
	}
	registry := pipeline.Registry{
		domain.CloneScore: cloneHandler(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			result, err := (stepscore.Handler{Deps: stepscore.Deps{Measure: renderRuntime.ScoreForPipeline, Accepted: func(ctx context.Context, id string) (bool, error) {
				state, err := renderRuntime.State(id)
				return state.AcceptedAt != nil, err
			}}}).Run(ctx, s)
			if err != nil {
				_ = renderRuntime.FailQueuedExports(s.Job.ID, err)
			}
			return result, err
		}),
		domain.CloneExport:     stepexport.Handler{Deps: stepexport.Deps{Gate: renderRuntime.Gate, Export: renderRuntime.RunExport}},
		domain.CloneDownload:   download.New(&download.Deps{Tools: toolDeps, ResolveFfmpeg: toolKit.Ffmpeg.Resolve}),
		domain.CloneBackdrop:   stepbackdrop.New(stepbackdrop.ProductionDeps(postgres.NewBackdropStore(pool), backdropSvc)),
		domain.CloneTranscript: transcript.New(&transcript.Deps{ResolveFfmpeg: toolKit.Ffmpeg.Resolve, ResolveUv: toolKit.Uv.Resolve, TranscriptionStatus: tr.Status, RunEnv: tr.RunEnv()}),
		domain.CloneMetadata: cloneHandler(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			return metadataRun(ctx, s, nil, "")
		}),
		domain.CloneScenes: cloneHandler(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			resolved, err := ff(ctx)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			return (scenes.Handler{Deps: scenes.Deps{Ffmpeg: resolved.Ffmpeg, FFprobe: resolved.Ffprobe}}).Run(ctx, s)
		}),
		domain.CloneImages: cloneHandler(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			resolved, err := ff(ctx)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			model, err := generator(s)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			var refs domain.AgentModels
			if err := json.Unmarshal(s.Job.ModelRefs, &refs); err != nil {
				return pipeline.StepResult{}, err
			}
			chain := []domain.ModelRef{refs.Artist.Draw}
			if refs.Artist.FallbackOn {
				chain = append(chain, refs.Artist.DrawFallback...)
			}
			providers, _ := ai.Executors.imageProviders(ctx, chain)
			limit := ai.Executors.limit
			fallback := refs.Artist.FallbackOn
			drawer := imagegen.NewService(imagegen.Options{Providers: providers, Fallback: &fallback, Journal: ai.Executors.journal, DailyPaidImageLimit: &limit})
			return (images.Handler{Deps: images.Deps{Drawer: drawer, Model: model, Ffmpeg: resolved.Ffmpeg, FFprobe: resolved.Ffprobe, ReadManifest: cloneReadImageManifest(edits), Look: func(ctx context.Context, job domain.CloneJob) (string, error) {
				owner, err := queueStore.Owner(ctx, job.ChannelID)
				if err != nil || owner == nil {
					return "", err
				}
				return owner.Branding.Look, nil
			}}}).Run(ctx, s)
		}),
		domain.CloneBuild: cloneHandler(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			resolved, err := ff(ctx)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			return (build.Handler{Deps: build.Deps{Ffmpeg: resolved.Ffmpeg, FFprobe: resolved.Ffprobe}}).Run(ctx, s)
		}),
	}
	slots := queue.NewResourceSlots()
	q := queue.New(queue.Options{Pool: pool, Store: postgres.NewQueueStore(pool), Runner: service, Settings: settings, Probe: queue.YouTubeProbe(youtube.NewClient(youtube.Options{})),
		Scan: func(ctx context.Context, id string) error {
			_, err := scans.Control(ctx, id, domain.ScanActionCheck)
			return err
		},
		ImageSources: func(ctx context.Context) ([]domain.QueueImageQuota, error) {
			artist, err := ai.Service.ResolveArtist(ctx)
			if err != nil {
				return nil, err
			}
			providers, _ := ai.Executors.imageProviders(ctx, artist.DrawChain)
			quotas := make([]domain.QueueImageQuota, 0, len(providers))
			seen := map[string]bool{}
			for n, p := range providers {
				if seen[p.Name()] {
					continue
				}
				seen[p.Name()] = true
				role := "fallback"
				if n == 0 {
					role = "main"
				}
				quota := domain.QueueImageQuota{Provider: p.Name(), Period: "day", Role: role}
				if p.Paid() {
					limit := int64(ai.Executors.limit)
					quota.Limit = &limit
					now := time.Now()
					reset := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
					quota.ResetsAt = &reset
				}
				ai.Executors.mu.RLock()
				drawer := ai.Executors.drawer
				ai.Executors.mu.RUnlock()
				if drawer != nil {
					if reset, ok := drawer.QuotaReset(p.Name()); ok {
						quota.ResetsAt = &reset
					}
				}
				quotas = append(quotas, quota)
			}
			return quotas, nil
		}, Notify: func(name string) { publish(name, map[string]any{}) }, Diag: func(err error) { fmt.Fprintln(os.Stderr, "Hàng chờ:", err) }})
	for step, handler := range registry {
		handler = slots.Wrap(step, handler)
		if step == domain.CloneImages {
			inner := handler
			handler = pipeline.HandlerFunc(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
				s.ImageSlot = q.WithImageSlot(slots)
				return inner.Run(ctx, s)
			})
		}
		registry[step] = handler
	}
	runner := &pipeline.Runner{Store: store, Handlers: registry, Media: service.Media, Runner: toolDeps.Runner, Publish: publish, StagingRoot: filepath.Join(cfgDir, "clone-staging"), Settings: rules, Queue: q}
	runner.OnHumanState = func(ctx context.Context, job *domain.CloneJob, kind string) {
		text, label := "", "Mở"
		switch kind {
		case "review_ready": text, label = "Video chờ duyệt", "Duyệt ảnh"
		case "needs_attention": text = "Cần xử lý: " + job.Source.Title
		case "ready_to_post": text = "Video sẵn sàng đăng"
		default: return
		}
		spec := activity.NotificationSpec{Kind:kind, Text:text, Href:"#/?video="+job.ID, ActionLabel:label}
		if kind != "review_ready" { spec.Object = &domain.ObjectRef{Type:"video", ID:job.ID, Label:job.Source.Title} }
		if _, err := activityService.NotifyScoped(ctx, spec); err != nil { logStore := fmt.Sprintf("notification %s: %v", kind, err); fmt.Fprintln(os.Stderr, logStore) }
	}
	worker := &pipeline.Worker{Store: store, Runner: runner, Settings: rules, ID: fmt.Sprintf("%d", os.Getpid()), OnPrimary: func(ctx context.Context) error {
		if err := renderRuntime.Recover(ctx); err != nil {
			return err
		}
		return q.Start(ctx)
	}, OnStopped: q.Stop}
	service.Worker = worker
	statusSvc.RegisterSource(cloneStatusSource{queue: q})
	web.MountQueueDiag(g, q, func(err error) { fmt.Fprintln(os.Stderr, "Hàng chờ:", err) })
	web.MountRecap(g, &insight.Service{Store: postgres.NewActivityStore(pool), Forecast: func(ctx context.Context, now time.Time) (int, *time.Time, error) {
		view, err := q.List(ctx)
		if err != nil { return 0, nil, err }
		end := queue.LocalMidnight(now).AddDate(0, 0, 1)
		count := 0
		var last *time.Time
		for _, item := range view.Items {
			if item.ETADone == nil || !item.ETADone.Before(end) { continue }
			count++
			if last == nil || item.ETADone.After(*last) { at := *item.ETADone; last = &at }
		}
		return count, last, nil
	}})
	web.MountClone(g, service, q, service.Media)
	metadataService := &clonemetadata.Service{Edits: edits, Generate: metadataRewrite(edits, metadataRun), Channel: queueStore.Owner}
	worker.OnPoll = mountCloneOwnerOperations(g, pool, edits, service, ai, librarySvc, backdropSvc, toolKit, toolDeps, editorService, renderRuntime.ChannelName, metadataService)
	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- worker.Run(workerCtx) }()
	var once sync.Once
	var stopped error
	return func() error { once.Do(func() { stop(); stopped = <-done }); return stopped }, nil
}
