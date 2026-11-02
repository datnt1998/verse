//go:build !integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

// Hosted GenPickEmpty regressions: an accepted chapter without a bản đọc must
// enter the audio queue, get composed + locked by the web runner, and leave the
// SAME durable row queued and claimable by the remote worker. A pause/cancel
// while the model is running must leave no script and no render claim, and a
// restart must resume both queued rows and rows whose preparation lease was
// still live. Only the provider endpoint is local: the real ChatComposer, the
// real repositories and the real runner execute.

type prepHarness struct {
	tg     *testGateway
	pool   *postgres.Pool
	nar    *postgres.Narrative
	scope  postgres.ScopeRef
	jobs   *postgres.Jobs
	repo   *postgres.Audio
	svc    *audio.PGService
	st     library.Story
	ctx    context.Context
	cancel context.CancelFunc
}

// newPrepHarness provisions one hosted story with an accepted chapter and no
// audio script, and wires the gateway exactly like serve does (library, fact
// store, audio store, global model config pointing at providerURL).
func newPrepHarness(t *testing.T, projectSlug, storySlug, chapterText, providerURL string) *prepHarness {
	t.Helper()
	pool, nar, scope := provisionWebStoryRoles(t, projectSlug, storySlug)
	jobs := postgres.NewJobs(pool)
	repo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	if _, err := nar.AcceptRecord(context.Background(), scope, 1, domain.ChapterOriginGenerated,
		chapterText, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	if err := nar.ProgressSave(context.Background(), scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	tg := newTestGateway(t)
	tg.g.SetLibraryRepository(postgres.NewLibraryRepository(pool))
	st := library.Story{Project: projectSlug, Story: storySlug, ID: projectSlug + "/" + storySlug}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(context.Background(), pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: repo, Cache: cache, Nar: nar, Blobs: blobs})
	writePrepModelConfig(t, providerURL)
	svc, err := tg.g.pgAudioService(st)
	if err != nil {
		t.Fatalf("pg audio service: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &prepHarness{tg: tg, pool: pool, nar: nar, scope: scope, jobs: jobs, repo: repo, svc: svc, st: st, ctx: ctx, cancel: cancel}
}

func writePrepModelConfig(t *testing.T, providerURL string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".ainovel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","api_key":"sk-local-test","base_url":%q}}}`, providerURL)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

func prepSegmentResponse(text string) string {
	content, _ := json.Marshal(map[string]any{
		"segments": []map[string]any{{"speaker": "dan", "text": text, "tone": "", "pauseMs": 0}},
	})
	body, _ := json.Marshal(map[string]any{
		"model": "gpt-4o",
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": string(content)},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
	})
	return string(body)
}

// blockingComposer holds the provider call open until the test releases it, so
// pause/cancel and the job-id assertions run while the model is genuinely
// blocked.
type blockingComposer struct {
	server  *httptest.Server
	started chan struct{}
	release chan struct{}
	calls   *atomic.Int32
}

func newBlockingComposer(t *testing.T, text string) *blockingComposer {
	t.Helper()
	bc := &blockingComposer{started: make(chan struct{}, 4), release: make(chan struct{}), calls: &atomic.Int32{}}
	bc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bc.calls.Add(1)
		select {
		case bc.started <- struct{}{}:
		default:
		}
		<-bc.release
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, prepSegmentResponse(text))
	}))
	t.Cleanup(bc.server.Close)
	t.Cleanup(func() {
		select {
		case <-bc.release:
		default:
			close(bc.release)
		}
	})
	return bc
}

func (bc *blockingComposer) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-bc.started:
	case <-time.After(10 * time.Second):
		t.Fatal("provider call never started")
	}
}

func (bc *blockingComposer) answer() {
	select {
	case <-bc.release:
	default:
		close(bc.release)
	}
}

func postPrepQueue(t *testing.T, h *prepHarness, body map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "http://x/audio/queue?volume=1", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	if !h.tg.g.serveStoryAudio(w, r, admin, h.st, r.URL.Path) {
		t.Fatalf("POST audio queue not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
}

func waitPrep(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func prepJobs(t *testing.T, h *prepHarness) []postgres.Job {
	t.Helper()
	rows, err := h.jobs.List(context.Background(), h.scope)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	return rows
}

func prepJobInput(t *testing.T, row postgres.Job) struct {
	Kind        string `json:"kind"`
	Chapter     int    `json:"chapter"`
	Volume      int    `json:"volume"`
	NeedsScript bool   `json:"needsScript"`
	Quality     string `json:"quality"`
	Segments    []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"segments"`
} {
	t.Helper()
	var in struct {
		Kind        string `json:"kind"`
		Chapter     int    `json:"chapter"`
		Volume      int    `json:"volume"`
		NeedsScript bool   `json:"needsScript"`
		Quality     string `json:"quality"`
		Segments    []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(row.Input, &in); err != nil {
		t.Fatalf("job input: %v", err)
	}
	return in
}

func TestPGHostedQueueComposesMissingScript(t *testing.T) {
	provider := newBlockingComposer(t, "Trời đã tối.")
	h := newPrepHarness(t, "prep-proj", "novel-a", "Trời đã tối. Nàng nhìn ra cửa sổ.", provider.server.URL)
	h.tg.g.StartAudioPreparation(h.ctx)

	postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "voiceMode": "cast", "lock": true})
	provider.waitStarted(t)

	rows := prepJobs(t, h)
	if len(rows) != 1 {
		t.Fatalf("jobs = %d, want one preparation row", len(rows))
	}
	prepRow := rows[0]
	if prepRow.State != postgres.JobQueued || !prepJobInput(t, prepRow).NeedsScript {
		t.Fatalf("row = state %s input %s, want queued needsScript", prepRow.State, prepRow.Input)
	}
	if !prepRow.ID.Valid {
		t.Fatalf("preparation row has no id: %+v", prepRow)
	}
	prepID := postgres.UUIDString(prepRow.ID)

	provider.answer()
	waitPrep(t, "locked script", func() bool {
		script, err := h.svc.Script(context.Background(), 1, 1)
		return err == nil && script != nil && script.Status == audio.StatusLocked
	})
	script, err := h.svc.Script(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	if script.Status != audio.StatusLocked || script.LockedAt == nil || len(script.Segments) != 1 {
		t.Fatalf("script = %+v, want locked with one composed segment", script)
	}
	if script.SourceHash != audio.SourceHash("Trời đã tối. Nàng nhìn ra cửa sổ.") {
		t.Fatalf("source hash = %q", script.SourceHash)
	}
	if script.Segments[0].Text != "Trời đã tối." {
		t.Fatalf("composed text = %q", script.Segments[0].Text)
	}

	waitPrep(t, "queued render job", func() bool {
		rows := prepJobs(t, h)
		if len(rows) != 1 || rows[0].State != postgres.JobQueued {
			return false
		}
		return !prepJobInput(t, rows[0]).NeedsScript && rows[0].ProgressTotal != nil && *rows[0].ProgressTotal == 1
	})
	rows = prepJobs(t, h)
	if len(rows) != 1 || postgres.UUIDString(rows[0].ID) != prepID {
		t.Fatalf("prepared row must be the SAME job: got %+v want id %s", rows, prepID)
	}
	in := prepJobInput(t, rows[0])
	if len(in.Segments) != 1 || in.Segments[0].Text != "Trời đã tối." || in.Quality != "standard" {
		t.Fatalf("prepared input = %+v", in)
	}

	// Renderable: an authenticated tts worker claims the row and receives the
	// composed segment (the same input the remote worker protocol reads).
	workerHash := sha256.Sum256([]byte("prep-test-worker-token"))
	worker, err := h.jobs.RegisterWorker(context.Background(), "prep-test-worker", workerHash[:], []string{"tts"})
	if err != nil {
		t.Fatalf("register worker: %v", err)
	}
	claim, err := h.jobs.Claim(context.Background(), postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("worker claim: %v", err)
	}
	var claimed struct {
		Segments []struct {
			ID string `json:"id"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(claim.Input, &claimed); err != nil {
		t.Fatalf("claimed input: %v", err)
	}
	if len(claimed.Segments) != 1 || claimed.Segments[0].ID != script.Segments[0].ID {
		t.Fatalf("claimed segments = %+v, want the composed segment", claimed.Segments)
	}
}

func TestPGHostedPreparationPauseCancelLeavesNoScript(t *testing.T) {
	for _, action := range []string{"pause", "cancel"} {
		t.Run(action, func(t *testing.T) {
			provider := newBlockingComposer(t, "Đoạn chưa xong.")
			h := newPrepHarness(t, "prep-"+action, "novel-"+action, "Chương đang soạn dở.", provider.server.URL)
			h.tg.g.StartAudioPreparation(h.ctx)
			postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})
			provider.waitStarted(t)

			rows := prepJobs(t, h)
			if len(rows) != 1 {
				t.Fatalf("jobs = %d, want one", len(rows))
			}
			jobID := postgres.UUIDString(rows[0].ID)
			if action == "pause" {
				if err := h.svc.Pause(jobID); err != nil {
					t.Fatalf("pause: %v", err)
				}
			} else {
				if err := h.svc.Cancel(jobID); err != nil {
					t.Fatalf("cancel: %v", err)
				}
			}
			provider.answer()

			wantState := postgres.JobPaused
			if action == "cancel" {
				wantState = postgres.JobCancelled
			}
			waitPrep(t, "control state", func() bool {
				rows := prepJobs(t, h)
				return len(rows) == 1 && rows[0].State == wantState
			})
			// Give the publish a moment to prove it is refused: no script may
			// ever appear after the fence moved.
			time.Sleep(300 * time.Millisecond)
			script, err := h.svc.Script(context.Background(), 1, 1)
			if err != nil {
				t.Fatalf("script: %v", err)
			}
			if script != nil {
				t.Fatalf("script was persisted after %s: %+v", action, script)
			}
			rows = prepJobs(t, h)
			if len(rows) != 1 || rows[0].State != wantState {
				t.Fatalf("rows = %+v, want one %s row", rows, wantState)
			}
			if !prepJobInput(t, rows[0]).NeedsScript {
				t.Fatalf("control row lost needsScript: %s", rows[0].Input)
			}
			// Nothing is claimable by the remote worker: the row is not a
			// render job. A registered worker is used so a FK failure cannot
			// masquerade as "not claimable".
			workerHash := sha256.Sum256([]byte("prep-control-token-" + action))
			worker, werr := h.jobs.RegisterWorker(context.Background(), "prep-control-"+action, workerHash[:], []string{"tts"})
			if werr != nil {
				t.Fatalf("register worker: %v", werr)
			}
			if _, err := h.jobs.Claim(context.Background(), postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
				t.Fatalf("a %s preparation row was claimable by the worker: %v", action, err)
			}

			if action != "pause" {
				return
			}
			// Resume puts the same preparation back in the queue; the runner
			// wakes and finishes the composition.
			if err := h.svc.Resume(jobID); err != nil {
				t.Fatalf("resume: %v", err)
			}
			waitPrep(t, "resumed script", func() bool {
				script, err := h.svc.Script(context.Background(), 1, 1)
				return err == nil && script != nil && script.Status == audio.StatusLocked
			})
			waitPrep(t, "resumed render job", func() bool {
				rows := prepJobs(t, h)
				return len(rows) == 1 && rows[0].State == postgres.JobQueued && !prepJobInput(t, rows[0]).NeedsScript
			})
		})
	}
}

func TestPGHostedPreparationRecoversQueuedAndLeasedRows(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		provider := newBlockingComposer(t, "Đoạn chờ.")
		h := newPrepHarness(t, "prep-recover", "novel-queued", "Chương chờ soạn.", provider.server.URL)
		// Runner not started (a freshly restarted process): the row is queued.
		postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})
		rows := prepJobs(t, h)
		if len(rows) != 1 || rows[0].State != postgres.JobQueued || !prepJobInput(t, rows[0]).NeedsScript {
			t.Fatalf("rows = %+v, want one queued preparation", rows)
		}
		provider.answer()
		h.tg.g.StartAudioPreparation(h.ctx)
		waitPrep(t, "recovered script", func() bool {
			script, err := h.svc.Script(context.Background(), 1, 1)
			return err == nil && script != nil && script.Status == audio.StatusLocked
		})
	})

	t.Run("live lease", func(t *testing.T) {
		provider := newBlockingComposer(t, "Đoạn thuê.")
		h := newPrepHarness(t, "prep-lease", "novel-lease", "Chương đang giữ thuê.", provider.server.URL)
		postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})
		// Simulate a crashed process: the row was claimed and its lease is still
		// live. The restarted runner must not call the provider before expiry.
		if _, err := h.jobs.ClaimPreparation(context.Background(), h.scope, time.Second); err != nil {
			t.Fatalf("claim preparation: %v", err)
		}
		provider.answer()
		h.tg.g.StartAudioPreparation(h.ctx)
		time.Sleep(300 * time.Millisecond)
		if got := provider.calls.Load(); got != 0 {
			t.Fatalf("provider called %d times while the lease was live", got)
		}
		waitPrep(t, "leased recovery", func() bool {
			script, err := h.svc.Script(context.Background(), 1, 1)
			return err == nil && script != nil && script.Status == audio.StatusLocked
		})
		if got := provider.calls.Load(); got != 1 {
			t.Fatalf("provider calls = %d, want exactly one composed chapter", got)
		}
	})
}

func TestPGHostedPreparationKeepsStoriesIsolated(t *testing.T) {
	appDSN, maintDSN := prepDSNPair(t)
	textA := "Truyện A mở đầu."
	textB := "Truyện B mở đầu."
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		text := "Đoạn A."
		if strings.Contains(string(body), textB) {
			text = "Đoạn B."
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, prepSegmentResponse(text))
	}))
	t.Cleanup(provider.Close)
	hA := newPrepHarnessDSN(t, appDSN, maintDSN, "prep-iso", "story-a", textA, provider.URL)
	hB := newPrepHarnessDSN(t, appDSN, maintDSN, "prep-iso", "story-b", textB, provider.URL)
	// Both harnesses build their own gateway/config; the runner must keep each
	// story's source, script and job on its own scope.
	hA.tg.g.StartAudioPreparation(hA.ctx)
	hB.tg.g.StartAudioPreparation(hB.ctx)
	postPrepQueue(t, hA, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})
	postPrepQueue(t, hB, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})

	waitPrep(t, "story A script", func() bool {
		script, err := hA.svc.Script(context.Background(), 1, 1)
		return err == nil && script != nil && script.SourceHash == audio.SourceHash(textA)
	})
	waitPrep(t, "story B script", func() bool {
		script, err := hB.svc.Script(context.Background(), 1, 1)
		return err == nil && script != nil && script.SourceHash == audio.SourceHash(textB)
	})
	scriptA, _ := hA.svc.Script(context.Background(), 1, 1)
	scriptB, _ := hB.svc.Script(context.Background(), 1, 1)
	if scriptA.Segments[0].Text != "Đoạn A." || scriptB.Segments[0].Text != "Đoạn B." {
		t.Fatalf("composed texts crossed stories: A=%q B=%q", scriptA.Segments[0].Text, scriptB.Segments[0].Text)
	}
	rowsA := prepJobs(t, hA)
	rowsB := prepJobs(t, hB)
	if len(rowsA) != 1 || len(rowsB) != 1 {
		t.Fatalf("job rows A=%d B=%d", len(rowsA), len(rowsB))
	}
	if postgres.UUIDString(rowsA[0].ID) == postgres.UUIDString(rowsB[0].ID) {
		t.Fatal("two stories share a job id")
	}
	if in := prepJobInput(t, rowsA[0]); len(in.Segments) != 1 || in.Segments[0].Text != "Đoạn A." {
		t.Fatalf("story A job input = %+v", in)
	}
	if in := prepJobInput(t, rowsB[0]); len(in.Segments) != 1 || in.Segments[0].Text != "Đoạn B." {
		t.Fatalf("story B job input = %+v", in)
	}
}

func TestPGHostedPreparationProviderFailureRetryRecovers(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"provider sập tạm thời"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, prepSegmentResponse("Đoạn thử lại."))
	}))
	t.Cleanup(provider.Close)
	h := newPrepHarness(t, "prep-fail", "novel-retry", "Chương lỗi rồi thử lại.", provider.URL)
	h.tg.g.StartAudioPreparation(h.ctx)
	postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})

	waitPrep(t, "failed preparation", func() bool {
		rows := prepJobs(t, h)
		return len(rows) == 1 && rows[0].State == postgres.JobFailed
	})
	rows := prepJobs(t, h)
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !prepJobInput(t, rows[0]).NeedsScript {
		t.Fatalf("failed preparation lost needsScript: %s", rows[0].Input)
	}
	if len(rows[0].Error) == 0 {
		t.Fatal("failed preparation has no error reason")
	}
	script, err := h.svc.Script(context.Background(), 1, 1)
	if err != nil || script != nil {
		t.Fatalf("script after failure = %+v err %v", script, err)
	}

	// The existing queue Retry re-enters preparation and the second model call
	// produces a locked script + renderable row.
	if err := h.svc.Retry(context.Background(), postgres.UUIDString(rows[0].ID), ""); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitPrep(t, "retried script", func() bool {
		script, err := h.svc.Script(context.Background(), 1, 1)
		return err == nil && script != nil && script.Status == audio.StatusLocked
	})
	waitPrep(t, "retried render job", func() bool {
		rows := prepJobs(t, h)
		return len(rows) == 1 && rows[0].State == postgres.JobQueued && !prepJobInput(t, rows[0]).NeedsScript
	})
	if calls.Load() < 2 {
		t.Fatalf("provider calls = %d, want the retry to compose again", calls.Load())
	}
}

// prepDSNPair provisions one isolated schema so two stories can share it (the
// isolation regression); each harness still gets its own pool/service.
func prepDSNPair(t *testing.T) (string, string) {
	t.Helper()
	dsn := pgtest.Provision(t)
	return dsn, dsn
}

// newPrepHarnessDSN is newPrepHarness on an existing schema, for the two-story
// isolation regression.
func newPrepHarnessDSN(t *testing.T, appDSN, maintDSN, projectSlug, storySlug, chapterText, providerURL string) *prepHarness {
	t.Helper()
	pool, nar, scope := provisionWebStoryDSNs(t, projectSlug, storySlug, appDSN, maintDSN)
	jobs := postgres.NewJobs(pool)
	repo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	if _, err := nar.AcceptRecord(context.Background(), scope, 1, domain.ChapterOriginGenerated,
		chapterText, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	if err := nar.ProgressSave(context.Background(), scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	tg := newTestGateway(t)
	tg.g.SetLibraryRepository(postgres.NewLibraryRepository(pool))
	st := library.Story{Project: projectSlug, Story: storySlug, ID: projectSlug + "/" + storySlug}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(context.Background(), pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: repo, Cache: cache, Nar: nar, Blobs: blobs})
	writePrepModelConfig(t, providerURL)
	svc, err := tg.g.pgAudioService(st)
	if err != nil {
		t.Fatalf("pg audio service: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &prepHarness{tg: tg, pool: pool, nar: nar, scope: scope, jobs: jobs, repo: repo, svc: svc, st: st, ctx: ctx, cancel: cancel}
}
