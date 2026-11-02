# Image approve blocked — live ops findings (2026-11-02)

Read-only diagnosis. No source, deployment, restart, credential or production job/user state was
changed. No POST approve/retry/generate/export was sent. Public reads used the existing owned QA
admin cookie map (`/tmp/verse-perf-followup/session.json`, user `qa-perf-d4cab616`, viderse admin);
no login, reset, or impersonation. No user screenshot was provided for this report, so the report
does not claim any particular UI surface beyond server-visible state.

## Verdict

Approval is **not being rejected**. The one production job has all images approved and persisted,
but it is parked in `stage_group=review` with the subline "Chờ bạn duyệt ảnh" and it cannot advance
from the review screen: every `op:"approveAll"` re-invalidates the `build` step, the build publishes
another draft and stops again at `awaiting_review`, and the job lands back in review.

Live proof at report time: `approved=true`, manifest `revision=19`, all 6 items `status=approved`,
`image_approved_at=2026-11-02T00:49:05Z`; `clone_steps.build` = `waiting` /
`{"code":"awaiting_review"}` attempt 5; five `review_ready` notifications for the same job between
00:32:42Z and 01:00:31Z.

Code path at the deployed revision: `POST /api/clone/{job}/images {op:"approveAll"}` →
`CloneOperations.images` → `Service.ApproveAll(RevisionInput{ExpectedRevision})` with
`ApproveExport=false` → `Edits.Commit(..., from=CloneBuild, rerun=false)` unconditionally, even when
the manifest was already approved → build is invalidated and re-runs → `build` handler returns
`Waiting: awaiting_review` ("Bản dựng đang chờ duyệt") on every run → `StageGroupOf` maps that to
`review`, `Subline` renders "Chờ bạn duyệt ảnh". The images-only review ops cannot open the export
gate; only `ApproveExport=true` (bulk action `approve`) or `POST /api/clone/{job}/export
{"accept":true}` (LiveStudio) reaches `PrepareCloneExport`.

The exact HTTP status/body per user attempt is **not available**: the app logs only unexpected 500s
(`clone request failed`), not 4xx; launchd `viderse.log` contains no request lines; cloudflared logs
only tunnel errors. Persistence proves the commits that happened, not the wire bytes.

## Deployment revision (observed)

| Fact | Value |
|---|---|
| Public origin | `https://vid.aarverse.studio` (`/readyz` → 200 `{"ok":true}`) |
| launchd | `gui/501/com.dat.viderse`, state running, start 2026-11-02 06:14:08 +07 |
| Program | `~/server/bin/viderse-run.sh serve` (binds 127.0.0.1:8789, `PUBLIC_URL=https://vid.aarverse.studio`) |
| Running binary | `~/server/apps/viderse/bin/viderse`, sha256 `038b241ac1db4d1f1c509a89f1d344d3fe024e43efdfa866e9593bf3dd213141` |
| Built revision | `bin/viderse.commit` = `9ab528eb3be8206e941fe667e3045302ba012ab6`; same hash as `bin/viderse-9ab528eb…` (mtime 2026-11-02 06:14) |
| Source checkout | `~/server/src/verse` HEAD moved from `c6f425e7` to `4198f67` during the probe (Noverse paired release); `git diff --name-only 9ab528eb…HEAD` lists **0** files under `apps/viderse` |
| Viderse delta | `9ab528eb…HEAD` contains **no** `apps/viderse` changes → deployed Viderse code equals current repo source |

No Viderse rebuild/restart is part of the Noverse-only paired release; this route stays on
`9ab528eb…` until an explicit Viderse deploy. The public origin is proven to be this process: the
QA-session `GET /api/clone/…/images` over `https://vid.aarverse.studio` returns the same job id and
`revision=19` rows read from the local `viderse_go` database.

## Job / queue state (live Postgres `viderse_go`, metadata only)

Only one video job exists (`clone_jobs` total 1, review 1, superseded 0).

| Field | Value |
|---|---|
| job | `22b0b3c8-9821-426f-96fa-2f75aa74b054` (channel `aa72b6a1-7114-4a46-9ed5-539216fb81fb`) |
| status / stage_group / active_step | `waiting` / `review` / empty |
| generation / version / fence | 1 / 58 / 11 |
| superseded / paused / cancel_requested | no / no / no |
| worker lease | none (`lease_owner` null, `lease_expires_at` null) |
| image_approved_at | 2026-11-02T00:49:05.517519Z |
| created / updated | 2026-11-02T00:23:18Z / 2026-11-02T01:00:31Z (08:00:31 +07) |
| queue row | `clone_queue` state `started`, priority `pick`, started 00:23:18Z (no queued rows) |
| auto_fix / auto_review | null / null |

Steps (`clone_steps`):

| step | status | attempt | note |
|---|---|---|---|
| download, backdrop, transcript, metadata, scenes | succeeded | 1 | inputs complete |
| images | succeeded | 1 | output `{"images":6,"manifest":"images/manifest.json","revision":19}`; row last written 00:59:21Z by the last approve commit |
| build | **waiting** | **5** | error `{"code":"awaiting_review","message":"Bản dựng đang chờ duyệt"}`; started 00:59:21Z, stopped 01:00:31Z |
| score, export | pending | 0 | never reached |

`GET /api/clone/{job}/images` (200, QA admin): `approved=true`, `manifest.revision=19`,
`sceneRevision=1`, `sceneGeneration=1`, `waiting=null`, `approvedToday=1`.

| item | kind | status | currentVersion | reviewVersion | versions (v / path / sha256 / sourceFingerprint) |
|---|---|---|---|---|---|
| sc-000 | scene | approved | 1 | null | 1 / present / present / present |
| sc-001 | scene | approved | 1 | null | 1 / present / present / present |
| sc-002 | scene | approved | 1 | null | 1 / present / present / present |
| sc-003 | scene | approved | 1 | null | 1 / present / present / present |
| sc-004 | scene | approved | 1 | null | 1 / present / present / present |
| thumbnail | thumbnail | approved | 1 | null | 1 / present / present / present |

`GET /api/clone?limit=10` returns this job with `status=waiting`, `stageGroup=review`,
`subline="Chờ bạn duyệt ảnh"`, `version=58`; counts `{review:1, ready:0, attention:0}`.
No rejected/failed step, `error_events` holds one unrelated `task_done` (the app's own
`GET /api/errors` "Lỗi gần đây" feed returns `count7d:1`, that same row — no approval error was
recorded), anonymous `GET …/images` returns 401 `session_ended` (auth gate works), `/readyz` 200.

## Observed timeline (UTC; +07 in parentheses)

| time | event | source |
|---|---|---|
| 00:23:18 | job created, queue started | `clone_jobs` / `clone_queue` |
| 00:32:42 | `review_ready` #1 — images done, first draft build waiting | `notifications` |
| 00:49:05 | approval persisted (`image_approved_at`), images approved | `clone_jobs` |
| 00:50:14 | `review_ready` #2 (build attempt 2 waiting) | `notifications` |
| 00:51:35 | `review_ready` #3 (attempt 3) | `notifications` |
| 00:56:00 | `review_ready` #4 (attempt 4) | `notifications` |
| 00:59:21 | images row written again by approve; build attempt 5 starts | `clone_steps` |
| 01:00:31 | `review_ready` #5; build waiting; job updated; nothing since | `notifications`, `clone_jobs` |
| 03:49Z (10:49 +07) | probe time — still waiting/review, no lease | live Postgres + API |

Five `review_ready` rows for this one job = five build attempts that each ended at
`awaiting_review`; the four post-approval attempts are exactly the re-draft cycles produced by
repeated `approveAll`. The last press demonstrably committed (images row rewritten at 00:59:21Z),
so it was **not** a 4xx rejection; it was a successful write that could not change the stage.

## Request handler, status codes, actor permissions

Route (deployed): `POST /api/clone/{id}/images`, dispatched by
`MountCloneOperations` → `CloneOperations.route` → `ops.images`; body
`{"op":"approveAll","expectedRevision":N}` or per-image `{"op":"approve","version":V,"expectedRevision":N}`.
Response is `202 Accepted` with `MutationResult{jobId,revision}` (approveAll) or
`ReviewResult{approved,chosenChanged,invalidated,kicked,manifest.revision}` (approve); errors use
`cloneErrorWire{error,code,recovery,retryAt}`.

Status mapping (`WriteCloneError`): `invalid_request`→400, `not_found`→404,
`conflict`/`cancelled`/`stale`/`template_changed`/`quality_gate`/`awaiting_review`→409,
`quota`→429, `tool_missing`/`interrupted`→503, `source_unavailable`→410, anything else→500 and
**only that 500 is logged** (`slog.Error("clone request failed")`).

Gates that can reject an approve (all would be 409/400 before any write, and none is logged):
revision mismatch (`Danh sách ảnh đã thay đổi; tải lại trước khi sửa`), busy step
(`Ảnh hoặc bước phụ thuộc đang chạy`, `Video đang xử lý — chờ xong rồi thử lại`), scenes running or
stale, insufficient/unsupported image versions, snapshot CAS conflict. At the observed revision
(19) with no lease and no running step, none of these applies to the current state.

Actor: any authenticated `verse_session`; `/api/clone/…` is not in `machineAdminOnly`, so a
non-admin member with the viderse grant can approve too (handler ignores `identity.User`; no
per-channel ACL on this path). Missing/invalid session → 401 `session_ended`; missing viderse grant
→ 403 from the platform gate. QA user is global admin, so both would pass.

## What is proven vs what is missing

Proven by server state:
- All six images approved at revision 19; `image_approved_at` set; no stale/conflict state remains.
- The job is not paused, superseded, cancelled or leased; no failing step, no 500 in logs.
- The build step reached `awaiting_review` five times, i.e. the job kept re-entering review after
  approvals; the last approve wrote the images row at 00:59:21Z.
- Source at the deployed revision re-invalidates `build` on every `approveAll` and the build step
  always returns Waiting, so this screen cannot move an already-approved job forward.

Missing / not recoverable (stated, not fabricated):
- Exact HTTP status, body and timing of the user's last press(es): **not logged** (no access log,
  only 500s logged; cloudflared logs tunnel errors only). After 01:00:31Z there is no server-side
  write, so any later press either never reached the server or was rejected pre-commit (4xx,
  unlogged). Revision-cache staleness on the client cannot be confirmed or excluded from server
  state — that requires the browser/network trace.
- The user's screenshot/UI surface was not provided; the review screen (`ReviewScreen.tsx:396`,
  `MReviewScreen.tsx:190`) is the only surface whose approve maps to `approveAll`, and both loop.
- Whether a duplicate/concurrent press produced a 409 CAS toast is unknowable from these logs.

## Safe recovery after the fix (no image redraw)

Do not press approve again before the fix: it only restarts the build (another attempt) and returns
to review. The job needs one explicit draft/export approval, which is the designed phase-05
transition and touches no image and no provider:

1. Confirm the fixed Viderse build is actually live (`bin/viderse.commit` + binary sha256 + process
   start newer than the deploy), and `/readyz` 200.
2. Read the current image revision and ensure the same six images remain approved and the current draft is still parked. Do not assume revision19 if another owner action occurred.
3. Use the fixed normal review action: `POST /api/clone/{job}/images` with `{"op":"approveAll","expectedRevision":CURRENT_REVISION}`. Expected202. It records owner intent and opens the existing draft without invalidating/rebuilding it.
4. Verify read-only: build becomes `succeeded`, its attempt/output stay unchanged, images/checkpoint stay unchanged, and the worker advances to score/export or reports an actual downstream error.
5. Do **not** use `export {"accept":true}` as a recovery shortcut: that would override the quality decision. The fixed review route deliberately preserves normal score/export gates.
6. No production recovery write has been authorized or performed in this report. The original approved images and draft remain untouched.

Avoid: restarting the app (state is durable), cancelling/retrying the job, redrawing images, or
pressing approve repeatedly — none of these can open the gate and the first two discard context.

## Status

- Cause of "cannot approve images" identified in source and corroborated by live persisted state:
  successful approval followed by an unconditional build re-draft that returns the job to
  `review`. Not a rejection, not a stale row, not a broken image or quota.
- Production is on `9ab528eb…` (= current Viderse source); no Viderse change is deployed yet.
- Exact per-press HTTP status is missing by design (4xx not logged); the successful commit at
  00:59:21Z plus zero writes afterwards is the maximal server-side evidence.
- Recovery is one explicit export/draft approval after the fix is live; it does not redraw images.
  No production write was performed for this report.
