package identitycli

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
)

// TestGrantCLIParse verifies the strict operator grammar: only
// grant|revoke + one username + exactly one --app; every other flag
// (actor impersonation, DSN, role) or extra positional is rejected instead
// of being silently reinterpreted.
func TestGrantCLIParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want GrantCommand
	}{
		{"grant noverse", []string{"grant", "linh", "--app", "noverse"},
			GrantCommand{Verb: GrantVerbGrant, Username: "linh", App: identity.AppNoverse}},
		{"revoke viderse equals form", []string{"revoke", "linh", "--app=viderse"},
			GrantCommand{Verb: GrantVerbRevoke, Username: "linh", App: identity.AppViderse}},
		{"flag before username", []string{"grant", "--app", "viderse", "linh"},
			GrantCommand{Verb: GrantVerbGrant, Username: "linh", App: identity.AppViderse}},
	} {
		cmd, err := ParseGrantArgs(tc.args)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if cmd != tc.want {
			t.Fatalf("%s: %+v, want %+v", tc.name, cmd, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"empty", nil},
		{"unknown verb", []string{"promote", "linh", "--app", "noverse"}},
		{"missing username", []string{"grant", "--app", "noverse"}},
		{"extra positional", []string{"grant", "linh", "khac", "--app", "noverse"}},
		{"missing app", []string{"grant", "linh"}},
		{"app without value", []string{"grant", "linh", "--app"}},
		{"unknown app", []string{"grant", "linh", "--app", "chatverse"}},
		{"empty app value", []string{"grant", "linh", "--app="}},
		{"app twice", []string{"grant", "linh", "--app", "noverse", "--app", "viderse"}},
		{"actor impersonation rejected", []string{"grant", "linh", "--app", "noverse", "--actor", "admin"}},
		{"role escalation rejected", []string{"grant", "linh", "--app", "noverse", "--role", "admin"}},
		{"dsn on argv rejected", []string{"grant", "linh", "--app", "noverse", "--dsn", "postgres://u:p@h/db"}},
		{"dsn positional rejected", []string{"grant", "linh", "--app", "noverse", "postgres://u:p@h/db"}},
		{"invalid username", []string{"grant", "X", "--app", "noverse"}},
		{"password flag rejected", []string{"grant", "linh", "--app", "noverse", "--password", "x"}},
	} {
		if cmd, err := ParseGrantArgs(tc.args); err == nil {
			t.Fatalf("%s: expected parse error, got %+v", tc.name, cmd)
		}
	}
}

// TestGrantCLIOperatorBoundary proves the CLI stays inside its contract on a
// backend that cannot store grants: it never creates users, never promotes
// roles and surfaces the backend refusal verbatim (nonzero exit for the
// caller) instead of faking success.
func TestGrantCLIOperatorBoundary(t *testing.T) {
	// No app project rules are needed to exercise the grant contract; the
	// zero Options accepts every project id.
	store := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), identity.Options{})
	if _, err := store.CreateUser(identity.NewUser{Username: "linh", Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"grant", "linh", "--app", "viderse"},
		{"revoke", "linh", "--app", "viderse"},
	} {
		cmd, err := ParseGrantArgs(args)
		if err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		if err := RunGrant(store, cmd); !errors.Is(err, identity.ErrUnsupported) {
			t.Fatalf("RunGrant(%v) = %v, want ErrUnsupported (file backend has no grant table)", args, err)
		}
	}
	if err := RunGrant(nil, GrantCommand{Verb: GrantVerbGrant, Username: "linh", App: identity.AppNoverse}); err == nil {
		t.Fatal("a nil store must fail, not grant silently")
	}
	users, err := store.Users()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Role != identity.RoleMember {
		t.Fatalf("grant verbs must never create or promote accounts: %+v", users)
	}
}
