---
phase: 6
title: "Engine, tools, readers và IO ẩn không còn phụ thuộc thư mục"
status: completed
priority: P1
effort: 44h
dependencies: [5]
---

# Phase 06 — Chặn đường vòng đọc/ghi file của engine

## Progress evidence (2026-10-29, DbPhase06)

Done and verified:

- `store.Backend` now exposes `Docs() *store.IO` and `Comments() *comments.Store`;
  `Store.Docs/FileRoot/LatestReview/PlanChapters/ReviewChapters` are the only
  document/review surfaces. File mode maps them to `Dir`, postgres to rows.
- `host.Storage` injection (`internal/host/storage.go`): `WithStorage` +
  `StoryLease` (file flock or durable row). `Host.New` skips flock, on-disk log,
  `store.NewStore(OutputDir)` and session replay when injected; read host uses
  `newReadHostWithStore`. `Scratch` + `EngineScratchDir` give bounded private
  temp space instead of paths derived from `Dir() == ""`.
- Reader: `NewStoryReaderWithStore` no longer derives `"."`; `LatestReview` and
  chapter markers (plans/reviews) go through the backend. `peekBook`/story
  summaries no longer stat `""`; hosted `updated_at` comes from the story row
  (`library.Story.UpdatedAt`).
- Tools: `edit_chapter` applies the agentcore prose rules in memory
  (`agentcoretools.ApplyProseEdit`) and saves via `DraftStore` — no workspace
  editor. Comment hooks use the backend comment store. Ledger loads/saves via
  `store.Docs()` (`meta/ledger.json` mapped in postgres docstore); project and
  sibling story scans are explicit file-only helpers (`siblingStories`,
  `rules.SiblingOpeningsFor`).
- Web: review reader takes pack/style from run meta on hosted (no config file
  read); comments endpoints use `commentStoreFor` (scoped rows); chapters list
  counts come from the store comment surface; audio answers 501 on hosted until
  phase 07; notifications keep the deterministic fallback instead of scanning.
- Lease: `internal/persistence/postgres/story_lease.go` (claim/renew/release,
  `ErrLeaseLost`) and a write-transaction fence (`DocStore.SetFence`,
  `WithLeaseFence`) on every `BeginWrite` and compound tx. Migration
  `00006_story_lease_key.sql` adds the missing `(project_id, story_id)` key that
  `ON CONFLICT` needs.
- Tests: `internal/host/storage_test.go` (injection/scratch), `storage_pg_test.go`
  (PG Host with no library, scripted engine run on PG and file), 
  `internal/persistence/postgres/story_lease_test.go` (exclusive claim, stale
  fence cannot commit, released lease cannot write), 
  `internal/tools/storage_backend_test.go` (in-memory backend: edit_chapter,
  matching semantics, ledger doc).

**Phase 06c addendum (2026-10-29) — gate test + the defect it exposed**

- `internal/entry/web/storage_cutover_test.go` (`TestStorageCutoverHostedOverHTTP`,
  PG) is the gate: HOME/cwd isolated, library root = empty `t.TempDir()` with
  `chmod 0555`, a local OpenAI-compatible `httptest` endpoint scripts the writer
  (plan → draft → check → commit, same shape as `internal/host`'s
  `scriptedWriterModel`), and `Gateway.launchHosted` is replaced by an in-process
  worker built exactly like `openWorkerStorage` (resolve scope → `NewStoryLease`
  → `NewBackend(WithLeaseFence)` → `host.New(WithStorage)`), served over a
  loopback `httptest` so every call is a real HTTP hop. Flow: `POST …/stories`
  (create) → preseed writing state → `POST …/next` (review permit) → real engine
  writes chapter 1 → `GET …/chapters/1` + `GET …/chapters` (read) →
  `POST …/chapters/1/comments` + `GET` (comment round-trip). Assertions: permit
  consumed and `waitingFor=gate` (proves `commit_chapter` finished *including*
  the commit checkpoint and cleared `PendingCommit`), plus zero entries under the
  library root and the process cwd.
- **Defect found and fixed:** on postgres, `commit_chapter` failed at
  `appendCommitCheckpoint` — `digest artifact meta/chapter_records/000001.json:
  postgres document backend has no mapping ... unmapped document path`. The
  commit checkpoint digests `chapters/NN.md` + `summaries/NN.json` +
  `store.ChapterRecordPath(N)`, but the accepted record is a typed revision row.
  Result: chapter text/progress landed, the commit never completed, the story was
  left with `PendingCommit{stage: progress_marked}` and every later commit was
  rejected (“có bản nộp chương chưa khôi phục”).
  `internal/persistence/postgres/docstore.go` now resolves
  `meta/chapter_records/%06d.json` to a **read-only derived view** of the current
  revision row (read/serialize/exists), while document writes and deletes are
  refused (`errChapterRecordReadOnly`) — revisions still only append through
  `pgChapterRecords`. Pinned by
  `internal/persistence/postgres/backend_test.go › chapter_record_document_view`.
- Docs: `docs/architecture.md` gained a “Phase 06 engine cutover” block
  (injected storage+lease, hosted worker env/scratch, `Dir()==""` → explicit 501,
  the record doc view, the gate test).
- Fail-before evidence (mutations reverted after each run):
  - removing the record read view → `storage_cutover_test.go:484: advance permit
    not consumed: {... AdvancePermitChapter:1 WaitingFor:input
    RecoveryLabel:Khôi phục: nộp chương 1 bị gián đoạn}`;
  - making `host.New` write beside `OutputDir` on an injected store →
    `storage_cutover_test.go:517: process cwd "…" must stay empty; found 3
    entrie(s): [output output/novel output/novel/engine.log]`;
  - removing the view again, docstore subtest → `backend_test.go:384: read
    accepted record meta/chapter_records/000004.json: unmapped document path`.

Verification run (this phase): `go build ./...`; `gofmt -l internal/ cmd/`
(clean); `go vet` on store/postgres/host/tools/ledger/entry-web; `go test
./internal/{store,ledger,tools,revision,host}` (green); PG suites with
`deploy/compose.test.yml` + `REQUIRE_POSTGRES_TEST=1`
(`./internal/persistence/postgres`, host PG tests).
**Acceptance run (2026-10-29):**
`REQUIRE_POSTGRES_TEST=1 TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/ainovel_test
go test ./... -count=1 -timeout 1800s` → exit 0, 72 packages `ok`, 0 `FAIL`,
0 “skipping Postgres” lines (no PG suite skipped); `internal/entry/web`
170.1s, `internal/persistence/postgres` 25.1s, `internal/host` 19.7s.

Handover (phase 07/08 — none of these block the phase-06 gate):

1. Hosted engine worker launch — **done** (`launchHostedWorker`, env-scoped store,
   self-claimed lease, private scratch cwd). Kept here only as the entry point
   `workerStartError`/`openWorkerStorage` for phase 07's TTS/jobs work.
2. `storage_cutover_test.go` — **done** (see DbPhase06c evidence below).
3. Cocreate/assets/validate 501s: assets remains file-only by design until the
   asset repository exists; cocreate/validate still need their store-backed read
   paths (phase 07/08 scope).
4. Import/export/audio/exp/sim workspaces still classify as file-only; their
   scratch+repository migration is phase 07.
5. Hosted comment re-anchor: the journal drain now reads through the backend
   comment surface (`publishCommentReanchor`) and worker `/api/next` applies
   comments through `host.Host.CommentStore()`. Phase 07 must still verify the
   worker→gateway event bridge end-to-end under restart (same SSE reconnect
   checks as `TestEventReconnect`).
6. Remaining `Dir()` readers in `internal/entry/web` are explicit 501s
   (activity log, style samples, covers, exports/download, reveal, fs, library
   inspect, project/story config files). Phase 07 routes exports/activity/
   covers/style samples through repositories; hosted rule tiers
   (`rules_api.go`, `userrules`) belong to phase 07 as well.

## Context / requirements

Đây là rủi ro lớn nhất. Sau05, typed repositories tồn tại nhưng Host/reader/tools vẫn có IO bypass. Facts đã đọc lại: `Host.New` (`internal/host/host.go:120-177`) lấy lease ở `:136,143`, log ở `:163`, rồi **tự tạo store ở :175**. `NewStoryReader` cố định `<story>/output/novel` (`internal/host/storyread.go:22-42`), `LatestReview` đọc trực tiếp directory/files (`:518,532`). Gateway `launchWorker` thực ở `internal/entry/web/gateway.go:715-722`, không phải worker.go; child nhận cwd+env rồi `RunWorker` load config/assets và Host (`internal/entry/web/run_worker.go:24-36`).

Tool `NewEditChapterTool` gọi filesystem editor (`internal/tools/edit_chapter.go:32-35`). `afterChapterWrite` gọi file comment store (`internal/tools/comments_hook.go:13-15`); `commit_chapter.go:309,527,666,777`; `draft_chapter.go:122,145`; `edit_chapter.go:162,172`; `revision/service.go:214`; `revision/projector.go:216`. Ledger/sibling APIs còn path-based (`internal/tools/ledger_gate.go:13,17,28,43,48`, `retention.go:77`, `retention_context.go:21`, `save_foundation.go:177`). Không giải quyết bằng mount cả library lên VPS hoặc giả directory từ database.

## Architecture / flow

Entry identifies project/story UUID + actor/run ID → factory creates scoped facade and repository-backed config/assets → acquire durable story lease/fence → Host uses injected store → tools read/modify through contracts → compound transaction rechecks lease+actor+expected versions → events → observer/UI. File entry creates file backend/flock/paths unchanged. PG lease row stored in schema05, renew bound lifetime, every mutating tx requires fence/valid lease; DB disconnect/lost lease cancels Host and rejects late writes even if LLM returns later.

Scratch is explicit, private, bounded per operation, removed on exit; **not** content authority, never watcher source. Read-only embedded assets can remain; custom styles/rules/pack/source references come through repository/blob. Existing exact prose-match algorithm must remain, but string edit applies in-memory per bounded chapter then CAS save, not `agentcoretools` filesystem write. Reuse algorithm if library exposes pure matching; otherwise extract same behavior locally with existing edit fixtures, no fuzzy matching redesign.

## Files / ownership

`R=/Users/dat/Projects/ainovel-cli`. Phase06 owns these files exclusively after05:

- Modify `R/internal/host/{host,options,read_host,storyread,books,resume,usage_replay,web_revision,web_simulate,web_live,autoexport}.go`; verified current symbols above; `R/internal/entry/web/{gateway,run_worker,worker,worker_live,worker_results,worker_cocreate,storyread,projects,quality_api,nao_api,ledger_api,retention_api,comments,watch,config,lessons_api,universe_api,notify,activity_history,search_api,story_delete,cocreate_project,reveal,fs}.go` (filesystem owners/calls in evidence report).
- Modify `R/internal/tools/{edit_chapter,comments_hook,commit_chapter,draft_chapter,ledger_gate,retention,retention_context,save_foundation}.go`; `R/internal/revision/{service,scan,migration,projector}.go`; `R/internal/host/imp/{workspace,source,preview,runner,analyze,state,publish}.go`; `R/internal/host/sim/{scanner,import,runner}.go`; `R/internal/host/exp/{exporter,inspect,pattern}.go`; `R/internal/rules/openings.go`; `R/internal/ledger/{project,ledger}.go`.
- Modify `R/internal/bootstrap/configfile.go:98-165`; `R/docs/architecture.md`, `R/docs/import-pipeline.md`, `R/README.md`.
- Create `R/internal/host/storage.go`, `R/internal/persistence/postgres/story_lease.go`, `R/internal/tools/prose_edit.go`, `R/internal/host/storage_test.go`, `R/internal/tools/storage_backend_test.go`, `R/internal/entry/web/storage_cutover_test.go`.
- Do not delete offline adapters; remove dead hosted path fallback and obsolete file-mirror code created by extraction if any. No fake `Dir` returning temp to satisfy API.

## Steps

1. Add no-filesystem fail-before test: preseed DB+blobs, run PG Host and gateway with HOME/scratch isolated, no library mount and read-only working directory. Perform context/read/draft/edit/commit/review/outline/rewrite/comments/import-resume/export; instrument file adapters to fail if invoked in PG tests. Scratch permitted only explicit acquisition; tests must fail now on Host.New/StoryReader/NewProseEdit paths.
2. Add injected-store/lease/config options following existing `NewOption` convention (`internal/host/options.go:5-29`); no mutable package global factory. PG constructor does not call Store.Init file writes or legacy file upgrader. Preserve offline NewStore/flock initializer and logging ordering. Process logs stdout on VPS; any durable user-visible activity has repository event source, not tui.log needed on disk.
3. Update child engine launch: gateway factory identifies story UUID, passes backend config by env and opaque identity/run scope (never secrets argv); runtime working directory is scratch, not required story dir. Child receives account/service authorization scope and renewable story fence. Local `__web-worker` and new remote TTS pulling worker are distinct; do not move LLM engine to Mac by accident.
4. Replace reader construction and metadata stats. Actual direct NewStore hosted sites: `internal/entry/web/projects.go:98,1194`; `ledger_api.go:51,193,198` (5); Host constructors `internal/host/host.go:175`, `storyread.go:35`, `books.go:96` (3). `PeekBookStore` no longer tests os.Stat for PG. LatestReview/plans/reviews enumerate via repository, not os.ReadDir. Metadata timestamps come from rows, no mtime fallback.
5. Reader factory consumers (14 production calls): `audio.go:165,228,251,672`; `comments.go:60`; `lessons_api.go:71`; `notify.go:210`; `projects.go:939`; `quality_api.go:264`; `storyread.go:33`; `universe_api.go:126,143`; `watch.go:168`; `internal/host/autoexport.go:69`. Migrate all14 in owned files; audio identity reader substitutions are06, audio persistence changes07. Existing file-only constructor remains useful, not shim. Tests constructing readers (`jobs_export_test.go:21,62`, `review_view_test.go:32`, `story_builds_test.go:76`, host reader tests) keep file fixtures plus add PG cases.
6. Rewrite prose edit and path-dependent hooks: all comment hook callers listed Context, `rules.SiblingOpenings` callsites commit+projector, ledger load/save/history and sibling comparisons in retention/save foundation. Shared pure operations receive typed repository data; backend writes CAS+checkpoint. Comment lifecycle is non-blocking prose contract with durable idempotent reanchor event, not warning-only dropped work. Preserve NFC/quotes/whitespace matching behavior from existing EditTool tests.
7. Replace import workspace file primitives with typed workspace repository/artifacts/source reader; preserve manifest digest, intent, ordered range analyses, terminal failure raw content privacy and resume. CLI SourcePath still allowed via local source adapter; hosted upload/source IDs resolve via BlobStore, never arbitrary VPS path. External-edit revision Scan remains file capability; PG revision uses explicit draft/version. Reverse-export later reconstructs exact offline layout, not live mirror.
8. Move project/story/global custom config/rules/style/ledger into typed repo document precedence with env runtime defaults. Hosted no file picker/reveal/open-folder; UI capability flags and explicit unsupported response replace desktop-only operations, while upload/download/import/editor retain requested behavior. Do not silently return success no-op for xdg-open. No model/user settings lost because hidden paths were labeled “config”.
9. PG watchers publish only after DB commit; use persisted story/domain event cursor and reconnect revision so missed SSE reconnect refetches facts, no need Redis. File watcher remains for external local edits. Search/lessons/universe/notifications/activity/deletion use repo and scoped authorization, not path cards/size/log mtime.
10. Update architecture/import docs; enumerate remaining `os` operations and permit only file backend/import source adapter/scratch/read-only embeds. No business IO exceptions based solely on file name. 07 takes audio persistence and jobs, not leftover engine bypass.

## Fail-before test matrix

| Level | Cases | Gate |
|---|---|---|
| Unit | prose exact/ambiguous/NFC/quote/whitespace matching, checkpoint digest, comment hook | Match current tool semantics, no direct IO |
| Contract both backends | context/draft/edit/commit/rewrite/outline/review/rules/ledger/siblings | Same facts; every mutation goes through backend |
| Integration PG | two engine processes, lease lost while LLM pending, DB disconnect then resume | Stale fence cannot commit; no duplicate checkpoint/version |
| Integration PG | no library/HOME mount, read-only cwd, watcher reconnect, import pending resume | Full round-trip actual DB/blob, no filesystem facts |
| HTTP integration | reader/search/lessons/universe/jobs/activity/delete with stale permission | No cache cross-tenant leak; no old tui.log dependency |
| E2E | write→pause→restart container→resume→rewrite→export | No loss of durable progress/intent; user-facing content intact |

## Verification commands

```sh
go test ./internal/tools ./internal/revision ./internal/host/... -count=1
go test ./internal/host -run 'TestPostgresHostNoLibrary|TestStoryLeaseFence|TestStorageInjection' -count=1
go test ./internal/entry/web -run 'TestStorageCutover|TestStoryReaderBackend|TestEventReconnect|TestHostedCapabilities' -count=1
go test -race ./internal/store ./internal/tools ./internal/revision ./internal/host ./internal/comments
```

PG tests require isolated migrated database and `REQUIRE_POSTGRES_TEST=1`, BlobStore test local/MinIO. Race run owned by main verifier once after phase lands, not parallel phase workers. No commands executed in this planning assignment.

## Risk / rollback / done

- **Critical (L cao × I cao):** hidden engine/editor/config reads bypass SQL. Mitigate trace inventory + no-library real round-trip; blockers before audio/deploy.
- **High (L vừa × I cao):** two writers after lease drop. Durable fence verified on every transaction, cancel Host on heartbeat failure, no stale commit after reconnection.
- **High (L vừa × I cao):** changing literary/semantic behavior while adapting IO. Reuse existing domain logic/matching/tests; keep router and prompts unchanged.
- **High (L vừa × I cao):** state on shared factories/cache. Actor immutable per run/request, stable ID keyed metadata only; factories/pools own only clients.

Rollback: stop engine writers and claims, retain DB state/fences; deploy prior DB-compatible release. File return only after08 verified latest reverse-export. Never use scratch content as source for rollback. Gate: real PG engine/reader/tool flows work without mounted library; all14 reader and8 direct constructor sites intentionally migrated/file-only; no `Dir()` business IO on PG; remaining direct IO classified and allowed.07 may begin only after this gate.
