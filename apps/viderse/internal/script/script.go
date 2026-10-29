// Package script chứa contract kịch bản video và các phép đo thuần. Hình dạng
// `sections` tương đương contract của module writer (kind/title/narration/
// rehook/cta/scenes[{narration,visual_note}]): khi phần đã chia cảnh, lời đọc
// là các scenes[].narration và narration của phần chỉ là tóm tắt.
package script

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Scene là một cảnh trong phần: lời đọc + ghi chú hình ảnh.
type Scene struct {
	Narration  string `json:"narration"`
	VisualNote string `json:"visual_note"`
}

// Section là một phần của kịch bản, tên trường giữ đúng contract wire.
type Section struct {
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Narration string  `json:"narration"`
	Rehook    bool    `json:"rehook"`
	CTA       bool    `json:"cta"`
	Scenes    []Scene `json:"scenes"`
}

// Các trần độ dài từ contract wire; vượt trần là payload sai, không phải lỗi
// chính sách (thiếu CTA hay sai thứ tự là việc của cổng kiểm sau này).
const (
	MaxSections        = 60
	MaxScenes          = 200
	MaxTitleLen        = 300
	MaxNarrationLen    = 50_000
	MaxSceneNarration  = 20_000
	MaxVisualNoteLen   = 5_000
	DefaultWordsPerMin = 150
)

var sectionKindRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validate kiểm hình dạng sections theo contract: loại phần chữ thường không
// dấu, không được là "parts" (phần thân lưu là từng phần "part"); độ dài trường
// trong trần; ít nhất một phần.
func Validate(sections []Section) error {
	if len(sections) == 0 {
		return fmt.Errorf("kịch bản phải có ít nhất một phần")
	}
	if len(sections) > MaxSections {
		return fmt.Errorf("kịch bản có tối đa %d phần", MaxSections)
	}
	for i, s := range sections {
		where := fmt.Sprintf("phần %d", i+1)
		if !sectionKindRe.MatchString(s.Kind) {
			return fmt.Errorf("%s: loại phần %q chỉ gồm chữ thường không dấu, số và gạch dưới", where, s.Kind)
		}
		if s.Kind == "parts" {
			return fmt.Errorf("%s: phần thân lưu là từng phần \"part\", không lưu \"parts\"", where)
		}
		if utf8.RuneCountInString(s.Title) > MaxTitleLen {
			return fmt.Errorf("%s: title quá %d ký tự", where, MaxTitleLen)
		}
		if utf8.RuneCountInString(s.Narration) > MaxNarrationLen {
			return fmt.Errorf("%s: narration quá %d ký tự", where, MaxNarrationLen)
		}
		if len(s.Scenes) > MaxScenes {
			return fmt.Errorf("%s: tối đa %d cảnh", where, MaxScenes)
		}
		for j, sc := range s.Scenes {
			if utf8.RuneCountInString(sc.Narration) > MaxSceneNarration {
				return fmt.Errorf("%s cảnh %d: narration quá %d ký tự", where, j+1, MaxSceneNarration)
			}
			if utf8.RuneCountInString(sc.VisualNote) > MaxVisualNoteLen {
				return fmt.Errorf("%s cảnh %d: visual_note quá %d ký tự", where, j+1, MaxVisualNoteLen)
			}
		}
	}
	return nil
}

// FactRef là mã nối tham chiếu dữ kiện trong lời đọc: [f:<uuid>]. Lưu trong
// narration nhưng không tính vào đếm từ.
var factRefRe = regexp.MustCompile(`\[f:([0-9a-fA-F-]{36})\]`)

// FactRefs trả các id dữ kiện được tham chiếu trong một đoạn, theo thứ tự
// xuất hiện, không trùng.
func FactRefs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range factRefRe.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// wordRun kiểm một ký tự thuộc từ (chữ hoặc số Unicode).
func wordRun(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// Words tách từ Unicode, chữ thường, đã bỏ mã tham chiếu dữ kiện. Một từ gồm
// run chữ/số, cho phép nháy đơn ' ’ ở giữa (không stem, không bỏ stopword).
func Words(text string) []string {
	text = factRefRe.ReplaceAllString(text, " ")
	text = strings.ToLower(text)
	var out []string
	start := -1
	end := -1 // vị trí sau ký tự chữ/số cuối — nháy cuối từ không thuộc từ
	flush := func(i int) {
		if start >= 0 && end > start {
			out = append(out, text[start:end])
		}
		start, end = -1, -1
	}
	for i, r := range text {
		switch {
		case wordRun(r):
			if start < 0 {
				start = i
			}
			end = i + utf8.RuneLen(r)
		case r == '\'' || r == '’':
			// apostrophe chỉ nối khi có chữ/số hai phía: đang ở giữa thì chờ,
			// cuối từ hoặc đầu từ bị loại bởi end/flush.
		default:
			flush(i)
		}
	}
	flush(len(text))
	return out
}

// CountWords đếm từ của một đoạn (đã bỏ fact refs).
func CountWords(text string) int { return len(Words(text)) }

// SpokenText trả lời đọc của một phần: nối lời các cảnh nếu đã chia cảnh,
// không thì lời của phần (không cộng hai).
func SpokenText(s Section) string {
	if len(s.Scenes) == 0 {
		return s.Narration
	}
	var b strings.Builder
	for i, sc := range s.Scenes {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(sc.Narration)
	}
	return b.String()
}

// FullText là toàn văn do server dựng: lời đọc các phần theo thứ tự, cách
// bằng dòng trống.
func FullText(sections []Section) string {
	parts := make([]string, 0, len(sections))
	for _, s := range sections {
		parts = append(parts, SpokenText(s))
	}
	return strings.Join(parts, "\n\n")
}

// EstimateDurationSec quy số từ ra giây theo tốc độ đọc.
func EstimateDurationSec(wordCount int, wordsPerMinute float64) float64 {
	return float64(wordCount) / wordsPerMinute * 60
}

// SectionMeasure là số đo của một phần trong lần đo chung.
type SectionMeasure struct {
	WordCount      int     `json:"word_count"`
	EstDurationSec float64 `json:"est_duration_sec"`
	StartSec       float64 `json:"start_sec"`
}

// Measure là một lần đo dùng chung: toàn văn, số từ, thời lượng ước tính và
// điểm bắt đầu của từng phần.
type Measure struct {
	FullText       string           `json:"full_text"`
	WordCount      int              `json:"word_count"`
	EstDurationSec float64          `json:"est_duration_sec"`
	Sections       []SectionMeasure `json:"sections"`
}

// MeasureScript đo một kịch bản với tốc độ đọc cho trước.
func MeasureScript(sections []Section, wordsPerMinute float64) Measure {
	start := 0.0
	parts := make([]SectionMeasure, 0, len(sections))
	for _, s := range sections {
		wc := CountWords(SpokenText(s))
		dur := EstimateDurationSec(wc, wordsPerMinute)
		parts = append(parts, SectionMeasure{WordCount: wc, EstDurationSec: dur, StartSec: start})
		start += dur
	}
	total := 0
	for _, p := range parts {
		total += p.WordCount
	}
	return Measure{
		FullText:       FullText(sections),
		WordCount:      total,
		EstDurationSec: EstimateDurationSec(total, wordsPerMinute),
		Sections:       parts,
	}
}

// Brief là đầu vào của một lần chạy writer: chủ đề, khán giả và độ dài mục
// tiêu (giây HOẶC số từ — đúng một trong hai). Audience free-form: TS channel
// chia who/when/wants; khung Go giữ một chuỗi vì không có kênh.
type Brief struct {
	Topic          string `json:"topic"`
	Audience       string `json:"audience,omitempty"`
	TargetSeconds  int    `json:"target_seconds,omitempty"`
	TargetWords    int    `json:"target_words,omitempty"`
	WordsPerMinute int    `json:"words_per_minute,omitempty"`
}

// Validate kiểm brief: topic bắt buộc, đúng một dạng độ dài dương.
func (b Brief) Validate() error {
	if strings.TrimSpace(b.Topic) == "" {
		return fmt.Errorf("thiếu chủ đề (topic)")
	}
	secs, words := b.TargetSeconds, b.TargetWords
	switch {
	case secs > 0 && words > 0:
		return fmt.Errorf("chỉ nhận một dạng độ dài: target_seconds hoặc target_words")
	case secs < 0 || words < 0:
		return fmt.Errorf("độ dài mục tiêu phải dương")
	case secs == 0 && words == 0:
		return fmt.Errorf("thiếu độ dài mục tiêu: target_seconds hoặc target_words")
	}
	if b.WordsPerMinute < 0 {
		return fmt.Errorf("words_per_minute phải dương")
	}
	return nil
}

// WPM trả tốc độ đọc hiệu lực của brief.
func (b Brief) WPM() int {
	if b.WordsPerMinute > 0 {
		return b.WordsPerMinute
	}
	return DefaultWordsPerMin
}

// TargetDescription mô tả độ dài mục tiêu cho prompt (đã chuẩn hoá giây→từ
// qua WPM khi cần).
func (b Brief) TargetDescription() string {
	if b.TargetWords > 0 {
		return fmt.Sprintf("khoảng %d từ", b.TargetWords)
	}
	words := int(float64(b.TargetSeconds) * float64(b.WPM()) / 60)
	return fmt.Sprintf("khoảng %d giây (≈ %d từ ở %d từ/phút)", b.TargetSeconds, words, b.WPM())
}
