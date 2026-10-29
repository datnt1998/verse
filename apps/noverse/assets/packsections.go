package assets

import (
	"path/filepath"
	"strings"
)

// SectionTitles ánh xạ mã mục luật sang tiêu đề mục của gói: "Master §7" →
// "Đầu chương và cuối chương", "Bộ A §4.3" → "Trà xanh và phản diện".
//
// Khoá dùng đúng cách RuleItems() ghép mã mục (nhãn tệp + " §" + số mục sâu
// nhất) nên khớp 1:1 với rule_ref mà Editor trả về trong save_review. Nhãn tệp
// lấy từ `nhan_luat` của goi.json; gói cũ chưa khai thì dùng tên tệp không đuôi.
// Tiêu đề là phần chữ sau số mục của heading gần nhất có số (heading không số
// không tạo mã mục, giống RuleItems).
func (p *Pack) SectionTitles() map[string]string {
	var titles map[string]string
	p.walkSections(func(ref, title string) {
		if titles == nil {
			titles = make(map[string]string)
		}
		if _, seen := titles[ref]; !seen {
			titles[ref] = title
		}
	})
	return titles
}

// OrderedSectionRefs trả mã mục theo thứ tự tệp luật và heading trong gói.
func (p *Pack) OrderedSectionRefs() []string {
	var refs []string
	seen := map[string]bool{}
	p.walkSections(func(ref, _ string) {
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	})
	return refs
}

func (p *Pack) walkSections(visit func(ref, title string)) {
	if p == nil {
		return
	}
	var labels map[string]string
	if _, err := p.Section("nhan_luat", &labels); err != nil {
		labels = nil
	}
	for _, file := range p.Luat {
		label := strings.TrimSpace(labels[file.Name])
		if label == "" {
			label = strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
		}
		fence := ""
		for _, line := range strings.Split(file.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				marker := trimmed[:3]
				if fence == "" {
					fence = marker
				} else if fence == marker {
					fence = ""
				}
				continue
			}
			if fence != "" {
				continue
			}
			heading := packHeadingRE.FindStringSubmatch(trimmed)
			if heading == nil {
				continue
			}
			number := packNumberRE.FindStringSubmatch(heading[2])
			if number == nil {
				continue
			}
			ref := label + " §" + number[1]
			visit(ref, sectionTitleText(heading[2][len(number[1]):]))
		}
	}
}

// sectionTitleText bỏ dấu phân cách giữa số mục và tiêu đề (". ", ": ", " – ").
func sectionTitleText(rest string) string {
	return strings.TrimSpace(strings.TrimLeft(rest, " .:)-–—|"))
}
