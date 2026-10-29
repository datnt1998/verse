package bootstrap

import "testing"

func TestApplyContextWindowsKeepsConfigOwnership(t *testing.T) {
	original := Config{Provider: "one", ModelName: "old", Providers: map[string]ProviderConfig{"one": {APIKey: "secret", Models: []ModelConfig{{Name: "old", ContextWindow: 128000}}}}}
	source := Config{Providers: map[string]ProviderConfig{"one": {Models: []ModelConfig{{Name: "old", ContextWindow: 256000}}}}}
	updated := original
	updated.ApplyContextWindows(source)
	if original.Providers["one"].Models[0].ContextWindow != 128000 {
		t.Fatal("context refresh mutated config shared with another lock")
	}
	ms := &ModelSet{config: original}
	ms.ApplyContextWindows(source)
	if window, origin := ms.ResolveContextWindow("one", "old"); window != 256000 || origin != CtxWindowModelConfig {
		t.Fatalf("runtime window = %d %s", window, origin)
	}
	if ms.config.Provider != "one" || ms.config.ModelName != "old" || ms.config.Providers["one"].APIKey != "secret" {
		t.Fatal("context refresh changed selection or credentials")
	}
	ms.ApplyContextWindows(Config{})
	if _, origin := ms.ResolveContextWindow("one", "old"); origin != CtxWindowDefault {
		t.Fatalf("cleared override source = %s", origin)
	}
}
