package host

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"encoding/json"
	"math"
	"testing"

	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/agentcore/subagent"
)

// TestObserverModelResponseRefreshesContextMeter: thanh ngữ cảnh của một agent
// phải nhích ngay sau mỗi phản hồi model, không đợi ProgressContext (chỉ phát ở
// cuối tool/cuối lượt). Đây là "context không cập nhật liên tục" trên UI.
func TestObserverModelResponseRefreshesContextMeter(t *testing.T) {
	var events []Event
	o := testObserver(&events)

	// Lượt đầu đã được agentcore report: cửa sổ 200k, mới dùng 5 token.
	o.workerProgress(agentcore.ProgressPayload{
		Kind:  agentcore.ProgressContext,
		Agent: "writer",
		Meta:  json.RawMessage(`{"tokens":5,"context_window":200000,"percent":0.0025}`),
	})
	snap := contextOf(t, o, "writer")
	if snap.Context.ContextWindow != 200000 || snap.Context.Tokens != 5 {
		t.Fatalf("seed context = %+v, want window 200000 tokens 5", snap)
	}

	// Backend trả prompt 30954 token (đúng số production ghi ở tui.log).
	response := func(input int) agentcore.Event {
		return agentcore.Event{
			Type: agentcore.EventModelResponse,
			Message: llmtypes.Message{
				Role:    llmtypes.RoleAssistant,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock("...")},
				Usage:   &llmtypes.Usage{Input: input, Output: 10, TotalTokens: input + 10},
			},
		}
	}
	o.captureToolResult(subagent.RunMeta{Agent: "writer"}, response(30954))
	snap = contextOf(t, o, "writer")
	if snap.Context.Tokens != 30954 {
		t.Fatalf("tokens = %d, want 30954 ngay sau phản hồi model", snap.Context.Tokens)
	}
	if want := 30954.0 / 200000 * 100; math.Abs(snap.Context.Percent-want) > 1e-9 {
		t.Fatalf("percent = %v, want %v", snap.Context.Percent, want)
	}

	// Phản hồi kế tiếp trong cùng lượt lại nhích tiếp — không đứng yên.
	o.captureToolResult(subagent.RunMeta{Agent: "writer"}, response(40000))
	snap = contextOf(t, o, "writer")
	if snap.Context.Tokens != 40000 {
		t.Fatalf("tokens = %d, want 40000 ở phản hồi thứ hai", snap.Context.Tokens)
	}
}

// TestObserverModelResponseUsesConfiguredWindow: architect/editor không có
// ContextManager nên agentcore không bao giờ report cửa sổ; thanh ctx vẫn phải
// có mẫu số lấy từ cấu hình model. Chưa resolve được thì chỉ cập nhật Tokens,
// không bịa ra phần trăm.
func TestObserverModelResponseUsesConfiguredWindow(t *testing.T) {
	var events []Event
	o := testObserver(&events)
	o.contextWindowFor = func(agent string) int {
		if agent == "architect_long" {
			return 272000
		}
		return 0
	}

	report := func(agent string, input int) {
		o.captureToolResult(subagent.RunMeta{Agent: agent}, agentcore.Event{
			Type: agentcore.EventModelResponse,
			Message: llmtypes.Message{
				Role:  llmtypes.RoleAssistant,
				Usage: &llmtypes.Usage{Input: input},
			},
		})
	}

	report("architect_long", 40000)
	snap := contextOf(t, o, "architect_long")
	if snap.Context.ContextWindow != 272000 || snap.Context.Tokens != 40000 {
		t.Fatalf("snapshot = %+v, want window 272000 tokens 40000", snap.Context)
	}
	if want := 40000.0 / 272000 * 100; math.Abs(snap.Context.Percent-want) > 1e-9 {
		t.Fatalf("percent = %v, want %v", snap.Context.Percent, want)
	}

	report("editor", 1234)
	snap = contextOf(t, o, "editor")
	if snap.Context.Tokens != 1234 || snap.Context.Percent != 0 || snap.Context.ContextWindow != 0 {
		t.Fatalf("snapshot = %+v, want tokens 1234 with zero percent/window", snap.Context)
	}
}

func contextOf(t *testing.T, o *observer, agent string) AgentSnapshot {
	t.Helper()
	for _, snap := range o.agentSnapshots() {
		if snap.Name == agent {
			return snap
		}
	}
	t.Fatalf("không có snapshot cho agent %q", agent)
	return AgentSnapshot{}
}
