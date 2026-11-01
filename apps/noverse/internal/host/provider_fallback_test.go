package host

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

// AU4: automatic fallback must use a DIFFERENT provider and announce itself so
// the writing banner can say which model is writing and why.
func TestActivateProviderFallbackUsesDifferentProviderAndEmitsEvent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := bootstrap.Config{
		Provider: "openai-codex", ModelName: "gpt-6-luna",
		Providers: map[string]llmconfig.ProviderConfig{
			"openai-codex": {Type: "openai", APIKey: "a", Models: []llmconfig.ModelConfig{{Name: "gpt-6-luna"}}},
			"gemini":       {Type: "gemini", APIKey: "b", Models: []llmconfig.ModelConfig{{Name: "gemini-2.5-pro"}}},
		},
		Roles: map[string]roles.RoleConfig{
			"writer": {
				Provider: "openai-codex", Model: "gpt-6-luna",
				Fallbacks: []roles.ModelRef{{Provider: "gemini", Model: "gemini-2.5-pro"}},
			},
		},
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := &Host{cfg: cfg, models: models, events: make(chan Event, 4)}

	swap, ok := h.activateProviderFallback("writer", "openai-codex", providerFailureExpired)
	if !ok {
		t.Fatal("fallback should activate")
	}
	if swap.FromProvider != "openai-codex" || swap.ToProvider != "gemini" || swap.ToModel != "gemini-2.5-pro" {
		t.Fatalf("swap = %+v", swap)
	}
	provider, model, _ := models.CurrentSelection("writer")
	if provider != "gemini" || model != "gemini-2.5-pro" {
		t.Fatalf("writer selection = %s/%s", provider, model)
	}
	select {
	case ev := <-h.events:
		if ev.Kind != "fallback.active" || ev.FromProvider != "openai-codex" || ev.Provider != "gemini" || ev.Reason != "expired" {
			t.Fatalf("event = %+v", ev)
		}
	default:
		t.Fatal("missing fallback.active event")
	}
	snap := h.fallbackSnapshot()
	if snap == nil || !snap.Active || snap.Provider != "gemini" || snap.FromProvider != "openai-codex" {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestProviderFallbackRejectsSameProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := bootstrap.Config{
		Provider: "openai-codex", ModelName: "gpt-6-luna",
		Providers: map[string]llmconfig.ProviderConfig{
			"openai-codex": {Type: "openai", APIKey: "a", Models: []llmconfig.ModelConfig{{Name: "gpt-6-luna"}, {Name: "gpt-6-mini"}}},
		},
		Roles: map[string]roles.RoleConfig{
			"writer": {
				Provider: "openai-codex", Model: "gpt-6-luna",
				Fallbacks: []roles.ModelRef{{Provider: "openai-codex", Model: "gpt-6-mini"}},
			},
		},
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := &Host{cfg: cfg, models: models, events: make(chan Event, 4)}
	if _, ok := h.activateProviderFallback("writer", "openai-codex", providerFailureQuota); ok {
		t.Fatal("same-provider fallback must not activate")
	}
}
