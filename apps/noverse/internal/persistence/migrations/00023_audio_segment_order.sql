-- +goose Up
-- Segment IDs and render references stay stable while a bundle changes order.
-- Check final ordinal uniqueness at transaction commit, not between row upserts.
ALTER TABLE audio_segments
    DROP CONSTRAINT audio_segments_project_id_chapter_id_script_revision_ordina_key,
    ADD CONSTRAINT audio_segment_ordinal_unique
        UNIQUE (project_id, chapter_id, script_revision, ordinal)
        DEFERRABLE INITIALLY IMMEDIATE;

-- +goose Down
ALTER TABLE audio_segments
    DROP CONSTRAINT audio_segment_ordinal_unique,
    ADD CONSTRAINT audio_segments_project_id_chapter_id_script_revision_ordina_key
        UNIQUE (project_id, chapter_id, script_revision, ordinal);
