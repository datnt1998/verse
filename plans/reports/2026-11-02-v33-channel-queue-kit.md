# V33 channel/queue control cutover

Status: DONE_WITH_CONCERNS. The assigned desktop and mobile channel/queue surfaces are complete. This does not certify V33 outside this ownership scope.

## Implementation

- Channel underline navigation now uses controlled Kit `Tabs` with tab/panel associations, counts and roving keyboard navigation. A structural, non-shrinking wrapper keeps the tab bar visible in the desktop flex column. Inline count spacing keeps every tab fully visible at 360 px without overriding Kit button metrics.
- Desktop/mobile schedule zones and queue/video filters use Kit `Segmented`, including the measured sliding indicator. Existing controlled state, API requests, counts and filtered content are retained.
- Commands use Kit `Button` or the existing `ActionButton` adapter. Removed obsolete custom button, checkbox, segmented-control and menu styling. Remaining button xstyles are structural placement only.
- Mobile descriptive autonomy choices use `RadioCard`; source selection retains descriptive checkbox cards through Kit `Checkbox`. Identity safe-area toggles use Kit `Switch`.
- Desktop queue menus use `ActionMenu`; channel/source navigation remains native anchors inside the popup. Parent-approved menu dependency: original `c8f330cb`, locally cherry-picked as `13738bdc`. Parent already owns this dependency; integrate the consumer commit rather than replaying that dependency.
- Native navigation cards and genuine links retain href semantics. No application API changes, fake runtime fixtures, shared palette changes or global stylesheet changes were made by this consumer slice.

## Final source inventory

All source paths below are relative to `apps/viderse/web/src/screens/`. Counts are additions/deletions against the dependency HEAD.

| Path | + / - |
| --- | ---: |
| channel/ChannelControls.test.tsx (new) | 133 / 0 |
| channel/ChannelHeader.tsx | 3 / 7 |
| channel/ChannelScreen.tsx | 16 / 32 |
| channel/CreateStep.tsx | 14 / 34 |
| channel/DoneStep.tsx | 10 / 15 |
| channel/IdentityBlock.tsx | 13 / 29 |
| channel/PaceStepper.tsx | 5 / 5 |
| channel/ScheduleCard.tsx | 5 / 23 |
| channel/SettingsTab.tsx | 6 / 7 |
| channel/SourcesTab.tsx | 21 / 55 |
| channel/TemplatesTab.tsx | 22 / 44 |
| channel/VideoBulkBar.tsx | 10 / 13 |
| channel/VideoRow.tsx | 15 / 21 |
| channel/VideosFresh.tsx | 2 / 2 |
| channel/VideosList.tsx | 17 / 25 |
| channel/VideosTab.test.tsx | 1 / 2 |
| channel/channel.stylex.ts | 4 / 25 |
| channel/channels.test.tsx | 1 / 1 |
| channel/mobile/AutonomyMobile.tsx | 12 / 25 |
| channel/mobile/mchannel.stylex.ts | 0 / 250 |
| channel/videos.stylex.ts | 2 / 49 |
| queue/MobileQueue.test.tsx | 11 / 0 |
| queue/MobileQueue.tsx | 39 / 26 |
| queue/QueueScreen.test.tsx | 28 / 0 |
| queue/QueueScreen.tsx | 24 / 50 |
| queue/QueueSummarySheet.tsx | 6 / 7 |
| queue/queueMobile.stylex.ts | 3 / 20 |

Final production-source inventory found no raw `<button>`, native `<select>`, manually rendered tablist or menu in these two screen directories. The sole retained explicit radiogroup is `channel/mobile/AutonomyMobile.tsx:158`, containing descriptive Kit RadioCards; its 360/390 screenshots are listed below. The `role="menu"` text in VideoRow is only an event-delegation selector, not a rendered control.

## Final automated verification

Executed from `apps/viderse/web`, using the existing root dependency binaries; no installation or build was performed:

```text
../../../node_modules/.bin/tsc --noEmit --pretty false
../../../node_modules/.bin/vitest run src/screens/channel src/screens/queue src/checks/design-rules.test.ts --maxWorkers=1 --no-file-parallelism --reporter=json --outputFile=/tmp/v33-channel-queue-final-tests.json
```

- Application TypeScript: PASS after the final browser-driven layout repairs.
- Focused consumer suites and design rules: 103/103 tests, 13/13 files passed; includes 16 design-rule checks.
- New coverage exercises routed tab content/keyboard behavior, schedule-zone data, mobile autonomy save success/failure, desktop native menu links and keyboard navigation, and distinct-channel desktop/mobile filtering. Existing shift selection, disabled action, drag ordering, undo, autosave and notification consumer suites also passed.
- Final machine report: `/tmp/v33-channel-queue-final-tests.json`, SHA-256 `a8b2250f0f35bfa2cadf4540435968af2291f5b4ac09b32c5f22771fd1ce19be`.

## Real UI proof

Used the existing isolated backend 8047 through temporary Vite 8057, unique hostname `channel-kit.localhost`. Authentication was imported without logging credentials. One owned browser tab/context was used and closed; SLOT2 was released back to Main, and SettingsExploreKit was notified to await the parent's next grant. The helper/backend was never restarted or dropped. Temporary proxy, scripts and credential-import artifact were removed; screenshots and machine evidence remain available to the parent.

Native CDP mobile metrics verified `(pointer: coarse) === true`, at 390×844 and 360×844, with safe-top 59 px and safe-bottom 34 px. Desktop used 1440×1000 with fine pointer.

- All four channel tabs fit at both mobile widths; the last tab ends at x=340.15625 inside the 360 px viewport. Document scroll width equals viewport width at 390, 360 and 1440.
- Channel tabs changed real content. The video `Chờ bạn` filter selected and displayed the two actual matching videos.
- Schedule-zone selection moved the real Kit pill, measured width 81.0625 px and target transform `translate(47.9844px, 3px)` for New York. Both zones were exercised through the normal schedule API.
- Real pacing autosave changed 2→3; HTTP 200 channel reads and a page reload showed 3. It was restored to 2 through the UI.
- A descriptive autonomy choice changed pick `all`→`score2`; HTTP 200 read and page reload retained the selected card. It was restored to `all` through the UI. Final HTTP 200 read confirmed pace 2, pick all, priority steady.
- Queue channel-filter selection, Home reset, selected pill and first-row promotion disabled reason were exercised at both widths.
- Desktop menu Home focused its native channel anchor; End and ArrowDown focused the remove command; ArrowUp returned to the anchor. Escape closed the menu and restored trigger focus. Reduced-motion opening reported animation duration `0s`. No browser page errors were reported.

Machine browser manifest: `/tmp/v33-channel-queue-browser-proof.json`, SHA-256 `ccdb7798996358aa947954c62d9362c6282a610053d1a361ad363cc30f7bd887`.

Settled screenshot manifest:

```text
/tmp/v33-channel-390-source-tabs.png
/tmp/v33-channel-390-descriptive-cards.png
/tmp/v33-channel-390-video-filter.png
/tmp/v33-queue-390-filtered.png
/tmp/v33-queue-390-action-sheet.png
/tmp/v33-channel-360-source-tabs.png
/tmp/v33-channel-360-descriptive-cards.png
/tmp/v33-channel-360-video-filter.png
/tmp/v33-queue-360-filtered.png
/tmp/v33-queue-360-action-sheet.png
/tmp/v33-channel-1440-sources.png
/tmp/v33-channel-1440-settings.png
/tmp/v33-queue-1440-native-menu.png
```

## Concerns and limits

- The live queue fixture has four queued rows belonging to one channel. Real browser channel-filter selection correctly retains four rows and agrees with the API; actual narrowing across distinct channels is covered by the focused consumer tests, not claimed as a multi-channel browser observation.
- The live queue rows do not expose source URLs. Native external source-link preservation is covered by the consumer suite; the browser exercised the native internal channel anchor mixed with commands.
- Existing `channels.test.tsx`/queue tests emitted act-environment warnings and Explore fixture empty-image/localhost:3000 connection diagnostics during the verbose run. Tests and final typecheck passed; no warning suppression or unrelated shared/test-infrastructure edits were introduced.
- No deployment, production state, protected ports, personal HOME, sibling-owned source, global documentation or app-wide V33 completion is claimed.
