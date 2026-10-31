package activity

import (
	"testing"
	"time"
)

func TestNextScanGroupsEarliestHour(t *testing.T) {
	at := time.Date(2026, 1, 2, 2, 5, 0, 0, time.UTC)
	got := NextScanFromTimes([]time.Time{at.Add(20 * time.Minute), at.Add(2 * time.Hour), at})
	if got == nil || !got.At.Equal(at) || got.RivalChannels != 2 {
		t.Fatalf("next=%+v", got)
	}
	if NextScanFromTimes(nil) != nil {
		t.Fatal("empty schedule")
	}
}
