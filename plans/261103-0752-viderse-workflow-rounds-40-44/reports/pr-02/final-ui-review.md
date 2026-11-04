# PR2 · Final read-only UI review (viewer, mobile preview, review selection, kit confirm/dialog)

Status: **DONE_WITH_CONCERNS** · reviewer: `V36UiReview` · date 2026-11-04
Scope: final V36/PR2 changes only. Read-only; no source edits, no index/worktree mutation (only `git show`/read-only `git diff`), no build/lint/test/format ran by me.

## Method / evidence

Read in full or in the cited ranges (worktree = current PR2 state):

- `apps/viderse/web/src/components/image-viewer/{ImageViewer.tsx,image-viewer.stylex.ts,types.ts}`
- `apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx`, `screens/review/ReviewScreen.tsx`
- `apps/viderse/web/src/screens/library/PromptEditor.tsx`, `screens/library/template/CoverSection.tsx`
- `platform/ui/src/{confirm-store.ts,ConfirmDialog.tsx,Dialog.tsx,Sheet.tsx,Overlays.tsx,Button.tsx,useAsyncAction.ts}`
- `platform/ui/src/mobile/{keepAlive.tsx,page.tsx}`, `apps/viderse/web/src/{router.ts,App.tsx,shell/MobileShell.tsx,shell/MobileRouteStage.tsx,screens/RouteScreen.tsx}`
- `screens/workbench/workbench-logic.ts` + `.test.ts`, `WorkbenchDesktop.tsx`, `LiveStudio.tsx` (callers), backend `internal/domain/clone_view.go`, `internal/pipeline/steps/export/handler.go`
- PR2 delta vs merged baseline `a7341d37db67e7cfafdb8035818f700820a11594` (`git show`/`git diff` for tracked paths only; staged-deleted/untracked duplicates in this checkout were compared by content checksum, not by index).
- Parent-provided runtime evidence (not re-run by me): `reports/pr-02/real-source-completed.json`, `real-source-export-probe.json`, `runs/21-real-cover-native-trial/native-cover-ready.json`, `runs/22-real-cover-preview-parity` (4/4 desktop+mobile, normal+reduced), frontend final-quality 3719 pass / 4 skip, build passed; QA access removed and both paid images consumed.

Verdict: the viewer core (stable-ID selection, ←/→ skipping unwoven cells, Esc/scrim/× close, focus return, retry) and the hidden-form/one-aria-modal mobile preview hold up in code. I found **one high-severity mobile navigation/target bug** in the new URL-driven review selection, one medium stale-target regression, one medium latent layering gap (confirm above viewer), three low-severity gaps, and one coverage gap in the new `jobStage` fix.

---

## Findings

### F1 — HIGH · Mobile Review keeps a stale `useRoute()` inside a keep-alive pane: tab re-entry resets the selection and deep links can open the wrong job

Files/lines
- `apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx:49-51` (`useRoute()` inside the screen), `:70-75` (`address?.video`, `address?.img` → selection).
- `apps/viderse/web/src/screens/review/ReviewScreen.tsx:53-55` (`ReviewScreen` takes no route prop and mounts `MReviewScreen`).
- `apps/viderse/web/src/screens/RouteScreen.tsx:43` (route is passed down; `ReviewScreen` drops it).
- `apps/viderse/web/src/shell/MobileRouteStage.tsx:24-32,65-82` (review is a kept pane; `keepPane` updates the active pane's route and `renderScreen(pane.route)` supplies the correct route).
- `platform/ui/src/mobile/keepAlive.tsx:14-16` — `<Activity mode="hidden">`; kit test `keepAlive.test.tsx:31` ("ẩn rồi hiện lại: state còn nguyên, effect dừng khi ẩn và chạy lại khi hiện") asserts exactly the contract F1 depends on: state survives, effects are unmounted and re-mounted.
- `apps/viderse/web/src/router.ts:203-210` — `useRoute` state is only updated by `hashchange`; the effect has no mount-time resync.
- `apps/viderse/web/src/shell/MobileShell.tsx:25,44` — Review tab href is the static `paths.review()` (no `video`/`img`).

Mechanism
1. Review pane is visible, hash `#/review?video=A&img=3`.
2. User leaves Review (tab or pushed screen). The `hashchange` fires while the pane is still visible, so its `useRoute` state becomes the away route (e.g. `#/`); then React hides the pane and **detaches its effects** (including the `hashchange` listener) while preserving state.
3. Next activation happens through a `hashchange` dispatched while the pane is hidden (already detached), so the pane never sees it. On show, `MReviewScreen` renders with the stale hash (`#/`), so `address` is `undefined` → `job = jobs[0]`, `requestedIndex = 0`.

Reproduction A (tab return, every time after the first Review visit)
- 390×844 → Review → pick job/image 3 → bottom tab **Bàn dựng** → bottom tab **Duyệt**.
- Expected from this code (not run by me): Review opens on the **first job, first image**, URL `#/review`. Pre-PR2 the screen kept `jobId`/`index` in state, so the selection survived.

Reproduction B (deep link lands on the wrong job, then a wrong-target mutation is reachable)
- Visit Review once; go to Bàn dựng; open a reviewable video B → `Duyệt ảnh` (`apps/viderse/web/src/screens/workbench/mobile/MobileWorkbenchDetail.tsx:176` navigates `review?video=B`) or the image sheet link (`MobileImageSheet.tsx:81`).
- URL is `#/review?video=B` but the pane renders `jobs[0]` (some other video) with `img` absent. If the user then presses **Duyệt cả video**, `approve()` (`MReviewScreen.tsx:212-217`) uses `job.id` = `jobs[0]` → the wrong job is approved while the address claims B.
- Same path breaks direct/reload-style entries when the pane already exists; notification/recap entries (`screens/workbench/mobile/MobileRecap.tsx:77`) are bare and unaffected.

Why this is PR2 and not pre-existing
- PR2 moved `MReviewScreen` from `useState(() => search().get('video'))`+`index` to `useRoute()` (`git diff a7341… -- MReviewScreen.tsx`). That is correct for the desktop (not in an `Activity`) but wrong inside a kept mobile pane: it uses a second, pane-local copy of the route that goes stale whenever the pane is hidden, instead of the route `MobileRouteStage` already hands to `RouteScreen`.

Suggested fix (one of)
- Make `ReviewScreen`/`MReviewScreen` accept the route prop that `RouteScreen` already provides and use it for `address` (do not call `useRoute()` inside the pane); or
- Key/remount the review pane on `routePath(route)`; or
- Have the tab bar navigate to the pane's stored route and resync `useRoute` on pane activation (`useMobilePaneActive`) — weakest (one frame stale).

Acceptance gap: phase-03 item 1a ("direct link, reload, notification entry, Back/Esc … right image/context; missing ID → respond and pick a valid fallback, do not mutate the first image"), item 3 ("open the right image from Bàn dựng→Duyệt and keep the right job"), and the Critical row ("không gửi sửa nhầm target") / High row ("Click ảnh thứ3, direct img link/reload/Back/Esc … không reset mặc định ảnh0").

### F2 — MEDIUM · A missing/stale `video` param no longer falls back; shows "Đã duyệt hết" while other review jobs exist

Files/lines
- `apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx:70` and `:238-244` (`if (!job)` → empty branch).
- `apps/viderse/web/src/screens/review/ReviewScreen.tsx:84` and `:355` (same pattern on desktop).
- PR2 delta: baseline had `jobs.find(j => j.id === jobId) ?? jobs[0]` (both screens).

Reproduction
- Open `#/review?video=B` (or approve/auto-approve/redraw video B from another surface so it leaves the review group). After `review.jobs` refreshes, `B` is gone but `A`/`C` are still waiting. `job` becomes `undefined`, so the screen renders the "Đã duyệt hết" empty state with no toast/redirect. Before PR2 it fell back to `jobs[0]`.
- The image-id fallback already exists (`MReviewScreen.tsx:80-84`, `ReviewScreen.tsx:103-106`); the video-id parameter has no equivalent.

Suggested fix: once `review.jobs` and the images request have settled, if `jobId` is set but not found → `toast.err` + `navigate(paths.review({ channel }), true)` (or restore `?? jobs[0]`).

### F3 — MEDIUM (latent, spec gap) · A confirmation cannot actually sit above the shared viewer: Dialog's Tab trap and the viewer's global ←/→ fight the higher layer

Files/lines
- `platform/ui/src/Dialog.tsx:148-158` — document-level capture Tab trap: when focus is outside the Dialog (`outside = !node.contains(document.activeElement)`), forward Tab `preventDefault()` + focuses the Dialog's first control; `openDialogs` only knows about `Dialog`s (`ConfirmDialog` is a separate root, not registered).
- `platform/ui/src/ConfirmDialog.tsx:330` — portal to `document.body`; scrim `z-index:100` (`ConfirmDialog.tsx:38`), Dialog overlay `zIndex:40` (`Dialog.tsx:16`).
- `apps/viderse/web/src/components/image-viewer/ImageViewer.tsx:64-77` — window-level ←/→ with no top-layer/escape-layer check.
- Design `04-components.md:381-390` ("lớp này nằm trên panel, dưới hộp thoại xác nhận") and phase-03 line 38 ("Confirm đặt trên viewer theo spec … chỉ có một lớp nhận tương tác").

Reproduction (the spec-required arrangement)
- Mount `ImageViewer` and open any `confirm()` from `side`/`footer` while it stays mounted.
- Tab on the confirm's primary button → the viewer's document trap sees `outside` and moves focus to the viewer's × **behind** the confirm scrim; every subsequent Tab walks the viewer behind the confirm (the confirm's own trap never retains focus).
- ←/→ while the confirm is up still changes the viewer's selected image behind the confirm, so the confirm text/action can refer to a different image than the one selected when it opened.

Current reachability: all shipped viewer confirm entrypoints avoid it by closing the viewer first — `PromptEditor.tsx:402`, `CoverSection.tsx:298`, `SceneImagePanel.tsx:66-79` — so this is not a live user bug today, but it is precisely the arrangement PR2 was asked to support, and any new confirm in `side`/`footer` will hit it. Recommend fixing the kit trap (scope Tab to the topmost overlay, e.g. a shared modality/layer registry) and gating the viewer's key handler on "no higher layer". Do not add a confirm to the viewer side/footer before that.

### F4 — LOW · CoverSection mobile sample preview has no Esc layer, unlike the PromptEditor preview

Files/lines
- `apps/viderse/web/src/screens/library/PromptEditor.tsx:92` registers `useEscapeLayer(phone && viewing !== null, closeSamplePreview)`.
- `apps/viderse/web/src/screens/library/template/CoverSection.tsx:57-59,125-129,300-309` only wire the back button; `useEscapeLayer` is not even imported.
- `MTemplateScreen.tsx:158-175` is a `MobilePage`; `platform/ui/src/mobile/page.tsx`/`MobileShell` register no Esc handler for it.

Reproduction
- ≤767 px → Template → Ảnh bìa → open a sample → **Esc** → nothing happens (PromptEditor's equivalent returns to the form). If CoverSection is ever rendered under a Sheet/Dialog (e.g. a future template sheet), Esc would close that outer layer and discard the preview instead of returning to the form.
- Design #81 / PromptEditor's own comment ("Quay lại (hoặc Esc) giữ bản nháp, không đóng tấm chỉnh sửa") is therefore only half implemented. The reported mobile preview parity 4/4 does not state Esc as covered; this is a code-level gap, not a contradiction of that run.

Suggested fix: mirror `useEscapeLayer(previewing, closeSample)`.

### F5 — LOW · Cancelling "Vẽ thử" from the sample viewer loses the sample view (no restore on reject)

Files/lines
- `apps/viderse/web/src/screens/library/PromptEditor.tsx:402` and `screens/library/template/CoverSection.tsx:298`: `const sample = viewing; flushSync(() => setViewing(null)); void askSample/draw(...)`; on `accepted === false` nothing reopens the viewer.
- Existing convention: `apps/viderse/web/src/editor/v13/SceneImagePanel.tsx:66-80` captures `returnTo`, closes, awaits `confirm`, and `else if (returnTo) setViewing(returnTo)`.

Reproduction: desktop sample viewer → **Vẽ thử** → **Huỷ** → the viewer stays closed and the user is back in the editor; they must re-open the sample. This is new in PR2 (the old sample dialog had no footer action).

Suggested fix: keep the sample and reopen the viewer when `askSample`/`draw` is rejected or returns early (also covers the locked-reason path, which is click-blocked today by `ActionButton`/`Button` `aria-disabled`).

### F6 — LOW · `confirm()`'s new lazy chunk failure is unhandled and silent

Files/lines
- `platform/ui/src/confirm-store.ts:30`: `Promise.all([import('react-dom/client'), import('./ConfirmDialog')])` rejects if either chunk fetch fails.
- Callers `PromptEditor.tsx:105-113` (`askSample`) and `CoverSection.tsx:78-84` (`draw`) `await confirm(...)` without `try/catch`, and are invoked via `void askSample/draw`; no pending state is set before the confirm opens.
- PR2 added the second dynamic import (`ConfirmDialog` was previously reachable through the already-loaded boot graph), so this is a new fetch point at click time.

Reproduction: production build after a deploy (old tab / new asset hash) or offline-after-boot → click **Vẽ thử** → chunk 404/fetch error → no dialog, no toast, no disabled state; only an unhandled rejection in the console.

Suggested fix: `try/catch` around `confirm` (or in each caller) with an error toast; optionally retry once.

### F7 — INFO/COVERAGE · Reviewed the new `jobStage` attention fix: correct for the reported repro; one same-cause path remains `issue`

Files/lines
- `apps/viderse/web/src/screens/workbench/workbench-logic.ts:50-62` (new `steps` scan; quality only when the first failed/waiting step is `score` + `quality_gate`), tests `workbench-logic.test.ts:43-56`.
- `apps/viderse/internal/pipeline/steps/export/handler.go:38-41` — the **export** step can also fail with `quality_gate` (same user-facing "hoặc bấm Vẫn xuất" message).
- `apps/viderse/web/src/screens/workbench/WorkbenchDesktop.tsx:242-244` — "Vẫn xuất" / "Sửa trong Biên tập" render only for `stage === 'quality'`.
- `internal/domain/clone_view.go:444-456` — `attentionSubline` picks the first failed/waiting step with a message, matching the new scan order, so banner and actions stay aligned.

Verified semantics
- Reported repro (`attention`, `activeStep: null`, score failed `quality_gate`) → `quality` (previously `issue`, hiding "Vẫn xuất").
- `score` waiting/`tool_missing` → `issue` (matches the added regression).
- Any other failed step (e.g. `images`/`provider_auth`) → `issue`.
- Minimal summary without `steps` (or an empty array) preserves `activeStep === 'score' ? 'quality' : 'issue'`.

Remaining gap (not proven reachable in a real run)
- If the first blocking step is `export` with `quality_gate`, `jobStage` returns `issue` (condition is `blocked.step === 'score'`), so the same "Vẫn xuất" recourse is hidden and the banner drops to the generic issue line — while the backend error text tells the user to press Vẫn xuất. Reachability depends on the export gate re-checking after acceptance (`export/handler.go:38-41`); worth either covering `step === 'export' && code === 'quality_gate'` too, or recording why it is unreachable.
- No test asserts the export+`quality_gate` case or the no-`steps`/`activeStep: 'score'` fallback (the fallback is only preserved implicitly).

---

## Verified-OK (no finding)

- **Hidden-form preservation / one aria-modal (mobile sample preview).** PromptEditor keeps `.vlb-scroll` and the footer mounted and hides them with `display:none` (`PromptEditor.tsx:180,379,487`); focus moves to the preview back button (`:94`) and returns to the originating tile after `flushSync` (`:87-91`). The Sheet's Tab trap now filters zero-rect / `[hidden]`/`[inert]`/`visibility:hidden` controls (`Sheet.tsx:92-93`), so the hidden form is skipped and Tab cannot escape the only visible preview control. CoverSection does the same for its four cards (`CoverSection.tsx:334`) and restores focus (`:125-129`). No nested `aria-modal`; the Sheet remains the only modal.
- **Double writes.** Review approve/skip/redraw/version are guarded by the per-job lock in `useReview.ts:186` plus immediate `pending` (`:187`) and the `ActionButton` `aria-disabled` click block (`Button.tsx:109-110,142`); the new version path uses `useAsyncAction` (`MReviewScreen.tsx:43-49`), whose `run` is single-flight (`useAsyncAction.ts:25`), and matches the desktop `useImageVersion` action (`ReviewScreen.tsx:277-282`). The viewer footer can't double-open a confirm because the first click `flushSync`-unmounts the viewer before the second click of a double-click can dispatch.
- **Session confirmation cancellation / lazy init.** `confirm-store.ts:19-30` bumps a generation and dismisses active dialogs; the remembered path re-checks the generation before and after `onConfirm`; the dynamic-import path re-checks before creating the root. Cancelling a confirm resolves `false` before `trial.start`, so no mutation is sent, and `askSample`/`draw` return on `!accepted` (`PromptEditor.tsx:112`, `CoverSection.tsx:83`). No race between the generation check and `activeConfirmations.add` (same synchronous turn).
- **ImageViewer core.** Selection is by stable id (`types.ts` contract; `onSelect(id)`); ←/→ skip disabled/unwoven cells and are no-ops when no other viewable cell exists (`ImageViewer.tsx:55-63`); the strip only follows focus when the strip already owned it (`:151-161`); the scrim/×/Esc all route through `Dialog` with focus return; image load failure has a retry (`:117-139`); Dialog body `padBody={false}` + the frame grid give the expected header/stage/footer layout. `Dialog`'s new center-padding close (`Dialog.tsx:192`) is additive to the existing scrim close and no caller depended on the dead zone.
- **`jobStage` regression tests** (`workbench-logic.test.ts:43-56`) cover the three stated cause groups; the function remains total for missing/empty `steps`.

## Remaining unexercised concerns

- I did not run the app; F1's timeline follows the kit's `Activity` contract and its own test, but it has **not** been reproduced in the embedded browser by me. A 390×844 run of "Review → pick image → Workbench tab → Review tab" and of "Visit Review, then Bàn dựng → Duyệt on video B" would settle it in minutes and should be part of the real smoke before merge.
- CoverSection mobile sample history is still pending per the PR context; F4/F5 are code-level gaps that a 4/4 preview parity run does not necessarily cover (Esc/cancel restore).
- Cold-phone ≤200 KiB and Explore latency gates stay deferred per #83; I made no size measurement. Note `platform/ui/src/index.ts:18` still statically re-exports `ConfirmDialog`, so the dynamic import only produces a separate chunk if the barrel is tree-shaken by the app build — verify in the deferred gate rather than assuming the split.
- The embedded-template smoke's expected setup redirect (reported by the parent) was not investigated here.
- No iPhone/Safari real-device evidence exists in this review.

## Parent disposition after remediation

The findings above retain the source and evidence available to the original review. They are not silently rewritten as passes.

| Finding | Current disposition and evidence |
|---|---|
| F1 | Reproduced with two recorded-source QA replays: URL targeted B while the mobile pane displayed A. `RouteScreen` now passes its authoritative pane route through `ReviewScreen` to `MReviewScreen`; the bottom Review tab preserves the last full image address. No wrong approval was submitted. Native run28 checks tab return and Workbench→B at both motion settings. |
| F2 | Desktop and mobile now give explicit feedback for a stale video address, replace that address and show a valid waiting job. Native run28 checks this without mutations. |
| F3 | No shipped caller keeps the viewer mounted beneath a confirmation: Prompt/Cover/scene replacement close first. Prompt's desktop editor is a nonmodal section, not an enclosing Dialog; phone samples stay on the existing Sheet. Run28 checks viewer absent while confirmation is active, repeated Tab/Shift+Tab stays in it, and cancellation restores the same sample and focus. No second modal registry was added for an arrangement the actual callers do not create. |
| F4 | Cover mobile now registers the existing `useEscapeLayer` and returns to its settings/trigger. Both mobile motion projects pass run28. |
| F5 | Prompt/Cover capture the sample before confirmation and restore it on cancellation only while still mounted and still present in history. Run28 covers Huỷ, Esc and scrim, no API writes, same image URL and restored focus. |
| F6 | Lazy-module rejection fails closed with a Vietnamese recovery toast; stale-session cancellation stays silent. Kit261/261 passed. Production run33 passes4/4: fresh Account route has0pre-click shared-controls requests, the real dynamic JS request is aborted, toast appears,0logout and live session remain; native “Tải lại” recovers and confirmation Cancel keeps the session. Run32 retains two normal-motion harness failures: generic screenshot settling waited out the required4s toast. Capture timing was fixed, not the product lifetime. |
| F7 | Classification now includes `export` + `quality_gate`, but not unrelated export/tool/provider errors. Focused Review/mobile/quality suite28/28 passed; the actual original QA source exported to a decoded H.264/AAC MP4. |

Run28: `runs/28-review-address-and-guarded-native-cancel/interaction-results.json`, 14 passed /10 explicitly inapplicable project-state skips. These are current Vite-source checks, not the older embedded binary.

Logout follow-through: `SessionProvider.signOut` now confirms the shared Noverse/Viderse consequence before revocation; Cancel keeps the session. Existing consumer contracts were migrated rather than bypassing confirmation. Focused App/session/access-denied41/41 passed. Embedded run30 passes4/4 desktop/mobile normal/reduced with live `/api/me` retained and no logout request. Desktop exits incomplete setup through the native “Cài đặt nâng cao” link; the browser blocks and records automatic setup-start requests instead of falsifying setup state.

Quota run31 passes4/4 on the embedded build: actual daily cap2 and exactly2existing paid images yield0/2, real API reports exhausted quota, drawing is locked, force-click sends0APIwrites and no confirmation, and the existing saved cover still decodes/opens/closes with focus restored. Read-only temporary provider access is removed in the runner's `finally`; no third image request or synthetic ledger was used. See `runs/31-real-exhausted-quota/interaction-results.json`.
