package llm

import (
	"encoding/json"
)

// unknownToolName là tên thay thế cho tool_call thiếu name trong lịch sử.
// Giữ một tên rõ-ràng-không-tồn-tại để provider chấp nhận message (litellm từ
// chối tool_use name rỗng, hỏng toàn bộ hội thoại — xem CodexToolFix) và để
// model đọc được kết quả lỗi "tool not found" rồi tự sửa ở lượt sau.
const unknownToolName = "unknown_tool"

// repairToolCalls sửa lịch sử đã bị stream làm hỏng, để phiên có thể chạy
// tiếp. Hai dạng hỏng ghi lại được từ production (tui.log 2026-10-26, phiên
// writer chương 2):
//
//  1. Một lời gọi bị tách làm hai block CÙNG call_id: block đầu có name và
//     arguments rỗng, block sau có arguments đầy đủ nhưng name rỗng. litellm
//     chặn request ("duplicate tool use id"), provider cũng từ chối hai
//     function_call trùng id. Phải gộp về một block (name của nửa này,
//     arguments của nửa kia).
//  2. Lời gọi thiếu name (không có nửa mang tên): đổi thành "unknown_tool" —
//     thay vì xoá mất lượt (model sẽ không hiểu vì sao không có kết quả), cặp
//     call/result vẫn khớp và kết quả lỗi `"tool \"unknown_tool\" not found"`
//     cho model biết lần gọi đó không thực hiện được.
//
// Kèm theo: arguments rỗng phải thành "{}" (litellm đòi JSON hợp lệ) và mỗi
// call chỉ giữ MỘT kết quả (provider nhận đúng một function_call_output mỗi
// call_id; bản ghi hỏng có hai kết quả cho cùng id).
func RepairToolHistory(msgs []Message) []Message {
	out := make([]Message, 0, len(msgs))
	seenResults := make(map[string]bool)
	for _, msg := range msgs {
		if msg.Role == RoleAssistant {
			msg = repairAssistantToolCalls(msg)
		}
		if msg.Role == RoleTool && dropDuplicateToolResult(msg, seenResults) {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// repairAssistantToolCalls gộp các block tool call trùng id của một lượt
// assistant rồi chuẩn hoá name/arguments.
func repairAssistantToolCalls(msg Message) Message {
	hasToolCall := false
	for _, b := range msg.Content {
		if b.Type == ContentToolCall && b.ToolCall != nil {
			hasToolCall = true
			break
		}
	}
	if !hasToolCall {
		return msg
	}
	// Vị trí block đầu tiên của mỗi call_id; block trùng id về sau được gộp vào.
	positions := make(map[string]int, len(msg.Content))
	content := make([]ContentBlock, 0, len(msg.Content))
	for _, b := range msg.Content {
		if b.Type != ContentToolCall || b.ToolCall == nil {
			content = append(content, b)
			continue
		}
		tc := *b.ToolCall
		if at, ok := positions[tc.ID]; ok && tc.ID != "" {
			merged := *content[at].ToolCall
			if merged.Name == "" || merged.Name == unknownToolName {
				if tc.Name != "" {
					merged.Name = tc.Name
				}
			}
			merged.Args = mergeToolArgs(merged.Args, tc.Args)
			if merged.ThoughtSignature == "" {
				merged.ThoughtSignature = tc.ThoughtSignature
			}
			content[at] = ToolCallBlock(merged)
			continue
		}
		if tc.ID != "" {
			positions[tc.ID] = len(content)
		}
		content = append(content, ToolCallBlock(tc))
	}
	for i, b := range content {
		if b.Type != ContentToolCall || b.ToolCall == nil {
			continue
		}
		tc := *b.ToolCall
		if tc.Name == "" {
			tc.Name = unknownToolName
		}
		if len(tc.Args) == 0 {
			tc.Args = []byte("{}")
		}
		content[i] = ToolCallBlock(tc)
	}
	msg.Content = content
	return msg
}

// dropDuplicateToolResult báo message công cụ này là bản trùng cho một call_id
// đã có kết quả trước đó. Message role tool rỗng là không hợp lệ nên bản trùng
// bị bỏ hẳn; bản ghi hỏng có hai kết quả cho cùng call_id (kết quả của nửa
// block rỗng tên và của lời gọi thật) trong khi provider chỉ nhận đúng một
// function_call_output mỗi call_id.
func dropDuplicateToolResult(msg Message, seen map[string]bool) bool {
	if msg.Metadata == nil {
		return false
	}
	id, _ := msg.Metadata["tool_call_id"].(string)
	if id == "" {
		return false
	}
	if seen[id] {
		return true
	}
	seen[id] = true
	return false
}

// mergeToolArgs ghép arguments của hai nửa cùng một lời gọi. Chỉ một nửa có dữ
// liệu là trường hợp thường gặp; nếu cả hai đều có thì ưu tiên bản JSON hợp lệ
// (bản dài hơn khi cùng hợp lệ). Không bao giờ nối chuỗi thô — nối sẽ tạo JSON
// hỏng.
func mergeToolArgs(a, b json.RawMessage) json.RawMessage {
	switch {
	case len(a) == 0:
		return b
	case len(b) == 0:
		return a
	case string(a) == string(b):
		return a
	}
	validA, validB := json.Valid(a), json.Valid(b)
	switch {
	case validA && !validB:
		return a
	case validB && !validA:
		return b
	case len(b) > len(a):
		return b
	}
	return a
}
