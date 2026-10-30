# Code Review Summary — phase 02 machine/storage

## Scope and evidence

- Review target: uncommitted work against `HEAD`, including new/untracked files. Source and git state were not modified. This report is the only file written.
- Primary scope: `apps/viderse/internal/{storage,retention,activity,toolsvc,setup,logs,domain}/`, their requested HTTP and Postgres adapters, and `cmd/viderse/{serve,storage_wiring,machine_wiring}.go`. Relevant tools/process code and web clients were traced as dependents, not reviewed as independent feature owners.
- Scoped Go footprint: **12,387 lines including tests** from `wc -l`; this is not a changed-LOC count. `internal/web/screens.go`, Postgres `tools.go`, and migrations 00005/00006 were absent. The discovered migrations end at 00004.
- Read the structure/performance rules in `plan.md`, phase-02 acceptance, all six requested agent reports, design contract §11/§15/§23/§24/§24.1 and V7/V9/V17/V22/V23/V24. Compared retention and media-GC behavior with yt-factory commit `2c201e6` using local `git show`.
- Scout performed before systematic review: traced destination/source relationships, settings cutover writes, all recursive removals, pending-media input, receipt protection, task ownership, cache sharing, startup/shutdown ordering, and real HTTP/client states. These paths produced the findings below.
- MachineSetup is in flux: its report says composition and HTTP tests are unfinished, but the inspected tree already has `machine_wiring.go`, mounts in `serve.go`, and `web/machine_test.go`. Findings describe that inspected source, not the stale report. No full-tree or final-revision verification is claimed.
- **Evidence convention:** source statements and the test results below were observed. Production consequences and trigger interleavings described in findings are **[INFERENCE] from the cited source**, not independently executed reproductions. No network/paid calls, servers, git mutation, or destructive reproduction was performed.

## Overall assessment

**Do not commit this as production-ready.** Destination overwrite, incomplete move crash recovery, cleanup traversal/fail-open behavior, and shutdown ownership are blockers. Happy-path service tests pass, but omit the key failure windows. The new-machine UI also cannot start installation with the real backend state.

## Critical issues

### C1 — A move overwrites existing destination data before the move succeeds

**Evidence:** `apps/viderse/internal/storage/move.go:387-391,398-430`.

Each top-level entry is committed immediately. For files, `commitEntry` calls `os.Rename(staged, final)`, replacing an existing file on macOS/Linux; for a directory/file mismatch it explicitly removes the existing destination. There is no collision policy or rollback. If a later group fails or is cancelled, only `.viderse-incoming` is removed (`move.go:240`): already replaced destination files are gone and cancelled partial groups remain. Moving into a previous project folder or an ordinary nonempty folder can therefore destroy the destination version without a completed move.

**Fix:** reject conflicting destination content before mutation, or stage a complete destination plus rollback-safe journal and commit only after every group verifies. Never replace unowned destination files as an implicit merge.

**Regression test:** populate source and destination with different bytes at the same relative path, fail/cancel a later group, and assert both original trees and settings remain byte-for-byte unchanged.

## High priority — blocking

### H1 — The move journal cannot recover rename or settings-cutover failures

**Evidence:** `storage/move.go:255-272,319-326`; `storage/recover.go:14-29`; `storage/service.go:132-137,331-339,632-656`; `domain/storage.go:62-69` (paths relative to `apps/viderse/internal/`).

Same-volume rename removes the old root before `storage.location` changes. A crash in that window leaves the persisted root missing; startup recreates an empty old root **before** recovery, and recovery merely deletes staging and clears the marker. It never adopts the complete target. In the copy path, `previous` is armed for deletion and the move marker cleared **before** `setLocation`. A failed location write leaves the still-active source scheduled for recursive deletion seven days later; new work can continue there and then be lost. The marker has neither source nor commit phase to distinguish these states.

**Fix:** persist source, destination, and durable commit phase; recover before creating a root. Make location/previous/move-state cutover transactional, and never make the active root eligible for deletion. Keep a recovery marker until the cutover is committed.

**Regression test:** inject failure/crash after rename, after previous-write, after marker-clear, and during location-write; restart, advance seven days, and verify the selected root and all subsequently written data survive.

### H2 — There is no safe relationship/ownership validation for selected roots

**Evidence:** `storage/move.go:40-74,122-133,333-347,379-387`; `storage/service.go:408-421`; `storage/move.go:199-212`; `storage/service.go:652`.

Only exact equality with the current root is handled. A target below the source is accepted. Staging is then created inside the source tree and the source enumeration includes the target, so recursive copy copies its own growing staging tree. Conversely, any existing non-project directory can be adopted by `new_only` simply by writing a marker, then later treated as an entirely app-owned source and recursively deleted as `previous`. Home/config/source ancestors and unrelated user files are not protected.

**Fix:** use one authoritative root validator in both check and mutation paths. Reject ancestor/descendant overlap, home/config/system roots, and nonempty directories that are not an explicitly supported existing project root. Require a verified ownership boundary before whole-root deletion; a marker newly written into an arbitrary directory is not proof of ownership.

**Regression test:** attempt moving into `current/nested`, selecting a source ancestor, and adopting a populated non-project directory. Assert rejection before any marker, staging, settings write, or file deletion.

### H3 — Storage move ownership is not reserved atomically, and is released too early

**Evidence:** `storage/move.go:55-60,74-104,234-269`; `storage/move.go:156-184`.

Two HTTP callers can both observe `moving == nil`, perform settings/task writes, and start independent workers. Both can share the same staging path and delete each others staging; only the last run remains cancellable. `new_only` never reserves a transition at all. The worker also clears `s.moving` before previous/location/journal finalization, permitting a second transition during cutover. `UseDefaultForNew` has no exclusion with a running move.

**Fix:** serialize all root transitions with one operation owner, reserved before I/O and retained through durable finalization. Conditional cleanup must only clear that operation generation, not a newer operation.

**Regression test:** barrier two `Move` calls before task creation; assert exactly one succeeds. Also race move completion with `new_only`/use-default and assert one consistent location, previous record, staging owner, and hold set.

### H4 — Shutdown cancels contexts but does not await workers; moves ignore the server lifetime

**Evidence:** `storage/move.go:100-104,225-240,269`; `cmd/viderse/serve.go:80-88,130-146`; `toolsvc/install.go:127-152`; `cmd/viderse/machine_wiring.go:52-55`.

The move worker is rooted in `context.Background()`, not storage/server context. SIGTERM only cancels machine/storage contexts in deferred functions after HTTP shutdown, and there is no join before `runServe` returns and the process exits. Installer cancellation can require process-group termination grace, so signalling cancellation immediately before exit does not ensure children are gone. Background task finalization also uses unbounded Background contexts while the pool is about to close. Hash/verification and recursive deletion do not observe cancellation either.

**Fix:** give every worker the shared server lifetime and an owned completion handle. Stop admitting work, cancel workers, await copy/installer/process-group cleanup under a bounded shutdown context, then close hub/logs/database. Preserve durable recovery state if shutdown cannot finish.

**Regression test:** cancel the server lifetime during a blocked move and during an installer with a child that ignores SIGTERM; require shutdown to wait for worker exit/escalation, no surviving child, and recoverable filesystem/task state.

### H5 — Transient storage holds overwrite persistent user pause intent

**Evidence:** `storage/service.go:55,268-309`; `storage/recover.go:14-29`; `domain/storage.go:73-90`.

The service writes the same persisted `clone.queuePaused` flag as the user, but remembers ownership only in `wePaused` in RAM. After a crash during a move/offline hold, the DB remains paused while the new service has no ownership flag to release it. Conversely, a user choosing pause while a storage-owned pause is active is undone when the hold releases. The read-then-write sequences also race other hold/user transitions.

**Fix:** keep persisted user pause separate from transient storage holds and derive effective pause from both. If transient ownership must be persisted, reconcile it explicitly and use atomic versioned updates without changing later user intent.

**Regression test:** restart during a hold and verify effective pause clears when healthy; explicitly pause during a hold and verify release does not resume the queue.

### H6 — The Darwin volume cache has unsynchronized map reads/writes

**Evidence:** `storage/volumes_darwin.go:349-391`.

`fresh` releases its channel lock before `List` copies `c.vols` and before `Info` reads `c.byPath`. Another request can reset the map on expiry or write an uncached path under the lock while the first reads it without the lock. That is a real data race and can terminate the process with concurrent map read/write. Cache expiry and concurrent check/browse/storage requests reach it; existing tests use fake volumes rather than this cache.

**Fix:** protect every map/slice read and publication with a mutex, or atomically publish immutable snapshots. Keep expensive collection outside the publication lock with single-flight ownership.

**Regression test:** deterministic concurrent cache-hit, uncached-Info, and TTL-expiry operations against an injected Volumes backend; run that test with the race detector.

### H7 — Pending-media cleanup permits `..` to escape the media root

**Evidence:** `retention/media.go:159-168,224-252,101-128`; automatic invocation at `retention/retention.go:246-247`.

IDs loaded from `editor/media-reclaim-pending.json` are not validated. Unlike `reclaimMediaOf`, `reclaimPending` does not call `isPortableID` before joining paths and recursively deleting. For example `../projects` addresses the editor projects directory. If its age passes the grace check, automatic startup cleanup can delete it without a cleanup preview. Longer traversal can escape the project root entirely. This input is on-disk project content, not a trusted Go value.

**Fix:** validate every decoded ID at the boundary and reject any nonportable/local single component. Perform destructive operations through a contained filesystem root and reject symlink-parent escape as well.

**Regression test:** seed pending JSON with `../projects` and an escape to an old sibling directory, invoke RunAuto, and assert both trees remain intact and the invalid record is reported/rejected.

### H8 — Receipt lookup errors cause deletion of recoverable paid-image scratch

**Evidence:** `retention/candidates.go:85-108`; `persistence/postgres/storage.go:29-47`; `retention/retention.go:242-247,329-357`.

A failed `ScratchDirs` query is ignored and leaves the protection set empty. The startup/periodic sweep then deletes every old matching image-call directory, including one needed to adopt a previously paid interrupted generation. A transient DB/query failure changes “ownership unknown” into “safe to delete.”

**Fix:** fail closed for image scratch when receipt protection cannot be loaded. Propagate/report that group error and retain its directories; do not substitute an empty ownership set.

**Regression test:** use an old image-call directory and a receipt store returning an error; both preview and automatic cleanup must retain it. A later successful lookup must still permit recovery.

### H9 — Media GC treats unreadable references as no references and drops the source ports safety checks

**Evidence:** `retention/media.go:49-70,132-147,189-212,224-252`; `retention/files.go:93-103`.

A project directory read failure is skipped, and JSON file read failures in `tokensIn` are silently ignored. The resulting incomplete held-media set authorizes deletion of still-referenced media. Reference discovery is limited to the current root despite retained roots in the Roots contract. Candidates then capture a bare `RemoveAll` closure: references/claims are not rechecked under a shared import/media lock immediately before deletion. The pinned yt-factory `src/server/editor/media-gc.ts` explicitly propagates non-ENOENT reference-read failures and rechecks references under import/media locks.

**Fix:** make reference discovery fail closed, account for every retained readable project root, and restore a shared claim/deletion critical section that revalidates ownership immediately before removal. Do not use retention-private locks as a substitute for importer/editor coordination.

**Regression test:** unreadable project JSON/directory must preserve its media; a project on an alternate root must preserve a shared ID; a reference/claim added between collection and deletion must win over GC.

### H10 — Cleanup preview does not bind the approved deletion set or root

**Evidence:** `retention/retention.go:176-218`.

Only aggregate preview rows are saved. Run validates the ID, then independently collects and deletes the entire **current** eligible set. If the root changes after preview, a preview of root A authorizes deletion in root B. Newly eligible items are also included although never previewed. The ID check occurs before acquiring `runMu`, allowing two simultaneous run requests to consume one approval.

**Fix:** bind a one-shot preview to root/generation and stable candidate identities. Consume it atomically under the execution lock, revalidate only that approved set, and require a new preview if root/set expansion occurs.

**Regression test:** preview A, switch to populated B, then run the old ID: expect 409 and no deletion. Also add an eligible item after preview and race two run requests; it must not be deleted without new approval.

### H11 — Real new-machine setup never starts from the web screen

**Evidence:** `setup/setup.go:145-164,187-208`; `toolsvc/probe.go:235-258`; `apps/viderse/web/src/screens/setup/SetupScreen.tsx:109-114`.

A clean machine reports missing install steps as `pending` and ChatGPT as `needs_user`. The screen only POSTs start if **every** step is `pending`; that predicate is false for the real six-row response. There is no other start-install CTA on this screen, so the promised automatic five installs do not start. After logging in, the login row becomes `done`, which still makes that predicate false. Current UI tests begin with synthetic already-running/login-complete fixtures rather than this initial response.

**Fix:** start once when there are pending installable steps and no run, independent of the login row; alternatively make the backend own this initial orchestration with an explicit start state. Do not auto-retry failed steps indefinitely.

**Regression test:** feed the real service response for five missing tools plus logged-out ChatGPT into the screen, and assert exactly one POST start and subsequent ordered progress.

### H12 — Live tool overlays mutate the shared cached slice and race HTTP serialization

**Evidence:** `toolsvc/toolsvc.go:170-182,200-216,254-269`.

`snap := *cached` only copies the slice header. `applyLive` writes `snap.Tools[i]`, thereby modifying the cache itself. Once a row becomes installing it stays installing after the run disappears until another probe replaces the cache. More seriously, one handler can be encoding the returned rows while another handler changes the same backing array under the service mutex; HTTP serialization does not hold that mutex.

**Fix:** publish immutable snapshots and copy the small row slice before live overlays, including independently mutable nested state where needed. Never return storage that a later call will mutate.

**Regression test:** retain a returned snapshot, start/cancel an install, call Snapshot again, and assert the first value and cached baseline do not change. Concurrent HTTP encoding/snapshot calls must be race-free.

### H13 — Install-run ownership and Ensure joining are not atomic

**Evidence:** `toolsvc/install.go:37-55,61-79,127-152`.

Two StartInstall calls can both pass the running check, create/reset the same activity object, publish different runs, and launch two pumps. A worker unconditionally deletes the map entry even if it now belongs to another run. Ensure also rereads `s.running[id]` after StartInstall returns; a fast finishing worker can already have deleted it, causing a spurious “mất lượt cài” despite successful installation. It can instead pick a later run. Lower-level installer single-flight does not fix task/pump/cancel ownership.

**Fix:** reserve one run atomically per tool and return that exact run handle to internal waiters. Retain it through task finalization; only its owner may remove it. Join/stop the pump before publishing terminal state.

**Regression test:** barrier concurrent starts, plus an immediately completing installer and a second start during finalization; assert one pump, one task transition, correct cancellation owner, and Ensure returning the exact run result.

### H14 — A second serve attempt mutates live tasks before discovering the port is occupied

**Evidence:** `cmd/viderse/serve.go:82-111`; `cmd/viderse/machine_wiring.go:43-49`; `persistence/postgres/queries/activity.sql:56-66`.

Storage/retention workers start and `activities.Reconcile` marks **all** active task rows failed before `net.Listen`. Starting a duplicate server on the same address therefore corrupts the live servers running task states even though the duplicate subsequently exits with address-in-use. Reconcile has no owner/lease predicate. The same ordering can reconcile newly created startup-cleanup work from this process.

**Fix:** establish singleton/listener ownership before side effects, reconcile only proven-dead owners, and complete reconciliation before starting new workers.

**Regression test:** keep a live running task and an occupied listener, invoke startup, and assert no task/settings mutation or workers start. Seed dead-owner and live-owner tasks and reconcile only the former.

### H15 — Failed or reconciled storage tasks permanently lock the storage UI in moving mode

**Evidence:** `storage/move.go:288-298`; `activity/activity.go:180-190`; `apps/viderse/web/src/screens/settings/storage/StorageSection.tsx:446-447,488-500,205-233`.

A failed move intentionally remains in background_tasks with state failed and action retry. The client finds any `storage_move` task without filtering state, treats it as an active move, hides Change folder, and renders only Cancel. Cancel then returns no_move because no worker exists. A crash-reconciled task has the same effect. The retry action produced by the backend is never consumed here.

**Fix:** distinguish active and failed tasks; render a working retry/dismiss/recovery action for failed storage moves and keep folder selection available when no worker owns a move.

**Regression test:** return a failed/reconciled storage task with no running move; assert the UI is not locked into progress/cancel and Retry starts the correct target operation.

### H16 — Cookie redaction leaves later credential values in logs and exports

**Evidence:** `logs/logs.go:61-72,105-115,140-142`; `logs/bundle.go:73-82`; `logs/logs_test.go:33-38`.

The generic cookie regex only consumes a value until whitespace. The existing fixture `Cookie: SID=abc123; HSID=zzz` masks SID but leaves `HSID=zzz`; the test only asserts that abc123 disappeared. A Bilibili cookie header similarly leaves later `bili_jct`/session fields. Neither a full Cookie header nor named SESSDATA/bili_jct values are comprehensively removed. Reapplying the same patterns during bundle export does not remove the remainder.

**Fix:** redact complete Cookie/Set-Cookie headers and structured cookie fields, including multiline/escaped forms and platform credential names, before persistence and again at export boundaries.

**Regression test:** assert **every** credential fragment from multi-cookie headers, Bilibili cookies, and JSON-escaped log messages is absent both from disk and from every exported zip member. Keep useful nonsecret diagnostics.

## Medium priority — informational but required for the stated acceptance thresholds

### M1 — GET requests still spawn processes and make network calls

**Evidence:** `storage/snapshot.go:212-247`; `storage/volumes_darwin.go:349-385,53-61`; `logs/version.go:85-108`; `web/logs.go:112-113`; `web/tools.go:24-25`; `web/setup.go:25-26`; `toolsvc/toolsvc.go:174-186`.

Volume cache misses/30-second expiry synchronously call inner.List/Info, which spawn diskutil/plutil. Version GET synchronously calls GitHub on a cold/expired cache. GET fresh=1 explicitly invokes binary probes, and an empty tool cache joins the cold Warm probe rather than serving an immediately available cached/initial view. Thus the no-binary/no-network-per-GET requirement is not implemented; warm fake-backend Snapshot benchmarks do not cover it.

**Fix:** make GET read-only snapshots. Refresh on explicit POST and owned bounded background work; remove GET fresh mutations. Define an honest initial/checking state instead of awaiting external probes.

**Regression test:** instrument process/network seams, advance TTL and read cold caches, then assert zero external invocations for GET; POST check must be the operation that refreshes.

### M2 — Idle storage repeatedly walks the project and config trees

**Evidence:** `storage/snapshot.go:16-19,58-68,267-283`; `retention/retention.go:268-286`; plan idle rule `plan.md:177`.

The 15-second ticker performs five-minute full usage rescans even without any writes, and retention runs another six-hour sweep. This exceeds the explicit idle budget of SSE ping plus drive-watch stat only. Up to 500,000 entries per scan on a large/external disk is materially different from an idle stat. The Storage report describes these timers as accepted behavior, but they conflict with the owners stated threshold.

**Fix:** refresh usage from existing mutation/dirty signals and explicit user checks, not unconditional tree-scan timers. Reconcile any required periodic retention scheduling explicitly with the owner rather than claiming the idle gate is met.

**Regression test:** advance an injected clock through idle refresh/sweep intervals with no dirty event; assert no tree walk/DB scan/process enumeration beyond the allowed drive-watch checks.

### M3 — Forced move updates bypass the hard four-writes-per-second limit

**Evidence:** `storage/move.go:369-391,641-679`; `storage/perf_test.go:123-130`; `activity/activity.go:145-153`.

Initial progress plus start/end for each of four groups and the other group bypass throttling. Tiny files can produce well over four activity writes/events in one second. The test explicitly adds `2*len(Groups)+4` slack, so it verifies a weaker rule. Each progress update also emits storage in addition to activity, driving extra invalidations.

**Fix:** coalesce phase/group changes through the same per-task 250 ms writer; permit a single terminal flush only if the acceptance rule explicitly allows it. Preserve the latest group state, not every intermediate event.

**Regression test:** move all groups with tiny files under a fake clock, count actual store writes/notifies in rolling one-second windows, and enforce the real limit rather than elapsed-time-plus-group slack.

### M4 — Setup readiness is stale after login and can call an active install ready

**Evidence:** `toolsvc/toolsvc.go:96-98,170-182`; `toolsvc/report.go:25-35`; `cmd/viderse/machine_wiring.go:63-80`; `apps/viderse/web/src/data/eventKeys.ts:18`; `apps/viderse/web/src/screens/setup/SetupScreen.tsx:164-165`.

ChatGPT login status is cached with all binary probes for five minutes; AI events do not invalidate the backend tool cache. The screen invalidates its setup request after login but gets the same old needs_user result. Also rowCheck maps installing to ready, so setup queried during a tools-page install can claim a missing tool is already done/complete.

**Fix:** overlay live session readiness independently of expensive binary probes and connect login completion to the owning cache. Preserve an in-progress state in StepChecks/Status rather than mapping it to ready.

**Regression test:** warm logged-out state, log in, refetch without advancing TTL and expect done; query setup during an external missing-tool install and expect running/not complete until installation actually succeeds.

### M5 — Cleanup removal failures are converted into a successful HTTP result

**Evidence:** `retention/retention.go:295-320,214-222`; `web/storage.go:170-174,382-398`.

execute collects per-item/reaper failures in RunResult.Errors, but the handler discards them and always returns a success-shaped result. The service consumes the preview and marks the last-run timestamp even for failed removals. The user cannot distinguish a partial/failed cleanup from a successful one or identify what remains.

**Fix:** preserve partial-result errors in the service/HTTP contract with a stable nonsecret error summary, and let the client show partial failure/retry. Record last-run semantics explicitly rather than imply all items were removed.

**Regression test:** inject one removal failure among successes; assert accurate counts plus a visible partial-failure result and no false all-clean success message.

### M6 — Recent errors is not a seven-day history and count7d is capped at twenty

**Evidence:** `activity/errors.go:36-45,50-59,87-101`; `web/activity.go:179-181`; `persistence/postgres/queries/activity.sql:96-101`.

Errors is built from unfinished failed tasks and unread notifications, then truncated to 20. The HTTP count7d is just len of this truncated array. Reading a notification or successfully retrying a task removes its historical error, and 21+ recent errors still report twenty. Passing limit=200 does not change the Postgres query hardcoded to LIMIT 50.

**Fix:** query/count the durable seven-day error history separately from the visible page; do not use unread state or unfinished-task presence as the history definition. Either honor the store limit or remove the misleading argument.

**Regression test:** create 25 errors, read notifications and complete retries; expect count7d=25 and a limited, correctly ordered page that still includes historical events.

### M7 — The new log store is not connected to child-process output

**Evidence:** `cmd/viderse/serve.go:94-104`; `cmd/viderse/machine_wiring.go:52-73`; `toolsvc/install.go:139-148`; `logs/logs.go:159-162`.

The store is injected only as a logger for final install success/failure messages. Its Writer has no production caller in the inspected app, and the shared ExecRunner is not adapted to attach per-tool Spec.Log. The settings logs/export therefore omit the actual installer/process diagnostics that phase-02 requires; implementing an unused writer does not complete that path.

**Fix:** attach bounded, redacted per-tool child-output writers at the tools/runner composition seam and flush partial lines at completion without double logging or exposing secrets.

**Regression test:** run a fake child emitting stdout, stderr and a final unterminated line; assert all appear once in GET logs/export with secrets removed, including on cancellation/failure.

## Low priority — architecture and dead weight

### L1 — Remove parallel progress conversion and unused state; shrink concrete dependency seams

**Evidence:** identical progress wire conversion at `web/setup.go:74-112` and `web/tools.go:100-145`; duplicated recursive byte accounting at `storage/paths.go:141-163` and `retention/files.go:31-49`; `logs/logs.go:80,90-100`; `storage/service.go:59,125`; `toolsvc/toolsvc.go:104-109`; `toolsvc/report.go:60-87`.

Setup/tools each own the same progress DTO and conversion. Filesystem traversal is duplicated with different cancellation/bound rules. logs.files is never populated, so Close and its dedicated shutdown goroutine close nothing; storage.loaded is assigned but never read. Toolsvc takes the entire concrete Tools/Managed/Transcription graph rather than the small probe/install contracts it consumes. BaseDir/hand-written parentDir add an unused path API even though cmd already supplies baseDir to setup.

**Fix:** retain one progress converter in the HTTP adapter; consolidate only the actual shared filesystem primitive while preserving domain ownership; delete unused state/lifecycle/path APIs. Introduce consumer-owned narrow probe/install seams rather than another general manager. Do not move wire DTOs into domain to deduplicate them.

**Regression test:** shared progress mapping table tests must prove both endpoint shapes stay identical; fake narrow tool dependencies should cover orchestration without constructing the full tool graph. Existing service/wire tests must remain unchanged in behavior.

## Areas that look correct within the inspected limits

- **Single activity service:** `serve.go:72` constructs one instance and passes it to storage and machine; no second activity service was found in this composition.
- **Layer direction/domain:** the scoped business packages do not import web/Postgres/cmd; domain contains values, not I/O. HTTP generally has explicit wire DTOs rather than serializing domain tasks/providers directly. The concrete tool dependency noted in L1 is a maintainability concern, not a forbidden web/persistence import.
- **Authentication:** requested routes use Gateway.Handle and therefore pass session authentication in `platform/webapp/server.go:140-166`. Machine GET setup uses `/api/setup/state`, correctly avoiding the public account-setup route. This is a shared local-machine app; no tenant/project isolation requirement was invented.
- **Browse/open boundary:** browse intentionally accepts server-side absolute directory selection and returns directory names only; hidden entries are filtered. Open accepts no arbitrary request path and opens the current root via argv, not a shell (`storage/browse.go:91-101`, `cmd/viderse/storage_wiring.go:110-123`). The destructive cleanup input in H7 is a different boundary.
- **SQL:** inspected activity/storage SQL is parameterized. Snapshot uses a constant number of store calls rather than a DB call per task; notifications use a transaction/advisory lock for same-kind merge (`postgres/activity.go:239-276`). No N+1 query was found on these currently wired paths. There is still no database uniqueness guarantee for object tasks across processes; H14 addresses the directly reachable startup consequence.
- **Retention exclusions:** existing media eligibility retains overlay/recently claimed media; source/mp4 reclaim checks portable video IDs and keeps nonmatching evidence. The no-video adapter currently makes reclaim cards empty rather than deleting jobs. These correct predicates do not offset incomplete reference discovery.
- **Basic cancellation/process plumbing:** the current yt-dlp Install uses a cancellable flight (`tools/ytdlp.go:441-477`), so the MachineSetup report claim that it ignores context is stale. Process-group tracking/reaping and termination identity checks exist in tools. The composition still does not wait for them on graceful shutdown (H4).
- **Ticker cleanup:** the drive watch, usage loop, retention loop, and progress pump each defer ticker Stop on normal context exit. The remaining issues are extra idle work, worker joining, and detached move ownership, not a missing Stop on these ordinary paths.
- **Wire/client basics:** setup/state path, log cursor encoding, tools rows, and storage move `{taskId,done}` agree with the inspected typed clients. Named SSE key invalidation exists rather than invalidating the whole cache. Failed storage task state and setup readiness are the material mismatches reported above.

## Verification actually exercised

One authorized focused run, exactly under the supplied heavy lock/toolchain/CPU settings:

```text
lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -p 2 ./apps/viderse/internal/storage ./apps/viderse/internal/retention ./apps/viderse/internal/activity ./apps/viderse/internal/toolsvc ./apps/viderse/internal/setup ./apps/viderse/internal/logs

storage    ok 3.773s
retention  ok 0.337s
activity   ok (cached)
toolsvc    ok (cached)
setup      ok 0.288s
logs       ok (cached)
Wall time: 5.72s
```

No second test run, race run, build, linter, formatter, browser smoke, actual move/delete reproduction, DB integration run, or network/version check was performed. Type coverage, test coverage percentages, current lint issue count, production p95, SSE rate, RSS and bundle size were **not measured in this review**. Agent-reported measurements are not substituted for reviewer evidence.

## Plan follow-ups and recommended order

1. Resolve C1 and H1–H10 before permitting real moves/cleanup; add failure-injection/containment tests first. Validate retained-root ownership and rollback rather than suppressing errors.
2. Resolve H11–H16 and the setup/session state contract; then run the real clean-machine and failed-move flows, not only ready-state fixtures.
3. Main/integrator should run focused new regressions, relevant Postgres and HTTP tests, `TestLayeredAppImports`, then a single scoped race pass after all concurrent edits land. In particular, exercise real volumeCache rather than fake Volumes and compare real backend setup states to web tests.
4. Measure cold/expired-cache GETs and idle activity; existing warm Snapshot timings do not prove the performance contract. Count actual progress DB writes/SSE per rolling second.
5. Phase 02 remains **in progress**, consistent with its plan file. Wiring and HTTP tests have advanced beyond the MachineSetup report, but crash-safe movement, automatic clean-machine setup, no-secret logs, shutdown/orphan guarantees and the stated performance thresholds are not met. No plan state was changed. Browser smoke/gap-closure/commit/push/CI-zero requirements were not verified or performed in this read-only assignment.

## Unresolved questions / limitations

- MachineSetup may continue modifying the inspected files; the lead must reconcile findings with the final source. This report does not assume those later edits fix anything.
- No missing prerequisite blocked this review. Runtime failure windows remain to be reproduced by the lead in isolated test roots; they were not exercised against user data.

Status: DONE_WITH_CONCERNS
Summary: Reviewed the assigned machine/storage scope and wrote evidence-backed blockers and concrete regression ideas. The six focused service suites passed, but storage data safety, cleanup containment, lifecycle ownership and real setup behavior prevent a production-readiness approval.
