package llmretry

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
)

type retryableErr struct{ after time.Duration }

func (e retryableErr) Error() string             { return "429 rate limited" }
func (e retryableErr) Retryable() bool           { return true }
func (e retryableErr) RetryAfter() time.Duration { return e.after }

type fakeModel struct {
	genCalls atomic.Int32
	strCalls atomic.Int32
	gen      func(call int32) (*llmtypes.LLMResponse, error)
	stream   func(call int32) (<-chan llmtypes.StreamEvent, error)
	supports bool
}

func (m *fakeModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return m.gen(m.genCalls.Add(1))
}

func (m *fakeModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return m.stream(m.strCalls.Add(1))
}

func (m *fakeModel) SupportsTools() bool { return m.supports }

func okStream(text string) <-chan llmtypes.StreamEvent {
	ch := make(chan llmtypes.StreamEvent, 2)
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventTextDelta, Delta: text}
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventDone}
	close(ch)
	return ch
}

func TestRetryingModelRetriesOpenTimeError(t *testing.T) {
	inner := &fakeModel{gen: func(call int32) (*llmtypes.LLMResponse, error) {
		if call == 1 {
			return nil, retryableErr{after: time.Millisecond}
		}
		return &llmtypes.LLMResponse{}, nil
	}}
	var retries []agentcore.ProgressPayload
	ctx := agentcore.WithToolProgress(context.Background(), func(p agentcore.ProgressPayload) {
		retries = append(retries, p)
	})
	m := NewRetryingModel(inner, NewGate(), "writer", 3)

	resp, err := m.Generate(ctx, nil, nil)
	if err != nil || resp == nil {
		t.Fatalf("Generate = %v, %v", resp, err)
	}
	if inner.genCalls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", inner.genCalls.Load())
	}
	if len(retries) != 1 {
		t.Fatalf("retry reports = %d, want 1", len(retries))
	}
	p := retries[0]
	if p.Kind != agentcore.ProgressRetry || p.Attempt != 1 || p.MaxRetries != 3 || p.Agent != "writer" {
		t.Fatalf("payload = %+v", p)
	}
	var meta struct {
		DelayMS     int64 `json:"retry_delay_ms"`
		CanRetryNow bool  `json:"can_retry_now"`
	}
	if err := json.Unmarshal(p.Meta, &meta); err != nil || !meta.CanRetryNow || meta.DelayMS <= 0 {
		t.Fatalf("meta = %s (%v)", p.Meta, err)
	}
}

func TestRetryingModelWakeShortensWait(t *testing.T) {
	gate := NewGate()
	inner := &fakeModel{gen: func(call int32) (*llmtypes.LLMResponse, error) {
		if call == 1 {
			return nil, retryableErr{after: 30 * time.Second}
		}
		return &llmtypes.LLMResponse{}, nil
	}}
	m := NewRetryingModel(inner, gate, "writer", 3)
	done := make(chan error, 1)
	go func() {
		_, err := m.Generate(context.Background(), nil, nil)
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for gate.Waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("wrapper never waited on the gate")
		}
		time.Sleep(time.Millisecond)
	}
	start := time.Now()
	if !gate.Wake() {
		t.Fatal("Wake = false with wrapper waiting")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wake did not release the 30s wait")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wake took %s", elapsed)
	}
}

func TestRetryingModelDoesNotRetryMidStreamError(t *testing.T) {
	inner := &fakeModel{stream: func(call int32) (<-chan llmtypes.StreamEvent, error) {
		ch := make(chan llmtypes.StreamEvent, 2)
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventTextDelta, Delta: "hello"}
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventError, Err: retryableErr{}}
		close(ch)
		return ch, nil
	}}
	m := NewRetryingModel(inner, NewGate(), "writer", 3)
	ch, err := m.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var events []llmtypes.StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if inner.strCalls.Load() != 1 {
		t.Fatalf("stream calls = %d, want 1 (mid-stream error must pass through)", inner.strCalls.Load())
	}
	if len(events) != 2 || events[1].Type != llmtypes.StreamEventError {
		t.Fatalf("events = %+v", events)
	}
}

func TestRetryingModelRetriesFirstEventError(t *testing.T) {
	inner := &fakeModel{stream: func(call int32) (<-chan llmtypes.StreamEvent, error) {
		if call == 1 {
			ch := make(chan llmtypes.StreamEvent, 1)
			ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventError, Err: retryableErr{after: time.Millisecond}}
			close(ch)
			return ch, nil
		}
		return okStream("ok"), nil
	}}
	m := NewRetryingModel(inner, NewGate(), "writer", 3)
	ch, err := m.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var types []llmtypes.StreamEventType
	for ev := range ch {
		types = append(types, ev.Type)
	}
	if inner.strCalls.Load() != 2 {
		t.Fatalf("stream calls = %d, want 2", inner.strCalls.Load())
	}
	if len(types) != 2 || types[0] != llmtypes.StreamEventTextDelta || types[1] != llmtypes.StreamEventDone {
		t.Fatalf("event types = %v", types)
	}
}

func TestRetryingModelPassesThroughCancellation(t *testing.T) {
	inner := &fakeModel{gen: func(int32) (*llmtypes.LLMResponse, error) {
		return nil, context.Canceled
	}}
	m := NewRetryingModel(inner, NewGate(), "writer", 3)
	_, err := m.Generate(context.Background(), nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if inner.genCalls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", inner.genCalls.Load())
	}
}

func TestRetryingModelMarksExhaustedErrorNonRetryable(t *testing.T) {
	inner := &fakeModel{gen: func(int32) (*llmtypes.LLMResponse, error) {
		return nil, retryableErr{after: time.Millisecond}
	}}
	m := NewRetryingModel(inner, NewGate(), "writer", 1)
	_, err := m.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if inner.genCalls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (first + one retry)", inner.genCalls.Load())
	}
	if m.retryable(err) {
		t.Fatalf("exhausted error is still retryable: %v", err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("message lost: %v", err)
	}
}
