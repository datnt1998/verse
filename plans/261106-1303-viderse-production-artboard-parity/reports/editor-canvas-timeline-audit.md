# Canvas and timeline functional audit — 2026-11-09

Status: DONE_WITH_CONCERNS

## Summary

Source-proven gesture defects were repaired in the isolated canvas-audit worktree. The v13 timeline now has actual move/trim gestures backed by existing canonical commands, instead of clip buttons that could only select/seek. Canvas resizing exposes all eight engine-supported edges/corners, preserves rotated opposite-corner geometry, and does not author unrelated rounded properties or expression-owned values.

This is a source audit and authored regression coverage, not an exhaustive browser audit. No tests, typechecks, builds, linters, formatters, browser sessions, production changes, or Git/index mutations were run.

## Modified files

Line counts are resulting file lengths, not diff statistics.

| Path (relative to repository root) | Lines | Change |
| --- | ---: | --- |
| `apps/viderse/web/src/editor/v13/canvas-transform-interaction.tsx` | 504 | Single and linked/multi-selection gizmos; atomic geometry/keyframe/visual-style updates; protected ownership; percentage Scale units; rotation/anchor math; cancellation and no-op history preservation. |
| `apps/viderse/web/src/editor/v13/canvas-transform-interaction.test.tsx` | 601 | Consumer/history regressions for single and linked Subscribe-like components, evaluated animation, visual font/span scaling, rotated members, parent/child selections, visibility, cancellation, limits and driver conflicts. |
| `apps/viderse/web/src/editor/v13/preview.stylex.ts` | 40 | Edge/corner hit areas inside the selection box and separate rotation-handle placement. |
| `apps/viderse/web/src/editor/v13/Timeline.tsx` | 509 | Pointer move/trim previews and release-only atomic commands; individual cue/word moves and cue trims; locked/source/adjacency limits; linked selection reuse; Escape/capture/blur cancellation; correct scrub cancellation. |
| `apps/viderse/web/src/editor/v13/Timeline.test.tsx` | 276 | New consumer tests for gestures, isolated cue/word edits, rendered trim clipping, split/delete/select/seek/visibility/history and boundary calculations; actual visible-editor shortcut dispatch. |
| `apps/viderse/web/src/editor/v13/timeline.stylex.ts` | 64 | Trim hit areas only. Root height remains unchanged for the sibling's separately authorized layout integration. |
| `apps/viderse/web/src/editor/engine/features/preview/utils/transform-calculations.ts` | 284 | Parent-approved shared fix for inverse rotation about the actual anchor and fixed rendered opposite corner/edge. |
| `apps/viderse/web/src/editor/engine/features/preview/utils/transform-calculations.test.ts` | 169 | Rotations/edges/corners/implicit and explicit anchors/aspect locks plus snapping ownership regressions. |
| `apps/viderse/web/src/editor/engine/features/preview/stores/gizmo-store.ts` | 691 | Parent-approved avoidance of an incompatible scale snap solver for rotated/opposite-corner scaling, preserving exact calculated geometry and clearing guides. |
| This report | — | Findings, reproduction scenarios and unverified limits. |

`Preview.tsx`, `Editor.tsx`, editor/global utilities, vendor primitives, canonical artboards and the prior property report were not modified.

## Findings and implementation

1. **Rotated Ctrl-resize moved the opposite corner.** `calculateScale` used unrotated x/y offsets after sizing, and inverse-rotated pointer coordinates about the box center even when the renderer used an explicit anchor. It now computes the before/after rendered fixed point using the shared anchor contract. Explicit pixel anchors retain their authored values; no speculative anchor scaling was introduced. Default center-scale position and flip fields remain unchanged.
2. **The scale snap solver invalidated geometry.** `applyScaleSnapping` assumes axis-aligned center scaling, converts AABB extents into width/height, and can recenter positions at canvas sizes. Rotated and Ctrl/opposite-corner scaling now retain exact calculated geometry, including fractions, without misleading snap lines. Axis-aligned center scale snapping and translation snapping are unchanged.
3. **Only the southeast resize control was exposed.** All eight existing engine handles are now reachable inside selection bounds. Rotation no longer shares the north resize handle's location. No second geometry implementation was added; `ui/preview/gizmo-geometry.ts` is an older uniform-diagonal helper, not the active v13 engine gesture path.
4. **Alt-move/rotate could save unrelated rounded dimensions/positions.** The shared store's generic rounding affects all transform fields. The v13 consumer now limits translation writes to local x/y and rotation writes to local rotation. Scale still uses the engine's scale geometry.
5. **Expressions/links could be baked or silently overridden.** Commits previously used post-expression values as editable values. The consumer now uses pre-driver animation values, guards enabled scalar/vector driver ownership, and leaves expressions/links untouched. A driven x component can remain fixed while y is moved. A resize affecting a driven dimension is marked unavailable with an actionable Vietnamese tooltip/message; dynamic modifier/parent-space conflicts cancel without committing. Ordinary keyframed transforms remain editable.
6. **No-op gestures could dispatch needless transforms.** Empty transform/keyframe payloads are not dispatched, preserving redo availability. Existing release-only command behavior and merge-key isolation are retained.
7. **Timeline move/trim controls had no implementation.** Clip bodies now move real item selections; explicit edge hit areas trim real item boundaries. A gesture only previews local UI until pointer release, then executes one `item.moveMany` or `edit.trimItemStart/End` command with merge isolation. Existing trim/source/adjacency helpers and linked selection expansion are reused. Locks, frame-zero, one-frame minimum duration, source ranges and adjacent items constrain changes. Escape, lost pointer capture, pointer cancellation, blur, playback start and competing item/keyframe edits discard the preview.
8. **Scrub cancellation sought to cancellation coordinates.** `pointercancel` often supplies zero coordinates; the handler now finishes at the last preview frame rather than jumping to frame zero.
9. **Selection now uses the existing linked-selection helper.** Plain/Shift selection follows the shared item-selection contract rather than a parallel local toggle implementation.

10. **Displayed subtitle slices require individual cue editing.** Cue body gestures now change only that cue's times, bounded by neighboring cues and the containing item. They reuse `Inspector.tsx:162-175`'s atomic `item.update` plus `caption.setWords` contract: only words originally starting in that cue shift, and all other cues/words, container timing and estimated flags remain unchanged. Cue edge handles edit only the selected cue window with a one-frame minimum. Edge trims retain stored word timing because `caption-pages.ts:121-189` already clips/drops words to the selected rendered window; extending a cue back therefore preserves measured timings. Consumer regressions use two cues, verify untouched second-cue words, renderer clipping, cancellation and one-step Undo/Redo. Normal non-cue multiselection movement remains unchanged.

The active split/delete/visibility/mute handlers already dispatch real engine operations. They were inspected rather than replaced. Source inspection also followed evaluated keyframes/expressions, world/local parenting, renderer anchor conventions, track draw order, fit-coordinate conversion, preview quality and safe-area overlays. Safe-area decorations already use `pointer-events: none`; no cosmetic change was needed there.

## Shared-contract review

Main explicitly authorized the changes to `transform-calculations.ts`, its existing test file and `gizmo-store.ts` after receiving the concrete geometry/snapping findings. No LSP tool is exposed in this runtime, so exact text references were used as the fallback:

- `calculateTransform`: production caller `engine/features/preview/stores/gizmo-store.ts`; direct regression callers in `transform-calculations.test.ts`.
- `prepareScaleStartTransform`: active v13 gesture start, preview, command builder and the existing scale tests.
- `.updateInteraction(...)`: active production caller `v13/canvas-transform-interaction.tsx`; tests in `transform-calculations.test.ts` and `stores/use-item-gizmo-preview.test.tsx`.
- Anchor contract: `coordinate-transform.ts:getTransformAnchor`, frame-scene resolution and `shared/utils/transform-parenting.ts`.

## Parent verification commands

Run from `apps/viderse/web` after all integrated edits settle:

```sh
npm run test:engine -- src/editor/v13/canvas-transform-interaction.test.tsx src/editor/v13/Timeline.test.tsx src/editor/engine/features/preview/utils/transform-calculations.test.ts src/editor/engine/features/preview/stores/use-item-gizmo-preview.test.tsx
npm run typecheck
npm run typecheck:engine-tests
```

Then run the parent's integrated build/test/browser acceptance gates. These commands were not executed by this agent, so no passing claim is made.

## Real browser acceptance scenarios

Use a disposable project copy in the actual v13 editor, not an artboard or a mocked story.

1. Select a shape/image and use each `Đổi tỉ lệ clip (nw/n/ne/e/se/s/sw/w)` handle. Repeat at different fitted preview sizes by resizing editor panels/window, with safe-area overlays both on and off. Drag beyond the preview while pointer capture is active. Confirm one Undo restores the gesture and Redo reapplies it.
2. Set rotations 45°, -45° and 90°. Ctrl/Command-drag opposite-corner and edge resize handles, including an explicit off-center anchor. Confirm the opposite rendered point remains fixed. Repeat without Ctrl for center scaling and with Shift aspect-lock inversion. Rotated/opposite-corner resizing intentionally has no magnetic snap or snap guides; axis-aligned center resizing should still snap.
3. During a drag, press Escape, change selection/frame, start playback, or blur the window. Confirm no authored transform/keyframe change. Undo a completed gesture, cancel another gesture, then confirm Redo is still available.
4. Use fractional width/height and an existing rotation. Alt-drag Move and Rotate independently; save/reload and confirm unrelated dimensions and position/rotation are unchanged as appropriate.
5. With an animated parent at a rotated frame, move/resize a selected child. Confirm only the child's local values change and parent/keyframes remain intact. Repeat a composition instance; do not assume this edits its nested children.
6. Enable an expression on x only. Drag diagonally: x stays expression-owned while y changes. A width expression should prevent a coupled resize with the Vietnamese explanation; rotation/y and non-driven unlocked edges must remain usable. Disable the expression and repeat. Check source expression/link and history after blocked gestures.
7. In `Dòng thời gian`, drag a clip body to move, drag either edge to trim, then Undo/Redo. Repeat at several zoom levels, with snap on/off and Alt override. Verify frame-zero, one-frame duration, source limits, adjacent clips, linked/multiselected participants and locked tracks. Cancel via Escape and pointer cancellation before release.
8. Click/Shift-click timeline items, use ruler pointer/keyboard seek, split with `Tách`, delete with `Xoá`, and Undo each operation. Toggle track visibility/audio mute and confirm actual player behavior, not only button state.
9. Cancel a ruler scrub after moving away from the start; it must retain the last scrubbed frame rather than jump to zero.
10. Import a subtitle item with two separated cues and measured words. Drag one cue and confirm the other cue/words and the subtitle container's start/duration do not change. Drag both cue edges against the preceding/following cue, frame zero, and the container end. Confirm measured words outside a shortened visible window stay stored but are not rendered. Undo restores the exact original cue/word data; Redo reapplies only the selected cue edit. Repeat while another ordinary clip is selected: dragging a cue must not move that clip.
11. At frame 150 select the actual template `Nền nút Subscribe`. Confirm the backdrop/play icon/text remain selected and the box covers the whole component, not the first member. Resize at several fitted preview sizes: text glyphs/spans, icon, backdrop, offsets and pixel-valued strokes/radii/shadows must scale together. Rotate and drag the group; confirm all members move coherently. One Undo/Redo must restore/reapply the entire action. Press Escape mid-resize and verify typography and geometry both restore without losing Redo. Repeat Ctrl-corner resize, an edge resize with Shift, mixed geometric/font keyframes, and a selected parent/child pair. With linked selection disabled, single-text resize must still resize only its text box, not glyph sizes.

## Concerns / unverified limits

- Parent reported 148 narrow tests passing after the first integration corrections and before this linked-group follow-up. The new group source/regressions have not been run by this agent. Parent also reported successful real subtitle move/resize/cancel, cue edits/history, SRT import and exported video checks on the preceding baseline. None of that is evidence that the new group implementation has passed; its integrated tests/typechecks and real browser/export proof remain parent-owned gates.
- Rotated/opposite-corner scaling deliberately does **not** magnetically snap. Adding a rotation/anchor-compatible snap solver was not attempted.
- Expression/link-owned components are not invertible by arbitrary pointer gestures. The explicit ownership guard is intentional; the user must disable/unlink that driver to edit its target. Coupled resize conflicts can cancel when modifiers or parent-space conversion require a driven property.
- Timeline body movement is horizontal on the existing track; this change does not invent cross-track drops, track reordering, or ripple editing. Displayed subtitle slices now have isolated per-cue move/trim gestures rather than silently moving the subtitle container.
- Oversized/off-canvas items and dense tiny rotated items can still have clipped or overlapping controls inside the existing frame clipping model. The inspector remains the numeric editing route. This report does not claim these configurations were browser-tested or that all possible hit areas are proven.
- Parent must combine the trim-style additions with PanelResizeRepair's independent `timeline.stylex.ts` root-height change. This worktree did not change that declaration.

## Integrated failure follow-up

Main provided the failing integrated run; it was accepted as evidence and was not rerun by this agent.

- **Real vector-scale defect:** `animated-transform-resolver.ts:442-443` interprets coupled Scale values as percentages, while the v13 consumer had passed pixel dimensions to the vector auto-key helper. The commit builder now converts through the unanimated resolved base; scalar width/height lanes remain in pixels. Consumer tests start from 100% and 150% evaluated scales, place the gesture from the actual rendered geometry, assert a 1.5× rendered result and unchanged base data, and retain undo/redo assertions. Auto-key payload assertions now use percentage units; a scalar-width regression protects the other contract.
- **Escape fixture wiring:** `ShortcutProvider` requires a visible `[data-editor-root]` (`ui/shortcuts/index.tsx:139-140,153`). Both fixtures had omitted that root, so the real provider correctly ignored the key event. The fixtures now mount the actual editor root/provider/scope and supply only jsdom's missing visibility rectangle. Cancellation, unchanged data and redo assertions remain intact; no production shortcut bypass was introduced.
- **Normalized transform fixtures:** canonical `_updateItemTransform` materializes resolved defaults (`items-store.ts:791`). The consumer fixture now uses that existing mutation before gestures and captures the complete starting transform. Resize, cancellation and expression-axis tests compare against that captured normalized geometry, not raw partial fixtures or pinned incidental anchor values.
- **Cue floating-point precision:** cue times remain seconds and the Inspector-compatible delta is added in frame/fps units. The trim assertions now compare the resulting times in frame space at high precision while still checking unrelated cue fields, other cues, words and exact undo data unchanged. No production rounding or input special case was added.

Only `v13/canvas-transform-interaction.tsx`, its test file, `v13/Timeline.test.tsx` and this report changed in this follow-up. No builds/tests/typechecks were run in the isolated worktree.

## Linked/multi-selection component follow-up — 2026-11-10

### Corrected runtime diagnosis

Main's browser proof at frame 150 showed that selecting `Nền nút Subscribe` selects backdrop/play/text together. The preceding overlay **did show handles but resolved and changed only the first member**. The earlier apparent absence of handles was at frame zero before the component was visible. This follow-up does not remove grouping or silently reduce selection to expose handles.

### Implementation

- Builds a virtual group from rotation/anchor-aware AABBs using the existing `computeItemAabb` and `GroupTransformState` contracts. Existing engine gesture calculations, group scale snapping, world/local parent conversion and canonical `item.updateTransformsMap` are reused; no exported helper or deeper engine contract was changed.
- Translation applies one world delta to every visual member. Rotation maps each member's actual pivot about the common group center, recovers the box center, and adds the same angle to its existing rotation. Parent integration corrected the initial box-center-only formula, which drifted for off-center anchors. Uniform resize preserves relative offsets, dimensions and explicit anchors, including already-rotated members. Group resize remains uniform even with Shift because nonuniform scaling of independently rotated boxes would require unsupported shear.
- A private, ephemeral virtual gizmo ID prevents the group AABB from entering the primary member's imperative/single-item renderer path. All real members receive their own existing unified previews.
- Selected ancestors are resolved before converting child world targets back to local space. A selected child does not receive the same parent scale/rotation twice.
- Visual resize scales text font sizes and explicit rich-span sizes/tracking, plus pixel-valued padding, background radius, text stroke/shadow distances and shape stroke/radius. Unitless line height, colors/styles, text, words/timing, effects and shader parameters are left alone. Existing font/preset/keyframe operations are used rather than baking animated values into base fields. Animated text presets are evaluated through their own existing scale lane so preview and final renderer use the same canonical preset rounding.
- One release executes one atomic `item.updateTransformsMap` containing geometry entries, typography/shape `itemUpdates` and auto-key operations. Any driven member conflict rejects the whole group instead of partly moving unowned members. Existing font keyframes remain editable; current typed expression/link targets exclude font fields, and the generic target-ownership guard also checks enabled font targets if present in runtime data. No bindings are removed or rewritten.
- Escape, competing selection/item/keyframe/transport changes and capture loss restore every member's complete prior preview. Returning to the starting scale clears temporary glyph/style overrides and does not create history. Uniform shrink respects minimum glyph size and clears snap guides when a minimum prevents the snapped result.
- Single-text box-resize behavior and active-frame visibility remain unchanged.

### Authored regressions and remaining verification

The expanded consumer suite uses a linked backdrop/text/icon fixture with different sizes, rich spans, a rotated off-center-anchor member, and canonical normalized starting transforms. It covers three fit scales, actual linked selection from a canvas hit, group move/rotate/Ctrl-resize/edge resize/nudge, complete atomic history, Escape/redo preservation, returning to scale one, font minimums, visibility, mixed scalar/vector/font keyframes, preset animation, driven-property rejection and selected parent/child conversion. Raw word timing and span styles are checked unchanged where not scaled.

For this follow-up copy **only** `v13/canvas-transform-interaction.tsx`, `v13/canvas-transform-interaction.test.tsx` and this report from the isolated worktree. No timeline or style module changed in this follow-up; parent trim-width/root-height integrations remain untouched. No build, tests, lint, formatter or browser run was performed by this agent.

### Parent integrated verification

- Full frontend suite after group corrections:451files,3859passed/4skipped. Parent fixed undefined font reads (`resolveTextStyle` has nofontSize), custom-anchor rotation, and numerical precision assertions. No production rounding was added.
- Actual linked template retains three selected members through resize, one undo/redo and Escape. Saved font changes35→57.006; decoded worker-export frame verifies a further visible-frame resize atframe150 preserves the icon/text/backdrop alignment. Existing animation semantics edit current-frame keys, not every key in a lane.
- Independent review found two timeline issues: unsnapped leading edges masked valid trailing-edge snaps, and trim preview excluded ordinary selected neighbors. Parent reused `snapGroupDelta` and canonical trim exclusions, added consumer regressions, and verified20timeline tests passing.
- Final embedded build and guarded production deployment are recorded in [the owning functional audit](editor-ui-functional-audit.md); this isolated handoff is not a deployment receipt.
