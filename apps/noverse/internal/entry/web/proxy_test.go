package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func mustConfig(t *testing.T, trusted, publicURL string, secure bool) *ProxyConfig {
	t.Helper()
	cidrs, err := ParseTrustedProxies(trusted)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", trusted, err)
	}
	cfg := &ProxyConfig{Trusted: cidrs, SecureCookies: secure}
	if publicURL != "" {
		u, err := url.Parse(publicURL)
		if err != nil {
			t.Fatal(err)
		}
		cfg.PublicURL = u
	}
	return cfg
}

// TestTrustedProxyNormalization covers the boundary: headers are only honoured
// from peers inside TRUSTED_PROXIES, and the client IP is the first untrusted
// hop walking XFF right-to-left.
func TestTrustedProxyNormalization(t *testing.T) {
	cfg := mustConfig(t, "10.0.0.0/8, 2001:db8::/32", "", false)
	for _, tc := range []struct {
		name       string
		remoteAddr string
		xff        []string
		xfp        string
		wantIP     string
		wantSecure bool
	}{
		{
			name:       "single trusted proxy adds one hop",
			remoteAddr: "10.1.2.3:5000",
			xff:        []string{"203.0.113.9"},
			xfp:        "https",
			wantIP:     "203.0.113.9",
			wantSecure: true,
		},
		{
			name:       "chain of trusted proxies walks to first untrusted",
			remoteAddr: "10.0.0.5:1",
			xff:        []string{"203.0.113.9, 10.9.9.9, 10.1.1.1"},
			xfp:        "https",
			wantIP:     "203.0.113.9",
			wantSecure: true,
		},
		{
			name:       "same hop in two headers",
			remoteAddr: "10.0.0.5:1",
			xff:        []string{"203.0.113.9", "10.1.1.1"},
			xfp:        "https",
			wantIP:     "203.0.113.9",
			wantSecure: true,
		},
		{
			name:       "untrusted peer ignores forwarded headers",
			remoteAddr: "198.51.100.7:9",
			xff:        []string{"203.0.113.9"},
			xfp:        "https",
			wantIP:     "198.51.100.7",
			wantSecure: false,
		},
		{
			name:       "malformed xff stops at the trusted peer",
			remoteAddr: "10.1.2.3:5",
			xff:        []string{"203.0.113.9, not-an-ip"},
			xfp:        "https",
			wantIP:     "10.1.2.3",
			wantSecure: true,
		},
		{
			name:       "multiple proto values are discarded",
			remoteAddr: "10.1.2.3:5",
			xff:        []string{"203.0.113.9"},
			xfp:        "https, http",
			wantIP:     "203.0.113.9",
			wantSecure: false,
		},
		{
			name:       "unknown proto value is discarded",
			remoteAddr: "10.1.2.3:5",
			xff:        []string{"203.0.113.9"},
			xfp:        "wss",
			wantIP:     "203.0.113.9",
			wantSecure: false,
		},
		{
			name:       "ipv6 trusted proxy and client",
			remoteAddr: "[2001:db8::1]:443",
			xff:        []string{"2001:db8:1::7"},
			xfp:        "https",
			wantIP:     "2001:db8:1::7",
			wantSecure: true,
		},
		{
			name:       "ipv6 untrusted peer ignored",
			remoteAddr: "[2001:db9::1]:443",
			xff:        []string{"203.0.113.9"},
			xfp:        "https",
			wantIP:     "2001:db9::1",
			wantSecure: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			r.RemoteAddr = tc.remoteAddr
			for _, h := range tc.xff {
				r.Header.Add("X-Forwarded-For", h)
			}
			if tc.xfp != "" {
				r.Header.Set("X-Forwarded-Proto", tc.xfp)
			}
			r = cfg.normalize(r)
			if got := clientIP(r); got != tc.wantIP {
				t.Fatalf("clientIP = %q, want %q", got, tc.wantIP)
			}
			if got := secureRequest(r); got != tc.wantSecure {
				t.Fatalf("secureRequest = %v, want %v", got, tc.wantSecure)
			}
		})
	}
}

// TestTrustedProxySecureCookies pins the hosted policy: SECURE_COOKIES forces
// the flag even when a request reaches the container over plain HTTP (the
// only public entrance is TLS).
func TestTrustedProxySecureCookies(t *testing.T) {
	cfg := mustConfig(t, "10.0.0.0/8", "https://noverse.example", true)
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.RemoteAddr = "10.0.0.9:1"
	if !secureRequest(cfg.normalize(r)) {
		t.Fatal("SECURE_COOKIES=true must mark the request secure")
	}
	// Direct HTTP (no trusted proxy) still yields a Secure cookie, so a
	// bypass that skips Traefik cannot downgrade the session.
	r2 := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r2.RemoteAddr = "198.51.100.4:1"
	if !secureRequest(cfg.normalize(r2)) {
		t.Fatal("SECURE_COOKIES=true must not depend on peer trust")
	}
}

// TestTrustedProxiesReplaceLoopbackRule pins the env contract: when
// TRUSTED_PROXIES is set, a loopback peer outside the list is no longer
// trusted for X-Forwarded-* (the old implicit loopback rule is replaced, not
// OR-ed).
func TestTrustedProxiesReplaceLoopbackRule(t *testing.T) {
	cfg := mustConfig(t, "172.16.0.0/12", "", false)
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r = cfg.normalize(r)
	if got := clientIP(r); got != "127.0.0.1" {
		t.Fatalf("clientIP = %q, loopback outside CIDRs must not supply XFF", got)
	}
	if secureRequest(r) {
		t.Fatal("loopback outside CIDRs must not claim HTTPS through XFP")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if _, err := ParseTrustedProxies("0.0.0.0/0"); err == nil {
		t.Fatal("wildcard trust 0.0.0.0/0 must be rejected")
	}
	if _, err := ParseTrustedProxies("::/0"); err == nil {
		t.Fatal("wildcard trust ::/0 must be rejected")
	}
	if _, err := ParseTrustedProxies("172.18.0.0/33"); err == nil {
		t.Fatal("invalid prefix must be rejected")
	}
	if _, err := ParseTrustedProxies("traefik"); err == nil {
		t.Fatal("hostname must be rejected (CIDR only)")
	}
	nets, err := ParseTrustedProxies("172.18.0.0/16 10.0.0.5")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 2 {
		t.Fatalf("want 2 networks, got %d", len(nets))
	}
	if ones, _ := nets[1].Mask.Size(); ones != 32 {
		t.Fatalf("bare IP must become /32, got /%d", ones)
	}
	if nets[1].Contains(parseRemoteIP("10.0.0.5:1")) == false {
		t.Fatal("bare IP entry must contain itself")
	}
}

func TestProxyConfigFromEnv(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("SECURE_COOKIES", "")
	cfg, err := ProxyConfigFromEnv()
	if err != nil || cfg != nil && len(cfg.Trusted) != 0 {
		t.Fatalf("empty env: cfg=%+v err=%v", cfg, err)
	}

	// One name only: TRUSTED_PROXIES is the contract.
	t.Setenv("TRUSTED_PROXIES", "172.18.0.0/16")
	if cfg, err = ProxyConfigFromEnv(); err != nil || len(cfg.Trusted) != 1 {
		t.Fatalf("TRUSTED_PROXIES: cfg=%+v err=%v", cfg, err)
	}

	// The removed alias must never silently widen trust.
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.18.0.0/16")
	if cfg, err = ProxyConfigFromEnv(); err != nil || len(cfg.Trusted) != 0 {
		t.Fatalf("TRUSTED_PROXY_CIDRS must be ignored: cfg=%+v err=%v", cfg, err)
	}

	t.Setenv("SECURE_COOKIES", "true")
	if _, err := ProxyConfigFromEnv(); err == nil {
		t.Fatal("SECURE_COOKIES without https PUBLIC_URL must fail startup")
	}
	t.Setenv("PUBLIC_URL", "http://noverse.example")
	if _, err := ProxyConfigFromEnv(); err == nil {
		t.Fatal("SECURE_COOKIES with http PUBLIC_URL must fail startup")
	}
	t.Setenv("PUBLIC_URL", "https://noverse.example")
	if _, err := ProxyConfigFromEnv(); err != nil {
		t.Fatalf("valid hosted config: %v", err)
	}
	t.Setenv("SECURE_COOKIES", "yes")
	if _, err := ProxyConfigFromEnv(); err == nil {
		t.Fatal("non-strict boolean must fail")
	}
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
		cfg                        *ProxyConfig
		same, otherPort, otherHost string
	}{
		{"local", "127.0.0.1:8790", &ProxyConfig{}, "http://127.0.0.1:8790", "http://127.0.0.1:8789", "http://evil.example"},
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

	cfg := &ProxyConfig{}
	null := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	null.Host = "127.0.0.1:8790"
	null.Header.Set("Origin", "null")
	if !cfg.crossSiteRequest(null) {
		t.Error("Origin: null must be rejected")
	}
	get := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	get.Host = "127.0.0.1:8790"
	get.Header.Set("Origin", "http://127.0.0.1:8789")
	if cfg.crossSiteRequest(get) {
		t.Error("GET is not a state change and must not be origin-checked")
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
