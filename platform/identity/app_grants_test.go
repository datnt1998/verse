package identity

import (
	"errors"
	"testing"
)

// TestEffectiveGrants pins the grant model: a global admin is admin of both
// apps without rows, a member needs an explicit row, and an unknown app is
// never invented.
func TestEffectiveGrants(t *testing.T) {
	admin := EffectiveGrants(RoleAdmin, nil)
	if admin[AppNoverse] != RoleAdmin || admin[AppViderse] != RoleAdmin {
		t.Fatalf("global admin grants = %+v", admin)
	}
	member := EffectiveGrants(RoleMember, map[App]Role{AppNoverse: RoleMember})
	if member[AppNoverse] != RoleMember {
		t.Fatalf("explicit Noverse grant lost: %+v", member)
	}
	if _, ok := member[AppViderse]; ok {
		t.Fatalf("member without a Viderse row must have no Viderse grant: %+v", member)
	}
	if len(EffectiveGrants(RoleMember, nil)) != 0 {
		t.Fatal("a member with no rows must have no effective app")
	}
	if !AppNoverse.Valid() || !AppViderse.Valid() || App("other").Valid() {
		t.Fatal("app validity drifted from the storage contract")
	}
}

// TestViewGrantsAreDefensiveCopies pins that request-scoped views never alias
// the store's map: mutating a view must not change the cached account or the
// next projection.
func TestViewGrantsAreDefensiveCopies(t *testing.T) {
	u := User{
		Username: "thanh-vien",
		Role:     RoleMember,
		Projects: map[string]ProjectRole{"da": ProjectView},
		Grants:   map[App]Role{AppNoverse: RoleMember, AppViderse: RoleMember},
	}
	v := u.View()
	delete(v.Grants, AppViderse)
	v.Projects["khac"] = ProjectOwner
	if _, ok := u.Grants[AppViderse]; !ok {
		t.Fatal("view grants alias the account map")
	}
	if _, ok := u.Projects["khac"]; ok {
		t.Fatal("view projects alias the account map")
	}

	s := NewStore(t.TempDir()+"/web-accounts.json", testOptions())
	back := s.UserFromView(v)
	if _, ok := back.Grants[AppViderse]; ok {
		t.Fatal("UserFromView must copy the grants map")
	}
	delete(v.Grants, AppNoverse)
	if _, ok := back.Grants[AppNoverse]; !ok {
		t.Fatal("mutating the view after UserFromView changed the built user")
	}
}

// TestFileStoreGrantCapability pins the backend boundary: file mode conveys
// Noverse access through its role, but cross-app grant mutation needs the
// Postgres backend and must never pretend success.
func TestFileStoreGrantCapability(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "thanh-vien", Role: RoleMember, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	grants, err := s.AppGrants("thanh-vien")
	if err != nil {
		t.Fatal(err)
	}
	if grants[AppNoverse] != RoleMember {
		t.Fatalf("file member must have implicit Noverse access: %+v", grants)
	}
	if _, ok := grants[AppViderse]; ok {
		t.Fatalf("file member has no Viderse app: %+v", grants)
	}
	if err := s.SetAppGrant("thanh-vien", AppViderse, RoleMember, &AuditActor{Username: "admin"}, ""); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("file SetAppGrant = %v, want ErrUnsupported", err)
	}
	if err := s.OperatorRevokeAppGrant("thanh-vien", AppNoverse); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("file OperatorRevokeAppGrant = %v, want ErrUnsupported", err)
	}
	admin, err := s.AppGrants("admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin[AppNoverse] != RoleAdmin || admin[AppViderse] != RoleAdmin {
		t.Fatalf("file admin must inherit both apps: %+v", admin)
	}
}
