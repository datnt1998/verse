-- +goose Up
-- Phase 08: import ledger. Every source path the importer classifies becomes
-- exactly one import_items row, so a rerun can prove "0 new rows / 0 new
-- blobs" and a reverse-export can reproduce the original file layout without
-- guessing. import_runs is keyed by (source_id, manifest_sha256): the same
-- frozen source re-runs into the same run row and never forges a new one.
--
-- The ledger is append-only (see trigger below). A changed source file is a
-- new manifest hash → new run; the old run stays as the historical record.
--
-- Global library layout (dirs/autoExport) has no project scope; it belongs to
-- the imported instance, not to a project. One row per source snapshot.
CREATE TABLE library_settings (
    source_id           uuid PRIMARY KEY,
    payload             jsonb NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE import_runs (
    id                  uuid PRIMARY KEY,
    source_id           uuid NOT NULL,
    manifest_sha256     text NOT NULL CHECK (manifest_sha256 ~ '^[0-9a-f]{64}$'),
    status              text NOT NULL CHECK (status IN ('running','verified','failed')),
    importer_version    text NOT NULL,
    -- display name only (host paths are not business data); the frozen
    -- snapshot is addressed by source_id + manifest hash.
    source_label        text NOT NULL DEFAULT '',
    summary             jsonb NOT NULL DEFAULT '{}',
    error               text NOT NULL DEFAULT '',
    verified_at         timestamptz,
    started_at          timestamptz NOT NULL DEFAULT now(),
    finished_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (source_id, manifest_sha256)
);

CREATE TABLE import_items (
    run_id              uuid NOT NULL REFERENCES import_runs(id) ON DELETE RESTRICT,
    source_id           uuid NOT NULL,
    domain              text NOT NULL,
    -- source path relative to the frozen root, slash-separated. Empty path is
    -- the accounts file (outside the library root).
    relative_path       text NOT NULL,
    -- logical record inside a multi-record file (chapter number, stream seq…)
    record_key          text NOT NULL DEFAULT '',
    disposition         text NOT NULL CHECK (disposition IN
        ('imported','blob','derived','excluded','tolerated','quarantined')),
    target_kind         text NOT NULL DEFAULT '',
    target_id           text NOT NULL DEFAULT '',
    raw_sha256          text NOT NULL CHECK (raw_sha256 ~ '^[0-9a-f]{64}$'),
    semantic_sha256     text NOT NULL CHECK (semantic_sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes          bigint NOT NULL CHECK (size_bytes >= 0),
    reason              text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    version             bigint NOT NULL DEFAULT 1 CHECK (version = 1),
    PRIMARY KEY (source_id, relative_path, record_key)
);
CREATE INDEX import_items_run ON import_items (run_id, disposition);
CREATE INDEX import_items_domain ON import_items (source_id, domain, disposition);
CREATE INDEX import_items_target ON import_items (source_id, target_kind, target_id);

-- Serializes concurrent importers for one source+target. Session-scoped
-- advisory locks are released by Postgres when the connection dies, so a
-- killed importer never leaves a permanent lock behind.
CREATE FUNCTION import_lock_key(source uuid) RETURNS bigint
LANGUAGE sql IMMUTABLE AS $$
    SELECT ('x' || substr(md5(source::text), 1, 15))::bit(60)::bigint
$$;

CREATE TRIGGER trg_import_items_immutable BEFORE UPDATE OR DELETE ON import_items
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_change();
CREATE TRIGGER trg_import_runs_version_guard BEFORE UPDATE ON import_runs
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();

-- +goose Down
-- Down is for throwaway databases only.
DROP TRIGGER IF EXISTS trg_import_runs_version_guard ON import_runs;
DROP TRIGGER IF EXISTS trg_import_items_immutable ON import_items;
DROP FUNCTION IF EXISTS import_lock_key(uuid);
DROP TABLE IF EXISTS import_items;
DROP TABLE IF EXISTS library_settings;
DROP TABLE IF EXISTS import_runs;
