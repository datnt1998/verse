package assistant

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MaxTextRunes là độ dài tối đa của một câu nhắn trợ lý.
const MaxTextRunes = 500

// fold hạ chữ thường và bỏ dấu tiếng Việt để so tên người dùng gõ với tên
// kênh/đối thủ đã lưu ("nocturne stories" khớp "Nocturne Stories", "ma" khớp
// "Mà"). Chỉ dùng để so khớp, không bao giờ để hiển thị.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ':
			b.WriteRune('d')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// matchName báo `said` (người dùng/mô hình nêu) có chỉ tới `name` không: bằng
// nhau hoặc chứa nhau sau khi bỏ dấu, từ 3 ký tự trở lên để "a" không khớp
// mọi tên.
func matchName(said, name string) bool {
	a, b := fold(said), fold(name)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if utf8.RuneCountInString(a) < 3 {
		return false
	}
	return strings.Contains(b, a) || strings.Contains(a, b)
}

// clip cắt s còn tối đa n ký tự (rune), thêm … khi bị cắt.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// cleanLine gom khoảng trắng/xuống dòng thành một dòng — dữ liệu từ người dùng
// hoặc mô hình không được phá bố cục thẻ.
func cleanLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// youTubeLink lấy link người dùng dán trong câu (/posted [link]); bỏ dấu câu
// dính cuối. Rỗng khi câu không có link — link là tuỳ chọn.
func youTubeLink(text string) string {
	for _, field := range strings.Fields(text) {
		token := strings.Trim(field, `.,;:!?()[]{}<>"'“”‘’`)
		lower := strings.ToLower(token)
		for _, prefix := range []string{"http://", "https://", "youtu.be/", "www.youtube.com/", "youtube.com/", "m.youtube.com/"} {
			if strings.HasPrefix(lower, prefix) {
				return token
			}
		}
	}
	return ""
}
