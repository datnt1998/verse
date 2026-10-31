package bootstrap

import (
	"encoding/json"
	"github.com/datnt1998/verse/core/llmconfig"
	"os"
	"testing"
)

func TestCodexAPIUserConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data, err := os.ReadFile("testdata/codex-api-lockout.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("exact user config must validate: %v", err)
	}
	if _, err := NewModelSet(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestCodexAPIFixedProtocol(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"openai-codex", "subscription"} {
		for _, api := range []string{"", "responses", "chat", "invalid"} {
			t.Run(name+"/"+api, func(t *testing.T) {
				pc := llmconfig.ProviderConfig{Type: "openai-codex", API: api}
				cfg := Config{Provider: name, ModelName: "gpt-6.1-sol", Providers: map[string]llmconfig.ProviderConfig{name: pc}}
				wantOK := api == "" || api == "responses"
				if err := cfg.ValidateBase(); (err == nil) != wantOK {
					t.Errorf("validation: %v, wantOK=%v", err, wantOK)
				}
				if _, err := NewModelSet(cfg); (err == nil) != wantOK {
					t.Errorf("construction: %v, wantOK=%v", err, wantOK)
				}
			})
		}
	}
	for _, api := range []string{"chat", "responses"} {
		if err := ValidateProvider("xai-oauth", llmconfig.ProviderConfig{API: api}); err == nil {
			t.Fatalf("xAI must reject api %q", api)
		}
	}
}
