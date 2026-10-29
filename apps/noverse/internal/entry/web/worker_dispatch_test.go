package web

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/host"
)

type slowStartHost struct {
	*fakeHost
	release chan struct{}
	err     error
}

func (h *slowStartHost) StartPrepared(string) error {
	<-h.release
	return h.err
}

// E2ELIB-1: model-backed input returns promptly; a late failure becomes an
// ERROR event instead of an HTTP request that never answers.
func TestInputDispatchReturnsBeforeSlowModelAction(t *testing.T) {
	h := &slowStartHost{fakeHost: newFakeHost(), release: make(chan struct{}), err: errors.New("Không kết nối được nhà cung cấp AI")}
	s := newWorkerServer(h, "secret", make(chan struct{}), newBroadcaster())
	_, _, ch := s.b.subscribe()
	defer s.b.unsubscribe(ch)
	send := func(text string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/input", strings.NewReader(`{"text":"`+text+`"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Ainovel-Worker-Token", "secret")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	// StartPrepared blocks until release is closed, so a synchronous handler
	// never answers; hangLimit only reports that, it is not a latency budget.
	const hangLimit = 10 * time.Second
	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- send("ý tưởng") }()
	var w *httptest.ResponseRecorder
	select {
	case w = <-first:
	case <-time.After(hangLimit):
		close(h.release)
		t.Fatal("input waited for the model-backed action")
	}
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatalf("input = %d %s", w.Code, w.Body.String())
	}
	if w := send("lần hai"); w.Code != 409 || !strings.Contains(w.Body.String(), "Đang xử lý yêu cầu trước") {
		t.Fatalf("second input while dispatching = %d %s", w.Code, w.Body.String())
	}
	snap := httptest.NewRequest("GET", "/api/snapshot", nil)
	snap.Header.Set("X-Ainovel-Worker-Token", "secret")
	sw := httptest.NewRecorder()
	s.ServeHTTP(sw, snap)
	if !strings.Contains(sw.Body.String(), `"actionPending":true`) {
		t.Fatalf("snapshot hides the pending action, so delete could kill it: %s", sw.Body.String())
	}
	close(h.release)
	deadline := time.After(hangLimit)
	for {
		select {
		case m := <-ch:
			if ev, ok := m.data.(host.Event); ok && m.kind == "event" && ev.Category == "ERROR" && strings.Contains(ev.Summary, "Không kết nối được nhà cung cấp AI") {
				return
			}
		case <-deadline:
			t.Fatal("late action failure was not published")
		}
	}
}
