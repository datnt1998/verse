package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Hosted "AI sẽ soạn bản đọc" runner (GenPickEmpty). The HTTP queue writes a
// tts row with input.needsScript=true for an accepted chapter without a bản đọc
// (or with a stale one); the remote worker must never see that row. This runner
// claims it per story, resolves the story's model through the same hosted
// config chain the request paths use, composes + locks the script through
// audio.PGService, and lets CompletePreparationWith swap the same row into a
// render job.
//
// Lifecycle: one goroutine per story (compositions for one story are strictly
// sequential — no parallel provider calls), woken by the audio service on
// enqueue/resume/retry and by startup recovery. A live preparation lease is
// waited out with a timer to its exact expiry; when nothing is queued the loop
// exits instead of polling.

// audioPreparationLease outlives one chapter's compose call (the provider call
// is capped at audio.pgComposeChapterTimeout = 2m30s) so the publish still sees
// a live lease. After a crash, startup recovery waits exactly this long for the
// abandoned row.
const audioPreparationLease = 5 * time.Minute

// audioPreparationStopWait bounds Close's wait for an in-flight composition:
// the model call honours cancellation, and a lease that outlives the process is
// recovered by the next start.
const audioPreparationStopWait = 5 * time.Second

// StartAudioPreparation starts the runner once. It is a no-op in file mode (no
// hosted audio deps) and safe to call after SetAudioStore.
func (g *Gateway) StartAudioPreparation(ctx context.Context) {
	g.prepMu.Lock()
	if g.prepStarted {
		g.prepMu.Unlock()
		return
	}
	g.prepStarted = true
	runCtx, cancel := context.WithCancel(ctx)
	g.prepCancel = cancel
	g.prepWG.Add(1)
	g.prepMu.Unlock()
	go func() {
		defer g.prepWG.Done()
		g.runAudioPreparation(runCtx)
	}()
}

// stopAudioPreparation cancels the runner and waits briefly for an in-flight
// composition to release its row.
func (g *Gateway) stopAudioPreparation() {
	g.prepMu.Lock()
	cancel := g.prepCancel
	g.prepCancel = nil
	g.prepMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	done := make(chan struct{})
	go func() {
		g.prepWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(audioPreparationStopWait):
	}
}

// wakeAudioPreparation is the non-blocking hook installed on every hosted audio
// service: a new/resumed preparation row must start composing immediately.
func (g *Gateway) wakeAudioPreparation() {
	g.prepMu.Lock()
	ch := g.prepWake
	g.prepMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (g *Gateway) runAudioPreparation(ctx context.Context) {
	for {
		g.scanAudioPreparation(ctx)
		select {
		case <-ctx.Done():
			return
		case <-g.prepWake:
		}
	}
}

// scanAudioPreparation starts one loop per story that has preparation work.
// The scan itself is the only "poll": it runs at startup and on explicit wake.
func (g *Gateway) scanAudioPreparation(ctx context.Context) {
	deps, ok, _ := g.audioDeps()
	if !ok || deps == nil || deps.Jobs == nil {
		return
	}
	targets, err := deps.Jobs.ListPreparationTargets(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("Không quét được việc soạn bản đọc", "module", "audio-prepare", "err", err)
		}
		return
	}
	for _, target := range targets {
		key := target.ProjectSlug + "/" + target.StorySlug
		g.prepMu.Lock()
		if g.prepActive[key] {
			g.prepMu.Unlock()
			continue
		}
		g.prepActive[key] = true
		g.prepWG.Add(1)
		g.prepMu.Unlock()
		go func(target postgres.PreparationTarget, key string) {
			defer g.prepWG.Done()
			defer func() {
				g.prepMu.Lock()
				delete(g.prepActive, key)
				g.prepMu.Unlock()
				// A row queued while this loop was draining must not be missed:
				// rescan once (an empty scan is a single query, not a poll).
				g.wakeAudioPreparation()
			}()
			g.prepareStory(ctx, target)
		}(target, key)
	}
}

// prepareStory claims and composes this story's preparations sequentially.
func (g *Gateway) prepareStory(ctx context.Context, target postgres.PreparationTarget) {
	deps, ok, _ := g.audioDeps()
	if !ok || deps == nil || deps.Jobs == nil {
		return
	}
	lib, err := g.lib(ctx, nil)
	if err != nil {
		slog.Warn("Không mở được thư viện để soạn bản đọc", "module", "audio-prepare",
			"project_slug", target.ProjectSlug, "story_slug", target.StorySlug, "err", err)
		return
	}
	st, found := lib.ResolveStory(ctx, nil, target.ProjectSlug, target.StorySlug)
	if !found {
		// Deleted between the scan and now: nothing to do (never resurrect).
		return
	}
	svc, err := g.pgAudioService(st)
	if err != nil {
		slog.Warn("Không mở được dịch vụ âm thanh để soạn bản đọc", "module", "audio-prepare",
			"story_slug", st.ID, "err", err)
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := deps.Jobs.ClaimPreparation(ctx, target.Scope, audioPreparationLease)
		if err != nil {
			if errors.Is(err, postgres.ErrNoClaimableJob) {
				wait := g.preparationLeaseWait(ctx, target.Scope)
				if wait <= 0 {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-g.prepWake:
				case <-time.After(wait):
				}
				continue
			}
			if ctx.Err() == nil {
				slog.Warn("Không nhận được việc soạn bản đọc", "module", "audio-prepare",
					"story_slug", st.ID, "err", err)
			}
			return
		}
		g.processPreparation(ctx, svc, st, target.Scope, job)
	}
}

// preparationLeaseWait returns how long to wait for the story's live
// preparation lease to lapse, or 0 when nothing is claimable and nothing is
// leased (the loop exits, no idle polling).
func (g *Gateway) preparationLeaseWait(ctx context.Context, scope postgres.ScopeRef) time.Duration {
	deps, ok, _ := g.audioDeps()
	if !ok || deps == nil || deps.Jobs == nil {
		return 0
	}
	rows, err := deps.Jobs.List(ctx, scope)
	if err != nil {
		return 0
	}
	now := time.Now()
	var next time.Time
	expired := false
	for _, row := range rows {
		if row.State != postgres.JobQueued && row.State != postgres.JobRunning {
			continue
		}
		if row.LeaseUntil == nil {
			continue
		}
		var in struct {
			NeedsScript bool `json:"needsScript"`
		}
		if json.Unmarshal(row.Input, &in) != nil || !in.NeedsScript {
			continue
		}
		until := row.LeaseUntil.Add(250 * time.Millisecond)
		if !until.After(now) {
			// Already expired: claim again right away instead of leaving the
			// row for the next wake-up.
			expired = true
			continue
		}
		if next.IsZero() || until.Before(next) {
			next = until
		}
	}
	if expired {
		return 100 * time.Millisecond
	}
	if next.IsZero() {
		return 0
	}
	return time.Until(next)
}

// processPreparation resolves the story model + characters for this claimed
// job, then hands the composition to the audio service. A resolution failure is
// a retryable job error (needsScript stays, so Thử lại re-enters preparation).
func (g *Gateway) processPreparation(ctx context.Context, svc *audio.PGService, st library.Story, scope postgres.ScopeRef, job postgres.Job) {
	in, err := g.preparationInput(ctx, st, job)
	if err != nil {
		g.failPreparationJob(ctx, scope, job, err)
		return
	}
	if err := svc.PrepareClaimed(ctx, scope, job, in); err != nil && ctx.Err() == nil {
		slog.Warn("Soạn bản đọc chương thất bại", "module", "audio-prepare",
			"story_slug", st.ID, "err", err)
	}
}

// preparationInput builds the compose inputs the request paths use: the story's
// configured model (global/project/story chain, trusted internal actor), the
// chapter source the queue validated (same StoryReader resolution), and the
// character profiles from the story fact store. No fake provider is ever built.
func (g *Gateway) preparationInput(ctx context.Context, st library.Story, job postgres.Job) (audio.PreparationInput, error) {
	volume, chapter, ok := audio.PreparationCoords(job)
	if !ok {
		return audio.PreparationInput{}, fmt.Errorf("việc %s không phải là soạn bản đọc", postgres.UUIDString(job.ID))
	}
	composer, err := g.audioComposerInternal(ctx, st)
	if err != nil {
		return audio.PreparationInput{}, err
	}
	engine, err := g.openFactStoreInternal(st)
	if err != nil {
		return audio.PreparationInput{}, err
	}
	reader := host.NewStoryReaderWithStore(engine)
	source, _, err := audioVolumeSource(reader, volume)
	if err != nil {
		return audio.PreparationInput{}, err
	}
	sc, ok := audio.ChapterSource(source, chapter)
	if !ok {
		return audio.PreparationInput{}, fmt.Errorf("tập %d không có chương %d", volume, chapter)
	}
	ref, err := storyCharacters(reader)
	if err != nil {
		return audio.PreparationInput{}, err
	}
	return audio.PreparationInput{Composer: composer, Ref: ref, Source: sc}, nil
}

// failPreparationJob records a retryable failure on a claimed preparation whose
// inputs could not even be resolved.
func (g *Gateway) failPreparationJob(ctx context.Context, scope postgres.ScopeRef, job postgres.Job, cause error) {
	deps, ok, _ := g.audioDeps()
	if !ok || deps == nil || deps.Jobs == nil {
		return
	}
	msg := fmt.Sprintf("Không soạn được bản đọc: %v", cause)
	if errors.Is(cause, audio.ErrNoModel) {
		msg = audio.ErrNoModel.Error()
	}
	if _, err := deps.Jobs.FailPreparation(ctx, scope, job.ID, job.FencingToken,
		map[string]any{"message": msg, "error": cause.Error()}); err != nil {
		if !errors.Is(err, postgres.ErrJobLeaseLost) && ctx.Err() == nil {
			slog.Warn("Không ghi được lỗi soạn bản đọc", "module", "audio-prepare", "err", err)
		}
	}
}
