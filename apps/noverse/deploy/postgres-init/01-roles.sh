#!/bin/sh
# Postgres roles for the hosted stack, applied once by the official image's
# init hook. Three principals with separate ownership and runtime privileges:
#
#   postgres        bootstrap superuser (Dokploy service owner) — never the app
#   noverse_migrate migration role: owns the schema and runs goose DDL; also
#                   the maintenance principal for audit/history retention and
#                   blob/cache garbage collection (`storage purge-audit/gc`)
#   noverse_app     application role: DML only, no CREATE/ALTER/DROP;
#                   migrations 00010/00011 revoke UPDATE/DELETE on history and
#                   DELETE on blobs/export_cache (app marks expiry/tombstones)
#
# Secrets enter psql through \getenv (psql 15+), never argv. `:'name'` expands to a
# quoted SQL literal (quote_literal semantics) and `:"name"` to a quoted
# identifier. Interpolating them into the SQL text with the shell would break on
# an ordinary generated password containing a quote, and would make a
# compromised secret a superuser SQL injection. Passwords are never restricted
# to a "safe" alphabet — the quoting does the work.
#
# CREATE ROLE … PASSWORD and its SELECT contain plaintext. Creating a missing
# login requires superuser: transaction sampling is disabled session-wide BEFORE
# BEGIN, while statement/sample/debug/extension logging and activity tracking
# are disabled locally inside the CREATE transaction. The transaction rechecks
# sampling on its own backend so transaction poolers fail closed if sampled.
# Existing roles skip all password/silencing SQL; only grants are synchronized.
set -eu

: "${POSTGRES_MIGRATE_PASSWORD:?POSTGRES_MIGRATE_PASSWORD bắt buộc và không được rỗng}"
: "${POSTGRES_APP_PASSWORD:?POSTGRES_APP_PASSWORD bắt buộc và không được rỗng}"
: "${POSTGRES_DB:?POSTGRES_DB bắt buộc}"
export POSTGRES_MIGRATE_PASSWORD POSTGRES_APP_PASSWORD

psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set=database_name="$POSTGRES_DB" <<'SQL'
\set ON_ERROR_STOP on
\set ECHO none
\set ECHO_HIDDEN off
\getenv migrate_password POSTGRES_MIGRATE_PASSWORD
\getenv app_password POSTGRES_APP_PASSWORD

SELECT roles.*, NOT (noverse_migrate_exists AND noverse_app_exists)
    AS roles_need_password
FROM (
    SELECT
        EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_migrate')
            AS noverse_migrate_exists,
        EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_app')
            AS noverse_app_exists
) AS roles
\gset

\if :roles_need_password
-- Sampling is chosen at BEGIN. Changing it locally inside the transaction
-- cannot undo that decision, so disable it for this session first.
DO $$
BEGIN
    PERFORM set_config('log_transaction_sample_rate', '0', false);
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'Không thể tắt lấy mẫu nhật ký transaction (%)', SQLERRM;
END
$$;
SELECT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)
    AND current_setting('log_transaction_sample_rate')::float8 = 0
    AS roles_sampling_disabled
\gset
\if :roles_sampling_disabled
\else
DO $$ BEGIN RAISE EXCEPTION 'Không thể tắt lấy mẫu nhật ký — hãy chạy bằng superuser (init hook đã dùng superuser), hoặc tạo role không có mật khẩu rồi đặt bằng \password'; END $$;
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

-- Check the transaction's backend before sending any password-bearing SQL.
-- Query parameters do not help: statement logging records their values too.
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
    AS roles_logging_silenced
\gset
\if :roles_logging_silenced
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'Không thể tắt ghi nhật ký câu lệnh — hãy chạy bằng superuser (init hook đã dùng superuser), hoặc tạo role không có mật khẩu rồi đặt bằng \password'; END $$;
\quit
\endif

-- The shell's :? guard already refuses empty POSTGRES_*_PASSWORD. Keep the
-- same refusal when the SQL is pasted by hand, without logging the values.
\if :noverse_migrate_exists
\else
SELECT :'migrate_password' <> '' AS migrate_password_nonempty
\gset
\if :migrate_password_nonempty
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'Mật khẩu noverse_migrate rỗng — từ chối tạo login không có mật khẩu'; END $$;
\quit
\endif
SELECT format('CREATE ROLE noverse_migrate LOGIN PASSWORD %L', :'migrate_password')
\gexec
\endif

\if :noverse_app_exists
\else
SELECT :'app_password' <> '' AS app_password_nonempty
\gset
\if :app_password_nonempty
\else
ROLLBACK;
DO $$ BEGIN RAISE EXCEPTION 'Mật khẩu noverse_app rỗng — từ chối tạo login không có mật khẩu'; END $$;
\quit
\endif
SELECT format('CREATE ROLE noverse_app LOGIN PASSWORD %L', :'app_password')
\gexec
\endif
COMMIT;
\endif

-- The migration role owns the schema: CREATE EXTENSION (trusted: citext,
-- pg_trgm) and all tables it creates belong to it, not to the app.
-- Extensions are cluster-level objects: create them once as the bootstrap
-- superuser so the migration role never needs superuser (its migrations still
-- run `CREATE EXTENSION IF NOT EXISTS`, which is then a no-op).
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

ALTER SCHEMA public OWNER TO noverse_migrate;

GRANT CONNECT ON DATABASE :"database_name" TO noverse_migrate, noverse_app;
GRANT USAGE ON SCHEMA public TO noverse_app;

-- Future objects created by the migration role (including goose_db_version and
-- every table from migrations/*.sql) get DML by default. Migrations 00010/00011
-- revoke history UPDATE/DELETE and blob/cache DELETE as per-table exceptions.
ALTER DEFAULT PRIVILEGES FOR ROLE noverse_migrate IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO noverse_app;
ALTER DEFAULT PRIVILEGES FOR ROLE noverse_migrate IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO noverse_app;

-- The app must never run DDL. Explicitly no CREATE on schema public:
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
SQL
