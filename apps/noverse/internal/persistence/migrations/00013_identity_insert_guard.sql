-- +goose Up
-- Only a role that may promote accounts may create them. A sibling-app
-- identity role (viderse_identity) holds INSERT on users so first-run setup
-- works from either app, but no UPDATE on users.global_role. For such a role
-- an INSERT is accepted only while the store has no live account — first-run
-- setup, serialized by the auth_invariant_lock row its caller already holds.
-- Afterwards it cannot mint accounts, admin or member, behind the account
-- owner's back.
--
-- The privilege test reads the trigger's own table (TG_RELID) and the emptiness
-- check is schema-qualified with TG_TABLE_SCHEMA: an unqualified "users" would
-- follow the caller's search_path, where a temporary table could shadow it.
-- The function pins its own search_path for the same reason, so no object the
-- caller can create shadows the built-ins it calls.
-- +goose StatementBegin
CREATE FUNCTION guard_restricted_user_insert()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    taken boolean;
BEGIN
    IF has_column_privilege(TG_RELID, 'global_role', 'UPDATE') THEN
        RETURN NEW;
    END IF;
    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I.users WHERE deleted_at IS NULL)', TG_TABLE_SCHEMA)
        INTO taken;
    IF taken THEN
        RAISE EXCEPTION 'role % may create an account only in an empty account store', current_user
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER trg_users_restricted_insert BEFORE INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION guard_restricted_user_insert();

-- +goose Down
-- Throwaway databases only; removing the guard reopens account creation to
-- every role holding INSERT on users.
DROP TRIGGER IF EXISTS trg_users_restricted_insert ON users;
DROP FUNCTION IF EXISTS guard_restricted_user_insert();
