package llm

import "testing"

func TestParseThinkingLevelAcceptsMinimal(t *testing.T) {
	level, err := ParseThinkingLevel("minimal")
	if err != nil {
		t.Fatalf("minimal must be accepted: %v", err)
	}
	if level != ThinkingMinimal {
		t.Fatalf("level = %q, want minimal", level)
	}
	if _, err := ParseThinkingLevel("min"); err == nil {
		t.Fatal("unknown level must still fail")
	}
}

// AU2: the vocabulary has eight labels, but a model only offers the levels it
// supports. Minimal is one of them, not a global default.
func TestThinkingPolicyMinimalIsPerModel(t *testing.T) {
	exact := ThinkingPolicyFromCapabilities(Capabilities{Thinking: ThinkingCapabilities{
		Supported: SupportYes,
		Efforts:   []ThinkingLevel{ThinkingMinimal, ThinkingLow},
	}})
	if !exact.Allows(ThinkingMinimal) || !exact.Allows(ThinkingLow) {
		t.Fatalf("exact model must allow minimal+low: %+v", exact.Available)
	}
	if exact.Allows(ThinkingHigh) {
		t.Fatalf("exact model must not invent high: %+v", exact.Available)
	}
	baseline := ThinkingPolicyFromCapabilities(Capabilities{ProviderBaseline: true, Thinking: ThinkingCapabilities{Supported: SupportYes}})
	if !baseline.Allows(ThinkingMinimal) {
		t.Fatalf("baseline model with thinking must offer minimal: %+v", baseline.Available)
	}
	noThinking := ThinkingPolicyFromCapabilities(Capabilities{Thinking: ThinkingCapabilities{Supported: SupportNo}})
	if noThinking.Allows(ThinkingMinimal) || len(noThinking.Available) != 1 || noThinking.Available[0] != ThinkingAuto {
		t.Fatalf("model without thinking must offer only auto: %+v", noThinking.Available)
	}
}
