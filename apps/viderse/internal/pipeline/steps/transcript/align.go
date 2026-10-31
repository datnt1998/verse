// Khớp timing cue phụ đề với các anchor ASR đo được. Chuyển 1:1 từ
// yt-factory src/server/clone/caption-align.ts (2c201e6) — global sequence
// alignment (Needleman–Wunsch); onset distance phân rẽ cụm lặp.
//
// Quy tắc không thương lượng: cue đã có word timing đo được giữ nguyên; chỉ
// cue estimate mới được sửa. Cue không khớp anchor nào giữ bounds như cũ —
// không fabricate. Độ lệch vẫn hiện trong bảng đo (median/P95).
package transcript

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const bandMs = 1500.0

// AnchorWord là một từ do ASR đo (speechAnchorWordSchema).
type AnchorWord struct {
	Text        string   `json:"text"`
	StartMs     float64  `json:"startMs"`
	EndMs       float64  `json:"endMs"`
	Probability *float64 `json:"probability,omitempty"`
}

// AnchorModel là bằng chứng model tạo anchor (asrAnchorsSchema.model).
type AnchorModel struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Size        string `json:"size"`
	ComputeType string `json:"computeType"`
	Device      string `json:"device"`
}

// AsrAnchors là gói anchor lưu trong asr-anchors.json (asrAnchorsSchema).
type AsrAnchors struct {
	Version           int64        `json:"version"`
	SourceAudioSHA256 string       `json:"sourceAudioSha256"`
	Spans             []Interval   `json:"spans"`
	Words             []AnchorWord `json:"words"`
	Model             AnchorModel  `json:"model"`
	GeneratedAt       string       `json:"generatedAt"`
}

// NormalizeCaptionToken: NFD → bỏ dấu → hạ chữ → bỏ mọi ký tự không phải
// chữ/số (JS: normalize("NFD").replace(/\p{M}/gu,"").toLowerCase().replace(/[^\p{L}\p{N}]/gu,"")).
func NormalizeCaptionToken(text string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)))
	stripped, _, _ := transform.String(t, strings.ToLower(text))
	var b strings.Builder
	for _, r := range stripped {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// similarity: 2 khi giống hệt; 1 khi Levenshtein ratio ≥0.8; 0 nếu không.
func similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 2
	}
	// JavaScript indexes strings by UTF-16 code unit, including surrogate pairs.
	ra, rb := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	previous := make([]int64, len(rb)+1)
	for j := range previous {
		previous[j] = int64(j)
	}
	for i := int64(1); i <= int64(len(ra)); i++ {
		row := []int64{i}
		for j := int64(1); j <= int64(len(rb)); j++ {
			cost := int64(1)
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			row = append(row, min3(row[j-1]+1, previous[j]+1, previous[j-1]+cost))
		}
		previous = row
	}
	longest := len(ra)
	if len(rb) > longest {
		longest = len(rb)
	}
	if 1-float64(previous[len(rb)])/float64(longest) >= 0.8 {
		return 1
	}
	return 0
}
func min3(a, b, c int64) int64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

type token struct {
	text    string
	startMs float64
	endMs   float64
}

// matchTokens là global sequence alignment; onset distance phân rẽ cụm lặp.
// Trả cặp [word, anchor] theo thứ tự tăng dần.
func matchTokens(words, anchors []token, wordBand bool) [][2]int {
	width := len(anchors) + 1
	size := (len(words) + 1) * width
	scores := make([]int32, size)
	distances := make([]float64, size)
	directions := make([]uint8, size)
	left := make([]string, len(words))
	for i, w := range words {
		left[i] = NormalizeCaptionToken(w.text)
	}
	right := make([]string, len(anchors))
	for j, a := range anchors {
		right[j] = NormalizeCaptionToken(a.text)
	}
	for i := 1; i <= len(words); i++ {
		scores[i*width] = int32(-i)
		directions[i*width] = 2
	}
	for j := 1; j <= len(anchors); j++ {
		scores[j] = int32(-j)
		directions[j] = 3
	}
	for i := 1; i <= len(words); i++ {
		for j := 1; j <= len(anchors); j++ {
			index := i*width + j
			up := index - width
			back := index - 1
			diag := up - 1
			delta := math.Abs(words[i-1].startMs - anchors[j-1].startMs)
			value := similarity(left[i-1], right[j-1])
			if wordBand && delta > bandMs {
				value = 0
			}
			scores[index] = scores[up] - 1
			distances[index] = distances[up]
			directions[index] = 2
			if scores[back]-1 > scores[index] ||
				(scores[back]-1 == scores[index] && distances[back] < distances[index]) {
				scores[index] = scores[back] - 1
				distances[index] = distances[back]
				directions[index] = 3
			}
			if value > 0 && (scores[diag]+int32(value) > scores[index] ||
				(scores[diag]+int32(value) == scores[index] && distances[diag]+delta <= distances[index])) {
				scores[index] = scores[diag] + int32(value)
				distances[index] = distances[diag] + delta
				directions[index] = 1
			}
		}
	}
	pairs := [][2]int{}
	i, j := len(words), len(anchors)
	for i > 0 || j > 0 {
		switch directions[i*width+j] {
		case 1:
			i--
			j--
			pairs = append(pairs, [2]int{i, j})
		case 2:
			i--
		default:
			j--
		}
	}
	for l, r := 0, len(pairs)-1; l < r; l, r = l+1, r-1 {
		pairs[l], pairs[r] = pairs[r], pairs[l]
	}
	return pairs
}

// anchorsLen giữ gọi len() nhất quán với bản TS.
type tokenSlice []token

func (a tokenSlice) length() int { return len(a) }

// splitWords tách cue thành token theo text: mỗi piece có timing được chia tỉ
// lệ theo độ dài ký tự của từ (không chia đều).
func splitWords(cue Cue) []Word {
	pieces := cue.Words
	if len(pieces) == 0 {
		end := cue.StartMs
		if cue.EndMs != nil {
			end = *cue.EndMs
		}
		pieces = []Word{{Text: cue.Text, StartMs: cue.StartMs, EndMs: end, EstimatedFlag: new(true)}}
	}
	out := []Word{}
	for _, piece := range pieces {
		tokens := []string{}
		for _, t := range strings.Fields(piece.Text) {
			tokens = append(tokens, t)
		}
		length := 0
		for _, t := range tokens {
			length += jsLength(t)
		}
		position := 0
		for _, text := range tokens {
			var startMs, endMs float64
			if length > 0 {
				startMs = piece.StartMs + (piece.EndMs-piece.StartMs)*float64(position)/float64(length)
				position += jsLength(text)
				endMs = piece.StartMs + (piece.EndMs-piece.StartMs)*float64(position)/float64(length)
			} else {
				startMs, endMs = piece.StartMs, piece.EndMs
			}
			out = append(out, Word{
				StartMs: startMs, EndMs: endMs, Text: text,
				EstimatedFlag: piece.EstimatedFlag, TimingSource: piece.TimingSource,
				Probability: piece.Probability,
			})
		}
	}
	return out
}

// CaptionAlignmentSpans: mỗi cue estimate mở một cửa sổ quanh khoảng của nó;
// thêm tối đa 10 cửa sổ 20s lấy mẫu trên các cue đã đo (anchor cần phủ cả vùng
// chứng cứ, không chỉ chỗ cần sửa).
func CaptionAlignmentSpans(cues []Cue, durationMs float64) []Interval {
	spans := []Interval{}
	for _, c := range cues {
		if len(c.Words) > 0 && !cueWordsEstimated(c) {
			continue
		}
		end := c.StartMs + 20_000
		if c.EndMs != nil {
			end = *c.EndMs
		}
		spans = append(spans, Interval{
			StartMs: math.Max(0, c.StartMs-300),
			EndMs:   math.Min(durationMs, end+300),
		})
	}
	measured := []Cue{}
	for _, c := range cues {
		if c.Source == "captions" && len(c.Words) > 0 && !cueWordsEstimated(c) {
			measured = append(measured, c)
		}
	}
	count := min(10, len(measured))
	for i := range count {
		var cue Cue
		if count == 1 {
			cue = measured[0]
		} else {
			idx := int(math.Round(float64(i) * float64(len(measured)-1) / float64(count-1)))
			cue = measured[idx]
		}
		startMs := math.Max(0, math.Min(cue.StartMs, durationMs-20_000))
		spans = append(spans, Interval{startMs, math.Min(durationMs, startMs+20_000)})
	}
	filtered := spans[:0]
	for _, s := range spans {
		if s.EndMs > s.StartMs {
			filtered = append(filtered, s)
		}
	}
	return MergeIntervals(filtered, 0)
}

func cueWordsEstimated(c Cue) bool {
	for _, w := range c.Words {
		if w.Estimated() {
			return true
		}
	}
	return false
}

// AlignCaptionWords chỉ sửa cue estimate. Timing YouTube lấy mẫu giữ làm bằng
// chứng độc lập. Một match đẩy start cue tới/qua start của cue kế thuộc speech
// kế tiếp chứ không phải cue này — cue giữ cửa sổ riêng với word nội suy thay
// vì sập về độ dài 0.
func AlignCaptionWords(cue Cue, anchors []AnchorWord, nextStartMs float64) Cue {
	if len(cue.Words) > 0 && !cueWordsEstimated(cue) {
		return cue
	}
	windowEnd := math.Min(nextStartMs, cue.StartMs+20_000)
	if cue.EndMs != nil {
		windowEnd = *cue.EndMs
	}
	bounded := cue
	bounded.EndMs = &windowEnd
	words := splitWords(bounded)
	interpolated := func() Cue {
		if cue.EndMs == nil {
			return cue
		}
		out := cue
		out.Words = make([]Word, len(words))
		for i, w := range words {
			out.Words[i] = w
			out.Words[i].EstimatedFlag = new(true)
			out.Words[i].TimingSource = "interpolated"
		}
		return out
	}
	candidates := []AnchorWord{}
	for _, w := range anchors {
		if w.StartMs >= cue.StartMs-bandMs && w.StartMs <= windowEnd+bandMs {
			candidates = append(candidates, w)
		}
	}
	wordTokens := make(tokenSlice, len(words))
	for i, w := range words {
		wordTokens[i] = token{w.Text, w.StartMs, w.EndMs}
	}
	anchorTokens := make(tokenSlice, len(candidates))
	for j, a := range candidates {
		anchorTokens[j] = token{a.Text, a.StartMs, a.EndMs}
	}
	pairs := matchTokens(wordTokens, anchorTokens, false)
	if len(pairs) == 0 {
		return interpolated()
	}
	matched := map[int]AnchorWord{}
	for _, p := range pairs {
		matched[p[0]] = candidates[p[1]]
	}
	startMs := math.Max(0, candidates[pairs[0][1]].StartMs-80)
	if nextStartMs > 0 && startMs >= nextStartMs {
		return interpolated()
	}
	endMs := math.Max(startMs, math.Min(nextStartMs, candidates[pairs[len(pairs)-1][1]].EndMs+150))
	if nextStartMs <= 0 {
		endMs = candidates[pairs[len(pairs)-1][1]].EndMs + 150
	}
	aligned := make([]Word, len(words))
	for i, word := range words {
		if anchor, ok := matched[i]; ok {
			aligned[i] = Word{
				Text: word.Text, StartMs: anchor.StartMs, EndMs: anchor.EndMs,
				TimingSource: "aligned", Probability: anchor.Probability,
			}
		} else {
			aligned[i] = word
			aligned[i].EstimatedFlag = new(true)
			aligned[i].TimingSource = "interpolated"
		}
	}
	for i := 0; i < len(aligned); {
		if _, ok := matched[i]; ok {
			i++
			continue
		}
		first := i
		for i < len(aligned) {
			if _, ok := matched[i]; ok {
				break
			}
			i++
		}
		left := startMs
		if first > 0 {
			left = aligned[first-1].EndMs
		}
		right := endMs
		if i < len(aligned) {
			right = aligned[i].StartMs
		}
		right = math.Max(left, right)
		chars := 0
		for _, w := range aligned[first:i] {
			chars += jsLength(w.Text)
		}
		if chars == 0 {
			continue
		}
		consumed := 0
		for j := first; j < i; j++ {
			aligned[j].StartMs = left + (right-left)*float64(consumed)/float64(chars)
			consumed += jsLength(aligned[j].Text)
			aligned[j].EndMs = left + (right-left)*float64(consumed)/float64(chars)
		}
	}
	out := cue
	out.StartMs = startMs
	out.EndMs = &endMs
	out.Words = aligned
	return out
}

// PageWord là một từ trên page đã render (đầu vào của phần đo căn).
type PageWord struct {
	Text    string  `json:"text"`
	StartMs float64 `json:"startMs"`
	EndMs   float64 `json:"endMs"`
	Frame   int64   `json:"frame"`
}

// DisplayedCaptionPage là một khung phụ đề như đã render.
type DisplayedCaptionPage struct {
	ItemID    string     `json:"itemId"`
	Estimated bool       `json:"estimated"`
	Words     []PageWord `json:"words"`
}

// AlignmentWorst là anchor lệch nhất của một page.
type AlignmentWorst struct {
	ItemID  string  `json:"itemId"`
	Frame   int64   `json:"frame"`
	DeltaMs float64 `json:"deltaMs"`
	Text    string  `json:"text"`
}

// AlignmentMetric là độ lệch căn word tổng hợp.
type AlignmentMetric struct {
	MatchedRatio float64          `json:"matchedRatio"`
	MedianMs     *float64         `json:"medianMs"`
	P95Ms        *float64         `json:"p95Ms"`
	KaraokeRatio float64          `json:"karaokeRatio"`
	Worst        []AlignmentWorst `json:"worst"`
}

// MeasureCaptionAlignment đo từ các word renderer đã resolve, không dùng timing
// tiền-editor của transcript.
func MeasureCaptionAlignment(pages []DisplayedCaptionPage, anchors AsrAnchors) AlignmentMetric {
	deltas := []float64{}
	worst := []AlignmentWorst{}
	used := map[int]bool{}
	total := 0
	for _, page := range pages {
		words := []PageWord{}
		for _, w := range page.Words {
			in := false
			for _, s := range anchors.Spans {
				if w.StartMs >= s.StartMs-bandMs && w.StartMs < s.EndMs+bandMs {
					in = true
					break
				}
			}
			if in {
				words = append(words, w)
			}
		}
		if len(words) == 0 {
			continue
		}
		total += len(words)
		candidates := []AnchorWord{}
		for i, a := range anchors.Words {
			if used[i] {
				continue
			}
			if a.StartMs >= words[0].StartMs-bandMs && a.StartMs <= words[len(words)-1].EndMs+bandMs {
				candidates = append(candidates, a)
			}
		}
		// matched anchor index trong anchors.Words để đánh used
		candIdx := []int{}
		for i, a := range anchors.Words {
			if used[i] {
				continue
			}
			if a.StartMs >= words[0].StartMs-bandMs && a.StartMs <= words[len(words)-1].EndMs+bandMs {
				candIdx = append(candIdx, i)
			}
		}
		wordToks := make(tokenSlice, len(words))
		for i, w := range words {
			wordToks[i] = token{w.Text, w.StartMs, w.EndMs}
		}
		candToks := make(tokenSlice, len(candidates))
		for j, a := range candidates {
			candToks[j] = token{a.Text, a.StartMs, a.EndMs}
		}
		var worstWord *struct {
			frame   int64
			deltaMs float64
		}
		for _, p := range matchTokens(wordToks, candToks, true) {
			word := words[p[0]]
			anchor := candidates[p[1]]
			used[candIdx[p[1]]] = true
			deltaMs := word.StartMs - anchor.StartMs
			deltas = append(deltas, math.Abs(deltaMs))
			if worstWord == nil || math.Abs(deltaMs) > math.Abs(worstWord.deltaMs) {
				worstWord = &struct {
					frame   int64
					deltaMs float64
				}{word.Frame, deltaMs}
			}
		}
		if worstWord != nil {
			text := ""
			for _, w := range page.Words {
				if text != "" {
					text += " "
				}
				text += w.Text
			}
			worst = append(worst, AlignmentWorst{ItemID: page.ItemID, Frame: worstWord.frame, DeltaMs: worstWord.deltaMs, Text: text})
		}
	}
	sort.Float64s(deltas)
	var median, p95 *float64
	if len(deltas) > 0 {
		mid := len(deltas) / 2
		v := deltas[mid]
		if len(deltas)%2 == 0 {
			v = (deltas[mid-1] + deltas[mid]) / 2
		}
		median = &v
		p := deltas[int(math.Ceil(float64(len(deltas))*0.95))-1]
		p95 = &p
	}
	var matchedRatio float64
	if total > 0 {
		matchedRatio = float64(len(deltas)) / float64(total)
	}
	var karaokeRatio float64
	if len(pages) > 0 {
		measured := 0
		for _, p := range pages {
			if !p.Estimated {
				measured++
			}
		}
		karaokeRatio = float64(measured) / float64(len(pages))
	}
	sort.Slice(worst, func(i, j int) bool {
		return math.Abs(worst[i].DeltaMs) > math.Abs(worst[j].DeltaMs)
	})
	if len(worst) > 8 {
		worst = worst[:8]
	}
	return AlignmentMetric{
		MatchedRatio: matchedRatio, MedianMs: median, P95Ms: p95,
		KaraokeRatio: karaokeRatio, Worst: worst,
	}
}

// JavaScript string.length counts UTF-16 code units, not UTF-8 bytes.
func jsLength(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}
