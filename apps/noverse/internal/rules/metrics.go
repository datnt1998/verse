package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// Measurements chỉ bật khi gói khai do_do. Các đơn vị và cách tách văn bản
// giữ nguyên kiem_tra_truyen.py: tiếng = str.split(), đoạn = dòng không rỗng,
// thoại = nội dung giữa “ và ” (không tính dấu ngoặc kép thẳng).
type Measurements struct {
	DialogueMin       float64  `json:"dialogue_min,omitempty"`
	DialogueMax       float64  `json:"dialogue_max,omitempty"`
	ProceduralMax     int      `json:"procedural_max"`
	QuietMax          int      `json:"quiet_max"`
	SentenceMax       int      `json:"sentence_max"`
	ParagraphMax      int      `json:"paragraph_max"`
	RepeatedMin       int      `json:"repeated_min"`
	OpeningWords      int      `json:"opening_words"`
	OpeningOverlapMax float64  `json:"opening_overlap_max"`
	DayOrdinal        bool     `json:"day_ordinal"`
	EnglishWords      []string `json:"english_words"`
}

// PackSections là phần giao diện gói cần cho luật, không phụ thuộc assets
// (assets đã dùng tools/rules). nil nghĩa là văn phong cũ, không bật phép đo.
type PackSections interface {
	Section(string, any) (bool, error)
}

// PackDefaults giải mã một lần lúc dựng checker, trước các luật của người dùng.
// Không đưa kết quả vào BuildSnapshot: gói và ý định người dùng là hai kênh.
// fatigue_words là word→per_1000, không đi qua bộ giải mã ngưỡng legacy.
func PackDefaults(pack PackSections) (Candidate, error) {
	var fatigue map[string]float64
	var measurements Measurements
	if pack == nil {
		return SystemDefaults(), nil
	}
	fatigueDeclared, err := pack.Section("fatigue_words", &fatigue)
	if err != nil {
		return Candidate{}, err
	}
	candidate := SystemDefaults(fatigue)
	candidate.FatigueSources = make(map[string]string, len(candidate.Structured.FatigueWords))
	for word := range candidate.Structured.FatigueWords {
		candidate.FatigueSources[word] = "system_defaults"
	}
	for word, limit := range fatigue {
		word = strings.TrimSpace(word)
		if word != "" && limit >= 0 && candidate.Structured.FatigueWords[word] == FatiguePer1000(limit) {
			candidate.FatigueSources[word] = "genre_pack"
		}
	}
	if fatigueDeclared {
		candidate.Source = "genre_pack"
	}
	ok, err := pack.Section("do_do", &measurements)
	if err != nil || !ok {
		return candidate, err
	}
	candidate.Source = "genre_pack"
	var dialogue struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	}
	if _, err := pack.Section("thoai", &dialogue); err != nil {
		return Candidate{}, err
	}
	measurements.DialogueMin, measurements.DialogueMax = dialogue.Min/100, dialogue.Max/100
	candidate.Structured.Measurements = &measurements
	return candidate, nil
}

// Opening được tiêm từ các truyện anh em trong cùng dự án, không phải chương
// trước của truyện hiện tại. Chỉ chương một cần so sánh mở đầu.
type Opening struct {
	Name string
	Text string
}

type ChapterMeasurement struct {
	Name          string  `json:"name"`
	Words         int     `json:"words"`
	DialogueWords int     `json:"dialogue_words"`
	DialogueRatio float64 `json:"dialogue_ratio"`
	Procedural    int     `json:"procedural"`
}

type QuietStretch struct {
	Start int `json:"start"`
	End   int `json:"end"`
	Words int `json:"words"`
}

type OpeningOverlap struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

// MeasurementResult giữ số đo thô để golden so trực tiếp với script. Các dòng
// đánh số từ một, kể cả tiêu đề và dòng trống; quiet chỉ chứa quãng vượt ngưỡng.
type MeasurementResult struct {
	Chapters       []ChapterMeasurement `json:"chapters"`
	English        map[string]int       `json:"english"`
	DayLines       []int                `json:"day_lines"`
	SentenceLines  []int                `json:"sentence_lines"`
	ParagraphLines []int                `json:"paragraph_lines"`
	Repeated       []string             `json:"repeated"`
	Quiet          []QuietStretch       `json:"quiet"`
	Openings       []OpeningOverlap     `json:"openings"`
}

var (
	metricHeading   = regexp.MustCompile(`(?m)^(?:Chương [\p{Nd}]+:|\x{7b2c} ?[\p{Nd}]+ ?\x{7ae0}).*$`)
	metricDialogue  = regexp.MustCompile(`“([^”]*)”`)
	metricWords     = regexp.MustCompile(`[\p{L}\p{N}_]+`)
	metricParts     = regexp.MustCompile(`\n[\s\p{Z}\x{0085}\x{001c}-\x{001f}]*\n[\s\p{Z}\x{0085}\x{001c}-\x{001f}]*\n`)
	proceduralWords = []string{"xác nhận", "đối chiếu", "niêm phong", "phạm vi", "biên bản", "bảo toàn", "giám định", "lưu ký", "nghiệm thu", "giải ngân", "thủ tục", "văn bản"}
	metricStops     = func() map[string]bool {
		out := make(map[string]bool)
		for _, word := range strings.Fields("tôi cô ta anh chị em của và là đã không có một những cho rồi thì mà với trong ra vào được để này đó nó lại cũng còn người") {
			out[word] = true
		}
		return out
	}()
)

func pythonSpace(r rune) bool        { return unicode.IsSpace(r) || r >= '\x1c' && r <= '\x1f' }
func pythonFields(s string) []string { return strings.FieldsFunc(s, pythonSpace) }
func metricWC(s string) int {
	count, inWord := 0, false
	for _, r := range s {
		if pythonSpace(r) {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}
func pythonWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' }

// boundedCount tương đương (?<!\w)phrase(?!\w), với \w Unicode của Python.
// regexp của Go dùng \b ASCII nên không dùng được cho từ tiếng Việt.
func boundedCount(text, phrase string) int {
	if phrase == "" {
		return 0
	}
	n, offset := 0, 0
	for offset < len(text) {
		i := strings.Index(text[offset:], phrase)
		if i < 0 {
			break
		}
		i += offset
		end := i + len(phrase)
		before, _ := utf8.DecodeLastRuneInString(text[:i])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if (i == 0 || !pythonWord(before)) && (end == len(text) || !pythonWord(after)) {
			n++
		}
		offset = end
	}
	return n
}

func measureChapter(name, body string) ChapterMeasurement {
	c := ChapterMeasurement{Name: name, Words: metricWC(body)}
	for _, quote := range metricDialogue.FindAllStringSubmatch(body, -1) {
		c.DialogueWords += metricWC(quote[1])
	}
	if c.Words > 0 {
		c.DialogueRatio = float64(c.DialogueWords) / float64(c.Words)
	}
	low := strings.ToLower(body)
	for _, word := range proceduralWords {
		c.Procedural += boundedCount(low, word)
	}
	return c
}

// Measure dùng được cả bản editor ghép chương lẫn chính văn một chương.
// Các phép đo toàn truyện (cụm lặp, quãng im thoại) không reset ở tiêu đề,
// đúng với script; CheckChapter đưa thêm chương trước cho hai phép đo này.
func Measure(text string, config Measurements, previous []Opening) MeasurementResult {
	return measure(text, config, previous, true)
}

func measure(text string, config Measurements, previous []Opening, chapterStats bool) MeasurementResult {
	var out MeasurementResult
	if chapterStats {
		heads := metricHeading.FindAllStringIndex(text, -1)
		for i, h := range heads {
			end := len(text)
			if i+1 < len(heads) {
				end = heads[i+1][0]
			}
			out.Chapters = append(out.Chapters, measureChapter(strings.TrimSpace(text[h[0]:h[1]]), text[h[1]:end]))
		}
		if len(heads) == 0 {
			for _, body := range metricParts.Split(text, -1) {
				if metricWC(body) > 0 {
					out.Chapters = append(out.Chapters, measureChapter(fmt.Sprintf("Phần %d", len(out.Chapters)+1), body))
				}
			}
		}
	}
	if len(config.EnglishWords) > 0 {
		low := strings.ToLower(text)
		for _, word := range config.EnglishWords {
			word = strings.ToLower(word)
			if n := boundedCount(low, word); n > 0 {
				if out.English == nil {
					out.English = make(map[string]int)
				}
				out.English[word] = n
			}
		}
	}
	var body []string
	var quiet QuietStretch
	flushQuiet := func() {
		if config.QuietMax > 0 && quiet.Words > config.QuietMax {
			out.Quiet = append(out.Quiet, quiet)
		}
		quiet = QuietStretch{}
	}
	for i, line := range strings.Split(text, "\n") {
		words := metricWC(line)
		if words == 0 || metricHeading.MatchString(line) {
			continue
		}
		n := i + 1
		if config.RepeatedMin > 0 {
			body = append(body, line)
		}
		if config.DayOrdinal && dayOrdinalLine(line) {
			out.DayLines = append(out.DayLines, n)
		}
		if config.SentenceMax > 0 && longSentence(line, config.SentenceMax) {
			out.SentenceLines = append(out.SentenceLines, n)
		}
		if config.ParagraphMax > 0 && words > config.ParagraphMax {
			out.ParagraphLines = append(out.ParagraphLines, n)
		}
		if strings.Contains(line, "“") {
			flushQuiet()
		} else {
			if quiet.Start == 0 {
				quiet.Start = n
			}
			quiet.End = n
			quiet.Words += words
		}
	}
	flushQuiet()
	out.Repeated = repeatedGrams(body, config.RepeatedMin)
	out.Openings = openingOverlaps(text, config.OpeningWords, previous)
	return out
}

// dayOrdinalLine: dòng có "ngày thứ" + số hai–bảy (dễ nghe thành thứ trong tuần).
// Script chỉ có biên ở cuối, không có biên trước "ngày".
func dayOrdinalLine(line string) bool {
	l := strings.ToLower(line)
	for _, ordinal := range []string{"hai", "ba", "tư", "năm", "sáu", "bảy"} {
		phrase := "ngày thứ " + ordinal
		for start := 0; start < len(l); {
			j := strings.Index(l[start:], phrase)
			if j < 0 {
				break
			}
			end := start + j + len(phrase)
			r, _ := utf8.DecodeRuneInString(l[end:])
			if end == len(l) || !pythonWord(r) {
				return true
			}
			start = end
		}
	}
	return false
}

// repeatedGrams trả các cụm 5 tiếng nội dung xuất hiện ≥ min lần (thứ tự gặp đầu tiên).
func repeatedGrams(lines []string, min int) []string {
	if min <= 0 {
		return nil
	}
	tokens := metricWords.FindAllString(strings.ToLower(strings.Join(lines, " ")), -1)
	counts := make(map[[5]string]int)
	var order [][5]string
	for i := 0; i+5 <= len(tokens); i++ {
		g := [5]string(tokens[i : i+5])
		if counts[g] == 0 {
			order = append(order, g)
		}
		counts[g]++
	}
	var out []string
	for _, g := range order {
		if counts[g] < min {
			continue
		}
		content := true
		for _, word := range g {
			if metricStops[word] {
				content = false
				break
			}
		}
		if content {
			out = append(out, strings.Join(g[:], " "))
		}
	}
	return out
}

// openingOverlaps so 220 tiếng đầu của chương một với mở đầu các truyện anh em.
func openingOverlaps(text string, limit int, previous []Opening) []OpeningOverlap {
	if limit <= 0 || len(previous) == 0 {
		return nil
	}
	current := openingGrams(text, limit)
	var out []OpeningOverlap
	for _, old := range previous {
		grams := openingGrams(old.Text, limit)
		shared := 0
		for g := range current {
			if grams[g] {
				shared++
			}
		}
		ratio := float64(shared) / float64(max(len(current), 1))
		out = append(out, OpeningOverlap{Name: old.Name, Ratio: ratio})
	}
	return out
}

// sentenceLengths tách câu như script: cắt ngay sau . ! ? … khi gặp khoảng trắng.
func sentenceLengths(line string) []int {
	var out []int
	start, previous := 0, rune(0)
	for i, r := range line {
		if pythonSpace(r) && strings.ContainsRune(".!?…", previous) {
			out = append(out, metricWC(line[start:i]))
			start = i
		}
		previous = r
	}
	return append(out, metricWC(line[start:]))
}

func longSentence(line string, limit int) bool {
	for _, n := range sentenceLengths(line) {
		if n > limit {
			return true
		}
	}
	return false
}

// bodyLines là các đoạn (dòng có chữ, không phải tiêu đề chương) theo script.
func bodyLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if metricWC(line) == 0 || metricHeading.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func openingGrams(text string, limit int) map[[5]string]bool {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if metricWC(line) > 0 && !metricHeading.MatchString(line) {
			lines = append(lines, line)
		}
	}
	words := pythonFields(strings.ToLower(strings.Join(lines, " ")))
	words = words[:min(len(words), limit)]
	// grams5 loại dấu câu trong từng token, không tách ở dấu câu như cụm lặp.
	var tokens []string
	for _, word := range words {
		word = strings.Map(func(r rune) rune {
			if pythonWord(r) {
				return r
			}
			return -1
		}, word)
		if word != "" {
			tokens = append(tokens, word)
		}
	}
	grams := make(map[[5]string]bool)
	for i := 0; i+5 <= len(tokens); i++ {
		grams[[5]string(tokens[i:i+5])] = true
	}
	return grams
}

func appendMeasurements(vs []Violation, text string, config *Measurements, previous []Opening) []Violation {
	if config == nil {
		return vs
	}
	m := measure(text, *config, previous, config.DialogueMax > 0 || config.ProceduralMax > 0)
	add := func(rule string, actual, limit any, message string) {
		vs = append(vs, Violation{Rule: rule, Actual: actual, Limit: limit, Severity: SeverityWarning, Message: message})
	}
	for _, c := range m.Chapters {
		if config.DialogueMax > 0 && (c.DialogueRatio < config.DialogueMin || c.DialogueRatio > config.DialogueMax) {
			add("dialogue_ratio", c.DialogueRatio, []float64{config.DialogueMin, config.DialogueMax}, fmt.Sprintf("§13 Thoại %.0f%% ngoài khoảng %.0f–%.0f%% ở %s", c.DialogueRatio*100, config.DialogueMin*100, config.DialogueMax*100, c.Name))
		}
		if config.ProceduralMax > 0 && c.Procedural > config.ProceduralMax {
			add("procedural_density", c.Procedural, config.ProceduralMax, fmt.Sprintf("§14 Mật độ thủ tục cao (%d từ) ở %s", c.Procedural, c.Name))
		}
	}
	if len(m.English) > 0 {
		add("english_residue", m.English, nil, "§15.3 Từ tiếng Anh TTS dễ đọc sai")
	}
	if len(m.DayLines) > 0 {
		add("day_ordinal", m.DayLines, nil, "§15.3 'ngày thứ' + số hai–bảy ở các dòng đã ghi")
	}
	if len(m.SentenceLines) > 0 {
		add("sentence_length", m.SentenceLines, config.SentenceMax, "§15.1 Câu vượt giới hạn tiếng ở các dòng đã ghi")
	}
	if len(m.ParagraphLines) > 0 {
		add("paragraph_length", m.ParagraphLines, config.ParagraphMax, "§15.1 Đoạn vượt giới hạn tiếng ở các dòng đã ghi")
	}
	if len(m.Repeated) > 0 {
		add("repeated_5gram", m.Repeated, config.RepeatedMin, "§13 Cụm 5 tiếng lặp lại")
	}
	if len(m.Quiet) > 0 {
		add("quiet_stretch", m.Quiet, config.QuietMax, "§5 Quãng dài không có thoại")
	}
	for _, opening := range m.Openings {
		if opening.Ratio > config.OpeningOverlapMax {
			add("opening_overlap", opening.Ratio, config.OpeningOverlapMax, fmt.Sprintf("§17 Mở đầu trùng %.0f%% cụm 5 tiếng với %s", opening.Ratio*100, opening.Name))
		}
	}
	return vs
}

// ChapterStats là số đo thô của một chương cho bảng "Đo bằng máy" (D6): giá trị
// tuyệt đối của từng phép đo, không chỉ dòng vượt ngưỡng như Violation. Cùng
// định nghĩa đo với script (tiếng = token cách trắng, đoạn = dòng, câu cắt sau
// . ! ? …, thoại chỉ tính “ ”), nên số ở đây khớp số đã dùng để báo vi phạm.
type ChapterStats struct {
	Words         int     `json:"words"`
	DialogueWords int     `json:"dialogue_words"`
	DialogueRatio float64 `json:"dialogue_ratio"`
	Procedural    int     `json:"procedural"`
	MaxSentence   int     `json:"max_sentence"`
	MaxParagraph  int     `json:"max_paragraph"`
	QuietWords    int     `json:"quiet_words"`
	EnglishCount  int     `json:"english_count"`
	RepeatedGrams int     `json:"repeated_grams"`
	DayLines      int     `json:"day_lines"`
	// SentencesOver / ParagraphsOver / QuietOver đếm số câu, số đoạn và số quãng
	// vượt ngưỡng của gói — bảng "Đo bằng máy" (D6) hiện số lượng, không hiện
	// giá trị dài nhất; MaxSentence/MaxParagraph/QuietWords vẫn là giá trị lớn
	// nhất để hiện "dài nhất N tiếng".
	SentencesOver  int              `json:"sentences_over"`
	ParagraphsOver int              `json:"paragraphs_over"`
	QuietOver      int              `json:"quiet_over"`
	Openings       []OpeningOverlap `json:"openings,omitempty"`
}

// MeasureChapterStats đo một chương theo cấu hình `do_do` của gói. Khác Measure
// ở chỗ trả giá trị lớn nhất (câu/đoạn/quãng không thoại) thay vì dòng vi phạm.
func MeasureChapterStats(text string, config Measurements, previous []Opening) ChapterStats {
	text = metricChapterBody(text)
	base := measureChapter("", text)
	out := ChapterStats{
		Words:         base.Words,
		DialogueWords: base.DialogueWords,
		DialogueRatio: base.DialogueRatio,
		Procedural:    base.Procedural,
	}
	lines := bodyLines(text)
	quiet := 0
	flushQuiet := func() {
		if config.QuietMax > 0 && quiet > config.QuietMax {
			out.QuietOver++
		}
		quiet = 0
	}
	for _, line := range lines {
		words := metricWC(line)
		out.MaxParagraph = max(out.MaxParagraph, words)
		if config.ParagraphMax > 0 && words > config.ParagraphMax {
			out.ParagraphsOver++
		}
		for _, n := range sentenceLengths(line) {
			out.MaxSentence = max(out.MaxSentence, n)
			if config.SentenceMax > 0 && n > config.SentenceMax {
				out.SentencesOver++
			}
		}
		if strings.Contains(line, "“") {
			flushQuiet()
			continue
		}
		quiet += words
		out.QuietWords = max(out.QuietWords, quiet)
	}
	flushQuiet()
	low := strings.ToLower(text)
	for _, word := range config.EnglishWords {
		out.EnglishCount += boundedCount(low, strings.ToLower(word))
	}
	if config.DayOrdinal {
		for _, line := range lines {
			if dayOrdinalLine(line) {
				out.DayLines++
			}
		}
	}
	out.RepeatedGrams = len(repeatedGrams(lines, config.RepeatedMin))
	out.Openings = openingOverlaps(text, config.OpeningWords, previous)
	return out
}

// FatigueDensity là mật độ một từ lặp trong chương: số lần thực trên 1.000 tiếng
// và ngưỡng khai trong luật, cùng cách đếm với appendFatigueWords.
type FatigueDensity struct {
	Word    string  `json:"word"`
	Count   int     `json:"count"`
	Per1000 float64 `json:"per_1000"`
	Limit   float64 `json:"limit"`
}

// FatigueDensities đo mọi từ lặp khai trong luật cho một chương, sắp theo mật độ
// giảm dần rồi theo từ để kết quả ổn định (không dùng thứ tự map).
func FatigueDensities(text string, m map[string]FatiguePer1000) []FatigueDensity {
	if len(m) == 0 {
		return nil
	}
	tieng := domain.WordCount(text)
	out := make([]FatigueDensity, 0, len(m))
	for word, limit := range m {
		per1000 := limit.Per1000()
		if word == "" || per1000 < 0 {
			continue
		}
		n := strings.Count(text, word)
		if n == 0 {
			continue
		}
		density := float64(n) * 1000 / float64(max(tieng, 1))
		out = append(out, FatigueDensity{Word: word, Count: n, Per1000: density, Limit: per1000})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Per1000 != out[j].Per1000 {
			return out[i].Per1000 > out[j].Per1000
		}
		return out[i].Word < out[j].Word
	})
	return out
}
