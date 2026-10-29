package assets

import (
	"os"
	"strings"
	"testing"
)

func TestInterventionDiagnosticRoutingGolden(t *testing.T) {
	prompt := loadPrompts().ArbiterIntervention
	for _, phrase := range []string{"facts.diagnostic_context", "§N", "Giữ nguyên", "Góp ý sửa", "read_chapter", "source=final"} {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("intervention prompt missing diagnostic routing contract %q", phrase)
		}
	}
	const path = "testdata/intervention-golden.md"
	if os.Getenv("UPDATE_INTERVENTION_GOLDEN") == "1" {
		writeFile(t, path, prompt)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != prompt {
		t.Fatal("intervention prompt changed; update with UPDATE_INTERVENTION_GOLDEN=1")
	}
}
