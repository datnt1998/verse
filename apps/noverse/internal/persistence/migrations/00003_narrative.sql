-- +goose Up
-- Phase 05: narrative domain — stories, volumes, chapters with immutable
-- chapter_versions (author attribution + canonical hash), outlines,
-- characters, comments/replies (anchor revision FK), reviews/review_decisions
-- (kept decisions keyed per review), project_documents, story_state typed
-- singletons, story_artifacts keyed rows, append-only streams (story_events,
-- checkpoints, agent_sessions) and story_leases. pg_trgm search indexes.
-- DDL follows reports/schema-and-contracts.md §2–§3.11 (00003 allocation).
--
-- Trigger families created here (spec §3.1/3–4):
--   * prevent_immutable_change — blocks UPDATE/DELETE on append-only tables
--     for everyone. Retention paths must wrap the write in a transaction that
--     sets `SET LOCAL noverse.allow_immutable_write = 'on'` first; this is the
--     maintenance escape hatch (audit purge is the only live caller).
--   * enforce_row_version_guard — mutable narrative rows reject UPDATEs that
--     do not bump version by exactly 1 or that do not change updated_at. Go
--     repositories still perform CAS (WHERE version = $expected); the trigger
--     is a second line, not the mechanism.

-- +goose StatementBegin
CREATE FUNCTION prevent_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF current_setting('noverse.allow_immutable_write', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'append-only table % does not accept %', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'raise_exception';
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION enforce_row_version_guard()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'update on % must bump version by exactly 1 (% -> %)',
            TG_TABLE_NAME, OLD.version, NEW.version
            USING ERRCODE = 'raise_exception';
    END IF;
    IF NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at THEN
        RAISE EXCEPTION 'update on % must change updated_at', TG_TABLE_NAME
            USING ERRCODE = 'raise_exception';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE project_documents (
    project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    id                  uuid NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('idea','rules','style','ledger','config','asset')),
    logical_name        text NOT NULL,
    text_content        text,
    payload             jsonb,
    blob_id             uuid,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, kind, logical_name),
    FOREIGN KEY (project_id, blob_id) REFERENCES blobs(project_id, id) ON DELETE RESTRICT,
    CHECK (text_content IS NOT NULL OR payload IS NOT NULL OR blob_id IS NOT NULL)
);

CREATE TABLE stories (
    project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    id                  uuid NOT NULL,
    slug                text NOT NULL,
    title               text NOT NULL,
    short_title         text NOT NULL DEFAULT '',
    synopsis            text NOT NULL DEFAULT '',
    title_renamed       boolean NOT NULL DEFAULT false,
    status              text NOT NULL CHECK (status IN ('idea','writing','paused','done','published')),
    tags                text[] NOT NULL DEFAULT '{}',
    published_at        timestamptz,
    deleted_at          timestamptz,
    idea_document_id    uuid,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, slug),
    FOREIGN KEY (project_id, idea_document_id) REFERENCES project_documents(project_id, id) ON DELETE RESTRICT,
    CHECK (length(slug) > 0)
);

CREATE TABLE volumes (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    id                  uuid NOT NULL,
    number              integer NOT NULL CHECK (number > 0),
    title               text NOT NULL DEFAULT '',
    outline             jsonb NOT NULL DEFAULT '{}',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, story_id, number),
    UNIQUE (project_id, story_id, id),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE chapters (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    id                  uuid NOT NULL,
    volume_id           uuid,
    number              integer NOT NULL CHECK (number > 0),
    title               text NOT NULL DEFAULT '',
    draft_text          text NOT NULL DEFAULT '',
    current_revision    integer,
    deleted_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, story_id, number),
    UNIQUE (project_id, story_id, id),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, story_id, volume_id) REFERENCES volumes(project_id, story_id, id) ON DELETE RESTRICT,
    CHECK (current_revision IS NULL OR current_revision > 0)
);

CREATE TABLE chapter_versions (
    project_id          uuid NOT NULL,
    chapter_id          uuid NOT NULL,
    revision_no         integer NOT NULL CHECK (revision_no > 0),
    content             text NOT NULL,
    sha256              text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    origin              text NOT NULL CHECK (origin IN ('generated','user','import')),
    author_kind         text NOT NULL CHECK (author_kind IN ('user','agent','import')),
    author_user_id      uuid REFERENCES users(id) ON DELETE RESTRICT,
    agent_name          text,
    facts               jsonb NOT NULL DEFAULT '{}',
    style_delta         jsonb NOT NULL DEFAULT '{}',
    legacy_build        text,
    accepted_at         timestamptz NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (project_id, chapter_id, revision_no),
    FOREIGN KEY (project_id, chapter_id) REFERENCES chapters(project_id, id) ON DELETE RESTRICT,
    CHECK ((author_kind = 'user' AND author_user_id IS NOT NULL AND agent_name IS NULL)
        OR (author_kind = 'agent' AND agent_name IS NOT NULL AND author_user_id IS NULL)
        OR (author_kind = 'import' AND author_user_id IS NULL AND agent_name IS NULL)),
    CHECK (updated_at = created_at)
);

ALTER TABLE chapters
    ADD CONSTRAINT chapters_current_version_fk
    FOREIGN KEY (project_id, id, current_revision)
    REFERENCES chapter_versions(project_id, chapter_id, revision_no)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE outlines (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('flat','layered','premise','compass','retention')),
    payload             jsonb NOT NULL,
    text_content        text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id, kind),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE characters (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    id                  uuid NOT NULL,
    legacy_id           text NOT NULL,
    name                text NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, story_id, legacy_id),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE comments (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    -- id is the opaque external comment id (crypto/rand text), not a UUID;
    -- identity inside the DB is (project_id, id).
    id                  text NOT NULL,
    chapter_id          uuid NOT NULL,
    author_user_id      uuid REFERENCES users(id) ON DELETE RESTRICT,
    legacy_author_name  text,
    body                text NOT NULL,
    original_paragraph  integer,
    original_quote      text NOT NULL DEFAULT '',
    chips               text[] NOT NULL DEFAULT '{}',
    anchor              jsonb,
    anchor_revision     integer,
    candidates          jsonb NOT NULL DEFAULT '[]',
    status              text NOT NULL CHECK (status IN ('pending','sent','fixed','lost','unsure','done')),
    fixed_by            text CHECK (fixed_by IN ('agent','quote_gone')),
    sent_at             timestamptz,
    fixed_at            timestamptz,
    resolved_at         timestamptz,
    unfixed             boolean NOT NULL DEFAULT false,
    moved               boolean NOT NULL DEFAULT false,
    relinked            boolean NOT NULL DEFAULT false,
    whole               boolean NOT NULL DEFAULT false,
    new_quote           text NOT NULL DEFAULT '',
    deleted_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    FOREIGN KEY (project_id, story_id, chapter_id) REFERENCES chapters(project_id, story_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, chapter_id, anchor_revision) REFERENCES chapter_versions(project_id, chapter_id, revision_no) ON DELETE RESTRICT,
    CHECK (author_user_id IS NOT NULL OR legacy_author_name IS NOT NULL)
);

CREATE TABLE comment_replies (
    project_id          uuid NOT NULL,
    comment_id          text NOT NULL,
    id                  text NOT NULL,
    author_user_id      uuid REFERENCES users(id) ON DELETE RESTRICT,
    legacy_author_name  text,
    body                text NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    FOREIGN KEY (project_id, comment_id) REFERENCES comments(project_id, id) ON DELETE RESTRICT,
    CHECK (author_user_id IS NOT NULL OR legacy_author_name IS NOT NULL)
);

CREATE TABLE reviews (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    id                  uuid NOT NULL,
    scope_key           text NOT NULL,
    round               integer NOT NULL CHECK (round > 0),
    chapter_id          uuid,
    chapter_revision    integer,
    payload             jsonb NOT NULL,
    reviewer_kind       text NOT NULL CHECK (reviewer_kind IN ('user','agent','import')),
    reviewer_user_id    uuid REFERENCES users(id) ON DELETE RESTRICT,
    agent_name          text,
    invalidated_at      timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, story_id, scope_key, round),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, story_id, chapter_id) REFERENCES chapters(project_id, story_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, chapter_id, chapter_revision) REFERENCES chapter_versions(project_id, chapter_id, revision_no) ON DELETE RESTRICT,
    CHECK ((chapter_id IS NULL) = (chapter_revision IS NULL)),
    CHECK ((reviewer_kind = 'user' AND reviewer_user_id IS NOT NULL AND agent_name IS NULL)
        OR (reviewer_kind = 'agent' AND agent_name IS NOT NULL AND reviewer_user_id IS NULL)
        OR (reviewer_kind = 'import' AND reviewer_user_id IS NULL AND agent_name IS NULL))
);

CREATE TABLE review_decisions (
    project_id          uuid NOT NULL,
    review_id           uuid NOT NULL,
    item_key            text NOT NULL,
    decision            text NOT NULL CHECK (decision = 'kept'),
    actor_user_id       uuid REFERENCES users(id) ON DELETE RESTRICT,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, review_id, item_key),
    FOREIGN KEY (project_id, review_id) REFERENCES reviews(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE story_leases (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    holder_id           uuid,
    fencing_token       bigint NOT NULL DEFAULT 0 CHECK (fencing_token >= 0),
    lease_until         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK ((holder_id IS NULL) = (lease_until IS NULL))
);

CREATE TABLE story_state (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    kind                text NOT NULL,
    payload             jsonb NOT NULL,
    schema_version      integer NOT NULL CHECK (schema_version > 0),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id, kind),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK (kind IN ('book','progress','run_meta','user_rules','user_rules_runtime','signals','pending_commit','pending_revision',
        'world_rules','foreshadow','timeline','cast','address_terms','relationships','style_rules',
        'foundation_audit','simulation','format','config','import_manifest','import_intent','import_state','export_preferences','rewrites','comment_events'))
);

CREATE TABLE story_artifacts (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    kind                text NOT NULL,
    artifact_key        text NOT NULL,
    payload             jsonb,
    text_content        text,
    blob_id             uuid,
    schema_version      integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (project_id, story_id, kind, artifact_key),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, blob_id) REFERENCES blobs(project_id, id) ON DELETE RESTRICT,
    CHECK (kind IN ('chapter_plan','summary','snapshot','import_artifact','style_sample','custom_style','custom_rules','diagnostic')),
    CHECK (payload IS NOT NULL OR text_content IS NOT NULL OR blob_id IS NOT NULL)
);

CREATE TABLE story_events (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    stream              text NOT NULL,
    seq                 bigint NOT NULL CHECK (seq > 0),
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (project_id, story_id, stream, seq),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK (stream IN ('decisions','usage','runtime_queue','runtime_log','rule_violations','world_changes','relationships','rewrite_history','import_artifacts','import_failures','outline_feedback','comment_events')),
    CHECK (updated_at = created_at)
);

CREATE TABLE checkpoints (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    seq                 bigint NOT NULL CHECK (seq > 0),
    scope_key           text NOT NULL,
    step                text NOT NULL,
    artifact_ref        text NOT NULL,
    sha256              text NOT NULL,
    occurred_at         timestamptz NOT NULL,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (project_id, story_id, seq),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK (updated_at = created_at)
);

CREATE TABLE agent_sessions (
    project_id          uuid NOT NULL,
    story_id            uuid NOT NULL,
    agent               text NOT NULL,
    task_key            text NOT NULL,
    seq                 bigint NOT NULL CHECK (seq > 0),
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (project_id, story_id, agent, task_key, seq),
    FOREIGN KEY (project_id, story_id) REFERENCES stories(project_id, id) ON DELETE RESTRICT,
    CHECK (updated_at = created_at)
);

-- Search indexes (pg_trgm; scoped reads always filter project + deleted
-- before LIMIT — indexes make the scoped scan cheap, they do not grant a
-- cross-tenant result). Match keeps Vietnamese accents as-is; no
-- accent-insensitive promise.
CREATE INDEX stories_title_trgm ON stories USING gin (title gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX chapters_title_trgm ON chapters USING gin (title gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX chapter_content_trgm ON chapter_versions USING gin (content gin_trgm_ops);
CREATE INDEX characters_name_trgm ON characters USING gin (name gin_trgm_ops);
CREATE INDEX comments_chapter ON comments (project_id, chapter_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX reviews_scope ON reviews (project_id, story_id, scope_key, round DESC);
CREATE INDEX chapters_story_number ON chapters (project_id, story_id, number) WHERE deleted_at IS NULL;
CREATE INDEX story_events_stream_seq ON story_events (project_id, story_id, stream, seq DESC);

-- Append-only immutability: history never mutates for application writers.
CREATE TRIGGER trg_chapter_versions_immutable BEFORE UPDATE OR DELETE ON chapter_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();
CREATE TRIGGER trg_story_events_immutable BEFORE UPDATE OR DELETE ON story_events
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();
CREATE TRIGGER trg_checkpoints_immutable BEFORE UPDATE OR DELETE ON checkpoints
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();
CREATE TRIGGER trg_agent_sessions_immutable BEFORE UPDATE OR DELETE ON agent_sessions
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();
CREATE TRIGGER trg_audit_logs_immutable BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();

-- Mutable-row CAS guard: every UPDATE must bump version by exactly 1 and
-- change updated_at. DELETE is not guarded (FKs already restrict it).
CREATE TRIGGER trg_stories_version_guard BEFORE UPDATE ON stories
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_volumes_version_guard BEFORE UPDATE ON volumes
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_chapters_version_guard BEFORE UPDATE ON chapters
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_outlines_version_guard BEFORE UPDATE ON outlines
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_characters_version_guard BEFORE UPDATE ON characters
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_comments_version_guard BEFORE UPDATE ON comments
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_comment_replies_version_guard BEFORE UPDATE ON comment_replies
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_reviews_version_guard BEFORE UPDATE ON reviews
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_review_decisions_version_guard BEFORE UPDATE ON review_decisions
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_story_leases_version_guard BEFORE UPDATE ON story_leases
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_story_state_version_guard BEFORE UPDATE ON story_state
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_story_artifacts_version_guard BEFORE UPDATE ON story_artifacts
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_project_documents_version_guard BEFORE UPDATE ON project_documents
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();

-- +goose Down
-- Down is for throwaway databases only; the playbook forbids it on any
-- database with writers.
DROP TRIGGER IF EXISTS trg_project_documents_version_guard ON project_documents;
DROP TRIGGER IF EXISTS trg_story_artifacts_version_guard ON story_artifacts;
DROP TRIGGER IF EXISTS trg_story_state_version_guard ON story_state;
DROP TRIGGER IF EXISTS trg_story_leases_version_guard ON story_leases;
DROP TRIGGER IF EXISTS trg_review_decisions_version_guard ON review_decisions;
DROP TRIGGER IF EXISTS trg_reviews_version_guard ON reviews;
DROP TRIGGER IF EXISTS trg_comment_replies_version_guard ON comment_replies;
DROP TRIGGER IF EXISTS trg_comments_version_guard ON comments;
DROP TRIGGER IF EXISTS trg_characters_version_guard ON characters;
DROP TRIGGER IF EXISTS trg_outlines_version_guard ON outlines;
DROP TRIGGER IF EXISTS trg_chapters_version_guard ON chapters;
DROP TRIGGER IF EXISTS trg_volumes_version_guard ON volumes;
DROP TRIGGER IF EXISTS trg_stories_version_guard ON stories;
DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
DROP TRIGGER IF EXISTS trg_agent_sessions_immutable ON agent_sessions;
DROP TRIGGER IF EXISTS trg_checkpoints_immutable ON checkpoints;
DROP TRIGGER IF EXISTS trg_story_events_immutable ON story_events;
DROP TRIGGER IF EXISTS trg_chapter_versions_immutable ON chapter_versions;

DROP TABLE IF EXISTS agent_sessions;
DROP TABLE IF EXISTS checkpoints;
DROP TABLE IF EXISTS story_events;
DROP TABLE IF EXISTS story_artifacts;
DROP TABLE IF EXISTS story_state;
DROP TABLE IF EXISTS story_leases;
DROP TABLE IF EXISTS review_decisions;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS comment_replies;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS characters;
DROP TABLE IF EXISTS outlines;
DROP TABLE IF EXISTS chapters;
DROP TABLE IF EXISTS chapter_versions;
DROP TABLE IF EXISTS volumes;
DROP TABLE IF EXISTS stories;
DROP TABLE IF EXISTS project_documents;
DROP FUNCTION IF EXISTS enforce_row_version_guard();
DROP FUNCTION IF EXISTS prevent_immutable_change();
