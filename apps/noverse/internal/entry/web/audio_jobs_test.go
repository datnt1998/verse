package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

func TestAudioRowsGroupVolumesAndFailedChapters(t *testing.T) {
	dir := t.TempDir()
	layout := audio.Layout{StoryDir: dir, AudioDir: filepath.Join(dir, "audio"), AssetsDir: filepath.Join(dir, "tai-san"), ExportsDir: filepath.Join(dir, "xuat-ban")}
	t0 := time.Now().Add(-10 * time.Minute)
	jobs := []audio.Job{
		// Tập 1: chương 1 xong (10 đoạn), chương 2 đang chờ (paused), chương 3 lỗi.
		{ID: "a", Volume: 1, Chapter: 1, State: audio.JobDone, Segments: 10, SegmentsDone: 10, CreatedAt: t0, UpdatedAt: t0.Add(time.Minute)},
		{ID: "b", Volume: 1, Chapter: 2, State: audio.JobPaused, Segments: 10, SegmentsDone: 5, CreatedAt: t0, UpdatedAt: t0},
		{ID: "c", Volume: 1, Chapter: 3, State: audio.JobError, Error: "máy đọc đang bận", CreatedAt: t0, UpdatedAt: t0},
		// Tập 2: xong hết.
		{ID: "d", Volume: 2, Chapter: 9, State: audio.JobDone, Segments: 4, SegmentsDone: 4, CreatedAt: t0, UpdatedAt: t0.Add(2 * time.Minute)},
	}
	data, _ := json.Marshal(map[string]any{"jobs": jobs})
	if err := os.MkdirAll(layout.AudioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.AudioDir, "queue.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := audio.NewService(layout, audio.NullSynth{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	rows := audioRows(audioEntry{svc: svc, project: "p", story: "s"}, time.Now(), true)
	byID := map[string]Job{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	failed, ok := byID["p/s/audio/1/3"]
	if !ok || failed.Status != "failed" || failed.Error == nil || failed.Title != "Tạo giọng chương 3" {
		t.Fatalf("failed row = %+v (rows %+v)", failed, rows)
	}
	vol1 := byID["p/s/audio/1"]
	if vol1.Status != "paused" || vol1.Unit == nil || vol1.Unit.Done != 1 || vol1.Unit.Total != 2 {
		t.Fatalf("volume 1 row = %+v unit=%+v", vol1, vol1.Unit)
	}
	if vol1.Progress == nil || *vol1.Progress != 0.75 {
		t.Fatalf("progress must be segment-weighted (15/20): %v", vol1.Progress)
	}
	vol2 := byID["p/s/audio/2"]
	if vol2.Status != "done" || vol2.FinishedAt == nil || vol2.Result == nil || vol2.Result.Actions[0].Href != "/p/p/s/s/audio/2/listen" {
		t.Fatalf("volume 2 row = %+v", vol2)
	}
}
