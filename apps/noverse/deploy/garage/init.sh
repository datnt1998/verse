#!/bin/sh
set -eu
: "${S3_ACCESS_KEY_ID:?Set S3_ACCESS_KEY_ID (Garage key IDs start with GK)}"
: "${S3_SECRET_ACCESS_KEY:?Set S3_SECRET_ACCESS_KEY}"
: "${S3_BUCKET:?Set S3_BUCKET}"
# The server image is deliberately shell-free. This companion carries the same
# pinned Garage binary and reads only the node identity, not its database.
export GARAGE_RPC_HOST="$(garage node id -q)"
attempt=0
until garage status >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 60 ] || { echo 'Garage RPC did not become ready' >&2; exit 1; }
  sleep 1
done
# Do not modify an already applied layout on restart.
if garage layout show | grep -q 'Current cluster layout version: 0'; then
  garage layout assign -z local -c 1G "${GARAGE_RPC_HOST%%@*}"
  garage layout apply --version 1
fi
attempt=0
until garage health >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 60 ] || { echo 'Garage did not become healthy' >&2; exit 1; }
  sleep 1
done
# Redirect key output: import must never leak the secret into container logs.
if ! garage key info "$S3_ACCESS_KEY_ID" >/dev/null 2>&1; then
  garage key import --yes -n noverse "$S3_ACCESS_KEY_ID" "$S3_SECRET_ACCESS_KEY" >/dev/null
fi
if ! garage bucket info "$S3_BUCKET" >/dev/null 2>&1; then
  garage bucket create "$S3_BUCKET"
fi
garage bucket allow --read --write --owner "$S3_BUCKET" --key "$S3_ACCESS_KEY_ID"
echo 'Garage bucket ready'
