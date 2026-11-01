# Settings and Explore V33 kit cutover

Status: **DONE_WITH_CONCERNS** — the owned implementation is complete. Browser acceptance is partial for the explicitly documented fixture/authentication boundaries below. This report does not declare top-level V33 complete or deployed.

## Source commits and ownership

Branch: `fix/viderse-settings-explore-kit` in `/Users/dat/Projects/verse-controls-settings`.

Apply these source commits in order:

1. `0d9508db` — `fix(ui): contain intrinsic button content without changing alignment`. Parent-authorized, exclusive `platform/ui/src/Button.tsx` change: three layout declarations only, +3/-3. The cell is bounded; normal and pending slots can shrink. No width-100% content group, public prop, visual dimension, variant, or ref change.
2. `2ff65cea` — `fix(ui): contain select option labels beside metadata`. Parent-authorized, exclusive `platform/ui/src/Selects.tsx` change: +1/-1. CSS ellipsis contains long labels without changing full DOM/accessibility text or metadata layout.
3. `8537035a` — `fix(viderse): complete settings and explore kit controls`. The owned consumer cutover: 42 files, +548/-943. The only additional non-owned file is the parent-authorized deletion of the obsolete native-select-overlay assertion in `src/checks/design-rules.test.ts`.

No API, dependency, shell, workbench, palette, or production deployment files were changed. No packages were installed. The ordinary git index was not used: each commit used a fresh private index initialized with `git read-tree HEAD` and explicit file paths.

## Implemented behavior

- Notification switches now use kit `Switch`, including persisted settings and rejected-save feedback. Permission-request failures are reported rather than silently rejected.
- ModelCatalog text filters use the kit sliding `Segmented`; model toggles use kit `Checkbox`.
- YouTube profile and S3 region fields use `KitSelect`. Existing/custom stored regions remain selectable. A rejected YouTube settings write restores the persisted selection.
- Model pickers use grouped searchable kit `Combobox`, preserving provider/plan labels, real model references, capability guards, disabled reasons, reasoning/context notes and actual cost metadata. The optimistic agent document remains available through resource refresh, avoiding a stale selection/save base during immediate saves.
- Command and icon actions use kit `Button` variants/sizes without old consumer height/padding/background/border/font overrides. The provider connection handler also guards missing required input before making a request.
- Provider/storage choice grids and image-provider cards remain descriptive kit `RadioCard` choices. They were not collapsed into text segments.
- Explore time/cap key:value choices use `ChipSelect`. Bulk actions use the existing kit motion menu. Genuine navigation anchors remain anchors.
- The mobile agent sheet reserves real dropdown layout space and has a 16px body inset. Its final model option is reachable instead of being clipped outside an auto-height sheet.
- The accepted Explore sheet padding remains unchanged: **0px vertically and 16px horizontally**. Filter controls wrap within the mobile page rather than clipping their listboxes in a horizontal scroll container.
- Removed obsolete custom control styles and source/style/wording implementation assertions. Permanent tests exercise consumer behavior, not CSS source or mock echo. The agent fixture now persists writes for subsequent reads; provider creation uses distinct identities.

## Verification actually run

Final coherent consumer gate after the Button and sheet repairs:

```text
npm exec vitest -- run src/screens/settings src/screens/explore src/checks/design-rules.test.ts --maxWorkers=1
21 files passed; 206 tests passed, including 15 design-rule tests.
npm exec tsc -- --noEmit
Passed.
```

After the final Selects label-only repair:

```text
npm exec vitest -- run src/screens/settings/agents/AgentsSection.test.tsx src/checks/design-rules.test.ts --maxWorkers=1
2 files passed; 27 tests passed.
npm exec tsc -- --noEmit
Passed.
```

Coverage includes notification save/rejection, immediate model selection and real reference saving through keyboard/search, incompatible capability blocking, serialized agent saves, provider form guards, catalog filter/choice behavior, S3 region selection and invalidation of a previous connection check, existing storage safety/races, and desktop/mobile Explore consumers. No percentage coverage run is claimed. Existing React `act(...)` environment warnings occurred in the larger DOM suite; they were not suppressed.

The full shared-kit suite was not run here; the parent owns its integrated gate.

## Native browser evidence and limits

All PNGs were produced directly by Playwright `page.screenshot({ fullPage: true })` in an independent temporary Chromium, not a shared CDP tab or synthetic artboard. There was at most one owned active context/tab. Browser cookies were imported from the existing isolated fixture jars with the correct unique-host domain and were never printed.

The temporary Vite server used only reserved port 8058 at `settings-kit.localhost`, first targeting existing fixture 8051 and then parent-approved populated fixture 8047 with an Origin rewrite. No real provider credentials, protocol connections, external provider writes, fake network responses, new DB seed, or production state were used. Fixture 8047 was read-only. Safe-area variables were set to top59/bottom34; reduced-motion media was enabled.

Evidence directory: [`2026-11-02-settings-explore-kit-evidence/`](2026-11-02-settings-explore-kit-evidence/).

| Evidence | Observed result |
| --- | --- |
| `desktop-1440-agents.png`, `desktop-1440-connect.png` | Real desktop settings rendering before the later containment repairs. |
| `desktop-1440-connect-form-after.png` | Real guarded provider form and ordinary desktop primary CTA: height32, centered content group, zero center delta. |
| `mobile-390-safe59-34-agents.png` | Preserved BEFORE image showing the genuinely clipped auto-height model sheet. |
| `mobile-390-safe59-34-agents-after.png` | After Button/consumer sheet repair: page width/scrollWidth390/390, body padding16px, list bottom630.5 within body bottom784; last option fully reachable. |
| `mobile-360-safe59-34-agents-after.png` | After Button/consumer sheet repair: page width/scrollWidth360/360, body padding16px, list bottom584.5 within body bottom720; last option fully reachable. |
| `settings-ui-proof.json` | Actual sheet, CTA and guard measurements. Mobile ordinary CTA height48 with center delta0.0078125, guarded on missing endpoint/key. These captures precede the final Selects ellipsis-only commit. Empty premature row samples were removed and are not claimed as mounted-row geometry proof. |
| `desktop-1440-explore.png`, `ui-proof.json` | Fixture8051 real empty Explore state, width/scrollWidth1440/1440 and reduced-motion media. |
| `desktop-1440-explore-populated.png` | Real populated fixture8047 desktop video grid, rival choices and timeline. |
| `explore-ui-proof.json` | Real desktop filter changes immediately displayed `7 ngày` / `Tất cả`; actual `/videos` GETs returned200 with `within=7` / no within filter. |

An earlier populated390 run also displayed `7 ngày` immediately and completed the real filtered GET200. Its sheet computed `0px 16px`, correctly preserving the accepted horizontal inset; the temporary driver incorrectly expected all-side16 and stopped before taking that PNG. This was a driver assertion error, not a padding regression; the source padding was not changed to satisfy it.

The last authorized read-only batch encountered the shared fixture's first-run gate while inspecting the desktop preview: the page switched to onboarding, **5/6 complete, ChatGPT login required**. The browser then closed. Therefore these are **not claimed**:

- Final native proof of the post-`2ff65cea` option ellipsis/readability repair.
- Populated Explore native390/360 PNGs from the final source.
- Mounted descriptive-row bounding measurements on the final source.
- A browser change to a different eligible model: fixture8051 had only one eligible stored model. Its existing stored selection was displayed; the immediate-change/save path is covered by the consumer tests.
- A protocol-authenticated provider connection or populated ready-provider ModelCatalog browser flow. Those prerequisites were absent; no fake connection was introduced.

All owned browser contexts and the temporary8058 server were closed. Existing8051/8047 backends and protected ports8790/5391 were left untouched. Parent integration must exercise the remaining latest-build UI/protocol boundaries using an authenticated, root-ungated approved fixture.

## Intentionally preserved genuine interactions

- `PostingCard.tsx` posting-time slots and the activity heatmap are timeline/data interactions, not text filters.
- Desktop/mobile video thumbnail and title buttons are descriptive media-card preview/navigation interactions; their grids and virtualization remain intact.
- Image-provider and provider/storage choices remain descriptive RadioCard grids; PreviewPanel channel choices retain their avatar/name/context rather than becoming text segments.
- Real anchors remain for queue/member navigation, account/authentication URLs, error-log destinations, create/open-channel and existing-job navigation, YouTube sources, and mobile channel-source navigation. They were not replaced with `window.open` solely for kit appearance. Plain action-link styling was removed from the old button costume where appropriate.

## Complete consumer source inventory

Paths below are relative to `apps/viderse/web/src/`. Counts are added/deleted lines in `8537035a`.

| File | + / - |
| --- | ---: |
| `checks/design-rules.test.ts` | 0 / 17 |
| `screens/explore/AddRivalForm.test.tsx` | 1 / 1 |
| `screens/explore/AddRivalForm.tsx` | 5 / 6 |
| `screens/explore/BulkBar.tsx` | 15 / 35 |
| `screens/explore/ExploreScreen.test.tsx` | 10 / 23 |
| `screens/explore/ExploreScreen.tsx` | 3 / 3 |
| `screens/explore/FilterBar.tsx` | 15 / 32 |
| `screens/explore/PostingCard.tsx` | 7 / 10 |
| `screens/explore/PreviewPanel.test.tsx` | 0 / 9 |
| `screens/explore/PreviewPanel.tsx` | 18 / 55 |
| `screens/explore/RivalCard.tsx` | 13 / 25 |
| `screens/explore/RivalSidebar.tsx` | 9 / 10 |
| `screens/explore/ScanCard.tsx` | 5 / 22 |
| `screens/explore/SearchBox.tsx` | 3 / 5 |
| `screens/explore/VideoGrid.tsx` | 6 / 9 |
| `screens/explore/explore.stylex.ts` | 1 / 16 |
| `screens/explore/mobile/MExploreScreen.tsx` | 10 / 36 |
| `screens/explore/mobile/mexplore.stylex.ts` | 2 / 43 |
| `screens/settings/NotificationSettings.test.tsx` | 78 / 0 |
| `screens/settings/NotificationSettings.tsx` | 7 / 9 |
| `screens/settings/SettingsScreen.test.tsx` | 3 / 16 |
| `screens/settings/SettingsScreen.tsx` | 20 / 21 |
| `screens/settings/accounts/AccountsSection.test.tsx` | 4 / 37 |
| `screens/settings/accounts/BilibiliLogin.tsx` | 1 / 1 |
| `screens/settings/accounts/QrLogin.tsx` | 3 / 3 |
| `screens/settings/accounts/YouTubeLogin.tsx` | 6 / 7 |
| `screens/settings/agents/AgentsSection.test.tsx` | 46 / 29 |
| `screens/settings/agents/AgentsSection.tsx` | 31 / 34 |
| `screens/settings/agents/ModelPicker.tsx` | 41 / 203 |
| `screens/settings/connect/AddProvider.tsx` | 13 / 11 |
| `screens/settings/connect/ConnectSection.test.tsx` | 50 / 27 |
| `screens/settings/connect/ConnectSection.tsx` | 6 / 12 |
| `screens/settings/connect/ModelCatalog.tsx` | 16 / 37 |
| `screens/settings/connect/ProviderCard.tsx` | 31 / 34 |
| `screens/settings/connect/YoutubeDataKeyCard.tsx` | 6 / 6 |
| `screens/settings/settings.stylex.ts` | 14 / 66 |
| `screens/settings/storage/S3ConfigForm.tsx` | 5 / 5 |
| `screens/settings/storage/ServerStorageSection.test.tsx` | 23 / 0 |
| `screens/settings/storage/StorageProviderPicker.tsx` | 4 / 4 |
| `screens/settings/storage/StorageSection.tsx` | 12 / 18 |
| `screens/settings/storage/serverStorage.styles.ts` | 0 / 1 |
| `screens/settings/tools/ToolsSection.tsx` | 5 / 5 |
