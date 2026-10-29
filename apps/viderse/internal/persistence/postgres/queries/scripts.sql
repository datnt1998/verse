-- name: CreateRun :one
INSERT INTO agent_runs (id, status, provider, model, brief)
VALUES ($1, 'queued', $2, $3, $4)
RETURNING *;

-- name: GetRun :one
SELECT * FROM agent_runs WHERE id = $1;

-- name: ListRuns :many
SELECT * FROM agent_runs ORDER BY created_at DESC LIMIT $1;

-- name: MarkRunRunning :one
UPDATE agent_runs
SET status = 'running', started_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

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
