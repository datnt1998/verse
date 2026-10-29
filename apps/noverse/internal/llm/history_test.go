package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

// stubProvider ghi lại request đã qua validate của litellm.Client. Không gọi
// mạng: chỉ trả một phản hồi văn bản tối thiểu.
type stubProvider struct {
	got *litellm.Request
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Chat(_ context.Context, req *litellm.Request) (*litellm.Response, error) {
	s.got = req
	return &litellm.Response{
		Blocks:       []litellm.Block{litellm.TextBlock{Text: "ok"}},
		Provider:     "stub",
		Model:        "stub-model",
		FinishReason: litellm.FinishReasonStop,
	}, nil
}

func (s *stubProvider) Stream(context.Context, *litellm.Request) (litellm.Stream, error) {
	return nil, errStubNoStream
}

type stubErr string

func (e stubErr) Error() string { return string(e) }

const errStubNoStream = stubErr("stub: no stream")

// namelessTurn là hình dạng lịch sử production ghi lại được: assistant gọi một
// tool có call_id nhưng name rỗng (stream Codex mất tên), rồi tool_result lỗi
// "tool \"\" not found". Lượt sau (đúng như log) bị litellm chặn toàn request.
func namelessTurn() []AgentMessage {
	call := ToolCall{ID: "call_1", Name: "", Args: json.RawMessage(`{"chapter":2}`)}
	assistant := Message{
		Role: RoleAssistant,
		Content: []ContentBlock{
			TextBlock("Đọc chương 2."),
			ToolCallBlock(call),
		},
	}
	result := ToolResultMsg("call_1", json.RawMessage(`"tool \"\" not found"`), true)
	return []AgentMessage{UserMsg("Viết chương 2"), assistant, result}
}

func newStubAdapter(t *testing.T) (*ClientModel, *stubProvider) {
	t.Helper()
	provider := &stubProvider{}
	client, err := litellm.New(provider)
	if err != nil {
		t.Fatal(err)
	}
	return NewClientModel("stub-model", client), provider
}

// TestReplayConvertRepairsNamelessToolCall: guard phải đổi tool_use thiếu name
// thành unknown_tool (giữ call_id + arguments) để request hợp lệ.
func TestReplayConvertRepairsNamelessToolCall(t *testing.T) {
	msgs := RepairToolHistory(CollectMessages(namelessTurn()))
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3 (giữ nguyên lượt, không xoá)", len(msgs))
	}
	calls := msgs[1].ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(calls))
	}
	if calls[0].Name != unknownToolName {
		t.Fatalf("tool call name = %q, want %q", calls[0].Name, unknownToolName)
	}
	if calls[0].ID != "call_1" || string(calls[0].Args) != `{"chapter":2}` {
		t.Fatalf("repair must keep id/args, got %#v", calls[0])
	}
	if id, _ := msgs[2].Metadata["tool_call_id"].(string); id != "call_1" {
		t.Fatalf("tool result id = %q, want call_1", id)
	}
}

// TestReplayConvertEndToEndPassesLitellmValidation chứng minh fail-before /
// pass-after ở đúng biên đã chặn production: litellm.Client.validate.
func TestReplayConvertEndToEndPassesLitellmValidation(t *testing.T) {
	adapter, provider := newStubAdapter(t)

	// Trước guard (lịch sử thô đúng như đã lưu): request bị từ chối.
	if _, err := adapter.client.Chat(context.Background(), litellm.Request{Model: "stub-model", Messages: convertMessages(CollectMessages(namelessTurn()))}); err == nil {
		t.Fatal("lịch sử thiếu name phải bị litellm từ chối — nếu không, test không còn chứng minh được lỗi")
	} else if !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("err = %v, want missing name", err)
	}

	// Sau guard: request đi qua, tool_use mang tên thay thế, cặp result khớp id.
	if _, err := adapter.Generate(context.Background(), RepairToolHistory(CollectMessages(namelessTurn())), nil); err != nil {
		t.Fatalf("Generate sau repair: %v", err)
	}
	if provider.got == nil {
		t.Fatal("request chưa tới provider — không chứng minh được validate đã qua")
	}
	var uses, results int
	for _, msg := range provider.got.Messages {
		for _, b := range msg.Blocks {
			switch v := b.(type) {
			case litellm.ToolUseBlock:
				uses++
				if v.Name != unknownToolName || v.ID != "call_1" {
					t.Fatalf("tool use = %#v", v)
				}
			case litellm.ToolResultBlock:
				results++
				if v.ToolUseID != "call_1" {
					t.Fatalf("tool result = %#v", v)
				}
			}
		}
	}
	if uses != 1 || results != 1 {
		t.Fatalf("tool use/result = %d/%d, want 1/1", uses, results)
	}
}

// TestReplayConvertFillsEmptyArguments: call không name thường đi kèm arguments
// rỗng; litellm đòi arguments là JSON hợp lệ nên guard phải đặt "{}".
func TestReplayConvertFillsEmptyArguments(t *testing.T) {
	msgs := []AgentMessage{Message{
		Role:    RoleAssistant,
		Content: []ContentBlock{ToolCallBlock(ToolCall{ID: "call_2", Name: ""})},
	}}
	out := RepairToolHistory(CollectMessages(msgs))
	calls := out[0].ToolCalls()
	if len(calls) != 1 || string(calls[0].Args) != "{}" || calls[0].Name != unknownToolName {
		t.Fatalf("call = %#v, want name unknown_tool + args {}", calls)
	}
}

// splitToolTurn là nguyên văn lượt assistant đã lưu trong
// meta/sessions/agents/writer-ch02.jsonl (2026-10-26T14:56:31 và lần thử lại
// 14:57:32): stream Codex tách MỖI lời gọi làm hai block cùng call_id — block
// đầu mang name với arguments rỗng, block sau mang arguments đầy đủ với name
// rỗng — rồi sinh hai kết quả cho cùng call_id ("InputValidationError …" và
// `tool "" not found`). Lượt sau bị litellm chặn ở messages[4].
func splitToolTurn() []AgentMessage {
	const (
		readID = "call_ca78c217b7b54f3e99aad85db47f9841"
		planID = "call_2273fdde1e68435285c564bede94ffaf"
	)
	assistant := Message{
		Role: RoleAssistant,
		Content: []ContentBlock{
			ToolCallBlock(ToolCall{ID: readID, Name: "read_chapter"}),
			ToolCallBlock(ToolCall{ID: readID, Args: json.RawMessage(`{"chapter":1,"character":"","from":0,"max_runes":12000,"source":"final","to":0}`)}),
			ToolCallBlock(ToolCall{ID: planID, Name: "plan_chapter"}),
			ToolCallBlock(ToolCall{ID: planID, Args: json.RawMessage(`{"chapter":2,"goal":"giữ quyền tiếp cận bản thu"}`)}),
		},
	}
	return []AgentMessage{
		UserMsg("Viết chương 2"),
		assistant,
		ToolResultMsg(readID, json.RawMessage(`"InputValidationError: read_chapter failed due to the following issue:\nThe required parameter `+"`source`"+` is missing"`), true),
		ToolResultMsg(readID, json.RawMessage(`"tool \"\" not found"`), true),
		ToolResultMsg(planID, json.RawMessage(`"InputValidationError: plan_chapter failed due to the following issue:\nThe required parameter `+"`chapter`"+` is missing"`), true),
		ToolResultMsg(planID, json.RawMessage(`"tool \"\" not found"`), true),
	}
}

// TestReplayConvertMergesSplitToolCallHalves: lịch sử bị tách đôi phải được vá
// thành hai lời gọi đủ name + arguments, mỗi lời gọi một kết quả. Trước vá:
// block name rỗng bị đổi tên unknown_tool nhưng call_id trùng vẫn còn nên
// litellm từ chối "duplicate tool use id" — phiên writer không thể chạy tiếp.
func TestReplayConvertMergesSplitToolCallHalves(t *testing.T) {
	msgs := RepairToolHistory(CollectMessages(splitToolTurn()))
	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want 4 (user + assistant + 2 kết quả)", len(msgs))
	}
	calls := msgs[1].ToolCalls()
	want := []struct{ id, name, args string }{
		{"call_ca78c217b7b54f3e99aad85db47f9841", "read_chapter", `{"chapter":1,"character":"","from":0,"max_runes":12000,"source":"final","to":0}`},
		{"call_2273fdde1e68435285c564bede94ffaf", "plan_chapter", `{"chapter":2,"goal":"giữ quyền tiếp cận bản thu"}`},
	}
	if len(calls) != len(want) {
		t.Fatalf("tool calls = %d (%#v), want %d", len(calls), calls, len(want))
	}
	for i, w := range want {
		if calls[i].ID != w.id || calls[i].Name != w.name || string(calls[i].Args) != w.args {
			t.Fatalf("tool call[%d] = %#v, want id=%q name=%q args=%s", i, calls[i], w.id, w.name, w.args)
		}
	}
	seen := make(map[string]bool, len(msgs))
	for _, msg := range msgs {
		if msg.Role != RoleTool {
			continue
		}
		id, _ := msg.Metadata["tool_call_id"].(string)
		if seen[id] {
			t.Fatalf("kết quả trùng cho call %q — provider chỉ nhận một function_call_output mỗi call_id", id)
		}
		seen[id] = true
	}
}

// TestReplayConvertSplitTurnPassesLitellmValidation chứng minh fail-before ở
// đúng biên đã chặn production: lịch sử thô (và bản chỉ đổi tên) bị litellm
// từ chối, bản vá đi qua và tool_use/result khớp nhau.
func TestReplayConvertSplitTurnPassesLitellmValidation(t *testing.T) {
	adapter, provider := newStubAdapter(t)

	if _, err := adapter.client.Chat(context.Background(), litellm.Request{Model: "stub-model", Messages: convertMessages(CollectMessages(splitToolTurn()))}); err == nil {
		t.Fatal("lịch sử tách đôi phải bị litellm từ chối — nếu không, test không còn chứng minh được lỗi")
	} else if !strings.Contains(err.Error(), "duplicate tool use id") && !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("err = %v, want duplicate tool use id hoặc missing name", err)
	}

	if _, err := adapter.Generate(context.Background(), RepairToolHistory(CollectMessages(splitToolTurn())), nil); err != nil {
		t.Fatalf("Generate sau repair: %v", err)
	}
	if provider.got == nil {
		t.Fatal("request chưa tới provider — không chứng minh được validate đã qua")
	}
	var uses, results int
	for _, msg := range provider.got.Messages {
		for _, b := range msg.Blocks {
			switch v := b.(type) {
			case litellm.ToolUseBlock:
				uses++
				if v.Name == "" || v.Name == unknownToolName {
					t.Fatalf("tool use chưa được vá: %#v", v)
				}
			case litellm.ToolResultBlock:
				results++
			}
		}
	}
	if uses != 2 || results != 2 {
		t.Fatalf("tool use/result = %d/%d, want 2/2", uses, results)
	}
}
