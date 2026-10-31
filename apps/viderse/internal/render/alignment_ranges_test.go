package render

import (
	"testing"

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
