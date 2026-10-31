package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

func viderseRepo(pool *postgres.Pool) *identitypg.Accounts {
	return identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: false})
}

func loginOrFatal(t *testing.T, s *identity.Store, user, pass string) identity.NewSession {
	t.Helper()
	ns, err := s.Login(user, pass, "Mac", "")
	if err != nil {
		t.Fatalf("login %s: %v", user, err)
	}
	return ns
}

// TestLoginRotatesSessionAndHashesToken pins session-fixation and at-rest
// hardening: every login mints a new id + 32-byte secret, only
// SHA-256(secret) is stored, and a cookie id paired with another secret never
// authenticates.
func TestLoginRotatesSessionAndHashesToken(t *testing.T) {
	s, pool := provisionAccounts(t)
	if _, err := s.CreateUser(identity.NewUser{Username: "chu", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	first := loginOrFatal(t, s, "chu", "mat-khau-12345")
	second := loginOrFatal(t, s, "chu", "mat-khau-12345")
	if first.Cookie == second.Cookie {
		t.Fatal("two logins reused the same session cookie")
	}
	ctx := context.Background()
	for i, ns := range []identity.NewSession{first, second} {
		id, secret, ok := strings.Cut(ns.Cookie, ".")
		if !ok || len(secret) != 64 {
			t.Fatalf("cookie %d shape: %q", i, ns.Cookie)
		}
		var tokenHash []byte
		if err := pool.Pool().QueryRow(ctx,
			`SELECT token_hash FROM sessions WHERE id = $1`, id).Scan(&tokenHash); err != nil {
			t.Fatalf("read token hash %d: %v", i, err)
		}
		sum := sha256.Sum256([]byte(secret))
		if hex.EncodeToString(tokenHash) != hex.EncodeToString(sum[:]) {
			t.Fatalf("session %d does not store SHA-256(secret)", i)
		}
		var leaked int
		if err := pool.Pool().QueryRow(ctx,
			`SELECT count(*) FROM sessions WHERE token_hash = $1 OR user_agent LIKE '%' || $2 || '%'`,
			[]byte(secret), secret).Scan(&leaked); err != nil {
			t.Fatalf("plaintext probe %d: %v", i, err)
		}
		if leaked != 0 {
			t.Fatalf("plaintext secret persisted in session row %d", i)
		}
		if _, _, err := s.Authenticate(ns.Cookie); err != nil {
			t.Fatalf("fresh session %d must authenticate: %v", i, err)
		}
	}
	// Fixed session: an attacker's id with another secret must not pass.
	id, _, _ := strings.Cut(first.Cookie, ".")
	forged := id + "." + strings.Repeat("ab", 32)
	if _, _, err := s.Authenticate(forged); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("forged secret with a valid id = %v, want ErrUnauthenticated", err)
	}
}

// TestUnknownUserRunsPasswordVerifier pins the non-enumeration path: an
// unknown account and a wrong password both execute the Argon2 verifier with a
// real hash and return the same generic error.
func TestUnknownUserRunsPasswordVerifier(t *testing.T) {
	s, _ := provisionAccounts(t)
	if _, err := s.CreateUser(identity.NewUser{Username: "thanh-vien", Role: identity.RoleMember, Password: "mat-khau-mot-234"}); err != nil {
		t.Fatal(err)
	}
	dummy := identity.DummyHashForBackend()
	if !strings.HasPrefix(dummy, "$argon2id$") {
		t.Fatalf("dummy verifier hash is not Argon2id: %q", dummy)
	}
	if identity.VerifyPasswordForBackend("mat-khau-mot-234", dummy) {
		t.Fatal("dummy hash accepted a real password")
	}

	type attempt struct {
		user, pass string
	}
	var firstMsg, secondMsg string
	for i, tc := range []attempt{
		{"khong-ton-tai", "mat-khau-mot-234"},
		{"thanh-vien", "sai-mat-khau-1"},
	} {
		start := time.Now()
		_, err := s.Login(tc.user, tc.pass, "Mac", "10.0.0.1")
		elapsed := time.Since(start)
		if !errors.Is(err, identity.ErrInvalidCredentials) {
			t.Fatalf("login %q = %v, want ErrInvalidCredentials", tc.user, err)
		}
		if elapsed < 15*time.Millisecond {
			t.Fatalf("login %q answered in %s — the Argon2 verifier was skipped", tc.user, elapsed)
		}
		if i == 0 {
			firstMsg = err.Error()
		} else {
			secondMsg = err.Error()
		}
	}
	if firstMsg != secondMsg {
		t.Fatalf("unknown user and wrong password differ: %q vs %q", firstMsg, secondMsg)
	}
}

// TestPasswordChangeRevokesEverySession covers every credential/status change
// that must invalidate every session, including the calling one.
func TestPasswordChangeRevokesEverySession(t *testing.T) {
	t.Run("self_change", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "chu", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		one := loginOrFatal(t, s, "chu", "mat-khau-12345")
		two := loginOrFatal(t, s, "chu", "mat-khau-12345")
		if err := s.ChangePassword("chu", "mat-khau-12345", "mat-khau-moi-999"); err != nil {
			t.Fatal(err)
		}
		for _, ns := range []identity.NewSession{one, two} {
			if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
				t.Fatalf("session survived password change: %v", err)
			}
		}
		if _, err := s.Login("chu", "mat-khau-moi-999", "", ""); err != nil {
			t.Fatalf("login with new password: %v", err)
		}
	})

	t.Run("temporary_change", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "moi", Role: identity.RoleMember,
			Password: "temp-pass-111", MustChangePassword: true}); err != nil {
			t.Fatal(err)
		}
		ns := loginOrFatal(t, s, "moi", "temp-pass-111")
		if err := s.ChangePassword("moi", "temp-pass-111", "mat-khau-moi-999"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("restricted session survived the change: %v", err)
		}
		fresh, err := s.Login("moi", "mat-khau-moi-999", "", "")
		if err != nil {
			t.Fatalf("login after temp change: %v", err)
		}
		if fresh.User.MustChangePassword {
			t.Fatal("flag not cleared after the permanent password")
		}
	})

	t.Run("admin_reset", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		ns := loginOrFatal(t, s, "mem", "mat-khau-12345")
		force, temp := true, "temp-reset-777"
		if _, err := s.UpdateUser("mem", identity.UserPatch{Password: &temp, MustChangePassword: &force}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("reset did not revoke the session: %v", err)
		}
		again, err := s.Login("mem", temp, "", "")
		if err != nil || !again.User.MustChangePassword {
			t.Fatalf("temp login after reset: %v %+v", err, again.User)
		}
	})

	t.Run("disable", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		ns := loginOrFatal(t, s, "mem", "mat-khau-12345")
		off := true
		if _, err := s.UpdateUser("mem", identity.UserPatch{Disabled: &off}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("disable did not revoke the session: %v", err)
		}
		if _, err := s.Login("mem", "mat-khau-12345", "", ""); !errors.Is(err, identity.ErrInvalidCredentials) {
			t.Fatalf("disabled login = %v", err)
		}
	})

	t.Run("revoke_all", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "chu", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		one := loginOrFatal(t, s, "chu", "mat-khau-12345")
		two := loginOrFatal(t, s, "chu", "mat-khau-12345")
		if err := s.RevokeAll("chu", identity.AuditContext{}); err != nil {
			t.Fatal(err)
		}
		for _, ns := range []identity.NewSession{one, two} {
			if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
				t.Fatalf("revoke-all left a session alive: %v", err)
			}
		}
	})

	t.Run("recovery", func(t *testing.T) {
		s, _ := provisionAccounts(t)
		if _, err := s.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
			t.Fatal(err)
		}
		before := loginOrFatal(t, s, "admin", "mat-khau-12345")
		code, _, err := s.IssueRecovery("admin", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Recover("admin", code, "mat-khau-moi-999", "Mac", ""); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Authenticate(before.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("recovery did not revoke the old session: %v", err)
		}
	})
}

func auditCount(t *testing.T, pool *postgres.Pool, target, action string) int {
	t.Helper()
	var n int
	if err := pool.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE target_id = $1 AND action = $2`,
		target, action).Scan(&n); err != nil {
		t.Fatalf("count audit %q/%q: %v", target, action, err)
	}
	return n
}

// TestPrivilegeChangeRevokesAllSessions pins that a real global-role change
// revokes every session and is audited in the same transaction, a no-op does
// nothing, and a rejected mutation leaves both audit and sessions untouched.
func TestPrivilegeChangeRevokesAllSessions(t *testing.T) {
	s, pool := provisionAccounts(t)
	for _, u := range []identity.NewUser{
		{Username: "admin-a", Role: identity.RoleAdmin, Password: "mat-khau-12345"},
		{Username: "admin-b", Role: identity.RoleAdmin, Password: "mat-khau-12345"},
		{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"},
	} {
		if _, err := s.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	promote := identity.RoleAdmin
	first := loginOrFatal(t, s, "mem", "mat-khau-12345")
	second := loginOrFatal(t, s, "mem", "mat-khau-12345")
	if _, err := s.UpdateUser("mem", identity.UserPatch{Role: &promote, Actor: &identity.AuditActor{Username: "admin-a"}}); err != nil {
		t.Fatal(err)
	}
	for _, ns := range []identity.NewSession{first, second} {
		if _, _, err := s.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("role change left a session alive: %v", err)
		}
	}
	if n := auditCount(t, pool, "mem", "Đổi quyền hệ thống"); n != 2 {
		// one from CreateUser, one from the promotion
		t.Fatalf("promotion audit rows = %d, want 2", n)
	}
	after := loginOrFatal(t, s, "mem", "mat-khau-12345")
	if after.User.Role != identity.RoleAdmin || after.User.Grants[identity.AppViderse] != identity.RoleAdmin {
		t.Fatalf("new session must observe the promoted grants: %+v", after.User)
	}

	// Re-applying the same role is a no-op: the session survives and no audit
	// line is added.
	before := auditCount(t, pool, "mem", "Đổi quyền hệ thống")
	if _, err := s.UpdateUser("mem", identity.UserPatch{Role: &promote, Actor: &identity.AuditActor{Username: "admin-a"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(after.Cookie); err != nil {
		t.Fatalf("no-op role patch revoked the session: %v", err)
	}
	if n := auditCount(t, pool, "mem", "Đổi quyền hệ thống"); n != before {
		t.Fatalf("no-op role patch wrote an audit entry: %d → %d", before, n)
	}

	// Demote: only the explicit Noverse grant remains after a fresh login.
	demote := identity.RoleMember
	if _, err := s.UpdateUser("mem", identity.UserPatch{Role: &demote, Actor: &identity.AuditActor{Username: "admin-a"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(after.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("demotion left a session alive: %v", err)
	}
	demoted := loginOrFatal(t, s, "mem", "mat-khau-12345")
	if demoted.User.Grants[identity.AppNoverse] != identity.RoleMember {
		t.Fatalf("explicit Noverse grant lost after demotion: %+v", demoted.User.Grants)
	}
	if _, ok := demoted.User.Grants[identity.AppViderse]; ok {
		t.Fatalf("demoted member kept inherited Viderse admin: %+v", demoted.User.Grants)
	}

	// A rejected privilege mutation commits nothing. Demote admin-a first so
	// admin-b is the last active admin; demoting admin-b must fail in the
	// transaction, leaving its session and the audit log untouched.
	if _, err := s.UpdateUser("admin-a", identity.UserPatch{Role: &demote, Actor: &identity.AuditActor{Username: "admin-b"}}); err != nil {
		t.Fatal(err)
	}
	keep := loginOrFatal(t, s, "admin-b", "mat-khau-12345")
	before = auditCount(t, pool, "admin-b", "Đổi quyền hệ thống")
	if _, err := s.UpdateUser("admin-b", identity.UserPatch{Role: &demote, Actor: &identity.AuditActor{Username: "admin-b"}}); !errors.Is(err, identity.ErrLastAdmin) {
		t.Fatalf("demoting the last admin = %v, want ErrLastAdmin", err)
	}
	if _, _, err := s.Authenticate(keep.Cookie); err != nil {
		t.Fatalf("failed mutation revoked the session: %v", err)
	}
	if n := auditCount(t, pool, "admin-b", "Đổi quyền hệ thống"); n != before {
		t.Fatalf("failed mutation wrote audit: %d → %d", before, n)
	}
}

// TestAppGrantRequiresGlobalAdminAndAudits covers the grant mutation contract:
// only a global admin may change a grant (checked in the transaction), a real
// change writes audit and revokes every target session, a no-op does neither,
// and the trusted operator path records a system audit with granted_by NULL.
func TestAppGrantRequiresGlobalAdminAndAudits(t *testing.T) {
	s, pool := provisionAccounts(t)
	if _, err := s.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	session := loginOrFatal(t, s, "mem", "mat-khau-12345")
	before := auditCount(t, pool, "mem", "Cấp quyền ứng dụng")

	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "mem"}, "tự cấp"); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("member grant = %v, want ErrForbidden", err)
	}
	if grants, err := s.AppGrants("mem"); err != nil || grants[identity.AppViderse] != "" {
		t.Fatalf("forbidden grant wrote a row: %+v %v", grants, err)
	}
	if _, _, err := s.Authenticate(session.Cookie); err != nil {
		t.Fatalf("forbidden grant revoked the session: %v", err)
	}
	if n := auditCount(t, pool, "mem", "Cấp quyền ứng dụng"); n != before {
		t.Fatalf("forbidden grant wrote audit: %d → %d", before, n)
	}

	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, "cấp viderse"); err != nil {
		t.Fatal(err)
	}
	if grants, err := s.AppGrants("mem"); err != nil || grants[identity.AppViderse] != identity.RoleMember {
		t.Fatalf("grant missing: %+v %v", grants, err)
	}
	if n := auditCount(t, pool, "mem", "Cấp quyền ứng dụng"); n != before+1 {
		t.Fatalf("grant audit rows = %d, want %d", n, before+1)
	}
	if _, _, err := s.Authenticate(session.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("grant did not revoke the target session: %v", err)
	}

	// Idempotent repeat: same row, no audit, no revocation of the new session.
	again := loginOrFatal(t, s, "mem", "mat-khau-12345")
	before = auditCount(t, pool, "mem", "Cấp quyền ứng dụng")
	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, ""); err != nil {
		t.Fatal(err)
	}
	if n := auditCount(t, pool, "mem", "Cấp quyền ứng dụng"); n != before {
		t.Fatalf("idempotent grant wrote audit: %d → %d", before, n)
	}
	if _, _, err := s.Authenticate(again.Cookie); err != nil {
		t.Fatalf("idempotent grant revoked the session: %v", err)
	}

	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleAdmin,
		&identity.AuditActor{Username: "admin"}, ""); !errors.Is(err, identity.ErrInvalidInput) {
		t.Fatalf("app admin grant = %v, want ErrInvalidInput", err)
	}
	if err := s.SetAppGrant("mem", identity.App("other"), identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, ""); !errors.Is(err, identity.ErrUnknownApp) {
		t.Fatalf("unknown app grant = %v, want ErrUnknownApp", err)
	}

	if err := s.RevokeAppGrant("mem", identity.AppViderse,
		&identity.AuditActor{Username: "admin"}, "thu hồi"); err != nil {
		t.Fatal(err)
	}
	if grants, err := s.AppGrants("mem"); err != nil {
		t.Fatal(err)
	} else if _, ok := grants[identity.AppViderse]; ok {
		t.Fatalf("revoke left the row: %+v", grants)
	}
	if n := auditCount(t, pool, "mem", "Thu hồi quyền ứng dụng"); n != 1 {
		t.Fatalf("revoke audit rows = %d, want 1", n)
	}
	if _, _, err := s.Authenticate(again.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("revoke did not revoke the target session: %v", err)
	}
	// Revoking a missing row is a no-op with no audit.
	before = auditCount(t, pool, "mem", "Thu hồi quyền ứng dụng")
	if err := s.RevokeAppGrant("mem", identity.AppViderse,
		&identity.AuditActor{Username: "admin"}, ""); err != nil {
		t.Fatal(err)
	}
	if n := auditCount(t, pool, "mem", "Thu hồi quyền ứng dụng"); n != before {
		t.Fatalf("no-op revoke wrote audit: %d → %d", before, n)
	}

	// Trusted operator: granted_by NULL and a system audit.
	if err := s.OperatorSetAppGrant("mem", identity.AppViderse, identity.RoleMember); err != nil {
		t.Fatal(err)
	}
	var grantedByNull bool
	if err := pool.Pool().QueryRow(context.Background(),
		`SELECT granted_by IS NULL FROM app_grants g
		 JOIN users u ON u.id = g.user_id
		 WHERE u.username = 'mem' AND g.app = 'viderse'`).Scan(&grantedByNull); err != nil {
		t.Fatal(err)
	}
	if !grantedByNull {
		t.Fatal("operator grant must leave granted_by NULL")
	}
	var kind string
	if err := pool.Pool().QueryRow(context.Background(),
		`SELECT actor_kind FROM audit_logs
		 WHERE target_id = 'mem' AND action = 'Cấp quyền ứng dụng'
		 ORDER BY sequence DESC LIMIT 1`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "system" {
		t.Fatalf("operator audit actor_kind = %q, want system", kind)
	}
	if err := s.OperatorRevokeAppGrant("mem", identity.AppViderse); err != nil {
		t.Fatal(err)
	}

	// A deleted (tombstoned) account cannot receive a grant.
	if err := s.DeleteUser("mem"); err != nil {
		t.Fatal(err)
	}
	if err := s.OperatorSetAppGrant("mem", identity.AppViderse, identity.RoleMember); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("grant to a deleted account = %v, want ErrNotFound", err)
	}
}

// TestAppGrantSeenBySecondInstance proves grants are read from the database on
// every authentication, never cached in a session or process.
func TestAppGrantSeenBySecondInstance(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	poolA := openPool(t, dsn)
	poolB := openPool(t, dsn)
	a := identity.NewFromRepository(identityRepo(poolA), library.IdentityOptions())
	b := identity.NewFromRepository(identityRepo(poolB), library.IdentityOptions())
	if _, err := a.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	ns := loginOrFatal(t, b, "mem", "mat-khau-12345")
	if _, ok := ns.User.Grants[identity.AppViderse]; ok {
		t.Fatalf("Viderse grant appeared before it was issued: %+v", ns.User.Grants)
	}
	if err := a.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Authenticate(ns.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("grant did not revoke the session across instances: %v", err)
	}
	// The pre-existing session is gone; a fresh login through the second
	// instance observes the new grant.
	fresh := loginOrFatal(t, b, "mem", "mat-khau-12345")
	if fresh.User.Grants[identity.AppViderse] != identity.RoleMember {
		t.Fatalf("second instance did not observe the grant: %+v", fresh.User.Grants)
	}
	if err := a.OperatorRevokeAppGrant("mem", identity.AppViderse); err != nil {
		t.Fatal(err)
	}
	after := loginOrFatal(t, b, "mem", "mat-khau-12345")
	if _, ok := after.User.Grants[identity.AppViderse]; ok {
		t.Fatalf("second instance still observes the revoked grant: %+v", after.User.Grants)
	}
}

// TestSharedLimiterAcrossInstances pins one shared brute-force budget: two
// repository instances alternate failures and both see the block, and the
// account bucket is independent of the caller's casing once folded.
func TestSharedLimiterAcrossInstances(t *testing.T) {
	s1, pool := provisionAccounts(t)
	s2 := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	l1, l2 := s1.LoginLimiter(), s2.LoginLimiter()

	for i := range 4 {
		l := l1
		if i%2 == 1 {
			l = l2
		}
		l.Fail("user:alice", "ip:10.0.0.1")
	}
	if wait := l2.Wait("user:alice"); wait <= 0 {
		t.Fatal("second instance does not see the shared user block")
	}
	if wait := l1.Wait("ip:10.0.0.1"); wait <= 0 {
		t.Fatal("first instance does not see the shared IP block")
	}
	l1.Succeed("user:alice", "ip:10.0.0.1")
	if wait := l2.Wait("user:alice", "ip:10.0.0.1"); wait != 0 {
		t.Fatalf("clear on one instance did not clear the shared buckets: %s", wait)
	}

	// The HTTP layer folds the username before composing the key, so a casing
	// change never opens a second bucket.
	for range 4 {
		l1.Fail("user:" + strings.ToLower("Alice"))
	}
	if wait := l2.Wait("user:alice"); wait <= 0 {
		t.Fatal("casing opened a separate username budget")
	}
	if wait := l2.Wait("user:Alice"); wait != 0 {
		t.Fatal("raw casing must not be a distinct bucket key")
	}
}

// TestProjectAccessFalseSkipsProjectQueries proves the project-less backend
// really does not read project tables: after renaming project_members away,
// login, authentication and user listing still work, while every project,
// global-role, lifecycle and audit method is rejected with a typed error
// before SQL.
func TestProjectAccessFalseSkipsProjectQueries(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	full := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	if _, err := full.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := full.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember,
		Projects: map[string]identity.ProjectRole{"proj": identity.ProjectOwner}, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	ns := loginOrFatal(t, full, "mem", "mat-khau-12345")
	if len(ns.User.Projects) != 1 {
		t.Fatalf("fixture membership missing: %+v", ns.User.Projects)
	}
	ctx := context.Background()
	for _, stmt := range []string{
		`ALTER TABLE project_members RENAME TO project_members_hidden`,
		`ALTER TABLE projects RENAME TO projects_hidden`,
	} {
		if _, err := pool.Pool().Exec(ctx, stmt); err != nil {
			t.Fatalf("hide project table: %v", err)
		}
	}

	repo := viderseRepo(pool)
	u, sess, err := repo.Authenticate(ns.Cookie)
	if err != nil {
		t.Fatalf("project-less authenticate: %v", err)
	}
	if len(u.Projects) != 0 {
		t.Fatalf("project-less projection must not contain projects: %+v", u.Projects)
	}
	if u.Grants[identity.AppNoverse] != identity.RoleMember {
		t.Fatalf("grants must still load: %+v", u.Grants)
	}
	if !sess.Current {
		t.Fatal("session view lost its current flag")
	}
	if _, err := repo.Login("mem", "mat-khau-12345", "", ""); err != nil {
		t.Fatalf("project-less login: %v", err)
	}
	if users, err := repo.Users(); err != nil {
		t.Fatalf("project-less users: %v", err)
	} else {
		for _, v := range users {
			if len(v.Projects) != 0 {
				t.Fatalf("project-less list leaked projects: %+v", v.Projects)
			}
		}
	}
	if err := repo.ChangePassword("mem", "mat-khau-12345", "mat-khau-moi-999"); err != nil {
		t.Fatalf("project-less password change: %v", err)
	}

	unsupported := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, identity.ErrUnsupported) {
			t.Fatalf("%s = %v, want ErrUnsupported", what, err)
		}
	}
	unsupported("ProjectMembers", func() error { _, err := repo.ProjectMembers("proj"); return err }())
	unsupported("SetProjectMember", func() error {
		_, err := repo.SetProjectMember("proj", "mem", identity.ProjectView, nil)
		return err
	}())
	unsupported("RemoveProjectMember", repo.RemoveProjectMember("proj", "mem", nil))
	unsupported("TransferProjectOwner", repo.TransferProjectOwner("proj", "mem", nil))
	unsupported("DeleteUser", repo.DeleteUser("mem"))
	unsupported("Audit", func() error { _, err := repo.Audit(identity.AuditQuery{}); return err }())
	unsupported("PurgeAuditBefore", func() error {
		_, err := repo.PurgeAuditBefore(ctx, time.Now(), false)
		return err
	}())
	role := identity.RoleAdmin
	unsupported("UpdateUser(role)", func() error {
		_, err := repo.UpdateUser("mem", identity.UserPatch{Role: &role})
		return err
	}())
	off := true
	unsupported("UpdateUser(disabled)", func() error {
		_, err := repo.UpdateUser("mem", identity.UserPatch{Disabled: &off})
		return err
	}())
	projects := map[string]identity.ProjectRole{"proj": identity.ProjectView}
	unsupported("UpdateUser(projects)", func() error {
		_, err := repo.UpdateUser("mem", identity.UserPatch{Projects: &projects})
		return err
	}())
}

// TestConcurrentFirstAdminAcrossRepositories is the setup-race gate: separate
// repository instances compete on an empty schema and exactly one creates the
// admin (and its session); every loser gets ErrExists.
func TestConcurrentFirstAdminAcrossRepositories(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	const n = 4
	stores := make([]*identity.Store, n)
	for i := range stores {
		stores[i] = identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	sessions := make([]identity.NewSession, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessions[i], errs[i] = stores[i].CreateFirstAdmin(identity.NewUser{
				Username: "admin-race", Password: "mat-khau-12345"}, "Mac", "")
		}()
	}
	wg.Wait()
	wins := 0
	for i, err := range errs {
		switch {
		case err == nil:
			wins++
			if sessions[i].Cookie == "" {
				t.Fatal("winning setup must return a session")
			}
			if _, _, err := stores[i].Authenticate(sessions[i].Cookie); err != nil {
				t.Fatalf("winner session does not authenticate: %v", err)
			}
		case errors.Is(err, identity.ErrExists):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("first-admin race must have exactly one winner, got %d", wins)
	}
	var users, grants, sessionsCount int
	ctx := context.Background()
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM app_grants WHERE app = 'noverse'`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&sessionsCount); err != nil {
		t.Fatal(err)
	}
	if users != 1 || grants != 1 || sessionsCount != 1 {
		t.Fatalf("race left orphan rows: users=%d grants=%d sessions=%d", users, grants, sessionsCount)
	}
}
