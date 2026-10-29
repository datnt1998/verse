package webauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s := NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	s.now = c.now
	if _, err := s.CreateUser(NewUser{Username: "admin", Role: RoleAdmin, Password: "admin-pass"}); err != nil {
		t.Fatal(err)
	}
	return s, c
}

func TestLoginSessionLifecycle(t *testing.T) {
	s, c := newTestStore(t)
	if _, err := s.Login("admin", "wrong-pass", "Mac", "1.2.3.4"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := s.Login("ghost", "admin-pass", "Mac", "1.2.3.4"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user must look like a wrong password: %v", err)
	}
	ns, err := s.Login("admin", "admin-pass", "Mac", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if !ns.ExpiresAt.Equal(c.t.Add(30 * 24 * time.Hour)) {
		t.Fatalf("session must expire 30 days after login, got %v", ns.ExpiresAt)
	}
	if _, _, err := s.Authenticate(ns.Cookie + "x"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("tampered secret accepted: %v", err)
	}
	u, sess, err := s.Authenticate(ns.Cookie)
	if err != nil || u.Username != "admin" || !sess.Current {
		t.Fatalf("authenticate: %v %+v %+v", err, u, sess)
	}
	// 30 天是绝对期限：持续使用也不会续期。
	c.t = c.t.Add(29 * 24 * time.Hour)
	if _, _, err := s.Authenticate(ns.Cookie); err != nil {
		t.Fatalf("day 29: %v", err)
	}
	c.t = c.t.Add(24*time.Hour + time.Second)
	if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired session accepted: %v", err)
	}
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(s.Path()); st.Mode().Perm() != 0o600 {
		t.Fatalf("accounts file mode %v", st.Mode().Perm())
	}
	if strings.Contains(string(raw), "admin-pass") || strings.Contains(string(raw), ns.Cookie) {
		t.Fatal("password or session secret stored in clear text")
	}
}

func TestPairingIsSingleUseAndExpires(t *testing.T) {
	s, c := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "reader", Role: RoleMember, Projects: map[string]ProjectRole{"_": ProjectView}, Password: "reader-pass"}); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.NewPairing("reader", "admin")
	if err != nil {
		t.Fatal(err)
	}
	ns, err := s.Pair(code, "iPhone", "10.0.0.2")
	if err != nil || ns.User.Username != "reader" {
		t.Fatalf("pair: %v %+v", err, ns.User)
	}
	if _, err := s.Pair(code, "iPhone", "10.0.0.3"); !errors.Is(err, ErrInvalidPairing) {
		t.Fatalf("code reused: %v", err)
	}
	if !ns.ExpiresAt.Equal(c.t.Add(SessionTTL)) {
		t.Fatalf("QR session expiry %v", ns.ExpiresAt)
	}
	late, _, err := s.NewPairing("reader", "admin")
	if err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(PairingTTL + time.Second)
	if _, err := s.Pair(late, "iPhone", "10.0.0.2"); !errors.Is(err, ErrInvalidPairing) {
		t.Fatalf("expired code accepted: %v", err)
	}
}

func TestDisablingOrResettingPasswordRevokesSessions(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "writer", Role: RoleMember, Password: "writer-pass"}); err != nil {
		t.Fatal(err)
	}
	ns, err := s.Login("writer", "writer-pass", "Mac", "")
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	if _, err := s.UpdateUser("writer", UserPatch{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("disabled user still authenticated: %v", err)
	}
	if _, err := s.Login("writer", "writer-pass", "Mac", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled user logged in: %v", err)
	}

	keep, _ := s.Login("admin", "admin-pass", "Mac", "")
	other, _ := s.Login("admin", "admin-pass", "iPhone", "")
	if err := s.ChangePassword("admin", "wrong", "new-admin-pass", ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong current password: %v", err)
	}
	_, current, _ := s.Authenticate(keep.Cookie)
	if err := s.ChangePassword("admin", "admin-pass", "new-admin-pass", current.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(keep.Cookie); err != nil {
		t.Fatalf("current session must survive own password change: %v", err)
	}
	if _, _, err := s.Authenticate(other.Cookie); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("other session survived password change: %v", err)
	}
}

func TestLastAdminIsProtected(t *testing.T) {
	s, _ := newTestStore(t)
	writer := RoleMember
	if _, err := s.UpdateUser("admin", UserPatch{Role: &writer}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demoted last admin: %v", err)
	}
	if err := s.DeleteUser("admin"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("deleted last admin: %v", err)
	}
	if _, err := s.CreateUser(NewUser{Username: "admin2", Role: RoleAdmin, Password: "admin2-pass"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser("admin"); err != nil {
		t.Fatalf("deleting one of two admins: %v", err)
	}
}

func TestPermissions(t *testing.T) {
	for _, tc := range []struct {
		level              ProjectRole
		read, write, owner bool
	}{
		{"", false, false, false}, {ProjectView, true, false, false}, {ProjectCollab, true, true, false}, {ProjectOwner, true, true, true},
	} {
		u := User{Role: RoleMember, Projects: map[string]ProjectRole{"p": tc.level}}
		if u.CanRead("p/s") != tc.read || u.CanWrite("p/s") != tc.write || u.CanManageProject("p") != tc.owner || u.CanCreateStories() != tc.write {
			t.Fatalf("permission %q: %+v", tc.level, u)
		}
		if u.CanRead("new/s") {
			t.Fatal("new projects must not inherit access")
		}
	}
	admin := User{Role: RoleAdmin}
	if !admin.CanManageProject("future") {
		t.Fatal("admin must own future projects")
	}
	admin.Disabled = true
	if admin.CanRead("p/s") {
		t.Fatal("disabled account can read")
	}
}

func TestValidation(t *testing.T) {
	s, _ := newTestStore(t)
	for _, in := range []NewUser{
		{Username: "A", Password: "long-enough"},
		{Username: "Bad Name", Password: "long-enough"},
		{Username: "ok", Password: "short"},
		{Username: "ok", Role: "root", Password: "long-enough"},
		{Username: "ok", Projects: map[string]ProjectRole{"../etc": ProjectView}, Password: "long-enough"},
	} {
		if _, err := s.CreateUser(in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if _, err := s.CreateUser(NewUser{Username: "admin", Password: "long-enough"}); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestLimiterBacksOff(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter()
	l.now = c.now
	for range 4 {
		l.Fail("ip")
	}
	if l.Wait("ip") != 0 {
		t.Fatal("locked before 5 failures")
	}
	l.Fail("ip")
	if got := l.Wait("ip", "other"); got != 30*time.Second {
		t.Fatalf("first lock %v", got)
	}
	l.Fail("ip")
	if got := l.Wait("ip"); got != time.Minute {
		t.Fatalf("second lock %v", got)
	}
	for range 20 {
		l.Fail("ip")
	}
	if got := l.Wait("ip"); got != 15*time.Minute {
		t.Fatalf("cap %v", got)
	}
	l.Succeed("ip")
	if l.Wait("ip") != 0 {
		t.Fatal("success must reset")
	}
}

func TestLegacyBooksMigrateOnRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web-accounts.json")
	// 手工构造 v1 文件：books 用不带 "/" 的旧式目录名。
	old := `{"version":1,"users":[{"username":"w","name":"","role":"writer","books":["book","other"],"disabled":false,"passwordHash":"x","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}],"sessions":[],"pairings":[]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(path)
	users, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	got := users[0].Projects
	if !projectsEqual(got, map[string]ProjectRole{"_": ProjectCollab}) {
		t.Fatalf("legacy projects must migrate: %v", got)
	}
	// 任何一次写回都把文件升级到 version 2。
	if _, err := s.MarkOnboarded("w"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Version != fileVersion {
		t.Fatalf("file must be rewritten as v%d: %v %s", fileVersion, err, raw)
	}
}

func TestMustChangePasswordLifecycle(t *testing.T) {
	s, c := newTestStore(t)
	v, err := s.CreateUser(NewUser{Username: "temp", Role: RoleMember, Password: "temp-pass-1", MustChangePassword: true})
	if err != nil {
		t.Fatal(err)
	}
	if !v.MustChangePassword {
		t.Fatal("web-created user must be flagged")
	}
	v, err = s.CreateUser(NewUser{Username: "cli", Role: RoleMember, Password: "cli-pass-1"})
	if err != nil || v.MustChangePassword {
		t.Fatalf("CLI-style create must not be flagged: %v %v", v, err)
	}

	// 管理员重置他人密码 → 置标记；自己改密成功 → 清标记。
	tru, pass := true, "reset-pass-1"
	v, err = s.UpdateUser("cli", UserPatch{Password: &pass, MustChangePassword: &tru})
	if err != nil || !v.MustChangePassword {
		t.Fatalf("admin reset must flag: %v %v", v, err)
	}
	if _, err := s.Login("cli", "reset-pass-1", "Mac", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword("cli", "reset-pass-1", "final-pass-1", ""); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(time.Second)
	users, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username == "cli" && u.MustChangePassword {
			t.Fatal("successful password change must clear the flag")
		}
	}
}

func TestOnboardedAndUserListStats(t *testing.T) {
	s, c := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "ww", Role: RoleMember, Password: "writer-pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login("ww", "writer-pass", "Mac", ""); err != nil {
		t.Fatal(err)
	}
	v, err := s.MarkOnboarded("ww")
	if err != nil || v.OnboardedAt == nil || !v.OnboardedAt.Equal(c.t) {
		t.Fatalf("onboardedAt = %v, err %v", v.OnboardedAt, err)
	}
	c.t = c.t.Add(time.Hour)
	v2, err := s.MarkOnboarded("ww")
	if err != nil || !v2.OnboardedAt.Equal(*v.OnboardedAt) {
		t.Fatalf("second call must keep first timestamp: %v", v2.OnboardedAt)
	}
	c.t = c.t.Add(touchInterval + time.Minute)
	ns, err := s.Login("ww", "writer-pass", "iPhone", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ns.Cookie); err != nil {
		t.Fatal(err)
	}
	users, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username != "ww" {
			continue
		}
		if u.SessionCount != 2 || u.LastSeenAt == nil || !u.LastSeenAt.Equal(c.t) {
			t.Fatalf("sessionCount/lastSeenAt = %d/%v", u.SessionCount, u.LastSeenAt)
		}
	}
	if _, err := s.MarkOnboarded("ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestProjectPermissionValidation(t *testing.T) {
	s, _ := newTestStore(t)
	for i, projects := range []map[string]ProjectRole{{"p": ProjectCollab}, {"_": ProjectView}, {"p": ProjectOwner}, {"*": ProjectView}} {
		if _, err := s.CreateUser(NewUser{Username: fmt.Sprintf("valid%d", i), Projects: projects, Password: "writer-pass"}); err != nil {
			t.Fatal(err)
		}
	}
	for i, projects := range []map[string]ProjectRole{{"p": "writer"}, {".": ProjectView}, {"p/s": ProjectOwner}, {"../etc": ProjectView}} {
		if _, err := s.CreateUser(NewUser{Username: fmt.Sprintf("bad%d", i), Projects: projects, Password: "writer-pass"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("accepted invalid project permissions: %v", err)
		}
	}
}
