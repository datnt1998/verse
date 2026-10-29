package host

import (
	"strings"
	"testing"
)

const promptFixture = "## Quy tắc\n\n" +
	"- **Tiếp tục** → không phân công.\n" +
	askRuleStart + "\n" +
	"- **Hỏi người dùng** → xuất `ask` với `question` và `options`.\n" +
	askRuleEnd + "\n" +
	"- **Thông tin tác phẩm** → architect_short.\n"

func TestAskRulePromptToggle(t *testing.T) {
	off := askRulePrompt(promptFixture, false)
	if strings.Contains(off, "ask") || strings.Contains(off, askRuleStart) || strings.Contains(off, askRuleEnd) {
		t.Fatalf("ask_user tắt phải bỏ hẳn luật ask:\n%s", off)
	}
	if !strings.Contains(off, "**Tiếp tục**") || !strings.Contains(off, "**Thông tin tác phẩm**") {
		t.Fatalf("không được cắt nhầm luật khác:\n%s", off)
	}
	if strings.Contains(off, "\n\n\n") {
		t.Fatalf("bỏ khối không được để dòng trống đôi:\n%q", off)
	}
	on := askRulePrompt(promptFixture, true)
	if !strings.Contains(on, "xuất `ask`") || !strings.Contains(on, "**Tiếp tục**") {
		t.Fatalf("ask_user bật phải giữ luật ask, chỉ bỏ marker:\n%s", on)
	}
	if strings.Contains(on, askRuleStart) || strings.Contains(on, askRuleEnd) {
		t.Fatalf("marker không được lọt vào prompt:\n%s", on)
	}
	// Prompt không có marker (bundle cũ) → giữ nguyên.
	plain := "## Quy tắc\n- x\n"
	if got := askRulePrompt(plain, false); got != plain {
		t.Fatalf("prompt không marker phải giữ nguyên: %q", got)
	}
}
