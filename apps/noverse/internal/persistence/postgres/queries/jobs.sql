-- Durable jobs, workers and job events (phase 07).
--
-- Claim contract: one short transaction takes `FOR UPDATE SKIP LOCKED`, bumps
-- fencing_token and sets a bounded lease using server time. Every publish path
-- (heartbeat/complete/fail/release) matches (worker_id, fencing_token, state
-- 'running', lease_until > now()), so a stale worker's late ACK updates zero
-- rows instead of overwriting the new owner's work. Cancel/pause bump the
-- fence, which also invalidates an in-flight worker even before its lease
-- expires.
--
-- Idempotency: a job is unique per (project, story, kind, idempotency_key);
-- enqueue resolves the existing row instead of creating a duplicate job.

-- ── workers ──────────────────────────────────────────────────────────────────

-- name: InsertWorker :one
INSERT INTO workers (id, name, token_hash, capabilities, last_seen_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWorkerByTokenHash :one
SELECT * FROM workers WHERE token_hash = $1;

-- name: GetWorker :one
SELECT * FROM workers WHERE id = $1;

-- name: TouchWorker :execrows
UPDATE workers
SET last_seen_at = $2, updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeWorker :execrows
UPDATE workers
SET revoked_at = COALESCE(revoked_at, clock_timestamp()),
    updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND revoked_at IS NULL;

-- name: ListWorkers :many
SELECT * FROM workers ORDER BY created_at;

-- name: SetWorkerState :execrows
-- The reader publishes what it is doing (voice.ProvisionState as JSON). Liveness
-- is TouchWorker's job; this only records content and its own timestamp, so a
-- revoked credential stops publishing at once.
UPDATE workers
SET state = $2, state_updated_at = clock_timestamp(),
    updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND revoked_at IS NULL;

-- name: BumpWorkerCommand :one
-- One operator click = one command: the seq is monotonic so a worker that keeps
-- polling the same row never runs the same command twice.
UPDATE workers
SET command_seq = command_seq + 1, command = $2,
    updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- ── enqueue / read ───────────────────────────────────────────────────────────

-- name: InsertJob :one
-- ON CONFLICT DO NOTHING (instead of an error) keeps the transaction alive
-- when the same work is enqueued twice; the caller then reads the existing row
-- by its idempotency key.
INSERT INTO jobs (project_id, story_id, id, kind, state, idempotency_key, input, progress_total, requested_by)
VALUES ($1, $2, $3, $4, 'queued', $5, $6, $7, $8)
ON CONFLICT (project_id, story_id, kind, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetJobByIdempotency :one
SELECT * FROM jobs
WHERE project_id = $1 AND story_id = $2 AND kind = $3 AND idempotency_key = $4;

-- name: GetJob :one
SELECT * FROM jobs WHERE project_id = $1 AND id = $2;

-- name: GetStoryJob :one
SELECT * FROM jobs WHERE project_id = $1 AND story_id = $2 AND id = $3;

-- name: GetWorkerJob :one
-- Resolves a job by its id for the worker that currently holds it. The worker
-- never supplies (or learns) a project id for this lookup; the row's own
-- project belongs to the claim it was granted.
SELECT * FROM jobs WHERE id = $1 AND worker_id = $2;

-- name: ListStoryJobs :many
SELECT * FROM jobs
WHERE project_id = $1 AND story_id = $2
ORDER BY created_at, id;

-- name: ListClaimableJobs :many
-- Read-only view of what a claim would consider (operators/debug; the claim
-- itself must go through ClaimJob).
SELECT * FROM jobs
WHERE kind = $1 AND state IN ('queued', 'running')
  AND (state = 'queued' OR lease_until IS NULL OR lease_until < now())
ORDER BY created_at, id;

-- name: LockJob :one
SELECT * FROM jobs WHERE project_id = $1 AND id = $2 FOR UPDATE;

-- ── claim / lease ────────────────────────────────────────────────────────────

-- name: ClaimJob :one
WITH candidate AS (
    SELECT j.project_id, j.id
    FROM jobs j
    WHERE j.kind = $1
      AND j.state IN ('queued', 'running')
      AND (j.state = 'queued' OR j.lease_until IS NULL OR j.lease_until < now())
      AND ($2::uuid IS NULL OR j.project_id = $2::uuid)
    ORDER BY j.created_at, j.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE jobs
SET state = 'running',
    worker_id = $3,
    lease_until = now() + make_interval(secs => $4::int),
    fencing_token = fencing_token + 1,
    started_at = COALESCE(started_at, now()),
    cancelled_at = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
FROM candidate
WHERE jobs.project_id = candidate.project_id AND jobs.id = candidate.id
RETURNING jobs.*;

-- name: HeartbeatJob :one
UPDATE jobs
SET lease_until = now() + make_interval(secs => $5::int),
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2
  AND worker_id = $3 AND fencing_token = $4
  AND state = 'running' AND lease_until > now()
RETURNING *;

-- name: CompleteJob :one
-- A duplicate ACK matches no row (the job is already terminal), which the
-- repository turns into ErrJobLeaseLost; the HTTP layer answers a replay with
-- the stored result, so a retried ACK never appends a second event or blob.
UPDATE jobs
SET state = 'completed',
    result = $5,
    progress_done = $6,
    progress_total = COALESCE(sqlc.narg(progress_total)::int, progress_total),
    finished_at = now(),
    lease_until = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2
  AND worker_id = $3 AND fencing_token = $4
  AND state = 'running' AND lease_until > now()
RETURNING *;

-- name: FailJob :one
UPDATE jobs
SET state = 'failed',
    error = $5,
    progress_total = COALESCE(sqlc.narg(progress_total)::int, progress_total),
    finished_at = now(),
    lease_until = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2
  AND worker_id = $3 AND fencing_token = $4
  AND state = 'running' AND lease_until > now()
RETURNING *;

-- name: ReleaseJob :one
-- Graceful worker stop: hand the job back without losing progress.
UPDATE jobs
SET state = 'queued',
    worker_id = NULL,
    lease_until = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2
  AND worker_id = $3 AND fencing_token = $4
  AND state = 'running' AND lease_until > now()
RETURNING *;

-- ── control ──────────────────────────────────────────────────────────────────

-- name: CancelJob :one
UPDATE jobs
SET state = 'cancelled',
    cancelled_at = now(),
    finished_at = now(),
    worker_id = NULL,
    lease_until = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state NOT IN ('completed', 'cancelled')
RETURNING *;

-- name: PauseJob :one
UPDATE jobs
SET state = 'paused',
    worker_id = NULL,
    lease_until = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state IN ('queued', 'running', 'waiting')
RETURNING *;

-- name: ResumeJob :one
UPDATE jobs
SET state = 'queued',
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state = 'paused'
RETURNING *;

-- name: RetryJob :one
-- Re-run a finished/failed job. Input may be replaced by the caller (retry
-- options such as "Ghép không nhạc"); the fence is bumped so any late output
-- from the previous attempt is rejected.
UPDATE jobs
SET state = 'queued',
    input = COALESCE($3::jsonb, input),
    result = NULL,
    error = NULL,
    progress_done = 0,
    worker_id = NULL,
    lease_until = NULL,
    cancelled_at = NULL,
    started_at = NULL,
    finished_at = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state IN ('failed', 'cancelled', 'completed')
RETURNING *;

-- name: UpdateJobProgress :one
UPDATE jobs
SET progress_done = $3,
    progress_total = COALESCE(sqlc.narg(progress_total)::int, progress_total),
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state IN ('queued', 'running', 'waiting')
RETURNING *;

-- name: RequeueExpiredJobs :execrows
-- Boot-time recovery: a process that died mid-job leaves a running row whose
-- lease is gone. Normalising it back to queued loses no work and makes it
-- claimable again; a live worker keeps its lease and is untouched.
UPDATE jobs
SET state = 'queued',
    worker_id = NULL,
    lease_until = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE state = 'running' AND (lease_until IS NULL OR lease_until < now());

-- ── events ───────────────────────────────────────────────────────────────────

-- name: NextJobEventSeq :one
SELECT (COALESCE(MAX(seq), 0) + 1)::bigint AS seq
FROM job_events WHERE project_id = $1 AND job_id = $2;

-- name: InsertJobEvent :one
INSERT INTO job_events (project_id, job_id, seq, event_type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListJobEvents :many
SELECT * FROM job_events
WHERE project_id = $1 AND job_id = $2 AND seq > $3
ORDER BY seq;

-- name: CountQueuedJobs :one
-- Số việc của một loại đang xếp hàng trên toàn máy chủ (không lọc project):
-- GET /api/tts/status dùng nó cho `queueWaiting` khi máy đọc chưa sẵn sàng.
SELECT count(*)::int
FROM jobs
WHERE kind = $1 AND state = 'queued';
