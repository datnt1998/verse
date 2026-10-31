-- Noverse engine-worker role: the least-privilege login handed to hosted
-- worker child processes (NOVERSE_WORKER_DATABASE_URL). A worker opens facts
-- (narrative/document/checkpoint tables) and self-claims its story lease —
-- it must never reach account/grant/audit/job/blob rows or run DDL.
--
-- Run as the schema owner (or a superuser) AFTER `storage migrate up` and
-- BEFORE enabling hosted stories:
--
--   psql "$ADMIN_DSN" -v worker_password="$NOVERSE_WORKER_PASSWORD" \
--        -v worker_schema=public -f deploy/postgres-init/03-engine-worker-role.sql
--
-- worker_password is required; without it the script skips itself so the
-- PostgreSQL image's docker-entrypoint-initdb.d hook can still run this
-- directory safely before an operator provisions the role (init runs before
-- migrations exist, so this file must be re-run after migrate anyway).
--
-- The role is NOINHERIT and NOT a member of noverse_app/noverse_migrate;
-- every right below is deliberate. The grant list mirrors the tables the
-- narrative backend actually touches (source 00003_narrative.sql); when the
-- schema changes, update both this file and the worker-role test — new
-- tables are never covered by default privileges on purpose.
\set ON_ERROR_STOP on

\if :{?worker_password}
\else
  \echo 'noverse_engine_worker: worker_password not set; skipping role provisioning.'
  \quit
\endif

\if :{?worker_schema}
\else
  \set worker_schema public
\endif

-- Create the login only if absent. format(%L) is quote_literal, so a generated
-- password containing quotes is safe and never needs to be shell-escaped.
SELECT format(
    'CREATE ROLE noverse_engine_worker LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD %L',
    :'worker_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_engine_worker')
\gexec

-- Enforce the attributes even on an existing role; rerunning never widens them.
ALTER ROLE noverse_engine_worker LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;

-- Leave any application/migration role before granting anything. NOINHERIT
-- already disables inheritance; this removes the membership itself.
DO $$
DECLARE
    other text;
BEGIN
    FOREACH other IN ARRAY ARRAY['noverse_app', 'noverse_migrate', 'viderse_identity'] LOOP
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = other) THEN
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
