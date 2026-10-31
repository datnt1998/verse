package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/platform/identity"
)

// Nguồn: 07-data-contract §23 + 01-ia-flows W13 — hình JSON của /api/storage*.

type fakeStorage struct {
	snap      storage.Snapshot
	check     storage.CheckResult
	checkErr  error
	taskID    string
	done      bool
	moveErr   error
	delErr    error
	useErr    error
	browse    storage.BrowseResult
	browseErr error
	openErr   error
	// ghi lại tham số để test route
	lastCheck struct {
		path, mode string
	}
	lastMove struct {
		path, mode string
	}
	lastCancel string
}

func (f *fakeStorage) Snapshot(context.Context) (storage.Snapshot, error) { return f.snap, nil }
func (f *fakeStorage) Check(_ context.Context, path, mode string) (storage.CheckResult, error) {
	f.lastCheck.path, f.lastCheck.mode = path, mode
	return f.check, f.checkErr
}
func (f *fakeStorage) Move(_ context.Context, path, mode string) (string, bool, error) {
	f.lastMove.path, f.lastMove.mode = path, mode
	return f.taskID, f.done, f.moveErr
}
func (f *fakeStorage) CancelMove(_ context.Context, taskID string) error {
	f.lastCancel = taskID
	return nil
}

func (f *fakeStorage) RetryMove(context.Context) (string, bool, error) {
	return "t_retry", false, nil
}

func (f *fakeStorage) DismissMove(context.Context) error      { return nil }
func (f *fakeStorage) DeletePrevious(context.Context) error   { return f.delErr }
func (f *fakeStorage) UseDefaultForNew(context.Context) error { return f.useErr }
func (f *fakeStorage) Recheck(context.Context) (bool, error)  { return true, nil }
func (f *fakeStorage) Browse(context.Context, string) (storage.BrowseResult, error) {
	return f.browse, f.browseErr
}
func (f *fakeStorage) Open(context.Context) error { return f.openErr }

type fakeCleanup struct {
	items     []retention.ReclaimItem
	preview   *retention.Preview
	runErr    error
	previewID string
}

func (f *fakeCleanup) ReclaimList(context.Context) ([]retention.ReclaimItem, error) {
	return f.items, nil
}
func (f *fakeCleanup) Reclaim(_ context.Context, id string) (retention.ReclaimItem, error) {
	return retention.ReclaimItem{ID: id, Title: "Video cũ", Bytes: 10, Count: 1}, nil
}
func (f *fakeCleanup) PreviewCleanup(context.Context) (*retention.Preview, error) {
	return f.preview, nil
}
func (f *fakeCleanup) RunCleanup(_ context.Context, id string) (retention.RunResult, error) {
	f.previewID = id
	return retention.RunResult{Removed: 2, Freed: 20}, f.runErr
}

func newStorageHandlers(t *testing.T) (*testGateway, *fakeStorage, *fakeCleanup, string) {
	t.Helper()
	tg := newTestGateway(t)
	store := &fakeStorage{}
	clean := &fakeCleanup{}
	MountStorage(tg.g, store, clean)
	return tg, store, clean, tg.sessionCookie()
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("JSON: %v — %s", err, body)
	}
}

func TestStorageGetShapesPreviousAndSwitch(t *testing.T) {
	tg, store, _, cookie := newStorageHandlers(t)
	after := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 10, 30, 22, 14, 0, 0, time.UTC)
	store.snap = storage.Snapshot{
		Projects: storage.Location{
			Path: "/Volumes/T7/Viderse Projects", UsedBytes: 3 << 30,
			Online: true,
			Volume: storage.Volume{Name: "T7", Kind: storage.VolumeExternal, FS: "APFS"},
			Previous: &domain.StoragePrevious{
				Path: "/Users/dat/.viderse/projects", Bytes: 2 << 30,
				DeleteAfter: after, Mode: storage.ModeMove,
			},
			LastSwitch: &domain.StorageSwitch{Mode: storage.ModeMove, At: runAt},
		},
		StayBytes:    1 << 30,
		Suggestions:  []storage.Suggestion{{Path: "/Volumes/T7/Viderse Projects"}},
		Usage:        []storage.UsageGroup{{ID: "images", Bytes: 1 << 30}},
		CleanLastRun: &runAt,
	}
	w := tg.do(http.MethodGet, "/api/storage", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	projects := body["projects"].(map[string]any)
	prev := projects["previous"].(map[string]any)
	if prev["mode"] != "move" || prev["deleteAfter"] != "2026-11-06T12:00:00Z" {
		t.Fatalf("previous = %v", prev)
	}
	if last := projects["lastSwitch"].(map[string]any); last["mode"] != "move" {
		t.Fatalf("lastSwitch = %v", last)
	}
	if body["clean"].(map[string]any)["lastRunAt"] != "2026-10-30T22:14:00Z" {
		t.Fatalf("clean = %v", body["clean"])
	}
	if usage := body["usage"].([]any); len(usage) != 1 || usage[0].(map[string]any)["id"] != "images" {
		t.Fatalf("usage = %v", body["usage"])
	}
}

func TestStorageGetNewOnlyPreviousHasNoDeleteAfter(t *testing.T) {
	tg, store, _, cookie := newStorageHandlers(t)
	store.snap = storage.Snapshot{
		Projects: storage.Location{
			Path: "/Volumes/T7/Viderse Projects", Online: true,
			Volume:     storage.Volume{Name: "T7", Kind: storage.VolumeExternal},
			Previous:   &domain.StoragePrevious{Path: "/Users/dat/.viderse/projects", Mode: storage.ModeNewOnly},
			LastSwitch: &domain.StorageSwitch{Mode: storage.ModeNewOnly, At: time.Now()},
		},
	}
	w := tg.do(http.MethodGet, "/api/storage", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	prev := body["projects"].(map[string]any)["previous"].(map[string]any)
	if _, ok := prev["deleteAfter"]; ok {
		t.Fatalf("new_only không được có deleteAfter: %v", prev)
	}
	if prev["mode"] != "new_only" {
		t.Fatalf("mode = %v", prev["mode"])
	}
}

func TestStorageErrorCodes(t *testing.T) {
	tg, store, clean, cookie := newStorageHandlers(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		setup  func()
		status int
		code   string
	}{
		{"check volume gone", http.MethodPost, "/api/storage/check", `{"path":"/Volumes/T7","mode":"move"}`,
			func() { store.checkErr = &storage.Error{Code: storage.ErrVolumeGone, Message: "Ổ chưa cắm"} },
			http.StatusNotFound, "volume_gone"},
		{"delete new_only previous", http.MethodPost, "/api/storage/previous/delete", `{}`,
			func() { store.delErr = &storage.Error{Code: storage.ErrNotDeletable, Message: "vẫn đang dùng"} },
			http.StatusConflict, "not_deletable"},
		{"run without preview", http.MethodPost, "/api/storage/cleanup/run", `{"previewId":"old"}`,
			func() { clean.runErr = &storage.Error{Code: storage.ErrPreviewRequired, Message: "xem lại"} },
			http.StatusConflict, "preview_required"},
		{"browse missing", http.MethodGet, "/api/storage/browse?path=/nope", ``,
			func() { store.browseErr = &storage.Error{Code: storage.ErrNotFound, Message: "không có"} },
			http.StatusNotFound, "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			w := tg.do(tc.method, tc.path, cookie, tc.body)
			if w.Code != tc.status {
				t.Fatalf("status = %d, muốn %d: %s", w.Code, tc.status, w.Body)
			}
			var body map[string]any
			decodeJSON(t, w.Body.Bytes(), &body)
			if body["code"] != tc.code {
				t.Fatalf("code = %v, muốn %s", body["code"], tc.code)
			}
		})
	}
}

func TestStorageMoveShapes(t *testing.T) {
	tg, store, _, cookie := newStorageHandlers(t)
	store.taskID, store.done = "t_1", false
	w := tg.do(http.MethodPost, "/api/storage/move", cookie, `{"path":"/Volumes/T7","mode":"move"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	if body["taskId"] != "t_1" || body["done"] != false {
		t.Fatalf("body = %v", body)
	}
	if store.lastMove.path != "/Volumes/T7" || store.lastMove.mode != "move" {
		t.Fatalf("tham số = %+v", store.lastMove)
	}
	// Huỷ qua đường dẫn con.
	if w := tg.do(http.MethodPost, "/api/storage/move/t_1/cancel", cookie, `{}`); w.Code != http.StatusOK {
		t.Fatalf("cancel status %d: %s", w.Code, w.Body)
	}
	if store.lastCancel != "t_1" {
		t.Fatalf("cancel = %q", store.lastCancel)
	}
	// "Chỉ video mới" trả done ngay khi đã ở đích.
	store.taskID, store.done = "", true
	w = tg.do(http.MethodPost, "/api/storage/move", cookie, `{"path":"/Volumes/T7","mode":"new_only"}`)
	decodeJSON(t, w.Body.Bytes(), &body)
	if body["done"] != true {
		t.Fatalf("body = %v", body)
	}
}

func TestStorageOpenUnsupported(t *testing.T) {
	tg, store, _, cookie := newStorageHandlers(t)
	store.openErr = &storage.Error{Code: storage.ErrNotSupported, Message: "không mở được trên máy chủ này"}
	w := tg.do(http.MethodPost, "/api/storage/open", cookie, `{}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	if body["code"] != "not_supported" {
		t.Fatalf("code = %v", body["code"])
	}
}

func TestStorageMoveInfoWireAndActions(t *testing.T) {
	tg, store, _, cookie := newStorageHandlers(t)
	at := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	store.snap.Move = storage.MoveInfo{
		Active: false,
		Failed: &domain.StorageMoveFailed{TaskID: "t_old", Target: "/Volumes/T7/Viderse Projects", Phase: "Chuyển không xong", At: at},
	}
	w := tg.do(http.MethodGet, "/api/storage", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	move := body["move"].(map[string]any)
	if move["active"] != false {
		t.Fatalf("active = %v", move["active"])
	}
	failed := move["failed"].(map[string]any)
	if failed["target"] != "/Volumes/T7/Viderse Projects" || failed["taskId"] != "t_old" {
		t.Fatalf("failed = %v", failed)
	}
	// Retry + dismiss endpoints.
	w = tg.do(http.MethodPost, "/api/storage/move/retry", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("retry status %d: %s", w.Code, w.Body)
	}
	var res map[string]any
	decodeJSON(t, w.Body.Bytes(), &res)
	if res["taskId"] != "t_retry" || res["done"] != false {
		t.Fatalf("retry = %v", res)
	}
	w = tg.do(http.MethodPost, "/api/storage/move/dismiss", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("dismiss status %d: %s", w.Code, w.Body)
	}
}

func TestStorageReclaimAndCleanup(t *testing.T) {
	tg, _, clean, cookie := newStorageHandlers(t)
	clean.items = []retention.ReclaimItem{
		{ID: "tmp", Title: "Tệp tạm", Bytes: 100, Count: 2, Desc: "còn 7 ngày"},
	}
	w := tg.do(http.MethodGet, "/api/storage/reclaim", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var list struct {
		Items []struct {
			ID    string `json:"id"`
			Bytes int64  `json:"bytes"`
			Count int    `json:"count"`
		} `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].ID != "tmp" || list.Items[0].Bytes != 100 {
		t.Fatalf("items = %+v", list.Items)
	}
	clean.preview = &retention.Preview{
		ID: "p_1", Freed: 100,
		Items: []retention.PreviewItem{{Kind: "temp-dir", Label: "Tệp tạm", Count: 2, Bytes: 100}},
	}
	w = tg.do(http.MethodPost, "/api/storage/cleanup/preview", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("preview status %d: %s", w.Code, w.Body)
	}
	var prev struct {
		ID    string `json:"previewId"`
		Freed int64  `json:"freedBytes"`
		Items []struct {
			Kind  string `json:"kind"`
			Count int    `json:"count"`
		} `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &prev)
	if prev.ID != "p_1" || prev.Freed != 100 || len(prev.Items) != 1 || prev.Items[0].Kind != "temp-dir" {
		t.Fatalf("preview = %+v", prev)
	}
	w = tg.do(http.MethodPost, "/api/storage/cleanup/run", cookie, `{"previewId":"p_1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("run status %d: %s", w.Code, w.Body)
	}
	if clean.previewID != "p_1" {
		t.Fatalf("previewId = %q", clean.previewID)
	}
}

// Artboard in "~/Library/…" chứ không in /Users/<tên>; path ngoài home giữ nguyên.
func TestTildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	cases := []struct{ in, want string }{
		{home, "~"},
		{home + "/Library/Application Support/Viderse/data/projects", "~/Library/Application Support/Viderse/data/projects"},
		{home + "x/file", home + "x/file"}, // tiền tố gần giống nhưng khác thư mục
		{"/Volumes/Samsung T7/Viderse Projects", "/Volumes/Samsung T7/Viderse Projects"},
		{"", ""},
	}
	for _, c := range cases {
		if got := tildePath(c.in); got != c.want {
			t.Fatalf("tildePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

type spyFilesystem struct{ calls int }

func (f *spyFilesystem) Open(string) (fs.File, error) {
	f.calls++
	return nil, fs.ErrPermission
}

type untouchedStorage struct {
	StorageService
	fs *spyFilesystem
}

func (s untouchedStorage) Snapshot(context.Context) (storage.Snapshot, error) {
	_, err := s.fs.Open("/")
	return storage.Snapshot{}, err
}
func (s untouchedStorage) Check(context.Context, string, string) (storage.CheckResult, error) {
	_, err := s.fs.Open("/")
	return storage.CheckResult{}, err
}
func (s untouchedStorage) Move(context.Context, string, string) (string, bool, error) {
	_, err := s.fs.Open("/")
	return "", false, err
}
func (s untouchedStorage) CancelMove(context.Context, string) error {
	_, err := s.fs.Open("/")
	return err
}
func (s untouchedStorage) RetryMove(context.Context) (string, bool, error) {
	_, err := s.fs.Open("/")
	return "", false, err
}
func (s untouchedStorage) DismissMove(context.Context) error    { _, err := s.fs.Open("/"); return err }
func (s untouchedStorage) DeletePrevious(context.Context) error { _, err := s.fs.Open("/"); return err }
func (s untouchedStorage) UseDefaultForNew(context.Context) error {
	_, err := s.fs.Open("/")
	return err
}
func (s untouchedStorage) Recheck(context.Context) (bool, error) {
	_, err := s.fs.Open("/")
	return false, err
}
func (s untouchedStorage) Browse(context.Context, string) (storage.BrowseResult, error) {
	_, err := s.fs.Open("/")
	return storage.BrowseResult{}, err
}
func (s untouchedStorage) Open(context.Context) error { _, err := s.fs.Open("/"); return err }

type untouchedCleanup struct {
	CleanupService
	fs *spyFilesystem
}

func (s untouchedCleanup) ReclaimList(context.Context) ([]retention.ReclaimItem, error) {
	_, err := s.fs.Open("/")
	return nil, err
}
func (s untouchedCleanup) Reclaim(context.Context, string) (retention.ReclaimItem, error) {
	_, err := s.fs.Open("/")
	return retention.ReclaimItem{}, err
}

func TestServerStorageRefusesPathOperationsBeforeIO(t *testing.T) {
	tg := newTestGateway(t)
	filesystem := &spyFilesystem{}
	MountStorage(tg.g, untouchedStorage{fs: filesystem}, untouchedCleanup{fs: filesystem})
	tg.g.SetServerStorageMode(func() bool { return true })
	cookie := tg.sessionCookie()
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/storage"}, {"GET", "/api/storage/browse?path=/"},
		{"POST", "/api/storage/open"}, {"POST", "/api/storage/check"},
		{"POST", "/api/storage/recheck"}, {"POST", "/api/storage/move"},
		{"POST", "/api/storage/move/retry"}, {"POST", "/api/storage/move/dismiss"},
		{"POST", "/api/storage/move/task/cancel"},
		{"POST", "/api/storage/previous/delete"}, {"POST", "/api/storage/use-default-for-new"},
		{"GET", "/api/storage/reclaim"}, {"POST", "/api/storage/reclaim/location"},
	} {
		t.Run(route.path, func(t *testing.T) {
			w := tg.do(route.method, route.path, cookie, "{}")
			if w.Code != http.StatusForbidden {
				t.Fatalf("path operation not refused: status=%d body=%s", w.Code, w.Body)
			}
			var body struct {
				Code string `json:"code"`
			}
			decodeJSON(t, w.Body.Bytes(), &body)
			if body.Code != "storage_path_unavailable" {
				t.Fatalf("unexpected error: %s", w.Body)
			}
			if filesystem.calls != 0 {
				t.Fatalf("filesystem accessed %d times", filesystem.calls)
			}
		})
	}
}

type cleanupViewerSpy struct {
	fakeCleanup
	viewer  retention.CleanupViewer
	present bool
	runErr  error
}

func (s *cleanupViewerSpy) PreviewCleanup(ctx context.Context) (*retention.Preview, error) {
	s.viewer, s.present = retention.Viewer(ctx)
	return &retention.Preview{ID: "preview"}, nil
}

func (s *cleanupViewerSpy) RunCleanup(ctx context.Context, _ string) (retention.RunResult, error) {
	s.viewer, s.present = retention.Viewer(ctx)
	return retention.RunResult{}, s.runErr
}

func TestCleanupHandlersForwardAuthenticatedViewerAndForbidden(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleAdmin, identity.RoleMember} {
		t.Run(string(role), func(t *testing.T) {
			clean := &cleanupViewerSpy{}
			h := storageHandlers{clean: clean}
			user := identity.User{Username: "actor", Role: role}
			preview := httptest.NewRecorder()
			h.preview(preview, httptest.NewRequest(http.MethodPost, "/api/storage/cleanup/preview", nil), user)
			if preview.Code != http.StatusOK {
				t.Fatalf("preview: %d %s", preview.Code, preview.Body)
			}
			if !clean.present || clean.viewer.Actor != "actor" || clean.viewer.Admin != (role == identity.RoleAdmin) {
				t.Fatalf("preview lost authenticated cleanup viewer: present=%v viewer=%+v", clean.present, clean.viewer)
			}
			clean.present = false
			run := httptest.NewRecorder()
			h.run(run, httptest.NewRequest(http.MethodPost, "/api/storage/cleanup/run", strings.NewReader(`{"previewId":"preview"}`)), user)
			if run.Code != http.StatusOK {
				t.Fatalf("run: %d %s", run.Code, run.Body)
			}
			if !clean.present || clean.viewer.Actor != "actor" || clean.viewer.Admin != (role == identity.RoleAdmin) {
				t.Fatalf("run lost authenticated cleanup viewer: present=%v viewer=%+v", clean.present, clean.viewer)
			}
			clean.runErr = &serverstorage.Error{Status: http.StatusForbidden, Message: "Chỉ quản trị viên được dọn bộ đệm"}
			forbidden := httptest.NewRecorder()
			h.run(forbidden, httptest.NewRequest(http.MethodPost, "/api/storage/cleanup/run", strings.NewReader(`{"previewId":"preview"}`)), user)
			if forbidden.Code != http.StatusForbidden {
				t.Fatalf("cleanup denial not preserved: %d %s", forbidden.Code, forbidden.Body)
			}
		})
	}
}
