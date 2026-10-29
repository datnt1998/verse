package tools

import (
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// premiseHeadingAliases 把 premise 的二级标题归一到越南语规范名。
// 左列保留旧中文标题：已落盘的历史 premise 仍按中文标题书写，读入时必须照旧解析；
// 右列是当前提示词（assets/prompts/architect-*.md）要求模型输出的越南语标题。
var premiseHeadingAliases = map[string]string{
	// ── 旧中文标题（历史数据兼容）──
	"题材定位":    "Định vị thể loại",
	"题材和基调":   "Thể loại và giọng điệu",
	"核心冲突":    "Xung đột cốt lõi",
	"主角目标":    "Mục tiêu nhân vật chính",
	"结局方向":    "Hướng kết cục",
	"终局方向":    "Hướng kết cục",
	"写作禁区":    "Vùng cấm sáng tác",
	"差异化卖点":   "Điểm bán hàng khác biệt",
	"差异化钩子":   "Móc câu khác biệt",
	"核心兑现承诺":  "Cam kết cốt lõi",
	"故事引擎":    "Động cơ câu chuyện",
	"关系/成长主线": "Tuyến quan hệ/trưởng thành",
	"升级路径":    "Lộ trình nâng cấp",
	"中段转折":    "Chuyển hướng trung kỳ",
	"中期转向":    "Chuyển hướng trung kỳ",
	"终局命题":    "Mệnh đề kết cục",
	"短篇适配性":   "Tính phù hợp với truyện ngắn",
	"本作为什么适合短篇/单卷收束": "Tính phù hợp với truyện ngắn",

	// ── 越南语规范标题 ──
	"Thể loại và giọng điệu":       "Thể loại và giọng điệu",
	"Định vị thể loại":             "Định vị thể loại",
	"Xung đột cốt lõi":             "Xung đột cốt lõi",
	"Mục tiêu nhân vật chính":      "Mục tiêu nhân vật chính",
	"Hướng kết cục":                "Hướng kết cục",
	"Vùng cấm sáng tác":            "Vùng cấm sáng tác",
	"Điểm bán hàng khác biệt":      "Điểm bán hàng khác biệt",
	"Móc câu khác biệt":            "Móc câu khác biệt",
	"Cam kết cốt lõi":              "Cam kết cốt lõi",
	"Động cơ câu chuyện":           "Động cơ câu chuyện",
	"Tuyến quan hệ/trưởng thành":   "Tuyến quan hệ/trưởng thành",
	"Lộ trình nâng cấp":            "Lộ trình nâng cấp",
	"Chuyển hướng trung kỳ":        "Chuyển hướng trung kỳ",
	"Mệnh đề kết cục":              "Mệnh đề kết cục",
	"Tính phù hợp với truyện ngắn": "Tính phù hợp với truyện ngắn",
}

func parsePremiseSections(premise string) map[string]string {
	lines := strings.Split(premise, "\n")
	sections := make(map[string]string)
	var current string
	var body []string

	flush := func() {
		if current == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text != "" {
			sections[current] = text
		}
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if heading, ok := canonicalPremiseHeading(trimmed); ok {
			flush()
			current = heading
			continue
		}
		if current != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

func canonicalPremiseHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if title == "" {
		return "", false
	}
	canonical, ok := premiseHeadingAliases[title]
	return canonical, ok
}

func premiseStructure(premise string, tier domain.PlanningTier) map[string]any {
	sections := parsePremiseSections(premise)
	required := requiredPremiseHeadings(tier)
	found := make([]string, 0, len(required))
	var missing []string
	for _, heading := range required {
		if _, ok := sections[heading]; ok {
			found = append(found, heading)
			continue
		}
		missing = append(missing, heading)
	}

	structure := map[string]any{
		"template_ready": len(missing) == 0,
		"found":          found,
		"missing":        missing,
	}
	if len(sections) > 0 {
		structure["section_count"] = len(sections)
	}
	return structure
}

func requiredPremiseHeadings(tier domain.PlanningTier) []string {
	common := []string{
		"Thể loại và giọng điệu",
		"Định vị thể loại",
		"Xung đột cốt lõi",
		"Mục tiêu nhân vật chính",
		"Hướng kết cục",
		"Vùng cấm sáng tác",
		"Điểm bán hàng khác biệt",
		"Móc câu khác biệt",
		"Cam kết cốt lõi",
	}

	switch tier {
	case domain.PlanningTierLong:
		return append(common,
			"Động cơ câu chuyện",
			"Tuyến quan hệ/trưởng thành",
			"Lộ trình nâng cấp",
			"Chuyển hướng trung kỳ",
			"Mệnh đề kết cục",
		)
	case domain.PlanningTierMid:
		return append(common,
			"Động cơ câu chuyện",
			"Chuyển hướng trung kỳ",
		)
	case domain.PlanningTierShort:
		return append(common,
			"Tính phù hợp với truyện ngắn",
		)
	default:
		return common
	}
}
