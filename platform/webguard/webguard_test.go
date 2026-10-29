package webguard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func mustConfig(t *testing.T, trusted, publicURL string, secure bool) *Config {
	t.Helper()
	cidrs, err := ParseTrustedProxies(trusted)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", trusted, err)
	}
	cfg := &Config{Trusted: cidrs, SecureCookies: secure}
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
			r = cfg.Normalize(r)
			if got := ClientIP(r); got != tc.wantIP {
				t.Fatalf("ClientIP = %q, want %q", got, tc.wantIP)
			}
			if got := SecureRequest(r); got != tc.wantSecure {
				t.Fatalf("SecureRequest = %v, want %v", got, tc.wantSecure)
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
	if !SecureRequest(cfg.Normalize(r)) {
		t.Fatal("SECURE_COOKIES=true must mark the request secure")
	}
	// Direct HTTP (no trusted proxy) still yields a Secure cookie, so a
	// bypass that skips the edge proxy cannot downgrade the session.
	r2 := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r2.RemoteAddr = "198.51.100.4:1"
	if !SecureRequest(cfg.Normalize(r2)) {
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
	r = cfg.Normalize(r)
	if got := ClientIP(r); got != "127.0.0.1" {
		t.Fatalf("ClientIP = %q, loopback outside CIDRs must not supply XFF", got)
	}
	if SecureRequest(r) {
		t.Fatal("loopback outside CIDRs must not claim HTTPS through XFP")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if _, err := ParseTrustedProxies("0.0.0.0/0"); err == nil {
		t.Error("0.0.0.0/0 must be refused")
	}
	if _, err := ParseTrustedProxies("::/0"); err == nil {
		t.Error("::/0 must be refused")
	}
	if _, err := ParseTrustedProxies("172.18.0.0/33"); err == nil {
		t.Error("invalid CIDR must fail")
	}
	if _, err := ParseTrustedProxies("traefik"); err == nil {
		t.Error("non-IP must fail")
	}
	nets, err := ParseTrustedProxies("172.18.0.0/16 10.0.0.5")
	if err != nil {
		t.Fatalf("valid list: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("parsed %d networks, want 2", len(nets))
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("SECURE_COOKIES", "")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("empty env: %v", err)
	}
	if len(cfg.Trusted) != 0 || cfg.PublicURL != nil || cfg.SecureCookies {
		t.Fatal("empty env must yield an empty config")
	}

	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	if cfg, err = ConfigFromEnv(); err != nil || len(cfg.Trusted) != 1 {
		t.Fatalf("trusted CIDR: cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("TRUSTED_PROXIES", "")
	if cfg, err = ConfigFromEnv(); err != nil || len(cfg.Trusted) != 0 {
		t.Fatalf("empty trusted: cfg=%+v err=%v", cfg, err)
	}

	t.Setenv("PUBLIC_URL", "not a url")
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("relative PUBLIC_URL must fail")
	}
	t.Setenv("PUBLIC_URL", "ftp://x.example")
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("non-http PUBLIC_URL must fail")
	}
	t.Setenv("PUBLIC_URL", "https://app.example")
	if cfg, err = ConfigFromEnv(); err != nil {
		t.Fatalf("https PUBLIC_URL: %v", err)
	}

	t.Setenv("SECURE_COOKIES", "maybe")
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("invalid SECURE_COOKIES must fail")
	}
	t.Setenv("SECURE_COOKIES", "true")
	if cfg, err = ConfigFromEnv(); err != nil || !cfg.SecureCookies {
		t.Fatalf("SECURE_COOKIES=true with https PUBLIC_URL: cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("PUBLIC_URL", "http://app.example")
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("SECURE_COOKIES=true without https PUBLIC_URL must fail")
	}
}

func TestCrossSiteRequest(t *testing.T) {
	local := &Config{}
	post := func(origin, fetch string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/x", nil)
		r.Host = "127.0.0.1:8790"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if fetch != "" {
			r.Header.Set("Sec-Fetch-Site", fetch)
		}
		return r
	}
	if !local.CrossSiteRequest(post("null", "")) {
		t.Error("Origin: null must be rejected")
	}
	if !local.CrossSiteRequest(post("http://127.0.0.1:8789", "")) {
		t.Error("sibling app port must be rejected")
	}
	if !local.CrossSiteRequest(post("", "cross-site")) {
		t.Error("cross-site fetch metadata must be rejected")
	}
	if local.CrossSiteRequest(post("", "")) {
		t.Error("no Origin and no cross-site marker must pass")
	}
	if local.CrossSiteRequest(post("http://127.0.0.1:8790", "same-origin")) {
		t.Error("same-origin must pass")
	}
	get := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	get.Host = "127.0.0.1:8790"
	get.Header.Set("Origin", "http://127.0.0.1:8789")
	if local.CrossSiteRequest(get) {
		t.Error("GET is not a state change and must not be origin-checked")
	}
}

// Which same-host origins count as "this deployment". Local mode serves only
// on IP literals and localhost, so a domain name whose Host matches is a
// DNS-rebinding page. Hosted mode trusts PUBLIC_URL alone, not whatever Host
// the proxy forwarded.
func TestAllowedOriginHosts(t *testing.T) {
	local := &Config{}
	hosted := mustConfig(t, "10.0.0.0/8", "https://noverse.example", true)
	for _, tc := range []struct {
		name   string
		cfg    *Config
		host   string
		origin string
		want   bool
	}{
		{"local loopback", local, "127.0.0.1:8790", "http://127.0.0.1:8790", true},
		{"local localhost", local, "localhost:8790", "http://localhost:8790", true},
		{"local ipv6 loopback", local, "[::1]:8790", "http://[::1]:8790", true},
		{"phone on LAN address", local, "192.168.1.20:8790", "http://192.168.1.20:8790", true},
		{"localhost page posting to 127.0.0.1", local, "127.0.0.1:8790", "http://localhost:8790", false},
		{"sibling app port", local, "127.0.0.1:8790", "http://127.0.0.1:8789", false},
		{"dns rebinding name", local, "evil.example:8790", "http://evil.example:8790", false},
		{"hosted public url", hosted, "noverse.example", "https://noverse.example", true},
		{"hosted plain http", hosted, "noverse.example", "http://noverse.example", false},
		{"hosted forwarded host only", hosted, "internal.lan:8080", "http://internal.lan:8080", false},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/x", nil)
		r.Host = tc.host
		if got := tc.cfg.AllowedOrigin(r, tc.origin); got != tc.want {
			t.Errorf("%s: AllowedOrigin(Host=%s, Origin=%s) = %v, want %v", tc.name, tc.host, tc.origin, got, tc.want)
		}
	}
}
