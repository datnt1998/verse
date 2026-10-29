-- +goose Up
-- Uniform mutable-row guard for the identity/project/blob/library tables
-- created in 00002 and 00005. These tables already had version/updated_at
-- columns; this migration attaches the same enforce_row_version_guard()
-- function that migration 00003 gave the narrative domain and 00007 gave
-- jobs/audio. It is the second line: Go repositories still CAS on the
-- expected version (WHERE version = $n); the trigger rejects a plain UPDATE
-- that forgets to bump version exactly once or leaves updated_at unchanged.
--
-- Deliberate exemptions (documented matrix, see migrations package docs):
--   * noverse_meta          — schema bookkeeping only, no version column.
--   * auth_invariant_lock   — lock-only singleton (SELECT … FOR UPDATE),
--                             never a business CAS target.
-- Append-only history tables are guarded by prevent_immutable_change() in
-- 00003/00005/00007 and are not touched here.

CREATE TRIGGER trg_users_version_guard BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_user_credentials_version_guard BEFORE UPDATE ON user_credentials
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_user_identities_version_guard BEFORE UPDATE ON user_identities
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_temporary_passwords_version_guard BEFORE UPDATE ON temporary_passwords
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_password_reset_tokens_version_guard BEFORE UPDATE ON password_reset_tokens
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_devices_version_guard BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_sessions_version_guard BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_device_pairings_version_guard BEFORE UPDATE ON device_pairings
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_recovery_codes_version_guard BEFORE UPDATE ON recovery_codes
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_login_rate_limits_version_guard BEFORE UPDATE ON login_rate_limits
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_projects_version_guard BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_project_members_version_guard BEFORE UPDATE ON project_members
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_blobs_version_guard BEFORE UPDATE ON blobs
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();
CREATE TRIGGER trg_library_settings_version_guard BEFORE UPDATE ON library_settings
    FOR EACH ROW EXECUTE FUNCTION enforce_row_version_guard();

-- +goose Down
-- Down is for throwaway databases only; the playbook forbids it on any
-- database with writers.
DROP TRIGGER IF EXISTS trg_library_settings_version_guard ON library_settings;
DROP TRIGGER IF EXISTS trg_blobs_version_guard ON blobs;
DROP TRIGGER IF EXISTS trg_project_members_version_guard ON project_members;
DROP TRIGGER IF EXISTS trg_projects_version_guard ON projects;
DROP TRIGGER IF EXISTS trg_login_rate_limits_version_guard ON login_rate_limits;
DROP TRIGGER IF EXISTS trg_recovery_codes_version_guard ON recovery_codes;
DROP TRIGGER IF EXISTS trg_device_pairings_version_guard ON device_pairings;
DROP TRIGGER IF EXISTS trg_sessions_version_guard ON sessions;
DROP TRIGGER IF EXISTS trg_devices_version_guard ON devices;
DROP TRIGGER IF EXISTS trg_password_reset_tokens_version_guard ON password_reset_tokens;
DROP TRIGGER IF EXISTS trg_temporary_passwords_version_guard ON temporary_passwords;
DROP TRIGGER IF EXISTS trg_user_identities_version_guard ON user_identities;
DROP TRIGGER IF EXISTS trg_user_credentials_version_guard ON user_credentials;
DROP TRIGGER IF EXISTS trg_users_version_guard ON users;
