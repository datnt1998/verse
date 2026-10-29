-- +goose Up
-- Phase 05 (continues 00003): document-backend parity surface.
--
-- The story domain keeps two storage classes on Postgres:
--   * typed rows for everything that must be queried/joined (stories,
--     volumes, chapters, chapter_versions, outlines, characters, comments,
--     reviews, review_decisions, checkpoints);
--   * keyed documents for the remaining engine facts the file backend keeps
--     as single JSON/JSONL artefacts (run meta, user rules, signals, world
--     facts, summaries, plans, agent sessions).
--
-- This migration only widens the two document tables so no domain fact has to
-- live in a local file on the hosted path:
--   * story_state gains the singleton kinds the file backend writes as
--     meta/*.json (plus the legacy append-array shapes timeline.json /
--     state_changes.json that the append log migrates from).
--   * story_artifacts gains chapter text and the human-readable Markdown
--     views (derived, never authoritative).
--   * story_events gains log_key so one stream can carry several independent
--     append logs (runtime task logs, cocreate session) without inventing a
--     new stream name per file.
--
-- Every new kind/stream is additive: existing rows and CHECK values stay
-- valid, so this migration is safe while 00003 data is already serving.

ALTER TABLE story_state DROP CONSTRAINT IF EXISTS story_state_kind_check;
ALTER TABLE story_state ADD CONSTRAINT story_state_kind_check CHECK (kind IN (
    'book','progress','run_meta','user_rules','user_rules_runtime','signals',
    'pending_commit','pending_revision',
    'world_rules','foreshadow','timeline','cast','address_terms','relationships',
    'style_rules','foundation_audit','simulation','format','config',
    'import_manifest','import_intent','import_state','export_preferences',
    'rewrites','comment_events',
    -- added by 00004: file-backend singletons without a typed table
    'project_format','user_rules_pin','user_rules_cache','lesson_rules',
    'last_commit','last_review','usage','author_revision_style','characters',
    'outline_feedback_resolution','state_changes'
));

ALTER TABLE story_artifacts DROP CONSTRAINT IF EXISTS story_artifacts_kind_check;
ALTER TABLE story_artifacts ADD CONSTRAINT story_artifacts_kind_check CHECK (kind IN (
    'chapter_plan','summary','snapshot','import_artifact','style_sample',
    'custom_style','custom_rules','diagnostic',
    -- added by 00004
    'chapter_text','markdown_view'
));

ALTER TABLE story_events ADD COLUMN log_key text NOT NULL DEFAULT '';
ALTER TABLE story_events DROP CONSTRAINT IF EXISTS story_events_pkey;
ALTER TABLE story_events
    ADD CONSTRAINT story_events_pkey PRIMARY KEY (project_id, story_id, stream, log_key, seq);

ALTER TABLE story_events DROP CONSTRAINT IF EXISTS story_events_stream_check;
ALTER TABLE story_events ADD CONSTRAINT story_events_stream_check CHECK (stream IN (
    'decisions','usage','runtime_queue','runtime_log','rule_violations',
    'world_changes','relationships','rewrite_history','import_artifacts',
    'import_failures','outline_feedback','comment_events',
    -- added by 00004
    'timeline','state_changes','sessions'
));

DROP INDEX IF EXISTS story_events_stream_seq;
CREATE INDEX story_events_stream_seq ON story_events (project_id, story_id, stream, log_key, seq DESC);

-- +goose Down
-- Down is for throwaway databases only; the playbook forbids it on any
-- database with writers.
DROP INDEX IF EXISTS story_events_stream_seq;
DELETE FROM story_events WHERE stream IN ('timeline','state_changes','sessions') OR log_key <> '';
ALTER TABLE story_events DROP CONSTRAINT IF EXISTS story_events_stream_check;
ALTER TABLE story_events ADD CONSTRAINT story_events_stream_check CHECK (stream IN (
    'decisions','usage','runtime_queue','runtime_log','rule_violations',
    'world_changes','relationships','rewrite_history','import_artifacts',
    'import_failures','outline_feedback','comment_events'
));
ALTER TABLE story_events DROP CONSTRAINT IF EXISTS story_events_pkey;
ALTER TABLE story_events
    ADD CONSTRAINT story_events_pkey PRIMARY KEY (project_id, story_id, stream, seq);
ALTER TABLE story_events DROP COLUMN IF EXISTS log_key;
CREATE INDEX story_events_stream_seq ON story_events (project_id, story_id, stream, seq DESC);
ALTER TABLE story_artifacts DROP CONSTRAINT IF EXISTS story_artifacts_kind_check;
ALTER TABLE story_artifacts ADD CONSTRAINT story_artifacts_kind_check CHECK (kind IN (
    'chapter_plan','summary','snapshot','import_artifact','style_sample',
    'custom_style','custom_rules','diagnostic'
));
DELETE FROM story_artifacts WHERE kind IN ('chapter_text','markdown_view');
ALTER TABLE story_state DROP CONSTRAINT IF EXISTS story_state_kind_check;
ALTER TABLE story_state ADD CONSTRAINT story_state_kind_check CHECK (kind IN (
    'book','progress','run_meta','user_rules','user_rules_runtime','signals',
    'pending_commit','pending_revision',
    'world_rules','foreshadow','timeline','cast','address_terms','relationships',
    'style_rules','foundation_audit','simulation','format','config',
    'import_manifest','import_intent','import_state','export_preferences',
    'rewrites','comment_events'
));
DELETE FROM story_state WHERE kind IN (
    'project_format','user_rules_pin','user_rules_cache','lesson_rules',
    'last_commit','last_review','usage','author_revision_style','characters',
    'outline_feedback_resolution','state_changes'
);
