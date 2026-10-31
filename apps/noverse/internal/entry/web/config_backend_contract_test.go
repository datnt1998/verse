package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The same authenticated HTTP contract runs against disk and repository scopes.
// No direct handler calls: sessions, membership, feedback ETags and conflict
// bodies all go through ServeHTTP.
func TestConfigBackendHTTPContract(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			var g *Gateway
			var project, story, admin, owner, viewer, outsider string
			if backend == "file" {
				f := newConfigTestGateway(t)
				g, project, story = f.g, "proj", "01-story"
				t.Cleanup(g.Close)
				admin, owner, viewer, outsider = f.login("admin", "admin-pass"), f.login("writer", "writer-pass"), f.login("reader", "reader-pass"), f.login("outsider", "outsider-pass")
			} else {
				f := provisionHostedGateway(t, "config-contract", "story")
				g, project, story = f.g, f.project, f.story
				tg := &testGateway{t: t, g: g}
				admin, owner, viewer, outsider = tg.login("hosted-admin", "mat-khau-12345"), tg.login("hosted-owner", "mat-khau-12345"), tg.login("hosted-viewer", "mat-khau-12345"), tg.login("hosted-outsider", "mat-khau-12345")
			}
			call := func(method, path, cookie, body string, headers map[string]string) *httptest.ResponseRecorder {
				return doFrom(t, g, method, path, cookie, body, "127.0.0.1:1234", headers)
			}
			assertStatus := func(w *httptest.ResponseRecorder, status int) {
				t.Helper()
				if w.Code != status {
					t.Fatalf("status=%d want %d: %s", w.Code, status, w.Body.String())
				}
			}
			global := call(http.MethodPut, "/api/config?scope=global", admin, `{"style":"default","context_window":8000,"provider":"test","model":"scripted","providers":{"test":{"type":"openai","api_key":"test-global-secret"}}}`, nil)
			assertStatus(global, 200)
			projectURL := "/api/config?scope=project&project=" + project
			storyURL := "/api/config?scope=story&project=" + project + "&story=" + story
			for _, path := range []string{projectURL, storyURL} {
				got := call(http.MethodGet, path, owner, "", nil)
				assertStatus(got, 200)
				m := decodeResp(t, got)
				if m["target"].(map[string]any)["exists"] != false {
					t.Fatalf("missing layer: %s", got.Body.String())
				}
				if m["sources"].(map[string]any)["style"] != "global" {
					t.Fatalf("initial sources: %s", got.Body.String())
				}
				assertStatus(call(http.MethodGet, path, outsider, "", nil), 403)
				assertStatus(call(http.MethodPut, path, outsider, `{"style":"xianxia"}`, nil), 403)
				assertStatus(call(http.MethodGet, path, viewer, "", nil), 200)
				rejected := call(http.MethodPut, path, owner, `{"providers":{"test":{"api_key":"must-not-be-persisted"}}}`, nil)
				assertStatus(rejected, 400)
			}
			assertStatus(call(http.MethodPut, projectURL, viewer, `{"style":"xianxia"}`, nil), 403)
			assertStatus(call(http.MethodPut, storyURL, viewer, `{"style":"xianxia"}`, nil), 403)
			for _, item := range []struct {
				path, body, style, source string
				max                       float64
				maxSource                 string
			}{
				{projectURL, `{"style":"xianxia","context_window":4000}`, "xianxia", "project", 4000, "project"},
				{storyURL, `{"style":"nu-tan-bao-thu"}`, "nu-tan-bao-thu", "story", 4000, "project"},
			} {
				initial := call(http.MethodGet, item.path, owner, "", nil)
				assertStatus(initial, 200)
				etag := initial.Header().Get("ETag")
				if etag == "" {
					t.Fatal("GET did not provide ETag")
				}
				saved := call(http.MethodPut, item.path, owner, item.body, map[string]string{"If-Match": etag})
				assertStatus(saved, 200)
				nextTag := saved.Header().Get("ETag")
				if nextTag == "" || nextTag == etag {
					t.Fatalf("ETag did not advance: %q -> %q", etag, nextTag)
				}
				m := decodeResp(t, saved)
				effective := m["effective"].(map[string]any)
				sources := m["sources"].(map[string]any)
				if effective["style"] != item.style || effective["context_window"] != item.max || sources["style"] != item.source || sources["context_window"] != item.maxSource {
					t.Fatalf("effective inheritance: %s", saved.Body.String())
				}
				target := m["target"].(map[string]any)
				if target["exists"] != true || target["config"].(map[string]any)["style"] != item.style {
					t.Fatalf("target: %s", saved.Body.String())
				}
				conflict := call(http.MethodPut, item.path, owner, `{"style":"default"}`, map[string]string{"If-Match": etag})
				assertStatus(conflict, 409)
				var envelope struct {
					Conflict bool            `json:"conflict"`
					Current  json.RawMessage `json:"current"`
					Version  string          `json:"version"`
				}
				if err := json.Unmarshal(conflict.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if !envelope.Conflict || len(envelope.Current) == 0 || envelope.Version == "" {
					t.Fatalf("conflict contract: %s", conflict.Body.String())
				}
				fresh := call(http.MethodGet, item.path, owner, "", nil)
				assertStatus(fresh, 200)
				if decodeResp(t, fresh)["effective"].(map[string]any)["style"] != item.style {
					t.Fatal("conflicting write changed config")
				}
			}
			// Removing a declaration restores inheritance and its field provenance.
			reset := call(http.MethodPut, storyURL, owner, `{}`, nil)
			assertStatus(reset, 200)
			m := decodeResp(t, reset)
			if m["effective"].(map[string]any)["style"] != "xianxia" || m["sources"].(map[string]any)["style"] != "project" {
				t.Fatalf("reset inheritance: %s", reset.Body.String())
			}
		})
	}
}
