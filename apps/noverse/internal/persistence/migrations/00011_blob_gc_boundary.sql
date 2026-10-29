-- +goose Up
-- created_at is the stage timestamp; marking ready updates updated_at so a
-- long-running upload starts a fresh ready-blob grace window. The nullable job
-- link covers worker/export uploads; synchronous imports/assets have no job.
ALTER TABLE blobs ADD COLUMN upload_job_id uuid;
ALTER TABLE blobs ADD CONSTRAINT blobs_upload_job_fk
    FOREIGN KEY (project_id, upload_job_id) REFERENCES jobs(project_id, id) ON DELETE RESTRICT;
CREATE INDEX blobs_staged_age ON blobs (created_at) WHERE state = 'staged';

-- Production defaults grant DML on new tables. Physical blob/cache removal is
-- maintenance-only; application invalidation is an UPDATE of expires_at/state.
-- Re-assert history exceptions for upgraded installations as defense in depth.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_app') THEN
        REVOKE DELETE ON blobs, export_cache FROM noverse_app;
        REVOKE UPDATE, DELETE ON audit_logs, chapter_versions, import_items FROM noverse_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Throwaway databases only: restore the pre-00011 boundary.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_app') THEN
        GRANT DELETE ON blobs, export_cache TO noverse_app;
    END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX blobs_staged_age;
ALTER TABLE blobs DROP CONSTRAINT blobs_upload_job_fk;
ALTER TABLE blobs DROP COLUMN upload_job_id;
