package postgres_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/datnt1998/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/webauth"
)

// TestConcurrentFirstAdmin races N first-admin requests on an empty schema:
// đúng một người thắng qua auth_invariant_lock + check-trong-tx; còn lại
// ErrExists. Hai process (hai Accounts instance, cùng pool) cũng ra kết quả
// nối tiếp chứ không race.
func TestConcurrentFirstAdmin(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	const n = 4
	stores := make([]*webauth.Store, n)
	for i := range stores {
		stores[i] = webauth.NewFromRepository(postgres.NewAccounts(pool))
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = stores[i].CreateFirstAdmin(webauth.NewUser{
				Username: "admin-race", Password: "mat-khau-12345"}, "Mac", "")
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, webauth.ErrExists):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("first-admin race must have exactly one winner, got %d", wins)
	}
}

// TestConcurrentLastAdmin races demote-last-admin mutations: một bên thắng,
// bên kia ErrLastAdmin, và không bao giờ về 0 admin.
func TestConcurrentLastAdmin(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	member := webauth.RoleMember
	s1 := webauth.NewFromRepository(postgres.NewAccounts(pool))
	s2 := webauth.NewFromRepository(postgres.NewAccounts(pool))
	for _, u := range []webauth.NewUser{
		{Username: "a1", Role: webauth.RoleAdmin, Password: "mat-khau-12345"},
		{Username: "a2", Role: webauth.RoleAdmin, Password: "mat-khau-12345"},
	} {
		if _, err := s1.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, errs[0] = s1.UpdateUser("a1", webauth.UserPatch{Role: &member}) }()
	go func() { defer wg.Done(); _, errs[1] = s2.UpdateUser("a2", webauth.UserPatch{Role: &member}) }()
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, webauth.ErrLastAdmin) {
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
		if u.Role == webauth.RoleAdmin && !u.Disabled {
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
	s1 := webauth.NewFromRepository(postgres.NewAccounts(pool))
	s2 := webauth.NewFromRepository(postgres.NewAccounts(pool))
	for _, u := range []webauth.NewUser{
		{Username: "old", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"da": webauth.ProjectOwner}, Password: "mat-khau-12345"},
		{Username: "new1", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"da": webauth.ProjectCollab}, Password: "mat-khau-12345"},
		{Username: "new2", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"da": webauth.ProjectCollab}, Password: "mat-khau-12345"},
	} {
		if _, err := s1.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	actor := &webauth.AuditActor{Username: "old"}
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
		if m.Role == webauth.ProjectOwner {
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
		if err != nil && !errors.Is(err, webauth.ErrLastOwner) && !errors.Is(err, webauth.ErrNotFound) {
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	members, err = s1.ProjectMembers("da")
	if err != nil {
		t.Fatal(err)
	}
	owners = owners[:0]
	for _, m := range members {
		if m.Role == webauth.ProjectOwner {
			owners = append(owners, m.Username)
		}
	}
	if len(owners) != 1 {
		t.Fatalf("owner invariant broken after remove×transfer race: %+v", members)
	}
}
