package identity

import "fmt"

// App names an application that shares the one account store. The values are
// the storage contract (app_grants.app check constraint), not display labels.
type App string

const (
	AppNoverse App = "noverse"
	AppViderse App = "viderse"
)

// Valid reports whether the app is a known storage value.
func (a App) Valid() bool { return a == AppNoverse || a == AppViderse }

// AppGrant is one explicit per-app grant row. Only RoleMember is stored today;
// platform admins inherit both apps without a row.
type AppGrant struct {
	App  App  `json:"app"`
	Role Role `json:"role"`
}

// AppGranter is the optional backend capability for per-app grants. File mode
// has no grant rows: its Noverse-only store already conveys effective Noverse
// access, and cross-app grants require the Postgres backend.
type AppGranter interface {
	AppGrants(username string) (map[App]Role, error)
	SetAppGrant(username string, app App, role Role, actor *AuditActor, reason string) error
	RevokeAppGrant(username string, app App, actor *AuditActor, reason string) error
	// OperatorSetAppGrant/OperatorRevokeAppGrant are the trusted local-operator
	// entry point (CLI): no actor user, granted_by stays NULL.
	OperatorSetAppGrant(username string, app App, role Role) error
	OperatorRevokeAppGrant(username string, app App) error
}

// ErrUnknownApp reports a grant request naming an app outside the contract.
var ErrUnknownApp = fmt.Errorf("%w: ứng dụng không hợp lệ", ErrInvalidInput)

func granterOf(repo Repository) (AppGranter, error) {
	g, ok := repo.(AppGranter)
	if !ok {
		return nil, fmt.Errorf("%w: quyền ứng dụng chỉ có trên backend Postgres", ErrUnsupported)
	}
	return g, nil
}

// AppGrants returns the stored grant rows of an account on backends with a
// grant table; the row-less file backend reports its effective Noverse access
// instead (there is nothing to distinguish inherited from explicit there).
func (s *Store) AppGrants(username string) (map[App]Role, error) {
	if s.repo != nil {
		g, err := granterOf(s.repo)
		if err != nil {
			return nil, err
		}
		return g.AppGrants(username)
	}
	d, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	u := d.user(username)
	if u == nil {
		return nil, ErrNotFound
	}
	return fileGrants(u), nil
}

// SetAppGrant grants app to username on behalf of actor. The backend verifies
// the actor is a global admin inside the mutation transaction.
func (s *Store) SetAppGrant(username string, app App, role Role, actor *AuditActor, reason string) error {
	if s.repo != nil {
		g, err := granterOf(s.repo)
		if err != nil {
			return err
		}
		return g.SetAppGrant(username, app, role, actor, reason)
	}
	return fmt.Errorf("%w: quyền ứng dụng chỉ có trên backend Postgres", ErrUnsupported)
}

// RevokeAppGrant removes an explicit grant row on behalf of a global admin.
func (s *Store) RevokeAppGrant(username string, app App, actor *AuditActor, reason string) error {
	if s.repo != nil {
		g, err := granterOf(s.repo)
		if err != nil {
			return err
		}
		return g.RevokeAppGrant(username, app, actor, reason)
	}
	return fmt.Errorf("%w: quyền ứng dụng chỉ có trên backend Postgres", ErrUnsupported)
}

// OperatorSetAppGrant is the trusted server-operator path used by the CLI.
func (s *Store) OperatorSetAppGrant(username string, app App, role Role) error {
	if s.repo != nil {
		g, err := granterOf(s.repo)
		if err != nil {
			return err
		}
		return g.OperatorSetAppGrant(username, app, role)
	}
	return fmt.Errorf("%w: quyền ứng dụng chỉ có trên backend Postgres", ErrUnsupported)
}

// OperatorRevokeAppGrant is the trusted server-operator removal path used by
// the CLI.
func (s *Store) OperatorRevokeAppGrant(username string, app App) error {
	if s.repo != nil {
		g, err := granterOf(s.repo)
		if err != nil {
			return err
		}
		return g.OperatorRevokeAppGrant(username, app)
	}
	return fmt.Errorf("%w: quyền ứng dụng chỉ có trên backend Postgres", ErrUnsupported)
}

// fileGrants is the effective grant map of a file-mode account: Noverse only,
// admin inherited from the global role. Kept next to the file store's View so
// request-scoped copies and stored data never alias.
func fileGrants(u *User) map[App]Role {
	out := map[App]Role{AppNoverse: RoleMember}
	if u.Role == RoleAdmin {
		out[AppNoverse] = RoleAdmin
		out[AppViderse] = RoleAdmin
	}
	return out
}

// EffectiveGrants folds explicit grant rows and the global role into the
// per-app view: a global admin is admin of both apps without rows; members
// need an explicit row.
func EffectiveGrants(global Role, explicit map[App]Role) map[App]Role {
	out := make(map[App]Role, len(explicit)+2)
	if global == RoleAdmin {
		out[AppNoverse] = RoleAdmin
		out[AppViderse] = RoleAdmin
		return out
	}
	for app, role := range explicit {
		out[app] = role
	}
	return out
}

// cloneGrants returns an independent copy so a request cannot mutate the map
// held by the store or by another caller.
func cloneGrants(in map[App]Role) map[App]Role {
	out := make(map[App]Role, len(in))
	for app, role := range in {
		out[app] = role
	}
	return out
}
