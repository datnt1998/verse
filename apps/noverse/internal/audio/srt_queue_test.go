package audio

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSegmentTimelineAndSRT(t *testing.T) {
	segs := []TimedSegment{
		{Text: "Câu một.", DurationSec: 1.5, PauseMs: 500},
		{Text: "Câu hai.", DurationSec: 2.0, PauseMs: 0},
	}
	items := SegmentTimeline(0, segs)
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].StartMs != 0 || items[0].EndMs != 1500 {
		t.Fatalf("item0 = %d..%d", items[0].StartMs, items[0].EndMs)
	}
	// 停顿算在上一段之后：第二段从 1500+500 开始。
	if items[1].StartMs != 2000 || items[1].EndMs != 4000 {
		t.Fatalf("item1 = %d..%d", items[1].StartMs, items[1].EndMs)
	}
	srt := BuildSRT(items)
	want := "1\n00:00:00,000 --> 00:00:01,500\nCâu một.\n\n2\n00:00:02,000 --> 00:00:04,000\nCâu hai.\n"
	if srt != want {
		t.Fatalf("srt =\n%q\nwant\n%q", srt, want)
	}
}

func TestSRTTimeFormatsHours(t *testing.T) {
	cases := map[int64]string{
		0:         "00:00:00,000",
		999:       "00:00:00,999",
		61_500:    "00:01:01,500",
		3_600_000: "01:00:00,000",
		7_384_031: "02:03:04,031",
	}
	for ms, want := range cases {
		if got := srtTime(ms); got != want {
			t.Fatalf("srtTime(%d) = %q, want %q", ms, got, want)
		}
	}
}

func TestFormatMarkTimeAndDuration(t *testing.T) {
	if got := FormatMarkTime(12); got != "00:12" {
		t.Fatalf("FormatMarkTime(12) = %q", got)
	}
	if got := FormatMarkTime(3723); got != "1:02:03" {
		t.Fatalf("FormatMarkTime(3723) = %q", got)
	}
	if got := FormatDurationVN(8671); got != "2 giờ 24 phút 31 giây" {
		t.Fatalf("FormatDurationVN = %q", got)
	}
}

func TestQueuePersistenceSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/queue.json"
	q, err := LoadQueue(path)
	if err != nil {
		t.Fatal(err)
	}
	id, job := q.Enqueue(1, 3)
	if job.State != JobQueued || id == "" {
		t.Fatalf("enqueue = %+v", job)
	}
	q.Update(id, func(j *Job) {
		j.State = JobRendering
		j.Segments = 46
		j.SegmentsDone = 12
		j.Speaker = "dan"
		j.CurrentText = "đang đọc"
	})
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("queue file not written: %v", err)
	}

	reloaded, err := LoadQueue(path)
	if err != nil {
		t.Fatal(err)
	}
	jobs := reloaded.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("want 1 job after restart, got %d", len(jobs))
	}
	if jobs[0].State != JobQueued {
		t.Fatalf("rendering job must fall back to queued after restart, got %q", jobs[0].State)
	}
	if jobs[0].SegmentsDone != 12 || jobs[0].Segments != 46 {
		t.Fatalf("progress lost: %+v", jobs[0])
	}
}

func TestQueueEnqueueReusesOpenJob(t *testing.T) {
	q, err := LoadQueue(t.TempDir() + "/queue.json")
	if err != nil {
		t.Fatal(err)
	}
	id1, _ := q.Enqueue(1, 2)
	id2, _ := q.Enqueue(1, 2)
	if id1 != id2 {
		t.Fatalf("open job should be reused: %q vs %q", id1, id2)
	}
	if got := len(q.Snapshot()); got != 1 {
		t.Fatalf("want 1 job, got %d", got)
	}
	q.Update(id1, func(j *Job) { j.State = JobDone })
	id3, _ := q.Enqueue(1, 2)
	if id3 == id1 {
		t.Fatalf("done job should not be reused")
	}
	if got := len(q.Snapshot()); got != 2 {
		t.Fatalf("want 2 jobs, got %d", got)
	}
}

func TestQueueCorruptFileFailsLoud(t *testing.T) {
	path := t.TempDir() + "/queue.json"
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadQueue(path); err == nil {
		t.Fatal("corrupt queue.json must be an error, not a silent reset")
	}
}

func TestHubBroadcastsAndDropsSlowSubscribers(t *testing.T) {
	hub := NewHub()
	ctx, cancel := contextWithCancel(t)
	defer cancel()
	ch := hub.Subscribe(ctx)
	ev := NewEvent(EventRender, map[string]any{"chapter": 3})
	if ev.Category != EventCategory {
		t.Fatalf("category = %q", ev.Category)
	}
	hub.Publish(ev)
	select {
	case got := <-ch:
		if got.Type != EventRender || got.Data["chapter"] != 3 {
			t.Fatalf("got %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}
	// 塞满缓冲不阻塞发布。
	for range 100 {
		hub.Publish(ev)
	}
	hub.Close()
	if _, open := <-ch; open {
		// 通道已关闭（可能还剩缓冲值），读到 close 才是正确信号。
		for range ch {
		}
	}
}

func TestWriteFileAtomicReplaces(t *testing.T) {
	path := t.TempDir() + "/x.json"
	if err := writeFileAtomic(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("got %q", data)
	}
}

func TestExportMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	layout := Layout{ExportsDir: dir}
	volumeDir := layout.VolumeExportDir(1)
	if err := os.MkdirAll(volumeDir+"/chuong", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(volumeDir+"/Tập 1.mp3", []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(volumeDir+"/Tập 1.srt", []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := exportMeta{ExportedAt: time.Now(), DurationSec: 100, Voices: 6, ChapterCount: 8, Format: "mp3",
		ChapterMarks: []ChapterMark{{AtSec: 0, Label: "Mở đầu"}}}
	data, _ := json.Marshal(meta)
	if err := writeFileAtomic(volumeDir+"/"+exportMetaFile, data); err != nil {
		t.Fatal(err)
	}
	svc := &Service{layout: layout}
	info, err := svc.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	if info.ExportedAt == nil || info.ChapterCount != 8 || info.Voices != 6 {
		t.Fatalf("meta lost: %+v", info)
	}
	if len(info.Files) != 2 {
		t.Fatalf("files = %d (hidden meta must be skipped): %+v", len(info.Files), info.Files)
	}
	var names []string
	for _, f := range info.Files {
		names = append(names, f.Path)
		if f.Format != "audio" && f.Format != "other" {
			t.Fatalf("format %q", f.Format)
		}
	}
	if !strings.Contains(strings.Join(names, ","), "Tập 1.mp3") {
		t.Fatalf("mp3 missing from listing: %v", names)
	}
}

func TestRenderMarksAndSanitize(t *testing.T) {
	out := renderMarks([]ChapterMark{{AtSec: 0, Label: "Mở đầu"}, {AtSec: 12, Label: "Chương 1 · Vai"}})
	if out != "00:00 Mở đầu\n00:12 Chương 1 · Vai\n" {
		t.Fatalf("marks = %q", out)
	}
	if got := sanitizeFileName(`a/b:c*d?.mp3`); got != "a b c d.mp3" {
		t.Fatalf("sanitize = %q", got)
	}
	if got := chapterFileSlug(1, "Vai của tôi là đổ canh"); got != "01-vai-cua-toi-la-do-canh" {
		t.Fatalf("slug = %q", got)
	}
}

func TestPickMusic(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"outro-ket.mp3", "intro-dit-dang.mp3", "note.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	intro, outro, err := pickMusic(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(intro, "intro-dit-dang.mp3") || !strings.HasSuffix(outro, "outro-ket.mp3") {
		t.Fatalf("intro=%q outro=%q", intro, outro)
	}
	if i, o, err := pickMusic(t.TempDir()); err != nil || i != "" || o != "" {
		t.Fatalf("missing music dir should be empty, got %q %q %v", i, o, err)
	}
}
