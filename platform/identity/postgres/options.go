package postgres

import "time"

// Options are the immutable repository parameters. They never change after
// NewAccounts: the same instance serves every request, so per-request state
// (current user, app, grant) must stay in arguments — never in this struct.
type Options struct {
	// ProjectAccess enables the project/member surface of the Noverse app.
	// False (Viderse) makes the user projection skip project queries entirely
	// and rejects project, global-role, user-lifecycle and audit operations
	// before any SQL runs.
	ProjectAccess bool
	// ValidProjectID validates membership project IDs; nil accepts every ID.
	ValidProjectID func(string) bool
	// Now overrides the clock; nil means time.Now (tests use it for TTLs).
	Now func() time.Time
}
