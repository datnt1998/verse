package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/webguard"
)

func mustConfig(t *testing.T, trusted, publicURL string, secure bool) *webguard.Config {
	t.Helper()
	cidrs, err := webguard.ParseTrustedProxies(trusted)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", trusted, err)
	}
	cfg := &webguard.Config{Trusted: cidrs, SecureCookies: secure}
	if publicURL != "" {
		u, err := url.Parse(publicURL)
		if err != nil {
			t.Fatal(err)
		}
		cfg.PublicURL = u
	}
	return cfg
}

// TestStateChangingRequestsRequireSameOrigin runs method × mode × Origin ×
// Sec-Fetch-Site through ServeHTTP. Only a same-origin request, or one without
// Origin that the browser does not mark cross-site, reaches the session check
// (401 without a cookie); every other combination is 403 before any handler.
// "other port" is the sibling-app case SameSite=Strict cannot stop.
func TestStateChangingRequestsRequireSameOrigin(t *testing.T) {
	tg := newTestGateway(t)
	modes := []struct {
		name, host                 string
		cfg                        *webguard.Config
		same, otherPort, otherHost string
	}{
		{"local", "127.0.0.1:8790", &webguard.Config{}, "http://127.0.0.1:8790", "http://127.0.0.1:8789", "http://evil.example"},
		{"hosted", "noverse.example", mustConfig(t, "10.0.0.0/8", "https://noverse.example", true), "https://noverse.example", "https://noverse.example:8443", "https://evil.example"},
	}
	for _, mode := range modes {
		tg.g.SetProxyConfig(mode.cfg)
		origins := []struct{ name, value string }{{"same origin", mode.same}, {"other port", mode.otherPort}, {"other host", mode.otherHost}, {"no origin", ""}}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			for _, origin := range origins {
				for _, fetch := range []string{"same-origin", "same-site", "cross-site", ""} {
					r := httptest.NewRequest(method, "/api/projects", strings.NewReader(`{}`))
					r.Host = mode.host
					r.Header.Set("Content-Type", "application/json")
					if origin.value != "" {
						r.Header.Set("Origin", origin.value)
					}
					if fetch != "" {
						r.Header.Set("Sec-Fetch-Site", fetch)
					}
					w := httptest.NewRecorder()
					tg.g.ServeHTTP(w, r)
					want := http.StatusForbidden
					if fetch != "cross-site" && (origin.value == "" || origin.value == mode.same) {
						want = http.StatusUnauthorized
					}
					if w.Code != want {
						t.Errorf("%s %s origin=%s sec-fetch-site=%q: status %d, want %d", mode.name, method, origin.name, fetch, w.Code, want)
					}
				}
			}
		}
	}
}

// PATCH and DELETE bodies get the JSON contract POST and PUT already had; a
// bodiless DELETE (the normal form) still reaches the session check.
func TestPatchAndDeleteBodiesMustBeJSON(t *testing.T) {
	tg := newTestGateway(t)
	for _, tc := range []struct {
		method, contentType, body string
		want                      int
	}{
		{http.MethodPatch, "text/plain", "x", http.StatusBadRequest},
		{http.MethodDelete, "text/plain", "x", http.StatusBadRequest},
		{http.MethodPatch, "application/json", `{}`, http.StatusUnauthorized},
		{http.MethodDelete, "application/json", `{}`, http.StatusUnauthorized},
		{http.MethodDelete, "", "", http.StatusUnauthorized},
	} {
		r := httptest.NewRequest(tc.method, "/api/projects/proj", strings.NewReader(tc.body))
		if tc.contentType != "" {
			r.Header.Set("Content-Type", tc.contentType)
		}
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %q body=%q: status %d, want %d", tc.method, tc.contentType, tc.body, w.Code, tc.want)
		}
	}
}
