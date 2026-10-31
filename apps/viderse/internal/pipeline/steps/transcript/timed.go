// Phần thuần của bước phụ đề: parse json3 có timing, toán khoảng và khung
// hình, emit SRT/VTT. Chuyển 1:1 từ yt-factory src/server/clone/timed-captions.ts
// (2c201e6). Không I/O.
package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Word là một mảnh từ có timing: text utf8 nguyên văn cùng đoạn đo được.
type Word struct {
	StartMs float64 `json:"startMs"`
	EndMs   float64 `json:"endMs"`
	Text    string  `json:"text"`
	// Estimated đúng khi mảnh gồm nhiều từ — timing dùng chung, không phải từng từ.
	EstimatedFlag *bool    `json:"estimated,omitempty"`
	TimingSource  string   `json:"timingSource,omitempty"` // "aligned" | "interpolated"
	Probability   *float64 `json:"probability,omitempty"`
}

// Estimated báo timing của word là ước lượng (nội suy) chứ không đo được.
func (w Word) Estimated() bool {
	return w.EstimatedFlag != nil && *w.EstimatedFlag
}

// Cue là một dòng phụ đề có mốc thời gian trên trục ms của nguồn.
type Cue struct {
	ID      string  `json:"id"`
	StartMs float64 `json:"startMs"`
	// EndMs nil khi nguồn không mang duration — review/ASR phải điền.
	EndMs  *float64 `json:"endMs"`
	Text   string   `json:"text"`
	Words  []Word   `json:"words"`
	Source string   `json:"source"` // "captions" | "asr" | "edited"
}

// CaptionWarning là một cảnh báo trong manifest phụ đề.
type CaptionWarning struct {
	Code   string `json:"code"`
	CueID  string `json:"cueId,omitempty"`
	Detail string `json:"detail"`
}

// ParsedCaptions là kết quả parse một tài liệu json3 thô.
type ParsedCaptions struct {
	Cues      []Cue
	Warnings  []CaptionWarning
	RawSHA256 string
}

func clean(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

type json3Seg struct {
	UTF8      *string  `json:"utf8"`
	TOffsetMs *float64 `json:"tOffsetMs"`
}

type json3Event struct {
	TStartMs    *float64   `json:"tStartMs"`
	DDurationMs *float64   `json:"dDurationMs"`
	AAppend     *float64   `json:"aAppend"`
	Segs        []json3Seg `json:"segs"`
}

// EventWords trả các mảnh từ của một event: mỗi seg có tOffsetMs đo được thành
// một mảnh kết thúc khi mảnh kế bắt đầu (hoặc tại cuối event); mảnh không có
// timing không có entry — không bao giờ nội suy. Mảnh ôm nhiều từ bị gắn cờ
// estimated.
func EventWords(e json3Event) []Word {
	start := 0.0
	if e.TStartMs != nil {
		start = *e.TStartMs
	}
	segs := make([]json3Seg, 0, len(e.Segs))
	for _, s := range e.Segs {
		if s.UTF8 != nil && strings.TrimSpace(*s.UTF8) != "" {
			segs = append(segs, s)
		}
	}
	hasMissing := false
	for _, s := range segs {
		if s.TOffsetMs == nil {
			hasMissing = true
		}
	}
	var eventEnd *float64
	if e.DDurationMs != nil {
		eventEnd = new(start + *e.DDurationMs)
	}
	if len(segs) == 0 || hasMissing {
		text := ""
		for _, s := range segs {
			if s.UTF8 != nil {
				text += *s.UTF8
			}
		}
		text = clean(text)
		if text != "" && eventEnd != nil {
			return []Word{{StartMs: start, EndMs: *eventEnd, Text: text, EstimatedFlag: new(true)}}
		}
		return nil
	}
	words := []Word{}
	for i, seg := range segs {
		segStart := start
		if seg.TOffsetMs != nil {
			segStart += *seg.TOffsetMs
		}
		var segEnd float64
		if i+1 < len(segs) && segs[i+1].TOffsetMs != nil {
			segEnd = start + *segs[i+1].TOffsetMs
		} else if eventEnd != nil {
			segEnd = *eventEnd
		} else {
			segEnd = segStart
		}
		text := ""
		if seg.UTF8 != nil {
			text = *seg.UTF8
		}
		// whitespace collapse giữ nguyên các khoảng trắng đơn lẻ
		text = collapseSpace(text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		var est *bool
		if len(strings.Fields(text)) > 1 {
			est = new(true)
		}
		words = append(words, Word{
			StartMs: segStart, EndMs: math.Max(segEnd, segStart), Text: text, EstimatedFlag: est,
		})
	}
	return words
}

// collapseSpace gộp whitespace lặp thành một khoảng trắng, giữ các khoảng trắng
// đơn ở đầu/cuối (bản TS replace(/\s+/g," ") rồi giữ space đơn).
func collapseSpace(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// CueID là id ổn định từ nội dung+thời điểm (deterministic qua các lần parse).
func CueID(startMs float64, text string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v:%s", startMs, text)))
	return fmt.Sprintf("c%v-%s", startMs, hex.EncodeToString(sum[:])[:10])
}

// ParseTimedJson3 parse tài liệu json3 thô của YouTube thành cue đã khử trùng.
//
// Trộn rolling-track: YouTube gửi lại (các) dòng trước trong cửa sổ kế khi lời
// thoại còn chạy. Khi text gộp của một event là tiền tố của text gộp của event
// sau trên cùng dòng đang lớn dần, hoặc event sau mở trong lúc event trước còn
// hiển thị và mở lại bằng text của event trước, cả hai mô tả cùng một cụm —
// giữ start sớm nhất, end muộn nhất và text dài nhất.
func ParseTimedJson3(raw []byte, sourceDurationMs float64) (ParsedCaptions, error) {
	warnings := []CaptionWarning{}
	var data struct {
		Events []json3Event `json:"events"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ParsedCaptions{}, fmt.Errorf("json3 không hợp lệ: %w", err)
	}
	sum := sha256.Sum256(raw)
	rawSHA := hex.EncodeToString(sum[:])

	type working struct {
		startMs    float64
		endMs      *float64
		text       string
		words      []Word
		eventEndMs *float64
	}
	merged := []working{}
	for _, event := range data.Events {
		if event.TStartMs == nil {
			continue
		}
		text := ""
		for _, s := range event.Segs {
			if s.UTF8 != nil {
				text += *s.UTF8
			}
		}
		text = clean(text)
		if text == "" {
			continue // marker aAppend newline, event đầu rỗng
		}
		startMs := math.Max(0, *event.TStartMs)
		var eventEnd *float64
		if event.DDurationMs != nil {
			eventEnd = new(startMs + *event.DDurationMs)
		}
		words := EventWords(event)

		if n := len(merged); n > 0 {
			last := &merged[n-1]
			if last.eventEndMs != nil && startMs <= *last.eventEndMs+200 && IsRollingContinuation(last.text, text) {
				if eventEnd != nil && (last.eventEndMs == nil || *eventEnd > *last.eventEndMs) {
					last.eventEndMs = eventEnd
					last.endMs = eventEnd
				}
				if len(text) > len(last.text) {
					last.text = text
					if len(words) > 0 {
						last.words = mergeWordLists(last.words, words)
					}
				}
				continue
			}
		}
		merged = append(merged, working{
			startMs: startMs, endMs: eventEnd, eventEndMs: eventEnd, text: text, words: words,
		})
	}

	// Kẹp đơn điệu: cue không được bắt đầu trước start của cue trước, cũng
	// không được kết thúc trước start của chính nó; mỗi end cắt theo start của
	// cue kế để danh sách lưu không chồng nhau, và end kẹp theo duration đã dò.
	prevStart := -1.0
	cues := make([]Cue, 0, len(merged))
	for i, w := range merged {
		startMs, endMs := w.startMs, w.endMs
		if startMs < prevStart {
			warnings = append(warnings, CaptionWarning{
				Code: "non_monotonic_start", Detail: fmt.Sprintf("cue %d start %v < previous %v", i, startMs, prevStart)})
			startMs = prevStart
		}
		if endMs != nil {
			var nextStart *float64
			if i+1 < len(merged) {
				nextStart = new(math.Max(merged[i+1].startMs, startMs))
			}
			clipped := nextStart != nil && *endMs > *nextStart
			if clipped {
				warnings = append(warnings, CaptionWarning{
					Code: "clipped_end_to_next", Detail: fmt.Sprintf("cue %d end %v > next cue start %v", i, *endMs, *nextStart)})
				endMs = nextStart
			}
			if *endMs <= startMs {
				if !clipped {
					warnings = append(warnings, CaptionWarning{
						Code: "clamped_end", Detail: fmt.Sprintf("cue %d end %v <= start %v", i, *endMs, startMs)})
				}
				endMs = new(startMs)
			}
			if sourceDurationMs > 0 && *endMs > sourceDurationMs {
				warnings = append(warnings, CaptionWarning{
					Code: "clamped_to_duration", Detail: fmt.Sprintf("cue %d end %v > duration %v", i, *endMs, sourceDurationMs)})
				endMs = new(sourceDurationMs)
			}
		}
		prevStart = startMs
		cues = append(cues, Cue{
			ID: CueID(startMs, w.text), StartMs: startMs, EndMs: endMs,
			Text: w.text, Words: w.words, Source: "captions",
		})
	}

	return ParsedCaptions{Cues: cues, Warnings: warnings, RawSHA256: rawSHA}, nil
}

// IsRollingContinuation: prefix regrowth cần ít nhất hai token chung, không bao
// giờ một từ lặp lại đơn lẻ.
func IsRollingContinuation(prev, next string) bool {
	a := strings.Fields(strings.TrimSpace(prev))
	b := strings.Fields(strings.TrimSpace(next))
	overlap := len(a)
	if len(b) < overlap {
		overlap = len(b)
	}
	if overlap < 2 {
		return false
	}
	for i := 0; i < overlap; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mergeWordLists: khi text khác nhau, giữ danh sách dài hơn.
func mergeWordLists(prev, next []Word) []Word {
	if prev == nil {
		return next
	}
	if next == nil {
		return prev
	}
	if len(next) >= len(prev) {
		return next
	}
	return prev
}

/* --------------------------- toán khoảng --------------------------- */

// Interval là một đoạn [start,end) trên trục ms.
type Interval struct {
	StartMs float64 `json:"startMs"`
	EndMs   float64 `json:"endMs"`
}

// MergeIntervals sắp xếp + trộn các đoạn [start,end); khe dưới mergeGapMs gộp lại.
func MergeIntervals(intervals []Interval, mergeGapMs float64) []Interval {
	sorted := append([]Interval(nil), intervals...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartMs != sorted[j].StartMs {
			return sorted[i].StartMs < sorted[j].StartMs
		}
		return sorted[i].EndMs < sorted[j].EndMs
	})
	out := []Interval{}
	for _, iv := range sorted {
		if n := len(out); n > 0 && iv.StartMs-out[n-1].EndMs <= mergeGapMs {
			out[n-1].EndMs = math.Max(out[n-1].EndMs, iv.EndMs)
		} else {
			out = append(out, iv)
		}
	}
	return out
}

// IntervalGaps trả phần của `intervals` KHÔNG bị `cover` phủ, trên cùng trục ms.
func IntervalGaps(intervals, cover []Interval) []Interval {
	ivs := MergeIntervals(intervals, 0)
	cv := MergeIntervals(cover, 0)
	gaps := []Interval{}
	ci := 0
	for _, iv := range ivs {
		cursor := iv.StartMs
		for ci < len(cv) && cv[ci].EndMs <= cursor {
			ci++
		}
		j := ci
		for j < len(cv) && cv[j].StartMs < iv.EndMs {
			if cv[j].StartMs > cursor {
				gaps = append(gaps, Interval{cursor, cv[j].StartMs})
			}
			cursor = math.Max(cursor, cv[j].EndMs)
			if cursor >= iv.EndMs {
				break
			}
			j++
		}
		if cursor < iv.EndMs {
			gaps = append(gaps, Interval{cursor, iv.EndMs})
		}
	}
	return gaps
}

// Coverage là mức phủ khung hình của phụ đề trên lời thoại.
type Coverage struct {
	FPS           float64 `json:"fps"`
	SpeechFrames  int64   `json:"speechFrames"`
	CoveredFrames int64   `json:"coveredFrames"`
	Ratio         float64 `json:"ratio"`
}

// FrameCoverage tính độ phủ khung hình của covered lên speech ở fps — không
// làm tròn kết quả: một khung được đếm khi nhịp [f/fps,(f+1)/fps) của nó chạm
// một khoảng covered; coverage = khung khớp / khung speech chính xác.
func FrameCoverage(speech, covered []Interval, fps float64) Coverage {
	sp := MergeIntervals(speech, 0)
	cv := MergeIntervals(covered, 0)
	var speechFrames, coveredFrames int64
	for _, iv := range sp {
		firstFrame := int64(math.Floor(iv.StartMs / 1000 * fps))
		lastFrame := int64(math.Ceil(iv.EndMs/1000*fps)) - 1
		for f := firstFrame; f <= lastFrame; f++ {
			fStart := float64(f) * 1000 / fps
			fEnd := float64(f+1) * 1000 / fps
			if fEnd <= iv.StartMs || fStart >= iv.EndMs {
				continue
			}
			speechFrames++
			for _, c := range cv {
				if c.StartMs < fEnd && c.EndMs > fStart {
					coveredFrames++
					break
				}
			}
		}
	}
	ratio := 1.0
	if speechFrames != 0 {
		ratio = float64(coveredFrames) / float64(speechFrames)
	}
	return Coverage{FPS: fps, SpeechFrames: speechFrames, CoveredFrames: coveredFrames, Ratio: ratio}
}

// CueInterval là đoạn có timing của một cue; nil khi cue không có end.
func CueInterval(cue Cue) *Interval {
	if cue.EndMs != nil && *cue.EndMs > cue.StartMs {
		return &Interval{cue.StartMs, *cue.EndMs}
	}
	return nil
}

/* --------------------------- emit SRT / VTT --------------------------- */

// SrtStamp định dạng ms thành dấu thời gian SRT (HH:MM:SS,mmm).
func SrtStamp(ms float64) string {
	ims := int64(math.Floor(ms))
	h := ims / 3_600_000
	m := (ims % 3_600_000) / 60_000
	s := (ims % 60_000) / 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ims%1000)
}

func vttStamp(ms float64) string {
	return strings.Replace(SrtStamp(ms), ",", ".", 1)
}

// CuesToSrt render chỉ những cue có end đo được; cue không timing không bao giờ
// nhận end giả.
func CuesToSrt(cues []Cue) string {
	timed := []Cue{}
	for _, c := range cues {
		if c.EndMs != nil {
			timed = append(timed, c)
		}
	}
	var b strings.Builder
	for i, c := range timed {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n", i+1, SrtStamp(c.StartMs), SrtStamp(*c.EndMs), c.Text)
	}
	return b.String()
}

// CuesToVtt render VTT cho các cue có end đo được.
func CuesToVtt(cues []Cue) string {
	timed := []Cue{}
	for _, c := range cues {
		if c.EndMs != nil {
			timed = append(timed, c)
		}
	}
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i, c := range timed {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s --> %s\n%s\n", vttStamp(c.StartMs), vttStamp(*c.EndMs), c.Text)
	}
	return b.String()
}
