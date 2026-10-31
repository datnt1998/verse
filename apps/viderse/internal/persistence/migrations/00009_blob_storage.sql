-- +goose Up
CREATE TABLE storage_stores (
 id text PRIMARY KEY,
 driver text NOT NULL CHECK (driver IN ('local','s3')),
 config jsonb NOT NULL,
 credentials bytea,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE storage_settings (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 active_store text NOT NULL REFERENCES storage_stores(id),
 pending_store text REFERENCES storage_stores(id),
 previous_store text REFERENCES storage_stores(id),
 revision bigint NOT NULL DEFAULT 1,
 generation bigint NOT NULL DEFAULT 1
);
CREATE TABLE blobs (
 object_key text PRIMARY KEY CHECK (object_key ~ '^v1/sha256/[0-9a-f]{2}/[0-9a-f]{64}$'),
 sha256 text NOT NULL UNIQUE CHECK (sha256 ~ '^[0-9a-f]{64}$'),
 bytes bigint NOT NULL CHECK (bytes >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK (object_key = 'v1/sha256/' || substr(sha256,1,2) || '/' || sha256)
);
CREATE TABLE blob_locations (
 store_id text NOT NULL REFERENCES storage_stores(id),
 object_key text NOT NULL REFERENCES blobs(object_key),
 source_path text,
 verified_at timestamptz NOT NULL,
 protected_until timestamptz,
 unreferenced_at timestamptz,
 PRIMARY KEY (store_id,object_key)
);
CREATE TABLE blob_refs (
 logical_path text PRIMARY KEY CHECK (logical_path <> '' AND logical_path !~ '(^/|(^|/)\.{1,2}(/|$)|\\|//|/$)'),
 object_key text NOT NULL REFERENCES blobs(object_key) ON DELETE RESTRICT,
 content_type text NOT NULL,
 filename text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX blob_refs_object_idx ON blob_refs(object_key);
CREATE TABLE storage_journal (
 seq bigserial PRIMARY KEY,
 relation_name text NOT NULL,
 operation text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

-- The database fence also covers CLI processes and every domain mutation,
-- including cascades. It never covers preprocessing (imagegen/yt-dlp/ffmpeg).
-- +goose StatementBegin
CREATE FUNCTION storage_publication_fence() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock_shared(253025,1);
 RETURN NULL;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION storage_record_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO storage_journal(relation_name,operation) VALUES (TG_TABLE_NAME,TG_OP);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER storage_overlay_fence BEFORE INSERT OR UPDATE OR DELETE ON overlays FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_template_fence BEFORE INSERT OR UPDATE OR DELETE ON templates FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_channel_fence BEFORE INSERT OR UPDATE OR DELETE ON owner_channels FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_backdrop_fence BEFORE INSERT OR UPDATE OR DELETE ON backdrop_videos FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_overlay_journal AFTER INSERT OR UPDATE OR DELETE ON overlays FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE TRIGGER storage_template_journal AFTER INSERT OR UPDATE OR DELETE ON templates FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE TRIGGER storage_channel_journal AFTER INSERT OR UPDATE OR DELETE ON owner_channels FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE TRIGGER storage_backdrop_journal AFTER INSERT OR UPDATE OR DELETE ON backdrop_videos FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();

CREATE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION
 SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION
 SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION
 SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> '';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM blob_refs) OR EXISTS (SELECT 1 FROM blob_locations) THEN
  RAISE EXCEPTION 'Refusing to drop populated blob storage; retain additive schema during rollback';
 END IF;
END $$;
-- +goose StatementEnd
DROP VIEW storage_live_refs;
DROP TRIGGER storage_overlay_fence ON overlays;
DROP TRIGGER storage_template_fence ON templates;
DROP TRIGGER storage_channel_fence ON owner_channels;
DROP TRIGGER storage_backdrop_fence ON backdrop_videos;
DROP TRIGGER storage_overlay_journal ON overlays;
DROP TRIGGER storage_template_journal ON templates;
DROP TRIGGER storage_channel_journal ON owner_channels;
DROP TRIGGER storage_backdrop_journal ON backdrop_videos;
DROP FUNCTION storage_publication_fence();
DROP FUNCTION storage_record_mutation();
DROP TABLE storage_journal, blob_refs, blob_locations, blobs, storage_settings, storage_stores;
