package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// Logs là phần log store mà HTTP cần — khai ở phía dùng. Bundle dựng gói báo
// lỗi đã che; composition (serve.go) lo phần "lấy gì" (settings, phiên bản).
type Logs interface {
	Read(tool string, minLevel string, limit int, before time.Time) ([]logs.Entry, *time.Time, error)
	Bundle(ctx context.Context) (*logs.Bundle, error)
}

// Versions là bộ kiểm phiên bản app cho thẻ phiên bản ở mục Nhật ký.
type Versions interface {
	Get(ctx context.Context) logs.VersionInfo
	Check(ctx context.Context) logs.VersionInfo
}

// MountLogs đăng ký route nhật ký công cụ, gói báo lỗi và phiên bản app.
func MountLogs(g *Gateway, svc Logs, versions Versions) {
	g.Handle(http.MethodGet, "/api/tools/logs/export", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		bundle, err := svc.Bundle(r.Context())
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+bundle.FileName+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle.Bytes)
	})
	g.Handle(http.MethodGet, "/api/tools/logs", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		q := r.URL.Query()
		level := q.Get("level")
		if level == "" {
			level = "info"
		}
		switch level {
		case "info", "warn", "error":
		default:
			webapp.Error(w, http.StatusBadRequest, "level không hợp lệ")
			return
		}
		limit := 200
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				webapp.Error(w, http.StatusBadRequest, "limit không hợp lệ")
				return
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
		var before time.Time
		if raw := q.Get("cursor"); raw != "" {
			decoded, err := base64.RawURLEncoding.DecodeString(raw)
			if err != nil {
				webapp.Error(w, http.StatusBadRequest, "cursor không hợp lệ")
				return
			}
			parsed, err := time.Parse(time.RFC3339Nano, string(decoded))
			if err != nil {
				webapp.Error(w, http.StatusBadRequest, "cursor không hợp lệ")
				return
			}
			before = parsed
		} else if raw := q.Get("before"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				webapp.Error(w, http.StatusBadRequest, "before không hợp lệ")
				return
			}
			before = parsed
		}
		entries, nextBefore, err := svc.Read(q.Get("tool"), level, limit, before)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]logEntryWire, 0, len(entries))
		// Mới nhất trước: màn nhật ký cuộn từ trên xuống, "Cũ hơn" nối trang
		// bằng cursor của dòng cũ nhất trong trang.
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			out = append(out, logEntryWire{
				TS:      e.TS.UTC().Format(time.RFC3339),
				Tool:    e.Tool,
				Level:   strings.TrimSpace(string(e.Level)),
				Message: e.Message,
			})
		}
		body := map[string]any{"entries": out, "nextCursor": nil}
		if nextBefore != nil {
			body["nextCursor"] = base64.RawURLEncoding.EncodeToString([]byte(nextBefore.UTC().Format(time.RFC3339Nano)))
			body["nextBefore"] = nextBefore.UTC().Format(time.RFC3339)
		}
		webapp.WriteJSON(w, http.StatusOK, body)
	})
	g.Handle(http.MethodGet, "/api/version", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		webapp.WriteJSON(w, http.StatusOK, versionToWire(versions.Get(r.Context())))
	})
	g.Handle(http.MethodPost, "/api/version/check", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		webapp.WriteJSON(w, http.StatusOK, versionToWire(versions.Get(r.Context())))
	})
}

type logEntryWire struct {
	TS      string `json:"ts"`
	Tool    string `json:"tool"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type versionWire struct {
	Version   string  `json:"version"`
	Latest    *string `json:"latest"`
	CheckedAt string  `json:"checkedAt"`
}

func versionToWire(v logs.VersionInfo) versionWire {
	out := versionWire{Version: v.Version, Latest: v.Latest}
	if !v.CheckedAt.IsZero() {
		out.CheckedAt = v.CheckedAt.UTC().Format(time.RFC3339)
	}
	return out
}
