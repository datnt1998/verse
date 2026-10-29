package audio

import (
	"context"
	"testing"
	"time"
)

func TestExportChapterSelectionCanonicalNames(t *testing.T) {
	source := []SourceChapter{{Chapter: 5}, {Chapter: 3}, {Chapter: 1}, {Chapter: 2}, {Chapter: 4}}
	selected, ids, err := selectExportChapters(source, 1, []int{5, 1, 3, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 4 || selected[0].Chapter != 1 || selected[3].Chapter != 5 {
		t.Fatalf("selection not sorted/deduplicated: %+v", selected)
	}
	if got := exportChapterSuffix(ids); got != "-ch1-3_5" {
		t.Fatalf("disjoint selection filename suffix=%q", got)
	}
	for _, test := range []struct {
		ids  []int
		want string
	}{{nil, ""}, {[]int{2}, "-ch2"}, {[]int{1, 2, 3, 4, 5, 6, 7, 8, 9}, "-ch1-9"}, {[]int{1, 3}, "-ch1_3"}} {
		if got := exportChapterSuffix(test.ids); got != test.want {
			t.Fatalf("suffix %v=%q want %q", test.ids, got, test.want)
		}
	}
}

type blockingExportSynth struct {
	stoppedSynth
	started  chan struct{}
	canceled chan struct{}
}

func (s *blockingExportSynth) Synthesize(ctx context.Context, _ SynthRequest) (Audio, error) {
	close(s.started)
	<-ctx.Done()
	close(s.canceled)
	return Audio{}, ctx.Err()
}

func TestCancelRunningExportCancelsItsContext(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	svc := newTestService(t)
	synth := &blockingExportSynth{started: make(chan struct{}), canceled: make(chan struct{})}
	svc.synth = synth
	voice := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Role: RoleNarrator, Name: "Dẫn truyện", VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(svc.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào.", Rendered: &SegmentAudio{Key: "ready", DurationSec: .5}}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSegmentAudio(svc.layout, 1, 1, "ready", testWAV(48000, 1, 24000, 8000)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Export(context.Background(), 1, "Truyện", []SourceChapter{{Chapter: 1, Title: "Mở đầu"}}, ExportOptions{Format: "m4b", WhenReady: true, ChapterTitles: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-synth.started:
	case <-time.After(3 * time.Second):
		t.Fatal("export never started title synthesis")
	}
	if err := svc.ControlExport(1, "cancel"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-synth.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not cancel the running export context")
	}
	svc.runWG.Wait()
	if requests := svc.ExportRequests(); len(requests) != 1 || requests[0].State != JobCanceled {
		t.Fatalf("export completion overwrote cancellation: %+v", requests)
	}
	if len(svc.Queue()) != 0 {
		t.Fatal("canceling export changed render queue")
	}
}
