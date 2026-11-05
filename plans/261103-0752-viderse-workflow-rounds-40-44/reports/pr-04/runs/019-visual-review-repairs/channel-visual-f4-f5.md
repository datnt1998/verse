# Channel visual F4/F5 repair proposal

Status: DONE

## Deliverable

`channel-visual-f4-f5.patch` is a functions.edit hashline patch against current source snapshots:

- `apps/viderse/web/src/screens/channel/SourcesTab.tsx#5919`
- `apps/viderse/web/src/i18n/channels.ts#C5AF`

Only those two product files are targeted. No product writes or execution checks were performed.

## Proposed repair

- F4: Mobile unlink uses a channel-owned `Sheet` with `height="auto"`, full-width title/body, and a two-column equal-width ghost/dsoft action grid. No warning-icon column, indented bullet list, content-width footer spacer, or fabricated device chrome.
- Desktop remains the approved kit `confirm()` with dsoft tone and its existing lead/bullets composition. Both surfaces use the same title, consequence strings, cancel/confirm labels, and one shared actual-unlink callback. The reference replacement consequence now explicitly says that nothing is regenerated, matching canonical copy.
- The callback retains the actual `unlinkSource(channel.id, rivalId)` request, both existing channel invalidations, and the success toast chosen from the server's actual `autoFetchPaused` result. There is no new request or success fallback.
- The mobile owner uses the existing `useAsyncAction` lock/error/pending helper. A synchronous ref also guards dismissal before React publishes busy state. Cancel, scrim, Esc and drag dismissal cannot close an in-flight request; both actions are disabled while busy; dragging is disabled while busy. Failure stays visible and retryable; the sheet closes only after actual success or idle cancellation. The mounted sheet is retained through its own exit, then removed, resetting errors for the next confirmation.
- The new confirmation is outside the phone/desktop content branch, so an already-open mobile request is not abandoned merely by crossing the responsive breakpoint.
- F5: Restore localized secondary guidance and the canonical `trend`, `radar`, and `trash` icons, plus the unlink action ellipsis. Existing destinations and actual action semantics remain unchanged.
- Preserve the existing `SourceRow` ActionSheet `onExited` handoff verbatim. Selection only records the unlink intent; confirmation is created after the action sheet releases its overlay/focus locks, avoiding stacked or deferred invisible confirmations.

## Why not ConfirmSheet

Inspected `platform/ui/src/mobile/sheets.tsx:158–239` and `Sheet.tsx:43`: ConfirmSheet supports actual async failures/pending, but its list layout does not expose height and therefore inherits Sheet's 640px default. It also exposes only primary/deep-danger button styling, not ghost+dsoft. Those constraints cannot express the canonical compact confirmation without shared-kit changes. The local composition reuses the existing Sheet, Button, tokens and async-action helper instead; no shared-kit changes are proposed. Parent explicitly approved this owner-local route.

## Source/evidence read

- `channel-visual-acceptance.md`, F4/F5 and comparison findings.
- Existing `017-final-canonical-comparisons/MChannel.sheet.unlink.png` and `MChannel.sheet.rival.png` viewed natively; canonical left, application right.
- `apps/viderse/docs/design/v1/artboards/MChannel.dc.html:1043–1065,1159–1161`: compact unlink structure, guidance, icon semantics, and no-regeneration consequence.
- `apps/viderse/docs/design/v1/11-hoi-dap.md:298–302`: desktop kit dsoft approval.
- Current SourcesTab/i18n owner reads, ConfirmDialog desktop flow, mobile sheets API, Sheet lifecycle, and useAsyncAction contract.

## Parent verification after integration

Not exercised here: type/build/test/lint/browser/API or rendered visual checks. Parent should verify cancellation produces no unlink request; duplicate clicks produce only one request; all pending dismissal paths remain locked; rejection keeps the confirmation open and retryable; success retains the actual invalidation/toast behavior; both reference-next and last-source consequences remain accurate; ActionSheet-to-confirmation has only one mounted overlay; mobile full-width text and equal action geometry match the existing canonical context. No blanket visual or functional acceptance is claimed by this proposal.
