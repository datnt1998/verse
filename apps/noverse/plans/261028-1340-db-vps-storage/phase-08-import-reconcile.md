---
phase: 8
title: "Importer idempotent, đối soát và phục hồi dữ liệu"
status: done — 2026-10-29 (bằng chứng: reports/handover-evidence.md, mục DbPhase08)
priority: P1
effort: 20h
dependencies: [4, 5, 6, 7]
---

# Phase 08 — Nhập dữ liệu file có đối soát và đường quay lại

## Context / requirements

Chặn bởi toàn bộ identity/narrative/engine/audio adapters04–07; không nhập domain vào backend còn thiếu implementation. Nguồn là `~/NoverseLive` hoặc root explicit, account path mặc định `~/.ainovel/web-accounts.json` từ `cmd/ainovel-cli/serve.go:130-149`. Root config/custom dirs thực `internal/library/library.go:47-96`; engine path hiện fixed `<story>/output/novel` (`internal/host/storyread.go:22-35`), comments **`<story>/.ainovel/gop-y/<n>.json` + `_events.json`** (`internal/comments/comments.go:121-140,182-183,895-896`), audio paths Việt (`internal/audio/paths.go:21-35,81-117`). Không import nhầm giả định comments/events.jsonl hay scripts/cast.json.

Requirements: read-only dry-run, deterministic IDs, retries idempotent, counts+raw/semantic hashes, conflicts loud, bounded transactions, all files classified, owner map; preservation role/status/temp/hash; immutable source copy; backup+restore; reverse-export đủ dữ liệu phát sinh sau cutover. Không tự tạo lịch sử chương mà file chưa từng lưu.

## Architecture / data flow

Freeze source writers → copy read-only snapshot outside repo → inventory manifest (source ID, library config, relative path, record key, size/raw hash/kind/disposition) → decode versioned structures → normalize canonical domain fields and semantic hash → deterministic source-to-target map → upload immutable blobs with SHA-256 verify → transaction import rows+import_items by bounded story batches → reconciliation read-back (counts/FK/semantic hash/blob download) → mark run verified → publish under maintenance gate.

`import_runs` unique(source_id,manifest_sha256), `import_items` unique(source_id,relative_path,record_key), target IDs deterministic and explicit mapping. Same bytes rerun no extra row/blob/event. Changed source manifest/reused key conflicts if DB row differs, no blind ON CONFLICT UPDATE overwriting live edits. Preseeded partial import resume only matches stored source hash+target version, otherwise stops. Process lock/advisory import lock for one source+target, app not serving writes during initial publish.

Credentials: preserve encoded Argon2id (including recovery codes/mask), map username stable UUID; versions1/2 books expand only correct root, v3 Projects exact. Temp current must-change becomes temporary_password with cutover+24h expiry, not permanent credential; bearer sessions/pairings invalidated once with audit/count report. Disabled metadata/onboarding preserved. Exact-one-owner conflicts show explicit repair map; do not guess grant all projects.

## Files / ownership

`R=/Users/dat/Projects/verse/apps/noverse`.

- Modify `R/cmd/ainovel-cli/storage.go` (created02/extended03), `R/internal/webauth/repository.go` and decoder adapters04, `R/internal/store/file_backend.go` (02), `R/README.md`, `R/docs/architecture.md`; no direct mutation of users' source library.
- Create `R/internal/storageimport/{manifest,decode,importer,reconcile,export,report}.go`, corresponding `manifest_test.go`, `importer_test.go`, `reconcile_test.go`, `export_test.go`; `R/internal/persistence/migrations/00005_import_ledger.sql`; `R/docs/storage-migration.md` (runbook), `R/docs/storage-backup.md` (backup/restore/reverse-export).
- Source adapters read actual `R/internal/library/{files,scan,ops}.go`; `R/internal/store/{chapter_records,progress,run_meta,revisions,checkpoints,session}.go`; `R/internal/revision/migration.go:158-203`; `R/internal/host/imp/workspace.go:51-74,143-193`; `R/internal/audio/{paths,queue,script,export_cache,export_result_cache}.go`; `R/internal/comments/comments.go:121-140,426-548`; `R/internal/webauth/members.go:97-195`.
- No production Delete; scaffolding/temp fixtures stay isolated and cleaned within test. Shared docs/storage.go ownership passed from07, then09.

## Steps

1. Add fail-before fixtures for actual paths/formats plus custom dirs/adopted Unicode project/legacy `_`, v1/v2/v3 accounts, old chapters lacking baseline, pending commit/revision/import, comments lost/unsure/pending and event queue, missing/corrupt audio cache, partial JSONL tail, symlink escapes, unknown files. Include new **account JSON version4** from04 and **project format3/history** from05 independently for reverse-export; current source project format2 is not account version2.
2. Implement pure versioned decoders; do not instantiate file Store with Init/mutate-on-read migration against original account/root. Read source via confined filesystem adapter; reject external symlinks unless explicit allowed source file copied into snapshot with audit path; no ambient HOME or wildcard permission expansion against default library.
3. Implement dry-run and report all conflicts before apply: duplicate username/email case, owner0/>1, duplicate project/story slugs/IDs, invalid JSON/version/hash, records vs prose mismatch, unsupported required file. Every source path disposition imported/derived/excluded/quarantined; quarantine actual bytes in private snapshot/report and block publish if required fact missing. Scratch/locks/logs can be excluded only with documented reason/hash; no silent skip unknown.
4. Import accounts/projects/memberships/audit first, then stories/volumes/chapters+versions/outline/characters/comments/reviews/state/events/checkpoints/sessions, then audio/jobs/cache/blob references. All DDL already migrated, not run goose inside per-story importer. Preserve raw bytes hash separately from canonical content semantic hash. Existing record revision_no may have gaps because prior revisions weren't saved; preserve known revision/content/origin/build and attribute unknown historic author to `import`, never infer real user/agent falsely.
5. Handle accepted text vs edited prose: record canonical remains accepted; differing nonblank chapters/*.md becomes pending workspace/draft revision state for explicit user acceptance. If no record, use existing legacy baseline decoding rules (`internal/revision/migration.go:158-203`) from source sessions/import artifacts; unresolved facts quarantine and require repair, do not fabricate successful commit/checkpoint. Keep user advance mode/permit/pending steer/import intent; after import engine resumes same deterministic state.
6. Upload binaries streaming via03, record object key/size/hash; DB references only ready verified objects. Queue running entries normalize to recoverable queued with historical info; missing binary result not marked completed. Full hash verify every published blob for initial migration (expensive but required), cached optional derivative can be excluded/rebuilt only explicitly listed and counted.
7. Implement `storage reconcile`: expected vs actual counts by domain/project/story, typed semantic digest per logical record, raw source inventory, blob read-back SHA-256, orphan/missing FK checks, owner/admin invariant, ordered stream counts/max seq. Output machine-readable JSON plus Vietnamese summary, exit nonzero on any required mismatch. Count excluded/derived files explicitly; “totals match” alone insufficient if records swapped.
8. Idempotency test: import twice same source/manifest, interrupt after upload/before row commit and after bounded batch, then resume; zero duplicate revisions/jobs/audit/objects and no overwrite of intentional later DB edit. Source drift check before/after scan; freeze final source/rescan must match initial manifest.
9. Implement reverse-export `storage export-files`: new empty destination only, recreate library project/story config, account file with current grants/status/Argon2 hashes/temp state (revoke all sessions/pairings on backend change), accepted+draft/history/comments/audio/workspace/checkpoints/agent streams and referenced blobs. Schema version recognized by file backend05 includes immutable history directory; legacy revision gaps remain gaps. Export→file contracts→new empty PG reimport semantic hashes match. No plaintext session/token recovery or key leakage in ordinary report.
10. Rehearse backup/restore under write freeze: DB custom-format dump + schema version + source/import manifest + ready blob manifest/bucket snapshot or verified copy. Restore to new DB/prefix and run reconciliation/engine resume. Record measured restore time/data loss boundary; propose nightly off-VPS backups/RPO24h default configurable, not claim zero RPO without WAL/PITR setup. Final cutover freeze has RPO0 relative snapshot and no active writers.

## Fail-before test matrix

| Layer | Cases | Gate |
|---|---|---|
| Unit | path/version/mapping/hash/owner/case conflicts, actual comments/audio names | Correct decoding/classification, original bytes unchanged |
| Integration | importer twice/killed/resume/source changes/concurrent importer | Row/blob/event counts stable; changed data conflict loud |
| Integration | new DB user edit then reimport old source | Cannot clobber live edit |
| Integration | raw+semantic counts/hashes, missing/same-size corrupt blob | Reconcile nonzero exit catches mismatch |
| Round-trip | PG→file new history/temp/schema→PG new target | Same canonical facts/hash/history/permissions; no revived sessions |
| Restore/E2E | snapshot/dump+blobs→fresh infra→login/write-resume/audio/export | Real restored data usable; measured RTO reported |

## Exact verification commands

Implementer sets `IMPORT_ROOT` to an **isolated read-only source snapshot**, `IMPORT_ACCOUNTS` to snapshot file, persistent `IMPORT_SOURCE_ID` UUID, explicit `IMPORT_OWNER_MAP`, `IMPORT_MANIFEST`, `RESTORE_FILES_DIR` empty/nonexistent; secrets via env/config not argv. New flags/commands below are implementation deliverables.

```sh
go test ./internal/storageimport -count=1
go run ./cmd/ainovel-cli storage import --root "$IMPORT_ROOT" --accounts "$IMPORT_ACCOUNTS" --source-id "$IMPORT_SOURCE_ID" --owner-map "$IMPORT_OWNER_MAP" --manifest "$IMPORT_MANIFEST" --dry-run
go run ./cmd/ainovel-cli storage import --root "$IMPORT_ROOT" --accounts "$IMPORT_ACCOUNTS" --source-id "$IMPORT_SOURCE_ID" --owner-map "$IMPORT_OWNER_MAP" --manifest "$IMPORT_MANIFEST" --resume
go run ./cmd/ainovel-cli storage import --root "$IMPORT_ROOT" --accounts "$IMPORT_ACCOUNTS" --source-id "$IMPORT_SOURCE_ID" --owner-map "$IMPORT_OWNER_MAP" --manifest "$IMPORT_MANIFEST" --resume
go run ./cmd/ainovel-cli storage reconcile --manifest "$IMPORT_MANIFEST" --verify-blobs
go run ./cmd/ainovel-cli storage export-files --destination "$RESTORE_FILES_DIR" --revoke-sessions
PGSERVICE="$BACKUP_SERVICE" pg_dump --format=custom --file="$BACKUP_DUMP"
PGSERVICE="$RESTORE_SERVICE" pg_restore --exit-on-error --no-owner "$BACKUP_DUMP"
```

Set `BACKUP_SERVICE`/`RESTORE_SERVICE` names in private `.pg_service.conf`, password via mode0600 `.pgpass` or secret env, never password DSN argv. RESTORE_SERVICE targets a newly created **disposable** database, not production. Use restored app env/blob prefix to rerun reconcile. No commands run during planning.

## Risks / rollback / success

- **High (L cao × I cao):** unknown formats/hidden directories/normalization data loss. Complete manifest + current path constants + required quarantine blockers + raw/semantic checks.
- **High (L vừa × I cao):** idempotent import overwrites DB edits. Ledger+CAS/source digest+frozen source; changed source conflict, no unconditional UPSERT.
- **High (L vừa × I cao):** incomplete rollback after first production write. Reverse-export full domain/history/temp/audio and round-trip test **before**09, no promise from old snapshot.
- **High (L vừa × I cao):** backup only DB loses blob data. Snapshot manifests + retained immutable bucket objects/verified copy, restore rehearsal same checkpoint.

Rollback failed import: app still maintenance, leave source untouched; remove only rows/objects owned by unpublished import run in disposable target, or recreate target; never sweep shared referenced blobs. Published/live DB rollback uses freeze+latest verified reverse-export or compatible app release; not delete imported accounts/history. Gate: required mismatch=0, every path classified, second run changed rows/blobs=0, interrupted runs recover, restore/reverse-export round-trip verified.09 cannot go live without these recorded outputs.
