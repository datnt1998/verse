# Phase 03 — ScanStore (Postgres) delivery

Phạm vi: `apps/viderse/internal/persistence/postgres/{scan.go,scan_pg_test.go}`,
`internal/persistence/postgres/queries/scan.sql` (+ `gen/`), migration
`internal/persistence/migrations/00006_rivals.sql` (chưa phát hành, sửa tại chỗ).

## What this store is

One transaction per method, every channel-scoped write fenced by
`scan_state.lease_*`. This round rebuilt the transaction skeleton so the fence
is authoritative for the whole transaction, and ported the remaining
`scan-repository.ts` semantics (stable discovery block, initial-import
continuation, bounded upload sampling, live-state definition, head-check cost).

### Lock order (fixed everywhere)

```
rival_channels (FOR UPDATE / INSERT)
  → scan_state      (ensure + row lock)
    → scan_jobs
      → rival_videos
```

`lockChannelStateTx` is the single entry for a channel-scoped write: it locks
the parent row, then creates-or-locks the state row (`ScanEnsureState`'s
`ON CONFLICT` branch takes the row lock). The fence is checked **after** the
state row is locked (`leaseHeld`), so there is no read-then-write window, and
`DeleteChannel` (parent first, then cascade) can never deadlock against a page
commit. Real database errors are returned as-is; only `pgx.ErrNoRows` on the
parent/state read maps to `channel_not_found`.

**Deferrable constraint (justification).** `rival_videos` now declares

```sql
CONSTRAINT rival_videos_channel_position_key UNIQUE (channel_id, position)
    DEFERRABLE INITIALLY DEFERRED
```

because the one operation that permutes ordering keys (the safe-bound rebase,
which renumbers the whole channel in a single `UPDATE`) is guaranteed to
collide with itself under an immediate unique index. Deferring the check to
commit keeps the invariant exactly as strong (a transaction cannot commit a
duplicate) while making the rebase a single pass instead of two offset passes
that would still need a free key range. Nothing uses `ON CONFLICT
(channel_id, position)`, so the arbiter is unaffected; the insert/upsert paths
still conflict on the primary key only.

## Fix round 1 — finding → change → test

Each row states **red before** (what the current/previous code did, i.e. what
the test would have caught) and **green after** (how the test now passes).
Tests live in `scan_pg_test.go`, names carry no plan/finding IDs.

### C2 — fence TOCTOU inside claim/transitions

- **Change.** `ClaimJob`, every `SetJob*`/`CompleteJob`/`RestartJobCursor`
  (`fencedJob`), `CommitPage`, `CompleteHeadCheck` and `RecordUploadTime` all
  go through `lockChannelStateTx` (parent → state row lock) and only then run
  `leaseHeld` (token identity + `lease_until > now`). `assertLeaseTx` (the
  unlocked `SELECT` that produced the window) is gone, together with
  `stateExistsTx`/`ScanGetState`.
- **Test.**
  - `TestScanFenceMatrixRejectsWrongAndExpiredTokens` — subtest per transition
    (backoff, error, capped, paused, complete, restart, claim, page, head,
    upload) × (wrong token, expired fence): every call must be `lease_lost`,
    the job must stay `running` with `attempts=0`, a rejected page must leave
    no row, and a rejected upload write must leave `uploaded_at` NULL.
    *Red before:* expired-fence transitions still wrote (the unlocked read saw
    the old token). *Green:* all 20 subtests pass.
  - `TestScanFenceHeldDuringTransitionBlocksTakeover` — deterministic
    two-connection interleaving, one subtest per write path (`error`, `backoff`,
    `capped`, `paused`, `complete`, `restart`, `claim`, `page`), using the
    `gatedDB`/`gatedTx` harness that suspends the store's transaction on the
    chosen SQL statement. In each subtest A is suspended exactly between its
    fence check and its write; B's `AcquireChannelLease` must **block** (returns
    a context/connection error, not a business code) and a raw
    `UPDATE scan_state` must also block; after A resumes it writes, and only
    after the fence expires can B acquire — then A's stale token gets
    `lease_lost`.
    *Red before:* B's acquire succeeded while A sat between read and write, and
    A then overwrote B's job (the review's `TestReviewFenceTakeoverDuringTransition`).
    *Green:* for every write path B cannot take over while A holds the row lock.
  - `TestScanTwoRealLeasesOnlyOnePageLands` — two leases both issued by the
    store (one expired, then a real re-acquisition), claims raced via a start
    barrier; the expired lease loses the claim and its page write is rejected,
    only the live lease's page lands.
    *Replaces* the previous phantom test, which raced a valid token against a
    fabricated one and never wrote a page.

### M13 — delete vs page commit lock order

- **Change.** `DeleteChannel` locks the parent row first (`ScanLockChannel`),
  then deletes the `background_tasks` rows, then the channel (cascade).
  `CommitPage` locks the parent before state/jobs/videos — the same order —
  so Postgres has no cycle to detect, and a page whose channel is already gone
  reads no parent row and returns `channel_not_found`.
- **Test.**
  - `TestScanDeleteDuringPageCommitSerializes` — A (gated) is suspended after
    parent+state locks, B's `DeleteChannel` blocks, then both complete in
    order: A's page is written, then the delete cascades. No `40P01`.
  - `TestScanPageAfterRemovedChannelReportsStopped` — B (gated) holds the
    parent lock before `DELETE FROM rival_channels`; A's page blocks, then
    after B commits A returns `channel_not_found` and writes nothing.
    *Red before:* the opposite order (state first, parent FK later) gave the
    review a reproducible `40P01 deadlock detected` inside the page.

### M1 — discovery block reserved once, no per-page rebase

- **Change.** `StartDiscovery` → `reserveDiscoveryBlockTx`: reserve
  `base = min(position) - 10_000` **once**, persist it on the job
  (`scan_jobs.order_base`); rebase the channel order only when
  `min - 10_000 < -KeyLimit` (source semantics), then base starts at `-10_000`.
  `CommitPage` uses `job.order_base + job.next_ordinal + page index`, never
  rebases mid-run, and enforces the block bound
  (`ordinal0 + len(page) > DiscoveryBlock` → `key_range`) plus
  `|key| <= KeyLimit`. Anchors are the persisted head anchors ∪ newest
  non-history rows, capped at `AnchorMax`. The job-less `Checkpoint` path
  (types, SQL, branch) is removed — every discovery run has a job.
- **Test.**
  - `TestScanDiscoveryPagesKeepOrderAndPositions` — page 1 (new + overlapping
    anchor) then page 2 in the same run: positions `base, base+2, base+3`, the
    known row keeps its old position/origin, `order_base` and `next_ordinal`
    advance by page length. *Red before:* page 2 hit
    `23505 rival_videos_channel_id_position_key` and reversed page order.
  - `TestScanDiscoveryOrdinalBound` — at `next_ordinal = 9995` a 5-row page
    lands, the next row is `key_range` and writes nothing.
  - `TestScanDiscoveryRebaseOnlyWhenNearSafeBound` — seeding positions at
    `-KeyLimit + 5` makes `StartDiscovery` rebase once: order becomes `0..n-1`
    and the new base is `-10_000`; the deferrable constraint lets the single
    renumbering `UPDATE` succeed.

### M3 — baseline saves its continuation; history takes it over

- **Change.** `CommitPage` serializes the page continuation once and writes it
  to `scan_state.history_cursor/history_remainder` **iff** the page is the
  initial baseline with no job (`saveBookmark := baseline && job == nil`) — the
  inverse of the old condition. `StartBackfill` inserts the history job with
  that cursor/remainder and clears the bookmark in the same transaction. A
  discovery page never touches the bookmark.
- **Test.** `TestScanBaselineStoresProfileOrderAndContinuation` asserts the
  stored endpoint of `SerializeChannelCursor`/`SerializeScannedVideos` after a
  baseline page, then `TestScanHistoryStartTakesOverBaselineContinuation`
  asserts the job carries them, the state bookmark is cleared, and a
  discovery page leaves `history_cursor` NULL.
  *Red before:* a successful Add discarded its cursor and history restarted
  from the head.

### M4 — upload candidates bounded to the newest 30

- **Change.** `ScanNextUploadCandidate` selects the newest 30 rows per channel
  in the lateral subquery *first* and filters `upload_checked_at IS NULL`
  in the join (`ON r.upload_checked_at IS NULL`), so the sample is positional
  and never drains into history.
- **Test.** `TestScanUploadCandidateIsBoundedToNewestSample` — 41 rows, newest
  30 all checked → candidate is nil; unchecking position 7 returns exactly that
  row. *Red before:* the filter ran before `ORDER BY … LIMIT 30`, so position
  30 was returned and a 10k channel produced 10k probes.

### M8 — stopped discovery is not "live"

- **Change.** One definition of *live* everywhere (query, due selection,
  control): `queued, running, backoff, paused, capped, error` — identical to
  the unique-live index. `cancelled` is no longer "live", so
  `ScanLiveJob`/`ScanLatestJob` differ and `DueChannels` no longer sees a
  cancelled run as scheduled work. `StartDiscovery(channel, replaceStopped)`:
  a cancelled (or absent) run → new run with a freshly reserved block;
  paused/error/capped → requeued only when the caller is an explicit user
  action; queued/running/backoff → `false` (idempotent).
- **Test.** `TestScanStartDiscoveryReplacesStoppedRun` — after a cancel the
  channel is due again, `StartDiscovery(false)` creates a *new* runnable job;
  a paused run is excluded from `DueChannels` and left untouched by the
  schedule while `StartDiscovery(true)` requeues it.
  *Red before:* `StartDiscovery` returned `false` forever after a cancel and
  nothing could ever run again.

### M12 — head check writes only what changed

- **Change.** `CompleteHeadCheck` reads the locked channel's previous median
  and recomputes whole-channel scores **only** when the median actually changed
  (null-safe `sameMedian`). `ScanRecomputeScores` adds
  `AND score IS DISTINCT FROM <new formula>`, so even a real median change
  writes only rows whose score differs. The channel row itself
  (`median_views`, `median_at`, `scanned_at`) is always touched — that is the
  freshness stamp, one row.
- **Test.** `TestScanHeadCheckWritesOnlyChangedScores` — xmin snapshots of
  every video row: unchanged median ⇒ **0** rewritten score rows; changed
  median ⇒ rewritten rows == the number of rows whose score differs from the
  new formula (computed before the call), strictly fewer than all rows because
  null-view rows keep `score = NULL`.
  `TestScanHeadCheckCost10000` measures a 10,000-row channel (below).
  *Red before:* every head check rewrote all rows unconditionally.

### m1 — store side of the history/duplicate-Add fixes

- **Change.** `StartBackfill` returns `(nil, nil)` when `scan_state.history_end`
  (ended tab needs no job) instead of inserting a queued job. `CommitPage` of a
  profile-bearing baseline uses a strict `INSERT … ON CONFLICT (id) DO NOTHING`
  (`ScanInsertChannel`): 0 rows means another Add already created the channel,
  which now returns the new `StoreChannelExists` code so the service can map it
  to `exists` after re-reading the winner. The old
  `ScanUpsertChannel`/`ScanPlantChannelLease` pair is deleted.
- **Test.** `TestScanHistoryEndStartIsNoop` (no job row created) and
  `TestScanDuplicateBaselineRaceYieldsExists` (two concurrent baseline commits
  through a start barrier: exactly one `nil` and one `channel_exists`, and the
  channel holds only the winner's rows/counter).
  *Red before:* an ended tab produced a queued job; the racing loser surfaced
  as `lease_lost` (mapped to `lease_busy`).

### m3 — unused alternate paths removed

- **Change.** Deleted: `Checkpoint` branch + `ScanInsertCheckpointJob` +
  `CommitPageInput.Checkpoint/OrderBase/Ordinal/Anchors`, `JobByTask` +
  `ScanJobByTask`, `CancelDiscovery`/`ResumeDiscovery` (resume is
  `ControlBackfill(resume)` for history plus `StartDiscovery(true)` for
  discovery), `ScanGetState`, `ScanPlantChannelLease`, `ScanUpsertChannel`,
  `ScanEnsureSchedule`, `ScanScheduleRow`, and the per-deadline selectors
  (`NextRescanAt`, `NextBackoffAt`, `UploadsPausedUntil`) folded into
  `NextDeadline`.
- **Test.** `go vet ./internal/persistence/postgres/` and the store-wide
  `var _ scan.Store = (*ScanStore)(nil)` assertion; the new SQL set is
  regenerated deterministically (`sqlc generate` twice → identical sha256).

### Interface additions for the worker/CLI (from ScanCore)

Implemented exactly as declared: `RecordUploadTime(lease, …)` fenced,
`NextRunnableJobForChannel` (filtered in SQL — M9),
`NextDeadline(rescanHours, now)` (single sleep target: future-only job
backoff, rescan of channels without a live discovery run, upload-pause end),
`ClearTaskLink` (M10), `AcquireWorkerLease` returns the **stored owner**
(C1; previously it returned the platform name, so the first renewal lost the
lease), and `Status` fills `WorkerLive` internally (`ScanWorkerLeaseLive` is no
longer part of the interface).

### Smaller parity points restored in the same pass

- The state's `head_newest_id` / `head_checked_at` are written only by a
  job-less page (the initial baseline) or by `CompleteHeadCheck`; a discovery
  *page* carries its watermark on `scan_jobs.newest_id`, so an unfinished run
  cannot shift the channel's rescan window (source writes the state watermark
  only for `!jobId` pages).
- `CommitPageResult.NextOrdinal` is the run's next ordinal: `ordinal0 +
  len(page)` for discovery, `ordinal0` otherwise (matches the source's
  `nextOrdinal` expression; the worker's `>= DiscoveryBlock` page-boundary
  check reads it).
- The 10,000-key bound is evaluated on the deduplicated page (the ordinals the
  run will actually consume), not on the raw provider page.

### RivalsAPI index

`00006_rivals.sql` gains, after `scan_jobs_runnable_idx`:

```sql
CREATE INDEX IF NOT EXISTS scan_jobs_latest_idx
    ON scan_jobs (channel_id, kind,
                  (state NOT IN ('complete', 'cancelled')) DESC, updated_at DESC);
```

RivalsAPI reported the sidebar (50 rivals × 100k videos × 50k retained jobs)
at p50 21.9 ms / p95 34.3 ms with this index (462.9/558.0 ms in the review's
fixture). The unique-live index and `scan_jobs_runnable_idx` are untouched.

## Ported source cases (parity table in the review)

| Source case | Go coverage |
|---|---|
| Exclusive leases until expiry + token fencing, L151 | `TestScanFenceMatrixRejectsWrongAndExpiredTokens`, `TestScanTwoRealLeasesOnlyOnePageLands`, `TestScanFenceHeldDuringTransitionBlocksTakeover`, `TestScanWorkerLeaseReturnsStoredOwner` |
| Unknown/idle/dead-worker status, L224 | rivals read tests (RivalsAPI) + `Status` snapshot; store keeps the live/latest split |
| Initialize state from existing rows, L250 | `TestScanStateInitializesFromExistingRows` (count, ordered anchors, newest id, defaults) |
| Start/cap/control continuation, L263 | `TestScanHistoryCapCapsAndResumeMapsRetry` |
| Finished-tab start + no-job controls no-op, L295 | `TestScanHistoryEndStartIsNoop` |
| Initial import atomic profile/order/bookmark/freshness, L310 | `TestScanBaselineStoresProfileOrderAndContinuation` (stored values, not counts) |
| History after import appends/drains saved continuation, L335 | `TestScanHistoryStartTakesOverBaselineContinuation` |
| Replay immutability/null preservation/metadata refresh, L356 | `TestScanPageKeepsIdentityAndKnownValues` (title/view/position/origin/first_seen/updated_at asserted) |
| Foreign-owned ID never moved, L387 | `TestScanForeignChannelIDNeverMoves` (title/views/position/origin of the foreign row) |
| Wrong/expired/taken-over fence, L402 | `TestScanFenceMatrixRejectsWrongAndExpiredTokens` (all transitions) + takeover test |
| Stale profile cannot resurrect a deleted rival, L437 | `TestScanPageAfterRemovedChannelReportsStopped` (a page for a deleted channel is `channel_not_found`, writes nothing) |
| Rejected page rolls back newly created profile, L422 | `TestScanRejectedBaselineRollsBackProfile` (invalid continuation ⇒ no channel/state/video rows, and the channel can still be added afterwards) |
| Stopped job rejects whole page atomically, L461 | `TestScanStoppedJobRejectsWholePage` |
| Stable discovery block / anchors / pre-upsert overlap, L488 | `TestScanDiscoveryPagesKeepOrderAndPositions`, `TestScanDiscoveryOverlapCompletesHeadCheck` |
| 10,000 overflow / safe-bound rebase, L524 | `TestScanDiscoveryOrdinalBound`, `TestScanDiscoveryRebaseOnlyWhenNearSafeBound` |
| Unfinished discovery blocks second run / freshness, L590 | `TestScanStartDiscoveryReplacesStoppedRun` |
| Median newest 150 then known views, L610 | `TestScanHeadCheckKeepsRunningRequirementAndExactMedian` (exact 50 with a null-view row inside the sample, exact score) |
| Null median / expired fence / rollback, L631 | `TestScanFenceMatrixRejectsWrongAndExpiredTokens` (`head`, `upload` expiry) + null-view score assertion |
| Finishing head job must still be running, L655 | `TestScanHeadCheckKeepsRunningRequirementAndExactMedian` (paused job → `job_stopped`, `scanned_at` unchanged) |
| Runnable priority / stopped jobs / skip exhaustion, L669 | `TestScanRunnableSelectionCoversStoppedAndTargetChannel` |
| Claim requires fence + runnable job, L698 | `TestScanClaimRequiresRunnableJob` + fence matrix |
| Failure writes fenced/running/sanitized, L715 | `TestScanBackoffWritesAreFencedAndSanitized` + fence matrix |
| Cursor restart / replay target / progress retention, L733 | `TestScanCursorRestartClearsContinuation` |
| Delete cascades state/jobs/videos, L758 | `TestScanDeleteCascadesVideosStateJobsAndTasks` (video/state/job/task rows all gone, second delete not-found) |
| Legacy SQLite migration, L99 | Not applicable (new Postgres schema) |
| Request slot / jitter / 429 penalty (durable), L177/L205 | Not store scope: platform pause lives in the platforms store (ScanCore) |

Additional store-level cases: `TestScanPageAndRemainderLimits`,
`TestScanNextDeadlineIgnoresPastAndLiveDiscovery`,
`TestScanUploadTimeWriteIsFencedToChannel`,
`TestScanCrashRecoveryContinuesFromCursorWithoutDuplicates`,
`TestScanCommitLatency`.

## Cost measurement — 10,000-row head check

`TestScanHeadCheckCost10000` seeds one 10,000-row channel (3 baseline rows +
10,000 rows) on local Postgres and logs these numbers on every run:

| Measurement | Before (review) | After |
|---|---|---|
| Head check, median unchanged | **2.926–3.785 s** (unconditional whole-channel UPDATE) | **10.5 ms**, **0** video rows rewritten |
| Whole-channel score UPDATE, same shape | 2,329.502 ms (10,000 rows in, 251,280 shared buffer hits, 421 buffers dirtied) | not executed when the median is unchanged |
| Head check, median actually changed | ~2.3 s of score UPDATE | **607 ms** total (median sample + conditional score UPDATE) |
| Unconditional UPDATE measured in the same test run | — | 317.3 ms, 10,003 rows rewritten (contrast: the fixed path writes 0) |

A regression shows up as a changed log line plus a rewritten-row count > 0.
`TestScanCommitLatency` (20-video page, 40 samples): p50 4.12 ms, p95 8.97 ms,
max 14.95 ms. The review measured the same channel shape at 2.926–3.785 s per
head check (its `EXPLAIN (ANALYZE, BUFFERS)` of the unconditional update:
2,329.502 ms, 251,280 shared buffer hits, 421 buffers dirtied).

## Verification

All heavy commands ran under `lockf -k /tmp/verse-heavy.lock nice -n 10`,
`GOTOOLCHAIN=go1.26.0`, `GOMAXPROCS=4`; Postgres
`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`.

| Check | Result |
|---|---|
| `go test -p 2 -count=1 ./apps/viderse/internal/persistence/postgres ./archtest` | PASS (56.4 s + 1.0 s, whole package incl. rivals/activity/platforms) |
| `go test -p 2 -count=5 -run 'TestScan' …/postgres` | PASS, twice, no flakes (58.6 s / 87.6 s across runs) |
| `go test -race -p 1 -count=1 -run 'TestScan' …/postgres` | PASS (30.2 s) |
| `go test ./archtest` (`TestLayeredAppImports`) | PASS |
| `go build ./...`, `go vet ./apps/viderse/... ./platform/webapp/...` | clean |
| `sqlc generate` twice | identical `gen/scan.sql.go` sha256 (`0fc66159…`) |
| `gofmt -l` with go1.26.0 and go1.27.0 on `scan.go`, `scan_pg_test.go` | clean |

Not covered here (other owners): worker/heartbeat lifecycle, task
operations, platform pause, CLI inline loop. The store side of those seams is
implemented and tested (fenced upload write, owner-carrying worker lease,
target-channel runnable selection, single next-deadline, task unlink).

## Fix round 2

### R2-02 — fence expiry is judged from a clock read **after** the locks

**Change.** `lockChannelStateTx` (parent → state) and `lockStateTx` (state, for
the baseline-insert path) now return the time they read *after* both row locks
are held; every fenced path uses that value and nothing else as the authority
for fence validity: `ClaimJob`, `fencedJob` (all `SetJob*`/`CompleteJob`/
`RestartJobCursor`), `CommitPage`, `CompleteHeadCheck`, `RecordUploadTime`.
`in.Now` / the caller's `now` argument remain metadata timestamps
(`first_seen_at`, `updated_at`, `median_at`, `checked_at`) and are never
consulted for expiry.

**Test — `TestScanLeaseExpiryDuringLockWait`** — 5 paths (transition, page,
head, upload, claim) × 2 waits (parent row, state row) = 10 subtests. A second
connection takes the row lock; the operation is started and blocks on it; the
test then sleeps past a 250 ms lease and only afterwards releases the lock.
Every subtest must get `lease_lost` and the side effect is checked with stored
values (job still `running`/`queued`, no page row, median still NULL, no
`uploaded_at`).
*Red before:* with the clock captured at call entry (the old code, and
`in.Now` for `CommitPage`) `transition_state` returned **nil error** and wrote
the transition after the lease had expired — reproduced by temporarily
restoring the entry-time clock; the same patch also flips the other subtests.
*Green:* all 10 subtests pass with the post-lock clock.

### R2-05 — a stopped discovery run is retired, not resumed

**Change.** `StartDiscovery(channel, replaceStopped=true)` no longer
`ScanRequeueJob`s a paused/error/capped discovery run (which kept its cursor,
anchors, reserved base, newest id and `next_ordinal`). It now cancels the
stopped row under the same parent/state locks and falls through to reserve a
**new** block and insert a fresh `queued` run: `next_ordinal = 0`, no
cursor/remainder, anchors recomputed from the current head. History
continuation is untouched (it is a different kind and a different method).

**Test — `TestScanStoppedDiscoveryRetryStartsFreshRun`** — drives a run into
`error / gap_incomplete` with `next_ordinal = 10000`, then performs the
explicit replacement: the old row must be `cancelled`, the new runnable job is
a different id with ordinal 0, empty continuation, empty progress, a freshly
reserved block at `min-10 000` and current anchors, and its first page commits
at ordinal 0 (so the exhausted block is genuinely gone).
*Red before:* with the old requeue the test fails at
`lượt cũ phải bị bỏ hẳn: queued` — the same row came back at ordinal 10 000.
*Green:* new run, ordinal 0, first page lands.

### R2-06 — task rows use the real stored shape, and creation is atomic

**Change (shape).** `ScanDeleteChannelTasks` filtered
`object_ref->>'id'`; the real rows are written from `domain.ObjectRef` with
untagged Go fields, i.e. `"ID"`. The query now filters `object_ref->>'ID'`.

**Change (atomicity).** New `SyncScanTask(ctx, jobID, channelID, spec, state)` is
now the only way a scan task row is created (ScanCore replaced its
`Tasks.Start` + `LinkTask` + compensating `Finish` with one call to it). It does
the whole task write in one transaction in the fixed lock order: lock
`rival_channels` → `scan_state`, require the job to exist, belong to the
channel and not be terminal, then create-or-adopt the `background_tasks` row
and `ScanLinkTask` it; a link that affects **0 rows** is `job_stopped` and
rolls back the task insert. A `DeleteChannel` therefore either wins before the
lock (→ `channel_not_found`, no row) or runs after the commit (→ its
`object_ref->>'ID'` sweep removes the row): no interleaving and no crash window
can leave an orphan task. The standalone `LinkTask` method was removed with
ScanCore's interface change — nothing calls it any more.

**Tests.**
- `TestScanRemoveClearsRealActivityRows` — creates the row through the real
  `activity.Service` on the real store, asserts it exists by the canonical
  `ID` key, then `DeleteChannel` must leave `0` rows.
  *Red before:* `1` row survived (the lower-case filter missed it).
- `TestScanTaskCreateLinkIsAtomic` — two deterministic orders with the gated
  store: (a) `DeleteChannel` holds the parent lock while `SyncScanTask` waits →
  `channel_not_found`, no task id, `0` task rows, and a retry after the delete
  still creates nothing; (b) task+link first, then `DeleteChannel` → the row is
  gone and the job's link is cleared.
  *Red before* (task inserted outside the transaction, as the old
  `Start`+`LinkTask` pair did): `bỏ lại 1 dòng Việc nền mồ côi`.

### R2-07 — adoption keeps the displayed progress

**Change.** New `AdoptTask` query/store method (member of `activity.Store`)
rewrites content, phase, state, action and owner but writes progress only when
the spec carries one (`COALESCE(spec_progress, progress)`). `activity.Service.Start`
now routes by the existing row's state: `failed` → `ResetTask` (retry clears
progress, unchanged), anything else → `AdoptTask`. `ScanStore.SyncScanTask`
uses the same adopt semantics, so a scan task adopted on recovery or by a
re-link never loses its progress, and any real progress write still goes
through the caller's ≤4/s throttle as a normal `Update`.

**Tests.**
- `TestStartAdoptionKeepsProgressUntilFailed` (activity, mem store): adopt a
  paused task with `3/150` → same id, progress intact, content rewritten;
  then `failed` + start → progress cleared (retry semantics preserved).
  *Red before:* `nhận lại xoá mất tiến độ: <nil>`.
- `TestScanTaskAdoptionKeepsProgress` (real Postgres rows): create through
  `SyncScanTask`, write real progress `3/150` via `activity.Service.Update`,
  adopt again (`waiting`) → same row id, progress kept, state updated; then
  three consecutive page progress writes and another adoption → `6/150`
  survives and only one task row exists in total.

### Store query for R2-03 — one scheduling read

`NextDeadline` is replaced by `NextWakeup(ctx, rescanHours, now) (scan.Wakeup, error)`
(single statement, `ScanWakeup`): `Actionable` is true when there is a
`queued`/`running` job (including crash-recovered ones), a `backoff` job whose
`next_attempt_at` has passed, a channel whose rescan deadline has passed and
which has no live discovery run, or an unread video inside the newest-30 upload
sample while the upload pause has elapsed. `Next` is the minimum *future*
instant across job backoff, rescan (same live-discovery exclusion), the upload
pause end (only while a candidate exists) and `scan_state.lease_until` of
channels that hold a live job or an upload candidate — so an idle heartbeat
wakes exactly when a fence expires instead of claim-spinning. Past deadlines
are never returned as a sleep target.

**Test — `TestScanWakeupSeparatesActionableFromFuture`** — walks: nothing to do
→ a job queued by "another process" is actionable → future backoff is only a
sleep target and a past backoff is actionable → a running job with a lease
reports the lease expiry as `Next` → a due rescan is actionable, a future
rescan is only `Next` → a paused discovery run suppresses the rescan deadline
entirely (no spin) → the upload pause is a sleep target until it elapses and
then becomes actionable. *Red before:* the old `NextDeadline` returned `nil`
for queued/recovered work, so an idle loop could never learn about it.

### Retention

**Finding.** Finished `scan_jobs` rows grew without bound: the rivals review
measured 32 replacements leaving 32 `cancelled` rows, and every hourly rescan
completes one `discovery` row (50 rivals × 24/day ≈ 438k rows/year). Only the
newest finished row is ever read (`ListRivals` takes the latest row's `state`
per `(channel_id, kind)`), and `scan_state.head_checked_at` already holds the
last check time, so older rows carry no information.

**Change.** New `ScanPruneFinishedJobs` deletes a channel/kind's `complete` and
`cancelled` rows, keeping the single newest one (`ORDER BY updated_at DESC,
id DESC`). It runs inside the transaction that already holds the channel's
parent and state locks, in three places:

- insertion of a new job — both `StartDiscovery` and `StartBackfill`;
- `CompleteJob` (a history run finishing);
- the `discovery` finish inside `CompleteHeadCheck`.

Pruning at finish is what makes the invariant hold *between* inserts: otherwise
the row kept by the previous insert plus the row that just finished would leave
two, and only the next insert would collapse them. Both paths keep the row that
just became the newest finished one, so nothing a live task needs and nothing
the rivals read uses is touched: `scan_jobs.task_id` is only consulted for live
jobs (`Finish`/`Update` on the job the worker is holding), no table references
`scan_jobs`, and the retained row is exactly the one the lateral read picks.
Delete of a child row happens after the parent is locked, so the lock order
`rival_channels` → `scan_state` → `scan_jobs` → `rival_videos` is unchanged.

**Test — `TestScanFinishedJobsArePruned`.** One channel goes through 5 failed
discovery runs that are replaced (each leaving a `cancelled` row), 5 hourly
head checks (each completing a `discovery` row) and 5 finished history runs,
all through the real store API; a second channel is seeded with 4 finished rows
and must be untouched. Asserts at most one finished row per kind, at most two
job rows in total for the channel (one live, one finished), that the surviving
finished row is the newest one, and that the other channel still has all 4.
*Red before:* with the three `pruneFinishedTx` calls removed the test fails at
`discovery để lại 10 dòng đã dừng, muốn tối đa 1` (5 cancelled + 5 complete).
*Green:* 1 finished discovery row, 1 finished history row, other channel
untouched.

## Fix round 3

### R3-M1 / R3-M2 — the idle scheduling read now models claimable work

**Change.** `ScanWakeup` was rewritten around one notion: a per-channel set of
*due* work (`channel_work`: a `queued` job or a due `backoff`, a due rescan with
no live discovery run, an unchecked row in the newest-30 upload sample while the
upload pause has elapsed) and then two questions.

- `Actionable` = a `running` job exists (the live worker's own work), **or**
  some due channel has no live `scan_state.lease_until`. Ready upload work is
  therefore explicit and no longer rides on a `scan_schedule` row: with the row
  missing the pause reads as `epoch`, so a newly added rival with unchecked head
  videos is claimable immediately. Work whose channel is fenced by another
  owner is never actionable; its fence expiry goes into `Next` instead.
  An elapsed pause on its own still implies nothing — the branch also requires
  an unchecked sample.
- `Next` = min future instant of job `next_attempt_at`, rescan deadline (live
  discovery excluded), upload-pause end (only while an unchecked sample exists)
  and the fence expiry of every channel that has a live job or due work. Past
  instants are dropped, so a sleep target is always in the future.

`ScanNextUploadCandidate` also skips channels with `lease_until > now`, so a
fenced first candidate cannot mask an eligible channel — the probe reaches
channel B immediately instead of retrying A until its fence ends.

**Tests.** `TestScanWakeupSeesUnpausedUploadWork` (no `scan_schedule` row at
all, unchecked sample, no fence → `Actionable=true`, and the candidate query
returns the row) and `TestScanWakeupKeepsFencedWorkOutOfActionable` (expired
pause + another owner's fence → `Actionable=false` with `Next` at the fence,
candidate selection returns nothing; adding an unfenced channel with an
unchecked sample → `Actionable=true`).
*Red before:* removing the upload branch reproduces R3-M1 exactly —
`Actionable=false Next:+1h`; dropping the fence condition from `ready`
reproduces R3-M2 — `Actionable=true` while the only work is fenced.
The round-2 scheduling test was updated where the new contract differs: a
`queued` job on a channel whose lease the test still held is no longer
actionable, so the test releases the fence first (and the sample test clears the
fence before expecting a candidate).

### R3-M3 — task presentation follows the locked job

**Change.** `SyncScanTask` no longer trusts the caller's state: it maps the
presentation from the job it has just locked (`running`→running,
`queued`/`backoff`→waiting, `paused`→paused, `error`→failed) and rejects
`capped`/`complete`/`cancelled` with `job_stopped` **before** creating any task
row. The caller's `state` argument can only *downgrade* a running job
(waiting/paused/failed, e.g. a failed page); it can never turn a paused or
backoff job into a running task, and a capped job can never get a row at all.

**Test — `TestScanTaskSyncFollowsLockedJobState`.** Pause commits first, then a
stale running snapshot syncs → the row is `paused`, not running. A history job
driven to 150 indexed videos and capped (stale running snapshot) → `job_stopped`
and zero task rows for that channel.
*Red before:* with the old mapping (caller state, only complete/cancelled
rejected) the pause case stores `running`.

### R3-M4 — adoption writes no progress of its own

**Change.** The store contract is now exact: on adoption, `spec.Progress == nil`
means the progress column is not assigned at all; a non-nil spec writes it once.
A new task initialises progress once, in the insert. `AdoptTask`'s
`COALESCE($narg, progress)` already kept the value; what R3-M4 needed from the
store is that nothing synthesises a payload, so the service's limiter is the
only thing that can create a progress write.

**Test — `TestScanTaskAdoptionWritesNoProgressByItself`.** A test-only trigger
on `background_tasks` counts updates where `progress IS DISTINCT FROM OLD`,
created through a maintenance connection (the app role has no DDL rights) with a
`SECURITY DEFINER` counting function. Ten adoptions at one instant with a nil
progress spec → **0** progress-changing updates and the stored `3/150` intact;
one adoption carrying `7/150` → exactly **1**.
*Red before:* simulating the pre-fix call pattern (adoption always sending the
current progress) makes the same test report a progress write instead of zero.
Service-side budgeting (one limiter, post-sync update carrying phase/action
only) is ScanCore's half; the store half is the 0/1 accounting above.

### R3-m2 — schema names cannot collide

**Change.** `platform/pgtest.SchemaName` appends 12 random hex characters
(`crypto/rand`, clock fallback) instead of `UnixNano()%1_000_000`, and truncates
the test name to 36 characters. Schema identifiers stay ≤ 56 bytes, so the
`pgtest_app_<name>` role that the noverse migrate test derives is ≤ 60 without
cutting the random suffix. Cleanup (drop schema cascade) is unchanged.

**Test.** `platform/pgtest`: 2 000 names in one process are unique and ≤ 63
bytes. Consumers are unchanged (`Provision`/`ProvisionRoles`/`SchemaOf` keep
their signatures); the viderse `postgres` package — the main consumer — passes
the round-3 acceptance run below.

## Fix round 4

### R4-M1 — retained `running` rows are fenced like every other kind of work

**Change.** `ScanWakeup` no longer has an unconditional "a `running` job exists →
actionable" branch. `running` joins `queued` and due `backoff` inside
`channel_work`, so a retained running row is claimable only when its channel has
no live `scan_state.lease_until`; a valid fence (of any owner — an idle worker
holds no channel fence) contributes only its expiry to `Next`. The fence
expiry list still covers every channel with a live job, so the worker sleeps
exactly until the residual fence dies instead of re-selecting the same row.

**Test — `TestScanWakeupGatesRecoveredRunningJobOnChannelFence`.** A real
history job is claimed (durable `running` row), upload samples are marked read
and the head check time is fresh so nothing else can make the read actionable.
With a live residual fence: `Actionable=false`, `Next` = the fence expiry.
After the fence lapses: `Actionable=true`.
*Red before:* restoring the unconditional running branch makes the same test
report `Actionable=true Next:<future>` — precisely the reviewed hot-loop input.
The round-2 scheduling test was updated for the new rule (running + valid fence
is no longer actionable; clearing the fence makes it actionable).

### R4-M2 — the task row is written from the locked job's presentation

**Change.** `SyncScanTask` now takes no caller-supplied state (`SyncScanTask(ctx,
jobID, channelID, spec)`, state/phase/action removed from the contract), locks
the job, calls `scan.TaskPresentation(lockedJob)` and writes `view.State`,
`view.Phase` and `view.Action` — the caller's `spec.Phase`/`spec.Action` are
overwritten before the row is built, and `live=false` (complete, cancelled,
capped) returns `job_stopped` before any insert or adopt. The old
`taskStateForJob` mapping and the "caller may downgrade running" escape are
gone: there is one authority, the job read under the parent/state locks.
Optional progress keeps the round-3 budget rule (nil ⇒ the column is not
assigned, non-nil ⇒ one write).

**Test — `TestScanTaskSyncUsesLockedJobPresentation`.** A running history job is
paused; a page's stale spec (running phase, cancel button, progress 5/150) is
then synced. The stored row must be `paused`, with the **phase** and **action**
of `scan.TaskPresentation(pausedJob)` and `action.Op == resume` — the button
offers recovery, not cancellation — while progress 5/150 is still written.
`TestScanTaskSyncFollowsLockedJobState` (round 3) keeps the paused/capped
assertions under the new signature.
*Red before:* with the previous mapping only the state enum followed the locked
job; phase and action were copied from the stale spec, so the row said
`paused` + "Đang đọc tab Videos" + cancel — reproduced exactly:
`câu mô tả phải theo việc đang khoá: "Đang đọc tab Videos" (muốn "Đã tạm dừng")`.

## Fix round 5

### R5-M2 — the busy-channel set is applied before `LIMIT`

**Change.** `ScanRunnableJobs` takes the caller's busy-channel set as a SQL
parameter (`channel_id <> ALL(@skip::text[])`) applied in the same `WHERE` as the
runnable predicate, before `ORDER … LIMIT 50`. `NextRunnableJob` passes the set
(sorted for a stable statement) and returns the first row; the Go-side
post-filter is gone, so 50 busy rows can no longer hide an eligible 51st job.

**Test — `TestScanRunnableSkipAppliesBeforeLimit`.** 51 channels with a queued
history job each, `updated_at` pinned to a one-second ladder so the order is
deterministic; the first 50 are in the skip set and the 51st is free. Without a
skip set the selector returns the oldest job; with the 50-channel skip it must
return the 51st; with all 51 skipped it returns nil. *Red before:* with the
old "fetch 50 then filter in Go" shape the same test fails with
`phải chọn được kênh rảnh thứ 51 sau 50 kênh bận: <nil>`. The existing
single-fenced-candidate guard test still passes.

### R5-M3 — the failure transition records its notice atomically

**Change.** `SyncScanTask` now returns `scan.TaskSync` and, inside the same
transaction as the task write:

1. reads the existing task row (by kind + object) and remembers its state;
2. writes the presentation from the locked job, as before;
3. when the locked view is `failed` **and** the previous row was absent or not
   `failed`, records the notice — a notifications upsert with the §11 ten-second
   merge rule under the per-kind advisory xact lock, plus the `error_events` row
   carrying `spec.Object` — and returns its id as `NotificationID`.

The bodies of `ActivityStore.UpsertNotification` and `InsertErrorEvent` were
extracted into `upsertNotificationTx` / `insertErrorEventTx` (tx-scoped,
package-private); `ActivityStore` keeps its exact behaviour, including the
post-commit re-read of the merged notification. A new `withTxConn` gives the
store the transaction handle so the advisory lock and both writes share the
task/link transaction. Refreshes of an already-failed row, a stale failed
snapshot after a resume, and `job_stopped` record nothing.

**Test — `TestScanFailureNoticeRecordsOncePerRealFailure`.** A running history
job is marked `error` and synced: one notification row with `count=1`, one
`error_events` row whose `object_ref` is the channel, `NotificationID`
non-empty, view `failed`. Two further syncs of the still-failed row:
`NotificationID` empty and the counts stay 1/1. Then the job is resumed
(`ControlBackfill resume`) and synced: not failed, still 1/1. A second genuine
failure (claim → `SetJobError` → sync) gives `count=2` and two error events.
*Red before:* with the notice emitted on every failed view (the reviewed
behaviour) the refresh assertion fails — `làm mới không được báo lại:
<uuid>` — which is exactly the reviewer's "one failure + two refreshes = 3".

### Verification (round 5)

`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -p 2
-count=3 -run '^Test(Scan|Activity)' ./apps/viderse/internal/persistence/postgres`
PASS (64.3 s). `go tool sqlc diff` clean after regenerating the runnable query;
`go build ./...` clean; `go vet ./apps/viderse/...` clean; `gofmt -l` empty on
the touched files under go1.26.0 and go1.27.0. `ActivityStore` behaviour is
unchanged apart from the extracted helpers (its unit coverage in the same
package, `TestActivity*`, passes in the run above).

## Verification (round 4)

`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -p 2
-count=3 -run '^Test(Scan|Activity)' ./apps/viderse/internal/persistence/postgres`
PASS (64.0 s). `go tool sqlc diff` clean after regenerating the wakeup query,
`go vet ./apps/viderse/...` clean, `go build ./...` clean, `gofmt -l` empty on
the touched files under go1.26.0 and go1.27.0. Red evidence for both findings is
the run recorded above; the round-2 scheduling test was adjusted for the
fence-gated running rule, and the round-2 adoption test now compares the stored
state against `scan.TaskPresentation` of the locked job instead of a
caller-supplied state that no longer exists.

## Verification (round 3)

`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -p 2
-count=3 -run '^Test(Scan|Activity)' ./apps/viderse/internal/persistence/postgres`
PASS (62.5 s). `go tool sqlc diff` clean after regenerating from the rewritten
wakeup query. `go vet` clean on the touched packages. `gofmt -l` empty under
go1.26.0 and go1.27.0. `platform/pgtest` unit test (2 000 unique names, ≤ 63
bytes) PASS, and the viderse `postgres` package — the main consumer — passes
above. One noverse migrate test (`TestMigrateApplicationRoleCannotDDL`) fails
in this environment both before and after this change: its fixture substitutes
`://postgres:test@` in `TEST_DATABASE_URL`, which this workstation's
`postgres://dat@…` DSN does not contain (verified by rerunning with the previous
`pgtest.go`).

## Verification (round 2)

Everything below was run from `apps/viderse` with `GOTOOLCHAIN=go1.26.0`,
`GOMAXPROCS=4`, `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`,
`lockf -k /tmp/verse-heavy.lock nice -n 10`, `go test -p 2`.

| check | result |
|---|---|
| `go test -count=3 ./internal/persistence/postgres ./internal/activity` | PASS — postgres 143.6 s, activity 0.5 s |
| `go test -count=2 ./internal/persistence/postgres` (after retention) | PASS — 94.6 s |
| `go test -count=1` on the new regressions | PASS — wakeup, task create/link/adopt, remove sweep, lease-expiry matrix, stopped-discovery restart, 10 subtests × 0.8 s |
| `go tool sqlc generate` then `go tool sqlc diff` | clean (no drift) |
| `go build ./...`, `go vet ./apps/viderse/...` | clean |
| `gofmt -l` on touched files under go1.26.0 **and** go1.27.0 | no output |

Red-before was demonstrated by temporarily reverting each change, running its
test, and restoring:
- entry-time fence clock → `TestScanLeaseExpiryDuringLockWait` (transition wrote
  after expiry, no error);
- requeue-instead-of-retire → `TestScanStoppedDiscoveryRetryStartsFreshRun`
  (`lượt cũ phải bị bỏ hẳn: queued`);
- task row inserted outside the transaction (the old `Start`+`LinkTask` pair) →
  `TestScanTaskCreateLinkIsAtomic` (`bỏ lại 1 dòng Việc nền mồ côi`);
- `Start` always resetting → `TestStartAdoptionKeepsProgressUntilFailed`
  (`nhận lại xoá mất tiến độ: <nil>`);
- `ScanDeleteChannelTasks` on `object_ref->>'id'` → the sweep left the row;
- `ScanWakeup` without the queued/running branch → `TestScanWakeupSeparatesActionableFromFuture`
  (`việc queued phải là actionable`).

Head check on 10 000 videos: the unchanged-median case reports 0 rows written
and the score-update statement is the only one touching `rival_videos` besides
the position trim, so the previously unconditional whole-column update is gone
(the round-1 numbers — 10 000-row head check before/after — are in the section
above).



## Fix round 2 — follow-up from the read side

RivalsAPI's `scan_jobs_latest_idx` also sorts "live first"; its predicate was
`state <> 'complete'`, which counted a `cancelled` run as live and could show a
cancelled run over a newer finished one. It now uses the same
`NOT IN ('complete', 'cancelled')` set as `liveJobTx`, in the index and in both
"latest job" laterals, with a real-Postgres regression on their side. I re-ran
the whole `postgres` package against a freshly migrated database afterwards
(index build + both suites): PASS.

### Retention verification

`go test -count=2 ./internal/persistence/postgres` PASS (94.6 s), `sqlc diff`
clean after regeneration, `go vet ./apps/viderse/...` clean, `gofmt -l` empty on
the touched files under go1.26.0 and go1.27.0. `TestScanFinishedJobsArePruned`
was red before the change (`discovery để lại 10 dòng đã dừng, muốn tối đa 1`)
and green after, with the other channel's 4 finished rows untouched.

Status: DONE — store findings for rounds 2–5 are fixed with red-before /
green-after regressions. Round 4 closed the recovered-running scheduling hole
(fence-gated `running` rows, fence expiry as the only sleep target) and made the
task row a projection of the locked job (`scan.TaskPresentation` writes state,
phase and action; caller presentation ignored; terminal jobs write nothing).
Round 5 applied the busy-channel skip before `LIMIT` and made the failure notice
a property of the failed transition, recorded in the same transaction as the
task row (`scan.TaskSync.NotificationID`).
Acceptance: `go test -p 2 -count=3 -run '^Test(Scan|Activity)'
./apps/viderse/internal/persistence/postgres` PASS (64.3 s), `sqlc diff` clean,
`go build ./...` / `go vet` clean, `gofmt` clean on go1.26.0 and go1.27.0.
