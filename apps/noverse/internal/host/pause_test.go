package host

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/agent/subagent"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

func TestPauseAfterStepFinishesCurrentAgentBeforeStopping(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một", CoreEvent: "a"}, {Chapter: 2, Title: "Hai", CoreEvent: "b"}, {Chapter: 3, Title: "Ba", CoreEvent: "c"}}); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var block sync.Once
	script := scriptedWriterModel()
	model := &scriptedChatModel{fn: func(messages []llmtypes.Message) llmtypes.Message {
		block.Do(func() { close(entered); <-release })
		return script.fn(messages)
	}}
	writer := subagent.Config{Name: "writer", Description: "test", Model: model, SystemPrompt: "test", MaxTurns: 10, StopAfterTools: []string{"commit_chapter"}, Tools: []agentcore.Tool{tools.NewPlanChapterTool(st), tools.NewDraftChapterTool(st), tools.NewCheckConsistencyTool(st), tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st))}}
	engine, _, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)
	h := &Host{engine: engine, lifecycle: lifecycleRunning, observer: engine.observer, events: make(chan Event, 8)}
	engine.onPause = func(summary string) { h.abortWithEvent(summary, "info") }
	if !engine.start(nil) {
		t.Fatal("start")
	}
	<-entered
	if !h.PauseAfterStep() {
		t.Fatal("pause request rejected")
	}
	// Requesting a boundary pause must not publish paused or cancel the current
	// model/tool context before the agent has committed its chapter.
	h.mu.Lock()
	state := h.lifecycle
	h.mu.Unlock()
	if state != lifecycleRunning {
		t.Fatalf("pause interrupted the active step: %s", state)
	}
	select {
	case <-done:
		t.Fatal("engine stopped while its step was blocked")
	default:
	}
	close(release)
	waitEngineDone(t, done)
	engine.wait()
	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("pause must preserve chapter 1 and prevent chapter 2: %v", progress.CompletedChapters)
	}
	h.mu.Lock()
	state = h.lifecycle
	h.mu.Unlock()
	if state != lifecyclePaused {
		t.Fatalf("boundary did not publish paused state: %s", state)
	}
	select {
	case event := <-h.events:
		if event.Summary != "Đã tạm dừng sau bước hiện tại" {
			t.Fatalf("wrong pause event: %+v", event)
		}
	default:
		t.Fatal("missing paused event")
	}
}
