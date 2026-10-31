package images

import (
	"context"
	"fmt"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestSceneOverridePricesOnlyFourDrawnOfSix(t *testing.T) {
	s, store := testService()
	store.m.Items = nil
	for i := range 6 {
		item := model.Item{ID: fmt.Sprintf("scene-%d", i), Kind: "scene"}
		if i < 4 {
			item.Versions = []model.Version{{Version: 1}}
		}
		store.m.Items = append(store.m.Items, item)
	}
	s.Prompts = promptCatalog{}
	s.QuotaLeft = func(context.Context) (int, error) { return 12, nil }
	id := "new-scene"
	plan, err := s.PlanOverrides(context.Background(), "job", OverrideInput{ScenePromptID: OptionalString{Set: true, Value: &id}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Redraw.Scenes != 4 || plan.Redraw.Cover || plan.Cost.Images != 4 || plan.Rebuild || plan.QuotaLeft != 12 {
		t.Fatalf("unexpected pricing: %+v", plan)
	}
	if _, err := s.ApplyOverrides(context.Background(), "job", OverrideInput{ScenePromptID: OptionalString{Set: true, Value: &id}, ExpectedRevision: new(2)}); err == nil {
		t.Fatal("stale revision accepted")
	} else if clone, ok := err.(*domain.CloneError); !ok || clone.Code != domain.CloneErrConflict {
		t.Fatalf("unexpected stale error: %v", err)
	}
}

func TestPromptChangeClearsReviewOrigin(t *testing.T) {
	s, store := testService()
	store.m.ReviewOrigin = true
	s.Prompts = promptCatalog{}
	s.QuotaLeft = func(context.Context) (int, error) { return 10, nil }
	id := "new-scene"
	l, err := s.load(context.Background(), "job", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := s.prepareOverrides(context.Background(), l, OverrideInput{ScenePromptID: OptionalString{Set: true, Value: &id}})
	if err != nil {
		t.Fatal(err)
	}
	if next.ReviewOrigin {
		t.Fatal("full prompt rerun retained review origin")
	}
}
