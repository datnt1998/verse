package web

import (
	"encoding/json"
	"math"
	"net/http"
	"testing"

	"github.com/datnt1998/noverse/internal/audio"
)

func TestAudioQueueTimelineV(t *testing.T) {
	f := newAudioFixture(t)
	response := f.do(http.MethodGet, "/audio/queue?volume=1", "")
	if response.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", response.Code, response.Body.String())
	}
	var view struct {
		Chapters []struct {
			Chapter int      `json:"chapter"`
			State   string   `json:"state"`
			Sec     *float64 `json:"sec"`
			EstSec  float64  `json:"estSec"`
			Pct     float64  `json:"pct"`
		} `json:"chapters"`
		MadeSec     float64 `json:"madeSec"`
		EstTotalSec float64 `json:"estTotalSec"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Chapters) != 2 {
		t.Fatalf("timeline chapters = %d, want 2; body=%s", len(view.Chapters), response.Body.String())
	}
	total := 0.0
	for _, chapter := range view.Chapters {
		expected := (&audio.Script{Segments: []audio.Segment{{Text: f.texts[chapter.Chapter]}}}).EstimatedDuration()
		if chapter.State != "unlocked" || chapter.Sec != nil || chapter.EstSec != expected || chapter.Pct != 0 {
			t.Fatalf("chapter %+v, estimated %v", chapter, expected)
		}
		total += expected
	}
	if view.MadeSec != 0 || math.Abs(view.EstTotalSec-total) > .001 {
		t.Fatalf("totals: made=%v estimated=%v want=%v", view.MadeSec, view.EstTotalSec, total)
	}
}
