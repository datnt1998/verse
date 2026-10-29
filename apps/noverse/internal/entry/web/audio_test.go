package web

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/identity"
)

// fakeSynth 是测试用的读音引擎：任何文本都返回 0.3 秒 440 Hz，并记录请求。
type fakeSynth struct {
	mu    sync.Mutex
	calls []audio.SynthRequest
	fail  error
}

func (*fakeSynth) Capabilities() audio.Capabilities { return audio.Capabilities{} }
func (f *fakeSynth) Status(context.Context) audio.Status {
	return audio.Status{Ready: true, Engine: "fake", Model: "v3", Device: "cpu", State: "ready"}
}

func (f *fakeSynth) Voices(context.Context) ([]audio.Voice, error) {
	return []audio.Voice{{ID: "giong-nu", Name: "Nữ trong", Gender: "female", Source: "builtin", DurationSec: 12}}, nil
}

func (f *fakeSynth) Synthesize(ctx context.Context, req audio.SynthRequest) (audio.Audio, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	fail := f.fail
	f.mu.Unlock()
	if fail != nil {
		return audio.Audio{}, fail
	}
	if err := ctx.Err(); err != nil {
		return audio.Audio{}, err
	}
	wav := sineWAV(48000, 0.3, 440)
	return audio.Audio{WAV: wav, SampleRate: 48000, DurationSec: audio.WAVDurationSec(wav)}, nil
}

func (f *fakeSynth) AddVoice(context.Context, string, string) (audio.Voice, error) {
	return audio.Voice{ID: "clone", Name: "clone", Source: "clone"}, nil
}

func (f *fakeSynth) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// sineWAV 生成 s16 单声道正弦波（测试音频）。
func sineWAV(sampleRate int, seconds, freq float64) []byte {
	frames := int(float64(sampleRate) * seconds)
	var buf bytes.Buffer
	dataLen := frames * 2
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVEfmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataLen))
	for i := range frames {
		v := int16(12000 * math.Sin(2*math.Pi*freq*float64(i)/float64(sampleRate)))
		binary.Write(&buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

// audioFixture 是一个有 2 章已验收故事 + 假引擎的网关。
type audioFixture struct {
	tg    *testGateway
	admin string
	st    library.Story
	svc   *audio.Service
	synth *fakeSynth
	texts map[int]string
}

func newAudioFixture(t *testing.T) *audioFixture {
	t.Helper()
	tg := newTestGateway(t)
	storyDir := mkdirAll(t, tg.root, "proj", "kich_ban", "01-tra-xanh")
	s := store.NewStore(filepath.Join(storyDir, "output", "novel"))
	if err := s.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := s.Book.Save(domain.BookMetadata{Title: "Trà Xanh Cướp Kịch Bản", Synopsis: "tóm tắt"}); err != nil {
		t.Fatal(err)
	}
	outline := []domain.OutlineEntry{
		{Chapter: 1, Title: "Vai của tôi là đổ canh", CoreEvent: "mở màn"},
		{Chapter: 2, Title: "Lên sóng", CoreEvent: "cao trào"},
	}
	if err := s.Outline.SaveOutline(outline); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(len(outline)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	texts := map[int]string{
		1: "Trên tivi, cô ta nói về đứa con tinh thần suốt ba năm. Tôi chờ 90 giây rồi tắt.",
		2: "Tôi bước lên sóng. 45% khán giả đã chờ sẵn, và 3-5 người gọi tới.",
	}
	for ch := 1; ch <= 2; ch++ {
		if err := s.Drafts.SaveFinalChapter(ch, texts[ch]); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 10, "cliff", "main"); err != nil {
			t.Fatal(err)
		}
	}
	st := library.Story{
		Project: "proj", Story: "01-tra-xanh", ID: "proj/01-tra-xanh",
		Dir: storyDir, ProjectDir: filepath.Join(tg.root, "proj"), Root: tg.root,
	}

	synth := &fakeSynth{}
	old := audioSynthesizer
	audioSynthesizer = func() audio.Synthesizer { return synth }
	t.Cleanup(func() { audioSynthesizer = old })
	if cached, ok := audioServices.Load(storyDir); ok {
		cached.(audioEntry).svc.Close()
		audioServices.Delete(storyDir)
	}
	svc, err := tg.g.fileAudioService(st)
	if err != nil {
		t.Fatalf("audio service: %v", err)
	}
	t.Cleanup(func() {
		svc.Close()
		audioServices.Delete(storyDir)
	})

	f := &audioFixture{tg: tg, admin: tg.login("admin", "admin-pass"), st: st, svc: svc, synth: synth, texts: texts}
	// 叙述者有声音（否则生成会被 409 拦下）。
	cast := audio.Cast{
		Mode: audio.ModeCast,
		Single: audio.SingleCast{
			DialogueShift: true, KeepTones: true,
		},
		Speakers: []audio.Speaker{
			{ID: audio.NarratorID, Name: "Dẫn truyện", Role: audio.RoleNarrator, VoiceID: strPtr("giong-nu")},
			{ID: "lth", Name: "Lâm Tri Hạ", Role: audio.RoleMain, VoiceID: strPtr("giong-nu")},
		},
	}
	if err := audio.WriteCast(svc.Layout(), &cast); err != nil {
		t.Fatal(err)
	}
	return f
}

func strPtr(v string) *string { return &v }

func (f *audioFixture) seedScript(t *testing.T, chapter int, status string) {
	t.Helper()
	segments := []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Trên tivi, cô ta nói về đứa con tinh thần.", Source: "Trên tivi…", PauseMs: 200, Tone: ""},
		{ID: "s0002", Speaker: "lth", Text: "Tôi chờ chín mươi giây rồi tắt.", Source: "Tôi chờ 90 giây rồi tắt.", PauseMs: 0},
	}
	script := &audio.Script{
		Chapter: chapter, Volume: 1, Status: status,
		SourceHash: audio.SourceHash(f.texts[chapter]), Segments: segments,
	}
	if status == audio.StatusLocked {
		now := time.Now()
		script.LockedAt = &now
	}
	if err := audio.WriteScript(f.svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
}

func (f *audioFixture) path(tail string) string {
	return "/api/projects/proj/stories/01-tra-xanh" + tail
}

func (f *audioFixture) do(method, tail, body string) *httptest.ResponseRecorder {
	return f.tg.do(method, f.path(tail), f.admin, body)
}

func TestGatewayListResponseEnvelopes(t *testing.T) {
	f := newAudioFixture(t)
	for _, tc := range []struct {
		path string
		keys []string
	}{
		{"/chapters", []string{"chapters"}},
		{"/characters", []string{"characters", "supporting"}},
		{"/foreshadow", []string{"entries"}},
		{"/chapters/1/revisions", []string{"revisions"}},
		{"/chapters/1/comments", []string{"comments"}},
		{"/exports", []string{"files", "matrix"}},
		{"/audio/queue", []string{"jobs"}},
		{"/audio/flags", []string{"flags"}},
		{"/audio/volumes/1", []string{"chapters"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := f.do(http.MethodGet, tc.path, "")
			if res.Code != http.StatusOK {
				t.Fatalf("GET %s: %d %s", tc.path, res.Code, res.Body.String())
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(res.Body.Bytes(), &object); err != nil {
				t.Fatalf("expected object envelope: %v; body %s", err, res.Body.String())
			}
			for _, key := range tc.keys {
				raw, ok := object[key]
				if !ok || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
					t.Errorf("%q must be a JSON array, not absent/null: %s", key, res.Body.String())
				}
			}
		})
	}
}

func TestTtsEngineResponseEnvelope(t *testing.T) {
	g := &Gateway{}
	res := httptest.NewRecorder()
	g.ttsEngines(res, httptest.NewRequest(http.MethodGet, "/api/tts/engines", nil))
	var object map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &object); err != nil {
		t.Fatalf("expected engines object envelope: %v; body %s", err, res.Body.String())
	}
	if res.Code != http.StatusOK || !bytes.HasPrefix(bytes.TrimSpace(object["engines"]), []byte("[")) {
		t.Fatalf("expected engines array in object envelope: %d %s", res.Code, res.Body.String())
	}
	var active string
	if err := json.Unmarshal(object["active"], &active); err != nil {
		t.Fatalf("expected active engine string: %v; body %s", err, res.Body.String())
	}
}

func TestAudioScriptArrayFields(t *testing.T) {
	f := newAudioFixture(t)
	f.seedScript(t, 1, audio.StatusReview)
	res := f.do(http.MethodGet, "/audio/volumes/1/scripts/1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("script: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Segments []map[string]json.RawMessage `json:"segments"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Segments) == 0 {
		t.Fatal("expected seeded script segments")
	}
	for i, segment := range body.Segments {
		for _, key := range []string{"normalized", "flags"} {
			if !bytes.HasPrefix(bytes.TrimSpace(segment[key]), []byte("[")) {
				t.Errorf("segments[%d].%s must be an array, not absent/null: %s", i, key, res.Body.String())
			}
		}
	}
}

// waitJob 等队列里出现指定章节且状态到达 want（超时失败）。
func (f *audioFixture) waitJob(t *testing.T, chapter int, want ...string) audio.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range f.svc.Queue() {
			if j.Chapter != chapter {
				continue
			}
			for _, w := range want {
				if j.State == w {
					return j
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job for chapter %d never reached %v: %+v", chapter, want, f.svc.Queue())
	return audio.Job{}
}

// awaitExportDone POSTs an export (§AE: 202 + jobId, không giữ request) rồi
// đợi việc nền xong; trả request ở trạng thái cuối.
func (f *audioFixture) awaitExportDone(t *testing.T, body string) audio.ExportRequest {
	t.Helper()
	w := f.do("POST", "/audio/volumes/1/exports", body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("export POST: %d %s", w.Code, w.Body.String())
	}
	ack := decodeJSONBody(t, w)
	if ack["jobId"] == "" {
		t.Fatalf("missing jobId: %s", w.Body.String())
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, r := range f.svc.ExportRequests() {
			if r.State == audio.JobDone || r.State == audio.JobError {
				return r
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("export job never finished: %+v", f.svc.ExportRequests())
	return audio.ExportRequest{}
}

// waitExportState 等某卷的导出请求到指定状态。
func (f *audioFixture) waitExportState(t *testing.T, states ...string) audio.ExportRequest {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, r := range f.svc.ExportRequests() {
			for _, want := range states {
				if r.State == want {
					return r
				}
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("export request never reached %v: %+v", states, f.svc.ExportRequests())
	return audio.ExportRequest{}
}

// waitExportInfo 等 GET /exports 出现"đã xuất"信息（ExportInfo.Files 非空）。
func waitExportInfo(t *testing.T, f *audioFixture, volume int) audio.ExportInfo {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		w := f.do("GET", fmt.Sprintf("/audio/volumes/%d/exports", volume), "")
		if w.Code == http.StatusOK {
			var info audio.ExportInfo
			if json.Unmarshal(w.Body.Bytes(), &info) == nil && len(info.Files) > 0 {
				return info
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("export info never populated for volume %d", volume)
	return audio.ExportInfo{}
}

func TestAudioReadyAndVolumes(t *testing.T) {
	f := newAudioFixture(t)

	w := f.do("GET", "/audio/ready", "")
	if w.Code != http.StatusOK {
		t.Fatalf("ready: %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	engine := body["engine"].(map[string]any)
	if body["ready"] != true || engine["engine"] != "fake" {
		t.Fatalf("ready body = %v", body)
	}

	w = f.do("GET", "/audio/volumes/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("volumes: %d %s", w.Code, w.Body.String())
	}
	var vol audio.VolumeView
	if err := json.Unmarshal(w.Body.Bytes(), &vol); err != nil {
		t.Fatal(err)
	}
	if len(vol.Chapters) != 2 {
		t.Fatalf("chapters = %d: %+v", len(vol.Chapters), vol.Chapters)
	}
	for _, ch := range vol.Chapters {
		if ch.Status != audio.StatusTodo || ch.Segments != 0 {
			t.Fatalf("chapter %d should be todo: %+v", ch.Chapter, ch)
		}
	}

	if w := f.do("GET", "/audio/volumes/9", ""); w.Code != http.StatusConflict {
		t.Fatalf("unknown volume: %d %s", w.Code, w.Body.String())
	}
	if w := f.do("GET", "/audio/nope", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown audio path: %d", w.Code)
	}
}

func TestAudioScriptLockRenderAndChapterFile(t *testing.T) {
	f := newAudioFixture(t)
	f.seedScript(t, 1, audio.StatusReview)

	w := f.do("GET", "/audio/volumes/1/scripts/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("script: %d %s", w.Code, w.Body.String())
	}
	var view audio.ScriptView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Status != audio.StatusReview || len(view.Segments) != 2 {
		t.Fatalf("script view = %+v", view)
	}
	if view.Segments[0].Audio == nil || view.Segments[0].Audio.State != "none" {
		t.Fatalf("segment audio state = %+v", view.Segments[0].Audio)
	}
	if !strings.Contains(view.Segments[0].Audio.URL, "/audio/segments/1-s0001/audio") {
		t.Fatalf("segment url = %q", view.Segments[0].Audio.URL)
	}

	// 未 chốt 就想生成 → 409。
	if w := f.do("POST", "/audio/volumes/1/chapters/1/render", "{}"); w.Code != http.StatusConflict {
		t.Fatalf("render before lock: %d %s", w.Code, w.Body.String())
	}

	if w := f.do("POST", "/audio/volumes/1/scripts/1/lock", "{}"); w.Code != http.StatusOK {
		t.Fatalf("lock: %d %s", w.Code, w.Body.String())
	}
	// autoOnLock 缺省开启：锁完自动排队并跑完（假引擎很快）。
	job := f.waitJob(t, 1, audio.JobDone, audio.JobError)
	if job.State != audio.JobDone {
		t.Fatalf("render job = %+v", job)
	}
	if job.SegmentsDone != 2 || job.Percent != 100 {
		t.Fatalf("job progress = %+v", job)
	}

	// 锁定后状态为 locked，且两段都有音频。
	w = f.do("GET", "/audio/volumes/1", "")
	var vol audio.VolumeView
	_ = json.Unmarshal(w.Body.Bytes(), &vol)
	ch := vol.Chapters[0]
	if ch.Status != audio.StatusLocked || ch.Audio == nil || ch.Audio.SegmentsDone != 2 || ch.DurationSec == nil {
		t.Fatalf("chapter after render = %+v", ch)
	}

	w = f.do("GET", "/audio/volumes/1/scripts/1", "")
	var locked audio.ScriptView
	_ = json.Unmarshal(w.Body.Bytes(), &locked)
	for _, seg := range locked.Segments {
		if seg.Audio == nil || seg.Audio.State != "ready" || seg.Audio.DurationSec <= 0 {
			t.Fatalf("segment not ready: %+v", seg.Audio)
		}
	}

	// 整章 mp3 支持 Range（iOS Safari 播放在线音频必须有 206 + Accept-Ranges）。
	w = f.do("GET", "/audio/volumes/1/chapters/1.mp3", "")
	if w.Code != http.StatusOK {
		t.Fatalf("chapter mp3: %d %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "audio") && ct != "application/octet-stream" {
		t.Fatalf("content type = %q", ct)
	}
	r := httptest.NewRequest("GET", f.path("/audio/volumes/1/chapters/1.mp3"), nil)
	r.Header.Set("Range", "bytes=0-99")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
	rw := httptest.NewRecorder()
	f.tg.g.ServeHTTP(rw, r)
	if rw.Code != http.StatusPartialContent {
		t.Fatalf("range request: %d", rw.Code)
	}
	if rw.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("Accept-Ranges = %q", rw.Header().Get("Accept-Ranges"))
	}
	if cr := rw.Header().Get("Content-Range"); !strings.HasPrefix(cr, "bytes 0-99/") {
		t.Fatalf("Content-Range = %q", cr)
	}
	if rw.Body.Len() != 100 {
		t.Fatalf("range length = %d", rw.Body.Len())
	}

	// 波形峰值。
	w = f.do("GET", "/audio/volumes/1/chapters/1/peaks", "")
	if w.Code != http.StatusOK {
		t.Fatalf("peaks: %d %s", w.Code, w.Body.String())
	}
	var peaks struct {
		Peaks       []float64 `json:"peaks"`
		DurationSec float64   `json:"durationSec"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &peaks); err != nil {
		t.Fatal(err)
	}
	if len(peaks.Peaks) == 0 || peaks.DurationSec <= 0 {
		t.Fatalf("peaks = %+v", peaks)
	}

	// 解锁后回到 review。
	if w := f.do("POST", "/audio/volumes/1/scripts/1/unlock", "{}"); w.Code != http.StatusOK {
		t.Fatalf("unlock: %d", w.Code)
	}
	w = f.do("GET", "/audio/volumes/1/scripts/1", "")
	var unlocked audio.ScriptView
	_ = json.Unmarshal(w.Body.Bytes(), &unlocked)
	if unlocked.Status != audio.StatusReview || unlocked.LockedAt != nil {
		t.Fatalf("unlock = %+v", unlocked)
	}
}

func TestAudioSegmentPreviewRerenderAccept(t *testing.T) {
	f := newAudioFixture(t)
	f.seedScript(t, 1, audio.StatusReview)

	w := f.do("POST", "/audio/segments/1-s0001/preview", "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	var prev audio.PreviewResult
	if err := json.Unmarshal(w.Body.Bytes(), &prev); err != nil {
		t.Fatal(err)
	}
	if prev.Cached || prev.DurationSec <= 0 || prev.URL == "" {
		t.Fatalf("preview = %+v", prev)
	}
	// 第二次命中缓存。
	w = f.do("POST", "/audio/segments/1-s0001/preview", "{}")
	var again audio.PreviewResult
	_ = json.Unmarshal(w.Body.Bytes(), &again)
	if !again.Cached {
		t.Fatalf("second preview should be cached: %+v", again)
	}
	if f.synth.count() != 1 {
		t.Fatalf("synth calls = %d, want 1 (cache must be reused)", f.synth.count())
	}

	// 段音频地址可直接播放（返回的就是可请求的 /api 路径）。
	r := httptest.NewRequest("GET", prev.URL, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
	rw := httptest.NewRecorder()
	f.tg.g.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK || !strings.Contains(rw.Header().Get("Content-Type"), "audio") {
		t.Fatalf("segment audio: %d %s", rw.Code, rw.Header().Get("Content-Type"))
	}

	// 重读 → 候选，不覆盖现有音频。
	w = f.do("POST", "/audio/segments/1-s0001/rerender", `{"speed":0.9,"tone":"Thì thầm"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("rerender: %d %s", w.Code, w.Body.String())
	}
	var cand audio.Candidate
	if err := json.Unmarshal(w.Body.Bytes(), &cand); err != nil {
		t.Fatal(err)
	}
	if cand.ID == "" || cand.URL == "" || cand.DurationSec <= 0 {
		t.Fatalf("candidate = %+v", cand)
	}
	r = httptest.NewRequest("GET", cand.URL, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
	rw = httptest.NewRecorder()
	f.tg.g.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK {
		t.Fatalf("candidate audio: %d", rw.Code)
	}

	// 采纳候选：段的音频参数改为候选参数。
	if w := f.do("POST", "/audio/segments/1-s0001/accept", fmt.Sprintf(`{"candidateId":%q}`, cand.ID)); w.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", w.Code, w.Body.String())
	}
	script, err := audio.ReadScript(f.svc.Layout(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if script.Segments[0].Rendered == nil || !script.Segments[0].Rendered.Rerendered || script.Segments[0].Rendered.Speed != 0.9 {
		t.Fatalf("accepted segment = %+v", script.Segments[0].Rendered)
	}
	// 候选文件已清理。
	r = httptest.NewRequest("GET", cand.URL, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
	rw = httptest.NewRecorder()
	f.tg.g.ServeHTTP(rw, r)
	if rw.Code != http.StatusNotFound {
		t.Fatalf("candidate should be gone: %d", rw.Code)
	}

	// 未知段 → 404。
	if w := f.do("POST", "/audio/segments/1-s9999/preview", "{}"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown segment: %d", w.Code)
	}
}

func TestAudioSettingsCastAndFlags(t *testing.T) {
	f := newAudioFixture(t)

	w := f.do("GET", "/audio/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("settings: %d", w.Code)
	}
	var settings audio.Settings
	_ = json.Unmarshal(w.Body.Bytes(), &settings)
	if settings.Quality != audio.QualityStandard || !settings.AutoOnLock {
		t.Fatalf("settings = %+v", settings)
	}
	w = f.do("PUT", "/audio/settings", `{"quality":"high","autoOnLock":false,"normalize":{"numbers":true,"dates":true,"money":true,"symbols":true,"abbreviations":true,"chapterTitles":true,"dropBrackets":true}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put settings: %d %s", w.Code, w.Body.String())
	}
	if got := f.svc.Settings(); got.Quality != audio.QualityHigh || got.AutoOnLock {
		t.Fatalf("settings not saved: %+v", got)
	}
	if w := f.do("PUT", "/audio/settings", `{"quality":"ultra","autoOnLock":true}`); w.Code != http.StatusConflict {
		t.Fatalf("bad quality: %d", w.Code)
	}

	// 分派：首次 GET 用角色档案初始化并落盘。
	w = f.do("GET", "/audio/cast", "")
	if w.Code != http.StatusOK {
		t.Fatalf("cast: %d %s", w.Code, w.Body.String())
	}
	var cast audio.Cast
	_ = json.Unmarshal(w.Body.Bytes(), &cast)
	if cast.Mode != audio.ModeCast || len(cast.Speakers) == 0 || cast.Speakers[0].ID != audio.NarratorID {
		t.Fatalf("cast = %+v", cast)
	}
	if _, err := os.Stat(filepath.Join(f.svc.Layout().AudioDir, "vai.json")); err != nil {
		t.Fatalf("vai.json not written: %v", err)
	}
	w = f.do("PUT", "/audio/cast", `{"mode":"single","single":{"voiceId":"giong-nu","dialogueShift":false,"keepTones":true,"dialoguePauseMs":300},"speakers":[{"id":"dan","name":"Dẫn truyện","role":"narrator","voiceId":"giong-nu"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put cast: %d %s", w.Code, w.Body.String())
	}
	if got, _ := f.do("GET", "/audio/cast", "").Body, 0; got == nil {
		t.Fatal("cast get failed")
	}

	// 听审标记。
	w = f.do("POST", "/audio/flags", `{"chapter":1,"segmentId":"s0002","atSec":12.5,"reason":"giọng bị rè"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("add flag: %d %s", w.Code, w.Body.String())
	}
	var flag audio.Flag
	_ = json.Unmarshal(w.Body.Bytes(), &flag)
	w = f.do("GET", "/audio/flags", "")
	var flags struct {
		Flags []audio.Flag `json:"flags"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &flags)
	if len(flags.Flags) != 1 || flags.Flags[0].Reason != "giọng bị rè" {
		t.Fatalf("flags = %+v", flags)
	}
	if w := f.do("DELETE", "/audio/flags/"+flag.ID, ""); w.Code != http.StatusOK {
		t.Fatalf("delete flag: %d", w.Code)
	}
	if w := f.do("POST", "/audio/flags", `{"chapter":1,"reason":""}`); w.Code != http.StatusConflict {
		t.Fatalf("flag without reason: %d", w.Code)
	}

	// 设置可写权限：读者不行。
	reader := f.tg.login("reader", "reader-pass")
	r := httptest.NewRequest("PUT", f.path("/audio/settings"), strings.NewReader(`{"quality":"fast","autoOnLock":true}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: reader})
	rw := httptest.NewRecorder()
	f.tg.g.ServeHTTP(rw, r)
	if rw.Code != http.StatusForbidden {
		t.Fatalf("reader without access should see 403, got %d", rw.Code)
	}
}

func TestAudioProjectVoicesAndPronunciations(t *testing.T) {
	f := newAudioFixture(t)

	w := f.tg.do("GET", "/api/projects/proj/voices", f.admin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("voices: %d %s", w.Code, w.Body.String())
	}
	var voices []audio.ProjectVoice
	if err := json.Unmarshal(w.Body.Bytes(), &voices); err != nil {
		t.Fatal(err)
	}
	if len(voices) != 1 || !voices[0].BuiltIn || voices[0].ID != "giong-nu" {
		t.Fatalf("engine voices = %+v", voices)
	}

	// 上传 12 秒样本 → 入库（builtIn=false）。
	upload := func(seconds float64, name string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, err := mw.CreateFormFile("file", "clip.wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(sineWAV(48000, seconds, 220)); err != nil {
			t.Fatal(err)
		}
		_ = mw.WriteField("name", name)
		_ = mw.WriteField("gender", "nu")
		_ = mw.Close()
		r := httptest.NewRequest("POST", "/api/projects/proj/voices", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
		rw := httptest.NewRecorder()
		f.tg.g.ServeHTTP(rw, r)
		return rw
	}
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg not available")
	}
	w = upload(12, "Giọng Tri Hạ")
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var saved audio.ProjectVoice
	_ = json.Unmarshal(w.Body.Bytes(), &saved)
	if saved.ID == "" || saved.BuiltIn || saved.DurationSec < 11 || saved.DurationSec > 13 {
		t.Fatalf("saved voice = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(f.st.ProjectDir, saved.File)); err != nil {
		t.Fatalf("voice file missing: %v", err)
	}
	if w := upload(3, "Quá ngắn"); w.Code != http.StatusBadRequest {
		t.Fatalf("short clip: %d %s", w.Code, w.Body.String())
	}
	// 上传后列表里既有引擎预设也有项目样本。
	w = f.tg.do("GET", "/api/projects/proj/voices", f.admin, "")
	_ = json.Unmarshal(w.Body.Bytes(), &voices)
	if len(voices) != 2 {
		t.Fatalf("voices after upload = %+v", voices)
	}

	// 读音词典（项目级）。
	if w := f.tg.do("GET", "/api/projects/proj/pronunciations", f.admin, ""); w.Code != http.StatusOK {
		t.Fatalf("pronunciations: %d", w.Code)
	}
	w = f.tg.do("PUT", "/api/projects/proj/pronunciations", f.admin, `[{"written":"live","spoken":"lai"},{"written":"show","spoken":"sô"}]`)
	if w.Code != http.StatusOK {
		t.Fatalf("put pronunciations: %d %s", w.Code, w.Body.String())
	}
	var entries []audio.Pronunciation
	_ = json.Unmarshal(w.Body.Bytes(), &entries)
	if len(entries) != 2 || entries[0].Written != "live" {
		t.Fatalf("entries = %+v", entries)
	}
	// 也接受 {entries:[...]} 包装。
	if w := f.tg.do("PUT", "/api/projects/proj/pronunciations", f.admin, `{"entries":[{"written":"MC","spoken":"em-xi"}]}`); w.Code != http.StatusOK {
		t.Fatalf("wrapped pronunciations: %d %s", w.Code, w.Body.String())
	}
	// 落盘位置：<project>/.ainovel/cach-doc.json。
	if _, err := os.Stat(filepath.Join(f.st.ProjectDir, ".ainovel", "cach-doc.json")); err != nil {
		t.Fatalf("cach-doc.json missing: %v", err)
	}
	if w := f.tg.do("PUT", "/api/projects/proj/pronunciations", f.admin, `[{"written":"only"}]`); w.Code != http.StatusConflict {
		t.Fatalf("invalid entry: %d %s", w.Code, w.Body.String())
	}
}

func TestAudioExportVolume(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg not available")
	}
	f := newAudioFixture(t)
	for ch := 1; ch <= 2; ch++ {
		f.seedScript(t, ch, audio.StatusLocked)
		if _, err := f.svc.Enqueue(1, ch); err != nil {
			t.Fatalf("enqueue %d: %v", ch, err)
		}
		f.waitJob(t, ch, audio.JobDone, audio.JobError)
	}

	// 还有章节没音频时导出 → 409（第 3 章不存在，这里改成"没有 bản đọc"的场景）。
	// §AE：POST 立即 202 + jobId；产物随后台任务完成（经 ExportInfo 断言）。
	w := f.do("POST", "/audio/volumes/1/exports", `{"format":"mp3","intro":false,"chapterTitles":false,"subtitles":true,"loudness":true,"gapSec":2}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	if decodeJSONBody(t, w)["jobId"] == "" {
		t.Fatalf("missing jobId: %s", w.Body.String())
	}
	request := f.waitExportState(t, audio.JobDone, audio.JobError)
	if request.State != audio.JobDone {
		t.Fatalf("export failed: %+v", request)
	}
	if request.Result == nil || len(request.Result.Files) == 0 {
		t.Fatalf("no result files: %+v", request)
	}
	infoCheck := waitExportInfo(t, f, 1)
	if len(infoCheck.ChapterMarks) != 2 {
		t.Fatalf("chapter marks = %+v", infoCheck.ChapterMarks)
	}
	var sawVolume, sawChapter, sawSRT bool
	for _, file := range request.Result.Files {
		switch {
		case strings.HasSuffix(file.Name, ".mp3") && !strings.Contains(file.Path, "/chuong/"):
			sawVolume = true
			if file.Size <= 0 || file.Format != "audio" || file.Volume == nil || *file.Volume != 1 {
				t.Fatalf("volume file = %+v", file)
			}
		case strings.HasPrefix(file.Path, "tap-01/chuong/"):
			sawChapter = true
		case strings.HasSuffix(file.Name, ".srt"):
			sawSRT = true
		}
	}
	if !sawVolume || !sawChapter || !sawSRT {
		t.Fatalf("missing outputs (volume=%v chapter=%v srt=%v): %+v", sawVolume, sawChapter, sawSRT, request.Result.Files)
	}
	exportDir := f.svc.Layout().VolumeExportDir(1)
	if _, err := os.Stat(filepath.Join(exportDir, "Tập 1.ban-doc.txt")); err != nil {
		t.Fatalf("ban-doc.txt missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportDir, "Tập 1.moc-chuong.txt")); err != nil {
		t.Fatalf("moc-chuong.txt missing: %v", err)
	}

	// GET volumes/1/exports → "đã xuất" 状态。
	w = f.do("GET", "/audio/volumes/1/exports", "")
	if w.Code != http.StatusOK {
		t.Fatalf("export info: %d %s", w.Code, w.Body.String())
	}
	var info audio.ExportInfo
	_ = json.Unmarshal(w.Body.Bytes(), &info)
	if info.ExportedAt == nil || info.ChapterCount != 2 || info.Voices != 2 || info.DurationSec <= 0 {
		t.Fatalf("export info = %+v", info)
	}
	// 波形约 2 点/秒，覆盖整卷时长。
	if want := int(math.Ceil(info.DurationSec * 2)); len(info.Peaks) < want-1 || len(info.Peaks) > want+1 {
		t.Fatalf("peaks = %d for %.2fs, want ≈%d", len(info.Peaks), info.DurationSec, want)
	}
	if info.Volume != 1 || info.Dir != "xuat-ban/tap-01" || info.TotalBytes <= 0 {
		t.Fatalf("export info dir/size = %+v", info)
	}
	// path 相对导出目录：直接喂给既有下载接口 exports/file?path= 要能下到，
	// 且必须支持 Range（iOS 上的导出音频播放器走同一接口）。
	for _, file := range info.Files {
		if !strings.HasSuffix(file.Name, ".srt") {
			continue
		}
		if w := f.do("GET", "/exports/file?path="+url.QueryEscape(file.Path), ""); w.Code != http.StatusOK {
			t.Fatalf("download %s: %d %s", file.Path, w.Code, w.Body.String())
		}
		req := httptest.NewRequest("GET", f.path("/exports/file?path="+url.QueryEscape(file.Path)), nil)
		req.Header.Set("Range", "bytes=0-3")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
		res := httptest.NewRecorder()
		f.tg.g.ServeHTTP(res, req)
		if res.Code != http.StatusPartialContent || res.Header().Get("Accept-Ranges") != "bytes" || res.Body.Len() != 4 {
			t.Fatalf("range download %s: code=%d accept=%q len=%d", file.Path, res.Code, res.Header().Get("Accept-Ranges"), res.Body.Len())
		}
	}

	// 导出产物出现在既有导出列表的 ExportRow.audio 里。
	w = f.do("GET", "/exports", "")
	if w.Code != http.StatusOK {
		t.Fatalf("exports list: %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Matrix []ExportRowAPI `json:"matrix"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Matrix) == 0 || len(list.Matrix[0].Audio) == 0 {
		t.Fatalf("ExportRow.audio not populated: %+v", list.Matrix)
	}
	if list.Matrix[0].TXT != nil && strings.Contains(list.Matrix[0].TXT.Name, "ban-doc") {
		t.Fatalf("companion txt must not become the volume txt export: %+v", list.Matrix[0].TXT)
	}
}

func TestAudioComposeWithoutModel(t *testing.T) {
	f := newAudioFixture(t)
	for _, path := range []string{"/audio/volumes/1/scripts", "/audio/volumes/1/scripts/1/refresh"} {
		w := f.do("POST", path, `{"chapters":[1]}`)
		var body struct {
			Error       string `json:"error"`
			NeedsConfig bool   `json:"needsConfig"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusConflict || !body.NeedsConfig || body.Error != needsModelMessage {
			t.Fatalf("%s without model: %d %s", path, w.Code, w.Body.String())
		}
	}
}

// sseRecorder 是并发安全的 ResponseWriter（SSE 测试用）。
type sseRecorder struct {
	mu     sync.Mutex
	header http.Header
	buf    bytes.Buffer
	status int
}

func (s *sseRecorder) Header() http.Header {
	if s.header == nil {
		s.header = http.Header{}
	}
	return s.header
}

func (s *sseRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = code
}

func (s *sseRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *sseRecorder) Flush() {}

func (s *sseRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestAudioEventsStream(t *testing.T) {
	f := newAudioFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("GET", f.path("/audio/events"), nil).WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
	w := &sseRecorder{}
	done := make(chan struct{})
	go func() {
		f.tg.g.serveStoryAudio(w, r, identity.User{Username: "admin", Role: identity.RoleAdmin}, f.st, "/audio/events")
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.svc.Hub().Publish(audio.NewEvent(audio.EventRender, map[string]any{"chapter": 1, "percent": 42.0}))
		if strings.Contains(w.body(), "audio.render") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SSE handler did not stop on context cancel")
	}
	body := w.body()
	if !strings.Contains(body, "event: audio.render") || !strings.Contains(body, `"category":"AUDIO"`) {
		t.Fatalf("sse body = %q", body)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content type = %q", w.Header().Get("Content-Type"))
	}
}

func TestAudioCastVersionConflictAndChangeEvent(t *testing.T) {
	f := newAudioFixture(t)
	sub, _, _ := f.tg.g.events().subscribe(identity.User{Username: "admin", Role: identity.RoleAdmin}, 0, nil)
	defer f.tg.g.events().unsubscribe(sub)

	w := f.do("GET", "/audio/cast", "")
	if w.Code != http.StatusOK {
		t.Fatalf("cast: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Version == "" || w.Header().Get("ETag") == "" {
		t.Fatalf("cast GET must carry version: %s", w.Body.String())
	}
	body := `{"mode":"single","single":{"voiceId":"Mai Anh","dialogueShift":false,"keepTones":true,"dialoguePauseMs":300},"speakers":[]}`
	put := func(ifMatch string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", f.path("/audio/cast"), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		rec := httptest.NewRecorder()
		f.tg.g.ServeHTTP(rec, req)
		return rec
	}
	if w := put(got.Version); w.Code != http.StatusOK {
		t.Fatalf("matching If-Match: %d %s", w.Code, w.Body.String())
	}
	// 旧版本再写 → 409，带 current。
	w = put(got.Version)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"conflict":true`) || !strings.Contains(w.Body.String(), `"version":`) {
		t.Fatalf("stale If-Match: %d %s", w.Code, w.Body.String())
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-sub.C:
			if ev.Type == "story.changed" && ev.Meta.(Event).Story == f.st.Story {
				return
			}
		case <-deadline:
			t.Fatal("no story.changed after cast PUT")
		}
	}
}
