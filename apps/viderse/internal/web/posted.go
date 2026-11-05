package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/posted"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/webapp"
)

// PostedService là phần Service mà HTTP dùng cho mark-posted (§21).
type PostedService interface {
	Post(ctx context.Context, jobID, youtubeURL string) (string, error)
}

// MountPosted gắn POST /api/clone/:id/posted. V38 đã bỏ route số liệu
// (GET performance) và áp dụng nhận xét (insights/apply); chỉ còn mark-posted.
// Phải gọi sau MountClone.
func MountPosted(g *Gateway, svc PostedService) {
	g.clonePostedRoute = func(w http.ResponseWriter, r *http.Request) bool {
		rest := strings.TrimPrefix(r.URL.Path, "/api/clone/")
		id, tail, ok := strings.Cut(rest, "/")
		if !ok || tail != "posted" {
			return false
		}
		if r.Method != http.MethodPost {
			cloneNotFound(w)
			return true
		}
		if !cloneSegment(id) {
			invalidClone(w, "Mã video không hợp lệ.")
			return true
		}
		postVideo(w, r, svc, id)
		return true
	}
}

func postVideo(w http.ResponseWriter, r *http.Request, svc PostedService, id string) {
	var body struct {
		YoutubeURL string `json:"youtubeUrl"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<10))
	if err != nil {
		invalidClone(w, "Nội dung không hợp lệ.")
		return
	}
	if strings.TrimSpace(string(raw)) != "" {
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if dec.Decode(&body) != nil {
			invalidClone(w, "Nội dung không hợp lệ.")
			return
		}
	}
	youtubeID, err := svc.Post(r.Context(), id, body.YoutubeURL)
	switch {
	case err == nil:
		var out any
		if youtubeID != "" {
			out = youtubeID
		}
		w.Header().Set("Cache-Control", "no-store")
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"youtubeId": out})
	case errors.Is(err, posted.ErrBadURL):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "field": "youtubeUrl"})
	case errors.Is(err, posted.ErrNotFound):
		webapp.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, posted.ErrNotReady):
		webapp.Error(w, http.StatusConflict, err.Error())
	default:
		var ce *domain.CloneError
		if errors.As(err, &ce) {
			WriteCloneError(w, err)
			return
		}
		webapp.Error(w, http.StatusInternalServerError, "Chưa đánh dấu đã đăng được lúc này.")
	}
}
