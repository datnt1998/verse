package audio

import (
	"math"
	"testing"
)

func TestTimelineVEstimatesAndStates(t *testing.T) {
	svc := newTestService(t)
	source := []SourceChapter{{Chapter: 1, Title: "Đã tạo"}, {Chapter: 2, Title: "Đang tạo"}, {Chapter: 3, Title: "Chờ tạo"}, {Chapter: 4, Title: "Chưa chốt"}}
	scripts := []*Script{
		{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{Text: "Một hai ba.", PauseMs: 500, Rendered: &SegmentAudio{DurationSec: 10}}}},
		{Volume: 1, Chapter: 2, Status: StatusLocked, Segments: []Segment{{Text: "Một hai ba.", Rendered: &SegmentAudio{DurationSec: 4}}, {Text: "Bốn năm sáu."}}},
		{Volume: 1, Chapter: 3, Status: StatusLocked},
	}
	for _, script := range scripts {
		if err := WriteScript(svc.Layout(), script); err != nil {
			t.Fatal(err)
		}
	}
	id, _ := svc.queue.Enqueue(1, 2)
	svc.queue.Update(id, func(job *Job) { job.State = JobRendering; job.Percent = 50 })
	view, err := svc.Timeline(source, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i, state := range []string{"done", "rendering", "queued", "unlocked"} {
		chapter := view.Chapters[i]
		if chapter.State != state || chapter.EstSec <= 0 {
			t.Fatalf("chapter %d = %+v, want %s and positive estimate", i+1, chapter, state)
		}
		if i > 0 && chapter.Sec != nil {
			t.Fatalf("unfinished chapter %d has actual seconds", i+1)
		}
	}
	if *view.Chapters[0].Sec != 10.5 || view.Chapters[1].Pct != 50 {
		t.Fatalf("actual / progress: %+v", view.Chapters)
	}
	if view.Chapters[2].EstSec != 10.5 || view.Chapters[3].EstSec != 10.5 {
		t.Fatalf("unknown chapters must use finished mean: %+v", view.Chapters)
	}
	if view.MadeSec != 14.5 {
		t.Fatalf("madeSec = %v, want actual generated segments including pauses", view.MadeSec)
	}
	expected := 31.5 + scripts[1].EstimatedDuration()
	if math.Abs(view.EstTotalSec-expected) > .001 {
		t.Fatalf("estimated total = %v, want %v", view.EstTotalSec, expected)
	}
	queued, _ := svc.queue.Enqueue(1, 3)
	svc.queue.Update(id, func(job *Job) { job.State = JobPaused })
	svc.queue.Update(queued, func(job *Job) { job.State = JobPaused })
	paused, err := svc.Timeline(source, 1)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Chapters[1].State != "rendering" || paused.Chapters[2].State != "queued" {
		t.Fatalf("pausing the volume must not turn waiting chapters into active chapters: %+v", paused.Chapters)
	}
}

func TestTimelineVUnknownVolumeEqualShares(t *testing.T) {
	svc := newTestService(t)
	view, err := svc.Timeline([]SourceChapter{{Chapter: 1}, {Chapter: 2}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.Chapters[0].EstSec <= 0 || view.Chapters[0].EstSec != view.Chapters[1].EstSec {
		t.Fatalf("unknown chapters need equal nonzero shares: %+v", view.Chapters)
	}
}

func TestTimelineOnlyIncludesSelectedVolumeJobs(t *testing.T) {
	svc := newTestService(t)
	svc.queue.Enqueue(1, 1)
	id, _ := svc.queue.Enqueue(2, 9)
	view, err := svc.Timeline([]SourceChapter{{Chapter: 9, Title: "Tập hai"}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Jobs) != 1 || view.Jobs[0].ID != id || view.Jobs[0].Volume != 2 {
		t.Fatalf("selected volume 2 leaked jobs from another volume: %+v", view.Jobs)
	}
}
