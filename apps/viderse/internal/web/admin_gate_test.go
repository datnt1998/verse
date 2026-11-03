package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
)

// gateSessions dựng gateway trên kho tài khoản thật với một quản trị viên và
// một thành viên có quyền Viderse; trả hai cookie phiên.
func gateSessions(t *testing.T) (p *pgGateway, admin, member string) {
	t.Helper()
	p = newPGGateway(t)
	if _, err := p.fx.Admin.CreateUser(identity.NewUser{Username: "admin", Password: "admin-pass", Role: identity.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	p.fx.CreateMember(t, "member", "member-pass", false, identity.AppViderse)
	return p, sessionCookieOf(t, p.login("admin", "admin-pass")), sessionCookieOf(t, p.login("member", "member-pass"))
}

func TestMachineAdminOnlyRoutes(t *testing.T) {
	gated := []struct{ method, path string }{
		{http.MethodPost, "/api/ai/providers"},
		{http.MethodPatch, "/api/ai/providers/openai-1"},
		{http.MethodDelete, "/api/ai/providers/openai-1"},
		{http.MethodPost, "/api/ai/providers/openai-1/test"},
		{http.MethodPost, "/api/ai/providers/openai-1/models/check"},
		{http.MethodPost, "/api/ai/providers/chatgpt/login"},
		{http.MethodGet, "/api/ai/providers/chatgpt/login"},
		{http.MethodPost, "/api/ai/providers/chatgpt/logout"},
		{http.MethodPut, "/api/ai/agents"},
		{http.MethodPut, "/api/platforms/youtube/login"},
		{http.MethodDelete, "/api/platforms/youtube/login"},
		{http.MethodGet, "/api/platforms/youtube/login/profiles"},
		{http.MethodPost, "/api/platforms/bilibili/login/qr"},
		{http.MethodGet, "/api/platforms/bilibili/login/qr/key"},
		{http.MethodPost, "/api/tools/check"},
		{http.MethodPost, "/api/tools/ytdlp/install"},
		{http.MethodPost, "/api/tools/ytdlp/cancel"},
		{http.MethodPost, "/api/setup/start"},
		{http.MethodPost, "/api/setup/retry"},
		{http.MethodPost, "/api/setup/check"},
		{http.MethodPost, "/api/storage/cleanup/preview"},
		{http.MethodPost, "/api/storage/cleanup/run"},
		{http.MethodPost, "/api/storage/move"},
		{http.MethodGet, "/api/storage/browse"},
		{http.MethodGet, "/api/storage/reclaim"},
		{http.MethodPut, "/api/notifications/settings"},
	}
	for _, c := range gated {
		if !machineAdminOnly(c.method, c.path) {
			t.Errorf("%s %s phải chỉ dành cho quản trị viên", c.method, c.path)
		}
	}
	open := []struct{ method, path string }{
		{http.MethodGet, "/api/ai/providers"},
		{http.MethodGet, "/api/ai/providers/openai-1/models"},
		{http.MethodGet, "/api/ai/agents"},
		{http.MethodGet, "/api/platforms"},
		{http.MethodPost, "/api/platforms/requests"},
		{http.MethodGet, "/api/tools"},
		{http.MethodGet, "/api/tools/logs"},
		{http.MethodGet, "/api/setup/state"},
		{http.MethodGet, "/api/storage/server"},
		{http.MethodGet, "/api/notifications/settings"},
		{http.MethodPost, "/api/clone/enqueue"},
	}
	for _, c := range open {
		if machineAdminOnly(c.method, c.path) {
			t.Errorf("%s %s phải mở cho thành viên", c.method, c.path)
		}
	}
}

func TestMemberCannotWriteAIProviders(t *testing.T) {
	ai := &fakeAI{providers: []aiconn.ProviderView{{
		ID: "compat-1", Kind: domain.KindOpenAICompat, Auth: domain.AuthAPIKey, Label: "Tương thích OpenAI",
		KeyHint: "••••7c2e", BaseURL: "https://llm.example.com/v1", Status: domain.StatusReady, Selected: []string{"m"},
	}}}
	p, admin, member := gateSessions(t)
	MountAI(p.g, ai)

	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/ai/providers", `{"kind":"ollama","baseUrl":"http://127.0.0.1:8334"}`},
		{http.MethodPatch, "/api/ai/providers/compat-1", `{"baseUrl":"https://attacker.example"}`},
		{http.MethodDelete, "/api/ai/providers/compat-1", `{}`},
		{http.MethodPost, "/api/ai/providers/compat-1/test", `{}`},
		{http.MethodPut, "/api/ai/agents", `{}`},
	} {
		w := p.do(c.method, c.path, member, c.body)
		if w.Code != http.StatusForbidden {
			t.Errorf("member %s %s = %d, muốn 403 (%s)", c.method, c.path, w.Code, w.Body.String())
		}
	}
	if ai.createIn.Kind != "" || ai.patchIn.BaseURL != nil || ai.lastID != "" {
		t.Fatalf("handler vẫn chạy với thành viên: create=%+v patch=%+v id=%q", ai.createIn, ai.patchIn, ai.lastID)
	}

	// Đọc phục vụ công việc vẫn mở nhưng không lộ khoá hay địa chỉ của chủ.
	w := p.do(http.MethodGet, "/api/ai/providers", member, "")
	if w.Code != http.StatusOK {
		t.Fatalf("member đọc provider = %d", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "7c2e") || strings.Contains(body, "llm.example.com") {
		t.Fatalf("lộ khoá/địa chỉ cho thành viên: %s", body)
	}
	w = p.do(http.MethodGet, "/api/ai/providers", admin, "")
	if body := w.Body.String(); !strings.Contains(body, "7c2e") || !strings.Contains(body, "llm.example.com") {
		t.Fatalf("admin mất khoá/địa chỉ: %s", body)
	}

	// Quản trị viên vẫn ghi được.
	w = p.do(http.MethodPatch, "/api/ai/providers/compat-1", admin, `{"baseUrl":"https://llm2.example.com/v1","apiKey":"sk-new"}`)
	if w.Code != http.StatusOK || ai.patchIn.BaseURL == nil {
		t.Fatalf("admin PATCH = %d %s", w.Code, w.Body.String())
	}
}

// Cổng nằm ở gateway, không ở handler: handler nào đăng ký dưới các đường dẫn
// cấu hình toàn máy cũng không chạy với thành viên.
func TestGatewayGateStopsMemberBeforeAnyHandler(t *testing.T) {
	p, admin, member := gateSessions(t)
	ran := 0
	spy := func(w http.ResponseWriter, _ *http.Request, _ identity.User) {
		ran++
		w.WriteHeader(http.StatusNoContent)
	}
	routes := []struct{ method, path string }{
		{http.MethodPut, "/api/platforms/youtube/login"},
		{http.MethodGet, "/api/platforms/youtube/login/profiles"},
		{http.MethodPost, "/api/tools/ytdlp/install"},
		{http.MethodPost, "/api/setup/start"},
		{http.MethodPost, "/api/storage/cleanup/run"},
		{http.MethodPut, "/api/notifications/settings"},
	}
	for _, rt := range routes {
		p.g.Handle(rt.method, rt.path, spy)
	}
	p.g.Handle(http.MethodPost, "/api/platforms/requests", spy)
	for _, rt := range routes {
		if w := p.do(rt.method, rt.path, member, "{}"); w.Code != http.StatusForbidden {
			t.Errorf("member %s %s = %d, muốn 403", rt.method, rt.path, w.Code)
		}
		if w := p.do(rt.method, rt.path, admin, "{}"); w.Code != http.StatusNoContent {
			t.Errorf("admin %s %s = %d, muốn 204", rt.method, rt.path, w.Code)
		}
	}
	if ran != len(routes) {
		t.Fatalf("handler chạy %d lần, muốn %d (chỉ admin)", ran, len(routes))
	}
	// Yêu cầu nền tảng là việc của thành viên.
	if w := p.do(http.MethodPost, "/api/platforms/requests", member, "{}"); w.Code != http.StatusNoContent {
		t.Fatalf("member POST /api/platforms/requests = %d", w.Code)
	}
}
