# V40 workbench/review — bounded source review

Status: DONE_WITH_CONCERNS

## Scope and evidence

Read `workbench-review/reports/handoff.json`, all 10 mapped changed files, `parent/src/components/autonomy/ScoreParts.tsx`, and `parent/src/screens/workbench/jobActions.tsx`. Checked against accepted phase-07 (especially steps 2, 6, 7 and the 44px matrix), authoritative `01-ia-flows.md` §6.2 r44 / W6, Workflow sections 06–09, and the relevant MMain/MReview artboard sections. Read the shared Button/Sheet and existing request/image/backdrop contracts only to resolve concrete behavior below.

This is one bounded source review of the supplied scratch sources, not a claim of model independence or full-app acceptance. No source edits, build, lint, tests, formatters, browser/server, provider calls or API mutations were performed. Only this assigned report was written.

Paths below are web-relative unless explicitly prefixed with `platform/` or `internal/`; changed source is in `workbench-review/src`, and the two parent-owned files are in `parent/src`.

## Findings

### 1. P2 — New information-sheet actions are 30px, not 44px touch targets

**Affected source:** `screens/workbench/mobile/MobileWorkbenchDetail.tsx:308,317,321,325,329,331`; links at `307,313,334`; `screens/workbench/mobile/mworkbench.stylex.ts:76-80`.

The newly consolidated Thông tin video sheet renders transcript **Sao chép**, backdrop **Thử lại**, **Thêm clip**, **Đổi ngẫu nhiên tất cả**, per-clip **Đổi ngẫu nhiên / Bỏ clip**, and fixed-clip **Bỏ clip** with `Button size="sm"` and no size override. `platform/ui/src/Button.tsx:51,124` makes that size exactly 30px tall; it is not automatically promoted for mobile. The source/Explore/template/library links use `mw.moreLink`, whose minimum height is 36px. The kit's coarse-pointer rule is only for `.kit-pop .kit-option`, not these sheet controls (`apps/viderse/web/src/styles/viderse.css:454-457`). This confirms the parent's specific concern and misses phase-07's 44px gate.

**Proposed fix:** use the existing mobile `size="lg"` (44px) for every interactive button added to this sheet, and give its links a local minimum 44px hit area (both dimensions where relevant). Let action rows wrap rather than reducing height. Do not globally change desktop `sm` sizing.

The same gate is still unmet by controls in these reviewed mobile screens: detail content actions `MobileWorkbenchDetail.tsx:243-245` (30px), and review `MReviewScreen.tsx:294` / `mreview.stylex.ts:22` (Video sau 36×36), `MReviewScreen.tsx:362` / styles `:41` (compare 32px), `:374,406` (30px), and `:456` / styles `:141` (feedback chips explicitly 34px). These are the same touch-target issue, not separate findings; source artboard examples with compact visual controls do not satisfy the accepted implementation's explicit 44px matrix.

### 2. P2 — Legacy prompts are still presented as a fabricated Cảnh part

**Affected source:** `screens/review/mobile/MReviewScreen.tsx:175-178,477-485`; matching desktop display `screens/review/ImageRail.tsx:348-366`.

The mobile renderer filters `submittedVersion.parts` into the four known kinds, but does not exclude `submittedVersion.partsLegacy`. Its “Nguyên văn đã gửi” fallback only runs when the filtered array is empty. Real server legacy versions do **not** have empty `parts`: `internal/clone/images/versions.go:78-83` explicitly synthesizes one `scene` part containing the whole old prompt and marks `partsLegacy: true`; `versions.go:123-125` returns the original `sentText` and flag. The existing real server test `versions_test.go:366-390` documents this exact payload.

Consequently opening a saved legacy version displays the entire old prompt under **Cảnh · Bản cũ · chưa tách phần**, instead of showing it as raw text. Desktop has the same issue because it branches on `version.parts.length`, despite already computing `legacyParts` at `ImageRail.tsx:98`. This contradicts the handoff's explicit claim that legacy whole prompts are no longer labelled Cảnh. Editing is correctly disabled by the existing flag; display is not.

**Proposed fix:** only construct/display the four-part view when `!partsLegacy`; for legacy versions show `sentText` through the existing raw/“Nguyên văn đã gửi” fallback on both layouts. Preserve the original bytes and existing read-only restriction. Add a fixture with the actual server shape (`partsLegacy: true`, nonempty synthesized scene part), not only `parts: []`.

### 3. P2 — Feedback-chip removal can delete part of the owner's own sentence

**Affected source:** `screens/review/mobile/MReviewScreen.tsx:456`.

The newly reversible chips infer selection using `feedback.includes(suggestion)` and remove the first matching substring with `replace(suggestion, '')`. For example, after the owner types **“Tông lạnh hơn một chút nhưng giữ chiếc ô đỏ”**, the **Tông lạnh hơn** chip is already reported pressed. Tapping it deletes those words and leaves **“một chút nhưng giữ chiếc ô đỏ”**. This is not removing an independently inserted feedback sentence; it mutates the owner's existing sentence, which can then be submitted to the real redraw API.

**Proposed fix:** track chip additions separately, or only match/remove an exact delimited feedback sentence, preserving manually authored text and punctuation. Extend the new reversible-chip test with a manually typed sentence containing a suggestion as a substring; the present test at `MReviewScreen.test.tsx:114-128` only covers unrelated owner text.

## Additional integration note (not a claimed test execution)

`MReviewScreen.test.tsx:298` still expects `image.versions[0].prompt` (`“Góc máy rộng, chưa có mưa”`) after choosing the non-cancelled modern version 1. The fixture supplies its modern scene text as `“Cảnh 1 phần Cảnh: góc máy rộng.”` at line 27, and the modified renderer intentionally uses that modern text. This is a source-level assertion/fixture mismatch; update that assertion to the selected modern part (and add the real legacy fixture above). No test was run and no observed failure is claimed.

## Source-confirmed positive coverage / boundaries

- Detail has exactly Nội dung đăng / counted Ảnh tabs, content default and reset on job change (`MobileWorkbenchDetail.tsx:48,114,165-204`); info consolidates assistants/source/template/backdrops in one Sheet (`286-337`).
- Review-stage image taps navigate with the actual clicked image ID (`103-106,186`); other stages retain MobileImageSheet. No round45 viewer or mobile editor was added.
- The list folds up to three real suggestion rows into Chờ bạn with real accept/dismiss handlers (`MobileWorkbenchList.tsx:37-38,56-69`); the standalone proposal group is gone.
- Backdrop add/reroll/remove use the existing endpoint and revision and honor offline/build/busy locks (`MobileWorkbenchDetail.tsx:61-73,319-331`). Metadata, package copy, pause/resume/run, export playback and MP4 removal remain attached to real existing operations. No fake result/state source was found in these paths.
- Mobile redraw pins job/image/baseVersion and only submits the actual edited scene; incoming versions do not replace the pinned edit source (`MReviewScreen.tsx:57,69-79,148-155,247-267`). Compare chooses an earlier usable version with an image URL (`159`); choosing a version changes the prompt source. Four modern parts use shared canonical order (`imageView.ts:153-159`, `MReviewScreen.tsx:176-177`, `ImageRail.tsx:349`).
- Mobile prompt/feedback use one discriminated `sheet` state rather than concurrent open booleans (`53,439,474`). Shared Sheet keeps a closing layer mounted and inert for its exit animation (`platform/ui/src/Sheet.tsx:60-71,115`); this is not two simultaneously active sheets. Runtime transition/focus/scrim behavior remains a parent UI gate, not exercised here.
- Mobile Why/learned UI is removed from the reviewed live render paths; parent ScorePanel no longer accepts imageId or opens a ReasonPopover (`parent/.../ScoreParts.tsx:54-67`), and its changed desktop caller matches (`ReviewScreen.tsx:563`). The leftover unused `whyBtn` style is not a consumer-visible finding.
- New tab-entry motion uses existing duration/easing tokens and disables itself for reduced motion (`mworkbench.stylex.ts:299-303`). Other inspected motion/Sheet behavior is reused, not a new animation framework. Native prompt text is not translated.
- Parent restart/remove/stop still call the real preview/restart/delete/cancel operations and navigate after successful results (`parent/.../jobActions.tsx:11-49`). No source-level regression was identified in those two parent-owned files.

## Known blockers retained, not new findings

- Exact non-approving undo for Bỏ ảnh này remains the already identified API/product blocker. The worker retains a truthful success toast and does not secretly approve/redraw as an undo.
- Mobile tool-recheck location remains the separately identified product/design blocker, outside this workbench/review slice.

## Parent verification handoff

After integrating/fixing, run the focused mobile workbench/review tests plus desktop ImageRail/ReviewScreen and the shared ScorePanel caller typecheck/build. Exercise actual backdrop mutations, selected-version prompt/copy/edit, manually authored chip-substring feedback, and all information-sheet hit areas. Check only one active overlay during prompt→edit/overflow→confirmation transitions, normal/reduced motion, and the accepted real-Safari keyboard/safe-area gate. None of these checks were executed by this reviewer.

Summary: The supplied slice wires real actions and removes the requested Why surface without introducing a round45 viewer. Three concrete consumer-visible concerns remain: undersized mobile touch targets, legacy prompts falsely shown as a Cảnh part, and feedback-chip substring deletion; one additional stale test assertion needs parent correction.

Concerns: Findings 1–3 above; known unskip/tool-recheck blockers are unchanged.
