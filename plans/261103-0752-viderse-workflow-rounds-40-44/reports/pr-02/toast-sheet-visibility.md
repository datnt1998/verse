# Toast ↔ bottom sheet — placement cause and fix

Scope: `platform/ui/src/Toast.tsx` activity branch, `platform/ui/src/Toast.css`, native provider audit. Noverse (`<ToastHost />`, non-activity) placement is unchanged.

## What the run46 evidence actually shows

`runs/46-real-local-provider-catalog-fixed/browser/provider-catalog-audit.pw.-bd683-ce-and-owned-record-removal-mobile/`

The `.png` labelled “failure toast blurred behind the sheet” is **not** a stacking/layer
failure. It is the pill caught inside its own entrance animation:

| time | frame | state |
| --- | --- | --- |
| 1.2 s | sheet settled, no toast | — |
| 1.4 s | sheet settled, no toast | — |
| 1.6 s | `video.webm` frame | pill **sharp and on top of** the white CTA (glass darkens the button, text crisp) |

`vv-tIn` (`Toast.css`) starts at `opacity 0 · blur(4px) · translateY(18px) · scale(.94)`
and runs `.5s`; the PNG was captured inside that window. `vv-tOut` would have shown
`scale(.5)` at 65 %, which the PNG does not.

Measured stack (native probe, 390×844, real session, sheet open):

- `.m-overlay` `z-index: 80` (`platform/ui/src/kit.css:203`), `.m-sheet` has no `z-index`
  of its own (it stacks inside the overlay).
- Activity hosts: pill `z-index: 1001`, card stack `z-index: 1000`.
- `#root`: `position: static`, `z-index: auto` → no stacking context, so 1001 really does
  beat 80 at the root level. No ancestor `filter`/`transform`/`opacity` trap.
- `inert` is only on the overlay itself while closing (`Sheet.tsx`: `inert={!open}`), and
  the toast is not inside the overlay subtree.
- `pointer-events` on the pill host is `none`, so the CTA underneath stays clickable.

Conclusion: **the toast is above the sheet.** Raising `zIndex` would change nothing and
hiding it below the scrim would lose the feedback.

## The real defect: both layers are bottom-anchored

`--m-toast-bottom` on the settings screen resolves to `calc(max(26px,0) + 0 + 12px)` = 38 px.
The sheet's footer CTA sits in the same band. With the add sheet open (390×844):

| rect | value |
| --- | --- |
| sheet `offsetHeight` | 592 → final top y = 252 |
| CTA “Kết nối và lấy model” | y 749.7 … 797.7 (48 px tall) |
| pill before fix (`bottom: 38px`) | y 766 … 806 |
| overlap | **31.7 px of the 48 px CTA covered** |

So the 4 s error pill covers the very button that produced it. Same collision on the
“Gỡ Ollama?” confirm sheet and on the “Đã gỡ Ollama.” result. This is the phase-03 rule
“toast không che action” applied to the sheet layer.

## Fix

`ActivityToastHost` now measures the topmost open sheet and lifts both activity hosts above
it (Material: snackbar above a bottom sheet):

- `openSheet()` → `.m-overlay:not(.m-out) > .m-sheet`, last one in DOM order = topmost.
- Lift starts at `sheet.offsetHeight + 12`. Parent integration caps it using the **measured tallest host content**, not a fixed40px pill assumption, so multiline cards also remain in the viewport.
- `offsetHeight` is layout height, so the sheet's `m-sheet-in` transform and the user's
  drag do not make the toast jitter.
- `bottom` is written as an inline style on both hosts (beats `ax.stack`'s media-query
  value); with no sheet the hosts fall back to `--m-toast-bottom` / `--vv-toast-bottom`
  exactly as before.
- Re-measure on: overlay mount/unmount (`MutationObserver` on `body` childList),
  `.m-out` on the overlay (observer on the overlay node), sheet and host-content size
  (`ResizeObserver`), window resize.
- `.ui-toast-sheet-layer` transitions bottom over the existing sheet-close rhythm `.26s` with canonical `var(--ease-out)`; reduced-motion disables the transition.

Measured after the change (same probe, sheet open):

| | before | after |
| --- | --- | --- |
| mobile, sheet 592 | host `bottom: 38px` → pill y 766…806 | host `bottom: 604px` → pill y 200…240, sheet top 252 → **12 px clear, 0 overlap** |
| desktop, sheet 561 | host `bottom` from `--vv-toast-bottom` | host `bottom: 573px` → pill bottom 12 px above sheet top |

Both hosts carry the lift (pill + card stack). 4 s duration, accessible full text, undo,
actions, `dismissToasts` session cleanup and Noverse defaults are untouched — the change
lives only in the Viderse-only activity branch.

## Tests

Parent integration removed the two new unit tests that only copied synthetic offsetHeight into expected inline-bottom literals. Permanent coverage instead belongs in `provider-catalog-audit.pw.ts`: wait for the real finite entrance/placement animations, assert the actual feedback rectangle stays in the viewport and does **not overlap the real submit button**, then capture it. This catches the consumer-visible collision.

Integrated checks after the parent geometry adjustment: full UI kit **261/261**, UI kit typecheck passed; affected Viderse Review/Connect **32/32**, Viderse typecheck passed. Existing icon/act/intentional chunk-fault diagnostics remain visible; no thresholds or timeouts changed. [Native run51](runs/51-provider-feedback-layout/interaction-results.json) **8/8 passed** with real feedback/submit bounding boxes, viewport/no-overlap assertions and settled painted screenshots in both viewports and motion modes.

## Evidence limits

- The delegate's custom probe could not obtain a painted post-fix screenshot because its Chrome page stayed hidden and rAF did not progress; it measured DOM geometry only. Parent run51 supersedes that limit: ordinary Playwright contexts ran the actual entrance/placement animations to completion and produced painted mobile screenshots, inspected directly. The pill is crisp above the Sheet, and after Esc the actual provider card—not an exiting Sheet—is shown. No CSS injection or animation disabling was used.
- No provider record was created and no paid endpoint was called: the only network
  mutation used for the probe was a **browser-aborted** `POST /api/ai/providers`
  (`blocked: ["POST http://127.0.0.1:8044/api/ai/providers"]`), which is the same
  legitimate failure path as the run46 “real refused” capture. All other `/api/` traffic
  was GET.
