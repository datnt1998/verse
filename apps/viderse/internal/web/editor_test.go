package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

func editorHTTPFixture(t *testing.T) editorHTTP {
	t.Helper()
	_, pool := planFixture(t)
	repository := postgres.NewEditorRepository(pool)
	blobs := postgres.NewBlobRepository(pool)
	_, err := blobs.Initialize(context.Background(), media.Descriptor{ID: "editor-http", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	files := media.New(blobs, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	staging := t.TempDir()
	s := &editor.Service{Store: editor.Store{Media: files, Repository: repository, StagingDir: staging}, Media: files, Repository: repository, StagingDir: staging}
	s.Store.Templates = library.New(library.Deps{Store: postgres.NewLibraryStore(pool), Media: files})
	return editorHTTP{service: s, jobs: editor.NewJobQueue(editor.DefaultJobConcurrency)}
}
func editorRequest(h editorHTTP, method, path, body, revision string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if revision != "" {
		r.Header.Set("If-Match", strconv.Quote(revision))
	}
	w := httptest.NewRecorder()
	h.serve(w, r)
	return w
}
func editorObject(t *testing.T, w *httptest.ResponseRecorder) editor.Object {
	t.Helper()
	var value editor.Object
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatalf("status=%d body=%s err=%v", w.Code, w.Body.String(), err)
	}
	return value
}

// Project reads carry an opaque application revision independently of ETag.
// Quoted If-Match saves preserve the real stale-write HTTP 409 contract.
func TestEditorHTTPProjectWireAndRevisionConflict(t *testing.T) {
	h := editorHTTPFixture(t)
	baseline := editorRequest(h, "GET", "/api/editor/projects", "", "")
	initialCount := len(editorObject(t, baseline)["projects"].([]any))
	create := editorRequest(h, "POST", "/api/editor/projects", `{"name":"HTTP project","template":"youtube-1080p"}`, "")
	if create.Code != 201 {
		t.Fatalf("create=%d %s", create.Code, create.Body.String())
	}
	value := editorObject(t, create)
	p := value["project"].(map[string]any)
	revision := value["revision"].(string)
	id := p["id"].(string)
	load := editorRequest(h, "GET", "/api/editor/projects/"+id, "", "")
	if load.Code != 200 || editorObject(t, load)["name"] != "HTTP project" || load.Header().Get("ETag") != strconv.Quote(revision) || load.Header().Get("X-Editor-Revision") != revision {
		t.Fatalf("load=%d %s %v", load.Code, load.Body.String(), load.Header())
	}
	save := editorRequest(h, "PUT", "/api/editor/projects/"+id, `{"name":"Changed","metadata":{"width":1920,"height":1080,"fps":30}}`, revision)
	if save.Code != 200 {
		t.Fatalf("save=%d %s", save.Code, save.Body.String())
	}
	stale := editorRequest(h, "PATCH", "/api/editor/projects/"+id, `{"name":"Stale"}`, revision)
	if stale.Code != 409 || editorObject(t, stale)["code"] != "revision_conflict" {
		t.Fatalf("stale=%d %s", stale.Code, stale.Body.String())
	}
	missing := editorRequest(h, "PUT", "/api/editor/projects/"+id, `{"name":"No revision"}`, "")
	if missing.Code != 400 {
		t.Fatalf("missing revision=%d %s", missing.Code, missing.Body.String())
	}
	malformed := editorRequest(h, "PUT", "/api/editor/projects/"+id, `{broken`, revision)
	if malformed.Code != 400 {
		t.Fatalf("JSON=%d", malformed.Code)
	}
	list := editorRequest(h, "GET", "/api/editor/projects", "", "")
	if list.Code != 200 || len(editorObject(t, list)["projects"].([]any)) != initialCount+1 {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
}

func TestEditorHTTPAuxiliaryRevisionHeaders(t *testing.T) {
	h := editorHTTPFixture(t)
	create := editorRequest(h, http.MethodPost, "/api/editor/projects", `{"name":"Revision headers"}`, "")
	if create.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", create.Code, create.Body.String())
	}
	id := editorObject(t, create)["project"].(map[string]any)["id"].(string)
	for _, path := range []string{
		"/api/editor/projects/" + id + "/sidecars/animation-presets",
		"/api/editor/projects/" + id + "/sidecars/render-queue",
		"/api/editor/workspace/effect-presets",
	} {
		t.Run(path, func(t *testing.T) {
			load := editorRequest(h, http.MethodGet, path, "", "")
			if load.Code != http.StatusOK || load.Header().Get("X-Editor-Revision") != "empty" || load.Header().Get("ETag") != `"empty"` {
				t.Fatalf("empty load=%d %s %v", load.Code, load.Body.String(), load.Header())
			}
			save := editorRequest(h, http.MethodPut, path, `{"version":1,"presets":[]}`, load.Header().Get("X-Editor-Revision"))
			if save.Code != http.StatusOK {
				t.Fatalf("save=%d %s", save.Code, save.Body.String())
			}
			revision := editorObject(t, save)["revision"].(string)
			loaded := editorRequest(h, http.MethodGet, path, "", "")
			if loaded.Code != http.StatusOK || loaded.Header().Get("X-Editor-Revision") != revision || loaded.Header().Get("ETag") != strconv.Quote(revision) || editorObject(t, loaded)["version"] != float64(1) {
				t.Fatalf("load=%d %s %v", loaded.Code, loaded.Body.String(), loaded.Header())
			}
			stale := editorRequest(h, http.MethodPut, path, `{"version":2}`, "empty")
			if stale.Code != http.StatusConflict || editorObject(t, stale)["code"] != "revision_conflict" {
				t.Fatalf("stale=%d %s", stale.Code, stale.Body.String())
			}
		})
	}
}

func TestEditorHTTPConflictErrorCodes(t *testing.T) {
	for _, code := range []string{"conflict", "revision_conflict"} {
		t.Run(code, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeEditorError(w, &editor.Error{Code: code, Message: "Rejected"})
			if w.Code != http.StatusConflict || editorObject(t, w)["code"] != code {
				t.Fatalf("conflict=%d %s", w.Code, w.Body.String())
			}
		})
	}
}

// The upstream single-range streaming contract is kept by net/http's range
// implementation for full, closed, suffix and unsatisfiable ranges.
func TestEditorHTTPRangeStreaming(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "range")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("0123456789"); err != nil {
		t.Fatal(err)
	}
	file.Close()
	for _, test := range []struct {
		header             string
		status             int
		body, contentRange string
	}{{"", 200, "0123456789", ""}, {"bytes=2-5", 206, "2345", "bytes 2-5/10"}, {"bytes=-3", 206, "789", "bytes 7-9/10"}, {"bytes=50-", 416, "invalid range: failed to overlap\n", "bytes */10"}} {
		t.Run(test.header, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/editor/media/range", nil)
			if test.header != "" {
				r.Header.Set("Range", test.header)
			}
			w := httptest.NewRecorder()
			if err := editorStream(w, r, &media.Handle{Path: file.Name()}, "video/mp4"); err != nil {
				t.Fatal(err)
			}
			if w.Code != test.status || w.Body.String() != test.body || w.Header().Get("Content-Range") != test.contentRange {
				t.Fatalf("range=%q: %d %q %v", test.header, w.Code, w.Body.String(), w.Header())
			}
		})
	}
}

func TestEditorBinaryUploadExceptionsStayScoped(t *testing.T) {
	for _, test := range []struct {
		method, path string
		want         bool
	}{{"POST", "/api/editor/media", true}, {"PUT", "/api/editor/media/id", true}, {"PUT", "/api/editor/projects/id/exports/export", true}, {"POST", "/api/editor/projects", false}, {"POST", "/api/editor/media/id/background", false}, {"PUT", "/api/editor/projects/id", false}, {"PUT", "/api/editor/media/id/metadata", false}, {"PUT", "/api/editor/projects/id/exports/export/extra", false}, {"POST", "/api/anything", false}} {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader("bytes"))
		r.Header.Set("Content-Type", "application/octet-stream")
		if got := allowFileUpload(r); got != test.want {
			t.Errorf("%s %s=%v", test.method, test.path, got)
		}
	}
}

func TestEditorRoutesRetainGatewayAuthAndOrigin(t *testing.T) {
	tg := newTestGateway(t)
	MountEditor(tg.g, nil, nil)
	for _, path := range []string{"/api/editor/projects", "/api/editor/media/id", "/api/editor/id/quality"} {
		w := tg.do(http.MethodGet, path, "", "")
		if w.Code != 401 {
			t.Errorf("unauthenticated %s=%d %s", path, w.Code, w.Body.String())
		}
	}
	cookie := tg.sessionCookie()
	r := httptest.NewRequest("POST", "/api/editor/projects", strings.NewReader(`{"name":"blocked"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Cookie", cookie)
	r.Host = "127.0.0.1:8789"
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross origin=%d %s", w.Code, w.Body.String())
	}
}

// Same route/shape as yt-factory projects.ts:338-425; template data
// stays in templates.project and its own numeric CAS revision remains opaque.
func TestEditorHTTPTemplateProjectAuthoritativeRevision(t *testing.T) {
	h := editorHTTPFixture(t)
	registry := h.service.Store.Templates.(*library.Service)
	baseline := editorRequest(h, http.MethodGet, "/api/editor/projects", "", "")
	initialCount := len(editorObject(t, baseline)["projects"].([]any))
	template, err := registry.Create(context.Background(), "Editable template")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/editor/projects/" + template.ID
	load := editorRequest(h, http.MethodGet, path, "", "")
	if load.Code != 200 {
		t.Fatalf("template load=%d %s", load.Code, load.Body.String())
	}
	project := editorObject(t, load)
	if project["id"] != template.ID || project["schemaVersion"] != float64(15) {
		t.Fatalf("project=%v", project)
	}
	revision := load.Header().Get("X-Editor-Revision")
	if revision != "1" {
		t.Fatalf("template revision=%q", revision)
	}
	project["description"] = "authored in timeline"
	project["name"] = "Timeline template"
	body, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	save := editorRequest(h, http.MethodPut, path, string(body), revision)
	if save.Code != 200 {
		t.Fatalf("template save=%d %s", save.Code, save.Body.String())
	}
	if editorObject(t, save)["revision"] != "2" {
		t.Fatalf("new revision=%s", save.Body.String())
	}
	stale := editorRequest(h, http.MethodPut, path, string(body), revision)
	if stale.Code != 409 || editorObject(t, stale)["code"] != "revision_conflict" {
		t.Fatalf("stale=%d %s", stale.Code, stale.Body.String())
	}
	persisted, got, err := registry.ReadEditorProject(context.Background(), template.ID)
	if err != nil || got != "2" || persisted["description"] != "authored in timeline" || persisted["name"] != "Timeline template" {
		t.Fatalf("authoritative template=%v rev=%s err=%v", persisted, got, err)
	}
	if _, err = h.service.Repository.Get(context.Background(), "projects/"+template.ID); err == nil {
		t.Fatal("template was copied to workspace revision pointers")
	}
	project["schemaVersion"] = 14
	invalid, _ := json.Marshal(project)
	rejected := editorRequest(h, http.MethodPut, path, string(invalid), "2")
	if rejected.Code != 400 {
		t.Fatalf("invalid schema=%d %s", rejected.Code, rejected.Body.String())
	}
	after, got, err := registry.ReadEditorProject(context.Background(), template.ID)
	if err != nil || got != "2" || after["schemaVersion"] != float64(15) {
		t.Fatalf("invalid save changed template=%v rev=%s err=%v", after, got, err)
	}
	list := editorRequest(h, http.MethodGet, "/api/editor/projects", "", "")
	if list.Code != 200 || len(editorObject(t, list)["projects"].([]any)) != initialCount+1 {
		t.Fatalf("template list=%d %s", list.Code, list.Body.String())
	}
	deleted := editorRequest(h, http.MethodDelete, path, "", "")
	if deleted.Code != 204 {
		t.Fatalf("template delete=%d %s", deleted.Code, deleted.Body.String())
	}
	if _, _, err = registry.ReadEditorProject(context.Background(), template.ID); err == nil {
		t.Fatal("deleted template remained in authoritative registry")
	}
}
