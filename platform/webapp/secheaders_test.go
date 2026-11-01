package webapp_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webguard"
)

// TestSecurityHeadersOnEveryResponse pins the shared header set on the Viderse
// server: health, the API, the SPA and its static files all carry it, and
// /robots.txt is served instead of the SPA fallback.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	ts := newTestServer(t, true)
	for _, path := range []string{"/healthz", "/readyz", "/api/me", "/", "/assets/app-abc123.js", "/khong-co"} {
		w := ts.do(http.MethodGet, path, "", "")
		for name, want := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "same-origin",
			"X-Frame-Options":         "DENY",
			"Content-Security-Policy": "frame-ancestors 'none'",
			"X-Robots-Tag":            "noindex, nofollow",
		} {
			if got := w.Header().Get(name); got != want {
				t.Fatalf("%s: %s = %q, muốn %q", path, name, got, want)
			}
		}
		if got := w.Header().Get("Strict-Transport-Security"); got != "" {
			t.Fatalf("%s: HSTS %q trên deployment http", path, got)
		}
	}

	w := ts.do(http.MethodGet, "/robots.txt", "", "")
	if w.Code != http.StatusOK || w.Body.String() != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("robots = %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("robots content-type = %q", ct)
	}
}

// TestHSTSOnlyOnHTTPSDeployment pins the one conditional header: HSTS is added
// only when PUBLIC_URL is https, never on a plain-http local or desktop run.
func TestHSTSOnlyOnHTTPSDeployment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		publicURL *url.URL
		want      string
	}{
		{"https", &url.URL{Scheme: "https", Host: "vid.example"}, "max-age=15552000"},
		{"http", &url.URL{Scheme: "http", Host: "127.0.0.1:8789"}, ""},
		{"unset", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t, true)
			ts.s.SetProxyConfig(&webguard.Config{PublicURL: tc.publicURL})
			w := ts.do(http.MethodGet, "/healthz", "", "")
			if got := w.Header().Get("Strict-Transport-Security"); got != tc.want {
				t.Fatalf("HSTS = %q, muốn %q", got, tc.want)
			}
		})
	}
}

// TestSecurityHeadersDoNotOverrideStricterOnes pins the boundary with the
// routes that set their own: a handler that Sets later wins, so blob/render
// responses keep their sandbox CSP and no-referrer.
func TestSecurityHeadersDoNotOverrideStricterOnes(t *testing.T) {
	ts := newTestServer(t, true)
	cookie := ts.sessionCookie()
	ts.s.Handle(http.MethodGet, "/api/strict", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.WriteHeader(http.StatusNoContent)
	})
	w := ts.do(http.MethodGet, "/api/strict", cookie, "")
	if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
		t.Fatalf("CSP bị ghi đè: %q", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy bị ghi đè: %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff thiếu: %q", got)
	}
}
