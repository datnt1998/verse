package host

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

func TestModelThinkingLevelsProbeConfiguredModel(t *testing.T) {
	cfg := bootstrap.Config{Providers: map[string]bootstrap.ProviderConfig{
		"openrouter": {APIKey: "sk-test", Models: []bootstrap.ModelConfig{{Name: "openai/gpt-5"}}},
	}}
	levels := modelThinkingLevels(cfg, "openrouter", "openai/gpt-5")
	if len(levels) < 2 || levels[0] != "" {
		t.Fatalf("levels = %q, want auto first plus concrete levels", levels)
	}
	if !strings.Contains(strings.Join(levels, ","), "high") {
		t.Fatalf("levels = %q, want high available", levels)
	}
}

func TestModelThinkingLevelsUnknownProviderFallsBackToAuto(t *testing.T) {
	levels := modelThinkingLevels(bootstrap.Config{}, "missing", "x")
	if len(levels) != 1 || levels[0] != "" {
		t.Fatalf("levels = %q, want only auto", levels)
	}
}
