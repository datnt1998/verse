package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// SuggestService là phần suggest.Service mà HTTP cần.
type SuggestService interface {
	List(ctx context.Context, channelID string, limit int) ([]suggest.Suggestion, int, error)
	Accept(ctx context.Context, id string, confirm bool) (suggest.AcceptResult, error)
	Dismiss(ctx context.Context, id string) error
}

func writeSuggestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, suggest.ErrNotFound):
		webapp.WriteJSON(w, http.StatusNotFound, map[string]any{"error": err.Error(), "code": "not_found"})
	case errors.Is(err, suggest.ErrBadOp), errors.Is(err, suggest.ErrNavigation):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "code": "invalid_request"})
	case errors.Is(err, suggest.ErrGone):
		webapp.WriteJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "gone"})
	default:
		var confirm *suggest.ConfirmRequiredError
		if errors.As(err, &confirm) {
			webapp.WriteJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "confirm_required", "cost": map[string]int{"images": confirm.Images}})
			return
		}
		webapp.Error(w, http.StatusInternalServerError, err.Error())
	}
}

// MountSuggestions gắn Đề xuất của trợ lý: danh sách, nhận và bỏ qua. notify
// phát SSE suggestions (ref là channelId hoặc "").
func MountSuggestions(g *Gateway, suggestions SuggestService, notify func(string)) {
	g.Handle(http.MethodGet, "/api/suggestions", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		list, total, err := suggestions.List(r.Context(), r.URL.Query().Get("channelId"), suggest.MaxOpen)
		if err != nil {
			writeSuggestError(w, err)
			return
		}
		if list == nil {
			list = []suggest.Suggestion{}
		}
		w.Header().Set("Cache-Control", "private, no-store")
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"suggestions": list, "total": total})
	})
	g.Handle(http.MethodPost, "/api/suggestions/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/suggestions/"), "/")
		id, action, ok := strings.Cut(rest, "/")
		if !ok || id == "" {
			webapp.Error(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		switch action {
		case "accept":
			var in struct {
				Confirm bool `json:"confirm"`
			}
			if !decodeOptional(w, r, &in) {
				return
			}
			res, err := suggestions.Accept(r.Context(), id, in.Confirm)
			if err != nil {
				writeSuggestError(w, err)
				return
			}
			if notify != nil {
				notify("")
			}
			webapp.WriteJSON(w, http.StatusOK, res)
		case "dismiss":
			var in struct{}
			if !decodeOptional(w, r, &in) {
				return
			}
			if err := suggestions.Dismiss(r.Context(), id); err != nil {
				writeSuggestError(w, err)
				return
			}
			if notify != nil {
				notify("")
			}
			webapp.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		default:
			webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		}
	})
}

// decodeOptional đọc JSON body nếu có; body rỗng là hợp lệ (mọi trường mặc định).
func decodeOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return webapp.DecodeJSON(w, r, v)
}
