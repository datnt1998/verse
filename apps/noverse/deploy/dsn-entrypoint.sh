#!/bin/sh
# Builds correctly URI-encoded Postgres DSNs for the LOCAL compose mirror, then
# execs the binary with the given args (serve, storage migrate up, …).
#
# Why: a DSN embeds userinfo inside a URI. Inserting a generated password into
# `postgres://user:PASSWORD@host/db` without percent-encoding changes the DSN's
# meaning as soon as the password contains one of `:@/?#%` or a quote — ordinary
# secret-generator output. The official Postgres image takes the raw password
# for role creation (deploy/postgres-init/01-roles.sh), so the raw value and the
# URI-encoded value must stay one source: the POSTGRES_*_PASSWORD env var here.
#
# In Dokploy this script is not used: the platform prints complete DSNs
# (already percent-encoded) and the operator pastes them into DATABASE_URL /
# MIGRATION_DATABASE_URL. Nothing else reads these component variables.
#
# DSN_TARGETS selects what to build: "app" (default) or "app,migrate". The app
# container must never receive a migration DSN, so it builds only the app one.
set -eu

# percent-encode every byte outside RFC 3986 unreserved (A-Z a-z 0-9 - . _ ~)
uri_encode() {
    printf '%s' "$1" | od -An -tu1 -v | awk '
        {
            for (i = 1; i <= NF; i++) {
                b = $i
                if ((b >= 48 && b <= 57) || (b >= 65 && b <= 90) || (b >= 97 && b <= 122) ||
                    b == 45 || b == 46 || b == 95 || b == 126) {
                    printf "%c", b
                } else {
                    printf "%%%02X", b
                }
            }
        }'
}

dsn() {
    # $1 user, $2 password; host/port/db/sslmode come from the environment.
    printf 'postgres://%s:%s@%s:%s/%s?sslmode=%s' \
        "$(uri_encode "$1")" "$(uri_encode "$2")" \
        "${POSTGRES_HOST:?POSTGRES_HOST is required}" "${POSTGRES_PORT:-5432}" \
        "$(uri_encode "${POSTGRES_DB:?POSTGRES_DB is required}")" \
        "${POSTGRES_SSLMODE:-disable}"
}

targets=",${DSN_TARGETS:-app},"
case "$targets" in
    *,app,*)
        if [ -z "${DATABASE_URL:-}" ]; then
            DATABASE_URL=$(dsn "${POSTGRES_APP_USER:?POSTGRES_APP_USER is required}" "${POSTGRES_APP_PASSWORD:?POSTGRES_APP_PASSWORD is required}")
            export DATABASE_URL
        fi
        ;;
esac
case "$targets" in
    *,migrate,*)
        if [ -z "${MIGRATION_DATABASE_URL:-}" ]; then
            MIGRATION_DATABASE_URL=$(dsn "${POSTGRES_MIGRATE_USER:?POSTGRES_MIGRATE_USER is required}" "${POSTGRES_MIGRATE_PASSWORD:?POSTGRES_MIGRATE_PASSWORD is required}")
            export MIGRATION_DATABASE_URL
        fi
        ;;
    *,worker,*)
        if [ -z "${NOVERSE_WORKER_DATABASE_URL:-}" ]; then
            NOVERSE_WORKER_DATABASE_URL=$(dsn "${POSTGRES_WORKER_USER:-noverse_engine_worker}" "${POSTGRES_WORKER_PASSWORD:?POSTGRES_WORKER_PASSWORD is required}")
            export NOVERSE_WORKER_DATABASE_URL
        fi
        ;;
esac

exec ainovel-cli "$@"
