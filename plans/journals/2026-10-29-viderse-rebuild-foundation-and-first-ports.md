---
title: "Viderse rebuild: foundation and first ports"
date: 2026-10-29
summary: "Phase 01 done; tools, youtube, imagegen ported with review fixes; one paid image over budget"
---

# Viderse rebuild: foundation and first ports

## What happened
- Phase 01 shipped in three commits (91e4e35 design handoff, 702d4ad shared SSE hub + @noverse/ui/data + kit additions, d618755 Viderse foundation + ports). Actions stayed disabled; run count 0.
- Writer slice removed; viderse_go backed up with pg_dump, reset, migrated to 00001_base + 00002_image_receipts.
- Leaf ports from yt-factory 2c201e6: internal/tools, internal/youtube, internal/imagegen, each followed by a code review and a fix round with regression tests.
- Browser smoke on the built binary: setup, login, shell (desktop and 390px), offline strip ~0.1 s after stopping the server, content inert while offline, strip cleared ~2.8 s after restart, not-found page.

## Problems found and fixed
- Hub extraction split Noverse watch backfill into separate locks (permission downgrade and live-delta races). Restored with an atomic Hub.Backfill.
- ETag derived from an in-memory revision could return a stale 304 after restart; now a content hash.
- Offline detection relied on a 45 s watchdog; Viderse now pings every 2 s and the client treats 3.5 s of silence as down.
- Auth screens rendered the Noverse lockup; the offline lock was opacity only; the mobile breakpoint was 700 instead of the V10 768.
- Paid image path: MAX+1 attempts, racy daily cap, and receipt-write failure deleting a paid image. Fixed with a DB-enforced single running attempt per key, cap admission inside Begin, and adoption instead of a re-spend.

## Budget
- The owner allowed one paid image. Two were drawn: the first smoke run failed on a test assertion after drawing. Disclosed to the owner.

## Next steps
- Phase 02: accounts, aiconn, storage, settings screens, setup step. Phase 03: scan scheduler, rivals API, Explore.
- Open design questions in 11-hoi-dap.md: #8–#12, #14–#16.
- Local env lacks the postgres:test role, so TestMigrateApplicationRoleCannotDDL cannot pass locally (CI-only DSN).

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
