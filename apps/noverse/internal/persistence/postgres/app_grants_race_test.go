package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

// TestAppGrantRollsBackWhenAuditFails forces the audit INSERT to fail and
// checks the grant row and session revocation rolled back with it: the audit
// log is part of the same transaction, never a best-effort side channel.
func TestAppGrantRollsBackWhenAuditFails(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	s := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	if _, err := s.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	ns := loginOrFatal(t, s, "mem", "mat-khau-12345")
	ctx := context.Background()
	// Drop a column the account audit writer needs so its INSERT fails after
	// the grant row was already written in the same transaction.
	if _, err := pool.Pool().Exec(ctx, `ALTER TABLE audit_logs DROP COLUMN details`); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "admin"}, ""); err == nil {
		t.Fatal("grant succeeded although its audit INSERT failed")
	}
	if grants, err := s.AppGrants("mem"); err != nil {
		t.Fatal(err)
	} else if _, ok := grants[identity.AppViderse]; ok {
		t.Fatalf("failed audit left the grant row: %+v", grants)
	}
	if _, _, err := s.Authenticate(ns.Cookie); err != nil {
		t.Fatalf("failed grant revoked the session instead of rolling back: %v", err)
	}
}

// TestAppGrantActorCheckedAfterLockWait pins the in-transaction actor check:
// the request waits on the invariant lock while the actor is demoted, then
// must observe the new role and refuse the grant. The check is not performed
// before taking the lock.
func TestAppGrantActorCheckedAfterLockWait(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	s := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	if _, err := s.CreateUser(identity.NewUser{Username: "admin-a", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(identity.NewUser{Username: "admin-b", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM auth_invariant_lock WHERE id = 1 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
			&identity.AuditActor{Username: "admin-a"}, "")
	}()

	// Wait for the request to block on the invariant row lock.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := pool.Pool().QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND query ILIKE '%auth_invariant_lock%'
			  AND pid <> pg_backend_pid()`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("grant request never waited on the invariant lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Demote the actor while its grant request is blocked, then release it.
	if _, err := tx.Exec(ctx, `UPDATE users
		SET global_role = 'member', updated_at = now(), version = version + 1
		WHERE username = 'admin-a'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("grant after concurrent demotion = %v, want ErrForbidden", err)
	}
	if grants, err := s.AppGrants("mem"); err != nil {
		t.Fatal(err)
	} else if _, ok := grants[identity.AppViderse]; ok {
		t.Fatalf("demoted actor slipped a grant through: %+v", grants)
	}
	if n := auditCount(t, pool, "mem", "Cấp quyền ứng dụng"); n != 0 {
		t.Fatalf("refused grant wrote audit: %d", n)
	}
}
