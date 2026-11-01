package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

// hostedTTSForWatcher migrates a fresh schema and hands back the real jobs
// repository without injecting it: the caller controls SetAudioStore so the
// test can reproduce the production order NewGateway → SetAudioStore.
func hostedTTSForWatcher(t *testing.T) (*testGateway, *postgres.Jobs) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	// The gateway is constructed first (providers already watching) and only
	// afterwards does the test inject the hosted store — the real startup order.
	tg := newTestGateway(t)
	return tg, postgres.NewJobs(pool)
}

func streamHas(w *streamWriter, phrase string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Contains(w.body.String(), phrase)
}

func waitHubJob(t *testing.T, g *Gateway, id string, ok func(Job) bool, want string) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if job, found := g.jobsHub().get(id); found && ok(job) {
			return job
		}
		if time.Now().After(deadline) {
			job, found := g.jobsHub().get(id)
			t.Fatalf("job %q did not reach %q in the registry (found=%v job=%+v)", id, want, found, job)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// NewGateway starts the jobs watchers before serve calls SetAudioStore. The
// watcher must not latch the local install backend: after the hosted store is
// injected, worker state changes have to reach the jobs registry and the SSE
// stream without any GET /api/jobs refresh.
func TestVoiceInstallWatcherFollowsHostedStoreInjection(t *testing.T) {
	old := voiceHostedPollInterval
	voiceHostedPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { voiceHostedPollInterval = old })

	tg, jobs := hostedTTSForWatcher(t)
	admin := tg.login("admin", "admin-pass")
	worker := hostedWorker(t, jobs, "mac-mini")

	provider := voiceJobsProvider{g: tg.g}
	if provider.hosted() {
		t.Fatal("hosted backend selected before SetAudioStore")
	}
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs})
	if !provider.hosted() {
		t.Fatal("hosted backend still not selected after SetAudioStore")
	}

	// Live SSE subscription first; the handshake event proves the stream is
	// attached before the worker publishes anything.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	writer := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	done := make(chan struct{})
	go func() {
		tg.g.serveEvents(writer, request, identity.User{Username: "admin", Role: identity.RoleAdmin})
		close(done)
	}()
	handshake := time.Now().Add(2 * time.Second)
	for !streamHas(writer, "event: test.ready") {
		if time.Now().After(handshake) {
			cancel()
			<-done
			t.Fatal("SSE subscription never became live")
		}
		tg.g.events().Publish(Event{Type: "test.ready", Project: "p", Story: "s"})
		time.Sleep(5 * time.Millisecond)
	}

	// No GET /api/jobs in this test: the registry and the stream must learn
	// about the hosted row from the watcher alone.
	setWorkerState(t, jobs, worker.ID, `{"state":"provisioning","step":"deps","percent":40,"message":"Đang cài thư viện…","updatedAt":"2026-01-02T03:04:05Z"}`)
	running := waitHubJob(t, tg.g, voiceInstallJobID, func(j Job) bool { return j.Status == JobRunning }, "running")
	if running.Step != "Bước 4/6 · Cài thư viện" {
		t.Fatalf("hosted row step = %q", running.Step)
	}
	body := writer.until(t, `"id":"voice/install"`)
	if !strings.Contains(body, `"status":"running"`) {
		t.Fatalf("SSE did not carry the hosted running row: %s", body)
	}

	// A newer worker state proves the watcher is on the hosted backend for
	// good, not latched to the local install state it started with.
	setWorkerState(t, jobs, worker.ID, `{"state":"error","step":"error","message":"Cài bộ đọc thất bại","error":{"code":"uv_failed","message":"chưa có uv trên máy"},"updatedAt":"2026-01-02T03:05:05Z"}`)
	failed := waitHubJob(t, tg.g, voiceInstallJobID, func(j Job) bool { return j.Status == JobFailed }, "failed")
	if info, ok := failed.Error.(JobErrorInfo); !ok || info.Cause != "uv_failed" {
		t.Fatalf("hosted failure error = %#v", failed.Error)
	}
	body = writer.until(t, `"status":"failed"`)
	if !strings.Contains(body, "uv_failed") {
		t.Fatalf("SSE did not carry the hosted failure: %s", body)
	}
	cancel()
	<-done

	// Real endpoint still serves the same hosted row (no local row wins).
	var got *Job
	for _, job := range jobsOf(t, tg, admin) {
		if job.ID == voiceInstallJobID {
			copied := job
			got = &copied
		}
	}
	if got == nil || got.Status != JobFailed {
		t.Fatalf("GET /api/jobs missing the hosted failure: %+v", got)
	}
}

// Cancelling a failed voice install removes that attempt from the dock, but a
// provider refresh must not resurrect it. A genuinely new attempt shows again.
func TestVoiceFailedInstallDismissalSurvivesRefresh(t *testing.T) {
	eng := &testTtsEngine{
		steps: []voice.InstallProgress{{Step: "source", Percent: new(5), Message: "Đang tải mã nguồn VieNeu-TTS…"}},
		fail:  errors.New("chưa có uv trên máy"),
	}
	useTtsService(t, eng)
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	if w := tg.do("POST", "/api/tts/install", admin, `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("POST /api/tts/install → %d %s", w.Code, w.Body.String())
	}
	waitJob(t, tg, admin, voiceInstallJobID, JobFailed)

	if w := tg.do("POST", "/api/jobs/"+voiceInstallJobID+"/cancel", admin, `{}`); w.Code != http.StatusOK {
		t.Fatalf("cancel failed install → %d %s", w.Code, w.Body.String())
	}
	for i := range 2 {
		for _, job := range jobsOf(t, tg, admin) {
			if job.ID == voiceInstallJobID {
				t.Fatalf("refresh %d resurrected the dismissed failed attempt: %+v", i+1, job)
			}
		}
	}

	// A newer attempt is a different state and must be visible again.
	eng.mu.Lock()
	eng.fail = nil
	eng.mu.Unlock()
	if w := tg.do("POST", "/api/tts/install", admin, `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("retry install → %d %s", w.Code, w.Body.String())
	}
	waitJob(t, tg, admin, voiceInstallJobID, JobDoneS)

	// Dismissing a finished row keeps working: the same result stays gone
	// across provider refreshes.
	if w := tg.do("POST", "/api/jobs/"+voiceInstallJobID+"/cancel", admin, `{}`); w.Code != http.StatusOK {
		t.Fatalf("dismiss finished install → %d %s", w.Code, w.Body.String())
	}
	for i := range 2 {
		for _, job := range jobsOf(t, tg, admin) {
			if job.ID == voiceInstallJobID {
				t.Fatalf("refresh %d resurrected the dismissed result: %+v", i+1, job)
			}
		}
	}
}
