# Selected-version frontend — PR2 / V36 / Q84 option A

Status: Integrated and independently reviewed. Final Viderse suite3755 passed /4 skipped, focused app consumers58/58, editor25/25; both typechecks, production frontend build and four-context embedded native source proof passed. Not merged/deployed.

## Intent

Editing or adding references while viewing a non-used version (for example `usedVersion=1`, viewing `version=2`) keeps that version's saved prompt parts and refs. Only requested overrides are sent; the backend rebuilds the trusted provider/tool wrapper from the explicit base.

## Changed

- `apps/viderse/web/src/api/images.ts` — `RedrawBody.baseVersion?: number` with explicit-single-image semantics in the contract comment.
- `apps/viderse/web/src/screens/review/useImageEditing.ts` — draft source is captured when entering edit/raw-with-draft; passive SSE base changes no longer reset draft, refs, changed parts, or source. Review keys it by job plus image identity, so matching scene IDs from different jobs do not share drafts.
- `apps/viderse/web/src/screens/review/ReviewScreen.tsx` — desktop edit/feedback source is the displayed version, or the failed version while the failure card is current. Sends `baseVersion` on edit, feedback, and failed-Grok retry. Builds the request body before the confirm dialog so a later selection change cannot alter a pending request. Explicit version picking resets editing and then adopts the new version; passive SSE does not. ImageRail stays on the pinned source while editing.
- `apps/viderse/web/src/editor/v13/SceneImagePanel.tsx` — pins the displayed source and draft on entering edit, sends `baseVersion`, restores the picked version on cancelled confirmation, and resets editing on explicit version selection. Legacy full prompts cannot become editable scene text.
- `apps/viderse/web/src/screens/workbench/LiveStudio.tsx` — viewer feedback snapshots `viewerVersion.n` and sends `baseVersion`.
- `apps/viderse/web/src/screens/workbench/WorkbenchDesktop.tsx` — failed Grok retry sends `baseVersion: failedVersion.n` with its existing parts/refs flow.
- `apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx` — mobile one-image redraw now uses canonical `redrawImages` with `baseVersion`; no more `review.mutate` `regenerate`/`editPrompt`.
  - Raw full prompt remains read-only in the prompt sheet.
  - The existing one-scene edit field is populated from the modern `JobImage[].versions[].parts` scene part of the selected version, never from `sentText`/guard text.
  - Untouched parts and refs are preserved server-side via the base; only `parts.scene` is sent when editing.
  - Legacy versions without structured parts disable the edit field/action with an honest limitation message; feedback redraw remains available.
  - `sheetBase` snapshots job/image/version when the sheet opens. Passive SSE cannot switch the pending source; job/image navigation closes the old draft. An old request finishing cannot close or clear the new job's sheet.

Batch/all redraw remains unchanged: no single base is attached to multiple image IDs.

## Regression coverage

- `screens/review/ReviewScreen.test.tsx`
  - edits the viewed version (used=1, viewing/editing 2) with distinct v2 scene + v2 refs; Cancel keeps draft/refs; submit sends `baseVersion: 2`, no use-version write.
  - pending confirmation keeps the captured base after the viewed version changes.
  - passive SSE adding a new latest version / changing usedVersion does not reset draft or base; submit still sends `baseVersion: 2`.
  - existing expectations updated for explicit feedback/edit/Grok bases.
- `screens/review/mobile/MReviewScreen.test.tsx`
  - selected version raw prompt vs modern scene-part edit source (v1, used v2).
  - feedback submits `{ imageIds, baseVersion, feedback }` via canonical `/redraw`, no legacy `mutate`.
  - edit submits only `{ parts: { scene } }` plus `baseVersion`, untouched parts/refs left to backend.
  - passive SSE addition of a new version while the sheet is open keeps draft and base 1.
- `editor/v13/scene-images.test.tsx`
  - viewing/editing version 2 (used 1) sends `baseVersion: 2` with only the edited scene.
  - used version changes during confirmation; request keeps the captured `baseVersion: 1`.
  - existing feedback expectation updated with `baseVersion: 1`.
- `screens/workbench/LiveStudio.test.tsx`
  - viewer feedback sends `baseVersion: 2` while the video still uses 1.
  - explicitly selecting old version 1 sends `baseVersion: 1`.
- `screens/workbench/WorkbenchDesktop.test.tsx`
  - failed Grok retry resends the failed version (`baseVersion: 2`) with its saved scene part.

## Observed parent checks

- Four app consumer files:58 tests passed, including cancelled-history fallback, fresh redraw, selected-vs-used sources, confirmation/SSE pinning, refs-only fresh submission and upload/pending-request source navigation.
- Editor scene file:25 tests passed, including pinned preview/draft, cancelled history, explicit version changes and read-only legacy prompts.
- `npm run typecheck` and `npm run typecheck:engine-tests` passed.
- `npm run build` passed. Existing Zod annotation and >500kB chunk warnings remain visible.
- Final full affected suite:3755 passed,0 failed,4 skipped. [Native final source/checkpoint proof](runs/61-selected-source-final-empty-cancelled-history/selected-base-native-results.json): PC/mobile × normal/reduced, actual202, source2/current1/refs2, cancelled3 disabled, overlays contained, no stacked active modals or horizontal overflow; source/replay restored and0provider.

## Scope and limitations

- Desktop Review now pins the stage/version selection as well as the edit rail during editing. A newly completed version does not replace the image under the user's draft.
- Mobile does not gain a four-part editor or viewer-45 scope. Legacy structured-prompt data keeps an honest read-only edit limitation.
- No additional paid image/model/provider calls. The isolated native API uses the reviewed daily cap of two and has no QA provider credential file; its selected-source replay is paused before submission.
- Cancelled history stays visible and disabled; it is never a redraw source. With no usable source, desktop and mobile support ordinary fresh feedback redraw without `baseVersion`; edit stays read-only rather than manufacturing source0. New refs can also use that narration path.
- A dirty source that becomes unusable is not silently replaced. Its draft/unchanged fields stay pinned, submit is disabled with a reason, and R cannot send feedback-only.
- Reference picker and upload callbacks respect drawing/busy state. Upload completion compares job/image/source identity before attach, so it cannot dirty another draft.
