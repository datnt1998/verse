package web

// POST /api/config/models/list、testConfig 的 contextWindow/contextSource、
// PUT /api/config 409 model_in_use 的测试。全部走 httptest 假端点，无外网。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func resetModelListCache(t *testing.T) {
	t.Helper()
	modelListCache.Lock()
	modelListCache.items = map[string]modelListCacheEntry{}
	modelListCache.Unlock()
}

// listReq 组装 models/list 请求体：draft config 里声明 provider 指向 srv。
func listReq(provider, typ, base, key string) string {
	cfg := fmt.Sprintf(`{"provider":%q,"providers":{%q:{"type":%q,"api_key":%q,"base_url":%q}}}`,
		provider, provider, typ, key, base)
	return fmt.Sprintf(`{"provider":%q,"config":%s}`, provider, cfg)
}

func decodeModelList(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("models/list -> %d: %s", w.Code, w.Body.String())
	}
	return decodeResp(t, w)
}

// OpenAI 系（openai/deepseek/qwen/glm/grok/mimo/minimax/opencode-go + 自定义 openai 代理）：
// GET {base}/models + Bearer。
func TestModelListOpenAIStyle(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var auth string
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"deepseek-chat"},{"id":"my-custom-llm"}]}`)
	}))
	defer srv.Close()

	m := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("deepseek", "deepseek", srv.URL, "sk-ds")))
	if path != "/models" || auth != "Bearer sk-ds" {
		t.Fatalf("request = %s %q", path, auth)
	}
	if m["error"] != nil || m["total"] != float64(2) || m["fetchedAt"] == "" {
		t.Fatalf("response = %v", m)
	}
	models := m["models"].([]any)
	first := models[0].(map[string]any)
	if first["name"] != "deepseek-chat" || first["displayName"] != "deepseek-chat" {
		t.Fatalf("model = %v", first)
	}
	if first["recommended"] != true || first["registryWindow"] != float64(131072) {
		t.Fatalf("registry = %v", first)
	}
	if first["contextSource"] != "registry" || first["contextWindow"] != float64(131072) {
		t.Fatalf("window = %v", first)
	}
	second := models[1].(map[string]any)
	if second["recommended"] != false || second["contextSource"] != "default" || second["contextWindow"] != float64(200000) {
		t.Fatalf("unknown model = %v", second)
	}

	// 自定义代理（任意名字 + type:openai）走同一个适配器。
	resetModelListCache(t)
	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("myproxy", "openai", srv.URL, "k")))
	if m["total"] != float64(2) {
		t.Fatalf("custom proxy = %v", m)
	}
}

// OpenRouter：context_length、pricing USD/token → USD/1M、name/description、recommended。
func TestModelListOpenRouter(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"id":"qwen/qwen3-max","name":"Qwen3 Max","description":"Flagship","context_length":262144,"pricing":{"prompt":"0.0000012","completion":"0.000006"}},
			{"id":"tiny/fast-model","name":"Tiny","context_length":8192,"pricing":{"prompt":"0","completion":"0"}}
		]}`)
	}))
	defer srv.Close()

	w := tg.do("POST", "/api/config/models/list", admin, listReq("openrouter", "openrouter", srv.URL, "sk-or"))
	t.Logf("openrouter response: %s", w.Body.String())
	m := decodeModelList(t, w)
	models := m["models"].([]any)
	qwen := models[0].(map[string]any)
	if qwen["contextWindow"] != float64(262144) || qwen["contextSource"] != "provider" {
		t.Fatalf("qwen window = %v", qwen)
	}
	price := qwen["price"].(map[string]any)
	if price["input"] != 1.2 || price["output"] != 6.0 {
		t.Fatalf("price = %v", price)
	}
	if qwen["displayName"] != "Qwen3 Max" || qwen["description"] != "Flagship" {
		t.Fatalf("qwen meta = %v", qwen)
	}
	if qwen["recommended"] != true {
		t.Fatalf("qwen/qwen3-max phải recommended: %v", qwen)
	}
	tiny := models[1].(map[string]any)
	if tiny["recommended"] != false || tiny["registryWindow"] != float64(0) {
		t.Fatalf("tiny = %v", tiny)
	}
	// 草稿里该 model 已声明 context_window → model_config 优先于 provider 上报。
	resetModelListCache(t)
	draft := fmt.Sprintf(`{"provider":"openrouter","config":{"providers":{"openrouter":{"type":"openrouter","api_key":"sk-or","base_url":%q,"models":[{"name":"qwen/qwen3-max","context_window":99999}]}}}}`, srv.URL)
	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, draft))
	first := m["models"].([]any)[0].(map[string]any)
	if first["contextWindow"] != float64(99999) || first["contextSource"] != "model_config" {
		t.Fatalf("model_config phải thắng: %v", first)
	}
}

// Anthropic：x-api-key + anthropic-version 头，has_more/last_id 分页。
func TestModelListAnthropic(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant" || r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			fmt.Fprint(w, `{"data":[{"id":"claude-sonnet-4.5","display_name":"Claude Sonnet 4.5"}],"has_more":true,"last_id":"claude-sonnet-4.5"}`)
		default:
			if r.URL.Query().Get("after_id") != "claude-sonnet-4.5" {
				t.Errorf("after_id = %q", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":[{"id":"claude-haiku-4.5","display_name":"Claude Haiku 4.5"}],"has_more":false,"last_id":"claude-haiku-4.5"}`)
		}
	}))
	defer srv.Close()

	// base_url 填成 …/v1 也要归一。
	m := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("anthropic", "anthropic", srv.URL+"/v1", "sk-ant")))
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("pagination calls = %d", calls)
	}
	models := m["models"].([]any)
	if len(models) != 2 || models[1].(map[string]any)["name"] != "claude-haiku-4.5" {
		t.Fatalf("models = %v", models)
	}
	if models[0].(map[string]any)["displayName"] != "Claude Sonnet 4.5" {
		t.Fatalf("display = %v", models[0])
	}
}

// Gemini：x-goog-api-key、剥 models/ 前缀、只留 generateContent、nextPageToken 分页。
func TestModelListGemini(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "g-key" {
			t.Errorf("x-goog-api-key = %q", r.Header.Get("x-goog-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			fmt.Fprint(w, `{"models":[
				{"name":"models/gemini-2.5-flash","displayName":"Gemini 2.5 Flash","inputTokenLimit":1048576,"supportedGenerationMethods":["generateContent"]},
				{"name":"models/text-embedding-004","displayName":"Embed","supportedGenerationMethods":["embedContent"]}
			],"nextPageToken":"p2"}`)
		default:
			if r.URL.Query().Get("pageToken") != "p2" {
				t.Errorf("pageToken = %q", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"models":[{"name":"models/gemini-2.5-pro","inputTokenLimit":1048576,"supportedGenerationMethods":["generateContent"]}]}`)
		}
	}))
	defer srv.Close()

	m := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("gemini", "gemini", srv.URL+"/v1beta", "g-key")))
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("pagination calls = %d", calls)
	}
	models := m["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("embed model phải bị lọc: %v", models)
	}
	flash := models[0].(map[string]any)
	if flash["name"] != "gemini-2.5-flash" || flash["contextWindow"] != float64(1048576) || flash["contextSource"] != "provider" {
		t.Fatalf("flash = %v", flash)
	}
}

// Ollama：base_url 的 /v1 剥掉后打 /api/tags，不需要 khoá。
func TestModelListOllama(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"models":[{"name":"qwen2.5:14b"},{"name":"llama3.1:8b"}]}`)
	}))
	defer srv.Close()

	m := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("ollama", "ollama", srv.URL+"/v1", "")))
	if path != "/api/tags" {
		t.Fatalf("path = %s", path)
	}
	if m["total"] != float64(2) {
		t.Fatalf("response = %v", m)
	}
	// Ollama 的默认模型 qwen2.5:14b 是预设 defaultModel → recommended。
	if m["models"].([]any)[0].(map[string]any)["recommended"] != true {
		t.Fatalf("ollama default phải recommended: %v", m)
	}
}

// 错误面：401/403 → key_rejected；网络 → unreachable；没 key → missing_key；
// bedrock → unsupported；订阅未登录 → not_logged_in；非 admin → 403。
func TestModelListErrors(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")

	w := tg.do("POST", "/api/config/models/list", writer, `{"provider":"openai"}`)
	if w.Code != 403 {
		t.Fatalf("non-admin -> %d", w.Code)
	}

	resetModelListCache(t)
	reject := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"message":"invalid key"}}`)
	}))
	defer reject.Close()
	m := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("openai", "openai", reject.URL, "sk-bad")))
	if m["error"] != "key_rejected" || !strings.Contains(m["message"].(string), "OpenAI") || m["models"] == nil {
		t.Fatalf("401 = %v", m)
	}
	if len(m["models"].([]any)) != 0 {
		t.Fatalf("error phải trả models rỗng: %v", m)
	}

	resetModelListCache(t)
	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("openai", "openai", "http://127.0.0.1:1", "sk-x")))
	if m["error"] != "unreachable" || !strings.Contains(m["message"].(string), "OpenAI") {
		t.Fatalf("refused = %v", m)
	}

	// 缺 key：openai 这类 key provider 直接 missing_key，不发请求。
	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, listReq("openai", "openai", "http://127.0.0.1:1", "")))
	if m["error"] != "missing_key" || !strings.Contains(m["message"].(string), "OpenAI") {
		t.Fatalf("missing key = %v", m)
	}

	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, `{"provider":"bedrock","config":{"providers":{"bedrock":{"type":"bedrock"}}}}`))
	if m["unsupported"] != true || m["error"] != nil {
		t.Fatalf("bedrock = %v", m)
	}

	m = decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, `{"provider":"openai-codex"}`))
	if m["error"] != "not_logged_in" {
		t.Fatalf("codex chưa đăng nhập = %v", m)
	}
}

// 缓存：10 分钟内同 key 不重复打上游、fetchedAt 保持原值；refresh 绕过。
func TestModelListCache(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"m1"}]}`)
	}))
	defer srv.Close()

	body := listReq("openai", "openai", srv.URL, "sk-a")
	first := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, body))
	second := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, body))
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("cache hit không được gọi lại upstream: %d", calls)
	}
	if first["fetchedAt"] != second["fetchedAt"] {
		t.Fatalf("cache hit phải giữ fetchedAt: %v vs %v", first["fetchedAt"], second["fetchedAt"])
	}

	refreshed := decodeModelList(t, tg.do("POST", "/api/config/models/list", admin, strings.TrimSuffix(body, "}")+`,"refresh":true}`))
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("refresh phải gọi lại upstream: %d", calls)
	}
	_ = refreshed
}

// 草稿 khoá 与已存 khoá 分开缓存；草稿带新 khoá 时用它而不是存的。
func TestModelListDraftKey(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"m1"}]}`)
	}))
	defer srv.Close()
	tg.globalConfig(t, fmt.Sprintf(`{"provider":"openai","providers":{"openai":{"type":"openai","api_key":"sk-saved","base_url":%q}}}`, srv.URL))

	// 草稿带新 khoá → 用草稿 khoá。
	tg.do("POST", "/api/config/models/list", admin, listReq("openai", "openai", srv.URL, "sk-draft"))
	// 草稿不带 khoá → 继承已存 khoá（与 /api/config/test、PUT 同一约定）。
	draftNoKey := fmt.Sprintf(`{"provider":"openai","config":{"providers":{"openai":{"type":"openai","base_url":%q}}}}`, srv.URL)
	tg.do("POST", "/api/config/models/list", admin, draftNoKey)
	if len(keys) != 2 || keys[0] != "Bearer sk-draft" || keys[1] != "Bearer sk-saved" {
		t.Fatalf("khoá dùng sai: %v", keys)
	}
}

// /api/config/test ok:true 现在带 contextWindow/contextSource。
func TestConfigTestContextWindow(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	resetModelListCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"fake","choices":[{"message":{"content":"OK"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	post := func(cfg string) map[string]any {
		body := fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","config":%s}`, cfg)
		w := tg.do("POST", "/api/config/test", admin, body)
		m := decodeResp(t, w)
		if m["ok"] != true {
			t.Fatalf("test phải ok: %v", m)
		}
		return m
	}

	// registry：gpt-4o 在目录模糊命中 128000。
	base := fmt.Sprintf(`{"providers":{"openai":{"type":"openai","api_key":"k","base_url":%q}}}`, srv.URL)
	m := post(base)
	if m["contextWindow"] != float64(128000) || m["contextSource"] != "registry" {
		t.Fatalf("registry = %v", m)
	}

	// model_config：草稿声明的窗口优先。
	draft := fmt.Sprintf(`{"providers":{"openai":{"type":"openai","api_key":"k","base_url":%q,"models":[{"name":"gpt-4o","context_window":99000}]}}}`, srv.URL)
	m = post(draft)
	if m["contextWindow"] != float64(99000) || m["contextSource"] != "model_config" {
		t.Fatalf("model_config = %v", m)
	}

	// provider：缓存清单上报值（models/list 拉过一次）。
	modelListCachePut(modelListCacheKey("openai", "openai", srv.URL, "k"), modelListCacheEntry{
		models:    []fetchedModel{{name: "gpt-4o", window: 64000}},
		fetchedAt: time.Now(),
	})
	m = post(base)
	if m["contextWindow"] != float64(64000) || m["contextSource"] != "provider" {
		t.Fatalf("provider cache = %v", m)
	}
}

// PUT /api/config global：摘掉还在被助理用的 model → 409 model_in_use。
func TestPutConfigModelInUse(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	saved := `{
		"provider": "openrouter",
		"model": "qwen/qwen3-max",
		"providers": {
			"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "qwen/qwen3-max"}, {"name": "deepseek/deepseek-chat"}]}
		},
		"roles": {
			"writer": {"provider": "openrouter", "model": "deepseek/deepseek-chat", "fallbacks": [{"provider": "openrouter", "model": "qwen/qwen3-max"}]}
		}
	}`
	tg.globalConfig(t, saved)
	put := func(body string) *httptest.ResponseRecorder {
		return tg.do("PUT", "/api/config?scope=global", admin, body)
	}

	// 摘掉 writer 正在用的 model → 409，roles 指名 "Người viết"。
	dropWriter := `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "qwen/qwen3-max"}]}},
		"roles": {"writer": {"provider": "openrouter", "model": "deepseek/deepseek-chat", "fallbacks": [{"provider": "openrouter", "model": "qwen/qwen3-max"}]}}
	}`
	w := put(dropWriter)
	t.Logf("409 body: %s", w.Body.String())
	if w.Code != 409 {
		t.Fatalf("bỏ model đang dùng -> %d: %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["code"] != "model_in_use" || m["model"] != "deepseek/deepseek-chat" || m["provider"] != "openrouter" {
		t.Fatalf("409 = %v", m)
	}
	roles := m["roles"].([]any)
	if len(roles) != 1 || roles[0] != "Người viết" {
		t.Fatalf("roles = %v", roles)
	}
	if msg := m["error"].(string); !strings.Contains(msg, "Người viết") {
		t.Fatalf("message = %q", msg)
	}
	// 409 不能写盘：文件保持原样。
	if got, _ := readFile(tg.home + "/.ainovel/config.json"); !strings.Contains(got, "deepseek/deepseek-chat") {
		t.Fatalf("409 không được ghi: %s", got)
	}

	// 摘掉顶层默认 model → 409 "Mặc định"（writer fallback 也引用，两个名都进文案）。
	dropDefault := `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "deepseek/deepseek-chat"}]}},
		"roles": {"writer": {"provider": "openrouter", "model": "deepseek/deepseek-chat", "fallbacks": [{"provider": "openrouter", "model": "qwen/qwen3-max"}]}}
	}`
	w = put(dropDefault)
	m = decodeResp(t, w)
	if w.Code != 409 || m["model"] != "qwen/qwen3-max" {
		t.Fatalf("drop default = %d %v", w.Code, m)
	}
	if msg := m["error"].(string); !strings.Contains(msg, "Mặc định") || !strings.Contains(msg, "dự phòng") {
		t.Fatalf("labels = %q", msg)
	}

	// 摘掉没人用的 model → 成功。
	dropUnused := `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "qwen/qwen3-max"}]}},
		"roles": {"writer": {"provider": "openrouter", "model": "qwen/qwen3-max"}}
	}`
	if w := put(dropUnused); w.Code != 200 {
		t.Fatalf("bỏ model không dùng -> %d: %s", w.Code, w.Body.String())
	}

	// 加 model → 成功。
	addModel := `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "qwen/qwen3-max"}, {"name": "google/gemini-2.5-flash"}]}},
		"roles": {"writer": {"provider": "openrouter", "model": "qwen/qwen3-max"}}
	}`
	if w := put(addModel); w.Code != 200 {
		t.Fatalf("thêm model -> %d: %s", w.Code, w.Body.String())
	}
}

// 旧配置 role 引用了一个从未进 models 列表的 model（legacy）：保存不应被 409 卡。
func TestPutConfigLegacyUnlistedModelNotBlocked(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	tg.globalConfig(t, `{
		"provider": "openai", "model": "gpt-4o",
		"providers": {"openai": {"type": "openai", "api_key": "sk-x", "models": []}},
		"roles": {"writer": {"provider": "openai", "model": "gpt-4o"}}
	}`)
	w := tg.do("PUT", "/api/config?scope=global", admin, `{
		"provider": "openai", "model": "gpt-4o",
		"providers": {"openai": {"type": "openai", "api_key": "sk-x", "models": [{"name": "gpt-4o-mini"}]}},
		"roles": {"writer": {"provider": "openai", "model": "gpt-4o"}}
	}`)
	if w.Code != 200 {
		t.Fatalf("legacy role model không chặn -> %d: %s", w.Code, w.Body.String())
	}
}

// 删整个 provider 而助理还在用 → 409。
func TestPutConfigDropProviderInUse(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	tg.globalConfig(t, `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {"openrouter": {"type": "openrouter", "api_key": "sk-or", "models": [{"name": "qwen/qwen3-max"}]}}
	}`)
	w := tg.do("PUT", "/api/config?scope=global", admin, `{
		"provider": "openrouter", "model": "qwen/qwen3-max",
		"providers": {}
	}`)
	m := decodeResp(t, w)
	if w.Code != 409 || m["code"] != "model_in_use" || m["model"] != "qwen/qwen3-max" {
		t.Fatalf("drop provider = %d %v", w.Code, m)
	}
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}
