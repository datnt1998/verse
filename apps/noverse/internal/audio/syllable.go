package audio

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Kiểm tra cấu trúc âm tiết tiếng Việt, phục vụ cờ "english" của englishWords:
// chỉ được cờ một từ KHÔNG phải là âm tiết Việt hợp lệ ("in", "to", "can", "hot"
// là tiếng Anh nhưng cũng là âm tiết Việt — không cờ; "live", "email" thì cờ).

// vietnameseToneMarks chứa 5 dấu thanh; dấu chữ (â, ă, ô, ê, ơ, ư) là phần
// của âm tiết nên phải giữ lại.
var vietnameseToneMarks = map[rune]bool{
	'́': true, // sắc
	'̀': true, // huyền
	'̃': true, // ngã
	'̉': true, // hỏi
	'̣': true, // nặng
}

// stripVietnameseTones trả về dạng gốc của âm tiết: bỏ dấu thanh, đ→d,
// giữ nguyên chữ cái có dấu chữ ("toàn"→"toan", "quốc"→"quôc", "nhược"→"nhược").
func stripVietnameseTones(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if vietnameseToneMarks[r] {
			continue
		}
		b.WriteRune(r)
	}
	out := norm.NFC.String(b.String())
	out = strings.ReplaceAll(out, "Đ", "D")
	out = strings.ReplaceAll(out, "đ", "d")
	return out
}

// viOnsets là âm đầu tiếng Việt, dài nhất trước để ghép không nuốt vần.
var viOnsets = []string{
	"ngh", "ng", "nh", "kh", "gh", "ph", "th", "tr", "ch", "gi", "qu",
	"b", "c", "d", "g", "h", "k", "l", "m", "n", "p", "r", "s", "t", "v", "x",
	"", // âm tiết mở bằng âm chính (ấy, ông, yêu…): không âm đầu
}

// viRhymes là tập vần (âm chính + âm cuối) hợp lệ của tiếng Việt,
// đã bỏ dấu thanh nhưng GIỮ dấu chữ (a|ă|â|e|ê|i|o|ô|ơ|u|ư|y).
var viRhymes = map[string]bool{
	// a / ă / â
	"a": true, "ac": true, "ai": true, "am": true, "an": true, "ang": true, "anh": true,
	"ao": true, "ap": true, "at": true, "ach": true, "au": true, "ay": true,
	"ăc": true, "ăm": true, "ăn": true, "ăng": true, "ăp": true, "ăt": true,
	"âc": true, "âm": true, "ân": true, "âng": true, "âp": true, "ât": true, "âu": true, "ây": true,
	// e / ê
	"e": true, "em": true, "en": true, "eng": true, "eo": true, "ep": true,
	"et": true, "ec": true, "ech": true, "ê": true, "êch": true, "êm": true, "ên": true, "ênh": true,
	"êp": true, "êt": true, "êu": true,
	// i / ia / iê (y sau "qu" hoặc đầu âm tiết: y, quý, yên, quyết, yểu, quỳnh, quýt, yểng)
	"i": true, "ia": true, "im": true, "in": true, "inh": true, "ip": true,
	"it": true, "iu": true, "ich": true,
	"iêm": true, "iên": true, "iêng": true, "iêp": true, "iêt": true, "iêu": true, "iêc": true,
	"y": true, "yêm": true, "yên": true, "yêt": true, "yêu": true, "ynh": true, "yt": true, "yêng": true,
	// o / ô / ơ
	"o": true, "oa": true, "oai": true, "oam": true, "oan": true, "oang": true,
	"oanh": true, "oao": true, "oap": true, "oat": true, "oay": true, "oc": true, "oach": true, "oac": true,
	"oe": true, "oen": true, "oet": true, "oi": true, "om": true, "on": true,
	"ong": true, "oong": true, "oop": true, "op": true, "ot": true,
	"oăc": true, "oăm": true, "oăn": true, "oăng": true, "oăt": true,
	"ô": true, "ôc": true, "ôi": true, "ôm": true, "ôn": true, "ông": true, "ôp": true, "ôt": true,
	"ơ": true, "ơi": true, "ơm": true, "ơn": true, "ơng": true, "ơp": true, "ơt": true,
	// u / ua / uô / ư / ươ / uy (uyên cả "uy" đứng sau qu: uỵ)
	"u": true, "ua": true, "uc": true, "ui": true, "um": true, "un": true,
	"ung": true, "uôc": true, "uôi": true, "uôm": true, "uôn": true, "uông": true,
	"uôt": true, "uôu": true, "up": true, "ut": true, "uya": true, "uyên": true,
	"uyêt": true, "uynh": true, "uyt": true, "uyu": true, "uy": true, "uych": true,
	"uân": true, "uât": true, "uây": true, "uê": true, "uênh": true, "uêt": true, "uệ": true, "uêch": true,
	"uơ": true, "uăt": true, "uồi": true,
	"ư": true, "ưa": true, "ưc": true, "ưi": true, "ưng": true, "ươc": true,
	"ươi": true, "ươm": true, "ươn": true, "ương": true, "ươp": true, "ươt": true,
	"ươu": true, "ưt": true, "ưu": true,
}

// vietnameseNucleusFor k/ gh/ ngh chỉ đứng trước e, ê, i, y.
func vietnameseNucleusFor(onset string) bool {
	switch onset {
	case "k", "gh", "ngh":
		return true
	}
	return false
}

// isVietnameseSyllable báo s có phải đúng một âm tiết Việt viết đúng chính tả.
// Chuỗi có chữ ngoài bảng chữ Việt (f, j, w, z, q đứng một mình) hoặc kết cấu
// âm đầu–vần sai ("mail", "file", "story") → false.
func isVietnameseSyllable(s string) bool {
	base := strings.ToLower(stripVietnameseTones(s))
	if base == "" {
		return false
	}
	for _, onset := range viOnsets {
		if !strings.HasPrefix(base, onset) {
			continue
		}
		rest := base[len(onset):]
		if vietnameseNucleusFor(onset) {
			r, _ := utf8.DecodeRuneInString(rest)
			if r != 'e' && r != 'ê' && r != 'i' && r != 'y' {
				continue // âm đầu này không đi được với vần này; thử âm đầu ngắn hơn
			}
		}
		if viRhymes[rest] {
			return true
		}
	}
	return false
}
