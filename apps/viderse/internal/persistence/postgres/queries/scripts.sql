-- name: CreateRun :one
INSERT INTO agent_runs (id, status, provider, model, brief)
VALUES ($1, 'queued', $2, $3, $4)
RETURNING *;

-- name: GetRun :one
SELECT * FROM agent_runs WHERE id = $1;

-- name: ListRuns :many
SELECT * FROM agent_runs ORDER BY created_at DESC LIMIT $1;

-- name: ClaimNextRun :one
-- The database is the run queue: oldest queued run becomes running,
-- SKIP LOCKED so concurrent dispatchers never take the same row.
UPDATE agent_runs
SET status = 'running', started_at = now()
WHERE id = (
    SELECT id FROM agent_runs
    WHERE status = 'queued'
    ORDER BY created_at, id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: FailInterruptedRuns :execrows
-- Runs still 'running' when a dispatcher starts belong to a dead process.
UPDATE agent_runs
SET status = 'failed', error = $1, finished_at = now()
WHERE status = 'running';

-- name: FinishRun :one
UPDATE agent_runs
SET status = $2,
    input_tokens = $3,
    output_tokens = $4,
    cost_usd = $5,
    error = $6,
    finished_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateScript :one
INSERT INTO scripts (id, run_id, title, sections, full_text, word_count, est_duration_sec)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetScript :one
SELECT * FROM scripts WHERE id = $1;

-- name: ListScripts :many
SELECT * FROM scripts ORDER BY created_at DESC LIMIT $1;
