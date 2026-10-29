# Import / reverse-export fidelity

Scope: `internal/storageimport/*` (import, reconcile, export), plus the exported
document mapping in `internal/persistence/postgres/docstore.go`. No schema or
query changes were needed (`go tool sqlc diff` clean).

Verification used the shared disposable services:

```sh
export TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/ainovel_test
export MINIO_TEST_ENDPOINT=http://127.0.0.1:59000
export REQUIRE_POSTGRES_TEST=1 REQUIRE_MINIO_TEST=1
go test -count=1 ./internal/storageimport/... ./internal/persistence/... ./internal/store/...
go build ./...
go vet ./...
go tool sqlc diff
go run ./cmd/ainovel-cli storage import --dry-run --root <temp copy> --accounts <fixture accounts> \
  --source-id 6f1d0f9e-3f2a-4c88-9d0e-1b1a2b3c4d5e --manifest /tmp/dryrun-manifest.json \
  --owner-map 'du-an=tac-gia,_=quan-tri'
```

**Fail-before method.** The pre-fix code is `HEAD` (my work is uncommitted).
I materialised it with `git archive HEAD` into `/tmp/prefix-check` and ran one
adapted check per defect there (same fixture, same helpers, old public API).
Every quoted line below is the actual pre-fix failure message; the working-tree
tests assert the same behaviour through the new API.

## Defects

### 1. Chapter-record fields were dropped on import

- **Cause.** `decodeChapterRecord` decoded only version/chapter/revision/build/
  origin/content and invented the title from the first heading. `rows.go`
  inserted `chapter_versions` without `facts`/`style_delta`, used `SHA256 =
  RawDigest(content)` instead of the record's normalized digest, and
  `accepted_at = import time` (`tsOr(nonZeroTime(v.AcceptedAt, im.opts.Now))`,
  while the decoder never filled `AcceptedAt`). Reconciliation hashed a
  `ChapterVersion` that had no facts, so it agreed with the lossy row: "0 lệch".
- **Fix.** The record is decoded as a complete `domain.ChapterRecord` shape
  (raw `facts`/`style_delta` kept verbatim, `content_sha256` verified against
  `domain.ChapterContentSHA256`, `accepted_at` required, `origin` limited to
  generated/user), the title comes from `facts.title` with a heading fallback,
  and `insertChapter` writes `facts`, `style_delta` and the record times.
  `ChapterVersion` now carries facts/style delta, so `chapterDigest` compares
  them; a row with empty facts is reported.
- **Regression test.** `TestReconcileReportsMissingChapterFacts` damages the
  stored revision (`SET LOCAL noverse.allow_immutable_write`, `facts='{}'`) and
  requires a mismatch naming `chapter/000001`.
- **Fail-before.** `đối soát phải báo lệch khi mất facts` (reconciliation
  returned OK because the digest ignored facts). Field loss itself is covered
  by defect 2's facade test: pre-fix `bản ghi mất trường: facts={Title:
  Summary: Characters:[] ...} accepted=2026-10-29 02:50:19.157827 +0000 UTC
  sha="d1e968..."` — empty facts, import-time acceptance, raw digest.

### 2. Import wrote under keys the production Store never reads

- **Cause.** Plans were stored as artifact `chapter_plan` with key
  `drafts/02.plan.json`; the Store reads kind `chapter_plan`, key `02`
  (`docstore.resolveDoc`, `chapterFromKey`). Chapter prose never became a
  `chapter_text` artifact (the divergent-text path wrote `draft_text`
  instead). `characters.json` produced only typed `characters` rows, never
  `story_state(kind='characters')` which `Characters.Load` reads. The same
  class affected summaries, snapshots, `meta/usage.json`,
  `meta/state_changes.jsonl` (written as stream `world_changes`), `timeline.jsonl`
  and `meta/sessions/cocreate.jsonl` (written as an agent session).
- **Fix.** `postgres.ResolveDocument`/`DocumentPath` expose the existing
  `resolveDoc` mapping (plus its inverse over `stateKinds`, `outlineDocs`,
  `streamDocs`) as a small read helper. `decodeStoryDocument` routes plans,
  drafts, chapter prose, summaries, snapshots, usage, timeline, state changes,
  sessions and unmapped `meta/*.json` engine documents through it, so import
  writes exactly the keys the Store reads. Project documents now store
  project-relative paths (`<proj>/.ainovel/config.json`, rules, assets) instead
  of whole source paths, and ideas use the library idea id (file stem).
  Stream rows gained `log_key` so per-file logs (session transcripts) keep their
  identity.
- **Regression test.** `TestImportReadableThroughProductionStore` builds
  `postgres.NewBackend` + `store.NewFromBackend` (the hosted facade) and reads
  plan 2, chapter 1 text, characters and the accepted record 1, comparing field
  by field with the source.
- **Fail-before.**
  - `Store production không đọc được plan đã nhập: <nil>`
  - `chương khác nguồn: got="" err=<nil>`
  - `nhân vật khác nguồn: got=[] err=<nil>`

### 3. Reverse-exported chapter records failed to load

- **Cause.** The record writer emitted only version/chapter/revision/build/
  origin/content; no `content_sha256`, no `facts`/`style_delta`, no
  `accepted_at`. `store/file_backend` rejects that with a digest error.
- **Fix.** `exportRecord` is a complete record: normalized digest verified
  against the stored content, raw facts/style bytes, real acceptance time.
  DB origin `import` is exported as `user` (the file backend accepts only
  generated/user; an imported acceptance was authored outside the engine),
  and the record format version is `domain.ChapterRecordVersion`.
- **Regression test.** `TestExportRoundTrip` opens the exported tree with
  `store.NewStore` and loads revision 1.
- **Fail-before.** `bản ghi export không mở được: Đọc bản ghi nghiệm thu
  chương 1: content digest mismatch`

### 4. Reverse export was not a usable rollback

- **Cause.** Projects were enumerated from `import_items` (so post-cutover
  projects vanished); no accounts, checkpoints, event streams or retained
  revisions were written; blobs were enumerated from the ledger and missing
  objects were skipped; project-document logical names were full source paths,
  so export produced `du-an/du-an/.ainovel/...`; artifacts kept the
  `chapter_plan/` prefix (`.../chapter_plan/drafts/02.plan.json`); and
  soft-deleted rows were exported because the queries had no `deleted_at`
  filter.
- **Fix.**
  - Projects, stories, chapters, comments, reviews and audio metadata are
    enumerated from live DB rows (`deleted_at IS NULL`, invalidated reviews
    excluded).
  - Accounts: `<destination>/.ainovel/web-accounts.json`, the one file
    `webauth.NewStore` reads. Layout decision: accounts sit inside the exported
    library because that is the file backends' own layout; `webauth` is pointed
    at this path. Users, credentials (password hash + change time),
    project memberships, recovery code, temporary password and audit rows are
    written; sessions/pairings are not exported (only hashes exist) and
    `--revoke-sessions` still revokes them.
  - Checkpoints → `meta/checkpoints.jsonl` as `domain.Checkpoint` lines.
  - Event streams → the file each stream maps to (`meta/decisions.jsonl`,
    `meta/rewrites.jsonl`, `meta/timeline.jsonl`, `meta/state_changes.jsonl`,
    `meta/relationship_log.jsonl`, `meta/sessions/cocreate.jsonl`, …). A stream
    without a file layout is still written under `meta/streams/` and reported.
  - Retained history → `meta/chapter_versions/<NNNNNN>/<revision>.json` (the
    file-project format v3 layout) for every immutable revision; the latest
    record stays the authority in `meta/chapter_records/`.
  - Ready blobs are enumerated from `blobs`; imported objects keep the ledger
    path, post-cutover objects get a key-derived path inside their live
    project/story. Any ready blob that cannot be fetched fails the export.
  - `writeFile` refuses a second write to the same path with different bytes,
    and document/artifact paths come from the shared mapping, so neither
    `du-an/du-an/...` nor `chapter_plan/...` can reappear.
  - Blobs whose owner project/story is soft-deleted are skipped (ledger paths
    are matched against live/deleted directories loaded from the DB); chapters,
    stories and projects that are deleted are never written.
  - Legacy stories of the anonymous project (`_`) are written back at the
    library root, not under `<stories>/`, so `library.Scan` still classifies
    them as legacy stories. The round-trip test asserts the exported tree opens
    with `library.Open`/`Scan`/`Stories`/`ResolveStory`.
  - Reviews: latest round → `reviews/NN[-global].json`, older rounds →
    `reviews/rounds/NN[-global]-rK.json`.
- **Regression tests.** `TestExportRoundTrip` (rewritten: file Store over the
  exported tree + fresh webauth path; asserts plan, text, characters, record
  fields, history file, 3 checkpoints, decision stream, review round, accounts
  and credentials, no `du-an/du-an`, no `chapter_plan/`),
  `TestExportIncludesPostCutoverContent`,
  `TestExportFailsWhenBlobUnreadable`, `TestExportSkipsSoftDeletedContent`.
- **Fail-before.**
  - `tài khoản sau export: 0 err=<nil>` (no accounts file)
  - `thiếu dự án tạo sau cutover: ... du-an-moi/project.json: no such file or directory`
  - `export phải fail khi thiếu blob`
  - `export làm sống lại truyện đã xoá mềm: <nil>`
  - `export lặp tiền tố dự án: du-an/du-an/[.ainovel tai-san]`
  - `export giữ tiền tố chapter_plan/: [du-an/kich_ban/01-dem-trang/output/novel/chapter_plan/drafts/02.draft.md du-an/kich_ban/01-dem-trang/output/novel/chapter_plan/drafts/02.plan.json]`

### 5. A fact file could change between scan and import

- **Cause.** `processFile` reread the file and decoded it without comparing the
  bytes with the manifest hash (only blob import verified the content).
- **Fix.** Unless the file is the accounts file or a synthetic manifest entry,
  `processFile` compares size and `RawDigest` with the manifest and quarantines
  the file with `nội dung đổi giữa lúc quét và lúc nhập (snapshot không đóng
  băng)`; a quarantined file fails the run.
- **Regression test.** `TestImportRejectsFileChangedAfterScan`.
- **Fail-before.** `import phải fail khi file đổi sau quét`

### Review follow-up: export completeness and inverse mapping

Six more defects were found in review of the first revision. Each fix has a
regression test and the pre-fix failure line observed by reverting that exact
behaviour in the worktree (the tests were run against the reverted code, then
the fix restored; defect 1/4 evidence came from the same `git archive HEAD`
checkout).

**a. Draft-only chapters lost their draft.** `exportChapters` hit
`continue` when a chapter had no accepted version, before the draft writer.
The fixture's chapter 2 is exactly that state.
Fix: the draft is written before the version check (deleted chapters still
excluded). Test: `TestExportRoundTrip` now loads chapter 2's draft through the
file Store. Fail-before: `bản nháp chương 2 mất sau export: "" err=open
.../drafts/02.draft.md: no such file or directory`.

**b. A resumed import overwrote an edited draft.** `applyChapterDraft` ran
`UPDATE ... WHERE draft_text <> $3`, so a differing stored draft was replaced
and `writeItem`'s digest comparison never ran (the update reported a row).
Fix: absent chapter → insert; empty draft → fill; any differing draft is left
alone and reported as a conflict, exactly like other items. Test:
`TestImportDraftConflictIsNotOverwritten` (drops the ledger row to model the
resume, edits the draft, re-runs). Fail-before: `phải báo xung đột bản nháp
(runErr=<nil>, conflicts=[])`.

**c. Importer-only state kinds were skipped.** `signals` and `comment_events`
have no production document path (the file backends read `meta/signals.json`
and `.ainovel/gop-y/_events.json` directly), so the new mapping pushed them to
`Skipped` while the export succeeded.
Fix: the non-document singleton table maps `format`, `signals` and
`comment_events`; every other unmappable state, outline, artifact or review
kind now fails the export instead of being skipped (artifacts still keep their
source key when it is a path, so import-only artifacts round-trip). Test:
`TestExportRoundTrip` asserts `meta/signals.json` and `gop-y/_events.json`
decode equal to the source. Fail-before: `đọc .../meta/signals.json: no such
file or directory`.

**d. Artifacts of soft-deleted chapters were written back.** Only
`exportChapters` filtered `deleted_at`, so a retained `chapter_text` artifact
became readable again.
Fix: the deleted chapter numbers are loaded once per story and chapter-scoped
artifacts (`chapter_text`, `chapter_plan`) plus comments of those chapters are
excluded. Test: `TestExportSkipsDeletedChapter` (one chapter deleted, the
story stays live). Fail-before: `export làm sống lại chương đã xoá mềm: <nil>`.

**e. Legacy prose digest did not match the export invariant.** The synthesized
baseline revision stored raw bytes with `RawDigest`, while export requires
`domain.ChapterContentSHA256` of normalized content; BOM/CRLF prose imported
then failed export.
Fix: the baseline stores `domain.NormalizeChapterContent(text)` and its
normalized digest. Test: `TestLegacyProseRoundTripsNormalizedDigest` imports
BOM+CRLF legacy prose and exports byte-identically. Fail-before: `export: dự án
_: truyện truyen-cu: chương 1: revision 1: sha256 trong DB không khớp nội dung`.

**f. `DocumentPath` was not a true inverse of `resolveDoc`.** The snapshot
branch accepted `vbad`, agent-session tasks containing a hyphen could not
round-trip (the forward parser splits at the last hyphen), keyed streams and
markdown views were accepted outside their families, and the comment cited a
test that did not exist.
Fix: `DocumentPath` now takes the whole `DocumentRef`, rejects identities the
forward parser cannot produce, and canonical chapter/record keys are part of
`DocumentRef`. New invariant test
`TestDocumentPathIsInverseOfResolve` pins every state / outline / stream /
session / runtime / artifact / draft / record / markdown-view path family, and
`TestDocumentPathRejectsUnproducibleIdentities` pins the rejections. Fail-before
(on the loose version): `DocumentPath accepted vbad ->
"meta/snapshots/vbad.json"` and `DocumentPath accepted hyphenated task ->
"meta/sessions/agents/writer-ch-01.jsonl"`.

### Review follow-up: guards that were too strict

**g. Extensionless story files were rejected.** The "preservable import-only
path" guard looked for `/` or `.`, so a valid story-level file such as
`<story>/notes` (role `story_file`, stored as `import_artifact` key `notes`)
failed the whole export.
Fix: the decision is a safety check on the recorded relative path
(`safeRelativePath`: non-empty, relative, backslash-free, `path.Clean` stable,
no `..`), never punctuation. Test: `TestExportKeepsExtensionlessStoryFile`
imports and exports such a file and checks its bytes. Fail-before: `export phải
nhận file không phần mở rộng: dự án du-an: truyện 01-dem-trang: artifact
import_artifact/"notes" không có đường dẫn file — dữ liệu sẽ mất`.

**h. Unanchored global reviews failed the export.** `reviewFileName` returned
empty for `Chapter <= 0`, although the Postgres store supports an unanchored
global review and the file backend names it `reviews/00-global.json`.
Fix: the name comes from the file backend's own
`store.ReviewScopeOf(scope).FileName(chapter)`, so chapter, arc and unanchored
global reviews all use the exact on-disk name (no hand-written format here).
Test: `TestExportKeepsUnanchoredGlobalReview` saves a global review through the
Postgres store, exports, and loads it back with `World.LoadGlobalReview(0)`.
Fail-before: `export với global review không mốc: ... review global không có
đường dẫn file — dữ liệu sẽ mất`.

**i. Session task keys containing `/` did not round-trip.** The forward session
regex accepts `meta/sessions/agents/writer-ch/01.jsonl` (task `ch/01`), but the
inverse rejected any `/` in the task. Only a hyphen is ambiguous (the forward
parser splits at the last hyphen).
Fix: the inverse permits `/` in the task and still rejects hyphens; the case is
added to `TestDocumentPathIsInverseOfResolve`. Fail-before:
`DocumentPath(ResolveDocument("meta/sessions/agents/writer-ch/01.jsonl")):
no path for {Class:agent_session Kind:agent_sessions Key:ch/01 Agent:writer}`.

### Guard

`TestImportReconcilesClean` keeps "0 lệch" honest: a faithful import must still
reconcile clean after the stricter comparison. It passes before and after.

## Files changed

- `internal/persistence/postgres/docstore.go` — exported `DocumentRef`,
  `DocClass`, `ResolveDocument`, `DocumentPath` (thin views over `resolveDoc`
  and its inverse tables); `DocumentPath` takes a `DocumentRef` and rejects
  identities the forward parser cannot produce.
- `internal/persistence/postgres/docpath_test.go` — round-trip invariant for
  every path family plus the rejection cases.
- `internal/storageimport/decode.go` — complete chapter-record decoding,
  chapter_text/draft artifacts, characters state, shared-mapping document
  decoder, project-relative document names, idea ids, stream log keys.
- `internal/storageimport/rows.go` — canonical artifact kind/key, facts and
  style delta on versions, `log_key` on events, chapter draft writes, legacy
  prose baseline only (no fabricated draft).
- `internal/storageimport/reconcile.go` — facts/style in the chapter digest,
  chapter_text and draft loaders, canonical artifact lookup, event log keys,
  draft-backed chapter coverage.
- `internal/storageimport/importer.go` — frozen-snapshot check, draft deferral.
- `internal/storageimport/export.go` — rewritten for a usable rollback (live
  row enumeration, accounts, checkpoints, streams, history, blobs, path
  mapping, deletion filters, duplicate-path guard).
- `internal/storageimport/scan.go` — dropped the unused `LibraryConfig.Extra`
  field that leaked a non-schema `Extra` key into library payloads.
- `internal/storageimport/importer_test.go` — the tests above.

No commits were made.

## Left open

- **Account v1/v2 legacy role migration** (out of scope). Exported accounts are
  v3/v4-shaped; the fixture is v3 and imports fine, but an old v1/v2 file with
  legacy role names is still not migrated by import.
- **Sessions / device pairings / agent sessions / jobs** are not part of the
  reverse export. Tokens only exist as hashes, and agent transcripts/jobs were
  outside the listed rollback data; `--revoke-sessions` still revokes them.
- **Retained history layout** `meta/chapter_versions/<NNNNNN>/<revision>.json`
  follows the file-project format v3 plan; today's file backend does not read
  it (latest record stays authority). On re-import those files are preserved as
  engine artifacts rather than versions.
- **Review current/archive distinction** cannot be reconstructed per row: the
  table stores rounds, not which round was the live file. Export treats the
  newest round as the current `reviews/NN.json` and archives the older ones.
- **`meta/format.json`** has no entry in the production document mapping; the
  exporter keeps it through an explicit singleton path so the engine format
  file round-trips.
- **Audio script volume is not stored** (the `audio_scripts` row has no volume
  column; the tap number only lived in the source path), so exported scripts
  land in `audio/ban-doc/tap-00/NN.json`. The bytes and the chapter identity
  round-trip; only the tap folder is normalised. Audio/TTS work itself is out
  of scope.
- **Non-empty export destination is refused** (the exporter would otherwise mix
  trees). This is a behaviour choice, not a data gap.
- **Streams with no file layout** (written by hosted code, not by import) are
  still preserved under `meta/streams/` and reported rather than failing the
  export; importer-produced streams all have canonical paths. Every other
  unmappable state/outline/review kind fails the export.
- **Story-level files stored as `import_artifact`** export under the engine
  directory (`output/novel/<key>`) even when the source sat beside
  `story.json`; the database does not record which base the key used. The bytes
  and key are preserved, and a re-import stores them where the export put
  them.
