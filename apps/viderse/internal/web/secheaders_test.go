package web

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/datnt1998/verse/platform/webguard"
)

// TestGatewayResponsesCarrySecurityHeaders pins the header set on the Viderse
// gateway itself (the layer above platform/webapp): health, the API, the SPA and
// /robots.txt all carry it, and HSTS appears only on an https deployment.
func TestGatewayResponsesCarrySecurityHeaders(t *testing.T) {
	tg := newTestGateway(t)
	tg.g.SetProxyConfig(&webguard.Config{})
	for _, path := range []string{"/healthz", "/readyz", "/api/me", "/", "/assets/missing.js", "/khong-co"} {
		w := tg.do(http.MethodGet, path, "", "")
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

	w := tg.do(http.MethodGet, "/robots.txt", "", "")
	if w.Code != http.StatusOK || w.Body.String() != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("robots = %d %q", w.Code, w.Body.String())
	}

	tg.g.SetProxyConfig(&webguard.Config{PublicURL: &url.URL{Scheme: "https", Host: "vid.example"}})
	w = tg.do(http.MethodGet, "/healthz", "", "")
	if got := w.Header().Get("Strict-Transport-Security"); got != "max-age=15552000" {
		t.Fatalf("HSTS = %q khi PUBLIC_URL https", got)
	}
}

// TestRenderBypassCarriesSecurityHeaders pins the one path that skips
// webapp.Server.ServeHTTP: /api/render/* must not answer without nosniff and
// frame-ancestors, and the render handler's own stricter headers still win.
func TestRenderBypassCarriesSecurityHeaders(t *testing.T) {
	tg := newTestGateway(t)
	tg.g.SetProxyConfig(&webguard.Config{})
	tg.g.scopedRender = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("render"))
	})

	w := tg.do(http.MethodGet, "/api/render/abc", "", "")
	if w.Code != http.StatusOK || w.Body.String() != "render" {
		t.Fatalf("render = %d %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff = %q", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy bị hạ cấp: %q", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
		t.Fatalf("CSP của render bị ghi đè: %q", got)
	}
}
