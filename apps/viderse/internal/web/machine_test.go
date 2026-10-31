// Hợp đồng HTTP của mảng máy: auth, hình dữ liệu, mã lỗi. Mock service ở
// phía test; logic thật đã có test riêng ở từng gói.
package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/setup"
	"github.com/datnt1998/verse/apps/viderse/internal/toolsvc"
)

type fakeActivity struct {
	snap activity.Snapshot
	read []string
}

func (f *fakeActivity) Snapshot(context.Context) (activity.Snapshot, error) { return f.snap, nil }
func (f *fakeActivity) MarkRead(_ context.Context, ids []string) error {
	f.read = ids
	return nil
}
func (f *fakeActivity) ErrorsCount(context.Context) (int, error) { return 1, nil }

func (f *fakeActivity) Errors(context.Context) ([]activity.ErrorItem, error) {
	return []activity.ErrorItem{{
		ID: "t1", At: time.Now(), What: "Xử lý hình và tiếng", Why: "Mạng bị ngắt",
		Status: "Cần xử lý", Href: "#/settings?section=tools", Link: "Thử lại",
		Object: &domain.ObjectRef{Type: "tool", ID: "ffmpeg"},
		Action: domain.TaskAction{Op: domain.TaskOpRetry}, Kind: "task_failed",
	}}, nil
}

type fakeTools struct {
	snap    toolsvc.Snapshot
	started []string
}

func (f *fakeTools) Snapshot(context.Context, bool) (toolsvc.Snapshot, error) { return f.snap, nil }
func (f *fakeTools) StartInstall(_ context.Context, id toolsvc.ToolID) (string, error) {
	f.started = append(f.started, id)
	return "task-1", nil
}
func (f *fakeTools) Cancel(context.Context, toolsvc.ToolID) error { return nil }
func (f *fakeTools) DataDir() string                              { return "/tmp/viderse" }

type fakeSetup struct {
	status setup.Status
	retry  string
}

func (f *fakeSetup) Status(context.Context, bool) (setup.Status, error) { return f.status, nil }
func (f *fakeSetup) Start(context.Context) (setup.Status, error)        { return f.status, nil }
func (f *fakeSetup) Retry(_ context.Context, id domain.SetupStepID) (setup.Status, error) {
	if id != domain.SetupStepChrome {
		return setup.Status{}, errFakeStep
	}
	f.retry = id
	return f.status, nil
}

var errFakeStep = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "bước không hợp lệ" }

type fakeLogs struct {
	entries []logs.Entry
	cursor  *time.Time
	bundle  *logs.Bundle
}

func (f *fakeLogs) Read(string, string, int, time.Time) ([]logs.Entry, *time.Time, error) {
	return f.entries, f.cursor, nil
}
func (f *fakeLogs) Bundle(context.Context) (*logs.Bundle, error) { return f.bundle, nil }

type fakeVersions struct{}

func (fakeVersions) Get(context.Context) logs.VersionInfo {
	latest := "1.5.0"
	return logs.VersionInfo{Version: "1.4.0", Latest: &latest, CheckedAt: time.Now()}
}
func (f fakeVersions) Check(ctx context.Context) logs.VersionInfo { return f.Get(ctx) }

func TestMachineRoutesRequireSession(t *testing.T) {
	tg := newTestGateway(t)
	MountActivity(tg.g, &fakeActivity{})
	MountTools(tg.g, &fakeTools{})
	MountSetup(tg.g, &fakeSetup{})
	MountLogs(tg.g, &fakeLogs{}, fakeVersions{})

	for _, path := range []string{
		"/api/activity", "/api/errors", "/api/tools",
		"/api/tools/logs", "/api/tools/logs/export", "/api/version", "/api/setup/state",
	} {
		if w := tg.do(http.MethodGet, path, "", ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s = %d, muốn 401", path, w.Code)
		}
	}
	if w := tg.do(http.MethodPost, "/api/setup/start", "", "{}"); w.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/setup/start = %d, muốn 401", w.Code)
	}
}

func TestActivityWireShape(t *testing.T) {
	tg := newTestGateway(t)
	done, total := int64(3), int64(10)
	svc := &fakeActivity{snap: activity.Snapshot{
		Tasks: []activity.Task{{
			ID: "t1", Kind: domain.TaskSetup, Title: "Công cụ vẽ ảnh", Phase: "Đang tải",
			Object:   domain.ObjectRef{Type: "tool", ID: "codex", Label: "Công cụ vẽ ảnh"},
			Progress: &domain.TaskProgress{Done: done, Total: total},
			State:    domain.TaskRunning,
			Action:   domain.TaskAction{Label: "Huỷ", Op: domain.TaskOpCancel},
		}},
		VideoJobs: 2, Queued: 1,
		Unread: []activity.Notification{{ID: "n1", Kind: "needs_attention", Count: 3,
			Text: "Không thấy ổ T7", Href: "#/settings?section=storage", ActionLabel: "Mở",
			CreatedAt: time.Now()}},
	}}
	MountActivity(tg.g, svc)
	cookie := tg.sessionCookie()
	body := decode(t, tg.do(http.MethodGet, "/api/activity", cookie, ""))

	if got := body["videoJobs"].(float64); got != 2 {
		t.Fatalf("videoJobs = %v", got)
	}
	if got := body["queued"].(float64); got != 1 {
		t.Fatalf("queued = %v", got)
	}
	tasks := body["running"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("running = %v", tasks)
	}
	task := tasks[0].(map[string]any)
	if task["id"] != "t1" || task["state"] != "running" {
		t.Fatalf("task = %v", task)
	}
	progress := task["progress"].(map[string]any)
	if progress["done"].(float64) != 3 || progress["total"].(float64) != 10 {
		t.Fatalf("progress = %v", progress)
	}
	if task["objectRef"].(map[string]any)["type"] != "tool" {
		t.Fatalf("objectRef = %v", task["objectRef"])
	}
	action := task["action"].(map[string]any)
	if action["op"] != "cancel" || action["label"] != "Huỷ" {
		t.Fatalf("action = %v", action)
	}
	unread := body["unread"].([]any)[0].(map[string]any)
	actions := unread["actions"].([]any)
	if unread["count"].(float64) != 3 || len(actions) != 1 || actions[0].(map[string]any)["href"] != "#/settings?section=storage" {
		t.Fatalf("unread = %v", unread)
	}
	if _, ok := unread["readAt"]; !ok {
		t.Fatal("readAt phải có mặt (null)")
	}
}

func TestActivityMarkReadAllAndErrorsShape(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakeActivity{}
	MountActivity(tg.g, svc)
	cookie := tg.sessionCookie()

	if w := tg.do(http.MethodPost, "/api/notifications/read", cookie, `{"all":true}`); w.Code != http.StatusOK {
		t.Fatalf("mark all = %d %s", w.Code, w.Body)
	}
	if svc.read != nil {
		t.Fatalf("all phải gửi nil, nhận %v", svc.read)
	}
	if w := tg.do(http.MethodPost, "/api/notifications/read", cookie, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("rỗng = %d, muốn 400", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/notifications/read", cookie, `{"ids":["a"]}`); w.Code != http.StatusOK {
		t.Fatalf("theo ids = %d", w.Code)
	}
	if len(svc.read) != 1 || svc.read[0] != "a" {
		t.Fatalf("ids = %v", svc.read)
	}

	body := decode(t, tg.do(http.MethodGet, "/api/errors", cookie, ""))
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v", items)
	}
	item := items[0].(map[string]any)
	if item["tone"] != "warn" || item["op"] != "retry" || item["what"] != "Xử lý hình và tiếng" {
		t.Fatalf("item = %v", item)
	}
	if body["count7d"].(float64) != 1 {
		t.Fatalf("count7d = %v", body["count7d"])
	}
}

func TestToolsWireShapeAndActions(t *testing.T) {
	tg := newTestGateway(t)
	total := int64(100)
	svc := &fakeTools{snap: toolsvc.Snapshot{
		CheckedAt: time.Now(),
		Tools: []toolsvc.Row{
			{ID: toolsvc.ToolYtDlp, State: toolsvc.StateUpdate, Version: "2025.1.1",
				Hint: "Có bản 2026.1.1", Update: &toolsvc.UpdateInfo{Current: "2025.1.1", Latest: "2026.1.1"}},
			{ID: toolsvc.ToolChrome, State: toolsvc.StateInstalling, Hint: "Đang cài…",
				Progress: &domain.InstallProgress{Phase: "downloading", BytesDone: 50, BytesTotal: &total}},
		},
	}}
	MountTools(tg.g, svc)
	cookie := tg.sessionCookie()
	body := decode(t, tg.do(http.MethodGet, "/api/tools", cookie, ""))
	rows := body["tools"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	first := rows[0].(map[string]any)
	if first["state"] != "update" || first["update"].(map[string]any)["latest"] != "2026.1.1" {
		t.Fatalf("row 0 = %v", first)
	}
	progress := rows[1].(map[string]any)["progress"].(map[string]any)
	if progress["pct"].(float64) != 50 || progress["phase"] != "downloading" {
		t.Fatalf("progress = %v", progress)
	}

	w := tg.do(http.MethodPost, "/api/tools/yt-dlp/install", cookie, "{}")
	if w.Code != http.StatusAccepted {
		t.Fatalf("install = %d %s", w.Code, w.Body)
	}
	if body := decode(t, w); body["taskId"] != "task-1" {
		t.Fatalf("install body = %v", body)
	}
	if len(svc.started) != 1 || svc.started[0] != "yt-dlp" {
		t.Fatalf("started = %v", svc.started)
	}
	if w := tg.do(http.MethodPost, "/api/tools/nope/install", cookie, "{}"); w.Code != http.StatusNotFound {
		t.Fatalf("id lạ = %d, muốn 404", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/tools/yt-dlp/whatever", cookie, "{}"); w.Code != http.StatusNotFound {
		t.Fatalf("action lạ = %d, muốn 404", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/tools/chrome/cancel", cookie, "{}"); w.Code != http.StatusOK {
		t.Fatalf("cancel = %d", w.Code)
	}
}

func TestSetupWireShapeAndRetry(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakeSetup{status: setup.Status{
		State: setup.OverallLogin, Complete: false, Running: false,
		Steps: []setup.Step{
			{ID: domain.SetupStepYtDlp, State: setup.StepDone},
			{ID: domain.SetupStepFfmpeg, State: setup.StepFailed, Error: "Mạng bị ngắt"},
			{ID: domain.SetupStepCodexLogin, State: setup.StepNeedsUser},
		},
		Disk: setup.Disk{FreeBytes: 100, RequiredBytes: 40, Enough: true},
	}}
	MountSetup(tg.g, svc)
	cookie := tg.sessionCookie()
	body := decode(t, tg.do(http.MethodGet, "/api/setup/state?fresh=1", cookie, ""))
	if body["state"] != "login" || body["running"] != false {
		t.Fatalf("body = %v", body)
	}
	steps := body["steps"].([]any)
	if steps[1].(map[string]any)["error"] != "Mạng bị ngắt" {
		t.Fatalf("steps = %v", steps)
	}
	if body["disk"].(map[string]any)["enough"] != true {
		t.Fatalf("disk = %v", body["disk"])
	}
	if w := tg.do(http.MethodPost, "/api/setup/start", cookie, "{}"); w.Code != http.StatusAccepted {
		t.Fatalf("start = %d", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/setup/retry", cookie, `{"id":"chrome"}`); w.Code != http.StatusAccepted {
		t.Fatalf("retry = %d %s", w.Code, w.Body)
	}
	if svc.retry != domain.SetupStepChrome {
		t.Fatalf("retry id = %q", svc.retry)
	}
	if w := tg.do(http.MethodPost, "/api/setup/retry", cookie, `{"id":"chrome-login"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("retry bước lạ = %d, muốn 400", w.Code)
	}
}

func TestLogsWireShapeCursorAndExport(t *testing.T) {
	tg := newTestGateway(t)
	oldest := time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC)
	svc := &fakeLogs{
		entries: []logs.Entry{
			{TS: oldest, Tool: "ffmpeg", Level: "error", Message: "hỏng"},
			{TS: oldest.Add(time.Minute), Tool: "ffmpeg", Level: "info", Message: "xong"},
		},
		cursor: &oldest,
		bundle: &logs.Bundle{FileName: "viderse-logs-x.zip", Bytes: []byte("PK"), Files: 1},
	}
	MountLogs(tg.g, svc, fakeVersions{})
	cookie := tg.sessionCookie()

	body := decode(t, tg.do(http.MethodGet, "/api/tools/logs?tool=ffmpeg&level=warn&limit=2", cookie, ""))
	entries := body["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	// Mới nhất trước.
	if entries[0].(map[string]any)["message"] != "xong" {
		t.Fatalf("thứ tự = %v", entries)
	}
	if body["nextCursor"] == nil || body["nextBefore"] == nil {
		t.Fatalf("thiếu cursor: %v", body)
	}
	if _, err := base64.RawURLEncoding.DecodeString(body["nextCursor"].(string)); err != nil {
		t.Fatalf("cursor không phải base64url: %v", err)
	}
	if w := tg.do(http.MethodGet, "/api/tools/logs?cursor=!!!", cookie, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("cursor hỏng = %d, muốn 400", w.Code)
	}
	if w := tg.do(http.MethodGet, "/api/tools/logs?level=nope", cookie, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("level lạ = %d, muốn 400", w.Code)
	}
	if w := tg.do(http.MethodGet, "/api/tools/logs?limit=abc", cookie, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("limit lạ = %d, muốn 400", w.Code)
	}

	w := tg.do(http.MethodGet, "/api/tools/logs/export", cookie, "")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export = %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "viderse-logs-x.zip") {
		t.Fatalf("disposition = %q", w.Header().Get("Content-Disposition"))
	}

	ver := decode(t, tg.do(http.MethodGet, "/api/version", cookie, ""))
	if ver["version"] != "1.4.0" || ver["latest"] != "1.5.0" {
		t.Fatalf("version = %v", ver)
	}
	if w := tg.do(http.MethodPost, "/api/version/check", cookie, "{}"); w.Code != http.StatusOK {
		t.Fatalf("check = %d", w.Code)
	}
}

// TestLogsExportBundleIsZip guards the real store + handler path together.
func TestLogsExportBundleIsZip(t *testing.T) {
	tg := newTestGateway(t)
	store := logs.New(t.TempDir(), "")
	t.Cleanup(func() { _ = store.Close() })
	store.Append("ffmpeg", logs.LevelError, "hỏng")
	MountLogs(tg.g, realLogs{store: store}, fakeVersions{})
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodGet, "/api/tools/logs/export", cookie, "")
	if _, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len())); err != nil {
		t.Fatalf("không phải zip: %v (%d byte)", err, w.Body.Len())
	}
}

type realLogs struct{ store *logs.Store }

func (r realLogs) Read(tool, level string, limit int, before time.Time) ([]logs.Entry, *time.Time, error) {
	return r.store.Read(tool, level, limit, before)
}
func (r realLogs) Bundle(ctx context.Context) (*logs.Bundle, error) {
	return r.store.BuildBundle(ctx, logs.BundleOptions{Version: "dev"})
}

var _ = json.Valid
