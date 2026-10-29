// Anchor resolution: một vị trí góp ý = trích dẫn + ngữ cảnh trước/sau, không
// còn chỉ là số đoạn. Tách đoạn phải khớp bộ đếm của FE (web/src/lib/markdown.tsx):
// chỉ khối paragraph được đếm; tiêu đề, trích, danh sách và dấu ngắt cảnh bị bỏ.
package comments

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	anchorBreakRe   = regexp.MustCompile(`^\s*(?:---+|\*\s*\*\s*\*|___+)\s*$`)
	anchorHeadingRe = regexp.MustCompile(`^#{1,6}\s+.*$`)
	anchorQuoteRe   = regexp.MustCompile(`^\s*>\s?.*$`)
	anchorListRe    = regexp.MustCompile(`^\s*[-*+]\s+.*$`)
	anchorInlineRe  = regexp.MustCompile(`(\*\*[^*]+\*\*|__[^_]+__|\*[^*\n]+\*|_[^_\n]+_|` + "`[^`]+`" + `)`)
)

// anchorContextLen là số rune ngữ cảnh trước/sau trích dẫn lưu trong Anchor.
const anchorContextLen = 48

// anchorFuzzyFloor là ngưỡng tương đồng tối thiểu để một đoạn lọt vào ứng viên.
// Dice trigram trên văn bản đã chuẩn hoá; ngưỡng 0.6 tránh cả dời nhầm lẫn
// "không thấy gì cũng gán".
const anchorFuzzyFloor = 0.6

// anchorCandidatesMax giới hạn số ứng viên lưu trong thẻ Không chắc vị trí.
const anchorCandidatesMax = 3

// Paragraphs tách chính văn thành các đoạn văn xuôi (kind=="paragraph" của FE),
// giữ nguyên thứ tự và chỉ số mà FE dùng để gắn badge. Mỗi phần tử là văn bản
// thô của đoạn (chưa strip nhấn mạnh, chưa gom dòng).
func Paragraphs(markdown string) []string {
	text := strings.TrimSpace(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(markdown))
	if text == "" {
		return nil
	}
	var paras []string
	var para []string
	inQuote, inList := false, false
	flush := func() {
		if len(para) > 0 {
			paras = append(paras, strings.Join(para, "\n"))
			para = nil
		}
		inQuote, inList = false, false
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if anchorBreakRe.MatchString(line) {
			flush()
			continue
		}
		if anchorHeadingRe.MatchString(line) {
			flush()
			continue
		}
		if anchorQuoteRe.MatchString(line) {
			if !inQuote {
				flush()
			}
			inQuote = true
			continue
		}
		if anchorListRe.MatchString(line) {
			if !inList {
				flush()
			}
			inList = true
			continue
		}
		inQuote, inList = false, false
		para = append(para, line)
	}
	flush()
	return paras
}

// normalizeText gom khoảng trắng như FE (\s+ → " ") sau khi bỏ nhấn mạnh inline.
func normalizeText(text string) string {
	text = anchorInlineRe.ReplaceAllStringFunc(text, func(token string) string {
		switch {
		case strings.HasPrefix(token, "**"), strings.HasPrefix(token, "__"), strings.HasPrefix(token, "`"):
			return token[2 : len(token)-2]
		default:
			return token[1 : len(token)-1]
		}
	})
	return strings.Join(strings.Fields(text), " ")
}

// normalizedParagraphs trả về cùng giá trị FE so sánh (textContent + \s+ → " ").
func normalizedParagraphs(markdown string) []string {
	raw := Paragraphs(markdown)
	out := make([]string, len(raw))
	for i, p := range raw {
		out[i] = normalizeText(p)
	}
	return out
}

// anchorOutcome phân loại kết quả giải mã vị trí sau một lần ghi chương.
type anchorOutcome int

const (
	outcomeNone  anchorOutcome = iota // không đoạn nào đủ giống → Mất vị trí
	outcomeExact                      // khớp nguyên văn duy nhất → dời bình thường
	outcomeFuzzy                      // khớp gần đúng duy nhất → dời + cờ moved
	outcomeMany                       // nhiều đoạn gần đúng → Không chắc vị trí
)

type resolution struct {
	outcome    anchorOutcome
	paragraph  int
	anchor     *Anchor
	candidates []Candidate
}

// resolveAnchor giải vị trí của một trích dẫn trên văn bản mới.
// hint: chỉ số đoạn gốc (có thể -1); old: ngữ cảnh đã lưu (có thể nil khi
// dữ liệu cũ chưa có anchor — bản đọc-trước-không-sửa không suy đoán ngữ cảnh).
func resolveAnchor(paras []string, quote string, hint int, old *Anchor) resolution {
	quote = normalizeText(quote)
	if quote == "" || len(paras) == 0 {
		return resolution{outcome: outcomeNone, paragraph: -1}
	}
	var prefix, suffix string
	if old != nil {
		prefix, suffix = old.Prefix, old.Suffix
	}
	if old != nil && normalizeText(old.Quote) != "" && normalizeText(old.Quote) != quote {
		// Anchor đã theo bản gần đúng: ưu tiên truy tiếp câu đang hiển thị,
		// trích gốc chỉ còn vai trò dự phòng.
		tracked := resolveAnchor(paras, old.Quote, hint, old)
		if tracked.outcome == outcomeExact {
			return tracked
		}
	}
	type hit struct {
		para, at int
	}
	var hits []hit
	qr := []rune(quote)
	for i, para := range paras {
		pr := []rune(para)
		for at := 0; at+len(qr) <= len(pr); {
			j := indexRunes(pr[at:], qr)
			if j < 0 {
				break
			}
			hits = append(hits, hit{para: i, at: at + j})
			at += j + len(qr)
		}
	}
	if len(hits) == 1 {
		h := hits[0]
		return resolution{outcome: outcomeExact, paragraph: h.para, anchor: buildAnchor(paras, h.para, h.at, len(qr))}
	}
	if len(hits) > 1 {
		// Thử phân biệt bằng ngữ cảnh đã lưu và gợi ý đoạn gốc.
		type scored struct {
			hit
			score float64
		}
		var ranked []scored
		for _, h := range hits {
			before, after := contextAround(paras, h.para, h.at, len(qr))
			score := (trigramDice(prefix, before) + trigramDice(suffix, after)) / 2
			if h.para == hint {
				score += 0.1
			}
			ranked = append(ranked, scored{hit: h, score: score})
		}
		sort.Slice(ranked, func(a, b int) bool { return ranked[a].score > ranked[b].score })
		if ranked[0].score-ranked[1].score >= 0.15 {
			h := ranked[0]
			return resolution{outcome: outcomeExact, paragraph: h.para, anchor: buildAnchor(paras, h.para, h.at, len(qr))}
		}
		cands := make([]Candidate, 0, anchorCandidatesMax)
		seenPara := map[int]bool{}
		for _, r := range ranked {
			if seenPara[r.para] || len(cands) == anchorCandidatesMax {
				continue
			}
			seenPara[r.para] = true
			cands = append(cands, Candidate{Paragraph: r.para, Snippet: snippetAround(paras, r.para, r.at, len(qr))})
		}
		return resolution{outcome: outcomeMany, paragraph: -1, candidates: cands}
	}
	// Không còn nguyên văn: tìm đoạn gần giống nhất.
	type fuzzy struct {
		para     int
		score    float64
		windowAt int
		window   string
	}
	var found []fuzzy
	for i, para := range paras {
		best := fuzzy{para: i}
		paraRunes := []rune(para)
		if len(paraRunes) < len(qr) {
			if score := trigramDice(quote, para); score > best.score {
				best.score, best.window = score, para
			}
		} else {
			for start := 0; start+len(qr) <= len(paraRunes); start += 3 {
				window := string(paraRunes[start : start+len(qr)])
				if score := trigramDice(quote, window); score > best.score {
					best.score, best.window, best.windowAt = score, window, start
				}
			}
		}
		if best.score >= anchorFuzzyFloor {
			found = append(found, best)
		}
	}
	if len(found) == 0 {
		return resolution{outcome: outcomeNone, paragraph: -1}
	}
	// Ngữ cảnh đã lưu phân biệt các đoạn gần giống nhau.
	for i := range found {
		before, after := contextAround(paras, found[i].para, found[i].windowAt, utf8.RuneCountInString(found[i].window))
		found[i].score += 0.15 * (trigramDice(prefix, before) + trigramDice(suffix, after)) / 2
	}
	sort.Slice(found, func(a, b int) bool { return found[a].score > found[b].score })
	if len(found) == 1 && found[0].score >= anchorFuzzyFloor {
		f := found[0]
		return resolution{
			outcome: outcomeFuzzy, paragraph: f.para,
			anchor: buildAnchor(paras, f.para, f.windowAt, len([]rune(f.window))),
		}
	}
	cands := make([]Candidate, 0, anchorCandidatesMax)
	for k := 0; k < len(found) && k < anchorCandidatesMax; k++ {
		cands = append(cands, Candidate{Paragraph: found[k].para, Snippet: snippetAround(paras, found[k].para, found[k].windowAt, len([]rune(found[k].window)))})
	}
	return resolution{outcome: outcomeMany, paragraph: -1, candidates: cands}
}

// indexRunes tìm needle trong haystack theo chỉ số rune (strings.Index tính byte).
func indexRunes(haystack, needle []rune) int {
outer:
	for i := 0; i+len(needle) <= len(haystack); i++ {
		for j := range needle {
			if haystack[i+j] != needle[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// buildAnchor lưu trích đoạn đang khớp kèm ngữ cảnh trước/sau (mượn qua ranh
// giới đoạn khi trích nằm sát đầu/cuối, để nhận diện được bản sao trùng lặp).
func buildAnchor(paras []string, para, atRune, quoteLenRune int) *Anchor {
	if q := []rune(paras[para]); atRune+quoteLenRune <= len(q) {
		return &Anchor{
			Paragraph: para,
			Quote:     string(q[atRune : atRune+quoteLenRune]),
			Prefix:    contextBefore(paras, para, atRune),
			Suffix:    contextAfter(paras, para, atRune+quoteLenRune),
		}
	}
	return &Anchor{Paragraph: para, Quote: paras[para], Prefix: contextBefore(paras, para, 0), Suffix: contextAfter(paras, para, len([]rune(paras[para])))}
}

func contextBefore(paras []string, para, atRune int) string {
	runes := []rune(paras[para])
	start := atRune - anchorContextLen
	if start < 0 {
		start = 0
	}
	out := string(runes[start:atRune])
	// Mượn đuôi đoạn trước khi ngữ cảnh trong đoạn quá ngắn.
	for i := para - 1; i >= 0 && utf8.RuneCountInString(out) < anchorContextLen; i-- {
		prev := paras[i]
		need := anchorContextLen - utf8.RuneCountInString(out)
		pr := []rune(prev)
		if len(pr) > need {
			pr = pr[len(pr)-need:]
		}
		out = string(pr) + " " + out
	}
	return strings.TrimSpace(out)
}

func contextAfter(paras []string, para, atRune int) string {
	runes := []rune(paras[para])
	end := atRune + anchorContextLen
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[atRune:end])
	for i := para + 1; i < len(paras) && utf8.RuneCountInString(out) < anchorContextLen; i++ {
		next := paras[i]
		need := anchorContextLen - utf8.RuneCountInString(out)
		nr := []rune(next)
		if len(nr) > need {
			nr = nr[:need]
		}
		out += " " + string(nr)
	}
	return strings.TrimSpace(out)
}

func contextAround(paras []string, para, atRune, lenRune int) (string, string) {
	return contextBefore(paras, para, atRune), contextAfter(paras, para, atRune+lenRune)
}

// snippetAround lấy đoạn hiển thị quanh một khớp, cắt bớt với dấu "…" khi dài.
func snippetAround(paras []string, para, atRune, lenRune int) string {
	const radius = 90
	runes := []rune(paras[para])
	start := atRune - radius
	if start < 0 {
		start = 0
	}
	end := atRune + lenRune + radius
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return strings.TrimSpace(out)
}

// trigramDice đo độ giống hai chuỗi bằng hệ số Dice trên tập trigram rune.
// Chuỗi < 3 rune chỉ khớp khi bằng hệt.
func trigramDice(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) < 3 || len(br) < 3 {
		return 0
	}
	set := make(map[[3]rune]struct{}, len(ar))
	for i := 0; i+3 <= len(ar); i++ {
		set[[3]rune{ar[i], ar[i+1], ar[i+2]}] = struct{}{}
	}
	shared := 0
	seen := make(map[[3]rune]struct{}, len(br))
	for i := 0; i+3 <= len(br); i++ {
		g := [3]rune{br[i], br[i+1], br[i+2]}
		if _, dup := seen[g]; dup {
			continue
		}
		seen[g] = struct{}{}
		if _, ok := set[g]; ok {
			shared++
		}
	}
	return 2 * float64(shared) / float64(len(set)+len(seen))
}
