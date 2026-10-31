package images

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSceneViewTimingAndNarration(t *testing.T) {
	s, store := testService()
	store.m.Items[0].StartFrame = 30
	store.m.Items[0].EndFrame = 120
	store.m.Items[0].Narration = "The scene narration."
	view, err := s.Read(context.Background(), "job")
	if err != nil { t.Fatal(err) }
	data, err := json.Marshal(view.Items[0])
	if err != nil { t.Fatal(err) }
	var item struct {
		Start float64 `json:"startSec"`
		End float64 `json:"endSec"`
		Duration float64 `json:"durationSec"`
		Narration string `json:"narration"`
	}
	if err := json.Unmarshal(data, &item); err != nil { t.Fatal(err) }
	if item.Start != 1 || item.End != 4 || item.Duration != 3 || item.Narration != "The scene narration." {
		t.Fatalf("wrong timing: %s", data)
	}
}
