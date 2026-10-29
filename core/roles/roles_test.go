package roles

import (
	"encoding/json"
	"reflect"
	"testing"
)

// File cấu hình của người dùng lưu đúng shape này: decode → encode lại phải
// cho ra cùng một JSON (so qua map[string]any), đảm bảo tag không trôi.
func TestRoleConfigRoundTrip(t *testing.T) {
	raw := `{
		"provider": "openai-codex",
		"model": "gpt-6-sol",
		"reasoning_effort": "high",
		"providers": {
			"openai-codex": {"type": "openai-codex"},
			"anthropic-oauth": {"type": "anthropic-oauth"}
		},
		"roles": {
			"architect": {"provider": "anthropic-oauth", "model": "claude-opus-4-8", "reasoning_effort": "xhigh"},
			"writer": {
				"provider": "openai-codex",
				"model": "gpt-6-luna",
				"fallbacks": [
					{"provider": "anthropic-oauth", "model": "claude-sonnet-4-6"},
					{"provider": "openai-codex", "model": "gpt-6-sol"}
				]
			}
		}
	}`

	var doc struct {
		Provider        string                  `json:"provider"`
		Model           string                  `json:"model"`
		ReasoningEffort string                  `json:"reasoning_effort,omitempty"`
		Providers       map[string]ProviderStub `json:"providers"`
		Roles           map[string]RoleConfig   `json:"roles"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	if err := json.Unmarshal([]byte(raw), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip khác:\n got: %s\nwant: %s", out, raw)
	}
}

// ProviderStub tối thiểu đứng vào ô providers của fixture trên — phần provider
// không nằm trong gói này, chỉ cần map JSON nguyên vẹn.
type ProviderStub map[string]any

// Một role chỉ provider khác mặc định phải resolve đúng provider đó và model
// của nó; role không cấu hình rơi về mặc định và explicit=false.
func TestResolveRoleOverrides(t *testing.T) {
	all := map[string]RoleConfig{
		"writer": {Provider: "anthropic-oauth", Model: "claude-opus-4-8"},
		"editor": {Provider: "", Model: "gpt-6-luna"},
	}

	provider, model, explicit := Resolve(all, "writer", "openai-codex", "gpt-6-sol")
	if provider != "anthropic-oauth" || model != "claude-opus-4-8" || !explicit {
		t.Fatalf("writer resolve = %s/%s explicit=%v", provider, model, explicit)
	}

	// Vế trống rơi về mặc định, vế còn lại vẫn ghi đè và explicit.
	provider, model, explicit = Resolve(all, "editor", "openai-codex", "gpt-6-sol")
	if provider != "openai-codex" || model != "gpt-6-luna" || !explicit {
		t.Fatalf("editor resolve = %s/%s explicit=%v", provider, model, explicit)
	}

	for _, role := range []string{"", "default", "unknown"} {
		provider, model, explicit = Resolve(all, role, "openai-codex", "gpt-6-sol")
		if provider != "openai-codex" || model != "gpt-6-sol" || explicit {
			t.Fatalf("role %q resolve = %s/%s explicit=%v, want default", role, provider, model, explicit)
		}
	}
}

// reasoning_effort: role-level ưu tiên hơn mặc định, rỗng thì rơi về mặc định.
func TestResolveReasoningEffort(t *testing.T) {
	all := map[string]RoleConfig{
		"writer": {ReasoningEffort: "xhigh"},
		"editor": {Provider: "p", Model: "m"},
	}
	if got := ResolveReasoningEffort(all, "writer", "medium"); got != "xhigh" {
		t.Fatalf("writer effort = %q", got)
	}
	for _, role := range []string{"", "default", "editor", "unknown"} {
		if got := ResolveReasoningEffort(all, role, "medium"); got != "medium" {
			t.Fatalf("role %q effort = %q, want default", role, got)
		}
	}
}
