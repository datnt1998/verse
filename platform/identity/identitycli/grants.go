// Package identitycli holds the app-grant operator commands shared by every
// app binary that owns the identity DSN. Each binary parses its own env and
// opens its own account store; this package owns the one grammar and the
// trusted-operator calls so no app re-implements or diverges from it.
//
// The operator is a local principal holding the database credential: there is
// no actor impersonation (--actor is rejected), no role promotion (only
// member rows exist), and no account lifecycle here — grant and revoke only.
package identitycli

import (
	"fmt"
	"strings"

	"github.com/datnt1998/verse/platform/identity"
)

// GrantVerb names the mutation the operator asked for.
type GrantVerb string

const (
	GrantVerbGrant  GrantVerb = "grant"
	GrantVerbRevoke GrantVerb = "revoke"
)

// GrantCommand is one parsed `user grant|revoke <username> --app <app>`.
type GrantCommand struct {
	Verb     GrantVerb
	Username string
	App      identity.App
}

// GrantUsage describes the grammar once so every binary prints the same help.
const GrantUsage = `  user grant <tên> --app noverse|viderse   cấp quyền dùng một app
  user revoke <tên> --app noverse|viderse  thu hồi quyền một app`

// ParseGrantArgs parses `grant|revoke <username> --app <app>` strictly. The
// grammar takes exactly one positional username and the --app flag (either
// `--app value` or `--app=value`) exactly once; any other flag — an actor
// override, a DSN, a role — is an error, never a guess.
func ParseGrantArgs(args []string) (GrantCommand, error) {
	cmd := GrantCommand{}
	if len(args) == 0 {
		return cmd, fmt.Errorf("thiếu lệnh (grant|revoke)\n%s", GrantUsage)
	}
	switch GrantVerb(args[0]) {
	case GrantVerbGrant:
		cmd.Verb = GrantVerbGrant
	case GrantVerbRevoke:
		cmd.Verb = GrantVerbRevoke
	default:
		return cmd, fmt.Errorf("lệnh không hợp lệ %q (grant|revoke)\n%s", args[0], GrantUsage)
	}
	appSet := false
	for i := 1; i < len(args); i++ {
		token := args[i]
		var value string
		switch {
		case token == "--app":
			if i+1 >= len(args) {
				return cmd, fmt.Errorf("--app cần giá trị\n%s", GrantUsage)
			}
			i++
			value = args[i]
			if appSet {
				return cmd, fmt.Errorf("--app chỉ được đặt một lần\n%s", GrantUsage)
			}
			appSet = true
			cmd.App = identity.App(value)
		case strings.HasPrefix(token, "--app="):
			if appSet {
				return cmd, fmt.Errorf("--app chỉ được đặt một lần\n%s", GrantUsage)
			}
			appSet = true
			cmd.App = identity.App(strings.TrimPrefix(token, "--app="))
		case strings.HasPrefix(token, "-"):
			return cmd, fmt.Errorf("cờ không hợp lệ %q (chỉ chấp nhận --app)\n%s", token, GrantUsage)
		case cmd.Username == "":
			cmd.Username = token
		default:
			return cmd, fmt.Errorf("tham số thừa %q\n%s", token, GrantUsage)
		}
	}
	if cmd.Username == "" {
		return cmd, fmt.Errorf("thiếu tên đăng nhập\n%s", GrantUsage)
	}
	if !identity.ValidUsername(cmd.Username) {
		return cmd, fmt.Errorf("tên đăng nhập %q không hợp lệ", cmd.Username)
	}
	if !appSet {
		return cmd, fmt.Errorf("thiếu --app noverse|viderse\n%s", GrantUsage)
	}
	if !cmd.App.Valid() {
		return cmd, fmt.Errorf("--app=%q không hợp lệ (noverse|viderse)", cmd.App)
	}
	return cmd, nil
}

// RunGrant executes one parsed command through the trusted operator entry of
// the shared account store: audit actor stays system and granted_by NULL —
// the operator is trusted precisely because they already hold the DSN.
// A missing user, an unsupported backend or a rejected role all surface as
// the repository error verbatim so the operator sees the real cause.
func RunGrant(store *identity.Store, cmd GrantCommand) error {
	if store == nil {
		return fmt.Errorf("thiếu kho tài khoản (cần backend Postgres)")
	}
	switch cmd.Verb {
	case GrantVerbGrant:
		return store.OperatorSetAppGrant(cmd.Username, cmd.App, identity.RoleMember)
	case GrantVerbRevoke:
		return store.OperatorRevokeAppGrant(cmd.Username, cmd.App)
	default:
		return fmt.Errorf("lệnh không hợp lệ %q", cmd.Verb)
	}
}
