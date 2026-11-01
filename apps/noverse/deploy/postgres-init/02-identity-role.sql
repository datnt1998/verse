-- Viderse identity role: the least-privilege login used by the Viderse app to
-- read/write the shared account tables in the Noverse database.
--
-- Creating a missing login needs a superuser. Before BEGIN the script disables
-- transaction sampling for the session; inside the CREATE ROLE transaction it
-- silences statement/debug logging and activity tracking before any password SQL.
-- Run AFTER `storage migrate up` and BEFORE Viderse starts:
--
--   ( set -a; . /secure/identity-role.env; set +a; \
--       exec psql -X "$IDENTITY_DSN_ADMIN" -v identity_schema=public \
--            -f deploy/postgres-init/02-identity-role.sql )   # env file 0600
--
-- The password comes from IDENTITY_ROLE_PASSWORD (read with \getenv, psql 15+)
-- so it never appears on a command line. Use the environment for automation
-- too, never a password-bearing -v argument. The admin DSN should carry no
-- password either (PGPASSFILE). An existing login needs no password: a schema
-- owner with CREATEROLE and ADMIN OPTION on the login may rerun to sync grants.
-- If the login is missing and no password is supplied, the initdb hook skips
-- provisioning; a supplied empty password is refused.
--
-- Without superuser the creation path stops BEFORE sending a password-bearing
-- statement. Create the role without a password and use \password instead;
-- then rerun as its schema/role administrator to sync grants. Do not LOAD
-- auto_explain/pgaudit in this session; already-loaded modules are silenced
-- transaction-locally. The pre-BEGIN sampling setting is session-scoped; a
-- transaction pooler that chooses another backend is checked again inside the
-- transaction and refused unless that backend also has sampling disabled.
--
-- The role must NOT be a member of noverse_app, noverse_migrate or
-- viderse_migrate, and it gets no default privileges: every right below is
-- deliberate and limited to the account/grant surface. In particular it can
-- never read projects, stories, project_members or audit history, never change
-- users.global_role/status, never issue temporary passwords and never run DDL.
\set ON_ERROR_STOP on
-- Never echo password-bearing SQL back to the terminal, whatever -a, -e or
-- ECHO the caller passed.
\set ECHO none
\set ECHO_HIDDEN off

\if :{?identity_schema}
\else
  \set identity_schema public
\endif

SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = 'viderse_identity')
    AS identity_role_exists
\gset
\if :identity_role_exists
\else
\unset identity_password
\getenv identity_password IDENTITY_ROLE_PASSWORD
\if :{?identity_password}
\else
  \echo 'viderse_identity: IDENTITY_ROLE_PASSWORD not set; skipping role provisioning.'
  \quit
\endif

-- Transaction sampling is chosen at BEGIN, so turning it off transaction-
-- locally would be too late. No password is sent during this preflight.
DO $$
BEGIN
    PERFORM set_config('log_transaction_sample_rate', '0', false);
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'Không thể tắt lấy mẫu nhật ký transaction (%)', SQLERRM;
END
$$;
SELECT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)
    AND current_setting('log_transaction_sample_rate')::float8 = 0
    AS identity_sampling_disabled
\gset
\if :identity_sampling_disabled
\else
DO $$ BEGIN RAISE EXCEPTION 'viderse_identity: không thể tắt lấy mẫu nhật ký — hãy chạy bằng superuser, hoặc tạo role không có mật khẩu rồi đặt bằng \password viderse_identity'; END $$;
\quit
\endif

BEGIN;
DO $$
BEGIN
    PERFORM set_config('track_activities', 'off', true);
    PERFORM set_config('log_statement', 'none', true);
    PERFORM set_config('log_min_duration_statement', '-1', true);
    PERFORM set_config('log_min_duration_sample', '-1', true);
    PERFORM set_config('debug_print_parse', 'off', true);
    PERFORM set_config('debug_print_rewritten', 'off', true);
    PERFORM set_config('debug_print_plan', 'off', true);
    PERFORM set_config('log_min_error_statement', 'panic', true);
    PERFORM set_config('pg_stat_statements.track', 'none', true);
    PERFORM set_config('pg_stat_statements.track_utility', 'off', true);
    PERFORM set_config('auto_explain.log_min_duration', '-1', true);
    PERFORM set_config('pgaudit.log', 'none', true);
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'Không thể tắt ghi nhật ký câu lệnh (%)', SQLERRM;
END
$$;

-- Recheck the sampling decision on this transaction's backend, plus every
-- local logging/activity knob, before sending any password-bearing SQL. Bind
-- parameters are not an alternative: logging records their values too.
SELECT
    (SELECT rolsuper FROM pg_roles WHERE rolname = current_user) AND
    current_setting('log_transaction_sample_rate')::float8 = 0
    AND current_setting('track_activities') = 'off'
    AND current_setting('log_statement') = 'none'
    AND current_setting('log_min_duration_statement') = '-1'
    AND current_setting('log_min_duration_sample') = '-1'
    AND current_setting('debug_print_parse') = 'off'
    AND current_setting('debug_print_rewritten') = 'off'
    AND current_setting('debug_print_plan') = 'off'
    AND current_setting('log_min_error_statement') = 'panic'
    AND current_setting('pg_stat_statements.track', true) = 'none'
    AND current_setting('pg_stat_statements.track_utility', true) = 'off'
    AND current_setting('auto_explain.log_min_duration', true) = '-1'
    AND current_setting('pgaudit.log', true) = 'none'
    AS identity_logging_silenced
\gset
\if :identity_logging_silenced
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'viderse_identity: không thể tắt ghi nhật ký câu lệnh — hãy chạy bằng superuser, hoặc tạo role không có mật khẩu rồi đặt bằng \password viderse_identity'; END $$;
\quit
\endif

SELECT :'identity_password' <> '' AS identity_password_nonempty
\gset
\if :identity_password_nonempty
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'viderse_identity: IDENTITY_ROLE_PASSWORD rỗng — từ chối tạo login không có mật khẩu'; END $$;
\quit
\endif

-- Both this SELECT and the generated CREATE contain the password; the literal
-- quoting handles generated passwords with arbitrary quotes and whitespace.
SELECT format(
    'CREATE ROLE viderse_identity LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD %L',
    :'identity_password')
\gexec
COMMIT;
\endif

-- SUPERUSER/CREATEDB changes require the corresponding authority even when
-- already false. Only request privileged demotions that are actually needed;
-- an unsafe existing login still fails closed before grants.
SELECT 'ALTER ROLE viderse_identity NOSUPERUSER'
FROM pg_roles WHERE rolname = 'viderse_identity' AND rolsuper
\gexec
SELECT 'ALTER ROLE viderse_identity NOCREATEDB'
FROM pg_roles WHERE rolname = 'viderse_identity' AND rolcreatedb
\gexec
ALTER ROLE viderse_identity LOGIN NOCREATEROLE NOINHERIT;

-- Leave any application/migration role before granting anything. NOINHERIT
-- already disables inheritance, this removes the membership itself.
DO $$
DECLARE
    other text;
BEGIN
    FOREACH other IN ARRAY ARRAY['noverse_app', 'noverse_migrate', 'viderse_migrate'] LOOP
        IF EXISTS (
            SELECT FROM pg_auth_members m
            JOIN pg_roles r ON r.oid = m.roleid
            JOIN pg_roles member_role ON member_role.oid = m.member
            WHERE r.rolname = other AND member_role.rolname = 'viderse_identity'
        ) THEN
            EXECUTE format('REVOKE %I FROM viderse_identity', other);
        END IF;
    END LOOP;
END
$$;

-- Converge on rerun: drop every stale object grant first, then grant exactly
-- the contract below.
SELECT format('REVOKE ALL ON DATABASE %I FROM viderse_identity', current_database())
\gexec
REVOKE ALL ON SCHEMA :"identity_schema" FROM viderse_identity;
REVOKE ALL ON ALL TABLES IN SCHEMA :"identity_schema" FROM viderse_identity;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA :"identity_schema" FROM viderse_identity;

-- Connect to the identity database and resolve unqualified account tables only
-- through the identity schema.
SELECT format('GRANT CONNECT ON DATABASE %I TO viderse_identity', current_database())
\gexec
GRANT USAGE ON SCHEMA :"identity_schema" TO viderse_identity;

-- Account rows: read for login/projection, insert for first-run setup, and
-- UPDATE only on the columns the identity flow owns. No UPDATE on global_role,
-- status, deleted_at, display_name or onboarding columns. Lacking UPDATE on
-- global_role is also what the users insert guard (migration
-- 00013_identity_insert_guard) keys on: this role inserts only into an empty
-- account store.
GRANT SELECT, INSERT ON TABLE :"identity_schema".users TO viderse_identity;
GRANT UPDATE (must_change_password, updated_at, version)
    ON TABLE :"identity_schema".users TO viderse_identity;

-- Credentials are written on password change and first-admin setup; devices,
-- sessions and known-device tokens are created/touched by every login and
-- revoked (never deleted) whenever the account's sessions are revoked.
GRANT SELECT, INSERT, UPDATE ON TABLE :"identity_schema".user_credentials,
    :"identity_schema".devices, :"identity_schema".sessions,
    :"identity_schema".device_tokens, :"identity_schema".recovery_codes
    TO viderse_identity;

-- Temporary passwords and QR pairings are consumed/revoked, never issued by
-- Viderse in this scope (no INSERT).
GRANT SELECT, UPDATE ON TABLE :"identity_schema".temporary_passwords,
    :"identity_schema".device_pairings TO viderse_identity;

-- The singleton invariant row is locked (SELECT … FOR UPDATE) by first-admin
-- setup and grant mutations.
GRANT SELECT, UPDATE ON TABLE :"identity_schema".auth_invariant_lock
    TO viderse_identity;

-- Shared login/recovery rate-limit buckets and per-app grant rows are fully
-- mutable by the account flow; both are identity-only tables.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE :"identity_schema".login_rate_limits,
    :"identity_schema".app_grants TO viderse_identity;

-- Audit is append-only for this role, and only through the account/grant
-- column list; project-scoped audit and audit history stay unreadable.
GRANT INSERT (id, actor_user_id, actor_kind, action, target_kind, target_id,
    request_id, details, occurred_at)
    ON TABLE :"identity_schema".audit_logs TO viderse_identity;

-- audit_logs.sequence is generated by an identity sequence. Column INSERT
-- alone does not require sequence privileges, but grant USAGE explicitly so a
-- later non-identity default keeps working. Resolve the real name through the
-- catalog instead of hardcoding it.
SELECT format('GRANT USAGE ON SEQUENCE %s TO viderse_identity',
              pg_get_serial_sequence(format('%I.audit_logs', :'identity_schema'), 'sequence'))
WHERE pg_get_serial_sequence(format('%I.audit_logs', :'identity_schema'), 'sequence') IS NOT NULL
\gexec
