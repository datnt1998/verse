package learn

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Change là một kiểu sửa mà code nhận ra được. Key gom các lần sửa giống nhau
// trong một kênh; Text là quy tắc sẽ lưu nếu người dùng đồng ý; Verb là vế câu
// “lần thứ 3 bạn <Verb>” trong gợi ý.
type Change struct {
	Part string
	Key  string
	Text string
	Verb string
}

const (
	KeyTrailingParens = "title.trailing_parens"
	KeyTitleEmoji     = "title.emoji"
	KeyDescEmoji      = "description.emoji"
	KeyTitleShorter   = "title.shorter"
	KeyYearAdded      = "title.year_added"
	KeyYearRemoved    = "title.year_removed"
	KeyTagAdd         = "tags.add:"
	KeyTagRemove      = "tags.remove:"
	// KeyOther gom các lần sửa mà code không phân loại được; model đọc chúng
	// tối đa một lần mỗi ngày theo kênh.
	KeyOther = "other"
)

var (
	trailingParens = regexp.MustCompile(`\s*[(\[（【][^()\[\]（）【】]*[)\]）】]\s*$`)
	yearPattern    = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
)

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	case r == 0xFE0F || r == 0x200D || r == 0x2122 || r == 0x00A9 || r == 0x00AE:
		return true
	}
	return false
}
func hasEmoji(s string) bool {
	for _, r := range s {
		if isEmoji(r) && r != 0x200D && r != 0xFE0F {
			return true
		}
	}
	return false
}
func hasTrailingParens(s string) bool { return trailingParens.MatchString(strings.TrimSpace(s)) }

// ClassifyText phân loại một phần chữ (tiêu đề, mô tả) giữa bản Đạo diễn viết
// và bản chủ kênh sửa. Không có kiểu nào khớp mà chữ đã đổi → KeyOther.
func ClassifyText(part, before, after string) []Change {
	before, after = strings.TrimSpace(before), strings.TrimSpace(after)
	if before == after {
		return nil
	}
	var out []Change
	switch part {
	case "title":
		if hasTrailingParens(before) && !hasTrailingParens(after) {
			out = append(out, Change{part, KeyTrailingParens, "Không thêm phần trong ngoặc ở cuối tiêu đề", "bỏ phần trong ngoặc ở cuối tiêu đề"})
		}
		if hasEmoji(before) && !hasEmoji(after) {
			out = append(out, Change{part, KeyTitleEmoji, "Không dùng emoji trong tiêu đề", "bỏ emoji khỏi tiêu đề"})
		}
		bl, al := utf8.RuneCountInString(before), utf8.RuneCountInString(after)
		// Rút ngắn do bỏ ngoặc/emoji đã có kiểu riêng; không đếm hai lần.
		if len(out) == 0 && bl-al >= 10 && float64(bl-al)/float64(bl) >= 0.15 {
			out = append(out, Change{part, KeyTitleShorter, "", "rút ngắn tiêu đề"})
		}
		by, ay := yearPattern.MatchString(before), yearPattern.MatchString(after)
		if !by && ay {
			out = append(out, Change{part, KeyYearAdded, "Thêm năm vào tiêu đề", "thêm năm vào tiêu đề"})
		}
		if by && !ay {
			out = append(out, Change{part, KeyYearRemoved, "Không thêm năm vào tiêu đề", "bỏ năm khỏi tiêu đề"})
		}
	case "description":
		if hasEmoji(before) && !hasEmoji(after) {
			out = append(out, Change{part, KeyDescEmoji, "Không dùng emoji trong mô tả", "bỏ emoji khỏi mô tả"})
		}
	}
	if len(out) == 0 {
		out = append(out, Change{Part: part, Key: KeyOther})
	}
	return out
}

func tagSet(tags []string) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			m[strings.ToLower(t)] = t
		}
	}
	return m
}

// ClassifyTags nhận ra tag luôn được thêm hoặc luôn bị bỏ.
func ClassifyTags(before, after []string) []Change {
	b, a := tagSet(before), tagSet(after)
	var out []Change
	var added, removed []string
	for k := range a {
		if _, ok := b[k]; !ok {
			added = append(added, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	for _, k := range added {
		out = append(out, Change{"tags", KeyTagAdd + k, fmt.Sprintf("Luôn thêm tag “%s”", a[k]), fmt.Sprintf("thêm tag “%s”", a[k])})
	}
	for _, k := range removed {
		out = append(out, Change{"tags", KeyTagRemove + k, fmt.Sprintf("Không dùng tag “%s”", b[k]), fmt.Sprintf("bỏ tag “%s”", b[k])})
	}
	return out
}

// shorterText dựng quy tắc độ dài từ các tiêu đề đã rút gọn: ngưỡng là độ dài
// dài nhất trong các bản bạn viết, làm tròn lên bội số của 5.
func shorterText(afters []string) string {
	most := 0
	for _, s := range afters {
		if n := utf8.RuneCountInString(strings.TrimSpace(s)); n > most {
			most = n
		}
	}
	limit := (most + 4) / 5 * 5
	if limit <= most {
		limit += 5
	}
	return fmt.Sprintf("Giữ tiêu đề dưới %d ký tự", limit)
}
