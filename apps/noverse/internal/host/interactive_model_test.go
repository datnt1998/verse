package host

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/arbiter"
)

// interactiveHangLimit turns an unbounded retry loop into a failure; it is not a latency budget.
const interactiveHangLimit = 10 * time.Second

// E2ELIB-1/5: a configured but unreachable provider must fail the user's
// action immediately with the Vietnamese provider message, not be retried
// until the (formerly unbounded) context ends.
func TestActionArbiterUnreachableProviderFailsFast(t *testing.T) {
	model := &previewFailureModel{err: retryableNetworkError{}}
	h, _ := newPreviewFailureHost(t, model)
	done := make(chan error, 1)
	go func() {
		ctx, cancel := h.actionContext()
		defer cancel()
		_, err := arbiter.DecidePlanStart(ctx, h.actionArbiterModel(), "Chọn người lập kế hoạch.", "Viết truyện ngắn", "default")
		done <- userFacingProviderError(err)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, llmtypes.ErrProviderNetwork) || !strings.HasPrefix(err.Error(), providerUnreachableMessage) {
			t.Fatalf("unreachable provider error = %v", err)
		}
	case <-time.After(interactiveHangLimit):
		t.Fatal("unreachable provider was retried instead of failing the action")
	}
	if model.calls != 1 {
		t.Fatalf("provider called %d times, want 1", model.calls)
	}
}

func TestPrepareUserRulesUnreachableProviderDoesNotHang(t *testing.T) {
	model := &previewFailureModel{err: retryableNetworkError{}}
	h, _ := newPreviewFailureHost(t, model)
	done := make(chan error, 1)
	go func() { done <- h.PrepareUserRules("Không dùng từ \"bỗng nhiên\".") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("normalization failure must degrade, got %v", err)
		}
	case <-time.After(interactiveHangLimit):
		t.Fatal("rule normalization retried an unreachable provider without bound")
	}
}

func TestActionContextBoundedAndCancelledByAbort(t *testing.T) {
	h, _ := newPreviewFailureHost(t, &previewFailureModel{})
	before := time.Now()
	ctx, cancel := h.actionContext()
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || deadline.Before(before.Add(ActionCallTimeout)) || deadline.After(time.Now().Add(ActionCallTimeout)) {
		t.Fatalf("action deadline=%v ok=%v, want now+%v", deadline, ok, ActionCallTimeout)
	}
	if !h.Abort() {
		t.Fatal("Abort ignored a pending action call")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Abort did not cancel the pending action call")
	}
	cancel()
	if h.Abort() {
		t.Fatal("finished action call stayed registered")
	}
}

// Interactive calls still retry ordinary provider answers (e.g. 429) inside the
// action deadline; only transport failures skip retries.
func TestInteractiveModelKeepsRetryableProviderErrors(t *testing.T) {
	want := retryAfterError{}
	_, err := interactiveModel{ChatModel: &previewFailureModel{err: want}}.Generate(context.Background(), nil, nil)
	var retryable interface{ Retryable() bool }
	if !errors.As(err, &retryable) || !retryable.Retryable() {
		t.Fatalf("rate limit lost retryability: %v", err)
	}
}

type retryAfterError struct{}

func (retryAfterError) Error() string   { return "429 rate limited" }
func (retryAfterError) Retryable() bool { return true }
