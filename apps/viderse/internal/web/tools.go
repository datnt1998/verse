package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/toolsvc"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// Tools là phần toolsvc mà HTTP cần — khai ở phía dùng.
type Tools interface {
	Snapshot(ctx context.Context, fresh bool) (toolsvc.Snapshot, error)
	StartInstall(ctx context.Context, id toolsvc.ToolID) (string, error)
	Cancel(ctx context.Context, id toolsvc.ToolID) error
	// DataDir là thư mục app cài công cụ và model — hàng “Thư mục dữ liệu”
	// của vùng Nâng cao.
	DataDir() string
}

// MountTools đăng ký các route của Cài đặt › Công cụ trên máy.
func MountTools(g *Gateway, svc Tools) {
	g.Handle(http.MethodGet, "/api/tools", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		snap, err := svc.Snapshot(r.Context(), false)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, toolsSnapshotToWire(snap, svc.DataDir()))
	})
	g.Handle(http.MethodPost, "/api/tools/check", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		snap, err := svc.Snapshot(r.Context(), true)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, toolsSnapshotToWire(snap, svc.DataDir()))
	})
	// Đuôi động sau /api/tools/: <id>/install và <id>/cancel. Đăng ký sau
	// các route chính xác ở trên để chúng thắng khớp.
	g.Handle(http.MethodPost, "/api/tools/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		id, action, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/api/tools/"), "/")
		if !ok || !isToolID(id) {
			webapp.Error(w, http.StatusNotFound, "không có công cụ này")
			return
		}
		switch action {
		case "install":
			taskID, err := svc.StartInstall(r.Context(), id)
			if err != nil {
				webapp.Error(w, http.StatusBadRequest, err.Error())
				return
			}
			webapp.WriteJSON(w, http.StatusAccepted, map[string]any{"started": true, "taskId": taskID})
		case "cancel":
			if err := svc.Cancel(r.Context(), id); err != nil {
				webapp.Error(w, http.StatusInternalServerError, err.Error())
				return
			}
			webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			webapp.Error(w, http.StatusNotFound, "không có hành động này")
		}
	})
}

func isToolID(id string) bool {
	for _, known := range toolsvc.ToolIDs {
		if known == id {
			return true
		}
	}
	return false
}

// toolsWire là hình của GET /api/tools.
type toolsWire struct {
	Tools     []toolRowWire `json:"tools"`
	CheckedAt string        `json:"checkedAt"`
	// DataDir là “Thư mục dữ liệu” của vùng Nâng cao (rỗng = không rõ).
	DataDir string `json:"dataDir,omitempty"`
}

type toolRowWire struct {
	ID       string               `json:"id"`
	State    string               `json:"state"`
	Hint     string               `json:"hint"`
	Version  string               `json:"version,omitempty"`
	Update   *updateWire          `json:"update,omitempty"`
	Progress *installProgressWire `json:"progress"`
	Detail   string               `json:"detail,omitempty"`
	RetryAt  *string              `json:"retryAt,omitempty"`
	// SizeBytes là dung lượng bước chiếm khi cài xong (0 = không đo được).
	SizeBytes int64 `json:"sizeBytes"`
	// Provided = công cụ có sẵn trên máy, không phải tải (Chrome hệ thống).
	Provided bool `json:"provided,omitempty"`
}

type updateWire struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

func toolsSnapshotToWire(snap toolsvc.Snapshot, dataDir string) toolsWire {
	out := toolsWire{Tools: make([]toolRowWire, 0, len(snap.Tools)), DataDir: dataDir}
	if !snap.CheckedAt.IsZero() {
		out.CheckedAt = snap.CheckedAt.UTC().Format(time.RFC3339)
	}
	for _, row := range snap.Tools {
		out.Tools = append(out.Tools, toolRowToWire(row))
	}
	return out
}

func toolRowToWire(row toolsvc.Row) toolRowWire {
	tw := toolRowWire{
		ID: row.ID, State: row.State, Hint: row.Hint,
		Version: row.Version, Detail: row.Detail,
		SizeBytes: row.SizeBytes, Provided: row.Provided,
	}
	if row.Update != nil {
		tw.Update = &updateWire{Current: row.Update.Current, Latest: row.Update.Latest}
	}
	tw.Progress = installProgressToWire(row.Progress)
	if row.RetryAt != nil {
		at := row.RetryAt.UTC().Format(time.RFC3339)
		tw.RetryAt = &at
	}
	return tw
}
