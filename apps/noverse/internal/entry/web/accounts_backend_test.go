package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

// pgAccountsGateway migrates an isolated Postgres schema and returns a Gateway
// whose accounts facade is backed by the shared identity repository. The account routes
// exercise the Repository facade end-to-end over HTTP.
func pgAccountsGateway(t *testing.T) *Gateway {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(mkdirAll(t, root, "proj"), "project.json"), []byte(`{"name":"Proj"}`), 0o644))
	mkdirAll(t, root, "proj", "kich_ban")

	accounts := identity.NewFromRepository(identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: true, ValidProjectID: library.ValidProjectID}), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass"},
		{Username: "writer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectOwner}, Password: "writer-pass"},
		{Username: "member", Role: identity.RoleMember, Password: "member-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGateway(root, accounts, version.Info{})
	t.Cleanup(g.Close)
	return g
}

// TestAccountsBackendLoginMeMemberLogout chạy vòng đời account cơ bản trên
// Postgres backend qua HTTP thật: login → /api/me → owner đổi vai member →
// logout, rồi xác nhận cookie cũ bị thu hồi.
func TestAccountsBackendLoginMeMemberLogout(t *testing.T) {
	g := pgAccountsGateway(t)

	// Login qua HTTP trên PG backend.
	login := authDo(t, g, http.MethodPost, "/api/login", "", `{"username":"writer","password":"writer-pass"}`)
	if login.Code != 200 {
		t.Fatalf("login = %d: %s", login.Code, login.Body)
	}
	cookie := authSession(t, login)

	// GET /api/me phải đọc user từ PG và trả grants.
	me := authReq(g, "GET", "/api/me", "", cookie)
	if me.Code != 200 {
		t.Fatalf("me = %d: %s", me.Code, me.Body)
	}
	var meBody struct {
		User struct {
			Username string                          `json:"username"`
			Projects map[string]identity.ProjectRole `json:"projects"`
		} `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatal(err)
	}
	if meBody.User.Username != "writer" || meBody.User.Projects["proj"] != identity.ProjectOwner {
		t.Fatalf("me = %+v", meBody.User)
	}

	// Owner đổi vai member (outsider → collab) qua members API.
	rec := authReq(g, "POST", "/api/projects/proj/members", `{"username":"member","role":"collab"}`, cookie)
	if rec.Code != 201 {
		t.Fatalf("invite member = %d: %s", rec.Code, rec.Body)
	}
	rec = authReq(g, "PATCH", "/api/projects/proj/members/member", `{"role":"view"}`, cookie)
	if rec.Code != 200 {
		t.Fatalf("patch member = %d: %s", rec.Code, rec.Body)
	}
	var member identity.ProjectMember
	if err := json.Unmarshal(rec.Body.Bytes(), &member); err != nil {
		t.Fatal(err)
	}
	if member.Role != identity.ProjectView {
		t.Fatalf("member role = %q", member.Role)
	}

	// Logout → cookie cũ bị thu hồi, /api/me trả 401.
	if rec := authReq(g, "POST", "/api/logout", "", cookie); rec.Code != 200 && rec.Code != 204 {
		t.Fatalf("logout = %d: %s", rec.Code, rec.Body)
	}
	if rec := authReq(g, "GET", "/api/me", "", cookie); rec.Code != 401 {
		t.Fatalf("me after logout = %d, want 401", rec.Code)
	}
}
