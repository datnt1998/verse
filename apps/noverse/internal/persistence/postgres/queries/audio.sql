-- Audio metadata (phase 07): settings, voices, cast, scripts, segments and
-- renders on Postgres, binaries only through BlobStore.
--
-- Keys are content-addressed, never path/mtime based:
--   * a script is bound to the accepted chapter revision + its sha256;
--   * segments are keyed by (script revision, stable segment id);
--   * a render's synth_key hashes voice/tone/speed/pitch/quality/engine/model,
--     so a changed parameter produces a new candidate instead of a false hit;
--   * exactly one render per segment may be selected (partial unique index).

-- ── settings ─────────────────────────────────────────────────────────────────

-- name: GetAudioSettings :one
SELECT * FROM audio_settings WHERE project_id = $1 AND story_id = $2;

-- name: UpsertAudioSettings :one
INSERT INTO audio_settings (project_id, story_id, payload)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, story_id) DO UPDATE
SET payload = EXCLUDED.payload,
    updated_at = clock_timestamp(),
    version = audio_settings.version + 1
RETURNING *;

-- ── voices (project-scoped) ──────────────────────────────────────────────────

-- name: ListAudioVoices :many
SELECT * FROM audio_voices WHERE project_id = $1 ORDER BY legacy_id;

-- name: GetAudioVoiceByLegacy :one
SELECT * FROM audio_voices WHERE project_id = $1 AND legacy_id = $2;

-- name: GetAudioVoice :one
SELECT * FROM audio_voices WHERE project_id = $1 AND id = $2;

-- name: UpsertAudioVoice :one
INSERT INTO audio_voices (project_id, id, legacy_id, metadata, sample_blob_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, legacy_id) DO UPDATE
SET metadata = EXCLUDED.metadata,
    sample_blob_id = EXCLUDED.sample_blob_id,
    updated_at = clock_timestamp(),
    version = audio_voices.version + 1
RETURNING *;

-- name: DeleteAudioVoice :execrows
DELETE FROM audio_voices WHERE project_id = $1 AND legacy_id = $2;

-- ── cast (story-scoped) ──────────────────────────────────────────────────────

-- name: ListAudioCast :many
SELECT * FROM audio_cast WHERE project_id = $1 AND story_id = $2 ORDER BY role_key;

-- name: GetAudioCastRole :one
SELECT * FROM audio_cast WHERE project_id = $1 AND story_id = $2 AND role_key = $3;

-- name: UpsertAudioCast :one
INSERT INTO audio_cast (project_id, story_id, role_key, voice_id, payload)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, story_id, role_key) DO UPDATE
SET voice_id = EXCLUDED.voice_id,
    payload = EXCLUDED.payload,
    updated_at = clock_timestamp(),
    version = audio_cast.version + 1
RETURNING *;

-- name: DeleteAudioCastRole :execrows
DELETE FROM audio_cast WHERE project_id = $1 AND story_id = $2 AND role_key = $3;

-- ── scripts ──────────────────────────────────────────────────────────────────

-- name: UpsertAudioScript :one
INSERT INTO audio_scripts (project_id, story_id, chapter_id, revision_no,
    source_revision, source_sha256, status, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (project_id, chapter_id, revision_no) DO UPDATE
SET source_revision = EXCLUDED.source_revision,
    source_sha256 = EXCLUDED.source_sha256,
    status = EXCLUDED.status,
    payload = EXCLUDED.payload,
    updated_at = clock_timestamp(),
    version = audio_scripts.version + 1
RETURNING *;

-- name: InsertAudioScript :one
INSERT INTO audio_scripts (project_id, story_id, chapter_id, revision_no,
    source_revision, source_sha256, status, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAudioScriptCAS :one
-- Externally versioned replacement: the HTTP ETag is the expected row
-- version and the predicate lives in the same statement as the write.
-- Zero rows is a lost update, never a silent overwrite.
UPDATE audio_scripts
SET source_revision = $5,
    source_sha256 = $6,
    status = $7,
    payload = $8,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND chapter_id = $2 AND revision_no = $3 AND version = $4
RETURNING *;

-- name: GetAudioScript :one
SELECT * FROM audio_scripts
WHERE project_id = $1 AND chapter_id = $2 AND revision_no = $3;

-- name: LatestAudioScript :one
SELECT * FROM audio_scripts
WHERE project_id = $1 AND chapter_id = $2
ORDER BY revision_no DESC
LIMIT 1;

-- ── segments ─────────────────────────────────────────────────────────────────

-- name: UpsertAudioSegment :one
INSERT INTO audio_segments (project_id, chapter_id, script_revision, segment_id,
    ordinal, speaker, text_content, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (project_id, chapter_id, script_revision, segment_id) DO UPDATE
SET ordinal = EXCLUDED.ordinal,
    speaker = EXCLUDED.speaker,
    text_content = EXCLUDED.text_content,
    payload = EXCLUDED.payload,
    updated_at = clock_timestamp(),
    version = audio_segments.version + 1
RETURNING *;

-- name: ListAudioSegments :many
SELECT * FROM audio_segments
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
ORDER BY ordinal;

-- name: GetAudioSegment :one
SELECT * FROM audio_segments
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3 AND segment_id = $4;


-- name: DeleteAudioSegmentRenders :execrows
DELETE FROM audio_renders
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3 AND segment_id = $4;

-- name: DeleteAudioSegment :execrows
DELETE FROM audio_segments
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3 AND segment_id = $4;

-- name: ListAudioSegmentIDs :many
SELECT segment_id FROM audio_segments
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3;
-- ── renders ──────────────────────────────────────────────────────────────────

-- name: InsertAudioRender :one
INSERT INTO audio_renders (project_id, chapter_id, script_revision, segment_id,
    synth_key, blob_id, duration_sec, sample_rate, parameters)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (project_id, chapter_id, script_revision, segment_id, synth_key) DO UPDATE
SET blob_id = EXCLUDED.blob_id,
    duration_sec = EXCLUDED.duration_sec,
    sample_rate = EXCLUDED.sample_rate,
    parameters = EXCLUDED.parameters,
    updated_at = clock_timestamp(),
    version = audio_renders.version + 1
RETURNING *;

-- name: DeleteAudioRender :execrows
DELETE FROM audio_renders
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
  AND segment_id = $4 AND synth_key = $5;

-- name: CarryAudioRenders :execrows
-- Copies the previous revision's renders onto a new script revision, but only
-- for segments whose audible text is unchanged (same segment_id, same
-- text_content). Every copied render keeps its own synth_key and selection;
-- edited segments simply have no row on the new revision, so they read as
-- "no audio" until re-synthesized — same rule as the file backend.
INSERT INTO audio_renders (project_id, chapter_id, script_revision, segment_id,
    synth_key, blob_id, duration_sec, sample_rate, selected, parameters)
SELECT r.project_id, r.chapter_id, $3, r.segment_id,
    r.synth_key, r.blob_id, r.duration_sec, r.sample_rate, r.selected, r.parameters
FROM audio_renders r
JOIN audio_segments old_seg
  ON old_seg.project_id = r.project_id AND old_seg.chapter_id = r.chapter_id
 AND old_seg.script_revision = r.script_revision AND old_seg.segment_id = r.segment_id
JOIN audio_segments new_seg
  ON new_seg.project_id = r.project_id AND new_seg.chapter_id = r.chapter_id
 AND new_seg.script_revision = $3 AND new_seg.segment_id = r.segment_id
 AND new_seg.text_content = old_seg.text_content
WHERE r.project_id = $1 AND r.chapter_id = $2 AND r.script_revision = $4
ON CONFLICT DO NOTHING;

-- name: ListAudioRenders :many
SELECT * FROM audio_renders
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
ORDER BY segment_id, created_at;

-- name: ListSegmentRenders :many
SELECT * FROM audio_renders
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3 AND segment_id = $4
ORDER BY created_at;

-- name: SelectedAudioRender :one
SELECT * FROM audio_renders
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
  AND segment_id = $4 AND selected;

-- name: ClearSelectedAudioRenders :execrows
UPDATE audio_renders
SET selected = false, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
  AND segment_id = $4 AND selected;

-- name: SelectAudioRender :one
UPDATE audio_renders
SET selected = true, updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND chapter_id = $2 AND script_revision = $3
  AND segment_id = $4 AND synth_key = $5
RETURNING *;

-- ── blobs (shared registry) ──────────────────────────────────────────────────

-- name: StageBlob :one
INSERT INTO blobs (id, project_id, object_key, storage_driver, sha256,
    size_bytes, content_type, state, created_by, upload_job_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'staged', $8, $9)
RETURNING *;

-- name: MarkBlobReady :one
UPDATE blobs
SET state = 'ready',
    sha256 = $3,
    size_bytes = $4,
    updated_at = clock_timestamp(),
    version = version + 1
WHERE project_id = $1 AND id = $2 AND state = 'staged'
RETURNING *;

-- name: GetBlob :one
SELECT * FROM blobs WHERE project_id = $1 AND id = $2;

-- name: GetBlobByKey :one
SELECT * FROM blobs WHERE project_id = $1 AND object_key = $2;

-- ── export cache ─────────────────────────────────────────────────────────────

-- name: GetExportCache :one
SELECT * FROM export_cache
WHERE project_id = $1 AND story_id = $2 AND cache_key = $3 AND kind = $4
  AND (expires_at IS NULL OR expires_at > now());

-- name: UpsertExportCache :one
INSERT INTO export_cache (project_id, story_id, cache_key, blob_id, kind, input_manifest, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (project_id, story_id, cache_key, kind) DO UPDATE
SET blob_id = EXCLUDED.blob_id,
    input_manifest = EXCLUDED.input_manifest,
    expires_at = EXCLUDED.expires_at,
    updated_at = clock_timestamp(),
    version = export_cache.version + 1
RETURNING *;

-- name: ExpireExportCache :execrows
UPDATE export_cache
SET expires_at = clock_timestamp(), updated_at = clock_timestamp(), version = version + 1
WHERE project_id = $1 AND story_id = $2
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListExportCache :many
SELECT * FROM export_cache
WHERE project_id = $1 AND story_id = $2
ORDER BY kind, cache_key;

-- ── storage gc (admin command: `ainovel-cli storage gc`) ─────────────────────
--
-- GC is idempotent: every state transition/delete is re-checked under
-- a row lock, and the caller removes the registry row inside the transaction,
-- then the object after commit. A row without an object would be a dangling
-- reference; an object without a row is an orphan the next run collects.

-- name: ListDeletingBlobs :many
SELECT * FROM blobs WHERE state = 'deleting' ORDER BY created_at;

-- name: ListUnreferencedBlobs :many
-- Blobs no owner table points at, older than the grace window. A blob is
-- referenced by the export cache, a render, a voice sample, a project
-- document or a story artifact — everything else is unreachable bytes.
SELECT b.* FROM blobs b
WHERE b.state = 'ready' AND b.created_at < $1 AND b.updated_at < $1
  AND NOT EXISTS (SELECT 1 FROM export_cache c WHERE c.project_id = b.project_id AND c.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM audio_renders r WHERE r.project_id = b.project_id AND r.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM audio_voices v WHERE v.project_id = b.project_id AND v.sample_blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM project_documents d WHERE d.project_id = b.project_id AND d.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM story_artifacts a WHERE a.project_id = b.project_id AND a.blob_id = b.id)
ORDER BY b.created_at;

-- name: ListBlobObjectKeys :many
SELECT object_key FROM blobs;

-- name: LockBlobForDelete :one
-- Row lock for one GC decision; only valid inside a transaction. A concurrent
-- attachment takes a KEY SHARE lock through its foreign key, so it either
-- commits before this lock (and BlobReferenced then sees it) or waits and
-- fails its FK check after the row is gone — no reference can point at a blob
-- this transaction deleted.
SELECT * FROM blobs WHERE id = $1 FOR UPDATE;

-- name: BlobReferenced :one
SELECT EXISTS (SELECT 1 FROM export_cache c WHERE c.project_id = $1 AND c.blob_id = $2)
    OR EXISTS (SELECT 1 FROM audio_renders r WHERE r.project_id = $1 AND r.blob_id = $2)
    OR EXISTS (SELECT 1 FROM audio_voices v WHERE v.project_id = $1 AND v.sample_blob_id = $2)
    OR EXISTS (SELECT 1 FROM project_documents d WHERE d.project_id = $1 AND d.blob_id = $2)
    OR EXISTS (SELECT 1 FROM story_artifacts a WHERE a.project_id = $1 AND a.blob_id = $2)
    AS referenced;

-- name: DeleteBlobRow :execrows
DELETE FROM blobs WHERE id = $1;

-- name: ListExpiredExportCache :many
SELECT project_id, story_id, cache_key, kind, blob_id FROM export_cache
WHERE expires_at IS NOT NULL AND expires_at < now()
ORDER BY expires_at;


-- name: DeleteExportCacheEntryIfExpired :execrows
-- Expiry is re-checked at delete time: a concurrent UpsertExportCache may have
-- renewed the entry between the listing and this transaction, and a renewed
-- entry must survive.
DELETE FROM export_cache
WHERE project_id = $1 AND story_id = $2 AND cache_key = $3 AND kind = $4
  AND expires_at IS NOT NULL AND expires_at < now();

-- name: ListStaleStagedBlobs :many
SELECT b.* FROM blobs b
WHERE b.state = 'staged' AND b.created_at < $1
  AND NOT EXISTS (
      SELECT 1 FROM jobs j WHERE j.project_id = b.project_id
        AND (b.upload_job_id IS NULL OR j.id = b.upload_job_id)
        AND (j.state IN ('queued','paused','waiting')
             OR (j.state = 'running' AND j.lease_until IS NULL)
             OR j.lease_until > now()))
  AND NOT EXISTS (SELECT 1 FROM export_cache c WHERE c.project_id = b.project_id AND c.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM audio_renders r WHERE r.project_id = b.project_id AND r.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM audio_voices v WHERE v.project_id = b.project_id AND v.sample_blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM project_documents d WHERE d.project_id = b.project_id AND d.blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM story_artifacts a WHERE a.project_id = b.project_id AND a.blob_id = b.id)
ORDER BY b.created_at;

-- name: LockBlobUploadJobs :many
-- Match completion's lock order (job, then blob). Unlinked legacy/synchronous
-- stages conservatively consider all project jobs instead of guessing an owner.
SELECT COALESCE(j.state IN ('queued','paused','waiting')
        OR (j.state = 'running' AND j.lease_until IS NULL)
        OR j.lease_until > now(), false)::boolean AS active
FROM jobs j JOIN blobs b ON j.project_id = b.project_id
    AND (b.upload_job_id IS NULL OR j.id = b.upload_job_id)
WHERE b.id = $1
ORDER BY j.id
FOR UPDATE OF j;

-- name: TombstoneStagedBlob :execrows
UPDATE blobs SET state = 'deleting', updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND state = 'staged';
