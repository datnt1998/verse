package postgres_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

// TestConcurrentLastAdmin races demote-last-admin mutations: một bên thắng,
// bên kia ErrLastAdmin, và không bao giờ về 0 admin.
func TestConcurrentLastAdmin(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	member := identity.RoleMember
	s1 := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	s2 := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "a1", Role: identity.RoleAdmin, Password: "mat-khau-12345"},
		{Username: "a2", Role: identity.RoleAdmin, Password: "mat-khau-12345"},
	} {
		if _, err := s1.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, errs[0] = s1.UpdateUser("a1", identity.UserPatch{Role: &member}) }()
	go func() { defer wg.Done(); _, errs[1] = s2.UpdateUser("a2", identity.UserPatch{Role: &member}) }()
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, identity.ErrLastAdmin) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("last-admin race must keep exactly one admin, succeeded=%d", succeeded)
	}
	users, err := s1.Users()
	if err != nil {
		t.Fatal(err)
	}
	admins := 0
	for _, u := range users {
		if u.Role == identity.RoleAdmin && !u.Disabled {
			admins++
		}
	}
	if admins != 1 {
		t.Fatalf("must remain exactly one active admin, got %d", admins)
	}
}

// TestConcurrentOwner races transfer-owner requests: kết quả nối tiếp —
// project luôn còn đúng một owner, không mất owner, không hai owner.
func TestConcurrentOwner(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	s1 := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	s2 := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "old", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"da": identity.ProjectOwner}, Password: "mat-khau-12345"},
		{Username: "new1", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"da": identity.ProjectCollab}, Password: "mat-khau-12345"},
		{Username: "new2", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"da": identity.ProjectCollab}, Password: "mat-khau-12345"},
	} {
		if _, err := s1.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	actor := &identity.AuditActor{Username: "old"}
	wg.Add(2)
	go func() { defer wg.Done(); errs[0] = s1.TransferProjectOwner("da", "new1", actor) }()
	go func() { defer wg.Done(); errs[1] = s2.TransferProjectOwner("da", "new2", actor) }()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("transfer raced to error: %v", err)
		}
	}
	members, err := s1.ProjectMembers("da")
	if err != nil {
		t.Fatal(err)
	}
	owners := []string{}
	for _, m := range members {
		if m.Role == identity.ProjectOwner {
			owners = append(owners, m.Username)
		}
	}
	if len(owners) != 1 || (owners[0] != "new1" && owners[0] != "new2") {
		t.Fatalf("exactly one owner must survive the race: %+v", members)
	}

	// Race grant-owner × remove-owner: cuối cùng vẫn đúng một owner.
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = s1.RemoveProjectMember("da", owners[0], actor)
	}()
	go func() {
		defer wg.Done()
		errs[1] = s2.TransferProjectOwner("da", "old", actor)
	}()
	wg.Wait()
	// Một trong hai có thể thua (ErrLastOwner) — nhưng invariant không vỡ.
	for _, err := range errs {
		if err != nil && !errors.Is(err, identity.ErrLastOwner) && !errors.Is(err, identity.ErrNotFound) {
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	members, err = s1.ProjectMembers("da")
	if err != nil {
		t.Fatal(err)
	}
	owners = owners[:0]
	for _, m := range members {
		if m.Role == identity.ProjectOwner {
			owners = append(owners, m.Username)
		}
	}
	if len(owners) != 1 {
		t.Fatalf("owner invariant broken after remove×transfer race: %+v", members)
	}
}
