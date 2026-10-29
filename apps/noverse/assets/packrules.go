package assets

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

var (
	packHeadingRE = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	packNumberRE  = regexp.MustCompile(`^(\d+(?:\.\d+)*)(?:\.|\s|$)`)
	packBulletRE  = regexp.MustCompile(`^\s*(?:[-+*]|\d+[.)])\s+(.+)$`)
	packLabelRE   = regexp.MustCompile(`^(?:\*\*)?\[(LỖI|CẢNH BÁO)\]`)
)

// RuleItems phân tích các file luat, bỏ qua khối mã (kể cả script nhúng).
// Gói cũ chưa khai nhan_luat dùng tên file không đuôi làm nhãn tham chiếu.
func (p *Pack) RuleItems() ([]domain.PackRuleItem, error) {
	if p == nil {
		return nil, nil
	}
	var labels map[string]string
	if _, err := p.Section("nhan_luat", &labels); err != nil {
		return nil, err
	}
	var items []domain.PackRuleItem
	for _, file := range p.Luat {
		label := strings.TrimSpace(labels[file.Name])
		if label == "" {
			label = strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
		}
		var sections [6]string
		fence := ""
		active := -1
		for _, line := range strings.Split(file.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				marker := trimmed[:3]
				if fence == "" {
					fence = marker
				} else if fence == marker {
					fence = ""
				}
				active = -1
				continue
			}
			if fence != "" {
				continue
			}
			if heading := packHeadingRE.FindStringSubmatch(trimmed); heading != nil {
				depth := len(heading[1]) - 1
				for i := depth; i < len(sections); i++ {
					sections[i] = ""
				}
				if number := packNumberRE.FindStringSubmatch(heading[2]); number != nil {
					sections[depth] = number[1]
				}
				active = -1
				continue
			}
			if bullet := packBulletRE.FindStringSubmatch(line); bullet != nil {
				active = -1
				section := ""
				for i := len(sections) - 1; i >= 0; i-- {
					if sections[i] != "" {
						section = sections[i]
						break
					}
				}
				if section == "" {
					continue
				}
				item := domain.PackRuleItem{Ref: label + " §" + section, Text: bullet[1]}
				if marker := packLabelRE.FindStringSubmatch(item.Text); marker != nil {
					item.Label = marker[1]
				}
				items = append(items, item)
				active = len(items) - 1
			} else if trimmed == "" || strings.HasPrefix(trimmed, "|") {
				active = -1
			} else if active >= 0 {
				items[active].Text += "\n" + strings.TrimSpace(line)
			}
		}
	}
	return items, nil
}
