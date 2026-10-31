-- +goose Up
-- Project revision is an internal counter; content_revision remains the exact
-- sha256 wire revision. Immutable blob publication and pointer CAS share a tx.
CREATE TABLE editor_projects (
    project_id text PRIMARY KEY CHECK (project_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
    revision bigint NOT NULL CHECK (revision > 0),
    logical_path text NOT NULL REFERENCES blob_refs(logical_path) ON DELETE RESTRICT,
    content_revision text NOT NULL CHECK (content_revision ~ '^sha256:[0-9a-f]{64}$'),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE editor_resources (
    resource_key text PRIMARY KEY CHECK (
        resource_key <> '' AND resource_key !~ '(^/|(^|/)\.{1,2}(/|$)|\\|//|/$)'
        AND resource_key !~ '^projects/[A-Za-z0-9_-]+$'
    ),
    revision bigint NOT NULL CHECK (revision > 0),
    logical_path text NOT NULL REFERENCES blob_refs(logical_path) ON DELETE RESTRICT,
    content_revision text NOT NULL CHECK (content_revision ~ '^sha256:[0-9a-f]{64}$'),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- Deleting a pointer must not resurrect its retained phase-05 clone artifact.
CREATE TABLE editor_deleted_resources (
    resource_key text PRIMARY KEY CHECK (
        resource_key <> '' AND resource_key !~ '(^/|(^|/)\.{1,2}(/|$)|\\|//|/$)'
    ),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX editor_projects_logical_path_idx ON editor_projects(logical_path);
CREATE INDEX editor_resources_logical_path_idx ON editor_resources(logical_path);
CREATE TRIGGER storage_editor_project_fence BEFORE INSERT OR UPDATE OR DELETE ON editor_projects FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_editor_project_journal AFTER INSERT OR UPDATE OR DELETE ON editor_projects FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE TRIGGER storage_editor_resource_fence BEFORE INSERT OR UPDATE OR DELETE ON editor_resources FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_editor_resource_journal AFTER INSERT OR UPDATE OR DELETE ON editor_resources FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();
CREATE TRIGGER storage_editor_deleted_fence BEFORE INSERT OR UPDATE OR DELETE ON editor_deleted_resources FOR EACH STATEMENT EXECUTE FUNCTION storage_publication_fence();
CREATE TRIGGER storage_editor_deleted_journal AFTER INSERT OR UPDATE OR DELETE ON editor_deleted_resources FOR EACH STATEMENT EXECUTE FUNCTION storage_record_mutation();

CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION
 SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION
 SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION
 SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION
 SELECT path, content_type, 'clone' FROM clone_artifacts
 UNION
 SELECT logical_path, content_type, 'library' FROM blob_refs
 WHERE logical_path IN ('library/standard-template/scene-cool.webp', 'library/standard-template/scene-warm.webp', 'library/standard-template/voice-placeholder.wav', 'library/standard-template/snow-loop.mp4')
 UNION
 SELECT p.logical_path, b.content_type, 'editor' FROM editor_projects p JOIN blob_refs b USING (logical_path)
 UNION
 SELECT r.logical_path, b.content_type, 'editor' FROM editor_resources r JOIN blob_refs b USING (logical_path);

-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_app') THEN
  GRANT SELECT, INSERT, UPDATE, DELETE ON editor_projects, editor_resources, editor_deleted_resources TO viderse_app;
 END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Refuse a rollback that would orphan authored resources or lose their CAS head.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM editor_projects) OR EXISTS (SELECT 1 FROM editor_resources) OR EXISTS (SELECT 1 FROM editor_deleted_resources) THEN
  RAISE EXCEPTION 'Refusing to drop populated editor pointers; retain additive schema during rollback';
 END IF;
END $$;
-- +goose StatementEnd
CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION
 SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION
 SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION
 SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION
 SELECT path, content_type, 'clone' FROM clone_artifacts
 UNION
 SELECT logical_path, content_type, 'library' FROM blob_refs
 WHERE logical_path IN ('library/standard-template/scene-cool.webp', 'library/standard-template/scene-warm.webp', 'library/standard-template/voice-placeholder.wav', 'library/standard-template/snow-loop.mp4');
DROP TABLE editor_deleted_resources, editor_resources, editor_projects;
