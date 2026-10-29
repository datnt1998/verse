package rules

import (
	"sort"
	"strings"
)

// ManagedRulesFile là tệp duy nhất mà giao diện quản lý ở mỗi cấp (mỗi quy tắc một dòng).
// Các tệp .md khác người dùng tự tạo vẫn được đọc nguyên văn như trước.
//
// Khác với tệp thường, tệp này CỘNG DỒN giữa các cấp: quy tắc của dự án và của "Mọi truyện"
// vẫn có hiệu lực khi truyện có quy-tac.md riêng (xem RawFileSources). Nếu dùng chung quy
// tắc "cùng tên thì cấp gần thắng", một dòng ở cấp truyện sẽ vô hiệu hoá toàn bộ cấp trên.
const ManagedRulesFile = "quy-tac.md"

// ParseRuleLines tách nội dung quy-tac.md thành từng quy tắc (thứ tự dòng = thứ tự hiển thị).
// Dòng trống và tiêu đề (#) bị bỏ qua; dấu đầu dòng "- " hoặc "* " được cắt đi.
func ParseRuleLines(text string) []string {
	var out []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line == "-" || line == "*" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "- "); ok {
			line = strings.TrimSpace(rest)
		} else if rest, ok := strings.CutPrefix(line, "* "); ok {
			line = strings.TrimSpace(rest)
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// MergeCandidates gộp các ứng viên (thấp→cao) của cùng một nguồn thành một ứng viên mang nhãn
// source: cùng luật hợp nhất như BuildSnapshot, lời dặn nối bằng xuống dòng (không lặp tiêu đề nguồn).
func MergeCandidates(source string, parts []Candidate) Candidate {
	out := Candidate{Source: source}
	var prefs []string
	for _, c := range parts {
		out.Structured = mergeStructured(out.Structured, sanitizeStructured(c.Structured))
		if p := strings.TrimSpace(c.Preferences); p != "" {
			prefs = append(prefs, p)
		}
		out.Uncertain = append(out.Uncertain, c.Uncertain...)
		out.Degraded = out.Degraded || c.Degraded
	}
	out.Preferences = strings.Join(prefs, "\n")
	return out
}

// ChipKind phân loại "Noverse hiểu là…" của một quy tắc.
type ChipKind string

const (
	ChipBan   ChipKind = "ban"   // cụm từ cấm
	ChipChar  ChipKind = "chr"   // ký tự cấm
	ChipMax   ChipKind = "max"   // từ dễ lặp, ngưỡng theo lần/1.000 tiếng
	ChipGenre ChipKind = "genre" // thể loại
	ChipPref  ChipKind = "pref"  // lời dặn cho trợ lý (không kiểm máy)
)

// Chip là một cách hiểu có cấu trúc của quy tắc. Text là cụm từ/ký tự/thể loại/lời dặn;
// Limit/Per1000 chỉ dùng với ChipMax: Limit là số lần tương đương trên chương chuẩn
// 1.500 tiếng (nhãn cũ "lần/chương" của giao diện vẫn đọc được), Per1000 là ngưỡng thật.
type Chip struct {
	Kind    ChipKind `json:"kind"`
	Text    string   `json:"text"`
	Limit   int      `json:"limit,omitempty"`
	Per1000 float64  `json:"per_1000,omitempty"`
}

// Chips diễn giải một ứng viên thành các chip theo thứ tự cố định:
// thể loại → cụm từ cấm → ký tự cấm → từ dễ lặp (theo chữ cái) → lời dặn. Luôn trả slice không nil.
func Chips(c Candidate) []Chip {
	s := sanitizeStructured(c.Structured)
	chips := []Chip{}
	if s.Genre != "" {
		chips = append(chips, Chip{Kind: ChipGenre, Text: s.Genre})
	}
	for _, p := range s.ForbiddenPhrases {
		chips = append(chips, Chip{Kind: ChipBan, Text: p})
	}
	for _, ch := range s.ForbiddenChars {
		chips = append(chips, Chip{Kind: ChipChar, Text: ch})
	}
	words := make([]string, 0, len(s.FatigueWords))
	for w := range s.FatigueWords {
		words = append(words, w)
	}
	sort.Strings(words)
	for _, w := range words {
		f := s.FatigueWords[w]
		chips = append(chips, Chip{Kind: ChipMax, Text: w, Limit: f.MaxPerChapter(), Per1000: f.Per1000()})
	}
	if p := strings.TrimSpace(c.Preferences); p != "" {
		chips = append(chips, Chip{Kind: ChipPref, Text: p})
	}
	return chips
}

// CheckChapter chạy lint nền và luật người dùng trên chương hiện tại. previous
// chứa chính văn đã nộp, mới nhất trước: hai chương gần nhất dùng so sao chép;
// toàn bộ phần trước dùng đo quãng im thoại/cụm lặp theo script toàn truyện.
func CheckChapter(text string, s Structured, previous []string, openings ...Opening) []Violation {
	vs := Lint(text)
	metrics := s.Measurements
	s.Measurements = nil
	vs = append(vs, Check(text, s)...)
	if metrics != nil {
		config := *metrics
		if len(previous) > 0 {
			config.QuietMax, config.RepeatedMin = 0, 0
		}
		vs = appendMeasurements(vs, metricChapterBody(text), &config, openings)
		if len(previous) > 0 && (metrics.QuietMax > 0 || metrics.RepeatedMin > 0) {
			chapters := make([]string, 0, len(previous)+1)
			for i := len(previous) - 1; i >= 0; i-- {
				chapters = append(chapters, metricChapterBody(previous[i]))
			}
			chapters = append(chapters, metricChapterBody(text))
			storyConfig := Measurements{QuietMax: metrics.QuietMax, RepeatedMin: metrics.RepeatedMin}
			vs = appendMeasurements(vs, strings.Join(chapters, "\n\n\n"), &storyConfig, nil)
		}
	}
	compared := previous
	if metrics != nil {
		compared = previous[:min(len(previous), 2)]
	}
	vs = append(vs, CheckAgainstPrevious(text, compared)...)
	return appendOpeningReplay(vs, text, previous)
}

// Store ghi tiêu đề dạng #; editor của script ghi tiêu đề có số chương.
// Bỏ riêng dòng tiêu đề Markdown đầu tiên, giữ số dòng và toàn bộ chính văn.
func metricChapterBody(text string) string {
	first, rest, found := strings.Cut(text, "\n")
	if strings.HasPrefix(first, "# ") {
		if found {
			return "\n" + rest
		}
		return ""
	}
	return text
}
