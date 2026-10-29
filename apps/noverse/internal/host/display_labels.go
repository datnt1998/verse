package host

import "strings"

// RoleLabel 把引擎角色代号换成界面用的越南语名称。
//
// 名称与 web/src/ui/agentPalette.ts、web/src/screens/cmdresults/logic.ts 的
// ROLE_META 一致；提示词与 tool schema 仍用引擎代号，只有面向用户的文案走这里。
// 未知角色原样返回，避免把不认识的代号静默改名。
func RoleLabel(role string) string {
	key := strings.ToLower(strings.TrimSpace(role))
	switch {
	case key == "" || key == "default":
		return "Mặc định"
	case strings.HasPrefix(key, "architect"):
		return "Kiến trúc sư"
	case strings.HasPrefix(key, "writer"):
		return "Người viết"
	case strings.HasPrefix(key, "editor"):
		return "Biên tập viên"
	case strings.HasPrefix(key, "arbiter"):
		return "Trọng tài"
	}
	return role
}

// thinkingLabels 与 web/src/screens/cmdresults/logic.ts THINKING_LABEL、
// TUI allThinkingOptions 同表；键是引擎的推理强度代号。
var thinkingLabels = map[string]string{
	"":        "Mặc định (kế thừa)",
	"off":     "Tắt",
	"minimal": "Tối thiểu",
	"low":     "Thấp",
	"medium":  "Vừa",
	"high":    "Cao",
	"xhigh":   "Rất cao",
	"max":     "Tối đa",
}

// ThinkingLabel 把推理强度代号换成界面文案；未知值原样返回。
func ThinkingLabel(level string) string {
	key := strings.ToLower(strings.TrimSpace(level))
	if label, ok := thinkingLabels[key]; ok {
		return label
	}
	return level
}

// phaseLabels 与 web/src/i18n/vi.ts `phase`、TUI snapshotPhaseLabel 同表。
var phaseLabels = map[string]string{
	"init":     "Khởi tạo",
	"premise":  "Tiền đề",
	"outline":  "Đại cương",
	"writing":  "Sáng tác",
	"complete": "Hoàn thành",
}

// PhaseLabel 把进度阶段代号换成界面文案；未知值原样返回。
func PhaseLabel(phase string) string {
	if label, ok := phaseLabels[strings.ToLower(strings.TrimSpace(phase))]; ok {
		return label
	}
	return phase
}

// plannerStartLabels 把启动裁定选出的规划师代号换成界面文案
// （Activity.dc.html › Khởi động sáng tác）。枚举封闭：architect_long|architect_short，
// 其余值交回 RoleLabel（architect* → Kiến trúc sư）。
var plannerStartLabels = map[string]string{
	"architect_long":  "Kiến trúc sư lập kế hoạch truyện dài",
	"architect_short": "Kiến trúc sư lập kế hoạch truyện ngắn",
}

// PlannerStartLabel 把启动规划师代号换成界面文案；未知值回退 RoleLabel。
func PlannerStartLabel(planner string) string {
	if label, ok := plannerStartLabels[strings.ToLower(strings.TrimSpace(planner))]; ok {
		return label
	}
	return RoleLabel(planner)
}

// foundationStepLabels 把 FoundationMissing 的缺项键换成“下一步”界面文案。
// 与 web/src/lib/labels.ts 的 TOOLS/FOUNDATION 表同文案（07-copy.md › Bảng từ）：
// 缺什么工件，下一步就是对应的保存动作。
var foundationStepLabels = map[string]string{
	"book":             "Lưu thông tin truyện",
	"premise":          "Lưu tiền đề",
	"outline":          "Lưu dàn ý",
	"layered_outline":  "Lưu dàn ý",
	"characters":       "Lưu nhân vật",
	"world_rules":      "Lưu quy tắc thế giới",
	"compass":          "Cập nhật hướng đi",
	"foundation_audit": "Soát nền truyện",
}

// FoundationStepLabel 把基础设定缺项键换成界面文案；未知键返回空串（调用方降级）。
func FoundationStepLabel(key string) string {
	return foundationStepLabels[strings.ToLower(strings.TrimSpace(key))]
}
