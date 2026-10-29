package host

import (
	"regexp"
	"strings"
)

// Sự kiện TOOL chỉ mang 800 ký tự đầu của kết quả (EventText), mà với chương dài thì
// `content` đứng trước `word_count` trong JSON — phần đuôi bị cắt mất, thẻ bước trong
// Luồng viết không còn số chữ để nói “đã đọc 1.546 chữ”. Trích sẵn vài con số người đọc
// cần từ kết quả gốc thành một object JSON nhỏ (`ResultHint`) trước khi cắt.
var hintPattern = regexp.MustCompile(`"(word_count|count|issues)":(\d+)`)

// resultHint trả `{"word_count":1546,"issues":2}` cho các khoá có trong raw; "" nếu không có.
// Giữ thứ tự xuất hiện (word_count thường đứng trước count/issues khi host ghi kết quả).
func resultHint(raw []byte) string {
	seen := make(map[string]bool, 3)
	var parts []string
	for _, match := range hintPattern.FindAllSubmatch(raw, -1) {
		key := string(match[1])
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, `"`+key+`":`+string(match[2]))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}
