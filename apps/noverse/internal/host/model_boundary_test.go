package host

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

// An unreachable gateway reload must not pin a running/paused story to its old
// provider. The next worker boundary reads the committed global selection;
// unchanged turns do not reconstruct clients or discard a runtime fallback.
func TestWorkerBoundaryReloadsCommittedSelectionWithoutGateway(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := bootstrap.Config{
		Provider: "a", ModelName: "old",
		Providers: map[string]llmconfig.ProviderConfig{
			"a": {Type: "openai", APIKey: "a", Models: []llmconfig.ModelConfig{{Name: "old"}}},
			"b": {Type: "openai", APIKey: "b", Models: []llmconfig.ModelConfig{{Name: "replacement-model"}}},
		},
	}
	path := bootstrap.DefaultConfigPath()
	if err := bootstrap.SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loads := 0
	h := &Host{cfg: cfg, models: models, configLoader: func() (bootstrap.Config, error) { loads++; return bootstrap.LoadConfigFile(path) }}
	workerModel := models.ForRole("writer")
	if err := h.refreshModelSelectionForTurn(); err != nil {
		t.Fatal(err)
	}
	cfg.Provider = "b"
	cfg.ModelName = "replacement-model"
	cfg.Roles = map[string]roles.RoleConfig{"writer": {Provider: "b", Model: "replacement-model"}}
	if err := bootstrap.SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	// No immediate ReloadModelSelection call: this is the failed delivery path.
	if err := h.refreshModelSelectionForTurn(); err != nil {
		t.Fatal(err)
	}
	if p, m, _ := h.CurrentModelSelection("writer"); p != "b" || m != "replacement-model" {
		t.Fatalf("next turn kept stale selection: %s/%s", p, m)
	}
	if workerModel != models.ForRole("writer") {
		t.Fatal("assembled worker model address changed")
	}
	if err := h.refreshModelSelectionForTurn(); err != nil {
		t.Fatal(err)
	}
	if loads != 2 {
		t.Fatalf("unchanged boundary rebuilt clients: loads=%d", loads)
	}
	// Removing the role override must reset that same assembled slot.
	cfg.Provider, cfg.ModelName, cfg.Roles = "a", "old", nil
	if err := bootstrap.SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	if err := h.refreshModelSelectionForTurn(); err != nil {
		t.Fatal(err)
	}
	if p, m, explicit := h.CurrentModelSelection("writer"); p != "a" || m != "old" || explicit {
		t.Fatalf("removed override did not restore inheritance: %s/%s (%v)", p, m, explicit)
	}
	if workerModel != models.ForRole("writer") {
		t.Fatal("removing override replaced the assembled worker slot")
	}
	if err := models.Swap("default", "b", "replacement-model"); err != nil {
		t.Fatal(err)
	}
	if p, m, _ := models.CurrentSelection("writer"); p != "b" || m != "replacement-model" {
		t.Fatalf("default swap left inherited slot stale: %s/%s", p, m)
	}
}
