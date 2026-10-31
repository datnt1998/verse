-- +goose Up
-- The schema already admits deleting. Retain that row as the unique-key claim
-- until bytes are removed, and forbid new references after the claim commits.
-- +goose StatementBegin
CREATE FUNCTION noverse_guard_blob_attachment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    blob uuid;
    blob_state text;
BEGIN
    blob := (to_jsonb(NEW) ->> TG_ARGV[0])::uuid;
    IF blob IS NULL THEN RETURN NEW; END IF;
    SELECT state INTO blob_state FROM blobs
        WHERE project_id = NEW.project_id AND id = blob FOR KEY SHARE;
    IF blob_state = 'deleting' THEN
        RAISE EXCEPTION 'cannot attach deleting blob %', blob USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER trg_export_cache_blob_guard BEFORE INSERT OR UPDATE OF blob_id ON export_cache
    FOR EACH ROW EXECUTE FUNCTION noverse_guard_blob_attachment('blob_id');
CREATE TRIGGER trg_audio_renders_blob_guard BEFORE INSERT OR UPDATE OF blob_id ON audio_renders
    FOR EACH ROW EXECUTE FUNCTION noverse_guard_blob_attachment('blob_id');
CREATE TRIGGER trg_audio_voices_blob_guard BEFORE INSERT OR UPDATE OF sample_blob_id ON audio_voices
    FOR EACH ROW EXECUTE FUNCTION noverse_guard_blob_attachment('sample_blob_id');
CREATE TRIGGER trg_project_documents_blob_guard BEFORE INSERT OR UPDATE OF blob_id ON project_documents
    FOR EACH ROW EXECUTE FUNCTION noverse_guard_blob_attachment('blob_id');
CREATE TRIGGER trg_story_artifacts_blob_guard BEFORE INSERT OR UPDATE OF blob_id ON story_artifacts
    FOR EACH ROW EXECUTE FUNCTION noverse_guard_blob_attachment('blob_id');

-- The maintenance boundary remains unchanged: no app DELETE privilege.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_app') THEN
        REVOKE DELETE ON blobs, export_cache FROM noverse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER trg_export_cache_blob_guard ON export_cache;
DROP TRIGGER trg_audio_renders_blob_guard ON audio_renders;
DROP TRIGGER trg_audio_voices_blob_guard ON audio_voices;
DROP TRIGGER trg_project_documents_blob_guard ON project_documents;
DROP TRIGGER trg_story_artifacts_blob_guard ON story_artifacts;
DROP FUNCTION noverse_guard_blob_attachment();
