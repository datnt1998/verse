package llmretry

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// slowRetryErr 声明 30 秒 Retry-After，让"未被唤醒"与"被唤醒"的耗时差异足够明显。
type slowRetryErr struct{}

func (slowRetryErr) Error() string             { return "rate limited" }
func (slowRetryErr) Retryable() bool           { return true }
func (slowRetryErr) RetryAfter() time.Duration { return 30 * time.Second }

type flakyGenerator struct {
	calls atomic.Int32
}

func (g *flakyGenerator) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	if g.calls.Add(1) == 1 {
		return nil, slowRetryErr{}
	}
	return &llmtypes.LLMResponse{}, nil
}

func TestGateWakeReleasesBackoffImmediately(t *testing.T) {
	gate := NewGate()
	model := &flakyGenerator{}
	done := make(chan error, 1)
	go func() {
		_, err := Generate(context.Background(), model, Config{Agent: "test", Gate: gate}, nil)
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	for gate.Waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("retry never registered a waiter on the gate")
		}
		time.Sleep(time.Millisecond)
	}
	start := time.Now()
	if !gate.Wake() {
		t.Fatal("Wake = false with a waiting retry")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Generate still blocked after Wake (30s Retry-After not interrupted)")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wake took %s, want immediate", elapsed)
	}
	if got := model.calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2 (retry after wake)", got)
	}
}

func TestGateWakeWithoutWaiter(t *testing.T) {
	gate := NewGate()
	if gate.Wake() {
		t.Fatal("Wake = true with no waiter")
	}
	// 空唤醒不得留下信号让下一次退避提前结束。
	done := make(chan error, 1)
	go func() { done <- gate.Wait(context.Background(), 40*time.Millisecond) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after the timer")
	}
}

func TestGateWaitHonoursContext(t *testing.T) {
	gate := NewGate()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- gate.Wait(ctx, time.Minute) }()
	deadline := time.Now().Add(time.Second)
	for gate.Waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("waiter never registered")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait ignored context cancellation")
	}
}
