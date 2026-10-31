// Manifest phụ đề của job clone: schema captions.json, trộn đoạn ASR vào cue
// nguồn, và đường sửa của chủ nhân (applyCaptionEdits). Chuyển 1:1 từ
// yt-factory src/server/clone/transcript.ts (2c201e6).
package transcript

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// CloneDefaultFPS là fps mặc định của pipeline (CLONE_DEFAULT_FPS).
const CloneDefaultFPS = 30.0

const (
	// ASRDupWindowMs: text cue/ASR gần hơn mốc này được so trùng.
	ASRDupWindowMs = 800.0
	// ASRDupTokenRatio: overlap token chuẩn hoá đánh dấu đoạn ASR trùng.
	ASRDupTokenRatio = 0.6
	// ASRLowConfidence: dưới avg_logprob này cue ASR vào danh sách review.
	ASRLowConfidence = -1.0
	// UntimedCueAsrMs là lookahead qua start của cue không timing khi vùng nó
	// được phiên âm.
	UntimedCueAsrMs = 20_000.0
	// UntimedCueBackoffMs lùi nhẹ trước start của cue không timing.
	UntimedCueBackoffMs = 400.0
)

// ReviewItem là một mục cần chủ nhân duyệt.
type ReviewItem struct {
	CueID  string `json:"cueId"`
	Reason string `json:"reason"`
}

// CaptionTrack mô tả track phụ đề nguồn đã parse.
type CaptionTrack struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// AsrDetected là ngôn ngữ ASR dò được.
type AsrDetected struct {
	Language    string  `json:"language"`
	Probability float64 `json:"probability"`
}

// AsrInfo là bằng chứng một lượt ASR gap-fill trong manifest.
type AsrInfo struct {
	Model              *AsrModel    `json:"model"`
	Detected           *AsrDetected `json:"detected"`
	SegmentsAdded      int64        `json:"segmentsAdded"`
	SegmentsRejected   int64        `json:"segmentsRejected"`
	RegionsTranscribed []Interval   `json:"regionsTranscribed"`
	PeakRSSBytes       float64      `json:"peakRssBytes"`
}

// CaptionSource ghi nguồn cue của manifest.
type CaptionSource struct {
	Track     *CaptionTrack `json:"track"`
	RawSHA256 *string       `json:"rawSha256"`
}

// CaptionManifest là captions.json — manifest chỉnh được của chủ nhân.
type CaptionManifest struct {
	Version         int64            `json:"version"`
	Revision        int64            `json:"revision"`
	Language        *string          `json:"language"`
	Cues            []Cue            `json:"cues"`
	SpeechIntervals []Interval       `json:"speechIntervals"`
	VadEvidence     *VadEvidence     `json:"vadEvidence"`
	Coverage        Coverage         `json:"coverage"`
	Source          CaptionSource    `json:"source"`
	Asr             *AsrInfo         `json:"asr"`
	UnresolvedGaps  []Interval       `json:"unresolvedGaps"`
	NeedsReview     []ReviewItem     `json:"needsReview"`
	Warnings        []CaptionWarning `json:"warnings"`
	GeneratedAt     string           `json:"generatedAt"`
}

// CaptionEdit là một sửa cue của chủ nhân (đường edit API).
type CaptionEdit struct {
	CueID   string   `json:"cueId"`
	Text    *string  `json:"text,omitempty"`
	StartMs *float64 `json:"startMs,omitempty"`
	EndMs   *float64 `json:"endMs,omitempty"`
}

// normalizeText chuẩn hoá text mức từ cho so trùng nguồn/ASR.
func normalizeText(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func tokenOverlapRatio(a, b string) float64 {
	ta := map[string]bool{}
	for _, t := range strings.Fields(normalizeText(a)) {
		ta[t] = true
	}
	tb := map[string]bool{}
	for _, t := range strings.Fields(normalizeText(b)) {
		tb[t] = true
	}
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	shared := 0
	for t := range ta {
		if tb[t] {
			shared++
		}
	}
	smaller := len(ta)
	if len(tb) < smaller {
		smaller = len(tb)
	}
	return float64(shared) / float64(smaller)
}

// MergeResult là kết quả trộn đoạn ASR vào cue hiện có.
type MergeResult struct {
	Cues        []Cue
	Added       int64
	Rejected    int64
	NeedsReview []ReviewItem
}

// MergeAsrSegments trộn đoạn ASR vào bộ cue nguồn. Một đoạn ASR bị bỏ khi cue
// nguồn cùng mốc đã nói cùng điều (overlap token ≥0.6 hoặc chứa trọn text);
// đoạn sống sót thành cue `source: "asr"`. Khe không có text ASR vẫn là khe —
// không bao giờ đệm text giả.
func MergeAsrSegments(cues []Cue, segments []AsrSegment, sourceDurationMs float64) MergeResult {
	needsReview := []ReviewItem{}
	added := []Cue{}
	rejected := int64(0)
	timed := []Cue{}
	for _, c := range cues {
		if c.EndMs != nil {
			timed = append(timed, c)
		}
	}
	for _, seg := range segments {
		startMs := math.Max(0, math.Min(seg.StartMs, sourceDurationMs))
		endMs := math.Max(startMs, math.Min(seg.EndMs, sourceDurationMs))
		overlapping := []Cue{}
		for _, c := range timed {
			if c.StartMs < endMs+ASRDupWindowMs && *c.EndMs > startMs-ASRDupWindowMs {
				overlapping = append(overlapping, c)
			}
		}
		dup := false
		for _, c := range overlapping {
			if tokenOverlapRatio(c.Text, seg.Text) >= ASRDupTokenRatio ||
				strings.Contains(normalizeText(c.Text), normalizeText(seg.Text)) {
				dup = true
				break
			}
		}
		if dup {
			rejected++
			continue
		}
		var words []Word
		if len(seg.Words) > 0 {
			words = make([]Word, 0, len(seg.Words))
			for _, w := range seg.Words {
				words = append(words, Word{
					StartMs: math.Max(startMs, w.StartMs),
					EndMs:   math.Min(endMs, math.Max(w.EndMs, w.StartMs)),
					Text:    w.Text,
				})
			}
		}
		cue := Cue{
			ID:      fmt.Sprintf("asr-%g-%d", startMs, len(added)),
			StartMs: startMs, EndMs: &endMs,
			Text: seg.Text, Words: words, Source: "asr",
		}
		added = append(added, cue)
		if seg.AvgLogprob != nil && *seg.AvgLogprob < ASRLowConfidence {
			needsReview = append(needsReview, ReviewItem{
				CueID: cue.ID, Reason: fmt.Sprintf("low ASR confidence (%.2f)", *seg.AvgLogprob)})
		}
	}
	all := append(append([]Cue(nil), cues...), added...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].StartMs < all[j].StartMs })
	return MergeResult{Cues: all, Added: int64(len(added)), Rejected: rejected, NeedsReview: needsReview}
}

// ApplyCaptionEdits áp một loạt sửa text/timing lên bản sao cue, sắp xếp lại,
// tăng revision. Cue bị sửa thành `source: "edited"`; các bước sau mất hiệu
// lực theo revision mà runner fingerprint. Manifest vào không bị mutate.
func ApplyCaptionEdits(manifest CaptionManifest, edits []CaptionEdit, nowMs int64) CaptionManifest {
	cloned := make([]Cue, len(manifest.Cues))
	for i, c := range manifest.Cues {
		cloned[i] = c
		if c.Words != nil {
			cloned[i].Words = append([]Word(nil), c.Words...)
		}
	}
	byID := map[string]*Cue{}
	for i := range cloned {
		byID[cloned[i].ID] = &cloned[i]
	}
	editedIDs := map[string]bool{}
	for _, edit := range edits {
		cue, ok := byID[edit.CueID]
		if !ok {
			continue
		}
		previousStart := cue.StartMs
		if edit.Text != nil && *edit.Text != cue.Text {
			cue.Text = *edit.Text
			cue.Words = nil
		}
		if edit.StartMs != nil {
			cue.StartMs = math.Max(0, *edit.StartMs)
		}
		if edit.EndMs != nil {
			cue.EndMs = new(math.Max(cue.StartMs, *edit.EndMs))
		}
		if cue.Words != nil && cue.StartMs != previousStart {
			delta := cue.StartMs - previousStart
			for i := range cue.Words {
				cue.Words[i].StartMs += delta
				cue.Words[i].EndMs += delta
			}
		}
		cue.Source = "edited"
		editedIDs[cue.ID] = true
	}
	cues := cloned
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].StartMs < cues[j].StartMs })
	spans := []Interval{}
	for _, c := range cues {
		if iv := CueInterval(c); iv != nil {
			spans = append(spans, *iv)
		}
	}
	coverage := FrameCoverage(manifest.SpeechIntervals, spans, CloneDefaultFPS)
	coveredMerged := MergeIntervals(spans, 0)
	unresolved := []Interval{}
	for _, iv := range manifest.SpeechIntervals {
		cursor := iv.StartMs
		for _, c := range coveredMerged {
			if c.EndMs <= cursor {
				continue
			}
			if c.StartMs >= iv.EndMs {
				break
			}
			if c.StartMs > cursor {
				unresolved = append(unresolved, Interval{cursor, c.StartMs})
			}
			cursor = math.Max(cursor, c.EndMs)
		}
		if cursor < iv.EndMs {
			unresolved = append(unresolved, Interval{cursor, iv.EndMs})
		}
	}
	filtered := unresolved[:0]
	for _, g := range unresolved {
		if g.EndMs-g.StartMs >= 40 {
			filtered = append(filtered, g)
		}
	}
	needsReview := []ReviewItem{}
	for _, r := range manifest.NeedsReview {
		if !editedIDs[r.CueID] {
			needsReview = append(needsReview, r)
		}
	}
	for _, c := range cues {
		if editedIDs[c.ID] && c.EndMs == nil {
			needsReview = append(needsReview, ReviewItem{CueID: c.ID, Reason: "edited cue still lacks end timing"})
		}
	}
	next := manifest
	next.Revision = manifest.Revision + 1
	next.Cues = cues
	next.Coverage = coverage
	next.UnresolvedGaps = filtered
	next.NeedsReview = needsReview
	next.GeneratedAt = epochISO(nowMs)
	return next
}

func epochISO(ms int64) string {
	// Bản TS dùng new Date(ms).toISOString() — ISO-8601 UTC mili-giây.
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}
