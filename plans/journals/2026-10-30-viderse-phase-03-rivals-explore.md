---
title: "Viderse rebuild: phase 03 rivals and Explore"
date: 2026-10-30
summary: "Rival channels, background scan worker, outlier score, §12 posting slots and the desktop Explore screen; scan backend needed five review rounds, smoke found idle DB writes, an SSE auth query per ping and a wrong weekly rate"
---

# Viderse rebuild: phase 03 rivals and Explore

## What happened
- Backend: `youtube` (channel reference parsing, exact upload time from `/player`), `scan` (one in-process worker holding a YouTube scanner lease, per-channel fence, history and new-video discovery jobs, pause/resume/cancel, restart recovery), `rivals` (follow/unfollow, outlier score = views / median of the newest 150, cursor paging, §12 posting slots, usage), Postgres stores and migration 00006 (`rival_channels`, `rival_videos`, `scan_state`, `scan_jobs`, `scan_schedule`, pg_trgm).
- CLI `viderse rival add|list|scan|videos|posting-slots|remove`, sharing the scan-service constructor with `serve`.
- Web: desktop Explore against the `Explore` artboard: rival sidebar, virtualised grid, filters and sorts, preview + why panel, multi-select, rival card, history-scan card, posting schedule (VN/New York, weekday × 2-hour grid). Controls that need owner channels or the queue render locked with a reason.
- Kit additions (opt-in, Noverse unchanged): `Segmented fit="content"`, `ActionMenu width`. Shared JSON decoder in `platform/webapp` now rejects trailing data.

## Review and QA rounds
| Area | R1 | R2 | R3 | R4 | R5 | R6 |
|---|---|---|---|---|---|---|
| Scan backend (critical / major / minor) | 2 / 14 / 3 | 0 / 8 / 3 | 1 / 4 / 2 | 0 / 2 / 1 | 0 / 4 / 0 | 0 / 0 / 1 |
| Rivals API + CLI (critical / major / minor) | 0 / 6 / 3 | 0 / 4 / 2 | 0 / 0 / 0 | — | — | — |
| Explore artboard QA (majors / minors) | 4 / 13 | 1 / 3 | 0 / 3 | — | — | — |

The three QA round-3 minors (slot hover alpha, label brightening on hover, avatar initials) and the scan round-6 minor (a concurrent writer's notification returned after commit) were fixed after their rounds; the minor has a red-first regression test, the QA items were checked in the live browser by computed style.

## Problems found and fixed
- Worker lease expiry did not cancel an in-flight fetch; a page could commit after another process took over. Work now runs under a deadline at the held lease expiry.
- A channel fenced by another owner made the worker re-select it without a timer (probe: 25 500 acquisitions in 40 ms). Running jobs are now fence-gated in the wake-up query and the worker waits until the fence expires.
- Task rows could show "running" with a Cancel button after a committed pause. Every scan task write now derives state, phase and action from the locked job.
- Finished `scan_jobs` rows grew without bound; the sidebar query took 462 ms at 50 rivals × 2 000 videos + 50 000 jobs. Retention keeps the newest finished job per channel and kind; index fix brings p95 to 51 ms.
- SSE re-authenticated the session on every 2 s ping (one DB query per open tab every 2 s). Recheck is now every 15 s.
- Reading the YouTube pause wrote the `platform_state` row on every idle heartbeat (`UPDATE … RETURNING`). It is now a SELECT that writes only to clear an expired pause: idle writes 8 → 4 per minute.
- Phase-02 bug: `.conn-tile` used the undeclared `--vv-s-64`, so its min-height never applied. `--vv-hover`/`--vv-hover-row` were also undeclared, so every Explore hover was a no-op. A design-rules test now fails on any `var(--x)` without a fallback that is not declared.
- A filter chip lost `position: relative`, so its invisible native select covered the viewport. Guarded by a design-rules test.
- "Mỗi tuần" divided by calendar weeks touched: a 3-video-a-day channel showed 15 instead of ~22. Now (videos − 1) / real sample span, floor one week.
- The posting card said "Trinh sát đang đọc dần" when nothing was pending but no slot repeated. It now distinguishes paused, pending, nothing read, and no repeating slot.
- During a scan the `activity` stream sent up to 6 frames per second (a task created, updated and deleted within 0,5 s, plus a second notifier for the same type). Every event type now has exactly one publisher that coalesces to ≤ 4 frames/s with a trailing frame for the final state; live: ≤ 2 frames/s per type.
- A failed scan task notified on every refresh and the dispatched "Huỷ" on a waiting new-video check cancelled the history scan. The failure notice is now written in the same transaction as the task, only on the transition into failed; the waiting check shows "Mở".
- Thumbnails that fail to load (offline, blocked) are hidden so the card background shows instead of a broken-image icon.

## Numbers (built binary, real YouTube, 2 rivals)
- Add rival: 4–6 s (first Videos page + median). History scan, cap 150: 4 pages, 120 added, ~8 s; SSE ≤ 2 frames/s per type during a scan.
- `kill -9` mid-scan then restart: the history scan resumed and finished at 150/150.
- Video list p95 at 20 000 videos: 27 ms (slowest: all rivals + search). Sidebar p95: 51 ms.
- Idle 60 s: CPU 0,10 %, RSS ~18 MB, 0 child processes, SSE 0,5 frames/s, 4 DB writes/min (lease heartbeat), ~25 transactions/min including the measuring queries.
- First-load JS 129,1 kB gzip; Explore is a lazy chunk. 20 000-video rival: 18 cards at rest, 24 while scrolling, 660 DOM nodes.

## Budget
- No paid calls. Live calls were free public YouTube pages in the lead's smoke only.

## Next steps
- Open design questions: 11-hoi-dap.md #31–#37 (locked controls, scan-state copy, menu row padding, weekly rate and no-slot copy, search box, English examples, phone).
- Phase 04 (Channels, Library) unlocks the owner-channel controls; phase 05 the queue; phase 09 `MExplore`.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
