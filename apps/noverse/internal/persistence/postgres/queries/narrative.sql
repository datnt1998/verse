-- Narrative domain queries (phase 05): stories, volumes, chapters with
-- immutable versions, outlines, characters, comments/replies, reviews,
-- story_state singletons, story_events/checkpoints/agent_sessions append
-- streams, story_leases and project-scoped search.
--
-- Every read/write carries the project predicate; composite FKs make
-- cross-project links impossible at the row level and repositories carry the
-- membership/actor scope in the same transaction.

-- ── scope resolution ─────────────────────────────────────────────────────────

-- name: GetMemberRole :one
SELECT role FROM project_members
WHERE project_id = $1 AND user_id = $2 AND removed_at IS NULL;

-- name: LockProjectRow :one
SELECT id FROM projects WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: GetStoryBySlug :one
SELECT * FROM stories
WHERE project_id = $1 AND slug = $2 AND deleted_at IS NULL AND status <> 'draft';

-- name: GetStoryByID :one
SELECT * FROM stories
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: GetDraftBySlug :one
SELECT * FROM stories
WHERE project_id = $1 AND slug = $2 AND deleted_at IS NULL AND status = 'draft';

-- name: LockDraftBySlug :one
SELECT * FROM stories
WHERE project_id = $1 AND slug = $2 AND deleted_at IS NULL AND status = 'draft'
FOR NO KEY UPDATE;

-- name: PromoteDraft :one
UPDATE stories SET slug = $3, title = $4, status = 'idea',
    updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND status = 'draft' AND deleted_at IS NULL
RETURNING *;

-- name: LockStoryByID :one
SELECT id, slug, title, short_title, synopsis, title_renamed, status, tags,
    published_at, idea_document_id, version
FROM stories
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL
FOR UPDATE;

-- ── stories ──────────────────────────────────────────────────────────────────

-- name: InsertStory :one
INSERT INTO stories (project_id, id, slug, title, short_title, synopsis,
    title_renamed, status, tags, published_at, idea_document_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListStoriesByProject :many
SELECT * FROM stories
WHERE project_id = $1 AND deleted_at IS NULL AND status <> 'draft'
ORDER BY slug;

-- name: UpdateStoryCAS :one
UPDATE stories
SET slug = $3, title = $4, short_title = $5, synopsis = $6,
    title_renamed = $7, status = $8, tags = $9, published_at = $10,
    idea_document_id = $11,
    updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL
  AND version = $12
RETURNING *;

-- name: SoftDeleteStory :execrows
UPDATE stories
SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: StoryIdeaText :one
SELECT text_content FROM project_documents
WHERE project_id = $1 AND id = $2;

-- name: UpsertProjectDocument :one
INSERT INTO project_documents (project_id, id, kind, logical_name, text_content)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, kind, logical_name)
DO UPDATE SET text_content = EXCLUDED.text_content,
    updated_at = clock_timestamp(), version = project_documents.version + 1
RETURNING id;


-- name: GetProjectDocumentByID :one
SELECT * FROM project_documents
WHERE project_id = $1 AND id = $2;

-- name: GetProjectDocumentByName :one
SELECT * FROM project_documents
WHERE project_id = $1 AND kind = $2 AND logical_name = $3;

-- name: ListProjectDocumentsByKind :many
SELECT * FROM project_documents
WHERE project_id = $1 AND kind = $2
ORDER BY logical_name;

-- name: UpsertProjectBlobDocument :one
-- Binary project documents (asset uploads): the row carries payload metadata
-- and a blobs FK; the bytes live on the BlobStore object key.
INSERT INTO project_documents (project_id, id, kind, logical_name, payload, blob_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (project_id, kind, logical_name)
DO UPDATE SET payload = EXCLUDED.payload, blob_id = EXCLUDED.blob_id,
    updated_at = clock_timestamp(), version = project_documents.version + 1
RETURNING *;

-- name: DeleteProjectDocumentByName :execrows
DELETE FROM project_documents
WHERE project_id = $1 AND kind = $2 AND logical_name = $3;

-- name: ListStoryIdeaRefs :many
SELECT s.slug, d.logical_name AS idea_name
FROM stories s
JOIN project_documents d ON d.project_id = s.project_id
  AND d.id = s.idea_document_id
WHERE s.project_id = $1 AND s.deleted_at IS NULL AND s.status <> 'draft'
ORDER BY s.slug;

-- ── volumes / chapters ───────────────────────────────────────────────────────

-- name: UpsertVolume :exec
INSERT INTO volumes (project_id, story_id, id, number, title, outline)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (project_id, story_id, number)
DO UPDATE SET title = EXCLUDED.title, outline = EXCLUDED.outline,
    updated_at = clock_timestamp(), version = volumes.version + 1;

-- name: ListVolumes :many
SELECT * FROM volumes
WHERE project_id = $1 AND story_id = $2
ORDER BY number;

-- name: GetVolumeByNumber :one
SELECT * FROM volumes
WHERE project_id = $1 AND story_id = $2 AND number = $3;

-- name: GetVolumeByID :one
SELECT * FROM volumes
WHERE project_id = $1 AND id = $2;

-- name: DeleteVolumesForStory :exec
DELETE FROM volumes
WHERE project_id = $1 AND story_id = $2;

-- name: EnsureChapter :one
-- Insert-or-fetch a LIVE chapter row. DO NOTHING avoids a no-op UPDATE
-- (which the row version guard rightly rejects); the second SELECT is
-- restricted to live rows so a soft-deleted chapter number is never handed
-- back as if it were writable. A tombstone keeps its uniqueness (no
-- resurrection/reuse), and the repository turns the empty result into an
-- explicit conflict.
WITH ins AS (
    INSERT INTO chapters (project_id, story_id, id, number)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (project_id, story_id, number) DO NOTHING
    RETURNING *
)
SELECT * FROM ins
UNION ALL
SELECT * FROM chapters
WHERE project_id = $1 AND story_id = $2 AND number = $4 AND deleted_at IS NULL
LIMIT 1;

-- name: GetChapterByNumber :one
SELECT * FROM chapters
WHERE project_id = $1 AND story_id = $2 AND number = $3 AND deleted_at IS NULL;

-- name: LockChapterByNumber :one
SELECT * FROM chapters
WHERE project_id = $1 AND story_id = $2 AND number = $3 AND deleted_at IS NULL
FOR UPDATE;

-- name: LockChapterByID :one
SELECT * FROM chapters
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL
FOR UPDATE;

-- name: ListChapters :many
SELECT c.*, v.number AS volume_number FROM chapters c
LEFT JOIN volumes v ON v.project_id = c.project_id AND v.id = c.volume_id
WHERE c.project_id = $1 AND c.story_id = $2 AND c.deleted_at IS NULL
ORDER BY c.number;

-- name: SetChapterVolume :execrows
UPDATE chapters
SET volume_id = $4, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND story_id = $2 AND number = $3 AND deleted_at IS NULL;

-- name: SaveChapterDraftCAS :execrows
UPDATE chapters
SET draft_text = $4, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL
  AND version = $3;

-- name: SaveChapterDraft :execrows
UPDATE chapters
SET draft_text = $3, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: SoftDeleteChapter :execrows
UPDATE chapters
SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND story_id = $2 AND number = $3 AND deleted_at IS NULL;

-- name: SetChapterCurrentRevision :exec
UPDATE chapters
SET current_revision = sqlc.narg(revision), updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: InsertChapterVersion :one
INSERT INTO chapter_versions (project_id, chapter_id, revision_no, content,
    sha256, origin, author_kind, author_user_id, agent_name, facts,
    style_delta, legacy_build, accepted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetChapterVersion :one
SELECT * FROM chapter_versions
WHERE project_id = $1 AND chapter_id = $2 AND revision_no = $3;

-- name: LatestChapterVersion :one
SELECT * FROM chapter_versions
WHERE project_id = $1 AND chapter_id = $2
ORDER BY revision_no DESC
LIMIT 1;

-- name: ListChapterVersions :many
SELECT * FROM chapter_versions
WHERE project_id = $1 AND chapter_id = $2 AND revision_no < $3
ORDER BY revision_no DESC
LIMIT $4;

-- name: CountChapterVersions :one
SELECT count(*) FROM chapter_versions
WHERE project_id = $1 AND chapter_id = $2;

-- ── outlines ─────────────────────────────────────────────────────────────────

-- name: GetOutline :one
SELECT * FROM outlines
WHERE project_id = $1 AND story_id = $2 AND kind = $3;

-- name: UpsertOutline :exec
INSERT INTO outlines (project_id, story_id, kind, payload, text_content)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, story_id, kind)
DO UPDATE SET payload = EXCLUDED.payload,
    text_content = EXCLUDED.text_content,
    updated_at = clock_timestamp(), version = outlines.version + 1;

-- name: DeleteOutline :exec
DELETE FROM outlines
WHERE project_id = $1 AND story_id = $2 AND kind = $3;

-- ── characters ───────────────────────────────────────────────────────────────

-- name: ListCharacters :many
SELECT * FROM characters
WHERE project_id = $1 AND story_id = $2
ORDER BY name;

-- name: InsertCharacter :exec
INSERT INTO characters (project_id, story_id, id, legacy_id, name, payload)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (project_id, story_id, legacy_id)
DO UPDATE SET name = EXCLUDED.name, payload = EXCLUDED.payload,
    updated_at = clock_timestamp(), version = characters.version + 1;

-- name: DeleteCharactersForStory :exec
DELETE FROM characters
WHERE project_id = $1 AND story_id = $2;

-- ── comments / replies ───────────────────────────────────────────────────────

-- name: ListCommentsByChapter :many
SELECT cm.* FROM comments cm
JOIN chapters ch ON ch.project_id = cm.project_id AND ch.id = cm.chapter_id
WHERE cm.project_id = $1 AND cm.story_id = $2 AND ch.number = $3
  AND cm.deleted_at IS NULL
ORDER BY cm.created_at, cm.id;

-- name: LockCommentsByChapter :many
SELECT cm.* FROM comments cm
JOIN chapters ch ON ch.project_id = cm.project_id AND ch.id = cm.chapter_id
WHERE cm.project_id = $1 AND cm.story_id = $2 AND ch.number = $3
  AND cm.deleted_at IS NULL
ORDER BY cm.created_at, cm.id
FOR UPDATE OF cm;

-- name: CommentChapters :many
SELECT DISTINCT ch.number FROM comments cm
JOIN chapters ch ON ch.project_id = cm.project_id AND ch.id = cm.chapter_id
WHERE cm.project_id = $1 AND cm.story_id = $2 AND cm.deleted_at IS NULL;

-- name: InsertComment :exec
INSERT INTO comments (project_id, story_id, id, chapter_id, author_user_id,
    legacy_author_name, body, original_paragraph, original_quote, chips,
    anchor, anchor_revision, candidates, status, fixed_by, sent_at, fixed_at,
    resolved_at, unfixed, moved, relinked, whole, new_quote)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21, $22, $23);

-- name: UpdateCommentCAS :execrows
UPDATE comments
SET author_user_id = $4, legacy_author_name = $5, body = $6,
    original_paragraph = $7, original_quote = $8, chips = $9, anchor = $10,
    anchor_revision = $11, candidates = $12, status = $13, fixed_by = $14,
    sent_at = $15, fixed_at = $16, resolved_at = $17, unfixed = $18,
    moved = $19, relinked = $20, whole = $21, new_quote = $22,
    updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL
  AND version = $3;

-- name: SoftDeleteComment :execrows
UPDATE comments
SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: InsertCommentReply :exec
INSERT INTO comment_replies (project_id, comment_id, id, author_user_id,
    legacy_author_name, body)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListCommentReplies :many
SELECT * FROM comment_replies
WHERE project_id = $1 AND comment_id = $2
ORDER BY created_at, id;

-- ── reviews ──────────────────────────────────────────────────────────────────

-- name: InsertReviewNextRound :one
INSERT INTO reviews (project_id, story_id, id, scope_key, round, chapter_id,
    chapter_revision, payload, reviewer_kind, reviewer_user_id, agent_name)
VALUES ($1, $2, $3, $4,
    (SELECT coalesce(max(round), 0) + 1 FROM reviews
        WHERE project_id = $1 AND story_id = $2 AND scope_key = $4),
    $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: LatestReviewByScope :one
SELECT * FROM reviews
WHERE project_id = $1 AND story_id = $2 AND scope_key = $3
  AND invalidated_at IS NULL
ORDER BY round DESC
LIMIT 1;

-- name: ListReviewRounds :many
SELECT * FROM reviews
WHERE project_id = $1 AND story_id = $2 AND scope_key = $3
ORDER BY round;

-- name: LatestReviewsForStory :many
SELECT DISTINCT ON (scope_key) * FROM reviews
WHERE project_id = $1 AND story_id = $2 AND invalidated_at IS NULL
ORDER BY scope_key, round DESC;

-- name: UpsertReviewDecision :exec
INSERT INTO review_decisions (project_id, review_id, item_key, decision,
    actor_user_id, payload)
VALUES ($1, $2, $3, 'kept', $4, $5)
ON CONFLICT (project_id, review_id, item_key)
DO UPDATE SET payload = EXCLUDED.payload, actor_user_id = EXCLUDED.actor_user_id,
    updated_at = clock_timestamp(), version = review_decisions.version + 1;

-- name: ListReviewDecisions :many
SELECT rd.* FROM review_decisions rd
JOIN reviews r ON r.project_id = rd.project_id AND r.id = rd.review_id
WHERE rd.project_id = $1 AND r.story_id = $2;

-- name: DeleteReviewDecision :execrows
DELETE FROM review_decisions
WHERE project_id = $1 AND review_id = $2 AND item_key = $3;

-- ── story_state / artifacts / streams ────────────────────────────────────────

-- name: GetStoryState :one
SELECT * FROM story_state
WHERE project_id = $1 AND story_id = $2 AND kind = $3;

-- name: UpsertStoryState :exec
INSERT INTO story_state (project_id, story_id, kind, payload, schema_version)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, story_id, kind)
DO UPDATE SET payload = EXCLUDED.payload,
    schema_version = EXCLUDED.schema_version,
    updated_at = clock_timestamp(), version = story_state.version + 1;

-- name: DeleteStoryState :exec
DELETE FROM story_state
WHERE project_id = $1 AND story_id = $2 AND kind = $3;

-- name: GetStoryArtifact :one
SELECT * FROM story_artifacts
WHERE project_id = $1 AND story_id = $2 AND kind = $3 AND artifact_key = $4;

-- name: UpsertStoryArtifact :exec
INSERT INTO story_artifacts (project_id, story_id, kind, artifact_key,
    payload, text_content, schema_version)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (project_id, story_id, kind, artifact_key)
DO UPDATE SET payload = EXCLUDED.payload,
    text_content = EXCLUDED.text_content,
    schema_version = EXCLUDED.schema_version,
    updated_at = clock_timestamp(), version = story_artifacts.version + 1;

-- name: AppendStoryEvent :one
INSERT INTO story_events (project_id, story_id, stream, seq, payload)
VALUES ($1, $2, $3,
    (SELECT coalesce(max(seq), 0) + 1 FROM story_events
        WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND log_key = ''),
    $4)
RETURNING seq;

-- name: ListStoryEvents :many
SELECT * FROM story_events
WHERE project_id = $1 AND story_id = $2 AND stream = $3 AND seq > $4
ORDER BY seq
LIMIT $5;

-- name: AppendCheckpoint :one
INSERT INTO checkpoints (project_id, story_id, seq, scope_key, step,
    artifact_ref, sha256, occurred_at, payload)
VALUES ($1, $2,
    (SELECT coalesce(max(seq), 0) + 1 FROM checkpoints
        WHERE project_id = $1 AND story_id = $2),
    $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetCheckpointByTuple :one
SELECT * FROM checkpoints
WHERE project_id = $1 AND story_id = $2 AND scope_key = $3 AND step = $4
  AND artifact_ref = $5 AND sha256 = $6
ORDER BY seq DESC
LIMIT 1;

-- name: ListCheckpoints :many
SELECT * FROM checkpoints
WHERE project_id = $1 AND story_id = $2
ORDER BY seq;

-- name: LatestCheckpoint :one
SELECT * FROM checkpoints
WHERE project_id = $1 AND story_id = $2
ORDER BY seq DESC
LIMIT 1;

-- name: AppendAgentSession :one
INSERT INTO agent_sessions (project_id, story_id, agent, task_key, seq, payload)
VALUES ($1, $2, $3, $4,
    (SELECT coalesce(max(seq), 0) + 1 FROM agent_sessions
        WHERE project_id = $1 AND story_id = $2 AND agent = $3 AND task_key = $4),
    $5)
RETURNING seq;

-- name: ListAgentSessions :many
SELECT * FROM agent_sessions
WHERE project_id = $1 AND story_id = $2 AND agent = $3 AND task_key = $4
ORDER BY seq;

-- ── story lease ──────────────────────────────────────────────────────────────

-- name: GetStoryLease :one
SELECT * FROM story_leases
WHERE project_id = $1 AND story_id = $2;

-- name: GetStoryLeaseForWrite :one
-- Share-locks the lease row for the rest of the write transaction: a takeover
-- updates this row, so it waits until the fenced write commits or rolls back.
SELECT * FROM story_leases
WHERE project_id = $1 AND story_id = $2
FOR SHARE;

-- name: ClaimStoryLease :execrows
INSERT INTO story_leases (project_id, story_id, holder_id, fencing_token, lease_until)
SELECT s.project_id, s.id, $3, 1, $4
FROM stories s
WHERE s.project_id = $1 AND s.id = $2 AND s.deleted_at IS NULL
    AND (s.status = 'draft') = sqlc.arg(draft)::boolean
ON CONFLICT (project_id, story_id)
DO UPDATE SET holder_id = EXCLUDED.holder_id,
    lease_until = EXCLUDED.lease_until,
    fencing_token = story_leases.fencing_token + 1,
    updated_at = clock_timestamp(), version = story_leases.version + 1
WHERE story_leases.lease_until IS NULL OR story_leases.lease_until < now();

-- name: RenewStoryLease :execrows
-- Extends only a live lease this holder still owns at its fencing token; a
-- lost lease is never taken back by renewal, even when the row is free again.
UPDATE story_leases
SET lease_until = $5, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND story_id = $2 AND holder_id = $3 AND fencing_token = $4
  AND lease_until > now();

-- name: ReleaseStoryLease :execrows
UPDATE story_leases
SET holder_id = NULL, lease_until = NULL,
    updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND story_id = $2 AND holder_id = $3;

-- ── search (pg_trgm; project scope enforced before LIMIT) ────────────────────

-- name: SearchStoryTitles :many
SELECT s.slug, s.title, similarity(s.title, $2) AS score FROM stories s
WHERE s.project_id = $1 AND s.deleted_at IS NULL AND s.status <> 'draft'
  AND (s.title ILIKE '%' || $2::text || '%' OR s.title % $2::text)
ORDER BY score DESC, s.slug
LIMIT $3;

-- name: SearchChapterVersions :many
SELECT c.number AS chapter, cv.revision_no, cv.sha256,
    left(cv.content, 200) AS snippet, similarity(cv.content, $3) AS score
FROM chapter_versions cv
JOIN chapters c ON c.project_id = cv.project_id AND c.id = cv.chapter_id
WHERE cv.project_id = $1 AND c.story_id = $2 AND c.deleted_at IS NULL
  AND (cv.content ILIKE '%' || $3::text || '%' OR cv.content % $3::text)
ORDER BY score DESC, c.number, cv.revision_no DESC
LIMIT $4;

-- name: SearchChapterTitles :many
SELECT c.number AS chapter, c.title, similarity(c.title, $3) AS score
FROM chapters c
WHERE c.project_id = $1 AND c.story_id = $2 AND c.deleted_at IS NULL
  AND (c.title ILIKE '%' || $3::text || '%' OR c.title % $3::text)
ORDER BY score DESC, c.number
LIMIT $4;

-- name: SearchCharacters :many
SELECT ch.name, ch.legacy_id, similarity(ch.name, $3) AS score
FROM characters ch
WHERE ch.project_id = $1 AND ch.story_id = $2
  AND (ch.name ILIKE '%' || $3::text || '%' OR ch.name % $3::text)
ORDER BY score DESC, ch.name
LIMIT $4;
