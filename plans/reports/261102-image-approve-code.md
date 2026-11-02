# Image approval — source cause and repair (2026-11-02)

Status: **backend fix verified by full Viderse Go suite and a real local HTTP/PostgreSQL smoke.**
No production job was changed, no image redraw/generation ran, and deployment is pending.

## Verdict

The review CTA **“Duyệt cả video & dựng”** sends `POST /api/clone/{id}/images
{op:"approveAll"}`. That route called `Images.ApproveAll(..., RevisionInput{ExpectedRevision})`
with `ApproveExport` false. Consequences:

1. Images were approved and the manifest was committed — this part worked.
2. The commit had `InvalidateFrom = CloneBuild` and no durable owner-approval intent. The worker
   rebuilt; `build.Handler` always parks at `Waiting: awaiting_review`
   (`pipeline/steps/build/handler.go:576`), so the job returned to `stage_group = review`.
3. The only remaining “approve” affordance on that screen was the same bulk CTA. Every press
   invalidated the parked draft and re-ran the build, while per-image “Dùng bản N” was already a
   no-op because all items were approved. The job could never reach score/export.

The missing piece is the phase-05 owner-intent transition: the approval action must record
`ApproveExport` and open the existing draft through `PrepareCloneExport` (no `accept`), not
invalidate/rebuild the draft.

## Live evidence (read-only, macmini, deployed 9ab528eb)

Deployed `bin/viderse` sha256 matches `viderse-9ab528eb…`; `viderse.commit` =
`9ab528eb3be8206e941fe667e3045302ba012ab6`. Approval files are byte-identical between 9ab528eb and
this worktree. Detailed probes/recovery route: `plans/reports/261102-image-approve-ops.md`.

Only job `22b0b3c8-9821-426f-96fa-2f75aa74b054` (generation 1):

| Observed | Value |
|---|---|
| job | status `waiting`, stage_group `review`, active_step null, no lease, not paused/superseded/cancelled |
| `image_approved_at` | `2026-11-02T00:49:05Z` |
| images step | `succeeded`, revision **19**, 6 items **all `approved`**, `currentVersion=1`, `reviewVersion=null`, no `inFlight` |
| versions | 6 × `ready`, path+SHA256+64-char fingerprint, `refHash == item.reference.sha256` |
| GET images view | `manifest.revision=19`, `approved=true`, `approvedToday=1` |
| build step | `waiting`, attempt **5**, error `{code:"awaiting_review"}`, started 00:59:21Z, finished 01:00:31Z |
| score / export | `pending` |
| notifications | `review_ready` at 00:32:42, 00:50:14, 00:51:35, 00:56:00, 01:00:31 → five build loops |

`images.updated_at = 00:59:21.013Z` and build attempt 5 started `00:59:21.063Z`: the last successful
approveAll committed and immediately invalidated the build. No 4xx is logged on the success path
(`WriteCloneError` logs only unexpected 500s), and none is needed to explain the state.

## Ruled out

- **HTTP rejection / permissions / tenant scope:** approvals persisted for the same job; GET reports
  approved; all reads/writes are job-scoped.
- **Storage / artifacts:** every selected version has path + SHA256; the draft build completed.
- **Fingerprint / revision drift:** `ImagesReady` true, revision 19 served and matching the
  checkpoint; fingerprints present, ref hashes equal.
- **Busy state:** no lease, no running step, no inFlight.
- **Per-image vs all:** per-image approve was irrelevant after the first pass (all approved; UI
  disables “Dùng bản N” when `version === used`).
- **No-op click:** each press is a real commit; only the terminal transition was missing.

## Repair

### 1) The CTA records the owner intent

`apps/viderse/internal/web/clone_images.go:225-230` — `approveAll` passes
`RevisionInput{ExpectedRevision, ApproveExport: true}`. No `accept` flag is involved, so the
quality/score gate is untouched. The handler calls the service once per request; a failed retry
cannot double-publish because the service path below is idempotent.

### 2) Existing approved draft is opened, not rebuilt

`apps/viderse/internal/clone/images/service.go` — `ApproveAll` short-circuits whenever
`model.ImagesApproved(l.manifest)`: every repeat (route, bulk, auto-review, legacy client) calls
`Edits.StageDraftApproval(ctx, id, rawImageRevision(...))` and returns the same revision. There is no
flag/capability branch and no fallback commit.

`apps/viderse/internal/clone/jobedit/service.go` — `Store.ApproveDraft(context.Context, string, int)
error` is now a **mandatory** method on the interface; `StageDraftApproval` returns `error` only and
delegates unconditionally. On success it publishes `clone.job.updated` and wakes the worker. All
`jobedit.Store` implementations (production `*postgres.CloneStore` and every test fake) implement
the method, so no store can silently fall back to the invalidating path.

`apps/viderse/internal/persistence/postgres/clone_bulk_reviews.go` — `ApproveDraft(ctx, jobID,
imageRevision)` in one transaction:

1. locks the job row; rejects superseded/cancelled jobs;
2. `FOR UPDATE` reads the images checkpoint revision and CAS-checks it against the revision the
   service approved (`CloneErrStale` on drift — a concurrent redraw cannot be approved by accident);
3. reads the build step and branches:
   - `waiting` + `awaiting_review`: upsert the durable `clone_bulk_review_intents(job_id,
     image_revision)` row, then run the **unchanged** `PrepareCloneExport` guarded transition in the
     same transaction (build `waiting→succeeded` with `output_manifest` preserved, only `score`/
     `export` reset to pending, job pending, lease/superseded/draft evidence checked). Because the
     transition ran, the intent is deleted in the same transaction so a later dispatcher cannot
     reset score/export a second time;
   - `pending`/`running`: record the intent only; the normal dispatcher opens the gate when the
     draft parks (no extra enqueue, no invalidation);
   - `succeeded`: no-op (gate already open/downstream running);
   - anything else: “Chưa có bản dựng chờ duyệt để xuất”.
4. live lease during `waiting` → intent recorded, transition left to the dispatcher after the lease.

`inTx` reuse is proven at `persistence/postgres/clone.go:52-55`: when `mediaTxKey` is already in the
context, `inTx` calls the callback directly instead of opening a new transaction. `ApproveDraft`
enters via `inTx`, so the nested `PrepareCloneExport` runs in the same transaction and its
`FOR UPDATE` locks/CAS see the intent insert.

Initial approval (not all images approved) still goes through `CommitOwnerApproval`:
`ApproveExport` intent + `InvalidateFrom=CloneBuild`, so the pipeline builds once against the newly
approved manifest. Bulk `approve`/autoreview call `ApproveAll(ApproveExport:true)` and take the same
(staging or initial) path; their existing behavior is preserved.

### 3) Subline tells the technical stage apart

`apps/viderse/internal/domain/clone_view.go` — `waitingAtReview` + `Subline`:

- images `waiting/awaiting_review` → “Chờ bạn duyệt ảnh”;
- build `waiting/awaiting_review` → “Bản dựng đang chờ duyệt” (same owner action, different
  technical stage; not a second decision);
- both parked → image gate wins.

This is the hook the parent can use to keep draft-parking jobs out of the MReview image queue if
needed; no public field was added (`steps[]`/`subline` already exist).

## Regression tests (fail before the repair)

`apps/viderse/internal/clone/images/service_test.go`
- `TestApproveAllInitialApprovalCommitsOwnerExport`: first approval (items in review) reaches
  `CommitOwnerApproval` (`ApproveExport=true`, `InvalidateFrom=CloneBuild`, no rerun) and does not
  stage.
- `TestApproveAllRepeatStagesExistingDraft`: approved manifest + build parked + intent → 0 commits,
  exactly one `ApproveDraft` call with the raw revision (3), revision result unchanged. Before: one
  commit with `InvalidateFrom=CloneBuild`, restarting the draft.
- `TestApproveAllRepeatStagesEvenWithoutExportIntent`: already-approved legacy repeat also stages
  (0 commits), because the branch is no longer flag/capability-dependent.
- Existing `TestApproveAllCommitsBuildInvalidation`, stale scene/revision tests unchanged.

`apps/viderse/internal/domain/clone_redrawing_test.go`
- `TestReviewSublineSeparatesBuildDraftFromImageApproval`: build gate vs image gate vs both.

`apps/viderse/internal/persistence/postgres/clone_bulk_review_test.go`
- `TestApproveDraftOpensExistingBuildDraftWithoutRebuild`: build parked, images approved at rev 2;
  `ApproveDraft` flips build to `succeeded` with **same attempt and same `output_manifest`**, queues
  only score/export, leaves the images checkpoint byte-identical, consumes its intent atomically,
  a subsequent `DispatchReviews` does not re-render, and a low-score run still fails at
  `CloneErrQualityGate` with zero export calls.
- `TestApproveDraftRejectsStaleImageRevision`: rev mismatch → `CloneErrStale`, no intent, build still
  parked.
- `TestApproveDraftKeepsCancelledOrSupersededJobsClosed`: both states reject, no intent, draft
  untouched.
- `TestApproveDraftRepeatedWhileBuildRunningKeepsOneIntentAndAttempt`: two approvals on a running
  build keep one intent at the image revision and leave the build attempt/status unchanged.

Existing durable-intent coverage (`TestBulkApprovalDurablyResumesThroughRenderGateButNeverBypassesLowScore`)
is unchanged and still proves low score is never bypassed. All other `jobedit.Store` fakes
(`cmd/viderse`, backdrop, bundle, metadata, images draw-source) implement the mandatory
`ApproveDraft` method.

## Docs

`apps/viderse/docs/design/v1/07-data-contract.md`
- §2: both review sublines and precedence.
- §14: `approveAll` records the durable owner intent (no `accept`); first approval commits; an
  already-approved manifest with a parked build opens that exact draft through `PrepareCloneExport`
  in the same transaction (build output preserved, only score/export reset), does not rebuild;
  `pending`/`running` leaves the intent for the worker; `succeeded` is a no-op; the quality gate
  still blocks export.

## Parent verification

- `go test -p 2 ./apps/viderse/... -count=1`: all packages passed, with PostgreSQL enabled on an empty isolated test database.
- Actual candidate binary served port8053. A new owned synthetic fixture used existing selected QA images, an already-approved manifest and a parked draft at build attempt5.
- Real `POST /api/clone/{fixture}/images` with `op:approveAll` returned202. PostgreSQL showed build `succeeded`, attempt still5, exact draft output preserved and image checkpoint byte-identical.
- The real worker advanced to `score` (attempt1). This fixture deliberately had no render project, so scoring stopped with `invalid_request` / “Job chưa có project — chạy bước dựng trước”. This proves approval leaves review; it does **not** prove a full media export.
- Repeating the normal POST returned202 without changing any step/checkpoint/output or adding a build attempt.
- Receipt: `/tmp/verse-ui-fix-proof/image-approve-real-api.json`. Exact owned fixture was deleted after capture. No provider call occurred.
- The global queue pause does not stop already-created jobs from being dispatched; the smoke observed that distinction rather than assuming an idle worker.
- Removed the newly added mock-forwarding-only HTTP test; permanent PostgreSQL regressions cover preserved draft, stale revision, cancellation/supersession, durable intent and the low-score export barrier.
- Live stuck job `22b0b3c8-…` is unchanged. After deployment, the fixed normal review action opens the existing selected images/draft without redrawing; scoring/export still use their normal quality gates.

## Files changed

- `apps/viderse/internal/web/clone_images.go` — approveAll records `ApproveExport`.
- `apps/viderse/internal/web/clone_images_test.go` — route regression + fixture capture.
- `apps/viderse/internal/clone/images/service.go` — staged repeat path, legacy no-op, raw revision.
- `apps/viderse/internal/clone/images/service_test.go` — three regressions + fake `ApproveDraft`.
- `apps/viderse/internal/clone/images/autoreview.go` — comment follows the shared approval path.
- `apps/viderse/internal/clone/jobedit/service.go` — mandatory `Store.ApproveDraft` + `StageDraftApproval`.
- `apps/viderse/internal/persistence/postgres/clone_bulk_reviews.go` — guarded `ApproveDraft`.
- `apps/viderse/internal/persistence/postgres/clone_bulk_review_test.go` — draft-reuse regression.
- `apps/viderse/internal/domain/clone_view.go` — build/image subline split.
- `apps/viderse/internal/domain/clone_redrawing_test.go` — subline regression.
- `apps/viderse/docs/design/v1/07-data-contract.md` — §2/§14 contract notes.
- `cmd/viderse`, `clone/backdrop`, `clone/bundle`, `clone/images`, `clone/metadata` test stores — migrated to the mandatory `ApproveDraft` method.

## Privacy

No token, cookie, DSN, prompt, chapter/scene text or media URL is quoted; only IDs, statuses,
revisions, timestamps, error codes and boolean/format checks.
