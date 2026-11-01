package web

// HTTP của mảng kết nối AI (§15): chỉ đọc request → gọi service → ghi JSON.
// Kiểu wire của §15 nằm ở đây để service tiến hoá không ngầm đổi hợp đồng.

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// AIService là phần aiconn mà tầng HTTP dùng (khai phía consumer).
type AIService interface {
	Providers(ctx context.Context) ([]aiconn.ProviderView, error)
	Models(ctx context.Context, providerID, q, filter, cursor string, limit int) (aiconn.ModelPage, error)
	CreateProvider(ctx context.Context, in aiconn.CreateProviderInput) (aiconn.ProviderView, error)
	PatchProvider(ctx context.Context, id string, in aiconn.PatchProviderInput) (aiconn.ProviderView, error)
	DeleteProvider(ctx context.Context, id string) error
	TestProvider(ctx context.Context, id string) (aiconn.TestResult, error)
	CheckModel(ctx context.Context, id, model string) (aiconn.CheckModelResult, error)
	Agents(ctx context.Context) (domain.AgentModels, error)
	PutAgents(ctx context.Context, agents domain.AgentModels) (domain.AgentModels, error)
	LoginStatus(ctx context.Context, id string) (*aiconn.LoginSession, error)
	StartLogin(ctx context.Context, id, mode string) (*aiconn.LoginSession, error)
	CancelLogin(ctx context.Context, id string) error
	SubmitLoginCode(ctx context.Context, id, code string) error
	Logout(ctx context.Context, id string) error
}

// ── wire DTO (§15) ─────────────────────────────────────────────────────────

type aiQuotaWire struct {
	Used    int64  `json:"used"`
	Limit   int64  `json:"limit"`
	Period  string `json:"period"`
	ResetAt string `json:"resetAt"`
}

type aiProviderWire struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Auth    string `json:"auth"`
	Label   string `json:"label"`
	KeyHint string `json:"keyHint,omitempty"`
	BaseURL string `json:"baseUrl,omitempty"`
	Status  string `json:"status"`
	// Plan: gói đã lưu của credential (chatgpt_plan_type); rỗng với khoá API
	// và với xAI (token xAI không mang thông tin gói).
	Plan  string       `json:"plan,omitempty"`
	Quota *aiQuotaWire `json:"quota,omitempty"`
	// DailyPaidImageLimit: trần ảnh trả phí/ngày của provider khoá API (D7);
	// 0 = không tự tiêu. Thẻ nguồn vẽ đọc số này thay vì viết cứng 50.
	DailyPaidImageLimit int      `json:"dailyPaidImageLimit"`
	CatalogAt           string   `json:"catalogAt,omitempty"`
	CatalogCount        int      `json:"catalogCount"`
	Selected            []string `json:"selected"`
}

type aiPriceWire struct {
	InPerM   float64 `json:"inPerM,omitempty"`
	OutPerM  float64 `json:"outPerM,omitempty"`
	PerImage float64 `json:"perImage,omitempty"`
}

type aiModelWire struct {
	ID          string       `json:"id"`
	Caps        []string     `json:"caps"`
	ContextK    int          `json:"contextK,omitempty"`
	Price       *aiPriceWire `json:"price,omitempty"`
	Recommended bool         `json:"recommended,omitempty"`
	Manual      bool         `json:"manual,omitempty"`
}

// aiTestWire là body của POST .../test.
type aiTestWire struct {
	OK    bool   `json:"ok"`
	Ms    int64  `json:"ms"`
	Error string `json:"error,omitempty"`
}

// aiCheckWire là body của POST .../models/check (và của create/patch khi lỗi).
type aiCheckWire struct {
	OK       bool     `json:"ok"`
	Ms       int64    `json:"ms"`
	Caps     []string `json:"caps,omitempty"`
	ContextK int      `json:"contextK,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// aiLoginWire là trạng thái luồng đăng nhập — GET poll 2 giây khi hộp mở.
type aiLoginWire struct {
	Phase     string `json:"phase"`
	AuthURL   string `json:"authUrl,omitempty"`
	Code      string `json:"code,omitempty"`
	VerifyURL string `json:"verifyUrl,omitempty"`
	// ExpiresAt: hạn mã thiết bị (expires_in của provider) — UI đếm ngược.
	ExpiresAt string     `json:"expiresAt,omitempty"`
	Account   *aiAccount `json:"account,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// aiAccount là tài khoản đã đăng nhập (tách email/plan từ nhãn core/auth).
type aiAccount struct {
	Email string `json:"email,omitempty"`
	Plan  string `json:"plan,omitempty"`
}

func providerToWire(v aiconn.ProviderView) aiProviderWire {
	w := aiProviderWire{
		ID: v.ID, Kind: v.Kind, Auth: v.Auth, Label: v.Label,
		KeyHint: v.KeyHint, BaseURL: v.BaseURL, Status: v.Status,
		Plan: v.Plan, CatalogCount: v.Count, Selected: v.Selected,
		DailyPaidImageLimit: v.DailyPaidImageLimit,
	}
	if w.Selected == nil {
		w.Selected = []string{}
	}
	if v.CatalogAt > 0 {
		w.CatalogAt = formatMillis(v.CatalogAt)
	}
	if v.Quota != nil {
		w.Quota = &aiQuotaWire{
			Used: v.Quota.Used, Limit: v.Quota.Limit,
			Period: v.Quota.Period, ResetAt: v.Quota.ResetAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		}
	}
	return w
}

func providersToWire(views []aiconn.ProviderView) []aiProviderWire {
	out := make([]aiProviderWire, 0, len(views))
	for _, v := range views {
		out = append(out, providerToWire(v))
	}
	return out
}

func modelToWire(m domain.AIModel) aiModelWire {
	w := aiModelWire{ID: m.ID, Caps: m.Caps, ContextK: m.ContextK,
		Recommended: m.Recommended, Manual: m.Manual}
	if w.Caps == nil {
		w.Caps = []string{}
	}
	if m.InPerM > 0 || m.OutPerM > 0 || m.PerImage > 0 {
		w.Price = &aiPriceWire{InPerM: m.InPerM, OutPerM: m.OutPerM, PerImage: m.PerImage}
	}
	return w
}

// aiModelPageWire là body của GET .../models: một trang + tổng + con trỏ sau.
type aiModelPageWire struct {
	Models     []aiModelWire `json:"models"`
	Total      int           `json:"total"`
	NextCursor *string       `json:"nextCursor"`
}

// ── wire của Trợ lý AI (§15 AgentModels) ────────────────────────────────────
// Web tự sở hữu hình dạng JSON thay vì serialize domain.AgentModels, để đổi
// kiểu nghiệp vụ không âm thầm đổi hợp đồng.

type aiModelRefWire struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type aiDirectorWire struct {
	Model    aiModelRefWire   `json:"model"`
	Effort   string           `json:"effort"`
	Fallback []aiModelRefWire `json:"fallback"`
}

type aiArtistWire struct {
	PromptModel  aiModelRefWire   `json:"promptModel"`
	Draw         aiModelRefWire   `json:"draw"`
	FallbackOn   bool             `json:"fallbackOn"`
	DrawFallback []aiModelRefWire `json:"drawFallback"`
}

type aiAgentsWire struct {
	Director aiDirectorWire `json:"director"`
	Artist   aiArtistWire   `json:"artist"`
}

func refFromWire(w aiModelRefWire) domain.ModelRef {
	return domain.ModelRef{Provider: w.Provider, Model: w.Model}
}

func refsFromWire(in []aiModelRefWire) []domain.ModelRef {
	out := make([]domain.ModelRef, 0, len(in))
	for _, r := range in {
		out = append(out, refFromWire(r))
	}
	return out
}

func refToWire(r domain.ModelRef) aiModelRefWire {
	return aiModelRefWire{Provider: r.Provider, Model: r.Model}
}

func refsToWire(in []domain.ModelRef) []aiModelRefWire {
	out := make([]aiModelRefWire, 0, len(in))
	for _, r := range in {
		out = append(out, refToWire(r))
	}
	return out
}

func agentsToModel(w aiAgentsWire) domain.AgentModels {
	return domain.AgentModels{
		Director: domain.DirectorModels{
			Model:    refFromWire(w.Director.Model),
			Effort:   w.Director.Effort,
			Fallback: refsFromWire(w.Director.Fallback),
		},
		Artist: domain.ArtistModels{
			PromptModel:  refFromWire(w.Artist.PromptModel),
			Draw:         refFromWire(w.Artist.Draw),
			FallbackOn:   w.Artist.FallbackOn,
			DrawFallback: refsFromWire(w.Artist.DrawFallback),
		},
	}
}

func agentsToWire(a domain.AgentModels) aiAgentsWire {
	return aiAgentsWire{
		Director: aiDirectorWire{
			Model:    refToWire(a.Director.Model),
			Effort:   a.Director.Effort,
			Fallback: refsToWire(a.Director.Fallback),
		},
		Artist: aiArtistWire{
			PromptModel:  refToWire(a.Artist.PromptModel),
			Draw:         refToWire(a.Artist.Draw),
			FallbackOn:   a.Artist.FallbackOn,
			DrawFallback: refsToWire(a.Artist.DrawFallback),
		},
	}
}

func modelsToWire(models []domain.AIModel) []aiModelWire {
	out := make([]aiModelWire, 0, len(models))
	for _, m := range models {
		out = append(out, modelToWire(m))
	}
	return out
}

func aiLoginToWire(s *aiconn.LoginSession) aiLoginWire {
	if s == nil {
		return aiLoginWire{Phase: aiconn.LoginIdle}
	}
	w := aiLoginWire{Phase: s.Phase, AuthURL: s.AuthURL, Code: s.Code,
		VerifyURL: s.VerifyURL, Error: s.Error}
	if !s.ExpiresAt.IsZero() {
		w.ExpiresAt = s.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if s.Account != "" {
		w.Account = aiAccountFromLabel(s.Account)
	}
	return w
}

// aiAccountFromLabel tách "email · tổ chức (plan)" thành email/plan hiển thị.
func aiAccountFromLabel(label string) *aiAccount {
	acc := &aiAccount{}
	rest := label
	if i := strings.Index(rest, " · "); i >= 0 {
		rest = rest[:i]
	}
	if strings.Contains(rest, "@") {
		acc.Email = rest
	}
	if i := strings.LastIndex(label, "("); i >= 0 && strings.HasSuffix(label, ")") {
		acc.Plan = label[i+1 : len(label)-1]
	}
	return acc
}

// formatMillis in mốc Unix mili-giây theo ISO-8601 UTC.
func formatMillis(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

// ── routes ────────────────────────────────────────────────────────────────

// MountAI đăng ký các route /api/ai của §15 trên gateway.
func MountAI(g *Gateway, svc AIService) {
	g.Handle(http.MethodGet, "/api/ai/providers", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		list, err := svc.Providers(r.Context())
		if err != nil {
			writeAIError(w, err)
			return
		}
		wire := providersToWire(list)
		if !isAdmin(user) {
			for i := range wire {
				wire[i].KeyHint, wire[i].BaseURL = "", ""
			}
		}
		webapp.WriteJSON(w, http.StatusOK, wire)
	})
	g.Handle(http.MethodPost, "/api/ai/providers", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		var in aiconn.CreateProviderInput
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		view, err := svc.CreateProvider(r.Context(), in)
		if err != nil {
			if ae, ok := aiconn.AsAPIError(err); ok && ae.Reason == domain.ReasonAuthFailed {
				// Thẻ vẫn được tạo với status auth_failed; body mang reason + status.
				// Thẻ đã tạo với status auth_failed; body mang thêm id/keyHint
				// để UI mở thẳng thẻ đó. "status" trong Fields là HTTP status
				// của provider (401) theo hợp đồng.
				writeAIErrorBody(w, ae, map[string]any{
					"id": view.ID, "keyHint": view.KeyHint,
				})
				return
			}
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusCreated, providerToWire(view))
	})
	g.Handle(http.MethodGet, "/api/ai/agents", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		agents, err := svc.Agents(r.Context())
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, agentsToWire(agents))
	})
	g.Handle(http.MethodPut, "/api/ai/agents", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		var in aiAgentsWire
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		agents, err := svc.PutAgents(r.Context(), agentsToModel(in))
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, agentsToWire(agents))
	})
	g.Handle(http.MethodGet, "/api/ai/providers/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeAIProvider(w, r, svc)
	})
	g.Handle(http.MethodPatch, "/api/ai/providers/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeAIProvider(w, r, svc)
	})
	g.Handle(http.MethodPost, "/api/ai/providers/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeAIProvider(w, r, svc)
	})
	g.Handle(http.MethodDelete, "/api/ai/providers/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeAIProvider(w, r, svc)
	})
}

// routeAIProvider phân nhánh phần sau /api/ai/providers/:
// <id> | <id>/models | <id>/models/check | <id>/test | <id>/login |
// <id>/login/cancel | <id>/login/code | <id>/logout.
func routeAIProvider(w http.ResponseWriter, r *http.Request, svc AIService) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/ai/providers/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		webapp.Error(w, http.StatusNotFound, "Không có route này")
		return
	}
	id := parts[0]
	ctx := r.Context()
	method := r.Method

	switch {
	case len(parts) == 1 && method == http.MethodPatch:
		var in aiconn.PatchProviderInput
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		view, err := svc.PatchProvider(ctx, id, in)
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, providerToWire(view))
	case len(parts) == 1 && method == http.MethodDelete:
		if err := svc.DeleteProvider(ctx, id); err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 2 && parts[1] == "models" && method == http.MethodGet:
		query := r.URL.Query()
		limit := 0
		if raw := query.Get("limit"); raw != "" {
			n, cerr := strconv.Atoi(raw)
			if cerr != nil || n < 0 {
				webapp.Error(w, http.StatusBadRequest, "limit không hợp lệ")
				return
			}
			limit = n
		}
		page, err := svc.Models(ctx, id, query.Get("q"), query.Get("filter"), query.Get("cursor"), limit)
		if err != nil {
			writeAIError(w, err)
			return
		}
		out := aiModelPageWire{Models: modelsToWire(page.Models), Total: page.Total}
		if page.HasMore && len(page.Models) > 0 {
			last := page.Models[len(page.Models)-1].ID
			out.NextCursor = &last
		}
		webapp.WriteJSON(w, http.StatusOK, out)
	case len(parts) == 3 && parts[1] == "models" && parts[2] == "check" && method == http.MethodPost:
		var in struct {
			Model string `json:"model"`
		}
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		res, err := svc.CheckModel(ctx, id, in.Model)
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, aiCheckWire{
			OK: res.OK, Ms: res.Ms, Caps: res.Caps, ContextK: res.ContextK, Error: res.Error,
		})
	case len(parts) == 2 && parts[1] == "test" && method == http.MethodPost:
		res, err := svc.TestProvider(ctx, id)
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, aiTestWire{OK: res.OK, Ms: res.Ms, Error: res.Error})
	case len(parts) == 2 && parts[1] == "login" && method == http.MethodGet:
		sess, err := svc.LoginStatus(ctx, id)
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, aiLoginToWire(sess))
	case len(parts) == 2 && parts[1] == "login" && method == http.MethodPost:
		var in struct {
			Mode string `json:"mode"`
		}
		// Body tuỳ chọn: {} nghĩa là luồng trình duyệt mặc định.
		if r.ContentLength > 0 && !webapp.DecodeJSON(w, r, &in) {
			return
		}
		sess, err := svc.StartLogin(ctx, id, in.Mode)
		if err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusAccepted, aiLoginToWire(sess))
	case len(parts) == 3 && parts[1] == "login" && parts[2] == "cancel" && method == http.MethodPost:
		if err := svc.CancelLogin(ctx, id); err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 3 && parts[1] == "login" && parts[2] == "code" && method == http.MethodPost:
		var in struct {
			Code string `json:"code"`
		}
		if !webapp.DecodeJSON(w, r, &in) {
			return
		}
		if err := svc.SubmitLoginCode(ctx, id, in.Code); err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 2 && parts[1] == "logout" && method == http.MethodPost:
		if err := svc.Logout(ctx, id); err != nil {
			writeAIError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		webapp.Error(w, http.StatusNotFound, "Không có route này")
	}
}

// writeAIError ghi lỗi nghiệp vụ {error, reason?, ...fields}; lỗi khác là 500
// với câu trung tính (không lộ chi tiết nội bộ).
func writeAIError(w http.ResponseWriter, err error) {
	if ae, ok := aiconn.AsAPIError(err); ok {
		writeAIErrorBody(w, ae, nil)
		return
	}
	webapp.Error(w, http.StatusInternalServerError, "Không xử lý được yêu cầu")
}

func writeAIErrorBody(w http.ResponseWriter, ae *aiconn.Error, extra map[string]any) {
	body := map[string]any{"error": ae.Message, "reason": ae.Reason}
	for k, v := range ae.Fields {
		body[k] = v
	}
	for k, v := range extra {
		body[k] = v
	}
	webapp.WriteJSON(w, ae.Status, body)
}
