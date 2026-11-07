---
title: Mac mini reader restored and export timing deployed
date: 2026-11-07
summary: Restore missing Colima TTS forward; deploy measured export I/O optimization without quality-setting changes.
---

# Mac mini reader restored and export timing deployed

## What happened
Production native reader repeatedly exited because host 127.0.0.1:8081 refused connections while the container listener returned 401 correctly. Colima logs show forwarding recreation failed on November 5. Restored only the missing forward through the existing SSH control connection; no app/database restart. Authenticated production preview synthesized a new sentence: WAV307244bytes,3.2s/48kHz,decoded non-silent PCM,HTTP200 in1.4s. This is operational recovery, not an upstream retry fix.

## Export release
PR22 deployed main bd49e0f92b0c4a0d9d0998dc16a7306a6709c74d with safe backup/rehearsal. Native hash and served frontend match tested artifacts; readyz and authenticated1920x1080playback pass. Full-pipeline original acquisitions3→2; original decoded900frames/audio/coverage retained. No demonstrated whole-run speedup or rerun of57-minute production export. Stage/resource logs will be collected on next normal export.

## Boundaries
No auto-enqueued production export, no change to encoding settings, no full artboard/Safari acceptance. Colima recurrence needs VM maintenance evidence, not blind app restart. AgentWiki publish skipped.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
