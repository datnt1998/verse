package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/platform/identity"
)

// The same HTTP contract must hold for a project directory and a hosted
// project row, including a project with no story-dependent media setup.
func TestProjectAudioBackendParity(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			var g *Gateway
			var owner, viewer, outsider, collab identity.User
			project := "proj"
			story := ""
			if backend == "file" {
				f := newAudioFixture(t)
				g, owner = f.tg.g, adminUser()
				story = f.st.Story
				viewer = identity.User{Username: "viewer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{project: identity.ProjectView}}
				collab = identity.User{Username: "collab", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{project: identity.ProjectCollab}}
				outsider = identity.User{Username: "outsider", Role: identity.RoleMember}
			} else {
				f := provisionHostedMedia(t)
				g, owner, viewer, outsider, project = f.fx.g, f.fx.owner, f.fx.viewer, f.fx.outsider, f.fx.project
				story = f.st.Story
				if _, err := f.fx.accounts.CreateUser(identity.NewUser{Username: "hosted-collab", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{project: identity.ProjectCollab}, Password: "mat-khau-12345"}); err != nil {
					t.Fatal(err)
				}
				collab = loginUser(t, f.fx.accounts, "hosted-collab")
			}
			base := "/api/projects/" + project
			call := func(user identity.User, method, path, body, etag string) *httptest.ResponseRecorder {
				t.Helper()
				r := httptest.NewRequest(method, base+path, strings.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
				if etag != "" {
					r.Header.Set("If-Match", etag)
				}
				w := httptest.NewRecorder()
				g.serveProjects(w, r, user)
				return w
			}
			t.Run("pronunciations", func(t *testing.T) {
				w := call(owner, "GET", "/pronunciations", "", "")
				if w.Code != 200 {
					t.Fatalf("GET: %d %s", w.Code, w.Body.String())
				}
				initial := w.Header().Get("ETag")
				if initial == "" {
					t.Fatal("missing ETag")
				}
				w = call(owner, "PUT", "/pronunciations", `[{"written":" MC ","spoken":" em-xi "},{"written":"MC","spoken":"em xi"}]`, initial)
				if w.Code != 200 {
					t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
				}
				var entries []audio.Pronunciation
				if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil || len(entries) != 1 || entries[0].Spoken != "em xi" {
					t.Fatalf("normalize: %s %v", w.Body.String(), err)
				}
				current := w.Header().Get("ETag")
				if current == initial {
					t.Fatal("version did not advance")
				}
				w = call(owner, "PUT", "/pronunciations", `[]`, initial)
				if w.Code != 409 {
					t.Fatalf("stale PUT: %d %s", w.Code, w.Body.String())
				}
				w = call(owner, "PUT", "/pronunciations", `{"entries":[{"written":"show","spoken":"sô"}]}`, current)
				if w.Code != 200 {
					t.Fatalf("wrapped PUT: %d %s", w.Code, w.Body.String())
				}
				if allowed := call(collab, "PUT", "/pronunciations", `[{"written":"show","spoken":"sô"}]`, ""); allowed.Code != 200 {
					t.Fatalf("collaborator PUT: %d %s", allowed.Code, allowed.Body.String())
				}
				w = call(viewer, "GET", "/pronunciations", "", "")
				if w.Code != 200 || !strings.Contains(w.Body.String(), "show") {
					t.Fatalf("viewer read: %d %s", w.Code, w.Body.String())
				}
				if w = call(viewer, "PUT", "/pronunciations", `[]`, ""); w.Code != 403 {
					t.Fatalf("viewer write: %d", w.Code)
				}
				if w = call(outsider, "GET", "/pronunciations", "", ""); w.Code == 200 {
					t.Fatal("outsider can read")
				}
				if w = call(owner, "PUT", "/pronunciations", `[{"written":"empty"}]`, ""); w.Code != 409 {
					t.Fatalf("validation: %d", w.Code)
				}
				for _, m := range []string{"POST", "DELETE"} {
					if w = call(owner, m, "/pronunciations", `[]`, ""); w.Code != 404 {
						t.Fatalf("unsupported %s: %d", m, w.Code)
					}
				}
			})
			t.Run("voices", func(t *testing.T) {
				if w := call(owner, "GET", "/voices", "", ""); w.Code != 200 {
					t.Fatalf("GET: %d %s", w.Code, w.Body.String())
				}
				if w := call(owner, "PUT", "/voices", `{}`, ""); w.Code != 404 {
					t.Fatalf("unsupported PUT: %d", w.Code)
				}
				if !audio.FFmpegAvailable() {
					t.Skip("ffmpeg needed to convert uploaded voice sample")
				}
				uploadPath := base + "/voices"
				upload := func(user identity.User) *httptest.ResponseRecorder {
					var body bytes.Buffer
					mw := multipart.NewWriter(&body)
					_ = mw.WriteField("name", "Giọng kiểm thử")
					_ = mw.WriteField("gender", "female")
					fw, err := mw.CreateFormFile("file", "sample.wav")
					if err != nil {
						t.Fatal(err)
					}
					_, _ = fw.Write(testWAV(t, 8000, 10.2))
					_ = mw.Close()
					r := httptest.NewRequest("POST", uploadPath, &body)
					r.Header.Set("Content-Type", mw.FormDataContentType())
					w := httptest.NewRecorder()
					g.serveProjects(w, r, user)
					return w
				}
				if w := upload(viewer); w.Code != 403 {
					t.Fatalf("viewer upload: %d %s", w.Code, w.Body.String())
				}
				w := upload(owner)
				if w.Code != 200 {
					t.Fatalf("upload: %d %s", w.Code, w.Body.String())
				}
				var voice audio.ProjectVoice
				if err := json.Unmarshal(w.Body.Bytes(), &voice); err != nil || voice.ID == "" || voice.Gender != "nu" {
					t.Fatalf("voice: %s %v", w.Body.String(), err)
				}
				w = upload(owner)
				var second audio.ProjectVoice
				_ = json.Unmarshal(w.Body.Bytes(), &second)
				if w.Code != 200 || second.ID == voice.ID {
					t.Fatalf("duplicate: %d %s", w.Code, w.Body.String())
				}
				w = call(viewer, "GET", "/voices/"+url.PathEscape(voice.ID)+"/sample", "", "")
				if w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("RIFF")) {
					t.Fatalf("sample: %d %s", w.Code, w.Body.String())
				}
				if duration := audio.WAVDurationSec(w.Body.Bytes()); duration <= 0 || duration > 5.01 {
					t.Fatalf("sample must be a short preview: %g", duration)
				}
				if w = call(outsider, "GET", "/voices/"+url.PathEscape(voice.ID)+"/sample", "", ""); w.Code == 200 {
					t.Fatal("outsider sample")
				}
				if w = call(viewer, "DELETE", "/voices/"+url.PathEscape(voice.ID), "", ""); w.Code != 403 {
					t.Fatalf("viewer delete: %d", w.Code)
				}
				if w = call(owner, "DELETE", "/voices/"+url.PathEscape(voice.ID), "", ""); w.Code != 200 {
					t.Fatalf("delete: %d %s", w.Code, w.Body.String())
				}
				if w = call(owner, "GET", "/voices/"+url.PathEscape(voice.ID)+"/sample", "", ""); w.Code != 400 {
					t.Fatalf("deleted sample: %d", w.Code)
				}
				if w = call(owner, "DELETE", "/voices/"+url.PathEscape(voice.ID), "", ""); w.Code != 404 {
					t.Fatalf("missing delete: %d %s", w.Code, w.Body.String())
				}
				w = call(owner, "GET", "/voices", "", "")
				var voices []audio.ProjectVoice
				if err := json.Unmarshal(w.Body.Bytes(), &voices); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, v := range voices {
					if v.ID == voice.ID {
						t.Fatal("deleted voice still listed")
					}
					if v.ID == second.ID {
						found = true
					}
				}
				if !found {
					t.Fatal("other voice removed")
				}
				t.Run("story_upload", func(t *testing.T) {
					uploadPath = base + "/stories/" + story + "/audio/voices"
					denied := upload(viewer)
					if denied.Code != 403 && denied.Code != 404 {
						t.Fatalf("viewer story upload: %d %s", denied.Code, denied.Body.String())
					}
					w := upload(owner)
					if w.Code != 200 {
						t.Fatalf("story upload: %d %s", w.Code, w.Body.String())
					}
					var added audio.ProjectVoice
					if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil {
						t.Fatal(err)
					}
					samplePrefix := base + "/voices/"
					if !strings.HasPrefix(added.SampleURL, samplePrefix) {
						t.Fatalf("sample URL not canonical project resource: %s", added.SampleURL)
					}
					req := httptest.NewRequest("GET", added.SampleURL, nil)
					sample := httptest.NewRecorder()
					g.serveProjects(sample, req, owner)
					if sample.Code != 200 || !bytes.HasPrefix(sample.Body.Bytes(), []byte("RIFF")) {
						t.Fatalf("story sample: %d", sample.Code)
					}
					if allowed := upload(collab); allowed.Code != 200 {
						t.Fatalf("collaborator story upload: %d %s", allowed.Code, allowed.Body.String())
					}
				})
			})
		})
	}
}
