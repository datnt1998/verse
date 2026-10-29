package host

import "strings"

// Marker trong assets/prompts/arbiter-intervention.md đánh dấu khối luật `ask`
// (Trọng tài dừng lại hỏi người dùng). Bật/tắt bằng cấu hình ask_user.
const (
	askRuleStart = "<!-- ask-user:start -->"
	askRuleEnd   = "<!-- ask-user:end -->"
)

// askRulePrompt trả hợp đồng can thiệp đã cắt marker; khi enabled=false thì bỏ
// hẳn khối luật ask để mô hình không biết tới hành động bị khoá.
func askRulePrompt(prompt string, enabled bool) string {
	out := prompt
	for {
		start := strings.Index(out, askRuleStart)
		if start < 0 {
			break
		}
		rest := out[start+len(askRuleStart):]
		end := strings.Index(rest, askRuleEnd)
		if end < 0 {
			break
		}
		block := rest[:end]
		if !enabled {
			block = ""
			// Nuốt luôn dòng trống ngay sau khối để danh sách quy tắc không bị hở.
			after := strings.TrimPrefix(rest[end+len(askRuleEnd):], "\n")
			out = out[:start] + after
			continue
		}
		out = out[:start] + block + rest[end+len(askRuleEnd):]
	}
	return out
}
