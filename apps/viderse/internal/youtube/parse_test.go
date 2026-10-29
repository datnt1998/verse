// Ca test chuyển từ yt-factory src/server/youtube/parse.test.ts.
package youtube

import (
	"testing"
	"time"
)

func TestParseCompactNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"1.2M", 1_200_000, true},
		{"21.3M subscribers", 21_300_000, true},
		{"950K views", 950_000, true},
		{"2B", 2_000_000_000, true},
		{"1,234 views", 1234, true},
		{"539 videos", 539, true},
		{"No views", 0, true},
		{"Premieres soon", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseCompactNumber(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseCompactNumber(%q) = %v,%v; muốn %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"5:08", 308, true},
		{"1:02:03", 3723, true},
		{"0:59", 59, true},
		{"LIVE", 0, false},
		{"SHORTS", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseDuration(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseDuration(%q) = %v,%v; muốn %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"10d ago", -10 * day, true},
		{"3 weeks ago", -21 * day, true},
		{"9mo ago", -270 * day, true},
		{"Streamed 2 years ago", -730 * day, true},
		{"5h ago", -5 * time.Hour, true},
		{"30 minutes ago", -30 * time.Minute, true},
		{"Scheduled for 10/12/26", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseRelativeTime(c.in, now)
		if ok != c.ok {
			t.Errorf("ParseRelativeTime(%q) ok=%v; muốn %v", c.in, ok, c.ok)
			continue
		}
		if ok && !got.Equal(now.Add(c.want)) {
			t.Errorf("ParseRelativeTime(%q) = %v; muốn %v", c.in, got, now.Add(c.want))
		}
	}
}

func TestMedian(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if got := Median([]*float64{f(3), nil, f(1), f(2)}); got == nil || *got != 2 {
		t.Fatalf("Median = %v", got)
	}
	if got := Median([]*float64{f(4), f(1), f(3), f(2)}); got == nil || *got != 2.5 {
		t.Fatalf("Median = %v", got)
	}
	if got := Median([]*float64{nil}); got != nil {
		t.Fatalf("Median = %v", got)
	}
}
