package scenes

// Ported from yt-factory 2c201e6 src/server/clone/scene-parser.test.ts.
import (
	"math"
	"reflect"
	"testing"
)

func TestSceneMetadataCandidates(t *testing.T) {
	frames := ParseSceneMetadata("garbage\nframe:2 pts:1 pts_time:NaN\nframe:45 pts:23040 pts_time:1.5\nlavfi.scd.score=30.366\nframe:46 pts:23552 pts_time:1.53333\nlavfi.scd.score=0\nlavfi.scd.time=1.5\nframe:90 pts:46080 pts_time:3\nlavfi.scdet.score=15.625\n")
	got := SceneCandidates(frames, 10)
	if len(frames) != 3 || len(got) != 3 || got[0].TimeSec != 1.5 || got[1].TimeSec != 1.5 || got[2].Score != 15.625 {
		t.Fatalf("frames=%+v candidates=%+v", frames, got)
	}
	same := SceneCandidates([]DetectFrame{{Index: 45, PTSSec: 1.5, Score: ptr(50.), SCDTime: ptr(1.5)}, {Index: 90, PTSSec: 3, Score: ptr(20.), SCDTime: ptr(3.)}}, 10)
	if len(same) != 2 || same[1].Score != 20 {
		t.Fatalf("borrowed score: %+v", same)
	}
}
func TestNormalizeAndSelectBoundaries(t *testing.T) {
	input := []Candidate{{TimeSec: 1, Score: 40}, {TimeSec: .5, Score: 60}, {TimeSec: 10.499, Score: 20}, {TimeSec: 1.0004, Score: 35}, {TimeSec: 5, Score: 18}, {TimeSec: 9.7, Score: 50}, {TimeSec: math.NaN(), Score: 99}}
	n := NormalizeBoundaries(input, 10, .2, .5, 30)
	if !reflect.DeepEqual(n.Boundaries, []float64{.5, 4.5, 9.2}) || len(n.Dropped) != 4 {
		t.Fatalf("normalized=%+v", n)
	}
	var c []Candidate
	for _, v := range []float64{21, 30, 33, 78, 90} {
		c = append(c, Candidate{TimeSec: v, Score: 30})
	}
	s := SelectTargetBoundaries(NormalizeBoundaries(c, 100, .2, 0, 30), 100, 5, .2, 30)
	if !reflect.DeepEqual(s.Boundaries, []float64{21, 33, 60, 78}) {
		t.Fatalf("target=%+v", s)
	}
	tight := NormalizeBoundaries([]Candidate{{TimeSec: 2.9, Score: 50}, {TimeSec: 3, Score: 50}}, 10, .05, 0, 30)
	if got := SelectTargetBoundaries(tight, 10, 5, .2, 30); !reflect.DeepEqual(got.Boundaries, []float64{2.9, 4, 6, 8}) {
		t.Fatalf("tight=%+v", got)
	}
}
func TestWholeFrameSpansPreserveExactSourceTimes(t *testing.T) {
	spans, total, err := SpansFromBoundaries([]float64{1.017, 2.056}, 2.371, 30)
	if err != nil || total != 71 || len(spans) != 3 || spans[0].EndSec != 1.017 || spans[2].EndSec != 2.371 {
		t.Fatalf("%+v %d %v", spans, total, err)
	}
	sum := 0
	for i, s := range spans {
		if s.Frames < 1 || (i > 0 && s.StartFrame != spans[i-1].EndFrame) {
			t.Fatal("noncontiguous span")
		}
		sum += s.Frames
	}
	if sum != total {
		t.Fatal("frame drift")
	}
	spans, _, err = SpansFromBoundaries([]float64{1, 1.01, 3}, 6, 30)
	if err != nil || len(spans) != 3 {
		t.Fatalf("same-frame cut: %+v %v", spans, err)
	}
	for _, duration := range []float64{0, -1, math.NaN(), .001} {
		if _, _, err := SpansFromBoundaries(nil, duration, 30); err == nil {
			t.Fatalf("accepted %v", duration)
		}
	}
}
func TestCaptionOverlapAndHonestSilentContext(t *testing.T) {
	cues := []Cue{{ID: "c1", StartMS: 0, EndMS: ptr(1500.), Text: "Mở đầu"}, {ID: "c2", StartMS: 1400, EndMS: ptr(3200.), Text: "Qua cảnh"}, {ID: "c3", StartMS: 5200, EndMS: ptr(6000.), Text: "Kết"}, {ID: "u1", StartMS: 4000, Text: "Untimed"}}
	a := AssignSceneText(0, 1.5, cues)
	b := AssignSceneText(1.5, 3, cues)
	silent := AssignSceneText(3.2, 5, cues)
	if len(a.CueIDs) != 2 || len(b.CueIDs) != 1 || b.CueIDs[0] != "c2" || silent.Text != "" || silent.Context == nil || silent.Context.CueID != "c2" || silent.Context.Position != "previous" {
		t.Fatalf("%+v %+v %+v", a, b, silent)
	}
	next := AssignSceneText(0, 2, cues[2:3])
	if next.Context == nil || next.Context.Position != "next" {
		t.Fatalf("%+v", next)
	}
}
func ptr[T any](v T) *T { return &v }
