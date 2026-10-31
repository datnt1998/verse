-- +goose Up
ALTER TABLE storage_migrations ADD COLUMN purged_at timestamptz;
ALTER TABLE storage_migration_items ADD COLUMN source_purged_at timestamptz;
CREATE INDEX storage_source_purge_pending ON storage_migration_items(migration_id,object_key) WHERE source_purged_at IS NULL;

-- +goose Down
DROP INDEX storage_source_purge_pending;
ALTER TABLE storage_migration_items DROP COLUMN source_purged_at;
ALTER TABLE storage_migrations DROP COLUMN purged_at;
