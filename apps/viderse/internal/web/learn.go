package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/learn"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// SuggestService là phần suggest.Service mà HTTP cần.
type SuggestService interface {
	List(ctx context.Context, channelID string, limit int) ([]suggest.Suggestion, int, error)
	Accept(ctx context.Context, id string, confirm bool) (suggest.AcceptResult, error)
	Dismiss(ctx context.Context, id string, forever bool) error
	Undo(ctx context.Context, op string, params json.RawMessage) error
}

// LearnService là phần learn.Service mà HTTP cần.
type LearnService interface {
	List(ctx context.Context, f learn.RuleFilter) ([]learn.Rule, error)
	Create(ctx context.Context, in learn.RuleInput) (learn.Rule, error)
	Patch(ctx context.Context, id string, enabled *bool, text *string) (learn.Rule, error)
	Delete(ctx context.Context, id string) error
}

type ruleScopeWire struct {
	ChannelID     string `json:"channelId,omitempty"`
	ImagePromptID string `json:"imagePromptId,omitempty"`
}
type learnedRuleWire struct {
	ID          string            `json:"id"`
	Scope       ruleScopeWire     `json:"scope"`
	Agent       string            `json:"agent"`
	Text        string            `json:"text"`
	Source      string            `json:"source"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"createdAt"`
	LearnedFrom learn.LearnedFrom `json:"learnedFrom"`
}

func ruleToWire(r learn.Rule) learnedRuleWire {
	w := learnedRuleWire{ID: r.ID, Agent: r.Agent, Text: r.Text, Source: r.Label(), Enabled: r.Enabled, CreatedAt: r.CreatedAt, LearnedFrom: r.LearnedFrom}
	w.LearnedFrom.Kind = r.WireKind()
	if r.ChannelID != nil {
		w.Scope.ChannelID = *r.ChannelID
	}
	if r.ImagePromptID != nil {
		w.Scope.ImagePromptID = *r.ImagePromptID
	}
	return w
}

func writeLearnError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, learn.ErrChannelNotFound), errors.Is(err, learn.ErrRuleNotFound), errors.Is(err, suggest.ErrNotFound):
		webapp.WriteJSON(w, http.StatusNotFound, map[string]any{"error": err.Error(), "code": "not_found"})
	case errors.Is(err, learn.ErrRuleLimit):
		webapp.WriteJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "limit"})
	case errors.Is(err, learn.ErrInvalid), errors.Is(err, suggest.ErrBadOp), errors.Is(err, suggest.ErrNavigation):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "code": "invalid_request"})
	case errors.Is(err, suggest.ErrGone), errors.Is(err, suggest.ErrNoUndo):
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

// MountLearn gắn Quy tắc đã học, Đề xuất và hoàn tác. notifyRules/notifySuggestions
// phát SSE learned.rules / suggestions (ref là channelId hoặc imagePromptId).
func MountLearn(g *Gateway, rules LearnService, suggestions SuggestService, notifyRules, notifySuggestions func(string)) {
	g.Handle(http.MethodGet, "/api/learned-rules", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		q := r.URL.Query()
		list, err := rules.List(r.Context(), learn.RuleFilter{ChannelID: q.Get("channelId"), ImagePromptID: q.Get("imagePromptId")})
		if err != nil {
			writeLearnError(w, err)
			return
		}
		out := make([]learnedRuleWire, len(list))
		for i, rule := range list {
			out[i] = ruleToWire(rule)
		}
		w.Header().Set("Cache-Control", "private, no-store")
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"rules": out, "limit": learn.MaxRulesPerScope})
	})
	g.Handle(http.MethodPost, "/api/learned-rules", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var in struct {
			Scope       ruleScopeWire      `json:"scope"`
			Agent       string             `json:"agent"`
			Text        string             `json:"text"`
			Source      string             `json:"source"`
			LearnedFrom *learn.LearnedFrom `json:"learnedFrom"`
		}
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		source := learn.SourceManual
		if in.Source == learn.SourceAssistant {
			source = learn.SourceAssistant
		}
		rule, err := rules.Create(r.Context(), learn.RuleInput{ChannelID: in.Scope.ChannelID, ImagePromptID: in.Scope.ImagePromptID, Agent: in.Agent, Text: in.Text, Source: source, LearnedFrom: in.LearnedFrom})
		if err != nil {
			writeLearnError(w, err)
			return
		}
		notifyRule(notifyRules, rule)
		webapp.WriteJSON(w, http.StatusOK, ruleToWire(rule))
	})
	for _, m := range []string{http.MethodPatch, http.MethodDelete} {
		method := m
		g.Handle(method, "/api/learned-rules/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/learned-rules/"), "/")
			if id == "" || strings.Contains(id, "/") {
				webapp.Error(w, http.StatusNotFound, "không tìm thấy")
				return
			}
			if method == http.MethodDelete {
				before := scopeRef(r.Context(), rules, id)
				if err := rules.Delete(r.Context(), id); err != nil {
					writeLearnError(w, err)
					return
				}
				if notifyRules != nil {
					notifyRules(before)
				}
				webapp.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			var in struct {
				Enabled *bool   `json:"enabled"`
				Text    *string `json:"text"`
			}
			if !webapp.DecodeJSON(w, r, &in) {
				return
			}
			if in.Enabled == nil && in.Text == nil {
				webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "thiếu enabled hoặc text", "code": "invalid_request"})
				return
			}
			rule, err := rules.Patch(r.Context(), id, in.Enabled, in.Text)
			if err != nil {
				writeLearnError(w, err)
				return
			}
			notifyRule(notifyRules, rule)
			webapp.WriteJSON(w, http.StatusOK, ruleToWire(rule))
		})
	}
	g.Handle(http.MethodGet, "/api/suggestions", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		list, total, err := suggestions.List(r.Context(), r.URL.Query().Get("channelId"), suggest.MaxOpen)
		if err != nil {
			writeLearnError(w, err)
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
				writeLearnError(w, err)
				return
			}
			if notifySuggestions != nil {
				notifySuggestions("")
			}
			if notifyRules != nil && res.RuleID != "" {
				notifyRules("")
			}
			webapp.WriteJSON(w, http.StatusOK, res)
		case "dismiss":
			var in struct {
				Forever bool `json:"forever"`
			}
			if !decodeOptional(w, r, &in) {
				return
			}
			if err := suggestions.Dismiss(r.Context(), id, in.Forever); err != nil {
				writeLearnError(w, err)
				return
			}
			if notifySuggestions != nil {
				notifySuggestions("")
			}
			webapp.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		default:
			webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		}
	})
	g.Handle(http.MethodPost, "/api/undo", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var in struct {
			Op     string          `json:"op"`
			Params json.RawMessage `json:"params"`
		}
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		if err := suggestions.Undo(r.Context(), in.Op, in.Params); err != nil {
			writeLearnError(w, err)
			return
		}
		if notifyRules != nil {
			notifyRules("")
		}
		if notifySuggestions != nil {
			notifySuggestions("")
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}

// decodeOptional đọc JSON body nếu có; body rỗng là hợp lệ (mọi trường mặc định).
func decodeOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return webapp.DecodeJSON(w, r, v)
}

func notifyRule(notify func(string), r learn.Rule) {
	if notify == nil {
		return
	}
	switch {
	case r.ChannelID != nil:
		notify(*r.ChannelID)
	case r.ImagePromptID != nil:
		notify(*r.ImagePromptID)
	default:
		notify("")
	}
}

func scopeRef(ctx context.Context, rules LearnService, id string) string {
	list, err := rules.List(ctx, learn.RuleFilter{})
	if err != nil {
		return ""
	}
	for _, r := range list {
		if r.ID == id {
			if r.ChannelID != nil {
				return *r.ChannelID
			}
			if r.ImagePromptID != nil {
				return *r.ImagePromptID
			}
		}
	}
	return ""
}
