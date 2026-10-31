package web

import (
	"math"
	"testing"
)

func TestRulesEffectiveMetadataBothBackends(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := newDocsFixture(t, backend)
			for _, path := range []string{
				"/api/rules/effective?scope=project&project=" + f.project,
				"/api/rules/effective?scope=story&project=" + f.project + "&story=" + f.story,
				"/api/projects/" + f.project + "/stories/" + f.story + "/rules/effective",
			} {
				w := f.request("GET", path, f.admin, "", "")
				docsStatus(t, w, 200)
				body := decodeResp(t, w)
				words, ok := body["fatigueWords"].([]any)
				if !ok || len(words) != 16 {
					t.Fatalf("%s fatigueWords: %v", path, body["fatigueWords"])
				}
				for _, raw := range words {
					word := raw.(map[string]any)
					if word["word"] == "" || word["perChapter"] != math.Round(word["per1000"].(float64)*1.5) {
						t.Fatalf("invalid reference chapter threshold: %v", word)
					}
					if source := word["source"]; source != "default" && source != "pack" {
						t.Fatalf("new default provenance: %v", word)
					}
				}
				forbidden, ok := body["forbiddenDefaults"].([]any)
				if !ok || len(forbidden) != 4 {
					t.Fatalf("immutable defaults: %v", body["forbiddenDefaults"])
				}
			}
		})
	}
}
