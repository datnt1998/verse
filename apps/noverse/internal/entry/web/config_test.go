package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/anthropicoauth"
	"github.com/datnt1998/noverse/internal/auth"
	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/llm/wire"
	"github.com/datnt1998/noverse/internal/version"
	"github.com/datnt1998/noverse/internal/webauth"
	"github.com/datnt1998/noverse/internal/xai"
)

// configTestGateway 建带项目/故事目录结构与三个账号的网关。
// 目录：<root>/proj/{project.json, kich_ban/01-story}；writer 拥有 proj/*。
type configTestGateway struct {
	*testGateway
	home    string
	project string
	story   string
}

func newConfigTestGateway(t *testing.T) *configTestGateway {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	story := filepath.Join(project, "kich_ban", "01-story")
	if err := os.MkdirAll(story, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{"name":"P"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	accounts := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	for _, u := range []webauth.NewUser{
		{Username: "admin", Role: webauth.RoleAdmin, Password: "admin-pass"},
		{Username: "writer", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"proj": webauth.ProjectOwner}, Password: "writer-pass"},
		{Username: "reader", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"proj": webauth.ProjectView}, Password: "reader-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	return &configTestGateway{
		testGateway: &testGateway{t: t, g: NewGateway(root, accounts, version.Info{}), root: root},
		home:        home,
		project:     project,
		story:       story,
	}
}

func (tg *configTestGateway) globalConfig(t *testing.T, content string) {
	t.Helper()
	dir := filepath.Join(tg.home, ".ainovel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const configWithKey = `{
  "provider": "openai",
  "model": "gpt-4o",
  "providers": {
    "openai": {"type": "openai", "api_key": "sk-secret-1234567890", "models": [{"name": "gpt-4o"}]}
  }
}`

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %d: %v\n%s", w.Code, err, w.Body.String())
	}
	return m
}

// bodyText 递归收集 JSON 响应中的全部字符串值，供断言敏感内容不泄漏。
func bodyText(v any) string {
	var b strings.Builder
	var walk func(x any)
	walk = func(x any) {
		switch e := x.(type) {
		case map[string]any:
			for _, v := range e {
				walk(v)
			}
		case []any:
			for _, v := range e {
				walk(v)
			}
		case string:
			b.WriteString(e)
			b.WriteByte('\n')
		}
	}
	walk(v)
	return b.String()
}

// doWith 与 testGateway.do 相同，但允许在发送前改写请求（如 RemoteAddr）。
func (tg *testGateway) doWith(method, path, cookie, body string, tweak func(*http.Request)) *httptest.ResponseRecorder {
	tg.t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	if tweak != nil {
		tweak(req)
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, req)
	return w
}

// api_key 永不离开服务器：GET 任何 scope 的响应只含 api_key_set/api_key_hint。
func TestGetConfig_NeverReturnsAPIKey(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	admin := tg.login("admin", "admin-pass")

	for _, scope := range []string{"global", "project", "story"} {
		path := "/api/config?scope=" + scope
		if scope != "global" {
			path += "&project=proj"
		}
		if scope == "story" {
			path += "&story=01-story"
		}
		w := tg.do("GET", path, admin, "")
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d: %s", path, w.Code, w.Body.String())
		}
		m := decodeResp(t, w)
		if strings.Contains(bodyText(m), "sk-secret-1234567890") {
			t.Fatalf("GET %s làm lộ api_key", path)
		}
		eff := m["effective"].(map[string]any)
		pc := eff["providers"].(map[string]any)["openai"].(map[string]any)
		if pc["api_key_set"] != true {
			t.Fatalf("GET %s phải có api_key_set=true, nhận được %v", path, pc)
		}
		if hint, _ := pc["api_key_hint"].(string); hint == "" || strings.Contains(hint, "secret") {
			t.Fatalf("api_key_hint không được chứa phần giữa của khoá, nhận được %q", hint)
		}
	}
}

// 非 admin：providers 只剩 type/models。
func TestGetConfig_NonAdminProvidersStripped(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	reader := tg.login("reader", "reader-pass")

	w := tg.do("GET", "/api/config?scope=global", reader, "")
	if w.Code != 200 {
		t.Fatalf("GET -> %d: %s", w.Code, w.Body.String())
	}
	pc := decodeResp(t, w)["effective"].(map[string]any)["providers"].(map[string]any)["openai"].(map[string]any)
	if len(pc) != 2 || pc["type"] != "openai" || pc["models"] == nil {
		t.Fatalf("provider với người dùng không phải admin chỉ được còn type/models, nhận được %v", pc)
	}
	for _, forbidden := range []string{"api_key", "api_key_set", "base_url", "extra_body"} {
		if _, ok := pc[forbidden]; ok {
			t.Fatalf("người dùng không phải admin không được thấy %s", forbidden)
		}
	}
}

// PUT global：api_key 缺失时保留旧 key；clear_api_key 时清除。
func TestPutConfig_GlobalKeepsAndClearsKey(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	admin := tg.login("admin", "admin-pass")

	// 1) 不带 api_key → 旧 key 保留。
	w := tg.do("PUT", "/api/config?scope=global", admin,
		`{"provider":"openai","model":"gpt-4o-mini","providers":{"openai":{"type":"openai","models":[{"name":"gpt-4o-mini"}]}}}`)
	if w.Code != 200 {
		t.Fatalf("PUT -> %d: %s", w.Code, w.Body.String())
	}
	onDisk, _ := os.ReadFile(filepath.Join(tg.home, ".ainovel", "config.json"))
	if !strings.Contains(string(onDisk), "sk-secret-1234567890") {
		t.Fatalf("PUT không kèm api_key phải giữ key cũ, file: %s", onDisk)
	}
	if strings.Contains(w.Body.String(), "sk-secret-1234567890") {
		t.Fatal("phản hồi làm lộ api_key")
	}

	// 2) clear_api_key → 删除。
	w = tg.do("PUT", "/api/config?scope=global", admin,
		`{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","clear_api_key":true,"models":[{"name":"gpt-4o"}]}}}`)
	if w.Code != 200 {
		t.Fatalf("PUT clear -> %d: %s", w.Code, w.Body.String())
	}
	onDisk, _ = os.ReadFile(filepath.Join(tg.home, ".ainovel", "config.json"))
	if strings.Contains(string(onDisk), "sk-secret") || strings.Contains(string(onDisk), "api_key") {
		t.Fatalf("sau clear_api_key file không được còn api_key: %s", onDisk)
	}
}

// Bước Ngôn ngữ của Onboard ghi `language: "vi"`; zh (khoá thẻ Tiếng Trung) và giá trị lạ → 400.
func TestPutConfig_GlobalLanguage(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	admin := tg.login("admin", "admin-pass")
	body := func(lang string) string {
		return `{"provider":"openai","model":"gpt-4o-mini","language":"` + lang + `","providers":{"openai":{"type":"openai","models":[{"name":"gpt-4o-mini"}]}}}`
	}
	w := tg.do("PUT", "/api/config?scope=global", admin, body("vi"))
	if w.Code != 200 {
		t.Fatalf("PUT language=vi -> %d: %s", w.Code, w.Body.String())
	}
	onDisk, _ := os.ReadFile(filepath.Join(tg.home, ".ainovel", "config.json"))
	if !strings.Contains(string(onDisk), `"language": "vi"`) {
		t.Fatalf("language phải được ghi vào cấu hình: %s", onDisk)
	}
	if !strings.Contains(w.Body.String(), `"language"`) {
		t.Fatalf("phản hồi phải trả language: %s", w.Body.String())
	}
	for _, lang := range []string{"zh", "klingon"} {
		w := tg.do("PUT", "/api/config?scope=global", admin, body(lang))
		if w.Code != 400 || !strings.Contains(w.Body.String(), "Hiện chỉ hỗ trợ viết tiếng Việt") {
			t.Fatalf("language=%s phải trả 400 tiếng Việt, nhận %d: %s", lang, w.Code, w.Body.String())
		}
	}
}

// project/story scope 拒绝 api_key 与 notify；writer(p/*) 可写项目层。
func TestPutConfig_ProjectRejectsSecrets(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, configWithKey)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")

	w := tg.do("PUT", "/api/config?scope=project&project=proj", admin,
		`{"model":"x","providers":{"openai":{"api_key":"sk-x"}}}`)
	if w.Code != 400 {
		t.Fatalf("project PUT kèm api_key phải trả 400, nhận được %d: %s", w.Code, w.Body.String())
	}
	w = tg.do("PUT", "/api/config?scope=project&project=proj", admin, `{"model":"x","notify":{"enabled":true}}`)
	if w.Code != 400 {
		t.Fatalf("project PUT kèm notify phải trả 400, nhận được %d: %s", w.Code, w.Body.String())
	}

	// writer(p/*) 写项目层成功，round-trip 后 effective 反映覆盖。
	w = tg.do("PUT", "/api/config?scope=project&project=proj", writer, `{"model":"proj-model","style":"romance"}`)
	if w.Code != 200 {
		t.Fatalf("writer PUT project -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	target := m["target"].(map[string]any)
	if target["scope"] != "project" || target["exists"] != true {
		t.Fatalf("target phải là lớp project đã tồn tại: %v", target)
	}
	eff := m["effective"].(map[string]any)
	if eff["model"] != "proj-model" || eff["provider"] != "openai" {
		t.Fatalf("effective phải hợp nhất model lớp project với provider toàn cục: %v", eff)
	}
	sources := m["sources"].(map[string]any)
	if sources["model"] != "project" || sources["provider"] != "global" {
		t.Fatalf("quy nguồn sources sai: %v", sources)
	}

	// reader 不能写项目层（可读但不可管理 → 403，与 POST /stories 一致）。
	w = tg.do("PUT", "/api/config?scope=project&project=proj", tg.login("reader", "reader-pass"), `{"model":"y"}`)
	if w.Code != 403 {
		t.Fatalf("reader PUT phải trả 403, nhận được %d", w.Code)
	}
}

// 三级链的 effective/sources：故事层覆盖项目层、项目层覆盖全局。
func TestGetConfig_StoryChainSources(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, `{"provider":"openai","model":"g-model","reasoning_effort":"medium","providers":{"openai":{"type":"openai","api_key":"sk-secret-1234567890"}}}`)
	if err := os.MkdirAll(filepath.Join(tg.project, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tg.project, ".ainovel", "config.json"), []byte(`{"model":"p-model","style":"romance"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tg.story, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tg.story, ".ainovel", "config.json"), []byte(`{"model":"s-model","advance_mode":"review"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	w := tg.do("GET", "/api/config?scope=story&project=proj&story=01-story", tg.login("admin", "admin-pass"), "")
	if w.Code != 200 {
		t.Fatalf("GET story -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	eff := m["effective"].(map[string]any)
	if eff["model"] != "s-model" || eff["style"] != "romance" || eff["advance_mode"] != "review" {
		t.Fatalf("effective phải hợp nhất ba cấp: %v", eff)
	}
	sources := m["sources"].(map[string]any)
	if sources["model"] != "story" || sources["style"] != "project" || sources["reasoning_effort"] != "global" || sources["advance_mode"] != "story" {
		t.Fatalf("quy nguồn sources sai: %v", sources)
	}
	chain := m["chain"].([]any)
	if len(chain) != 3 {
		t.Fatalf("chuỗi phải có 3 lớp, nhận được %v", chain)
	}
}

// /api/rules：链、文件 CRUD、权限。
func TestRulesEndpoints(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	// 全局放一条规则，故事层写一条同名的（更近层赢 —— 由运行时 RawFileSources 保证，这里验证 API 层列链）。
	grules := filepath.Join(tg.home, ".ainovel", "rules")
	if err := os.MkdirAll(grules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grules, "tone.md"), []byte("global tone"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := tg.do("GET", "/api/rules?scope=story&project=proj&story=01-story", admin, "")
	if w.Code != 200 {
		t.Fatalf("GET rules -> %d: %s", w.Code, w.Body.String())
	}
	chain := decodeResp(t, w)["chain"].([]any)
	if len(chain) != 3 {
		t.Fatalf("chuỗi quy tắc phải có ba lớp global/project/story, nhận được %v", chain)
	}
	globalDoc := chain[0].(map[string]any)
	if globalDoc["scope"] != "global" || globalDoc["exists"] != true {
		t.Fatalf("lớp global phải tồn tại: %v", globalDoc)
	}

	// PUT 故事层规则文件。
	w = tg.do("PUT", "/api/rules/file", admin, `{"scope":"story","project":"proj","story":"01-story","name":"tone.md","text":"story tone"}`)
	if w.Code != 200 {
		t.Fatalf("PUT rule -> %d: %s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(tg.story, ".ainovel", "rules", "tone.md"))
	if err != nil || string(data) != "story tone" {
		t.Fatalf("file quy tắc chưa được ghi: %v %q", err, data)
	}

	// GET file。
	w = tg.do("GET", "/api/rules/file?scope=story&project=proj&story=01-story&name=tone.md", admin, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "story tone") {
		t.Fatalf("GET rule file -> %d: %s", w.Code, w.Body.String())
	}

	// 非法文件名。
	w = tg.do("GET", "/api/rules/file?scope=story&project=proj&story=01-story&name=../evil.md", admin, "")
	if w.Code != 400 {
		t.Fatalf("tên file thoát đường dẫn phải trả 400, nhận được %d", w.Code)
	}

	// reader 写被拒。
	w = tg.do("PUT", "/api/rules/file", reader, `{"scope":"story","project":"proj","story":"01-story","name":"x.md","text":"x"}`)
	if w.Code != 403 {
		t.Fatalf("reader PUT rule phải trả 403, nhận được %d", w.Code)
	}

	// 删除。
	w = tg.do("POST", "/api/rules/file/delete", admin, `{"scope":"story","project":"proj","story":"01-story","name":"tone.md"}`)
	if w.Code != 200 {
		t.Fatalf("DELETE rule -> %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tg.story, ".ainovel", "rules", "tone.md")); !os.IsNotExist(err) {
		t.Fatal("file quy tắc chưa bị xoá")
	}
}

// /api/system 返回合约固定字段。
func TestSystemEndpoint(t *testing.T) {
	tg := newConfigTestGateway(t)
	w := tg.do("GET", "/api/system", tg.login("reader", "reader-pass"), "")
	if w.Code != 200 {
		t.Fatalf("GET system -> %d", w.Code)
	}
	m := decodeResp(t, w)
	if m["retries"] != float64(7) || m["defaultContextWindow"] != float64(200000) {
		t.Fatalf("thiếu trường system: %v", m)
	}
	if d, _ := m["deadlock"].([]any); len(d) != 2 {
		t.Fatalf("deadlock phải là [3,5]: %v", m["deadlock"])
	}
}

// /api/config/test 用假 OpenAI 端点做真实最小调用。
func TestConfigTestEndpoint(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"fake","choices":[{"message":{"content":"OK"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	body := fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","config":{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","api_key":"sk-fake","base_url":%q}}}}`, srv.URL)
	w := tg.do("POST", "/api/config/test", admin, body)
	if w.Code != 200 {
		t.Fatalf("POST test -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["ok"] != true {
		t.Fatalf("test phải thành công: %v", m)
	}
	if hits == 0 {
		t.Fatal("provider giả không nhận được request")
	}

	// 失败路径：不可达端点 → ok:false 而非 HTTP 错误。
	body = `{"provider":"openai","model":"gpt-4o","config":{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","api_key":"sk-fake","base_url":"http://127.0.0.1:1"}}}}`
	w = tg.do("POST", "/api/config/test", admin, body)
	if w.Code != 200 {
		t.Fatalf("POST test -> %d: %s", w.Code, w.Body.String())
	}
	if m := decodeResp(t, w); m["ok"] != false {
		t.Fatalf("endpoint không truy cập được phải ok=false: %v", m)
	}
}

// /api/config/account + oauth/logout round-trip（credential 存进 temp HOME 的 auth.json）。
func TestConfigAccountAndLogout(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("GET", "/api/config/account?provider=openai-codex", admin, "")
	if w.Code != 200 || decodeResp(t, w)["connected"] != false {
		t.Fatalf("chưa đăng nhập phải connected=false: %d %s", w.Code, w.Body.String())
	}

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(auth.OpenAICodexID, auth.Credential{
		Access: "at", Refresh: "rt", Expires: time.Now().Add(time.Hour).UnixMilli(),
		Email: "u@example.com", Plan: "pro",
	}); err != nil {
		t.Fatal(err)
	}
	w = tg.do("GET", "/api/config/account?provider=openai-codex", admin, "")
	m := decodeResp(t, w)
	if m["connected"] != true || m["email"] != "u@example.com" || m["plan"] != "pro" {
		t.Fatalf("account phải trả về metadata của credential: %v", m)
	}

	w = tg.do("POST", "/api/config/oauth/logout", admin, `{"provider":"openai-codex"}`)
	if w.Code != 200 {
		t.Fatalf("logout -> %d: %s", w.Code, w.Body.String())
	}
	w = tg.do("GET", "/api/config/account?provider=openai-codex", admin, "")
	if decodeResp(t, w)["connected"] != false {
		t.Fatal("sau logout vẫn connected")
	}
}

// ---------- OAuth start/status/cancel（假 OAuth provider + fake server） ----------

// fakeOAuthProvider 实现 auth.Provider：device 流回报 userCode 后立刻发证；
// browser 流回报 url 后阻塞到 ctx（等用户/取消）。
type fakeOAuthProvider struct {
	id string
}

func (p *fakeOAuthProvider) ID() string   { return p.id }
func (p *fakeOAuthProvider) Name() string { return "Fake OAuth" }
func (p *fakeOAuthProvider) Refresh(ctx context.Context, cred auth.Credential) (auth.Credential, error) {
	return cred, nil
}
func (p *fakeOAuthProvider) Login(ctx context.Context, ctrl auth.Controller, opts auth.LoginOptions) (auth.Credential, error) {
	if opts.Device {
		ctrl.OnAuth("https://fake.example/device", "Nhập mã: ABCD-1234")
		ctrl.OnDeviceCode("ABCD-1234", "https://fake.example/device")
		return auth.Credential{Access: "fake-at", Refresh: "fake-rt", Expires: time.Now().Add(time.Hour).UnixMilli(), Email: "dev@example.com"}, nil
	}
	ctrl.OnAuth("https://fake.example/authorize?state=x", "")
	<-ctx.Done()
	return auth.Credential{}, ctx.Err()
}

func registerFakeOAuth(t *testing.T) string {
	t.Helper()
	p := &fakeOAuthProvider{id: "fake-oauth"}
	auth.Register(p)
	return p.id
}

// device 流：start 直接给出 userCode/verifyURL；随后 status 变 done、account 就位。
func TestOAuthDeviceFlow(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	pid := registerFakeOAuth(t)

	w := tg.do("POST", "/api/config/oauth/start", admin, fmt.Sprintf(`{"provider":%q,"device":true}`, pid))
	if w.Code != 200 {
		t.Fatalf("start -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["mode"] != "device" || m["userCode"] != "ABCD-1234" || m["verifyURL"] != "https://fake.example/device" || m["id"] == nil {
		t.Fatalf("phản hồi start thiếu trường: %v", m)
	}
	id := m["id"].(string)

	// auth.Login 在 goroutine 里跑——轮询等待发证（上限 2s）。
	for range 200 {
		w = tg.do("GET", "/api/config/oauth/status?id="+id, admin, "")
		m = decodeResp(t, w)
		if m["state"] != "pending" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m["state"] != "done" {
		t.Fatalf("status phải là done: %v", m)
	}
	account := m["account"].(map[string]any)
	if account["email"] != "dev@example.com" {
		t.Fatalf("account email: %v", account)
	}

	// credential 落进 temp HOME 的 auth.json。
	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	cred, ok, err := store.Get(pid)
	if err != nil || !ok || cred.Access != "fake-at" {
		t.Fatalf("credential chưa được lưu: %v %v %v", cred, ok, err)
	}
}

// browser 流：loopback 允许、非 loopback 400；pending → cancel 后 failed。
func TestOAuthBrowserFlowGuardAndCancel(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	pid := registerFakeOAuth(t)

	// 非 loopback 请求 → 400。
	w := tg.doWith("POST", "/api/config/oauth/start", admin, fmt.Sprintf(`{"provider":%q,"device":false}`, pid), func(r *http.Request) {
		r.RemoteAddr = "203.0.113.7:9999"
	})
	if w.Code != 400 {
		t.Fatalf("browser start không phải loopback phải trả 400, nhận được %d: %s", w.Code, w.Body.String())
	}

	// loopback → 200，mode=browser，url 就位。
	w = tg.doWith("POST", "/api/config/oauth/start", admin, fmt.Sprintf(`{"provider":%q,"device":false}`, pid), func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:5555"
	})
	if w.Code != 200 {
		t.Fatalf("start -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["mode"] != "browser" || m["url"] != "https://fake.example/authorize?state=x" {
		t.Fatalf("browser start: %v", m)
	}
	id := m["id"].(string)

	w = tg.do("GET", "/api/config/oauth/status?id="+id, admin, "")
	if decodeResp(t, w)["state"] != "pending" {
		t.Fatalf("phải là pending: %s", w.Body.String())
	}
	w = tg.do("POST", "/api/config/oauth/cancel", admin, fmt.Sprintf(`{"id":%q}`, id))
	if w.Code != 200 {
		t.Fatalf("cancel -> %d: %s", w.Code, w.Body.String())
	}
	// cancel 先把状态钉成 failed，再断 ctx（竞态下 goroutine 不得覆盖）。
	for range 50 {
		w = tg.do("GET", "/api/config/oauth/status?id="+id, admin, "")
		m = decodeResp(t, w)
		if m["state"] == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m["state"] != "failed" || m["error"] == "" {
		t.Fatalf("sau cancel phải là failed: %v", m)
	}
}

// deviceOnlyOAuthProvider giả lập xai-oauth: chỉ có device-code, từ chối nếu Login không nhận Device.
type deviceOnlyOAuthProvider struct{ fakeOAuthProvider }

func (p *deviceOnlyOAuthProvider) DeviceOnly() bool { return true }
func (p *deviceOnlyOAuthProvider) Login(ctx context.Context, ctrl auth.Controller, opts auth.LoginOptions) (auth.Credential, error) {
	if !opts.Device {
		return auth.Credential{}, fmt.Errorf("device-only provider nhận luồng trình duyệt")
	}
	return p.fakeOAuthProvider.Login(ctx, ctrl, opts)
}

func TestXAIOAuthStartAlwaysUsesDeviceFlowAndListsModels(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	orig, _ := auth.Lookup(xai.ProviderName)
	auth.Register(&deviceOnlyOAuthProvider{fakeOAuthProvider{id: xai.ProviderName}})
	t.Cleanup(func() { auth.Register(orig) })
	var gotBase string
	prev := listXAIModels
	listXAIModels = func(_ context.Context, base string, _ xai.TokenSource) ([]xai.Model, error) {
		gotBase = base
		return []xai.Model{{Slug: "grok-4.6", DisplayName: "Grok 4.6"}}, nil
	}
	t.Cleanup(func() { listXAIModels = prev })

	// Trình duyệt ở máy khác (không loopback) và gửi device:false: xAI vẫn phải chạy device-code.
	w := tg.doWith("POST", "/api/config/oauth/start", admin, `{"provider":"xai-oauth","device":false}`, func(r *http.Request) {
		r.RemoteAddr = "203.0.113.7:9999"
	})
	if w.Code != 200 {
		t.Fatalf("start -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["mode"] != "device" || m["userCode"] != "ABCD-1234" || m["verifyURL"] != "https://fake.example/device" {
		t.Fatalf("start = %v", m)
	}
	id := m["id"].(string)
	for range 200 {
		m = decodeResp(t, tg.do("GET", "/api/config/oauth/status?id="+id, admin, ""))
		if m["state"] != "pending" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m["state"] != "done" || m["error"] != "" {
		t.Fatalf("status = %v", m)
	}
	models := m["models"].([]any)
	if len(models) != 1 || models[0].(map[string]any)["name"] != "grok-4.6" {
		t.Fatalf("status.models = %v", m["models"])
	}
	listed := decodeResp(t, tg.do("GET", "/api/config/models?provider=xai-oauth", admin, ""))
	if got := listed["models"].([]any); len(got) != 1 || gotBase != "" {
		t.Fatalf("models = %v base=%q", listed, gotBase)
	}
	acct := decodeResp(t, tg.do("GET", "/api/config/account?provider=xai-oauth", admin, ""))
	if acct["connected"] != true || acct["email"] != "dev@example.com" {
		t.Fatalf("account = %v", acct)
	}
}

func TestPresetsListXAIOAuthAsDeviceOnlySubscription(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	presets := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))
	var order []string
	found := map[string]map[string]any{}
	for _, item := range presets["providers"].([]any) {
		p := item.(map[string]any)
		order = append(order, p["id"].(string))
		found[p["id"].(string)] = p
	}
	x := found["xai-oauth"]
	if x == nil || x["oauth"] != true || x["deviceOnly"] != true || x["keyRequired"] != false || x["defaultModel"] != "grok-4.6" || x["api"] != "" {
		t.Fatalf("xai-oauth preset = %v", x)
	}
	if found["openai-codex"]["deviceOnly"] != false {
		t.Fatalf("codex có luồng trình duyệt: %v", found["openai-codex"])
	}
	codexAt, xaiAt := slices.Index(order, "openai-codex"), slices.Index(order, "xai-oauth")
	if xaiAt != codexAt+1 {
		t.Fatalf("xai-oauth phải đứng ngay sau openai-codex: %v", order)
	}
}

func TestConnectionFailureForXAIOAuthIsVietnameseAndActionable(t *testing.T) {
	pc := bootstrap.ProviderConfig{}
	msg, _ := connectionFailure("xai-oauth", pc, fmt.Errorf("request failed: %w", auth.NotLoggedInError("xai-oauth")))
	if !strings.Contains(msg, "Chưa đăng nhập") || strings.Contains(msg, "Không kết nối được") {
		t.Fatalf("chưa đăng nhập: %q", msg)
	}
	denied := litellm.NewHTTPError("grok", 403, `{"error":{"code":"xai_oauth_tier_denied","message":"x"}}`)
	if msg, _ := connectionFailure("xai-oauth", pc, denied); msg != auth.XAITierDeniedMessage {
		t.Fatalf("403 tier: %q", msg)
	}
	unauthorized := litellm.NewHTTPError("grok", 401, `{"error":"bad token"}`)
	if msg, _ := connectionFailure("xai-oauth", pc, unauthorized); !strings.Contains(msg, "đăng nhập lại") || strings.Contains(msg, "khoá API") {
		t.Fatalf("401: %q", msg)
	}
}

// pasteOAuthProvider giả lập anthropic-oauth: báo URL rồi chờ mã dán tay (state "st-1").
type pasteOAuthProvider struct{ fakeOAuthProvider }

func (p *pasteOAuthProvider) PasteCode() bool { return true }
func (p *pasteOAuthProvider) Login(ctx context.Context, ctrl auth.Controller, opts auth.LoginOptions) (auth.Credential, error) {
	ctrl.OnAuth("https://claude.example/authorize?state=st-1", "")
	for {
		select {
		case sub := <-opts.Manual:
			code, state := auth.ParseAuthorizationInput(sub.Input)
			if code == "" || state != "st-1" {
				sub.Result <- fmt.Errorf("mã dán sai state")
				continue
			}
			sub.Result <- nil
			return auth.Credential{Access: "at", Refresh: "rt", Expires: time.Now().Add(time.Hour).UnixMilli(), Email: "dev@example.com", OrgName: "Truyện Việt"}, nil
		case <-ctx.Done():
			return auth.Credential{}, ctx.Err()
		}
	}
}

func TestAnthropicOAuthPasteCodeFlowFromRemoteBrowser(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	orig, _ := auth.Lookup(anthropicoauth.ProviderName)
	auth.Register(&pasteOAuthProvider{fakeOAuthProvider{id: anthropicoauth.ProviderName}})
	t.Cleanup(func() { auth.Register(orig) })
	prev := listAnthropicOAuthModels
	listAnthropicOAuthModels = func(context.Context, string, anthropicoauth.TokenSource) ([]anthropicoauth.Model, error) {
		return []anthropicoauth.Model{{Slug: "claude-opus-4-8", DisplayName: "Claude Opus 4.8"}}, nil
	}
	t.Cleanup(func() { listAnthropicOAuthModels = prev })

	// Mở từ máy khác, device:false: không báo lỗi mà chuyển sang chế độ dán mã.
	w := tg.doWith("POST", "/api/config/oauth/start", admin, `{"provider":"anthropic-oauth","device":false}`, func(r *http.Request) {
		r.RemoteAddr = "203.0.113.7:9999"
	})
	if w.Code != 200 {
		t.Fatalf("start -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["mode"] != "paste" || m["url"] != "https://claude.example/authorize?state=st-1" || m["userCode"] != "" {
		t.Fatalf("start = %v", m)
	}
	id := m["id"].(string)

	if w := tg.do("POST", "/api/config/oauth/complete", admin, fmt.Sprintf(`{"id":%q,"code":"   "}`, id)); w.Code != 400 {
		t.Fatalf("mã trống phải 400, nhận %d", w.Code)
	}
	w = tg.do("POST", "/api/config/oauth/complete", admin, fmt.Sprintf(`{"id":%q,"code":"abc#wrong"}`, id))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "sai state") {
		t.Fatalf("mã sai state: %d %s", w.Code, w.Body.String())
	}
	if st := decodeResp(t, tg.do("GET", "/api/config/oauth/status?id="+id, admin, ""))["state"]; st != "pending" {
		t.Fatalf("mã sai không được kết thúc luồng: %v", st)
	}
	w = tg.do("POST", "/api/config/oauth/complete", admin, fmt.Sprintf(`{"id":%q,"code":"abc#st-1"}`, id))
	if w.Code != 200 {
		t.Fatalf("complete -> %d: %s", w.Code, w.Body.String())
	}
	for range 200 {
		m = decodeResp(t, tg.do("GET", "/api/config/oauth/status?id="+id, admin, ""))
		if m["state"] != "pending" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m["state"] != "done" || m["mode"] != "paste" {
		t.Fatalf("status = %v", m)
	}
	acct := m["account"].(map[string]any)
	if acct["label"] != "dev@example.com · Truyện Việt" {
		t.Fatalf("account = %v", acct)
	}
	if models := m["models"].([]any); len(models) != 1 || models[0].(map[string]any)["name"] != "claude-opus-4-8" {
		t.Fatalf("models = %v", m["models"])
	}
	if w := tg.do("POST", "/api/config/oauth/complete", admin, fmt.Sprintf(`{"id":%q,"code":"abc#st-1"}`, id)); w.Code != 409 {
		t.Fatalf("complete sau khi xong phải 409, nhận %d", w.Code)
	}
}

func TestOAuthCompleteRejectsSessionWithoutPasteSupport(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	pid := registerFakeOAuth(t)
	m := decodeResp(t, tg.do("POST", "/api/config/oauth/start", admin, fmt.Sprintf(`{"provider":%q,"device":true}`, pid)))
	id := m["id"].(string)
	for range 200 { // chờ luồng device ghi xong auth.json trước khi TempDir bị dọn
		if decodeResp(t, tg.do("GET", "/api/config/oauth/status?id="+id, admin, ""))["state"] != "pending" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := tg.do("POST", "/api/config/oauth/complete", admin, fmt.Sprintf(`{"id":%q,"code":"x#y"}`, id))
	if w.Code != 400 {
		t.Fatalf("complete cho provider không hỗ trợ dán mã phải 400, nhận %d: %s", w.Code, w.Body.String())
	}
}

func TestPresetsListAnthropicOAuthAsPasteCodeSubscription(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	presets := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))
	found := map[string]map[string]any{}
	for _, item := range presets["providers"].([]any) {
		p := item.(map[string]any)
		found[p["id"].(string)] = p
	}
	c := found["anthropic-oauth"]
	if c == nil || c["oauth"] != true || c["pasteCode"] != true || c["deviceOnly"] != false || c["keyRequired"] != false || c["defaultModel"] != "claude-sonnet-4-6" || c["api"] != "" {
		t.Fatalf("anthropic-oauth preset = %v", c)
	}
	if found["anthropic"]["oauth"] != false || found["anthropic"]["pasteCode"] != false || found["anthropic"]["keyRequired"] != true {
		t.Fatalf("anthropic (API key) phải giữ nguyên: %v", found["anthropic"])
	}
}

func TestConnectionFailureForAnthropicOAuthUsesVietnameseMessages(t *testing.T) {
	pc := bootstrap.ProviderConfig{}
	msg, _ := connectionFailure("anthropic-oauth", pc, fmt.Errorf("request failed: %w", auth.NotLoggedInError("anthropic-oauth")))
	if !strings.Contains(msg, "Chưa đăng nhập") {
		t.Fatalf("chưa đăng nhập: %q", msg)
	}
	limited := litellm.NewHTTPError("anthropic", 429, `{"error":{"code":"anthropic_oauth_rate_limited","message":"Gói Claude Pro/Max đã chạm giới hạn sử dụng"}}`)
	if msg, _ := connectionFailure("anthropic-oauth", pc, limited); !strings.Contains(msg, "chạm giới hạn") {
		t.Fatalf("429: %q", msg)
	}
	forbidden := litellm.NewHTTPError("anthropic", 403, `{"error":{"code":"anthropic_oauth_forbidden","message":"Gói chưa được cho phép"}}`)
	if msg, _ := connectionFailure("anthropic-oauth", pc, forbidden); !strings.Contains(msg, "chưa được cho phép") {
		t.Fatalf("403: %q", msg)
	}
}

// Văn phong tự tạo ở ~/.ainovel/style/styles/*.md phải hiện trong presets.styles
// (thẻ “Văn phong của bạn” ở Tuỳ chỉnh › Cách viết), cùng với 5 văn phong có sẵn.
func TestPresetsStylesIncludeUserStyles(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	list := func() []string {
		presets := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))
		var out []string
		for _, item := range presets["styles"].([]any) {
			out = append(out, item.(string))
		}
		return out
	}
	base := list()
	for _, want := range []string{"default", "romance", "suspense", "fantasy", "wuxia"} {
		if !slices.Contains(base, want) {
			t.Fatalf("thiếu văn phong có sẵn %q: %v", want, base)
		}
	}
	dir := filepath.Join(tg.home, ".ainovel", "style", "styles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gioi-tre.md"), []byte("# Giọng trẻ\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := list(); !slices.Contains(got, "gioi-tre") {
		t.Fatalf("văn phong tự tạo không có trong presets.styles: %v", got)
	}
}

// Gói thể loại (nhúng + ~/.ainovel/style/packs) phải hiện trong presets.styles —
// gói được chọn bằng khoá `style` — kèm metadata ở presets.packs cho màn chọn D2.
func TestPresetsStylesIncludePacks(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	// Gói ở tầng home: người dùng tự thêm gói không cần sửa code.
	packDir := filepath.Join(tg.home, ".ainovel", "style", "packs", "goi-thu")
	if err := os.MkdirAll(filepath.Join(packDir, "luat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "goi.json"),
		[]byte(`{"ten":"Gói thử","so_chuong":9,"nguoi_nghe":"Nữ test","luat":["a.md"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "luat", "a.md"), []byte("luật"), 0o644); err != nil {
		t.Fatal(err)
	}

	presets := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))
	var styles []string
	for _, item := range presets["styles"].([]any) {
		styles = append(styles, item.(string))
	}
	// Gói KHÔNG được trộn vào styles: màn Cách viết hiện mọi mục lạ thành thẻ
	// "Văn phong của bạn" với nhãn là slug thô — gói chỉ lộ qua mảng packs.
	for _, banned := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van", "goi-thu"} {
		if slices.Contains(styles, banned) {
			t.Fatalf("gói %q không được lọt vào presets.styles: %v", banned, styles)
		}
	}
	packs, ok := presets["packs"].([]any)
	if !ok || len(packs) != 4 {
		t.Fatalf("presets.packs phải có 4 gói, nhận được: %v", presets["packs"])
	}
	var homePack map[string]any
	for _, item := range packs {
		p := item.(map[string]any)
		if p["id"] == "goi-thu" {
			homePack = p
		}
	}
	if homePack == nil {
		t.Fatalf("thiếu gói goi-thu trong packs: %v", packs)
	}
	if homePack["ten"] != "Gói thử" || homePack["so_chuong"].(float64) != 9 || homePack["nguoi_nghe"] != "Nữ test" {
		t.Fatalf("metadata gói sai: %v", homePack)
	}
}

// Tuỳ chỉnh › “Theo dự án”: gửi lại tệp cấp truyện không còn khoá `roles.writer` phải XOÁ khoá ở cấp đó
// (PUT thay nguyên tệp của cấp) và `sources` quay về cấp dự án; effective lấy lại giá trị của dự án.
func TestPutConfig_StoryScopeDeleteKeyRestoresInheritance(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	tg.globalConfig(t, `{"provider":"openai","model":"g-model","providers":{"openai":{"type":"openai","api_key":"sk-secret-1234567890","models":[{"name":"g-model"},{"name":"p-model"},{"name":"s-model"}]}}}`)
	if err := os.MkdirAll(filepath.Join(tg.project, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tg.project, ".ainovel", "config.json"), []byte(`{"roles":{"writer":{"provider":"openai","model":"p-model"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	const url = "/api/config?scope=story&project=proj&story=01-story"

	put := tg.do("PUT", url, admin, `{"roles":{"writer":{"provider":"openai","model":"s-model"}}}`)
	if put.Code != 200 {
		t.Fatalf("PUT khoá riêng của truyện -> %d: %s", put.Code, put.Body.String())
	}
	m := decodeResp(t, tg.do("GET", url, admin, ""))
	if got := m["sources"].(map[string]any)["roles.writer.model"]; got != "story" {
		t.Fatalf("sau khi ghi, nguồn phải là story, nhận %v", got)
	}

	// “Theo dự án”: tệp truyện gửi lên không còn roles.writer
	reset := tg.do("PUT", url, admin, `{}`)
	if reset.Code != 200 {
		t.Fatalf("PUT xoá khoá -> %d: %s", reset.Code, reset.Body.String())
	}
	m = decodeResp(t, tg.do("GET", url, admin, ""))
	if roles, ok := m["target"].(map[string]any)["config"].(map[string]any)["roles"]; ok {
		t.Fatalf("khoá roles phải bị xoá khỏi tệp cấp truyện: %v", roles)
	}
	if got := m["sources"].(map[string]any)["roles.writer.model"]; got != "project" {
		t.Fatalf("sau khi xoá, nguồn phải quay về project, nhận %v", got)
	}
	writer := m["effective"].(map[string]any)["roles"].(map[string]any)["writer"].(map[string]any)
	if writer["model"] != "p-model" {
		t.Fatalf("effective phải lấy lại model của dự án: %v", writer)
	}
}
