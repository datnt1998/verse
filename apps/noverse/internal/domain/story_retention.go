package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AudienceLabels 的索引是 Master §1.2 的固定组号（Câu 18），不是自由文本。
var AudienceLabels = [...]string{
	"Phụ nữ trung niên, nghe khi làm việc nhà", "Nữ 18–35", "Nam trung niên", "Nam trẻ", "Người cao tuổi",
}

// StoryDifference bỏ các câu nhắc khoá ngữ cảnh thay vì nội dung cho người đọc.
// Giữ nguyên câu so sánh thật, không tự sinh nội dung thay dữ liệu cũ.
func StoryDifference(value string) string {
	var clean strings.Builder
	for _, sentence := range strings.SplitAfter(strings.TrimSpace(value), ". ") {
		if strings.Contains(sentence, "other_stories") || strings.Contains(sentence, "foundation_memory") {
			continue
		}
		clean.WriteString(sentence)
	}
	return strings.TrimSpace(clean.String())
}

// AudienceIndices 将旧文件或 gói.json 的组名转换为固定组号；新故事字段只存数组。
func AudienceIndices(labels string) ([]int, error) {
	out := []int{}
	if strings.TrimSpace(labels) == "" {
		return out, nil
	}
	for _, label := range strings.Split(labels, " · ") {
		found := false
		for i, group := range AudienceLabels {
			if strings.EqualFold(strings.TrimSpace(label), group) {
				out = append(out, i)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("nguoi_nghe: nhóm người nghe không hợp lệ: %q", label)
		}
	}
	return out, nil
}

// UnmarshalJSON 读取旧版字符串并迁移为组号；Marshal 使用结构默认行为，始终写数组。
// 部分 premise 更新省略 nguoi_nghe 时保留已有选择。
func (s *StoryRetention) UnmarshalJSON(data []byte) error {
	type plain StoryRetention
	value := plain(*s)
	wire := struct {
		NguoiNghe json.RawMessage `json:"nguoi_nghe"`
		*plain
	}{plain: &value}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.NguoiNghe) > 0 {
		if wire.NguoiNghe[0] == '"' {
			var labels string
			if err := json.Unmarshal(wire.NguoiNghe, &labels); err != nil {
				return err
			}
			indices, err := AudienceIndices(labels)
			if err != nil {
				return err
			}
			value.NguoiNghe = indices
		} else if err := json.Unmarshal(wire.NguoiNghe, &value.NguoiNghe); err != nil {
			return err
		}
	}
	*s = StoryRetention(value)
	return nil
}
