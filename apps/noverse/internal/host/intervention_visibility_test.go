package host

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

func TestInterventionJudgmentIsInternal(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	model := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg(`{"answer":"Em sẽ kiểm tra cảnh báo.","rules":null,"hold":null,"reopen":null,"ask":null,"dispatch":null,"reason":"Người dùng hỏi khả năng xử lý lỗi"}`)
	}}
	h := &Host{cfg: bootstrap.Config{}, store: st, events: make(chan Event, 32), streamCh: make(chan string, 4), engine: &engine{}, models: &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "m", model, nil)}, interventionPrompt: "sys"}
	h.observer = newObserver(st, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	if err := h.doIntervention("xử lý cảnh báo §8", false); err != nil {
		t.Fatal(err)
	}
	answer, judgment := false, false
	for len(h.events) > 0 {
		ev := <-h.events
		if strings.HasPrefix(ev.Summary, "Phán định:") {
			judgment = true
			if ev.Kind != "internal" || ev.Agent != "arbiter" {
				t.Errorf("judgment published as user chat event: %+v", ev)
			}
		}
		if ev.Summary == "Em sẽ kiểm tra cảnh báo." {
			answer = true
			if ev.Kind == "internal" {
				t.Fatal("public answer hidden")
			}
		}
	}
	if !answer || !judgment {
		t.Fatalf("answer=%v judgment=%v", answer, judgment)
	}
}
