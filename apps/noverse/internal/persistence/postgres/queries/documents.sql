-- Document-backend queries (phase 05): typed rows for the engine facts the
-- file backend keeps as JSON/JSONL artefacts. Every statement stays inside a
-- project+story scope; append-only tables are only deleted through the
-- documented maintenance escape (`noverse.allow_immutable_write`) handled by
-- the repository, never by a handler.

-- ── artifacts (plans, summaries, chapter text, markdown views) ───────────────

-- name: GetStoryArtifactByKey :one
SELECT * FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3 AND artifact_key = $4;

-- name: UpsertStoryArtifactByKey :exec
INSERT INTO story_artifacts (project_id, story_id, kind, artifact_key,
    payload, text_content, schema_version)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (project_id, story_id, kind, artifact_key)
DO UPDATE SET payload = EXCLUDED.payload,
    text_content = EXCLUDED.text_content,
    schema_version = EXCLUDED.schema_version,
    updated_at = clock_timestamp(), version = story_artifacts.version + 1;

-- name: DeleteStoryArtifact :execrows
DELETE FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3 AND artifact_key = $4;

-- name: DeleteStoryArtifactsByKind :exec
DELETE FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3;

-- name: ListStoryArtifactKeys :many
SELECT artifact_key FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3
  AND artifact_key LIKE $4 || '%'
ORDER BY artifact_key;

-- name: ListStoryArtifactsByKind :many
SELECT * FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3
ORDER BY artifact_key;

-- name: ListStoryArtifactKinds :many
SELECT DISTINCT kind FROM story_artifacts
WHERE project_id = $1 AND story_id = $2;

-- ── typed document streams (story_events keyed by log_key) ───────────────────

-- name: AppendStoryEventLog :one
INSERT INTO story_events (project_id, story_id, stream, log_key, seq, payload)
VALUES ($1, $2, $3, $4,
    (SELECT coalesce(max(seq), 0) + 1 FROM story_events
        WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND log_key = $4),
    $5)
RETURNING seq;

-- name: InsertStoryEventExact :exec
INSERT INTO story_events (project_id, story_id, stream, log_key, seq, payload)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListStoryEventsPage :many
SELECT * FROM story_events
WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND log_key = $4
  AND seq > $5
ORDER BY seq
LIMIT $6;

-- name: DeleteStoryEventsLog :exec
DELETE FROM story_events
WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND log_key = $4;

-- name: DeleteStoryEventsFromSeq :exec
DELETE FROM story_events
WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND log_key = $4
  AND seq >= $5;

-- ── checkpoints: exact-sequence writes for document parity ───────────────────

-- name: InsertCheckpointExact :exec
INSERT INTO checkpoints (project_id, story_id, seq, scope_key, step,
    artifact_ref, sha256, occurred_at, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: DeleteCheckpoints :exec
DELETE FROM checkpoints WHERE project_id = $1 AND story_id = $2;

-- ── agent sessions: exact-sequence writes + directory listing ────────────────

-- name: InsertAgentSessionExact :exec
INSERT INTO agent_sessions (project_id, story_id, agent, task_key, seq, payload)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteAgentSession :exec
DELETE FROM agent_sessions
WHERE project_id = $1 AND story_id = $2 AND agent = $3 AND task_key = $4;

-- name: DeleteAgentSessionsFromSeq :exec
DELETE FROM agent_sessions
WHERE project_id = $1 AND story_id = $2 AND agent = $3 AND task_key = $4
  AND seq >= $5;

-- name: ListAgentSessionKeys :many
SELECT DISTINCT agent, task_key FROM agent_sessions
WHERE project_id = $1 AND story_id = $2
ORDER BY agent, task_key;

-- ── reviews: invalidation on rewrite + latest-per-scope reads ────────────────

-- name: InvalidateReviewsFromChapter :exec
UPDATE reviews r
SET invalidated_at = now(), updated_at = clock_timestamp(),
    version = r.version + 1
FROM chapters c
WHERE r.project_id = $1 AND r.story_id = $2
  AND c.project_id = r.project_id AND c.id = r.chapter_id
  AND c.number >= $3 AND r.invalidated_at IS NULL;

-- name: DeleteReviewDecisionsOfInvalidated :exec
DELETE FROM review_decisions rd
USING reviews r
WHERE rd.project_id = $1 AND rd.review_id = r.id
  AND r.project_id = $1 AND r.story_id = $2 AND r.invalidated_at IS NOT NULL;

-- name: ListLatestReviews :many
SELECT DISTINCT ON (r.scope_key) r.*, c.number AS chapter_number
FROM reviews r
LEFT JOIN chapters c ON c.project_id = r.project_id AND c.id = r.chapter_id
WHERE r.project_id = $1 AND r.story_id = $2 AND r.invalidated_at IS NULL
ORDER BY r.scope_key, r.round DESC;

-- name: LatestGlobalReviewAtOrBefore :one
SELECT r.* FROM reviews r
JOIN chapters c ON c.project_id = r.project_id AND c.id = r.chapter_id
WHERE r.project_id = $1 AND r.story_id = $2
  AND r.scope_key LIKE '%-global' AND r.invalidated_at IS NULL
  AND c.number <= $3
ORDER BY c.number DESC, r.round DESC
LIMIT 1;

-- name: HasReviewRoundsForScope :one
SELECT count(*) FROM reviews
WHERE project_id = $1 AND story_id = $2 AND scope_key = $3;

-- name: ListStoryEventKeys :many
SELECT DISTINCT log_key FROM story_events
WHERE project_id = $1 AND story_id = $2 AND stream = $3
ORDER BY log_key;

-- name: DeleteCheckpointsFromSeq :exec
DELETE FROM checkpoints
WHERE project_id = $1 AND story_id = $2 AND seq >= $3;
