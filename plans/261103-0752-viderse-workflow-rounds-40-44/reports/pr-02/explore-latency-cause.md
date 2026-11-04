# Explore latency — cause analysis for `TestRivalVideosEndpointLatency`

Read-only investigation. No source, schema, threshold or test change; no database, service, build,
lint, formatter or Git command was run. Nothing here re-labels the failed full gate as passing.

Inputs read: `rivals_bench_pg_test.go`, `reports/pr-02/backend-full-gate.json`,
`reports/pr-02/board-latency-isolated.txt`, the production code reached by the benchmark
(`internal/web/rivals.go`, `internal/rivals/rivals.go`,
`internal/persistence/postgres/rivals.go`, `queries/rivals.sql`, `gen/rivals.sql.go`),
the schema (`migrations/00006_rivals.sql`, `00012_clone.sql`, `00017_queue_removed_at.sql`),
`platform/pgtest`, and the historical measurement reports for the same test.

Labels: **[observed]** = present in the evidence files or in source read here;
**[inference]** = reasoned from source shape, not observed at runtime.

---

## 1. Exact path measured

`TestRivalVideosEndpointLatency` (`rivals_bench_pg_test.go:182`) builds the real gateway
(`newBenchGateway`, `:60`), mounts `rivals.New(postgres.NewRivalsStore(pool))` (`:72`) and issues
authenticated in-process requests (`benchGateway.get`, `:98`). `httptest.ResponseRecorder`, no
socket: the number is handler + store + JSON wall time.**[observed]**

Per measured request (`GET /api/rivals/{id}/videos?sort=…&limit=200`):

| # | Step | Source |
|---|---|---|
| 1 | auth + cookie check | gateway, via `newBenchGateway` |
| 2 | `RivalExists` — channel-scoped only | `internal/rivals/rivals.go:243-251` |
| 3 | `CountVideos` — always, **no cursor, no LIMIT** | `rivals.go:292`, store `rivals.go:63-77` |
| 4 | `PageVideos` — page query with keyset cursor | `rivals.go:296`, store `rivals.go:79-155` |
| 5 | `ListVideoUsage` for the ~201 returned ids | `enrichPage`, `rivals.go:353` |
| 6 | `ListAutoPickBaselines` | `enrichPage`, `rivals.go:371` |
| 7 | `ListRivals` — the whole sidebar aggregate | `internal/web/rivals.go:477` |
| 8 | map 201 videos + `insight.ScoreReason` + `WriteJSON` | `web/rivals.go:473-521` |

So one page read costs **5 SQL statements** on the all-channel path (6 on a channel path) — steps
2–7 — plus JSON of 201 videos. Step 7 is a second full read of the video table on every page
request.

---

## 2. The all-channel outlier query

### 2.1 Count (always runs, no LIMIT)

`queries/rivals.sql:62-96`, generated text identical in `gen/rivals.sql.go:countRivalVideos`:

```sql
SELECT count(*)::bigint
FROM rival_videos v
CROSS JOIN LATERAL (                       -- rivals.sql:70
    SELECT array_agg(DISTINCT u.channel_id) AS channels
    FROM ( SELECT j.channel_id::text FROM clone_jobs j
           WHERE j.source_key IN ('channel:'||j.channel_id::text||':video:'||v.id,'video:'||v.id)
             AND j.superseded_at IS NULL AND j.status <> 'cancelled'
           UNION
           SELECT q.channel_id::text FROM clone_queue q
           WHERE q.source_video_id = v.id AND q.state IN ('queued','started')
             AND q.removed_at IS NULL ) u
) made
WHERE … ($7 IS NULL OR $7 = ANY(COALESCE(made.channels,…))) … ;
```

### 2.2 Page (sort=outlier is the default branch)

`queries/rivals.sql:98-140`, same `CROSS JOIN LATERAL` at `:106`, then:

```sql
AND ($11::text IS NULL OR ($12::float8 IS NOT NULL AND
       (v.score < $12 OR (v.score = $12 AND v.id > $11) OR v.score IS NULL))
     OR ($12 IS NULL AND v.score IS NULL AND v.id > $11))
ORDER BY v.score DESC NULLS LAST, v.id ASC
LIMIT $13::bigint            -- service passes limit+1 = 201
```

The same lateral is in the newest, views and position page queries (`rivals.sql:148,190,230`) —
five occurrences in total (`grep -c` = 5).**[observed]**

### 2.3 Why the lateral runs for every candidate row

`count(*)` has no LIMIT, so the lateral is executed once per matching `rival_videos` row — for the
all-channel fixture that is 20,000 subplan executions per request.**[inference from SQL; the plan
itself was not observed]**

For the page query the lateral sits in an inner join, so it must be executed for every row the
outer scan emits before `ORDER BY`/`LIMIT` can stop. Early stop is only possible when the plan walks
an ordering index and the lateral is applied to the 201 surviving rows. Whether that happens is a
planner decision nothing in the source pins down.**[inference]**

The four `made.*` predicates are all constant-true when the request carries no `made/notMade`
parameter (`$7/$8 IS NULL`, `$9/$10 false`) — and the benchmark never sends one — but they are
**runtime parameters**, not constants, so PostgreSQL cannot prove `made` is unused and cannot
remove the join. `channelArg`/`cursorID`/`cursorFloat8` pass SQL `NULL` for absent filters and
cursors (`postgres/rivals.go:280-299`).**[observed: code; inference: planner behaviour]**

### 2.4 Indexes available

| Index | Definition | Required order |
|---|---|---|
| `rival_videos_score_idx` | `(score DESC NULLS LAST, id)` — `00006_rivals.sql:70` | matches all-channel outlier ORDER BY |
| `rival_videos_published_idx` | `(coalesce(uploaded_at, published_at) DESC NULLS LAST, id)` — `:73` | matches all-channel newest |
| `rival_videos_channel_score_idx` | `(channel_id, score DESC NULLS LAST, position)` — `:66` | channel-scoped; tie-breaker is `position`, not `id`, so a sort of the channel's rows is still needed |
| `rival_videos_channel_views_idx` | `(channel_id, view_count DESC NULLS LAST, position)` — `:68` | no **global** `view_count` index exists |
| `clone_jobs_active_source_idx` | unique partial `(source_key) WHERE superseded_at IS NULL` — `00012_clone.sql:28` | usable by the literal `source_key IN` arm |
| `clone_queue` | `(source_key) WHERE state='queued'` and `(state,priority,channel_id,position)` — `00012:95-96`, `00017:3-6,13-14` | **no index on `source_video_id`** — the queue arm of the lateral cannot be an index lookup on that column |

`00006_rivals.sql` is unchanged since 2026-10-30 and no migration was added on 2026-11-03/04, so the
index set the benchmark sees today is the same one the pre-lateral measurements used.**[observed:
file contents and mtimes]**

### 2.5 Fixture cardinality (`seedRivalBenchVideos`, `:139-180`)

* 4 channels × 5,000 videos = **20,000** rows; `channel_id = UC…(n%4)+1`.
* `score`: `NULL` for `n%20=0` (5%), else `(n%1500)/100` → 1,500 distinct values, ≈13 rows each.
* 10% of titles contain `truyện đêm`; `published_at` spread over 400 days; 25% have `uploaded_at`.
* 16 variants; deep variants walk `deepCursor` = `pages × 200` rows (channel: 12 → 2,400 skipped;
  all: 30 → 6,000 skipped), `LIMIT` 201 each (`:191-206`, `:208-226`).
* **`clone_jobs` and `clone_queue` stay empty**: the fixture inserts only `rival_channels`,
  `rival_videos`, `scan_state`. `pgtest` provisions a fresh schema per test and drops it
  (`platform/pgtest/pgtest.go:73-103`), so the lateral's two inputs have no rows in every sample.

### 2.6 Benchmark lifetime / concurrency (`:233-268`)

* Strictly sequential: 16 variants in declaration order, no goroutines, one pool, one process.
* 1 untimed warm-up request, then **120 samples** per variant; `p95 = int(120×0.95) = 114` — the
  114th smallest, i.e. only **6 samples** decide p95.
* Only the worst p95 is asserted (`>400` fails, `:266`); every per-variant `p50/p95` is written with
  `t.Logf` (`:259`) but the captured artifact keeps only the worst.
* The two `all/outlier` variants are the 9th and 10th entries of the 16-variant sequence
  (`:223-224`), i.e. they are measured back to back in one time window.

---

## 3. What the evidence shows

**[observed]** `backend-full-gate.json`: `go test -p1 ./...`, `REQUIRE_POSTGRES_TEST=1`, isolated
PostgreSQL QA environment, result **failed**, `all/outlier/deep` p95 **1371.48 ms** >
`limitMs` 400, `allOutlierFirstP95Ms` **505.89**, total elapsed **528.76 s**.

**[observed]** Parent-supplied measurements for this run (the JSON artifact itself keeps only the
worst p95): channel-scoped variants **37–58 ms**; the other all-channel variants **118–230 ms**;
`TestCloneBoardThousandJobsP95` failed in an earlier full run at
**106.46 ms** against 100 ms and measured **37.855916 ms** alone (`board-latency-isolated.txt`) —
a **2.81×** inflation of the same query in the contended run.

**[observed]** Historical measurements of *this exact benchmark*, before the usage lateral existed:

* `P3-RivalsAPI.md:87-88`: `all/outlier/first` p95 **2.42 ms**, `all/outlier/deep` **19.43 ms**;
  worst variant in that run **27.09 ms**.
* Same report `:98-113`: `EXPLAIN (ANALYZE, BUFFERS)` of the all+q+outlier first page is
  `Limit → Sort → Seq Scan on rival_videos` with **no lateral**, 8.3 ms; the companion count 7.8 ms.
* `Review-p3-rivals.md:426`: `all/views first` **9.15 ms**, channel `outlier deep` **12.65 ms**.

**[observed]** The lateral was introduced on 2026-11-02 by the PC-parity work
(`plans/reports/261102-desktop-parity/pc-discovery-complete.md:5-24`: "Added a per-row
`CROSS JOIN LATERAL` … `CountRivalVideos` + all four page queries now filter"). That report records
`gofmt` + `go build` only — **no latency measurement and no test at all** for the new filters. A
search of every `*_test.go` under `apps/viderse/internal` for `MadeFor`/`NotMadeFor`/`MadeAny`/
`NotMadeAny` returns **zero hits**: the code the lateral implements is entirely untested.**[observed]**

---

## 4. Analysis

### 4.1 H1 — structural: per-row usage lateral scales with candidate rows (primary, partially quantified)

Under H1 the failure is a real read-path regression introduced by the usage filter, not by PR2:

* The request cost now has a term proportional to **every candidate row**, twice (count + page),
  where before it was proportional to the page size plus an index/sort. The count query alone
  evaluates the lateral 20,000 times for `all` and 5,000 times for a channel.**[inference]**
* The channel-scoped cluster (37–58 ms) and the all-channel floor (118–230 ms) are consistent with
  that linear term at roughly 2–7 µs per lateral loop (5,000 → ~20,000 rows, plus the per-request
  `ListRivals`).
* `deep` must touch more rows than `first` by construction: `first` can stop after 201 ordering
  index entries (if the planner picks that path); `deep` has to skip 6,000 entries and evaluate the
  cursor OR over the remaining ~14,000 rows. The pre-lateral data shows the same direction (2.42 → 19.43 ms), so the 505 → 1371
  ordering is expected from the query shape, and the excess is the lateral multiplied by 14,000.

What H1 does **not** close: `all/outlier/first` at 505.89 ms. If the first page walks
`rival_videos_score_idx`, it applies the lateral 201 times (~1 ms) and should land in the 118–230 ms
band; if it seq-scans 20,000 rows it lands near the cost of the other all-channel scans — and the
`all/views` variants, which have no global `view_count` index and therefore *must* scan every
candidate row, are part of the reported 118–230 ms band (the artifact does not enumerate them; if
that band excludes `all/views`, this cross-check is weaker and the loop model's gap is larger). Either way the reported 505.89 exceeds the all-channel floor by ~275–390 ms, and the deep
number exceeds it by ~1.1–1.25 s. **[inference]** A pure loop-count model therefore explains the
*ordering* of the variants but not the *magnitude* of the excess; the page plan would have to be
materially worse than the sibling all-channel page plans (e.g. heap fetches from the score index
in cursor order plus a 14,000-row top-N sort), which source inspection cannot confirm.

### 4.2 H2 — host stall inside the two consecutive variant windows (secondary, not excluded)

p95 tolerates 6 slow samples. A p95 of 1371 ms means ≥6 of 120 sequential requests were blocked
≥1.37 s — roughly 8 s of blocked time inside a ~20–30 s variant window. The variant immediately
before it (`all/outlier/first`) needs ~3 s of blocked time for its 505.89 ms p95. One disturbance of
~10 s starting in variant 9 would produce exactly the reported pattern and leave every other
variant untouched. The two slowest variants are adjacent in the declared order (`:223-224`), which
is consistent with both H1 (same query shape) and H2 (same time window).**[inference]**

Supporting, but not sufficient: the same full run inflated an unrelated latency test 2.81× over its
isolated value (`board-latency-isolated.txt`), and the full run overlapped finite mobile2 tests and
an isolated ffmpeg download. A 2.81× multiplier applied to 505.89 ms gives ~180 ms — under the gate —
so contention alone does not explain a 3.4× breach either. Notably, `TestCloneBoardThousandJobsP95`
uses `p95 = samples[18]` of only 20 samples, a *weaker* tail statistic than 114 of 120, so its
inflation is a lower bound on the disturbance.**[inference]**

### 4.3 Excluded

* The 400 ms threshold and the assertions were not touched (`:266`), and neither was the query text
  (mtimes 2026-11-02 or earlier).
* `q`, `long`, `within` are empty/0/false in every measured variant, so the `title_fold` GIN index and
  the `long`/`within` predicates are not the cause of the inter-variant spread.
* `ListVideoUsage`/`AutoPickBaselines`/`ListRivals` are constant across variants (same page size,
  same fixture) and therefore cannot produce the outlier-only pattern.

### 4.4 What cannot be concluded from the files present

The distinguishing evidence is missing: the run's **per-variant `p50` lines** (`:259`) and any
`EXPLAIN` of the two failing variants. `backend-full-gate.json` stores only the worst p95. `p50`
would separate the hypotheses in one line: p50 ≈ 500/1370 ms ⇒ query-bound (H1); p50 in the tens of
ms with only the tail high ⇒ stall-bound (H2).

---

## 5. Can PR2-touched paths affect the query semantics?

**No, on the evidence available.** The measured path is `internal/web/rivals.go` →
`internal/rivals` → `internal/persistence/postgres/{rivals.go,queries/rivals.sql,gen/rivals.sql.go}`
plus the migration set. PR2/V36 scope (`phase-03-v36-viewer-and-actions.md:22-26`) touches
`internal/web/performance.go`, `admin_gate.go`, `internal/media/media.go`,
`postgres/{blobs,clone,clone_cutout,clone_editor}.go`, `pipeline/service.go`,
`editor/projects.go`, `cover_trials.go` and frontend files — none of them is on this read path, and
`performance.go` is route *mounting*, not per-request middleware.

Corroborating filesystem state: `rivals.go` (store) 2026-11-02 12:26, `queries/rivals.sql` and
`gen/rivals.sql.go` 2026-11-02 12:24, `00006_rivals.sql` 2026-10-30; PR2 files 2026-11-03 23:31 –
2026-11-04 00:10; no migration newer than `00037` (2026-11-02).**[observed: mtimes; no `git diff`
was run during this investigation, so this is supporting evidence, not a diff audit.]**

Two caveats worth stating:

1. PR2's focused backend command includes `./internal/persistence/postgres`
   (`phase-03:51`), which runs this benchmark — so PR2 *executes* the gate but does not alter it.
2. The PR2 work does change `clone_jobs`/`clone_queue` write paths (delete transaction,
   `clone_cutout`, `clone_editor`). In this fixture both tables are empty, so even a semantic change
   there cannot alter a single measured row. It can only reach this measurement through host
   resource usage (the browser/build/node suites and ffmpeg download running beside the full gate).

Conclusion: PR2 can explain **test/host resources**, not query semantics.

---

## 6. Smallest safe future fix

Ordered; each step is smaller than the next and does not change the 400 ms gate.

1. **Classify before fixing (no code change).** Retrieve the `p50` line for
   `all/outlier/first` and `all/outlier/deep` from the failed run's stdout if it is still retained.
   That single pair decides H1 vs H2 (§4.4) and costs nothing.
2. **Make the gate measure the query, not the host — test-only, mirrors an existing pattern.**
   `TestRivalListAcceptanceLatency` already handles this exact problem for the sidebar
   (`rivals_bench_pg_test.go:276-298`): `t.Skip` under `-short` plus best-of-2 per sample, with the
   comment that both repetitions are slow if the query is slow. Applying the same two-line sampling
   rule to `TestRivalVideosEndpointLatency` is the smallest safe change: no threshold, assertion or
   product change, and it doubles as the H1/H2 discriminator. This is the recommended minimum
   regardless of which hypothesis holds.
3. **Only if an isolated/beat-of-2 measurement still exceeds 400 ms — drop the unused lateral.**
   With no `made/notMade` parameter all four `made.*` predicates are constant-true, so the usage
   subquery is provably unused. The minimal product change is a no-usage-filter variant of
   `CountRivalVideos` and of the four page queries, selected in `CountVideos`/`PageVideos` from the
   already-validated `PageQuery` (which already knows whether any made/notMade filter is set). An
   alternative with fewer new statements is to move the predicates into correlated `EXISTS(...)`
   expressions in the `WHERE` so PostgreSQL's OR short-circuit skips the lookups when the
   parameters are `NULL` — but that depends on the planner keeping the subplans as qual-level
   `SubPlan`s and must be proven with `EXPLAIN`, so it is not the safe default.
   **Prerequisite:** add the missing `made/notMade` regression tests (§3) — there is currently no
   test to prove the split preserves semantics.
4. **Do not** add `clone_queue(source_video_id)` for this failure. That index is the correct fix for
   the *filtered* path at real scale (the lateral/EXISTS currently cannot use an index on that
   column), but the benchmark's `clone_queue` is empty, so it cannot move these numbers at all.

---

## 7. Verification oracle

All commands are for the parent to run; they are listed, not executed here.

| # | Command / inspection | Pass condition |
|---|---|---|
| O1 | Re-read the failed run's stdout for `all/outlier/first` and `all/outlier/deep` `p50` | p50 ≈ p95 ⇒ H1; p50 ≪ p95 ⇒ H2 |
| O2 | `REQUIRE_POSTGRES_TEST=1 TEST_DATABASE_URL=… go test -p1 -count=1 -run '^TestRivalVideosEndpointLatency$' -v ./apps/viderse/internal/persistence/postgres`, alone on an idle host | All 16 variants p95 < 400 (expected: ≤ ~25 ms if H2; ≥ ~1 s on `all/outlier/deep` if H1). Reference values from the pre-lateral build: 2.42 / 19.43 / worst 27.09 ms |
| O3 | `EXPLAIN (ANALYZE, BUFFERS)` on `CountRivalVideos` and `PageRivalVideosByOutlier` with `$1 NULL`, `$11/$12 NULL`, then with the deep-cursor values | `Subquery Scan on made` loops ≈ 201 for a first page ⇒ index early stop; loops ≈ 20 000 / ≈ 14 000 ⇒ full-scan lateral. Also record `Index Scan using rival_videos_score_idx` vs `Seq Scan` + `Sort` |
| O4 | After fix 3: same isolated bench run | No variant regresses; `all/outlier/first` and `deep` return to the O2 reference band with no threshold change |
| O5 | After fix 3: new `made=any` / `notMade=any` / `madeFor` / `notMadeFor` store tests | Totals and page contents equal the values produced by the current lateral implementation (record them *before* the split) |

O2 doubles as the release-gate re-measurement: it must be reported as its own run, not as a
reinterpretation of the failed full gate (same rule already applied to `board-latency-isolated.txt`).

---

## 8. Status

**DONE_WITH_CONCERNS**

* Findings that are fully evidenced: the exact request path and its 5–6 SQL statements per page; the
  unconditional per-row usage lateral in the count and all four page queries; the index set and the
  missing `clone_queue(source_video_id)` index; the fixture (20 000 rows, empty clone tables,
  per-test schema); the benchmark mechanics (sequential, 120 samples, 6 samples define p95, two
  failing variants adjacent); the pre-lateral 2.42/19.43/27.09 ms reference; the 2.81× board
  inflation in the same run as evidence of host contention; and that no PR2-touched file is on this
  read path.
* Concern: the artifacts preserved for the failing run contain only the worst p95, so the split
  between "usage lateral made the all-channel outlier path genuinely slow" and "those two adjacent
  sample windows were blocked on the host" cannot be decided read-only. The loop-count model
  explains which variants should be worst but under-explains the absolute excess of both failing
  numbers; no claim of a proven single root cause is made here.

## Parent disposition

- Không áp dụng `-short`/best-of-2 để bỏ một lượt lỗi: đó không phải sửa nguyên nhân và có thể che hồi quy. Giữ nguyên test, ngưỡng400ms và mọi sample.
- Query Khám phá không thuộc thay đổi backend V36 được cấp phép; không tự thêm index hay tách query trong PR2. Nếu đo riêng ở điều kiện không chạy build/full UI suites vẫn lỗi, cần báo blocker ngoài V36 thay vì sửa lấn scope.
- Một lượt đo riêng có điều kiện tài nguyên khác chỉ là bằng chứng chẩn đoán; không đổi trạng thái lượt full đã lỗi thành pass.
