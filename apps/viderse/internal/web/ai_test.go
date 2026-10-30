// Test hợp đồng HTTP của Kết nối AI (§15): hình body, mã lỗi, phân nhánh
// route và auth. Không gọi mạng — service giả.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeAI là AIService giả ghi lại lời gọi.
type fakeAI struct {
	providers  []aiconn.ProviderView
	models     []domain.AIModel
	createIn   aiconn.CreateProviderInput
	patchIn    aiconn.PatchProviderInput
	createErr  error
	patchErr   error
	deleteErr  error
	testErr    error
	checkErr   error
	agents     domain.AgentModels
	putErr     error
	sess       *aiconn.LoginSession
	loginMode  string
	codeIn     string
	logoutID   string
	lastID     string
	lastQ      string
	lastFilt   string
	lastCursor string
	lastModel  string
	hasMore    bool
}

func (f *fakeAI) Providers(ctx context.Context) ([]aiconn.ProviderView, error) {
	return f.providers, nil
}

func (f *fakeAI) Models(ctx context.Context, id, q, filter, cursor string, limit int) (aiconn.ModelPage, error) {
	f.lastID, f.lastQ, f.lastFilt, f.lastCursor = id, q, filter, cursor
	return aiconn.ModelPage{Models: f.models, Total: len(f.models), HasMore: f.hasMore}, nil
}

func (f *fakeAI) CreateProvider(ctx context.Context, in aiconn.CreateProviderInput) (aiconn.ProviderView, error) {
	f.createIn = in
	if f.createErr != nil {
		return aiconn.ProviderView{ID: "new-1", Status: domain.StatusAuthFailed,
			KeyHint: "••••bad"}, f.createErr
	}
	return aiconn.ProviderView{ID: "new-1", Kind: in.Kind,
		Auth: domain.AuthAPIKey, Label: "OpenAI", Status: domain.StatusReady,
		KeyHint: "••••7c2e", Selected: []string{}, Count: 2}, nil
}

func (f *fakeAI) PatchProvider(ctx context.Context, id string, in aiconn.PatchProviderInput) (aiconn.ProviderView, error) {
	f.lastID, f.patchIn = id, in
	if f.patchErr != nil {
		return aiconn.ProviderView{}, f.patchErr
	}
	return aiconn.ProviderView{ID: id, Status: domain.StatusReady}, nil
}

func (f *fakeAI) DeleteProvider(ctx context.Context, id string) error {
	f.lastID = id
	return f.deleteErr
}

func (f *fakeAI) TestProvider(ctx context.Context, id string) (aiconn.TestResult, error) {
	f.lastID = id
	if f.testErr != nil {
		return aiconn.TestResult{}, f.testErr
	}
	return aiconn.TestResult{OK: true, Ms: 12}, nil
}

func (f *fakeAI) CheckModel(ctx context.Context, id, model string) (aiconn.CheckModelResult, error) {
	f.lastID, f.lastModel = id, model
	if f.checkErr != nil {
		return aiconn.CheckModelResult{}, f.checkErr
	}
	return aiconn.CheckModelResult{OK: true, Ms: 40, Caps: []domain.Capability{"text", "vision"}, ContextK: 128}, nil
}

func (f *fakeAI) Agents(ctx context.Context) (domain.AgentModels, error) {
	if f.agents.Director.Model.Provider == "" {
		return domain.DefaultAgentModels(), nil
	}
	return f.agents, nil
}

func (f *fakeAI) PutAgents(ctx context.Context, agents domain.AgentModels) (domain.AgentModels, error) {
	if f.putErr != nil {
		return domain.AgentModels{}, f.putErr
	}
	f.agents = agents
	return agents, nil
}

func (f *fakeAI) LoginStatus(ctx context.Context, id string) (*aiconn.LoginSession, error) {
	f.lastID = id
	if f.sess == nil {
		return &aiconn.LoginSession{Phase: aiconn.LoginIdle}, nil
	}
	return f.sess, nil
}

func (f *fakeAI) StartLogin(ctx context.Context, id, mode string) (*aiconn.LoginSession, error) {
	f.lastID, f.loginMode = id, mode
	return &aiconn.LoginSession{Phase: aiconn.LoginAuthorize, AuthURL: "https://auth.example/x"}, nil
}

func (f *fakeAI) CancelLogin(ctx context.Context, id string) error { f.lastID = id; return nil }

func (f *fakeAI) SubmitLoginCode(ctx context.Context, id, code string) error {
	f.lastID, f.codeIn = id, code
	return nil
}

func (f *fakeAI) Logout(ctx context.Context, id string) error { f.logoutID = id; return nil }

// aiGateway dựng gateway có MountAI + cookie đăng nhập admin.
func aiGateway(t *testing.T, ai *fakeAI) (*testGateway, string) {
	t.Helper()
	tg := newTestGateway(t)
	MountAI(tg.g, ai)
	return tg, tg.sessionCookie()
}

func TestAIProvidersShapeAndAuth(t *testing.T) {
	ai := &fakeAI{providers: []aiconn.ProviderView{
		{ID: "chatgpt", Kind: domain.KindChatGPT, Auth: domain.AuthSubscription,
			Label: "ChatGPT", Status: domain.StatusReady, Selected: []string{"gpt-6.1-sol"},
			CatalogAt: time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC).UnixMilli(),
			Count:     2,
			Quota: &domain.Quota{Used: 3, Limit: 50, Period: "day",
				ResetAt: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}},
	}}
	tg, cookie := aiGateway(t, ai)
	// Không cookie → 401.
	if w := tg.do(http.MethodGet, "/api/ai/providers", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("thiếu đăng nhập = %d", w.Code)
	}
	w := tg.do(http.MethodGet, "/api/ai/providers", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %v", list)
	}
	p := list[0]
	if p["id"] != "chatgpt" || p["auth"] != "subscription" || p["catalogCount"] != float64(2) {
		t.Fatalf("provider = %v", p)
	}
	if p["catalogAt"] != "2026-10-30T09:00:00.000Z" {
		t.Fatalf("catalogAt = %v", p["catalogAt"])
	}
	quota, ok := p["quota"].(map[string]any)
	if !ok || quota["used"] != float64(3) || quota["limit"] != float64(50) || quota["period"] != "day" {
		t.Fatalf("quota = %v", p["quota"])
	}
	if _, exists := p["keyEnc"]; exists {
		t.Fatal("lộ trường nội bộ")
	}
}

func TestAICreateProviderAuthFailed(t *testing.T) {
	ai := &fakeAI{createErr: &aiconn.Error{Status: http.StatusUnprocessableEntity,
		Reason: domain.ReasonAuthFailed, Message: "Nhà cung cấp từ chối khoá API này.",
		Fields: map[string]any{"status": 401}}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodPost, "/api/ai/providers", cookie,
		`{"kind":"openai","apiKey":"sk-bad"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", w.Code)
	}
	body := decode(t, w)
	if body["reason"] != domain.ReasonAuthFailed || body["status"] != float64(401) ||
		body["id"] != "new-1" || body["keyHint"] != "••••bad" {
		t.Fatalf("body = %v", body)
	}
	if ai.createIn.Kind != domain.KindOpenAI || ai.createIn.APIKey != "sk-bad" {
		t.Fatalf("input = %+v", ai.createIn)
	}
	ai.createErr = nil
	if w := tg.do(http.MethodPost, "/api/ai/providers", cookie, `{"kind":"openai"}`); w.Code != http.StatusCreated {
		t.Fatalf("tạo thành công = %d", w.Code)
	}
}

func TestAIDeleteInUse(t *testing.T) {
	ai := &fakeAI{deleteErr: &aiconn.Error{Status: http.StatusConflict,
		Reason: domain.ReasonInUse, Message: "đang dùng",
		Fields: map[string]any{"agents": []string{"director", "artist"}}}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodDelete, "/api/ai/providers/p1", cookie, `{}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d", w.Code)
	}
	body := decode(t, w)
	if body["reason"] != domain.ReasonInUse {
		t.Fatalf("body = %v", body)
	}
	agents, _ := body["agents"].([]any)
	if len(agents) != 2 {
		t.Fatalf("agents = %v", body["agents"])
	}
}

func TestAIModelsRoute(t *testing.T) {
	ai := &fakeAI{models: []domain.AIModel{
		{ID: "gpt-6.1-sol", Caps: []domain.Capability{"text", "vision"}, ContextK: 272,
			InPerM: 1.25, OutPerM: 10, Recommended: true},
		{ID: "gpt-image-2", Caps: []domain.Capability{"image"}, PerImage: 0.04},
	}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodGet, "/api/ai/providers/chatgpt/models?q=gpt&filter=selected", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ai.lastID != "chatgpt" || ai.lastQ != "gpt" || ai.lastFilt != "selected" {
		t.Fatalf("tham số = %q %q %q", ai.lastID, ai.lastQ, ai.lastFilt)
	}
	var page struct {
		Models     []map[string]any `json:"models"`
		Total      int              `json:"total"`
		NextCursor *string          `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	models := page.Models
	if page.Total != 2 || page.NextCursor != nil {
		t.Fatalf("page = %+v", page)
	}
	if len(models) != 2 || models[0]["contextK"] != float64(272) || models[0]["recommended"] != true {
		t.Fatalf("models = %v", models)
	}
	price, _ := models[0]["price"].(map[string]any)
	if price["inPerM"] != 1.25 {
		t.Fatalf("price = %v", price)
	}
	if _, ok := models[1]["price"].(map[string]any); !ok {
		t.Fatalf("model ảnh thiếu price: %v", models[1])
	}
}

func TestAIModelsCheckAndTest(t *testing.T) {
	ai := &fakeAI{}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodPost, "/api/ai/providers/p1/models/check", cookie, `{"model":"custom-vl"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decode(t, w)
	if body["ok"] != true || body["contextK"] != float64(128) || len(body["caps"].([]any)) != 2 {
		t.Fatalf("check = %v", body)
	}
	if ai.lastModel != "custom-vl" {
		t.Fatalf("model = %q", ai.lastModel)
	}
	w = tg.do(http.MethodPost, "/api/ai/providers/p1/test", cookie, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("test status = %d", w.Code)
	}
	if body := decode(t, w); body["ok"] != true || body["ms"] != float64(12) {
		t.Fatalf("test = %v", body)
	}
}

func TestAIPatchSelected(t *testing.T) {
	ai := &fakeAI{}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodPatch, "/api/ai/providers/p1", cookie, `{"selected":["a","b"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ai.patchIn.Selected == nil || len(*ai.patchIn.Selected) != 2 {
		t.Fatalf("patchIn = %+v", ai.patchIn)
	}
}

func TestAIAgentsPut(t *testing.T) {
	ai := &fakeAI{}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodGet, "/api/ai/agents", cookie, "")
	body := decode(t, w)
	director := body["director"].(map[string]any)
	if director["effort"] != "auto" {
		t.Fatalf("agents = %v", body)
	}
	w = tg.do(http.MethodPut, "/api/ai/agents", cookie, `{
		"director":{"model":{"provider":"p1","model":"m1"},"effort":"high","fallback":[]},
		"artist":{"promptModel":{"provider":"p1","model":"m2"},"draw":{"provider":"p1","model":"m3"},
			"fallbackOn":false,"drawFallback":[]}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ai.agents.Director.Effort != "high" || ai.agents.Artist.Draw.Model != "m3" {
		t.Fatalf("agents = %+v", ai.agents)
	}
}

func TestAIAgentsCapabilityMissing(t *testing.T) {
	ai := &fakeAI{putErr: &aiconn.Error{Status: http.StatusUnprocessableEntity,
		Reason: domain.ReasonCapabilityMissing, Message: "thiếu",
		Fields: map[string]any{"need": "vision", "field": "artist.promptModel"}}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodPut, "/api/ai/agents", cookie, `{"director":{},"artist":{}}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", w.Code)
	}
	body := decode(t, w)
	if body["reason"] != domain.ReasonCapabilityMissing || body["need"] != "vision" {
		t.Fatalf("body = %v", body)
	}
}

func TestAILoginRoutes(t *testing.T) {
	ai := &fakeAI{sess: &aiconn.LoginSession{Phase: aiconn.LoginCode, Code: "ABCD-1234",
		VerifyURL: "https://auth.example/device"}}
	tg, cookie := aiGateway(t, ai)
	// GET trạng thái khi đang chờ mã.
	w := tg.do(http.MethodGet, "/api/ai/providers/chatgpt/login", cookie, "")
	body := decode(t, w)
	if body["phase"] != "code" || body["code"] != "ABCD-1234" || body["verifyUrl"] != "https://auth.example/device" {
		t.Fatalf("login = %v", body)
	}
	// POST khởi động với mode thiết bị.
	w = tg.do(http.MethodPost, "/api/ai/providers/grok/login", cookie, `{"mode":"device"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start = %d", w.Code)
	}
	if ai.loginMode != "device" || ai.lastID != "grok" {
		t.Fatalf("start = %q %q", ai.loginMode, ai.lastID)
	}
	if body := decode(t, w); body["phase"] != "authorize" || body["authUrl"] == "" {
		t.Fatalf("start body = %v", body)
	}
	// Body rỗng là luồng mặc định.
	if w := tg.do(http.MethodPost, "/api/ai/providers/chatgpt/login", cookie, `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("body rỗng = %d", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/ai/providers/chatgpt/login/code", cookie, `{"code":"XYZ"}`); w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	if ai.codeIn != "XYZ" {
		t.Fatalf("code = %q", ai.codeIn)
	}
	if w := tg.do(http.MethodPost, "/api/ai/providers/chatgpt/login/cancel", cookie, `{}`); w.Code != http.StatusOK {
		t.Fatalf("cancel = %d", w.Code)
	}
	if w := tg.do(http.MethodPost, "/api/ai/providers/chatgpt/logout", cookie, `{}`); w.Code != http.StatusOK {
		t.Fatalf("logout = %d", w.Code)
	}
	if ai.logoutID != "chatgpt" {
		t.Fatalf("logout = %q", ai.logoutID)
	}
}

func TestAIUnknownRoute(t *testing.T) {
	ai := &fakeAI{}
	tg, cookie := aiGateway(t, ai)
	if w := tg.do(http.MethodGet, "/api/ai/providers/p1/bogus", cookie, ""); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

// Trang danh mục có con trỏ sau: nextCursor = id cuối trang, total = tổng bộ
// lọc (không phải số phần tử trang) — UI dừng phân trang khi null.
func TestAIModelsPaginationWire(t *testing.T) {
	ai := &fakeAI{hasMore: true, models: []domain.AIModel{
		{ID: "m-1", Caps: []domain.Capability{"text"}},
		{ID: "m-2", Caps: []domain.Capability{"text"}},
	}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodGet, "/api/ai/providers/p1/models?cursor=m-0&limit=2", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var page struct {
		Models     []map[string]any `json:"models"`
		Total      int              `json:"total"`
		NextCursor *string          `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if ai.lastCursor != "m-0" {
		t.Fatalf("cursor = %q", ai.lastCursor)
	}
	if page.NextCursor == nil || *page.NextCursor != "m-2" {
		t.Fatalf("nextCursor = %v", page.NextCursor)
	}
	if page.Total != 2 || len(page.Models) != 2 {
		t.Fatalf("page = %+v", page)
	}
}

// API chỉ trả keyHint đã che; không có trường ciphertext nào trong JSON.
func TestAIProviderWireNeverExposesCiphertext(t *testing.T) {
	ai := &fakeAI{providers: []aiconn.ProviderView{{
		ID: "openai-1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", KeyHint: "••••7c2e", Status: domain.StatusReady,
		BaseURL: "https://api.openai.com/v1",
	}}}
	tg, cookie := aiGateway(t, ai)
	w := tg.do(http.MethodGet, "/api/ai/providers", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, forbidden := range []string{"keyEnc", "encKey", "sk-live", "apiKey"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("body lộ %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "••••7c2e") {
		t.Fatalf("thiếu keyHint: %s", body)
	}
}
