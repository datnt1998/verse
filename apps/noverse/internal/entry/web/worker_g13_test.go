package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/startup"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
)

// ── G4: POST /api/retry ──

func TestRetryEndpoint(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/retry", `{}`)
	if w.Code != 409 {
		t.Fatalf("no waiter status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["error"] != "Không có lượt nào đang chờ thử lại" {
		t.Fatalf("error = %v", v["error"])
	}

	h.mu.Lock()
	h.retryWake = true
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/retry", `{}`)
	if w.Code != 200 {
		t.Fatalf("waiter status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["ok"] != true {
		t.Fatalf("body = %v", v)
	}
}

// ── G9: POST /api/import/preview ──

func TestImportPreviewEndpoint(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)
	s.storyDir = t.TempDir()

	src := strings.Join([]string{
		"Chương 1: Khởi đầu",
		"",
		"Một hai ba bốn.",
		"Chương 2: Tiếp",
		"",
		"Năm sáu.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(s.storyDir, "ban-thao.txt"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	w := doReq(t, s, "POST", "/api/import/preview", `{"path":"ban-thao.txt"}`)
	if w.Code != 200 {
		t.Fatalf("preview status = %d: %s", w.Code, w.Body.String())
	}
	v := decodeLiveBody(t, w)
	chapters := v["chapters"].([]any)
	if len(chapters) != 2 {
		t.Fatalf("chapters = %v", chapters)
	}
	first := chapters[0].(map[string]any)
	if first["n"] != float64(1) || first["title"] != "Chương 1: Khởi đầu" || first["words"] != float64(4) {
		t.Fatalf("first chapter = %v", first)
	}
	if v["encoding"] != "utf-8" {
		t.Fatalf("encoding = %v", v["encoding"])
	}
	if warnings, ok := v["warnings"].([]any); !ok || len(warnings) != 0 {
		t.Fatalf("warnings = %v", v["warnings"])
	}

	// 预览是只读的：源文件旁不得出现导入工作区。
	if _, err := os.Stat(filepath.Join(s.storyDir, "meta", "import")); !os.IsNotExist(err) {
		t.Fatalf("preview created import workspace: %v", err)
	}

	w = doReq(t, s, "POST", "/api/import/preview", `{"path":"thieu.txt"}`)
	if w.Code != 400 {
		t.Fatalf("missing file status = %d: %s", w.Code, w.Body.String())
	}
	w = doReq(t, s, "POST", "/api/import/preview", `{}`)
	if w.Code != 400 {
		t.Fatalf("empty path status = %d: %s", w.Code, w.Body.String())
	}
}

// ── G13: POST /api/command ──

func (h *liveFakeHost) SetAdvanceMode(mode domain.ChapterAdvanceMode) error {
	h.call("advanceMode:" + string(mode))
	return nil
}

func (h *liveFakeHost) AdvanceOneChapter() error {
	h.call("next")
	return nil
}

func (h *liveFakeHost) Reopen(direction string) error {
	h.call("reopen:" + direction)
	return nil
}

func (h *liveFakeHost) Resume() (string, error) {
	h.call("resume")
	return "Chương 5", nil
}

func (h *liveFakeHost) Export(_ context.Context, opts exp.Options) (*exp.Result, error) {
	h.call("export:" + opts.OutPath)
	return &exp.Result{Path: "/tmp/truyen.txt", Chapters: 3, Bytes: 2048, Skipped: []int{4}}, nil
}

func commandBody(line string) string {
	return `{"line":` + quoteJSON(line) + `}`
}

func quoteJSON(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(s) + `"`
}

func TestCommandSyntaxAndIdleErrors(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/command", commandBody("/review"))
	if w.Code != 400 {
		t.Fatalf("missing arg status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["error"] != "Cách dùng: /review on|off" {
		t.Fatalf("usage error = %v", v["error"])
	}

	w = doReq(t, s, "POST", "/api/command", commandBody("/khong-co"))
	if w.Code != 400 {
		t.Fatalf("unknown status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); !strings.Contains(v["error"].(string), "Lệnh không xác định") {
		t.Fatalf("unknown error = %v", v["error"])
	}

	h.mu.Lock()
	h.snapshot = host.UISnapshot{IsRunning: true}
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/command", commandBody("/next"))
	if w.Code != 409 {
		t.Fatalf("busy status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["error"] != "Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /next" {
		t.Fatalf("busy error = %v", v["error"])
	}
}

func TestCommandDispatchesHostActions(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)

	w := doReq(t, s, "POST", "/api/command", commandBody("/review on"))
	if w.Code != 200 {
		t.Fatalf("review status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["message"] != "Đã bật duyệt từng chương" {
		t.Fatalf("review message = %v", v["message"])
	}

	w = doReq(t, s, "POST", "/api/command", commandBody("/next"))
	if w.Code != 200 {
		t.Fatalf("next status = %d: %s", w.Code, w.Body.String())
	}

	h.mu.Lock()
	h.snapshot = host.UISnapshot{Phase: "complete"}
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/command", commandBody("/reopen viết tiếp tập 2"))
	if w.Code != 200 {
		t.Fatalf("reopen status = %d: %s", w.Code, w.Body.String())
	}
	if got := callsJoined(h); !strings.Contains(got, "reopen:viết tiếp tập 2") || !strings.Contains(got, "resume") {
		t.Fatalf("reopen calls: %s", got)
	}

	w = doReq(t, s, "POST", "/api/command", commandBody("/export from=2 to=9"))
	if w.Code != 200 {
		t.Fatalf("export status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["result"].(map[string]any)["kind"] != "job" {
		t.Fatalf("export must return a tracked job = %v", v)
	}
	if event := waitJobDone(t, s, "export"); event.Export.(map[string]any)["chapters"] != 3 {
		t.Fatalf("missing export result: %+v", event)
	}

	h.mu.Lock()
	h.changes = []host.RevisionChange{{Chapter: 3, Title: "Ba"}}
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/command", commandBody("/sync --check"))
	if w.Code != 200 {
		t.Fatalf("sync status = %d: %s", w.Code, w.Body.String())
	}
	if v := decodeLiveBody(t, w); v["result"].(map[string]any)["kind"] != "syncCheck" {
		t.Fatalf("missing sync check result = %v", v)
	}

	w = doReq(t, s, "POST", "/api/command", commandBody("/help"))
	if w.Code != 200 {
		t.Fatalf("help status = %d: %s", w.Code, w.Body.String())
	}
}

func TestCommandStartsImportAndStartFile(t *testing.T) {
	h := newLiveFakeHost()
	s := newLiveServer(h)
	dir := t.TempDir()
	s.storyDir = dir

	if err := os.WriteFile(filepath.Join(dir, "nguon.txt"), []byte("Chương 1\nnội dung"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := doReq(t, s, "POST", "/api/command", commandBody(`/import "nguon.txt" --yes --story=open --guide="gộp phần mở đầu"`))
	if w.Code != 200 {
		t.Fatalf("import status = %d: %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	if len(h.impOpts) != 1 {
		h.mu.Unlock()
		t.Fatalf("import not started: %+v", h.impOpts)
	}
	opts := h.impOpts[0]
	h.mu.Unlock()
	if !opts.AutoConfirm || opts.StoryResolution != "open" || opts.Guidance != "gộp phần mở đầu" || opts.SourcePath != filepath.Join(dir, "nguon.txt") {
		t.Fatalf("import opts = %+v", opts)
	}

	if err := os.WriteFile(filepath.Join(dir, "de-cuong.md"), []byte("Một truyện ngắn."), 0o644); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.snapshot = host.UISnapshot{}
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/command", commandBody(`/start "de-cuong.md"`))
	if w.Code != 200 {
		t.Fatalf("start status = %d: %s", w.Code, w.Body.String())
	}
	if got := callsJoined(h); !strings.Contains(got, "prepare,start") {
		t.Fatalf("start calls: %s", got)
	}

	// /start 只在欢迎页可用。
	h.mu.Lock()
	h.snapshot = host.UISnapshot{Phase: "writing"}
	h.mu.Unlock()
	w = doReq(t, s, "POST", "/api/command", commandBody(`/start "de-cuong.md"`))
	if w.Code != 409 {
		t.Fatalf("start on writing status = %d: %s", w.Code, w.Body.String())
	}
}

// G5：worker 叠加导入待确认与共创等待原因。
func TestSnapshotWaitingForOverlay(t *testing.T) {
	h := newLiveFakeHost()
	h.snapshot = host.UISnapshot{WaitingFor: "input"}
	s := newLiveServer(h)

	if got := s.snapshot().WaitingFor; got != "input" {
		t.Fatalf("base waitingFor = %q", got)
	}

	s.jobs.record(jobEvent{Kind: "import", Awaiting: "confirm"})
	if got := s.snapshot().WaitingFor; got != "confirm" {
		t.Fatalf("confirm waitingFor = %q", got)
	}

	s.co.session = nil
	s.jobs.record(jobEvent{Kind: "import", Awaiting: "", Done: true})
	h.snapshot = host.UISnapshot{WaitingFor: "input"}
	s.co.mu.Lock()
	s.co.session = startup.NewCoCreateSession("gợi ý")
	s.co.mu.Unlock()
	if got := s.snapshot().WaitingFor; got != "cocreate" {
		t.Fatalf("cocreate waitingFor = %q", got)
	}
}
