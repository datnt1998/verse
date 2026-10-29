package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMobileQueueErrorContract(t *testing.T) {
	s := newTestService(t)
	script := &Script{Volume: 1, Chapter: 11, Status: StatusLocked, Segments: []Segment{{Text: "Một đoạn."}}}
	if err := WriteScript(s.Layout(), script); err != nil {
		t.Fatal(err)
	}
	id, _ := s.queue.Enqueue(1, 11)
	s.queue.Update(id, func(j *Job) { j.State = JobError; j.Error = "Máy đọc chưa sẵn sàng"; j.Segments = 1 })
	view, err := s.Timeline([]SourceChapter{{Chapter: 11, Title: "Chương mới"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(view.Chapters[0])
	var row map[string]any
	_ = json.Unmarshal(raw, &row)
	if row["state"] != "error" || row["reason"] != "Máy đọc chưa sẵn sàng" || row["quality"] != "standard" {
		t.Fatalf("mobile queue contract missing error detail: %s", raw)
	}
}

func TestMobileTrialPreservesFullExport(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	source := []SourceChapter{{Chapter: 4, Title: "Bản thử"}}
	script := &Script{Volume: 1, Chapter: 4, Status: StatusLocked, Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Xin chào.", Rendered: &SegmentAudio{Key: "ready", DurationSec: .5}}}}
	if err := WriteScript(s.layout, script); err != nil {
		t.Fatal(err)
	}
	if err := WriteSegmentAudio(s.layout, 1, 4, "ready", testWAV(48000, 1, 24000, 8000)); err != nil {
		t.Fatal(err)
	}
	full, err := s.Export(context.Background(), 1, "Truyện", source, ExportOptions{Format: "mp3"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.layout.ExportsDir, full.Files[0].Path)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(s.layout.VolumeExportDir(1), exportMetaFile)
	meta, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := s.Export(context.Background(), 1, "Truyện", source, ExportOptions{Format: "mp3", Chapters: []int{4}, Trial: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range trial.Files {
		if strings.Contains(file.Name, "(bản thử)") && strings.Contains(file.Name, "Chương 04") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing isolated trial filename: %+v", trial.Files)
	}
	after, _ := os.ReadFile(path)
	afterMeta, _ := os.ReadFile(manifest)
	if !bytes.Equal(before, after) || !bytes.Equal(meta, afterMeta) {
		t.Fatal("trial overwrote full export or its manifest")
	}
}

func TestMobileQueueLocksSelectedChapters(t *testing.T) {
	s := newTestService(t)
	// Hold the worker so the persisted waiting state can be asserted deterministically.
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
	defer func() { s.runMu.Lock(); s.running = false; s.runMu.Unlock() }()
	source := []SourceChapter{{Chapter: 11, Text: "Một."}, {Chapter: 12, Text: "Hai."}}
	for _, sc := range source {
		if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: sc.Chapter, Status: StatusReview, SourceHash: SourceHash(sc.Text), Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: sc.Text, Flags: []string{FlagUnknownSpeaker}}}}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{11, 12}, Quality: "standard", VoiceMode: "cast", Lock: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chapters) != 2 || len(result.Warnings) != 2 || result.ETASec != 0 || result.ETAReason == "" {
		t.Fatalf("batch response: %+v", result)
	}
	for _, sc := range source {
		script, err := s.Script(1, sc.Chapter)
		if err != nil {
			t.Fatal(err)
		}
		if script.Status != StatusLocked || script.LockedAt == nil || script.SourceHash != SourceHash(sc.Text) {
			t.Fatalf("chapter not locked: %+v", script)
		}
	}
	for _, job := range s.Queue() {
		if job.State != JobQueued || job.Quality != "standard" {
			t.Fatalf("chapter not waiting: %+v", job)
		}
	}
}

func TestMobileQueueAcceptsChaptersWithoutScripts(t *testing.T) {
	s := newTestService(t)
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
	defer func() { s.runMu.Lock(); s.running = false; s.runMu.Unlock() }()
	source := []SourceChapter{{Chapter: 4, Text: "Một chương đã nghiệm thu.", Completed: true}, {Chapter: 5, Text: "Chương tiếp theo.", Completed: true}}
	result, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{4, 5}, Quality: "standard", Lock: true})
	if err != nil {
		t.Fatalf("unprepared chapters must enter the queue: %v", err)
	}
	if len(result.Chapters) != 2 || len(s.Queue()) != 2 {
		t.Fatalf("missing queued chapters: result=%+v jobs=%+v", result, s.Queue())
	}
	for _, job := range s.Queue() {
		if job.State != JobQueued {
			t.Fatalf("chapter must wait before composition: %+v", job)
		}
	}
}

func TestMobileQueueComposesLocksAndRendersInOrder(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	s.synth = &stoppedSynth{}
	voiceID := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	source := []SourceChapter{{Chapter: 4, Text: "Chương bốn.", Completed: true}, {Chapter: 5, Text: "Chương năm.", Completed: true}}
	var order []int
	s.queuedComposer = func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
		if chapter == 5 {
			prior, _ := ReadScript(s.layout, 1, 4)
			done, _ := prior.AudioStats()
			if done != 1 || prior.Status != StatusLocked {
				t.Errorf("chapter 5 composed before chapter 4 rendered: %+v", prior)
			}
		}
		order = append(order, chapter)
		sc := source[chapter-4]
		return sc, &fakeComposer{result: ComposeResult{Model: "test", Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: sc.Text}}}}, CharacterRef{}, nil
	}
	if _, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{4, 5}, Lock: true, Quality: "standard"}); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	if len(order) != 2 || order[0] != 4 || order[1] != 5 {
		t.Fatalf("composition order: %v", order)
	}
	for _, job := range s.Queue() {
		script, _ := ReadScript(s.layout, 1, job.Chapter)
		if job.State != JobDone || job.NeedsScript || job.Step != "render" || script.LockedAt == nil || script.SourceHash != SourceHash(source[job.Chapter-4].Text) {
			t.Fatalf("incomplete compose-lock-render pipeline: job=%+v script=%+v", job, script)
		}
	}
}

func TestMobileQueueMissingModelCanRetryComposition(t *testing.T) {
	s := newTestService(t)
	source := []SourceChapter{{Chapter: 4, Text: "Bốn.", Completed: true}}
	if _, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{4}, Lock: true}); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	job := s.Queue()[0]
	if job.State != JobError || !job.NeedsScript || job.Error != ErrNoModel.Error() {
		t.Fatalf("missing model must be actionable and retryable: %+v", job)
	}
	s.queuedComposer = func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
		return source[0], &fakeComposer{result: ComposeResult{Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: source[0].Text}}}}, CharacterRef{}, nil
	}
	if err := s.Retry(job.ID, ""); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	script, _ := ReadScript(s.layout, 1, 4)
	retried := s.Queue()[0]
	if script == nil || script.Status != StatusLocked || retried.NeedsScript || retried.Step != "render" {
		t.Fatalf("retry did not resume composition: script=%+v job=%+v", script, retried)
	}
}

func TestMobileQueueCompositionResumesAfterReload(t *testing.T) {
	s := newTestService(t)
	source := []SourceChapter{{Chapter: 4, Text: "Chương bốn.", Completed: true}}
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
	if _, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{4}, Lock: true}); err != nil {
		t.Fatal(err)
	}
	s.runMu.Lock()
	s.running = false
	s.runMu.Unlock()
	// Reload from queue.json, not from the request's in-memory source.
	resumed, err := NewService(s.layout, NullSynth{}, func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
		return source[0], &fakeComposer{result: ComposeResult{Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: source[0].Text}}}}, CharacterRef{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	resumed.runWG.Wait()
	script, _ := ReadScript(s.layout, 1, 4)
	job := resumed.Queue()[0]
	if script == nil || script.Status != StatusLocked || job.NeedsScript || job.Step != "render" {
		t.Fatalf("queued composition lost on restart: script=%+v job=%+v", script, job)
	}
}

func TestMobileResumeKeepsFinishedSegments(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	synth := &stoppedSynth{}
	s.synth = synth
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Đoạn đầu."}, {ID: "s2", Speaker: NarratorID, Text: "Đoạn sau."}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(context.Background(), 1, 1, "s1"); err != nil {
		t.Fatal(err)
	}
	if synth.calls != 1 {
		t.Fatalf("preview calls=%d", synth.calls)
	}
	id, _ := s.queue.Enqueue(1, 1)
	s.queue.Update(id, func(j *Job) { j.State = JobPaused; j.SegmentsDone = 1; j.Segments = 2; j.Quality = "standard" })
	if err := s.Resume(id); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	job, _ := s.queue.Find(id)
	if job.State != JobDone || synth.calls != 2 {
		t.Fatalf("resume redid finished segments or stalled: state=%s calls=%d", job.State, synth.calls)
	}
}
