package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestAgentInstructionsMergePreservesOtherAgentsAndDoesNotAlias(t *testing.T) {
	current := domain.AgentInstructions{"director": "old direction", "artist": "warm palette"}
	patch := domain.AgentInstructions{"director": "  new direction  ", "qa": "check continuity"}
	got, err := domain.MergeAgentInstructions(current, patch)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.AgentInstructions{"director": "new direction", "artist": "warm palette", "qa": "check continuity"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge=%+v", got)
	}
	got["artist"] = "mutated result"
	if current["artist"] != "warm palette" || current["director"] != "old direction" || patch["director"] != "  new direction  " {
		t.Fatal("merge aliases or mutates inputs")
	}
	deleted, err := domain.MergeAgentInstructions(current, domain.AgentInstructions{"director": " \t "})
	if err != nil || !reflect.DeepEqual(deleted, domain.AgentInstructions{"artist": "warm palette"}) {
		t.Fatalf("delete=%+v err=%v", deleted, err)
	}
}

func TestAgentInstructionsValidateWholePatchBeforeReturningMergedState(t *testing.T) {
	current := domain.AgentInstructions{"artist": "keep"}
	for _, patch := range []domain.AgentInstructions{
		{"director": "valid", "unknown": "invalid"},
		{"qa": strings.Repeat("ệ", domain.MaxAgentInstructionRunes+1)},
	} {
		got, err := domain.MergeAgentInstructions(current, patch)
		if err == nil || got != nil || current["artist"] != "keep" {
			t.Fatalf("invalid patch partially applied: %+v %v", got, err)
		}
	}
	text := strings.Repeat("ệ", domain.MaxAgentInstructionRunes)
	got, err := domain.MergeAgentInstructions(current, domain.AgentInstructions{"qa": text})
	if err != nil || got["qa"] != text {
		t.Fatalf("rune boundary rejected: %v", err)
	}
}
