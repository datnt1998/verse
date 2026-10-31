package images

import "testing"

func TestNarrationChangeInvalidatesPaidImageSelection(t *testing.T) {
	item := Item{ID: "scene", Kind: "scene", Narration: "before", Status: "approved"}
	n := 1
	item.CurrentVersion = &n
	item.Versions = []Version{{Version: n, SourceFingerprint: SourceFingerprint(item)}}
	item.Narration = "after"
	if Current(item) != nil {
		t.Fatal("changed narration retained the paid image selection")
	}
}

func TestRecipeChangeInvalidatesOnlyAffectedImageSelections(t *testing.T) {
	n := 1
	scene := Item{ID: "scene", Kind: "scene", Narration: "story", Status: "approved", CurrentVersion: &n}
	thumb := Item{ID: "thumbnail", Kind: "thumbnail", Narration: "title", Status: "approved", CurrentVersion: &n}
	for _, i := range []*Item{&scene, &thumb} {
		i.Versions = []Version{{Version: n, SourceFingerprint: SourceFingerprint(*i)}}
	}
	old := Manifest{Version: 1, Revision: 1, Style: "full", Thumbnail: "clean", Items: []Item{scene, thumb}}
	for _, tc := range []struct {
		name      string
		change    func(*Manifest)
		sceneKept bool
	}{{"style", func(m *Manifest) { m.Style = "cutout" }, false}, {"prompt", func(m *Manifest) { m.TemplatePrompt = "new art direction" }, false}, {"thumbnail", func(m *Manifest) { m.Thumbnail = "hook" }, true}} {
		t.Run(tc.name, func(t *testing.T) {
			fresh := Manifest{Version: 1, Revision: 1, Style: "full", Thumbnail: "clean", Items: []Item{{ID: "scene", Kind: "scene", Narration: "story", Status: "pending"}, {ID: "thumbnail", Kind: "thumbnail", Narration: "title", Status: "pending"}}}
			tc.change(&fresh)
			got := Reconcile(old, fresh)
			if (Current(got.Items[0]) != nil) != tc.sceneKept || Current(got.Items[1]) != nil {
				t.Fatalf("changed recipe retained stale choices: %+v", got.Items)
			}
			if len(got.Items[0].Versions) != 1 {
				t.Fatal("lost paid image history")
			}
		})
	}
}
