package render

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
)

func TestAlignmentRangesExcludePassingAnchorsAndKeepMedianFailures(t *testing.T) {
	f := newScorerFixture(t)
	metric := transcript.AlignmentMetric{Worst: []transcript.AlignmentWorst{
		{ItemID: "caption", Frame: 0, Text: "exact", DeltaMs: 0},
		{ItemID: "caption", Frame: 15, Text: "boundary", DeltaMs: 120},
		{ItemID: "caption", Frame: 30, Text: "late", DeltaMs: 150},
		{ItemID: "caption", Frame: 45, Text: "early", DeltaMs: -400},
	}}
	ranges := scoreAlignmentRanges(metric, scoreReadProject(f.input.Project), 30)
	if len(ranges) != 2 {
		t.Fatalf("got%d ranges; exact and120ms anchors must not be failures: %+v", len(ranges), ranges)
	}
	if ranges[0].Text != "late" || ranges[1].Text != "early" {
		t.Fatalf("wrong failure ranges: %+v", ranges)
	}
	if *ranges[0].OffsetSec != 0.15 || *ranges[1].OffsetSec != -0.4 {
		t.Fatalf("lost signed timing evidence: %+v", ranges)
	}
}

func TestScoreRangesFiltersPersistedPassingAnchors(t *testing.T) {
	f := newScorerFixture(t)
	score := f.score()
	score.Items = []quality.Item{{ID: "captionAlignment", Status: "fail", Measured: map[string]any{"ranges": []quality.Range{
		{Start: 0, End: 1, OffsetSec: new(0.)},
		{Start: 1, End: 2, OffsetSec: new(0.12)},
		{Start: 2, End: 3, OffsetSec: new(-0.15)},
	}}}}
	ranges := ScoreRanges(score, f.input.Project)
	if len(ranges) != 1 || ranges[0].OffsetSec == nil || *ranges[0].OffsetSec != -0.15 {
		t.Fatalf("persisted ranges retained passing anchors: %+v", ranges)
	}
}
