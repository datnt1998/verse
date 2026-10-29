-- +goose Up
-- Phase 06: story_leases needs a key.
--
-- The lease is one row per (project, story) and every claim/release statement
-- uses ON CONFLICT on that pair, but 00003 created the table without a unique
-- constraint. Without the key ON CONFLICT cannot be planned (SQLSTATE 42P10),
-- so the durable engine fence could never be claimed.
ALTER TABLE story_leases
    ADD CONSTRAINT story_leases_pkey PRIMARY KEY (project_id, story_id);

-- +goose Down
ALTER TABLE story_leases DROP CONSTRAINT IF EXISTS story_leases_pkey;
