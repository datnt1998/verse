package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

type ServerStorageService interface {
	Snapshot(context.Context, bool) (serverstorage.Snapshot, error)
	Test(context.Context, string, serverstorage.S3Config) (serverstorage.TestResult, error)
	Probe(context.Context, string, string, bool, string) (serverstorage.TestResult, error)
	Save(context.Context, string, serverstorage.S3Config, string) (serverstorage.SaveResult, error)
	Migrate(context.Context) (string, error)
	MigrationAction(context.Context, string, string) error
}

func storageAdmin(user identity.User) bool {
	return user.Role == identity.RoleAdmin
}
func serverStorageError(w http.ResponseWriter, err error) {
	var typed *serverstorage.Error
	switch {
	case errors.As(err, &typed):
		webapp.Error(w, typed.Status, typed.Message)
	case errors.Is(err, media.ErrConflict), errors.Is(err, media.ErrBusy):
		webapp.Error(w, 409, "Trạng thái lưu trữ đã thay đổi; vui lòng tải lại")
	default:
		webapp.Error(w, 500, "Không thực hiện được thao tác lưu trữ")
	}
}
func MountServerStorage(g *Gateway, svc ServerStorageService) {
	g.Handle(http.MethodGet, "/api/storage/server", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		w.Header().Set("Cache-Control", "private, no-store")
		result, err := svc.Snapshot(r.Context(), storageAdmin(user))
		if err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, result)
	})
	admin := func(fn func(http.ResponseWriter, *http.Request, identity.User)) func(http.ResponseWriter, *http.Request, identity.User) {
		return func(w http.ResponseWriter, r *http.Request, user identity.User) {
			w.Header().Set("Cache-Control", "private, no-store")
			if !storageAdmin(user) {
				webapp.Error(w, 403, "Chỉ quản trị viên được quản lý kho lưu trữ")
				return
			}
			fn(w, r, user)
		}
	}
	g.Handle(http.MethodPost, "/api/storage/server/test", admin(func(w http.ResponseWriter, r *http.Request, user identity.User) {
		var req struct {
			Config serverstorage.S3Config `json:"config"`
		}
		if !webapp.DecodeJSON(w, r, &req) {
			return
		}
		out, err := svc.Test(r.Context(), user.Username, req.Config)
		if err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, out)
	}))
	g.Handle(http.MethodPost, "/api/storage/server/test/", admin(func(w http.ResponseWriter, r *http.Request, user identity.User) {
		tail := strings.TrimPrefix(r.URL.Path, "/api/storage/server/test/")
		id, action, ok := strings.Cut(tail, "/")
		if !ok || id == "" || action != "probe" {
			webapp.Error(w, 404, "Không tìm thấy lượt kiểm tra")
			return
		}
		var req struct {
			OK     *bool  `json:"ok"`
			Reason string `json:"reason"`
		}
		if !webapp.DecodeJSON(w, r, &req) {
			return
		}
		if req.OK == nil {
			webapp.Error(w, 400, "Thiếu kết quả kiểm tra trình duyệt")
			return
		}
		out, err := svc.Probe(r.Context(), user.Username, id, *req.OK, req.Reason)
		if err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, out)
	}))
	g.Handle(http.MethodPut, "/api/storage/server/config", admin(func(w http.ResponseWriter, r *http.Request, user identity.User) {
		var req struct {
			Config serverstorage.S3Config `json:"config"`
			TestID string                 `json:"testId"`
		}
		if !webapp.DecodeJSON(w, r, &req) {
			return
		}
		out, err := svc.Save(r.Context(), user.Username, req.Config, req.TestID)
		if err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, out)
	}))
	g.Handle(http.MethodPost, "/api/storage/server/migrate", admin(func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		id, err := svc.Migrate(r.Context())
		if err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, map[string]string{"taskId": id})
	}))
	g.Handle(http.MethodPost, "/api/storage/server/migrate/", admin(func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		tail := strings.TrimPrefix(r.URL.Path, "/api/storage/server/migrate/")
		id, action, ok := strings.Cut(tail, "/")
		if !ok || id == "" || strings.Contains(action, "/") {
			webapp.Error(w, 404, "Không tìm thấy lượt chuyển kho")
			return
		}
		if err := svc.MigrationAction(r.Context(), id, action); err != nil {
			serverStorageError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, map[string]bool{"ok": true})
	}))
}
