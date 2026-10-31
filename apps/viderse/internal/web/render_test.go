package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/platform/blobstore"
)

func renderTokenHTTP(tg *testGateway, method, path, token string, query bool, body, origin string) *httptest.ResponseRecorder {
	tg.t.Helper()
	if query && token != "" {
		path += "?token=" + url.QueryEscape(token)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:8789"
	r.RemoteAddr = "192.0.2.1:1234"
	if !query && token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func renderHTTPStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("HTTP status = %d, want %d: %s", w.Code, want, w.Body.String())
	}
}

func renderPublicRequest(t *testing.T, w *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var envelope struct {
		Request map[string]json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Request == nil {
		t.Fatalf("missing public request: %s", w.Body.String())
	}
	for _, secret := range []string{"token", "chrome"} {
		if _, exists := envelope.Request[secret]; exists {
			t.Fatalf("public response exposes %s", secret)
		}
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("render credential unexpectedly issued a session cookie")
	}
	raw, err := json.Marshal(envelope.Request)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRenderTokenScope(t *testing.T) {
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			tg := newTestGateway(t)
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			store := &render.Store{Root: t.TempDir(), Now: func() time.Time { return now }}
			MountRender(tg.g, &render.Service{Store: store}, context.Background())
			MountEditor(tg.g, &editor.Service{}, nil)
			MountClone(tg.g, nil, nil, nil)
			binding := quality.Binding{ProjectRevision: "revision", SettingsHash: "settings", ArtifactHash: "artifacts"}
			var id, token, nonce, fence, otherID, otherToken string
			var cancel func() error
			if kind == "render" {
				request, err := store.CreateExport("project", "job")
				if err != nil {
					t.Fatal(err)
				}
				other, err := store.CreateExport("project", "job")
				if err != nil {
					t.Fatal(err)
				}
				id, token, nonce = request.ID, request.Token, request.Nonce
				otherID, otherToken = other.ID, other.Token
				cancel = func() error { _, err := store.CancelExport(id, nonce); return err }
			} else {
				request, err := store.CreateMeasurement("job", "project", binding)
				if err != nil {
					t.Fatal(err)
				}
				other, err := store.CreateMeasurement("job", "project", binding)
				if err != nil {
					t.Fatal(err)
				}
				id, token, nonce = request.ID, request.Token, request.Nonce
				otherID, otherToken = other.ID, other.Token
				// HTTP measurement claim derives the current project binding through
				// the pipeline. Claim directly here to isolate the token boundary.
				claimed, err := store.ClaimMeasurement(id, nonce, "headless-runtime", binding)
				if err != nil {
					t.Fatal(err)
				}
				fence = claimed.Fence
				cancel = func() error { _, err := store.CancelMeasurement(id, nonce); return err }
			}
			base := "/api/" + kind + "/" + id

			if kind == "render" {
				t.Run("claim without session", func(t *testing.T) {
					w := renderTokenHTTP(tg, http.MethodPost, base+"/claim", token, false, `{"nonce":"`+nonce+`"}`, "")
					renderHTTPStatus(t, w, http.StatusOK)
					var claimed render.ExportRequest
					if err := json.Unmarshal(renderPublicRequest(t, w), &claimed); err != nil {
						t.Fatal(err)
					}
					if claimed.ID != id || claimed.Status != "running" || claimed.Fence == "" {
						t.Fatalf("claim did not acquire requested export: %+v", claimed)
					}
					fence = claimed.Fence
				})
			}
			body := `{"fence":"` + fence + `"}`
			for _, query := range []bool{false, true} {
				name := "bearer heartbeat without session"
				if query {
					name = "query heartbeat without session"
				}
				t.Run(name, func(t *testing.T) {
					now = now.Add(10 * time.Second)
					w := renderTokenHTTP(tg, http.MethodPost, base+"/heartbeat", token, query, body, "")
					renderHTTPStatus(t, w, http.StatusOK)
					var got struct {
						ID          string `json:"id"`
						Status      string `json:"status"`
						Fence       string `json:"fence"`
						HeartbeatAt string `json:"heartbeatAt"`
					}
					if err := json.Unmarshal(renderPublicRequest(t, w), &got); err != nil {
						t.Fatal(err)
					}
					if got.ID != id || got.Status != "running" || got.Fence != fence || got.HeartbeatAt != now.Format(time.RFC3339Nano) {
						t.Fatalf("heartbeat failed to renew the scoped request: %+v", got)
					}
					if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
						t.Fatalf("unsafe render response headers: %v", w.Header())
					}
				})
			}
			wrong := "0" + token[1:]
			if wrong == token {
				wrong = "1" + token[1:]
			}
			for _, tc := range []struct {
				name, path, token string
			}{
				{"missing token", base + "/heartbeat", ""},
				{"wrong token", base + "/heartbeat", wrong},
				{"another request token", base + "/heartbeat", otherToken},
				{"token cannot address another request", "/api/" + kind + "/" + otherID + "/heartbeat", token},
				{"token cannot change kind", "/api/" + map[string]string{"render": "measure", "measure": "render"}[kind] + "/" + id + "/heartbeat", token},
			} {
				t.Run(tc.name, func(t *testing.T) {
					w := renderTokenHTTP(tg, http.MethodPost, tc.path, tc.token, false, body, "")
					renderHTTPStatus(t, w, http.StatusUnauthorized)
				})
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				t.Run("cross-site "+method, func(t *testing.T) {
					w := renderTokenHTTP(tg, method, base+"/heartbeat", token, false, body, "https://attacker.example")
					renderHTTPStatus(t, w, http.StatusForbidden)
				})
			}
			for _, path := range []string{"/api/editor/projects", "/api/me", "/api/clone/job/export"} {
				for _, query := range []bool{false, true} {
					t.Run("cookie required "+path+" query="+map[bool]string{true: "true", false: "false"}[query], func(t *testing.T) {
						method := http.MethodGet
						if path == "/api/clone/job/export" {
							method = http.MethodPost
						}
						w := renderTokenHTTP(tg, method, path, token, query, `{}`, "")
						renderHTTPStatus(t, w, http.StatusUnauthorized)
					})
				}
			}
			t.Run("expired token", func(t *testing.T) {
				before := now
				now = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC).Add(render.RequestTokenTTL)
				defer func() { now = before }()
				w := renderTokenHTTP(tg, http.MethodPost, base+"/heartbeat", token, false, body, "")
				renderHTTPStatus(t, w, http.StatusUnauthorized)
			})
			t.Run("terminal token", func(t *testing.T) {
				if err := cancel(); err != nil {
					t.Fatal(err)
				}
				w := renderTokenHTTP(tg, http.MethodPost, base+"/heartbeat", token, false, body, "")
				renderHTTPStatus(t, w, http.StatusUnauthorized)
			})
		})
	}
	t.Run("project media isolation", testRenderTokenProjectMedia)
	t.Run("authenticated export acceptance", testRenderTokenExportAcceptance)
}

// This read-only repository exposes the fake's published legacy bindings. Any
// unexpected mutation fails rather than pretending to persist a resource.
type renderHTTPReadRepository struct{ files *stepstest.FakeMedia }

func (r renderHTTPReadRepository) Get(context.Context, string) (editor.Resource, error) {
	return editor.Resource{}, blobstore.ErrNotFound
}
func (r renderHTTPReadRepository) List(context.Context, string) ([]editor.Resource, error) {
	return []editor.Resource{}, nil
}
func (r renderHTTPReadRepository) CAS(context.Context, string, string, editor.Resource) error {
	return errors.New("render HTTP fixture repository is read-only")
}
func (r renderHTTPReadRepository) Delete(context.Context, string, string) error {
	return errors.New("render HTTP fixture repository is read-only")
}
func (r renderHTTPReadRepository) Paths(_ context.Context, prefix string) ([]string, error) {
	var paths []string
	for _, path := range r.files.Published {
		if strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func renderHTTPPutJSON(t *testing.T, files *stepstest.FakeMedia, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	files.Put(path, data)
	if _, err := files.Acquire(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}

func testRenderTokenProjectMedia(t *testing.T) {
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	repo := renderHTTPReadRepository{files: files}
	edit := &editor.Service{Store: editor.Store{Media: files, Repository: repo}, Repository: repo}
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	for i, pid := range []string{"project", "other-project"} {
		project, err := editor.New(pid, pid, 1920, 1080, 30, time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		project["timeline"] = editor.Object{"items": []editor.Object{{"id": "image", "type": "image", "mediaId": ids[i]}}}
		renderHTTPPutJSON(t, files, "editor/projects/"+pid+"/project.json", project)
		renderHTTPPutJSON(t, files, "editor/media/"+ids[i]+"/metadata.json", editor.Object{
			"id": ids[i], "fileName": "image.webp", "mime": "image/webp", "size": len(pid),
			"contentHash": "sha256:fixture", "sourceFile": "source.webp",
		})
		files.Put("editor/media/"+ids[i]+"/source.webp", []byte(pid))
	}
	store := &render.Store{Root: t.TempDir()}
	binding := quality.Binding{ProjectRevision: "revision", SettingsHash: "settings", ArtifactHash: "artifacts"}
	export, err := store.CreateExport("project", "job")
	if err != nil {
		t.Fatal(err)
	}
	measure, err := store.CreateMeasurement("job", "project", binding)
	if err != nil {
		t.Fatal(err)
	}
	tg := newTestGateway(t)
	MountRender(tg.g, &render.Service{Store: store, Editor: edit}, context.Background())
	for _, request := range []struct{ kind, id, token string }{
		{"render", export.ID, export.Token}, {"measure", measure.ID, measure.Token},
	} {
		t.Run(request.kind, func(t *testing.T) {
			base := "/api/" + request.kind + "/" + request.id + "/media/"
			w := renderTokenHTTP(tg, http.MethodGet, base+ids[0], request.token, true, "", "")
			renderHTTPStatus(t, w, http.StatusOK)
			if w.Body.String() != "project" || w.Header().Get("Content-Type") != "image/webp" {
				t.Fatalf("wrong project media: headers=%v body=%q", w.Header(), w.Body.String())
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w = renderTokenHTTP(tg, method, base+ids[1], request.token, true, "", "")
				renderHTTPStatus(t, w, http.StatusUnauthorized)
				if strings.Contains(w.Body.String(), "other-project") {
					t.Fatal("cross-project source bytes leaked")
				}
			}
		})
	}
}

type renderHTTPJobRepository struct {
	job     domain.CloneJob
	records []domain.CloneStepRecord
}

func (r renderHTTPJobRepository) GetJob(_ context.Context, id string) (*domain.CloneJob, error) {
	if id != r.job.ID {
		return nil, nil
	}
	job := r.job
	return &job, nil
}
func (r renderHTTPJobRepository) ListSteps(context.Context, string) ([]domain.CloneStepRecord, error) {
	return r.records, nil
}

func testRenderTokenExportAcceptance(t *testing.T) {
	project, err := editor.New("project", "Export fixture", 1920, 1080, 30, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := editor.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	files.Put("editor/projects/project/project.json", raw)
	templateBytes := []byte("pinned template media")
	files.Put("template/source.webp", templateBytes)
	sum := sha256.Sum256(templateBytes)
	snapshot, err := json.Marshal(library.Snapshot{
		Project:    raw,
		Binding:    library.TemplateBinding{ProjectID: "template", Media: []library.TemplateMediaBinding{{MediaID: "template-media", ContentHash: "sha256:" + hex.EncodeToString(sum[:])}}},
		MediaFiles: map[string]string{"template-media": "template/source.webp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	job := stepstest.Job("job")
	job.Status = domain.CloneWaiting
	job.TemplateSnapshot = snapshot
	repo := renderHTTPJobRepository{job: job, records: []domain.CloneStepRecord{{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}}}}
	edit := &editor.Service{Store: editor.Store{Media: files}, ExportOwner: func(context.Context, string) (string, error) { return "job", nil }}
	resumes := 0
	service := &render.Service{
		Store: &render.Store{Root: t.TempDir()}, Editor: edit, Jobs: repo, Media: files,
		Resume: func(_ context.Context, id string) error {
			if id != "job" {
				return errors.New("wrong clone job resumed")
			}
			resumes++
			return nil
		},
	}
	tg := newTestGateway(t)
	MountClone(tg.g, nil, nil, nil)
	MountRender(tg.g, service, context.Background())
	cookie := tg.sessionCookie()
	path := "/api/clone/job/export"
	for _, body := range []string{`{"accept":"true"}`, `{"accept":1}`, `{"accept":null}`} {
		w := tg.do(http.MethodPost, path, cookie, body)
		renderHTTPStatus(t, w, http.StatusBadRequest)
	}
	for _, body := range []string{`{}`, `{"accept":false}`} {
		w := tg.do(http.MethodPost, path, cookie, body)
		renderHTTPStatus(t, w, http.StatusConflict)
	}
	if resumes != 0 {
		t.Fatalf("invalid or unaccepted approval resumed work %d times", resumes)
	}
	var firstID string
	for i := range 2 {
		w := tg.do(http.MethodPost, path, cookie, `{"accept":true}`)
		renderHTTPStatus(t, w, http.StatusAccepted)
		var response struct {
			RequestID string `json:"requestId"`
			StateURL  string `json:"stateUrl"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		var request render.ExportRequest
		if err := json.Unmarshal(renderPublicRequest(t, w), &request); err != nil {
			t.Fatal(err)
		}
		if response.RequestID == "" || response.StateURL != "/api/clone/job/exports/"+response.RequestID || request.ID != response.RequestID || request.JobID != "job" || request.ProjectID != "project" || request.Status != "queued" {
			t.Fatalf("incorrect accepted export shape: %s", w.Body.String())
		}
		if i == 0 {
			firstID = response.RequestID
		} else if response.RequestID != firstID {
			t.Fatalf("duplicate HTTP approval created a second request: %s != %s", response.RequestID, firstID)
		}
	}
	if resumes != 1 {
		t.Fatalf("duplicate HTTP approval resumed pipeline %d times, want 1", resumes)
	}
	requests, err := service.Store.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].ID != firstID {
		t.Fatalf("duplicate HTTP approval persisted extra intents: %+v", requests)
	}
	state, err := service.State("job")
	if err != nil {
		t.Fatal(err)
	}
	if state.AcceptedAt == nil {
		t.Fatal("accept:true did not persist the explicit quality-gate acceptance")
	}
}
