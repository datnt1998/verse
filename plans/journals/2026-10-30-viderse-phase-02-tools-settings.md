---
title: "Viderse rebuild: phase 02 tools, connections, settings"
date: 2026-10-30
summary: "Settings (6 sections) and the setup step ported; five independent artboard QA rounds down to 0 majors / 0 minors; keychain prompt and slow-shutdown bugs found by smoke"
---

# Viderse rebuild: phase 02 tools, connections, settings

## What happened
- Backend packages added: aiconn, secretbox, accounts, platforms, storage, retention, fsio, activity, toolsvc, setup, logs; migrations 00003 (AI connections), 00004 (platforms), 00005 (machine tasks).
- Web: the six Settings sections plus the `#/setup` step, desktop and phone, all against the `Settings`, `MSettings` and `Onboard` artboards.
- Kit additions (opt-in, Noverse unchanged): Button `compact` (32), `lg` (44), `xl` (48), `warnGhost`; Banner `icon`/`align`/`role`/`actions`; Sheet `size="md"`; ConfirmDialog `size`; icon `scan-line`.
- Code review rounds per slice (secrets, machine, web), each finding fixed with a regression test.
- Artboard fidelity: screenshot pairs (artboard + app) for every tweak state at 1440×900 and 390×844, judged by independent reviewers who built their own binary, DB and Chrome.

## QA rounds (majors / minors)
| Area | R1 | R2 | R3 | R4 | R5 |
|---|---|---|---|---|---|
| Machine | 30 / 100 | 8 / 32 | 2 / 4 | 0 / 1 | 0 / 0 |
| Connect | 33 / 89 | 8 / 29 | 1 / 6 | 0 / 1 | 0 / 0 |
| Storage | 37 / 139 | 0 / 27 groups | 1 / 11 | 0 / 1 | 0 / 0 |
| Accounts | 18 / 53 | 17 / 53 | 1 / 5 | 0 / 2 | 0 / 0 |

## Problems found and fixed
- macOS keychain prompts: the app's Chrome probes launched without `--use-mock-keychain`. Every app-launched headless Chrome now goes through one argument builder; a test pins the flags.
- Fresh desktop setup never auto-started (inverted guard) while the UI claimed "Đang cài".
- Storage destination check raced: a stale success could enable the move button for another drive.
- Installed sizes showed the free-space reservation (4,3 GB for transcription, measured 1,2 GB). Size is now measured once after install and persisted; GET never walks the tree.
- Codex from Homebrew was labelled "Đã cài · 300 MB"; now "Có sẵn" like system Chrome.
- Install phases without a known total rendered a frozen "0%" bar for minutes; now phase name only.
- Graceful stop with any browser tab open took the full 10 s and exited 1: `http.Server.Shutdown` waited for SSE streams that only end when the hub closes. The hub now closes at shutdown start.
- A setup test depended on the wall clock (ETA appeared after 09:00 UTC); Date is frozen in that suite.
- Flaky test made deterministic: the accounts busy-gate probe now waits on an `entered` channel instead of a TryLock poll (`-race -count=100` clean).

## Numbers (final smoke, built binary, fresh HOME, real install)
- GET p95 sequential: all 13 `/api/*` GETs ≤ 4,6 ms (`/api/tools/logs` 4,5; `/api/status` 3,9; rest < 1).
- Idle 60 s: CPU 0,13 %, RSS 21 MB, 0 child processes, SSE 0,5 frames/s (hello + 2 s ping).
- First-load JS 126,2 kB gzip; Settings sections are separate lazy chunks. DOM elements per Settings section on a fresh machine: desktop 135–230, phone 105–178.
- Keychain prompts during smoke: 0.

## Budget
- No paid calls in phase 02.

## Next steps
- Needs the owner: Grok device-code login, Bilibili QR scan with a real account (V24 ≤ 2 s), adding a real API key.
- Open design questions: 11-hoi-dap.md #6, #8–#13, #14–#23, #25–#30.
- Phase 03: scan scheduler, rivals API, Explore.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
