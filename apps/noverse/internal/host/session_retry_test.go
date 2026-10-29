package host

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/agent"
	"github.com/datnt1998/noverse/internal/llmretry"
)

// G3：每条事件都带当前引擎会话 ID；beginSession 每次自增。
func TestEmitEventStampsSessionID(t *testing.T) {
	h := &Host{events: make(chan Event, 8)}

	h.emitEvent(Event{Summary: "trước phiên"})
	if ev := <-h.events; ev.SessionID != "" {
		t.Fatalf("event before any session has sessionId %q, want empty", ev.SessionID)
	}

	first := h.beginSession()
	h.emitEvent(Event{Summary: "trong phiên 1"})
	got1 := <-h.events
	if first == "" || got1.SessionID != first {
		t.Fatalf("session = %q, event sessionId = %q", first, got1.SessionID)
	}
	raw, err := json.Marshal(got1)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if !strings.Contains(string(raw), `"sessionId":"`+first+`"`) {
		t.Fatalf("event JSON missing sessionId: %s", raw)
	}

	second := h.beginSession()
	if second == first {
		t.Fatalf("beginSession did not increment: %q then %q", first, second)
	}
	h.emitEvent(Event{Summary: "trong phiên 2"})
	if ev := <-h.events; ev.SessionID != second {
		t.Fatalf("event sessionId = %q, want %q", ev.SessionID, second)
	}

	// 显式会话 ID（重放/桥接）不被覆盖。
	h.emitEvent(Event{Summary: "ngoại lai", SessionID: "manual"})
	if ev := <-h.events; ev.SessionID != "manual" {
		t.Fatalf("explicit sessionId overwritten: %q", ev.SessionID)
	}
}

// G4：没有等待者时 RetryNow 返回 false；有退避等待时立即放行。
func TestRetryNowWakesBackoff(t *testing.T) {
	h := &Host{retryGate: llmretry.NewGate()}
	if h.RetryNow() {
		t.Fatal("RetryNow = true with no waiter, want false")
	}

	done := make(chan error, 1)
	go func() {
		done <- h.retryGate.Wait(context.Background(), time.Minute)
	}()
	deadline := time.Now().Add(time.Second)
	for h.retryGate.Waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("waiter never registered")
		}
		time.Sleep(time.Millisecond)
	}
	if !h.RetryNow() {
		t.Fatal("RetryNow = false with a waiter, want true")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RetryNow did not release the backoff wait")
	}
	if h.retryGate.Waiting() != 0 {
		t.Fatalf("waiters left: %d", h.retryGate.Waiting())
	}
}

// G5：等待原因映射。
func TestDeriveWaitingFor(t *testing.T) {
	cases := []struct {
		name       string
		state      lifecycle
		snap       UISnapshot
		cocreating bool
		gate       bool
		exclusive  bool
		want       string
	}{
		{name: "running", state: lifecycleRunning, snap: UISnapshot{IsRunning: true}, want: ""},
		{name: "gate hold", state: lifecyclePaused, snap: UISnapshot{HasAdvanceHold: true}, want: "gate"},
		{name: "gate pause flag", state: lifecyclePaused, gate: true, want: "gate"},
		{name: "cocreate", state: lifecyclePaused, cocreating: true, want: "cocreate"},
		{name: "idle", state: lifecycleIdle, want: "input"},
		{name: "paused by failure", state: lifecyclePaused, want: "input"},
		{name: "exclusive job", state: lifecycleIdle, exclusive: true, want: ""},
		{name: "completed", state: lifecycleCompleted, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveWaitingFor(tc.state, tc.snap, tc.cocreating, tc.gate, tc.exclusive); got != tc.want {
				t.Fatalf("deriveWaitingFor = %q, want %q", got, tc.want)
			}
		})
	}
}

// G4：只有上游声明 can_retry_now 的退避行才显示"Thử lại ngay"。
func TestRetryProgressCanRetryNow(t *testing.T) {
	cases := []struct {
		meta string
		want bool
	}{
		{`{"retry_delay_ms":1000,"can_retry_now":true}`, true},
		{`{"retry_delay_ms":1000}`, false},
		{``, false},
		{`not-json`, false},
	}
	for _, tc := range cases {
		p := &agentcore.ProgressPayload{Meta: []byte(tc.meta)}
		if got := retryProgressCanRetryNow(p); got != tc.want {
			t.Fatalf("meta %q → %v, want %v", tc.meta, got, tc.want)
		}
	}
	if retryProgressCanRetryNow(nil) {
		t.Fatal("nil payload must be false")
	}
}
