-- +goose Up
CREATE TABLE blob_stores (
    id text PRIMARY KEY,
    config jsonb NOT NULL,
    credentials bytea,
    key_id text NOT NULL DEFAULT 'aes256gcm-v1',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE storage_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    active_store text REFERENCES blob_stores(id),
    pending_store text REFERENCES blob_stores(id),
    version bigint NOT NULL DEFAULT 1
);
INSERT INTO storage_settings(singleton) VALUES (true);
ALTER TABLE blobs ADD COLUMN store_id text REFERENCES blob_stores(id);
ALTER TABLE blobs ADD COLUMN usage_kind text NOT NULL DEFAULT 'unclassified';
CREATE INDEX blobs_usage_ready ON blobs(usage_kind) WHERE state='ready';
CREATE INDEX blobs_store_ready ON blobs(store_id, object_key) WHERE state='ready';
CREATE TABLE storage_migrations (
    id text PRIMARY KEY,
    source_store text NOT NULL REFERENCES blob_stores(id),
    target_store text NOT NULL REFERENCES blob_stores(id),
    state text NOT NULL CHECK (state IN ('running','paused','error','completed','cancelled')),
    error_code text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    copied_count bigint NOT NULL DEFAULT 0,
    copied_bytes bigint NOT NULL DEFAULT 0,
    total_count bigint NOT NULL DEFAULT 0,
    total_bytes bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX storage_one_migration ON storage_migrations ((true)) WHERE state IN ('running','paused','error');
CREATE TABLE storage_migration_items (
    migration_id text NOT NULL REFERENCES storage_migrations(id),
    object_key text NOT NULL,
    sha256 text NOT NULL,
    size_bytes bigint NOT NULL,
    verified boolean NOT NULL DEFAULT false,
    error_code text NOT NULL DEFAULT '',
    PRIMARY KEY(migration_id,object_key)
);
CREATE TABLE storage_probes (
    store_id text NOT NULL REFERENCES blob_stores(id),
    object_key text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY(store_id,object_key)
);
-- Every publisher takes the shared side of the migration write fence before
-- uploading. The trigger also fences direct registry writers and stamps the
-- exact store identity, including S3-to-S3 transitions.
-- +goose StatementBegin
CREATE FUNCTION noverse_blob_store_stamp() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        SELECT active_store INTO NEW.store_id FROM storage_settings WHERE singleton;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER trg_blob_store_stamp BEFORE INSERT ON blobs FOR EACH ROW EXECUTE FUNCTION noverse_blob_store_stamp();
-- Explicit grants mirror 00010/00011: no widening of history or blob DELETE.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname='noverse_app') THEN
        -- Provisioning grants future app tables DELETE by default. Settings
        -- and migration history must remain durable; 00018 restores only the
        -- FK-protected DELETE needed to reclaim unused probe store configs.
        REVOKE DELETE ON storage_settings,storage_migrations,storage_migration_items,blob_stores FROM noverse_app;
        GRANT SELECT,INSERT,UPDATE ON blob_stores,storage_settings,storage_migrations,storage_migration_items TO noverse_app;
        GRANT SELECT,INSERT,DELETE ON storage_probes TO noverse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER trg_blob_store_stamp ON blobs;
DROP FUNCTION noverse_blob_store_stamp();
DROP TABLE storage_probes,storage_migration_items,storage_migrations;
ALTER TABLE blobs DROP COLUMN store_id;
ALTER TABLE blobs DROP COLUMN usage_kind;
DROP TABLE storage_settings,blob_stores;
