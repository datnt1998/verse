package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAccountContractFile pins the web-accounts file contract: envelope
// (version 3, users/sessions/pairings/audit), role names, permission grants,
// session lifecycle, migration of legacy books[] grants, and the
// absent-vs-corrupt distinction. It is a baseline of current file behavior —
// the Postgres backend must produce the same observable results.

func contractStorePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "web-accounts.json")
}

func TestAccountContractFile(t *testing.T) {
	t.Run("envelope_and_roles", func(t *testing.T) {
		path := contractStorePath(t)
		s := NewStore(path, testOptions())
		v, err := s.CreateUser(NewUser{
			Username: "tac-gia",
			Name:     "Tác Giả Mẫu",
			Role:     RoleMember,
			Projects: map[string]ProjectRole{"du-an": ProjectOwner, "du-an-khac": ProjectView},
			Password: "mat-khau-mau-1",
		})
		if err != nil {
			t.Fatal(err)
		}
		if v.Role != RoleMember || v.Projects["du-an"] != ProjectOwner || v.Projects["du-an-khac"] != ProjectView {
			t.Fatalf("grants must round-trip: %+v", v)
		}
		// role names are the exact persisted tokens
		bad := Role("collaborator")
		if _, err := s.CreateUser(NewUser{Username: "badrole", Role: bad, Password: "mat-khau-mau-1"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("global role 'collaborator' must be rejected: %v", err)
		}
		if _, err := s.CreateUser(NewUser{Username: "badproj", Role: RoleMember, Projects: map[string]ProjectRole{"du-an": "editor"}, Password: "mat-khau-mau-1"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("project role 'editor' must be rejected: %v", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			Version int `json:"version"`
			Users   []struct {
				Username string            `json:"username"`
				Role     string            `json:"role"`
				Projects map[string]string `json:"projects"`
				Hash     string            `json:"passwordHash"`
			} `json:"users"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatal(err)
		}
		if head.Version != fileVersion {
			t.Fatalf("file envelope must stay version %d, got %d", fileVersion, head.Version)
		}
		u := head.Users[0]
		if u.Role != "member" || u.Projects["du-an"] != "owner" || !strings.HasPrefix(u.Hash, "$argon2id$") {
			t.Fatalf("persisted user shape drifted: %+v", u)
		}
		if strings.Contains(string(raw), "mat-khau-mau-1") {
			t.Fatal("password must never persist in clear text")
		}
	})

	t.Run("login_session_revoke_change_password", func(t *testing.T) {
		path := contractStorePath(t)
		s := NewStore(path, testOptions())
		if _, err := s.CreateUser(NewUser{Username: "quan-tri", Role: RoleAdmin, Password: "mat-khau-quan-tri-1"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login("quan-tri", "sai-mat-khau", "Mac", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("wrong password must be generic credential error: %v", err)
		}
		ns, err := s.Login("quan-tri", "mat-khau-quan-tri-1", "Mac", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); err != nil {
			t.Fatalf("fresh session must authenticate: %v", err)
		}
		// self password change keeps the current session, drops others
		ns2, err := s.Login("quan-tri", "mat-khau-quan-tri-1", "Phone", "127.0.0.2")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ChangePassword("quan-tri", "mat-khau-quan-tri-1", "mat-khau-moi-12345", sessionIDOf(t, ns.Cookie)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); err != nil {
			t.Fatalf("kept session must survive password change: %v", err)
		}
		if _, _, err := s.Authenticate(ns2.Cookie); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("other session must be revoked on password change: %v", err)
		}
		if _, err := s.Login("quan-tri", "mat-khau-quan-tri-1", "Mac", ""); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("old password must stop working: %v", err)
		}
		// admin reset sets mustChangePassword and revokes sessions
		tru := true
		if _, err := s.UpdateUser("quan-tri", UserPatch{MustChangePassword: &tru}); err != nil {
			t.Fatal(err)
		}
		if u, _, err := s.Authenticate(ns.Cookie); err != nil || !u.MustChangePassword {
			t.Fatalf("mustChangePassword flag must persist: %v %+v", err, u)
		}
	})

	t.Run("disabled_and_grants", func(t *testing.T) {
		path := contractStorePath(t)
		s := NewStore(path, testOptions())
		if _, err := s.CreateUser(NewUser{Username: "quan-tri", Role: RoleAdmin, Password: "mat-khau-quan-tri-1"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(NewUser{Username: "doc-gia", Role: RoleMember, Projects: map[string]ProjectRole{"du-an": ProjectView}, Password: "mat-khau-doc-gia-1"}); err != nil {
			t.Fatal(err)
		}
		ns, err := s.Login("doc-gia", "mat-khau-doc-gia-1", "Mac", "")
		if err != nil {
			t.Fatal(err)
		}
		off := true
		if _, err := s.UpdateUser("doc-gia", UserPatch{Disabled: &off, Reason: "kiểm thử"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("disabled user session must be revoked: %v", err)
		}
		if _, err := s.Login("doc-gia", "mat-khau-doc-gia-1", "Mac", ""); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("disabled login must be generic credential error: %v", err)
		}
	})

	t.Run("absent_vs_corrupt_file", func(t *testing.T) {
		path := contractStorePath(t)
		s := NewStore(path, testOptions())
		if has, err := s.HasUsers(); err != nil || has {
			t.Fatalf("absent file must mean no users: %v %v", has, err)
		}
		fixture, err := os.ReadFile(filepath.Join("..", "..", "apps", "noverse", "internal", "store", "testdata", "storage-contract", "accounts", "web-accounts.corrupt.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, fixture, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.HasUsers(); err == nil || os.IsNotExist(err) {
			t.Fatalf("corrupt file must error, not look absent: %v", err)
		}
	})

	t.Run("v3_fixture_login_and_grants", func(t *testing.T) {
		fixture, err := os.ReadFile(filepath.Join("..", "..", "apps", "noverse", "internal", "store", "testdata", "storage-contract", "accounts", "web-accounts.v3.json"))
		if err != nil {
			t.Fatal(err)
		}
		path := contractStorePath(t)
		if err := os.WriteFile(path, fixture, 0o600); err != nil {
			t.Fatal(err)
		}
		s := NewStore(path, testOptions())
		ns, err := s.Login("tac-gia", "fixture-mat-khau-456", "Mac", "")
		if err != nil {
			t.Fatalf("fixture hash must verify: %v", err)
		}
		if ns.User.Role != RoleMember {
			t.Fatalf("fixture role must be member: %+v", ns.User)
		}
		u, _, err := s.Authenticate(ns.Cookie)
		if err != nil || !u.CanManageProject("du-an") || !u.CanCollaborateProject("du-an-khac") {
			t.Fatalf("fixture grants must apply: %v %+v", err, u)
		}
		if u.Projects["du-an"] != ProjectOwner || u.Projects["du-an-khac"] != ProjectCollab {
			t.Fatalf("fixture project map must round-trip: %+v", u.Projects)
		}
		users, err := s.Users()
		if err != nil || len(users) != 3 {
			t.Fatalf("fixture users must load sorted: %v %d", err, len(users))
		}
		if users[0].Username != "doc-gia" || users[1].Username != "quan-tri" || users[2].Username != "tac-gia" {
			t.Fatalf("user list must be sorted by username: %v", users)
		}
		for _, u := range users {
			if u.Username == "doc-gia" && !u.MustChangePassword {
				t.Fatal("fixture mustChangePassword must round-trip")
			}
		}
	})

	t.Run("v2_legacy_migrates_grants", func(t *testing.T) {
		fixture, err := os.ReadFile(filepath.Join("..", "..", "apps", "noverse", "internal", "store", "testdata", "storage-contract", "accounts", "web-accounts.v2-legacy.json"))
		if err != nil {
			t.Fatal(err)
		}
		path := contractStorePath(t)
		if err := os.WriteFile(path, fixture, 0o600); err != nil {
			t.Fatal(err)
		}
		s := NewStore(path, testOptions())
		s.SetProjectSource(func() ([]string, error) { return []string{"du-an", "du-an-khac"}, nil })
		users, err := s.Users()
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]UserListView{}
		for _, u := range users {
			byName[u.Username] = u
		}
		if byName["quan-tri-cu"].Role != RoleAdmin {
			t.Fatalf("admin must stay admin: %+v", byName["quan-tri-cu"])
		}
		w := byName["nguoi-viet-cu"]
		if w.Role != RoleMember || w.Projects["du-an"] != ProjectCollab || w.Projects["truyen-cu"] != ProjectCollab {
			t.Fatalf("writer must become member+collab on named projects: %+v", w)
		}
		r := byName["nguoi-doc-cu"]
		if r.Role != RoleMember || r.Projects["_"] != ProjectView {
			t.Fatalf("reader of bare book name must become member+view on '_': %+v", r)
		}
		// migrated file is rewritten as v3 and the original bytes are backed up
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(raw, &head); err != nil || head.Version != fileVersion {
			t.Fatalf("migration must rewrite as v%d: %v %s", fileVersion, err, raw)
		}
		if _, err := os.Stat(path + ".v2.bak"); err != nil {
			t.Fatalf("migration must keep the original bytes: %v", err)
		}
	})
}

func sessionIDOf(t *testing.T, cookie string) string {
	t.Helper()
	id, _, ok := strings.Cut(cookie, ".")
	if !ok || id == "" {
		t.Fatalf("cookie shape must be id.secret: %q", cookie)
	}
	return id
}
