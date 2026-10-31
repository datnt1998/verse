package webguard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// envConfig parses ConfigFromEnv under a controlled environment.
func envConfig(t *testing.T, vars map[string]string) (*Config, error) {
	t.Helper()
	for _, key := range []string{
		"TRUSTED_PROXIES", "PUBLIC_URL", "SECURE_COOKIES",
		"VERSE_COOKIE_DOMAIN", "VERSE_APP_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	for key, value := range vars {
		t.Setenv(key, value)
	}
	return ConfigFromEnv()
}

func mustEnvConfig(t *testing.T, vars map[string]string) *Config {
	t.Helper()
	cfg, err := envConfig(t, vars)
	if err != nil {
		t.Fatalf("ConfigFromEnv(%v): %v", vars, err)
	}
	return cfg
}

// TestCookieDomainConfig covers the strict VERSE_COOKIE_DOMAIN contract:
// canonicalisation, and every rejected shape (public suffix, single label,
// IP, wildcard, port, insecure deployment, domain outside PUBLIC_URL).
func TestCookieDomainConfig(t *testing.T) {
	hosted := map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true"}
	for _, tc := range []struct {
		name, domain string
		want         string
	}{
		{"bare domain", "example.com", "example.com"},
		{"leading dot canonicalised", ".example.com", "example.com"},
		{"uppercase lowered", "EXAMPLE.com", "example.com"},
	} {
		vars := map[string]string{"VERSE_COOKIE_DOMAIN": tc.domain}
		for k, v := range hosted {
			vars[k] = v
		}
		cfg, err := envConfig(t, vars)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if cfg.CookieDomain != tc.want {
			t.Fatalf("%s: CookieDomain = %q, want %q", tc.name, cfg.CookieDomain, tc.want)
		}
	}
	// Every DNS letter is allowed: only separators, whitespace and wildcards
	// are refused before the label check.
	for _, domain := range []string{"verse.net", "studio.test", "tet.vn"} {
		if got, err := ParseCookieDomain(domain); err != nil || got != domain {
			t.Fatalf("ParseCookieDomain(%q) = %q, %v; want the domain back", domain, got, err)
		}
	}
	if _, err := ParseCookieDomain("exa\tmple.com"); err == nil {
		t.Fatal("a tab inside the domain must be refused")
	}
	for _, tc := range []struct {
		name string
		vars map[string]string
	}{
		{"public suffix com", map[string]string{"PUBLIC_URL": "https://a.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "com"}},
		{"public suffix co.uk", map[string]string{"PUBLIC_URL": "https://a.co.uk", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "co.uk"}},
		{"single label", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "internal"}},
		{"ip literal", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "127.0.0.1"}},
		{"wildcard", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "*.example.com"}},
		{"port", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com:443"}},
		{"url not domain", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "https://example.com"}},
		{"does not contain public url", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "other.example.com"}},
		{"lookalike does not contain", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "evil-example.com"}},
		{"domain without secure cookies", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "VERSE_COOKIE_DOMAIN": "example.com"}},
		{"domain without public url", map[string]string{"SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com"}},
	} {
		if _, err := envConfig(t, tc.vars); err == nil {
			t.Fatalf("%s: expected config error", tc.name)
		}
	}
}

// TestAppOriginsConfig covers VERSE_APP_ORIGINS parsing: canonicalisation,
// dedup, and the hosted/local restrictions.
func TestAppOriginsConfig(t *testing.T) {
	cfg := mustEnvConfig(t, map[string]string{
		"PUBLIC_URL":        "https://noverse.example.com",
		"SECURE_COOKIES":    "true",
		"VERSE_APP_ORIGINS": "HTTPS://Viderse.Example.com/, https://noverse.example.com:443 ,https://viderse.example.com",
	})
	want := []string{"https://viderse.example.com", "https://noverse.example.com"}
	if !slices.Equal(cfg.AppOrigins, want) {
		t.Fatalf("AppOrigins = %v, want %v", cfg.AppOrigins, want)
	}
	local := mustEnvConfig(t, map[string]string{
		"VERSE_APP_ORIGINS": "http://127.0.0.1:18790, http://127.0.0.1:18789/",
	})
	if !local.LocalSSO() || local.Hosted() {
		t.Fatalf("local allowlist should mark LocalSSO: %+v", local)
	}
	for _, tc := range []struct {
		name string
		vars map[string]string
	}{
		{"wildcard", map[string]string{"VERSE_APP_ORIGINS": "*"}},
		{"null", map[string]string{"VERSE_APP_ORIGINS": "null"}},
		{"userinfo", map[string]string{"VERSE_APP_ORIGINS": "http://u@127.0.0.1:18790"}},
		{"path", map[string]string{"VERSE_APP_ORIGINS": "http://127.0.0.1:18790/app"}},
		{"query", map[string]string{"VERSE_APP_ORIGINS": "http://127.0.0.1:18790/?x=1"}},
		{"fragment", map[string]string{"VERSE_APP_ORIGINS": "http://127.0.0.1:18790/#a"}},
		{"scheme relative", map[string]string{"VERSE_APP_ORIGINS": "//127.0.0.1:18790"}},
		{"named host local", map[string]string{"VERSE_APP_ORIGINS": "http://devbox.local:18790"}},
		{"lan ip local", map[string]string{"VERSE_APP_ORIGINS": "http://192.168.1.5:18790"}},
		{"hosted http origin", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_APP_ORIGINS": "http://viderse.example.com"}},
		{"hosted origin outside domain", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com", "VERSE_APP_ORIGINS": "https://viderse.other.com"}},
		{"hosted origin suffix lookalike", map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com", "VERSE_APP_ORIGINS": "https://evil-example.com"}},
	} {
		if _, err := envConfig(t, tc.vars); err == nil {
			t.Fatalf("%s: expected config error", tc.name)
		}
	}
}

// TestSharedCookiePolicy pins the cookie attributes of the shared session
// contract: local host-only, hosted parent domain, HttpOnly + SameSite=Strict
// + Secure on HTTPS, and a clear that matches the set scope exactly.
func TestSharedCookiePolicy(t *testing.T) {
	expires := time.Now().Add(24 * time.Hour).UTC()

	t.Run("local host only", func(t *testing.T) {
		policy := NewSessionCookie("verse_session", &Config{})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:18790/api/me", nil)
		policy.Set(w, r, "cookie-value", expires)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("set emitted %d cookies", len(cookies))
		}
		c := cookies[0]
		if c.Name != "verse_session" || c.Value != "cookie-value" || c.Domain != "" || c.Path != "/" ||
			!c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Secure {
			t.Fatalf("local cookie attributes: %+v", c)
		}
		w = httptest.NewRecorder()
		policy.Clear(w, r)
		clears := w.Result().Cookies()
		if len(clears) != 1 || clears[0].Name != "verse_session" || clears[0].Domain != "" ||
			clears[0].Path != "/" || clears[0].MaxAge > 0 || clears[0].Expires.After(time.Now()) {
			t.Fatalf("clear must match host-only scope: %+v", clears)
		}
	})

	t.Run("hosted parent domain", func(t *testing.T) {
		cfg := mustEnvConfig(t, map[string]string{
			"PUBLIC_URL":          "https://noverse.example.com",
			"SECURE_COOKIES":      "true",
			"VERSE_COOKIE_DOMAIN": ".example.com",
		})
		policy := NewSessionCookie("verse_session", cfg)
		w := httptest.NewRecorder()
		policy.Set(w, secureReq(), "cookie-value", expires)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("set emitted %d cookies", len(cookies))
		}
		c := cookies[0]
		if c.Domain != "example.com" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
			t.Fatalf("hosted cookie attributes: %+v", c)
		}
		w = httptest.NewRecorder()
		policy.Clear(w, secureReq())
		clears := w.Result().Cookies()
		var domainClear *http.Cookie
		for _, c := range clears {
			if c.Domain == "example.com" {
				domainClear = c
			}
			if c.MaxAge > 0 || !c.Secure || c.Expires.After(time.Now()) {
				t.Fatalf("clear must expire securely: %+v", c)
			}
		}
		if domainClear == nil {
			t.Fatalf("clear must expire the domain scope: %+v", clears)
		}
	})

	t.Run("legacy names expire at both scopes", func(t *testing.T) {
		cfg := mustEnvConfig(t, map[string]string{
			"PUBLIC_URL":          "https://noverse.example.com",
			"SECURE_COOKIES":      "true",
			"VERSE_COOKIE_DOMAIN": "example.com",
		})
		policy := NewSessionCookie("verse_session", cfg)
		w := httptest.NewRecorder()
		policy.Expire(w, secureReq(), "ainovel_session")
		policy.Expire(w, secureReq(), "viderse_session")
		var hostScoped, domainScoped int
		for _, c := range w.Result().Cookies() {
			if c.MaxAge != -1 || !c.Expires.Before(time.Now()) {
				t.Fatalf("expiry must be in the past: %+v", c)
			}
			if c.Domain == "example.com" {
				domainScoped++
			} else {
				hostScoped++
			}
		}
		if hostScoped != 2 || domainScoped != 2 {
			t.Fatalf("want 2 host-only + 2 domain expirations, got %d/%d", hostScoped, domainScoped)
		}
	})

	t.Run("secure cannot be spoofed by headers", func(t *testing.T) {
		policy := NewSessionCookie("verse_session", &Config{})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:18790/api/me", nil)
		r.RemoteAddr = "198.51.100.4:9"
		r.Header.Set("X-Forwarded-Proto", "https")
		policy.Set(w, r, "v", expires)
		if c := w.Result().Cookies()[0]; c.Secure {
			t.Fatal("forged X-Forwarded-Proto marked the cookie Secure")
		}
	})
}

// secureReq builds a normalized request the boundary treats as HTTPS.
func secureReq() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "https://noverse.example.com/api/me", nil)
	return (&Config{SecureCookies: true}).Normalize(r)
}

// TestSSOOriginAllowlistMatrix runs the accept/reject matrix against the
// config-level rule: exact allowlist entries pass, every lookalike fails, and
// cross-site metadata is refused even for an allowlisted origin.
func TestSSOOriginAllowlistMatrix(t *testing.T) {
	local := mustEnvConfig(t, map[string]string{
		"VERSE_APP_ORIGINS": "http://127.0.0.1:18790,http://127.0.0.1:18789",
	})
	req := func(origin, fetch string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18789/api/login", strings.NewReader(`{}`))
		r.Host = "127.0.0.1:18789"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if fetch != "" {
			r.Header.Set("Sec-Fetch-Site", fetch)
		}
		return r
	}
	for _, tc := range []struct {
		name   string
		origin string
		want   bool
	}{
		{"own origin", "http://127.0.0.1:18789", true},
		{"allowlisted sibling port", "http://127.0.0.1:18790", true},
		{"allowlisted trailing slash canonicalised", "http://127.0.0.1:18790/", true},
		{"allowlisted case canonicalised", "HTTP://127.0.0.1:18790", true},
		{"non allowlisted sibling port", "http://127.0.0.1:18791", false},
		{"lookalike suffix host", "http://127.0.0.1.evil.example:18790", false},
		{"null origin", "null", false},
		{"foreign origin", "https://evil.example", false},
		{"userinfo smuggle", "http://u@127.0.0.1:18790", false},
		{"path smuggle", "http://127.0.0.1:18790/x", false},
		{"scheme relative", "//127.0.0.1:18790", false},
		{"malformed", "ht!tp://[", false},
		{"localhost vs ip", "http://localhost:18790", false},
	} {
		r := req(tc.origin, "same-site")
		if got := local.AllowedOrigin(r, tc.origin); got != tc.want {
			t.Fatalf("%s: AllowedOrigin(%q) = %v, want %v", tc.name, tc.origin, got, tc.want)
		}
	}
	// Cross-site metadata always rejects, even for an allowlisted origin.
	if !local.CrossSiteRequest(req("http://127.0.0.1:18790", "cross-site")) {
		t.Fatal("cross-site metadata must reject an allowlisted origin")
	}
	// Same-site / same-origin metadata on an allowlisted origin passes.
	if local.CrossSiteRequest(req("http://127.0.0.1:18790", "same-site")) {
		t.Fatal("allowlisted same-site request rejected")
	}
	if local.CrossSiteRequest(req("http://127.0.0.1:18790", "same-origin")) {
		t.Fatal("allowlisted same-origin request rejected")
	}
	// Missing Origin keeps the non-browser convention.
	if local.CrossSiteRequest(req("", "")) {
		t.Fatal("request without Origin rejected")
	}
	// A request without Origin still rejects when the browser marks it.
	if !local.CrossSiteRequest(req("", "cross-site")) {
		t.Fatal("cross-site metadata without Origin must reject")
	}

	hosted := mustEnvConfig(t, map[string]string{
		"PUBLIC_URL":          "https://noverse.example.com",
		"SECURE_COOKIES":      "true",
		"VERSE_COOKIE_DOMAIN": "example.com",
		"VERSE_APP_ORIGINS":   "https://viderse.example.com",
	})
	hr := func(origin string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "https://noverse.example.com/api/login", strings.NewReader(`{}`))
		r.Host = "noverse.example.com"
		r.Header.Set("Origin", origin)
		return r
	}
	for _, tc := range []struct {
		name   string
		origin string
		want   bool
	}{
		{"own public url", "https://noverse.example.com", true},
		{"allowlisted sibling app", "https://viderse.example.com", true},
		{"other subdomain not allowlisted", "https://other.example.com", false},
		{"http downgrade of sibling", "http://viderse.example.com", false},
		{"sibling with foreign port", "https://viderse.example.com:8443", false},
		{"lookalike domain", "https://noverse.evil-example.com", false},
	} {
		r := hr(tc.origin)
		if got := hosted.AllowedOrigin(r, tc.origin); got != tc.want {
			t.Fatalf("%s: AllowedOrigin(%q) = %v, want %v", tc.name, tc.origin, got, tc.want)
		}
	}
	// Hosted public URL still wins for its own origin even when also listed.
	if hosted.AppOrigins == nil {
		t.Fatal("hosted allowlist not parsed")
	}
	if u, _ := url.Parse(hosted.AppOrigins[0]); u.Scheme != "https" {
		t.Fatal("hosted allowlist must stay https")
	}
}
