package ttsworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// previewServer is the private preview surface the worker pulls from in tests:
// it hands out queued previews and records failures. Transport success is
// covered by throwaway/native smoke, so this fixture only exercises boundaries
// and failure behaviour.
type previewServer struct {
	mu       sync.Mutex
	pending  []previewRequest
	failures map[string]string
	srv      *httptest.Server
}

func newPreviewServer(t *testing.T, pending ...previewRequest) *previewServer {
	t.Helper()
	ps := &previewServer{
		pending:  pending,
		failures: map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/worker/previews/next", ps.next)
	mux.HandleFunc("POST /internal/worker/previews/{id}/fail", ps.fail)
	ps.srv = httptest.NewServer(mux)
	t.Cleanup(ps.srv.Close)
	return ps
}

func (p *previewServer) authOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return false
	}
	return true
}

func (p *previewServer) next(w http.ResponseWriter, r *http.Request) {
	if !p.authOK(w, r) {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pending) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	next := p.pending[0]
	p.pending = p.pending[1:]
	_ = json.NewEncoder(w).Encode(next)
}

func (p *previewServer) fail(w http.ResponseWriter, r *http.Request) {
	if !p.authOK(w, r) {
		return
	}
	body, _ := io.ReadAll(r.Body)
	p.mu.Lock()
	p.failures[r.PathValue("id")] = string(body)
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// Worker không bật nghe thử (bản cũ) không được kéo request: nó không xử lý
// được, nên không được nhận.
func TestWorkerWithoutPreviewCapabilityPullsNothing(t *testing.T) {
	srv := newPreviewServer(t, previewRequest{ID: "p1", Text: "Xin chào"})
	w, err := New(Config{APIURL: srv.srv.URL, Token: testToken, Engine: &fakeEngine{}, StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})
	if w.servePreview(context.Background()) {
		t.Fatal("a worker without the preview capability pulled a preview")
	}
	srv.mu.Lock()
	remaining := len(srv.pending)
	srv.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("pending previews = %d, want the queue untouched", remaining)
	}
}

// Bộ đọc lỗi: báo máy chủ ngay để người dùng nhận câu lỗi thay vì chờ hết hạn.
func TestWorkerReportsPreviewFailure(t *testing.T) {
	srv := newPreviewServer(t, previewRequest{ID: "p2", Text: "làm hỏng"})
	eng := &fakeEngine{failOn: "làm hỏng"}
	w, err := New(Config{APIURL: srv.srv.URL, Token: testToken, Engine: eng, Preview: true, StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})

	if !w.servePreview(context.Background()) {
		t.Fatal("servePreview reported no work for a failing preview")
	}
	srv.mu.Lock()
	message := srv.failures["p2"]
	srv.mu.Unlock()
	if message == "" {
		t.Fatal("the worker did not report the failed preview")
	}
}

// Máy chủ luôn có một lượt nghe thử đang chờ không được bỏ đói chương: mỗi
// vòng chỉ phục vụ một lượt rồi vẫn thử nhận việc. Nếu vòng lặp quay lại ngay
// sau nghe thử, job sẽ không bao giờ được nhận trong thời gian test.
func TestPreviewPriorityDoesNotStarveChapters(t *testing.T) {
	input := snapshotInput(t, "create", Segment{ID: "seg-1", Text: "Câu một."})
	fs := newFakeServer(t, job{ID: "job-1", Kind: "tts", State: "running", Fence: 4, Input: input})
	fs.mu.Lock()
	fs.previewEvery = true
	fs.mu.Unlock()
	w, err := New(Config{
		APIURL: fs.srv.URL, Token: testToken, Engine: &fakeEngine{}, Preview: true,
		Lease: 120 * time.Second, Heartbeat: 50 * time.Millisecond, Poll: 5 * time.Millisecond,
		StateDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatalf("run returned %v, want the worker to keep serving", err)
	}

	fs.mu.Lock()
	completed, previews, failed := len(fs.completed), fs.previewClaims, fs.failed
	fs.mu.Unlock()
	if previews == 0 {
		t.Fatal("no preview was served, the starvation case was never exercised")
	}
	if completed != 1 {
		t.Fatalf("completed chapters = %d while previews kept arriving, want the job to progress", completed)
	}
	if failed != 0 {
		t.Fatalf("/fail called %d time(s): a preview must not break the chapter", failed)
	}
}

// deadlineEngine records whether Synthesize ran with an already-expired context,
// and behaves like a real adapter by refusing work after the deadline.
type deadlineEngine struct {
	fakeEngine
	sawExpired bool
}

func (e *deadlineEngine) Synthesize(ctx context.Context, req voice.SynthRequest) (voice.Audio, error) {
	e.sawExpired = ctx.Err() != nil
	if e.sawExpired {
		return voice.Audio{}, ctx.Err()
	}
	return e.fakeEngine.Synthesize(ctx, req)
}

// Máy chủ nói lúc người dùng thôi chờ; máy đọc không được đọc tiếp sau mốc đó.
func TestWorkerHonoursClaimDeadline(t *testing.T) {
	srv := newPreviewServer(t, previewRequest{ID: "p3", Text: "Xin chào",
		Deadline: time.Now().Add(-time.Second).Format(time.RFC3339Nano)})
	eng := &deadlineEngine{}
	w, err := New(Config{APIURL: srv.srv.URL, Token: testToken, Engine: eng, Preview: true, StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})

	if !w.servePreview(context.Background()) {
		t.Fatal("servePreview reported no work for an expired preview")
	}
	if !eng.sawExpired {
		t.Fatal("the engine was not given the claim deadline")
	}
	srv.mu.Lock()
	failure := srv.failures["p3"]
	srv.mu.Unlock()
	if failure != "" {
		t.Fatalf("a preview whose deadline already passed reported a result: %s", failure)
	}
}
