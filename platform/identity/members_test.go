package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestADAuditAppendOnlyRetention(t *testing.T) {
	s, c := newTestStore(t)
	actor := &AuditActor{Username: "admin", Name: "Quản trị"}
	if _, err := s.CreateUser(NewUser{Username: "member", Role: RoleMember, Projects: map[string]ProjectRole{"p": ProjectCollab}, Password: "member-pass", Actor: actor}); err != nil {
		t.Fatal(err)
	}
	before, err := s.Audit(AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(time.Hour)
	projects := map[string]ProjectRole{"p": ProjectView}
	if _, err := s.UpdateUser("member", UserPatch{Projects: &projects, Actor: actor, Reason: "Đổi nhiệm vụ"}); err != nil {
		t.Fatal(err)
	}
	page, err := s.Audit(AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != len(before.Items)+1 {
		t.Fatalf("audit must append: before %d after %d", len(before.Items), len(page.Items))
	}
	if !reflect.DeepEqual(page.Items[1:], before.Items) {
		t.Fatal("existing audit entries changed")
	}
	if page.Items[0].Actor.Username != "admin" || page.Items[0].Target != "member" || page.Items[0].Reason != "Đổi nhiệm vụ" || page.Items[0].Type != "perm" || page.Items[0].At != c.t {
		t.Fatalf("audit context missing: %+v", page.Items[0])
	}
	first, _ := s.Audit(AuditQuery{Limit: 1})
	second, _ := s.Audit(AuditQuery{Before: first.Next, Limit: 1})
	if first.Next == "" || len(second.Items) != 1 || first.Items[0].ID == second.Items[0].ID {
		t.Fatal("audit pagination must not repeat entries")
	}
	filtered, _ := s.Audit(AuditQuery{Type: "perm", Actor: "admin"})
	if len(filtered.Items) != 3 {
		t.Fatalf("audit filters must include creation role, project grant and later change: %+v", filtered.Items)
	}
	for _, entry := range filtered.Items {
		if entry.Type != "perm" || entry.Actor == nil || entry.Actor.Username != "admin" {
			t.Fatal("audit filter returned another type or actor")
		}
	}
	c.t = c.t.AddDate(1, 0, 0)
	// Boundary is kept; entries strictly older than one calendar year are pruned.
	page, err = s.Audit(AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Reason != "Đổi nhiệm vụ" {
		t.Fatalf("retention: %+v", page.Items)
	}
	raw, _ := os.ReadFile(s.Path())
	if strings.Contains(string(raw), `"action": "Tạo tài khoản"`) {
		t.Fatal("expired audit entries remain on disk")
	}
	if err := s.DeleteUser("member", AuditContext{Actor: actor, Reason: "Đã kết thúc"}); err != nil {
		t.Fatal(err)
	}
	page, _ = s.Audit(AuditQuery{})
	if page.Items[0].Target != "member" || page.Items[0].Reason != "Đã kết thúc" {
		t.Fatal("deleted account audit lost identity")
	}
}

func TestADWildcardMigrationAndImmediateExternalChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	fixture := `{"version":2,"users":[{"username":"admin","role":"admin","books":[]},{"username":"writer","role":"writer","books":["p/story","p/other","*"]},{"username":"reader","role":"reader","books":["p/story"]}],"sessions":[],"pairings":[]}`
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(path, testOptions())
	s.SetProjectSource(func() ([]string, error) { return []string{"p", "q"}, nil })
	users, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]UserView{}
	for _, u := range users {
		by[u.Username] = u.UserView
	}
	if by["admin"].Role != RoleAdmin || !projectsEqual(by["writer"].Projects, map[string]ProjectRole{"p": ProjectCollab, "q": ProjectCollab}) || !projectsEqual(by["reader"].Projects, map[string]ProjectRole{"p": ProjectView}) {
		t.Fatalf("incorrect migration: %+v", by)
	}
	if _, ok := by["writer"].Projects["future"]; ok {
		t.Fatal("wildcard became future access")
	}
	page, _ := s.Audit(AuditQuery{Type: "perm"})
	if len(page.Items) != 1 || page.Items[0].Action != "Di trú quyền" {
		t.Fatal("migration audit missing")
	}
	s.SetProjectSource(func() ([]string, error) { return []string{"p", "q", "future"}, nil })
	if _, err := s.Users(); err != nil {
		t.Fatal(err)
	}
	page, _ = s.Audit(AuditQuery{Type: "perm"})
	if len(page.Items) != 1 {
		t.Fatal("migration ran twice")
	}
	raw, _ := os.ReadFile(path)
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"books"`) {
		t.Fatal("legacy books persisted")
	}
	// Independent store objects emulate a CLI edit while a serve session is active.
	live, _ := newTestStore(t)
	if _, err := live.CreateUser(NewUser{Username: "member", Projects: map[string]ProjectRole{"p": ProjectCollab}, Password: "member-pass"}); err != nil {
		t.Fatal(err)
	}
	login, err := live.Login("member", "member-pass", "Mac", "")
	if err != nil {
		t.Fatal(err)
	}
	if u, _, err := live.Authenticate(login.Cookie); err != nil || !u.CanWrite("p/story") {
		t.Fatal("initial access missing")
	}
	external := NewStore(live.Path(), testOptions())
	external.now = live.now
	projects := map[string]ProjectRole{"p": ProjectView}
	if _, err := external.UpdateUser("member", UserPatch{Projects: &projects}); err != nil {
		t.Fatal(err)
	}
	if u, _, err := live.Authenticate(login.Cookie); err != nil || u.CanWrite("p/story") || !u.CanRead("p/story") {
		t.Fatal("session cached external permissions")
	}
	if err := external.DeleteUser("member"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := live.Authenticate(login.Cookie); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("deleted account session survived")
	}
}

func TestADLastAdminGuardAtomic(t *testing.T) {
	s, _ := newTestStore(t)
	disabled := true
	member := RoleMember
	for _, patch := range []UserPatch{{Disabled: &disabled}, {Role: &member}} {
		if _, err := s.UpdateUser("admin", patch); !errors.Is(err, ErrLastAdmin) {
			t.Fatalf("last-admin mutation allowed: %v", err)
		}
		users, _ := s.Users()
		if len(users) != 1 || users[0].Disabled || users[0].Role != RoleAdmin {
			t.Fatal("rejected mutation changed stored admin")
		}
	}
	if err := s.DeleteUser("admin"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last-admin deletion allowed: %v", err)
	}
}
