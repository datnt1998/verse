#!/bin/sh
# HTTPS smoke test for a running deployment (or the local Dokploy mirror).
#
#   PUBLIC_URL=https://noverse.example \
#   SMOKE_USERNAME=admin SMOKE_PASSWORD='…' ./deploy/smoke.sh
#
# Optional:
#   SMOKE_CA_CERT=/path/ca.pem   trust a private/self-signed certificate
#   SMOKE_RESOLVE=host:port:ip   pin name resolution (local mirror)
#
# Requires: curl, jq. Creates one project and one story with a timestamped
# name; it never deletes data (the runbook removes its own artifacts).
set -eu

: "${PUBLIC_URL:?PUBLIC_URL is required, e.g. https://noverse.example}"
: "${SMOKE_USERNAME:?SMOKE_USERNAME is required}"
: "${SMOKE_PASSWORD:?SMOKE_PASSWORD is required}"
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }

CURL="curl -sS"
if [ -n "${SMOKE_CA_CERT:-}" ]; then CURL="$CURL --cacert $SMOKE_CA_CERT"; fi
if [ -n "${SMOKE_RESOLVE:-}" ]; then CURL="$CURL --resolve $SMOKE_RESOLVE"; fi
JAR=$(mktemp "${TMPDIR:-/tmp}/noverse-smoke.XXXXXX")
trap 'rm -f "$JAR"' EXIT
chmod 600 "$JAR"
STAMP=$(date -u +%Y%m%dT%H%M%SZ)

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

# 1. Liveness: no auth, no dependency, JSON (not the SPA index).
code=$($CURL -o /tmp/smoke-health.$$ -w '%{http_code}' "$PUBLIC_URL/healthz")
[ "$code" = 200 ] || fail "/healthz returned $code"
jq -e '.status == "ok"' /tmp/smoke-health.$$ >/dev/null || fail "/healthz body is not ok"
pass "/healthz 200"

# 2. Readiness: database schema version + blob store.
code=$($CURL -o /tmp/smoke-ready.$$ -w '%{http_code}' "$PUBLIC_URL/readyz")
[ "$code" = 200 ] || { cat /tmp/smoke-ready.$$ >&2; fail "/readyz returned $code"; }
jq -e '.status == "ok" and (.schemaVersion != null)' /tmp/smoke-ready.$$ >/dev/null \
    || fail "/readyz body missing status/schemaVersion"
pass "/readyz 200 schemaVersion=$(jq -r '.schemaVersion' /tmp/smoke-ready.$$)"
rm -f /tmp/smoke-health.$$ /tmp/smoke-ready.$$

# 3. Anonymous session access must be rejected.
code=$($CURL -o /dev/null -w '%{http_code}' "$PUBLIC_URL/api/me")
[ "$code" = 401 ] || fail "anonymous /api/me returned $code (want 401)"
pass "anonymous /api/me 401"

# 4. Real login over HTTPS; cookie must be Secure.
code=$($CURL -D /tmp/smoke-login.$$ -c "$JAR" -o /tmp/smoke-login-body.$$ -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg u "$SMOKE_USERNAME" --arg p "$SMOKE_PASSWORD" '{username:$u,password:$p}')" \
    "$PUBLIC_URL/api/login")
[ "$code" = 200 ] || { cat /tmp/smoke-login-body.$$ >&2; fail "login returned $code"; }
grep -qi 'set-cookie: .*Secure' /tmp/smoke-login.$$ || fail "session cookie is not Secure"
pass "login 200 with Secure cookie"

# 5. Create a story through the public API (write path + authorization).
project=$($CURL -b "$JAR" -X POST -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg n "Smoke $STAMP" '{name:$n}')" "$PUBLIC_URL/api/projects" | jq -r '.id // empty')
[ -n "$project" ] || fail "creating a project returned no id"
story=$($CURL -b "$JAR" -X POST -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg n "Smoke chapter $STAMP" '{name:$n}')" \
    "$PUBLIC_URL/api/projects/$project/stories" | jq -r '.id // empty')
[ -n "$story" ] || fail "creating a story returned no id"
pass "created project=$project story=$story"

# 6. Read it back. Story ids are "<project>/<slug>"; the URL path already
# carries the project, so the read uses the slug part.
story_slug=${story##*/}
title=$($CURL -b "$JAR" "$PUBLIC_URL/api/projects/$project/stories/$story_slug/meta" | jq -r '.title // empty')
[ "$title" = "Smoke chapter $STAMP" ] || fail "story read back title=$(printf %s "$title" | head -c 60)"
pass "read story title=$title"

# 7. The worker listener never answers on the public router.
code=$($CURL -o /dev/null -w '%{http_code}' -X POST "$PUBLIC_URL/internal/worker/claim")
[ "$code" = 404 ] || fail "worker path over PUBLIC_URL returned $code (want 404)"
pass "worker path absent from public router (404)"
rm -f /tmp/smoke-login.$$ /tmp/smoke-login-body.$$

echo "SMOKE OK: $PUBLIC_URL"
