#!/bin/sh
# Postgres roles for the hosted stack, applied once by the official image's
# init hook. Three principals, matching the phase 09 contract:
#
#   postgres        bootstrap superuser (Dokploy service owner) — never the app
#   noverse_migrate migration role: owns the schema and runs goose DDL; also
#                   the maintenance principal for audit/history retention and
#                   blob/cache garbage collection (`storage purge-audit/gc`)
#   noverse_app     application role: DML only, no CREATE/ALTER/DROP;
#                   migrations 00010/00011 revoke UPDATE/DELETE on history and
#                   DELETE on blobs/export_cache (app marks expiry/tombstones)
#
# The app pool therefore cannot migrate, and the migration role is the only
# principal able to change schema. Passwords arrive as container env vars
# (Dokploy secrets), never baked into the image or committed.
#
# Secrets reach SQL only through psql variables: `:'name'` expands to a properly
# quoted SQL literal (quote_literal semantics) and `:"name"` to a quoted
# identifier. Interpolating them into the SQL text with the shell would break on
# an ordinary generated password containing a quote, and would make a
# compromised secret a superuser SQL injection. Passwords are never restricted
# to a "safe" alphabet — the quoting does the work.
set -eu

: "${POSTGRES_MIGRATE_PASSWORD:?POSTGRES_MIGRATE_PASSWORD is required}"
: "${POSTGRES_APP_PASSWORD:?POSTGRES_APP_PASSWORD is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set=migrate_password="$POSTGRES_MIGRATE_PASSWORD" \
    --set=app_password="$POSTGRES_APP_PASSWORD" \
    --set=database_name="$POSTGRES_DB" <<'SQL'
-- Create each login role only if absent. format(%L) is quote_literal, so the
-- password never has to be SQL-safe text.
SELECT format('CREATE ROLE noverse_migrate LOGIN PASSWORD %L', :'migrate_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_migrate')
\gexec

SELECT format('CREATE ROLE noverse_app LOGIN PASSWORD %L', :'app_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'noverse_app')
\gexec

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
