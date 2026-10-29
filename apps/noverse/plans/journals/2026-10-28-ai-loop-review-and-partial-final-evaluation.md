---
title: AI loop review and partial final evaluation
date: 2026-10-28
summary: Loop causes fixed at contract boundaries; held-out editor did not improve; quota blocked writer and smoke reruns
---

# AI loop review and partial final evaluation

## What happened
- User reported the intervention arbiter re-asking for scope. Root causes:
  - missing replies in the arbiter context;
  - truncated scope;
  - failed decisions severing history.
- Fixed in arbiter commits, live `f94228c`. A real follow-up ("Chương 1,2,3,4,5,8") was dispatched on the first try.
- Other loop sources found during review, each fixed at its contract boundary instead of raising limits:
  - architect `novel_context(chapter=1)` instead of foundation mode (fix: explicit `chapter=null`);
  - recheck mode switch rejected after saving the review;
  - invalid `due_by` discovered only after freeze;
  - prompt/validator mismatch for the final-chapter `hook_type` (`ket_truyen` vs `seed`).
- The baseline binary still shows the architect loop: 13 identical context calls in 40 seconds. HEAD does not.

## Evaluation
- Dev editor: 12/18 → 18/18 runs pass, false positives 8 → 0.
- Held-out editor: 7/18 → 7/18, critical hits 8/9 → 7/9, false positives 16 → 21. This is an overfit signal, so phase 08 stays open.
- Provider usage limit (HTTP 429) hit at about 11:16. All writer cases and the smoke reruns were blocked. HEAD results were regraded offline from artifacts; 48/48 outcomes matched the live run output.

## Lessons
- Contract mismatches between prompt and validator show up as slow "loops" with high-latency models. Each rejected 30 KB outline cost ~7 minutes.
- Long paid eval batches share quota with the user's live writing. Check quota headroom before launching a repeat-3 suite.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
