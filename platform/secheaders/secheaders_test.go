package secheaders

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/datnt1998/verse/platform/webguard"
)

// TestApplyWritesTheHeaderSet pins the headers every public response carries.
func TestApplyWritesTheHeaderSet(t *testing.T) {
	rec := httptest.NewRecorder()
	Policy{}.Apply(rec)
	for name, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "same-origin",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Robots-Tag":            "noindex, nofollow",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Fatalf("%s = %q, muốn %q", name, got, want)
		}
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS = %q trên policy không https", got)
	}
}

// TestApplyLeavesStricterHeadersAlone pins the boundary with blob delivery and
// render routes: a header already present (a sandbox CSP, no-referrer) is not
// downgraded.
func TestApplyLeavesStricterHeadersAlone(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	rec.Header().Set("Referrer-Policy", "no-referrer")
	Policy{HSTS: true}.Apply(rec)
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
		t.Fatalf("CSP = %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=15552000" {
		t.Fatalf("HSTS = %q", got)
	}
}

// TestFromProxyPinsHSTSToHTTPS pins the only conditional decision.
func TestFromProxyPinsHSTSToHTTPS(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  *url.URL
		want bool
	}{
		{"https", &url.URL{Scheme: "https", Host: "app.example"}, true},
		{"http", &url.URL{Scheme: "http", Host: "127.0.0.1:8789"}, false},
		{"unset", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromProxy(&webguard.Config{PublicURL: tc.url}); got.HSTS != tc.want {
				t.Fatalf("HSTS = %v, muốn %v", got.HSTS, tc.want)
			}
		})
	}
	if got := FromProxy(nil); got.HSTS {
		t.Fatal("config nil không được bật HSTS")
	}
}

// TestRobotsServesDisallowAll pins the crawler policy: both apps are
// invitation-only, and the SPA fallback must not answer /robots.txt.
func TestRobotsServesDisallowAll(t *testing.T) {
	rec := httptest.NewRecorder()
	if !(Policy{}).Robots(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil)) {
		t.Fatal("phải nhận /robots.txt")
	}
	if rec.Body.String() != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}

	rec = httptest.NewRecorder()
	if !(Policy{}).Robots(rec, httptest.NewRequest(http.MethodHead, "/robots.txt", nil)) {
		t.Fatal("HEAD /robots.txt phải nhận")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD trả body: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	if !(Policy{}).Robots(rec, httptest.NewRequest(http.MethodPost, "/robots.txt", nil)) {
		t.Fatal("phải nhận /robots.txt")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /robots.txt = %d", rec.Code)
	}

	if (Policy{}).Robots(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/me", nil)) {
		t.Fatal("không được nhận đường khác")
	}
}
