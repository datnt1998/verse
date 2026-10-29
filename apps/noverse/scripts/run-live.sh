#!/bin/sh
# Live Noverse: separate worktree (branch live), separate book directory, separate port.
# Update: apps/noverse/scripts/run-live.sh update  (merge main → live, rebuild)
# Under launchd, restart after an update:
#   launchctl kickstart -k gui/$(id -u)/dev.verse.noverse-live
set -e
cd "$(dirname "$0")/../../.."
ROOT="${NOVERSE_LIVE_ROOT:-$HOME/NoverseLive}"
ADDR="${NOVERSE_LIVE_ADDR:-127.0.0.1:8790}"

if [ "$1" = "update" ]; then
  git merge --ff-only main
  go build -o bin/noverse ./apps/noverse/cmd/ainovel-cli
  echo "Đã cập nhật lên $(git log --oneline -1)"
  exit 0
fi

mkdir -p "$ROOT"
# Absolute path so `ps` shows which worktree's binary is serving.
exec "$(pwd)/bin/noverse" serve -root "$ROOT" -addr "$ADDR" "$@"
