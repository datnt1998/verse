# Admin project picker clipped by the header slot — root cause and fix (2026-11-02)

Follow-up to `261102-admin-projects-visibility.md`. This is the UI cause behind the production
symptom: the API returns both projects, but the desktop project picker is clipped to zero visible rows.

## Measured production evidence (read-only, QA admin)

Browser: `https://nov.aarverse.studio/?probe=3#/p/dit-dang-audio`, viewport 1440×900.

| Measure | Before fix |
|---|---|
| crumb button | `{ x:52, y:12, w:160.8, h:30 }` |
| popover (`.kit-pop`) | `{ x:52, y:48, w:340, h:185.4 }`, `display:block` |
| hit test inside popover | `BUTTON.pj-scard` (underlying sidebar project card), not the popover |
| clipping ancestor | `DIV { x:16, y:12, w:436.34, h:30 }`, `overflow:hidden`, `position:static` |
| outer viewport | `overflow:hidden` but 1440×900, not the cause |

Ancestor chain from `.kit-pop` upward: `.kit-pop` (absolute) → relative crumb wrapper → static
ContextCrumb root (`overflow:visible`) → **header left grid slot `DIV` with `overflow:hidden` and a
30px-tall rect** → `HEADER` → app root. The 30px slot clips the popover that starts at `y=48`.

The slot is the first grid child in `apps/noverse/web/src/shell/AppHeader.tsx`; it previously carried
`overflow:'hidden'` to truncate long project names. The popover is a child of that slot, so the mask
also hid the popover. This is why a DOM query finds 2 options while a native hit test finds 0.

## Source fix (minimal, no portal, no global kit change)

- `apps/noverse/web/src/shell/AppHeader.tsx`: remove `overflow:'hidden'` from the header left grid
  slot only.
- `apps/noverse/web/src/shell/ContextCrumb.tsx`: keep text clipping on the crumb itself —
  `minWidth:0`, `maxWidth:'100%'`, `overflow:'hidden'` on `crumbBase`; `minWidth:0` on the project and
  story wrappers. The popover is a sibling of the crumb button, so it escapes while the label still
  ellipsizes.

The real pointer smoke exposed a second cause: `ContextCrumb` closed on document mousedown even when the press was inside its project/story wrapper, unmounting an option before click. Removed that redundant handler; the popovers now treat their trigger+content wrapper as one outside-click boundary, so options stay mounted and pressing the trigger again closes rather than reopens.

## Native after-fix simulation (not deployment proof)

The exact candidate CSS was applied to the live DOM at the same route/viewport:

| Measure | After candidate CSS |
|---|---|
| slot overflow | `visible` |
| crumb overflow / max-width | `hidden` / `100%` |
| popover | `{ top:48, bottom:233, left:52, width:340 }` |
| options | 2 (`dit-dang-audio`, `Trốn Audio`) |
| option-center hit test | topmost element inside `.kit-pop` for both |
| popover probes (top/mid/bottom) | all `inside:true` |
| screenshot | `/tmp/verse-ui-fix-proof/urgent-admin-production-picker-simfix.png` |
| proof JSON | `/tmp/verse-ui-fix-proof/urgent-admin-production-picker-simfix.json` |

Long-name containment was also simulated: with a 288-character label the nav X stayed 468.34px,
slot `scrollWidth == clientWidth == 436`, and document width stayed 1440 (no overflow). At 1100px
viewport the crumb was 161px inside a 266px slot, nav at x=298, document width 1100.

This simulation only proves the CSS direction. It is **not** a build/deployment proof.

## Regression proof policy

JSDOM/happy-dom cannot detect ancestor clipping, and style-property assertions do not prove a visible
hit target. No permanent incidental CSS/source assertions are kept for this fix. `AppHeader.test.tsx`
retains its three original consumer behaviours (nav reachability, cross-project story isolation, root project label) plus a real pointer-down→click project switch/trigger-toggle regression. The previous style/markup assertions were removed.

Regression proof must be, after a real Noverse build:

1. Open the project route at 1440×900 as an admin.
2. Click the crumb and assert each project option's centre resolves inside `.kit-pop` natively.
3. Confirm both option rows are visible in the PNG.

## Actual built local proof

- Real served entry `/assets/index-Dm1bUboW.js`, viewport1440×1000; no CSS/DOM patching or intercepted API.
- Real `/api/library` returned3 owned QA projects; all3 option centres hit their actual option, not underlying content.
- Trigger open→close→open and a normal pointer project selection both worked; URL and visible project heading changed to the selected project.
- Receipt `/tmp/verse-ui-fix-proof/picker-fixed-local.json`; PNG `picker-fixed-local.png`. No browser errors, content writes or provider calls.
- Focused `AppHeader.test.tsx`:4 tests passed; typecheck/build passed.
- Production still needs this actual native hit-test/toggle/navigation check after the next Noverse deployment. Original story remains paused.
