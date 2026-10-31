-- +goose Up
ALTER TABLE clone_queue
    ADD COLUMN scene_prompt_id text,
    ADD COLUMN cover_prompt_id text,
    ADD COLUMN no_images_override boolean,
    ADD COLUMN override_revision bigint NOT NULL DEFAULT 1 CHECK (override_revision > 0);

-- +goose Down
ALTER TABLE clone_queue
    DROP COLUMN no_images_override,
    DROP COLUMN override_revision,
    DROP COLUMN cover_prompt_id,
    DROP COLUMN scene_prompt_id;
