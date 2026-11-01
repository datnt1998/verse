package host

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/subagent"
	llm "github.com/datnt1998/verse/core/llm"
)

func TestReloadBoundaryPrecedesDeadlockArbiter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	stale := &scriptedChatModel{fn: func([]llm.Message) llm.Message {
		calls++
		return testTextMsg(`{"action":"abort","dispatch":null,"reason":"stale"}`)
	}}
	e, _, _ := newTestEngine(t, store.NewStore(t.TempDir()), subagent.NewRunner(), stale)
	inst := &flow.Instruction{Agent: "editor", Task: "preserve pending dispatch"}
	e.next = inst
	e.lastKey, e.repeats = instructionKey(inst), deadlockConsultAt-1
	e.prepareWorkerTurn = func() error { return errors.New("global selection temporarily unreadable") }
	e.onPause = func(string) {}
	e.run(context.Background())
	if calls != 0 {
		t.Fatalf("reload boundary called stale Arbiter %d times, want 0", calls)
	}
	if got := e.takeNext(); got != inst {
		t.Fatalf("pending instruction lost: %+v", got)
	}
}

type replayModelError struct{ err error }

func (m replayModelError) SupportsTools() bool { return true }
func (m replayModelError) Generate(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (*llm.LLMResponse, error) {
	return nil, m.err
}
func (m replayModelError) GenerateStream(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (<-chan llm.StreamEvent, error) {
	return nil, m.err
}

func TestLateArbiterFailureDoesNotPauseNewSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	slot := bootstrap.NewSwappableModel("a", "a-model", replayModelError{errors.New("old provider failed")}, nil)
	_, oldError := slot.Generate(context.Background(), nil, nil)
	slot.Swap("b", "b-model", replayModelError{errors.New("unused")}, nil)
	e, _, _ := newTestEngine(t, store.NewStore(t.TempDir()), subagent.NewRunner(), replayModelError{oldError})
	e.onPause = func(body string) { t.Errorf("stale Arbiter paused B: %s", body) }
	inst := &flow.Instruction{Agent: "editor", Task: "keep dispatch"}
	e.lastKey, e.repeats = instructionKey(inst), deadlockConsultAt-1
	if e.trackDeadlock(context.Background(), &inst) {
		t.Fatal("late deadlock decision stopped B")
	}
	e.failedKey = instructionKey(inst)
	if e.handleWorkerError(context.Background(), inst, errors.New("semantic worker failure")) {
		t.Fatal("late failure decision stopped B")
	}
	if got := e.takeNext(); got != inst {
		t.Fatalf("dispatch lost after stale decision: %+v", got)
	}
	if got, err := e.retryPlanStart(context.Background(), "original prompt"); got != nil || !staleModelRequest(err) {
		t.Fatalf("stale plan-start decision not returned for next-turn retry: %+v %v", got, err)
	}
}
