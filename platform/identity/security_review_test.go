package identity

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestReviewMigrationDefersUntilActualRoot(t *testing.T) {
	s, _ := newTestStore(t)
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err = json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy["version"] = 2
	users := legacy["users"].([]any)
	users = append(users, map[string]any{"username": "writer", "role": "writer", "books": []string{"*", "Kenh Audio/01-x"}, "passwordHash": users[0].(map[string]any)["passwordHash"]})
	legacy["users"] = users
	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(s.Path(), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s = NewStore(s.Path(), testOptions())
	if _, err = s.Users(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.Path())
	if string(after) != string(raw) {
		t.Fatal("unknown serve root must defer migration without writing legacy data")
	}
	login, err := s.Login("admin", "admin-pass", "Mac", "")
	if err != nil {
		t.Fatalf("deferred migration must allow login: %v", err)
	}
	after, _ = os.ReadFile(s.Path())
	if !strings.Contains(string(after), `"books"`) || !strings.Contains(string(after), `"*"`) {
		t.Fatal("login lost unresolved legacy permissions")
	}
	s.SetProjectSource(func() ([]string, error) { return nil, errors.New("scan failed") })
	if _, _, err = s.Authenticate(login.Cookie); err != nil {
		t.Fatalf("scan failure must not lock out admin: %v", err)
	}
	after, _ = os.ReadFile(s.Path())
	var pending struct{ Version int }
	if err = json.Unmarshal(after, &pending); err != nil {
		t.Fatal(err)
	}
	if pending.Version != 2 {
		t.Fatal("failed scan must retain old version")
	}
	s.SetProjectSource(func() ([]string, error) { return []string{"Kenh Audio", "actual-root-only"}, nil })
	migrated, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	var writer UserListView
	for _, u := range migrated {
		if u.Username == "writer" {
			writer = u
		}
	}
	if !projectsEqual(writer.Projects, map[string]ProjectRole{"Kenh Audio": ProjectCollab, "actual-root-only": ProjectCollab}) {
		t.Fatalf("migration ignored actual root or adopted ID: %+v", writer.Projects)
	}
	if _, err = s.UpdateUser("writer", UserPatch{Projects: &writer.Projects}); err != nil {
		t.Fatalf("migrated permission map cannot be edited: %v", err)
	}
}
func TestReviewAdoptedProjectIDAndCreationGrants(t *testing.T) {
	s, _ := newTestStore(t)
	projects := map[string]ProjectRole{"Kenh Audio": ProjectOwner, "Dự án (2)": ProjectView}
	actor := &AuditActor{Username: "admin", Name: "Quản trị"}
	if _, err := s.CreateUser(NewUser{Username: "member", Role: RoleMember, Projects: projects, Password: "member-pass", Actor: actor}); err != nil {
		t.Fatalf("adoptable project IDs rejected: %v", err)
	}
	page, err := s.Audit(AuditQuery{Type: "perm"})
	if err != nil {
		t.Fatal(err)
	}
	role, grants := false, false
	for _, e := range page.Items {
		if e.Target == "member" && e.Actor != nil && e.Actor.Username == "admin" {
			role = role || e.To == "member"
			grants = grants || strings.Contains(e.To, "Kenh Audio")
		}
	}
	if !role || !grants {
		t.Fatalf("creation must record role and project grants: %+v", page.Items)
	}
}
func TestReviewSecurityFailureRejectsUntrustedTargets(t *testing.T) {
	s, _ := newTestStore(t)
	for _, username := range []string{strings.Repeat("a", 100000), "not-an-account", "../bad"} {
		if err := s.RecordSecurityFailure(username); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.Audit(AuditQuery{Type: "sec"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("untrusted targets entered audit: %d", len(page.Items))
	}

	if err := s.RecordSecurityFailure("admin"); err != nil {
		t.Fatal(err)
	}
	page, _ = s.Audit(AuditQuery{Type: "sec"})
	if len(page.Items) != 1 || page.Items[0].Target != "admin" {
		t.Fatalf("valid existing target missing: %+v", page.Items)
	}
}
func TestReviewCreationAuditGrants(t *testing.T) {
	s, _ := newTestStore(t)
	actor := &AuditActor{Username: "admin", Name: "Quản trị"}
	if _, err := s.CreateUser(NewUser{Username: "admin2", Role: RoleAdmin, Projects: map[string]ProjectRole{"p": ProjectOwner}, Password: "member-pass", Actor: actor}); err != nil {
		t.Fatal(err)
	}
	page, err := s.Audit(AuditQuery{Type: "perm"})
	if err != nil {
		t.Fatal(err)
	}
	role, projects := false, false
	for _, e := range page.Items {
		if e.Target == "admin2" {
			role = role || e.To == "admin"
			projects = projects || strings.Contains(e.To, `"p":"owner"`)
		}
	}
	if !role || !projects {
		t.Fatalf("creation omitted permission grants: %+v", page.Items)
	}
}
