package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

type fakeHost struct {
	*host.Host
	mu       sync.Mutex
	snapshot host.UISnapshot
	events   chan host.Event
	stream   chan string
	done     chan struct{}
	calls    []string
	// retryWake 控制 POST /api/retry 的返回值（G4）。
	retryWake bool
}

func (h *fakeHost) Snapshot() host.UISnapshot     { h.mu.Lock(); defer h.mu.Unlock(); return h.snapshot }
func (h *fakeHost) Events() <-chan host.Event     { return h.events }
func (h *fakeHost) Stream() <-chan string         { return h.stream }
func (h *fakeHost) Done() <-chan struct{}         { return h.done }
func (h *fakeHost) PrepareUserRules(string) error { h.call("prepare"); return nil }
func (h *fakeHost) StartPrepared(string) error    { h.call("start"); return nil }
func (h *fakeHost) Steer(string, string) error    { h.call("steer"); return nil }
func (h *fakeHost) RetryNow() bool {
	h.call("retry")
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.retryWake
}
func (h *fakeHost) Continue(string, string) error { h.call("continue"); return nil }
func (h *fakeHost) call(s string)                 { h.mu.Lock(); defer h.mu.Unlock(); h.calls = append(h.calls, s) }
func (h *fakeHost) CommentStore() *comments.Store { return nil }
func (h *fakeHost) Documents() *store.IO          { return nil }
func newFakeHost() *fakeHost {
	return &fakeHost{events: make(chan host.Event, 100), stream: make(chan string, 100), done: make(chan struct{}, 1)}
}
func (h *fakeHost) CoCreateStream(context.Context, []host.CoCreateMessage, func(string, string)) (host.CoCreateReply, error) {
	panic("unimplemented")
}
func (h *fakeHost) StageCoCreateStream(context.Context, []host.CoCreateMessage, func(string, string)) (host.CoCreateReply, error) {
	panic("unimplemented")
}
func (h *fakeHost) PauseForCoCreate() bool          { h.call("pauseCoCreate"); return true }
func (h *fakeHost) ResumeFromCoCreate(string) error { h.call("resumeCoCreate"); return nil }
func (h *fakeHost) CancelCoCreate()                 { h.call("cancelCoCreate") }
func (h *fakeHost) ImportFrom(context.Context, imp.Options) (<-chan imp.Event, error) {
	panic("unimplemented")
}
func (h *fakeHost) ImportResumeHint() string { return "" }
func (h *fakeHost) SimulateSource(context.Context, string) (<-chan sim.Event, error) {
	panic("unimplemented")
}
func (h *fakeHost) ImportSimulationProfile(context.Context, string) (<-chan sim.Event, error) {
	panic("unimplemented")
}
func (h *fakeHost) CheckRevisionChanges() ([]host.RevisionChange, error) { return nil, nil }
func (h *fakeHost) SyncChapterRevisions(context.Context) (*revision.Result, error) {
	return &revision.Result{}, nil
}
func (h *fakeHost) Diagnose() diag.Report                                { return diag.Report{} }
func (h *fakeHost) ModelReady() bool                                     { return true }
func (h *fakeHost) CurrentModelSelection(string) (string, string, bool)  { return "", "", false }
func (h *fakeHost) ModelCatalog() []host.CatalogProvider                 { return nil }
func (h *fakeHost) RefreshModelContexts() error                          { h.call("refreshModelContexts"); return nil }
func (h *fakeHost) SwitchModel(string, string, string) error             { return nil }
func (h *fakeHost) CurrentThinking(string) string                        { return "" }
func (h *fakeHost) AvailableThinkingLevels(string) []string              { return nil }
func (h *fakeHost) SetRoleThinking(string, string) error                 { return nil }
func (h *fakeHost) PauseAfterStep() bool                                 { h.call("pauseAfterStep"); return true }
func (h *fakeHost) DiagnoseView() host.DiagView                          { return host.DiagView{Report: h.Diagnose()} }
func (h *fakeHost) SimulationProfileView() (*host.SimulationView, error) { return nil, nil }
func (h *fakeHost) RevisionDiffFor(int) (host.RevisionDiff, error) {
	return host.RevisionDiff{}, host.ErrNoRevision
}
func (h *fakeHost) Chapters() ([]host.ChapterInfo, error)  { return nil, nil }
func (h *fakeHost) SimulationProfileFile() (string, error) { return "", host.ErrChapterNotFound }

type streamWriter struct {
	mu     sync.Mutex
	body   strings.Builder
	signal chan struct{}
	header http.Header
}

func (w *streamWriter) Header() http.Header { return w.header }
func (w *streamWriter) WriteHeader(int)     {}
func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, _ := w.body.Write(p)
	select {
	case w.signal <- struct{}{}:
	default:
	}
	return n, nil
}
func (w *streamWriter) Flush() {}
func (w *streamWriter) until(t *testing.T, phrase string) string {
	t.Helper()
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		w.mu.Lock()
		text := w.body.String()
		w.mu.Unlock()
		if strings.Contains(text, phrase) {
			return text
		}
		select {
		case <-w.signal:
		case <-timeout.C:
			t.Fatalf("SSE missing %q in %q", phrase, text)
		}
	}
}
func TestSSEReconnectReplayAndLive(t *testing.T) {
	h := newFakeHost()
	handler := NewWorkerHandler(h, "secret")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	r := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	r.Header.Set("X-Ainovel-Worker-Token", "secret")
	finished := make(chan struct{})
	go func() { handler.ServeHTTP(w, r); close(finished) }()
	w.until(t, "event: snapshot")
	h.events <- host.Event{ID: "a", Summary: "first"}
	first := w.until(t, `"summary":"first"`)
	h.stream <- "old"
	w.until(t, `"text":"old"`)
	h.stream <- host.StreamClearSentinel
	w.until(t, "event: clear")
	h.stream <- "current"
	w.until(t, `"text":"current"`)
	if strings.Index(first, "event: reset") > strings.Index(first, "event: snapshot") {
		t.Fatalf("initial SSE: %q", first)
	}
	cancel()
	<-finished
	w2 := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	r2 := r.Clone(ctx2)
	finished2 := make(chan struct{})
	go func() { handler.ServeHTTP(w2, r2); close(finished2) }()
	replay := w2.until(t, `"text":"current"`)
	indices := []int{strings.Index(replay, "event: reset"), strings.Index(replay, "event: snapshot"), strings.Index(replay, "event: event"), strings.Index(replay, "event: clear"), strings.LastIndex(replay, "event: delta")}
	for i := 1; i < len(indices); i++ {
		if indices[i] <= indices[i-1] {
			t.Fatalf("replay order: %q", replay)
		}
	}
	h.events <- host.Event{ID: "b", Summary: "live"}
	w2.until(t, `"summary":"live"`)
	cancel2()
	<-finished2
}
func TestStalledSubscriberDoesNotBlockOthers(t *testing.T) {
	b := newBroadcaster()
	_, _, slow := b.subscribe()
	_, _, fast := b.subscribe()
	received := 0
	for range 200 {
		b.emit(message{kind: "event", data: host.Event{Summary: "x"}})
		select {
		case <-fast:
			received++
		default:
			t.Fatal("active subscriber missed an event")
		}
	}
	if received != 200 {
		t.Fatalf("active subscriber received %d events", received)
	}
	for range 128 {
		<-slow
	}
	if _, ok := <-slow; ok {
		t.Fatal("slow subscriber not dropped")
	}
}
func TestInputRouting(t *testing.T) {
	h := newFakeHost()
	handler := NewWorkerHandler(h, "secret")
	send := func() {
		r := httptest.NewRequest("POST", "/api/input", strings.NewReader(`{"text":"idea"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Ainovel-Worker-Token", "secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("input status: %d %s", w.Code, w.Body.String())
		}
	}
	send()
	h.mu.Lock()
	h.snapshot = host.UISnapshot{Phase: "writing", IsRunning: true}
	h.mu.Unlock()
	send()
	h.mu.Lock()
	h.snapshot.IsRunning = false
	h.mu.Unlock()
	send()
	h.mu.Lock()
	defer h.mu.Unlock()
	if got := strings.Join(h.calls, ","); got != "prepare,start,steer,continue" {
		t.Fatalf("routing: %q", got)
	}
}

func TestSSEShutdownReleasesSubscriber(t *testing.T) {
	h := newFakeHost()
	shutdown := make(chan struct{})
	handler := newWorkerHandler(h, "secret", shutdown)
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	r := httptest.NewRequest("GET", "/api/events", nil)
	r.Header.Set("X-Ainovel-Worker-Token", "secret")
	done := make(chan struct{})
	go func() { handler.ServeHTTP(w, r); close(done) }()
	w.until(t, "event: snapshot")
	close(shutdown)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE subscriber blocked graceful shutdown")
	}
}

func TestWorkerRequiresInternalTokenAndJSON(t *testing.T) {
	h := newFakeHost()
	handler := NewWorkerHandler(h, "internal-secret")
	for _, tt := range []struct {
		token, contentType, method string
		want                       int
	}{
		{"", "", "GET", 401},
		{"wrong", "", "GET", 401},
		{"internal-secret", "", "GET", 200},
		{"internal-secret", "text/plain", "POST", 400},
	} {
		r := httptest.NewRequest(tt.method, "/api/snapshot", nil)
		r.Header.Set("X-Ainovel-Worker-Token", tt.token)
		if tt.contentType != "" {
			r.Header.Set("Content-Type", tt.contentType)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("worker auth %q %q: %d, want %d", tt.token, tt.method, w.Code, tt.want)
		}
	}
}
