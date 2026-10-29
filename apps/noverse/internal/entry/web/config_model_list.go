package web

// POST /api/config/models/list：向 provider 自己的列表 API 拉模型清单，
// 让“Kết nối AI”卡片能直接勾选模型，不必手写名字。
// 响应始终 200，错误以 {error, message, detail?} 内嵌返回（与 /api/config/test 同一约定）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/auth"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/models"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// providerModel / providerModelList 是该接口的响应形状（11-api-contract）。
type providerModelPrice struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type providerModel struct {
	Name           string              `json:"name"`
	DisplayName    string              `json:"displayName"`
	Description    string              `json:"description,omitempty"`
	ContextWindow  int                 `json:"contextWindow"`
	ContextSource  string              `json:"contextSource"`
	RegistryWindow int                 `json:"registryWindow"`
	Price          *providerModelPrice `json:"price,omitempty"`
	Recommended    bool                `json:"recommended"`
}

type providerModelList struct {
	Models      []providerModel `json:"models"`
	Total       int             `json:"total"`
	FetchedAt   string          `json:"fetchedAt"`
	Unsupported bool            `json:"unsupported,omitempty"`
	Error       string          `json:"error,omitempty"`
	Message     string          `json:"message,omitempty"`
	Detail      string          `json:"detail,omitempty"`
}

// contextSource 取值：model_config > provider > registry > default。
// 与 bootstrap.ContextWindowSource 并存：那边多了旧的顶层 config 一档，此处不用。
const (
	ctxSourceModelConfig = "model_config"
	ctxSourceProvider    = "provider"
	ctxSourceRegistry    = "registry"
	ctxSourceDefault     = "default"
)

// listHTTPClient 供测试替换为 httptest transport 或直接注入。
var listHTTPClient = &http.Client{Timeout: 20 * time.Second}

// listDefaultBaseURL 是 provider 没填 base_url 时的官方地址（与各 adapter 默认值一致）。
var listDefaultBaseURL = map[string]string{
	"openai":      "https://api.openai.com/v1",
	"anthropic":   "https://api.anthropic.com",
	"gemini":      "https://generativelanguage.googleapis.com",
	"openrouter":  "https://openrouter.ai/api/v1",
	"deepseek":    "https://api.deepseek.com/v1",
	"qwen":        "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"glm":         "https://open.bigmodel.cn/api/paas/v4",
	"grok":        "https://api.x.ai/v1",
	"mimo":        "https://api.xiaomimimo.com/v1",
	"minimax":     "https://api.minimax.io/v1",
	"opencode-go": "https://opencode.ai/inference/go/v1",
	"ollama":      "http://localhost:11434/v1",
}

const modelListCacheTTL = 10 * time.Minute
const modelListMaxBody = 8 << 20

// fetchedModel 是上游返回的一条模型记录（缓存的是它，窗口/推荐在响应时按草稿再解析）。
type fetchedModel struct {
	name        string
	displayName string
	description string
	window      int // provider 报告的窗口；0 = 没报
	price       *providerModelPrice
}

type modelListCacheEntry struct {
	models    []fetchedModel
	fetchedAt time.Time
}

var modelListCache = struct {
	sync.Mutex
	items map[string]modelListCacheEntry
}{items: map[string]modelListCacheEntry{}}

func modelListCacheKey(provider, typ, baseURL, apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return provider + "|" + typ + "|" + baseURL + "|" + hex.EncodeToString(sum[:])
}

func modelListCacheGet(key string) (modelListCacheEntry, bool) {
	modelListCache.Lock()
	defer modelListCache.Unlock()
	e, ok := modelListCache.items[key]
	if !ok || time.Since(e.fetchedAt) > modelListCacheTTL {
		return modelListCacheEntry{}, false
	}
	return e, true
}

func modelListCachePut(key string, e modelListCacheEntry) {
	modelListCache.Lock()
	defer modelListCache.Unlock()
	modelListCache.items[key] = e
}

// listError 是内嵌错误（HTTP 仍 200）。上游失败只分两档 code；文案在 handler 里带上 provider 名。
type listError struct {
	code   string // key_rejected | unreachable
	status int    // 上游 HTTP 状态；0 = 地址/网络失败，200 = 返回体读不懂
	detail string // 上游原文，可选
}

func (e *listError) message(label string) string {
	switch {
	case e.code == "key_rejected":
		return fmt.Sprintf("%s từ chối khoá API này — kiểm tra lại khoá", label)
	case e.status == http.StatusOK:
		return fmt.Sprintf("%s trả danh sách model không đọc được", label)
	case e.status != 0:
		return fmt.Sprintf("%s trả lỗi HTTP %d — thử lại sau", label, e.status)
	default:
		return fmt.Sprintf("Không kết nối được %s — kiểm tra địa chỉ và mạng", label)
	}
}

func (g *Gateway) listProviderModels(w http.ResponseWriter, r *http.Request, user webauth.User) {
	if user.Role != webauth.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Provider string            `json:"provider"`
		Config   *bootstrap.Config `json:"config,omitempty"`
		Refresh  bool              `json:"refresh,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		jsonError(w, 400, "Chưa chọn nhà cung cấp")
		return
	}
	cfg := bootstrap.Config{}
	if req.Config != nil {
		cfg = *req.Config
	} else if loaded, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath()); err == nil {
		cfg = loaded
	}
	if req.Config != nil {
		inheritSavedAPIKey(&cfg, provider)
	}
	pc := cfg.Providers[provider]
	typ, err := pc.ProviderType(provider)
	if err != nil {
		// 未知 provider 名且没写 type：无法判断它讲哪种 API，只能如实说没有列表 API。
		writeJSON(w, 200, providerModelList{Models: []providerModel{}, Unsupported: true})
		return
	}

	// 订阅制（OAuth）沿用 configModels 的 oauthModelList；未登录是预期状态，不算故障。
	if isSubscriptionProvider(typ) || pc.UsesOAuth(provider) {
		g.listOAuthProviderModels(w, r, provider, typ, pc, cfg, req.Refresh)
		return
	}
	if typ == "bedrock" {
		writeJSON(w, 200, providerModelList{Models: []providerModel{}, Unsupported: true})
		return
	}
	if strings.TrimSpace(pc.APIKey) == "" && typ != "ollama" {
		// Ollama 本机服务不需要 khoá；其余 key provider 没 key 就没法拉清单。
		writeJSON(w, 200, providerModelList{Models: []providerModel{}, Error: "missing_key", Message: missingAPIKeyMessage(provider)})
		return
	}
	base := effectiveListBase(typ, pc)
	key := modelListCacheKey(provider, typ, base, pc.APIKey)
	if !req.Refresh {
		if cached, ok := modelListCacheGet(key); ok {
			writeJSON(w, 200, buildModelListResponse(cfg, provider, typ, pc, cached))
			return
		}
	}
	fetched, lerr := fetchProviderModels(r.Context(), typ, base, pc.APIKey)
	if lerr != nil {
		resp := providerModelList{Models: []providerModel{}, Error: lerr.code, Message: lerr.message(providerLabel(provider))}
		if lerr.detail != "" {
			resp.Detail = lerr.detail
		}
		writeJSON(w, 200, resp)
		return
	}
	entry := modelListCacheEntry{models: fetched, fetchedAt: time.Now()}
	modelListCachePut(key, entry)
	writeJSON(w, 200, buildModelListResponse(cfg, provider, typ, pc, entry))
}

// listOAuthProviderModels 复用 oauthModelList（订阅 provider 的既有实现）。
func (g *Gateway) listOAuthProviderModels(w http.ResponseWriter, r *http.Request, provider, typ string, pc bootstrap.ProviderConfig, cfg bootstrap.Config, refresh bool) {
	store, err := auth.DefaultStore()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	key := modelListCacheKey(provider, typ, pc.BaseURL, "")
	if !refresh {
		if cached, ok := modelListCacheGet(key); ok {
			writeJSON(w, 200, buildModelListResponse(cfg, provider, typ, pc, cached))
			return
		}
	}
	infos, err := oauthModelList(r.Context(), typ, pc.BaseURL, store)
	if err != nil {
		resp := providerModelList{Models: []providerModel{}, Detail: err.Error()}
		switch {
		case errors.Is(err, auth.ErrNotLoggedIn):
			resp.Error = "not_logged_in"
			resp.Message = fmt.Sprintf("Chưa đăng nhập %s — đăng nhập tài khoản rồi thử lại", providerLabel(provider))
		default:
			resp.Error = "unreachable"
			resp.Message = fmt.Sprintf("Không lấy được danh sách model của %s — thử lại sau", providerLabel(provider))
		}
		writeJSON(w, 200, resp)
		return
	}
	fetched := make([]fetchedModel, 0, len(infos))
	for _, m := range infos {
		fetched = append(fetched, fetchedModel{name: m.Name, displayName: m.DisplayName, window: m.ContextWindow})
	}
	entry := modelListCacheEntry{models: fetched, fetchedAt: time.Now()}
	modelListCachePut(key, entry)
	writeJSON(w, 200, buildModelListResponse(cfg, provider, typ, pc, entry))
}

// effectiveListBase 选列表请求的地址：配置里的 base_url，否则该类型的官方默认。
func effectiveListBase(typ string, pc bootstrap.ProviderConfig) string {
	if base := strings.TrimSpace(pc.BaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	return strings.TrimRight(listDefaultBaseURL[typ], "/")
}

// fetchProviderModels 按有效类型调各自的列表 API，返回上游原始条目。
func fetchProviderModels(ctx context.Context, typ, base, apiKey string) ([]fetchedModel, *listError) {
	switch typ {
	case "openrouter":
		return fetchOpenRouterModels(ctx, base, apiKey)
	case "anthropic":
		return fetchAnthropicModels(ctx, base, apiKey)
	case "gemini":
		return fetchGeminiModels(ctx, base, apiKey)
	case "ollama":
		return fetchOllamaModels(ctx, base)
	default:
		// openai 系与自定义 openai 代理共用 GET {base}/models Bearer。
		return fetchOpenAIModels(ctx, base, apiKey)
	}
}

// doListGet 发带鉴权的 GET；401/403 → key_rejected，其他 HTTP/网络/解码错 → unreachable。
func doListGet(ctx context.Context, endpoint string, headers map[string]string, query map[string]string, out any) *listError {
	u := endpoint
	if len(query) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		parts := make([]string, 0, len(query))
		for k, v := range query {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		u += sep + strings.Join(parts, "&")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return &listError{code: "unreachable", detail: err.Error()}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := listHTTPClient.Do(req)
	if err != nil {
		return &listError{code: "unreachable", detail: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, modelListMaxBody))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &listError{code: "key_rejected", status: resp.StatusCode, detail: strings.TrimSpace(string(body))}
	}
	if resp.StatusCode != http.StatusOK {
		return &listError{code: "unreachable", status: resp.StatusCode, detail: strings.TrimSpace(string(body))}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &listError{code: "unreachable", status: http.StatusOK, detail: err.Error()}
	}
	return nil
}

func fetchOpenAIModels(ctx context.Context, base, apiKey string) ([]fetchedModel, *listError) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if lerr := doListGet(ctx, base+"/models", map[string]string{"Authorization": "Bearer " + apiKey}, nil, &payload); lerr != nil {
		return nil, lerr
	}
	out := make([]fetchedModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			out = append(out, fetchedModel{name: id})
		}
	}
	return out, nil
}

func fetchOpenRouterModels(ctx context.Context, base, apiKey string) ([]fetchedModel, *listError) {
	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if lerr := doListGet(ctx, base+"/models", map[string]string{"Authorization": "Bearer " + apiKey}, nil, &payload); lerr != nil {
		return nil, lerr
	}
	out := make([]fetchedModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		fm := fetchedModel{name: id, displayName: strings.TrimSpace(m.Name), description: strings.TrimSpace(m.Description), window: m.ContextLength}
		// pricing.prompt/completion 是 USD/token 字符串 → ×1e6 得 USD/1M token。
		if in, err1 := strconv.ParseFloat(m.Pricing.Prompt, 64); err1 == nil {
			if out_, err2 := strconv.ParseFloat(m.Pricing.Completion, 64); err2 == nil {
				fm.price = &providerModelPrice{Input: in * 1e6, Output: out_ * 1e6}
			}
		}
		out = append(out, fm)
	}
	return out, nil
}

func fetchAnthropicModels(ctx context.Context, base, apiKey string) ([]fetchedModel, *listError) {
	base = strings.TrimSuffix(base, "/v1") // 有人会把 endpoint 填成 …/v1
	var out []fetchedModel
	after := ""
	for range 50 { // 分页上限，防恶意/异常端点死循环
		var payload struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		headers := map[string]string{"x-api-key": apiKey, "anthropic-version": "2023-06-01"}
		query := map[string]string{}
		if after != "" {
			query["after_id"] = after
		}
		if lerr := doListGet(ctx, base+"/v1/models", headers, query, &payload); lerr != nil {
			return nil, lerr
		}
		for _, m := range payload.Data {
			if id := strings.TrimSpace(m.ID); id != "" {
				out = append(out, fetchedModel{name: id, displayName: strings.TrimSpace(m.DisplayName)})
			}
		}
		if !payload.HasMore || payload.LastID == "" {
			return out, nil
		}
		after = payload.LastID
	}
	return out, nil
}

func fetchGeminiModels(ctx context.Context, base, apiKey string) ([]fetchedModel, *listError) {
	base = strings.TrimSuffix(base, "/v1beta")
	var out []fetchedModel
	token := ""
	for range 50 {
		var payload struct {
			Models []struct {
				Name                       string   `json:"name"` // "models/gemini-2.5-flash"
				DisplayName                string   `json:"displayName"`
				Description                string   `json:"description"`
				InputTokenLimit            int      `json:"inputTokenLimit"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		query := map[string]string{}
		if token != "" {
			query["pageToken"] = token
		}
		if lerr := doListGet(ctx, base+"/v1beta/models", map[string]string{"x-goog-api-key": apiKey}, query, &payload); lerr != nil {
			return nil, lerr
		}
		for _, m := range payload.Models {
			// 只留能生成内容的模型（embedModels 等排除）。
			gen := false
			for _, method := range m.SupportedGenerationMethods {
				if method == "generateContent" {
					gen = true
					break
				}
			}
			id := strings.TrimPrefix(strings.TrimSpace(m.Name), "models/")
			if !gen || id == "" {
				continue
			}
			out = append(out, fetchedModel{name: id, displayName: strings.TrimSpace(m.DisplayName), description: strings.TrimSpace(m.Description), window: m.InputTokenLimit})
		}
		if payload.NextPageToken == "" {
			return out, nil
		}
		token = payload.NextPageToken
	}
	return out, nil
}

func fetchOllamaModels(ctx context.Context, base string) ([]fetchedModel, *listError) {
	// 用户照 OpenAI 兼容端点填 …/v1；/api/tags 在服务器根。
	host := strings.TrimSuffix(base, "/v1")
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if lerr := doListGet(ctx, host+"/api/tags", nil, nil, &payload); lerr != nil {
		return nil, lerr
	}
	out := make([]fetchedModel, 0, len(payload.Models))
	for _, m := range payload.Models {
		if name := strings.TrimSpace(m.Name); name != "" {
			out = append(out, fetchedModel{name: name})
		}
	}
	return out, nil
}

// buildModelListResponse 把缓存/新拉的原始条目按当前草稿解析成响应：
// 窗口取 model_config > provider > registry > default；recommended 查 Noverse 目录。
func buildModelListResponse(cfg bootstrap.Config, provider, typ string, pc bootstrap.ProviderConfig, entry modelListCacheEntry) providerModelList {
	entries := models.DefaultRegistry().List("")
	defaultModel := defaultModelFor(provider, typ)
	out := make([]providerModel, 0, len(entry.models))
	for _, fm := range entry.models {
		window, source := resolveListWindow(cfg, provider, fm)
		m := providerModel{
			Name:           fm.name,
			DisplayName:    fm.displayName,
			Description:    fm.description,
			ContextWindow:  window,
			ContextSource:  source,
			RegistryWindow: models.DefaultRegistry().ResolveContextWindow(fm.name),
			Price:          fm.price,
		}
		if m.DisplayName == "" {
			m.DisplayName = m.Name
		}
		m.Recommended = registryHasExact(entries, fm.name) || (defaultModel != "" && strings.EqualFold(fm.name, defaultModel))
		out = append(out, m)
	}
	return providerModelList{Models: out, Total: len(out), FetchedAt: entry.fetchedAt.Format(time.RFC3339)}
}

// resolveListWindow 解析清单里某个模型的窗口：草稿里该 model 的 context_window
// 优先，其次 provider 上报值，再查目录，最后 200000 兜底。
func resolveListWindow(cfg bootstrap.Config, provider string, fm fetchedModel) (int, string) {
	if mc, ok := cfg.Providers[provider].ModelConfig(fm.name); ok && mc.ContextWindow > 0 {
		return mc.ContextWindow, ctxSourceModelConfig
	}
	if fm.window > 0 {
		return fm.window, ctxSourceProvider
	}
	if rw := models.DefaultRegistry().ResolveContextWindow(fm.name); rw > 0 {
		return rw, ctxSourceRegistry
	}
	return bootstrap.DefaultContextWindow, ctxSourceDefault
}

// registryHasExact 判断目录有没有这个 id 的精确条目（大小写不敏感，可带 vendor/ 前缀）。
func registryHasExact(registry []models.ModelEntry, id string) bool {
	target := strings.ToLower(strings.TrimSpace(id))
	for _, e := range registry {
		if strings.EqualFold(e.ID, id) || strings.ToLower(e.Provider+"/"+e.ID) == target {
			return true
		}
	}
	return false
}

// cachedListWindow 在 provider 的缓存清单里找模型上报的窗口（testConfig 用）。
func cachedListWindow(provider string, pc bootstrap.ProviderConfig, model string) (int, bool) {
	typ, err := pc.ProviderType(provider)
	if err != nil || typ == "bedrock" {
		return 0, false
	}
	var key string
	if isSubscriptionProvider(typ) || pc.UsesOAuth(provider) {
		key = modelListCacheKey(provider, typ, pc.BaseURL, "")
	} else {
		key = modelListCacheKey(provider, typ, effectiveListBase(typ, pc), pc.APIKey)
	}
	entry, ok := modelListCacheGet(key)
	if !ok {
		return 0, false
	}
	for _, m := range entry.models {
		if m.name == model && m.window > 0 {
			return m.window, true
		}
	}
	return 0, false
}

// resolveTestContextWindow 解析 POST /api/config/test 成功时报告的窗口：
// 草稿 model_config > 已缓存的 provider 列表上报值 > Noverse 目录 > 200000 兜底。
func resolveTestContextWindow(cfg bootstrap.Config, provider, model string) (int, string) {
	pc := cfg.Providers[provider]
	if mc, ok := pc.ModelConfig(model); ok && mc.ContextWindow > 0 {
		return mc.ContextWindow, ctxSourceModelConfig
	}
	if w, ok := cachedListWindow(provider, pc, model); ok {
		return w, ctxSourceProvider
	}
	if rw := models.DefaultRegistry().ResolveContextWindow(model); rw > 0 {
		return rw, ctxSourceRegistry
	}
	return bootstrap.DefaultContextWindow, ctxSourceDefault
}

// ---------- PUT /api/config 409 model_in_use ----------

// roleLabel 把 roles 的 key 翻成界面上的助理名（与 Não 同一张表）；未知名称原样输出。
func roleLabel(role string) string {
	if label := naoRoleLabels[role]; label != "" {
		return label
	}
	return role
}

// orderedRoleKeys 让 409 文案稳定：先三个主角色，其余按字母序。
func orderedRoleKeys(roles map[string]bootstrap.RoleConfig) []string {
	keys := make([]string, 0, len(roles))
	seen := map[string]bool{}
	for _, r := range []string{"architect", "writer", "editor"} {
		if _, ok := roles[r]; ok {
			keys = append(keys, r)
			seen[r] = true
		}
	}
	rest := make([]string, 0, len(roles))
	for r := range roles {
		if !seen[r] {
			rest = append(rest, r)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

// rolesUsingModel 返回新配置里仍引用 (provider, model) 的助理名（去重、定序）。
// 顶层默认叫 "Mặc định"；角色 provider 为空 = 沿用顶层 provider。
func rolesUsingModel(cfg bootstrap.Config, provider, model string) []string {
	var labels []string
	if cfg.Provider == provider && cfg.ModelName == model {
		labels = append(labels, "Mặc định")
	}
	for _, role := range orderedRoleKeys(cfg.Roles) {
		rc := cfg.Roles[role]
		rp := rc.Provider
		if rp == "" {
			rp = cfg.Provider
		}
		label := roleLabel(role)
		if rp == provider && rc.Model == model {
			labels = append(labels, label)
		}
		for _, fb := range rc.Fallbacks {
			fp := fb.Provider
			if fp == "" {
				fp = rp
			}
			if fp == provider && fb.Model == model {
				labels = append(labels, label+" (dự phòng)")
			}
		}
	}
	return labels
}

// modelInUseConflict 找出"旧文件声明过、新文件摘掉（含整个 provider 删除），
// 但新配置仍有助理在用"的第一个 model；返回 409 响应体。
func modelInUseConflict(old, cfg bootstrap.Config) (map[string]any, bool) {
	providers := make([]string, 0, len(old.Providers))
	for name := range old.Providers {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		newPC, ok := cfg.Providers[provider]
		for _, saved := range old.Providers[provider].Models {
			name := strings.TrimSpace(saved.Name)
			if name == "" {
				continue
			}
			if ok {
				if _, still := newPC.ModelConfig(name); still {
					continue // model 仍在列表里，不算摘掉
				}
			}
			if labels := rolesUsingModel(cfg, provider, name); len(labels) > 0 {
				return map[string]any{
					"error":    fmt.Sprintf("Đang dùng cho %s. Đổi model của trợ lý đó trước khi bỏ chọn.", strings.Join(labels, ", ")),
					"code":     "model_in_use",
					"provider": provider,
					"model":    name,
					"roles":    labels,
				}, true
			}
		}
	}
	return nil, false
}
