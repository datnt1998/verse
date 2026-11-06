package library

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestTemplateImageModeChangesRemainEditable(t *testing.T) {
	for _, mode := range []string{ImagesFixed, ImagesPerMinutes, ImagesNone} {
		t.Run(mode, func(t *testing.T) {
			svc := testService(t, newMemStore())
			ctx := context.Background()
			template, err := svc.Create(ctx, "Switch image mode")
			if err != nil {
				t.Fatal(err)
			}
			recipe := template.Recipe
			// The form retains the other mode's last number when switching modes.
			recipe.Scenes.Images = domain.ImagesMode{Mode: mode, Count: 5, Minutes: 2}
			raw, err := json.Marshal(recipe)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := svc.Patch(ctx, template.ID, Patch{Revision: template.Revision, Recipe: raw})
			if err != nil {
				t.Fatal(err)
			}
			project, revision, err := svc.ReadEditorProject(ctx, template.ID)
			if err != nil {
				t.Fatal(err)
			}
			project["description"] = "Keep my authored edit"
			next, err := svc.WriteEditorProject(ctx, template.ID, project, revision)
			if err != nil {
				t.Fatalf("mode %s cannot be edited after a library save: %v", mode, err)
			}
			persisted, actual, err := svc.ReadEditorProject(ctx, template.ID)
			if err != nil || actual != next || persisted["description"] != "Keep my authored edit" {
				t.Fatalf("edit not retained: revision=%s err=%v", actual, err)
			}
			if next == revision || saved.Revision <= template.Revision {
				t.Fatal("successful edits did not advance revision")
			}
		})
	}
}
