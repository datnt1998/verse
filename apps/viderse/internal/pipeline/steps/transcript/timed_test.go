package transcript

// Chuyển ca test từ yt-factory src/server/clone/timed-captions.test.ts (2c201e6).

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseTimedJson3KeepsWordTiming(t *testing.T) {
	raw := `{"events":[
		{"tStartMs":0,"dDurationMs":200},
		{"tStartMs":500,"dDurationMs":2000,"segs":[{"utf8":"Một ","tOffsetMs":0},{"utf8":"câu ","tOffsetMs":500},{"utf8":"chuyện","tOffsetMs":1000}]},
		{"tStartMs":3000,"segs":[{"utf8":"Không có duration"}]}
	]}`
	parsed, err := ParseTimedJson3([]byte(raw), 60000)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Cues) != 2 {
		t.Fatalf("expected 2 cues, got %d", len(parsed.Cues))
	}
	first := parsed.Cues[0]
	if first.StartMs != 500 || first.EndMs == nil || *first.EndMs != 2500 || first.Text != "Một câu chuyện" || first.Source != "captions" {
		t.Fatalf("cue 0: %+v", first)
	}
	wantWords := []Word{
		{StartMs: 500, EndMs: 1000, Text: "Một "},
		{StartMs: 1000, EndMs: 1500, Text: "câu "},
		{StartMs: 1500, EndMs: 2500, Text: "chuyện"},
	}
	if !reflect.DeepEqual(first.Words, wantWords) {
		t.Fatalf("words: %+v", first.Words)
	}
	if parsed.Cues[1].EndMs != nil || parsed.Cues[1].StartMs != 3000 {
		t.Fatalf("cue 1: %+v", parsed.Cues[1])
	}
	if len(parsed.Warnings) != 0 {
		t.Fatalf("warnings: %+v", parsed.Warnings)
	}
}

func TestParseTimedJson3MergesRollingWindowDuplicates(t *testing.T) {
	raw := `{"events":[
		{"tStartMs":1000,"dDurationMs":3000,"segs":[{"utf8":"Hello there"}]},
		{"tStartMs":1800,"dDurationMs":3200,"segs":[{"utf8":"Hello there"},{"utf8":" friend","tOffsetMs":500}]},
		{"tStartMs":2500,"aAppend":1,"segs":[{"utf8":"\n"}]},
		{"tStartMs":2600,"dDurationMs":2000,"segs":[{"utf8":"World"}]}
	]}`
	parsed, err := ParseTimedJson3([]byte(raw), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Cues) != 2 {
		t.Fatalf("expected 2 cues, got %v", parsed.Cues)
	}
	c0 := parsed.Cues[0]
	if c0.Text != "Hello there friend" || c0.StartMs != 1000 {
		t.Fatalf("cue 0: %+v", c0)
	}
	if c0.EndMs == nil || *c0.EndMs != 2600 {
		t.Fatalf("end should clip to next cue start: %+v", c0.EndMs)
	}
	if parsed.Cues[1].Text != "World" {
		t.Fatalf("cue 1: %+v", parsed.Cues[1])
	}
	found := false
	for _, w := range parsed.Warnings {
		if w.Code == "clipped_end_to_next" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clipped_end_to_next warning: %+v", parsed.Warnings)
	}
}

func TestParseTimedJson3ClampsEndsToSourceDuration(t *testing.T) {
	raw := `{"events":[{"tStartMs":1000,"dDurationMs":99000,"segs":[{"utf8":"runs past the end"}]}]}`
	parsed, err := ParseTimedJson3([]byte(raw), 5000)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Cues[0].EndMs == nil || *parsed.Cues[0].EndMs != 5000 {
		t.Fatalf("end: %+v", parsed.Cues[0].EndMs)
	}
	if len(parsed.Warnings) == 0 || parsed.Warnings[0].Code != "clamped_to_duration" {
		t.Fatalf("warnings: %+v", parsed.Warnings)
	}
}

func TestParseTimedJson3WarnsOnNonMonotonicStarts(t *testing.T) {
	raw := `{"events":[
		{"tStartMs":2000,"dDurationMs":500,"segs":[{"utf8":"b"}]},
		{"tStartMs":1000,"dDurationMs":500,"segs":[{"utf8":"a"}]}
	]}`
	parsed, err := ParseTimedJson3([]byte(raw), 0)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Cues[1].StartMs != 2000 {
		t.Fatalf("cue 1 start clamped: %+v", parsed.Cues[1])
	}
	found := false
	for _, w := range parsed.Warnings {
		if w.Code == "non_monotonic_start" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings: %+v", parsed.Warnings)
	}
}

func TestIsRollingContinuation(t *testing.T) {
	if !IsRollingContinuation("Hello there", "Hello there friend") {
		t.Fatal("prefix regrowth should match")
	}
	if !IsRollingContinuation("Hello there friend", "Hello there") {
		t.Fatal("shorter next should still match (same growing line)")
	}
	if IsRollingContinuation("Hello", "Hello there") {
		t.Fatal("single shared token must not match")
	}
	if IsRollingContinuation("the cat", "the dog") {
		t.Fatal("different tail must not match")
	}
}

func TestMergeIntervals(t *testing.T) {
	merged := MergeIntervals([]Interval{
		{0, 100}, {90, 200}, {230, 300}, {1000, 1100},
	}, 40)
	if !reflect.DeepEqual(merged, []Interval{{0, 300}, {1000, 1100}}) {
		t.Fatalf("%+v", merged)
	}
	strict := MergeIntervals([]Interval{{0, 100}, {230, 300}}, 0)
	if !reflect.DeepEqual(strict, []Interval{{0, 100}, {230, 300}}) {
		t.Fatalf("%+v", strict)
	}
}

func TestIntervalGapsReportsUncoveredSpans(t *testing.T) {
	gaps := IntervalGaps(
		[]Interval{{0, 10000}},
		[]Interval{{1000, 3000}, {8000, 12000}},
	)
	if !reflect.DeepEqual(gaps, []Interval{{0, 1000}, {3000, 8000}}) {
		t.Fatalf("%+v", gaps)
	}
	if got := IntervalGaps(nil, []Interval{{0, 5}}); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestFrameCoverageCountsWholeFrames(t *testing.T) {
	speech := []Interval{{0, 3000}}
	covered := []Interval{{0, 2999}}
	got := FrameCoverage(speech, covered, 30)
	if got.SpeechFrames != 90 || got.CoveredFrames != 90 {
		t.Fatalf("%+v", got)
	}
	uncovered := FrameCoverage(speech, []Interval{{0, 2966}}, 30)
	if uncovered.CoveredFrames != 89 || uncovered.Ratio >= 1 {
		t.Fatalf("%+v", uncovered)
	}
	empty := FrameCoverage(nil, covered, 30)
	if empty.SpeechFrames != 0 || empty.CoveredFrames != 0 || empty.Ratio != 1 {
		t.Fatalf("%+v", empty)
	}
}

func TestSrtVttEmitOnlyTimedCues(t *testing.T) {
	cues := []Cue{
		{ID: "a", StartMs: 500, EndMs: new(2500.0), Text: "Timed", Source: "captions"},
		{ID: "b", StartMs: 3000, EndMs: nil, Text: "Untimed", Source: "captions"},
	}
	srt := CuesToSrt(cues)
	if !strings.Contains(srt, "00:00:00,500 --> 00:00:02,500\nTimed") {
		t.Fatalf("srt: %q", srt)
	}
	if strings.Contains(srt, "Untimed") {
		t.Fatalf("untimed cue got a fabricated end: %q", srt)
	}
	vtt := CuesToVtt(cues)
	if !strings.HasPrefix(vtt, "WEBVTT") || !strings.Contains(vtt, "00:00:00.500 --> 00:00:02.500") {
		t.Fatalf("vtt: %q", vtt)
	}
	if got := SrtStamp(3723456); got != "01:02:03,456" {
		t.Fatalf("stamp: %q", got)
	}
}

func TestRollingMergeBoundary(t *testing.T) {
	parse := func(text, next string, start float64) int {
		raw := `{"events":[
			{"tStartMs":0,"dDurationMs":1000,"segs":[{"utf8":"` + text + `"}]},
			{"tStartMs":` + fmt.Sprint(start) + `,"dDurationMs":1000,"segs":[{"utf8":"` + next + `"}]}
		]}`
		parsed, err := ParseTimedJson3([]byte(raw), 0)
		if err != nil {
			t.Fatal(err)
		}
		return len(parsed.Cues)
	}
	if got := parse("hello there", "hello there again", 1200); got != 1 {
		t.Fatalf("1200: %d", got)
	}
	if got := parse("hello there", "hello there again", 1201); got != 2 {
		t.Fatalf("1201: %d", got)
	}
	if got := parse("hello", "hello again", 500); got != 2 {
		t.Fatalf("single token: %d", got)
	}
}
