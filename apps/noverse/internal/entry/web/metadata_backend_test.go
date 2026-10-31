package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataBackendHTTPContract(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			var g *Gateway
			var project, story, owner string
			if backend == "file" {
				f := newConfigTestGateway(t)
				g, project, story, owner = f.g, "proj", "01-story", f.login("writer", "writer-pass")
				t.Cleanup(g.Close)
			} else {
				f := provisionHostedGateway(t, "metadata-contract", "story")
				g, project, story = f.g, f.project, f.story
				owner = (&testGateway{t: t, g: g}).login("hosted-owner", "mat-khau-12345")
			}
			call := func(method, path, body, version string) *httptest.ResponseRecorder {
				var headers map[string]string
				if version != "" {
					headers = map[string]string{"If-Match": version}
				}
				return doFrom(t, g, method, path, owner, body, "127.0.0.1:1234", headers)
			}
			for _, target := range []struct{ name, path, first, second string }{
				{"project", "/api/projects/" + project, `{"name":"Tên đã lưu","notes":"Ghi chú đã lưu"}`, `{"name":"Tên cũ","notes":"Không được ghi"}`},
				{"story", "/api/projects/" + project + "/stories/" + story + "/meta", `{"title":"Tên đã lưu","status":"idea","tags":["đã lưu"]}`, `{"title":"Tên cũ","status":"writing","tags":["không được ghi"]}`},
			} {
				t.Run(target.name, func(t *testing.T) {
					initial := call(http.MethodGet, target.path, "", "")
					if initial.Code != http.StatusOK {
						t.Fatalf("initial metadata: %d %s", initial.Code, initial.Body.String())
					}
					etag := initial.Header().Get("ETag")
					if etag == "" {
						t.Error("metadata GET must provide an ETag")
						etag = `"missing-metadata-version"`
					}
					first := call(http.MethodPut, target.path, target.first, etag)
					if first.Code != http.StatusOK {
						t.Fatalf("first metadata save: %d %s", first.Code, first.Body.String())
					}
					next := first.Header().Get("ETag")
					if next == "" || next == etag {
						t.Errorf("metadata save did not advance ETag: %q -> %q", etag, next)
					}
					stale := call(http.MethodPut, target.path, target.second, etag)
					if stale.Code != http.StatusConflict {
						t.Fatalf("stale metadata save must conflict: %d %s", stale.Code, stale.Body.String())
					}
					var conflict struct {
						Conflict bool            `json:"conflict"`
						Version  string          `json:"version"`
						Current  json.RawMessage `json:"current"`
					}
					if err := json.Unmarshal(stale.Body.Bytes(), &conflict); err != nil {
						t.Fatal(err)
					}
					if !conflict.Conflict || conflict.Version != strings.Trim(next, `"`) || !strings.Contains(string(conflict.Current), "Tên đã lưu") {
						t.Fatalf("metadata conflict must match file feedback shape: %s", stale.Body.String())
					}
					fresh := call(http.MethodGet, target.path, "", "")
					if fresh.Code != http.StatusOK || fresh.Header().Get("ETag") != next || !strings.Contains(fresh.Body.String(), "Tên đã lưu") || strings.Contains(fresh.Body.String(), "Không được ghi") {
						t.Fatalf("stale request changed metadata: %d %s", fresh.Code, fresh.Body.String())
					}
				})
			}
		})
	}
}
