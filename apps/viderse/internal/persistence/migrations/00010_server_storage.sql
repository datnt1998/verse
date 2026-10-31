-- +goose Up
CREATE TABLE storage_migrations (
 task_id text PRIMARY KEY,
 from_store text NOT NULL REFERENCES storage_stores(id),
 to_store text NOT NULL REFERENCES storage_stores(id),
 state text NOT NULL CHECK(state IN ('running','paused','error','done','cancelled')),
 revision bigint NOT NULL,
 journal_seq bigint NOT NULL DEFAULT 0,
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 progress jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE UNIQUE INDEX storage_one_migration_idx ON storage_migrations ((true)) WHERE state IN ('running','paused','error');
CREATE TABLE storage_migration_items (
 task_id text NOT NULL REFERENCES storage_migrations(task_id),
 logical_path text NOT NULL,
 object_key text NOT NULL REFERENCES blobs(object_key),
 usage_group text NOT NULL,
 state text NOT NULL CHECK(state IN ('waiting','verified','error')),
 error_code text,
 detail text,
 verified_at timestamptz,
 PRIMARY KEY(task_id,logical_path,object_key)
);
CREATE INDEX storage_migration_pending_idx ON storage_migration_items(task_id,state);
CREATE TABLE storage_health (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 store_id text NOT NULL REFERENCES storage_stores(id),
 online boolean NOT NULL,
 checked_at timestamptz NOT NULL,
 since timestamptz,
 error_code text
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM storage_migrations) THEN
  RAISE EXCEPTION 'Retain storage migration history during rollback';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE storage_health, storage_migration_items, storage_migrations;
