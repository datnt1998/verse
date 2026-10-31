package transcript

// Chuyển ca test từ yt-factory src/server/clone/transcript.test.ts — phần thuần
// (mergeAsrSegments, applyCaptionEdits) (2c201e6).

import "testing"

func TestMergeAsrSegmentsDropsDuplicates(t *testing.T) {
	cues := []Cue{
		{ID: "c1", StartMs: 1000, EndMs: new(3000.0), Text: "Xin chào các bạn", Source: "captions"},
	}
	merged := MergeAsrSegments(cues, []AsrSegment{
		{StartMs: 1100, EndMs: 2900, Text: "xin chào các bạn", AvgLogprob: new(-0.1)},
		{StartMs: 8000, EndMs: 9500, Text: "phần mới", AvgLogprob: new(-0.4)},
	}, 60000)
	if merged.Rejected != 1 || merged.Added != 1 || len(merged.Cues) != 2 {
		t.Fatalf("%+v", merged)
	}
	if merged.Cues[1].Source != "asr" {
		t.Fatalf("cue: %+v", merged.Cues[1])
	}
}

func TestApplyCaptionEdits(t *testing.T) {
	manifest := CaptionManifest{
		Version:  1,
		Revision: 1,
		Language: new("vi"),
		Cues: []Cue{
			{ID: "a", StartMs: 0, EndMs: nil, Text: "untimed", Source: "captions"},
			{ID: "b", StartMs: 5000, EndMs: new(6000.0), Text: "timed", Source: "captions"},
		},
		SpeechIntervals: []Interval{{0, 1000}, {5000, 6000}},
		Coverage:        Coverage{FPS: 30, SpeechFrames: 60, CoveredFrames: 30, Ratio: 0.5},
		UnresolvedGaps:  []Interval{{0, 1000}},
		NeedsReview:     []ReviewItem{{CueID: "a", Reason: "cue has no end timing"}},
		Warnings:        []CaptionWarning{},
		GeneratedAt:     "t",
	}
	next := ApplyCaptionEdits(manifest, []CaptionEdit{
		{CueID: "a", EndMs: new(1000.0), Text: new("untimed fixed")},
		{CueID: "ghost", Text: new("ignored")},
	}, 1700000000500)
	if next.Revision != 2 {
		t.Fatalf("revision: %d", next.Revision)
	}
	if next.Cues[0].Source != "edited" || next.Cues[0].EndMs == nil || *next.Cues[0].EndMs != 1000 {
		t.Fatalf("cue: %+v", next.Cues[0])
	}
	if len(next.UnresolvedGaps) != 0 || len(next.NeedsReview) != 0 {
		t.Fatalf("gaps/review: %+v %+v", next.UnresolvedGaps, next.NeedsReview)
	}
	if manifest.Cues[0].EndMs != nil || manifest.Revision != 1 {
		t.Fatal("source manifest mutated")
	}
}
