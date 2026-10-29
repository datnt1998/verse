package assets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPackRuleItemsRealPacks(t *testing.T) {
	for _, tc := range []struct {
		name, label      string
		errors, warnings int
	}{
		{"nu-tan-bao-thu", "Bộ A", 21, 10},
		{"nguoc-hon-nhan", "Bộ B", 17, 9},
		{"nien-dai-van", "Bộ C", 19, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pack := Load(tc.name, LoadOptions{}).Pack
			items, err := pack.RuleItems()
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string][2]int{}
			master7 := false
			subsection := false
			for _, item := range items {
				file := strings.Split(item.Ref, " §")[0]
				count := counts[file]
				switch item.Label {
				case "LỖI":
					count[0]++
				case "CẢNH BÁO":
					count[1]++
				}
				counts[file] = count
				master7 = master7 || item.Ref == "Master §7" && item.Label == "LỖI" && strings.Contains(item.Text, "trạng thái an toàn")
				subsection = subsection || item.Ref == "Master §15.1" && item.Label == "CẢNH BÁO"
				if strings.Contains(item.Text, "def main(") {
					t.Fatal("không được phân tích script nhúng thành luật")
				}
			}
			for label, want := range map[string][2]int{"Master": {44, 51}, tc.label: {tc.errors, tc.warnings}, "Định dạng": {0, 0}} {
				if got := counts[label]; got != want {
					t.Errorf("%s: số mục [LỖI, CẢNH BÁO] = %v, muốn %v", label, got, want)
				}
			}
			if !master7 || !subsection {
				t.Errorf("thiếu Master §7 LỖI trạng thái an toàn hoặc Master §15.1 CẢNH BÁO")
			}
		})
	}
}

func TestPackRuleItemsHeadingAndBulletFormats(t *testing.T) {
	pack := &Pack{Raw: json.RawMessage(`{"nhan_luat":{"rules.md":"Riêng"}}`), Luat: []PackRuleFile{{Name: "rules.md", Content: "## 7. Cha\n- **[LỖI]:** dòng một\n  dòng hai\n### Không đánh số\n  * [CẢNH BÁO] cảnh báo\n### 15.1 Con\n1. Không nhãn\n```md\n## 99. Giả\n- [LỖI] không đọc\n```\n## Không số cùng cấp\n- [LỖI] không có mục cha\n"}}}
	items, err := pack.RuleItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("nhận %d mục, muốn 3: %+v", len(items), items)
	}
	if items[0].Ref != "Riêng §7" || items[0].Label != "LỖI" || !strings.Contains(items[0].Text, "dòng một\ndòng hai") || items[1].Ref != "Riêng §7" || items[1].Label != "CẢNH BÁO" || items[2].Ref != "Riêng §15.1" || items[2].Label != "" {
		t.Fatalf("sai mục cha, nhãn hoặc nội dung: %+v", items)
	}
	pack.Raw = nil
	items, err = pack.RuleItems()
	if err != nil || items[0].Ref != "rules §7" {
		t.Fatalf("gói cũ dùng tên file: %+v, %v", items, err)
	}
	pack.Raw = json.RawMessage(`{"nhan_luat":123}`)
	if _, err := pack.RuleItems(); err == nil {
		t.Fatal("nhan_luat sai kiểu phải báo lỗi")
	}
}
