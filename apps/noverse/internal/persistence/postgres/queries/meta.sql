-- noverse_meta is the schema bookkeeping table created by 00001_foundation.
-- Repositories in later phases add their own queries/*.sql per domain;
-- nothing queries raw SQL strings outside this directory.

-- name: GetMetaValue :one
SELECT value FROM noverse_meta WHERE key = $1;

-- name: UpsertMetaValue :exec
INSERT INTO noverse_meta (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
