package web

import (
	"regexp"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/assets"
)

// metricRefs gắn mỗi số đo với dòng luật đã sinh ra nó: số hiệu § và mức nặng
// mà chính tệp luật của gói gán cho dòng đó (Câu 16). Giao diện dùng cặp này để
// hiện "Bắt sửa/Cảnh báo" + dòng phụ "Master §5". Không đoán: phép đo không có
// dòng luật mang nhãn thì bỏ trống.
type metricRef struct {
	Severity string // error | warning
	Ref      string // "Master §5", "Bộ C §14"
}

// measureRule chỉ cách tìm dòng luật của một phép đo: § chứa dòng đó (đúng số
// hiệu trong thông điệp vi phạm của internal/rules/metrics.go) và các cụm chữ
// dòng luật dùng để nói về phép đo (khớp một trong số đó).
type measureRule struct {
	Section string
	Phrases []string
}

// measureRules là bảng nối khoá số đo với dòng luật của gói. Section lấy từ
// thông điệp vi phạm sẵn có trong internal/rules/metrics.go ("§13 Thoại …",
// "§15.1 Câu vượt giới hạn tiếng", …) nên không phải số viết tay.
var measureRules = map[string]measureRule{
	"thoai":             {Section: "13", Phrases: []string{"Thoại chiếm"}},
	"cau_dai_nhat":      {Section: "15.1", Phrases: []string{"Câu tối đa"}},
	"doan_dai_nhat":     {Section: "15.1", Phrases: []string{"Đoạn tối đa"}},
	"quang_khong_thoai": {Section: "5", Phrases: []string{"250 tiếng"}},
	"tu_thu_tuc":        {Section: "14", Phrases: []string{"từ xác nhận", "quy trình"}},
	"tu_tieng_anh":      {Section: "15.3", Phrases: []string{"tiếng Anh"}},
	"ngay_thu":          {Section: "15.3", Phrases: []string{"ngày thứ"}},
	"cum_lap":           {Section: "13", Phrases: []string{"lặp lại"}},
	"trung_mo_dau":      {Section: "17", Phrases: []string{"câu mở đầu"}},
}

// ruleHeadingRe nhận heading "## 13. …" và "### 15.1 …".
var ruleHeadingRe = regexp.MustCompile(`^(#{2,3})\s+([0-9]+(?:\.[0-9]+)?)\.?\s`)

// packMeasureRefs trả cặp severity/ref cho từng khoá số đo của gói. Dòng "Ghi
// đè §x" trong tệp luật riêng của gói thắng Master (bộ C nâng mục thủ tục lên
// [LỖI]); không có dòng nào mang nhãn → khoá đó không xuất hiện trong map.
func packMeasureRefs(p *assets.Pack) map[string]metricRef {
	out := map[string]metricRef{}
	if p == nil {
		return out
	}
	names := map[string]string{}
	if ok, err := p.Section("nhan_luat", &names); err != nil || !ok {
		names = map[string]string{}
	}
	master := packRuleText(p, "master-rules-v3.md")
	ownFile := packRulesFile(p)
	own := packRuleText(p, ownFile)
	for key, rule := range measureRules {
		if ref, ok := findOverrideRule(own, names[ownFile], rule); ok {
			out[key] = ref
			continue
		}
		if ref, ok := findMasterRule(master, names["master-rules-v3.md"], rule); ok {
			out[key] = ref
		}
	}
	return out
}

// findOverrideRule tìm dòng "Ghi đè §x" có nhãn mức nặng trong tệp luật riêng
// của gói. Dòng ghi đè phải nhắc đúng § và đúng phép đo, để một câu ghi đè §17
// về lời khuyên pháp lý không bị hiểu thành ghi đè phép đo mở đầu.
func findOverrideRule(text, layer string, rule measureRule) (metricRef, bool) {
	marker := "ghi đè §" + rule.Section
	for _, line := range strings.Split(text, "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, marker) || !containsAny(low, rule.Phrases) {
			continue
		}
		if severity, ok := lineSeverity(line); ok {
			return metricRef{Severity: severity, Ref: layerRef(layer, rule.Section)}, true
		}
	}
	return metricRef{}, false
}

// findMasterRule tìm dòng luật có nhãn trong đúng § của tệp Master.
func findMasterRule(text, layer string, rule measureRule) (metricRef, bool) {
	for _, line := range ruleSectionLines(text, rule.Section) {
		if !containsAny(strings.ToLower(line), rule.Phrases) {
			continue
		}
		if severity, ok := lineSeverity(line); ok {
			return metricRef{Severity: severity, Ref: layerRef(layer, rule.Section)}, true
		}
	}
	return metricRef{}, false
}

// ruleSectionLines trả các dòng thân của mục "## x." hoặc "### x.y." (không gồm
// heading của mục con kế tiếp).
func ruleSectionLines(text, section string) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		m := ruleHeadingRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || m[2] != section {
			continue
		}
		body := make([]string, 0, 8)
		for _, next := range lines[i+1:] {
			if ruleHeadingRe.MatchString(strings.TrimSpace(next)) {
				break
			}
			body = append(body, next)
		}
		return body
	}
	return nil
}

// lineSeverity đọc nhãn mức nặng của một dòng luật. Dòng chú giải định nghĩa
// nhãn ("- **[LỖI]:** …") không phải một mục nên bị bỏ.
func lineSeverity(line string) (string, bool) {
	if strings.Contains(line, "**[LỖI]:**") || strings.Contains(line, "**[CẢNH BÁO]:**") {
		return "", false
	}
	switch {
	case strings.Contains(line, "[LỖI]"):
		return "error", true
	case strings.Contains(line, "[CẢNH BÁO]"):
		return "warning", true
	}
	return "", false
}

func containsAny(low string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(low, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

// layerRef ghép tên lớp luật (nhan_luat) với số hiệu §: "Master §5", "Bộ C §14".
func layerRef(layer, section string) string {
	layer = strings.TrimSpace(layer)
	if layer == "" {
		layer = "Master"
	}
	return layer + " §" + section
}
