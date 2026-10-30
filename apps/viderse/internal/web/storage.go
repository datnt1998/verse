package web

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// Lưu trữ (§23 + hỏi đáp #10): handler chỉ đọc yêu cầu → gọi service → ghi
// JSON. Kiểu wire khai ở đây để service tiến hoá không ngầm đổi hợp đồng.

// StorageService là phần storage mà HTTP cần.
type StorageService interface {
	Snapshot(ctx context.Context) (storage.Snapshot, error)
	Check(ctx context.Context, path, mode string) (storage.CheckResult, error)
	Move(ctx context.Context, path, mode string) (taskID string, done bool, err error)
	CancelMove(ctx context.Context, taskID string) error
	RetryMove(ctx context.Context) (taskID string, done bool, err error)
	DismissMove(ctx context.Context) error
	DeletePrevious(ctx context.Context) error
	UseDefaultForNew(ctx context.Context) error
	Recheck(ctx context.Context) (bool, error)
	Browse(ctx context.Context, path string) (storage.BrowseResult, error)
	Open(ctx context.Context) error
}

// CleanupService là phần dọn dẹp (retention) mà HTTP cần.
type CleanupService interface {
	ReclaimList(ctx context.Context) ([]retention.ReclaimItem, error)
	Reclaim(ctx context.Context, id string) (retention.ReclaimItem, error)
	PreviewCleanup(ctx context.Context) (*retention.Preview, error)
	RunCleanup(ctx context.Context, previewID string) (retention.RunResult, error)
}

// MountStorage đăng ký các route /api/storage*.
func MountStorage(g *Gateway, store StorageService, clean CleanupService) {
	h := &storageHandlers{store: store, clean: clean}
	// Thứ tự khớp: exact trước prefix (webapp giữ thứ tự đăng ký).
	g.Handle(http.MethodGet, "/api/storage", h.get)
	g.Handle(http.MethodPost, "/api/storage/check", h.check)
	g.Handle(http.MethodPost, "/api/storage/recheck", h.recheck)
	g.Handle(http.MethodPost, "/api/storage/move", h.move)
	g.Handle(http.MethodPost, "/api/storage/move/retry", h.moveRetry)
	g.Handle(http.MethodPost, "/api/storage/move/dismiss", h.moveDismiss)
	g.Handle(http.MethodPost, "/api/storage/move/", h.moveSub)
	g.Handle(http.MethodPost, "/api/storage/previous/delete", h.deletePrevious)
	g.Handle(http.MethodPost, "/api/storage/use-default-for-new", h.useDefault)
	g.Handle(http.MethodGet, "/api/storage/browse", h.browse)
	g.Handle(http.MethodPost, "/api/storage/open", h.open)
	g.Handle(http.MethodGet, "/api/storage/reclaim", h.reclaimList)
	g.Handle(http.MethodPost, "/api/storage/reclaim/", h.reclaimOne)
	g.Handle(http.MethodPost, "/api/storage/cleanup/preview", h.preview)
	g.Handle(http.MethodPost, "/api/storage/cleanup/run", h.run)
}

type storageHandlers struct {
	store StorageService
	clean CleanupService
}

/* -------------------------------- wire -------------------------------- */

type volumeWire struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FS        string `json:"fs"`
	FreeBytes int64  `json:"freeBytes"`
	Bus       string `json:"bus,omitempty"`
}

type previousWire struct {
	Path        string  `json:"path"`
	Bytes       int64   `json:"bytes"`
	DeleteAfter *string `json:"deleteAfter,omitempty"`
	Mode        string  `json:"mode"`
}

type switchWire struct {
	Mode string `json:"mode"`
	At   string `json:"at"`
}

type locationWire struct {
	Path string `json:"path"`
	// DisplayPath là path đã rút gọn thư mục nhà thành "~" cho UI — artboard
	// in "~/Library/Application Support/Viderse/data/projects", không in /Users.
	DisplayPath  string        `json:"displayPath,omitempty"`
	Volume       volumeWire    `json:"volume"`
	UsedBytes    int64         `json:"usedBytes"`
	Online       bool          `json:"online"`
	OfflineSince *string       `json:"offlineSince,omitempty"`
	Previous     *previousWire `json:"previous,omitempty"`
	LastSwitch   *switchWire   `json:"lastSwitch,omitempty"`
}

type suggestionWire struct {
	Path   string     `json:"path"`
	Volume volumeWire `json:"volume"`
}

type usageWire struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
}

type failedMoveWire struct {
	TaskID string `json:"taskId,omitempty"`
	Target string `json:"target"`
	Phase  string `json:"phase"`
	At     string `json:"at"`
}

type moveWire struct {
	Active bool            `json:"active"`
	TaskID string          `json:"taskId,omitempty"`
	Failed *failedMoveWire `json:"failed,omitempty"`
}

type storageWire struct {
	Projects    locationWire     `json:"projects"`
	StayBytes   int64            `json:"stayBytes"`
	Suggestions []suggestionWire `json:"suggestions"`
	Usage       []usageWire      `json:"usage"`
	Move        moveWire         `json:"move"`
	Clean       struct {
		LastRunAt *string `json:"lastRunAt"`
	} `json:"clean"`
}

type checkWire struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Text   string `json:"text"`
}

type checkResultWire struct {
	OK           bool        `json:"ok"`
	CreateSubdir bool        `json:"createSubdir"`
	Checks       []checkWire `json:"checks"`
	NeedBytes    int64       `json:"needBytes"`
	ETASec       *int        `json:"etaSec"`
}

type moveResultWire struct {
	TaskID string `json:"taskId"`
	Done   bool   `json:"done"`
}

type entryWire struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type browseWire struct {
	Path  string      `json:"path"`
	Roots []entryWire `json:"roots"`
	Dirs  []entryWire `json:"dirs"`
}

type reclaimItemWire struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Bytes int64  `json:"bytes"`
	Count int    `json:"count"`
	Desc  string `json:"desc"`
}

type previewItemWire struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

type previewWire struct {
	PreviewID        string            `json:"previewId"`
	Items            []previewItemWire `json:"items"`
	FreedBytes       int64             `json:"freedBytes"`
	StoppedProcesses int               `json:"stoppedProcesses"`
	Errors           []string          `json:"errors,omitempty"`
}

type cleanupRunWire struct {
	Removed          int      `json:"removed"`
	FreedBytes       int64    `json:"freedBytes"`
	StoppedProcesses int      `json:"stoppedProcesses"`
	Errors           []string `json:"errors,omitempty"`
}

// tildePath đổi "<home>/…" thành "~/…" để hiển thị; path lạ giữ nguyên.
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}

func volumeToWire(v storage.Volume) volumeWire {
	return volumeWire{Name: v.Name, Kind: v.Kind, FS: v.FS, FreeBytes: v.FreeBytes, Bus: v.Bus}
}

/* ------------------------------- handlers ------------------------------- */

func (h *storageHandlers) get(w http.ResponseWriter, r *http.Request, _ identity.User) {
	ctx := r.Context()
	snap, err := h.store.Snapshot(ctx)
	if err != nil {
		storageError(w, err)
		return
	}
	var out storageWire
	out.Projects = locationWire{
		Path: snap.Projects.Path, DisplayPath: tildePath(snap.Projects.Path),
		Volume:    volumeToWire(snap.Projects.Volume),
		UsedBytes: snap.Projects.UsedBytes, Online: snap.Projects.Online,
	}
	if snap.Projects.OfflineSince != nil {
		text := snap.Projects.OfflineSince.Format(time.RFC3339)
		out.Projects.OfflineSince = &text
	}
	if snap.Projects.Previous != nil {
		prev := snap.Projects.Previous
		mode := prev.Mode
		if mode == "" {
			mode = storage.ModeMove // dữ liệu cũ trước khi có mode
		}
		wire := &previousWire{Path: prev.Path, Bytes: prev.Bytes, Mode: mode}
		if !prev.DeleteAfter.IsZero() {
			text := prev.DeleteAfter.Format(time.RFC3339)
			wire.DeleteAfter = &text
		}
		out.Projects.Previous = wire
	}
	if snap.Projects.LastSwitch != nil {
		out.Projects.LastSwitch = &switchWire{
			Mode: snap.Projects.LastSwitch.Mode,
			At:   snap.Projects.LastSwitch.At.Format(time.RFC3339),
		}
	}
	out.StayBytes = snap.StayBytes
	out.Suggestions = make([]suggestionWire, 0, len(snap.Suggestions))
	for _, s := range snap.Suggestions {
		out.Suggestions = append(out.Suggestions, suggestionWire{Path: s.Path, Volume: volumeToWire(s.Volume)})
	}
	out.Usage = make([]usageWire, 0, len(snap.Usage))
	for _, u := range snap.Usage {
		out.Usage = append(out.Usage, usageWire{ID: u.ID, Bytes: u.Bytes})
	}
	if snap.CleanLastRun != nil {
		text := snap.CleanLastRun.Format(time.RFC3339)
		out.Clean.LastRunAt = &text
	}
	out.Move = moveWire{Active: snap.Move.Active, TaskID: snap.Move.TaskID}
	if snap.Move.Failed != nil {
		out.Move.Failed = &failedMoveWire{
			TaskID: snap.Move.Failed.TaskID, Target: snap.Move.Failed.Target,
			Phase: snap.Move.Failed.Phase, At: snap.Move.Failed.At.Format(time.RFC3339),
		}
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

func (h *storageHandlers) check(w http.ResponseWriter, r *http.Request, _ identity.User) {
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if !webapp.DecodeJSON(w, r, &body) {
		return
	}
	result, err := h.store.Check(r.Context(), body.Path, body.Mode)
	if err != nil {
		storageError(w, err)
		return
	}
	out := checkResultWire{OK: result.OK, CreateSubdir: result.CreateSubdir,
		NeedBytes: result.NeedBytes, ETASec: result.ETASec}
	out.Checks = make([]checkWire, 0, len(result.Checks))
	for _, c := range result.Checks {
		out.Checks = append(out.Checks, checkWire{Kind: c.Kind, Status: c.Status, Text: c.Text})
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

func (h *storageHandlers) recheck(w http.ResponseWriter, r *http.Request, _ identity.User) {
	online, err := h.store.Recheck(r.Context())
	if err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"online": online})
}

func (h *storageHandlers) move(w http.ResponseWriter, r *http.Request, _ identity.User) {
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if !webapp.DecodeJSON(w, r, &body) {
		return
	}
	mode := body.Mode
	if mode == "" {
		mode = storage.ModeMove
	}
	taskID, done, err := h.store.Move(r.Context(), body.Path, mode)
	if err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, moveResultWire{TaskID: taskID, Done: done})
}

// moveRetry phục vụ POST /api/storage/move/retry — chạy lại lần chuyển hỏng
// gần nhất (server giữ đích; client không cần đoán từ task).
func (h *storageHandlers) moveRetry(w http.ResponseWriter, r *http.Request, _ identity.User) {
	taskID, done, err := h.store.RetryMove(r.Context())
	if err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, moveResultWire{TaskID: taskID, Done: done})
}

// moveDismiss phục vụ POST /api/storage/move/dismiss — gác lần chuyển hỏng
// (không đụng file nào).
func (h *storageHandlers) moveDismiss(w http.ResponseWriter, r *http.Request, _ identity.User) {
	if err := h.store.DismissMove(r.Context()); err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// moveSub phục vụ POST /api/storage/move/:taskId/cancel.
func (h *storageHandlers) moveSub(w http.ResponseWriter, r *http.Request, _ identity.User) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/storage/move/")
	if !strings.HasSuffix(rest, "/cancel") {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	taskID := strings.TrimSuffix(rest, "/cancel")
	if taskID == "" || strings.Contains(taskID, "/") {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if err := h.store.CancelMove(r.Context(), taskID); err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *storageHandlers) deletePrevious(w http.ResponseWriter, r *http.Request, _ identity.User) {
	if err := h.store.DeletePrevious(r.Context()); err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *storageHandlers) useDefault(w http.ResponseWriter, r *http.Request, _ identity.User) {
	if err := h.store.UseDefaultForNew(r.Context()); err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *storageHandlers) browse(w http.ResponseWriter, r *http.Request, _ identity.User) {
	result, err := h.store.Browse(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		storageError(w, err)
		return
	}
	out := browseWire{Path: result.Path}
	out.Roots = make([]entryWire, 0, len(result.Roots))
	for _, e := range result.Roots {
		out.Roots = append(out.Roots, entryWire{Name: e.Name, Path: e.Path})
	}
	out.Dirs = make([]entryWire, 0, len(result.Dirs))
	for _, e := range result.Dirs {
		out.Dirs = append(out.Dirs, entryWire{Name: e.Name, Path: e.Path})
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

func (h *storageHandlers) open(w http.ResponseWriter, r *http.Request, _ identity.User) {
	if err := h.store.Open(r.Context()); err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *storageHandlers) reclaimList(w http.ResponseWriter, r *http.Request, _ identity.User) {
	items, err := h.clean.ReclaimList(r.Context())
	if err != nil {
		storageError(w, err)
		return
	}
	out := make([]reclaimItemWire, 0, len(items))
	for _, it := range items {
		out = append(out, reclaimWire(it))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *storageHandlers) reclaimOne(w http.ResponseWriter, r *http.Request, _ identity.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/storage/reclaim/")
	if id == "" || strings.Contains(id, "/") {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	item, err := h.clean.Reclaim(r.Context(), id)
	if err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, reclaimWire(item))
}

func reclaimWire(it retention.ReclaimItem) reclaimItemWire {
	return reclaimItemWire{ID: it.ID, Title: it.Title, Bytes: it.Bytes, Count: it.Count, Desc: it.Desc}
}

func (h *storageHandlers) preview(w http.ResponseWriter, r *http.Request, _ identity.User) {
	preview, err := h.clean.PreviewCleanup(r.Context())
	if err != nil {
		storageError(w, err)
		return
	}
	out := previewWire{PreviewID: preview.ID, FreedBytes: preview.Freed,
		StoppedProcesses: len(preview.Stopped), Errors: preview.Errors}
	out.Items = make([]previewItemWire, 0, len(preview.Items))
	for _, it := range preview.Items {
		out.Items = append(out.Items, previewItemWire{Kind: it.Kind, Label: it.Label, Count: it.Count, Bytes: it.Bytes})
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

func (h *storageHandlers) run(w http.ResponseWriter, r *http.Request, _ identity.User) {
	var body struct {
		PreviewID string `json:"previewId"`
	}
	if !webapp.DecodeJSON(w, r, &body) {
		return
	}
	result, err := h.clean.RunCleanup(r.Context(), body.PreviewID)
	if err != nil {
		storageError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, cleanupRunWire{Removed: result.Removed,
		FreedBytes: result.Freed, StoppedProcesses: len(result.Stopped), Errors: result.Errors})
}

/* ------------------------------- errors ------------------------------- */

// storageError đổi lỗi nghiệp vụ sang HTTP ở đúng một chỗ.
func storageError(w http.ResponseWriter, err error) {
	var re *retention.Error
	if errors.As(err, &re) {
		webapp.WriteJSON(w, storageStatus(re.Code), map[string]any{
			"error": re.Message, "code": re.Code,
		})
		return
	}
	if code := storage.CodeOf(err); code != "" {
		webapp.WriteJSON(w, storageStatus(code), map[string]any{
			"error": err.Error(), "code": code,
		})
		return
	}
	webapp.Error(w, http.StatusInternalServerError, "lỗi máy chủ")
}

func storageStatus(code string) int {
	switch code {
	case storage.ErrInvalidPath:
		return http.StatusBadRequest
	case storage.ErrNotFound, storage.ErrFolderMissing, storage.ErrVolumeGone,
		storage.ErrUnknownReclaim:
		return http.StatusNotFound
	case storage.ErrCloudSync, storage.ErrDestNotEmpty:
		return http.StatusUnprocessableEntity
	case storage.ErrDestConflict:
		return http.StatusConflict
	case storage.ErrMoveInProgress, storage.ErrNoMove,
		storage.ErrPreviewRequired, storage.ErrNotDeletable, storage.ErrNotSupported:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
