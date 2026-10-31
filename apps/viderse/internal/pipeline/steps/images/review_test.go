package images

import (
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

func reviewFixture() Manifest {
	n := 1
	a := Item{ID: "scene-1", Kind: "scene", Narration: "A scene", Status: "review", ReviewVersion: &n}
	b := Item{ID: "thumbnail", Kind: "thumbnail", Status: "review", ReviewVersion: &n}
	for _, i := range []*Item{&a, &b} {
		i.Versions = []Version{{Version: 1, SourceFingerprint: SourceFingerprint(*i)}}
	}
	return Manifest{Version: 1, Revision: 1, Style: "full", Items: []Item{a, b}}
}
func TestReviewRejectsBusyAndStaleVersions(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(*Item)
	}{
		{"running", domain.CloneErrConflict, func(i *Item) { i.Status = "running" }},
		{"inFlight", domain.CloneErrConflict, func(i *Item) { i.InFlight = &Flight{} }},
		{"stale", domain.CloneErrStale, func(i *Item) { i.Narration = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := reviewFixture()
			tc.mutate(&m.Items[0])
			_, err := Review(m, "scene-1", "approve", "", nil)
			var ce *domain.CloneError
			if !errors.As(err, &ce) || ce.Code != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
}
func TestCutoutApprovalRequiresSubjectButNotForCover(t *testing.T) {
	m := reviewFixture()
	m.Style = "cutout"
	if _, err := Review(m, "scene-1", "approve", "", nil); err == nil {
		t.Fatal("approved scene without cutout")
	}
	m.Items[0].Versions[0].Cutout = &scenes.FileRef{Path: "subject.png"}
	if _, err := Review(m, "scene-1", "approve", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Review(m, "thumbnail", "approve", "", nil); err != nil {
		t.Fatal(err)
	}
}
func TestReviewSkipClearsChosenAndCannotApproveEmptyManifest(t *testing.T) {
	m := reviewFixture()
	other := m.Items[0]
	other.ID = "scene-2"
	m.Items = append(m.Items, other)
	n := 1
	m.Items[0].CurrentVersion = &n
	got, err := Review(m, "scene-1", "skip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Items[0].CurrentVersion != nil || got.Items[0].ReviewVersion != nil {
		t.Fatal("skipped scene retained choice")
	}
	if ImagesApproved(Manifest{}) || ImagesReady(Manifest{}) {
		t.Fatal("empty manifest passed review gate")
	}
}
func TestReviewRetainsDirectionAndRejectsNoImageRedraw(t *testing.T) {
	m := reviewFixture()
	got, err := Review(m, "scene-1", "regenerate", "  warmer light  ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Items[0].Direction != "warmer light" || got.Items[0].Status != "pending" || len(got.Items[0].Versions) != 1 {
		t.Fatalf("bad redraw: %+v", got.Items[0])
	}
	m.ImageMode = "none"
	if _, err := Review(m, "thumbnail", "regenerate", "", nil); err == nil {
		t.Fatal("no-image recipe queued paid redraw")
	}
}
func TestReconcilePreservesVideoOverridesAcrossSceneEdits(t *testing.T) {
	old := reviewFixture()
	old.Overrides = &Overrides{ScenePromptID: "custom-scene", ScenePromptSet: true}
	old.TemplatePrompt = "video direction"
	fresh := reviewFixture()
	fresh.SceneRevision = 2
	fresh.TemplatePrompt = "template direction"
	got := Reconcile(old, fresh)
	if got.Overrides == nil || got.Overrides.ScenePromptID != "custom-scene" || got.TemplatePrompt != "video direction" {
		t.Fatalf("lost video override: %+v", got)
	}
}
