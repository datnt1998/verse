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
-- itself must go through ClaimJob). The needsScript filter mirrors ClaimJob so
-- the debug view never promises work the remote worker cannot take.
SELECT * FROM jobs
WHERE kind = $1 AND state IN ('queued', 'running')
  AND (state = 'queued' OR lease_until IS NULL OR lease_until < now())
  AND (input->>'needsScript') IS DISTINCT FROM 'true'
ORDER BY created_at, id;

-- name: LockChapterJobs :many
-- Every tts row of one chapter, locked so enqueue can keep the "one active
-- job per chapter" invariant when a Generate click races another one: the
-- caller rewrites the stale row (or inserts a new one) and cancels the rest in
-- the same transaction. The chapterId lives in the frozen input, so this is
-- exact for the chapter, not for its number.
SELECT * FROM jobs
WHERE project_id = $1 AND story_id = $2 AND kind = 'tts'
  AND input->>'chapterId' = sqlc.arg(chapter_id)::text
ORDER BY created_at, id
FOR UPDATE;

-- name: LockJob :one
SELECT * FROM jobs WHERE project_id = $1 AND id = $2 FOR UPDATE;

-- ── claim / lease ────────────────────────────────────────────────────────────

-- name: ClaimJob :one
-- Preparation rows (`input.needsScript:true`) are owned by the web compose
-- runner, never by the remote worker: the exclusion is a plain text compare so
-- a malformed/absent flag can never throw while a worker is claiming.
WITH candidate AS (
    SELECT j.project_id, j.id
    FROM jobs j
    WHERE j.kind = $1
      AND j.state IN ('queued', 'running')
      AND (j.state = 'queued' OR j.lease_until IS NULL OR j.lease_until < now())
      AND (j.input->>'needsScript') IS DISTINCT FROM 'true'
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

-- ── hosted preparation (tts kind + input.needsScript) ────────────────────────
--
-- A missing bản đọc is queued as an ordinary kind='tts' row whose input carries
-- `needsScript:true` (no new kind: the jobs CHECK is closed). The web compose
-- runner owns that row until the script is locked; ClaimJob never sees it.
--
-- The runner's claim holds a lease on the same row **without** a worker_id: the
-- state stays 'queued' (the tts CHECK only requires a worker when a row is
-- 'running'), and `lease_until` is what serializes two compose attempts. Every
-- publish (fail/release/complete) must present the exact fence while that lease
-- is live, so cancel/pause/retry — which bump the fence — beat a compose that
-- finishes after the user changed their mind.

-- name: ListPreparationTargets :many
-- Trusted internal scan (startup/recovery): every active story with TTS work
-- still waiting for a bản đọc. Deleted projects/stories and un-promoted drafts
-- are never resurrected by a restart.
SELECT DISTINCT j.project_id, j.story_id,
    p.slug AS project_slug, s.slug AS story_slug
FROM jobs j
JOIN projects p ON p.id = j.project_id AND p.deleted_at IS NULL
JOIN stories s ON s.project_id = j.project_id AND s.id = j.story_id
    AND s.deleted_at IS NULL AND s.status <> 'draft'
WHERE j.kind = 'tts'
  AND j.state IN ('queued', 'running')
  AND COALESCE(j.input->>'needsScript', 'false') = 'true'
ORDER BY p.slug, s.slug;

-- name: ClaimPreparation :one
-- Oldest preparation of this exact story: queued with no live lease, or a
-- legacy running row whose lease already expired. A live lease is untouched, so
-- two runners cannot compose the same chapter twice.
WITH candidate AS (
    SELECT j.project_id, j.id
    FROM jobs j
    WHERE j.project_id = $1 AND j.story_id = $2
      AND j.kind = 'tts'
      AND j.worker_id IS NULL
      AND j.state IN ('queued', 'running')
      AND (j.lease_until IS NULL OR j.lease_until <= now())
      AND COALESCE(j.input->>'needsScript', 'false') = 'true'
    ORDER BY j.created_at, j.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE jobs
SET state = 'queued',
    worker_id = NULL,
    lease_until = now() + make_interval(secs => sqlc.arg(ttl)::int),
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
FROM candidate
WHERE jobs.project_id = candidate.project_id AND jobs.id = candidate.id
RETURNING jobs.*;

-- name: LockPreparation :one
-- Locks the preparation row and validates the caller's exact live lease in the
-- same statement, before any script write happens inside the transaction.
SELECT * FROM jobs
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND worker_id IS NULL
  AND state IN ('queued', 'running')
  AND fencing_token = $4
  AND lease_until IS NOT NULL AND lease_until > now()
  AND COALESCE(input->>'needsScript', 'false') = 'true'
FOR UPDATE;

-- name: FailPreparation :one
-- Terminal compose error. The row keeps needsScript so Retry re-enters
-- preparation instead of handing a scriptless chapter to the worker.
UPDATE jobs
SET state = 'failed',
    error = $5,
    finished_at = now(),
    lease_until = NULL,
    worker_id = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND worker_id IS NULL
  AND state IN ('queued', 'running')
  AND fencing_token = $4
  AND lease_until IS NOT NULL AND lease_until > now()
  AND COALESCE(input->>'needsScript', 'false') = 'true'
RETURNING *;

-- name: ReleasePreparation :one
-- Graceful stop: the same row becomes claimable by the next runner pass.
UPDATE jobs
SET state = 'queued',
    lease_until = NULL,
    worker_id = NULL,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND worker_id IS NULL
  AND state IN ('queued', 'running')
  AND fencing_token = $4
  AND lease_until IS NOT NULL AND lease_until > now()
  AND COALESCE(input->>'needsScript', 'false') = 'true'
RETURNING *;

-- name: CompletePreparation :one
-- Same row, now real TTS work: the composed input replaces the seed, the flag
-- is stripped (so ClaimJob can take it), and the lease/fence move so any late
-- compose output from the previous attempt is rejected. Callers must have locked
-- the row through LockPreparation in this transaction.
UPDATE jobs
SET input = sqlc.arg(input)::jsonb - 'needsScript',
    progress_total = sqlc.arg(progress_total)::int,
    progress_done = 0,
    state = 'queued',
    worker_id = NULL,
    lease_until = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND worker_id IS NULL
  AND state IN ('queued', 'running')
  AND fencing_token = $4
  AND COALESCE(input->>'needsScript', 'false') = 'true'
RETURNING *;

-- name: CompletePreparationEmpty :one
-- A prepared script can legitimately need no synthesis: every segment still has
-- a selected render valid for the current parameters (chapter text changed only
-- in ways the spoken text did not). Queuing it would hand the worker a
-- zero-segment job, which it hard-fails forever, so the honest outcome is the
-- file backend's "already voiced": the request completes on the same row.
UPDATE jobs
SET input = sqlc.arg(input)::jsonb - 'needsScript',
    progress_total = 0,
    progress_done = 0,
    state = 'completed',
    finished_at = now(),
    worker_id = NULL,
    lease_until = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND worker_id IS NULL
  AND state IN ('queued', 'running')
  AND fencing_token = $4
  AND COALESCE(input->>'needsScript', 'false') = 'true'
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

-- name: SupersedeChapterJob :one
-- A stale active row (queued render from before the chapter changed, a paused
-- or running one) is rewritten in place with the fresh frozen work and put
-- back in the queue under a bumped fence. A worker or preparation runner still
-- holding the old fence can no longer publish, and the chapter never ends up
-- with two active rows.
UPDATE jobs
SET input = sqlc.arg(input)::jsonb,
    progress_total = sqlc.arg(progress_total)::int,
    progress_done = 0,
    result = NULL,
    error = NULL,
    state = 'queued',
    worker_id = NULL,
    lease_until = NULL,
    started_at = NULL,
    finished_at = NULL,
    cancelled_at = NULL,
    fencing_token = fencing_token + 1,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND story_id = $2 AND id = $3
  AND kind = 'tts'
  AND state IN ('queued', 'running', 'paused')
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
-- needsScript rows belong to the web compose runner, not to the reader
-- machine, so they are not chapters lost while that machine is offline.
SELECT count(*)::int
FROM jobs
WHERE kind = $1 AND state = 'queued'
  AND (input->>'needsScript') IS DISTINCT FROM 'true';
