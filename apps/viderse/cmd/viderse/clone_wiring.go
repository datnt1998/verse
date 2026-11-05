package main

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

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
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
	"github.com/datnt1998/verse/apps/viderse/internal/preview"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmretry"
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
	return domain.ImageConcurrencyDefault
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

func mountClonePipeline(ctx context.Context, g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, statusSvc *status.Service, cfgDir string, toolKit *tools.Tools, toolDeps tools.Deps, ai *aiWiring, librarySvc *library.Service, backdropSvc *backdrop.Service, files *media.Service, scans *scan.Service, origin, browserOrigin string, suggestions *suggestRuntime) (func() error, error) {
	store := postgres.NewCloneStore(pool)
	queueStore := postgres.NewQueueStore(pool)
	rules := cloneSettings{settings}
	publish := func(name string, data any) { hub.Publish(events.Event{Type: name, Data: data}); statusSvc.Bump() }
	service := &pipeline.Service{Store: store, Media: files, Publish: publish, Queue: cloneQueueBoard{queueStore}}
	service.ResolveQueuedImageOverrides = func(ctx context.Context, item domain.CloneQueueItem) (json.RawMessage, error) {
		return queuedImageCheckpoint(ctx, librarySvc, item)
	}
	edits := cloneOwnerEdits(store, files, publish, service.Wake)
	service.ResolveTemplate = func(ctx context.Context, item domain.CloneQueueItem) (string, json.RawMessage, json.RawMessage, int64, error) {
		owner, err := queueStore.Owner(ctx, item.ChannelID)
		if err != nil {
			return "", nil, nil, 0, err
		}
		if owner == nil {
			return "", nil, nil, 0, pipeline.ErrNotFound
		}
		// Candidate: override của mục hàng chờ, ngược lại default hiện tại của
		// kênh. Chỉ là gợi ý — admission chốt lại dưới khoá hàng kênh.
		id := owner.DefaultTemplateID
		if item.TemplateID != nil {
			id = *item.TemplateID
		}
		if id == "" {
			return "", nil, nil, 0, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Kênh chưa có template mặc định"}
		}
		snap, err := librarySvc.SnapshotTemplate(ctx, id)
		if err != nil {
			return "", nil, nil, 0, err
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			return "", nil, nil, 0, err
		}
		models, err := ai.Service.Agents(ctx)
		if err != nil {
			return "", nil, nil, 0, err
		}
		modelJSON, err := json.Marshal(models)
		return id, raw, modelJSON, snap.Revision, err
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
	// Nhập template cũ (POST /api/templates/import) cần mặt media của editor;
	// service Thư viện dựng trước service Editor nên gắn ở đây, một lần, trước
	// khi listener nhận request.
	librarySvc.BindMediaImporter(editorService)
	activityService := activity.New(postgres.NewActivityStore(pool), web.Notifier(hub, "activity"))
	librarySvc.BindMediaReader(editorService)
	// Bản xem thử dùng chính engine Fill và project registry của editor; không
	// có renderer/provider riêng. Gắn trước khi listener nhận request.
	previewSvc := preview.New(preview.Options{
		Templates: librarySvc,
		Jobs:      service,
		Editor:    editorService,
		Media:     files,
		Runner:    toolDeps.Runner,
		Ffmpeg: func(ctx context.Context) (string, bool) {
			resolved := toolKit.Ffmpeg.Resolve(ctx)
			if resolved == nil {
				return "", false
			}
			return resolved.Ffmpeg, true
		},
		Tasks: activityService,
	})
	librarySvc.BindPreviewBuilder(previewSvc)
	if err := previewSvc.Reconcile(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Dọn bản xem thử:", err)
	}
	renderRuntime := mountRender(ctx, g, store, files, editorService, cfgDir, origin, browserOrigin, toolKit, toolDeps, activityService)
	renderRuntime.ChannelName = func(ctx context.Context, id string) (string, error) {
		owner, err := queueStore.Owner(ctx, id)
		if err != nil || owner == nil {
			return "", err
		}
		return owner.Name, nil
	}
	// Huỷ lần vẽ ảnh: tầng API và worker dùng chung một sổ trong tiến trình.
	cancels := images.NewCancelRegistry()
	binder := &cloneimages.ProjectBinder{Edits: edits, Editor: editorService, Measure: func(ctx context.Context, jobID string) error {
		_, _, err := renderRuntime.Measure(ctx, jobID)
		return err
	}}
	imageRebind := func(ctx context.Context, job domain.CloneJob, req images.RebindRequest) error {
		_, err := binder.Rebind(ctx, cloneimages.RebindRequest{JobID: job.ID, ImageID: req.ImageID, SceneID: req.SceneID, Kind: req.Kind, Version: req.Version, Path: req.Path, SHA256: req.SHA256, Mime: req.Mime, Width: req.Width, Height: req.Height, Cutout: req.Cutout, ApplyProject: req.ApplyProject})
		return err
	}
	imageBuilt := func(ctx context.Context, job domain.CloneJob) (string, bool, error) {
		snapshot, err := edits.Read(ctx, job.ID, domain.CloneBuild)
		if err != nil {
			var cloneErr *domain.CloneError
			if errors.As(err, &cloneErr) && cloneErr.Code == domain.CloneErrNotFound {
				return "", false, nil
			}
			return "", false, err
		}
		if snapshot.Step.Status != domain.CloneSucceeded || snapshot.Step.Output == nil {
			return "", false, nil
		}
		projectID, _ := snapshot.Step.Output.Data["projectId"].(string)
		return projectID, projectID != "", nil
	}
	imagePromptSource := func(ctx context.Context, job domain.CloneJob, overrides *images.Overrides, kind string) images.PromptPartSource {
		id := ""
		if overrides != nil {
			if kind == "thumbnail" {
				id = overrides.CoverPromptID
			} else {
				id = overrides.ScenePromptID
			}
		}
		label := "Prompt ảnh cảnh"
		if kind == "thumbnail" {
			label = "Prompt ảnh bìa"
		}
		source := images.PromptPartSource{Type: "image_prompt", ID: id, Label: label}
		if id == "" {
			source.Type = "template"
			source.Label = "Prompt mặc định của template"
			return source
		}
		if prompt, err := librarySvc.GetPrompt(ctx, id); err == nil && prompt != nil && prompt.Name != "" {
			source.Label = "prompt \"" + prompt.Name + "\""
		}
		return source
	}
	tr := tools.NewTranscription(toolDeps, toolKit.Uv, nil)
	metadataRun := func(ctx context.Context, s *pipeline.StepContext, parts []string, direction string) (pipeline.StepResult, error) {
		model, err := generator(s)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		return (metadata.Handler{Deps: metadata.Deps{Model: model, Selected: parts, Direction: direction, OwnerChannel: func(ctx context.Context, job domain.CloneJob) (*metadata.ChannelView, error) {
			owner, err := queueStore.Owner(ctx, job.ChannelID)
			if err != nil || owner == nil {
				return nil, err
			}
			note, footer := "", ""
			if owner.StyleNote != nil {
				note = *owner.StyleNote
			}
			if owner.DescriptionFooter != nil {
				footer = *owner.DescriptionFooter
			}
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
			return (scenes.Handler{Deps: scenes.Deps{Ffmpeg: resolved.Ffmpeg, FFprobe: resolved.Ffprobe, ReadNoImagesOverride: func(ctx context.Context, job domain.CloneJob) (*bool, error) {
				manifest, err := cloneReadImageManifest(edits)(ctx, job)
				if err != nil || manifest == nil || manifest.Overrides == nil {
					return nil, err
				}
				return manifest.Overrides.NoImages, nil
			}}}).Run(ctx, s)
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
			chain = withVideoDrawSource(ctx, edits, s.Job, chain)
			limit := ai.Executors.limit
			fallback := refs.Artist.FallbackOn
			drawerFor := func(ctx context.Context, chain []domain.ModelRef) images.Drawer {
				providers, _ := ai.Executors.imageProviders(ctx, chain)
				return imagegen.NewService(imagegen.Options{Providers: providers, Fallback: &fallback, Journal: ai.Executors.journal, DailyPaidImageLimit: &limit})
			}
			providerLimit := func(ctx context.Context, job domain.CloneJob, provider string) int {
				if provider == "" {
					return imagegen.RefLimit(chain[0].Provider)
				}
				return imagegen.RefLimit(provider)
			}
			return (images.Handler{Deps: images.Deps{Drawer: drawerFor(ctx, chain), Model: model, Ffmpeg: resolved.Ffmpeg, FFprobe: resolved.Ffprobe, ReadManifest: cloneReadImageManifest(edits), Cancels: cancels,
				RefLimit: providerLimit, PromptSource: imagePromptSource, Built: imageBuilt, Rebind: imageRebind,
				// Prompt viết cho nguồn sẽ vẽ: nguồn riêng của ảnh, không có thì
				// đầu chuỗi — chỉ Codex có chỉ dẫn công cụ đi kèm.
				PromptProvider: func(ctx context.Context, job domain.CloneJob, provider string) string {
					if provider != "" {
						return provider
					}
					return chain[0].Provider
				},
				AcquireUpload: func(ctx context.Context, id string) (string, func(), error) {
					handle, err := editorService.AcquireMedia(ctx, id)
					if err != nil {
						return "", nil, err
					}
					return handle.Path, handle.Release, nil
				},
				DrawerFor: func(ctx context.Context, job domain.CloneJob, provider string) (images.Drawer, error) {
					selected := []domain.ModelRef{}
					for _, ref := range chain {
						if strings.EqualFold(ref.Provider, provider) {
							selected = append(selected, ref)
						}
					}
					if len(selected) == 0 {
						selected = append(selected, domain.ModelRef{Provider: provider})
					}
					return drawerFor(ctx, selected), nil
				},
				Look: func(ctx context.Context, job domain.CloneJob) (string, error) {
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
		// Trinh sát dùng chuỗi text Đạo diễn đã cấu hình để hiểu chỉ dẫn Tự
		// lấy thành bộ lọc nguồn; không có model/scout riêng.
		ScoutModel: func(ctx context.Context) (llmretry.Generator, error) {
			chain, err := ai.Service.ResolveDirector(ctx)
			if err != nil {
				return nil, err
			}
			return cloneText{ai.Executors.textModels(ctx, chain)}, nil
		},
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
				quota := domain.QueueImageQuota{Provider: p.Name(), Label: imagegen.DisplayLabel(p), Period: "day", Role: role}
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
	suggestions.Queue = q
	closeCoverTrials := mountCoverTrials(coverTrialDeps{
		Pool: pool, Library: librarySvc, Jobs: service, Media: files, Tasks: activityService,
		Preview: previewSvc, AI: ai, Slot: q.WithImageSlot(slots),
	})
	service.PlanRestart = q.RestartPlan
	service.EnqueueRestart = q.EnqueueRestart
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
	autonomySvc := newAutonomyService(pool, publish, func(err string) { fmt.Fprintln(os.Stderr, "Tự duyệt/tự sửa:", err) })
	runner := &pipeline.Runner{Store: store, Handlers: registry, Media: service.Media, Runner: toolDeps.Runner, Publish: publish, StagingRoot: filepath.Join(cfgDir, "clone-staging"), Settings: rules, Queue: q, AutoFix: autonomySvc}
	runner.OnHumanState = func(_ context.Context, _ *domain.CloneJob, _ string) {
		activityService.Changed("")
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
	web.MountClone(g, service, q, service.Media)
	metadataService := &clonemetadata.Service{Edits: edits, Generate: metadataRewrite(edits, metadataRun), Channel: queueStore.Owner}
	worker.OnPoll = mountCloneOwnerOperations(g, pool, edits, service, ai, librarySvc, backdropSvc, toolKit, toolDeps, editorService, renderRuntime.ChannelName, metadataService, autonomySvc, binder, cancels)
	workerCtx, stop := context.WithCancel(ctx)
	mountPosted(g, pool, store, service, metadataService, publish)
	done := make(chan error, 1)
	go func() { done <- worker.Run(workerCtx) }()
	var once sync.Once
	var stopped error
	return func() error {
		once.Do(func() {
			stopped = closeCoverTrials()
			previewCtx, previewCancel := context.WithTimeout(context.Background(), 10*time.Second)
			stopped = errors.Join(stopped, previewSvc.Close(previewCtx))
			previewCancel()
			stop()
			if workerErr := <-done; stopped == nil {
				stopped = workerErr
			}
		})
		return stopped
	}, nil
}
