// mkaccounts writes the real v4 web-accounts.json sources the cutover
// rehearsal feeds to `storage sso-import`. Every file goes through the same
// identity.Store the file-mode apps used — flock, atomic replace, argon2id —
// never a hand-written fixture.
//
// Plaintext passwords arrive only through the SSO_FIXTURE_PASSWORDS env var
// (a JSON object keyed by username), never through argv or a file. The one
// live file-mode session the rehearsal needs is printed to stdout exactly
// once, prefixed, and is never logged anywhere else.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/datnt1998/verse/platform/identity"
)

const passwordsEnv = "SSO_FIXTURE_PASSWORDS"

// accountUsers is the fixed account cast of the rehearsal: two Noverse
// members plus one admin, a one-time temp-password member, and the Viderse
// pair. nvmember also lands in the colliding Viderse variant.
var accountUsers = []string{"nvadmin", "nvmember", "nvtemp", "vdadmin", "vdmember"}

type accountSpec struct {
	user    string
	role    identity.Role
	temp    bool // created with a one-time temporary password (must change)
	session bool // additionally mint a live file-mode session
}

// writeAccounts builds one account file through the real store and returns
// the cookie of the session it minted, if any spec asked for one.
func writeAccounts(path string, specs []accountSpec, passwords map[string]string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	store := identity.NewStore(path, identity.Options{})
	session := ""
	for _, spec := range specs {
		pw := passwords[spec.user]
		if len(pw) < identity.MinPasswordLen {
			return "", fmt.Errorf("%s thiếu hoặc quá ngắn cho %q", passwordsEnv, spec.user)
		}
		if _, err := store.CreateUser(identity.NewUser{
			Username:           spec.user,
			Name:               spec.user,
			Role:               spec.role,
			Password:           pw,
			MustChangePassword: spec.temp,
		}); err != nil {
			return "", fmt.Errorf("tạo %s trong %s: %w", spec.user, path, err)
		}
		if spec.session {
			ns, err := store.Login(spec.user, pw, "mkaccounts", "")
			if err != nil {
				return "", fmt.Errorf("mở phiên %s: %w", spec.user, err)
			}
			session = ns.Cookie
		}
	}
	return session, nil
}

// mkaccounts writes noverse/, viderse/ and viderse-collide/ account files
// under dir. viderse-collide additionally carries nvmember so the importer
// can demonstrate the two_sources preflight stop.
func mkaccounts(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return fmt.Errorf("-mkaccounts cần đường dẫn tuyệt đối (%q)", dir)
	}
	var passwords map[string]string
	if err := json.Unmarshal([]byte(os.Getenv(passwordsEnv)), &passwords); err != nil {
		return fmt.Errorf("%s phải là JSON {username: mật khẩu}: %w", passwordsEnv, err)
	}
	for user := range passwords {
		if !slices.Contains(accountUsers, user) {
			return fmt.Errorf("%s chứa username ngoài kịch bản: %q", passwordsEnv, user)
		}
	}
	if len(passwords) != len(accountUsers) {
		return fmt.Errorf("%s cần đúng %d username: %v", passwordsEnv, len(accountUsers), accountUsers)
	}

	session, err := writeAccounts(filepath.Join(dir, "noverse", "web-accounts.json"), []accountSpec{
		{user: "nvadmin", role: identity.RoleAdmin},
		{user: "nvmember", role: identity.RoleMember, session: true},
		{user: "nvtemp", role: identity.RoleMember, temp: true},
	}, passwords)
	if err != nil {
		return err
	}
	viderse := []accountSpec{
		{user: "vdadmin", role: identity.RoleAdmin},
		{user: "vdmember", role: identity.RoleMember},
	}
	if _, err := writeAccounts(filepath.Join(dir, "viderse", "web-accounts.json"), viderse, passwords); err != nil {
		return err
	}
	collide := append(slices.Clone(viderse), accountSpec{user: "nvmember", role: identity.RoleMember})
	if _, err := writeAccounts(filepath.Join(dir, "viderse-collide", "web-accounts.json"), collide, passwords); err != nil {
		return err
	}
	// The bearer token leaves the fixture exactly once, on stdout, for the
	// harness to hand to the browser jar; it is never logged or persisted.
	fmt.Printf("nvmember-session %s\n", session)
	return nil
}
