// Package webauthtest chứa contract suite dùng chung cho mọi
// webauth.Repository backend (file web-accounts.json, postgres). Mỗi backend
// cung cấp Factory; suite assert cùng ngữ nghĩa quan sát được: envelope/role
// names, generic credential errors, mật khẩu tạm một lần + restricted session,
// session/revoke/pairing/recovery, member APIs và các invariant owner/admin.
//
// Suite KHÔNG assert storage internals (token_hash bytea, .v3.bak…) — các
// kiểm chứng đó ở test riêng của từng backend.
package webauthtest

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// Factory trả một backend mới, rỗng cho mỗi case. Thêm user phải qua các API
// public (CreateUser/CreateFirstAdmin…) — không seed đường tắt.
type Factory func(t *testing.T) *webauth.Store

const (
	pass1 = "mat-khau-mot-234"
	pass2 = "mat-khau-hai-345"
)

func sessionIDOf(t *testing.T, cookie string) string {
	t.Helper()
	id, _, ok := strings.Cut(cookie, ".")
	if !ok || id == "" {
		t.Fatalf("cookie shape must be id.secret: %q", cookie)
	}
	return id
}

func isErr(t *testing.T, err, want error, what string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: got %v, want %v", what, err, want)
	}
}

// Run chạy toàn bộ contract suite trên backend do factory dựng.
func Run(t *testing.T, factory Factory) {
	t.Run("create_users_and_roles", func(t *testing.T) {
		s := factory(t)
		v, err := s.CreateUser(webauth.NewUser{
			Username: "ta", Name: "Hai Ký Tự", Role: webauth.RoleMember,
			Projects: map[string]webauth.ProjectRole{"du-an": webauth.ProjectOwner},
			Password: pass1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if v.Role != webauth.RoleMember || v.Projects["du-an"] != webauth.ProjectOwner {
			t.Fatalf("create view: %+v", v)
		}
		if _, err := s.CreateUser(webauth.NewUser{Username: "ta", Role: webauth.RoleMember, Password: pass1}); !errors.Is(err, webauth.ErrExists) {
			t.Fatalf("duplicate username: %v", err)
		}
		// unique không phân biệt hoa/thường (PG citext parity)
		if _, err := s.CreateUser(webauth.NewUser{Username: "TA", Role: webauth.RoleMember, Password: pass1}); err == nil {
			t.Fatal("case-variant username must not claim the account")
		}
		for _, bad := range []struct {
			name string
			in   webauth.NewUser
		}{
			{"global collaborator", webauth.NewUser{Username: "bad1", Role: webauth.Role("collaborator"), Password: pass1}},
			{"project editor", webauth.NewUser{Username: "bad2", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"p": "editor"}, Password: pass1}},
			{"bad username", webauth.NewUser{Username: "UPPER CASE", Role: webauth.RoleMember, Password: pass1}},
			{"short password", webauth.NewUser{Username: "bad3", Role: webauth.RoleMember, Password: "abc"}},
			{"one-char username", webauth.NewUser{Username: "x", Role: webauth.RoleMember, Password: pass1}},
		} {
			if _, err := s.CreateUser(bad.in); !errors.Is(err, webauth.ErrInvalidInput) {
				t.Fatalf("%s: got %v, want ErrInvalidInput", bad.name, err)
			}
		}
		// Owner lấy qua projects của user mới → owner thứ hai bị chặn.
		if _, err := s.CreateUser(webauth.NewUser{Username: "owner2", Role: webauth.RoleMember,
			Projects: map[string]webauth.ProjectRole{"du-an": webauth.ProjectOwner}, Password: pass1}); !errors.Is(err, webauth.ErrOwnerTaken) {
			t.Fatalf("second owner grant: %v", err)
		}
		// Thiếu global role mặc định member; Users liệt kê kèm session stats.
		if _, err := s.CreateUser(webauth.NewUser{Username: "nguoi-moi", Password: pass1}); err != nil {
			t.Fatal(err)
		}
		users, err := s.Users()
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]webauth.UserListView{}
		for _, u := range users {
			seen[u.Username] = u
		}
		if len(seen) != 2 || seen["nguoi-moi"].Role != webauth.RoleMember || seen["ta"].Projects["du-an"] != webauth.ProjectOwner {
			t.Fatalf("users: %+v", users)
		}
		if has, err := s.HasUsers(); err != nil || !has {
			t.Fatalf("HasUsers: %v %v", has, err)
		}
	})

	t.Run("login_generic_errors_and_timing", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "thanh-vien", Role: webauth.RoleMember, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		// unknown user must still pay a full argon2 verify (no enumeration).
		for _, tc := range []struct{ user, pass string }{
			{"khong-ton-tai", pass1},
			{"thanh-vien", "sai-mat-khau-1"},
		} {
			start := time.Now()
			_, err := s.Login(tc.user, tc.pass, "Mac", "10.0.0.1")
			isErr(t, err, webauth.ErrInvalidCredentials, "login "+tc.user)
			if d := time.Since(start); d < 15*time.Millisecond {
				t.Fatalf("login %q answered in %s — credential check was skipped (enumeration)", tc.user, d)
			}
		}
		off := true
		if _, err := s.UpdateUser("thanh-vien", webauth.UserPatch{Disabled: &off}); err != nil {
			t.Fatal(err)
		}
		isErr(t, errorCheck(func() error { _, err := s.Login("thanh-vien", pass1, "", ""); return err }), webauth.ErrInvalidCredentials, "disabled login")
		on := false
		if _, err := s.UpdateUser("thanh-vien", webauth.UserPatch{Disabled: &on}); err != nil {
			t.Fatal(err)
		}
		ns, err := s.Login("thanh-vien", pass1, "Mac", "10.0.0.1")
		if err != nil {
			t.Fatalf("re-enabled login: %v", err)
		}
		if ns.User.Username != "thanh-vien" {
			t.Fatalf("login view: %+v", ns.User)
		}
	})

	t.Run("session_lifecycle", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "chu", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		one, err := s.Login("chu", pass1, "Mac", "10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		two, err := s.Login("chu", pass1, "Phone", "10.0.0.2")
		if err != nil {
			t.Fatal(err)
		}
		u, cur, err := s.Authenticate(one.Cookie)
		if err != nil || u.Username != "chu" || !cur.Current {
			t.Fatalf("authenticate: %v %+v %+v", err, u, cur)
		}
		sessions, err := s.Sessions("chu", sessionIDOf(t, one.Cookie))
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 2 {
			t.Fatalf("sessions: %+v", sessions)
		}
		// Revoke the other session by id; current survives.
		isErr(t, s.RevokeSession(sessionIDOf(t, two.Cookie), func(owner string) bool { return owner == "chu" }), nil, "revoke other")
		isErr(t, errorCheck(func() error { _, _, err := s.Authenticate(two.Cookie); return err }), webauth.ErrUnauthenticated, "revoked session")
		// allowed=false must deny.
		isErr(t, s.RevokeSession(sessionIDOf(t, one.Cookie), func(string) bool { return false }), webauth.ErrForbidden, "revoke denied")
		// RevokeAll keeps the named session only.
		three, err := s.Login("chu", pass1, "Tablet", "10.0.0.3")
		if err != nil {
			t.Fatal(err)
		}
		isErr(t, s.RevokeAll("chu", sessionIDOf(t, three.Cookie), webauth.AuditContext{}), nil, "revoke all")
		isErr(t, errorCheck(func() error { _, _, err := s.Authenticate(one.Cookie); return err }), webauth.ErrUnauthenticated, "revoke-all dropped old")
		if _, _, err := s.Authenticate(three.Cookie); err != nil {
			t.Fatalf("kept session: %v", err)
		}
	})

	t.Run("password_lifecycle", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "chu", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		one, _ := s.Login("chu", pass1, "Mac", "")
		two, _ := s.Login("chu", pass1, "Phone", "")
		isErr(t, s.ChangePassword("chu", "sai-hien-tai", pass2, sessionIDOf(t, one.Cookie)), webauth.ErrForbidden, "wrong current")
		isErr(t, s.ChangePassword("chu", pass1, pass2, sessionIDOf(t, one.Cookie)), nil, "change")
		if _, _, err := s.Authenticate(one.Cookie); err != nil {
			t.Fatalf("current session must survive password change: %v", err)
		}
		isErr(t, errorCheck(func() error { _, _, err := s.Authenticate(two.Cookie); return err }), webauth.ErrUnauthenticated, "other session revoked")
		isErr(t, errorCheck(func() error { _, err := s.Login("chu", pass1, "", ""); return err }), webauth.ErrInvalidCredentials, "old password")
		if _, err := s.Login("chu", pass2, "", ""); err != nil {
			t.Fatalf("new password login: %v", err)
		}
		// admin reset: password patch + MustChangePassword → temp một lần.
		force := true
		newPass := "mat-khau-tam-456"
		if _, err := s.UpdateUser("chu", webauth.UserPatch{Password: &newPass, MustChangePassword: &force}); err != nil {
			t.Fatal(err)
		}
		isErr(t, errorCheck(func() error { _, _, err := s.Authenticate(one.Cookie); return err }), webauth.ErrUnauthenticated, "reset revokes sessions")
		ns, err := s.Login("chu", newPass, "", "")
		if err != nil {
			t.Fatalf("temp login: %v", err)
		}
		if !ns.User.MustChangePassword {
			t.Fatal("temp login must stay restricted")
		}
	})

	t.Run("temporary_password_one_time", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "moi", Role: webauth.RoleMember, Password: "temp-pass-111", MustChangePassword: true}); err != nil {
			t.Fatal(err)
		}
		ns, err := s.Login("moi", "temp-pass-111", "Mac", "")
		if err != nil {
			t.Fatalf("first temp login: %v", err)
		}
		if !ns.User.MustChangePassword {
			t.Fatal("restricted session expected")
		}
		// Mã một lần: login thứ hai bằng cùng mật khẩu tạm phải hỏng.
		isErr(t, errorCheck(func() error { _, err := s.Login("moi", "temp-pass-111", "Mac2", ""); return err }), webauth.ErrInvalidCredentials, "temp reuse")
		// Phiên restricted đổi được mật khẩu dù mã đã đốt.
		isErr(t, s.ChangePassword("moi", "temp-pass-111", pass1, sessionIDOf(t, ns.Cookie)), nil, "restricted change")
		u, _, err := s.Authenticate(ns.Cookie)
		if err != nil || u.MustChangePassword {
			t.Fatalf("change must clear restriction and keep session: %v %+v", err, u)
		}
		isErr(t, errorCheck(func() error { _, err := s.Login("moi", "temp-pass-111", "", ""); return err }), webauth.ErrInvalidCredentials, "temp after change")
		if _, err := s.Login("moi", pass1, "", ""); err != nil {
			t.Fatalf("permanent login: %v", err)
		}
	})

	t.Run("pairing", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "chu", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		isErr(t, errorCheck(func() error { _, err := s.Pair("deadbeefdeadbeef", "", ""); return err }), webauth.ErrInvalidPairing, "bad pairing")
		code, expires, err := s.NewPairing("chu", "chu")
		if err != nil || code == "" || expires.Before(time.Now()) {
			t.Fatalf("new pairing: %q %v %v", code, expires, err)
		}
		ns, err := s.Pair(code, "Phone", "")
		if err != nil {
			t.Fatalf("pair: %v", err)
		}
		if ns.User.Username != "chu" {
			t.Fatalf("paired user: %+v", ns.User)
		}
		// Mã QR một lần.
		isErr(t, errorCheck(func() error { _, err := s.Pair(code, "", ""); return err }), webauth.ErrInvalidPairing, "pairing reuse")
	})

	t.Run("recovery", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "admin1", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(webauth.NewUser{Username: "mem1", Role: webauth.RoleMember, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		before, _ := s.Login("admin1", pass1, "Mac", "")
		// Recovery chỉ dành cho admin.
		isErr(t, errorCheck(func() error { _, _, err := s.IssueRecovery("mem1", false); return err }), webauth.ErrForbidden, "member recovery")
		code, _, err := s.IssueRecovery("admin1", false)
		if err != nil || code == "" {
			t.Fatalf("issue: %q %v", code, err)
		}
		st, err := s.RecoveryStatus("admin1")
		if err != nil || st.CreatedAt == nil || st.Used || st.Mask == "" || !strings.Contains(st.Mask, "•") {
			t.Fatalf("status: %+v %v", st, err)
		}
		if strings.Contains(st.Mask, code) {
			t.Fatalf("mask leaked code: %q", st.Mask)
		}
		isErr(t, errorCheck(func() error { _, _, err := s.IssueRecovery("admin1", false); return err }), webauth.ErrRecoveryExists, "second issue")
		isErr(t, errorCheck(func() error { _, _, err := s.Recover("admin1", "AAAA-BBBB-CCCC-DDDD", pass2, "", ""); return err }), webauth.ErrInvalidRecovery, "wrong code")
		ns, revoked, err := s.Recover("admin1", code, pass2, "Mac", "")
		if err != nil {
			t.Fatalf("recover: %v", err)
		}
		if len(revoked) == 0 {
			t.Fatal("recover must report revoked session ids")
		}
		isErr(t, errorCheck(func() error { _, _, err := s.Authenticate(before.Cookie); return err }), webauth.ErrUnauthenticated, "pre-recovery session")
		isErr(t, errorCheck(func() error { _, _, err := s.Recover("admin1", code, pass1, "", ""); return err }), webauth.ErrInvalidRecovery, "code reuse")
		if _, _, err := s.Authenticate(ns.Cookie); err != nil {
			t.Fatalf("recovery session: %v", err)
		}
		isErr(t, errorCheck(func() error { _, err := s.Login("admin1", pass1, "", ""); return err }), webauth.ErrInvalidCredentials, "old password")
		if _, err := s.Login("admin1", pass2, "", ""); err != nil {
			t.Fatalf("new password: %v", err)
		}
		// rotate cấp mã mới, mã cũ chết ngay.
		code2, _, err := s.IssueRecovery("admin1", true)
		if err != nil || code2 == code {
			t.Fatalf("rotate: %q %v", code2, err)
		}
	})

	t.Run("project_memberships", func(t *testing.T) {
		s := factory(t)
		actor := &webauth.AuditActor{Username: "chu", Name: "Chủ"}
		if _, err := s.CreateUser(webauth.NewUser{Username: "chu", Role: webauth.RoleMember,
			Projects: map[string]webauth.ProjectRole{"da": webauth.ProjectOwner}, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(webauth.NewUser{Username: "ctv", Role: webauth.RoleMember, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(webauth.NewUser{Username: "xem", Role: webauth.RoleMember, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		members, err := s.ProjectMembers("da")
		if err != nil || len(members) != 1 || members[0].Username != "chu" || members[0].Role != webauth.ProjectOwner {
			t.Fatalf("members: %+v %v", members, err)
		}
		if _, err := s.SetProjectMember("da", "ctv", webauth.ProjectCollab, actor); err != nil {
			t.Fatalf("invite collab: %v", err)
		}
		if _, err := s.SetProjectMember("da", "xem", webauth.ProjectView, actor); err != nil {
			t.Fatalf("invite view: %v", err)
		}
		// owner chỉ đi qua TransferProjectOwner.
		isErr(t, errorCheck(func() error { _, err := s.SetProjectMember("da", "xem", webauth.ProjectOwner, actor); return err }), webauth.ErrInvalidInput, "owner via set")
		// Hạ owner cuối → ErrLastOwner.
		isErr(t, errorCheck(func() error { _, err := s.SetProjectMember("da", "chu", webauth.ProjectCollab, actor); return err }), webauth.ErrLastOwner, "demote last owner")
		isErr(t, s.RemoveProjectMember("da", "chu", actor), webauth.ErrLastOwner, "remove last owner")
		isErr(t, s.RemoveProjectMember("da", "khong-co", actor), webauth.ErrNotFound, "remove missing")
		// Transfer: đúng một owner, owner cũ về collab.
		isErr(t, s.TransferProjectOwner("da", "ctv", actor), nil, "transfer")
		members, err = s.ProjectMembers("da")
		if err != nil {
			t.Fatal(err)
		}
		roles := map[string]webauth.ProjectRole{}
		owners := 0
		for _, m := range members {
			roles[m.Username] = m.Role
			if m.Role == webauth.ProjectOwner {
				owners++
			}
		}
		if owners != 1 || roles["ctv"] != webauth.ProjectOwner || roles["chu"] != webauth.ProjectCollab || roles["xem"] != webauth.ProjectView {
			t.Fatalf("after transfer: %+v", members)
		}
		isErr(t, s.TransferProjectOwner("da", "khong-co", actor), webauth.ErrNotFound, "transfer missing user")
		isErr(t, s.RemoveProjectMember("da", "xem", actor), nil, "remove viewer")
	})

	t.Run("admin_invariants_and_delete", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "admin1", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(webauth.NewUser{Username: "own", Role: webauth.RoleMember,
			Projects: map[string]webauth.ProjectRole{"da": webauth.ProjectOwner}, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		member := webauth.RoleMember
		isErr(t, errorCheck(func() error { _, err := s.UpdateUser("admin1", webauth.UserPatch{Role: &member}); return err }), webauth.ErrLastAdmin, "demote last admin")
		off := true
		isErr(t, errorCheck(func() error { _, err := s.UpdateUser("admin1", webauth.UserPatch{Disabled: &off}); return err }), webauth.ErrLastAdmin, "disable last admin")
		isErr(t, s.DeleteUser("admin1"), webauth.ErrLastAdmin, "delete last admin")
		// Không xoá/khoá owner cuối của project.
		isErr(t, s.DeleteUser("own"), webauth.ErrLastOwner, "delete last owner")
		isErr(t, errorCheck(func() error { _, err := s.UpdateUser("own", webauth.UserPatch{Disabled: &off}); return err }), webauth.ErrLastOwner, "disable last owner")
		// Xoá member thường: tombstone chặn tái sử dụng username.
		if _, err := s.CreateUser(webauth.NewUser{Username: "tam", Role: webauth.RoleMember, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login("tam", pass1, "", ""); err != nil {
			t.Fatal(err)
		}
		isErr(t, s.DeleteUser("tam"), nil, "delete member")
		isErr(t, errorCheck(func() error { _, err := s.Login("tam", pass1, "", ""); return err }), webauth.ErrInvalidCredentials, "deleted login")
		if _, err := s.CreateUser(webauth.NewUser{Username: "tam", Role: webauth.RoleMember, Password: pass1}); !errors.Is(err, webauth.ErrExists) {
			t.Fatalf("deleted username must stay reserved: %v", err)
		}
	})

	t.Run("audit_and_security_failures", func(t *testing.T) {
		s := factory(t)
		if _, err := s.CreateUser(webauth.NewUser{Username: "actor1", Role: webauth.RoleAdmin, Password: pass1}); err != nil {
			t.Fatal(err)
		}
		isErr(t, s.RecordSecurityFailure("actor1"), nil, "record security failure")
		isErr(t, s.RecordSecurityFailure("khong-ton-tai"), nil, "unknown target is a no-op")
		page, err := s.Audit(webauth.AuditQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 0 {
			t.Fatal("audit must record account actions")
		}
		secOnly, err := s.Audit(webauth.AuditQuery{Type: "sec"})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range secOnly.Items {
			if e.Type != "sec" {
				t.Fatalf("type filter leaked %q", e.Type)
			}
		}
	})

	t.Run("login_limiter_parity", func(t *testing.T) {
		s := factory(t)
		l := s.LoginLimiter()
		for range 5 {
			l.Fail("ip:test")
		}
		if wait := l.Wait("ip:test"); wait <= 0 {
			t.Fatal("limiter must block after repeated failures")
		}
		if wait := l.Wait("ip:other"); wait != 0 {
			t.Fatalf("unrelated key blocked: %s", wait)
		}
		l.Succeed("ip:test")
		if wait := l.Wait("ip:test"); wait != 0 {
			t.Fatalf("succeed did not clear: %s", wait)
		}
	})
}

// errorCheck wraps a call so helpers can assert on the returned error.
func errorCheck(f func() error) error { return f() }
