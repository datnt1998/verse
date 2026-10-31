-- Per-app grants. One explicit row per (user, app); global admins inherit both
-- apps without a row, so the effective map is computed in Go from the user row
-- plus these rows. The repository never caches the map in a session.

-- name: ListAppGrants :many
SELECT app, role FROM app_grants WHERE user_id = $1 ORDER BY app;

-- name: GetAppGrant :one
SELECT app, role, granted_by, granted_at FROM app_grants
WHERE user_id = $1 AND app = $2;

-- name: InsertAppGrant :exec
INSERT INTO app_grants (user_id, app, role, granted_by, granted_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateAppGrant :exec
UPDATE app_grants SET role = $3, granted_by = $4, granted_at = $5
WHERE user_id = $1 AND app = $2;

-- name: DeleteAppGrant :exec
DELETE FROM app_grants WHERE user_id = $1 AND app = $2;
