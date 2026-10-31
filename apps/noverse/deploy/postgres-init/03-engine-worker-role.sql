-- Noverse engine-worker role: the least-privilege login handed to hosted
-- worker child processes (NOVERSE_WORKER_DATABASE_URL). A worker opens facts
-- (narrative/document/checkpoint tables) and self-claims its story lease —
-- it must never reach account/grant/audit/job/blob rows or run DDL.
--
-- Creating a missing login needs a superuser. Before BEGIN the script disables
-- transaction sampling for the session; inside the CREATE ROLE transaction it
-- silences statement/debug logging and activity tracking before any password SQL.
-- Run AFTER `storage migrate up` and BEFORE enabling hosted stories:
--
--   ( set -a; . /secure/worker-role.env; set +a; \
--       exec psql -X "$ADMIN_DSN" -v worker_schema=public \
--            -f deploy/postgres-init/03-engine-worker-role.sql )   # env file 0600
--
-- The password comes from WORKER_ROLE_PASSWORD (read with \getenv, psql 15+)
-- so it never appears on a command line. Use the environment for automation
-- too, never a password-bearing -v argument. An existing login needs no
-- password: a schema owner with CREATEROLE and ADMIN OPTION on the login may
-- rerun to sync grants. If the login is missing and no password is supplied,
-- the initdb hook skips provisioning (migrations are not available there yet).
-- A supplied empty password is refused. The env name is deliberately not
-- passed by the compose file, so initdb skips a missing login safely.
--
-- Without superuser the creation path stops BEFORE sending a password-bearing
-- statement. Create the role without a password and use \password instead;
-- then rerun as its schema/role administrator to sync grants. Do not LOAD
-- auto_explain/pgaudit in this session; already-loaded modules are silenced
-- transaction-locally. The pre-BEGIN sampling setting is session-scoped; a
-- transaction pooler that chooses another backend is checked again inside the
-- transaction and refused unless that backend also has sampling disabled.
--
-- The role is NOINHERIT and NOT a member of noverse_app/noverse_migrate;
-- every right below is deliberate. The grant list mirrors the tables the
-- narrative backend actually touches (source 00003_narrative.sql); when the
-- schema changes, update both this file and the worker-role test — new
-- tables are never covered by default privileges on purpose.
\set ON_ERROR_STOP on
-- Never echo password-bearing SQL back to the terminal, whatever -a, -e or
-- ECHO the caller passed.
\set ECHO none
\set ECHO_HIDDEN off

\if :{?worker_schema}
\else
  \set worker_schema public
\endif

SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_engine_worker')
    AS worker_role_exists
\gset
\if :worker_role_exists
\else
\unset worker_password
\getenv worker_password WORKER_ROLE_PASSWORD
\if :{?worker_password}
\else
  \echo 'noverse_engine_worker: WORKER_ROLE_PASSWORD not set; skipping role provisioning.'
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
    AS worker_sampling_disabled
\gset
\if :worker_sampling_disabled
\else
DO $$ BEGIN RAISE EXCEPTION 'noverse_engine_worker: không thể tắt lấy mẫu nhật ký — hãy chạy bằng superuser, hoặc tạo role không có mật khẩu rồi đặt bằng \password noverse_engine_worker'; END $$;
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
    AS worker_logging_silenced
\gset
\if :worker_logging_silenced
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'noverse_engine_worker: không thể tắt ghi nhật ký câu lệnh — hãy chạy bằng superuser, hoặc tạo role không có mật khẩu rồi đặt bằng \password noverse_engine_worker'; END $$;
\quit
\endif

SELECT :'worker_password' <> '' AS worker_password_nonempty
\gset
\if :worker_password_nonempty
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'noverse_engine_worker: WORKER_ROLE_PASSWORD rỗng — từ chối tạo login không có mật khẩu'; END $$;
\quit
\endif

-- Both this SELECT and the generated CREATE contain the password; the literal
-- quoting handles generated passwords with arbitrary quotes and whitespace.
SELECT format(
    'CREATE ROLE noverse_engine_worker LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD %L',
    :'worker_password')
\gexec
COMMIT;
\endif

-- SUPERUSER/CREATEDB changes require the corresponding authority even when
-- already false. Only request privileged demotions that are actually needed;
-- an unsafe existing login still fails closed before grants.
SELECT 'ALTER ROLE noverse_engine_worker NOSUPERUSER'
FROM pg_roles WHERE rolname = 'noverse_engine_worker' AND rolsuper
\gexec
SELECT 'ALTER ROLE noverse_engine_worker NOCREATEDB'
FROM pg_roles WHERE rolname = 'noverse_engine_worker' AND rolcreatedb
\gexec
ALTER ROLE noverse_engine_worker LOGIN NOCREATEROLE NOINHERIT;

-- Leave any application/migration role before granting anything. NOINHERIT
-- already disables inheritance; this removes the membership itself.
DO $$
DECLARE
    other text;
BEGIN
    FOREACH other IN ARRAY ARRAY['noverse_app', 'noverse_migrate', 'viderse_identity'] LOOP
        IF EXISTS (
            SELECT FROM pg_auth_members m
            JOIN pg_roles r ON r.oid = m.roleid
            JOIN pg_roles member_role ON member_role.oid = m.member
            WHERE r.rolname = other AND member_role.rolname = 'noverse_engine_worker'
        ) THEN
            EXECUTE format('REVOKE %I FROM noverse_engine_worker', other);
        END IF;
    END LOOP;
END
$$;

-- Converge on rerun: drop every stale object grant first, then grant exactly
-- the contract below.
SELECT format('REVOKE ALL ON DATABASE %I FROM noverse_engine_worker', current_database())
\gexec
REVOKE ALL ON SCHEMA :"worker_schema" FROM noverse_engine_worker;
REVOKE ALL ON ALL TABLES IN SCHEMA :"worker_schema" FROM noverse_engine_worker;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA :"worker_schema" FROM noverse_engine_worker;

SELECT format('GRANT CONNECT ON DATABASE %I TO noverse_engine_worker', current_database())
\gexec
GRANT USAGE ON SCHEMA :"worker_schema" TO noverse_engine_worker;

-- Scope resolution and OpenPool's schema-version check are read-only.
GRANT SELECT ON TABLE :"worker_schema".projects,
    :"worker_schema".noverse_meta, :"worker_schema".goose_db_version
    TO noverse_engine_worker;
-- The worker refreshes story statistics/locks on the story it leased.
GRANT SELECT, UPDATE ON TABLE :"worker_schema".stories TO noverse_engine_worker;

-- Lease ownership: a child claims, renews and releases the lease itself.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE :"worker_schema".story_leases
    TO noverse_engine_worker;

-- Facts pipeline: the narrative backend reads and writes these end to end.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
    :"worker_schema".project_documents, :"worker_schema".volumes,
    :"worker_schema".chapters, :"worker_schema".outlines,
    :"worker_schema".characters, :"worker_schema".comments,
    :"worker_schema".comment_replies, :"worker_schema".reviews,
    :"worker_schema".review_decisions, :"worker_schema".story_state,
    :"worker_schema".story_artifacts, :"worker_schema".story_events,
    :"worker_schema".checkpoints, :"worker_schema".agent_sessions
    TO noverse_engine_worker;

-- History is append-only for every principal: the worker writes new versions
-- but must never UPDATE/DELETE chapter_versions.
GRANT SELECT, INSERT ON TABLE :"worker_schema".chapter_versions
    TO noverse_engine_worker;

-- Intentionally absent: account tables (users, user_identities,
-- user_credentials, sessions, devices, device_pairings, app_grants,
-- audit_logs, auth_invariant_lock, login_rate_limits, recovery_codes,
-- password_reset_tokens, temporary_passwords), app-service tables
-- (project_members, jobs, job_events, blobs, workers, audio_*,
-- import_runs, import_items, library_settings, export_cache), DDL,
-- sequences (facts keys are UUIDs) and default privileges — a future
-- table the worker needs must be granted here deliberately.
