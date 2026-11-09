---
title: Viderse editor resize and opaque controls release
date: 2026-11-09
summary: "Deployed verified editor panel/group resizing, opaque controls and timeline repairs to Mac mini."
---

# Viderse editor resize and opaque controls release

## What happened
Viderse editor semantic utilities were absent, leaving portal menus transparent. The active shell lacked panel resize, canvas transformed only the first linked member, and timeline cue edits needed isolated move/trim. Integration exposed incorrect vector scale units, missing fontSize resolver fields and off-center-anchor group rotation. Review also caught trailing-edge snapping and selected-neighbor trim previews.

## Decision
Reuse canonical render/command/keyframe and react-resizable-panels contracts; preserve artboards and dirty primary checkout. Existing animation edits remain current-frame keyed, not whole-animation rewrites.

## Evidence
Full frontend3859passed/4skipped; final timeline20passed; final typecheck/build and native smoke passed. Real UI, saved data, decoded6s worker export and production10control smoke verified. Main7f90abe51faf55aeb6c2b532e6487dbe1d4ead94 deployed to Mac mini with backup/restore guard and matching binary SHA256; native/public readiness passed. Private QA project, adapter and owned processes cleaned.

## Limits
Full artboard matrix/physical Safari and dense or off-canvas handle layout are not certified; no claim all editor features are complete. AgentWiki publish skipped.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
