package host

import (
	"context"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/agent/subagent"
	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/flow"
	llmtypes "github.com/datnt1998/noverse/internal/llm"
	storepkg "github.com/datnt1998/noverse/internal/store"
)

func TestWorkerFinalQuestionIsNotSuccess(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	model := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg("**Đã đọc chương.** Bạn muốn tôi tóm tắt hay kiểm tra lại?")
	}}
	var events []Event
	e := &engine{store: st, observer: testObserver(&events), workers: subagent.NewRunner(subagent.Config{
		Name: "editor", Model: model, SystemPrompt: "test",
	})}
	if err := e.runWorker(context.Background(), &flow.Instruction{Agent: "editor", Task: "Đọc chương"}); err == nil {
		t.Fatal("unanswered final question must be handed to arbiter, not discarded as successful worker output")
	}
	last := events[len(events)-1]
	if last.Failed || last.Level != "warn" || last.Kind != "attention" {
		t.Fatalf("worker handoff must be attention, not execution failure or success: %+v", last)
	}
}

func TestWorkerQuestionUsesExistingAskPath(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "ask enabled", false: "ask disabled"}[enabled], func(t *testing.T) {
			st := storepkg.NewStore(t.TempDir())
			if err := st.Init(); err != nil {
				t.Fatal(err)
			}
			if err := st.RunMeta.Init("default", "test", "test"); err != nil {
				t.Fatal(err)
			}
			if err := st.Progress.Init(3); err != nil {
				t.Fatal(err)
			}
			if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
				t.Fatal(err)
			}
			calls := 0
			model := &scriptedChatModel{fn: func(msgs []llmtypes.Message) llmtypes.Message {
				calls++
				if calls == 1 {
					return testTextMsg(`{"answer":null,"rules":null,"hold":null,"reopen":null,"dispatch":null,"ask":{"question":"Tóm tắt hay kiểm tra lại?","chapter":1,"options":[{"label":"Tóm tắt","reply":"Tóm tắt chương 1 theo nhiệm vụ đang làm","is_default":true},{"label":"Kiểm tra lại","reply":"Đọc lại chương 1 trước khi tóm tắt","is_default":false}]},"reason":"Cần chọn cách xử lý"}`)
				}
				return testTextMsg(`{"answer":"Đã chọn tóm tắt.","rules":null,"hold":null,"reopen":null,"ask":null,"dispatch":{"agent":"editor","task":"Tóm tắt chương 1 theo nhiệm vụ đang làm"},"reason":"Thực hiện phương án đã chọn"}`)
			}}
			h := &Host{cfg: bootstrap.Config{AskUser: &enabled}, store: st, events: make(chan Event, 64), streamCh: make(chan string, 32),
				models: &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "m", model, nil)}}
			h.observer = newObserver(st, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
			h.engine = &engine{store: st, running: true, observer: h.observer, emitEvent: h.emitEvent, resolveQuestion: h.resolveWorkerQuestion, onPause: func(string) {}, notify: func(string, string, string, string) {}}
			h.engine.workers = subagent.NewRunner(subagent.Config{Name: "editor", Model: &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
				return testTextMsg("Bạn muốn tôi tóm tắt hay kiểm tra lại?")
			}}, SystemPrompt: "test"})
			inst := &flow.Instruction{Agent: "editor", Task: "Tóm tắt chương 1"}
			err := h.engine.runWorker(context.Background(), inst)
			handled, stop := h.engine.handleWorkerQuestion(context.Background(), inst, err)
			if !handled || stop != enabled {
				t.Fatalf("question boundary handled=%v stop=%v enabled=%v", handled, stop, enabled)
			}
			meta, err := st.RunMeta.Load()
			if err != nil {
				t.Fatal(err)
			}
			if enabled {
				if meta.PendingAsk == nil || len(meta.PendingAsk.Options) != 2 || meta.PendingAsk.Options[0].Reply == "" {
					t.Fatalf("real question with actionable choices required: %+v", meta.PendingAsk)
				}
				if calls != 1 {
					t.Fatalf("must wait for the user's answer, calls=%d", calls)
				}
				if err := h.doIntervention(meta.PendingAsk.Options[0].Reply, false); err != nil {
					t.Fatal(err)
				}
				h.engine.applyPendingOps(context.Background())
				meta, _ = st.RunMeta.Load()
			} else {
				if calls != 2 {
					t.Fatalf("arbiter must decide using its auto-choice, calls=%d", calls)
				}
				found := false
				for len(h.events) > 0 {
					if strings.Contains((<-h.events).Summary, "Trọng tài tự chọn: Tóm tắt chương 1") {
						found = true
					}
				}
				if !found {
					t.Fatal("missing arbiter auto-choice activity log")
				}
			}
			if meta.PendingAsk != nil {
				t.Fatalf("resolved question remains pending: %+v", meta.PendingAsk)
			}
			if h.engine.next == nil || h.engine.next.Agent != "editor" {
				t.Fatalf("answer must use existing dispatch path, next=%+v", h.engine.next)
			}
		})
	}
}
