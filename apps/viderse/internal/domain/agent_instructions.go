package domain

import (
	"fmt"
	"sort"
	"strings"
)

// AgentInstructions là chỉ dẫn riêng của chủ kênh cho từng trợ lý ("Dặn trợ lý
// về kênh này"). Khoá là id trợ lý (AgentIDs); giá trị là câu
// chỉ dẫn đã cắt khoảng trắng, không rỗng.
//
// Đây là dữ liệu người dùng, không phải cấu hình máy: lưu là có readback, không
// gọi model. Chỉ dẫn được chốt vào job lúc nhận vào hàng chờ
// (clone_jobs.instructions) nên sửa kênh không đổi video đã bắt đầu.
type AgentInstructions map[string]string

// MaxAgentInstructionRunes là trần một câu chỉ dẫn (ô nhắn một dòng).
const MaxAgentInstructionRunes = 500

// NewAgentInstructions trả map rỗng (không nil) để wire luôn là `{}`.
func NewAgentInstructions() AgentInstructions { return AgentInstructions{} }

// KnownAgent báo id trợ lý có thật.
func KnownAgent(id string) bool {
	for _, a := range AgentIDs {
		if a == id {
			return true
		}
	}
	return false
}

// NormalizeAgentInstructions bỏ khoá lạ, cắt khoảng trắng và bỏ khoá rỗng.
// Dùng khi đọc từ DB (dữ liệu sửa tay không được làm hỏng trang Kênh).
func NormalizeAgentInstructions(in AgentInstructions) AgentInstructions {
	out := NewAgentInstructions()
	for k, v := range in {
		if !KnownAgent(k) {
			continue
		}
		if text := strings.TrimSpace(v); text != "" {
			out[k] = text
		}
	}
	return out
}

// MergeAgentInstructions áp một bản vá lên bản hiện có: khoá có mặt chỉ đổi
// đúng trợ lý đó (giữ nguyên các trợ lý khác); giá trị rỗng xoá khoá. Khoá lạ
// hoặc câu quá dài là lỗi đầu vào.
func MergeAgentInstructions(current, patch AgentInstructions) (AgentInstructions, error) {
	keys := make([]string, 0, len(patch))
	for k := range patch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !KnownAgent(k) {
			return nil, fmt.Errorf("trợ lý không hợp lệ")
		}
		if n := len([]rune(strings.TrimSpace(patch[k]))); n > MaxAgentInstructionRunes {
			return nil, fmt.Errorf("chỉ dẫn cho %s dài %d ký tự, tối đa %d", agentNames[k], n, MaxAgentInstructionRunes)
		}
	}
	out := NormalizeAgentInstructions(current)
	for _, k := range keys {
		text := strings.TrimSpace(patch[k])
		if text == "" {
			delete(out, k)
			continue
		}
		out[k] = text
	}
	return out, nil
}

// InstructionFor trả chỉ dẫn đã chốt của một trợ lý ("" khi không có).
func (a AgentInstructions) InstructionFor(agent string) string {
	return a[agent]
}
