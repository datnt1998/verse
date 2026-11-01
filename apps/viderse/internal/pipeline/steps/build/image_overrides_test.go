package build

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

func TestBuildHonorsVideoImageOverridesWithoutChangingTemplate(t *testing.T) {
	_, s, m := fixture(t)
	doc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	manifest := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	manifest.TemplatePrompt = "video-only direction"
	manifest.Overrides = &images.Overrides{ScenePromptSet: true, ScenePromptID: "video-prompt"}
	if _, _, err := buildImageVersions(manifest, doc, s.Job.Source.Title); err != nil {
		t.Fatalf("video scene prompt rejected: %v", err)
	}
	manifest.Overrides = nil
	if _, _, err := buildImageVersions(manifest, doc, s.Job.Source.Title); err == nil {
		t.Fatal("unowned template prompt drift accepted")
	}
}
func TestBuildHonorsVideoNoImagesOverride(t *testing.T) {
	_, s, m := fixture(t)
	doc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	manifest := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	none := true
	manifest.ImageMode = "none"
	manifest.Overrides = &images.Overrides{NoImages: &none}
	manifest.Items = manifest.Items[2:]
	versions, _, err := buildImageVersions(manifest, doc, s.Job.Source.Title)
	if err != nil || len(versions) != 0 {
		t.Fatalf("video no-images override rejected: %v %v", versions, err)
	}
}

// A video can turn images back on for a template that draws none: the scenes
// step records six fixed images, and the build must accept that recipe.
func TestBuildAcceptsSceneRecipeFromReenabledImages(t *testing.T) {
	_, s, m := fixture(t)
	snap := fixtureTemplate(t, s)
	snap.Images = domain.ImagesMode{Mode: library.ImagesNone}
	s.Job.TemplateSnapshot = marshalFixture(t, snap)
	doc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	doc.Recipe.ImagesMode, doc.Recipe.ImageCount = library.ImagesFixed, 6
	enabled := false
	if err := validateBuildRecipe(s, doc, &enabled); err != nil {
		t.Fatalf("re-enabled recipe rejected: %v", err)
	}
	if err := validateBuildRecipe(s, doc, nil); err == nil {
		t.Fatal("six images recorded for a no-image template without a video override accepted")
	}
}
