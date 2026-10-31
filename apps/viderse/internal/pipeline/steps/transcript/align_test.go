package transcript

// Chuyển ca test từ yt-factory src/server/clone/caption-align.test.ts (2c201e6).
// Ca "unmatched words … karaoke" gắn buildCaptionPages/effectiveCaptionPreset của
// editor engine (phase 06) — chỉ giữ phần measureCaptionAlignment.

import (
	"reflect"
	"testing"
)

func alignCue(text string, startMs, endMs float64) Cue {
	return Cue{ID: "cue", StartMs: startMs, EndMs: new(endMs), Text: text, Source: "captions"}
}

func anchors(text string, startMs float64) []AnchorWord {
	words := []AnchorWord{}
	for i, token := range splitWS(text) {
		start := startMs + float64(i)*300
		prob := 0.9
		words = append(words, AnchorWord{Text: token, StartMs: start, EndMs: start + 200, Probability: &prob})
	}
	return words
}

func splitWS(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func TestNormalizeCaptionToken(t *testing.T) {
	if got := NormalizeCaptionToken("Việt!"); got != "viet" {
		t.Fatalf("%q", got)
	}
	if got := NormalizeCaptionToken("2026,"); got != "2026" {
		t.Fatalf("%q", got)
	}
}

func TestAlignExactAccentNumericTokens(t *testing.T) {
	aligned := AlignCaptionWords(alignCue("Việt 2026 works", 1000, 4000), anchors("viet 2026 works", 1200), 0)
	if aligned.StartMs != 1120 || aligned.EndMs == nil || *aligned.EndMs != 2150 {
		t.Fatalf("bounds: %+v", aligned)
	}
	if len(aligned.Words) != 3 {
		t.Fatalf("words: %+v", aligned.Words)
	}
	for i, w := range aligned.Words {
		if w.StartMs != 1200+float64(i)*300 || w.Estimated() || w.TimingSource != "aligned" {
			t.Fatalf("word %d: %+v", i, w)
		}
	}
}

func TestAlignDroppedAndInsertedWords(t *testing.T) {
	aligned := AlignCaptionWords(alignCue("one missing two three", 1000, 4000), anchors("one extra two three", 1200), 0)
	if len(aligned.Words) != 4 {
		t.Fatalf("words: %+v", aligned.Words)
	}
	if !aligned.Words[1].Estimated() || aligned.Words[1].TimingSource != "interpolated" {
		t.Fatalf("word 1: %+v", aligned.Words[1])
	}
	if aligned.Words[2].StartMs != 1800 || aligned.Words[3].StartMs != 2100 {
		t.Fatalf("later words drifted: %+v", aligned.Words)
	}
}

func TestAlignNearSubstitutions(t *testing.T) {
	got := AlignCaptionWords(alignCue("words", 1000, 4000), anchors("word", 1200), 0)
	if got.Words[0].TimingSource != "aligned" {
		t.Fatalf("ratio 0.8 should align: %+v", got.Words[0])
	}
	got = AlignCaptionWords(alignCue("cat", 1000, 4000), anchors("dog", 1200), 0)
	if !got.Words[0].Estimated() {
		t.Fatalf("unrelated substitution must stay estimated: %+v", got.Words[0])
	}
}

func TestAlignRepeatedPhrasesChooseNearest(t *testing.T) {
	aligned := AlignCaptionWords(
		alignCue("hello there", 2000, 2700),
		append(anchors("hello there", 800), anchors("hello there", 2100)...),
		0,
	)
	if aligned.Words[0].StartMs != 2100 {
		t.Fatalf("chose far occurrence: %+v", aligned.Words)
	}
}

func TestAlignEmptyAnchorsPreserveBounds(t *testing.T) {
	original := alignCue("nothing measured", 1000, 4000)
	aligned := AlignCaptionWords(original, nil, 0)
	if aligned.StartMs != original.StartMs {
		t.Fatalf("bounds moved: %+v", aligned)
	}
	for _, w := range aligned.Words {
		if !w.Estimated() || w.TimingSource != "interpolated" {
			t.Fatalf("word not honest: %+v", w)
		}
	}
}

func TestAlignBoundsDoNotOverlapNextCue(t *testing.T) {
	aligned := AlignCaptionWords(alignCue("one two", 1000, 4000), anchors("one two", 1200), 1750)
	if aligned.EndMs == nil || *aligned.EndMs != 1750 {
		t.Fatalf("end: %+v", aligned.EndMs)
	}
}

func TestCaptionAlignmentSpans(t *testing.T) {
	estimates := []Cue{
		alignCue("one", 100, 900),
		alignCue("two", 1100, 1400),
	}
	spans := CaptionAlignmentSpans(estimates, 5000)
	if !reflect.DeepEqual(spans, []Interval{{0, 1700}}) {
		t.Fatalf("%+v", spans)
	}
	measured := make([]Cue, 20)
	for i := range measured {
		c := alignCue("word", float64(i)*30000, float64(i)*30000+1000)
		c.Words = []Word{{Text: "word", StartMs: float64(i) * 30000, EndMs: float64(i)*30000 + 200}}
		measured[i] = c
	}
	spans = CaptionAlignmentSpans(measured, 600000)
	if len(spans) != 10 {
		t.Fatalf("expected 10 sampled spans: %+v", spans)
	}
	for _, s := range spans {
		if s.EndMs-s.StartMs != 20000 {
			t.Fatalf("span: %+v", s)
		}
	}
}

func TestMeasureCaptionAlignment(t *testing.T) {
	speech := anchors("one two three", 1200)
	pages := []DisplayedCaptionPage{{
		ItemID:    "subtitle",
		Estimated: false,
		Words:     nil,
	}}
	for i, w := range speech {
		pages[0].Words = append(pages[0].Words, PageWord{
			Text: w.Text, StartMs: w.StartMs + 400, EndMs: w.EndMs + 400, Frame: int64(48 + i*9),
		})
	}
	metric := MeasureCaptionAlignment(pages, AsrAnchors{Words: speech, Spans: []Interval{{1000, 3000}}})
	if metric.MatchedRatio != 1 || *metric.MedianMs != 400 || *metric.P95Ms != 400 {
		t.Fatalf("%+v", metric)
	}
	if metric.KaraokeRatio != 1 {
		t.Fatalf("karaoke: %+v", metric)
	}
	if len(metric.Worst) == 0 || metric.Worst[0].ItemID != "subtitle" || metric.Worst[0].DeltaMs != 400 {
		t.Fatalf("worst: %+v", metric.Worst)
	}
}

func TestAlignUntimedCues(t *testing.T) {
	untimed := alignCue("one two", 1000, 4000)
	untimed.EndMs = nil
	if got := AlignCaptionWords(untimed, nil, 0); got.EndMs != nil {
		t.Fatalf("untimed cue got fabricated end: %+v", got)
	}
	aligned := AlignCaptionWords(untimed, anchors("one two", 1200), 0)
	if aligned.EndMs == nil || *aligned.EndMs != 1850 {
		t.Fatalf("end: %+v", aligned.EndMs)
	}
	for _, w := range aligned.Words {
		if w.TimingSource != "aligned" {
			t.Fatalf("word: %+v", w)
		}
	}
}

func TestMeasureMedianAndP95(t *testing.T) {
	speech := anchors("one two", 1200)
	page := DisplayedCaptionPage{ItemID: "subtitle"}
	for i, w := range speech {
		shift := 20.0
		if i != 0 {
			shift = 80
		}
		page.Words = append(page.Words, PageWord{
			Text: w.Text, StartMs: w.StartMs + shift, EndMs: w.EndMs + shift, Frame: int64(i * 10),
		})
	}
	metric := MeasureCaptionAlignment(
		[]DisplayedCaptionPage{page},
		AsrAnchors{Words: speech, Spans: []Interval{{1000, 3000}}},
	)
	if metric.MedianMs == nil || *metric.MedianMs != 50 {
		t.Fatalf("median: %+v", metric.MedianMs)
	}
	if metric.P95Ms == nil || *metric.P95Ms != 80 {
		t.Fatalf("p95: %+v", metric.P95Ms)
	}
}
