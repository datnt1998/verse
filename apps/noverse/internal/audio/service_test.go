package audio

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeComposer 是"AI soạn bản đọc"的替身：返回固定段落，记录输入。
type fakeComposer struct {
	mu     sync.Mutex
	result ComposeResult
	err    error
	inputs []ComposeInput
}

func (f *fakeComposer) Compose(_ context.Context, in ComposeInput) (ComposeResult, error) {
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	f.mu.Unlock()
	if f.err != nil {
		return ComposeResult{}, f.err
	}
	return f.result, nil
}

// fakeChatModel 是 agentcore.ChatModel 的替身：返回固定 JSON 回复。
type fakeChatModel struct {
	mu     sync.Mutex
	reply  string
	err    error
	prompt string
	user   string
}

func (m *fakeChatModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range messages {
		for _, b := range msg.Content {
			if msg.Role == llmtypes.RoleSystem {
				m.prompt += b.Text
			} else {
				m.user += b.Text
			}
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:    llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.TextBlock(m.reply)},
	}}, nil
}

func (m *fakeChatModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	ch := make(chan llmtypes.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *fakeChatModel) SupportsTools() bool { return false }

func (f *fakeComposer) lastInput() ComposeInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.inputs) == 0 {
		return ComposeInput{}
	}
	return f.inputs[len(f.inputs)-1]
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"
	svc, err := NewService(layout, NullSynth{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}

// waitScript 轮询等待某章 bản đọc落盘。
func waitScript(t *testing.T, svc *Service, volume, chapter int) *Script {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s, err := ReadScript(svc.Layout(), volume, chapter); err == nil && s != nil {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("script %d/%d never appeared", volume, chapter)
	return nil
}

func TestComposeProducesNormalizedScript(t *testing.T) {
	svc := newTestService(t)
	settings := svc.Settings()
	settings.AutoOnLock = false
	if _, err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	model := &fakeChatModel{reply: `{"segments":[
		{"speaker":"Lâm Tri Hạ","text":"Tôi chờ 90 giây.","tone":"Lạnh","pauseMs":200},
		{"speaker":"dan","text":"Cô ta cười.","pauseMs":0},
		{"speaker":"Người lạ","text":"Ai đó nói.","guess":[{"who":"Lâm Tri Hạ","p":0.6},{"who":"dan","p":0.3}]}
	]}`}
	composer := &ChatComposer{Model: model, ID: "fake/model"}
	source := []SourceChapter{{Chapter: 1, Title: "Mở màn", Text: "Nội dung chương một.", Completed: true}}
	ref := CharacterRef{Characters: []Character{{Name: "Lâm Tri Hạ", Aliases: []string{"Hạ"}}}}
	if err := svc.Compose(source, 1, []int{1}, composer, ref); err != nil {
		t.Fatal(err)
	}
	script := waitScript(t, svc, 1, 1)
	if script.Status != StatusReview || script.SourceHash != SourceHash(source[0].Text) || script.Model != "fake/model" {
		t.Fatalf("script meta = %+v", script)
	}
	if len(script.Segments) != 3 {
		t.Fatalf("segments = %d", len(script.Segments))
	}
	// 名字解析成 id，数字被读成字，并记录变化。
	first := script.Segments[0]
	if first.Speaker != "lth" {
		t.Fatalf("speaker not resolved: %q", first.Speaker)
	}
	if first.Text != "Tôi chờ chín mươi giây." {
		t.Fatalf("text not normalized: %q", first.Text)
	}
	if len(first.Normalized) != 1 || first.Normalized[0].From != "90" {
		t.Fatalf("normalized changes = %+v", first.Normalized)
	}
	if first.Source != "Tôi chờ 90 giây." {
		t.Fatalf("source must be kept: %q", first.Source)
	}
	// 未知说话人：保留名字、打标记、保留概率建议。
	unknown := script.Segments[2]
	if unknown.Speaker != "Người lạ" || !hasFlag(unknown, FlagUnknownSpeaker) {
		t.Fatalf("unknown speaker = %+v", unknown)
	}
	if len(unknown.SpeakerGuess) != 2 || unknown.SpeakerGuess[0].P < unknown.SpeakerGuess[1].P {
		t.Fatalf("guesses = %+v", unknown.SpeakerGuess)
	}
	// 模型收到了角色清单与模式（拼在 prompt 里）。
	if !strings.Contains(model.user, "lth = Lâm Tri Hạ") || !strings.Contains(model.user, "Hạ") {
		t.Fatalf("speaker list missing from prompt: %q", model.user)
	}
	if !strings.Contains(model.prompt, "bản đọc") || strings.Contains(model.user, "TTS") {
		t.Fatalf("prompt language wrong: %q", model.prompt[:80])
	}
}

func hasFlag(seg Segment, flag string) bool {
	for _, f := range seg.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

func TestComposeFlagsLongSentenceAndEnglish(t *testing.T) {
	svc := newTestService(t)
	settings := svc.Settings()
	settings.AutoOnLock = false
	if _, err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("một hai ba bốn năm sáu bảy tám chín mười ", 4) // 40 词
	model := &fakeChatModel{reply: fmt.Sprintf(`{"segments":[{"speaker":"dan","text":%q,"pauseMs":0}]}`, long+" live stream.")}
	composer := &ChatComposer{Model: model, ID: "fake/model"}
	source := []SourceChapter{{Chapter: 2, Text: "x", Completed: true}}
	if err := svc.Compose(source, 1, []int{2}, composer, CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	script := waitScript(t, svc, 1, 2)
	seg := script.Segments[0]
	if !hasFlag(seg, FlagLongSentence) {
		t.Fatalf("long sentence not flagged: %+v", seg.Flags)
	}
	if !hasFlag(seg, FlagEnglish) {
		t.Fatalf("english words not flagged: %+v", seg.Flags)
	}
}

func TestComposeReportsErrorsAsEvents(t *testing.T) {
	svc := newTestService(t)
	ctx, cancel := contextWithCancel(t)
	defer cancel()
	events := svc.Hub().Subscribe(ctx)
	composer := &fakeComposer{err: context.DeadlineExceeded}
	source := []SourceChapter{{Chapter: 1, Text: "x", Completed: true}}
	if err := svc.Compose(source, 1, []int{1}, composer, CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == EventCompose && ev.Data["state"] == "error" {
				if ev.Data["error"] == "" {
					t.Fatal("error event must carry a reason")
				}
				return
			}
		case <-deadline:
			t.Fatal("no error event")
		}
	}
}

func TestComposeSkipsIncompleteChapter(t *testing.T) {
	svc := newTestService(t)
	composer := &fakeComposer{}
	source := []SourceChapter{{Chapter: 1, Text: "x", Completed: false}}
	if err := svc.Compose(source, 1, []int{1}, composer, CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if len(composer.inputs) != 0 {
		t.Fatal("incomplete chapter must not be composed")
	}
	if s, _ := ReadScript(svc.Layout(), 1, 1); s != nil {
		t.Fatal("no script should be written for an incomplete chapter")
	}
}

func TestLockUnlockAndSaveScript(t *testing.T) {
	svc := newTestService(t)
	settings := svc.Settings()
	settings.AutoOnLock = false
	if _, err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	script := &Script{
		Chapter: 1, Volume: 1, Status: StatusReview, SourceHash: SourceHash("abc"),
		Segments: []Segment{
			{ID: "s0001", Speaker: NarratorID, Text: "Một.", Rendered: &SegmentAudio{Key: "k1", DurationSec: 1.5}},
			{ID: "s0002", Speaker: NarratorID, Text: "Hai."},
		},
	}
	if err := WriteScript(svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Lock(context.Background(), 1, 1, SourceHash("abc")); err != nil {
		t.Fatal(err)
	}
	locked, _ := ReadScript(svc.Layout(), 1, 1)
	if locked.Status != StatusLocked || locked.LockedAt == nil {
		t.Fatalf("lock = %+v", locked)
	}
	if got := DeriveStatus(locked, false, SourceHash("changed")); got != StatusStale {
		t.Fatalf("status after text change = %q", got)
	}
	// 只改第二段：第一段的音频保留。
	edit := ScriptEdit{Segments: []Segment{
		{ID: "s0001", Speaker: NarratorID, Text: "Một."},
		{ID: "s0002", Speaker: NarratorID, Text: "Hai, đã sửa."},
	}}
	saved, err := svc.SaveScript(1, 1, edit)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Segments[0].Rendered == nil {
		t.Fatal("unchanged segment must keep its audio")
	}
	if saved.Segments[1].Rendered != nil {
		t.Fatal("edited segment must lose its audio")
	}
	if _, err := svc.Unlock(1, 1); err != nil {
		t.Fatal(err)
	}
	unlocked, _ := ReadScript(svc.Layout(), 1, 1)
	if unlocked.Status != StatusReview || unlocked.LockedAt != nil {
		t.Fatalf("unlock = %+v", unlocked)
	}
}

func TestRefreshMergesUnchangedAudio(t *testing.T) {
	svc := newTestService(t)
	settings := svc.Settings()
	settings.AutoOnLock = false
	if _, err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	old := &Script{
		Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash("bản cũ"),
		Segments: []Segment{
			{ID: "s0001", Speaker: NarratorID, Text: "Câu giữ nguyên.", Source: "Câu giữ nguyên.", Rendered: &SegmentAudio{Key: "keep", DurationSec: 2}},
			{ID: "s0002", Speaker: NarratorID, Text: "Câu sẽ đổi.", Source: "Câu sẽ đổi.", Rendered: &SegmentAudio{Key: "drop", DurationSec: 2}},
		},
	}
	now := time.Now()
	old.LockedAt = &now
	if err := WriteScript(svc.Layout(), old); err != nil {
		t.Fatal(err)
	}
	composer := &fakeComposer{result: ComposeResult{Segments: []Segment{
		{ID: "s0001", Speaker: NarratorID, Text: "Câu giữ nguyên."},
		{ID: "s0002", Speaker: NarratorID, Text: "Câu đã đổi."},
	}}}
	source := []SourceChapter{{Chapter: 1, Text: "bản mới", Completed: true}}
	if err := svc.Refresh(source, 1, 1, composer, CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var merged *Script
	for time.Now().Before(deadline) {
		if s, err := ReadScript(svc.Layout(), 1, 1); err == nil && s != nil && s.SourceHash == SourceHash("bản mới") {
			merged = s
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if merged == nil {
		t.Fatal("refresh did not rewrite the script")
	}
	if merged.Status != StatusLocked || merged.LockedAt == nil {
		t.Fatalf("refresh must keep the locked state: %+v", merged)
	}
	if merged.Segments[0].Rendered == nil || merged.Segments[0].Rendered.Key != "keep" {
		t.Fatalf("unchanged segment lost its audio: %+v", merged.Segments[0].Rendered)
	}
	if merged.Segments[1].Rendered != nil {
		t.Fatalf("changed segment kept stale audio: %+v", merged.Segments[1].Rendered)
	}
}

func TestRefreshNoopWhenNotStale(t *testing.T) {
	svc := newTestService(t)
	script := &Script{Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash("same"), Segments: []Segment{{ID: "s1", Text: "x"}}}
	if err := WriteScript(svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
	composer := &fakeComposer{}
	source := []SourceChapter{{Chapter: 1, Text: "same", Completed: true}}
	if err := svc.Refresh(source, 1, 1, composer, CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if len(composer.inputs) != 0 {
		t.Fatal("refresh must not call the model when nothing changed")
	}
}

func TestEnqueueRequiresLockedScript(t *testing.T) {
	svc := newTestService(t)
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusReview, Segments: []Segment{{ID: "s1", Text: "x"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Enqueue(1, 1); err == nil {
		t.Fatal("enqueue before lock must fail")
	}
	if _, err := svc.Enqueue(1, 5); err == nil {
		t.Fatal("enqueue without script must fail")
	}
}

func TestPreviewWithoutEngineReturnsClearError(t *testing.T) {
	svc := newTestService(t) // NullSynth
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusReview,
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "x"}}}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Preview(context.Background(), 1, 1, "s0001")
	if err == nil || !strings.Contains(err.Error(), "Bộ đọc") {
		t.Fatalf("preview error = %v", err)
	}
	ready := svc.Ready(context.Background())
	if ready.Ready || ready.Message == "" {
		t.Fatalf("ready = %+v", ready)
	}
}

func TestVolumesViewCountsAndStatuses(t *testing.T) {
	svc := newTestService(t)
	now := time.Now()
	script := &Script{Chapter: 1, Volume: 1, Status: StatusLocked, LockedAt: &now, SourceHash: SourceHash("một"),
		Segments: []Segment{
			{ID: "s1", Speaker: NarratorID, Text: "a", Rendered: &SegmentAudio{Key: "k", DurationSec: 2}, PauseMs: 500},
			{ID: "s2", Speaker: "lth", Text: "Mở email đi.", Flags: []string{FlagEnglish}},
		}}
	voiceID := "giong-nam"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Volumes(context.Background(), []SourceChapter{
		{Chapter: 1, Title: "Một", Text: "một", Completed: true},
		{Chapter: 2, Title: "Hai", Text: "hai", Completed: false},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chapters) != 2 {
		t.Fatalf("chapters = %d", len(view.Chapters))
	}
	first := view.Chapters[0]
	if first.Status != StatusLocked || first.Segments != 2 || first.Speakers != 2 || first.Issues != 1 {
		t.Fatalf("first = %+v", first)
	}
	if first.Audio == nil || first.Audio.SegmentsDone != 1 || first.Audio.Percent != 50 {
		t.Fatalf("audio = %+v", first.Audio)
	}
	if first.DurationSec != nil {
		t.Fatalf("duration must be nil until all segments exist: %v", *first.DurationSec)
	}
	if second := view.Chapters[1]; second.Status != StatusTodo || second.Audio != nil {
		t.Fatalf("second = %+v", second)
	}
}

// stoppedSynth 模拟"已装好但进程没跑"的引擎：Status 不是 Ready，但 Synthesize 会自动拉起。
type stoppedSynth struct{ calls int }

func (*stoppedSynth) Capabilities() Capabilities { return Capabilities{} }
func (s *stoppedSynth) Status(context.Context) Status {
	return Status{Engine: "vieneu", Model: "m", State: "stopped"}
}
func (s *stoppedSynth) Voices(context.Context) ([]Voice, error) {
	return []Voice{{ID: "Mai Anh", Name: "Mai Anh", Source: "builtin"}}, nil
}
func (s *stoppedSynth) Synthesize(context.Context, SynthRequest) (Audio, error) {
	s.calls++
	return Audio{WAV: testWAV(48000, 1, 24000, 8000), SampleRate: 48000, DurationSec: 0.5}, nil
}
func (s *stoppedSynth) AddVoice(context.Context, string, string) (Voice, error) {
	return Voice{}, ErrEngineMissing
}

func TestPreviewStartsStoppedEngine(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"
	synth := &stoppedSynth{}
	svc, err := NewService(layout, synth)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	voiceID := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(layout, &Script{Chapter: 1, Volume: 1, Status: StatusReview,
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào."}}}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Preview(context.Background(), 1, 1, "s0001")
	if err != nil {
		t.Fatalf("stopped engine must still synthesize: %v", err)
	}
	if synth.calls != 1 || res.DurationSec <= 0 {
		t.Fatalf("calls=%d result=%+v", synth.calls, res)
	}
}

func TestVoiceFlagsFollowCurrentCast(t *testing.T) {
	svc := newTestService(t)
	now := time.Now()
	if err := WriteScript(svc.Layout(), &Script{Chapter: 1, Volume: 1, Status: StatusLocked, LockedAt: &now, SourceHash: SourceHash("một"),
		Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "a", Flags: []string{FlagNoVoice}}}}); err != nil {
		t.Fatal(err)
	}
	voiceID := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	volume, err := svc.Volumes(context.Background(), []SourceChapter{{Chapter: 1, Title: "Một", Text: "một", Completed: true}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if volume.Chapters[0].Issues != 0 {
		t.Fatalf("voice chosen after compose must not count no_voice: %+v", volume.Chapters[0])
	}
	view, err := svc.ScriptView(context.Background(), 1, 1, SourceHash("một"))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Segments[0].Flags) != 0 {
		t.Fatalf("script view keeps stale flags: %v", view.Segments[0].Flags)
	}
}

func TestRenderPersistsCurrentVoiceFlags(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"
	svc, err := NewService(layout, &stoppedSynth{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	voiceID := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := WriteScript(layout, &Script{Chapter: 1, Volume: 1, Status: StatusLocked, LockedAt: &now,
		Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào, đây là email.", Flags: []string{FlagEnglish, FlagNoVoice}}}}); err != nil {
		t.Fatal(err)
	}
	id, err := svc.Enqueue(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		job, _ := svc.queue.Find(id)
		if job.State == JobDone {
			break
		}
		if job.State == JobError || time.Now().After(deadline) {
			t.Fatalf("render did not finish: %+v", job)
		}
		time.Sleep(20 * time.Millisecond)
	}
	script, err := ReadScript(layout, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	seg := script.Segments[0]
	if seg.Rendered == nil || hasFlag(seg, FlagNoVoice) || !hasFlag(seg, FlagEnglish) {
		t.Fatalf("rendered segment flags = %v rendered=%+v", seg.Flags, seg.Rendered)
	}
}

func TestStatusIdentIgnoresRuntimeModelLabel(t *testing.T) {
	stopped := Status{Engine: "vieneu", Model: "vieneu-v3-turbo", ModelID: "vieneu-v3-turbo", State: "stopped"}
	running := Status{Engine: "vieneu", Model: "vieneu-v3-turbo (48000 Hz) · int8", ModelID: "vieneu-v3-turbo", State: "ready", Ready: true}
	e1, m1 := statusIdent(stopped)
	e2, m2 := statusIdent(running)
	if e1 != e2 || m1 != m2 {
		t.Fatalf("cache identity must not change when the engine starts: %s/%s vs %s/%s", e1, m1, e2, m2)
	}
	if _, m := statusIdent(Status{Engine: "x", Model: "only-label"}); m != "only-label" {
		t.Fatalf("fallback model = %q", m)
	}
	view := (&Service{synth: &stoppedSynth{}}).Ready(context.Background())
	if FFmpegAvailable() && !view.Ready {
		t.Fatalf("installed-but-stopped engine must report ready: %+v", view)
	}
}

type deadlineChatModel struct {
	fakeChatModel
	deadline time.Time
}

func (m *deadlineChatModel) Generate(ctx context.Context, _ []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.deadline, _ = ctx.Deadline()
	return nil, context.DeadlineExceeded
}

func TestChatComposerProviderFailures(t *testing.T) {
	composer := &ChatComposer{Model: &fakeChatModel{err: fmt.Errorf("dial tcp: connection refused")}}
	_, err := composer.Compose(context.Background(), ComposeInput{Text: "Xin chào."})
	if err == nil || !strings.Contains(err.Error(), "Không kết nối được nhà cung cấp AI") || !strings.Contains(err.Error(), "Cài đặt › Nhà cung cấp") {
		t.Fatalf("provider failure must explain recovery in Vietnamese: %v", err)
	}
	model := &deadlineChatModel{}
	composer.Model = model
	start := time.Now()
	_, err = composer.Compose(context.Background(), ComposeInput{Text: "Xin chào."})
	if model.deadline.IsZero() || model.deadline.After(start.Add(2*time.Minute+time.Second)) {
		t.Fatalf("composition must bound the model request: %v", model.deadline)
	}
	if err == nil || !strings.Contains(err.Error(), "phản hồi quá lâu") {
		t.Fatalf("timeout must explain recovery: %v", err)
	}
}
