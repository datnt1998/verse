# AI loop review — verified causes and corrections

## Intervention clarification loop

The intervention arbiter originally received prior user inputs and reasons but not its own replies. Short confirmations therefore lost their referent. Initial hotfix restored replies; independent review then found unsafe first-option inference, truncation of scope constraints, and history slicing by a counter that excluded failed records.

Corrections:
- Keep the bounded recent conversation with complete retained user input/reply; propagate `input_truncated` when the audit store itself truncated a very long input.
- Keep failed/unapplied decisions from severing earlier user scope. Describe audit operations as `planned_action`, not execution evidence.
- Preserve explicit choices such as “both” and chapter exclusions. Never infer the first option or whole-book scope from a generic confirmation.
- Empty queues and idle runtime do not prove completion.

Four real-model replay scenarios passed: long scope with chapter 1–6 exclusion; “both”; an unapplied dispatch followed by confirmation; and a completion question without execution evidence. The last correctly reported completion as unverified rather than claiming success. Regression tests cover failed audit records, long constraints, and truncation signals.

Live ports: `f94228c` (scope correction), followed by `21c7a69` (planning-context correction below). Main scope correction: `a58913a`. Both live updates passed their affected Go suites and `/` returned HTTP 200 after restart. No story text was edited directly and unreleased memory features were not ported. Previous live binaries remain `/tmp/noverse-live-e4b89e5.bak` and `/tmp/noverse-live-f94228c.bak` for rollback.

## New-book planning context loop

A real isolated server run stalled with repeated successful `novel_context(chapter=1)` calls and eventually reached the 15-turn limit. That call returns chapter-writing context, not the missing-foundation/progress view needed by the architect. Increasing limits would repeat the same wrong operation.

The tool schema now exposes an explicit nullable integer; architects request `chapter=null`. Legacy omitted-field calls still work. Both short/long planning prompts explain the distinction. After restarting the isolated server with this correction and resuming the unchanged request, the architect made 3 context calls, 2 `save_book`, 6 `save_foundation`, and 1 `audit_foundation` call, then handed off to the writer.

End-to-end evidence:
- Real server at isolated port 8796; new project/book created through authenticated HTTP APIs, not a mocked host.
- Same five-chapter request resumed after repair: `phase=complete`, completed chapters `[1,2,3,4,5]`, no pending rewrites/rechecks; automatic advance enabled.
- Five actual chapter files, 3,492 total words in final runtime snapshot.
- The single main thread carried `due_by=4` and resolved at chapter 4.
- Actual project UI rendered all five chapter titles and the resolved-thread label/chapter.
- This run did not schedule an editor review, so it is not evidence for the goal-checklist/recheck UI; those paths have separate eval/regression coverage.
- Browser screenshots repeatedly timed out even though rendered DOM was readable. Pixel-level review remains unverified; no screenshot is claimed.
- Temporary server/browser were stopped and isolated account/provider credential copies removed. Raw artifacts remain ignored under `workspace/evals/live-smoke-library/`.

## Additional verified loop and memory defects

- Fresh rewrite/polish commits skipped quote validation before freezing; now invalid quotes are rejected before pending/final/record mutation. Frozen historical payloads remain recoverable.
- Invalid planted-thread deadlines were discovered only after freezing a commit. Preflight now rejects `due_by <= chapter`; same-chapter duplicate plants obey the same bound.
- Recheck results legitimately switching between rewrite and polish could be rejected after saving the review. The atomic store operation now allows only validated queued rechecks to make that transition, preserving unrelated work. A real temporary-store smoke exercised both directions and observed rewrite queue `[1]`, rechecks `[]`.
- Graph context deleted selected thread reminders and could reintroduce abandoned/future state; it now rebuilds as-of-chapter reminders, hides later closure details, and retains a single overdue reminder. Import continuity consumes abandonment and never reopens closed IDs on a later advance.
- Passage extraction now enforces the 700-rune cap without cutting quote sentences, skips unusable evidence before reserving slots, coalesces bridging windows, and reports only quotes still present after trimming.
- Scene heading detection distinguishes standalone `CẢNH 3` / `CẢNH 3 — …` labels from a legitimate prose sentence beginning `CẢNH 3 của vở kịch…`.

These are cause-aligned repairs, not retry-limit increases or warning suppression. Paid final evaluation was already running when the last deterministic boundary fixes landed; its report must identify that binary snapshot separately from the final regression-tested source.
