---
title: Export progress fill repaired and deployed
date: 2026-11-07
summary: Restore visible live progress through StyleX backgroundImage; preserve running exports during rollout.
---

# Export progress fill repaired and deployed

## Cause
Production export fill width changed but computed background was transparent/none. Replaced the dropped StyleX background shorthand with backgroundImage using a token preserving the exact gradient.

## Evidence
Actual authenticated job data with candidate assets advanced463→467/1000 and visible width46→47% without reload. Build/typecheck/size,8workbench tests,native build/help pass. PR24 main ae9d628374bc31d180dfa9c124b9a36611a152ef deployed only after no active leases remained. Production asset/native fingerprints and readiness pass. Isolated GET/SSE replay with deployed assets verified377→600/1000 and38→60% fill; no server writes.

## Incident boundary
The real export failed before rollout at750/1000 due to browser storage quota. No auto-retry, data deletion or unrelated quota fix. No full artboard/Safari acceptance. Temporary preview and browser tabs stopped. AgentWiki publish skipped.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
