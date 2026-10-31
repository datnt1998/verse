package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
)

func TestRenderScopedMediaUsesBuildMediaIdentities(t *testing.T) {
	const pid = "clone-47c2bbb3-af91-4de4-aebe-2b211c6ed2f2-g1"
	const mid = "media-7a695f5074553cfae69f0f8f34a04b55"
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	repo := renderHTTPReadRepository{files: files}
	edit := &editor.Service{Store: editor.Store{Media: files, Repository: repo}, Repository: repo}
	project, err := editor.New(pid, "Build fixture", 1920, 1080, 30, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	project["mediaItems"] = []editor.Object{{"id": mid}}
	project["timeline"] = editor.Object{"items": []editor.Object{{"id": "voice", "type": "audio", "mediaId": mid, "trackId": "audio", "from": 0, "durationInFrames": 180}}}
	renderHTTPPutJSON(t, files, "editor/projects/"+pid+"/project.json", project)
	renderHTTPPutJSON(t, files, "editor/media/"+mid+"/metadata.json", editor.Object{"id": mid, "fileName": "voice.m4a", "mime": "audio/mp4", "size": 5, "contentHash": "sha256:fixture", "sourceFile": "source.m4a"})
	files.Put("editor/media/"+mid+"/source.m4a", []byte("audio"))
	store := &render.Store{Root: t.TempDir()}
	request, err := store.CreateExport(pid, "")
	if err != nil {
		t.Fatal(err)
	}
	tg := newTestGateway(t)
	MountRender(tg.g, &render.Service{Store: store, Editor: edit}, context.Background())
	base := "/api/render/" + request.ID + "/media/"
	w := renderTokenHTTP(tg, http.MethodGet, base+mid, request.Token, false, "", "")
	renderHTTPStatus(t, w, http.StatusOK)
	if w.Body.String() != "audio" {
		t.Fatalf("wrong source bytes: %q", w.Body.String())
	}
	// The UUID inside a clone project name is not a media reference.
	w = renderTokenHTTP(tg, http.MethodGet, base+"47c2bbb3-af91-4de4-aebe-2b211c6ed2f2", request.Token, false, "", "")
	renderHTTPStatus(t, w, http.StatusUnauthorized)
}
