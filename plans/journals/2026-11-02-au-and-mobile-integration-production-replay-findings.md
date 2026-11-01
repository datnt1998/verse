---
title: "AU and mobile integration: production replay findings"
date: 2026-11-02
summary: App-wide assistants deployed; native playback/export ownership repaired with production proof.
---

# AU and mobile integration: production replay findings

## What happened
The integrated AU change made assistant provider/model/reasoning/fallback app-wide and admin-only, with immediate persistence, classified expired/key/quota recovery and bulk swap/undo. Independent contract and visual reviews closed the repaired findings. Both apps were deployed through their owning Mac mini scripts.

Actual native-reader replay exposed failures that unit-only verification had missed: chapter-playback and named-export manifests shared cache storage but permissive JSON decoding confused their families; aggregate duration was replaced by a per-file row. Separate identity checks now preserve download filenames and duration. The real Listen surface also used synthesis-worker ffmpeg state for a server-owned assembly capability; readiness now probes the serving gateway.

## Verification and boundaries
Final complete Noverse backend, Noverse web, Viderse backend, Viderse web and shared-kit gates passed. Production native rendering automatically published MP3; Chromium played it via HTTP206, and the downloaded M4B plus API metadata reported10.64s. Hosted/native Noverse revision is c6f425e7; Viderse functional release is9ab528eb. No commercial provider key or assistant selection was changed for QA.

Three independent-process production cold measurements reduced Viderse entry to134.3KiB and first6s application JS to198.86KiB; fonts are3files/40.95KiB. CSS remains70.17KiB versus the proposed40KiB, and request/outlier timing limits remain explicit in the performance report. Do not call every proposed budget passed.

## Decisions and lessons
Do not hash row versions into audible identity; an identical redo may correctly retain the same canonical playback identity. Do not remove deliberate idle prefetch merely to lower request counts, or claim unsupported StyleX splitCss configuration. A same-document hash goto is not a new cold navigation. The ordinary git index was stale; all integration commits used fresh private indexes and explicit ownership paths to avoid staging unrelated work.

## Remaining acceptance
Production performance exceptions and physical-device/long-list measurement limits stay in the owning plan/report until explicitly resolved. Stateful release receipts are not evergreen product authority. AgentWiki publish skipped.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
