-- +goose Up
-- Audit/history mutability matrix (S13). Per-table classification, applied to
-- the shared prevent_immutable_change():
--
--   HISTORY (append-only to the application; purge/retention only under the
--   maintenance principal — noverse_migrate or a superuser):
--     audit_logs, chapter_versions, import_items
--
--   DOCUMENT-RESETTABLE ENGINE STREAMS (the document backend owns tail repair
--   and reset for file parity; the transaction marker remains sufficient):
--     story_events, checkpoints, agent_sessions, job_events
--
-- The escape hatch is no longer a caller-settable string alone: history
-- mutations require BOTH `noverse.allow_immutable_write = 'on'` AND an actual
-- privileged principal. The application role additionally loses UPDATE/DELETE
-- on the history tables, so even a compromised session cannot purge audit or
-- rewrite accepted chapter versions. Reads never delete anything; audit
-- retention is an explicit `storage purge-audit` maintenance command.
--
-- noverse_meta (bookkeeping) and auth_invariant_lock (lock-only singleton) are
-- exempt from business CAS; they are not history tables.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION noverse_is_maintenance()
RETURNS boolean
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE((SELECT rolsuper FROM pg_roles WHERE rolname = current_user), false)
        OR CASE
               WHEN to_regrole('noverse_migrate') IS NULL THEN false
               ELSE pg_has_role(current_user, 'noverse_migrate', 'member')
           END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    is_history boolean := TG_TABLE_NAME IN ('audit_logs', 'chapter_versions', 'import_items');
BEGIN
    IF is_history THEN
        -- Privilege is the gate; the marker only says "this transaction is a
        -- deliberate maintenance run", never "trust me".
        IF noverse_is_maintenance()
           AND current_setting('noverse.allow_immutable_write', true) = 'on' THEN
            IF TG_OP = 'DELETE' THEN
                RETURN OLD;
            END IF;
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'bảng lịch sử % chỉ được sửa bởi vai trò bảo trì (%)', TG_TABLE_NAME, TG_OP
            USING ERRCODE = 'raise_exception';
    END IF;

    IF current_setting('noverse.allow_immutable_write', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'append-only table % does not accept %', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'raise_exception';
END;
$$;
-- +goose StatementEnd

-- Defense in depth: the app role must not even attempt history mutation. The
-- migration role owns the tables (and is the supported purge principal).
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'noverse_app') THEN
        EXECUTE 'REVOKE UPDATE, DELETE ON audit_logs, chapter_versions, import_items FROM noverse_app';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Down is for throwaway databases only.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'noverse_app') THEN
        EXECUTE 'GRANT UPDATE, DELETE ON audit_logs, chapter_versions, import_items TO noverse_app';
    END IF;
END
$$;
-- +goose StatementEnd
DROP FUNCTION IF EXISTS noverse_is_maintenance();
-- Restore the pre-00010 escape hatch (marker only) so a throwaway down still
-- leaves a working schema.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF current_setting('noverse.allow_immutable_write', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'append-only table % does not accept %', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'raise_exception';
END;
$$;
-- +goose StatementEnd
