package queue

// Ported from yt-factory 2c201e6 queue.test.ts and queue-eta.test.ts.
import (
	"testing"
	"time"
)

func scheduleNow() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local) }
func testLane(id string, count, pace int) Lane {
	lane := Lane{ChannelID: id, Pace: pace}
	for i := 0; i < count; i++ {
		lane.Items = append(lane.Items, Item{ID: id + string(rune('a'+i)), ChannelID: id, Priority: "batch", Position: float64(i + 1), DurationSec: 60})
	}
	return lane
}

func TestScheduleFairRotation(t *testing.T) {
	lanes := []Lane{testLane("a", 10, 50), testLane("b", 10, 50), testLane("c", 10, 50)}
	counts := map[string]int{}
	last := ""
	for round := 0; round < 5; round++ {
		items := Order(lanes, 500)
		for _, item := range items[:2] {
			if last == item.ChannelID {
				t.Fatalf("same channel started consecutively: %s", last)
			}
			last = item.ChannelID
			counts[last]++
			for i := range lanes {
				if lanes[i].ChannelID == last {
					lanes[i].Items = lanes[i].Items[1:]
					lanes[i].LastSequence = int64(round*2 + counts["a"] + counts["b"] + counts["c"])
					lanes[i].LastStart = scheduleNow()
				}
			}
		}
	}
	min, max := 10, 0
	for _, n := range counts {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	if max-min > 1 {
		t.Fatalf("unfair distribution: %v", counts)
	}
}

func TestPickBypassesPaceAndLeadsBatch(t *testing.T) {
	a, b := testLane("a", 2, 1), testLane("b", 1, 1)
	a.UsedToday = 1
	a.Items[1].Priority = "pick"
	order := Order([]Lane{b, a}, 500)
	if order[0].ID != "ab" {
		t.Fatalf("pick must lead all batches, got %s", order[0].ID)
	}
	now := scheduleNow()
	etas := Estimate([]Lane{a, b}, EstimateInput{Now: now, Workers: 1, Basis: DefaultBasis()})
	if etas["ab"].Start == nil || !etas["ab"].Start.Equal(now) {
		t.Fatalf("pick must start despite pace: %+v", etas["ab"])
	}
	if etas["ba"].Start == nil || etas["ba"].Start.Day() != now.Day() {
		t.Fatalf("capped channel must not occupy a worker while another channel is eligible: %+v", etas)
	}
}

func TestETAFallbackScalingAndExportSerialization(t *testing.T) {
	lane := testLane("a", 3, 50)
	now := scheduleNow()
	etas := Estimate([]Lane{lane}, EstimateInput{Now: now, Workers: 2, Basis: DefaultBasis()})
	a, b, c := etas["aa"], etas["ab"], etas["ac"]
	if a.Start == nil || a.Done == nil || a.Done.Sub(*a.Start) != 326*time.Second {
		t.Fatalf("one-minute fallback ETA: %+v", a)
	}
	if !a.Start.Equal(*b.Start) || b.Done.Sub(*a.Done) != 37*time.Second || !c.Start.Equal(*a.Done) {
		t.Fatalf("job/export lanes not serialized: %+v", etas)
	}
	lane.Items[0].DurationSec = 120
	etas = Estimate([]Lane{lane}, EstimateInput{Now: now, Workers: 2, Basis: DefaultBasis()})
	if got := etas["aa"].Done.Sub(*etas["aa"].Start); got != 542*time.Second {
		t.Fatalf("two-minute fallback=%s", got)
	}
}

func TestETAPaceRollsAtLocalMidnightAndStopsAfterSevenDays(t *testing.T) {
	lane := testLane("a", 12, 1)
	now := scheduleNow()
	etas := Estimate([]Lane{lane}, EstimateInput{Now: now, Workers: 2, Basis: DefaultBasis()})
	for i := 1; i < 8; i++ {
		expected := LocalMidnight(now).AddDate(0, 0, i)
		if start := etas[lane.Items[i].ID].Start; start == nil || !start.Equal(expected) {
			t.Fatalf("item %d start=%v want %v", i, start, expected)
		}
	}
	if etas[lane.Items[8].ID].Start != nil {
		t.Fatal("item beyond seven-day horizon has an ETA")
	}
}

func TestETACappedLaneDoesNotDelayEligibleLane(t *testing.T) {
	a, b := testLane("a", 1, 1), testLane("b", 1, 5)
	a.UsedToday = 1
	now := scheduleNow()
	etas := Estimate([]Lane{a, b}, EstimateInput{Now: now, Workers: 1, Basis: DefaultBasis()})
	if start := etas["ba"].Start; start == nil || !start.Equal(now) {
		t.Fatalf("eligible lane blocked by capped lane: %v", start)
	}
}

func TestETAWorksetBound(t *testing.T) {
	lane := testLane("a", 600, 50)
	etas := Estimate([]Lane{lane}, EstimateInput{Now: scheduleNow(), Workers: 2, Basis: DefaultBasis()})
	if len(etas) != 500 {
		t.Fatalf("ETA workset=%d", len(etas))
	}
}

func TestETABasisLatestTwentyNormalizedMedians(t *testing.T) {
	now := scheduleNow()
	samples := []Sample{}
	for i := 0; i < 21; i++ {
		for _, step := range etaSteps {
			elapsed := 2 * time.Second
			if i == 0 {
				elapsed = 1000 * time.Second
			}
			samples = append(samples, Sample{Step: step, Elapsed: elapsed, SourceSeconds: 120, FinishedAt: now.Add(time.Duration(i) * time.Second), SceneCount: 10})
		}
	}
	basis := ObservedBasis(samples)
	lane := testLane("a", 1, 50)
	etas := Estimate([]Lane{lane}, EstimateInput{Now: now, Workers: 2, Basis: basis})
	if got := etas["aa"].Done.Sub(*etas["aa"].Start); got != 14*time.Second {
		t.Fatalf("normalized median ETA=%s", got)
	}
	if basis.ScenesPerMinute != 5 {
		t.Fatalf("scene median=%v", basis.ScenesPerMinute)
	}
}

func TestMoveUsesFractionalPositionAndRejectsCrossLane(t *testing.T) {
	lane := testLane("a", 4, 50)
	moved, err := Move(lane.Items, "ad", "ab", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range moved {
		if item.ID == "ad" && item.Position != 1.5 {
			t.Fatalf("midpoint=%v", item.Position)
		}
	}
	lane.Items[1].Position = 1 + 1e-8
	moved, err = Move(lane.Items, "ac", "ab", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range moved {
		if item.ID == "ac" && item.Position != 1.5 {
			t.Fatalf("renormalized midpoint=%v", item.Position)
		}
	}
	if _, err := Move(lane.Items, "aa", "other", false); err == nil {
		t.Fatal("cross-lane move accepted")
	}
}

func TestETAFuturePacedExportDoesNotBlockEligibleWork(t *testing.T) {
	a, b := testLane("a", 1, 1), testLane("b", 1, 5)
	a.UsedToday = 1
	now := scheduleNow()
	etas := Estimate([]Lane{a, b}, EstimateInput{Now: now, Workers: 1, Basis: DefaultBasis()})
	if done := etas["ba"].Done; done == nil || !done.Equal(now.Add(326*time.Second)) {
		t.Fatalf("future paced export delayed eligible work: %+v", etas["ba"])
	}
}

func TestObservedBasisUsesLatestTwentyEvenWhenInputUnordered(t *testing.T) {
	samples := []Sample{}
	now := scheduleNow()
	for i := range 20 {
		samples = append(samples, Sample{Step: "backdrop", Elapsed: time.Second, FinishedAt: now.Add(time.Duration(i) * time.Second)})
	}
	for i := range 21 {
		samples = append(samples, Sample{Step: "backdrop", Elapsed: 9 * time.Second, FinishedAt: now.Add(time.Duration(i+20) * time.Second)})
	}
	if got := ObservedBasis(samples).Rates["backdrop"]; got != 9*time.Second {
		t.Fatalf("latest twenty median=%v", got)
	}
}
