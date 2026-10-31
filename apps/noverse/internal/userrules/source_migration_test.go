package userrules

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

func TestHistoricalGlobalSourcesBecomeProjectAndSystemDefaults(t *testing.T) {
	original := rules.BuildSnapshot([]rules.Candidate{
		rules.SystemDefaults(),
		{Source: "global:tone.md", Preferences: "Luật cũ"},
		{Source: "global:lessons", Preferences: "Bài học đã duyệt"},
	})
	migrated := migratedRuleSources(&original)
	if strings.Contains(strings.Join(migrated.Sources, ","), "global:") || strings.Contains(migrated.Preferences, "[global:") {
		t.Fatalf("retired source exposed: %+v", migrated)
	}
	if !strings.Contains(migrated.Preferences, "[project:tone.md]") || !strings.Contains(migrated.Preferences, "[default:lessons]") || !strings.Contains(migrated.Preferences, "Bài học đã duyệt") {
		t.Fatalf("source migration lost approved content: %+v", migrated)
	}
	if !strings.Contains(original.Preferences, "[global:tone.md]") {
		t.Fatal("read changed the persisted chapter snapshot")
	}
}
