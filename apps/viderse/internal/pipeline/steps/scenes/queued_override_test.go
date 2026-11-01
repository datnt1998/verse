package scenes

import (
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
)

func TestQueuedNoImagesOverrideEnablesScenes(t *testing.T) {
	raw, err := json.Marshal(library.Snapshot{Images: domain.ImagesMode{Mode: library.ImagesNone}})
	if err != nil {
		t.Fatal(err)
	}
	enabled := false
	recipe, err := ResolveRecipeWithOverride(raw, false, &enabled, 600)
	if err != nil || recipe.ImagesMode != library.ImagesFixed || recipe.ImageCount != 6 {
		t.Fatalf("a no-image template re-enabled for one video draws 6 scene images: recipe=%+v err=%v", recipe, err)
	}
}

func TestReenabledImagesKeepTemplateImageCount(t *testing.T) {
	raw, err := json.Marshal(library.Snapshot{Images: domain.ImagesMode{Mode: library.ImagesFixed, Count: 3}})
	if err != nil {
		t.Fatal(err)
	}
	enabled := false
	recipe, err := ResolveRecipeWithOverride(raw, true, &enabled, 600)
	if err != nil || recipe.ImagesMode != library.ImagesFixed || recipe.ImageCount != 3 {
		t.Fatalf("recipe=%+v err=%v", recipe, err)
	}
}
