package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/webguard"
)

// Hosted runs bind 0.0.0.0 inside the container and are reached through the
// proxy: PUBLIC_URL is the only address a member can open. The pairing link
// must therefore be that address alone — never the container's private
// interface (172.x) that the browser cannot reach.
func TestPairingLinkIsThePublicURLOnly(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://nov.aarverse.studio")
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := webguard.ConfigFromEnv()
	if err != nil {
		t.Fatalf("hosted proxy config: %v", err)
	}
	tg := newTestGateway(t)
	// serve builds the gateway exactly this way: proxy contract installed at
	// startup, listener bound to 0.0.0.0.
	tg.g.SetProxyConfig(cfg)
	admin := tg.login("admin", "admin-pass")

	r := httptest.NewRequest("POST", "/api/me/pairings", strings.NewReader("{}"))
	r.RemoteAddr = "172.30.200.1:4444"
	r.Host = "172.30.100.3:8080"
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: admin})
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("pairing: %d %s", w.Code, w.Body.String())
	}
	var p struct {
		Code  string `json:"code"`
		Links []struct {
			URL string `json:"url"`
		} `json:"links"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("pairing body %q: %v", w.Body.String(), err)
	}
	if len(p.Links) != 1 {
		t.Fatalf("want exactly one pairing link, got %+v", p.Links)
	}
	if want := "https://nov.aarverse.studio/p/" + p.Code; p.Links[0].URL != want {
		t.Fatalf("pairing link = %q, want %q", p.Links[0].URL, want)
	}
}
