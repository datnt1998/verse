package llmretry

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/agentcore/subagent"
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

// streamIdleErr mirrors the watchdog abort that reaches the wrapper as the
// stream's first event (no byte arrived before the idle window).
type streamIdleErr struct{}

func (streamIdleErr) Error() string             { return "stream idle timeout: no event received for 5m0s" }
func (streamIdleErr) Retryable() bool           { return true }
func (streamIdleErr) RetryAfter() time.Duration { return time.Millisecond }

func failedFirstEvent(err error) <-chan llmtypes.StreamEvent {
	ch := make(chan llmtypes.StreamEvent, 1)
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventError, Err: err}
	close(ch)
	return ch
}

// Pre-stream failures never reach the loop's own retry path, so the session
// log only counts them if the wrapper hands the signals back to the run: one
// retry per swallowed attempt, plus stream_idle when that was the cause. The
// user-facing retry line must still appear once per attempt, not twice.
func TestRetryingModelReportsPreStreamRetriesToTheRun(t *testing.T) {
	inner := &fakeModel{stream: func(call int32) (<-chan llmtypes.StreamEvent, error) {
		switch call {
		case 1:
			return failedFirstEvent(streamIdleErr{}), nil
		case 2:
			return failedFirstEvent(retryableErr{after: time.Millisecond}), nil
		}
		msg := llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("done")}, StopReason: llmtypes.StopReasonStop}
		ch := make(chan llmtypes.StreamEvent, 1)
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventDone, Message: msg, StopReason: msg.StopReason}
		close(ch)
		return ch, nil
	}}
	var progress []agentcore.ProgressPayload
	ctx := agentcore.WithToolProgress(context.Background(), func(p agentcore.ProgressPayload) {
		if p.Kind == agentcore.ProgressRetry {
			progress = append(progress, p)
		}
	})
	var signals []string
	runner := subagent.NewRunner(subagent.Config{
		Name:     "architect_long",
		Model:    NewRetryingModel(inner, NewGate(), "architect", 3),
		MaxTurns: 2,
		OnEvent: func(agent, _ string, ev agentcore.Event) {
			switch ev.Type {
			case agentcore.EventRetry:
				signals = append(signals, fmt.Sprintf("%s retry %d/%d %s", agent, ev.RetryInfo.Attempt, ev.RetryInfo.MaxRetries, agentcore.ErrorKind(ev.Err)))
			case agentcore.EventStreamIdle:
				signals = append(signals, agent+" stream_idle")
			}
		},
	})
	if _, err := runner.Run(ctx, "architect_long", "plan"); err != nil {
		t.Fatal(err)
	}
	want := []string{"architect_long retry 1/3 stream_idle", "architect_long stream_idle", "architect_long retry 2/3 rate_limit"}
	if strings.Join(signals, "\n") != strings.Join(want, "\n") {
		t.Fatalf("signals:\n%s\nwant:\n%s", strings.Join(signals, "\n"), strings.Join(want, "\n"))
	}
	if len(progress) != 2 || progress[0].Agent != "architect" || progress[1].Attempt != 2 {
		t.Fatalf("retry progress lines = %+v, want the wrapper's two lines only", progress)
	}
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
