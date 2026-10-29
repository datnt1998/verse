package audio

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFoldUnspeakableMergesPunctuationIntoNeighbour(t *testing.T) {
	rendered := &SegmentAudio{Key: "old", DurationSec: 1}
	got := foldUnspeakable([]Segment{
		{ID: "s0001", Text: "“", PauseMs: 100},
		{ID: "s0002", Text: "Mở cửa", Rendered: rendered},
		{ID: "s0003", Text: "“VPN”", Source: "“VPN”", Rendered: rendered},
		{ID: "s0004", Text: ",", Source: ",", PauseMs: 300},
		{ID: "s0005", Text: "và", Rendered: rendered},
	})
	want := []struct {
		id, text string
		pause    int
	}{{"s0002", "“Mở cửa", 100}, {"s0003", "“VPN”,", 300}, {"s0005", "và", 0}}
	if len(got) != len(want) {
		t.Fatalf("segments = %+v", got)
	}
	for i, w := range want {
		if got[i].ID != w.id || got[i].Text != w.text || got[i].PauseMs != w.pause {
			t.Fatalf("segment %d = %+v, want %+v", i, got[i], w)
		}
	}
	if got[1].Source != "“VPN”," {
		t.Fatalf("source = %q", got[1].Source)
	}
	if got[0].Rendered != nil || got[1].Rendered != nil || got[2].Rendered != rendered {
		t.Fatal("only segments whose text changed may lose their audio")
	}
	if out := foldUnspeakable([]Segment{{Text: "."}, {Text: "—"}}); len(out) != 0 {
		t.Fatalf("all-punctuation script must fold to nothing, got %+v", out)
	}
}

// Bản đọc cũ có đoạn chỉ là "," đã ghi 0 giây: tạo giọng lại phải gộp đoạn đó,
// đọc lại đúng đoạn nhận dấu câu và ra được file nghe cả chương.
func TestRenderRepairsStoredPunctuationSegment(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	synth := &stoppedSynth{}
	svc := newRenderTestService(t, synth)
	now := time.Now()
	if err := WriteScript(svc.Layout(), &Script{Chapter: 4, Volume: 1, Status: StatusLocked, LockedAt: &now, Segments: []Segment{
		{ID: "s0052", Speaker: NarratorID, Text: "“VPN”"},
		{ID: "s0053", Speaker: NarratorID, Text: ",", Rendered: &SegmentAudio{Key: "zero", DurationSec: 0}},
		{ID: "s0054", Speaker: NarratorID, Text: "“Môi trường phát triển”"},
	}}); err != nil {
		t.Fatal(err)
	}
	waitJobState(t, svc, mustEnqueue(t, svc, 1, 4), JobDone)
	script, err := ReadScript(svc.Layout(), 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Segments) != 2 || script.Segments[0].Text != "“VPN”," {
		t.Fatalf("segments = %+v", script.Segments)
	}
	if synth.calls != 2 {
		t.Fatalf("synth calls = %d, want 2", synth.calls)
	}
	if _, err := svc.ChapterAudioFile(context.Background(), 1, 4); err != nil {
		t.Fatalf("chapter audio must be playable after repair: %v", err)
	}
}

// emptySynth trả WAV 0 giây cho mọi đoạn — mô phỏng bộ đọc không phát ra tiếng.
type emptySynth struct{ stoppedSynth }

func (s *emptySynth) Synthesize(context.Context, SynthRequest) (Audio, error) {
	s.calls++
	return Audio{WAV: testWAV(48000, 1, 0, 0), SampleRate: 48000}, nil
}

// Âm thanh rỗng là lỗi có lời giải thích, không được ghi vào bộ nhớ đệm: nếu ghi,
// mỗi lần "tạo lại" sẽ trúng lại đúng tệp rỗng ấy và chương không bao giờ xong.
func TestRenderRejectsEmptyAudioWithoutCaching(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	synth := &emptySynth{}
	svc := newRenderTestService(t, synth)
	now := time.Now()
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusLocked, LockedAt: &now,
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào."}}}); err != nil {
		t.Fatal(err)
	}
	job := waitJobState(t, svc, mustEnqueue(t, svc, 1, 1), JobError)
	if !strings.Contains(job.Error, "s0001") {
		t.Fatalf("error must name the segment: %q", job.Error)
	}
	waitJobState(t, svc, mustEnqueue(t, svc, 1, 1), JobError)
	if synth.calls != 2 {
		t.Fatalf("retry must call the engine again instead of a cached empty file: calls=%d", synth.calls)
	}
	script, err := ReadScript(svc.Layout(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if script.Segments[0].Rendered != nil {
		t.Fatalf("empty audio must not be recorded: %+v", script.Segments[0].Rendered)
	}
}

func TestSaveScriptFoldsPunctuationSegment(t *testing.T) {
	svc := newTestService(t)
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusReview, Segments: []Segment{
		{ID: "s0001", Speaker: NarratorID, Text: "Một", Rendered: &SegmentAudio{Key: "a", DurationSec: 1}},
		{ID: "s0002", Speaker: NarratorID, Text: "Hai", Rendered: &SegmentAudio{Key: "b", DurationSec: 1}},
	}}); err != nil {
		t.Fatal(err)
	}
	script, err := svc.SaveScript(1, 1, ScriptEdit{Segments: []Segment{
		{ID: "s0001", Speaker: NarratorID, Text: "Một"},
		{ID: "s0002", Speaker: NarratorID, Text: "Hai"},
		{ID: "s0003", Speaker: NarratorID, Text: "."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Segments) != 2 || script.Segments[1].Text != "Hai." {
		t.Fatalf("segments = %+v", script.Segments)
	}
	if script.Segments[0].Rendered == nil || script.Segments[1].Rendered != nil {
		t.Fatal("only the segment receiving the punctuation must be re-read")
	}
	_, err = svc.SaveScript(1, 1, ScriptEdit{Segments: []Segment{{ID: "s0001", Text: "…"}}})
	var state *StateError
	if !errors.As(err, &state) {
		t.Fatalf("punctuation-only script must be rejected with a message, got %v", err)
	}
}

// Thẻ audio trên điện thoại giữ tệp theo URL: version phải có khi chương đủ giọng
// và đổi khi một đoạn được đọc lại, để URL phát đổi theo.
func TestChapterAudioVersionFollowsRenderedContent(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	svc := newRenderTestService(t, &stoppedSynth{})
	now := time.Now()
	source := []SourceChapter{{Chapter: 1, Title: "Một", Text: "một", Completed: true}}
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusLocked, LockedAt: &now, SourceHash: SourceHash("một"),
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào."}}}); err != nil {
		t.Fatal(err)
	}
	version := func() string {
		t.Helper()
		view, err := svc.Volumes(context.Background(), source, 1)
		if err != nil {
			t.Fatal(err)
		}
		return view.Chapters[0].Audio.Version
	}
	if got := version(); got != "" {
		t.Fatalf("chapter without audio must have no version, got %q", got)
	}
	waitJobState(t, svc, mustEnqueue(t, svc, 1, 1), JobDone)
	first := version()
	if first == "" {
		t.Fatal("fully rendered chapter must expose a version")
	}
	if _, err := svc.Unlock(1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveScript(1, 1, ScriptEdit{Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào bạn."}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Lock(context.Background(), 1, 1, SourceHash("một")); err != nil {
		t.Fatal(err)
	}
	waitJobState(t, svc, mustEnqueue(t, svc, 1, 1), JobDone)
	if second := version(); second == "" || second == first {
		t.Fatalf("re-rendered chapter must get a new version: first=%q second=%q", first, second)
	}
}

func newRenderTestService(t *testing.T, synth Synthesizer) *Service {
	t.Helper()
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"
	svc, err := NewService(layout, synth)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	voiceID := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func mustEnqueue(t *testing.T, svc *Service, volume, chapter int) string {
	t.Helper()
	id, err := svc.Enqueue(volume, chapter)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func waitJobState(t *testing.T, svc *Service, id, want string) Job {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		job, _ := svc.queue.Find(id)
		if job.State == want {
			return job
		}
		if job.State == JobDone || job.State == JobError || time.Now().After(deadline) {
			t.Fatalf("job state = %s (%s), want %s", job.State, job.Error, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
