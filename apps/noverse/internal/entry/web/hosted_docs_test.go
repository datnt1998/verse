package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type docsFixture struct {
	g                                               *Gateway
	project, story, admin, writer, reader, outsider string
}

func newDocsFixture(t *testing.T, backend string) *docsFixture {
	t.Helper()
	var tg *testGateway
	f := &docsFixture{}
	if backend == "postgres" {
		pg := provisionHostedGateway(t, "docs-project", "ignored")
		tg = &testGateway{t: t, g: pg.g}
		f.project = pg.project
		f.story = pg.story
		f.admin = tg.login("hosted-admin", "mat-khau-12345")
		f.writer = tg.login("hosted-owner", "mat-khau-12345")
		f.reader = tg.login("hosted-viewer", "mat-khau-12345")
		f.outsider = tg.login("hosted-outsider", "mat-khau-12345")
	} else {
		cfg := newConfigTestGateway(t)
		tg = cfg.testGateway
		f.project = "proj"
		f.story = "01-story"
		f.admin = tg.login("admin", "admin-pass")
		f.writer = tg.login("writer", "writer-pass")
		f.reader = tg.login("reader", "reader-pass")
		f.outsider = tg.login("outsider", "outsider-pass")
		t.Cleanup(tg.g.Close)
	}
	f.g = tg.g
	return f
}
func (f *docsFixture) request(method, path, cookie, body, etag string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	r.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: f.g.cookiePolicy().Name, Value: cookie})
	}
	if etag != "" {
		r.Header.Set("If-Match", etag)
	}
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	return w
}
func (f *docsFixture) query(scope string) string {
	return "scope=" + scope + "&project=" + url.QueryEscape(f.project) + "&story=" + url.QueryEscape(f.story)
}
func docsStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want %d, got %d: %s", status, w.Code, w.Body.String())
	}
}
func TestDocumentsRulesSharedHTTP(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := newDocsFixture(t, backend)
			for _, scope := range []string{"project", "story"} {
				t.Run(scope, func(t *testing.T) {
					body := func(text string) string {
						b, _ := json.Marshal(map[string]string{"scope": scope, "project": f.project, "story": f.story, "name": "tone.md", "text": text})
						return string(b)
					}
					put := f.request("PUT", "/api/rules/file", f.admin, body("Luật "+scope), "")
					docsStatus(t, put, 200)
					getPath := "/api/rules/file?" + f.query(scope) + "&name=tone.md"
					get := f.request("GET", getPath, f.writer, "", "")
					docsStatus(t, get, 200)
					etag := get.Header().Get("ETag")
					if etag == "" {
						t.Fatal("missing rule ETag")
					}
					if decodeResp(t, get)["text"] != "Luật "+scope {
						t.Fatal(get.Body.String())
					}
					docsStatus(t, f.request("PUT", "/api/rules/file", f.admin, body("Luật mới"), etag), 200)
					stale := f.request("PUT", "/api/rules/file", f.admin, body("Mất cập nhật"), etag)
					docsStatus(t, stale, 409)
					conflict := decodeResp(t, stale)
					current, ok := conflict["current"].(map[string]any)
					if conflict["conflict"] != true || !ok || current["text"] != "Luật mới" || conflict["version"] == "" {
						t.Fatalf("bad conflict: %s", stale.Body.String())
					}
					docsStatus(t, f.request("GET", "/api/rules?"+f.query(scope), f.admin, "", ""), 200)
					docsStatus(t, f.request("PUT", "/api/rules/file", f.reader, body("Không được ghi"), ""), 403)
					docsStatus(t, f.request("GET", "/api/rules/file?"+f.query(scope)+"&name=../evil.md", f.admin, "", ""), 400)
					if scope != "global" {
						docsStatus(t, f.request("GET", getPath, f.outsider, "", ""), 403)
					}
				})
			}
			chain := f.request("GET", "/api/rules?"+f.query("story"), f.admin, "", "")
			docsStatus(t, chain, 200)
			levels := decodeResp(t, chain)["chain"].([]any)
			if len(levels) != 2 {
				t.Fatalf("inheritance chain: %s", chain.Body.String())
			}
			for i, scope := range []string{"project", "story"} {
				level := levels[i].(map[string]any)
				files := level["files"].([]any)
				if level["scope"] != scope || level["exists"] != true || len(files) != 1 {
					t.Fatalf("bad level: %v", level)
				}
			}
			for _, scope := range []string{"project", "story"} {
				b, _ := json.Marshal(map[string]string{"scope": scope, "project": f.project, "story": f.story, "name": "tone.md"})
				path := "/api/rules/file?" + f.query(scope) + "&name=tone.md"
				etag := f.request("GET", path, f.admin, "", "").Header().Get("ETag")
				docsStatus(t, f.request("POST", "/api/rules/file/delete", f.admin, string(b), "stale"), 409)
				docsStatus(t, f.request("POST", "/api/rules/file/delete", f.admin, string(b), etag), 200)
				docsStatus(t, f.request("GET", path, f.admin, "", ""), 404)
			}
		})
	}
}
func TestDocumentsStyleSamplesSharedHTTP(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := newDocsFixture(t, backend)
			path := "/api/projects/" + f.project + "/stories/" + f.story + "/style-samples"
			call := func(method, suffix, body string) *httptest.ResponseRecorder {
				return f.request(method, path+suffix, f.writer, body, "")
			}
			empty := call("GET", "", "")
			docsStatus(t, empty, 200)
			if !strings.Contains(empty.Body.String(), `"samples":[]`) {
				t.Fatal(empty.Body.String())
			}
			for _, body := range []string{`{}`, `{"text":"   "}`, `{"text":"x","unexpected":true}`} {
				docsStatus(t, call("POST", "", body), 400)
			}
			docsStatus(t, f.request("GET", path, f.reader, "", ""), 200)
			docsStatus(t, f.request("POST", path, f.reader, `{"text":"Không được ghi"}`, ""), 403)
			docsStatus(t, f.request("GET", path, f.outsider, "", ""), 403)
			first := call("POST", "", `{"title":"Giọng/của tôi.txt","text":"Trời mưa rất to, con hẻm vắng tanh."}`)
			docsStatus(t, first, 201)
			var sample StyleSample
			if err := json.Unmarshal(first.Body.Bytes(), &sample); err != nil {
				t.Fatal(err)
			}
			if sample.Title != "Giọng-của tôi" || sample.Chars != 8 || sample.AddedAt == "" {
				t.Fatalf("bad sample: %+v", sample)
			}
			second := call("POST", "", `{"title":"Giọng/của tôi.txt","text":"Một đoạn khác."}`)
			docsStatus(t, second, 201)
			if decodeResp(t, second)["id"] == sample.ID {
				t.Fatal("duplicate sample overwrote original")
			}
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			part, err := mw.CreateFormFile("file", "Mẫu tải lên.md")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = part.Write([]byte("Gió lạnh thổi qua sân."))
			_ = mw.Close()
			r := httptest.NewRequest("POST", path, &buf)
			r.RemoteAddr = "192.0.2.1:1234"
			r.Host = "127.0.0.1:8686"
			r.Header.Set("Content-Type", mw.FormDataContentType())
			r.AddCookie(&http.Cookie{Name: f.g.cookiePolicy().Name, Value: f.writer})
			upload := httptest.NewRecorder()
			f.g.ServeHTTP(upload, r)
			docsStatus(t, upload, 201)
			if decodeResp(t, upload)["title"] != "Mẫu tải lên" {
				t.Fatal(upload.Body.String())
			}
			listed := call("GET", "", "")
			docsStatus(t, listed, 200)
			totals := decodeResp(t, listed)["total"].(map[string]any)
			if totals["count"] != float64(3) || totals["chars"] != float64(16) {
				t.Fatal(listed.Body.String())
			}
			if strings.Contains(listed.Body.String(), "simulate") {
				t.Fatal("leaked corpus folder")
			}
			docsStatus(t, call("DELETE", "/"+url.PathEscape(sample.ID), ""), 204)
			docsStatus(t, call("DELETE", "/"+url.PathEscape(sample.ID), ""), 404)
			docsStatus(t, call("DELETE", "/..%2Foutside.txt", ""), 404)
			for i := 2; i < maxStyleSamples; i++ {
				docsStatus(t, call("POST", "", fmt.Sprintf(`{"title":"Mẫu %d","text":"Một câu."}`, i)), 201)
			}
			docsStatus(t, call("POST", "", `{"text":"Quá giới hạn."}`), 409)
		})
	}
}
