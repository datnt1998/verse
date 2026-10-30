# Phase 03 scan backend — independent production-readiness review

**Decision: BLOCK COMMIT.** Existing happy-path tests pass, but independent boundary probes exposed broken global lease ownership, a reproducible stale-token write, a second-page discovery failure, and an idle busy loop. No product source or plan state was changed during review.

## Scope and method

- Reviewed `apps/viderse/internal/scan/*.go`, `internal/persistence/postgres/{scan.go,scan_pg_test.go,queries/scan.sql}`, `internal/youtube/{ref.go,uploadtime.go,uploadtime_test.go}`, `cmd/viderse/{scan_wiring.go,serve.go}`, the `internal/activity/activity.go` addition, and migration `00006_rivals.sql`. Also traced activity storage, platform pause/pacing, CLI construction, resolver HTTP boundaries, and web task-action consumers.
- Size: **5,773 lines** across the scoped source/test files, including existing context; this is not an added-LOC count. Generated scan SQL was inspected where needed to verify the executed query contract.
- Rulebook: root `AGENTS.md`, phase-03 plan, binding `local://p03-contract.md`, `local://p03-ScoutYtfScan.md`, implementation reports, base/API review checklists. Relevant source functions/tests in yt-factory were checked directly.
- Scout-first edge map: worker token crosses SQL → Go → heartbeat; channel state and job state have different locking; deleting a parent competes with child-first page writes; cursor/remainder crosses import → history; discovery ordering must remain stable across pages; task IDs survive cancellation/restart; exact-time sampling must select the newest 30 **before** filtering unchecked rows; scheduling must exclude work already parked in a job.
- No YouTube network calls. All added probes ran through temporary Go overlays under `/tmp/p3-review-scan`, without modifying repository test files. Temporary probes were removed after recording their results.

Paths in findings are relative to `apps/viderse/`, unless otherwise stated. Severity is critical / major / minor. Findings below include observed evidence or an explicit concrete interleaving, not hypothetical style concerns.

## Critical findings

### C1. Global worker lease is returned with the platform name instead of its owner, and losing it does not stop the worker

**Locations:** `internal/persistence/postgres/scan.go:1515-1531`; `queries/scan.sql:482-496`; `internal/scan/worker.go:250-262,315-377`.

`ScanAcquireWorkerLease` stores the random owner but returns `platform`; `AcquireWorkerLease` consequently returns `WorkerLease{Owner: "youtube"}`. Renewal/release compare this with `scanner_owner`, which actually contains `scan-<random>`. The first heartbeat therefore returns nil. The heartbeat simply exits: it neither cancels the current fetch nor cancels the main loop. The main loop never checks/reacquires its global lease; the passed lease is unused there.

**Observed:** `TestReviewWorkerLeaseIdentity` acquired owner `review-owner`, got `Owner="youtube"`, and immediately failed to renew (`renewed=<nil>, err=<nil>`). `TestReviewHeartbeatLossAbortsActiveFetch` waited for the real 15-second heartbeat; it terminated after renewal returned nil while the active fetch context remained live.

**Production interleaving:** A starts at t=0; its heartbeat stops at t=15; its database lease expires at t=60 although A continues scheduling/fetching. B claims the expired global lease and also runs. Channel fences may serialize individual channels but do not prevent concurrent scans on different channels, concurrent upload probes, or independent pacers. Even without B, channel heartbeats stop after the first failed global renewal.

**Fix:** Return/preserve the actual owner, not the platform. Use a worker-scoped cancellation cause on renewal loss, cancel both listing and upload fetches, stop the loop, and join the heartbeat before releasing ownership. Renewal failure past the known expiry must also stop work. Add a real store-backed worker acquire/renew/release test plus a two-process takeover/in-flight cancellation test.

### C2. Claim and transition fences have a TOCTOU window inside their transactions

**Locations:** `internal/persistence/postgres/scan.go:299-310,1232-1280`; `queries/scan.sql:43-44,287-310`.

`assertLeaseTx` performs an unlocked SELECT. `ClaimJob` and `fencedJob` then update a different row (`scan_jobs`). Being in the same READ COMMITTED transaction does **not** prevent another transaction from replacing the channel lease between that SELECT and the job update. Unlike `CommitPage`/`CompleteHeadCheck`, these paths do not first lock `scan_state` through its upsert.

**Observed deterministic interleaving:** `TestReviewFenceTakeoverDuringTransition` suspended A immediately after reading its valid state token; expired A in the database; B acquired a new token and claimed the running job; then A resumed `SetJobBackoff`. A succeeded and changed B’s job to `backoff`. This is an actual stale-owner write after takeover.

**Fix:** Serialize channel-fenced operations on a locked state row before checking the current token/expiry, using a consistent parent/state/job lock order. Alternatively use a correctly locked conditional write that enforces both the token and job state atomically. Preserve underlying database errors instead of converting every state-read error to “channel missing”. Test takeover between the fence read and every transition, not just an obviously wrong token at entry.

## Major findings

### M1. A second discovery page incorrectly rebases the run and can fail with a unique-position violation

**Locations:** `internal/persistence/postgres/scan.go:510-535`; `queries/scan.sql:158-167`; migration `00006_rivals.sql:61`; original yt-factory `scan-repository.ts:504-539`.

After page one inserts the newest video at the run base, that base becomes the channel minimum. On page two, `endKey >= minPos` is necessarily true for ordinary progress inside that same reserved block. The implementation rebases all existing positions. The immediate unique `(channel_id, position)` constraint can reject the in-place renumbering. Even when renumbering happens to succeed, it moves page-one rows to nonnegative positions and puts page-two rows back before them, reversing page order. The recalculated base is not persisted to the job either.

**Observed:** `TestReviewDiscoveryPreservesPageOrder` committed one non-overlapping discovery video with a continuation, then a second page. Page two failed with SQLSTATE **23505**, constraint `rival_videos_channel_id_position_key`.

**Fix:** Reserve a stable block once in `StartDiscovery`; only rebase before creating a run when approaching the safe key bound, as the source does. Make any required renumbering safe under the unique constraint and persist the run base. Enforce the 10,000-ordinal bound explicitly. Add multi-page/restart order assertions and check positions, not only the overlap boolean.

### M2. Idle scheduling spins on past deadlines and still polls when completely idle

**Locations:** `queries/scan.sql:428-432`; `internal/scan/worker.go:100-102,141-142,404-420`; `internal/persistence/postgres/scan.go:1506-1511`.

`ScanNextRescanAt` considers every state, including a channel whose discovery job is in backoff/error/cancelled. Its old rescan time wins over the future job backoff. `idle` sees a nonpositive duration and returns immediately, repeating settings/due/runnable/upload/timer queries as fast as the database permits. Separately, `UploadsPausedUntil` returns expired schedule timestamps, which produce the same busy loop after an upload pause expires. With no deadlines at all, the default `PollIdle=60s` still performs periodic database work, contrary to the no-polling contract.

**Observed:** `TestReviewDueTimerExcludesBackoff` put a due channel’s discovery job into a future one-hour backoff. `NextRescanAt` still returned a timestamp one hour in the past. The loop has no sleep on this path. The repository idle test overrides `PollIdle` to one hour and observes only 650ms, so it does not test production defaults or these past-deadline cases.

**Fix:** Compute the next actionable deadline from eligible channels/jobs; exclude channels already represented by a discovery run. Ignore expired upload-pause timestamps. Sleep on a wake signal indefinitely when nothing is scheduled, plus the separate mandatory lease heartbeat. Mutations/settings/platform-pause changes must invalidate/recompute the timer; cross-process mutations need a real wake mechanism, not a minute poll.

### M3. Initial baseline discards the continuation needed by history

**Location:** `internal/persistence/postgres/scan.go:552-570`.

`saveBookmark := in.JobID == "" && !baseline` is the reverse of the required import handoff. A successful Add stores videos but discards its next cursor/remainder; `StartBackfill` starts again from the head. This contradicts both the contract and the implementation report claiming the baseline continuation is saved.

**Observed:** `TestReviewBaselineSavesCursor` committed a baseline with a valid non-null continuation; `scan_state.history_cursor` was empty afterward.

**Fix:** Save the baseline cursor/remainder atomically with its initial page and transfer them into the history job on start. Do not overwrite the history bookmark from unrelated job-less discovery. Add the source import → history handoff test with an actual cursor/remainder and provider-call assertions.

### M4. Upload probes drain the entire channel instead of only its newest 30 rows

**Location:** `queries/scan.sql:443-454`.

The lateral query filters `upload_checked_at IS NULL` **before** `ORDER BY ... LIMIT 30`. Once the newest 30 have been checked, older unchecked rows enter the window. A channel with 10,000 indexed rows can therefore cause 10,000 `/player` probes instead of 30, monopolizing the worker and increasing blocking/rate-limit exposure.

**Observed:** `TestReviewUploadSampleDoesNotDrainHistory` seeded 38 videos and marked positions 0–29 checked. `NextUploadCandidate` returned the video at position 30 rather than nil.

**Fix:** Select the newest 30 per channel first, then filter that bounded sample for unchecked rows. Add a test with more than 30 videos and a fully checked head.

### M5. Upload-time writes have no channel fence at all

**Locations:** `internal/scan/worker.go:773-810`; `internal/scan/store.go:260`; `internal/persistence/postgres/scan.go:1482-1490`; `queries/scan.sql:457-462`.

The probe never acquires a channel lease; `RecordUploadTime` takes only video ID and unconditionally updates it. This cannot satisfy “every channel write checks its lease token in the same transaction”. Losing the worker lease also cannot cancel this fetch (it uses the outer context, not the registered active-fetch cancellation).

**Concrete interleaving:** A fetches upload data for V; the rival is removed and later re-added, or a new worker takes over; stale A returns and updates the replacement V row by the same video ID. There is no token/generation predicate capable of rejecting this stale result.

**Fix:** Acquire/register a channel fence for a probe, bind the fetch to worker/channel loss, and commit the result under the current token and channel ID in one transaction. Do not rely on the mere existence of the video or global lease.

### M6. Platform pause is displayed but not enforced by the scanner

**Locations:** `internal/scan/worker.go:315-377,684-688`; `cmd/viderse/scan_wiring.go:25-31`; `cmd/viderse/rivals_wiring.go:80-85`; `internal/youtube/pacer.go:11-15`.

A blocked job calls `RecordBlocked`, but neither the loop nor the YouTube client consults the durable platform pause before the next job/probe. The next channel can immediately issue requests while YouTube is supposedly paused. The idle timer does not include platform pause end. Further, each CLI constructs a separate in-memory YouTube pacer; Add does not hold the worker lease, so server/CLI Adds can bypass one another’s 429 penalty and 1–2s request spacing.

**Concrete evidence/interleaving:** `recordFailure(blocked)` writes the platform pause, returns after parking only that job; the next loop calls `NextRunnableJob` and `FirstChannelPage` without a pause check. `scanDeps` and `newCLIScanService` each call `youtube.NewClient` with a fresh default pacer. There is no persisted shared request schedule in this scan integration.

**Fix:** Reuse/enforce the platform pause boundary before all scanning/Add/probe requests and schedule wake at its end. If “everyone” includes concurrent CLI/server processes, persist shared request reservation/429 penalties rather than relying on one client instance. Keep requests single-attempt; do not fix this with hidden retries.

### M7. Task actions are not wired, so recovery controls are unreachable

**Locations:** `internal/scan/service.go:407-438`; `internal/scan/store.go:238`; `internal/web/activity.go:21-76`; `cmd/viderse/scan_wiring.go:48-65`.

The emitted tasks advertise cancel/resume/retry, but `Service.TaskOp` has no caller and `JobByTask` has no integration caller. No mounted task-op route bridges task ID → job ID → scan control. The web activity dispatch has no rival action implementation either. This is an explicit contract requirement, not an optional endpoint question.

**Impact:** Discovery failures and cursor-restart pauses show “Thử lại”/“Tiếp tục” but cannot actually be recovered through their task action. History controls elsewhere do not fix discovery recovery.

**Fix:** Wire the authenticated task operation through task ID lookup and the correct job kind, and connect the UI caller. Exercise HTTP → task lookup → service → durable transition in integration tests. Do not delete `TaskOp` as dead code until the required feature is wired.

### M8. Cancelling a discovery run permanently blocks subsequent checks

**Locations:** `queries/scan.sql:215-219,415-425`; `internal/persistence/postgres/scan.go:1154-1176,1374-1405`.

`ScanLiveJob` and due-channel exclusion treat cancelled discovery as live (`state <> complete`), while the unique-live index does not. `StartDiscovery` simply returns false for that cancelled row. No future scheduled discovery or user `check` can start; only the currently unreachable discovery resume seam can change it.

**Observed:** `TestReviewCancelledDiscoveryCanCheckAgain` started and cancelled discovery, then called `StartDiscovery`; it returned false and left no runnable work.

**Fix:** Define terminal/live discovery semantics consistently across selectors/indexes/control. A new check must requeue/reset an eligible stopped run or create a new run, and cancellation must not disable all future scheduled checks. Retain explicit user intervention for errors/pauses as required.

### M9. Inline worker exits when another channel happens to sort first

**Location:** `internal/scan/worker.go:328-335`.

When a non-target job is selected, inline mode adds it to `busy`, sets `job=nil`, then immediately returns because `busy` is nonempty. It never asks again for the target’s job. For example, B discovery sorts ahead of requested A history: `rival scan A --history` claims the global lease, reports success, and performs no A work even though A is runnable.

**Observed:** `TestReviewInlineSkipsOtherChannelBeforeStopping` returned after exactly one selection of an unrelated discovery job.

**Fix:** Filter runnable selection by target channel in SQL/the store, or continue selecting after excluding unrelated channels. Return only after proving there is no runnable job for the target, not after encountering an unrelated row.

### M10. Durable task IDs outlive deleted task rows; backoff and recovered ownership are not synchronized

**Locations:** `internal/scan/service.go:505-567`; `internal/scan/worker.go:676-682,726-734`; `internal/persistence/postgres/queries/activity.sql:18-28,38-49`; `cmd/viderse/serve.go:136-146`.

**Concrete interleaving:** cancel history → `syncTask` calls `Finish` but leaves `scan_jobs.task_id`; resume/start the same cancelled job → `syncTask` sees a nonempty task ID, calls `Update` on the deleted task, ignores the not-found error, and never recreates a task. The scan then runs invisibly.

After crash recovery, existing task IDs use `Update`, not `Start`/`ResetTask`, so `owner_pid` remains the dead PID. Startup starts the scan goroutine **before** `mountMachine` calls `Reconcile`; reconciliation can mark an already resumed task failed because its owner was never adopted. Backoff transitions also omit `refreshJobTask`, leaving the task visually running rather than waiting.

**Fix:** Atomically clear/relink task IDs when tasks finish; recreate missing rows; adopt the current owner on recovery and reconcile before launching the worker. Synchronize every durable state transition, including deferred/transient backoff. Test cancel → resume, crash → restart, and remove while task creation is in flight; do not swallow synchronization failures silently.

### M11. Scanner errors are discarded and shutdown does not join the worker/heartbeat

**Locations:** `cmd/viderse/scan_wiring.go:69-73`; `cmd/viderse/serve.go:128-138,177-179`; `internal/scan/worker.go:211-235`.

`runScanWorker` launches a goroutine with `_ = worker.Run(ctx)` and returns no completion signal. A single propagated DB error terminates scanning for the lifetime of an otherwise healthy server with no error log or supervisor. On shutdown `scanStop()` only requests cancellation; `scanSvc.Stop()` and eventual `pool.Close()` run without waiting for the worker or heartbeat. Startup error returns also skip `scanSvc.Stop()`.

**Concrete ordering:** a page fetch observes cancellation slowly; serve cancels, stops the service timer, finishes the other shutdown work and closes the pool; the scan goroutine later attempts its final state/task writes and deferred lease release. The API requests and timer callback can also race `Stop` because there is no closed state/join barrier.

**Fix:** Return a worker completion/error channel or runtime handle, surface unexpected termination, cancel and join worker + heartbeat before stopping the scan service and closing the pool. Apply the same teardown on every bootstrap failure. Bound cleanup contexts rather than using unbounded background DB operations.

### M12. Head checks rewrite all 10,000 indexed rows even when the median is unchanged

**Locations:** `internal/persistence/postgres/scan.go:828-847`; `queries/scan.sql:205-212`.

The whole-channel score update is unconditional. The contract requires whole-channel recomputation **when the median changes**; unchanged medians need no rewrite because page upserts already refresh touched rows using the current median. The current path repeatedly churns every score index and holds channel-state/video locks across the update.

**Observed local cost:** `TestReviewHeadCheckCost10000` measured three complete head-check transactions on one 10,000-row channel: **3.540s, 3.785s, 2.926s**. The latter two had unchanged medians. `EXPLAIN (ANALYZE, BUFFERS)` for the same whole-channel score-update shape reported **2,329.502ms**, 10,000 input rows, **251,280 shared buffer hits**, 421 dirtied/written buffers. These are local test measurements, not a production p95 claim. The head transaction holds the state-row lock for almost its whole duration, delaying controls, fencing and removal.

**Fix:** Compare old/new medians with null-safe equality under the fence; execute the whole-channel update only when it actually changes. Optionally avoid identical score writes too. Retain the current “select newest 150, then discard unknown views” median semantics. Add unchanged-median write-count and 10k-row transaction-cost tests.

### M13. Remove and page commit acquire parent/child locks in opposite order

**Locations:** `internal/persistence/postgres/scan.go:193-207,343-365,545-548`; migration `00006_rivals.sql:36,79,107`.

Page commit locks `scan_state` first, then updates/inserts videos and takes the parent FK lock. Deleting the channel locks the parent before cascading to those same children. This forms a parent ↔ state/video deadlock.

**Observed deterministic interleaving:** `TestReviewRemoveDuringPageCommit` suspended the page after its state upsert held the state lock, acquired the parent lock for deletion, then allowed both to continue. Postgres returned **40P01 deadlock detected** to the page while removal succeeded. PostgreSQL may instead choose the removing transaction as victim, returning a server error for Remove. This is not the intended clean “removed/stopped” path.

**Fix:** Lock the channel parent before state/job/video rows consistently in mutation and deletion transactions; use the same order for all fenced paths. Map a genuinely removed row to stopped/not-found. Add a synchronized parent-delete/page-commit regression rather than only testing deletion after work ends.

### M14. SSE throttle can emit two channel events only 10ms apart

**Location:** `internal/scan/service.go:113-140`.

An immediate emission after the 250ms interval does not remove its existing pending entry; `flushNotify` unconditionally emits every pending channel without checking its latest send time. A prior delayed event can therefore follow a fresh immediate event almost immediately. Different channel deadlines also share one timer without per-channel eligibility checks.

**Observed:** `TestReviewSSEFlushRespectsChannelInterval` emitted at t=0, queued at t=10ms, emitted immediately at t=250ms, and flushed at t=260ms. Recorded sends were **0ms, 250ms, 260ms**. Repeating this pattern can exceed 4/s.

**Fix:** Remove stale pending entries on immediate sends, recheck each channel’s earliest legal emission at flush, and schedule the next timer from remaining deadlines. Test multiple channels and immediate-send/trailing-flush overlap with a controllable clock.

## Minor findings / nonblocking after the above

### m1. Finished history starts a redundant job; concurrent duplicate Add returns the wrong error

**Locations:** `internal/persistence/postgres/scan.go:968-1024`; `internal/scan/service.go:198-239,645-658`.

`StartBackfill` never checks `state.HistoryEnd`. **Observed:** `TestReviewFinishedHistoryIsNoop` created a queued history job for an already-ended baseline. The source explicitly returns without a job in this case. Also, two Adds can both observe “channel missing”; the loser’s baseline transaction rolls back on the first Add’s lease token, but maps the result to `lease_busy` rather than `exists` with the existing rival. An Add head-check failure also bypasses the only lease release at line 239.

**Fix:** Honor the ended-tab no-op; handle an atomic duplicate-baseline winner as `exists`; release the Add lease with bounded cleanup on every post-baseline exit. Add true concurrent Add and import-ended tests.

### m2. Progress throttling and default backoff jitter are not actually centralized/enforced

**Locations:** `internal/scan/worker.go:138-139,447-448,643-655`; `internal/scan/service.go:505-567`.

The 250ms guard only protects `syncProgress`; every page also calls unthrottled `syncTask(..., true)`, which writes progress again, and `force` is never read. Cached remainder/control-driven pages bypass network pacing, so the pacer is not a progress-write limiter. `NewWorker` also defaults `Random` to a constant 0.5, making production backoff deterministic despite the stated ±20% jitter policy.

**Fix:** Enforce progress-write frequency at the actual writer, separately from state transitions; remove the unused `force` parameter or give it precise semantics that do not bypass progress limits. Default to a real random source and inject deterministic values only in tests.

### m3. Remove unused alternate execution paths instead of claiming untested parity

**Locations:** `internal/scan/store.go:121-150`; `internal/persistence/postgres/scan.go:390-401,658-709`; `cmd/viderse/scan_wiring.go:35-42`; `internal/scan/worker.go:114`; `internal/scan/errors.go:80`.

Repository-wide caller search found no product/test construction of `Checkpoint`, no caller of `newScanService` (CLI uses `newCLIScanService`), no use of `Worker.once`, and no use of `storeErr`. The old `youtube.Client.UploadTime` wrapper has no callers either. `JobByTask` is unused only because the required task route is missing; wire that seam rather than deleting the feature. The report claims checkpoint tests, but none exist in `scan_pg_test.go`.

**Fix:** Delete the unused checkpoint branch/types/SQL if the worker’s precreated-job design is the chosen cutover, together with truly unused wrappers/fields/helpers. Keep only one CLI constructor. Do not retain a parallel unexercised implementation solely for superficial source parity.

## Test evidence and limitations

All heavy commands used `lockf -k /tmp/verse-heavy.lock nice -n 10`, `GOTOOLCHAIN=go1.26.0`, and `GOMAXPROCS=4`; Postgres used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`.

| Exercised check | Result |
|---|---|
| `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` | PASS, 14.347s package time; five existing tests repeated 20 times; no race report in those exercised paths |
| `go test -p 2 -count=5 -run ^TestScan ./apps/viderse/internal/persistence/postgres` | PASS, 9.682s package time; 13 existing tests repeated five times; no observed flakes |
| Existing targeted resolver/upload-time fixture tests | PASS: unknown-handle classification and three upload-time parsing tests |
| Independent temporary store/worker probes | **12 failed behavior assertions / 1 successful cost measurement**; details above. Overlay test failures exposed product defects; no repository tests were altered or weakened |

No full build/vet/architecture run was performed by this reviewer. Coverage percentage, type-coverage percentage and lint counts were not measured. No actual `kill -9` subprocess recovery test was run; the existing crash test only expires a channel lease in SQL and reuses the same store/process. No production latency extrapolation is made from the local cost measurement.

## Source test parity: exactly what is not ported

The source list has **30 repository cases**. “Partial” below means an analogous named Go test exists but omits named invariants; it is not credited as a complete port. The SQLite migration itself is intentionally not applicable to a new Postgres schema.

| Source case / line in `scan-repository.test.ts` | Current Go coverage / missing assertions |
|---|---|
| Legacy migration, L99 | Not applicable as a SQLite migration; no equivalent old-schema state-seeding/order migration test required by this new schema |
| Exclusive leases until expiry + token fencing, L151 | **Partial**: wrong channel token and manual channel expiry are exercised; no worker acquire/renew/release lifecycle, exclusivity through expiry, or stale-transition interleaving |
| Request slot/jitter/crash expiry/429 penalty, L177 | **Not ported at the persistent multi-process scheduling boundary**; in-memory pacer unit tests are not equivalent |
| Far-away request window / monotonic finish, L205 | **Partial elsewhere** in in-memory pacer tests; no durable cross-process reservation/finish test |
| Unknown/idle/cached-count/dead-worker status, L224 | **Partial elsewhere** in rivals read tests for dead-worker display; scan service status permutations not tested |
| Initialize state from existing rows, L250 | **Not ported**: baseline test creates empty state before inserting videos; it does not initialize from pre-existing video rows |
| Start/cap/control continuation and durable pause/resume, L263 | **Partial**: cap/resume state only; no baseline handoff, remainder drain, durable cancel/restart/task effects |
| Finished-tab start + no-job controls no-op, L295 | **Not ported**; finished-tab probe fails |
| Initial import atomic profile/order/bookmark/freshness, L310 | **Partial**: channel/title/count/anchors checked; cursor/remainder, exact positions and freshness not asserted |
| History after import appends/drains saved continuation/counters, L335 | **Not ported** as an import handoff; crash test commits a manually supplied cursor into an existing job |
| Replay immutability/null preservation/metadata refresh, L356 | **Partial/phantom assertions**: test name claims position/origin/first-seen/null behavior, but it only checks added/scanned/indexed counts; no stored metadata values are inspected |
| Foreign-owned ID never moved, L387 | **Partial**: foreign channel count is checked, not foreign video ownership/title/views/position |
| Wrong/expired/taken-over page fence, L402 | **Partial**: wrong token and an expired head check; no page expiry/takeover matrix or transaction interleaving |
| Rejected page rolls back newly created profile, L422 | **Not ported** |
| Stale profile cannot resurrect a deleted rival, L437 | **Not ported** |
| Stopped job rejects whole page atomically, L461 | **Not ported**: failure-transition rejection is not a stopped-page rollback test |
| Bounded pages/checkpoint ownership restrictions, L476 | **Not ported** |
| Stable discovery block/history-excluded anchors/pre-upsert overlap, L488 | **Partial**: only a one-page overlap; no stable multi-page positions, history-origin anchor exclusion, or persisted watermark union |
| 10,000 overflow/safe-bound rebase, L524 | **Not ported** |
| Atomic job-less checkpoint / no checkpoint on overlap or end, L542 | **Not ported**; no checkpoint caller in current tests despite the implementation report claim |
| Unfinished discovery blocks second run / freshness / status, L590 | **Partial**: duplicate StartDiscovery false is checked, not freshness/status or stopped-run behavior |
| Median newest 150 then known views / exact result, L610 | **Partial**: head test merely asserts median is non-null; no exact median, null distribution, sample cutoff, rounding or stored score assertions |
| Null median / expired fence / throwing callback rollback, L631 | **Partial**: expired fence checked with scanned-at; null-median and full data/score rollback untested; source callback is not exposed by this Go API |
| Finishing head job must still be running, L655 | **Not ported** |
| Runnable priority/time/stopped jobs, L669 | **Partial**: one discovery beats a delayed history; stopped states, oldest-first ties, and skip exhaustion not asserted |
| Claim requires channel fence + runnable job, L698 | **Partial**: forged token rejects; stopped/not-yet-due claims and takeover during claim not covered |
| Failure writes fenced/running/sanitized, L715 | **Partial**: one backoff and a paused-job error rejection; no wrong-token transition/expiry/takeover matrix and only a weak sanitizer assertion |
| Cursor restart/replay-target clearing/progress retention, L733 | **Not ported** |
| Delete cascades state/jobs/videos, L758 | **Partial**: test checks channel/state read errors, not actual job/video/task deletion or mid-commit removal |

### Plan-required concurrency/crash tests

- **“Two claims, one write” is not proven:** `TestScanTwoConcurrentClaimsOnlyOneWrites` uses one valid token and one fabricated invalid token. It never races two real lease acquisitions and never performs a page write. C2 demonstrates a missing interleaving despite that green test.
- **Crash recovery is simulated, not process-level:** the existing test edits `lease_until` and manually commits a subsequent page. No worker/heartbeat/global lease, remainder, owner-PID adoption, post-commit/pre-complete crash, or actual killed process is exercised.
- No worker failure-sequence tests prove five consecutive failures → error, two cursor restarts → pause, discovery-before-history at real page boundaries, or the 10,000-video gap outcome. `ClassifyFailure`/backoff arithmetic tests alone do not test `recordFailure` state transitions.
- No tests for Add/Remove concurrent with the worker, mid-page pause/cancel, task-action routing, task cancellation/resumption, upload candidate bounds, SSE multi-channel timing, production idle defaults/past deadlines/platform pause, worker shutdown/join, or channel-fenced upload writes.
- No tests call `ParseChannelRef`, `ParseChannelCaptions`, or `FetchUploadTime`; existing upload-time tests cover only `ParseUploadTime`.

## Input/security and parity observations

- No direct SSRF was found in the inspected path: accepted URL hosts are checked for YouTube, and resolver input is sent as JSON to the client’s configured fixed YouTube `/navigation/resolve_url` endpoint, not fetched as an arbitrary supplied URL. The `@` shortcut is not an arbitrary-host HTTP fetch either. This is not a claim that all malformed references are correctly rejected.
- `ParseChannelRef` validation is permissive: any nonempty `@...` passes without segment/character checks; `/c/` and `/user/` accept arbitrary suffixes, and the reused channel-path regexp is unanchored. Add table tests for userinfo, ports, mixed-case hosts, Unicode/encoded path separators, control characters, malformed UC IDs, nested/video paths and long inputs; normalize to the accepted canonical channel forms at the boundary.
- Normal manual caption tracks (missing/non-ASR kind) and ASR exclusion match the requested distinction. Malformed/null caption entries currently also compare as non-ASR and return true; require a valid track shape before treating it as creator captions. No fixture currently proves the new client method’s caption output.
- All five `Recovery*` constants match the supplied Vietnamese RECOVERY text verbatim. Error classification and constants broadly match the source, but runtime sequencing is not established by the current tests; default backoff jitter is absent (m2). Median sampling order and upsert score formula look correct by inspection, but exact-value tests are missing and unconditional recomputation is expensive (M12).
- Existing rival HTTP routes use the gateway auth boundary; this review did not discover a new unauthenticated scan route, SQL interpolation, or user-input shell execution in scope. The sensitive task-operation route is missing rather than exposed without permission checks.

## Recommended disposition

1. Fix C1/C2 and M1/M2 first; these invalidate basic single-worker/fenced/recoverable/idle guarantees.
2. Complete task operations and lifecycle, enforce upload fences/sample bounds/platform pause, and fix bookmark/inline execution before calling phase-03 backend complete.
3. Fix removal lock order and scanner teardown/error reporting; remove unused alternate paths only after the intended live seams are connected.
4. Add deterministic regression cases described above, replace phantom assertions with stored-value assertions, then have the lead rerun race/repeated Postgres suites and the plan’s real killed-worker recovery scenario. Keep phase 03 **in progress**; no plan file was changed by this review.

Status: DONE_WITH_CONCERNS — review complete; commit blocked by two critical lease/fencing defects and the major production failures above. Existing race/repeated store suites passed, but 12 independent boundary probes failed; no product source was edited.


---

## Round 2 — fix-round-1 re-review

**Decision: STILL BLOCK COMMIT. Open findings: 0 critical / 8 major / 3 minor.** The original two critical defects are repaired, and all **12 original failing boundary scenarios now pass** in the recreated temporary probes. That does not establish readiness: the expanded review found **12 failing additional test cases**, grouped into the findings below, including an actual Go data race. The normal repeated suites still pass.

### Accepted decisions applied

Reviewed against `local://p03-fix-round1.md`, not against the superseded task-routing/pacer assumptions in Round 1:

- Rival task operations use authenticated `POST /api/rivals/:id/scan`; retry maps to resume. `TaskOp`/`TaskAction` helpers need not exist. Running discovery may expose Mở rather than cancellation.
- Heartbeat-only deadline reads while idle are accepted. Missing cross-process runnable work forever is **not** an accepted polling tradeoff.
- One in-memory pacer per process is accepted, provided the durable platform pause/429 boundary is enforced. No shared persistent request-slot scheduler is requested by this review.
- `resume` must recover both stopped history and stopped discovery. This does not authorize `check` to resume history that the user explicitly paused.

No product code, repository tests, or plan state was changed by the reviewer. Only this report was appended. Temporary probes used overlays under `/tmp/p3-review-scan` and were removed afterward. Existing source changes encountered during review were treated as the implementers’ changes; the valid capped-history recovery probe ran against the updated resume branch and passed.

### Original finding disposition

PASS refers to the original defect, not a blanket approval of the whole subsystem. “Partial” identifies a remaining/new failure in that original requirement.

| Original ID | Round 2 result | Evidence / disposition |
|---|---|---|
| C1 | **PASS original; new race open** | Real owner preserved, renew/release work; heartbeat loss now cancels the active fetch. New shared channel-lease mutation races with page commit: R2-01 |
| C2 | **PASS original; partial fence requirement** | Parent/state locks now prevent takeover while a transition is in flight. Expiry is still checked using a timestamp captured before waiting for the lock: R2-02 |
| M1 | **PASS** | Two non-overlapping discovery pages commit with increasing positions in one stable block. Deferrable `(channel_id, position)` uniqueness is used for rebase rather than per-page rebasing |
| M2 | **FAIL / regression** | The old past-rescan/backoff component probe passes, but idle heartbeat cannot discover a newly queued or already-due job: R2-03 |
| M3 | **PASS** | Baseline continuation persists; repository handoff test transfers it into history and clears the state bookmark |
| M4 | **PASS** | Newest 30 are selected before filtering unchecked rows; the original more-than-30 probe returns no candidate after that head is checked |
| M5 | **PASS original** | Upload result writes now take a channel lease, lock/check state, and scope the video update by channel. Probe fetch is registered for cancellation. R2-01 also affects the shared lease object used by probes |
| M6 | **FAIL / partial** | Worker page/probe entry gates and worker 429 recording exist; per-process pacer deviation is accepted. Add still fails open on a pause-read error and does not persist its 429 penalty: R2-04 |
| M7 | **PASS under accepted route mapping** | No task-specific endpoint required; backend scan action route exists, helper deletion is intentional. Valid capped-history + discovery-error resume probe passed. Global activity UI click dispatch was not exercised by this backend review |
| M8 | **PARTIAL** | Cancelled discovery no longer blocks a new check. Retrying a stopped exhausted run retains ordinal 10,000 and cannot recover: R2-05 |
| M9 | **PASS** | Inline selection uses `NextRunnableJobForChannel`; original unrelated-job probe makes no global selection |
| M10 | **FAIL / partial** | Missing-task relink/owner adoption and waiting-state refresh are present; startup reconciliation precedes launching this worker. Real task removal still fails, and create/link can outlive Remove: R2-06 |
| M11 | **PASS original** | Worker error channel/logging and shutdown cancellation/join added; bootstrap teardown is deferred. Join is bounded by a timeout, so the report does not claim an unbounded/non-cooperative dependency was tested |
| M12 | **PASS** | Unchanged medians skip the whole-channel score UPDATE. New local measurements below show 11–12ms unchanged checks, not seconds |
| M13 | **PASS** | Page and deletion now serialize parent → state; recreated remove-during-page test produces neither deadlock nor partial removal |
| M14 | **PASS** | Original 0/250/260ms overlap probe no longer emits the third event early; flush respects per-channel eligibility |
| m1 | **PASS** | Finished-tab no-op probe passes; baseline insert uses atomic DO NOTHING/exists mapping; Add lease release is deferred |
| m2 | **PARTIAL; progress regression is major** | Default jitter now uses a real random source, and explicit progress updates have a writer-side guard. Adopting a task still writes its progress to NULL outside that guard: R2-07 |
| m3 | **PARTIAL / minor** | Checkpoint types/path, duplicate CLI constructor and unused worker field removed. Unused `UploadTime` wrapper and `storeErr` remain: R2-11 |

### Original 12 probes — individual outcomes

All passed in Round 2:

1. `TestReviewWorkerLeaseIdentity` — C1 owner/renew/release.
2. `TestReviewHeartbeatLossAbortsActiveFetch` — C1 cancellation on loss (short injected heartbeat; no external request).
3. `TestReviewFenceTakeoverDuringTransition` — C2 takeover blocked while fenced transition holds the lock, old token rejected after takeover.
4. `TestReviewDiscoveryPreservesPageOrder` — M1 second-page stability.
5. `TestReviewDueTimerExcludesBackoff` — M2 original past-rescan/backoff component.
6. `TestReviewBaselineSavesCursor` — M3 baseline handoff bookmark.
7. `TestReviewUploadSampleDoesNotDrainHistory` — M4 newest-30 bound.
8. `TestReviewCancelledDiscoveryCanCheckAgain` — M8 cancelled-run recovery. Cancellation was seeded through SQL because the running-discovery cancel operation was intentionally removed by the lead decision.
9. `TestReviewInlineSkipsOtherChannelBeforeStopping` — M9 channel-filtered inline selection.
10. `TestReviewRemoveDuringPageCommit` — M13 parent-first serialization.
11. `TestReviewSSEFlushRespectsChannelInterval` — M14 delayed/immediate flush overlap.
12. `TestReviewFinishedHistoryIsNoop` — m1 ended-tab behavior.

## Round 2 open major findings

### R2-01. Heartbeat mutates the same lease struct that the page/probe goroutine reads — actual data race

**Locations:** `internal/scan/worker.go:269-298,551-554,639-646,909-914,954`.

`runOnePage` passes `&lease` to `setActive`. The heartbeat modifies `w.channelLease.Until` under `w.mu`, but the page goroutine copies that same struct into `CommitPageInput.Lease` without taking the mutex. Upload probes similarly share the lease pointer and later dereference it for `RecordUploadTime`. Locking only the writer does not protect the reader.

**Observed:** `TestReviewChannelHeartbeatDoesNotRacePageLease`, run with `-race`, reported a read in `Worker.runOnePage` at **worker.go:644** racing a write in `Worker.heartbeat` at **worker.go:296**. The test uses a 25ms provider delay and a 1ms injected heartbeat; production reaches the same overlap on a fetch lasting beyond a 15s heartbeat. This race is absent from the existing 20-repeat suite because its exercised fetches do not overlap renewal.

**Fix:** Keep an owned copy of the active lease inside the worker rather than a pointer to the page/probe local, or take immutable snapshots under the mutex for every reader. Renewal need not mutate the page’s token-bearing input at all; the database verifies current expiry. Add both listing and upload-probe race regressions.

### R2-02. Lease expiry is checked against time captured before acquiring locks

**Locations:** `internal/persistence/postgres/scan.go:253-259,1298-1315,1338-1354`; same pattern in `CommitPage`, `CompleteHeadCheck`, and upload-result checks.

Parent/state locking closes the old takeover window, but `now := time.Now()` (or caller `in.Now`) is captured before the transaction waits for the lock. A token that expires during that wait still passes `leaseHeld(state, lease, oldNow)` and can write after its expiry. Metadata timestamps and fence-validation time are being conflated.

**Observed:** `TestReviewExpiryAfterWaitingForParent` gave the lease another 100ms, paused the transition before its parent lock, waited until after expiry, then allowed it to lock/read/write. `SetJobBackoff` succeeded with **nil error** after the lease expired. No second owner or fabricated token was involved.

**Fix:** Check expiry with the current trusted time **after** acquiring the relevant locks (or database clock at the locked conditional write). Preserve `in.Now` only for intended metadata timestamps, never as authority for fence validity. Cover waiting-for-parent and waiting-for-state expiry for claim/page/head/transition/upload paths.

### R2-03. Idle heartbeat reads cannot wake for work created by another process, or for a deadline that became due

**Locations:** `queries/scan.sql:415-439`; `internal/scan/worker.go:464-508`.

`NextDeadline` only returns future backoff/rescan/upload-pause timestamps. It does not represent queued/running jobs or already-due backoff/rescan work. On heartbeat, `idle` stops the old timer, recomputes the query, and stays inside `idle` when that query returns nil. A CLI mutation’s in-memory wake does not reach the server’s service instance.

**Observed:** `TestReviewHeartbeatFindsNewRunnableWork` first let the loop enter idle, then made a job runnable as another process would and delivered a heartbeat tick. The worker never selected it and returned only on the test context deadline. This is an indefinite miss, not a ≤15s pickup delay. There is also a concrete timer interleaving: if heartbeat and the deadline fire together and heartbeat wins, the old timer is stopped and the now-past deadline is filtered out forever.

**Fix:** Make the single heartbeat scheduling read distinguish “actionable work now” from “no work”, including queued/recovered jobs, due backoff/rescan and relevant lease expiry. Exit idle when actionable work is discovered. Recompute a changed platform pause rather than keeping the original `extra` pointer indefinitely, and treat platform pause as a not-before gate rather than an earlier work deadline. Do not reintroduce busy looping on blocked/leased/stopped work.

### R2-04. Add bypasses the durable pause boundary when reading it fails, and does not persist its 429 penalty

**Locations:** `internal/scan/service.go:231-236,257-280,738-764`; `cmd/viderse/scan_wiring.go:27-58`.

`blockedFor` returns nil for a platform pause read error, allowing a YouTube request while the shared boundary is unavailable. Separately, Add maps a provider 429 to a response error but never calls `RecordBlocked`; a 429 on the inline client is therefore invisible to another process’s pause gate. The accepted in-process pacer deviation specifically retained the shared platform pause/429 boundary.

**Observed:** `TestReviewAddPauseReadFailureStopsRequest` injected a pause-store error and observed **one provider request** anyway. `TestReviewAddRateLimitIsPersisted` returned a provider 429 with one-minute Retry-After; Add returned rate_limited but the fake durable gate recorded **zero** penalty writes.

**Fix:** Propagate pause-store errors without issuing the provider call. Apply the same durable blocked/429 recording boundary for Add resolver/first-page failures as for worker page/probe failures, respecting Retry-After and returning the existing error mapping. Add is not exempt because it executes outside the worker lease.

### R2-05. Retrying an exhausted discovery run retains its exhausted cursor/ordinal/base rather than starting fresh

**Locations:** `internal/persistence/postgres/scan.go:1172-1185`; `queries/scan.sql:254-264`; `internal/scan/worker.go:663-674`.

For paused/error/capped discovery, `StartDiscovery(..., true)` calls the generic `ScanRequeueJob`, which resets failure counters but retains the old cursor, remainder, anchors, reserved base, newest ID and next ordinal. In particular, a gap_incomplete job remains at ordinal 10,000. Every subsequent check/retry immediately returns to gap_incomplete instead of performing a fresh check of the current head.

**Observed:** `TestReviewCheckAfterGapStartsFreshDiscovery` stopped discovery with `next_ordinal=10000`, invoked explicit replacement, and selected the next job: **NextOrdinal was still 10000**. The lead decision explicitly requires a fresh runnable check after a stopped run.

**Fix:** Retire the stopped discovery run under the existing parent/state lock and create a fresh run with a new stable reserved block/current anchors and ordinal zero. If a same-row reset is chosen, reset the entire run-specific state atomically, not only generic retry counters. Keep history continuation semantics separate.

### R2-06. Remove does not match real activity row JSON, and task creation/linking can outlive removal

**Locations:** `queries/scan.sql:384-397`; `internal/scan/service.go:607-652`; `internal/domain/task.go:47-51`; `internal/persistence/postgres/activity.go:90-95,128-140`.

Two concrete removal holes remain:

1. `ScanDeleteChannelTasks` filters `object_ref->>'id'`, but the real activity store marshals `domain.ObjectRef`, whose untagged Go fields produce **Type / ID / Label**. The SQL therefore misses real task rows even without concurrency. The repository cascade test manually inserts lower-case JSON and masks the mismatch.
2. `syncTask` reads the channel/job, calls `Tasks.Start`, and links the task in separate operations. `LinkTask` ignores zero affected rows. Remove can commit between the read and Start, after which Start inserts an orphan task and Link silently updates no job. Merely fixing JSON casing does not close this race.

**Observed:** `TestReviewRemoveClearsRealActivityRows` created a task through the actual `activity.Service`, verified it existed, performed ordinary Remove, and found **one task still present**. `TestReviewRemoveCannotLeaveOrphanTask` suspended real task creation until after Remove committed and also found **one orphan task**. Both use the real stored `ID` JSON key, not a fabricated lower-case shape.

**Fix:** Query the existing canonical stored identity shape consistently; test through the real activity service. Make task creation/linking atomic with live channel/job existence and the parent-first deletion order (or an equivalent ownership-safe operation). Treat failure to link to an existing live job as a real failure and ensure it cannot leave a newly created orphan. Do not swallow this as successful task synchronization.

### R2-07. Task adoption clears progress to NULL outside the progress throttle

**Locations:** `internal/scan/service.go:631-641`; `queries/activity.sql:38-49`; `internal/persistence/postgres/activity.go:145-160`.

`taskSpecOf` has nil progress. Every adoption calls `Tasks.Start`, which resets an existing task and explicitly assigns SQL NULL to progress. The subsequent Update only restores progress if `progressOf` was allowed and outside its 250ms guard. Control start/adoption uses `progress=false`, and page adoption can also clear the displayed progress while fetching. These writes bypass the purported centralized progress limiter.

**Observed:** `TestReviewTaskAdoptionPreservesProgress` created a real history task, set real progress to 3/150, then repeated start/adoption for the same history job. Its stored progress became **SQL NULL**. This is not just a missing initial progress assertion; existing progress was erased.

**Fix:** Adopt ownership/state without destructively resetting progress, or preserve current progress in the atomic adoption operation. Route all actual progress assignments, including NULL assignments, through the same frequency policy. Add real activity-store tests for repeated start, recovery adoption, consecutive pages and cached remainder pages.

### R2-08. Checking new videos silently resumes explicitly paused history

**Location:** `internal/scan/service.go:473-478`.

The check action unconditionally calls `ControlBackfill(...resume)` after scheduling discovery. This adds a side effect not authorized by the contract or `p03-fix-round1.md`: the accepted decision says **resume** recovers both streams; it does not say **check** unpauses history. A user can pause expensive history work, check new videos, and unexpectedly restart that history scan and its API traffic.

**Observed:** `TestReviewCheckDoesNotResumePausedHistory` paused an existing history job, invoked the real `Service.Control(check)`, then read the job: its state changed from **paused to queued**.

**Fix:** Keep check limited to a fresh discovery request. Keep the cross-stream behavior on the explicitly approved resume/retry action. If product intent is different, it needs an explicit revised decision rather than a source comment claiming an absent lead decision.

## Round 2 open minor findings

### R2-09. Reference table tests still omit malformed handles/nested paths that are accepted

**Location:** `internal/youtube/ref.go:22-23,43-51`.

**Observed:** `TestReviewRejectsMalformedHandles` accepted `@name/path`, `@name?bad`, `@name bad`, and `https://www.youtube.com/anything/channel/UCHnyfMqiRRG1u-2MsSQLbXA` (the test used the valid UC identifier from fixtures) as channel references. The last form is accepted because the channel regexp is unanchored. The ordinary valid/video/external-host table tests all passed.

**Fix:** Validate one complete handle segment and anchor accepted channel routes; reject embedded separators/query/whitespace before resolving. Keep valid international handles within YouTube’s documented format rather than enforcing an unnecessarily ASCII-only rule. No direct SSRF was found: the actual HTTP destination remains the configured YouTube endpoint.

### R2-10. Malformed caption entries are counted as creator captions

**Location:** `internal/youtube/uploadtime.go:67-76`.

**Observed:** `TestReviewMalformedCaptionEntriesAreNotCaptions` supplied `captionTracks: [null, {}]`; `ParseChannelCaptions` returned **true** because an absent kind becomes an empty string, which is not asr. Happy-path manual/ASR player fixture tests passed, but do not validate malformed track shape.

**Fix:** Require an actual valid caption-track object before applying the non-ASR distinction; ignore malformed/null entries. Add this table case without weakening manual-track support.

### R2-11. Two explicitly identified unused paths remain

**Locations:** `internal/youtube/uploadtime.go:104-108`; `internal/scan/errors.go:82-84`.

Caller search still finds no use of the `UploadTime` wrapper or `storeErr`. The unused checkpoint branch and duplicate constructor were removed correctly. This is the remaining portion of requested m3 cleanup, not a request for another abstraction or exported compatibility shim.

**Fix:** Delete these two unused paths, or document an actual verified external consumer before retaining a compatibility API. No such consumer was found in the repository.

### Round 2 exercised checks and measurements

All heavy commands were serialized through `lockf -k /tmp/verse-heavy.lock nice -n 10`, with `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4`. Store tests used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`. Package times below exclude some lock wait; they are not wall-clock performance comparisons.

| Check | Observed result |
|---|---|
| `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` (including DB integration tests) | **PASS**, 30.912s package time |
| `go test -p 2 -count=5 -run ^TestScan ./apps/viderse/internal/persistence/postgres` | **PASS**, 60.030s package time; no observed flakes |
| Recreated original 12 boundary probes | **12 PASS / 0 FAIL** |
| Expanded independent boundary probes | **12 FAIL / 1 PASS**; failing cases detailed above; valid capped-history + discovery retry case passed |
| `-race` on the independent worker probes | **FAIL: actual race**, worker.go:644 versus worker.go:296 |
| Current normal reference and FetchUploadTime fixture tests | **PASS**; independent malformed-reference and malformed-caption probes both failed |
| Repeated local head checks, 10,000 rows | Changed median **690.221ms**; unchanged medians **12.245ms**, **11.061ms**. M12’s conditional whole-channel update is fixed |

Initial temporary-fixture errors (cap 3, outside the schema’s minimum; lower-case task JSON selector) were corrected. Their invalid outcomes are excluded from the counts. The corrected real-shape task probes produced the failures recorded above. No network calls to YouTube were made; no failed product assertion was weakened to obtain a pass.

The store suite now has substantially stronger source-parity assertions, including stored metadata/positions/median/score values, real lease transitions, rollback, multi-page order, safe-bound rebase, full cascade and simulated cursor recovery. Nevertheless, no real killed subprocess/owner-PID restart exercise was performed by this reviewer, and the existing repeated suites do not cover the independently observed heartbeat/page race or cross-process idle liveness failure. Build/vet/global UI interaction were not rerun in this review.

### Round 2 disposition

Fix R2-01 through R2-08 before landing. The global lease owner and original locked-transition fence fixes are real improvements, but a passing 20-repeat race suite is not evidence against the independently exercised data race. Keep phase-03 readiness blocked until the new deterministic regressions pass and the plan’s process-level crash/restart check is exercised. No plan task state was changed by this review.

Status: DONE_WITH_CONCERNS — Round 2 complete. Original 12 boundary failures are fixed; **0 critical / 8 major / 3 minor findings remain open**, backed by 12 failing expanded cases including a race-detector report. Product source unchanged by reviewer; report appended and temporary probes removed.


---

## Round 3 — fix-round-2 re-review

**Decision: STILL BLOCK COMMIT. Open counts: 1 critical / 4 major / 2 minor.**

The **12 expanded probes that failed in Round 2 now all pass** when recreated against the new interfaces. Five additional top-level behavioral probes failed (one contains two failing state subtests). The required repeated checks are **not green**: the race repetition hit a false idle-test failure, and the Postgres repetition hit an isolated-schema naming collision. Neither was rerun merely to obtain a passing result.

Read-only review: no product code, permanent tests, migration, or plan state was edited. This section is the only repository output. Temporary Go overlays under `/tmp/p3-review-scan` were deleted after recording evidence. No YouTube network calls were made.

### Original R2 finding disposition

| ID | Recreated original probe / review result | Remaining qualification |
|---|---|---|
| R2-01 | **PASS** — `TestReviewChannelHeartbeatDoesNotRacePageLease` under `-race` | `setActive` owns a copy; no race warning in this probe. A different global-lease expiry defect is R3-C1 below |
| R2-02 | **PASS** — `TestReviewExpiryAfterWaitingForParent` | Clock is read after parent/state locks, not from metadata timestamps |
| R2-03 | **PASS original queued-job wake** — `TestReviewHeartbeatFindsNewRunnableWork` | New upload-only wake and fenced-upload spin defects remain: R3-M1/M2. Explicit heartbeat re-read of a cleared platform pause also passed |
| R2-04 | **PASS** — `TestReviewAddPauseReadFailureStopsRequest`, `TestReviewAddRateLimitIsPersisted` | No request on a pause-read error; provider 429 writes the durable penalty |
| R2-05 | **PASS** — `TestReviewCheckAfterGapStartsFreshDiscovery` | Replacement has a new job ID and ordinal zero |
| R2-06 | **PASS originals** — `TestReviewRemoveClearsRealActivityRows`, `TestReviewRemoveCannotLeaveOrphanTask` | Real stored `ID` shape is removed; atomic creation/link serializes with Remove. Current-state synchronization still fails: R3-M3 |
| R2-07 | **PASS original lost-progress case** — `TestReviewTaskAdoptionPreservesProgress` | Existing progress no longer becomes NULL. The hard progress-write budget is still bypassed: R3-M4 |
| R2-08 | **PASS** — `TestReviewCheckDoesNotResumePausedHistory` | Check leaves paused history paused |
| R2-09 | **PASS** — `TestReviewRejectsMalformedHandles` | Original malformed handle/nested-path cases rejected; international handle support remains |
| R2-10 | **PASS** — `TestReviewMalformedCaptionEntriesAreNotCaptions` | Null/empty tracks no longer count as creator captions |
| R2-11 | **PASS by caller/definition search** | Old `Client.UploadTime` wrapper and `storeErr` removed; only ParseUploadTime/FetchUploadTime remain |

The table accounts for all 12 previous failing cases: four worker cases, six store cases, and two YouTube parser cases. R2-11 was code cleanup, not one of those failing runtime cases.

## Round 3 critical finding

### R3-C1. An active fetch survives known global worker-lease expiry until the next heartbeat — and a blocking renewal has no expiry bound

**Location:** `apps/viderse/internal/scan/worker.go:263-288`; global-renewal call uses the outer worker context at line 279.

After a renewal error before expiry, heartbeat executes `continue` and waits for the next ticker event. Nothing is armed at `lease.Until`, so a fetch already in flight is not cancelled when ownership expires between ticks. `loop` checks `leaseExpired` only before the next page; it does not protect the current fetch/commit. A renewal call that blocks beyond the known expiry similarly cannot reach the expiry check because it has no lease-expiry deadline.

**Observed:** `TestReviewKnownWorkerExpiryCancelsFetchWithoutNextTick` used a 20ms heartbeat, a known lease expiring at 45ms, and temporary renewal errors. At 53ms the active fetch was **still live**, before the next 60ms tick. This is a scaled reproduction of a deadline between production 15s heartbeat ticks, not an assertion that production uses a 45ms TTL.

**Concrete production interleaving:** the last successful worker renewal sets expiry slightly after the ticker phase (e.g. 105.2s). Later renewals fail; the 105.0s tick fails before expiry and continues to 120s. A started a page at 60.4s, so its separately acquired channel lease can remain valid until 120.4s. At 105.2s B legally acquires the expired global lease and starts another channel. A still fetches and can commit at 110s under its valid channel token, only noticing global expiry afterward. There is one current DB owner, but **two processes performing scanner work**, violating the single-worker boundary. Channel fencing does not prevent work on different channels.

**Fix:** Enforce the known worker expiry independently of heartbeat cadence: an expiry deadline/watchdog must cancel the worker and active fetch even while renewal is blocked; successful renewal moves that deadline. Bound renewal itself by the currently held expiry. Do not wait for a later ticker or next-page check. Test a deadline between ticks and a renewal blocked past that deadline, plus takeover while the old fetch is in flight.

## Round 3 major findings

### R3-M1. `NextWakeup` omits ready upload-only work when no upload-pause row exists

**Locations:** `apps/viderse/internal/persistence/postgres/queries/scan.sql:470-510`; `internal/scan/worker.go:idle`.

`upload_candidate` is computed but is used only to include future channel-fence expiry. No ready row is added to `work` for it. Ready upload work is represented only indirectly through an existing `scan_schedule.upload_times_next_at` row. A newly added rival with unchecked head videos and no prior upload pause therefore has no actionable wake signal; a server already idle when another process adds that rival can wait until the future rescan deadline instead of probing the new sample.

**Observed:** `TestReviewWakeupIncludesUnpausedUploadOnlyWork` created a rival, completed its head check, released its lease, and verified `NextUploadCandidate` returned a video. `NextWakeup` returned **Actionable=false**, with the next rescan approximately one hour away. No job, pause, or busy fence was masking the work.

**Fix:** Represent eligible unpaused upload candidates explicitly as ready work, including the no-schedule-row case. Preserve the newest-30 sample boundary. Distinguish genuinely claimable upload work from a candidate still fenced by another owner so the fix does not introduce R3-M2 on every unchecked sample.

### R3-M2. Expired upload pause plus a busy channel fence causes a hot selection/claim loop

**Locations:** `apps/viderse/internal/persistence/postgres/queries/scan.sql:491-505`; `internal/scan/worker.go:958-976`, main loop’s probe → idle branch.

An expired `scan_schedule` pause makes `work.ready=true` whenever any unchecked upload sample exists, regardless of its channel lease. The probe selects that candidate, fails to acquire the channel fence, and returns `(false,nil)`. Unlike listing jobs, this path does not enter `idleFenced`; ordinary `idle` sees `Actionable=true` and returns immediately. The known future fence expiry in `Next` is ignored. The same candidate is selected repeatedly until its fence expires.

**Observed:** `TestReviewFencedUploadDoesNotBusyLoop` supplied exactly this SQL result shape (`Actionable=true`, a future fence expiry), an unchecked candidate, and a held channel fence. The real worker loop made **28,604 candidate selections in 40ms** using the in-memory store. This is a control-flow count, **not** a claim that Postgres executes that many queries in 40ms. With the real store each iteration performs repeated scheduling/selection/claim reads/writes.

**Fix:** On a probe fence miss, use the same bounded fenced-wait/eligible-selection semantics as listings, or exclude fenced candidates from the actionable result and expose the relevant expiry. Do not let expired pause state alone imply claimable work. Test both “no pause row” and “pause expired” against another process’s active fence, and allow other eligible channels to proceed.

### R3-M3. Atomic task sync still writes state from a stale caller snapshot and allows a running task for capped history

**Locations:** `apps/viderse/internal/persistence/postgres/scan.go:1529-1564`; `queries/scan.sql:ScanLinkTask`.

The new transaction fixes creation/removal atomicity and consistently locks parent → state. It reads the current job but only rejects complete/cancelled. Capped is accepted even though the service treats it as a task-ending state. For nonterminal states the current job state is ignored: the caller-supplied TaskRunning/phase can overwrite a paused job’s task after the pause committed.

**Observed:** `TestReviewTaskSyncRejectsStaleCappedAndPausedState` exercised two cases using real durable transitions:

- History reached 150 indexed videos and `SetHistoryCap(150)` made it capped. A queued/running snapshot then called `SyncScanTask`: it **created a running task** for that capped job.
- `ControlBackfill(pause)` committed first. A stale running snapshot then called `SyncScanTask`: the stored task became **running**, despite the job remaining paused.

**Concrete interleaving:** A selects/claims a job or prepares its task spec; B pauses it or lowers its cap; A subsequently gets the parent/state lock in SyncScanTask and adopts using the stale state. The lock prevents simultaneous writes but does not make A’s state current. A capped job is never run again to clean up its misleading running task.

**Fix:** Validate/map against the locked current job state at task commit, including rejecting capped/terminal jobs. Do not adopt a running presentation from a stale snapshot after pause/backoff/error/cap. Either derive the correct task presentation from the locked job or reject an incompatible expected-state/version so the current transition remains authoritative. Include pause/cap-versus-adoption regressions in addition to Remove-versus-insert tests.

### R3-M4. Progress adoption bypasses the limiter and duplicates each progress write

**Locations:** `apps/viderse/internal/scan/service.go:653-665,724-738`; `internal/persistence/postgres/scan.go:SyncScanTask → AdoptTask`.

When `progressOf` denies a progress update (or mode.progress is false), adoption unconditionally calls `progressNow`, which assigns the current timestamp without checking the budget. Its non-null progress is written once by SyncScanTask/AdoptTask and again by the subsequent `Tasks.Update`. Thus adoption has a separate unlimited path and duplicates the same update. The interface comment that spec.Progress is only used for a new task is not what the store implements.

**Observed:** `TestReviewAdoptionHonorsProgressBudget` performed ten adoptions of one existing history task at the **same injected instant**. It recorded **20 explicit non-null progress writes**: ten through the store and ten through activity updates. This does not count incidental SQL column assignments with nil progress; these are explicit progress payloads bypassing the central guard.

**Fix:** Preserve existing progress when merely adopting ownership/state. Limit every real progress payload through one writer budget; do not force a new payload when the guard says no. Initialize a genuinely new task once, and avoid writing the same progress again after atomic sync. If the second operation is only needed for notification, separate notification from another database progress write. Verify repeated start/adoption and cached-remainder pages stay within the same per-channel budget.

## Round 3 minor findings / verification reliability

### R3-m1. Idle test snapshots before the worker actually becomes idle, producing a false failure

**Location:** `apps/viderse/internal/scan/scan_test.go:178-197`.

**Observed required run:** `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` failed at `TestWorkerIdleDoesNotPoll`: **“worker thăm dò khi rảnh: 0 → 1”**. There was no race-detector warning in the output.

The test waits only for `runnable >= 1`, which is incremented **before** the initial upload-candidate selection and initial NextWakeup read. It can snapshot selects=0, yield to the worker, then count the worker’s legitimate first selection as forbidden polling. This is a test synchronization bug, not evidence that the empty-work loop polled.

**Fix:** Synchronize on the worker having completed its first idle scheduling read (or an explicit idle-entry barrier) before taking baseline counters. Keep cancellation/join cleanup even on fatal assertions. Do not solve this with a longer sleep or by weakening the no-poll assertion.

### R3-m2. Test schema naming can collide across repeated/stale/concurrent runs

**Location:** `platform/pgtest/pgtest.go:39-50` (shared harness, outside product scan code).

**Observed required run:** the repeated Postgres scan/activity command failed creating schema **pgtest_testscanfenceheldduringtransitionblockst_53000**, SQLSTATE **42P06 already exists**, in `TestScanFenceHeldDuringTransitionBlocksTakeover/capped` at `scan_pg_test.go:698`.

The helper truncates the test/subtest name to 40 characters and appends `UnixNano() % 1_000_000`; the suffix repeats every millisecond and is not a unique run identity. Long sibling/subsequent runs share the truncated prefix. The origin of the pre-existing schema was not established, and the reviewer did not delete another run’s schema. The observed failure is in fixture creation, not in the fenced transition under test.

**Fix:** Use a collision-resistant per-provision identifier (e.g. a random suffix with the full name bounded to Postgres’s identifier limit), rather than a modulo clock. Preserve isolated cleanup. Treat the repeated Postgres acceptance as failed until the fixture naming is repaired; do not rerun unchanged to hide the collision.

## Explicit rewrite checks that did not produce an open defect

### SyncScanTask locking and Remove

All channel mutators examined acquire the parent/state boundary before job/video work. SyncScanTask holds that same boundary across task insert/adopt and job link. The recreated mid-insert Remove probe blocked removal, then after task/link committed removal deleted everything. The ordinary real-activity-row removal probe also passed. No new parent/child deadlock was observed. This does **not** close the stale-state defect in R3-M3: serialization and current-state correctness are separate guarantees.

### Activity Start / phase-02 callers

Reviewed `toolsvc.initInstallRun`/`reserveInstall`/`runInstall`, storage startMove, and setup’s delegation to toolsvc.Ensure:

- failed rows still go through ResetTask, clearing stale progress on retry;
- active adoption uses AdoptTask and preserves unspecified progress;
- storage explicitly supplies 0/0 on a new move, so it still resets progress intentionally;
- toolsvc joins an already live local install rather than repeatedly starting a task;
- setup has no independent activity Start path; it delegates tool installation.

`TestReviewPhaseTwoStartAdoptAndRetry` used the real activity store to verify active tool adoption preserves 30/100, failed tool retry clears it, and storage’s explicit 0/0 still replaces old progress. **PASS**. Full `internal/activity` suite with `-count=3` also **PASS** (0.537s package time). No phase-02 production regression was established in these inspected paths.

### Cancel/reinsert retention and rivals latest-job read

A pruning change landed during review; its current implementation was inspected and exercised. `ScanPruneFinishedJobs` keeps the newest terminal row per channel/kind and is called in the insertion/completion transactions. It does not prune live paused/error/running work. The recreated 20-run error/replacement/completion sequence finished with **zero cancelled rows and one completed row**, not linear cancelled-row growth.

`rivals.sql` now orders using `(state NOT IN (complete,cancelled)) DESC, updated_at DESC`, and `scan_jobs_latest_idx` uses the matching expression/order. Cancelled no longer outranks complete. The real rivals sidebar read after the 20-run sequence reported no discovery running; ScanStore.Status correctly returned no live discovery plus caught_up state. **PASS**. The initial temporary retention probe incorrectly dereferenced a terminal DiscoveryJob (Status intentionally returns only live discovery); that fixture assumption was corrected and its panic is not counted as a product defect.

## Round 3 verification ledger

All heavy commands used `lockf -k /tmp/verse-heavy.lock nice -n 10`, `GOTOOLCHAIN=go1.26.0`, and `GOMAXPROCS=4`. Store tests used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`. Times are observed Go package times, not lock-wait wall times.

| Exercised check | Result |
|---|---|
| Scan `-race -p 1 -count=20` including DB integration tests | **FAIL**, 61.484s; idle-test initial-selection false failure (R3-m1), no reported data race |
| Postgres `-p 2 -count=3 -run ^Test(Scan|Activity)` | **FAIL**, 60.597s; schema 42P06 collision (R3-m2). No separate green claim for the combined acceptance |
| Full `internal/activity`, `-p 2 -count=3` | **PASS**, 0.537s |
| Recreated 12 Round-2 failing probes | **12 PASS / 0 FAIL** |
| Additional global-expiry probe | **FAIL**, active fetch live after known expiry |
| Additional upload-only wake / busy-upload / stale-task / progress-budget probes | **4 top-level tests FAIL**, with capped and paused subtests both failing inside the stale-task test |
| Heartbeat re-read of cleared platform pause | **PASS**; ready work did not bypass the active pause and resumed after the clearing heartbeat |
| Real activity phase-02 adoption/retry/storage semantics | **PASS** |
| 20-run retention/latest-status/sidebar probe | **PASS**; final terminal row count bounded and no stale running discovery |

No failed product assertion was weakened. The corrected terminal-status fixture assumption is explicitly excluded. No full build/vet/global UI interaction or real kill-9 subprocess recovery was rerun by this reviewer. Temporary overlays were removed after the results were recorded.

### Round 3 disposition

Repair R3-C1 and R3-M1–M4 before landing. The original twelve failures are closed, but lease-expiry cancellation, upload scheduling/fence behavior, durable task state and progress-write limits still violate production requirements. Repair the two verification reliability issues before treating repeated checks as acceptance evidence. No plan task state was mutated.

Status: DONE_WITH_CONCERNS — Round 3 complete; **1 critical / 4 major / 2 minor findings remain open**. Original twelve expanded probes pass; five additional behavioral tests fail, and both required repeated acceptance commands failed for documented harness/test reasons. Product source unchanged by reviewer; report appended and temporary probes removed.


---

## Round 4 — fix-round-3 re-review

**Decision: STILL BLOCK COMMIT. Open counts: 0 critical / 2 major / 1 minor.**

The global-lease expiry blocker is fixed, both required repeated suites are green, and the original R3 upload-only / upload-fence / paused-state / progress-budget cases pass. Expanded scheduling and task-presentation checks still found **five failing test scenarios**, grouped into the three findings below. No product source, permanent test, migration, or plan state was edited by the reviewer. Only this report was appended; temporary overlays under `/tmp/p3-review-scan` were removed afterward. No YouTube network calls were made.

Reviewed against `local://p03-fix-round3.md`, current source, ScanCore §8 Fix round 3, and ScanStore’s corresponding fix notes. Source checks covered lease-watch/session teardown/reacquisition, the executed `ScanWakeup` query, candidate filtering, task state/presentation writers and expected job_stopped handling.

### R3 finding disposition

| Original finding | Round 4 result | Exercised evidence |
|---|---|---|
| R3-C1 known expiry / blocked renewal | **PASS** | Deadline between ticks cancels the active fetch before the later tick; blocked renewal has a context deadline no later than the held expiry; zero commits and one release in each expired session. Real Postgres takeover also passed |
| R3-M1 upload-only wake without schedule | **PASS** | Fresh rival, completed head, released channel lease, unchecked sample, no scan_schedule row: candidate exists and Actionable=true |
| R3-M2 fenced upload spin / starvation | **PASS original upload cases** | Both no-pause-row and expired-pause variants return no foreign-fenced candidate and no actionable work; release makes the sample actionable. Another unfenced channel B is selected while A is fenced. The old adversarial repeat-candidate worker probe is bounded to at most two selections in 40ms. A separate recovered-running-job hot loop remains: R4-M1 |
| R3-M3 stale paused/capped task state | **PASS original state checks; partial requirement** | Capped snapshot returns job_stopped; stale running snapshot on a paused job writes paused, not running. But phase/action remain stale, and ordinary progress bypasses this locked writer: R4-M2 |
| R3-M4 adoption budget | **PASS** | Ten adoptions at one injected instant produce at most one explicit progress payload for both new-link and existing-link cases; no duplicate progress in the post-sync activity update |
| R3-m1 idle-test barrier | **PASS repeated suite** | Required `-race -count=20` completed green; no original initial-selection false failure |
| R3-m2 schema names | **PASS repeated suite** | Current random suffix preserves uniqueness through the identifier bound; required `-count=3` completed green, no 42P06 collision |

## Open major findings

### R4-M1. Any retained running row makes the worker hot-loop despite another process’s valid channel fence

**Locations:** `apps/viderse/internal/persistence/postgres/queries/scan.sql:499-505`; `internal/scan/worker.go:677-712` (`idleFenced`) and the listing fence-miss path.

`ScanWakeup` now correctly gates queued/backoff/rescan/upload work by channel fence, but separately adds unconditional ready work for **any** `scan_jobs.state = running`. Its comment calls that a running job “of this worker”, yet the query has no owner parameter or predicate that establishes that. A running row left by a dead/expired worker can still have a valid residual channel lease. The new owner cannot claim it until that lease expires.

The loop selects that row, fails AcquireChannelLease, exhausts its busy skip set, clears it, then calls idleFenced. The unconditional Actionable=true immediately returns to selection. This repeats without a timer. The bounded repeated-candidate guard added for upload probes does not cover listing jobs.

**Observed SQL evidence:** `TestReviewRunningForeignFenceIsNotActionable` used a real claimed running history row, a valid foreign/residual channel token, fresh head, and all upload samples marked checked. `NextWakeup` returned **Actionable=true** plus a future Next fence expiry. Thus neither upload work nor overdue rescan caused the ready bit.

**Observed runtime evidence:** `TestReviewRecoveredRunningFenceDoesNotBusyLoop` supplied that same query shape and the unavailable channel fence to the real worker loop. It attempted channel acquisition **25,500 times in 40ms** with the in-memory store. This is a control-flow count, not a claimed Postgres throughput rate.

**Concrete production scenario:** after expiry/restart the new global worker acquires ownership while an old running job’s channel lease still has time remaining. Global exclusivity is correct, but recovery pegs the scheduling/claim loop until that residual channel fence expires, instead of sleeping. A provider cancelled at worker expiry can also leave such a residual lease because its page cleanup uses the cancelled work context.

**Fix:** Treat retained running work as claimable only when its channel fence is available; include running rows in the same fenced channel-work calculation rather than an unconditional ready branch. Retain the future fence expiry in Next. An idle worker is not currently fetching under a channel lease, so a valid residual channel fence must not be assumed to belong to the current idle session. Add a real stored-running-row recovery regression and retain the worker-level bounded-selection assertion.

### R4-M2. Task presentation is not fully tied to current job state; stale progress can still undo a committed pause

**Locations:** `apps/viderse/internal/persistence/postgres/scan.go:1566-1598`; `internal/scan/service.go:671-678` (also the failed-task update at 685-695).

There are two remaining paths through the same stale-presentation boundary:

1. **Atomic adoption maps only State.** SyncScanTask reads the locked job and maps it to TaskPaused, but copies Phase and Action from the old caller spec unchanged. The service then reapplies those same old fields in a separate activity Update. Mapping the state enum alone does not produce a correct paused/recoverable task.
2. **Ordinary progress never uses that locked-state protection.** With an existing TaskID and adopt=false, syncTask writes `State`, `Phase`, and `Action` directly through Tasks.Update from its earlier job snapshot. A pause can commit after that snapshot and before this update; progress then overwrites the task back to running even though the durable job remains paused.

**Observed adoption evidence:** `TestReviewPausedTaskActionComesFromLockedState` paused a real history job, then supplied the formerly running spec to SyncScanTask. The stored row was **state=paused**, **phase=“Đang đọc tab Videos”**, **action={Label:Huỷ, Op:cancel}**, not the required paused/resume presentation. The task button offers cancellation rather than recovery.

**Observed real service/worker interleaving:** `TestReviewPageProgressCannotUndoCommittedPause` ran an actual page through Worker.RunInline with the real scan/activity stores. It stopped the prepared running progress update immediately before Activity.Update, committed Service.Control(pause) and its task update, then released the older progress update. Final database rows were **job=paused, task=running**. The worker joined successfully, leaving the incorrect task durable; this is not just a synthetic store call or transient UI render.

**Fix:** Make the same current-job boundary authoritative for **all** task state/phase/action writes, including normal progress and failed-task updates. Derive the full presentation from the locked current job or reject an incompatible snapshot and refresh it. A progress-only update must not reassert an old running state/action after pause/cap/cancel. After atomic sync, do not reapply stale phase/action in a second activity write; publish the change without undoing the authoritative result. Regressions must assert phase/action as well as enum state and include pause-after-snapshot-before-progress-commit.

## Open minor finding

### R4-m1. Caller cancellation during the lease-acquisition query returns context.Canceled instead of nil

**Location:** `apps/viderse/internal/scan/worker.go:269-271,314-318`.

Run correctly returns nil when cancellation occurs during an active session or a wait, but not while AcquireWorkerLease is blocked. acquire returns ctx.Err and Run returns that error directly before reaching the later cancellation normalization.

**Observed:** `TestReviewCancelDuringAcquireReturnsNil` blocked AcquireWorkerLease until the caller cancelled its context. Worker.Run returned **context canceled**, contrary to the requested normal caller-cancellation contract. The acquisition goroutine did exit; this is an error-contract inconsistency, not an observed leak.

**Fix:** Normalize acquisition errors caused by the now-cancelled caller to nil at the Run boundary while preserving real acquisition errors. If acquisition already returned a lease, still run bounded release cleanup rather than dropping ownership. Add cancellation-during-query coverage alongside the passing cancellation-during-session test.

## Lease reacquisition / teardown and stopped-job regression evidence

- **Known expiry between heartbeat ticks:** both fast-error renewal and blocked-renewal variants passed. Each session cancelled the provider, committed zero rows, joined heartbeat, and released once. Blocked renewal’s deadline was no later than the held lease expiry.
- **Real DB takeover mid-fetch:** the independent Postgres probe shortened the real stored global lease to 200ms. A second owner could not acquire before expiry. At expiry the old provider’s context was cancelled; the test kept its cleanup in-flight while the second owner acquired, then let old cleanup finish. A third owner was still rejected, proving old cleanup did not release the replacement owner. Video count was unchanged: no stale page commit. **PASS** under `-race`.
- **Reacquisition after expiry:** an instrumented two-session Run showed exactly two successful acquisitions and two releases, no acquisition while the previous session was still held, maximum one active renewal, and zero outstanding renewal calls/held leases when Run returned. Caller cancellation during the second active session returned nil. **PASS**. This verifies owned worker/heartbeat teardown in the exercised path, not a whole-process heap/goroutine census.
- **Expected job_stopped in service callers:** running/error snapshots whose atomic sync returned job_stopped produced one sync attempt, zero follow-up activity updates, and zero attention notifications; there was no retry/log loop. **PASS**. Actual capped-job SyncScanTask also returned job_stopped. Expected stopping is handled quietly; R4-M2 concerns successful stale writes that bypass or incompletely apply the current-state boundary.
- **Upload scheduling:** current SQL correctly handles no schedule row, expired pause, future foreign fence, and another eligible channel. The remaining scheduling defect is specifically the unconditional retained-running branch, not the previously repaired upload sample path.

## Round 4 verification ledger

All heavy commands used `lockf -k /tmp/verse-heavy.lock nice -n 10`, `GOTOOLCHAIN=go1.26.0`, `GOMAXPROCS=4`; Postgres checks used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`. No failed product assertion was weakened or rerun unchanged merely to obtain green output.

| Exercised check | Result |
|---|---|
| `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` | **PASS**, 67.279s package time; no reported race or original idle-test failure |
| `go test -p 2 -count=3 -run ^Test(Scan|Activity) ./apps/viderse/internal/persistence/postgres` | **PASS**, 64.949s package time; no observed flakes/schema collision |
| Recreated R3 lease deadline / blocked renewal probes | **PASS**, both variants; no post-expiry commit |
| Real stored-lease takeover while old provider cleanup remained in-flight | **PASS** under `-race`, 0.43s test duration |
| R3 upload-only wake; no-pause/expired-pause foreign fences; available channel B | **PASS** |
| R3 paused/capped state checks | **PASS**: paused maps to paused, capped rejects with job_stopped |
| R3 adoption progress budget, linked/unlinked variants | **PASS**, at most one explicit payload at one instant |
| Reacquisition/join and active-session caller cancellation | **PASS** |
| Expected stopped-sync notification/update-loop check | **PASS** |
| New recovered-running SQL readiness + worker hot-loop probes | **FAIL**, both evidence cases for R4-M1 |
| New paused phase/action + pause-before-progress-commit probes | **FAIL**, both evidence cases for R4-M2 |
| New cancellation-during-acquisition probe | **FAIL**, R4-m1 |

Temporary-probe setup mistakes were corrected before counting behavior: serve now takes WorkerLease by value, and the real takeover fixture uses the current fixture channel constant and measured baseline count. Their initial compile/not-found outcomes are not product defects. The corrected takeover case passed. All temporary probes were deleted afterward.

No full build/vet/UI interaction or actual kill-9 subprocess scenario was rerun by this reviewer. Phase-02 activity behavior was not changed or re-audited beyond the current scan integration; Round 3’s explicit adoption/retry/storage compatibility evidence remains the earlier exercised evidence.

### Round 4 disposition

Close the old global-lease expiry blocker and the original upload/progress cases, but do not land until R4-M1 and R4-M2 are repaired and the caller-cancellation contract is consistent. Keep the now-green repeated suites as baseline evidence; they do not cover the five new deterministic failing scenarios. No plan task state was changed.

Status: DONE_WITH_CONCERNS — Round 4 complete; **0 critical / 2 major / 1 minor open**. Both required repeated suites pass. Five new failing scenarios expose recovered-running hot polling, stale task state/actions, and acquisition-cancellation inconsistency. Product source unchanged by reviewer; report appended and temporary probes removed.


---

## Round 5 — fix-round-4 re-review

**Decision: STILL BLOCK COMMIT. Open counts: 0 critical / 4 major / 0 minor.**

All **five Round-4 failing scenarios now pass**, and both required repeated suites are green. The requested wider regression checks exposed **five additional failing scenarios**, grouped into four major findings below: a discovery button operates on history, skip filtering after LIMIT hides eligible work, failed-task notification is neither idempotent nor authoritative, and healthy task changes no longer publish activity events.

Read-only review against `local://p03-fix-round4.md`, ScanCore §9, ScanStore Fix round 4, and current source. No product source, permanent tests, SQL, migration or plan state was changed. Only this report was appended. All independent probes used temporary Go overlays under `/tmp/p3-review-scan`, subsequently deleted. No YouTube network calls were made.

### Previous findings / requested regression disposition

| Check | Round 5 result |
|---|---|
| Retained running row with residual fence — actual SQL readiness | **PASS**: Actionable=false and future fence deadline present |
| Recovered-running worker hot-loop count | **PASS**: bounded selection under an adversarial ready result; no original hot loop |
| Paused state + phase + resume action after stale sync | **PASS**: full presentation comes from locked job through TaskPresentation |
| Real store pause between page snapshot and progress sync | **PASS**: final job=paused and task=paused |
| Caller cancellation during AcquireWorkerLease | **PASS**: Run returns nil |
| No direct scan Tasks.Start/Tasks.Update paths | **PASS**: scan state/phase/action/progress writers use SyncScanTask; Tasks interface has only Finish/Notify. Terminal task deletion remains intentional |
| Expected job_stopped failure sync | **PASS**: one attempt, zero notifications, zero direct task writes |
| busyRetry with one fenced A and eligible B | **PASS**: B claimed without waiting for the long retry interval |
| Wake interrupting an entered repeated-candidate wait | **PASS**: Wake interrupted a configured 3s busyRetry; B selected within the test’s 150ms bound |
| Fairness past the first 50 skipped rows | **FAIL**, R5-M2 |
| One failure → exactly one notification, including refresh/recovery races | **FAIL**, R5-M3 |
| History Huỷ/Tiếp tục/Thử lại, normal discovery Mở, terminal visibility | **PASS** for exercised states; discovery backoff button is wrong, R5-M1 |
| Concurrent RecordBlocked between expired-pause read and clear | **PASS**: new pause/reason/count preserved; stale clear affected zero rows; next read sees new pause |
| Normal unpaused/active-pause reads | **PASS**: no clear/write on ordinary reads |

## Major findings

### R5-M1. Discovery backoff exposes Huỷ, but that action cancels history instead of discovery

**Locations:** `apps/viderse/internal/scan/service.go:573-578` (shared backoff presentation), `service.go:475-488` (Control pause/cancel dispatch).

TaskPresentation applies the history backoff action to both kinds: TaskWaiting + Huỷ/op=cancel. The accepted scan endpoint’s cancel operation is history-only. Unlike running/queued discovery’s Mở link, the backoff discovery button therefore invokes an operation on the wrong stream.

**Observed real-store test:** `TestReviewDiscoveryBackoffActionDoesNotCancelHistory` created a queued history job and a discovery job in future backoff, then invoked the exact op exposed by the discovery task through Service.Control. Result: **history=cancelled, discovery=backoff**. The clicked discovery run remained scheduled, while unrelated history work was stopped.

**Fix:** Under the accepted channel-scoped endpoint contract, give non-error/non-paused discovery backoff a non-destructive Mở action with the canonical channel href (while preserving its waiting state/phase). Do not expose a history-only operation on discovery. If discovery cancellation is desired instead, it requires an explicitly distinguishable API contract; the current generic cancel cannot identify that stream. Add an end-to-end action-dispatch test for every discovery state, not just a pure-function test of running discovery.

### R5-M2. Runnable skip filtering occurs after LIMIT 50, hiding eligible work and defeating the retry guard

**Locations:** `apps/viderse/internal/persistence/postgres/queries/scan.sql:294-301`; `internal/persistence/postgres/scan.go:1282-1295`; `internal/scan/worker.go:517-523,557-583`.

ScanRunnableJobs takes the first 50 ordered rows before NextRunnableJob filters the caller’s busy-channel set in Go. If those 50 are all skipped, it returns nil even when row 51 is eligible. The worker clears the busy set, sees Actionable from the eligible channel behind the query limit, and scans the same blocked prefix again. lastFenced remembers only the last channel in that prefix; the next cycle starts with a different channel, so the repeated-single-candidate sleep does not catch this cycle.

**Observed real-store test:** `TestReviewRunnableSkipMustPrecedeLimit` created 51 ordered history jobs; the first 50 channels retained valid fences and were in skip, while the 51st channel’s fence was released. `NextRunnableJob(skip)` returned **nil**, despite the known eligible 51st job.

**Impact/interleaving:** the first 50 candidates fail acquisition → all enter busy → selection returns nil → busy is cleared → NextWakeup says ready because the 51st channel is free → the same first 50 are fetched again. The eligible job is hidden until that prefix changes or fences expire, and the interval can hot-poll the database. The ordinary two-channel fairness and Wake tests pass but do not cover this bounded-query edge.

**Fix:** Apply skip/claimability filtering before the SQL LIMIT, or paginate the ordered candidates until a non-skipped job is found. Preserve discovery-first/oldest-first ordering and bounded reads; do not replace this with an unbounded full-table fetch or another arbitrary larger limit. Add a >50-candidate fairness regression alongside the single-fenced-candidate guard test.

### R5-M3. Failure notification is emitted from a stale snapshot on every sync, not once per real failure

**Locations:** `apps/viderse/internal/scan/service.go:662-685,689-692`; `internal/activity/activity.go:214-220`.

The locked store now derives authoritative task state correctly, but service computes a separate view from the earlier snapshot. After any successful sync it unconditionally notifies whenever that old view says failed. There is no failure-transition/idempotency check. The 10s notification merge is not deduplication: each call increments notification count and inserts another error_event. Also, a successful locked sync can now represent a resumed job while the pre-lock view still triggers a failure notification.

**Observed real worker/store evidence:**

- `TestReviewOneFailureNotifiesExactlyOnceAcrossRefresh` made **one** provider call returning a blocked error. Initial failure produced one notification. Two subsequent check actions merely refreshed the still-failed history task. Final result: **notification count=3, error_events=3, provider calls=1**.
- `TestReviewStaleErrorSnapshotDoesNotNotifyAfterResume` suspended an error snapshot immediately before atomic sync, durably resumed history, then let sync proceed. The stored job was **queued**, but notification count increased from **1 to 2** from the stale failed view.

The expected job_stopped test passes because sync returns an error there; it does not cover a still-live job whose current state differs from the caller’s snapshot.

**Fix:** Gate failure publication on the successful durable failure transition, not on refresh/render state. Record/identify that transition atomically (e.g. at the running→error write, or an idempotent record tied to it) so concurrent refresh/recovery and a crash between task sync and notification cannot duplicate or lose it. Make publication use authoritative current/transition data; a resumed job must not issue a new failure alert. Keep job_stopped silent. Add repeat-refresh, stale-after-resume and second-genuine-failure cases, asserting notification counts and error_events rather than just a single Notify call.

### R5-M4. Moving scan writes out of Activity.Update removed activity SSE publication for healthy tasks

**Locations:** `apps/viderse/internal/scan/service.go:681-686`; `internal/persistence/postgres/scan.go:1529-1618`; `cmd/viderse/serve.go:75`; compare `internal/activity/activity.go:169,195,207,220`.

SyncScanTask writes the database directly, but neither the store nor the healthy-success branch publishes an activity change. Previously Activity.Start/Update emitted through the activity service callback. The narrowed Tasks interface now has only Finish and Notify; on normal creation/progress/pause/resume there is no callback invocation at all. Rival SSE is a different topic and does not preserve the activity change contract. Failures can emit incidentally through Notify and deletions through Finish, masking the missing healthy path.

**Observed real-store test:** `TestReviewHealthyTaskPublishesActivityChange` supplied a real activity.Service with a recording change notifier, then started history through Service.Control. One background task was durably present, but the activity notifier was called **zero times**.

**Fix:** After a successful atomic sync, publish the activity task change through a non-mutating activity publisher, using the returned task ID. Reuse/expose the existing event callback capability without bringing back Tasks.Update with stale state/phase/action. Do not create notifications for ordinary progress just to trigger SSE. Test create, progress, pause and resume publication, and ensure rejected/stopped sync does not publish a fictitious task.

## Platform pause change — concurrency result

Reviewed `PlatformStateStore.Pause` and `ClearExpiredPlatformPause`. The conditional clear is safe for the requested storage race:

1. A reads an expired pause and is stopped immediately before ClearExpiredPlatformPause.
2. B calls real RecordBlocked and writes a new future pause/reason/failure count.
3. A continues its conditional UPDATE with `paused_until <= A.now`.
4. The UPDATE affects **zero rows**; B’s future timestamp and “concurrent new block” reason remain intact, and a subsequent Pause returns them.

`TestReviewExpiredPauseClearDoesNotEraseConcurrentBlock` **PASS**. A’s original call can still return nil from its earlier expired snapshot; the new durable pause is not erased, and the next read observes it. `TestReviewUnpausedAndActivePauseReadsDoNotWrite` **PASS**: fresh/unpaused and repeated active-pause reads execute no clear operation. No new defect was established in this change.

## Task-path / presentation audit

- Caller search and live-state probes found **no remaining scan Tasks.Start/Create or Tasks.Update**. Every live state’s task write goes through the 4-argument SyncScanTask. Existing task deletion uses Finish; attention notification uses Notify.
- Locked state/phase/action mapping repaired both original R4 stale-presentation scenarios. The real pause-after-snapshot test now intercepts before SyncScanTask rather than the removed Activity.Update path, commits pause first, then confirms **job=paused/task=paused**.
- Pure presentation/dispatch checks passed for history queued/running Huỷ, history backoff Huỷ, paused Tiếp tục, error Thử lại, normal discovery Mở with `#/explore?rival=<ChannelID>`, and no live task for capped/complete/cancelled. The discovery-backoff exception is R5-M1.
- Normal job_stopped remains quiet: one attempted atomic sync, no direct task writes or attention notifications. The duplicate/false failure problem is on successful sync, not on the stopped error branch.

## Round 5 verification ledger

All heavy commands used `lockf -k /tmp/verse-heavy.lock nice -n 10`, `GOTOOLCHAIN=go1.26.0`, `GOMAXPROCS=4`. Store/integration runs used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`.

| Exercised check | Observed result |
|---|---|
| `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` | **PASS**, 69.268s package time |
| `go test -p 2 -count=3 -run ^Test(Scan|Activity) ./apps/viderse/internal/persistence/postgres` | **PASS**, 63.509s package time |
| Five recreated R4 failing scenarios | **5 PASS / 0 FAIL** |
| Bounded retry with eligible B and Wake interruption | **PASS**, both targeted cases |
| No direct task writes + stopped-error notification suppression | **PASS** |
| Ordinary action mapping table | **PASS**; discovery backoff verified separately and failed end-to-end |
| Concurrent platform block/read/clear and normal read-only cases | **PASS**, both real-store cases |
| >50 skipped-candidate fairness | **FAIL**, available 51st job hidden |
| One physical failure across refresh; stale error after resume | **FAIL**, both real-store cases |
| Healthy activity change publication | **FAIL**, zero activity events despite task creation |
| Discovery-backoff action dispatch | **FAIL**, cancelled history and left discovery in backoff |

Independent scan integration probes ran under `-race`; no race-detector warning was reported. Their failures are behavioral assertions. No failed assertion was weakened, no product code was altered, and no unchanged failing check was rerun to obtain green output. All temporary probes were removed after recording the results.

No new full build/vet/browser interaction or actual kill-9 subprocess test was run by this reviewer; those earlier verification limits remain unchanged.

### Round 5 disposition

Close all Round-4 findings. Keep commit blocked on the four major regressions/gaps above. The repeated suites are a green baseline, but do not prove stream-safe actions, fairness beyond the query prefix, exactly-once failure publication, or activity SSE continuity. No plan task state was changed.

Status: DONE_WITH_CONCERNS — Round 5 complete; **0 critical / 4 major / 0 minor open**. Five previous failing scenarios are fixed and both repeated suites pass; five new failing scenarios substantiate the four major findings. Product source unchanged by reviewer; report appended and temporary probes removed.


---

## Round 6 — fix-round-5 and final SSE transport re-review

**Current open counts: 0 critical / 0 major / 1 minor. All four Round-5 major findings are closed by exercised evidence.** Earlier round dispositions above are historical, not additional open findings. One new, narrowly scoped Phase-02 helper return-value regression remains; fix it before landing to meet the unchanged-behavior requirement.

### Scope and scouting

Reviewed the current changes against `local://p03-fix-round5.md`, ScanCore §10 (including its final web-level throttle replacement), and the ScanStore helper extraction. Focus: scan task synchronization and failure publication, runnable selection, shared activity transaction behavior/lock order, and the final SSE wiring/transport. Followed dependents into `ActivityStore`, `activity.Service.Notify/Changed/Finish`, `web.Notifier`, the event hub/gateway, and `serve.go`/`machine_wiring.go` before forming findings.

Scouted boundary cases: failure followed only by refresh; stale failed snapshot after resume; second genuine failure; rollback after notification work; concurrent same-kind notifications from different channels; normal activity insertion holding the advisory lock while scan holds channel/task locks; skip sets exceeding the SQL limit; multiple task refs and Finish on one activity stream; trailing publication after shutdown. These probes target real stores and deterministic interleavings rather than only callback invocation counts.

Read-only product review. No product source, permanent tests, SQL, migrations or plan state changed. Only this report was appended. Temporary Go overlays under `/tmp/p3-review-scan` were used for independent probes and removed afterward. No YouTube network calls.

### Critical / major findings

None newly established in this round. The previous major findings are closed as documented below.

### Minor finding

#### R6-m1. ActivityStore discards its committed notification result and can return another writer’s row

**Location:** `apps/viderse/internal/persistence/postgres/activity.go:271-282`, especially the discarded helper result at line 271 and the post-commit `LatestNotification` at line 277.

`upsertNotificationTx` already returns the precise inserted/merged row while holding the same-kind advisory lock. Its public wrapper discards that result, commits (releasing the lock), then makes a new unprotected latest-by-kind query. Before extraction, the fresh-insert branch returned its own inserted row after commit; this changes that behavior. It also adds a fallible database read after successful durable insertion.

**Concrete observed interleaving:** `TestReviewActivityInsertReturnsItsOwnNotification` used two real `ActivityStore` instances and a valid zero merge window. A inserts notification A and commits; the probe suspends only A’s outside-transaction `LatestNotification`. B inserts a newer notification B of the same kind and commits. A continues its query and returns **B’s ID, text and href** instead of A’s. The assertion failed with A’s stored own ID `c3249c7f-01c4-43eb-992c-dcc3064cda6d`, but both returned objects identified B as `6b3d84e5-02e4-49f5-9124-476b9bac4d1a`, `Text:B`, `Href:#/b`.

The probe uses zero window to deterministically produce distinct rows without waiting ten seconds; the wrapper accepts that parameter. With the normal window, a concurrent merge can similarly change the returned text/count snapshot, and a sufficiently delayed caller can observe a later inserted ID. Database rows are not lost by this bug, so severity is minor rather than a scan data-integrity blocker. However, the generic activity API no longer reliably returns the row its operation produced; `activity.Service.Notify` returns/emits the returned ID, and the lead explicitly required unchanged helper behavior.

**Concrete fix:** retain the `*gen.Notification` returned by `upsertNotificationTx`, commit successfully, and convert/return that row. Do not replace the fresh insert result with a post-commit latest-by-kind lookup. Keep a regression covering concurrent same-kind writers and asserting the caller’s returned ID/text, not merely total row/count state. If retaining the old merge-path snapshot behavior is necessary, distinguish that deliberately; the fresh-insert branch must still return its own insertion.

### Round-5 regression disposition

| Previous failing scenario | Round 6 independent result |
|---|---|
| Discovery-backoff action cancelled history | **PASS**: current backoff presentation has the non-destructive canonical Mở link and no op; the old end-to-end dispatch assertion no longer fails. The repeated scan suite also covers discovery queued/running/backoff/error/paused dispatch. |
| First 50 skipped jobs hid available job 51 | **PASS**: the real-store 51-job reproduction now returns the available candidate beyond the skipped prefix. |
| One physical failure plus two refreshes made three notices/error events | **PASS**: provider calls=1, notification count=1, error events=1 after both refreshes. |
| Stale failed snapshot after durable resume emitted a false new notice | **PASS**: the barrier-before-sync/resume interleaving leaves notification count=1. |
| Healthy task persisted without activity change | **PASS**: real activity service receives the change when the healthy task is created. The full repeated suite covers create/pause/resume and stopped-sync silence. |

All five former failures were recreated against the new API/current source, not removed or weakened. Current task state/phase/action and failure-transition publication come from the same locked `SyncScanTask` boundary; service publishes returned IDs without reapplying the stale view.

### Atomic failure and shared activity helper probes

- **Rollback:** forced the error-event insert to fail inside failure synchronization. The transaction rolled back the task change, notice and event together: task remained running; notifications=0; error_events=0. A subsequent ordinary sync created the one intended failure notice.
- **Idempotency and second real failure:** repeated sync while failed returned no new NotificationID. Resume, sync active, claim, then a second genuine failure produced total notification count=2 and error_events=2. This tests a real transition rather than permanently suppressing all later errors.
- **Different-channel merge:** two failing channels synchronized concurrently. Both obtained the same merged NotificationID; durable state was one notification row with count=2 and two error events. No lost increment or duplicate fresh notification was observed.
- **Lock-order interleaving:** generic activity Upsert held the same-kind advisory lock. Scan synchronization then held its channel/task locks and reached that advisory lock. Releasing the activity writer let both operations finish within the probe’s three-second bound, with the same notification ID. Source audit found scan ordering channel/state/task → advisory → notification; the generic notification transaction has advisory → notification but no subsequent channel/task acquisition. No reverse transaction path was found in the reviewed production callers. This is exercised ordering evidence, not a claim that every possible application transaction was exhaustively model-checked.
- **Existing rules:** sequential real-store checks passed for ten-second merge, preserved CreatedAt, incremented count/replaced text, reset ReadAt on merge, a fresh row beyond the window, and error-event object/op roundtrip. **Return-value concurrency is the exception R6-m1**, not hidden by these passing row-state assertions.

### Runnable ordering and cost

The skip predicate is now in SQL before ORDER BY/LIMIT; no Go post-LIMIT filter remains. Nil and empty skip sets returned the first discovery candidate. With 20,800 jobs (400 queued discovery, 400 queued history, 20,000 completed history), a 500-channel skip set returned history candidate 501. Discovery still won with no skips even though the history rows had older timestamps.

Independent `EXPLAIN (ANALYZE, BUFFERS)` used `scan_jobs_runnable_idx` in both cases, not a scan of all 20,800 jobs. On this local fixture: no skips **0.535 ms / 19 shared buffer hits**; 500 skips **0.500 ms / 19 hits**, filtering 500 of 800 runnable candidates before the bounded result. Both used top-N heapsort (37 kB). These are fixture measurements, not a production latency guarantee; they establish retained index use and discovery/oldest ordering with a large skip parameter.

### Final SSE design — reviewed after the last §10 note

The earlier scan-level `refThrottle` is gone. The reviewed design limits the entire event type in `web.Notifier`, not individual refs. `serve.go:75-76,140` creates one activity notifier and passes it to both activity service and machine wiring; `machine_wiring.go:33,67` reuses that callback. Thus task sync, notification changes and Finish share one limiter. Reviewed production server consumers do not filter on `ref`: the gateway serializes event data and scopes visibility using event metadata. No server dependency requiring every per-ref hint was found; mixed refs can safely use the documented empty-ref refresh hint.

**Independent deterministic transport probe, under race detector:** forty alternating refs over 400 ms produced three frames at fake times 0/250/500 ms, monotonically increasing revs 1/2/3, mixed trailing ref empty, and final observed source version 40. Every emitted pair was at least 250 ms apart; the exercised rolling one-second windows contained at most four frames.

**Real long-history pipeline, under race detector:** sixteen provider pages produced **34 SyncScanTask calls, 34 raw Changed calls, but six activity frames**. Consecutive frame gaps were at least approximately 250 ms (publish times 21:08:32.680489, 32.931518, 33.182634, 33.433527, 33.683703, 33.934224 +07). The trailing frame exposed the final task deletion from Finish, not an earlier running snapshot. The raw counts also show why task-call count/per-ref throttling was not sufficient. Do not rely on the implementation note’s “sync already ≤1/page”: this exercised pipeline did more than one sync/page. Those calls are not equivalent to explicit progress payloads; the existing progress budget remains separate.

**Shutdown:** a real timer probe published a leading and pending change, then closed the hub. The queued trailing callback drained within one second; afterward the notifier had timer=nil and pending=false, had created only one timer, and produced no additional sequence increment during a further 300 ms. There is no notifier Stop API, and this is **bounded trailing drain, not immediate cancellation when hub.Close runs**. That matches the existing hub shutdown contract permitting producers to finish after subscribers close. The exercised notifier does not retain a periodic timer or resident goroutine. No claim of a process-wide goroutine census is made.

### Round 6 verification ledger

Final revision checks were run after the final web-level replacement landed. All heavy commands used `lockf -k /tmp/verse-heavy.lock nice -n 10`, `GOTOOLCHAIN=go1.26.0`, `GOMAXPROCS=4`; database runs used `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`.

| Exercised check | Result |
|---|---|
| `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` | **PASS**, 84.347s package time; no reported race/flaky assertion |
| `go test -p 2 -count=3 -run ^Test(Scan|Activity) ./apps/viderse/internal/persistence/postgres` | **PASS**, 65.567s package time |
| `go test -race -p 1 ./apps/viderse/internal/web ./apps/viderse/internal/activity` | **PASS**, 80.192s / 1.560s |
| Independent old-failure reproductions + sixteen-page final-state pipeline, scan overlay under `-race` | **PASS**, all five tests, 3.602s package time |
| Independent final per-type/mixed-ref and shutdown notifier overlay under `-race` | **PASS**, both tests, 2.215s package time |
| Independent Postgres overlay | **6 PASS / 1 FAIL**, 2.182s; sole failure R6-m1, not a race-detector diagnostic |

The Postgres passing overlay cases were >50 skip fairness, rollback/second failure, concurrent different-channel merge, generic-activity/scan lock ordering, old helper row-state rules, and large-skip EXPLAIN/ordering. Temporary harness compile errors from initially using an obsolete JobError field and omitting an event-list limit were corrected before these measured probes; they are not product findings. No product assertion was weakened and the new failing return-value probe was not rerun unchanged to obtain green output.

No new full build/vet/browser interaction, actual kill-9 subprocess scenario or broad phase-02 audit was performed by this reviewer. Existing earlier-round state-machine/fencing/coverage findings and evidence remain recorded above; this round does not claim to have rerun every earlier temporary experiment. Coverage percentages were not measured. No plan state was changed.

### Recommended action / plan follow-up

Close R5-M1 through R5-M4; the final transport replacement satisfies the exercised stream-rate and final-state cases, unlike the superseded per-ref attempt. Repair R6-m1 by returning the committed helper result, preserve the fresh-insert return contract with a concurrency regression, then let the lead rerun the focused activity/store check. Keep the raw sync-versus-frame counts above in the review record rather than treating the implementation note’s per-page assumption as evidence. No additional implementation scope or plan mutation is proposed.

Status: DONE_WITH_CONCERNS — Round 6 complete; **0 critical / 0 major / 1 minor open**. All five Round-5 failing scenarios now pass; final scan race×20, Postgres×3 and web/activity race suites pass. One deterministic new failure shows the generic notification wrapper can return another writer’s row after commit. Product source unchanged by reviewer; report appended and all temporary probes removed.
