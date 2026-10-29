package host

import "testing"

// 界面文案里的角色代号必须换成越南语名称（web/src/ui/agentPalette.ts 同名）。
// 逐项断言：改回英文代号即失败。
func TestRoleLabel(t *testing.T) {
	cases := map[string]string{
		"writer":          "Người viết",
		"editor":          "Biên tập viên",
		"architect_long":  "Kiến trúc sư",
		"architect_short": "Kiến trúc sư",
		"arbiter":         "Trọng tài",
		"default":         "Mặc định",
		"":                "Mặc định",
		"Writer":          "Người viết",
		"unknown_role":    "unknown_role",
	}
	for in, want := range cases {
		if got := RoleLabel(in); got != want {
			t.Errorf("RoleLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// 推理强度代号同理：事件摘要里不能出现 off/medium 这类引擎值。
func TestThinkingLabel(t *testing.T) {
	cases := map[string]string{
		"":       "Mặc định (kế thừa)",
		"off":    "Tắt",
		"low":    "Thấp",
		"medium": "Vừa",
		"high":   "Cao",
		"xhigh":  "Rất cao",
		"max":    "Tối đa",
		"weird":  "weird",
	}
	for in, want := range cases {
		if got := ThinkingLabel(in); got != want {
			t.Errorf("ThinkingLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// 阶段代号出现在用户可见的错误里（precheck 前置条件失败 → 暂停通知）。
func TestPhaseLabel(t *testing.T) {
	cases := map[string]string{
		"premise":  "Tiền đề",
		"outline":  "Đại cương",
		"writing":  "Sáng tác",
		"complete": "Hoàn thành",
		"init":     "Khởi tạo",
		"weird":    "weird",
	}
	for in, want := range cases {
		if got := PhaseLabel(in); got != want {
			t.Errorf("PhaseLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
