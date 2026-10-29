-- +goose Up
-- Phase 07: durable background jobs + audio metadata (allocation 00004 of
-- reports/schema-and-contracts.md §3.11; numbered 00007 because 00004–00006
-- were already released for story documents and the lease key).
--
-- Tables:
--   workers          — least-privilege service credentials (hashed tokens,
--                      capability list; 'tts' is the only capability today)
--   jobs             — durable queue rows: SKIP LOCKED claim, 120 s lease,
--                      fencing token, idempotency key per (story, kind)
--   job_events       — append-only job progress/history stream
--   audio_settings   — per-story audio config snapshot (JSONB payload)
--   audio_voices     — project-scoped voice registry (sample → blob)
--   audio_cast       — story role → voice assignment
--   audio_scripts    — TTS script bound to an accepted chapter revision
--   audio_segments   — script segments keyed by script revision + stable id
--   audio_renders    — synthesized candidates keyed by synth key; one
--                      selected render per segment (partial unique index)
--   export_cache     — export manifests + output blobs keyed by content hash,
--                      never mtime
--
-- Trigger families (created in 00002/00003):
--   prevent_immutable_change  → job_events
--   enforce_row_version_guard → every mutable table here

CREATE TABLE workers (
    id                  uuid PRIMARY KEY,
    name                text NOT NULL,
    token_hash          bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    capabilities        text[] NOT NULL,
    revoked_at          timestamptz,
    last_seen_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (capabilities <@ ARRAY['tts']::text[]),
    CHECK (cardinality(capabilities) > 0)
);

CREATE TABLE jobs (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    id                  uuid NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('tts','audio_export','text_export','engine','story_import','simulate','sync')),
    state               text NOT NULL CHECK (state IN ('queued','running','paused','waiting','failed','completed','cancelled')),
    idempotency_key     text NOT NULL,
    input               jsonb NOT NULL,
    result              jsonb,
    error               jsonb,
    progress_done       integer NOT NULL DEFAULT 0 CHECK (progress_done >= 0),
    progress_total      integer CHECK (progress_total >= progress_done),
    requested_by        uuid REFERENCES users(id) ON DELETE RESTRICT,
    worker_id           uuid REFERENCES workers(id) ON DELETE RESTRICT,
    lease_until         timestamptz,
    fencing_token       bigint NOT NULL DEFAULT 0 CHECK (fencing_token >= 0),
    cancelled_at        timestamptz,
    started_at          timestamptz,
    finished_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, story_id, kind, idempotency_key),
    UNIQUE (project_id, story_id, id),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK (kind <> 'tts' OR state <> 'running' OR (worker_id IS NOT NULL AND lease_until IS NOT NULL)),
    CHECK (state NOT IN ('completed','cancelled') OR finished_at IS NOT NULL)
);
CREATE INDEX jobs_claim ON jobs (kind, state, created_at) WHERE state IN ('queued','running');
CREATE INDEX jobs_story ON jobs (project_id, story_id, created_at DESC);
CREATE INDEX jobs_worker ON jobs (worker_id) WHERE worker_id IS NOT NULL;

CREATE TABLE job_events (
    project_id          uuid NOT NULL,
    job_id              uuid NOT NULL,
    seq                 bigint NOT NULL CHECK (seq > 0),
    event_type          text NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (project_id, job_id, seq),
    CHECK (updated_at = created_at),
    FOREIGN KEY (project_id, job_id) REFERENCES jobs(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE audio_settings (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE audio_voices (
    project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    id                  uuid NOT NULL,
    legacy_id           text NOT NULL,
    metadata            jsonb NOT NULL,
    sample_blob_id      uuid,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, legacy_id),
    FOREIGN KEY (project_id, sample_blob_id) REFERENCES blobs(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE audio_cast (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    role_key            text NOT NULL,
    voice_id            uuid,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id, role_key),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, voice_id) REFERENCES audio_voices(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE audio_scripts (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    chapter_id          uuid NOT NULL,
    revision_no         integer NOT NULL CHECK (revision_no > 0),
    source_revision     integer NOT NULL,
    source_sha256       text NOT NULL,
    status              text NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, chapter_id, revision_no),
    FOREIGN KEY (project_id, story_id, chapter_id) REFERENCES chapters(project_id, story_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, chapter_id, source_revision) REFERENCES chapter_versions(project_id, chapter_id, revision_no) ON DELETE RESTRICT
);

CREATE TABLE audio_segments (
    project_id          uuid NOT NULL,
    chapter_id          uuid NOT NULL,
    script_revision     integer NOT NULL,
    segment_id          text NOT NULL,
    ordinal             integer NOT NULL CHECK (ordinal >= 0),
    speaker             text NOT NULL,
    text_content        text NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, chapter_id, script_revision, segment_id),
    UNIQUE (project_id, chapter_id, script_revision, ordinal),
    FOREIGN KEY (project_id, chapter_id, script_revision) REFERENCES audio_scripts(project_id, chapter_id, revision_no) ON DELETE RESTRICT
);

CREATE TABLE audio_renders (
    project_id          uuid NOT NULL,
    chapter_id          uuid NOT NULL,
    script_revision     integer NOT NULL,
    segment_id          text NOT NULL,
    synth_key           text NOT NULL CHECK (synth_key ~ '^[0-9a-f]{64}$'),
    blob_id             uuid NOT NULL,
    duration_sec        double precision NOT NULL CHECK (duration_sec >= 0),
    sample_rate         integer NOT NULL CHECK (sample_rate > 0),
    selected            boolean NOT NULL DEFAULT false,
    parameters          jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, chapter_id, script_revision, segment_id, synth_key),
    FOREIGN KEY (project_id, chapter_id, script_revision, segment_id)
        REFERENCES audio_segments(project_id, chapter_id, script_revision, segment_id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, blob_id) REFERENCES blobs(project_id, id) ON DELETE RESTRICT
);
-- Exactly one selected render per segment; stale outputs stay as historical
-- candidates but can never become "current" silently.
CREATE UNIQUE INDEX audio_one_selected ON audio_renders (project_id, chapter_id, script_revision, segment_id) WHERE selected;

CREATE TABLE export_cache (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    cache_key           text NOT NULL,
    blob_id             uuid NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('txt','epub','wav','mp3','m4b','srt','zip','cover','script','marks')),
    input_manifest      jsonb NOT NULL,
    expires_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id, cache_key, kind),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, blob_id) REFERENCES blobs(project_id, id) ON DELETE RESTRICT
);

CREATE TRIGGER trg_job_events_immutable BEFORE UPDATE OR DELETE ON job_events
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();

CREATE TRIGGER trg_workers_version_guard BEFORE UPDATE ON workers
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_jobs_version_guard BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_settings_version_guard BEFORE UPDATE ON audio_settings
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_voices_version_guard BEFORE UPDATE ON audio_voices
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_cast_version_guard BEFORE UPDATE ON audio_cast
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_scripts_version_guard BEFORE UPDATE ON audio_scripts
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_segments_version_guard BEFORE UPDATE ON audio_segments
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_audio_renders_version_guard BEFORE UPDATE ON audio_renders
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_export_cache_version_guard BEFORE UPDATE ON export_cache
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();

-- +goose Down
DROP TRIGGER IF EXISTS trg_export_cache_version_guard ON export_cache;
DROP TRIGGER IF EXISTS trg_audio_renders_version_guard ON audio_renders;
DROP TRIGGER IF EXISTS trg_audio_segments_version_guard ON audio_segments;
DROP TRIGGER IF EXISTS trg_audio_scripts_version_guard ON audio_scripts;
DROP TRIGGER IF EXISTS trg_audio_cast_version_guard ON audio_cast;
DROP TRIGGER IF EXISTS trg_audio_voices_version_guard ON audio_voices;
DROP TRIGGER IF EXISTS trg_audio_settings_version_guard ON audio_settings;
DROP TRIGGER IF EXISTS trg_jobs_version_guard ON jobs;
DROP TRIGGER IF EXISTS trg_workers_version_guard ON workers;
DROP TRIGGER IF EXISTS trg_job_events_immutable ON job_events;

DROP TABLE IF EXISTS export_cache;
DROP TABLE IF EXISTS audio_renders;
DROP TABLE IF EXISTS audio_segments;
DROP TABLE IF EXISTS audio_scripts;
DROP TABLE IF EXISTS audio_cast;
DROP TABLE IF EXISTS audio_voices;
DROP TABLE IF EXISTS audio_settings;
DROP TABLE IF EXISTS job_events;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS workers;
