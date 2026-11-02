# Admin project visibility — live + source findings (2026-11-02)

Read-only production diagnosis plus a minimal client-lifecycle repair. No role, membership, ownership,
deletion, job, worker, or paid-call state was changed. No secrets or chapter prose are recorded here.

## Verdict

The server is **not** hiding projects from the admin. `dat.nguyen` resolves as a global admin, and the
hosted repository already returns every active project for that identity (verified live with the
disposable QA admin, which has the same admin bypass and no memberships). The only source-proven path
that can show an admin *fewer* projects than the server holds is the desktop `LibraryProvider`: it is
the shell's project list, it sits outside the shared `@noverse/ui/data` cache, and it used to fetch
only on mount/user change — never on `library.changed` / `project.changed`, reconnect, or focus. An
open tab therefore keeps the project set it fetched at mount indefinitely.

## Live evidence (production `nov.aarverse.studio`, image `c6f425e7`, PostgreSQL)

| Check | Observed |
|---|---|
| `users` row for `dat.nguyen` | `global_role=admin`, `status=active`, `deleted_at=NULL`, `must_change_password=false`, `version=1` (admin since creation, no role change) |
| Effective app grants | admin/admin (global admin is implicit; no `app_grants` rows needed) |
| Active projects | **2** — `dit-dang-audio` (created 2026-11-01 09:20 UTC), `tron-audio` (created 2026-10-31 05:28 UTC) |
| Soft-deleted projects | **1** — `qa-reader-smoke` (QA-owned smoke project; deleted 2026-11-01 23:56 UTC by the QA cleanup, together with its QA account) |
| `dat.nguyen` memberships | `tron-audio: owner` only; no row for `dit-dang-audio` |
| Project membership counts | Two memberships per active project; the admin bypass does not require a membership row |
| Live API as disposable QA admin (global admin, no memberships) | `GET /api/library` 200 → `dit-dang-audio`, `tron-audio`; `GET /api/projects` 200 → both; project detail 200 for both. Server admin bypass works. |
| Active `dat.nguyen` sessions | Mac Chrome since 2026-11-01 15:23 UTC, iPhone Chrome since 14:36 UTC (both after project creation); older Mac tab since 06:29 UTC |
| Production library root | `NOVERSE_LIBRARY_ROOT=/var/lib/noverse/library`, empty; hosted mode resolves from PostgreSQL (`storageMode=postgres`) |
| Legacy file-mode library | `~/NoverseLive` (also served by the local 8790 binary) contains only `tron-audio` with 2 stories — no unmigrated user project |

Missing count if the visible set is only `tron-audio`: **1 active project** (`dit-dang-audio`). The
soft-deleted QA project is not a user project and is deliberately excluded.

## Source trace

1. `apps/noverse/internal/persistence/postgres/library.go` `Scan`: `act == nil || act.isAdmin()` returns
   every non-deleted project. Deployed `c6f425e7` matches this tree.
2. `apps/noverse/internal/entry/web/projects.go` `serveLibrary`: per-project `user.CanReadProject`, and
   `platform/identity/store.go` `ProjectRole` returns `owner` for `RoleAdmin`. Admin cannot be filtered
   here. The handler filter is still required for the file backend, so it is kept.
3. `apps/noverse/web/src/api/library.tsx` `LibraryProvider` holds the desktop project list used by
   `ProjectPopover`, `AppHeader`, `ContextCrumb`, the settings/members picker and project fallbacks.
   It fetched on `[user, tick]` only.
4. `apps/noverse/web/src/data/Provider.tsx` invalidates the shared cache via
   `keysForEvent` (`library.changed`/`project.changed` → `keys.library()`), but that cache is not the
   `LibraryProvider` state, so the invalidation never reached the desktop list.
5. Project creation through the HTTP post-hook emits both events; a CLI/import/`EnsureProject` writer
   emits none in the serving process. Either way, an already-open desktop tab had no refresh path.

## Repair (4 files, frozen)

| File | Change |
|---|---|
| `apps/noverse/web/src/api/library.tsx` | Refresh on `library.changed` + `project.changed` (250 ms coalescing), `events.onReconnect`, and visible focus (15 s throttle). The existing `alive` guard still cancels the previous list request on user/tick change, so a stale response cannot overwrite a new user's list. |
| `apps/noverse/web/src/data/Provider.tsx` | On SSE reconnect, re-read `/api/me` so a role/project-grant change missed while offline cannot leave permission filters on the pre-drop identity. |
| `apps/noverse/web/src/api/library.test.tsx` | Event/reconnect/focus refetch regression; existing lazy-detail test kept. |
| `apps/noverse/internal/entry/web/library_backend_test.go` | Admin sees an added project with **no** `project_members` row; the same member still sees only its own project and cannot open the unowned one. |

No `Cache-Control` change, no authorization filter removal, no role/ownership/undelete.

## Verification

Verified in isolated tree `82d12c4dfd34d718606c53d552de804e2fc1b397`,
without the in-flight voice/mobile changes:

```sh
npm --prefix apps/noverse/web test -- src/api/library.test.tsx --pool=threads --maxWorkers=2
REQUIRE_POSTGRES_TEST=1 go test -p 2 ./apps/noverse/internal/entry/web \
  -run TestLibraryBackendIDOROverHTTP -count=1
npm --prefix apps/noverse/web test -- --pool=threads --maxWorkers=2
npm --prefix apps/noverse/web run build
go build ./apps/noverse/cmd/ainovel-cli
```

All passed: focused frontend 2 tests; PostgreSQL admin/member HTTP regression;
full frontend 223 files / 1,867 tests; typecheck/build and Go binary build.
The permanent frontend regression checks the consumer's visible project IDs
after each event, reconnect and focus, not just request counts.

Native browser smoke used the actual compiled candidate (`index-E0YTRwhu.js`)
and its real loopback PostgreSQL API. With the project picker already open,
creating one randomly named owned QA project through the normal HTTP endpoint
changed the visible options from **2 to 3 without page reload**; exactly one
additional library fetch, no browser errors and zero provider calls.
Receipt: `/tmp/verse-ui-fix-proof/urgent-admin-live-refresh.json`.

## Residual uncertainty

`dat.nguyen`'s own session cannot be read without impersonation. If their tab was loaded after the
project was created and still showed one project, the client lifecycle repair addresses the only
source path found, but their exact screen/host should be confirmed in the user smoke. No server-side
authorization or data repair is indicated.
