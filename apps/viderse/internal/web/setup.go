package web

import (
	"context"
	"net/http"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/setup"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// Setup là phần setup.Service mà HTTP cần — khai ở phía dùng.
type Setup interface {
	Status(ctx context.Context, fresh bool) (setup.Status, error)
	Start(ctx context.Context) (setup.Status, error)
	Retry(ctx context.Context, id domain.SetupStepID) (setup.Status, error)
}

// MountSetup đăng ký các route của bước Chuẩn bị máy (W1). GET dùng
// /api/setup/state vì /api/setup thuộc luồng thiết lập tài khoản của
// platform/webapp; start/retry không bị luồng đó nhận.
func MountSetup(g *Gateway, svc Setup) {
	g.Handle(http.MethodGet, "/api/setup/state", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		st, err := svc.Status(r.Context(), false)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, setupToWire(st))
	})
	g.Handle(http.MethodPost, "/api/setup/check", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		st, err := svc.Status(r.Context(), true)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, setupToWire(st))
	})
	g.Handle(http.MethodPost, "/api/setup/start", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		st, err := svc.Start(r.Context())
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusAccepted, setupToWire(st))
	})
	g.Handle(http.MethodPost, "/api/setup/retry", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		var req struct {
			ID string `json:"id"`
		}
		if !webapp.DecodeJSON(w, r, &req) {
			return
		}
		st, err := svc.Retry(r.Context(), req.ID)
		if err != nil {
			webapp.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusAccepted, setupToWire(st))
	})
}

// setupWire là hình của GET /api/setup.
type setupWire struct {
	State    string      `json:"state"`
	Complete bool        `json:"complete"`
	Running  bool        `json:"running"`
	Steps    []setupStep `json:"steps"`
	Disk     setupDisk   `json:"disk"`
}

type setupStep struct {
	ID       string               `json:"id"`
	State    string               `json:"state"`
	Progress *installProgressWire `json:"progress"`
	Error    string               `json:"error,omitempty"`
	// SizeBytes là dung lượng bước chiếm khi cài xong (0 = không đo được).
	SizeBytes int64 `json:"sizeBytes"`
	// Provided = công cụ có sẵn trên máy, không phải tải (Chrome hệ thống).
	Provided bool `json:"provided,omitempty"`
}

type setupDisk struct {
	FreeBytes     int64 `json:"freeBytes"`
	RequiredBytes int64 `json:"requiredBytes"`
	Enough        bool  `json:"enough"`
	// ApproxBytes là tổng cài đặt xấp xỉ của các bước còn thiếu — câu
	// “khoảng X GB”, không phải chỗ trống phải có.
	ApproxBytes int64 `json:"approxBytes"`
}

func setupToWire(st setup.Status) setupWire {
	out := setupWire{
		State: st.State, Complete: st.Complete, Running: st.Running,
		Steps: make([]setupStep, 0, len(st.Steps)),
		Disk:  setupDisk{FreeBytes: st.Disk.FreeBytes, RequiredBytes: st.Disk.RequiredBytes, Enough: st.Disk.Enough, ApproxBytes: st.Disk.ApproxBytes},
	}
	for _, s := range st.Steps {
		step := setupStep{ID: s.ID, State: s.State, Error: s.Error, SizeBytes: s.SizeBytes, Provided: s.Provided}
		step.Progress = installProgressToWire(s.Progress)
		out.Steps = append(out.Steps, step)
	}
	return out
}
