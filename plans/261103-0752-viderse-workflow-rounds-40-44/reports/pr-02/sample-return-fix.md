# PR2 · Sample viewer return fix (F4 + F5)

Status: **DONE** · worker `SampleReturnFix` · date 2026-11-04
Scope: only the two owned sources below + this report. No provider/draw/trial-start call, no browser, no build/lint/test/format run (parent runs the single post-integration gate).

## Sources changed

### 1. `apps/viderse/web/src/screens/library/template/CoverSection.tsx` — F4 (mobile preview Esc)

- Import `useEscapeLayer` from `@noverse/ui`.
- `useEscapeLayer(previewing, closeSample)` where `previewing = phone && viewing !== null`.
  - `closeSample` already does `flushSync(() => setViewing(null))` then focuses `triggerRef` (the exact sample tile that was pressed, captured in `openSample`).
  - Esc now returns to the cover settings (the three cards are un-hidden because `previewing` becomes false) and restores focus to the originating tile.
  - The layer is registered after any outer layer (mobile template page / Sheet registers on mount), and `onEscapeKey` runs in the capture phase with `preventDefault()` + `stopPropagation()`, so Esc no longer falls through to an outer layer/route — the parent surface is not closed.
  - Desktop is untouched: `previewing` is false there, and the desktop modal viewer (`ImageViewer` → `Dialog`) handles its own Esc as before.

### 2. `apps/viderse/web/src/screens/library/PromptEditor.tsx` — F5 (desktop sample viewer → Vẽ thử → Huỷ)

- `askSample(...)` now returns `Promise<boolean>`: `false` for the early-return paths (`!status`, no video, `trialLockedReason`) and for `confirm()` resolving `false`; `true` only after the accepted branch has awaited `trial.start(...)`. Both existing callers (sidebar `Vẽ thử`, failed-sample retry) discard the value with `void`, unchanged behaviour.
- New `trySampleFromViewer(sample)`:
  1. `flushSync(() => setViewing(null))` — viewer closes **before** the confirmation opens (no stacked dialogs; same convention as `SceneImagePanel.tsx:66-80`).
  2. `askSample(...)` runs once; on `false` and while the editor is still mounted, the viewer is reopened on the **same sample id**, using the latest row from `historyRef` (newest `trial.status.history`) so a background refresh during the confirm cannot restore stale row data.
  3. Accepted (`true`) keeps the viewer closed, and `trial.start` has been invoked exactly once.
- Viewer footer now calls `trySampleFromViewer(viewing)` instead of the inline close+`void askSample` snippet.
- `mounted` ref (`useRef(true)` + effect set-on-mount / clear-on-cleanup, same pattern as `AutonomyDialog.tsx:31`): a confirmation that resolves after the editor unmounted (sheet closed, route change, prompt switch — both mount sites key `PromptEditor` by prompt id) never reopens UI.
- Stale sample: if the cancelled sample has left the last-12 history during the confirm, reopen is skipped instead of showing an "image no longer in set" viewer.
- Cancelling through the confirm's Esc, scrim, or Huỷ all resolve `false` in `confirm-store` and therefore all restore; a dismissed session confirmation (`dismissConfirmations`) also resolves `false`, and the unmount guard suppresses the restore when that dismissal accompanies a navigation.

Preserved behaviour: ready/blocked/pending/trial lock reasons unchanged; the footer `lockedReason` gating (`viewing.promptKind !== kind ? copy.tryKindChanged : trialLockedReason`) is untouched, so the restore path is only observable when a click was actually possible (plus the rare render/click race where the lock appears between render and click — then no dialog showed and the viewer returns, which is the intended "could not ask" outcome).

One nuance for the parent: the reopened viewer is a fresh `Dialog`, so focus enters the viewer through the kit's normal first-focusable rule (close ×) rather than returning to the footer "Vẽ thử" button — identical to the `SceneImagePanel` convention. Context (selected sample, position in the strip, side prompt panel) is preserved because the same sample id is reselected. The pre-confirm focus restore inside `Dialog` returns to the originating tile before the confirm opens, and the confirm's cancel restores focus there too, so no focus is lost.

### 3. `apps/viderse/web/src/screens/library/template/CoverSection.tsx` — F5 (same convention for cover samples)

- `draw(...)` returns `Promise<boolean>` (`false` on the early locked/`!status` return and on cancel, `true` after `trial.start`).
- `drawFromViewer(sample)` mirrors `trySampleFromViewer`: close viewer → `draw` once → on `false` while mounted, reopen the same `CoverTrial` id from the latest `historyRef`; accepted stays closed and starts once.
- `drawSelected` / `retry` swallow the boolean as before.
- Same `mounted` guard and same "row left the 12-item history → do not reopen" rule.

## Not done (by instruction)

- No test added: there is no existing test file for `PromptEditor`, `CoverSection`, or their screens (`grep -rl "CoverSection\|PromptEditor" --include='*.test.*'` over `apps/viderse/web` → none), and synthetic wiring tests were explicitly out of scope.
- No kit change for the latent F3 top-layer/Tab-trap gap; all shipped viewer footer confirmations still close the viewer first, so the unsupported arrangement is not introduced.
- No change to the parent route F1/F2 work, mobile share/viewer, or backend.

## Browser scenarios for the parent (not run here)

Baseline: real cover history/source exists for the parent smoke; everything below is expected to produce **zero** paid POSTs unless a step explicitly accepts.

1. **F4 mobile Esc — cover section** (390×844, reduced motion and normal):
   - Template → **Ảnh bìa** → tap a ready sample tile (one of the four cards).
   - Press **Esc**. Expected: preview gone, the three cover cards visible again, page/sheet still open (not navigated back), and focus is on the same tile (visible focus ring; Enter reopens the same sample).
   - Repeat with twice-Esc: the second Esc is a no-op for the cover section (the preview layer is unregistered).
2. **F4 parity — prompt editor** (same phone width): prompt editor → sample → Esc still returns to the form with the draft intact (regression check only).
3. **F5 desktop cancel restore** (≥1024 px, library prompt editor or template Ảnh bìa):
   - Open a ready sample → footer **Vẽ thử** → confirm dialog appears with the viewer gone (one modal).
   - Cancel with **Huỷ**, then repeat and cancel with **Esc**, then with a scrim click. Each time: the viewer reopens on the same sample, same strip position, same side prompt, focus inside the viewer; no toast, no request.
   - After the restore, close the viewer with Esc/× → back in the form; open the same sample again → still the same image.
4. **F5 accepted path**:
   - **Vẽ thử** → confirm → viewer stays closed (no restore), sidebar/footer shows the running state after the refresh, and the network log shows exactly **one** POST (prompt samples or cover trials) for that click. No second POST while the trial runs.
   - Only do this after the parent decides to spend the remaining budget; images are already consumed per the run context, so this step is expected to be observed via a zero-cost path or skipped.
5. **F5 stale/unmounted safety** (no paid call): open a sample → **Vẽ thử** → while the confirm is up, press the browser Back (or close the mobile sheet / switch tab) → dismiss the confirm. Expected: no viewer flashes back, no console errors, no request.
6. **Regression**: blocked/locked states still show no confirm (locked buttons remain `aria-disabled`); mobile preview still keeps the draft and has a single `aria-modal`; desktop viewer ←/→ strip selection still works before and after a cancelled Vẽ thử.

## Checks

None run here (per instruction). Parent should run the single post-integration full-quality gate plus the browser scenarios above.
