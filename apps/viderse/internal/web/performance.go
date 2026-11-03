package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/performance"
	"github.com/datnt1998/verse/platform/webapp"
)

// PerformanceService là phần Service mà HTTP dùng (§21).
type PerformanceService interface {
	Post(ctx context.Context, jobID, youtubeURL string) (string, error)
	Performance(ctx context.Context, channelID string) (performance.View, error)
	Apply(ctx context.Context, channelID, insightID string) (performance.ApplyResult, error)
}

// MountPerformance gắn: POST /api/clone/:id/posted, GET /api/channels/:id/performance,
// POST /api/channels/:id/insights/:insightId/apply.
// Phải gọi sau MountClone và sau route /api/channels/summary.
func MountPerformance(g *Gateway, svc PerformanceService) {
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
	g.HandleChannel(http.MethodGet, "performance", func(w http.ResponseWriter, r *http.Request, id string, _ []string) {
		view, err := svc.Performance(r.Context(), id)
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, "Chưa đọc được số liệu của kênh.")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		webapp.WriteJSON(w, http.StatusOK, view)
	})
	g.HandleChannel(http.MethodPost, "insights/*/apply", func(w http.ResponseWriter, r *http.Request, id string, rest []string) {
		applyInsight(w, r, svc, id, rest[0])
	})
}

func postVideo(w http.ResponseWriter, r *http.Request, svc PerformanceService, id string) {
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
	case errors.Is(err, performance.ErrBadURL):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "field": "youtubeUrl"})
	case errors.Is(err, performance.ErrNotFound):
		webapp.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, performance.ErrNotReady):
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

func applyInsight(w http.ResponseWriter, r *http.Request, svc PerformanceService, channelID, insightID string) {
	result, err := svc.Apply(r.Context(), channelID, insightID)
	switch {
	case err == nil && result.AlreadyApplied:
		webapp.Error(w, http.StatusConflict, "Nhận xét này đã được áp dụng.")
	case err == nil:
		out := map[string]any{"kind": result.Kind}
		if result.RuleID != "" {
			out["ruleId"] = result.RuleID
		}
		webapp.WriteJSON(w, http.StatusOK, out)
	case errors.Is(err, performance.ErrInsightGone):
		webapp.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, performance.ErrNoRuleAdder):
		webapp.Error(w, http.StatusServiceUnavailable, err.Error())
	default:
		webapp.Error(w, http.StatusInternalServerError, "Chưa áp dụng được nhận xét lúc này.")
	}
}
