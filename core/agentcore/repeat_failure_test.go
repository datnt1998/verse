package agentcore

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// countingExecutor counts how many times the wrapped tool actually ran.
type countingExecutor struct {
	runs int
	err  error
}

func (c *countingExecutor) next(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	c.runs++
	if c.err != nil {
		return nil, c.err
	}
	return json.RawMessage(`{"ok":true}`), nil
}

// TestRepeatFailureMiddlewareBlocksIdenticalRetry: an identical call that
// failed RepeatFailureLimit times is refused BEFORE the tool runs again;
// different arguments still execute.
func TestRepeatFailureMiddlewareBlocksIdenticalRetry(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	exec := &countingExecutor{err: errors.New("context payload exceeds budget")}
	call := llmtypes.ToolCall{ID: "call_1", Name: "fetch_doc", Args: json.RawMessage(`{"chapter":0}`)}

	for i := range RepeatFailureLimit {
		if _, err := mw(context.Background(), call, exec.next); err == nil {
			t.Fatalf("call %d: want tool error", i+1)
		}
	}
	if exec.runs != RepeatFailureLimit {
		t.Fatalf("tool ran %d times, want %d", exec.runs, RepeatFailureLimit)
	}

	_, err := mw(context.Background(), call, exec.next)
	if err == nil {
		t.Fatal("call beyond the limit must be refused")
	}
	var blocked *ToolBlockedError
	if !errors.As(err, &blocked) || blocked.Reason != "repeat_failure" {
		t.Fatalf("refusal must be ToolBlockedError{repeat_failure}, got %v", err)
	}
	if !errors.Is(err, ErrToolBlocked) {
		t.Fatalf("refusal must match ErrToolBlocked, got %v", err)
	}
	if exec.runs != RepeatFailureLimit {
		t.Fatalf("tool ran (%d times) despite being blocked", exec.runs)
	}

	// The key is (tool, args): a different argument shape still executes.
	other := llmtypes.ToolCall{ID: "call_2", Name: "fetch_doc", Args: json.RawMessage(`{}`)}
	if _, err := mw(context.Background(), other, exec.next); err == nil {
		t.Fatal("want tool error for the differently-shaped call")
	}
	if exec.runs != RepeatFailureLimit+1 {
		t.Fatalf("differently-shaped call did not run, runs=%d", exec.runs)
	}
}

// TestRepeatFailureMiddlewareClassifiesBlocked: ToolErrorKindOf sees the
// refusal as "blocked" — the metric path for tool_errors_by_kind.blocked.
func TestRepeatFailureMiddlewareClassifiesBlocked(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	exec := &countingExecutor{err: errors.New("boom")}
	call := llmtypes.ToolCall{ID: "c", Name: "read_thing", Args: json.RawMessage(`{"id":1}`)}
	for range RepeatFailureLimit {
		mw(context.Background(), call, exec.next)
	}
	_, err := mw(context.Background(), call, exec.next)
	if got := ToolErrorKindOf(err); got != ToolErrorBlocked {
		t.Fatalf("kind = %q, want %q", got, ToolErrorBlocked)
	}
}

// TestRepeatFailureMiddlewareResetsOnSuccess: one success clears the counter,
// so a later failure streak gets the full allowance again.
func TestRepeatFailureMiddlewareResetsOnSuccess(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	call := llmtypes.ToolCall{ID: "call_1", Name: "read_thing", Args: json.RawMessage(`{"id":8}`)}

	failing := &countingExecutor{err: errors.New("boom")}
	if _, err := mw(context.Background(), call, failing.next); err == nil {
		t.Fatal("want tool error")
	}
	ok := &countingExecutor{}
	if _, err := mw(context.Background(), call, ok.next); err != nil {
		t.Fatalf("successful call: %v", err)
	}
	// After the success the counter is back at 0 → the full allowance again.
	again := &countingExecutor{err: errors.New("boom")}
	for i := range RepeatFailureLimit {
		if _, err := mw(context.Background(), call, again.next); err == nil {
			t.Fatalf("call %d after reset: want tool error", i+1)
		}
	}
	if again.runs != RepeatFailureLimit {
		t.Fatalf("counter did not reset: runs=%d, want %d", again.runs, RepeatFailureLimit)
	}
	if _, err := mw(context.Background(), call, again.next); err == nil {
		t.Fatal("third consecutive failure must be refused")
	}
	if again.runs != RepeatFailureLimit {
		t.Fatalf("tool still ran after being blocked: runs=%d", again.runs)
	}
}

// TestRepeatFailureMiddlewareScopeIsInstanceLifetime pins the counter scope:
// it lives in the middleware's closure, not per call or per "run" — whoever
// installs a shared instance shares the counts, so callers must get the
// blocking benefit across sequential executions of the same tool call.
func TestRepeatFailureMiddlewareScopeIsInstanceLifetime(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	call := llmtypes.ToolCall{ID: "c", Name: "read_thing", Args: json.RawMessage(`{"id":1}`)}

	// Simulates two sequential runs sharing the same middleware instance:
	// one failure each reaches the limit and the next attempt is refused.
	for i := range RepeatFailureLimit {
		exec := &countingExecutor{err: errors.New("boom")}
		if _, err := mw(context.Background(), call, exec.next); err == nil {
			t.Fatalf("run %d: want tool error", i+1)
		}
	}
	exec := &countingExecutor{err: errors.New("boom")}
	if _, err := mw(context.Background(), call, exec.next); err == nil {
		t.Fatal("a fresh executor on the shared middleware must still be refused")
	}
	if exec.runs != 0 {
		t.Fatalf("blocked call executed the tool: runs=%d", exec.runs)
	}
}

// TestRepeatFailureMiddlewareKeyCap: beyond RepeatFailureMaxKeys distinct
// keys the counters are dropped, so a previously blocked key re-arms.
func TestRepeatFailureMiddlewareKeyCap(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	fail := &countingExecutor{err: errors.New("boom")}
	call := llmtypes.ToolCall{ID: "c", Name: "t", Args: json.RawMessage(`{"id":0}`)}

	// Block key id=0.
	for range RepeatFailureLimit {
		mw(context.Background(), call, fail.next)
	}
	if _, err := mw(context.Background(), call, fail.next); err == nil {
		t.Fatal("key must be blocked")
	}
	before := fail.runs

	// Push the map past the cap with distinct keys.
	for i := 1; i <= RepeatFailureMaxKeys+2; i++ {
		args := json.RawMessage(`{"id":` + strings.Repeat("1", i) + `}`)
		extra := &countingExecutor{err: errors.New("boom")}
		mw(context.Background(), llmtypes.ToolCall{ID: "x", Name: "t", Args: args}, extra.next)
	}

	// The original key's counter was cleared → it runs (and fails) again.
	if _, err := mw(context.Background(), call, fail.next); err == nil {
		t.Fatal("want tool error after cap reset")
	}
	if fail.runs != before+1 {
		t.Fatalf("capped counter did not clear: runs=%d, want %d", fail.runs, before+1)
	}
}

// TestRepeatFailureMiddlewareCustomRefusal: an app may supply the refusal
// text (e.g. domain-specific hints) while the ToolBlockedError wrapper and
// reason stay intact.
func TestRepeatFailureMiddlewareCustomRefusal(t *testing.T) {
	mw := NewRepeatFailureMiddleware(&RepeatFailureOptions{
		Refusal: func(call llmtypes.ToolCall, failed int) error {
			return errors.New("custom refusal for " + call.Name)
		},
	})
	exec := &countingExecutor{err: errors.New("boom")}
	call := llmtypes.ToolCall{ID: "c", Name: "t", Args: json.RawMessage(`{}`)}
	for range RepeatFailureLimit {
		mw(context.Background(), call, exec.next)
	}
	_, err := mw(context.Background(), call, exec.next)
	if err == nil || !strings.Contains(err.Error(), "custom refusal for t") {
		t.Fatalf("want custom refusal text, got %v", err)
	}
	var blocked *ToolBlockedError
	if !errors.As(err, &blocked) || blocked.Reason != "repeat_failure" {
		t.Fatalf("custom refusal must keep the blocked wrapper, got %v", err)
	}
}

// TestRepeatFailureMiddlewareDefaultRefusal: the generic message names the
// tool, the failure count and tells the model to change arguments.
func TestRepeatFailureMiddlewareDefaultRefusal(t *testing.T) {
	mw := NewRepeatFailureMiddleware(nil)
	exec := &countingExecutor{err: errors.New("boom")}
	call := llmtypes.ToolCall{ID: "c", Name: "fetch_doc", Args: json.RawMessage(`{}`)}
	for range RepeatFailureLimit {
		mw(context.Background(), call, exec.next)
	}
	_, err := mw(context.Background(), call, exec.next)
	if err == nil {
		t.Fatal("want refusal")
	}
	for _, want := range []string{"fetch_doc", "2 times", "change the arguments"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal missing %q: %v", want, err)
		}
	}
}
