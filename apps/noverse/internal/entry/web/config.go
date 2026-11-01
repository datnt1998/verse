package web

// serveConfig 实现设计合约 §6：/api/config*、/api/rules*、/api/system。
// 继承链：~/.ainovel → <项目>/.ainovel → <故事>/.ainovel（GAP-10）；
// api_key 永不离开服务器（只回 api_key_set / api_key_hint）。

import (
	llmtypes "github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmconfig"

	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/auth/anthropicoauth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	litellm "github.com/datnt1998/verse/core/llm/wire"
	"github.com/datnt1998/verse/core/models"
	"github.com/datnt1998/verse/platform/identity"
)

// serveConfig 处理配置与规则接口；返回 false 表示路径不归它管。
func (g *Gateway) serveConfig(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	path, get, post, put := r.URL.Path, r.Method == http.MethodGet, r.Method == http.MethodPost, r.Method == http.MethodPut
	if strings.HasPrefix(path, "/api/rules") && r.URL.Query().Get("scope") == "global" {
		jsonError(w, http.StatusBadRequest, retiredGlobalRulesMessage)
		return true
	}
	switch {
	case path == "/api/system" && get:
		g.serveSystem(w, r, user)
	case path == "/api/system/pick-folder" && post:
		g.servePickFolder(w, r, user)
	case path == "/api/config" && get:
		g.getConfig(w, r, user)
	case path == "/api/config" && put:
		g.putConfig(w, r, user)
	case path == "/api/config/presets" && get:
		g.servePresets(w, r)
	case path == "/api/config/test" && post:
		g.testConfig(w, r, user)
	case path == "/api/config/models/list" && post:
		g.listProviderModels(w, r, user)
	case path == "/api/config/models" && get:
		g.configModels(w, r, user)
	case path == "/api/config/account" && get:
		g.configAccount(w, r, user)
	case path == "/api/config/oauth/start" && post:
		g.oauthStart(w, r, user)
	case path == "/api/config/oauth/status" && get:
		g.oauthStatus(w, r, user)
	case path == "/api/config/oauth/cancel" && post:
		g.oauthCancel(w, r, user)
	case path == "/api/config/oauth/complete" && post:
		g.oauthComplete(w, r, user)
	case path == "/api/config/oauth/logout" && post:
		g.oauthLogout(w, r, user)
	case path == "/api/rules" && get:
		g.getRules(w, r, user)
	case path == "/api/rules/effective" && get:
		g.serveRulesEffectiveScoped(w, r, user)
	case path == "/api/rules/understand" && post:
		g.serveRulesUnderstand(w, r, user)
	case path == "/api/rules/file" && get:
		g.getRuleFile(w, r, user)
	case path == "/api/rules/file" && put:
		g.putRuleFile(w, r, user)
	case path == "/api/rules/file/delete" && post:
		g.deleteRuleFile(w, r, user)
	case strings.HasPrefix(path, "/api/config") || strings.HasPrefix(path, "/api/rules") || path == "/api/system":
		jsonError(w, 404, "không tìm thấy")
	default:
		return false
	}
	return true
}

// ---------- 作用域解析 ----------

// configTarget 描述一次 scope 解析的结果。
type configTarget struct {
	scope   string // global | project | story
	project string // 项目 id（global 层为空）
	dir     string // 该层文件所在目录（global 时为 ""）
	path    string // 目标 config.json 绝对路径
	story   library.Story
	chainTo string // 传给 LoadConfigChain 的目录（""=只取全局层）
	hosted  bool   // repository document; never build a path from an empty Dir
}

func configFileIn(dir string) string {
	return filepath.Join(dir, ".ainovel", "config.json")
}

func rulesDirIn(dir string) string { return filepath.Join(dir, ".ainovel", "rules") }

const retiredGlobalRulesMessage = "Quy tắc viết chỉ áp dụng cho dự án hoặc truyện, không còn cấp toàn cục"

func (g *Gateway) resolveRulesScope(scope, project, story string) (configTarget, error) {
	if scope == "" {
		switch {
		case story != "":
			scope = "story"
		case project != "":
			scope = "project"
		default:
			scope = "global"
		}
	}
	if scope == "global" {
		return configTarget{}, errors.New(retiredGlobalRulesMessage)
	}
	return g.resolveConfigScope(scope, project, story)
}

// resolveConfigScope 把 scope/project/story 参数解析为磁盘位置。
// project/story 是书库内 id（story scope 需要两者）。
func (g *Gateway) resolveConfigScope(scope, project, story string) (configTarget, error) {
	if scope == "" {
		scope = "global"
	}
	t := configTarget{scope: scope, project: project}

	switch scope {
	case "global":
		t.path = bootstrap.DefaultConfigPath()
		return t, nil
	case "project", "story":
	default:
		return t, fmt.Errorf("scope không hợp lệ: %s", scope)
	}

	lib, err := g.lib(context.Background(), nil)
	if err != nil {
		return t, err
	}
	if project == "" {
		return t, errors.New("thiếu tham số project")
	}
	if project == library.LegacyProjectID {
		// 匿名项目没有自己的 .ainovel（根目录的 .ainovel 是 library.json 的家），
		// 其配置层在运行链上也不存在——直接拒绝，比建一个永不生效的文件诚实。
		return t, errors.New("dự án ẩn danh (_) không có cấu hình riêng")
	}
	ref, ok := lib.ResolveProject(context.Background(), nil, project)
	pdir := ref.Dir
	if !ok {
		return t, library.ErrNotFound
	}
	t.hosted = pdir == ""
	if !t.hosted {
		if st, err := os.Stat(filepath.Join(pdir, library.ProjectFileName)); err != nil || st.IsDir() {
			return t, library.ErrNotFound
		}
	}
	if scope == "project" {
		if !t.hosted {
			t.dir, t.path, t.chainTo = pdir, configFileIn(pdir), pdir
		}
		return t, nil
	}
	if story == "" {
		return t, errors.New("thiếu tham số story")
	}
	st, ok := lib.ResolveStory(context.Background(), nil, project, story)
	if !ok {
		return t, library.ErrNotFound
	}
	t.story = st
	if !t.hosted {
		t.dir, t.path, t.chainTo = st.Dir, configFileIn(st.Dir), st.Dir
	}
	return t, nil
}

// checkConfigWrite 校验目标层的写入权限（合约 §6：global=admin；project 需可管理、
// story 需可写）。故事级目标一律 ErrNotFound——与故事不存在时相同，不暴露存在性；
// 项目级仍 ErrForbidden——可读项目的不可管理者不隐藏（与 POST /:p/stories 的 403 一致）。
func checkConfigWrite(user identity.User, t configTarget) error {
	switch t.scope {
	case "global":
		if user.Role != identity.RoleAdmin {
			return identity.ErrForbidden
		}
	case "project":
		if !user.CanManageProject(t.project) {
			return identity.ErrForbidden
		}
	case "story":
		if !user.CanWrite(t.story.ID) {
			return library.ErrNotFound
		}
	}
	return nil
}

// configReadAllowed 校验目标层的读取权限：global 一律可读；project 需可列、
// story 需可读。不满足时调用方返回 404（见 scopeError）。
func configReadAllowed(user identity.User, t configTarget) bool {
	switch t.scope {
	case "project":
		return user.CanReadProject(t.project)
	case "story":
		return user.CanRead(t.story.ID)
	}
	return true
}

func scopeError(w http.ResponseWriter, err error) {
	if status, msg, ok := mapHTTPError(err); ok {
		jsonError(w, status, msg)
		return
	}
	if errors.Is(err, errNoLibrary) {
		jsonError(w, 409, err.Error())
		return
	}
	// Remaining failures are config validation (curated Vietnamese text);
	// unexpected causes still get the generic 500 from jsonError.
	jsonError(w, 400, err.Error())
}

// ---------- GET/PUT /api/config ----------

// configDoc 是合约 ConfigDoc：单层的 path/exists/config。
type configDoc struct {
	Scope  string         `json:"scope"`
	Path   string         `json:"path"`
	Exists bool           `json:"exists"`
	Config map[string]any `json:"config"`
}

func displayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "~" + string(filepath.Separator) + rel
		}
	}
	return path
}

func (g *Gateway) configDisplayPath(path, scope string, admin bool) string {
	if admin || scope == "global" {
		return displayPath(path)
	}
	return g.responsePath(path, false)
}

// sanitizeConfigMap 把 api_key 替换为 api_key_set/api_key_hint；
// 非管理员进一步裁剪 providers 到只剩 type/models（合约 §6）。
// 注意：会修改传入的 map——调用方传入的是刚解析/刚拷贝的对象。
func sanitizeConfigMap(m map[string]any, admin bool) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	providers, _ := m["providers"].(map[string]any)
	for name, pv := range providers {
		p, _ := pv.(map[string]any)
		if p == nil {
			continue
		}
		key, _ := p["api_key"].(string)
		if admin {
			delete(p, "api_key")
			p["api_key_set"] = key != ""
			if key != "" {
				p["api_key_hint"] = apiKeyHint(key)
			}
			continue
		}
		entry := map[string]any{}
		if typ, ok := p["type"]; ok {
			entry["type"] = typ
		}
		if models, ok := p["models"]; ok {
			entry["models"] = models
		}
		providers[name] = entry
	}
	// tts.api_key 与 providers.api_key 同样只暴露派生字段。
	if tts, ok := m["tts"].(map[string]any); ok {
		key, _ := tts["api_key"].(string)
		delete(tts, "api_key")
		if admin {
			tts["api_key_set"] = key != ""
			if key != "" {
				tts["api_key_hint"] = apiKeyHint(key)
			}
		}
	}
	return m
}

func apiKeyHint(key string) string {
	if len(key) <= 8 {
		return "…"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

// configSources 计算每个键路径（"roles.writer.model"）最后由哪一层声明。
// 叶子边界与 MergeConfig 的覆盖语义一致：models/extra_body/extra/fallbacks/budget/notify
// 整段替换 → 记为叶子；其余对象继续下钻。
func configSources(levels []bootstrap.ConfigLevel) map[string]string {
	sources := map[string]string{}
	whole := map[string]bool{
		"models": true, "extra_body": true, "extra": true, "fallbacks": true,
		"budget": true, "notify": true,
	}
	var walk func(prefix string, m map[string]any, scope string)
	walk = func(prefix string, m map[string]any, scope string) {
		for k, v := range m {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if child, ok := v.(map[string]any); ok && !whole[k] {
				walk(path, child, scope)
				continue
			}
			// 空串叶子视为"未声明"（与 MergeConfig 非零覆盖语义一致；写回文件里
			// provider:"" 这类字段不能误占来源归因）。
			if s, ok := v.(string); ok && s == "" {
				continue
			}
			sources[path] = scope
		}
	}
	for _, lvl := range levels {
		if !lvl.Exists {
			continue
		}
		raw, err := bootstrap.ConfigLevelJSON(lvl)
		if err != nil || raw == nil {
			continue
		}
		if lvl.Scope == "global" {
			delete(raw, "style")
		}
		walk("", raw, lvl.Scope)
	}
	return sources
}

// chainDocs 把链层转成 ConfigDoc（scope 过滤由 keepScopes 控制）。
func (g *Gateway) chainDocs(levels []bootstrap.ConfigLevel, keepScopes map[string]bool, admin bool) ([]configDoc, error) {
	docs := []configDoc{}
	for _, lvl := range levels {
		if !keepScopes[lvl.Scope] {
			continue
		}
		doc := configDoc{Scope: lvl.Scope, Path: g.configDisplayPath(lvl.Path, lvl.Scope, admin), Exists: lvl.Exists}
		if lvl.Exists {
			raw, err := bootstrap.ConfigLevelJSON(lvl)
			if err != nil {
				return nil, err
			}
			if lvl.Scope == "global" {
				delete(raw, "style")
			}
			doc.Config = sanitizeConfigMap(raw, admin)
		} else {
			doc.Config = map[string]any{}
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// buildConfigResponse 组装 ConfigResponse：target 文档、合并后的 effective、链、键来源，
// 以及 G7 的已连接 OAuth 账户（仅 admin 可见）。
func (g *Gateway) buildConfigResponse(ctx context.Context, t configTarget, user identity.User) (map[string]any, error) {
	admin := isAdmin(user)
	var levels []bootstrap.ConfigLevel
	var effective bootstrap.Config
	if t.hosted {
		docs, err := g.hostedConfigDocuments()
		if err != nil {
			return nil, err
		}
		var loadErr error
		levels, effective, loadErr = bootstrap.LoadHostedConfigChain(ctx, docs, configActor(user), t.project, t.story.Story)
		if loadErr != nil {
			return nil, loadErr
		}
	} else if t.chainTo == "" {
		path := bootstrap.DefaultConfigPath()
		lvl := bootstrap.ConfigLevel{Scope: "global", Path: path}
		if raw, err := bootstrap.ReadConfigJSON(path); err != nil {
			return nil, err
		} else if raw != nil {
			cfg, err := bootstrap.LoadConfigFile(path)
			if err != nil {
				return nil, err
			}
			lvl.Exists, lvl.Config, effective = true, cfg, cfg
		}
		levels = []bootstrap.ConfigLevel{lvl}
	} else {
		var err error
		levels, effective, err = bootstrap.LoadConfigChain(t.chainTo)
		if err != nil {
			return nil, err
		}
	}
	if t.scope == "global" {
		effective.Style = ""
	}

	keep := map[string]bool{"global": true, "project": t.scope != "global", "story": t.scope == "story"}
	docs, err := g.chainDocs(levels, keep, admin)
	if err != nil {
		return nil, err
	}

	var target *configDoc
	for i := range docs {
		if docs[i].Scope == t.scope {
			target = &docs[i]
		}
	}
	if target == nil {
		raw, err := bootstrap.ReadConfigJSON(t.path)
		if err != nil {
			return nil, err
		}
		target = &configDoc{Scope: t.scope, Path: g.configDisplayPath(t.path, t.scope, admin), Exists: raw != nil, Config: sanitizeConfigMap(raw, admin)}
	}

	effRaw, err := configToMap(effective)
	if err != nil {
		return nil, err
	}
	if t.scope == "global" {
		delete(effRaw, "style")
	}
	providerErrors := map[string]string{}
	for name, pc := range effective.Providers {
		if err := bootstrap.ValidateProvider(name, pc); err != nil {
			providerErrors[name] = err.Error()
		}
	}
	return map[string]any{
		"target":         target,
		"effective":      sanitizeConfigMap(effRaw, admin),
		"chain":          docs,
		"sources":        configSources(levels),
		"accounts":       g.configAccounts(admin),
		"providerErrors": providerErrors,
	}, nil
}

func configToMap(cfg bootstrap.Config) (map[string]any, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (g *Gateway) getConfig(w http.ResponseWriter, r *http.Request, user identity.User) {
	q := r.URL.Query()
	t, err := g.resolveConfigScope(q.Get("scope"), q.Get("project"), q.Get("story"))
	if err != nil {
		scopeError(w, err)
		return
	}
	if !configReadAllowed(user, t) {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	resp, err := g.buildConfigResponse(r.Context(), t, user)
	if err != nil {
		scopeError(w, err)
		return
	}
	writeJSON(w, 200, resp)
}

func (g *Gateway) putConfig(w http.ResponseWriter, r *http.Request, user identity.User) {
	q := r.URL.Query()
	t, err := g.resolveConfigScope(q.Get("scope"), q.Get("project"), q.Get("story"))
	if err != nil {
		scopeError(w, err)
		return
	}
	if err := checkConfigWrite(user, t); err != nil {
		scopeError(w, err)
		return
	}

	// 先按原始 JSON 对象读体：需要逐个 provider 处理 api_key / clear_api_key，
	// 并对 project/story 层拒绝 api_key 与 notify。
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		jsonError(w, 400, "JSON không hợp lệ")
		return
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		jsonError(w, 400, jsonBodyMessage(err))
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if t.scope == "global" {
		if _, has := body["style"]; has {
			jsonError(w, 400, "Văn phong chỉ đặt ở dự án hoặc truyện")
			return
		}
	}
	providers, _ := body["providers"].(map[string]any)
	for _, pv := range providers {
		p, _ := pv.(map[string]any)
		entries, _ := p["models"].([]any)
		for _, entry := range entries {
			model, _ := entry.(map[string]any)
			if _, has := model["context_window"]; has && user.Role != identity.RoleAdmin {
				jsonError(w, 403, "Chỉ quản trị được đổi ngữ cảnh")
				return
			}
		}
	}

	if t.scope != "global" {
		if _, ok := body["notify"]; ok {
			jsonError(w, 400, "Chỉ có thể đổi thông báo trong cài đặt chung, không đặt riêng cho dự án hoặc truyện")
			return
		}
		for _, pv := range providers {
			if p, _ := pv.(map[string]any); p != nil {
				if _, has := p["api_key"]; has {
					jsonError(w, 400, "Chỉ có thể đặt khoá truy cập dịch vụ mô hình trong cài đặt chung, không đặt riêng cho dự án hoặc truyện")
					return
				}
			}
		}
		if tts, _ := body["tts"].(map[string]any); tts != nil {
			if _, has := tts["api_key"]; has {
				jsonError(w, 400, "Chỉ có thể đặt khoá truy cập dịch vụ giọng đọc trong cài đặt chung, không đặt riêng cho dự án hoặc truyện")
				return
			}
		}
	}

	// api_key 语义：vắng/rỗng = giữ khoá cũ（从该层旧文件回插）；clear_api_key = xoá。
	// 旧文件损坏时拒绝静默覆盖（与 SaveProviderConfig 同一约定）。
	existingKeys := map[string]string{}
	existingTTSKey := ""
	saved, hasSaved, loadErr := g.loadTargetConfig(r.Context(), t, user)
	if loadErr != nil {
		scopeError(w, loadErr)
		return
	}
	if hasSaved {
		for name, pc := range saved.Providers {
			existingKeys[name] = pc.APIKey
		}
		existingTTSKey = saved.TTS.APIKey
	}
	// tts.api_key 与 providers.api_key 同语义：GET 只回 *_set/*_hint，写回时忽略这些派生键；
	// api_key rỗng ở cấp toàn cục = giữ khoá cũ。
	if tts, _ := body["tts"].(map[string]any); tts != nil {
		delete(tts, "api_key_set")
		delete(tts, "api_key_hint")
		if t.scope == "global" {
			if key, _ := tts["api_key"].(string); key == "" {
				if existingTTSKey != "" {
					tts["api_key"] = existingTTSKey
				} else {
					delete(tts, "api_key")
				}
			}
		}
	}
	for name, pv := range providers {
		p, _ := pv.(map[string]any)
		if p == nil {
			continue
		}
		delete(p, "api_key_set")
		delete(p, "api_key_hint")
		clear, _ := p["clear_api_key"].(bool)
		delete(p, "clear_api_key")
		if clear {
			delete(p, "api_key")
			continue
		}
		key, _ := p["api_key"].(string)
		if key == "" {
			if old := existingKeys[name]; old != "" {
				p["api_key"] = old
			} else {
				delete(p, "api_key")
			}
		}
	}

	// 严格解码为 Config：未知键（任何深度）→ 400，防止写进无效字段。
	clean, err := json.Marshal(body)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	var cfg bootstrap.Config
	dec := json.NewDecoder(strings.NewReader(string(clean)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		jsonError(w, 400, jsonBodyMessage(err))
		return
	}
	if cfg.AdvanceMode != "" && cfg.AdvanceMode != "auto" && cfg.AdvanceMode != "review" {
		jsonError(w, 400, "Chế độ đi tiếp chỉ có thể là Tự động hoặc Nghiệm thu từng chương")
		return
	}
	if !bootstrap.ValidLanguage(cfg.Language) {
		jsonError(w, 400, "Hiện chỉ hỗ trợ viết tiếng Việt")
		return
	}
	validationConfig, err := g.providerValidationConfig(r.Context(), t, user, cfg)
	if err != nil {
		scopeError(w, err)
		return
	}
	for name, pc := range cfg.Providers {
		resolved := validationConfig.Providers[name]
		if err := bootstrap.ValidateProvider(name, resolved); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		// Codex has a fixed Responses protocol. Accept old clients' redundant
		// value, but never write it back as a configurable endpoint.
		if resolved.OAuthID(name) == codex.ProviderName {
			pc.API = ""
			cfg.Providers[name] = pc
		}
		for _, model := range pc.Models {
			if model.ContextWindow != 0 && (model.ContextWindow < 8000 || model.ContextWindow > 2000000) {
				jsonError(w, 400, "Ngữ cảnh phải từ 8.000 đến 2.000.000")
				return
			}
		}
	}

	if user.Role != identity.RoleAdmin && hasSaved {
		for provider, pc := range saved.Providers {
			for _, model := range pc.Models {
				if model.ContextWindow == 0 {
					continue
				}
				next, _ := cfg.Providers[provider].ModelConfig(model.Name)
				if next.ContextWindow != model.ContextWindow {
					jsonError(w, 403, "Chỉ quản trị được đổi ngữ cảnh")
					return
				}
			}
		}
	}
	// global 层摘掉助理正在用的 model 必须拒绝——409 model_in_use，不写盘。
	if t.scope == "global" && hasSaved {
		if conflict, ok := modelInUseConflict(saved, cfg); ok {
			writeJSON(w, 409, conflict)
			return
		}
	}
	if err := g.saveTargetConfig(r.Context(), t, user, cfg, strings.Trim(r.Header.Get("If-Match"), "\"")); err != nil {
		if errors.Is(err, errs.ErrStorageConflict) {
			res := g.resourceFor(r, user, nil)
			version := g.resourceVersion(res)
			current, loadErr := g.buildConfigResponse(r.Context(), t, user)
			if loadErr != nil {
				scopeError(w, loadErr)
				return
			}
			current["version"] = version
			writeVersionToken(w, version)
			writeJSON(w, http.StatusConflict, map[string]any{"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": version, "current": current})
			return
		}
		jsonError(w, 500, err.Error())
		return
	}
	if err := g.refreshWorkerModelContexts(r.Context(), t.scope, t.project, q.Get("story")); err != nil {
		g.configChanged(t.scope, t.project, q.Get("story"))
		jsonError(w, 502, "Đã lưu thiết lập nhưng chưa cập nhật được ngữ cảnh của trợ lý đang chạy")
		return
	}
	resp, err := g.buildConfigResponse(r.Context(), t, user)
	if err != nil {
		scopeError(w, err)
		return
	}
	writeJSON(w, 200, resp)
}

// ---------- presets / test / models / account ----------

// providerDefaultModels 是每个预设做“Thử kết nối”时的默认模型（14 §G6）：
// 先按 provider id 查，再按 type（自定义代理）查。
var providerDefaultModels = map[string]string{
	"openai-codex":    "gpt-5.6-terra",
	"xai-oauth":       xai.DefaultModel,
	"anthropic-oauth": anthropicoauth.DefaultModel,
	"openai":          "gpt-4o-mini",
	"anthropic":       "claude-sonnet-4",
	"gemini":          "gemini-2.5-flash",
	"openrouter":      "google/gemini-2.5-flash",
	"opencode-go":     "kimi-k2.7-code",
	"deepseek":        "deepseek-chat",
	"qwen":            "qwen-plus",
	"glm":             "glm-4.6",
	"grok":            "grok-4",
	"bedrock":         "anthropic.claude-sonnet-4",
	"ollama":          "qwen2.5:14b",
	// type 回退：自定义代理只填协议时用它。
	"responses": "gpt-5.6-terra",
}

// defaultModelFor 返回 provider（或 type）的默认模型；都没有返回 ""。
func defaultModelFor(provider, typ string) string {
	if m := providerDefaultModels[provider]; m != "" {
		return m
	}
	return providerDefaultModels[typ]
}

func (g *Gateway) servePresets(w http.ResponseWriter, r *http.Request) {
	type preset struct {
		ID           string `json:"id"`
		Label        string `json:"label"`
		Type         string `json:"type"`
		BaseURL      string `json:"baseURL"`
		KeyRequired  bool   `json:"keyRequired"`
		OAuth        bool   `json:"oauth"`
		DeviceOnly   bool   `json:"deviceOnly"`
		PasteCode    bool   `json:"pasteCode"`
		API          string `json:"api"`
		DefaultModel string `json:"defaultModel"`
	}
	out := []preset{}
	for _, p := range bootstrap.ProviderPresets() {
		typ := p.Name
		if p.Name == "custom" {
			typ = ""
		}
		out = append(out, preset{
			ID: p.Name, Label: p.Label, Type: typ, BaseURL: p.BaseURL,
			KeyRequired: !p.APIKeyOptional, OAuth: p.OAuth, DeviceOnly: auth.DeviceOnly(p.Name), PasteCode: auth.PasteCode(p.Name),
			DefaultModel: defaultModelFor(p.Name, typ),
		})
	}
	// 风格清单 = 内置 + ~/.ainovel/style/styles/*.md 用户自建（与运行时 DefaultLoadOptions 的 home 层一致），排序保证输出稳定。
	// gói thể loại KHÔNG trộn vào styles: màn Cách viết render mọi mục lạ thành thẻ văn
	// phong với nhãn slug thô — gói chỉ xuất hiện trong mảng packs riêng (D2).
	bundle := assets.Load("default", assets.DefaultLoadOptions(""))
	styles := make([]string, 0, len(bundle.Styles))
	for name := range bundle.Styles {
		styles = append(styles, name)
	}
	type packInfo struct {
		ID        string `json:"id"`
		Ten       string `json:"ten"`
		Short     string `json:"short"` // goi.json `ten_ngan` — tên gói trên thẻ (Câu 12)
		Meta      string `json:"meta"`  // "8 chương · thoại 40–55%" từ do_do/thoai (Câu 7)
		SoChuong  int    `json:"so_chuong"`
		NguoiNghe string `json:"nguoi_nghe"`
		// ChapterWords là khoảng tiếng mỗi chương gói khai (Câu 1); null khi gói
		// không có `tieng_moi_chuong` — giao diện ẩn dòng độ dài thay vì đoán.
		ChapterWords *assets.PackTiengMoiChuong `json:"chapterWords"`
		packDetail                              // D2: desc/hasRules/available + ruột "Gói … gồm" (đọc từ goi.json + luat/)
	}
	packList := []packInfo{}
	for _, p := range assets.ListPacks(assets.DefaultLoadOptions("")) {
		packList = append(packList, packInfo{
			ID: p.Name, Ten: p.Ten, Short: packShort(p), Meta: packMeta(p),
			SoChuong: p.SoChuong, NguoiNghe: p.NguoiNghe, ChapterWords: p.TiengMoiChuong,
			packDetail: buildPackDetail(p),
		})
	}
	sort.Strings(styles)
	writeJSON(w, 200, map[string]any{
		"providers": out,
		"styles":    styles,
		"packs":     packList,
		"reasoning": []string{"off", "low", "medium", "high", "xhigh", "max"},
	})
}

func (g *Gateway) testConfig(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Provider string            `json:"provider"`
		Model    string            `json:"model"`
		Config   *bootstrap.Config `json:"config,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg := bootstrap.Config{}
	if req.Config != nil {
		cfg = *req.Config
	} else if loaded, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath()); err == nil {
		cfg = loaded
	}
	// G6：缺 model 时用预设的默认模型（按 provider 名，再按自定义代理的 type）。
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultModelFor(req.Provider, cfg.Providers[req.Provider].Type)
	}
	if model == "" {
		writeJSON(w, 200, map[string]any{"ok": false, "latencyMs": 0, "model": "", "message": "Thiếu model: hãy chọn model cho nhà cung cấp này"})
		return
	}
	cfg.Provider, cfg.ModelName = req.Provider, model
	cfg.Roles = nil
	if req.Config != nil {
		inheritSavedAPIKey(&cfg, req.Provider)
	}
	// message 永远是越南语；上游/系统英文原文只放 detail（可选，供“Chi tiết”展开与排障）。
	fail := func(start time.Time, message, detail string) {
		resp := map[string]any{"ok": false, "latencyMs": time.Since(start).Milliseconds(), "model": model, "message": message}
		if detail != "" {
			resp["detail"] = detail
		}
		writeJSON(w, 200, resp)
	}
	start := time.Now()
	pc := cfg.Providers[req.Provider]
	if presetNeedsAPIKey(req.Provider) && strings.TrimSpace(pc.APIKey) == "" {
		fail(start, missingAPIKeyMessage(req.Provider), "")
		return
	}
	if _, err := pc.ProviderType(req.Provider); err != nil {
		fail(start, "Cấu hình chưa hợp lệ: model mặc định: phân tích loại provider thất bại: "+err.Error(), "")
		return
	}
	if err := bootstrap.ValidateProvider(req.Provider, pc); err != nil {
		fail(start, "Cấu hình chưa hợp lệ: "+err.Error(), "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		msg, detail := connectionFailure(req.Provider, pc, err)
		fail(start, msg, detail)
		return
	}
	if _, err := models.Default.Generate(ctx, []llmtypes.Message{llmtypes.UserMsg("Reply OK.")}, nil); err != nil {
		msg, detail := connectionFailure(req.Provider, pc, err)
		fail(start, msg, detail)
		return
	}
	resp := map[string]any{"ok": true, "latencyMs": time.Since(start).Milliseconds(), "model": model, "message": "Kết nối thành công"}
	window, source := resolveTestContextWindow(cfg, req.Provider, model)
	resp["contextWindow"] = window
	resp["contextSource"] = source
	writeJSON(w, 200, resp)
}

// inheritSavedAPIKey：GET 永不回 api_key，Settings/Onboard 未重新输入时草稿不带 key——
// 与 PUT 同一约定（api_key vắng = giữ khoá cũ），用全局层已保存的 key 测试。
func inheritSavedAPIKey(cfg *bootstrap.Config, provider string) {
	pc, ok := cfg.Providers[provider]
	if !ok || strings.TrimSpace(pc.APIKey) != "" {
		return
	}
	saved, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
	if err != nil {
		return
	}
	if key := saved.Providers[provider].APIKey; key != "" {
		pc.APIKey = key
		cfg.Providers[provider] = pc
	}
}

// presetNeedsAPIKey：内置预设（OpenRouter、OpenAI…）即使草稿带了 type 也必须有 key；
// 自定义代理交给适配器判断（见 connectionTestMessage）。
func presetNeedsAPIKey(provider string) bool {
	for _, p := range bootstrap.ProviderPresets() {
		if p.Name == provider {
			return !p.APIKeyOptional && !p.OAuth
		}
	}
	return false
}

func missingAPIKeyMessage(provider string) string {
	return fmt.Sprintf("Chưa có khoá API cho %s — dán khoá API rồi thử lại", providerLabel(provider))
}

// providerLabel 返回预设的短名（"OpenRouter (Claude, Gemini…)" → "OpenRouter"）；自定义代理用带引号的名字。
func providerLabel(provider string) string {
	for _, p := range bootstrap.ProviderPresets() {
		if p.Name == provider {
			label, _, _ := strings.Cut(p.Label, " (")
			return label
		}
	}
	return fmt.Sprintf("%q", provider)
}

// providerEndpoint 返回用户能认出的服务地址（scheme://host）：先取配置的 base_url，再取预设默认值。
func providerEndpoint(provider string, pc llmconfig.ProviderConfig) string {
	base := strings.TrimSpace(pc.BaseURL)
	if base == "" {
		for _, p := range bootstrap.ProviderPresets() {
			if p.Name == provider {
				base = p.BaseURL
			}
		}
	}
	if u, err := url.Parse(base); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return base
}

// connectionFailure 把建模/调用失败拆成越南语 message 与原文 detail：适配器/net/http 的英文
// （"request failed: Post …"、"[validation, …]"）只进 detail；分类用 imp.ModelErrDetail。
func connectionFailure(provider string, pc llmconfig.ProviderConfig, err error) (message, detail string) {
	raw := err.Error()
	if strings.Contains(raw, "api key is required") {
		return missingAPIKeyMessage(provider), ""
	}
	if errors.Is(err, errs.ErrConfig) {
		return "Cấu hình chưa hợp lệ: " + raw, ""
	}
	if pc.UsesOAuth(provider) {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return fmt.Sprintf("Chưa đăng nhập %s — đăng nhập tài khoản rồi thử lại", providerLabel(provider)), raw
		}
		var le *litellm.LiteLLMError
		if errors.As(err, &le) {
			switch {
			case le.Code == "xai_oauth_tier_denied":
				return auth.XAITierDeniedMessage, ""
			case le.Code == anthropicoauth.CodeForbidden || le.Code == anthropicoauth.CodeRateLimited || le.Code == anthropicoauth.CodeUnauthorized:
				return le.Message, ""
			case le.StatusCode == http.StatusUnauthorized:
				return fmt.Sprintf("%s từ chối phiên đăng nhập — hãy đăng nhập lại", providerLabel(provider)), le.Error()
			}
		}
	}
	label := providerLabel(provider)
	var le *litellm.LiteLLMError
	isLite := errors.As(err, &le)
	if isLite {
		raw = le.Error() // 去掉 agentcore 追加的英文事实标签
	}
	if errors.Is(err, context.DeadlineExceeded) || isLite && le.Type == litellm.ErrorTypeTimeout {
		return fmt.Sprintf("%s không phản hồi trong 30 giây — thử lại sau hoặc kiểm tra địa chỉ máy chủ", label), raw
	}
	var netErr net.Error
	if isLite && le.Type == litellm.ErrorTypeNetwork || !isLite && errors.As(err, &netErr) {
		if endpoint := providerEndpoint(provider, pc); endpoint != "" {
			return fmt.Sprintf("Không kết nối được %s tại %s — kiểm tra máy chủ đã chạy và địa chỉ đúng", label, endpoint), raw
		}
		return fmt.Sprintf("Không kết nối được %s — kiểm tra mạng và địa chỉ máy chủ", label), raw
	}
	if isLite && le.Type == litellm.ErrorTypeAuth {
		return fmt.Sprintf("%s từ chối khoá API (%s) — kiểm tra lại khoá", label, imp.ModelErrDetail(err)), raw
	}
	if isLite {
		return fmt.Sprintf("Kết nối thất bại (%s)", imp.ModelErrDetail(err)), raw
	}
	return "Kết nối thất bại: " + raw, ""
}

// listCodexModels / listXAIModels 供测试替换：默认走真实的 ListModels。
var listCodexModels = func(ctx context.Context, baseURL string, tokens codex.TokenSource) ([]codex.Model, error) {
	return codex.ListModels(ctx, baseURL, tokens)
}

var listXAIModels = func(ctx context.Context, baseURL string, tokens xai.TokenSource) ([]xai.Model, error) {
	return xai.ListModels(ctx, baseURL, tokens)
}

var listAnthropicOAuthModels = func(ctx context.Context, baseURL string, tokens anthropicoauth.TokenSource) ([]anthropicoauth.Model, error) {
	return anthropicoauth.ListModels(ctx, baseURL, tokens)
}

type modelInfo struct {
	Name             string                        `json:"name"`
	DisplayName      string                        `json:"displayName"`
	ContextWindow    int                           `json:"contextWindow"`
	ContextSource    bootstrap.ContextWindowSource `json:"contextSource"`
	RegistryWindow   int                           `json:"registryWindow"`
	SummaryThreshold int                           `json:"summaryThreshold"`
	AutomaticWindow  int                           `json:"automaticWindow"`
	AutomaticSource  bootstrap.ContextWindowSource `json:"automaticSource"`
}

// configuredModels 返回 provider 配置里声明的 models（含 context_window）。
func configuredModels(provider string, requested ...string) []modelInfo {
	cfg, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
	if err != nil {
		return []modelInfo{}
	}
	pc := cfg.Providers[provider]
	out := make([]modelInfo, 0, len(pc.Models))
	for _, m := range pc.Models {
		out = append(out, resolvedModelInfo(cfg, provider, m.Name, m.Name))
	}
	add := func(name string) {
		if name == "" {
			return
		}
		for _, model := range out {
			if model.Name == name {
				return
			}
		}
		out = append(out, resolvedModelInfo(cfg, provider, name, name))
	}
	if cfg.Provider == provider {
		add(cfg.ModelName)
	}
	for _, role := range cfg.Roles {
		if role.Provider == provider || (role.Provider == "" && cfg.Provider == provider) {
			add(role.Model)
		}
	}
	for _, name := range requested {
		add(name)
	}
	return out
}

func resolvedModelInfo(cfg bootstrap.Config, provider, name, display string) modelInfo {
	window, source := cfg.ResolveContextWindow(provider, name)
	automatic, automaticSource := (bootstrap.Config{ContextWindow: cfg.ContextWindow}).ResolveContextWindow(provider, name)
	return modelInfo{Name: name, DisplayName: display, ContextWindow: window, ContextSource: source,
		RegistryWindow: models.DefaultRegistry().ResolveContextWindow(name), AutomaticWindow: automatic, AutomaticSource: automaticSource, SummaryThreshold: max(0, window-bootstrap.CompactReserveTokens(window))}
}

func (g *Gateway) configModels(w http.ResponseWriter, r *http.Request, user identity.User) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		jsonError(w, 400, "Chưa chọn nhà cung cấp mô hình")
		return
	}
	if user.Role != identity.RoleAdmin {
		writeJSON(w, 200, map[string]any{"models": configuredModels(provider, r.URL.Query()["model"]...)})
		return
	}
	if isSubscriptionProvider(provider) {
		store, err := auth.DefaultStore()
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		if _, ok, err := store.Get(provider); err != nil {
			jsonError(w, 500, err.Error())
			return
		} else if !ok {
			writeJSON(w, 200, map[string]any{"models": configuredModels(provider, r.URL.Query()["model"]...)})
			return
		}
		// Đăng nhập gói thuê bao có thể diễn ra trước khi có tệp cấu hình (máy mới) — thiếu tệp thì dùng cấu hình rỗng.
		cfg, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			jsonError(w, 500, err.Error())
			return
		}
		models, err := oauthModelList(r.Context(), provider, cfg.Providers[provider].BaseURL, store)
		if err != nil {
			jsonError(w, 502, err.Error())
			return
		}
		for i, model := range models {
			models[i] = resolvedModelInfo(cfg, provider, model.Name, model.DisplayName)
		}
		for _, configured := range configuredModels(provider, r.URL.Query()["model"]...) {
			found := false
			for _, model := range models {
				if model.Name == configured.Name {
					found = true
					break
				}
			}
			if !found {
				models = append(models, configured)
			}
		}
		writeJSON(w, 200, map[string]any{"models": models})
		return
	}
	writeJSON(w, 200, map[string]any{"models": configuredModels(provider, r.URL.Query()["model"]...)})
}

// isSubscriptionProvider: provider đăng nhập bằng gói thuê bao (OAuth) thay cho API key.
func isSubscriptionProvider(provider string) bool {
	return provider == codex.ProviderName || provider == xai.ProviderName || provider == anthropicoauth.ProviderName
}

// oauthModelList liệt kê model mà tài khoản đã đăng nhập của provider gói thuê bao được dùng.
func oauthModelList(ctx context.Context, provider, baseURL string, store *auth.Store) ([]modelInfo, error) {
	manager := auth.NewManager(store)
	if provider == anthropicoauth.ProviderName {
		models, err := listAnthropicOAuthModels(ctx, baseURL, anthropicoauth.ManagerTokens(manager))
		if err != nil {
			return nil, err
		}
		out := make([]modelInfo, 0, len(models))
		for _, m := range models {
			out = append(out, modelInfo{Name: m.Slug, DisplayName: m.DisplayName})
		}
		return out, nil
	}
	if provider == xai.ProviderName {
		models, err := listXAIModels(ctx, baseURL, xai.ManagerTokens(manager))
		if err != nil {
			return nil, err
		}
		out := make([]modelInfo, 0, len(models))
		for _, m := range models {
			out = append(out, modelInfo{Name: m.Slug, DisplayName: m.DisplayName})
		}
		return out, nil
	}
	models, err := listCodexModels(ctx, baseURL, codex.ManagerTokens(manager))
	if err != nil {
		return nil, err
	}
	out := make([]modelInfo, 0, len(models))
	for _, m := range models {
		out = append(out, modelInfo{Name: m.Slug, DisplayName: m.DisplayName, ContextWindow: m.ContextWindow})
	}
	return out, nil
}

// accountView 返回一个 OAuth provider 的持久账户状态（G7）；读 ~/.ainovel/auth.json。
func accountView(store *auth.Store, provider string) map[string]any {
	out := map[string]any{"connected": false, "label": "", "email": "", "plan": "", "expiresAt": nil}
	cred, ok, err := store.Get(provider)
	if err != nil || !ok {
		return out
	}
	out["connected"] = true
	out["label"] = cred.Label()
	out["email"] = cred.Email
	out["plan"] = cred.Plan
	if cred.Expires > 0 {
		out["expiresAt"] = cred.ExpiresAt().Format(time.RFC3339)
	}
	return out
}

// configAccounts 返回全部 OAuth 预设的已连接账户（仅 admin，见 14 §G7）。
func (g *Gateway) configAccounts(admin bool) map[string]any {
	out := map[string]any{}
	if !admin {
		return out
	}
	store, err := auth.DefaultStore()
	if err != nil {
		return out
	}
	for _, p := range bootstrap.ProviderPresets() {
		if !p.OAuth {
			continue
		}
		out[p.Name] = accountView(store, p.Name)
	}
	return out
}

func (g *Gateway) configAccount(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = auth.OpenAICodexID
	}
	store, err := auth.DefaultStore()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, accountView(store, provider))
}

// ---------- OAuth (GAP-16) ----------

// oauthSession 跟踪一次进行中的 OAuth 登录（gateway 进程内，不持久化）。
type oauthSession struct {
	id        string
	provider  string
	mode      string // browser | device | paste
	cancel    context.CancelFunc
	createdAt time.Time
	expiresAt time.Time

	manual   chan auth.ManualSubmission // mã dán tay (chỉ provider PasteCode)
	mu       sync.Mutex
	state    string // pending | done | failed
	url      string
	userCode string
	verify   string
	cred     *auth.Credential
	err      string
	ready    chan struct{}
	notify   func(string)
}

// notifyLocked also covers pending metadata becoming ready (URL/device code).
func (s *oauthSession) notifyLocked() {
	if s.notify != nil {
		s.notify(s.state)
	}
}

func (s *oauthSession) expire(now time.Time) {
	s.mu.Lock()
	expired := s.state == "pending" && !now.Before(s.expiresAt)
	if expired {
		s.state, s.err = "failed", "Đã hết thời gian đăng nhập"
		s.notifyLocked()
	}
	s.mu.Unlock()
	if expired && s.cancel != nil {
		s.cancel()
	}
}

var oauthSessions = struct {
	mu   sync.Mutex
	byID map[string]*oauthSession
}{byID: map[string]*oauthSession{}}

func newOAuthSessionID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func getOAuthSession(id string) *oauthSession {
	oauthSessions.mu.Lock()
	defer oauthSessions.mu.Unlock()
	// 顺手回收超过 20 分钟的会话，防泄漏（ctx 10 分钟超时已兜底 pending）。
	for k, s := range oauthSessions.byID {
		if time.Since(s.createdAt) > 20*time.Minute {
			delete(oauthSessions.byID, k)
		}
	}
	sess := oauthSessions.byID[id]
	if sess != nil {
		sess.expire(time.Now())
	}
	return sess
}

// isLoopbackRequest 判断发起请求的浏览器是否与 serve 同机（决定 browser 流是否可用）。
func isLoopbackRequest(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}

func (g *Gateway) oauthStart(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Device   bool   `json:"device"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" {
		jsonError(w, 400, "Chưa chọn nhà cung cấp mô hình")
		return
	}
	if _, ok := auth.Lookup(req.Provider); !ok {
		jsonError(w, 400, "Nhà cung cấp này chưa hỗ trợ đăng nhập bằng tài khoản")
		return
	}
	// xAI chỉ có device-code: luôn dùng mã thiết bị, không cần callback localhost.
	if auth.DeviceOnly(req.Provider) {
		req.Device = true
	}
	// Provider nhận mã dán tay (Claude): mở từ thiết bị khác thì tự chuyển sang dán mã thay vì báo lỗi.
	pasteCode := auth.PasteCode(req.Provider)
	if pasteCode && !req.Device && !isLoopbackRequest(r) {
		req.Device = true
	}
	if !req.Device && !isLoopbackRequest(r) {
		jsonError(w, 400, "Đăng nhập qua trình duyệt chỉ khả dụng khi bạn mở Noverse trên chính máy chạy serve — hãy dùng “Đăng nhập bằng mã”")
		return
	}
	store, err := auth.DefaultStore()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	mode := "browser"
	if req.Device {
		mode = "device"
	}
	if pasteCode && req.Device {
		mode = "paste" // không có mã thiết bị: mở link rồi dán mã trả về
	}
	sess := &oauthSession{
		id: newOAuthSessionID(), provider: req.Provider, mode: mode,
		state: "pending", createdAt: time.Now(), expiresAt: time.Now().Add(10 * time.Minute),
		ready: make(chan struct{}),
	}
	if pasteCode {
		sess.manual = make(chan auth.ManualSubmission)
	}
	sess.notify = func(status string) {
		g.events().Publish(Event{Type: "config.changed", User: user.Username, Data: map[string]any{"scope": "oauth", "flow": sess.id, "status": status}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	sess.cancel = cancel

	signalReady := sync.OnceFunc(func() { close(sess.ready) })
	ctrl := auth.Controller{
		OnAuth: func(url, _ string) {
			if sess.mode == "browser" || sess.mode == "paste" {
				sess.mu.Lock()
				if sess.state == "pending" {
					sess.url = url
					sess.notifyLocked()
				}
				sess.mu.Unlock()
				signalReady()
			}
			// device 流等 OnDeviceCode 再就绪（verify URL + mã cùng đến）。
		},
		OnDeviceCode: func(userCode, verifyURL string) {
			sess.mu.Lock()
			if sess.state == "pending" {
				sess.userCode, sess.verify = userCode, verifyURL
				sess.notifyLocked()
			}
			sess.mu.Unlock()
			signalReady()
		},
	}

	oauthSessions.mu.Lock()
	oauthSessions.byID[sess.id] = sess
	oauthSessions.mu.Unlock()
	sess.mu.Lock()
	sess.notifyLocked()
	sess.mu.Unlock()
	go func() {
		<-ctx.Done()
		sess.mu.Lock()
		defer sess.mu.Unlock()
		if sess.state == "pending" {
			sess.state, sess.err = "failed", "Đã huỷ"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				sess.err = "Đã hết thời gian đăng nhập"
			}
			sess.notifyLocked()
		}
	}()

	go func() {
		defer signalReady()
		defer cancel()
		cred, err := auth.Login(ctx, store, sess.provider, ctrl, auth.LoginOptions{Device: req.Device, From: "web", Manual: sess.manual})
		sess.mu.Lock()
		defer sess.mu.Unlock()
		if sess.state != "pending" {
			return
		}
		if err != nil {
			sess.state, sess.err = "failed", err.Error()
			if errors.Is(err, context.DeadlineExceeded) {
				sess.err = "Đã hết thời gian đăng nhập"
			}
			if errors.Is(err, context.Canceled) {
				sess.err = "Đã huỷ"
			}
			sess.notifyLocked()
			return
		}
		sess.state = "done"
		sess.cred = &cred
		sess.notifyLocked()
		g.configChanged("global", "", "")
	}()

	// 等待 provider 报出 URL / 设备码（或快速失败），让响应能直接带上 url/userCode。
	select {
	case <-sess.ready:
	case <-time.After(20 * time.Second):
	case <-r.Context().Done():
	}
	sess.mu.Lock()
	resp := map[string]any{
		"id": sess.id, "mode": sess.mode, "url": sess.url,
		"userCode": sess.userCode, "verifyURL": sess.verify,
		"expiresAt": sess.expiresAt.Format(time.RFC3339),
	}
	if sess.state == "failed" {
		resp["error"] = sess.err
	}
	sess.mu.Unlock()
	writeJSON(w, 200, resp)
}

func (g *Gateway) oauthStatus(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	sess := getOAuthSession(r.URL.Query().Get("id"))
	if sess == nil {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	sess.mu.Lock()
	resp := map[string]any{
		"state": sess.state, "mode": sess.mode,
		"url": sess.url, "userCode": sess.userCode, "verifyURL": sess.verify,
		"expiresAt": sess.expiresAt.Format(time.RFC3339),
		"account":   nil, "error": sess.err, "models": []modelInfo{},
	}
	cred := sess.cred
	provider := sess.provider
	sess.mu.Unlock()
	if cred != nil {
		resp["account"] = map[string]any{"label": cred.Label(), "email": cred.Email, "plan": cred.Plan}
		if isSubscriptionProvider(provider) {
			if store, err := auth.DefaultStore(); err == nil {
				if models, err := oauthModelList(r.Context(), provider, "", store); err == nil {
					resp["models"] = models
				}
			}
		} else {
			resp["models"] = configuredModels(provider)
		}
	}
	writeJSON(w, 200, resp)
}

func (g *Gateway) oauthCancel(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sess := getOAuthSession(req.ID)
	if sess == nil {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	sess.mu.Lock()
	if sess.state == "pending" {
		sess.state = "failed"
		sess.err = "Đã huỷ"
		sess.notifyLocked()
	}
	sess.mu.Unlock()
	sess.cancel()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// oauthComplete nhận mã dán tay (`code#state` hoặc URL chuyển hướng) của luồng đăng nhập đang chờ
// và chuyển cho provider kiểm tra; trả lỗi tiếng Việt nếu mã không dùng được (luồng vẫn chờ).
func (g *Gateway) oauthComplete(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sess := getOAuthSession(req.ID)
	if sess == nil {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	if sess.manual == nil {
		jsonError(w, 400, "Phiên đăng nhập này không nhận mã dán tay")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		jsonError(w, 400, "Hãy dán mã xác thực hoặc URL chuyển hướng")
		return
	}
	sess.mu.Lock()
	pending := sess.state == "pending"
	sess.mu.Unlock()
	if !pending {
		jsonError(w, 409, "Phiên đăng nhập đã kết thúc — hãy bắt đầu lại")
		return
	}
	result := make(chan error, 1)
	select {
	case sess.manual <- auth.ManualSubmission{Input: req.Code, Result: result}:
	case <-time.After(5 * time.Second):
		jsonError(w, 409, "Phiên đăng nhập không còn chờ mã — hãy bắt đầu lại")
		return
	case <-r.Context().Done():
		return
	}
	select {
	case err := <-result:
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
	case <-time.After(5 * time.Second):
		jsonError(w, 504, "Không nhận được phản hồi khi kiểm tra mã")
		return
	case <-r.Context().Done():
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (g *Gateway) oauthLogout(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Provider string `json:"provider"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" {
		req.Provider = auth.OpenAICodexID
	}
	store, err := auth.DefaultStore()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if _, err := store.Delete(req.Provider); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- /api/rules ----------

type ruleFileInfo struct {
	Name      string  `json:"name"`
	Size      int64   `json:"size"`
	UpdatedAt *string `json:"updatedAt"`
}

type rulesChainDoc struct {
	Scope  string         `json:"scope"`
	Dir    string         `json:"dir"`
	Exists bool           `json:"exists"`
	Files  []ruleFileInfo `json:"files"`
}

// rulesScopeDirs exposes project → story rules, matching the runtime loader.
func (g *Gateway) rulesScopeDirs(t configTarget, admin bool) ([]rulesChainDoc, error) {
	var dirs []rulesChainDoc
	var firstErr error
	addLevel := func(scope, dir string) {
		if dir == "" {
			return
		}
		display := g.configDisplayPath(dir, scope, admin)
		for _, d := range dirs {
			if d.Dir == display {
				return
			}
		}
		doc := rulesChainDoc{Scope: scope, Dir: display, Files: []ruleFileInfo{}}
		entries, err := os.ReadDir(dir)
		switch {
		case err == nil:
			doc.Exists = true
			for _, e := range entries {
				if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
					continue
				}
				info, serr := e.Info()
				if serr != nil {
					continue
				}
				ts := info.ModTime().UTC().Format(time.RFC3339)
				doc.Files = append(doc.Files, ruleFileInfo{Name: e.Name(), Size: info.Size(), UpdatedAt: &ts})
			}
		case !os.IsNotExist(err) && firstErr == nil:
			firstErr = err
		}
		dirs = append(dirs, doc)
	}

	switch t.scope {
	case "project":
		addLevel("project", rulesDirIn(t.dir))
	case "story":
		if t.story.Project != "" && t.story.Project != library.LegacyProjectID && t.story.ProjectDir != "" && t.story.ProjectDir != t.story.Dir {
			addLevel("project", rulesDirIn(t.story.ProjectDir))
		}
		addLevel("story", rulesDirIn(t.story.Dir))
	}
	return dirs, firstErr
}

func rulesHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".ainovel", "rules")
}

// rulesTargetDir 返回 PUT/DELETE/GET file 的目标层目录。
func (t configTarget) rulesTargetDir() string {
	switch t.scope {
	case "project", "story":
		return rulesDirIn(t.dir)
	}
	return ""
}

var ruleFileName = regexp.MustCompile(`^[\p{L}\p{N}._-]+\.md$`)

func validRuleName(name string) bool {
	return ruleFileName.MatchString(name)
}

func (g *Gateway) getRules(w http.ResponseWriter, r *http.Request, user identity.User) {
	q := r.URL.Query()
	t, err := g.resolveRulesScope(q.Get("scope"), q.Get("project"), q.Get("story"))
	if err != nil {
		scopeError(w, err)
		return
	}
	if !configReadAllowed(user, t) {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	if t.hosted {
		dirs, err := g.hostedRulesChain(r.Context(), user, t)
		if err != nil {
			scopeError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"chain": dirs})
		return
	}
	dirs, err := g.rulesScopeDirs(t, isAdmin(user))
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"chain": dirs})
}

func (g *Gateway) getRuleFile(w http.ResponseWriter, r *http.Request, user identity.User) {
	q := r.URL.Query()
	t, err := g.resolveRulesScope(q.Get("scope"), q.Get("project"), q.Get("story"))
	if err != nil {
		scopeError(w, err)
		return
	}
	if !configReadAllowed(user, t) {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	name := q.Get("name")
	if !validRuleName(name) {
		jsonError(w, 400, "tên tệp không hợp lệ")
		return
	}
	if t.hosted {
		g.hostedRuleFile(w, r, user, t, name, "", false)
		return
	}
	data, err := os.ReadFile(filepath.Join(t.rulesTargetDir(), name))
	if err != nil {
		if os.IsNotExist(err) {
			jsonError(w, 404, "không tìm thấy")
		} else {
			jsonError(w, 500, err.Error())
		}
		return
	}
	writeJSON(w, 200, map[string]any{"name": name, "text": string(data)})
}

func (g *Gateway) putRuleFile(w http.ResponseWriter, r *http.Request, user identity.User) {
	var req struct {
		Scope   string `json:"scope"`
		Project string `json:"project,omitempty"`
		Story   string `json:"story,omitempty"`
		Name    string `json:"name"`
		Text    string `json:"text"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	t, err := g.resolveRulesScope(req.Scope, req.Project, req.Story)
	if err != nil {
		scopeError(w, err)
		return
	}
	if err := checkConfigWrite(user, t); err != nil {
		scopeError(w, err)
		return
	}
	if !validRuleName(req.Name) {
		jsonError(w, 400, "tên tệp không hợp lệ")
		return
	}
	if t.hosted {
		g.hostedRuleFile(w, r, user, t, req.Name, req.Text, false)
		return
	}
	dir := t.rulesTargetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	path := filepath.Join(dir, req.Name)
	if err := os.WriteFile(path, []byte(req.Text), 0o644); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"name": req.Name, "size": info.Size(),
		"updatedAt": info.ModTime().UTC().Format(time.RFC3339),
	})
}

func (g *Gateway) deleteRuleFile(w http.ResponseWriter, r *http.Request, user identity.User) {
	var req struct {
		Scope   string `json:"scope"`
		Project string `json:"project,omitempty"`
		Story   string `json:"story,omitempty"`
		Name    string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	t, err := g.resolveRulesScope(req.Scope, req.Project, req.Story)
	if err != nil {
		scopeError(w, err)
		return
	}
	if err := checkConfigWrite(user, t); err != nil {
		scopeError(w, err)
		return
	}
	if !validRuleName(req.Name) {
		jsonError(w, 400, "tên tệp không hợp lệ")
		return
	}
	if t.hosted {
		g.hostedRuleFile(w, r, user, t, req.Name, "", true)
		return
	}
	if err := os.Remove(filepath.Join(t.rulesTargetDir(), req.Name)); err != nil {
		if os.IsNotExist(err) {
			jsonError(w, 404, "không tìm thấy")
		} else {
			jsonError(w, 500, err.Error())
		}
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- /api/system ----------

func (g *Gateway) serveSystem(w http.ResponseWriter, r *http.Request, user identity.User) {
	latest := g.latestRelease()
	writeJSON(w, 200, map[string]any{
		"version": g.build.Version,
		"commit":  g.build.Commit,
		"built":   g.build.Date,
		// P0-2：被动检查新版本的结果；检查未完成/失败/当前是 dev 时 latestVersion
		// 为空串、hasUpdate 为 false。
		"latestVersion": latest.Latest,
		"hasUpdate":     latest.HasUpdate,
		// D1 “Có gì mới”：bản mới (nếu có) + bản đang chạy; notes là thân release
		// markdown nguyên văn, rỗng khi ngoại tuyến/chưa có dữ liệu. Không gọi mạng
		// thêm — dùng kết quả đã cache 24h.
		"releases": version.Changelog(latest, g.build.Version, g.build.Date),
		"root":     g.responsePath(g.RootPath(), isAdmin(user)),
		// Nơi › Thư mục đã dịch sẵn; UI chỉ hiện chuỗi này, không hiện tên thư mục trần.
		"rootDisplay": g.rootDisplay(),
		// 合约固定值（engine 的确定性重试/死锁上界，见 internal/host/engine.go）。
		"retries":              7,
		"deadlock":             []int{3, 5},
		"compactThreshold":     bootstrap.CompactRatio,
		"promptCache":          true,
		"outputDir":            "./output/novel",
		"defaultContextWindow": bootstrap.DefaultContextWindow,
		// Only administrators connected directly from loopback may open server folders.
		"canReveal": g.libRepo == nil && isAdmin(user) && isDirectLoopback(r),
	})
}
