package channels

import (
	"context"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// guessLanguage đoán ngôn ngữ của kênh từ đối thủ tham chiếu (dòng liên kết
// đầu tiên): có dấu tiếng Việt trong tiêu đề/mô tả gần đây → "vi"; chỉ chữ
// Latin không dấu → "en"; không đủ dữ liệu → "" (giữ "auto").
func (s *Service) guessLanguage(ctx context.Context, id string) string {
	sample, err := s.store.ReferenceLanguageText(ctx, id)
	if err != nil {
		return ""
	}
	var parts []string
	parts = append(parts, sample.Title, sample.Description)
	parts = append(parts, sample.Titles...)
	text := strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if looksVietnamese(text) {
		return domain.LangVI
	}
	if looksLatin(text) {
		return domain.LangEN
	}
	return ""
}

// vietnameseRunes là các ký tự chỉ có trong tiếng Việt (hoặc dấu kết hợp
// tương ứng khi văn bản chưa được NFC). Đủ để phân biệt "vi" với "en" mà
// không cần từ điển.
const vietnameseRunes = "ăâđêôơưĂÂĐÊÔƠƯáàảãạắằẳẵặấầẩẫậéèẻẽẹếềểễệíìỉĩịóòỏõọốồổỗộớờởỡợúùủũụứừửữựýỳỷỹỵÁÀẢÃẠẮẰẲẴẶẤẦẨẪẬÉÈẺẼẸẾỀỂỄỆÍÌỈĨỊÓÒỎÕỌỐỒỔỖỘỚỜỞỠỢÚÙỦŨỤỨỪỬỮỰÝỲỶỸỴ"

// looksVietnamese báo văn bản có dấu riêng của tiếng Việt.
func looksVietnamese(text string) bool {
	for _, r := range text {
		if strings.ContainsRune(vietnameseRunes, r) {
			return true
		}
	}
	return false
}

// looksLatin báo mọi chữ cái trong văn bản là Latin không dấu (chữ số, dấu
// câu và khoảng trắng bỏ qua). Có chữ Hán/Nhật/Hàn/Nga → false.
func looksLatin(text string) bool {
	letters := 0
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if r > unicode.MaxLatin1 {
			// Latin mở rộng (dấu châu Âu) vẫn là Latin; ngoài Latin là chữ khác.
			if !unicode.In(r, unicode.Latin) {
				return false
			}
		}
	}
	return letters > 0
}
