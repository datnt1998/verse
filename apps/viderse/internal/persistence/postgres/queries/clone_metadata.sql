-- name: SaveCloneMetadataState :exec
INSERT INTO clone_metadata_states (job_id, state, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (job_id) DO UPDATE SET state = EXCLUDED.state, updated_at = EXCLUDED.updated_at;

-- name: GetCloneMetadataState :one
SELECT state FROM clone_metadata_states WHERE job_id = $1;
