-- +goose Up
-- Foundation: extensions shared by later migrations plus the schema metadata
-- table every backend uses to check compatibility before serving traffic.
-- Later phases append domain tables in dependency order:
--   00002 users/credentials/sessions/projects/project_members/audit/blobs
--   00003 narrative (stories…story_leases, triggers)
--   00004 workers/jobs/audio_*/export_cache
--   00005 import_runs/import_items

-- Extensions are database-global, so pin them to the stable `public` schema.
-- Creating them inside a per-test schema's first search_path entry lets one
-- test's DROP SCHEMA CASCADE remove citext from under another test.
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

-- noverse_meta is the schema bookkeeping table: key/value rows written by
-- migrations and read by the application's schema-compatibility gate
-- (postgres.Pool.CheckSchema). Domain tables never store data here.
CREATE TABLE IF NOT EXISTS noverse_meta (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- schema.format is the compatibility contract the application checks at
-- pool open: the running binary supports exactly the value in
-- postgres.SupportedSchemaFormat. It only changes on a breaking schema
-- transition; additive migrations keep it.
INSERT INTO noverse_meta (key, value) VALUES
    ('schema.format', '1'),
    ('schema.owner', 'noverse')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
-- Foundation removal is for throwaway databases only; the deployment
-- playbook forbids down on any database with writers. Drops no shared
-- extension because other schemas on the same cluster may rely on it.
DROP TABLE IF EXISTS noverse_meta;
