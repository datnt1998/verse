---
phase: 5
title: "Projects, stories, volumes, chapters, versions và dữ kiện truyện"
status: completed (2026-10-29 — hosted library repository real; gate: REQUIRE_POSTGRES_TEST=1 go test ./... green)
priority: P1
effort: 40h
dependencies: [4]
---

# Phase 05 — Chuyển toàn bộ dữ liệu truyện vào repository quan hệ

## Context / requirements

Sau accounts/projects/membership/blob anchors04. [DDL §2–3](./reports/schema-and-contracts.md). `internal/library/library.go:47-107` defines config/layout/story identity; `ops.go:18-49,143-209,214-238` creates/renames via directory operations. `internal/store/chapter_records.go:73-95` accepted chapter has canonical text/hash/facts/style/origin/revision/build. `internal/store/store.go:270-385` links outline/progress; `internal/comments/comments.go:37-101` has pending/sent/fixed/lost/unsure/done and anchor/candidate metadata, chưa có reply UI. File backend remains first-class.

Requested scope: projects/stories/volumes/chapters with immutable versions/author; outlines (flat/layered/premise/retention/compass), characters, world/cast/address/relationships, summaries/snapshots/user rules, comments/replies/reviews/decisions, progress/checkpoints/run/signals/runtime/usage/agent sessions/import state. Không để nửa engine metadata trong thư mục vì phase name chỉ nói chapters.

## Architecture / flow

HTTP/tool input + immutable actor/project/story scope → schema validators → domain repository transaction → relational facts + ordered append streams → commit → filtered event. PostgreSQL transaction không call LLM/S3; khi cần blob source/upload, upload/verify trước rồi attach bằng CAS. `chapter_versions` insert-only, authored by user/agent/import, hash canonical, history never mutate; `chapters.current_revision` deferred FK. Draft edits in `draft_text` + row version until accepted, not overwrite historical version.

`story_state` typed singletons only; per-chapter summaries/plans/snapshots/import artifacts use keyed rows (schema additive section) not rewrite one huge JSON snapshot. Project ideas/rules/style/ledger/config in project_documents; provider credentials encrypted with env key, not plain JSONB. File paths are local adapter details. No user-content filesystem mirror on PG mode.

Read authority: book title/synopsis state follows `meta/book.json` (`internal/store/book.go:18,38-41`); display story title respects `TitleRenamed` (`internal/library/library.go:79-96`). Write book facts updates non-renamed display in same tx. Rename slug preserves UUID/owner/blob keys, emits event, invalidates route cache; no aliases/shims after canonical URL switch except existing UI contract explicitly tested.

## Files / ownership

`R=/Users/dat/Projects/ainovel-cli`.

- Modify `R/internal/library/{library,files,scan,ops,drafts}.go` (source inventory §2 gives IO lines); `R/internal/store/{store,chapter_records,outline,drafts,book,characters,world,cast,progress,run_meta,signals,runtime,checkpoints,session,usage,simulation,decisions,revisions,summaries,user_rules,review_decisions,comment_review,nao_vault,nao_address,nao_relationships}.go`; `R/internal/comments/comments.go:121-140,426-548,895-951`; `R/internal/entry/web/{projects,storyread,comments,quality_api,nao_api,ledger_api,retention_api}.go`; `R/docs/architecture.md`, `R/README.md`.
- Create `R/internal/persistence/postgres/{library,stories,chapters,outlines,characters,comments,reviews,story_state,story_events,checkpoints,agent_sessions}.go`; `R/internal/persistence/migrations/00003_narrative.sql`; `R/internal/library/repository.go`; `R/internal/comments/repository.go`; `R/internal/persistence/postgres/narrative_test.go`; `R/internal/entry/web/narrative_backend_test.go`.
- Reuse/create shared contract tests under `R/internal/store/contract_test.go`, `R/internal/library/contract_test.go`, `R/internal/comments/contract_test.go`; existing `R/internal/store/chapter_records_test.go` and `R/internal/comments/comments_test.go` protect behavior.
- No delete historical files from users' libraries; file adapters retained. Package-local obsolete file-only helper reachable on hosted path removed or relocated to file backend, not kept as live shim. 06 owns remaining engine wiring, not parallel edits here.

## Progress (2026-10-29, completed)

Done: steps 1–3 (shared `narrativetest` suite, 00003 DDL, all repository methods;
`postgres.Backend` now exposes all 19 substores and passes the same suite over
document rows — `TestPostgresBackendNarrativeContract`), step 4 (project/story/
idea CRUD on Postgres — see below), step 5 (immutable attributed revisions,
`legacy_build` stamp, review invalidation on rewrite in the accept transaction;
the file-format v2→3 history saga is NOT implemented — see decision), step 8
(5/5 direct `NewStore` reads go through `Gateway.SetFactStore`; `storyread.go`
uses the hosted reader factory), step 9 (schema/mapping documented in
`docs/architecture.md` + README).

Hosted library (2026-10-29 follow-up, closes step 4 + the phase-05c remaining
list): `internal/persistence/postgres/library.go` is a real
`library.Repository` — not a stub:

- Scope: every entry point runs `memberProject` in the same transaction as the
  read/write (admin bypass; non-member → `ErrStorageNotFound` so another
  tenant's project is indistinguishable from a missing one; write requires
  owner|collab → `ErrStorageForbidden`). `Scan` lists only `ListMembershipsByUser`
  for members, everything for admins, and reports empty
  `Candidates`/`Legacy` (no folders on a hosted library).
- Resolve/CRUD: `ResolveProject`/`Stories`/`ResolveStory` build `library.ProjectRef`/
  `StoryDir`/`Story` with empty `Dir`/`ProjectDir`/`Root` (no filesystem mirror;
  `UpdatedAt` = row `updated_at`). `CreateProject` inserts the row + the actor's
  owner row in one tx (duplicate slug → `library.ErrExists`), `ProjectFile`/
  `UpdateProject` map `name/description/notes/universe_name/created_at`.
- Numbering: `CreateStory` takes `LockProjectRow` before reading
  `ListStoriesByProject`, then `SplitNumbered` max+1 → `NN-<slug>` (same
  `Slugify`/96-rune/temp-name rules as the directory backend; duplicate
  non-temp name → `ErrExists`; concurrent creates get distinct numbers).
  `RenameStory` keeps the `NN-` prefix, `DeleteStory` is a soft delete.
- Story facts: `LoadStoryFile`/`WriteStoryFile` map the story row onto
  `library.StoryFile` (title/shortTitle/titleRenamed/status/tags/publishedAt/
  createdAt) with idea = the linked `project_documents` logical name;
  `WriteStoryFile` locks the row and CAS-updates, preserving the engine-owned
  `synopsis` column.
- Ideas: `project_documents kind='idea'` with the file backend's
  `YYYYMMDD-HHmm-<slug>` names, `# Title` body, first-line title/excerpt rules,
  `-2` dedupe suffix under the project lock, and `usedBy` from
  `ListStoryIdeaRefs` (`stories.idea_document_id`).
- New sqlc queries: `ListProjects`, `InsertProject`, `UpdateProjectFile`,
  `GetProjectDocumentByID`, `GetProjectDocumentByName`,
  `ListProjectDocumentsByKind`, `ListStoryIdeaRefs`.
- HTTP: `libError` now maps `ErrStorageForbidden`→403, `ErrStorageNotFound`→404,
  `ErrStorageConflict`→409, `ErrStorageUnsupported`→501; `GET /api/projects/:p`
  reports an empty `assets` array on hosted (the `/assets` endpoint still 501s);
  `createProject` only mirrors global rules/config when the repository exposes a
  real directory (hosted no longer writes a relative `slug/.ainovel` tree).

Evidence:
- `internal/persistence/postgres/library_test.go`: project scope matrix
  (owner/collab/view/outsider/admin, CreateProject ownership + duplicate/invalid
  slug, file-only capabilities unsupported), story numbering/duplicate/rename/
  soft-delete, 5-way concurrent numbering, story.json parity + idea links.
- `internal/entry/web/library_backend_test.go` (`TestLibraryBackendIDOROverHTTP`):
  hosted gateway (PG accounts + PG library repo + PG fact store) — member reads,
  outsider 404 on project/stories/ideas/meta, stale file-mode grant rejected by
  the repository (read 404, write 403 with the row unchanged), viewer writes
  403, numbered story create, assets 501, story meta from rows, and nil-actor
  rejection by `bootstrap.OpenStoryStore`.
- Fail-before (mutations, each restored): dropping the repository membership
  check makes the stale-grant read return `200` with full story data (IDOR);
  dropping the 501 mapping turns assets into `500`.
- `REQUIRE_POSTGRES_TEST=1 TEST_DATABASE_URL=… go test ./... -count=1` green (no
  skips); `gofmt -l` / `go vet ./...` clean; test container removed (`down -v`).

Decision (logged): the file project format v2→3 chapter-history saga is out of
phase-05 gate scope — it exists only for migrating an EXISTING file project to
hosted and no migration path exists yet; the hosted repository never rewrites
user files. Follow-up (06): `serveProjectCoCreate` draft commits stay file-only
and answer 501 on hosted; the file watcher skips hosted stories (`Dir == ""`).

## Steps

1. Add fail-before cross-backend domain contract fixtures: slug rename/project CRUD/story title sync/flat-layered outlines/volumes/unvolumed chapter/immutable accepted record/CAS/search/comments stale-anchor/review rounds. Include deleted user legacy author display and permission denied vs corrupt file.
2. Implement 00003 DDL exactly with composite FKs to project/story/volume/chapter, comments reply/anchor revision, author attribution, immutable triggers, search indexes, version CAS. Test cross-project chapter→volume/blob/review/comment links rejected even direct SQL. Ensure review scope that is not chapter still works.
3. Implement repository methods covering each of19 facade substores. Move `.io`/private helper assumptions behind file implementation; PG performs proper queries, not JSON path key map. `story_artifacts` keyed rows for plan/summary/snapshot; append-only event tables for checkpoints/session/usage/decisions/relationships/import events. Preserve deterministic sequence and nil/empty distinctions from existing contracts.
4. Implement project/story/idea CRUD, directory-derived numbering under project row lock/unique constraint; preserve existing slug/read IDs including adopted project names (do not apply new-name regex to already adopted IDs). Store local browse capability remains only file mode; hosted list/create uses repo, not os.Stat. Project memberships from04 remain enforced on every operation.
5. Implement chapter draft/accept/rewrite transaction: lock authorized project/user scope→story lease/chapter → expected version/hash → append attributed revision → update pointer/progress/checkpoint → invalidate facts/reviews → save anchor event → commit. Same digest+facts no duplicate revision/checkpoint. File project format2→3 adds immutable `meta/chapter_versions/<chapter>/<revision>.json`; latest record remains authority, saga writes history before pointer/progress and replays idempotently. Account version3→4 is independent04. No invented missing old revisions or false cross-file atomicity.
6. Move comment anchoring pure operations out of file IO. Preserve non-blocking prose contract (`internal/tools/comments_hook.go:10-15`): PG commit atomically writes idempotent reanchor event if inline anchor processing cannot complete; background domain-event processor consumes with cursor and updates actual pending/sent/fixed/lost/unsure/done/candidates/unfixed/moved/relinked fields. No new broker, no dropped warning-only work. Public statuses do not become open/resolved/stale. Audio/review invalidation uses revision/hash not mtime; replies table stores future/explicit API records without inventing new comment UI outside request.
7. Implement revision/history/list/search keyset pagination. `pg_trgm` indexes title/name/content, enforce project filter and deleted flags before LIMIT; no accent-insensitive promise. Do not copy full story for every card; batched metadata/count reads, bounded text queries. Shared factory/facade identity is stable, actor stays request/run-scoped.
8. Wire HTTP narrative factory at direct NewStore sites `internal/entry/web/projects.go:98,1194` and `ledger_api.go:51,193,198` (5), plus story reader factory `internal/entry/web/storyread.go:33-34`; no fallback when PG data missing. Phase06 owns remaining reader consumers/engine injection and lists14 production calls. File CLI constructors remain file intentionally, not parallel migration owners.
9. Document domain schema/soft-delete/author behavior and preserve HTTP envelope fixtures. 06 then replaces engine helpers which still assume dir and child process cwd.

## Fail-before matrix

| Level | Cases | Assertions |
|---|---|---|
| Unit | canonical text/digest/idempotent record, outlines expand/append/revise, title authority | Match file semantic facts, same input no duplicate revision |
| Contract both backends | all19 substore operations, comments anchors/status/replies, reviews rounds/kept decisions, pending state | No domain left unsupported or fake empty |
| DB integration | concurrent draft save/accept/outline update/delete; immutable author/history; composite FK negative | CAS conflict or serial valid outcome; zero cross-project links |
| DB integration | forced failure after revision insert/before checkpoint; source event order | Full rollback; no pointer without version or partial progress |
| HTTP integration | create→rename→write/read/search/comment/review as none/view/collab/owner/admin | Current API field/status expectations and isolation |
| E2E | two sessions edit same chapter, first accept then stale submit | Conflict shown not silent overwrite; old revision still readable |

## Verification commands

```sh
go test ./internal/store ./internal/library ./internal/comments -count=1
go test ./internal/persistence/postgres -run 'TestNarrative|TestChapterVersion|TestNarrativeIsolation|TestOutlineTransaction|TestSearch' -count=1
go test ./internal/entry/web -run 'TestNarrativeBackend|TestComment|TestStory|TestProject' -count=1
```

For postgres run set isolated `TEST_DATABASE_URL`, `REQUIRE_POSTGRES_TEST=1`, migrated schema. New suite must cover19 component interfaces and real SQL; using only package compile/zero selected tests fails gate. E2E automation added09 against built real app, no mock server.

## Risk / security / rollback / done

- **High (L cao × I cao):** omitted metadata breaks resume/agent reads. Mitigate19-component parity manifest and explicit state/event/artifact mapping, pending import/revision fixtures.
- **High (L cao × I cao):** write graph not atomic. Composite FK + row locks + one compound transaction; file keeps known saga semantics.
- **High (L vừa × I cao):** tenant IDOR and stale cache. Explicit actor scope on list/get/write/search/download, immutable UUIDs, no actor in shared fields; SQL negative FK plus HTTP tests.
- **Medium:** trgm index size/unbounded JSON. Index selective text only, keyed artifacts, pagination/batched card queries, no per-request full mirror.

Rollback: before activation choose file, imported source untouched. After PG writes freeze, preserve DB and blobs; code rollback to compatible DB schema preferred; reverse-export latest data via08 before file fallback. Do not delete chapter history or down schema while parent pointer exists. Done:19 substore contracts work with actual Postgres, every narrative endpoint wired/scoped, chapter history immutable/attributed, phase tests and rollback fixture verified; engine still held behind06 gate, not declared migrated early.
