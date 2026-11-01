package web

// HTTP của Nhắn trợ lý (data-contract §9 và §22). Handler chỉ đọc request →
// gọi assistant.Service → ghi JSON; mọi quyết định nằm ở service. parse/plan
// KHÔNG làm gì; chỉ run/run-plan mới thực thi.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/assistant"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// AssistantService là mặt nghiệp vụ của trợ lý — hiện thực là *assistant.Service.
type AssistantService interface {
	Plan(ctx context.Context, scope assistant.Scope, text string) (assistant.Reply, error)
	Parse(ctx context.Context, scope assistant.Scope, text string) (assistant.Parsed, error)
	Run(ctx context.Context, intent assistant.IntentID, params []byte) (string, error)
	RunPlan(ctx context.Context, planID string, stepIDs []string) (assistant.RunStatus, error)
	RunStatus(runID string) (assistant.RunStatus, bool)
}

// MountAssistant đăng ký /api/assistant/*.
func MountAssistant(g *Gateway, svc AssistantService) {
	post := func(path string, h func(http.ResponseWriter, *http.Request, AssistantService)) {
		g.Handle(http.MethodPost, path, func(w http.ResponseWriter, r *http.Request, _ identity.User) { h(w, r, svc) })
	}
	post("/api/assistant/plan", assistantPlan)
	post("/api/assistant/parse", assistantParse)
	post("/api/assistant/run", assistantRun)
	post("/api/assistant/run-plan", assistantRunPlan)
	g.Handle(http.MethodGet, "/api/assistant/runs/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		id := strings.TrimPrefix(r.URL.Path, "/api/assistant/runs/")
		st, ok := svc.RunStatus(id)
		if !ok || id == "" || strings.Contains(id, "/") {
			assistantWriteError(w, &assistant.Error{Code: assistant.CodeNotFound, Message: "Không tìm thấy lượt chạy này."})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		webapp.WriteJSON(w, http.StatusOK, st)
	})
}

type assistantTextBody struct {
	Scope assistant.Scope `json:"scope"`
	Text  string          `json:"text"`
}

type assistantUnknownWire struct {
	OK          bool               `json:"ok"`
	Unknown     *assistant.Unknown `json:"unknown"`
	Suggestions []string           `json:"suggestions"`
}

type assistantAskWire struct {
	OK  bool           `json:"ok"`
	Ask *assistant.Ask `json:"ask"`
}

type assistantPlanWire struct {
	OK              bool             `json:"ok"`
	PlanID          string           `json:"planId"`
	Steps           []assistant.Step `json:"steps"`
	Total           assistant.Total  `json:"total"`
	ImagesLeftToday *int             `json:"imagesLeftToday,omitempty"`
}

type assistantCostWire struct {
	Images *int `json:"images,omitempty"`
}

type assistantConfirmWire struct {
	Title  string            `json:"title"`
	Detail string            `json:"detail"`
	Cost   assistantCostWire `json:"cost"`
	Reruns []string          `json:"reruns"`
}

type assistantParseWire struct {
	OK      bool                 `json:"ok"`
	Intent  assistant.IntentID   `json:"intent"`
	Params  json.RawMessage      `json:"params"`
	Confirm assistantConfirmWire `json:"confirm"`
}

func decodeAssistantBody(w http.ResponseWriter, r *http.Request, into any) bool {
	return webapp.DecodeJSONCode(w, r, into, assistant.CodeInvalid)
}

func writeAssistantReply(w http.ResponseWriter, reply assistant.Reply) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case reply.Plan != nil:
		webapp.WriteJSON(w, http.StatusOK, assistantPlanWire{OK: true, PlanID: reply.Plan.ID, Steps: reply.Plan.Steps,
			Total: reply.Plan.Total, ImagesLeftToday: reply.Plan.ImagesLeftToday})
	case reply.Ask != nil:
		webapp.WriteJSON(w, http.StatusOK, assistantAskWire{Ask: reply.Ask})
	default:
		flat := make([]string, 0, len(reply.Unknown.Suggestions))
		for _, s := range reply.Unknown.Suggestions {
			flat = append(flat, s.Text)
		}
		webapp.WriteJSON(w, http.StatusOK, assistantUnknownWire{Unknown: reply.Unknown, Suggestions: flat})
	}
}

func assistantPlan(w http.ResponseWriter, r *http.Request, svc AssistantService) {
	var body assistantTextBody
	if !decodeAssistantBody(w, r, &body) {
		return
	}
	reply, err := svc.Plan(r.Context(), body.Scope, body.Text)
	if err != nil {
		assistantWriteError(w, err)
		return
	}
	writeAssistantReply(w, reply)
}

func assistantParse(w http.ResponseWriter, r *http.Request, svc AssistantService) {
	var body assistantTextBody
	if !decodeAssistantBody(w, r, &body) {
		return
	}
	parsed, err := svc.Parse(r.Context(), body.Scope, body.Text)
	if err != nil {
		assistantWriteError(w, err)
		return
	}
	if parsed.Reply != nil {
		writeAssistantReply(w, *parsed.Reply)
		return
	}
	cost := assistantCostWire{}
	if parsed.Confirm.Cost.Images > 0 {
		cost.Images = &parsed.Confirm.Cost.Images
	}
	w.Header().Set("Cache-Control", "no-store")
	webapp.WriteJSON(w, http.StatusOK, assistantParseWire{OK: true, Intent: parsed.Intent, Params: parsed.Params,
		Confirm: assistantConfirmWire{Title: parsed.Confirm.Title, Detail: parsed.Confirm.Detail, Cost: cost, Reruns: parsed.Confirm.Reruns}})
}

type assistantRunBody struct {
	Intent assistant.IntentID `json:"intent"`
	Params json.RawMessage    `json:"params"`
}

func assistantRun(w http.ResponseWriter, r *http.Request, svc AssistantService) {
	var body assistantRunBody
	if !decodeAssistantBody(w, r, &body) {
		return
	}
	msg, err := svc.Run(r.Context(), body.Intent, body.Params)
	if err != nil {
		assistantWriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

type assistantRunPlanBody struct {
	PlanID string   `json:"planId"`
	Steps  []string `json:"steps"`
}

func assistantRunPlan(w http.ResponseWriter, r *http.Request, svc AssistantService) {
	var body assistantRunPlanBody
	if !decodeAssistantBody(w, r, &body) {
		return
	}
	st, err := svc.RunPlan(r.Context(), body.PlanID, body.Steps)
	if err != nil {
		assistantWriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	webapp.WriteJSON(w, http.StatusAccepted, st)
}

var assistantStatus = map[string]int{
	assistant.CodeTextInvalid:      http.StatusBadRequest,
	assistant.CodeInvalid:          http.StatusBadRequest,
	assistant.CodeNotFound:         http.StatusNotFound,
	assistant.CodeModelUnavailable: http.StatusServiceUnavailable,
	assistant.CodeRateLimited:      http.StatusTooManyRequests,
	assistant.CodePlanExpired:      http.StatusGone,
	assistant.CodeAssistantFailed:  http.StatusBadGateway,
	assistant.CodeConflict:         http.StatusConflict,
}

// assistantWriteError ghi vỏ lỗi {error, code}. Lỗi của service bên dưới giữ
// nguyên mã/HTTP status mà chính màn đó dùng.
func assistantWriteError(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	var ae *assistant.Error
	if errors.As(err, &ae) {
		webapp.WriteJSON(w, assistantStatus[ae.Code], map[string]any{"error": ae.Message, "code": ae.Code})
		return
	}
	var se *assistant.StepError
	if errors.As(err, &se) {
		var ce *domain.CloneError
		var che *channels.Error
		switch {
		case errors.As(se.Err, &ce):
			WriteCloneError(w, se.Err)
		case errors.As(se.Err, &che):
			writeChannelError(w, se.Err, nil)
		case se.Err == nil:
			webapp.WriteJSON(w, http.StatusConflict, map[string]any{"error": se.Message, "code": "cannot_run"})
		default:
			webapp.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": se.Message, "code": "step_failed"})
		}
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	webapp.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "Trợ lý gặp lỗi. Thử lại sau ít phút.", "code": "internal"})
}
