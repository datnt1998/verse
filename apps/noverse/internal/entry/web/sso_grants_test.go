package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/datnt1998/verse/platform/webguard"
)

// ssoFixture is a Postgres-backed gateway plus the handles a shared-store
// test needs: the accounts facade (operator calls), the pool (DB-down and
// DB-unchanged assertions) and ready-made sessions for a granted member.
type ssoFixture struct {
	g        *Gateway
	accounts *identity.Store
	pool     *postgres.Pool
}

// newSSOFixture migrates an isolated Postgres schema and returns a shared-
// store gateway. "member" carries the default Noverse grant the backend adds
// for accounts created without explicit Apps; "member" has no project rows.
func newSSOFixture(t *testing.T) *ssoFixture {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	mkdirAll(t, root, "proj", "kich_ban")
	if err := os.WriteFile(filepath.Join(root, "proj", "project.json"), []byte(`{"name":"Proj"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	accounts := identity.NewFromRepository(identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: true, ValidProjectID: library.ValidProjectID}), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass"},
		{Username: "member", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectOwner}, Password: "member-pass"},
		// A Viderse-only account is exactly the "valid session, no Noverse
		// grant" actor the gate exists for.
		{Username: "sibling", Role: identity.RoleMember, Apps: []identity.App{identity.AppViderse}, Password: "sibling-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewPostgresGateway(root, accounts, version.Info{})
	t.Cleanup(g.Close)
	return &ssoFixture{g: g, accounts: accounts, pool: pool}
}

func (f *ssoFixture) login(t *testing.T, username, password string) string {
	t.Helper()
	return authLogin(t, f.g, username, password)
}

// loginCookie authenticates and returns the session cookie however the
// request ended: a valid credential without a Noverse grant still gets a
// verse_session cookie inside the 403 body, per the shared-login contract.
func (f *ssoFixture) loginCookie(t *testing.T, username, password string) string {
	t.Helper()
	w := authDo(t, f.g, http.MethodPost, "/api/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password))
	if w.Code != 200 && w.Code != http.StatusForbidden {
		t.Fatalf("login %s: %d %s", username, w.Code, w.Body.String())
	}
	return authSession(t, w)
}

// TestNoverseGrantGateMatrix runs the handler contract end to end on the
// shared store: grant present/absent, the auth-neutral exceptions that must
// stay open, and every business route stopping before any handler.
func TestNoverseGrantGateMatrix(t *testing.T) {
	f := newSSOFixture(t)
	admin := f.login(t, "admin", "admin-pass")
	member := f.login(t, "member", "member-pass")
	sibling := f.loginCookie(t, "sibling", "sibling-pass")
	if w := authDo(t, f.g, http.MethodGet, "/api/me", member, ""); w.Code != 200 {
		t.Fatalf("granted member me = %d: %s", w.Code, w.Body)
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/users", admin, ""); w.Code != 200 {
		t.Fatalf("admin users = %d: %s", w.Code, w.Body)
	}

	// No-grant: /api/me is 403 with code/app/user — and user.grants is still
	// in the body so the UI can show the denied account.
	w := authDo(t, f.g, http.MethodGet, "/api/me", sibling, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-grant me = %d, want 403: %s", w.Code, w.Body)
	}
	var denied struct {
		Code string `json:"code"`
		App  string `json:"app"`
		User struct {
			Username string                 `json:"username"`
			Grants   map[string]interface{} `json:"grants"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatalf("denied body: %v", err)
	}
	if denied.Code != "app_access_denied" || denied.App != "noverse" || denied.User.Username != "sibling" {
		t.Fatalf("denied body = %+v", denied)
	}
	if denied.User.Grants == nil || denied.User.Grants["viderse"] == nil {
		t.Fatalf("denied user must carry its grants map: %+v", denied.User.Grants)
	}

	// Every business route — including writes, uploads, SSE and admin
	// surfaces — answers the same 403 and never reaches a handler.
	for _, route := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/projects", ""},
		{http.MethodPost, "/api/projects/proj/members", `{"username":"member","role":"collab"}`},
		{http.MethodGet, "/api/users", ""},
		{http.MethodPut, "/api/users/sibling/grants/noverse", `{"role":"member"}`},
		{http.MethodGet, "/api/events", ""},
		{http.MethodGet, "/api/config", ""},
		{http.MethodPost, "/api/me/onboarded", `{}`},
		{http.MethodPost, "/api/me/pairings", `{}`},
		{http.MethodGet, "/api/admin/audit", ""},
		{http.MethodPost, "/api/notify/push", `{}`},
		{http.MethodGet, "/api/me/unknown", ""},
	} {
		w := authDo(t, f.g, route.method, route.path, sibling, route.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s (no grant) = %d, want 403", route.method, route.path, w.Code)
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != "app_access_denied" {
			t.Fatalf("%s %s body = %s, want app_access_denied", route.method, route.path, w.Body)
		}
	}

	// The denied write must not have mutated anything: the project still has
	// exactly one member row.
	var memberCount int
	if err := f.pool.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM project_members`).Scan(&memberCount); err != nil {
		t.Fatalf("count members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("denied request wrote: project_members has %d rows, want 1", memberCount)
	}

	// Auth-neutral exceptions still work without a grant — own scope only.
	if w := authDo(t, f.g, http.MethodGet, "/api/me/sessions", sibling, ""); w.Code != 200 {
		t.Fatalf("no-grant sessions list = %d", w.Code)
	}
	// …but never another user's session. The sibling cannot revoke the
	// granted member's session even by guessing the id.
	memberSessions, err := f.accounts.Sessions("member", "")
	if err != nil || len(memberSessions) == 0 {
		t.Fatalf("member sessions: %v", err)
	}
	w = authDo(t, f.g, http.MethodPost, "/api/sessions/"+memberSessions[0].ID+"/revoke", sibling, `{}`)
	if w.Code == http.StatusOK {
		t.Fatal("no-grant session revoked another user's session")
	}
	if _, _, err := f.accounts.Authenticate(member); err != nil {
		t.Fatalf("member session was revoked by the no-grant branch: %v", err)
	}

	// Logout without a grant still revokes the session row — the shared
	// store contract — then clears the cookie.
	w = authDo(t, f.g, http.MethodPost, "/api/logout", sibling, "")
	if w.Code != 200 {
		t.Fatalf("no-grant logout = %d: %s", w.Code, w.Body)
	}
	if _, _, err := f.accounts.Authenticate(sibling); err == nil {
		t.Fatal("no-grant logout did not revoke the session")
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/me", sibling, ""); w.Code != 401 {
		t.Fatalf("me after logout = %d, want 401", w.Code)
	}
}

// TestAppGrantHTTPRequiresGlobalAdmin covers the grant endpoints' trust
// boundary: a member cannot grant (403 before the endpoint), forged bodies
// cannot widen the mutation (member role only, no global_role, no
// arbitrary app), a demoted admin's stale session cannot grant, and a real
// grant/revoke is audited and revokes the target's live sessions — never
// touching project memberships.
func TestAppGrantHTTPRequiresGlobalAdmin(t *testing.T) {
	f := newSSOFixture(t)
	admin := f.login(t, "admin", "admin-pass")
	member := f.login(t, "member", "member-pass")
	sibling := f.loginCookie(t, "sibling", "sibling-pass")

	// A member is refused by routePermission before the handler runs.
	for _, route := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/users/sibling/grants/noverse", `{"role":"member"}`},
		{http.MethodDelete, "/api/users/sibling/grants/noverse", ""},
		{http.MethodPut, "/api/users/sibling/grants/noverse", `{"role":"admin"}`},
		{http.MethodPut, "/api/users/sibling/grants/root", `{"role":"member"}`},
	} {
		w := authDo(t, f.g, route.method, route.path, member, route.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("member %s %s = %d, want 403", route.method, route.path, w.Code)
		}
	}

	// Forged bodies from an admin still cannot widen the mutation:
	// role must be exactly "member"; an unknown app is refused; unknown
	// fields (global_role, projects, actor) are rejected outright and grant
	// no row.
	w := authDo(t, f.g, http.MethodPut, "/api/users/sibling/grants/noverse", admin, `{"role":"admin"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("grant role=admin = %d, want 400", w.Code)
	}
	w = authDo(t, f.g, http.MethodPut, "/api/users/sibling/grants/chatverse", admin, `{"role":"member"}`)
	if w.Code == http.StatusOK {
		t.Fatal("granted an unknown app")
	}
	w = authDo(t, f.g, http.MethodPut, "/api/users/sibling/grants/noverse", admin, `{"role":"member","global_role":"admin","projects":{"proj":"owner"},"actor":{"username":"root"}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("forged grant body = %d, want 400: %s", w.Code, w.Body)
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/me", sibling, ""); w.Code != http.StatusForbidden {
		t.Fatalf("forged body changed sibling access: me = %d", w.Code)
	}
	w = authDo(t, f.g, http.MethodPut, "/api/users/sibling/grants/noverse", admin, `{"role":"member","reason":"cấp quyền"}`)
	if w.Code != 200 {
		t.Fatalf("admin grant = %d: %s", w.Code, w.Body)
	}
	var granted struct {
		Role     string                 `json:"role"`
		Grants   map[string]interface{} `json:"grants"`
		Projects map[string]interface{} `json:"projects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &granted); err != nil {
		t.Fatalf("grant body: %v", err)
	}
	if granted.Role != string(identity.RoleMember) {
		t.Fatalf("forged global_role promoted the user: %+v", granted)
	}
	if granted.Grants["noverse"] == nil || granted.Grants["viderse"] == nil {
		t.Fatalf("grant response missing grants map: %+v", granted.Grants)
	}
	if len(granted.Projects) != 0 {
		t.Fatalf("grant mutation touched Projects: %+v", granted.Projects)
	}

	// The grant revoked the target's live sessions: the old sibling cookie
	// is dead, and re-login sees the new grant.
	if w := authDo(t, f.g, http.MethodGet, "/api/me", sibling, ""); w.Code != 401 {
		t.Fatalf("stale sibling cookie after grant = %d, want 401", w.Code)
	}
	sibling = f.login(t, "sibling", "sibling-pass")
	if w := authDo(t, f.g, http.MethodGet, "/api/me", sibling, ""); w.Code != 200 {
		t.Fatalf("sibling after grant = %d: %s", w.Code, w.Body)
	}

	// Revoking removes access again, revokes sessions, and leaves project
	// membership untouched (member keeps its proj owner row throughout).
	w = authDo(t, f.g, http.MethodDelete, "/api/users/sibling/grants/noverse", admin, `{"reason":"dọn quyền"}`)
	if w.Code != 200 {
		t.Fatalf("revoke = %d: %s", w.Code, w.Body)
	}
	w = authDo(t, f.g, http.MethodGet, "/api/me", sibling, "")
	if w.Code != 401 {
		t.Fatalf("post-revoke cookie = %d, want 401 (sessions revoked)", w.Code)
	}
	// Project membership is untouched by grant changes: the granted member's
	// session and its proj owner row both survive sibling's revoke.
	if _, _, err := f.accounts.Authenticate(member); err != nil {
		t.Fatalf("granted member session wrongly revoked: %v", err)
	}

	// Grant mutations are audited: grant + revoke entries with actor admin.
	page, err := f.accounts.Audit(identity.AuditQuery{Limit: 200})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	var sawGrant, sawRevoke bool
	for _, item := range page.Items {
		if item.Target != "sibling" {
			continue
		}
		switch item.Action {
		case "Cấp quyền ứng dụng":
			sawGrant = true
			if item.Actor == nil || item.Actor.Username != "admin" {
				t.Fatalf("grant audit actor = %+v", item.Actor)
			}
		case "Thu hồi quyền ứng dụng":
			sawRevoke = true
		}
	}
	if !sawGrant || !sawRevoke {
		t.Fatal("grant/revoke mutations are not audited")
	}

	// A demoted admin cannot grant: demotion revokes its sessions, so the
	// cookie is dead (401) and even a direct facade call with the stale
	// actor fails the in-transaction admin check. A second admin performs
	// the demotion (the store requires one active admin).
	if _, err := f.accounts.CreateUser(identity.NewUser{
		Username: "admin2", Role: identity.RoleAdmin, Password: "admin2-pass",
	}); err != nil {
		t.Fatalf("create admin2: %v", err)
	}
	role := identity.RoleMember
	if _, err := f.accounts.UpdateUser("admin", identity.UserPatch{
		Role:  &role,
		Actor: &identity.AuditActor{Username: "admin2"},
	}); err != nil {
	}
	if w := authDo(t, f.g, http.MethodPut, "/api/users/sibling/grants/noverse", admin, `{"role":"member"}`); w.Code != 401 {
		t.Fatalf("demoted admin cookie = %d, want 401", w.Code)
	}
	if err := f.accounts.SetAppGrant("sibling", identity.AppNoverse, identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, ""); err == nil {
		t.Fatal("demoted admin granted through the facade")
	}
}

// TestSharedLogoutRevokesBeforeClear pins the order: logout revokes the
// session row first, then expires the cookie; a storage failure reports the
// error and must not tell the client the logout succeeded.
func TestSharedLogoutRevokesBeforeClear(t *testing.T) {
	f := newSSOFixture(t)
	member := f.login(t, "member", "member-pass")

	w := authDo(t, f.g, http.MethodPost, "/api/logout", member, "")
	if w.Code != 200 {
		t.Fatalf("logout = %d: %s", w.Code, w.Body)
	}
	var expired bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sharedSessionCookie && c.MaxAge < 0 {
			expired = true
		}
	}
	if !expired {
		t.Fatal("logout did not expire the shared cookie")
	}
	if _, _, err := f.accounts.Authenticate(member); err == nil {
		t.Fatal("logout left the session row valid")
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/me", member, ""); w.Code != 401 {
		t.Fatalf("me after logout = %d, want 401", w.Code)
	}

	// Storage failure: the session stays valid and the answer is not a
	// fake success — the browser keeps a cookie that still authenticates.
	member = f.login(t, "member", "member-pass")
	f.pool.Close()
	w = authDo(t, f.g, http.MethodPost, "/api/logout", member, "")
	if w.Code == 200 {
		t.Fatal("logout reported success while the session row could not be revoked")
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("logout on dead DB = %d, want 503", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sharedSessionCookie && c.MaxAge < 0 {
			t.Fatal("failed logout cleared the cookie")
		}
	}
}

// TestSSOLoginClearsLegacyCookies proves a shared login retires the
// per-app cookie names at every scope it could have been set on, issues a
// fresh verse_session, and never echoes a caller-supplied session id or
// secret back into the new cookie.
func TestSSOLoginClearsLegacyCookies(t *testing.T) {
	f := newSSOFixture(t)

	// A browser holding legacy cookies sends them along; login must expire
	// both names and set exactly one new shared cookie.
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"member","password":"member-pass"}`))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: "old-noverse-session"})
	r.AddCookie(&http.Cookie{Name: "viderse_session", Value: "old-viderse-session"})
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("login = %d: %s", w.Code, w.Body)
	}
	var newCookie, expiredAinovel, expiredViderse bool
	for _, c := range w.Result().Cookies() {
		switch {
		case c.Name == sharedSessionCookie && c.Value != "":
			newCookie = true
			if c.Value == "old-noverse-session" || c.Value == "old-viderse-session" {
				t.Fatal("login reused a caller-supplied session value")
			}
		case c.Name == fileSessionCookie && c.MaxAge < 0:
			expiredAinovel = true
		case c.Name == "viderse_session" && c.MaxAge < 0:
			expiredViderse = true
		}
	}
	if !newCookie || !expiredAinovel || !expiredViderse {
		t.Fatalf("login cookies = %+v (want new verse_session + both legacy expired)", w.Result().Cookies())
	}

	// The Postgres backend never reads the legacy names: a request carrying
	// only ainovel_session is unauthenticated, not dual-read.
	r = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: "old-noverse-session"})
	w = httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("legacy cookie read = %d, want 401 (no fallback)", w.Code)
	}
}

// TestForcedPasswordGateEveryBusinessRoute enumerates the routes a
// must-change-password session may and may not touch: the exact neutral set
// passes the gate (whether or not the handler then accepts the input),
// every other method+path — writes, SSE, unknown paths — is refused before
// any handler runs.
func TestForcedPasswordGateEveryBusinessRoute(t *testing.T) {
	f := newSSOFixture(t)
	if _, err := f.accounts.CreateUser(identity.NewUser{
		Username: "temp", Role: identity.RoleMember,
		Apps:     []identity.App{identity.AppNoverse},
		Password: "temp-pass", MustChangePassword: true,
	}); err != nil {
		t.Fatalf("create mustchange user: %v", err)
	}
	temp := f.login(t, "temp", "temp-pass")

	mustDeny := func(method, path, body string) {
		t.Helper()
		w := authDo(t, f.g, method, path, temp, body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s (must-change) = %d, want 403: %s", method, path, w.Code, w.Body)
		}
		var b struct {
			Code string `json:"code"`
			Flag bool   `json:"mustChangePassword"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || !b.Flag || b.Code != "password_change_required" {
			t.Fatalf("%s %s body is not the forced-password contract: %s", method, path, w.Body)
		}
	}
	mustReach := func(method, path, body string) {
		t.Helper()
		w := authDo(t, f.g, method, path, temp, body)
		var b struct {
			Flag bool `json:"mustChangePassword"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &b); err == nil && b.Flag {
			t.Fatalf("%s %s hit the forced-password gate: %s", method, path, w.Body)
		}
		if w.Code == http.StatusUnauthorized {
			t.Fatalf("%s %s = 401 (session is valid)", method, path)
		}
	}

	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/projects", ""},
		{http.MethodPost, "/api/projects/proj/members", `{"username":"member","role":"collab"}`},
		{http.MethodGet, "/api/users", ""},
		{http.MethodGet, "/api/events", ""},
		{http.MethodGet, "/api/config", ""},
		{http.MethodPost, "/api/me/pairings", `{}`},
		{http.MethodDelete, "/api/me/sessions", ""},
		{http.MethodDelete, "/api/me", ""},
		{http.MethodGet, "/api/me/unknown", ""},
		{http.MethodPost, "/api/upload-anything", `{}`},
		{http.MethodGet, "/api/admin/audit", ""},
	} {
		mustDeny(route.method, route.path, route.body)
	}
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/me", ""},
		{http.MethodPost, "/api/me/onboarded", `{}`},
		{http.MethodGet, "/api/me/sessions", ""},
		// Session-killing neutrals run last: reaching the handler revokes the
		// cookie they were called with, by design.
		{http.MethodPost, "/api/me/password", `{"current":"temp-pass","new":"final-pass-9"}`},
	} {
		mustReach(route.method, route.path, route.body)
	}
	// After the change the flag is cleared and the next login carries no gate.
	temp = f.login(t, "temp", "final-pass-9")
	w := authDo(t, f.g, http.MethodGet, "/api/me", temp, "")
	if w.Code != 200 {
		t.Fatalf("post-change me = %d: %s", w.Code, w.Body)
	}
	var meBody struct {
		User struct {
			Flag bool `json:"mustChangePassword"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meBody); err != nil || meBody.User.Flag {
		t.Fatalf("must-change flag not cleared: %s", w.Body)
	}
	// revoke-all and logout are also neutral: reachable on a must-change
	// session. Each kills the session it used and the temp password is
	// single-use, so every leg gets a fresh must-change account.
	for i, route := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/me/sessions/revoke-all", `{}`},
		{http.MethodPost, "/api/logout", `{}`},
	} {
		username := fmt.Sprintf("temp%d", i+2)
		if _, err := f.accounts.CreateUser(identity.NewUser{
			Username: username, Role: identity.RoleMember,
			Apps:     []identity.App{identity.AppNoverse},
			Password: "temp-pass", MustChangePassword: true,
		}); err != nil {
			t.Fatalf("create %s: %v", username, err)
		}
		cookie := f.login(t, username, "temp-pass")
		if w := authDo(t, f.g, route.method, route.path, cookie, route.body); w.Code != 200 {
			t.Fatalf("%s %s (must-change neutral) = %d: %s", route.method, route.path, w.Code, w.Body)
		}
	}
}

// TestForcedPasswordAdminKeepsOnlyOwnPowers: an admin still on a temporary
// password must not read data through a stale-version conflict (409 carries
// the current resource) nor revoke another account's session through the
// neutral session route.
func TestForcedPasswordAdminKeepsOnlyOwnPowers(t *testing.T) {
	f := newSSOFixture(t)
	if _, err := f.accounts.CreateUser(identity.NewUser{
		Username: "tempadmin", Role: identity.RoleAdmin,
		Password: "temp-pass", MustChangePassword: true,
	}); err != nil {
		t.Fatalf("create must-change admin: %v", err)
	}
	temp := f.login(t, "tempadmin", "temp-pass")
	member := f.login(t, "member", "member-pass")

	w := authDo(t, f.g, http.MethodPost, "/api/users/member", temp, `{"version":"stale","name":"Đổi tên"}`)
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), `"current"`) ||
		!strings.Contains(w.Body.String(), `"code":"password_change_required"`) {
		t.Fatalf("stale-version write by a must-change admin = %d %s", w.Code, w.Body)
	}

	sessions, err := f.accounts.Sessions("member", "")
	if err != nil || len(sessions) == 0 {
		t.Fatalf("member sessions: %v", err)
	}
	if w := authDo(t, f.g, http.MethodPost, "/api/sessions/"+sessions[0].ID+"/revoke", temp, `{}`); w.Code == http.StatusOK {
		t.Fatalf("must-change admin revoked another account's session: %s", w.Body)
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/me", member, ""); w.Code != http.StatusOK {
		t.Fatalf("member session after the refused revoke = %d", w.Code)
	}
	if w := authDo(t, f.g, http.MethodGet, "/api/me", temp, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "pendingLessons") {
		t.Fatalf("must-change admin /api/me = %d %s; want profile without business counts", w.Code, w.Body)
	}
}

// TestHostedWorkerEnvExcludesIdentitySecrets captures the environment a
// worker child would receive with a hostile parent environment: every DSN,
// migration credential and unknown secret must be filtered out, and the
// child's DATABASE_URL is the scoped worker DSN — never the app's.
func TestHostedWorkerEnvExcludesIdentitySecrets(t *testing.T) {
	const (
		appDSN      = "postgres://noverse_app:appsecret@db/noverse"
		migrateDSN  = "postgres://noverse_migrate:migsecret@db/noverse"
		workerDSN   = "postgres://noverse_engine_worker:worksecret@db/noverse"
		identityDSN = "postgres://viderse_identity:idsecret@db/noverse"
	)
	// A hostile parent environment: credentials that must never reach a child.
	t.Setenv("DATABASE_URL", appDSN)
	t.Setenv("MIGRATION_DATABASE_URL", migrateDSN)
	t.Setenv("NOVERSE_WORKER_DATABASE_URL", workerDSN)
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", identityDSN)
	t.Setenv("VIDERSE_DATABASE_URL", "postgres://viderse:vsec@db/viderse")
	t.Setenv("VIDERSE_MIGRATION_DATABASE_URL", "postgres://vm:vms@db/viderse")
	t.Setenv("DATABASE_URL_DUPLICATE", appDSN)
	t.Setenv("POSTGRES_APP_PASSWORD", "appsecret")
	t.Setenv("S3_SECRET_ACCESS_KEY", "s3secret")
	t.Setenv("TTS_WORKER_TOKEN", "ttstoken")
	t.Setenv("SECRET_API_KEY", "apikey")
	t.Setenv("HOME", "/home/noverse")
	t.Setenv("PATH", "/usr/bin")

	env := workerEnv("AINOVEL_WORKER_TOKEN=tok", "DATABASE_URL="+workerDSN)
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, leaked := range []string{appDSN, migrateDSN, identityDSN, "appsecret", "migsecret", "idsecret", "s3secret", "ttstoken", "apikey"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("worker env leaked %q: %v", leaked, env)
		}
	}
	var homeSet, pathSet bool
	dsnCount := 0
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "DATABASE_URL":
			dsnCount++
			if v != workerDSN {
				t.Fatalf("child DATABASE_URL = %q, want worker DSN", v)
			}
		case strings.Contains(k, "DATABASE_URL") || strings.Contains(k, "PASSWORD") || strings.Contains(k, "SECRET") || strings.Contains(k, "TOKEN"):
			if k != "AINOVEL_WORKER_TOKEN" {
				t.Fatalf("worker env kept secret key %s", k)
			}
		case k == "HOME":
			homeSet = v == "/home/noverse"
		case k == "PATH":
			pathSet = v == "/usr/bin"
		}
	}
	if dsnCount != 1 {
		t.Fatalf("DATABASE_URL appears %d times in child env", dsnCount)
	}
	if !homeSet || !pathSet {
		t.Fatalf("child env dropped required passthrough: %v", env)
	}

	// Hosted path: the storage contract yields the same filtered env and
	// fails closed without WorkerURL — never a fallback to the app DSN.
	cfg := &bootstrap.StorageConfig{
		Driver:      bootstrap.StoreDriverPostgres,
		DatabaseURL: appDSN,
		WorkerURL:   workerDSN,
	}
	hosted, err := hostedWorkerEnv(cfg, "tok", "proj", "story")
	if err != nil {
		t.Fatalf("hostedWorkerEnv: %v", err)
	}
	var hostedDSN string
	for _, kv := range hosted {
		if strings.HasPrefix(kv, "DATABASE_URL=") {
			hostedDSN = strings.TrimPrefix(kv, "DATABASE_URL=")
		}
	}
	if hostedDSN != workerDSN {
		t.Fatalf("hosted child DATABASE_URL = %q, want %q", hostedDSN, workerDSN)
	}
	if _, err := hostedWorkerEnv(&bootstrap.StorageConfig{
		Driver:      bootstrap.StoreDriverPostgres,
		DatabaseURL: appDSN,
	}, "tok", "proj", "story"); err == nil {
		t.Fatal("hosted worker started without NOVERSE_WORKER_DATABASE_URL")
	}
}

// TestSSOOriginAllowlistMatrix runs the browser-boundary matrix over real
// HTTP requests: every mutating method, every origin shape and every
// Sec-Fetch-Site metadata state, on both the local-SSO and hosted configs.
// A rejected request is answered before Authenticate and must leave the
// database untouched; an allowed one reaches the handler (or 401).
func TestSSOOriginAllowlistMatrix(t *testing.T) {
	f := newSSOFixture(t)
	member := f.login(t, "member", "member-pass")

	origins := []struct {
		name, origin string
		allowed      bool
	}{
		{"own", "http://127.0.0.1:8686", true},
		{"allowlisted sibling", "http://127.0.0.1:18789", true},
		{"non-allowlisted sibling", "http://127.0.0.1:18791", false},
		{"foreign", "https://evil.example", false},
		{"null", "null", false},
		{"missing", "", true},
	}
	fetches := []struct {
		name, fetch string
		allowed     bool
	}{
		{"same-origin", "same-origin", true},
		{"same-site", "same-site", true},
		{"cross-site", "cross-site", false},
		{"missing", "", true},
	}
	methods := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/me", `{"name":"CSRFProbe"}`},
		{http.MethodPut, "/api/me", `{"name":"CSRFProbe"}`},
		{http.MethodPatch, "/api/me", `{"name":"CSRFProbe"}`},
		{http.MethodDelete, "/api/me", ""},
	}

	for _, mode := range []string{"local", "hosted"} {
		switch mode {
		case "local":
			f.g.SetProxyConfig(&webguard.Config{
				AppOrigins: []string{"http://127.0.0.1:18789", "http://127.0.0.1:18790"},
			})
		case "hosted":
			u, _ := url.Parse("https://noverse.example.com")
			f.g.SetProxyConfig(&webguard.Config{
				PublicURL:     u,
				SecureCookies: true,
				CookieDomain:  "example.com",
				AppOrigins:    []string{"https://viderse.example.com"},
			})
		}
		hostedOrigins := []struct {
			name, origin string
			allowed      bool
		}{
			{"own", "https://noverse.example.com", true},
			{"allowlisted sibling", "https://viderse.example.com", true},
			{"non-allowlisted sibling", "https://other.example.com", false},
			{"foreign", "https://evil.example", false},
			{"null", "null", false},
			{"missing", "", true},
		}
		originSet := origins
		if mode == "hosted" {
			originSet = hostedOrigins
		}
		for _, m := range methods {
			for _, o := range originSet {
				for _, fe := range fetches {
					allowed := o.allowed && fe.allowed
					r := httptest.NewRequest(m.method, m.path, strings.NewReader(m.body))
					r.RemoteAddr = "192.0.2.1:1234"
					r.Host = "127.0.0.1:8686"
					r.Header.Set("Content-Type", "application/json")
					if o.origin != "" {
						r.Header.Set("Origin", o.origin)
					}
					if fe.fetch != "" {
						r.Header.Set("Sec-Fetch-Site", fe.fetch)
					}
					r.AddCookie(&http.Cookie{Name: f.g.cookiePolicy().Name, Value: member})
					w := httptest.NewRecorder()
					f.g.ServeHTTP(w, r)
					if allowed {
						// Reached the handler boundary: never a CSRF 403.
						// Any outcome below — 200, 404, even 405 — proves the
						// request passed the Origin check.
						if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "origin") {
							t.Fatalf("%s %s origin=%s fetch=%s: allowed request CSRF-rejected: %s",
								mode, m.method, o.name, fe.name, w.Body)
						}
					} else {
						if w.Code != http.StatusForbidden {
							t.Fatalf("%s %s origin=%s fetch=%s = %d, want 403",
								mode, m.method, o.name, fe.name, w.Code)
						}
					}
				}
			}
		}
	}
	f.g.SetProxyConfig(&webguard.Config{})

	// DB unchanged: every denied write must have stopped before the
	// handler. member's name was only ever written by allowed cells.
	users, err := f.accounts.Users()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username == "member" {
			if u.Name != "CSRFProbe" && u.Name != "" {
				t.Fatalf("a rejected write changed member.name to %q", u.Name)
			}
		}
	}
}

// TestIdentityErrorsAndLogsRedactSecrets (HTTP layer) proves request handling
// never surfaces credentials: neither the 403/503 bodies nor the server log
// may contain a session token, a password or a DSN credential, and a dead
// database is reported as temporarily unavailable without clearing the
// shared cookie.
func TestIdentityErrorsAndLogsRedactSecrets(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)

	f := newSSOFixture(t)
	const password = "member-pass"
	member := f.login(t, "member", password)

	// Login with a wrong password produces a generic error, never an echo.
	w := authDo(t, f.g, http.MethodPost, "/api/login", "", `{"username":"member","password":"`+password+`-wrong"}`)
	if w.Code != 401 {
		t.Fatalf("bad login = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), password) {
		t.Fatal("error body echoed the password")
	}

	// Session token must never appear in a response body.
	w = authDo(t, f.g, http.MethodGet, "/api/me/sessions", member, "")
	if w.Code != 200 {
		t.Fatalf("sessions = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), member) {
		t.Fatal("response body contained the raw session token")
	}

	// DB down: /api/me is 503, the shared cookie is NOT cleared, and the
	// failure is logged without the DSN.
	f.pool.Close()
	w = authDo(t, f.g, http.MethodGet, "/api/me", member, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("me on dead DB = %d, want 503: %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sharedSessionCookie && c.MaxAge < 0 {
			t.Fatal("DB-down request cleared the shared cookie")
		}
	}
	w = authDo(t, f.g, http.MethodGet, "/api/projects", member, "")
	if w.Code == http.StatusUnauthorized {
		t.Fatal("DB-down disguised as unauthenticated")
	}

	text := logs.String()
	for _, secret := range []string{password, member, "appsecret", "postgres://"} {
		if strings.Contains(text, secret) {
			t.Fatalf("server log leaked %q: %s", secret, text)
		}
	}
}
