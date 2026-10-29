package audio

import (
	"context"
	"strings"
	"testing"
)

// countingSynth 记录真实合成次数，并给出可测的 RTF（§AK updateEtaSec 用）。
type countingSynth struct {
	calls int
	rtf   *float64
}

func (*countingSynth) Capabilities() Capabilities { return Capabilities{} }
func (s *countingSynth) Status(context.Context) Status {
	return Status{Engine: "fake", Model: "fake-v1", State: "ready", Ready: true, RTF: s.rtf}
}
func (*countingSynth) Voices(context.Context) ([]Voice, error) {
	return []Voice{{ID: "Mai Anh", Name: "Mai Anh", Source: "builtin"}}, nil
}
func (s *countingSynth) Synthesize(context.Context, SynthRequest) (Audio, error) {
	s.calls++
	return Audio{WAV: testWAV(48000, 1, 24000, 8000), SampleRate: 48000, DurationSec: 0.5}, nil
}
func (*countingSynth) AddVoice(context.Context, string, string) (Voice, error) {
	return Voice{}, ErrEngineMissing
}

func akTwoSegments(text string) []Segment {
	return []Segment{
		{ID: "s0001", Speaker: NarratorID, Text: "Câu mở đầu.", Source: "Câu mở đầu."},
		{ID: "s0002", Speaker: NarratorID, Text: text, Source: text},
	}
}

func akService(t *testing.T) (*Service, *countingSynth) {
	t.Helper()
	s := newTestService(t)
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	synth := &countingSynth{}
	s.synth = synth
	return s, synth
}

// holdWorker 拦住后台循环，让"刚排进队列"的状态可以稳定断言。
func holdWorker(s *Service) {
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
}

// releaseWorker 放行并等这一轮跑完。
func releaseWorker(s *Service) {
	s.runMu.Lock()
	s.running = false
	s.runMu.Unlock()
	s.kick()
	s.runWG.Wait()
}

// latestJob 取某章最新的任务（旧任务可能是上一轮已完成的）。
func latestJob(s *Service, chapter int) Job {
	var best Job
	for _, j := range s.Queue() {
		if j.Chapter == chapter && (best.ID == "" || j.CreatedAt.After(best.CreatedAt)) {
			best = j
		}
	}
	return best
}

// §AK: một chương đã có giọng bị sửa chính văn → job kind=update và chỉ đọc lại
// đúng đoạn đổi (đoạn không đổi giữ nguyên audio + id).
func TestUpdateRendersOnlyChangedSegments(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s, synth := akService(t)
	versions := map[string][]Segment{
		"v1": akTwoSegments("Câu thứ hai."),
		"v2": akTwoSegments("Câu thứ hai đã sửa."),
	}
	current := "v1"
	s.queuedComposer = func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
		segs := versions[current]
		return SourceChapter{Chapter: chapter, Text: current + " 正文", Completed: true},
			&fakeComposer{result: ComposeResult{Model: "test", Segments: segs}}, CharacterRef{}, nil
	}
	if _, err := s.QueueChapters([]SourceChapter{{Chapter: 1, Text: "v1 正文", Completed: true}}, 1, QueueInput{Chapters: []int{1}, Lock: true}); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	if synth.calls != 2 {
		t.Fatalf("create calls = %d, want 2", synth.calls)
	}
	first, _ := ReadScript(s.layout, 1, 1)
	before, _ := s.queue.Find(s.Queue()[0].ID)

	// 正文被改：一段原文不变，一段变了。
	current = "v2"
	source := []SourceChapter{{Chapter: 1, Text: "v2 正文", Completed: true}}
	holdWorker(s)
	result, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{1}, Lock: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chapters) != 1 || result.Chapters[0] != 1 {
		t.Fatalf("update selection: %+v", result)
	}
	if job := latestJob(s, 1); job.Kind != JobKindUpdate {
		t.Fatalf("edited chapter must queue as update: %+v", job)
	}
	before = latestJob(s, 1)
	if before.ID == "" {
		t.Fatal("missing update job id")
	}
	releaseWorker(s)
	if synth.calls != 3 {
		t.Fatalf("update re-read %d segments, want exactly 1 (total calls %d)", synth.calls-2, synth.calls)
	}
	if done := latestJob(s, 1); done.Segments != 1 || done.SegmentsDone != 1 {
		t.Fatalf("update progress must count only the changed segment: %+v", done)
	}
	updated, _ := ReadScript(s.layout, 1, 1)
	if updated.Segments[0].ID != first.Segments[0].ID || updated.Segments[0].Rendered == nil {
		t.Fatalf("unchanged segment lost its id or audio: %+v", updated.Segments[0])
	}
	if updated.Segments[1].Text != "Câu thứ hai đã sửa." {
		t.Fatalf("changed segment text: %+v", updated.Segments[1])
	}

	// redo: đọc lại từ đầu, kể cả đoạn không đổi.
	s.queuedComposer = func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
		return source[0], &fakeComposer{result: ComposeResult{Model: "test", Segments: versions["v2"]}}, CharacterRef{}, nil
	}
	holdWorker(s)
	if _, err := s.QueueChapters(source, 1, QueueInput{Redo: []int{1}}); err != nil {
		t.Fatal(err)
	}
	if job := latestJob(s, 1); job.Kind != JobKindRedo {
		t.Fatalf("redo kind: %+v", job)
	}
	releaseWorker(s)
	if synth.calls != 5 {
		t.Fatalf("redo must re-read every segment: calls %d, want 5", synth.calls)
	}
}

// §AK: một lần xếp hàng gồm create + update + redo; position đánh theo hàng chờ.
func TestQueueKindsAndPosition(t *testing.T) {
	s := newTestService(t)
	// Giữ worker để trạng thái chờ quan sát được ổn định.
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
	defer func() { s.runMu.Lock(); s.running = false; s.runMu.Unlock() }()

	// ch1: có bản đọc đã đọc hết nhưng lệch dấu vân tay → update.
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash("cũ"),
		RenderedFrom: &RenderSource{ScriptHash: "cũ"},
		Segments:     []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Một.", Rendered: &SegmentAudio{Key: "k", DurationSec: 1}}}}); err != nil {
		t.Fatal(err)
	}
	// ch2: có bản đọc, đủ giọng, khớp hôm nay → redo.
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 2, Status: StatusLocked, SourceHash: SourceHash("hai"),
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Hai.", Rendered: &SegmentAudio{Key: "k", DurationSec: 1}}}}); err != nil {
		t.Fatal(err)
	}
	source := []SourceChapter{
		{Chapter: 1, Text: "một", Completed: true},
		{Chapter: 2, Text: "hai", Completed: true},
		{Chapter: 3, Text: "ba", Completed: true},
	}
	result, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{1, 3}, Redo: []int{2}, Lock: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chapters) != 3 {
		t.Fatalf("chapters = %v, want 1,3,2", result.Chapters)
	}
	kinds := map[int]string{}
	positions := map[int]int{}
	for _, j := range s.Queue() {
		kinds[j.Chapter] = j.Kind
		positions[j.Chapter] = j.Position
		if j.State != JobQueued {
			t.Fatalf("chapter %d not waiting: %+v", j.Chapter, j)
		}
	}
	if kinds[1] != JobKindUpdate || kinds[2] != JobKindRedo || kinds[3] != JobKindCreate {
		t.Fatalf("kinds = %v", kinds)
	}
	if positions[1] != 1 || positions[3] != 2 || positions[2] != 3 {
		t.Fatalf("positions = %v, want 1:1 3:2 2:3", positions)
	}

	// "Làm trước" đẩy chương 2 lên đầu hàng chờ.
	if err := s.Prioritize(1, 2); err != nil {
		t.Fatal(err)
	}
	for _, j := range s.Queue() {
		if j.Chapter == 2 && j.Position != 1 {
			t.Fatalf("prioritize did not move position: %+v", s.Queue())
		}
	}

	// Không được vừa cập nhật vừa tạo lại cùng chương.
	if _, err := s.QueueChapters(source, 1, QueueInput{Chapters: []int{2}, Redo: []int{2}, Lock: true}); err == nil {
		t.Fatal("overlapping chapters/redo must be rejected")
	}
	// Tạo lại chương chưa có bản đọc cũng bị từ chối.
	if _, err := s.QueueChapters(source, 1, QueueInput{Redo: []int{9}, Lock: true}); err == nil {
		t.Fatal("redo of an unknown chapter must be rejected")
	}
	// Position đếm riêng từng tập: tập 2 chờ 1 việc thì việc đó là "Tiếp theo".
	if _, err := s.QueueChapters(source, 2, QueueInput{Chapters: []int{1}, Lock: true}); err != nil {
		t.Fatal(err)
	}
	for _, j := range s.Queue() {
		if j.Volume == 2 && j.Chapter == 1 && j.Position != 1 {
			t.Fatalf("volume 2 position = %+v, want 1", j)
		}
	}
}

// §AK: AudioChapter.updateEtaSec ước tính "chỉ đọc lại đoạn đổi" bằng RTF thật.
func TestChapterUpdateEtaUsesMeasuredRTF(t *testing.T) {
	s := newTestService(t)
	rtf := 0.25
	s.synth = &countingSynth{rtf: &rtf}
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	script := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash("một"),
		Segments: []Segment{
			{ID: "s1", Speaker: NarratorID, Text: "Đoạn đã có giọng.", Rendered: &SegmentAudio{DurationSec: 4}},
			{ID: "s2", Speaker: NarratorID, Text: "Đoạn vừa sửa.", Rendered: &SegmentAudio{Key: "cũ", DurationSec: 6}},
			{ID: "s3", Speaker: NarratorID, Text: "Đoạn chưa đọc."},
		}}
	// 段 1 用"当前参数"算出真实键，才算 đã có giọng；段 2 故意留旧键。
	settings := s.Settings()
	cast, err := s.castForRender()
	if err != nil {
		t.Fatal(err)
	}
	voices, err := s.Voices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(context.Background())
	script.Segments[0].Rendered.Key = s.segmentKey(script.Segments[0], cast, voiceMap, settings, engine, model).Hash()
	if err := WriteScript(s.layout, script); err != nil {
		t.Fatal(err)
	}
	view, err := s.Volumes(context.Background(), []SourceChapter{{Chapter: 1, Text: "một"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	eta := view.Chapters[0].UpdateEtaSec
	if eta == nil {
		t.Fatal("missing updateEtaSec with a measured RTF")
	}
	// Đoạn 2 (6 giây có sẵn) + đoạn 3 (ước lượng từ chữ) × 0.25.
	pendingAudio := 6.0 + (&Script{Segments: []Segment{{Text: "Đoạn chưa đọc."}}}).EstimatedDuration()
	if want := pendingAudio * rtf; *eta <= 0 || *eta > want+0.001 || *eta < want-0.001 {
		t.Fatalf("updateEtaSec = %v, want %v", *eta, want)
	}

	// Chưa đo RTF → không bịa số.
	s.synth = &countingSynth{}
	view, err = s.Volumes(context.Background(), []SourceChapter{{Chapter: 1, Text: "một"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.Chapters[0].UpdateEtaSec != nil {
		t.Fatalf("unmeasured RTF must omit updateEtaSec: %v", *view.Chapters[0].UpdateEtaSec)
	}
}

// §AK: Kind/Position được lưu qua queue.json và Position tính lại khi đọc.
func TestQueueKindPositionSurviveReload(t *testing.T) {
	s := newTestService(t)
	s.runMu.Lock()
	s.running = true
	s.runMu.Unlock()
	id, _ := s.queue.Enqueue(1, 4)
	s.queue.Update(id, func(j *Job) { j.Kind = JobKindUpdate })
	id2, _ := s.queue.Enqueue(1, 5)
	s.queue.Update(id2, func(j *Job) { j.Kind = JobKindRedo })
	reloaded, err := LoadQueue(s.layout.queuePath())
	if err != nil {
		t.Fatal(err)
	}
	jobs := reloaded.Snapshot()
	if len(jobs) != 2 || jobs[0].Kind != JobKindUpdate || jobs[1].Kind != JobKindRedo {
		t.Fatalf("kinds lost on reload: %+v", jobs)
	}
	if jobs[0].Position != 1 || jobs[1].Position != 2 {
		t.Fatalf("positions: %+v", jobs)
	}
	s.runMu.Lock()
	s.running = false
	s.runMu.Unlock()
}

func TestQueueRedoErrorText(t *testing.T) {
	s := newTestService(t)
	_, err := s.QueueChapters([]SourceChapter{{Chapter: 2, Text: "hai", Completed: true}}, 1, QueueInput{Redo: []int{2}, Lock: true})
	if err == nil || !strings.Contains(err.Error(), "bản đọc") {
		t.Fatalf("redo without script error = %v", err)
	}
}
