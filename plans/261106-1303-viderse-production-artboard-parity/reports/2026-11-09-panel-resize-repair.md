# Production editor panel resizing

Status: DONE_WITH_CONCERNS

## Summary

The active v13 shell now uses the existing react-resizable-panels engine and vendor wrappers for library/preview, preview/inspector, and workspace/timeline splits. The obsolete shell is not imported or activated. Existing editor content, actions, preview overlays, timeline props, Vietnamese copy, narrow-screen contract, and data-editor-root marker remain intact.

Handles support pointer dragging, arrows (8px), Shift+arrows (32px), and Home to restore the adjacent panel. The header control restores all panels. Native separators supply focus and ARIA resize values. Preferences are project-scoped, validated before restoration, and safe when storage access or writes fail. Parent browser verification found double-click unreliable; the handler and its promise were removed rather than shipping a non-working control. Coarse-pointer resize targets use44px.

## Exact modified files

Paths below are relative to apps/viderse/web/src/editor/v13/ unless noted.

- Editor.tsx: active layout wiring and reset control.
- editor.stylex.ts: resize hosts, scrollability, sizing and title clipping.
- Inspector.stylex.ts: scoped width/flex variables retain standalone fallbacks.
- timeline.stylex.ts: scoped height retains286px fallback; interaction styles integrated separately.
- editor-layout.tsx: production layout using existing primitives.
- editor-layout-storage.ts: validated storage boundary.
- editor-layout-storage.test.ts: malformed persisted layouts and unavailable storage reads. Parent removed copy/round-trip/default-pinning assertions; runtime persistence is checked through the actual editor.
- plans/261106-1303-viderse-production-artboard-parity/reports/2026-11-09-panel-resize-repair.md: this scoped handoff.

No artboard, Preview.tsx, Timeline.tsx, canvas interaction, vendor primitive, global stylesheet, package manifest, index, HEAD, or production state was modified.

## Geometry and constraints

- Canonical defaults: 68px rail, 316px library and inspector at >=1280px; 240px library and 268px inspector below1280px; 286px timeline.
- Zero-layout-width/height separators preserve default geometry; expanded pointer targets and hover/focus indication provide interaction affordances.
- Library 240–480px; inspector 268–440px; preview minimum400px. Side panels preserve pixel sizes while preview absorbs window changes and the engine applies constraints.
- Workspace minimum220px; timeline minimum140px and maximum65%. Tiny-height windows can scroll the workspace host rather than permanently losing controls. The rail itself scrolls without shrinking buttons.
- Preview transport styles currently respond to viewport width, not resized panel width. The owned preview host provides horizontal overflow access rather than modifying Preview or hiding controls.

## Verification and parent acceptance scenarios

Delegate ran no checks. Parent subsequently verified all three pointer drags, keyboard ArrowLeft/Home/End, visible reset and reload persistence at1440×900. Widths1024/1280/1920 and narrow768/390 produced no document overflow; narrow sizes retain the explicit desktop-editor handoff. Full release checks remain in the owning functional audit.

From apps/viderse/web, after integration:

1. Run `npm run typecheck` and `npm test -- src/editor/v13/editor-layout-storage.test.ts`.
2. Run the parent's production build and editor functional regression suite.
3. At 1440px and 1024px widths, drag each handle both ways, confirm constraints, preview framing, accessible controls, scrolling, and unchanged editing/playback/selection functions.
4. Tab to each named separator; exercise arrows, Shift+arrows, Home, and native End handling. Confirm labels, focus indicators, values, and no timeline-seeking shortcut leakage.
5. Resize all panels; reload the same project; inspect restoration. Open another project and confirm isolation.
6. Corrupt the saved JSON and block localStorage access/writes: opening and resizing must continue with defaults or in-memory behavior.
7. Shrink and regrow the window, including crossing1024px and1280px and short heights; confirm narrow-screen message, no clipped critical controls, and reasonable constrained recovery.
8. Test the header reset and Home; reload and confirm reset persistence. At canonical dimensions, confirm316/316/286 defaults. Double-click reset is intentionally not advertised.
9. Confirm export locking makes resize controls inert.

## Concerns / integration notes

- Build, typecheck, actual pointer capture/hit targets, keyboard behavior, CSS compilation, screenshot geometry, and reload restoration are not exercised here.
- Merge the inspector/timeline declarations selectively with parent/CanvasTimelineAudit style work. CanvasTimelineAudit was notified of the authorized timeline height declaration.
- Parent's scoped semantic utility bridge is required by existing vendor Tailwind wrappers. No parallel palette or global semantic mapping was introduced.
