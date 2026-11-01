package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/datnt1998/verse/platform/webguard"
)

// TestSecurityHeadersOnEveryResponse pins the shared header set on the Noverse
// gateway: health, the API, the SPA, a 401 and the worker-facing 404 all carry
// it, and /robots.txt is answered instead of the SPA fallback.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	g := &Gateway{}
	for _, path := range []string{"/healthz", "/readyz", "/api/me", "/", "/assets/missing.js", "/khong-co"} {
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		for name, want := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "same-origin",
			"X-Frame-Options":         "DENY",
			"Content-Security-Policy": "frame-ancestors 'none'",
			"X-Robots-Tag":            "noindex, nofollow",
		} {
			if got := rec.Header().Get(name); got != want {
				t.Fatalf("%s: %s = %q, muốn %q", path, name, got, want)
			}
		}
		if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
			t.Fatalf("%s: HSTS %q trên deployment http", path, got)
		}
	}

	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("robots = %d %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("robots content-type = %q", ct)
	}
}

// TestHSTSOnlyOnHTTPSDeployment pins the one conditional header: HSTS is added
// only when PUBLIC_URL is https — never for a plain-http local/desktop run,
// where it would pin a loopback host to https and lock the user out.
func TestHSTSOnlyOnHTTPSDeployment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		publicURL *url.URL
		want      string
	}{
		{"https", &url.URL{Scheme: "https", Host: "nov.example"}, "max-age=15552000"},
		{"http", &url.URL{Scheme: "http", Host: "127.0.0.1:8686"}, ""},
		{"unset", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &Gateway{}
			g.SetProxyConfig(&webguard.Config{PublicURL: tc.publicURL})
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if got := rec.Header().Get("Strict-Transport-Security"); got != tc.want {
				t.Fatalf("HSTS = %q, muốn %q", got, tc.want)
			}
		})
	}
}
