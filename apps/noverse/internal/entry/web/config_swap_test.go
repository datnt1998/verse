package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
	"github.com/datnt1998/verse/platform/identity"
)

const swapConfig = `{
  "provider": "openai-codex",
  "model": "gpt-6-luna",
  "providers": {
    "openai-codex": {"models": [{"name": "gpt-6-luna"}]},
    "gemini": {"models": [{"name": "gemini-2.5-pro"}]}
  },
  "roles": {
    "writer": {"provider": "openai-codex", "model": "gpt-6-luna", "reasoning_effort": "high"},
    "editor": {"provider": "openai-codex", "model": "gpt-6-luna"},
    "architect": {"provider": "gemini", "model": "gemini-2.5-pro"},
    "reviewer": {"provider": "openai-codex", "model": "gpt-6-luna", "fallbacks": [{"provider": "gemini", "model": "gemini-2.5-pro"}]}
  },
  "budget": {"book_usd": 9.5},
  "notify": {"enabled": true}
}`

func TestSwapProviderIsAdminOnlyAndUndoRestoresOnlyAIValues(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, swapConfig)
	admin := tg.login("admin", "admin-pass")
	owner := tg.login("writer", "writer-pass")

	if w := tg.do("POST", "/api/config/swap-provider", owner, `{"from":"openai-codex","to":"gemini"}`); w.Code != http.StatusForbidden {
		t.Fatalf("non-admin swap = %d: %s", w.Code, w.Body.String())
	}

	w := tg.do("POST", "/api/config/swap-provider", admin, `{"from":"openai-codex","to":"gemini"}`)
	if w.Code != 200 {
		t.Fatalf("swap = %d: %s", w.Code, w.Body.String())
	}
	var swapped struct {
		Swapped []struct {
			Role string `json:"role"`
			From struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"from"`
			To struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"to"`
		} `json:"swapped"`
		UndoToken string `json:"undoToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &swapped); err != nil {
		t.Fatal(err)
	}
	if swapped.UndoToken == "" {
		t.Fatal("missing undoToken")
	}
	gotRoles := map[string]bool{}
	for _, row := range swapped.Swapped {
		gotRoles[row.Role] = true
		if row.To.Provider != "gemini" || row.To.Model != "gemini-2.5-pro" {
			t.Fatalf("row %s -> %+v", row.Role, row.To)
		}
	}
	for _, role := range []string{"default", "writer", "editor", "reviewer"} {
		if !gotRoles[role] {
			t.Fatalf("role %s was not swapped: %+v", role, gotRoles)
		}
	}
	if gotRoles["architect"] {
		t.Fatal("architect already used gemini and must not be reported as swapped")
	}

	// Stored document: AI values changed, everything else intact.
	path := filepath.Join(tg.home, ".ainovel", "config.json")
	cfg := readConfigMap(t, path)
	if cfg["provider"] != "gemini" || cfg["model"] != "gemini-2.5-pro" {
		t.Fatalf("default selection = %v/%v", cfg["provider"], cfg["model"])
	}
	roles := cfg["roles"].(map[string]any)
	writer := roles["writer"].(map[string]any)
	if writer["provider"] != "gemini" {
		t.Fatalf("writer = %+v", writer)
	}
	reviewer := roles["reviewer"].(map[string]any)
	if _, ok := reviewer["fallbacks"]; ok {
		t.Fatalf("fallback to the new primary/old provider must be dropped: %+v", reviewer)
	}
	budget := cfg["budget"].(map[string]any)
	if budget["book_usd"] != 9.5 {
		t.Fatalf("budget lost: %+v", budget)
	}
	notify := cfg["notify"].(map[string]any)
	if notify["enabled"] != true {
		t.Fatalf("notify lost: %+v", notify)
	}
	if _, added := cfg["style"]; added {
		t.Fatal("swap populated unrelated non-model defaults")
	}
	// A concurrent non-AI edit must survive undo.
	budget["book_usd"] = 17.25
	writeConfigMap(t, path, cfg)

	w = tg.do("POST", "/api/config/swap-provider/undo", admin, `{"token":"`+swapped.UndoToken+`"}`)
	if w.Code != 200 {
		t.Fatalf("undo = %d: %s", w.Code, w.Body.String())
	}
	cfg = readConfigMap(t, path)
	if cfg["provider"] != "openai-codex" || cfg["model"] != "gpt-6-luna" {
		t.Fatalf("default not restored: %v/%v", cfg["provider"], cfg["model"])
	}
	roles = cfg["roles"].(map[string]any)
	writer = roles["writer"].(map[string]any)
	if writer["provider"] != "openai-codex" || writer["model"] != "gpt-6-luna" || writer["reasoning_effort"] != "high" {
		t.Fatalf("writer not restored: %+v", writer)
	}
	reviewer = roles["reviewer"].(map[string]any)
	fallbacks, _ := reviewer["fallbacks"].([]any)
	if len(fallbacks) != 1 {
		t.Fatalf("reviewer fallbacks not restored: %+v", reviewer)
	}
	budget = cfg["budget"].(map[string]any)
	if budget["book_usd"] != 17.25 {
		t.Fatalf("concurrent budget edit lost by undo: %+v", budget)
	}
}

func TestSwapUndoRefusesConcurrentConfigChange(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, swapConfig)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("POST", "/api/config/swap-provider", admin, `{"from":"openai-codex","to":"gemini"}`)
	if w.Code != 200 {
		t.Fatalf("swap = %d: %s", w.Code, w.Body.String())
	}
	var swapped struct {
		UndoToken string `json:"undoToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &swapped); err != nil {
		t.Fatal(err)
	}

	// Another tab changes one of the affected AI values after the swap.
	path := filepath.Join(tg.home, ".ainovel", "config.json")
	cfg := readConfigMap(t, path)
	roles := cfg["roles"].(map[string]any)
	writer := roles["writer"].(map[string]any)
	writer["reasoning_effort"] = "low"
	writeConfigMap(t, path, cfg)

	w = tg.do("POST", "/api/config/swap-provider/undo", admin, `{"token":"`+swapped.UndoToken+`"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("undo after concurrent edit = %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"conflict":true`) {
		t.Fatalf("conflict payload = %s", w.Body.String())
	}
}

func TestSwapProviderIfMatchMismatchDoesNotWrite(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, swapConfig)
	admin := tg.login("admin", "admin-pass")
	before, err := os.ReadFile(filepath.Join(tg.home, ".ainovel", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	w := tg.doWith("POST", "/api/config/swap-provider", admin, `{"from":"openai-codex","to":"gemini"}`, func(r *http.Request) {
		r.Header.Set("If-Match", `"stale-version"`)
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("stale If-Match = %d: %s", w.Code, w.Body.String())
	}
	after, err := os.ReadFile(filepath.Join(tg.home, ".ainovel", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("stale If-Match still rewrote the config")
	}
}

func TestSwapProviderCommittedEvenWhenWorkerReloadFails(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, swapConfig)
	admin := tg.login("admin", "admin-pass")
	// Worker không với tới được: cấu hình đã ghi vẫn phải trả 200 + undoToken,
	// kèm cảnh báo; lượt sau worker tự nhận qua reload/restart.
	tg.g.workers["proj/01-story"] = &storyWorker{
		worker: &launchedWorker{addr: "127.0.0.1:1"}, running: true,
	}
	w := tg.do("POST", "/api/config/swap-provider", admin, `{"from":"openai-codex","to":"gemini"}`)
	if w.Code != 200 {
		t.Fatalf("committed swap = %d: %s", w.Code, w.Body.String())
	}
	var swapped struct {
		UndoToken string `json:"undoToken"`
		Warning   string `json:"warning"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &swapped); err != nil {
		t.Fatal(err)
	}
	if swapped.UndoToken == "" || swapped.Warning == "" {
		t.Fatalf("cần undoToken + cảnh báo khi worker lỗi: %s", w.Body.String())
	}
	cfg := readConfigMap(t, filepath.Join(tg.home, ".ainovel", "config.json"))
	if cfg["provider"] != "gemini" {
		t.Fatalf("cấu hình phải đã ghi: %v", cfg["provider"])
	}
	w = tg.do("POST", "/api/config/swap-provider/undo", admin, `{"token":"`+swapped.UndoToken+`"}`)
	if w.Code != 200 {
		t.Fatalf("committed undo = %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"warning"`) {
		t.Fatalf("undo phải trả cảnh báo thay vì 502: %s", w.Body.String())
	}
	cfg = readConfigMap(t, filepath.Join(tg.home, ".ainovel", "config.json"))
	if cfg["provider"] != "openai-codex" {
		t.Fatalf("undo phải khôi phục cấu hình: %v", cfg["provider"])
	}
}

// The fingerprint check and write must share PUT's resource lock; serializing
// only swap/undo would still race an ordinary settings save.
func TestProviderSwapSharesGlobalConfigWriteLock(t *testing.T) {
	tg := newConfigTestGateway(t)
	user := identity.User{Role: identity.RoleAdmin}
	config := tg.g.resourceFor(httptest.NewRequest(http.MethodPut, "/api/config", nil), user, url.Values{"scope": {"global"}})
	if config == nil {
		t.Fatal("missing global configuration resource")
	}
	for _, path := range []string{"/api/config/swap-provider", "/api/config/swap-provider/undo"} {
		resource := tg.g.resourceFor(httptest.NewRequest(http.MethodPost, path, nil), user, nil)
		if resource == nil || tg.g.versionLock(resource) != tg.g.versionLock(config) {
			t.Fatalf("%s does not serialize with a global settings save", path)
		}
	}
}

func TestSwapUndoRejectsRemovedCatalogReferencesWithoutConsumingToken(t *testing.T) {
	for _, removed := range []string{"provider", "model"} {
		t.Run(removed, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			tg.globalConfig(t, swapConfig)
			server := httptest.NewServer(tg.g)
			defer server.Close()
			// Exercise the live HTTP gateway over an ephemeral loopback port,
			// with this fixture's isolated HOME/root and no model calls.
			do := func(method, path, cookie, body string) *httptest.ResponseRecorder {
				t.Helper()
				req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				if path != "/api/login" && (method == http.MethodPost || method == http.MethodPut) {
					req.Header.Set("If-Match", `"`+fileVersion(filepath.Join(tg.home, ".ainovel", "config.json"))+`"`)
				}
				if cookie != "" {
					req.AddCookie(&http.Cookie{Name: tg.g.cookiePolicy().Name, Value: cookie})
				}
				resp, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				w := httptest.NewRecorder()
				w.HeaderMap = resp.Header.Clone()
				w.WriteHeader(resp.StatusCode)
				if _, err := io.Copy(w, resp.Body); err != nil {
					t.Fatal(err)
				}
				return w
			}
			admin := sessionFrom(t, do("POST", "/api/login", "", `{"username":"admin","password":"admin-pass"}`))
			w := do("POST", "/api/config/swap-provider", admin, `{"from":"openai-codex","to":"gemini"}`)
			if w.Code != http.StatusOK {
				t.Fatalf("swap = %d: %s", w.Code, w.Body.String())
			}
			var swapped struct {
				UndoToken string `json:"undoToken"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &swapped); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(tg.home, ".ainovel", "config.json")
			cfg := readConfigMap(t, path)
			providers := cfg["providers"].(map[string]any)
			oldProvider := providers["openai-codex"]
			if removed == "provider" {
				delete(providers, "openai-codex")
			} else {
				providers["openai-codex"] = map[string]any{"models": []any{map[string]any{"name": "other-model"}}}
			}
			cfg["budget"].(map[string]any)["book_usd"] = 77.5
			put, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			// Use the actual settings route, not a synthetic invalid catalog:
			// A is unused after the swap and its deletion is permitted.
			w = do(http.MethodPut, "/api/config?scope=global", admin, string(put))
			if w.Code != http.StatusOK {
				t.Fatalf("remove unused %s = %d: %s", removed, w.Code, w.Body.String())
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			w = do("POST", "/api/config/swap-provider/undo", admin, `{"token":"`+swapped.UndoToken+`"}`)
			if w.Code != http.StatusConflict {
				t.Fatalf("undo with removed %s = %d: %s", removed, w.Code, w.Body.String())
			}
			after, err := os.ReadFile(path)
			if err != nil || string(before) != string(after) {
				t.Fatalf("rejected undo changed the stored document: %v", err)
			}
			configSwapStore.Lock()
			_, retained := configSwapStore.entries[swapped.UndoToken]
			configSwapStore.Unlock()
			if !retained {
				t.Fatal("rejected undo consumed its token")
			}
			// Repair the catalog with NEW credentials; the same undo must now
			// succeed, preserving both those credentials and the budget edit.
			cfg = readConfigMap(t, path)
			repaired := oldProvider.(map[string]any)
			repaired["api_key"] = "new-current-key"
			cfg["providers"].(map[string]any)["openai-codex"] = repaired
			put, err = json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			w = do(http.MethodPut, "/api/config?scope=global", admin, string(put))
			if w.Code != http.StatusOK {
				t.Fatalf("repair catalog = %d: %s", w.Code, w.Body.String())
			}
			w = do("POST", "/api/config/swap-provider/undo", admin, `{"token":"`+swapped.UndoToken+`"}`)
			if w.Code != http.StatusOK {
				t.Fatalf("retry retained undo = %d: %s", w.Code, w.Body.String())
			}
			cfg = readConfigMap(t, path)
			if cfg["provider"] != "openai-codex" || cfg["budget"].(map[string]any)["book_usd"] != 77.5 {
				t.Fatalf("undo lost selection or concurrent budget: %+v", cfg)
			}
			if cfg["providers"].(map[string]any)["openai-codex"].(map[string]any)["api_key"] != "new-current-key" {
				t.Fatal("undo restored obsolete credentials")
			}
		})
	}
}

func TestSwapUndoValidatesRestoredFallbackCatalogAndPolicy(t *testing.T) {
	for _, invalid := range []string{"provider", "model", "same-provider"} {
		t.Run(invalid, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cfg := bootstrap.Config{
				Provider: "a", ModelName: "main",
				Providers: map[string]llmconfig.ProviderConfig{
					"a": {Type: "ollama", Models: []llmconfig.ModelConfig{{Name: "main"}}},
					"b": {Type: "ollama", Models: []llmconfig.ModelConfig{{Name: "backup"}}},
				},
				Roles: map[string]roles.RoleConfig{
					"writer": {Provider: "a", Model: "main", Fallbacks: []roles.ModelRef{{Provider: "b", Model: "backup"}}},
				},
			}
			switch invalid {
			case "provider":
				delete(cfg.Providers, "b")
			case "model":
				pc := cfg.Providers["b"]
				pc.Models = []llmconfig.ModelConfig{{Name: "different"}}
				cfg.Providers["b"] = pc
			case "same-provider":
				rc := cfg.Roles["writer"]
				rc.Fallbacks = []roles.ModelRef{{Provider: "a", Model: "main"}}
				cfg.Roles["writer"] = rc
			}
			if err := validateSwapUndoReferences(cfg, []swapUndoRow{{Row: swapRow{Role: "writer"}}}); err == nil {
				t.Fatalf("restored %s fallback was accepted", invalid)
			}
		})
	}
}

func TestConfigWriteSnapshotPreservesConcurrentBudgetEdit(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, swapConfig)
	tgt, err := tg.g.resolveConfigScope("global", "", "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := configWriteSnapshot(tgt, "")
	if err != nil {
		t.Fatal(err)
	}
	user := identity.User{Role: identity.RoleAdmin}
	stale, _, err := tg.g.loadTargetConfig(t.Context(), tgt, user)
	if err != nil {
		t.Fatal(err)
	}
	current := bootstrap.CloneConfig(stale)
	current.Budget.BookUSD = 93
	if err := bootstrap.SaveConfig(tgt.path, current); err != nil {
		t.Fatal(err)
	}
	if err := tg.g.saveConfigWriteSnapshot(t.Context(), tgt, user, stale, "", snapshot); err != errs.ErrStorageConflict {
		t.Fatalf("stale settings/swap write = %v", err)
	}
	got, err := bootstrap.LoadConfigFile(tgt.path)
	if err != nil || got.Budget.BookUSD != 93 {
		t.Fatalf("concurrent budget edit lost: %+v %v", got.Budget, err)
	}
}

// Gate the actual middleware/handler boundary over TCP, not request scheduling:
// A passes beginFeedback, a separate process durably saves B, then putConfig
// captures its snapshot. The client's A token must not be rebound to B.
func TestConfigIfMatchBindsMiddlewareToHandlerSnapshot(t *testing.T) {
	if path := os.Getenv("AU_IFMATCH_WRITER_PATH"); path != "" {
		cfg, err := bootstrap.LoadConfigFile(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Budget.BookUSD = 87.25
		if err := bootstrap.SaveConfig(path, cfg); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, scope := range []string{"global", "project", "story"} {
		t.Run(scope, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			tg.globalConfig(t, swapConfig)
			project, story := "", ""
			if scope != "global" {
				project = "proj"
			}
			if scope == "story" {
				story = "01-story"
			}
			target, err := tg.g.resolveConfigScope(scope, project, story)
			if err != nil {
				t.Fatal(err)
			}
			if scope != "global" {
				if err := bootstrap.SaveConfig(target.path, bootstrap.Config{Style: "old"}); err != nil {
					t.Fatal(err)
				}
			}
			bodyA, err := os.ReadFile(target.path)
			if err != nil {
				t.Fatal(err)
			}
			versionA := fileVersion(target.path)
			passedMiddleware := make(chan struct{})
			runHandler := make(chan struct{})
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			// This dispatch seam pauses between the real beginFeedback and
			// putConfig methods. No production hook or storage mock is used.
			user := identity.User{Username: "admin", Role: identity.RoleAdmin}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				res := tg.g.resourceFor(r, user, r.URL.Query())
				out, finish, ok := tg.g.beginFeedback(w, r, user, identity.Session{}, res)
				if !ok {
					return
				}
				defer finish()
				close(passedMiddleware)
				select {
				case <-runHandler:
					tg.g.putConfig(out, r, user)
				case <-ctx.Done():
					http.Error(out, "test gate timed out", http.StatusGatewayTimeout)
				}
			}))
			defer server.Close()
			q := url.Values{"scope": {scope}, "project": {project}, "story": {story}}
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/config?"+q.Encode(), bytes.NewReader(bodyA))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("If-Match", `"`+versionA+`"`)
			type response struct {
				resp *http.Response
				err  error
			}
			done := make(chan response, 1)
			go func() {
				resp, err := server.Client().Do(req)
				done <- response{resp: resp, err: err}
			}()
			select {
			case <-passedMiddleware:
			case early := <-done:
				if early.err != nil {
					t.Fatal(early.err)
				}
				defer early.resp.Body.Close()
				data, _ := io.ReadAll(early.resp.Body)
				t.Fatalf("request rejected before dispatch gate: %d: %s", early.resp.StatusCode, data)
			case <-ctx.Done():
				t.Fatal("request never passed the original If-Match middleware")
			}
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConfigIfMatchBindsMiddlewareToHandlerSnapshot$")
			childHome := t.TempDir()
			child.Env = append(os.Environ(), "HOME="+childHome, "USERPROFILE="+childHome, "AU_IFMATCH_WRITER_PATH="+target.path)
			if output, err := child.CombinedOutput(); err != nil {
				t.Fatalf("cross-process writer B: %v: %s", err, output)
			}
			bodyB, err := os.ReadFile(target.path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(bodyA, bodyB) {
				t.Fatal("writer B did not change the persisted document")
			}
			close(runHandler)
			result := <-done
			if result.err != nil {
				t.Fatal(result.err)
			}
			defer result.resp.Body.Close()
			data, err := io.ReadAll(result.resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if result.resp.StatusCode != http.StatusConflict || !bytes.Contains(data, []byte(`"conflict":true`)) {
				t.Fatalf("stale A after middleware passed = %d: %s", result.resp.StatusCode, data)
			}
			after, err := os.ReadFile(target.path)
			if err != nil || !bytes.Equal(after, bodyB) {
				t.Fatalf("stale A overwrote writer B: %v", err)
			}
		})
	}
}

func readConfigMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func writeConfigMap(t *testing.T, path string, cfg map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
