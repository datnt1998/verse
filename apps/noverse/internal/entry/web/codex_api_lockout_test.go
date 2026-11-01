package web

import (
	"encoding/json"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

func TestCodexAPIPutNormalizesFixedProtocol(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	admin := tg.login("admin", "admin-pass")
	for _, name := range []string{"openai-codex", "custom-codex"} {
		body := `{"providers":{"` + name + `":{"type":"openai-codex","api":"responses","models":[{"name":"gpt-6.1-sol"}]}}}`
		response := tg.do("PUT", "/api/config?scope=global", admin, body)
		if response.Code != 200 {
			t.Fatalf("save: %d %s", response.Code, response.Body.String())
		}
		saved, err := bootstrap.LoadConfigFile(filepath.Join(tg.home, ".ainovel", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		if saved.Providers[name].API != "" {
			t.Fatalf("fixed protocol persisted redundant api: %+v", saved.Providers[name])
		}
	}
}

func TestCodexAPIPutRejectsInvalidProviders(t *testing.T) {
	for _, provider := range []string{`{"type":"openai-codex","api":"chat"}`, `{"type":"xai-oauth","api":"responses"}`, `{"type":"anthropic","api":"chat"}`, `{"type":"not-a-provider"}`, `{"type":"openai","stream_idle_timeout":"bad"}`} {
		t.Run(provider, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			defer tg.g.Close()
			admin := tg.login("admin", "admin-pass")
			response := tg.do("PUT", "/api/config?scope=global", admin, `{"providers":{"broken":`+provider+`}}`)
			if response.Code != 400 || !strings.Contains(response.Body.String(), "broken") {
				t.Fatalf("invalid provider must return clear 400: %d %s", response.Code, response.Body.String())
			}
			if _, err := os.Stat(filepath.Join(tg.home, ".ainovel", "config.json")); !os.IsNotExist(err) {
				t.Fatalf("invalid config was persisted: %v", err)
			}
		})
	}
}

func TestCodexAPIUnusedInvalidProviderDoesNotLockWorker(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	tg.globalConfig(t, `{"provider":"xai-oauth","model":"grok-4.7","providers":{"xai-oauth":{},"broken":{"type":"xai-oauth","api":"chat"}}}`)
	admin := tg.login("admin", "admin-pass")
	installRealWorkerLauncher(t, tg.g)
	response := tg.do("GET", "/api/projects/proj/stories/01-story/snapshot", admin, "")
	if response.Code != 200 {
		t.Fatalf("snapshot: %d %s", response.Code, response.Body.String())
	}
	tg.g.mu.Lock()
	slot := tg.g.workers["proj/01-story"]
	tg.g.mu.Unlock()
	if slot == nil {
		t.Fatal("worker was not launched")
	}
	slot.mu.Lock()
	ready := slot.worker != nil && !slot.worker.noModel
	slot.mu.Unlock()
	if !ready {
		t.Fatal("unused invalid provider made worker read-only")
	}
	response = tg.do("GET", "/api/config?scope=global", admin, "")
	var body struct {
		ProviderErrors map[string]struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"providerErrors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.ProviderErrors["broken"].Message, "api") || body.ProviderErrors["broken"].Kind != "key" {
		t.Fatalf("missing provider warning: %s", response.Body.String())
	}
}

func TestCodexAPIReadOnlyReasonReachesGateway(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	tg.globalConfig(t, `{"provider":"xai-oauth","model":"grok-4.7","providers":{"xai-oauth":{"api":"chat"}}}`)
	admin := tg.login("admin", "admin-pass")
	installRealWorkerLauncher(t, tg.g)
	for _, request := range []struct{ method, path, body string }{{"GET", "snapshot", ""}, {"POST", "resume", `{}`}} {
		response := tg.do(request.method, "/api/projects/proj/stories/01-story/"+request.path, admin, request.body)
		if request.method == http.MethodPost && response.Code != 409 {
			t.Fatalf("guard: %d %s", response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "xai-oauth") || !strings.Contains(response.Body.String(), "api") {
			t.Fatalf("real reason was hidden: %s", response.Body.String())
		}
	}
}

func TestCodexAPIReferencedInvalidProviderBlocksModelReady(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, fallback := range []bool{false, true} {
		cfg := bootstrap.Config{OutputDir: t.TempDir(), Provider: "xai-oauth", ModelName: "grok-4.7", Providers: map[string]llmconfig.ProviderConfig{"xai-oauth": {}, "broken": {Type: "xai-oauth", API: "chat"}}}
		role := roles.RoleConfig{Provider: "broken", Model: "grok-4.7"}
		if fallback {
			role = roles.RoleConfig{Provider: "xai-oauth", Model: "grok-4.7", Fallbacks: []roles.ModelRef{{Provider: "broken", Model: "grok-4.7"}}}
		}
		cfg.Roles = map[string]roles.RoleConfig{"writer": role}
		h, err := host.New(cfg, assets.Bundle{}, host.WithUnconfiguredReads())
		if err != nil {
			t.Fatal(err)
		}
		if h.ModelReady() {
			t.Fatal("referenced invalid provider was ignored")
		}
		h.Close()
	}
}

func TestCodexAPIProviderProtocolIsGlobalOnly(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	tg.globalConfig(t, `{"providers":{"subscription":{"type":"openai-codex","models":[{"name":"gpt-6.1-sol"}]}}}`)
	admin := tg.login("admin", "admin-pass")
	// AU1: model list/provider protocol is app-wide. A story may not add or
	// change models any more; the old scoped save is rejected instead of being
	// silently dropped.
	response := tg.do("PUT", "/api/config?scope=story&project=proj&story=01-story", admin, `{"providers":{"subscription":{"api":"responses","models":[{"name":"gpt-6.1-sol","context_window":500000}]}}}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("scoped model list must be rejected: %d %s", response.Code, response.Body.String())
	}
	// Global save still normalizes the Codex protocol (fixed Responses).
	response = tg.do("PUT", "/api/config?scope=global", admin, `{"providers":{"subscription":{"type":"openai-codex","api":"responses","models":[{"name":"gpt-6.1-sol"}]}}}`)
	if response.Code != 200 {
		t.Fatalf("global provider save: %d %s", response.Code, response.Body.String())
	}
	saved, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if saved.Providers["subscription"].API != "" || saved.Providers["subscription"].Type != "openai-codex" {
		t.Fatalf("global save must normalize API: %+v", saved.Providers["subscription"])
	}
}
