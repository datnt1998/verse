# Channel visual F1–F3 proposal — revised

Status: DONE

Patch: `channel-visual-f1-f3.patch` (current functions.edit hashline proposal; not applied). This revision supersedes the earlier containment-only F2 proposal.

## Evidence and exact anchors

Read the channel visual acceptance report, current owning sources, read-only `components/responsive-overlays.tsx`, the shared KitSelect/ActionMenu/Icon contracts, and canonical artboards under `apps/viderse/docs/design/v1/artboards/`. Opened existing comparison PNGs for `Channel.tab.templates`, `MChannel.tab.templates`, `MCreate.step.create`, `MCreate.rivals.some`, `MCreate.rivals.empty`, `Channel.identity.generating`, and `MChannel.identity.generating`. LEFT is canonical; RIGHT is app.

Re-read current original targets before this revision:

- `styles/viderse.stylex.ts#9927`: add channelLayout token after line 256.
- `channel/channel.stylex.ts#87CF`: lines 307–308, 311.
- `channel/CreateStep.tsx#8ABB`: import line 12; insertion after 453; mobile branch before 461; desktop trigger line 472; branch close after 478.
- `channel/mobile/mchannel.stylex.ts#4A35`: import line 6 and old style line 644.
- `channel/IdentityBlock.tsx#5921`: move lines 216–222 after original 237.

`TemplatesTab.tsx#B73B` remains unchanged; its mobile early return is independently composed. Shared adapters, UI kit, and canonical artboards remain unchanged.

## F1 — Explicit desktop row and tokenized width boundary

Cause: `TemplatesTab.tsx:111` merges the base card's column direction with a template style that does not override it; the section lacks a maximum width. Canonical `Channel.dc.html:1021` defines wrapping row/center alignment, 860px maximum section width, and 280px text flex basis.

Proposal: explicitly set row direction, wrapping, and center alignment; add `channelLayout.tplSectionMaxWidth: "860px"` to the existing owning `styles/viderse.stylex.ts` token object imported by `channel.stylex.ts:3`, and consume it for section maximum width. Reuse the existing 280 spacing token for text flex basis. No raw new dimension enters component styles, and no existing export is renamed. The mobile TemplatesTab composition consumes different card/head/text/action styles and remains untouched.

## F2 — One full-width borderless mobile row-button

Cause: the side-by-side mobile label and selector, 60% trigger maximum, and fixed 44px height cannot contain the selected name and normally wrapping description. Canonical `MCreate.dc.html:965–966` uses one full-width row-button with a layer icon, flexible label/value column, and trailing chevron.

API investigation: `KitSelectProps` (`platform/ui/src/Selects.tsx:84–126`) exposes icon, prefix, valueStyle, and triggerStyle, but no custom trigger/value renderer. MobileSelect wraps prefix as nonflex content and separately emits its own selected value/description and chevron (`responsive-overlays.tsx:184–192`). Those APIs cannot cleanly render this complete canonical label/value column. No generated-content, absolute-positioning, hidden-value workaround, or shared adapter extension is proposed.

Use the existing `ResponsiveActionMenu` only for this mobile row. Its custom `trigger` accepts the complete canonical content: existing layer icon, one flexible column containing the Template label plus actual selected name and description, loading spinner when applicable, and existing `arrow-r` right chevron. The trigger is full-width, transparent, borderless, naturally sized, and has the existing 48 minimum token. Name and description wrap with normal white space and overflowWrap; neither is clipped or suppressed. Existing Viderse spacing, typography, and colors size every new style; icon dimensions also come from existing spacing tokens through Icon.xstyle.

Map the existing options directly into selected radio items, retaining each actual ID/name/description and exactly the existing empty-string-to-null selection mapping. `ResponsiveActionMenu` already supplies Sheet, radio checked state, keyboard navigation, focus/scroll behavior, close animation, and selection deferred to `onExited` (`responsive-overlays.tsx:253–357`). This intentionally uses its existing menu/menuitemradio semantics instead of select/listbox semantics; it is not a new local sheet implementation. The desktop branch retains its existing ResponsiveSelect and inlineDescription unchanged. The old compressed mobile selector style is replaced, not retained as dead code. Generic desktop templateSub remains desktop-only; mobile shows the real selected recipe description.

## F3 — Paint persistent chrome after generation overlay

Cause: the later full-inset opaque overlay paints over the earlier caption and desktop safe-area control. Canonical `Channel.dc.html:1028–1031` and `MChannel.dc.html:1002–1004` place persistent chrome after generation content.

Move the existing caption and desktop switch JSX, unchanged, after the overlay. Source inspection confirms overlay/chip/switch are positioned siblings without explicit z-index (`channel.stylex.ts:119,121,149`), so normal paint/hit-test order puts only the existing chrome rectangles above the overlay. No full-inset interactive wrapper is introduced. Banner and generation overlay have no click handlers; safe-area frame retains `pointerEvents: 'none'`. Thus this change does not create a full-banner click-through target. The mobile safe-area row remains outside the banner. The frame stays before the opaque overlay, preserving its existing generation presentation.

All handlers, switch state, regeneration locks/disabled state, pending behavior, image URL/rendering and old-image DOM retention, avatar treatment, and download behavior are unchanged. No photo/inference request is needed.

## Parent verification oracles — not executed

1. Apply with the listed current snapshot tags; refresh anchors if concurrent integration changes a target. After all proposals land, run normal web type/style/build checks once.
2. Desktop template tab at 1440: computed maximum section width 860; card direction row, alignment center, wrapping enabled. Artwork/text/actions share a row when space permits; narrower desktop wraps actions without squeezing or overflow. Mobile template tab retains existing thumbnail/text header, recipe below, equal actions.
3. Mobile creation at 390 and a narrower supported viewport: one full-width borderless button, leading layer icon, Template label, actual chosen name and description in one flexible column, right chevron. Long real names/recipes grow the row; adjacent title/divider/approval row do not overlap. Include spinner presence and empty/populated rival contexts. Open sheet, confirm selected radio checked state and descriptions, choose another actual option, and verify dismissal completes before template ID changes. Reopen and exercise keyboard navigation, focus return, and cancellation. Confirm desktop selection remains unchanged.
4. Generating identity on desktop/mobile, with absent and real existing prior art: caption visible; desktop safe-area switch visible, clickable, and checked state updates; mobile safe row still outside overlay. Center/status legible; regeneration locks unchanged. Prior image retains the same DOM URL until the existing data lifecycle replaces it; ready/failed presentation unchanged. Confirm switch clicks do not trigger unrelated actions.
5. Match viewport, scroll, fixture, and device chrome before comparing. Do not fabricate data to reconcile empty/populated rival captures or failed/running onboarding captures.

## Limits

Proposal only: no product source edits, git/commands, build/tests/lint/formatter, browser/server/API, downloads, or inference. No post-change rendering or runtime success is claimed. The revised proposal covers all three source-backed causes and complete canonical mobile trigger composition; parent owns integration and verification. No new color values, renamed exports, source comments, or newly dead helpers are introduced.
