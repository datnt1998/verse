package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
	"github.com/datnt1998/verse/platform/webguard"
	"github.com/jackc/pgx/v5"
)

func TestSSOSessionEndedCrossApp(t *testing.T) {
	for _, kind := range []string{"logout", "password_changed", "admin_changed", "reset", "locked", "expired"} {
		t.Run(kind, func(t *testing.T) {
			p := newPGGateway(t)
			p.fx.CreateMember(t, "member", "member-pass-99", false, identity.AppNoverse, identity.AppViderse)
			if _, err := p.fx.Admin.CreateUser(identity.NewUser{Username: "boss", Password: "boss-pass-99", Role: identity.RoleAdmin}); err != nil {
				t.Fatal(err)
			}
			actor := &identity.AuditActor{Username: "boss", App: identity.AppNoverse}
			ns, err := p.fx.Accounts.Login("member", "member-pass-99", "device", "")
			if err != nil {
				t.Fatal(err)
			}
			nov := webapp.New(p.fx.Admin, webapp.Options{App: identity.AppNoverse, AppName: "Noverse"})
			post := func(path, body string) {
				r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				r.AddCookie(&http.Cookie{Name: "verse_session", Value: ns.Cookie})
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				nov.ServeHTTP(w, r)
				if w.Code >= 300 {
					t.Fatalf("mutation %d %s", w.Code, w.Body.String())
				}
			}
			switch kind {
			case "logout":
				post("/api/logout", `{}`)
			case "password_changed":
				post("/api/me/password", `{"current":"member-pass-99","new":"new-member-pass-99"}`)
			case "admin_changed":
				err = p.fx.Admin.RevokeAppGrant("member", identity.AppNoverse, actor, "")
			case "reset":
				pw := "reset-member-pass-99"
				_, err = p.fx.Admin.UpdateUser("member", identity.UserPatch{Password: &pw, Actor: actor})
			case "locked":
				disabled := true
				_, err = p.fx.Admin.UpdateUser("member", identity.UserPatch{Disabled: &disabled, Actor: actor})
			case "expired":
				c, e := pgx.Connect(context.Background(), p.fx.AdminDSN)
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close(context.Background())
				_, err = c.Exec(context.Background(), `UPDATE sessions SET created_at=now()-interval '2 seconds', expires_at=now()-interval '1 second', updated_at=clock_timestamp(), version=version+1`)
			}
			if err != nil {
				t.Fatal(err)
			}
			w := p.do(http.MethodGet, "/api/me", ns.Cookie, "")
			b := decode(t, w)
			want := kind
			if kind == "reset" {
				want = "admin_changed"
			}
			if w.Code != 401 || b["reason"] != want || b["code"] != "session_ended" {
				t.Fatalf("ended status=%d body=%v want reason=%s", w.Code, b, want)
			}
			if kind != "expired" {
				if b["app"] != "noverse" {
					t.Fatalf("revoking app=%v", b["app"])
				}
			}
			// Knowing a real (revoked) session ID without its secret reveals nothing.
			id, _, _ := strings.Cut(ns.Cookie, ".")
			for _, token := range []string{"garbage", id + "." + strings.Repeat("0", 64)} {
				b = decode(t, p.do(http.MethodGet, "/api/me", token, ""))
				if b["reason"] != "unknown" || b["app"] != nil || b["user"] != nil {
					t.Fatalf("foreign token leaked metadata: %v", b)
				}
			}
		})
	}
}

func TestSSOAppLinksFiltered(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "member", "member-pass-99", false, identity.AppNoverse)
	for k, v := range map[string]string{"PUBLIC_URL": "https://vid.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com", "VERSE_APP_ORIGINS": "https://nov.example.com", "VERSE_APP_LINKS": "noverse=https://nov.example.com,viderse=https://vid.example.com"} {
		t.Setenv(k, v)
	}
	cfg, err := webguard.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	p.g.SetProxyConfig(cfg)
	ns, err := p.fx.Accounts.Login("member", "member-pass-99", "device", "")
	if err != nil {
		t.Fatal(err)
	}
	w := p.do(http.MethodGet, "/api/me", ns.Cookie, "")
	b := decode(t, w)
	apps, ok := b["apps"].([]any)
	if w.Code != 403 || !ok || len(apps) != 1 {
		t.Fatalf("denied apps not grant filtered: %d %v", w.Code, b)
	}
	app := apps[0].(map[string]any)
	if app["id"] != "noverse" || app["name"] != "Noverse" || app["origin"] != "https://nov.example.com" {
		t.Fatalf("app link=%v", app)
	}
	// A successful /me uses the same filtered projection, not all configured apps.
	nov := webapp.New(p.fx.Admin, webapp.Options{App: identity.AppNoverse, AppName: "Noverse"})
	nov.SetProxyConfig(cfg)
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.AddCookie(&http.Cookie{Name: "verse_session", Value: ns.Cookie})
	w = httptest.NewRecorder()
	nov.ServeHTTP(w, r)
	b = decode(t, w)
	apps, ok = b["apps"].([]any)
	if w.Code != http.StatusOK || !ok || len(apps) != 1 {
		t.Fatalf("successful me not grant filtered: %d %v", w.Code, b)
	}
}
