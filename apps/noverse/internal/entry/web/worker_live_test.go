package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
)

// liveFakeHost 在 fakeHost 之上补齐新端点行为：可配置共创回复、任务事件通道、
// 修订检查、诊断报告与模型/推理面板数据。
type liveFakeHost struct {
	*fakeHost

	coReply    host.CoCreateReply
	coErr      error
	coProgress []string // 依次经 onProgress 下发的 reply 预览（累积式）
	coCalls    []string // "cold" / "stage"
	lastHist   []host.CoCreateMessage

	impCh   chan imp.Event
	simCh   chan sim.Event
	impErr  error
	simErr  error
	impOpts []imp.Options

	changes []host.RevisionChange
	syncRes *revision.Result
	syncErr error
	report  diag.Report

	pauseOK bool
}

func newLiveFakeHost() *liveFakeHost {
	return &liveFakeHost{fakeHost: newFakeHost(), pauseOK: true}
}

func (h *liveFakeHost) runCoCreate(kind string, ctx context.Context, hist []host.CoCreateMessage, onProgress func(string, string)) (host.CoCreateReply, error) {
	h.mu.Lock()
	h.coCalls = append(h.coCalls, kind)
	h.lastHist = append([]host.CoCreateMessage(nil), hist...)
	h.mu.Unlock()
	for _, p := range h.coProgress {
		onProgress(host.CoCreateProgressReply, p)
	}
	if err := ctx.Err(); err != nil {
		return host.CoCreateReply{}, err
	}
	return h.coReply, h.coErr
}
func (h *liveFakeHost) CoCreateStream(ctx context.Context, hist []host.CoCreateMessage, onProgress func(string, string)) (host.CoCreateReply, error) {
	return h.runCoCreate("cold", ctx, hist, onProgress)
}
func (h *liveFakeHost) StageCoCreateStream(ctx context.Context, hist []host.CoCreateMessage, onProgress func(string, string)) (host.CoCreateReply, error) {
	return h.runCoCreate("stage", ctx, hist, onProgress)
}
func (h *liveFakeHost) PauseForCoCreate() bool { h.call("pauseCoCreate"); return h.pauseOK }
func (h *liveFakeHost) ResumeFromCoCreate(string) error {
	h.call("resumeCoCreate")
	return nil
}
func (h *liveFakeHost) CancelCoCreate() { h.call("cancelCoCreate") }

func (h *liveFakeHost) ImportFrom(_ context.Context, opts imp.Options) (<-chan imp.Event, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.impOpts = append(h.impOpts, opts)
	if h.impErr != nil {
		return nil, h.impErr
	}
	if h.impCh == nil {
		ch := make(chan imp.Event)
		close(ch)
		return ch, nil
	}
	return h.impCh, nil
}
func (h *liveFakeHost) SimulateSource(_ context.Context, dir string) (<-chan sim.Event, error) {
	h.call("simulate:" + dir)
	if h.simErr != nil {
		return nil, h.simErr
	}
	if h.simCh == nil {
		ch := make(chan sim.Event)
		close(ch)
		return ch, nil
	}
	return h.simCh, nil
}
func (h *liveFakeHost) ImportSimulationProfile(_ context.Context, path string) (<-chan sim.Event, error) {
	h.call("importsim:" + path)
	if h.simErr != nil {
		return nil, h.simErr
	}
	if h.simCh == nil {
		ch := make(chan sim.Event)
		close(ch)
		return h.simCh, nil
	}
	return h.simCh, nil
}
func (h *liveFakeHost) CheckRevisionChanges() ([]host.RevisionChange, error) {
	return h.changes, nil
}
func (h *liveFakeHost) SyncChapterRevisions(context.Context) (*revision.Result, error) {
	return h.syncRes, h.syncErr
}
func (h *liveFakeHost) Diagnose() diag.Report       { return h.report }
func (h *liveFakeHost) DiagnoseView() host.DiagView { return host.DiagView{Report: h.report} }
func (h *liveFakeHost) CurrentModelSelection(role string) (string, string, bool) {
	if role == "writer" {
		return "openrouter", "writer-model", true
	}
	return "openrouter", "base-model", true
}
func (h *liveFakeHost) ModelCatalog() []host.CatalogProvider {
	return []host.CatalogProvider{{
		Name: "openrouter", Type: "openrouter", Ready: true,
		Models: []host.CatalogModel{
			{Name: "base-model", ContextWindow: 200000, Thinking: []string{"", "low", "medium", "high"}},
			{Name: "writer-model", ContextWindow: 1000000, Thinking: []string{""}},
		},
	}}
}
func (h *liveFakeHost) CurrentThinking(role string) string { return "medium" }
func (h *liveFakeHost) AvailableThinkingLevels(role string) []string {
	return []string{"off", "low", "medium", "high"}
}

// newLiveServer 直接构造 workerServer（不经 SSE fan-out），供纯 handler 测试。
func newLiveServer(h workerHost) *workerServer {
	return &workerServer{h: h, token: "secret", b: newBroadcaster(), jobs: newJobTracker(), co: newCoCreateManager()}
}

func doReq(t *testing.T, s http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-Ainovel-Worker-Token", "secret")
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func decodeLiveBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return v
}

func waitJobDone(t *testing.T, s *workerServer, kind string) jobEvent {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ev, ok := s.jobs.latest(kind); ok && ev.Done {
			return ev
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached done", kind)
	return jobEvent{}
}

func callsJoined(h *liveFakeHost) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.calls, ",")
}

// ── 共创 ──

func TestCoCreateColdStartFlow(t *testing.T) {
	h := newLiveFakeHost()
	h.coReply = host.CoCreateReply{Message: "Đã rõ", Prompt: "bản nháp chỉ dẫn", Ready: true, Suggestions: []string{"gợi ý 1"}}
	s := newLiveServer(h)
	s.storyDir = t.TempDir()

	w := doReq(t, s, "POST", "/api/cocreate/message", `{"message":"Viết truyện kiếm hiệp"}`)
	if w.Code != 200 {
		t.Fatalf("message status %d: %s", w.Code, w.Body.String())
	}
	v := decodeLiveBody(t, w)
	if v["reply"] != "Đã rõ" || v["draft"] != "bản nháp chỉ dẫn" || v["ready"] != true {
		t.Fatalf("bad reply body: %v", v)
	}
	if got := callsJoined(h); strings.Contains(got, "pauseCoCreate") {
		t.Fatalf("cold start must not pause: %s", got)
	}

	w = doReq(t, s, "GET", "/api/cocreate", "")
	v = decodeLiveBody(t, w)
	if v["active"] != true {
		t.Fatalf("state not active: %v", v)
	}
	msgs := v["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %v", msgs)
	}
	// 契约 GET /cocreate 必须回显草稿/就绪/建议（前端刷新后仍能继续）。
	if v["draft"] != "bản nháp chỉ dẫn" || v["ready"] != true {
		t.Fatalf("state draft/ready: %v", v)
	}
	if sg := v["suggestions"].([]any); len(sg) != 1 || sg[0] != "gợi ý 1" {
		t.Fatalf("state suggestions: %v", v)
	}

	w = doReq(t, s, "POST", "/api/cocreate/commit", `{}`)
	v = decodeLiveBody(t, w)
	if v["action"] != "start" {
		t.Fatalf("commit action: %v", v)
	}
	if got := callsJoined(h); got != "prepare,start" {
		t.Fatalf("commit calls: %s", got)
	}
	w = doReq(t, s, "GET", "/api/cocreate", "")
	if v := decodeLiveBody(t, w); v["active"] != false {
		t.Fatalf("session not cleared: %v", v)
	}
}

func TestCoCreateStageFlow(t *testing.T) {
	h := newLiveFakeHost()
	h.snapshot = host.UISnapshot{Phase: "writing"}
	h.coReply = host.CoCreateReply{Message: "Hướng mới", Prompt: "đi theo hướng này", Ready: true}
	s := newLiveServer(h)
	s.storyDir = t.TempDir()

	w := doReq(t, s, "POST", "/api/cocreate/message", `{"message":"Đổi hướng"}`)
	if w.Code != 200 {
		t.Fatalf("message status %d: %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	kind := h.coCalls[0]
	h.mu.Unlock()
	if kind != "stage" {
		t.Fatalf("want stage stream, got %s", kind)
	}
	w = doReq(t, s, "POST", "/api/cocreate/commit", `{}`)
	if v := decodeLiveBody(t, w); v["action"] != "resume" {
		t.Fatalf("commit action: %v", v)
	}
	if got := callsJoined(h); got != "pauseCoCreate,resumeCoCreate" {
		t.Fatalf("calls: %s", got)
	}
}

func TestCoCreateCancelReleases(t *testing.T) {
	h := newLiveFakeHost()
	h.snapshot = host.UISnapshot{Phase: "writing"}
	s := newLiveServer(h)
	s.storyDir = t.TempDir()

	w := doReq(t, s, "POST", "/api/cocreate/cancel", `{}`)
	if w.Code != 200 {
		t.Fatalf("cancel %d", w.Code)
	}
	// cancel 在会话未开时不应触碰 host
	if got := callsJoined(h); got != "" {
		t.Fatalf("unexpected calls: %s", got)
	}

	doReq(t, s, "POST", "/api/cocreate/message", `{"message":"x"}`)
	w = doReq(t, s, "POST", "/api/cocreate/cancel", `{}`)
	if w.Code != 200 {
		t.Fatalf("cancel %d", w.Code)
	}
	if got := callsJoined(h); !strings.Contains(got, "cancelCoCreate") {
		t.Fatalf("stage cancel missing CancelCoCreate: %s", got)
	}
}

func TestCoCreateStreamsSSE(t *testing.T) {
	h := newLiveFakeHost()
	h.coProgress = []string{"Xin", "Xin chào"}
	h.coReply = host.CoCreateReply{Message: "Xin chào"}
	handler := newWorkerServer(h, "secret", nil, newBroadcaster())
	handler.storyDir = t.TempDir()
	go handler.pump()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	r := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	r.Header.Set("X-Ainovel-Worker-Token", "secret")
	fin := make(chan struct{})
	go func() { handler.ServeHTTP(w, r); close(fin) }()
	w.until(t, "event: snapshot")

	rec := doReq(t, handler, "POST", "/api/cocreate/message", `{"message":"chào"}`)
	if rec.Code != 200 {
		t.Fatalf("message %d: %s", rec.Code, rec.Body.String())
	}
	// 增量而非累积：两次预览 "Xin" → "Xin chào" 只下发新增部分。
	body := w.until(t, `"text":" chào"`)
	if !strings.Contains(body, "event: cocreate\ndata: {\"text\":\"Xin\"}") {
		t.Fatalf("missing first cocreate delta: %s", body)
	}
	if strings.Contains(body, `"text":"Xin chào"`) {
		t.Fatalf("cocreate must stream deltas, not cumulative text: %s", body)
	}
	cancel()
	<-fin
}

// ── 长任务 ──

func TestImportJobLifecycle(t *testing.T) {
	h := newLiveFakeHost()
	h.impCh = make(chan imp.Event, 8)
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/import", `{"path":"","storyStatus":"open"}`)
	v := decodeLiveBody(t, w)
	if w.Code != 200 || v["job"] == nil {
		t.Fatalf("import start: %d %v", w.Code, v)
	}
	h.mu.Lock()
	if h.impOpts[0].StoryResolution != "open" {
		t.Fatalf("opts: %+v", h.impOpts[0])
	}
	h.mu.Unlock()

	h.impCh <- imp.Event{Stage: imp.StageSegmenting, Current: 2, Total: 10, Message: "chia"}
	h.impCh <- imp.Event{Stage: imp.StageAwaitingConfirmation, Message: "xác nhận"}
	h.impCh <- imp.Event{Stage: imp.StageDone, Message: "xong"}
	close(h.impCh)
	ev := waitJobDone(t, s, "import")
	if ev.Level != "success" || ev.Stage != "done" {
		t.Fatalf("terminal event: %+v", ev)
	}
	if ev.Awaiting != "" {
		t.Fatalf("awaiting should clear on done: %+v", ev)
	}

	w = doReq(t, s, "GET", "/api/import", "")
	v = decodeLiveBody(t, w)
	if v["job"] == nil {
		t.Fatalf("last job missing: %v", v)
	}
}

func TestImportAwaitingConfirm(t *testing.T) {
	h := newLiveFakeHost()
	h.impCh = make(chan imp.Event, 8)
	s := newLiveServer(h)

	doReq(t, s, "POST", "/api/import", `{"path":""}`)
	h.impCh <- imp.Event{Stage: imp.StageAwaitingConfirmation, Message: "xem trước"}
	close(h.impCh)
	ev := waitJobDone(t, s, "import")
	if ev.Awaiting != "confirm" {
		t.Fatalf("awaiting: %+v", ev)
	}

	w := doReq(t, s, "POST", "/api/import/confirm", `{"accept":true,"storyStatus":"closed"}`)
	if w.Code != 200 {
		t.Fatalf("confirm %d: %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	last := h.impOpts[len(h.impOpts)-1]
	h.mu.Unlock()
	if !last.AcceptSegmentation || last.StoryResolution != "closed" {
		t.Fatalf("confirm opts: %+v", last)
	}
}

func TestImportConflict409(t *testing.T) {
	h := newLiveFakeHost()
	h.impCh = make(chan imp.Event) // 永不推进，保持占用
	s := newLiveServer(h)
	defer close(h.impCh)

	if w := doReq(t, s, "POST", "/api/import", `{"path":""}`); w.Code != 200 {
		t.Fatalf("first import %d", w.Code)
	}
	if w := doReq(t, s, "POST", "/api/import", `{"path":""}`); w.Code != 409 {
		t.Fatalf("second import should 409, got %d", w.Code)
	}
}

func TestJobBlockedWhileRunning(t *testing.T) {
	h := newLiveFakeHost()
	h.snapshot = host.UISnapshot{IsRunning: true}
	s := newLiveServer(h)
	for _, path := range []string{"/api/import", "/api/simulate", "/api/importsim", "/api/sync"} {
		if w := doReq(t, s, "POST", path, `{}`); w.Code != 409 {
			t.Fatalf("%s should 409 while running, got %d", path, w.Code)
		}
	}
}

func TestImportPathSandbox(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)
	s.storyDir = t.TempDir()
	s.root = s.storyDir
	outside := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := doReq(t, s, "POST", "/api/import", `{"path":"`+outside+`"}`); w.Code != 400 {
		t.Fatalf("outside path should 400, got %d", w.Code)
	}
	if w := doReq(t, s, "POST", "/api/import", `{"path":"missing.txt"}`); w.Code != 400 {
		t.Fatalf("missing path should 400, got %d", w.Code)
	}
}

func TestSimulateJob(t *testing.T) {
	h := newLiveFakeHost()
	h.simCh = make(chan sim.Event, 4)
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/simulate", `{"dir":""}`)
	if w.Code != 200 {
		t.Fatalf("simulate %d: %s", w.Code, w.Body.String())
	}
	h.simCh <- sim.Event{Stage: sim.StageAnalyze, Current: 1, Total: 3, Message: "phân tích"}
	h.simCh <- sim.Event{Stage: sim.StageDone, Message: "xong"}
	close(h.simCh)
	ev := waitJobDone(t, s, "simulate")
	if ev.Kind != "simulate" || ev.Level != "success" {
		t.Fatalf("event: %+v", ev)
	}
	if got := callsJoined(h); !strings.Contains(got, "simulate:") {
		t.Fatalf("SimulateSource not called: %s", got)
	}
}

func TestImportSimJob(t *testing.T) {
	h := newLiveFakeHost()
	h.simCh = make(chan sim.Event, 4)
	s := newLiveServer(h)
	s.storyDir = t.TempDir()
	s.root = s.storyDir
	profile := filepath.Join(s.storyDir, "profile.txt")
	if err := os.WriteFile(profile, []byte("mẫu văn phong"), 0o644); err != nil {
		t.Fatal(err)
	}

	if w := doReq(t, s, "POST", "/api/importsim", `{"path":""}`); w.Code != 400 {
		t.Fatalf("empty path should 400, got %d", w.Code)
	}
	w := doReq(t, s, "POST", "/api/importsim", `{"path":"profile.txt"}`)
	v := decodeLiveBody(t, w)
	if w.Code != 200 || v["job"] == nil {
		t.Fatalf("importsim start: %d %v", w.Code, v)
	}
	h.simCh <- sim.Event{Stage: sim.StageAnalyze, Current: 1, Total: 2, Message: "phân tích"}
	h.simCh <- sim.Event{Stage: sim.StageDone, Message: "xong"}
	close(h.simCh)
	ev := waitJobDone(t, s, "importsim")
	if ev.Kind != "importsim" || ev.Level != "success" || !ev.Done {
		t.Fatalf("event: %+v", ev)
	}
	if got := callsJoined(h); !strings.Contains(got, "importsim:"+profile) {
		t.Fatalf("ImportSimulationProfile not called with resolved path: %s", got)
	}
}

func TestSyncCheckAndRun(t *testing.T) {
	h := newLiveFakeHost()
	h.changes = []host.RevisionChange{{Chapter: 3, Title: "Chương ba", Kind: "edit"}}
	h.syncRes = &revision.Result{Applied: []int{3}}
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/sync", `{"check":true}`)
	v := decodeLiveBody(t, w)
	changed := v["changed"].([]any)
	if len(changed) != 1 || changed[0].(map[string]any)["chapter"] != float64(3) {
		t.Fatalf("changed: %v", v)
	}
	if got := callsJoined(h); strings.Contains(got, "sync") {
		t.Fatalf("check must not sync")
	}

	w = doReq(t, s, "POST", "/api/sync", `{"check":false}`)
	if w.Code != 200 {
		t.Fatalf("sync %d", w.Code)
	}
	ev := waitJobDone(t, s, "sync")
	if ev.Level != "success" || !strings.Contains(ev.Message, "Đã tiếp nhận") {
		t.Fatalf("sync event: %+v", ev)
	}
}

func TestJobEventsOverSSE(t *testing.T) {
	h := newLiveFakeHost()
	h.impCh = make(chan imp.Event, 8)
	handler := NewWorkerHandler(h, "secret")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	r := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	r.Header.Set("X-Ainovel-Worker-Token", "secret")
	fin := make(chan struct{})
	go func() { handler.ServeHTTP(w, r); close(fin) }()
	w.until(t, "event: snapshot")

	doReq(t, handler, "POST", "/api/import", `{"path":""}`)
	h.impCh <- imp.Event{Stage: imp.StageDone, Message: "xong"}
	close(h.impCh)
	body := w.until(t, `"done":true`)
	if !strings.Contains(body, "event: job") || !strings.Contains(body, `"kind":"import"`) {
		t.Fatalf("missing job event: %s", body)
	}
	cancel()
	<-fin
}

// ── 诊断 / 模型 / 推理 / start-file ──

func TestDiagSections(t *testing.T) {
	h := newLiveFakeHost()
	h.report = diag.Report{
		Stats: diag.Stats{CompletedChapters: 2, TotalChapters: 10, TotalWords: 5000, Phase: "writing"},
		Findings: []diag.Finding{
			{Rule: "StaleForeshadow", Category: diag.CatPlanning, Severity: diag.SevWarning, Title: "Phục bút đình trệ", Evidence: "ch 4", Suggestion: "cập nhật"},
			{Rule: "Loop", Category: diag.CatFlow, Severity: diag.SevCritical, Title: "Vòng lặp"},
		},
		Actions: []diag.Action{{Kind: diag.ActionEmitNotice, Summary: "Thông báo"}},
	}
	s := newLiveServer(h)

	w := doReq(t, s, "GET", "/api/diag", "")
	v := decodeLiveBody(t, w)
	if v["generatedAt"] == "" {
		t.Fatalf("generatedAt missing: %v", v)
	}
	sections := v["sections"].([]any)
	ids := []string{}
	for _, sec := range sections {
		ids = append(ids, sec.(map[string]any)["id"].(string))
	}
	want := []string{"overview", "planning", "flow", "actions"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("sections: %v", ids)
	}
}

func TestModelsEndpoints(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)

	w := doReq(t, s, "GET", "/api/models", "")
	v := decodeLiveBody(t, w)
	roles := v["roles"].([]any)
	if len(roles) != 4 {
		t.Fatalf("roles: %v", roles)
	}
	writer := roles[2].(map[string]any)
	if writer["model"] != "writer-model" || writer["thinking"] != "medium" {
		t.Fatalf("writer role: %v", writer)
	}
	providers := v["providers"].([]any)
	if len(providers) != 1 {
		t.Fatalf("providers: %v", providers)
	}
	prov := providers[0].(map[string]any)
	if prov["provider"] != "openrouter" || prov["label"] != "OpenRouter" || prov["ready"] != true {
		t.Fatalf("provider: %v", prov)
	}
	models := prov["models"].([]any)
	base := models[0].(map[string]any)
	if base["name"] != "base-model" || base["contextWindow"] != float64(200000) {
		t.Fatalf("base model: %v", base)
	}
	if th := base["thinking"].([]any); len(th) != 4 || th[0] != "" || th[3] != "high" {
		t.Fatalf("base thinking: %v", th)
	}

	for _, action := range []struct{ path, body string }{
		{"models", `{"role":"writer","provider":"openrouter","model":"base-model"}`},
		{"thinking", `{"role":"editor","level":"high"}`},
	} {
		if response := doReq(t, s, "POST", "/api/"+action.path, action.body); response.Code != http.StatusNotFound {
			t.Fatalf("story AI write through %s: %d %s", action.path, response.Code, response.Body.String())
		}
	}
}

func TestStartFile(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)
	s.storyDir = t.TempDir()
	s.root = s.storyDir
	prompt := filepath.Join(s.storyDir, "premise.md")
	if err := os.WriteFile(prompt, []byte("ý tưởng truyện"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := doReq(t, s, "POST", "/api/start-file", `{"path":"premise.md"}`)
	v := decodeLiveBody(t, w)
	if w.Code != 200 || v["action"] != "start" {
		t.Fatalf("start-file: %d %v", w.Code, v)
	}
	if got := callsJoined(h); got != "prepare,start" {
		t.Fatalf("calls: %s", got)
	}

	h.snapshot = host.UISnapshot{Phase: "writing"}
	if w := doReq(t, s, "POST", "/api/start-file", `{"path":"premise.md"}`); w.Code != 409 {
		t.Fatalf("started story should 409, got %d", w.Code)
	}
}

func TestJobReplayOnReconnect(t *testing.T) {
	h := newLiveFakeHost()
	h.impCh = make(chan imp.Event, 8)
	s := newLiveServer(h)
	doReq(t, s, "POST", "/api/import", `{"path":""}`)
	h.impCh <- imp.Event{Stage: imp.StageSegmenting, Current: 1, Total: 5}
	close(h.impCh)
	waitJobDone(t, s, "import")

	replays := s.jobs.replays()
	if len(replays) != 1 || replays[0].kind != "job" {
		t.Fatalf("replays: %v", replays)
	}
	ev := replays[0].data.(jobEvent)
	if ev.Kind != "import" || !ev.Done || ev.Stage != "segmenting" {
		t.Fatalf("replayed job: %+v", ev)
	}
}

func TestExportPathResolvesLibraryAliasesAndRejectsEscapingLinks(t *testing.T) {
	root := t.TempDir()
	story := filepath.Join(root, "book")
	if err := os.Mkdir(story, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "library-link")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	s := newLiveServer(newLiveFakeHost())
	s.storyDir, s.root = story, alias
	if _, err := s.resolveStoryPath("xuat-ban/new/book.txt", false); err != nil {
		t.Fatalf("new export inside aliased library rejected: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(story, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.resolveStoryPath("escape/book.txt", false); err == nil {
		t.Fatal("export escaped library through a directory symlink")
	}
}
