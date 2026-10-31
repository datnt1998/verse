-- +goose Up
CREATE TABLE clone_cutouts (
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    original_sha256 text NOT NULL CHECK (original_sha256 ~ '^[0-9a-f]{64}$'),
    logical_path text NOT NULL REFERENCES blob_refs(logical_path) ON DELETE RESTRICT,
    PRIMARY KEY (job_id, original_sha256),
    CHECK (logical_path ~ ('^clone/cutouts/' || original_sha256 || '/[0-9a-f]{64}\.png$'))
);
CREATE INDEX clone_cutouts_original_idx ON clone_cutouts(original_sha256);
CREATE TRIGGER storage_clone_cutout_fence BEFORE INSERT OR UPDATE OR DELETE ON clone_cutouts FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_clone_cutout_journal AFTER INSERT OR UPDATE OR DELETE ON clone_cutouts FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION SELECT path, content_type, 'clone' FROM clone_artifacts
 UNION SELECT logical_path, content_type, 'library' FROM blob_refs WHERE logical_path IN ('library/standard-template/scene-cool.webp', 'library/standard-template/scene-warm.webp', 'library/standard-template/voice-placeholder.wav', 'library/standard-template/snow-loop.mp4')
 UNION SELECT p.logical_path, b.content_type, 'editor' FROM editor_projects p JOIN blob_refs b USING (logical_path)
 UNION SELECT r.logical_path, b.content_type, 'editor' FROM editor_resources r JOIN blob_refs b USING (logical_path)
 UNION SELECT c.logical_path, b.content_type, 'clone' FROM clone_cutouts c JOIN blob_refs b USING (logical_path);
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
  GRANT SELECT, INSERT, UPDATE, DELETE ON clone_cutouts TO viderse_app;
 END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM clone_cutouts) THEN RAISE EXCEPTION 'Refusing to drop populated cutout index'; END IF;
END $$;
-- +goose StatementEnd
CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION SELECT path, content_type, 'clone' FROM clone_artifacts
 UNION SELECT logical_path, content_type, 'library' FROM blob_refs WHERE logical_path IN ('library/standard-template/scene-cool.webp', 'library/standard-template/scene-warm.webp', 'library/standard-template/voice-placeholder.wav', 'library/standard-template/snow-loop.mp4')
 UNION SELECT p.logical_path, b.content_type, 'editor' FROM editor_projects p JOIN blob_refs b USING (logical_path)
 UNION SELECT r.logical_path, b.content_type, 'editor' FROM editor_resources r JOIN blob_refs b USING (logical_path);
DROP TABLE clone_cutouts;
