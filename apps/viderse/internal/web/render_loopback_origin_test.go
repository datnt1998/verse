package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/platform/webguard"
)

const (
	scopedLoopbackOrigin = "http://127.0.0.1:8789"
	scopedPublicOrigin   = "https://viderse.example.test"
	scopedProjectMedia   = "11111111-1111-4111-8111-111111111111"
	scopedOtherMedia     = "22222222-2222-4222-8222-222222222222"
)

type scopedLoopbackFixture struct {
	tg      *testGateway
	service *render.Service
	now     time.Time
	binding quality.Binding
}

type scopedLoopbackIntent struct {
	kind, id, token, nonce string
}

func newScopedLoopbackFixture(t *testing.T) *scopedLoopbackFixture {
	t.Helper()
	// Reproduce the hosted deployment, including a locally connected trusted
	// proxy. The exception must use the raw socket peer, not forwarded facts.
	for key, value := range map[string]string{
		"PUBLIC_URL":          scopedPublicOrigin,
		"SECURE_COOKIES":      "true",
		"TRUSTED_PROXIES":     "127.0.0.1/32",
		"VERSE_COOKIE_DOMAIN": "",
		"VERSE_APP_ORIGINS":   "",
		"VERSE_APP_LINKS":     "",
	} {
		t.Setenv(key, value)
	}
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	repo := renderHTTPReadRepository{files: files}
	edit := &editor.Service{Store: editor.Store{Media: files, Repository: repo}, Repository: repo}
	for i, pid := range []string{"project", "other-project"} {
		project, err := editor.New(pid, pid, 1920, 1080, 30, time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		id := []string{scopedProjectMedia, scopedOtherMedia}[i]
		project["timeline"] = editor.Object{"items": []editor.Object{{"id": "image", "type": "image", "mediaId": id}}}
		renderHTTPPutJSON(t, files, "editor/projects/"+pid+"/project.json", project)
		renderHTTPPutJSON(t, files, "editor/media/"+id+"/metadata.json", editor.Object{
			"id": id, "fileName": "image.webp", "mime": "image/webp", "size": len(pid),
			"contentHash": "sha256:fixture", "sourceFile": "source.webp",
		})
		files.Put("editor/media/"+id+"/source.webp", []byte(pid))
	}
	f := &scopedLoopbackFixture{tg: newTestGateway(t), now: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}
	proxyCfg, err := webguard.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	f.tg.g.SetProxyConfig(proxyCfg)
	store := &render.Store{Root: t.TempDir(), Now: func() time.Time { return f.now }}
	f.service = &render.Service{
		Store: store, Editor: edit, Media: files,
		Jobs: renderHTTPJobRepository{
			job:     stepstest.Job("job"),
			records: []domain.CloneStepRecord{{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}}},
		},
		Supervisor: &render.Supervisor{Store: store, Origin: scopedLoopbackOrigin},
	}
	binding, err := f.service.CurrentBinding(context.Background(), "job")
	if err != nil {
		t.Fatal(err)
	}
	f.binding = binding
	MountRender(f.tg.g, f.service, context.Background())
	MountEditor(f.tg.g, edit, nil)
	MountClone(f.tg.g, nil, nil, nil)
	return f
}

func (f *scopedLoopbackFixture) intent(t *testing.T, kind string) scopedLoopbackIntent {
	t.Helper()
	if kind == "render" {
		r, err := f.service.Store.CreateExport("project", "job")
		if err != nil {
			t.Fatal(err)
		}
		return scopedLoopbackIntent{kind, r.ID, r.Token, r.Nonce}
	}
	r, err := f.service.Store.CreateMeasurement("job", "project", f.binding)
	if err != nil {
		t.Fatal(err)
	}
	return scopedLoopbackIntent{kind, r.ID, r.Token, r.Nonce}
}

func (r scopedLoopbackIntent) path(action string) string {
	return "/api/" + r.kind + "/" + r.id + "/" + action
}

func (r scopedLoopbackIntent) claimBody() string {
	return `{"nonce":"` + r.nonce + `","ownerId":"local-chrome"}`
}

func (r scopedLoopbackIntent) request(method, action, body string) *http.Request {
	req := httptest.NewRequest(method, r.path(action), strings.NewReader(body))
	req.Host = "127.0.0.1:8789"
	req.RemoteAddr = "127.0.0.1:45678"
	req.Header.Set("Origin", scopedLoopbackOrigin)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (f *scopedLoopbackFixture) serve(t *testing.T, req *http.Request, status int) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.tg.g.ServeHTTP(w, req)
	renderHTTPStatus(t, w, status)
	if len(w.Result().Cookies()) != 0 {
		t.Fatalf("scoped credential unexpectedly issued a cookie: %v", w.Result().Cookies())
	}
	if strings.HasPrefix(req.URL.Path, "/api/render/") || strings.HasPrefix(req.URL.Path, "/api/measure/") {
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("unsafe scoped response headers: %v", w.Header())
		}
	}
	return w
}

func (f *scopedLoopbackFixture) persisted(t *testing.T, r scopedLoopbackIntent) any {
	t.Helper()
	// Reopen the filesystem store so assertions cannot pass on response-only or
	// in-memory mutations which never made it to the request record.
	store := &render.Store{Root: f.service.Store.Root, Now: func() time.Time { return f.now }}
	if r.kind == "render" {
		got, err := store.GetExport(r.id)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	got, err := store.GetMeasurement(r.id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (f *scopedLoopbackFixture) unchanged(t *testing.T, r scopedLoopbackIntent, before any) {
	t.Helper()
	if after := f.persisted(t, r); !reflect.DeepEqual(after, before) {
		t.Fatalf("request changed despite rejected operation:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestScopedLoopbackClaimsThroughHostedGateway(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			r := f.intent(t, kind)
			req := r.request(http.MethodPost, "claim", r.claimBody())
			if kind == "measure" {
				// Old browsers may omit fetch metadata; a direct IPv6 loopback
				// socket still addresses the configured IPv4 HTTP authority.
				req.RemoteAddr = "[::1]:45678"
				req.Header.Del("Sec-Fetch-Site")
			}
			w := f.serve(t, req, http.StatusOK)
			if kind == "render" {
				got := f.persisted(t, r).(render.ExportRequest)
				if got.Status != "running" || got.Fence == "" || got.ClaimedAt != f.now.Format(time.RFC3339Nano) || got.HeartbeatAt != got.ClaimedAt {
					t.Fatalf("local browser did not persist its export claim: %+v", got)
				}
				var response render.ExportRequest
				if err := json.Unmarshal(renderPublicRequest(t, w), &response); err != nil {
					t.Fatal(err)
				}
				if response.ID != r.id || response.Status != got.Status || response.Fence != got.Fence {
					t.Fatalf("claim response disagrees with persisted export: %+v", response)
				}
				return
			}
			got := f.persisted(t, r).(render.MeasurementRequest)
			if got.Status != "running" || got.Fence == "" || got.OwnerID != "local-chrome" || got.ClaimedAt != f.now.Format(time.RFC3339Nano) {
				t.Fatalf("local browser did not persist its measurement claim: %+v", got)
			}
			var response render.MeasurementRequest
			if err := json.Unmarshal(renderPublicRequest(t, w), &response); err != nil {
				t.Fatal(err)
			}
			if response.ID != r.id || response.Status != got.Status || response.Fence != got.Fence {
				t.Fatalf("claim response disagrees with persisted measurement: %+v", response)
			}
			// Exercise the real bulk route, not merely the claim helper. Eight
			// zero bytes are one f32le stereo frame, requiring no media tools.
			chunk := r.request(http.MethodPost, "chunks?seq=0&sampleRate=48000&channels=2", string(make([]byte, 8)))
			chunk.Header.Set("Content-Type", "application/octet-stream")
			chunk.Header.Set("Sec-Fetch-Site", "")
			chunk.Header.Set("X-Render-Fence", got.Fence)
			w = f.serve(t, chunk, http.StatusOK)
			var ack render.ChunkResult
			if err := json.Unmarshal(w.Body.Bytes(), &ack); err != nil {
				t.Fatal(err)
			}
			got = f.persisted(t, r).(render.MeasurementRequest)
			if ack.ChunksReceived != 1 || ack.BytesReceived != 8 || got.ChunksReceived != 1 || got.BytesReceived != 8 || got.SampleRate == nil || *got.SampleRate != 48000 || got.Channels == nil || *got.Channels != 2 {
				t.Fatalf("local PCM did not persist: acknowledgment=%+v request=%+v", ack, got)
			}
			for _, tc := range []struct{ name, fence, seq string }{
				{"wrong fence", "not-the-fence", "1"},
				{"skipped sequence", got.Fence, "2"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					bad := r.request(http.MethodPost, "chunks?seq="+tc.seq+"&sampleRate=48000&channels=2", string(make([]byte, 8)))
					bad.Header.Set("X-Render-Fence", tc.fence)
					f.serve(t, bad, http.StatusConflict)
					f.unchanged(t, r, got)
				})
			}
		})
	}
}

func TestScopedLoopbackOriginAndPeerBoundaries(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	r := f.intent(t, "render")
	before := f.persisted(t, r)
	for _, tc := range []struct {
		name  string
		alter func(*http.Request)
	}{
		{"foreign origin", func(req *http.Request) { req.Header.Set("Origin", "https://attacker.example") }},
		{"opaque origin", func(req *http.Request) { req.Header.Set("Origin", "null") }},
		{"duplicate origin", func(req *http.Request) { req.Header.Add("Origin", scopedLoopbackOrigin) }},
		{"comma joined origin", func(req *http.Request) { req.Header.Set("Origin", scopedLoopbackOrigin+", https://attacker.example") }},
		{"sibling port", func(req *http.Request) { req.Header.Set("Origin", "http://127.0.0.1:8790") }},
		{"cross site", func(req *http.Request) { req.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{"same site is not same origin", func(req *http.Request) { req.Header.Set("Sec-Fetch-Site", "same-site") }},
		{"duplicate fetch metadata", func(req *http.Request) { req.Header.Add("Sec-Fetch-Site", "same-origin") }},
		{"remote peer", func(req *http.Request) { req.RemoteAddr = "192.0.2.1:45678" }},
		{"unparseable peer", func(req *http.Request) { req.RemoteAddr = "localhost:45678" }},
		{"host mismatch", func(req *http.Request) { req.Host = "127.0.0.1:8790" }},
		{"TLS is not launcher HTTP", func(req *http.Request) { req.TLS = &tls.ConnectionState{} }},
		{"forwarded loopback cannot replace peer", func(req *http.Request) {
			req.RemoteAddr = "192.0.2.1:45678"
			req.Header.Set("X-Forwarded-For", "127.0.0.1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := r.request(http.MethodPost, "claim", r.claimBody())
			tc.alter(req)
			f.serve(t, req, http.StatusForbidden)
			f.unchanged(t, r, before)
		})
	}
}

func TestScopedLoopbackRejectsLocalProxyIndicators(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	r := f.intent(t, "measure")
	before := f.persisted(t, r)
	for _, header := range []string{
		"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto",
		"X-Forwarded-Port", "X-Forwarded-Prefix", "X-Real-IP", "CF-Connecting-IP",
		"CF-Ray", "CF-Visitor", "CDN-Loop", "Via", "True-Client-IP",
	} {
		for _, value := range []string{"proxy-indicator", ""} {
			t.Run(header+"/"+map[bool]string{true: "empty", false: "present"}[value == ""], func(t *testing.T) {
				req := r.request(http.MethodPost, "claim", r.claimBody())
				req.Header.Set(header, value)
				f.serve(t, req, http.StatusForbidden)
				f.unchanged(t, r, before)
			})
		}
	}
}

func TestScopedLoopbackRequiresValidLauncherConfiguration(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	r := f.intent(t, "render")
	before := f.persisted(t, r)
	for _, origin := range []string{
		"", "https://127.0.0.1:8789", "http://localhost:8789", "http://192.0.2.1:8789",
		"http://127.0.0.1", "http://127.0.0.1:0", "http://127.0.0.1:65536",
		"http://user@127.0.0.1:8789", scopedLoopbackOrigin + "/", scopedLoopbackOrigin + "?query=1",
		scopedLoopbackOrigin + "?", scopedLoopbackOrigin + "#fragment", scopedLoopbackOrigin + "#",
	} {
		t.Run(origin, func(t *testing.T) {
			f.service.Supervisor.Origin = origin
			MountRender(f.tg.g, f.service, context.Background())
			f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusForbidden)
			f.unchanged(t, r, before)
		})
	}
	t.Run("no supervisor", func(t *testing.T) {
		f.service.Supervisor = nil
		MountRender(f.tg.g, f.service, context.Background())
		f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusForbidden)
		f.unchanged(t, r, before)
	})
}

func TestScopedLoopbackLauncherAuthorityFrozenAtMount(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	r := f.intent(t, "render")
	before := f.persisted(t, r)
	f.service.Supervisor.Origin = "http://127.0.0.1:8790"
	req := r.request(http.MethodPost, "claim", r.claimBody())
	req.Host = "127.0.0.1:8790"
	req.Header.Set("Origin", f.service.Supervisor.Origin)
	f.serve(t, req, http.StatusForbidden)
	f.unchanged(t, r, before)
	f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusOK)
	got := f.persisted(t, r).(render.ExportRequest)
	if got.Status != "running" || got.Fence == "" {
		t.Fatalf("mounted launcher authority changed after supervisor mutation: %+v", got)
	}
}

func TestScopedLoopbackDoesNotRelaxRequestCredentials(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			r := f.intent(t, kind)
			other := f.intent(t, kind)
			before := f.persisted(t, r)
			otherBefore := f.persisted(t, other)
			for _, tc := range []struct {
				name  string
				alter func(*http.Request)
			}{
				{"missing token", func(req *http.Request) { req.Header.Del("Authorization") }},
				{"invalid token", func(req *http.Request) { req.Header.Set("Authorization", "Bearer invalid-token") }},
				{"another request token", func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+other.token) }},
				{"wrong request kind", func(req *http.Request) {
					otherKind := map[string]string{"render": "measure", "measure": "render"}[kind]
					req.URL.Path = "/api/" + otherKind + "/" + r.id + "/claim"
				}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					req := r.request(http.MethodPost, "claim", r.claimBody())
					tc.alter(req)
					f.serve(t, req, http.StatusUnauthorized)
					f.unchanged(t, r, before)
				})
			}
			t.Run("expired token", func(t *testing.T) {
				now := f.now
				f.now = now.Add(render.RequestTokenTTL)
				defer func() { f.now = now }()
				f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusUnauthorized)
				f.unchanged(t, r, before)
			})
			t.Run("wrong nonce", func(t *testing.T) {
				f.serve(t, r.request(http.MethodPost, "claim", `{"nonce":"wrong","ownerId":"local-chrome"}`), http.StatusConflict)
				f.unchanged(t, r, before)
			})
			f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusOK)
			claimed := f.persisted(t, r)
			f.serve(t, r.request(http.MethodPost, "heartbeat", `{"fence":"wrong"}`), http.StatusConflict)
			f.unchanged(t, r, claimed)
			f.unchanged(t, other, otherBefore)
		})
	}
	t.Run("stale measurement binding", func(t *testing.T) {
		binding := f.binding
		binding.SettingsHash = "outdated-settings"
		created, err := f.service.Store.CreateMeasurement("job", "project", binding)
		if err != nil {
			t.Fatal(err)
		}
		r := scopedLoopbackIntent{"measure", created.ID, created.Token, created.Nonce}
		// Existing claim semantics return a persisted stale record, not a
		// conflict, when the server-derived project binding has changed.
		w := f.serve(t, r.request(http.MethodPost, "claim", r.claimBody()), http.StatusOK)
		got := f.persisted(t, r).(render.MeasurementRequest)
		if got.Status != "stale" || got.Fence != "" || got.OwnerID != "" || got.FinishedAt != f.now.Format(time.RFC3339Nano) || !quality.SameBinding(got.Binding, binding) {
			t.Fatalf("local origin bypassed stale measurement binding: %+v", got)
		}
		var response render.MeasurementRequest
		if err := json.Unmarshal(renderPublicRequest(t, w), &response); err != nil {
			t.Fatal(err)
		}
		if response.Status != "stale" || response.Fence != "" {
			t.Fatalf("stale claim response: %+v", response)
		}
	})
}

func TestScopedLoopbackMediaScopeAndHeaderlessReads(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			r := f.intent(t, kind)
			before := f.persisted(t, r)
			for _, headerless := range []bool{false, true} {
				req := r.request(http.MethodGet, "media/"+scopedProjectMedia+"?token="+url.QueryEscape(r.token), "")
				req.Header.Del("Authorization")
				if headerless {
					req.Header.Del("Origin")
					req.Header.Del("Sec-Fetch-Site")
					req.RemoteAddr = "192.0.2.1:45678"
				}
				w := f.serve(t, req, http.StatusOK)
				if w.Body.String() != "project" || w.Header().Get("Content-Type") != "image/webp" {
					t.Fatalf("valid scoped media read failed (headerless=%v): headers=%v body=%q", headerless, w.Header(), w.Body.String())
				}
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := f.serve(t, r.request(method, "media/"+scopedOtherMedia, ""), http.StatusUnauthorized)
				if strings.Contains(w.Body.String(), "other-project") {
					t.Fatal("cross-project source bytes leaked")
				}
			}
			f.unchanged(t, r, before)
		})
	}
}

func TestScopedLoopbackDoesNotAuthorizeOrdinaryRoutes(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	r := f.intent(t, "render")
	before := f.persisted(t, r)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/me?token=" + url.QueryEscape(r.token), http.StatusUnauthorized},
		{http.MethodGet, "/api/editor/projects", http.StatusUnauthorized},
		{http.MethodPost, "/api/editor/projects", http.StatusForbidden},
		{http.MethodGet, "/api/render-lookalike/" + r.id + "/claim", http.StatusUnauthorized},
		{http.MethodPost, "/api/render-lookalike/" + r.id + "/claim", http.StatusForbidden},
		{http.MethodGet, "/api/measure-lookalike/" + r.id + "/claim", http.StatusUnauthorized},
		{http.MethodPost, "/api/measure-lookalike/" + r.id + "/claim", http.StatusForbidden},
		{http.MethodPost, "/api/render", http.StatusForbidden},
		{http.MethodPost, "/api/clone/job/export", http.StatusForbidden},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := r.request(tc.method, "claim", r.claimBody())
			u, err := url.Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			req.URL = u
			f.serve(t, req, tc.status)
			f.unchanged(t, r, before)
		})
	}
}

func TestScopedPublicOriginStillUsesConfiguredGuard(t *testing.T) {
	f := newScopedLoopbackFixture(t)
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			r := f.intent(t, kind)
			before := f.persisted(t, r)
			req := r.request(http.MethodPost, "claim", r.claimBody())
			req.Host = "viderse.example.test"
			req.RemoteAddr = "192.0.2.1:45678"
			req.Header.Set("Origin", scopedPublicOrigin)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			f.serve(t, req, http.StatusForbidden)
			f.unchanged(t, r, before)
			req = r.request(http.MethodPost, "claim", r.claimBody())
			req.Host = "viderse.example.test"
			req.RemoteAddr = "192.0.2.1:45678"
			req.Header.Set("Origin", scopedPublicOrigin)
			req.Header.Set("Forwarded", "for=192.0.2.1;proto=https")
			w := f.serve(t, req, http.StatusOK)
			var response struct{ ID, Status, Fence string }
			if err := json.Unmarshal(renderPublicRequest(t, w), &response); err != nil {
				t.Fatal(err)
			}
			if response.ID != r.id || response.Status != "running" || response.Fence == "" || reflect.DeepEqual(f.persisted(t, r), before) {
				t.Fatalf("configured public origin lost normal scoped access: %+v", response)
			}
		})
	}
}
