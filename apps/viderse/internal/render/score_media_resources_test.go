package render

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/platform/blobstore"
)

type scoreMediaResources struct {
	editor.Repository
	resources map[string]editor.Resource
}

func (r scoreMediaResources) Get(_ context.Context, key string) (editor.Resource, error) {
	resource, ok := r.resources[key]
	if !ok {
		return editor.Resource{}, blobstore.ErrNotFound
	}
	return resource, nil
}

func TestQualityAcceptsCurrentEditorMediaWithoutLegacyAliases(t *testing.T) {
	f := newScorerFixture(t)
	resources := scoreMediaResources{resources: map[string]editor.Resource{}}
	for _, id := range []string{"image-a", "image-b", "source-audio"} {
		path := "revisions/" + id + ".json"
		f.putJSON(path, editor.Object{"id": id, "contentHash": "sha256:test"})
		delete(f.media.files, "editor/media/"+id+"/metadata.json")
		resources.resources["media/"+id] = editor.Resource{Key: "media/" + id, Path: path}
	}
	f.input.Media = artifactMedia{Base: f.media, Resources: resources}
	score := f.score()
	if !score.Passed || score.Total != 100 || len(score.Blockers) != 0 {
		t.Fatalf("valid current editor media blocked export: %+v", score)
	}

	// Removing the authoritative pointer must still block the same timeline;
	// stale legacy metadata must not resurrect deleted media.
	delete(resources.resources, "media/image-a")
	f.putJSON("editor/media/image-a/metadata.json", editor.Object{"id": "image-a"})
	score = f.score()
	for _, blocker := range score.Blockers {
		if blocker.ID == "media" {
			return
		}
	}
	t.Fatal("deleted current media was accepted through a stale legacy alias")
}
