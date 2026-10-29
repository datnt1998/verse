#!/bin/sh
# Nightly off-VPS backup for the hosted deployment.
#
# One run publishes one immutable set of files under the object-key scheme
# owned by internal/blobkey, all timestamps UTC:
#
#   {env}/backups/pg/YYYY/MM/DD/<ts>.dump          pg_dump -Fc, whole database
#   {env}/backups/pg/YYYY/MM/DD/<ts>.blobs.tsv     object_key TAB sha256 TAB size_bytes
#   {env}/backups/pg/YYYY/MM/DD/<ts>.schema.txt    schema.format + highest migration
#   {env}/backups/pg/YYYY/MM/DD/<ts>.SHA256SUMS    checksums of the files above
#
# {env} is STORAGE_ENV (dev|staging|prod; required, never defaulted). The prefix
# is the same one `storage gc` uses for isolation, so a backup object can never
# be mistaken for a blob and another environment's backups are never touched.
#
# Upload target is an S3-compatible bucket OUTSIDE the VPS. Provider-side
# object versioning / immutable retention is configured on the bucket, not
# here. A database backup alone never covers a local blob volume — set
# BLOB_LOCAL_ROOT when STORAGE_DRIVER=local and this script archives it too.
#
# Required:
#   DATABASE_URL        read-only/migration-role DSN (pg_dump needs read access)
#   S3_BACKUP_BUCKET    destination bucket
# Required:
#   STORAGE_ENV         dev|staging|prod — first key segment. Deliberately has
#                       no default here: a backup written into a guessed
#                       environment prefix is worse than a failed run. (The app
#                       and `storage gc` treat an empty value as dev.)
#   S3_REGION           region (required for uploads; auto for R2, us-east-1
#                       for GCS/MinIO)
#   S3_ENDPOINT         S3-compatible endpoint (omit for AWS default)
#   S3_ACCESS_KEY_ID / S3_SECRET_ACCESS_KEY / S3_SESSION_TOKEN
#   BLOB_LOCAL_ROOT     local blob volume path, archived when set
#   S3_BACKUP_DEST_DIR  local directory for a restore drill (skips upload)
#
# Only AWS CLI v2 is used to upload (one tool, one credential source): the
# S3_* contract is mapped onto AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY /
# AWS_SESSION_TOKEN, --region and --endpoint-url, and AWS_PROFILE is removed so
# a profile installed on the host can never select a different principal.
#
# Cron (on the VPS, 03:15 UTC — CRON_TZ makes the clock explicit):
#   CRON_TZ=UTC
#   15 3 * * * cd /opt/noverse && set -a && . ./backup.env && set +a \
#     && ./deploy/backup.sh >> /var/log/noverse-backup.log 2>&1
#
# Exit codes: 0 = dumped, checksummed and uploaded; non-zero = nothing usable
# was published (the script uploads only after every step succeeds).
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${S3_BACKUP_BUCKET:?S3_BACKUP_BUCKET is required}"

STORAGE_ENV=${STORAGE_ENV:-}
case "$STORAGE_ENV" in
    dev|staging|prod) ;;
    "")
        echo "[backup] STORAGE_ENV is required (dev|staging|prod); refusing to guess an environment prefix" >&2
        exit 1
        ;;
    *)
        echo "[backup] STORAGE_ENV='$STORAGE_ENV' is invalid (dev|staging|prod)" >&2
        exit 1
        ;;
esac

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
DAY=$(date -u +%Y/%m/%d)
PREFIX="$STORAGE_ENV/backups/pg/$DAY"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/noverse-backup.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM
OUT="$WORK"
mkdir -p "$OUT"

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$@"
    else
        shasum -a 256 "$@"
    fi
}

echo "[backup] $PREFIX/$STAMP: dumping database"
pg_dump "$DATABASE_URL" --format=custom --no-owner --no-privileges --file="$OUT/$STAMP.dump"

echo "[backup] $PREFIX/$STAMP: blob manifest"
psql "$DATABASE_URL" -At -F '	' -c \
    "SELECT object_key, sha256, size_bytes FROM blobs WHERE state = 'ready' ORDER BY object_key" \
    >"$OUT/$STAMP.blobs.tsv"

psql "$DATABASE_URL" -At -c \
    "SELECT 'schema.format=' || COALESCE((SELECT value FROM noverse_meta WHERE key = 'schema.format'), 'missing')" \
    >"$OUT/$STAMP.schema.txt"
psql "$DATABASE_URL" -At -c \
    "SELECT 'migration.version=' || COALESCE((SELECT max(version_id)::text FROM goose_db_version WHERE is_applied), '0')" \
    >>"$OUT/$STAMP.schema.txt"

if [ -n "${BLOB_LOCAL_ROOT:-}" ]; then
    if [ -d "$BLOB_LOCAL_ROOT" ]; then
        echo "[backup] $PREFIX/$STAMP: archiving local blob volume"
        tar -czf "$OUT/$STAMP.blobs-local.tgz" -C "$(dirname "$BLOB_LOCAL_ROOT")" "$(basename "$BLOB_LOCAL_ROOT")"
    else
        echo "[backup] BLOB_LOCAL_ROOT=$BLOB_LOCAL_ROOT does not exist — refusing a partial backup" >&2
        exit 1
    fi
fi

# Checksums cover exactly the payload files (never the checksum file itself).
(
    cd "$OUT"
    set -- "$STAMP.dump" "$STAMP.blobs.tsv" "$STAMP.schema.txt"
    if [ -f "$STAMP.blobs-local.tgz" ]; then
        set -- "$@" "$STAMP.blobs-local.tgz"
    fi
    sha256 "$@" >"$STAMP.SHA256SUMS"
    sha256 -c "$STAMP.SHA256SUMS" >/dev/null
)

if [ -n "${S3_BACKUP_DEST_DIR:-}" ]; then
    TARGET="$S3_BACKUP_DEST_DIR/$PREFIX"
    mkdir -p "$TARGET"
    cp -R "$OUT"/. "$TARGET"/
    echo "[backup] $PREFIX/$STAMP: OK (local drill) -> $TARGET"
    exit 0
fi

: "${S3_REGION:?S3_REGION is required}"
: "${S3_ACCESS_KEY_ID:?S3_ACCESS_KEY_ID is required}"
: "${S3_SECRET_ACCESS_KEY:?S3_SECRET_ACCESS_KEY is required}"
if ! command -v aws >/dev/null 2>&1; then
    echo "[backup] AWS CLI v2 ('aws') is required and was not found — backup left in $WORK" >&2
    exit 1
fi

AWS_ACCESS_KEY_ID=$S3_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY=$S3_SECRET_ACCESS_KEY
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
if [ -n "${S3_SESSION_TOKEN:-}" ]; then
    AWS_SESSION_TOKEN=$S3_SESSION_TOKEN
    export AWS_SESSION_TOKEN
fi
unset AWS_PROFILE 2>/dev/null || true

set -- --region "$S3_REGION"
if [ -n "${S3_ENDPOINT:-}" ]; then
    set -- "$@" --endpoint-url "$S3_ENDPOINT"
fi

DEST="s3://$S3_BACKUP_BUCKET/$PREFIX/"
echo "[backup] $PREFIX/$STAMP: uploading $(ls -1 "$OUT" | wc -l | tr -d ' ') files to $DEST"
aws "$@" s3 cp --recursive "$OUT" "$DEST" >/dev/null

# The upload is only complete when every payload file is visible at its key.
for name in "$STAMP.dump" "$STAMP.blobs.tsv" "$STAMP.schema.txt" "$STAMP.SHA256SUMS"; do
    if ! aws "$@" s3 ls "$DEST$name" >/dev/null 2>&1; then
        echo "[backup] $PREFIX/$STAMP: upload missing $name" >&2
        exit 1
    fi
done

echo "[backup] $PREFIX/$STAMP: OK -> $DEST$STAMP.dump"
