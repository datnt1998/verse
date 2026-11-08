---
title: Channel video list restored and deployed
date: 2026-11-08
summary: Restore channel-scoped Video tab on desktop and phone; verify eight production row destinations.
---

# Channel video list restored and deployed

## Delivery
User explicitly requested channel video lists after the compact workflow removed that tab. Recorded Q&A96, added Video as default tab on desktop/phone and retained other tabs. Reused existing channel-scoped API/cache and native Bàn dựng links; no backend schema, paid work or copied videos. Removed cross-key stale job rows to prevent wrong-channel links.

## Verification
53channel/router tests passed; final2new regression tests passed. Typecheck/build/size/nativehelp passed. Main 1491dddb1ba653841feaba1c1f025f9cb797ae2e deployed with safe backup/restored-DB rehearsal, schema41unchanged. Production shows4real ready videos at1440x900/390x844; all4row links per viewport load video content,8checks/0unresponsive; existing mobile tabs route correctly. Entry/native fingerprints match.

## Debugging lesson
Candidate-origin raw request interception ended before later lazy requests; the persistent test profile cached404responses for not-yet-deployed chunks. Public assets returned200while that profile returned404. Network-cache-disabled verification resolved all8destinations. Read-only counsel confirmed no anchor defect; no retry/preload workaround added. Existing unrelated warning remains visible.

## Boundaries
Artboards unchanged; requested IA exception is not full parity or physical Safari evidence. No user job was started, retried or deleted for the feature. Local preview and owned browser tabs/process were released. AgentWiki publish skipped.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
