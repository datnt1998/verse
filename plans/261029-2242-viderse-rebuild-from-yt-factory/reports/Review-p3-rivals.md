# Phase 03 — Rivals production-readiness review

## Scope and verdict

**Do not land unchanged. Six major defects were found. No critical security/data-loss defect was established within this assigned scope.**

Reviewed the uncommitted implementation against `local://p03-contract.md`, the phase-03 plan, both backend implementation reports, the schema contract, and existing gateway/composition conventions. Target: `internal/rivals/**`, Postgres rivals store/queries/tests, HTTP rivals handlers/tests/registration, and rival CLI/composition/platform usage. Scope totals **4,072 lines**, including tests and the complete existing `main.go`/`platforms_wiring.go`; this is inspected source size, not added-line count. Supporting reads included scan service/worker/store seams and shared HTTP/auth primitives. Paths below are relative to `apps/viderse/` unless stated otherwise.

Read-only review: no product, test, migration, generated, or plan-state files were edited. This report is the only repository write. Additional probes used Go overlays sourced from `/tmp/p3-review-rivals`; their isolated Postgres schemas were cleaned by pgtest and the throwaway files were removed. No YouTube requests were made.

### Edge-case scout, before systematic review

Dependency/data-flow scouting identified these concrete risk areas: CLI composition is separate from server composition; cap/action flags converge on an exclusive switch; list-state derivation duplicates scan status selection; nullable sort keys cross JSON cursors and SQL; upload-time probing changes effective publication keys; query clipping happens at the HTTP boundary; and retained completed scan jobs are consulted twice per sidebar rival. These determined the targeted probes below.

## Findings — major / blocking

### 1. `--cap` silently suppresses both `--history` and `--action`

**Location:** `cmd/viderse/rival.go:170-181`.

The `case capSet` arm excludes every requested scan action. The documented command `rival scan <id> --history --cap 150` only changes the cap; it never creates a history job. Similarly, `--cap 300 --action pause` leaves an existing queued/running job untouched. Both return success. This is not an invalid-flag combination: the CLI contract explicitly exposes these flags together.

**Evidence:** `TestReviewCLILeaseAndFlags`, using a real isolated database and a held server scanner lease, observed:

- `--history --cap 150` → exit 0, cap 150, history **idle**, `jobId=null`.
- After `--history` created a queued job, `--cap 300 --action pause` → exit 0, history still **queued**.
- `--action pause` without cap correctly changed that same job to paused.

**Fix:** apply `SetCap` first when supplied, then independently dispatch the explicit action or history start. Decide the cap-only branch explicitly rather than letting cap mask another flag. Run inline only for the effective run action; test the full flag combinations against persisted job state while another worker owns the lease.

### 2. Inline scanning can return success without running the requested channel

**Location:** `internal/scan/worker.go:325-336`, consumed by `cmd/viderse/rival.go:185-190`.

The inline worker asks for the globally next runnable job. If it belongs to another channel, it adds that channel to `busy`, sets `job=nil`, and immediately returns because `onlyChannel` is nonempty. It never searches for the target channel behind that unrelated job.

**Evidence:** `TestReviewInlineTargetIsolation` seeded channel A with a queued history job, channel B with a higher-priority queued discovery job, and fresh head-check timestamps to prevent automatic discovery from obscuring the setup. With the scanner lease free, `Service.RunInline(A)` returned **`ran=true`, nil error**, while A remained **queued** and the fixture provider recorded **zero calls**. No network was involved.

**Fix:** filter runnable selection by the requested channel in the store/query. Alternatively continue past unrelated jobs until the target has been searched, rather than treating the first non-target job as exhaustion. Preserve discovery-before-history ordering within the target. Add a regression with unrelated runnable work before the target and a separate held-lease case.

### 3. CLI composition omits durable activity and blocked-platform reporting

**Location:** `cmd/viderse/rivals_wiring.go:66-85`, particularly `newCLIScanService` at line 80.

The alternate CLI factory supplies only Store, YouTube, and Settings. `Tasks` and `Blocked` remain nil. These dependencies are not presentation-only: the service uses them to persist `background_tasks`, error events, needs-attention notifications, and platform blocked state. Disabling an in-process SSE hub for a CLI is reasonable; disabling durable effects is not.

**Evidence:** the real-DB held-lease CLI probe produced a live queued history job with **zero `background_tasks` rows**. Source evidence: `scan.Service.syncTask` at `internal/scan/service.go:507` and `refreshTask` at line 582 immediately return when `Tasks` is nil. The server worker does not repair paused/cancelled jobs by periodically reconciling all task rows.

Concrete stale-state interleaving: serve has a running history job and its running task; CLI pauses/cancels that job through this factory; refreshTask is skipped; heartbeat cancels the fetch; the stopped-fetch/commit branches return without refreshing the task (`worker.go:487-495, 565-570`); paused/cancelled work is not selected again. The existing task can remain running after the durable job has stopped. Standalone CLI failures also cannot invoke a nil BlockRecorder or write the required activity/error notification rows.

**Fix:** use one composition path for durable scan dependencies in server and CLI. Construct the real activity service and real blocked-state recorder for CLI as well; omit only transport-specific hub notifications/runtime wake hooks. Test CLI start/pause/cancel against an existing server-created task, and an inline fixture blocked failure against platform/activity/error rows.

### 4. Sidebar latency grows with every retained discovery job

**Location:** `internal/persistence/postgres/queries/rivals.sql:33-45`; missing supporting historical-job index in `internal/persistence/migrations/00006_rivals.sql:136-139`.

Each rival performs two correlated latest-job lookups. The partial `scan_jobs_live_idx` excludes completed jobs, but neither lateral subquery restricts itself to that index predicate. Hourly discovery retains completed rows indefinitely, so the query repeatedly scans the entire job table even when no history job exists.

**Evidence:** owner-analyzed fixture with **50 rivals × 2,000 videos**:

- No retained jobs: list p50 **19.48 ms**, p95 **39.34 ms**; EXPLAIN execution **24.873 ms**.
- Add **50,000 completed discovery jobs**, 1,000/channel (~42 days at hourly rescan): p50 **462.92 ms**, p95 **557.99 ms**; EXPLAIN execution **482.502 ms**.
- Both job-table sequential scans ran **50 loops**: the history lookup discarded 50,000 rows each time; discovery discarded 49,000 each time. Plan hit 99,992 shared buffers.
- A trial index in that isolated schema, `ON scan_jobs(channel_id, kind, (state <> $$complete$$) DESC, updated_at DESC)`, changed both lookups to bounded index scans; p50 **19.17 ms**, p95 **23.28 ms**. No repository migration was changed.

**Fix:** add a historical/latest-job index matching this selection, or split live-state lookup from indexed latest-completed lookup. The trial index is a concrete working option; use normal SQL string quoting in the migration. Include retained jobs in the performance fixture, not only videos. Do not delete job history merely to conceal the query defect.

### 5. Valid Vietnamese search input can become invalid UTF-8 and return 500

**Location:** `internal/web/rivals.go:405-406`.

`q.Query[:300]` clips bytes, not characters. Valid non-ASCII input can be cut in the middle of a code point before being passed as a Postgres text parameter. It also invents an HTTP-only search truncation rule absent from the binding contract and CLI.

**Evidence:** `TestReviewHTTPBoundaries` through the real authenticated gateway and Postgres store requested `q = "a" + strings.Repeat("ế", 100)` (301 valid UTF-8 bytes). The route returned **500**, body `{"error":"lỗi nội bộ"}`. The 300-byte slice ends inside the last three-byte character.

**Fix:** simplest contract-conforming option: remove the silent clipping. If a deliberate search-length limit is approved, validate it consistently in the shared read service and return 400 rather than rewriting the search; never split UTF-8. Add real-store Unicode boundary coverage.

### 6. CLI add returns fabricated counts and drops the actual scan timestamp

**Location:** `cmd/viderse/rival.go:84, 316-332`.

`rivalChannelJSON` unconditionally emits `videoCount:0`, `outliers:0`, `scannedAt:null`, and history idle. Add has already committed the first Videos page and completed its head check before returning. A short channel may already have history complete; a baseline may already contain outliers. This second projection contradicts both stored state and the HTTP Rival shape.

**Evidence:** source flow in `internal/scan/service.go:225-255` commits baseline, performs `CompleteHeadCheck`, rereads the channel, and returns it. The CLI then discards its `ScannedAt` and substitutes zeros. `TestRivalChannelJSONShape` explicitly pins the zero count instead of proving the contract. This was established by source/data-flow reasoning; no real add/network operation was performed.

**Fix:** after Add, query `rivals.Service.Rival` on the same pool and use `web.RivalToWire`, as HTTP Add already does. Remove `rivalChannelJSON`/its separate zero-value convention. Fixture-test CLI Add against List/HTTP-equivalent projection, including nonempty baseline, an outlier, and an ended channel.

## Findings — minor / non-blocking individually

### 7. Invalid `long` values are silently accepted

**Location:** `internal/web/rivals.go:380`.

Only equality with `1` is checked, so `long=banana`, `long=2`, or `long=true` silently becomes false. **Evidence:** authenticated real-store request `GET /api/rivals/all/videos?long=banana` returned 200. **Fix:** accept absent/0/1 only; reject all other supplied values with 400 `invalid_input`, and cover them without duplicating validation behavior inside a fake.

### 8. Mutation decoder accepts trailing JSON and bypasses a whole-body size limit

**Location:** `internal/web/rivals.go:315,432,455`, delegated to `platform/webapp/server.go:452-460`.

The shared helper decodes only the first JSON value. **Evidence:** `POST .../scan` with `{"action":"start"} {"action":"cancel"}` returned 200 and invoked start in the gateway probe. MaxBytesReader bounds bytes read by that first decode, not the entire supplied body when trailing content is never consumed.

**Fix:** after decoding the object, require a second decode to return EOF, with the same bounded reader; reject trailing values/garbage and an oversized trailing suffix before invoking the service. Existing tests did confirm that an oversized first object and unknown fields return 400 without invoking the mutation. Change the shared helper with corresponding shared-boundary regression coverage rather than creating a second decoder convention.

### 9. Some error paths break the promised machine-readable envelope

**Locations:** `internal/web/rivals.go:315,432,455,554-559`; `cmd/viderse/rival.go:390-392`.

Malformed/oversized mutation bodies use shared `webapp.Error` and omit `code`; neutral 500 does too. The contract makes HTTP `code` required. CLI missing arguments/unknown commands emit plain usage text and exit 2, whereas the rival CLI contract requires error JSON on stderr and exit 1.

**Evidence:** body-boundary probes observed 400 `{"error":"body không phải JSON hợp lệ"}`; the Unicode failure observed the code-less 500. `rival scan` without an id produced **exit 2, non-JSON stderr**.

**Fix:** preserve neutral internal-error messages but attach stable codes; make decode failures use the rival error contract without duplicating parsing. Route invalid CLI invocation through its JSON error envelope/exit 1; leave explicit help as successful human-readable help. Include fractional/mistyped `historyCap` under a deliberate `invalid_cap` policy rather than an unclassified decode failure.

## Verification and contract coverage

### Executed checks

All heavy commands were serialized with `lockf -k /tmp/verse-heavy.lock nice -n 10`, using `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4`, `go test -p 2`, and the specified local test database. No fixtures contacted YouTube.

1. Targeted existing rivals, web, CLI, Postgres tests and the **20k endpoint benchmark**: passed. Command filter was `Test(Rival|Posting|Zone|Usage|Cursor|Video)`; it was not the full repository suite.
2. Overlay probes: six sort/scope paging traversals; real gateway query boundaries; actual generated-query EXPLAIN ANALYZE; 50×2k sidebar with and without retained jobs; six posting edge cases; all mutation auth/origin/body boundaries; held-lease real-DB CLI flag behavior; free-lease inline target isolation.
3. The first version of the extra owner-analyzed retention probe had a local Go variable-redeclaration compile error; it was corrected in `/tmp` before obtaining the reported measurements. No product build failure was observed.

Raw evidence from this session: initial additional paging/plans `artifact://11311`, owner-analyzed/retention plans `artifact://11336`; held-lease CLI and posting/body probes are in the delivered tool output. Observational probes logged the defects above; their PASS labels do not mean those defective behaviors satisfy acceptance.

### HTTP/wire/error mapping

- Every specified route is registered through `g.Handle`; server calls `mountRivals` after scan composition. Gateway tests confirmed all four mutation shapes reject missing sessions (401) and foreign Origin with a valid session (403). No additional per-user resource model is introduced by this feature; shared gateway identity was the inspected boundary.
- Happy-path list/Rival/Video/ScanStatus/PostingSchedule shapes match camelCase, nullable fields, UTC conversions, thumbnail/watch URLs, and nextCursor null-at-end. Delete returns 204.
- Existing mapping tests exercised invalid_input/invalid_cap→400, not_found→404, exists→409, rate_limited→429, blocked/lease_busy→503, unexpected_page/transient→502, plus read cursor/parameter sentinels. Correct propagation of optional recovery/retryAt was checked by those tests. Missing generic/decode codes are finding 9.
- `sort`, numeric `within`, `limit` default/max, and `zone` are validated through the read service; long is finding 7. Empty defaults are preserved. Search uses parameterized ILIKE and escapes literal percent, underscore, and backslash.
- Add 409 can fall back to a raw channel projection if rereading fails (`rivals.go:517-523`), yielding incomplete aggregate/state fields; prefer a deliberately handled read failure over falsely authoritative counts. This is a related projection risk, not an independently reproduced blocker.

### Keyset correctness

The six real-DB matrix cases covered outlier/views/newest × per-rival/all, limit 2, ties, multiple NULL scores/views/publication times, repeated pages wholly inside the NULL tail, terminal cursor absence, and insertion of a new row before the continuation boundary between pages. Every preexisting row was traversed once: **16 single-rival / 32 all-scope rows**, with no duplicates/skips. Per-rival newest uses position; all-newest uses coalesce(uploaded_at,published_at). NULL scores/views/effective publication sort last. Existing tests exercised damaged cursor and query-fingerprint/sort mismatch rejection.

The predicates implement ordinary live keyset semantics, not snapshot pagination: newly inserted rows before the cursor are absent from the continuation; after-boundary rows may be included. Score/view refreshes or exact-time enrichment can move an existing row across a cursor and therefore cause ordinary live-pagination repeats/omissions. That is not an insert-order bug and the binding contract does not promise a frozen dataset. Separate count/page reads likewise do not promise one snapshot under concurrent commits. Additional permanent tests should insert rows on both sides of tied boundaries and cover cursor reuse across every filter/scope combination.

**HMAC decision:** a constant public binary key is not a trust boundary; an authenticated owner who knows it can recompute the tag. This is read-only navigation, so a secret signature is unnecessary. Recommend **base64url versioned JSON with strict validation**, bounded encoded length, valid video id, correct sort/key shape, finite score, integer views/position, valid timestamp, and the filter/scope fingerprint. Keep NULL-tail representation explicit. Reject malformed and mismatched cursors; a well-formed user-chosen read position is harmless. Do not introduce persistent secret management or encryption for this use case. Current decoder checks version/id nonempty/fingerprint/MAC, but does not independently validate sort/key shape; remove the MAC only together with explicit structural validation. This is a simplification recommendation, not a security blocker.

### Performance and indexes

Existing benchmark (20,000 videos, 4 rivals, authenticated handler, 120 iterations/variant) passed the contractual observed p95≤100 ms: worst **18.79 ms**, single-rival newest+q deep. Other notable p95s: all+q+outlier first **17.17 ms**, all+q+newest deep **14.38 ms**, all-newest deep **13.99 ms**.

Owner-analyzed exact generated queries (EXPLAIN ANALYZE with buffers):

| Variant | Execution | Access/result |
|---|---:|---|
| all + q + outlier first | 1.836 ms | score index + title filter, 3,627 removed |
| all + q + outlier deep | 2.305 ms | score/trigram BitmapAnd, 800 candidates |
| rival + q + newest deep | 1.534 ms | trigram bitmap + bounded top-N sort |
| all + within7 + outlier | 0.254 ms | expression-publication index + sort |
| rival + within30 + views | 0.253 ms | expression-publication index + sort |
| all views first | 3.003 ms | 20k-row seq scan + top-N sort |
| all newest after 6,000 rows | 1.330 ms | publication index, 6,000 filtered |
| rival views first | 0.081 ms | channel/views index + incremental sort |
| rival outlier first | 0.177 ms | channel/score index + incremental sort |
| selective q | 0.033 ms | title trigram index |

Deep all-newest is logically keyset but its OR predicate is applied as a filter: it still visits the first 6,000 index entries. No measured 20k acceptance failure resulted; avoid claiming depth-independent index seeks. Finding 4 is the production-growth failure that matters here.

Index audit for 00006:

- PK id / unique `(channel_id,position)`: integrity, positional paging, median/posting samples; keep.
- channel score / channel views: both observed in analyzed sort plans; keep. Their third column is position rather than the requested id tie-breaker, so incremental sort remains; not a correctness bug.
- global score: observed for all-outlier and deep NULL-tail; keep.
- global effective-publication expression: observed for all-newest and within filters; keep.
- title GIN trigram: observed for selective q and deep q; keep. The initial app-role/stale-statistics plans chose seq scans; owner-analyzed fixture proves it is not unused.
- raw channel/published: observed as the channel-prefix access path for sidebar aggregation; no target query orders or filters directly on raw published_at. Its only demonstrated value is that narrower prefix-count path, not within/all-newest support. It is a candidate to consolidate against the other channel-leading indexes, **not proven unused** in this workload; do not blindly remove an index just because its suffix is unused.
- live-job unique index: enforces live-job uniqueness; runnable partial index supports scheduling. Keep those invariants; neither replaces the historical index required by finding 4.

No index was removed from the repository in this read-only assignment. The audit found no wholly unused rival-video index that could responsibly be labelled dead from these plans alone.

### Posting-slot §12 algorithm

Source and existing tests cover calendar-local Monday weeks, adjacent-hour windows, ≥3 uploads over ≥2 weeks, rhythm priority weekly→most_weeks→new→biweekly, weekday merge by identical interval/rhythm, distinct-week union, count/weekday/start ordering, one isTop, offSlot, 7×12 grid, and fractional perWeek/null-under-two. No §12 algorithm defect was established.

Extra table probes all passed:

| Input | Observed contract-relevant result |
|---|---|
| 30 uploads at the same hour over 30 weeks | one weekly slot, weeksHit=30, perWeek=1 |
| 30 uploads in one hour of one week | no recurring slots, offSlot=30, perWeek=30 |
| New York spring DST Sunday→Monday boundary | two calendar weeks; correct Sunday hour blocks and Monday rollover |
| New York repeated fall 01:30 hour | one 01:00–02:00 Sunday slot, count=3 over 2 distinct weeks |
| Exactly 00:00 over three Mondays | Monday grid[0][0], 00:00–01:00 slot |
| Exactly 23:59 over three Mondays | Monday grid[0][11], 23:00–00:00 slot |

The midnight end is represented as `00:00` for the next boundary; no cross-weekday cyclic merging is performed, consistent with grouping independently by weekday. PostingSample selects the newest 30 rows by position before counting known exact timestamps/pending probes.

### Counts, scan semantics, and platforms

- List SQL counts outliers at score≥3 and newOutliers only for discovery-origin rows first seen within seven days; baseline/history origin does not count as new. Existing real-DB tests exercise those distinctions and stale-worker running→queued.
- Sidebar history defaults complete iff history_end, otherwise idle; a selected job overrides it, with running downgraded to queued after worker lease expiry. This matches the inspected scan status derivation for normal reachable job lifecycle. DiscoveryRunning intentionally includes queued/running/backoff, matching the scan service rather than only the literal running state.
- `platforms_wiring.go:47` now supplies `newRivalsUsage`; the counter returns the real rival row count on YouTube, and existing real-DB usage tests passed. No fabricated zeroUsage remains in production wiring. A complete UI cache-refresh assertion after rival add/remove was not exercised here; rivals events invalidate the rivals key, not the platforms key, so it belongs in integration coverage if live Accounts counts are expected while that query remains cached.

## Test quality and missing permanent cases

Useful tests prove algorithm output, ordering, totals, origin distinction, and keyset endpoints. However, green existing tests did not protect the integration seams:

- CLI tests cover mostly pre-DB invalid flags; replace the zero-count projection assertion and add real DB/fixture-provider flag combinations, held/free scanner lease, unrelated channel work, durable activity and blocked failures.
- Add persistent six-way real-DB paging coverage with ties and **multiple** NULL-tail pages; current store tests do not traverse all-scope newest or multi-page views. Check insertion before/after cursor, filter/scope mismatch, malformed shape, and literal ILIKE escaping behavior.
- Retain the Unicode and long validation cases; exercise malformed/trailing JSON and oversized trailing body for each mutation family, and verify required error codes.
- `TestRivalInternalErrorNeutralEnvelope` at `internal/web/rivals_test.go:481-484` compares the **whole response** with the secret string. A JSON body containing that secret would pass. Use substring assertions and assert the neutral error code/message. Inspection/current HTTP probes did not establish an actual secret leak.
- Benchmark only fails above **400 ms** (`rivals_bench_pg_test.go:265`) despite acceptance p95≤100 ms. Separate a noise-tolerant ordinary regression test from an explicit acceptance benchmark that actually enforces 100 ms, including retained jobs and within filters. Seed/statistics maintenance should run as the schema owner rather than assuming the app-role ANALYZE establishes representative statistics.
- Existing cap HTTP test asks a fake to return invalid_cap; it does not prove real cap enforcement. Preserve a small real-service boundary test for min/max, omitted, fractional, string, and out-of-range input.
- Add same-count slot tie ordering, rhythm thresholds at exactly 75%, disjoint-week weekday merge, more-than-30 stored videos with mixed exact/pending timestamps, and checked-but-no-exact-time sample coverage. The additional DST/midnight tables should become permanent.
- Add a platform endpoint projection test (not only the UsageCounter unit/store test) and lifecycle invalidation coverage if Accounts must update live.

## Plan status and recommended landing actions

The assigned read APIs, route registration, wire types, paging queries, posting algorithm, CLI dispatch, and real usage counter are present. Observed 20k read latency satisfies acceptance. End-to-end CLI control/output/activity and growing-history sidebar performance do not. The phase should remain in-progress; do not mark the backend complete based only on the existing green tests. No plan file or task state was changed by this review.

1. Fix all six major findings; coordinate the worker/store isolation and common composition changes with ScanCore.
2. Apply the small HTTP/CLI contract fixes and replace misleading assertions with the behavior cases above.
3. Main agent should run the focused tests after the combined fixes land, plus the owner-analyzed 20k and retained-job benchmarks, existing scan/race and architecture gates. A live YouTube smoke remains owned by the lead and was not attempted here.

**Metrics:** Type-coverage percentage not measured (Go scope); test-coverage percentage not measured; lint/build/race suites not run by this reviewer. Only the targeted test/benchmark/probe runs above are claimed.

**Unresolved questions:** none blocking the recommended fixes. Cursor signing can be removed without introducing a new secret system; snapshot-stable pagination is not a requirement in the binding contract.

Status: DONE_WITH_CONCERNS — review complete; six major correctness/integration/performance defects block landing. Core read paging and §12 edge probes passed, and observed 20k endpoint p95 was below 100 ms; CLI behavior, Unicode validation, and retained scan-job growth require fixes.


# Round 2 — independent re-review after fix round 1

## Verdict and remaining counts

**Do not land yet: 6 findings remain open — 4 major and 2 minor; no critical finding established.** The original findings 1–8 are fixed in their reported reproductions. Original finding 9 is partially fixed: HTTP errors and invalid CLI invocation now conform, but the newly added overlong-query CLI error lacks `invalid_input`. Four major regressions/edge defects and one minor cursor-validation defect were found in the revised paths.

This section supersedes the earlier landing assessment where fixes are verified below. It does not change plan/task state. Review followed `local://p03-fix-round1.md`, including unsigned validated cursors, the shared 200-rune search limit, one durable composition path, and the approved **400 ms regression gate**. That gate is not reported as a defect: measured latency is evaluated independently against the 100 ms target.

## Findings 1–9: pass/fail per original reproduction

| Original finding | Round-2 outcome | Independent evidence |
|---|---|---|
| 1. Cap masks history/action | **PASS** for the original defect | Real CLI with server lease held: `--history --cap 150` persisted queued/cap150; `--cap 300 --action pause` persisted paused/cap300; `--cap 400 --action resume` persisted queued/cap400. Separate cap-only/free-lease defect below. |
| 2. Inline target isolation | **PASS** | With A history queued and B higher-priority discovery queued, free-lease inline A completed A with exactly one fixture provider call; B no longer prevents A from being selected. |
| 3. Missing CLI durable dependencies | **PASS** | Actual openRivalMutations path created a task for queued history and changed it to paused on CLI pause. The shared constructor with only the provider replaced by a blocked fixture produced job error, one failed task, two error events, and a persisted platform pause. |
| 4. Retained-job sidebar growth | **PASS** | Owner-analyzed 50 rivals/100k videos/50k retained jobs: 40 ordinary authenticated GET samples, **p50 16.42 ms / p95 21.58 ms**, without best-of-two sampling. Latest-job probe used `scan_jobs_latest_idx`, one row per rival, 50 bounded index searches, 0.431 ms execution. |
| 5. Unicode clipping | **PASS** | Real gateway + Postgres: the original 101-rune/301-byte input returned 200, 200 Vietnamese runes returned 200, and 201 returned 400 `invalid_input`; no byte clipping/500. CLI rejects 201 runes, but its code mapping remains incomplete under finding 9. |
| 6. CLI add projection | **PASS** | runRivalAdd with a no-network mutation fixture and the real read store returned stored `videoCount=3`, `outliers=1`, non-null scannedAt, history complete. Existing revised stored-projection unit test also passed. |
| 7. Invalid long accepted | **PASS** | Real gateway + store: `long=banana` returned 400 `invalid_input`; `0` and `1` returned 200 with appropriate filtering. |
| 8. Trailing JSON / oversized suffix | **PASS** | POST add, POST scan, and PATCH cap each rejected both a second object and an extra 1 MiB whitespace suffix with 400 before any fake mutation was invoked. Add/scan use invalid_input; cap uses invalid_cap under the deliberate cap-specific policy. |
| 9. Error envelope / CLI invocation | **PARTIAL FAIL** | Neutral 500/internal, malformed JSON codes, missing command/id and unknown command JSON/exit1 are fixed. A 201-rune CLI query still emits error JSON/exit1 **without code**; details below. |

Durable CLI resume now also creates discovery work, so after resume the probe had two active task rows. Cancelling history removed its task while leaving discovery active, consistent with the lead decision that cancel applies to history only; this remaining discovery row is not reported as a leak.

## Open findings — major

### R2-A. Server-issued newest cursors reject normal discovery positions

**Location:** `apps/viderse/internal/rivals/cursor.go:165-170,186-192`; discovery positions originate at `internal/persistence/postgres/scan.go:1210-1230`.

The shared optionalInt validator forbids negative values for both view counts and positions. View counts are nonnegative, but per-rival position is signed: discovery reserves a block below the current minimum (`min - 10000`). Therefore normal new uploads have negative positions. The encoder emits such a cursor successfully; the decoder rejects it on the next page.

**Evidence:** independent real-DB probe seeded four discovery-origin rows at positions -9999 through -9996. `sort=newest&limit=2` returned two videos and a nextCursor. Passing that exact server-generated cursor back returned **ErrInvalidCursor**. Once discovery supplies enough recent rows to fill a page, the normal newest feed cannot continue.

**Fix:** separate signed position parsing from nonnegative view parsing. Accept an int64 position within the scan ordering domain, including negatives; keep views nonnegative. Add a real scan-write→read-page regression, or at least negative-position cursor round-trip followed by multiple SQL pages. Do not make stored positions nonnegative to satisfy the new validator: their signed range is part of discovery ordering.

The new TestCursorStrictValidation calls the service with SortOutlier for every case, even those carrying a newest/views fingerprint. Thus those cases can fail on fingerprint mismatch before reaching the claimed integer/key validation. Add sort-matched tests, not only malformed inputs that happen to fail elsewhere.

### R2-B. Cap-only CLI still runs jobs and makes network requests when the lease is free

**Location:** `apps/viderse/cmd/viderse/rival.go:180-201`, specifically line 196.

The revised switch correctly treats cap-only as SetCap without a control action, and its comment explicitly promises that cap-only does not run. The inline guard still uses `action == empty`, which is true for cap-only, so it acquires the scanner lease and executes any queued work for that channel.

**Evidence:** independent real CLI probe released the server lease, left runnable channel work, installed a fixture http.DefaultTransport that rejects/counts all HTTP locally, then invoked only `--cap 500`. It returned exit0 but made **two trapped HTTP calls**, and the history job changed from queued to **backoff**. No external network was contacted. Under a persisted platform pause it can instead wait for the pause despite the user only changing a setting.

**Fix:** calculate the effective run intent once: history/start, resume, or the no-flags discovery default. Exclude cap-only explicitly from RunInline. For example, run when history is true, action is resume, or both capSet and action are absent. Add both held- and free-lease cap-only tests; assert zero provider/HTTP calls and no unrelated job transition.

### R2-C. Checking new uploads silently resumes explicitly paused history

**Location:** `apps/viderse/internal/scan/service.go:474-479`.

The rewritten check branch starts discovery and then unconditionally invokes ControlBackfill(resume). This undoes an explicit history pause and can also resurrect cancelled history, since ControlBackfill accepts cancelled resume below cap. The binding contract describes check as discovery only. The supplied fix-round decision expands **resume** to stopped work and permits check to replace stopped **discovery**; it does not authorize check to resume history. The code comment claiming a lead decision is not supported by the supplied decision artifact.

**Evidence:** with the scanner lease held (so no provider requests), three actual CLI calls persisted history states:

1. `scan id --history` → queued.
2. `scan id --action pause` → paused.
3. `scan id` (default discovery check) → **queued**.

Thus the effect is persisted before any worker race can explain it. A user asking to check recent uploads loses the separate history pause.

**Fix:** remove the backfill-resume side effect from ScanActionCheck; retain discovery restart behavior. Only explicit resume/start or the existing cap-unpark rule should change paused/cancelled history. Test check against paused and cancelled history while asserting discovery is queued and history stays stopped. If this expansion truly is a separately approved product change, the lead must record that concrete contract change; it is absent from the review inputs.

### R2-D. Inline worker busy-channel handling spins on the same leased job

**Location:** `apps/viderse/internal/scan/worker.go:393-400,424-428`.

The new scoped nextRunnable path ignores busy channel ids. When a channel fence is held by another operation, AcquireChannelLease returns nil, loop adds the channel to busy, then immediately selects that same job through NextRunnableJobForChannel. There is no sleep, no scoped exhaustion branch, and no effect from the busy marker. The server-wide path has busy-channel handling; the rewritten inline path bypasses it.

**Evidence:** real-DB fixture held the target channel lease as an Add-like owner, left its history job queued, and ran the inline worker with a store wrapper that cancels on the fifth runnable selection. It made **five consecutive selections in 5.28 ms**, with zero provider calls, before the explicit cancellation stopped it. The loop has no delay between these unsuccessful claims. A slow Add/fence holder can therefore cause repeated database reads/CAS attempts until expiry rather than bounded waiting.

**Fix:** on a failed inline channel claim, use the existing bounded/context-aware busy-channel sleep before retrying, or incorporate the busy state into scoped selection plus a deliberate wait. Do not return a false successful exhaustion result while target work remains runnable. Add a deterministic contention test proving no immediate repeated claim loop and prompt context cancellation.

## Open findings — minor

### R2-E. Cursor JSON validation accepts invalid suffixes and an inconsistent sort

**Location:** `apps/viderse/internal/rivals/cursor.go:132-145`; caller at `internal/rivals/rivals.go:227-232`.

Decoder.More is not a top-level EOF check: it returns false on closing delimiters, even when those delimiters are invalid trailing input. The decoder also validates payload sort in isolation but never compares it with the requested sort; the filter fingerprint does not enforce that relationship when supplied alongside a deliberately inconsistent payload.

**Evidence:** base64url(valid cursor JSON + an extra closing brace) and base64url(valid JSON + a closing bracket) both decoded successfully. A views payload carrying a valid fingerprint for an outlier request was also accepted by Service.Videos with no error. The store then follows request sort while the populated cursor key belongs to another sort. This is read-only and does not bypass auth; it violates the promised malformed/mismatched→invalid_cursor behavior.

**Fix:** require a second decode to return EOF, exactly as the corrected shared HTTP helper does. Pass the expected sort into decodeCursor and compare it explicitly before selecting key shape. Keep the unsigned design; no HMAC or new secret is needed. Cover closing-delimiter suffixes and inconsistent payload sort with otherwise matching fingerprints.

### R2-F. CLI does not map the new query-length sentinel to invalid_input

**Location:** `apps/viderse/cmd/viderse/rival.go:350-363`.

ErrInvalidQuery was added to the shared read service and HTTP error mapping, but omitted from rivalErrCode.

**Evidence:** actual `rival videos all --q <201 Vietnamese runes>` returned exit1, valid error JSON, but **code was absent**. The equivalent HTTP query returned 400 invalid_input. This leaves the lead-required common search-validation contract inconsistent across transports.

**Fix:** include errors.Is(err, rivals.ErrInvalidQuery) in the CLI invalid_input mapping; add a public CLI test rather than only testing the service sentinel.

## Shared JSON decoder / Noverse compatibility

The revised platform/webapp helper correctly uses the same MaxBytesReader for both decodes and requires EOF before invoking a service. Independent oversized-trailing-whitespace probes confirmed that the byte limit cannot be bypassed by placing a valid object first. The shared helper preserves the old no-code envelope for ordinary DecodeJSON callers; only explicit DecodeJSONCode consumers add codes. No shared-decoder regression was found.

Executed full package compatibility checks:

- `go test -p 2 -count=1 ./apps/noverse/internal/entry/web ./platform/webapp` with the specified toolchain, database and heavy lock: **PASS**; Noverse web 176.613 s, shared webapp 2.359 s.
- Source scouting found that Noverse entry/web actually owns a local decodeJSON helper (`worker.go:586-601`), already requiring EOF; it does not import platform/webapp. The package run still validates the requested Noverse HTTP/API compatibility surface, but should not be described as direct execution of the modified shared decoder. platform/webapp tests exercise that helper directly.

## Verification summary and test limitations

All commands used the required heavy lock/nice wrapper, Go 1.26.0, GOMAXPROCS=4, `go test -p 2`, and the designated local test database. Only the report was changed in the repository. Overlays were confined to `/tmp/p3-review-rivals-r2`, isolated schemas cleaned by pgtest, and temporary files removed. All attempted HTTP in the free-lease CLI probe was intercepted by a local rejecting fixture transport; no YouTube requests were sent.

1. Revised existing rivals/posting/cursor, HTTP, CLI, Postgres and shared-decoder tests passed under the scoped test-name filter. Existing 20k endpoint benchmark worst p95 **24.02 ms** (all/q/deep). Existing retained-job benchmark logged p50 **16.45 ms**, p95 **22.90 ms** (its best-of-two method); independent raw samples gave the 21.58 ms p95 above.
2. Independent probes re-exercised all nine original findings and the new rewritten-path cases. Probe PASS labels indicate successful observation/execution, not acceptance of the logged defective behavior. Negative-position continuation, cap-only requests, history unpause, busy-loop selections, malformed cursor suffixes, sort mismatch and absent CLI code are reproduced facts.
3. The unsigned cursor choice is still appropriate. What needs repair is its validation against the real signed position domain and the request shape, not stronger signing.
4. Shared-constructor durable effects now work under both held-lease controls and a fixture blocked inline run. The held-lease CLI returned promptly; no new premature success from unrelated channel priority remained.
5. Updated tests still miss negative discovery positions, free-lease cap-only intent, check preserving stopped history, and contended inline fences. The cursor table has sort/fingerprint-mismatch confounding; correct that before treating its rows as branch coverage.
6. Full repository build, lint, race, sqlc generation and live YouTube smoke were not repeated by this reviewer. The Noverse/shared web package run and the targeted Viderse tests/probes above are the verification claimed here.

### Recommended next actions

Fix R2-A through R2-D before landing. Apply the small EOF/sort and CLI sentinel fixes alongside their tests. Have the lead reconcile the unsupported check-resumes-history claim with the binding decision; absent a recorded contract change, preserve the user pause. Re-run only the affected cursor/CLI/worker regressions after combined fixes, then the lead-owned final gates. The resolved sidebar performance issue should stay closed unless new measurement contradicts these results.

Status: DONE_WITH_CONCERNS — Round 2 complete. Eight original findings pass their reported reproductions; finding 9 is partially open. Six current findings remain: four major and two minor. Noverse/shared-decoder compatibility passed, durable CLI effects and retained-job sidebar latency are repaired, but negative-position paging and revised CLI/worker action handling still block landing.


# Round 3 — final-tree verification and cancelled-run investigation

## Verdict and open counts

**All six Round-2 findings now pass their independent reproductions. Open defect counts in this reviewed scope: 0 critical, 0 major, 0 minor.** One non-blocking capacity observation remains: terminal scan-job history has no retention bound. The binding contract specifies removal of terminal activity/task rows, not a TTL or count cap for historical scan_jobs; do not conflate those two tables.

The cancelled-run ordering flag identified a real defect in the previous sidebar predicate. The final tree already contains the appropriate SQL/index correction, and the probes below confirm it. No additional product fix was required from this read-only review.

## R2-A through R2-F — pass/fail

| Finding | Result | Final-tree evidence |
|---|---|---|
| R2-A: negative discovery positions reject their own cursor | **PASS / closed** | Independent real-store probe traversed all four negative-position videos across pages once. The permanent `TestRivalNewestPagesFromNegativeDiscoveryPositions` also passed through actual StartDiscovery/CommitPage writes and mixed negative/positive paging. Signed positions and nonnegative views now use distinct validators. |
| R2-B: cap-only runs inline | **PASS / closed** | Actual CLI cap-only probes with free and held scanner lease both made **0 HTTP calls**, retained history queued, and persisted cap400. Permanent free/held tests and the intent table passed. |
| R2-C: check resumes stopped history | **PASS / closed** | Actual CLI with scanner lease held: check preserved paused history, then preserved cancelled history. Permanent `TestCheckDoesNotResumePausedHistory` passed. The check branch now starts discovery only. |
| R2-D: busy inline channel spins | **PASS / closed** | Independent real-DB probe held the channel fence, ran inline with a 150 ms context, and observed **one runnable selection over 154.45 ms**, not the former five selections in 5.28 ms. It returned promptly on context cancellation without a provider call. Permanent bounded-retry test also passed. |
| R2-E: cursor EOF / payload sort mismatch | **PASS / closed** | Independent probes rejected extra closing brace, closing bracket, second JSON object, and a views payload carrying an outlier-request fingerprint. Final strict-validation table passed with each case using its own request sort. |
| R2-F: missing CLI invalid_input for query length | **PASS / closed** | Actual `videos all --q <201 Vietnamese runes>` emitted JSON stderr with `code=invalid_input`, exit1, in both free/held-lease fixtures. CLI mapping unit tests passed. |

These results close the remaining original Round-1 envelope finding as well as the six Round-2 findings. The unsigned cursor decision is preserved; no secret/signature system was introduced.

## Cancelled-run investigation

### 1. The old sidebar predicate was defective; the final predicate fixes it

Relevant paths:

- `apps/viderse/internal/persistence/postgres/queries/rivals.sql:40-52`: both history and discovery lateral subqueries.
- `apps/viderse/internal/persistence/migrations/00006_rivals.sql:143-146`: matching latest-job expression index.
- `apps/viderse/internal/persistence/postgres/queries/scan.sql:212-226`: live-job and latest-job definitions.
- `apps/viderse/internal/persistence/postgres/scan.go:900-930`: ScanStore.Status selection.

The previous `(state <> complete) DESC` promoted cancelled rows ahead of newly completed rows. Its problem was semantic, not simply an unused index. In a fixture with old cancelled history and newer completed history, executing the previous ordering still selected **cancelled**, while the final ordering selected **complete**.

The final implementation uses `(state NOT IN (complete, cancelled)) DESC, updated_at DESC` in both sidebar lookups and the matching index expression. This agrees with ScanLiveJob, which admits only queued/running/backoff/paused/capped/error. Source SQL, generated SQL, and the actual fresh-schema index definition agree. A genuinely live job continues to beat a terminal job, even if the terminal row has a later timestamp; that is the intended live-first status model, not stale-job selection.

ScanStore.Status was already different from the old sidebar query: history first selects a truly live job, otherwise the latest row by updated_at; discovery only selects a truly live job. Consequently an old cancelled discovery cannot be reported as running by the scan endpoint. The corrected sidebar now follows the same lifecycle semantics for its boolean/state projection.

### 2. Actual replacement/check lifecycle: both HTTP routes agree

An independent isolated-DB probe exercised both `StartDiscovery(replaceStopped=true)` and `Service.Control(check)`, with the **real authenticated gateway**, real rivals read store, and real scan status service. Only unused mutation methods were supplied by a test fixture; both GET projections under review were production paths.

For each route:

1. Create discovery, then persist a stopped gap_incomplete run with a stale cursor and next_ordinal=10000.
2. Replace it through the real store/service operation.
3. Confirm the stopped row became cancelled; the new queued row has a **different id**, empty cursor and next_ordinal=0.
4. Read `GET /api/rivals` and `GET /api/rivals/:id/scan`.
5. Complete the current discovery row in the fixture and read both endpoints again.

Observed results:

- While the replacement is queued: sidebar discoveryRunning=true and scan discovery.running=true.
- After replacement completion: both discovery flags=false.
- With an old cancelled history row and a newer completed history row present throughout: sidebar history.state=complete and scan history.state=complete.
- The original stopped cursor/ordinal was not reused by the fresh run.

The permanent `TestRivalListPicksLatestJobIgnoresFinishedRun` also passed both cancelled-old/complete-new and genuinely-live-old/complete-new cases. No stale cancelled-run projection was reproduced on the final tree.

**Concrete fix for the old defect:** the final live-first predicate and matching index are the correct minimal repair. Keep the history and discovery query/index expressions aligned with the scan store definition; do not revert to `state <> complete` or try to fix this by deleting cancelled rows.

### 3. Can cancelled rows accumulate without bound? Yes — historical rows, not live jobs

`StartDiscovery` deliberately keeps each superseded stopped run as cancelled and inserts a fresh run in the same locked transaction (`scan.go:1172-1213`). There is no terminal-job pruning query or count/age cap in the inspected scan persistence paths. Removing a rival cascades its jobs; otherwise both completed and cancelled run history remain stored.

**Concrete growth probe:** two store/check replacements followed by 30 more stopped-run replacements left **32 cancelled discovery rows, two completed discovery rows, and one live queued discovery row** (35 total). Thus each explicit stopped-run replacement adds a retained terminal row; the live-job uniqueness constraint still limits live runs. HistoryCap bounds indexed videos/history scanning, not scan_jobs history size.

**Assessment:** this is an explicit capacity characteristic, not a still-open phase-03 correctness violation. The provided contract does not promise bounded run retention, and the code states that old runs are retained as history. Automatic successful hourly discoveries already accumulate completed runs similarly. No unrequested pruning/retention policy was added or demanded to pass this review.

If bounded storage is a product requirement, the simplest future policy is a small, explicit terminal-run retention rule per channel/kind, preserving every live run and the latest terminal row needed for status/resume semantics, with old diagnostic events retained according to their own policy. Apply it to both complete and cancelled history; do not silently delete diagnostics solely to conceal a slow query. This is a non-blocking operational observation, not a new implementation blocker.

## Final-tree performance

All measurements used the required heavy lock/nice wrapper, Go 1.26.0, GOMAXPROCS=4, `go test -p 2`, and the designated local test database.

### 20k video endpoint benchmark

`TestRivalVideosEndpointLatency` passed every sort/scope/filter/deep-page variant. Worst observed p95: **22.02 ms**, all-scope q/outlier first page; all-scope q deep p95 **21.08 ms**. Other representative p95 values: single-rival newest first **13.95 ms**, outlier deep **12.65 ms**, all/views first **9.15 ms**. Every observed variant was below the 100 ms target. The lead-approved regression failure threshold remains 400 ms; it is not misrepresented as the acceptance target.

### Retained-job sidebar benchmark

- Permanent `TestRivalListAcceptanceLatency`: 50 rivals × 2,000 videos, 50,000 completed discovery jobs; **p50 20.63 ms / p95 22.33 ms** under its best-of-two sampling method.
- Independent owner-analyzed raw fixture: same 50 rivals/100k videos/50k terminal jobs, with **25,000 complete and 25,000 cancelled** rows; 40 ordinary authenticated GET samples, no best-of-two filtering: **p50 15.76 ms / p95 19.82 ms**.
- EXPLAIN ANALYZE for the latest-discovery lateral lookup over all 50 rivals used `scan_jobs_latest_idx`: 50 bounded index searches, one row per rival, **0.483 ms** total execution and 210 shared-buffer hits. The changed expression index remains effective with cancelled rows, not only completed rows.

The retained-job sidebar defect stays closed. An indefinitely growing archive is not mathematically constant-cost storage, but these reads no longer scan the entire job table for each rival.

## Execution evidence and limitations

Executed targeted final-tree regressions: cursor strict validation, cap-only free/held lease, CLI error mapping and scan intent table, check preserving paused/cancelled history, bounded busy-channel retry, actual negative-position scan-write/read paging, cancelled-vs-completed selection, 20k endpoint benchmark, and retained-job sidebar benchmark. All passed.

Independent overlay probes separately covered all R2-A–F reproductions, authenticated sidebar/scan replacement lifecycle, explicit terminal-row growth, and raw mixed-terminal latency/index behavior. Initial throwaway Postgres probes had a wrong test-side Job ordinal field name and then omitted the now-required provider dependency; those fixture errors were corrected before obtaining the lifecycle/latency results above. No product failure is inferred from those temporary harness errors. Previously completed cursor/CLI/negative-position probe results were reused rather than rerunning them unnecessarily.

No YouTube requests were made: CLI HTTP attempts were intercepted locally, and the read/contention lifecycle provider fixture cannot perform network calls. Temporary probes under `/tmp/p3-review-rivals-r3` were removed; pgtest cleaned isolated schemas. Only this report was modified in the repository. No build/lint/race/full-repository/Noverse rerun is claimed for Round 3; the lead owns final combined gates. Noverse/shared-decoder compatibility remains the exercised Round-2 result.

Status: DONE — Round 3 complete. R2-A through R2-F are closed; the cancelled-live sidebar/index defect is fixed and verified on the final tree. Open defects in this assigned scope: 0 critical, 0 major, 0 minor. One non-blocking capacity observation remains: historical terminal scan_jobs have no retention bound.
