-- Read-only account lookups used by the Noverse business repositories to
-- resolve audit authors and membership actors. The full account/session
-- repository lives in platform/identity/postgres; this file deliberately holds
-- only the projections non-identity code still needs.

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;
