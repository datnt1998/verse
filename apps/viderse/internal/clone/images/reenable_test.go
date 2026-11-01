package images

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

// A template that draws no images keeps one span per video. Turning images
// back on for a video must plan six scene images spread over the duration,
// not the single image that one span would give.
func reenableStore(t *testing.T, templateImages string) (*Service, *editStore) {
	t.Helper()
	s, store := testService()
	store.template = json.RawMessage(`{"images":` + templateImages + `}`)
	store.m.ImageMode = "none"
	store.m.Items = store.m.Items[1:]
	store.sceneDoc = json.RawMessage(`{"version":1,"revision":1,"durationSec":240,"recipe":{"imagesMode":"none"}}`)
	s.Prompts = promptCatalog{}
	s.QuotaLeft = func(context.Context) (int, error) { return 40, nil }
	return s, store
}

func TestReenablingImagesOnNoImageTemplatePlansSixSceneImages(t *testing.T) {
	s, _ := reenableStore(t, `{"mode":"none"}`)
	off := false
	plan, err := s.PlanOverrides(context.Background(), "job", OverrideInput{NoImages: &off})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Enables || plan.Redraw.Scenes != 6 || plan.Cost.Images != 6 {
		t.Fatalf("want 6 scene images announced before applying, got %+v", plan)
	}
}

func TestReenablingImagesRecutsScenesFromTheScenesStep(t *testing.T) {
	s, store := reenableStore(t, `{"mode":"none"}`)
	off, six, rev := false, 6, 3
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	if _, err := s.ApplyOverrides(context.Background(), "job", OverrideInput{ExpectedRevision: &rev, NoImages: &off, ConfirmCount: &six}); err != nil {
		t.Fatal(err)
	}
	if store.change.InvalidateFrom != domain.CloneScenes || !store.change.Rerun {
		t.Fatalf("scenes must be cut again, got %+v", store.change)
	}
	for _, item := range store.m.Items {
		if item.Kind == "scene" {
			t.Fatalf("stale single-span scene kept: %+v", item)
		}
	}
}

func TestReenablingImagesKeepsTemplateImageSettings(t *testing.T) {
	s, store := reenableStore(t, `{"mode":"fixed","count":3}`)
	store.sceneDoc = json.RawMessage(`{"version":1,"revision":1,"durationSec":240,"recipe":{"imagesMode":"fixed","imageCount":3},"scenes":[{"id":"sc-001"},{"id":"sc-002"},{"id":"sc-003"}]}`)
	off := false
	plan, err := s.PlanOverrides(context.Background(), "job", OverrideInput{NoImages: &off})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Redraw.Scenes == 6 {
		t.Fatalf("template with its own image count must not be forced to 6: %+v", plan)
	}
}
