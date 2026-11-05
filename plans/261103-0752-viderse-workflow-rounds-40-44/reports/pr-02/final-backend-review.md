# Final backend review — PR2 / V36

Status: DONE_WITH_CONCERNS

Scope: read-only review of the PR2 backend diff in `/Users/dat/Projects/verse/apps/viderse` against base `a7341d37db67e7cfafdb8035818f700820a11594`, using the worktree contents (the ordinary Git index is stale/user-owned). Reviewed:

- `internal/pipeline/steps/images/handler.go`, `handler_test.go`
- `internal/editor/projects.go`
- `internal/persistence/postgres/{clone_editor.go,cover_trials.go,clone.go,clone_cutout.go,blobs.go}`
- `internal/media/media.go`
- `internal/pipeline/service.go`
- `internal/pipeline/steps/download/source_cleanup.go`, `internal/editor/media_cleanup.go`, `internal/covertrial/run.go` as Unbind callers
- `reports/pr-02/real-cutout-repair.json`, `real-source-completed.json`, `real-source-export-probe.json`, and nearby receipts
- `reports/pr-02/runs/21-real-cover-native-trial/`, `reports/pr-02/runs/22-real-cover-preview-parity/`

No builds, tests, or formatters were run, per assignment. Findings below are code-reading results; test/gate evidence is treated separately under limits.

## Summary

No concrete functional regression, ownership data-loss bug, or security issue was found in the reviewed PR2 backend diff. The four intended mechanics are internally consistent:

1. **Normalized cutout source:** `handler.go` now derives the cutout from the normalized WebP (`cutout(slotctx, s, webp, cutfile)` at `handler.go:600`) instead of the provider-native PNG. `encode` normalizes to exactly `localItem.Width × localItem.Height` (`internal/pipeline/steps/images/media.go:19-49`), so the cutout geometry now agrees with the saved version dimensions. The raw provider PNG is still published as a separate immutable artifact in the same `s.Publish` (`handler.go:594-606`), and `version.Path`/`version.SHA256` still point at the normalized WebP. Build validation independently checks that the cutout is PNG and has `version.Width × version.Height` (`internal/pipeline/steps/build/validation.go:356-374`), and the on-demand cutout path already derives from `v.Path` with the same dimension check (`internal/clone/images/cutout.go:102-141`). No extra provider call is introduced. Current runtime evidence now closes the previous end-to-end gap: `real-source-completed.json` records the same QA job succeeding through build/score/export after the saved-derivative repair (`savedDerivativeRepairUsedAI: false`, one provider image), and `real-source-export-probe.json` reads the MP4 (H.264 1920×1080 + AAC, 6.000 s, 630653 bytes). The paid cover trial is separate: `native-cover-ready.json` shows a real provider cover trial at 1672×941; that raw provider PNG is intentionally the native sample, not the final 1280 cover, so it is not a recurrence of the cutout geometry defect.
2. **Deletion ownership:** `BlobRepository.Unbind` now refuses to remove a binding while `storage_live_refs` still names the path (`internal/persistence/postgres/blobs.go:240-250`). This makes shared editor/media/cutout ownership safe. `DeleteJob` retires the job's own editor project heads via the canonical editor delete path before releasing its artifacts (`internal/persistence/postgres/clone.go:1203-1220`, `clone_editor.go:39-83`), and `deleteCutouts` removes only the job's cutout rows before checking the binding (`clone_cutout.go:36-95`). `DeleteJobWithCutouts` runs inside the media publication transaction, so a failure in either phase rolls the whole deletion back.
3. **Autosave/read race:** `editor.Store.readWorkspace` captures an identity (`version`, `revision`, `path`, `absent`) at the same read that loads the head, and retries a missing read only when the head identity actually changed (`internal/editor/projects.go:97-175`). It loops by real head changes, checks `ctx`, and does not retry non-missing errors or an unchanged head. The new tests cover successive head moves, steady-head byte loss, unknown ID, and deleted project.
4. **Contract compatibility:** `ServiceStore.RequestCancel` was replaced by `CancelJob`; the only production implementation is `CloneStore` (`internal/persistence/postgres/clone.go:1028-1062`), and test doubles embed `ServiceStore` / implement the full surface, so no incompatible implementer was found. `CancelJob` atomically writes the cancel request, closes open steps, clears `paused_at`, and writes `cancelled`. `SetJobStatus(cancelled)` also clears `paused_at`, and `Service.Delete` rejects paused jobs with `ErrConflict` (`internal/pipeline/service.go:396-410`). The `cover_trials.go` prompt-sample insert now has 22 placeholders for 22 columns/arguments (`cover_trials.go:329-336`), fixing the prior 21/22 mismatch.

## Verified concerns (edge / residual)

### 1. Superseded paused job can remain undeletable (low severity, verified logic)

The new Delete guard rejects any job with `PausedAt != nil` (`internal/pipeline/service.go:404`). Two existing writers can leave `paused_at` set while the job becomes terminal:

- `SupersedeCloneJob` sets `status = 'cancelled'` but does not clear `paused_at` (`internal/persistence/postgres/queries/clone.sql:25-28`).
- A job can be paused and later superseded: the restart/supersede admission check only excludes `pending`/`running` status and a live lease (`internal/persistence/postgres/clone.go:483`); a `waiting` or `failed` paused job passes that check. `Service.Pause` itself does not reject superseded/terminal jobs (`service.go:358-365`).

Result: such a job returns `409` from Delete until `Cancel` is called (CancelJob clears the pause), and the UI may not offer Cancel for a job whose `StageGroupOf` is empty because `SupersededAt` is set. This is not the normal paused→cancel→delete path covered by the new tests. Recommend clearing `paused_at` in the supersede transaction or exempting superseded/terminal jobs from the pause guard; verify the UI path for superseded paused jobs.

### 2. Stale/contradictory evidence receipts (not a source bug)

The current worktree does not match the evidence files in several concrete ways:

- `delete-postgres-regression.txt` reports `TestDeleteJobRetiresEditorHeadsBeforeReleasingBindings` and `TestDeleteJobRollsBackRetiredHeadsWhenReleaseFails`; those names do not exist in the current tree. The current tests are `TestDeleteJobRetiresOwnProjectAndKeepsSharedEditorMedia`, `TestDeleteJobRollsBackRetiredProjectHeads`, `TestCanonicalMediaDeleteRetiresLastOwnerHeadAndBinding`, `TestDeleteJobRejectsPausedVideo`, and `TestCancelPausedVideoClearsPauseSoDeleteSucceeds`. The receipt is therefore not evidence for the reviewed test file.
- `unit-gates.json` names proposed source tree `065c281f...`; `proposed-source-check.json` names `7b8266...`. Neither is the reviewed worktree state, and `viewer-and-actions.md` itself notes the backend cutout/harness changes came after that tree. The aggregate Viderse count cannot be bound to this backend diff.
- `real-cutout-normalization-blocker.json` still says `"productionFixApplied": false` and describes the exact cause now fixed in `handler.go:600`. The blocker report is stale relative to the current source.
- `backend-full-gate.json` is a **failed** full-backend run (`TestRivalVideosEndpointLatency`, p95 1371.48 ms > 400 ms). If that is an Explore latency gate, design #83 permits deferral to the final release; it still means there is no passing full-backend gate and no deploy claim.
- Newer cover evidence is current and does not rely on the stale receipts: `runs/21-real-cover-native-trial/native-cover-ready.json` is an actual paid cover trial (`costImages: 1`, ready 1672×941, raw provider sample), and `runs/22-real-cover-preview-parity/interaction-results.json` reports 4 expected/4 passed browser cases (desktop, desktop-reduced, mobile, mobile-reduced) with 0 unexpected. These cover the cover-history/preview claim; they do not replace the missing current-tree receipt for the PostgreSQL read-race/delete tests.

### 3. Repair receipt is a one-off publication, not a production migration

`real-cutout-repair.json` records one CAS publication (`version: 2`, before `1672×941`, after `1920×1080`, `aiCalls: 0`, `checkpointCAS: true`, `originalImmutableCutoutPreserved: true`). The later runtime receipts (`real-source-completed.json`, `real-source-export-probe.json`) confirm that this repaired saved image actually builds and exports successfully, so the fix/repair chain is no longer only a JSON assertion. However, there is still no remaining repair helper or production endpoint in the reviewed source that reproduces this for arbitrary saved versions (searched backend Go/TS for cutout repair/normalization). Consequences:

- The receipt lacks logical paths and the resulting checkpoint revision, so the publication/CAS cannot be independently reproduced from the repository, and the “original immutable cutout preserved” field has no path/hash for the old derivative.
- Existing saved derivatives produced before this fix are not migrated by generic production code; they require redraw or an explicit repair operation. That is within the stated “one actual QA CAS publication” scope, but it is a real product-data limit and must not be presented as a general migration.

### 4. `Service.Unbind` is now best-effort; one local-file caller lacks a post-commit live recheck [INFERENCE]

The new live-ref guard means `Media.Unbind` can legally return success while leaving a binding in place. `pipeline.retireLocalCutouts` correctly rechecks the binding before deleting local bytes (`internal/pipeline/cutout_retirement.go`), but `editor.Service.unbindPaths` resolves local paths **before** calling `Unbind` and then deletes each resolved local file unconditionally (`internal/editor/media_cleanup.go:17-62`). I did not find a concrete production caller where the path is still live after commit (the `DeleteMedia` caller pre-filters `Roots`, and export paths are per-export), so this is not counted as a verified failure. It is a residual ownership risk worth a concurrent-owner test before relying on the new “best-effort” contract in editor cleanup.

## Repository-state note

`internal/persistence/postgres/cover_trials.go` and `internal/persistence/postgres/clone_editor.go` are untracked in the worktree while the ordinary index stages a deletion of `cover_trials.go`. The reviewed content is the worktree content and is correct, but a normal index-based commit would omit the placeholder fix/new helper. This is consistent with the stated stale/user-owned index and is recorded here only so hand-off does not treat the index as the source of truth.

## Limits / not verified

- No builds, tests, or linters were executed by this review.
- No current-tree raw receipt exists for the read-race/delete tests; the 9-case PostgreSQL claim in `viewer-and-actions.md` is not backed by a receipt whose test names exist in this worktree. The new cover/preview receipts do not change that.
- Cover history/preview now has current browser evidence (4/4 parity) and an actual paid cover trial; the raw 1672×941 cover PNG is intentionally native, not the final 1280 cover.
- I did not verify the real QA repair publication independently beyond the JSON receipt (no helper, no paths, no checkpoint revision in the receipt); the completed export is runtime evidence, not a reproducibility proof.
- The full backend gate remains failed; release/deploy gating is outside this read-only review. Phone budget 202.5 ms/200 ms remains an approved deferred release gate.
- All 2/2 authorized paid images are consumed; no further paid provider calls should be made from this work.

## Bottom line

The reviewed backend changes are coherent: normalized-source cutout, ownership-aware unbind, owner-first editor retirement with transactional rollback, cause-specific read retry, and the atomic cancel/delete contract. No concrete code bug was proven in those paths. The actual saved-image repair→build→export now succeeds, and cover preview parity is 4/4 with a real paid cover sample. The material concerns that remain are evidence hygiene for the older PostgreSQL/Go receipts, the one-off (non-migrating) nature of the cutout repair, the supersede/pause deletion edge, and the residual `unbindPaths` live-owner risk. None of these should be silently presented as a passing full backend/release gate.

## Parent disposition after the review

- Supersede/pause: reproduced with a real PostgreSQL restart of a paused waiting generation. `SupersedeCloneJob` now clears `paused_at` in the same transaction; `gen/clone.sql.go` was regenerated with sqlc1.31.1, not edited by hand. `TestRestartPausedVideoRetiresPauseSoOldGenerationCanBeDeleted` failed before the fix and passed after it. Current delete/read PostgreSQL run passes10/10; exact current test names/output are in [delete-read-postgres-current.txt](delete-read-postgres-current.txt). Older renamed-test receipts remain historical, not current-source proof.
- Source identity: [proposed-source-check.json](proposed-source-check.json) now records integrated tree `8406d47d0414fc174f20bfaee3e3d15077323180`; a fresh scoped archive passed typecheck. The ordinary index remains untouched. Backend test receipts and frontend test receipts are separate evidence, not interchangeable gates.
- Original cutout blocker JSON is intentionally retained as the pre-fix finding. It is superseded by [repair](real-cutout-repair.json), [completed source](real-source-completed.json) and [decoded export](real-source-export-probe.json); it does not describe the current pipeline source.
- Repair scope remains one actual QA saved derivative published through CAS from the already-normalized image. No general production migration has been claimed or added. Before deployment, inspect the actual production data for affected legacy cutouts; use approved #82 repair only for records actually found, retaining immutable originals.
- Legacy `editor.unbindPaths` concern is not treated as a proven PR2 regression. The changed DeleteMedia caller resolves canonical media and filters `Media.Roots(path)` before unlinking; `BlobRepository.Unbind` also checks `storage_live_refs`. Current PostgreSQL tests cover the shared-owner deletion path and rollback. Other historical unbind callers are not claimed as concurrent-owner-tested.
- Full backend latency remains FAIL1371.48ms/400ms and phone budget is now FAIL202.7KiB/200KiB (units corrected: phone budget is bytes, not milliseconds). Both are deferred only until the final deploy gate under approved #83; no release/deploy pass is implied.
