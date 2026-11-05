# Extra Library state controller — prepared, not executed

Status: DONE_WITH_CONCERNS. Only `extra-states.cjs` and this note were authored. No application source, controller.cjs, artboard, authentication state or Git state was modified. No browser, server, test, build, formatter or script syntax check was run by this worker.

## Parent execution

Run the executable script with the existing guarded current build already available at `http://127.0.0.1:8043`:

```sh
node /var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v38-acceptance-xxkx9151/parallel-acceptance-20261104/template-tweak-closure/extra-states.cjs
```

The Playwright package is resolved from `/Users/dat/Projects/verse/package.json`; Chrome defaults to the same local application executable as the existing controller. `EXTRA_STATES_CHROME` may select an already installed executable; the script never installs or downloads one.

If this build requires an existing authenticated session, the parent may supply its existing private browser-cookie JSON path in `EXTRA_STATES_COOKIES`. No credential file was inspected during preparation. There is no hard-coded session path or credential value. The script does not create or modify authentication; without an authorized session, authentication-required UI assertions will fail rather than manufacturing a user.

Runtime artifacts go into `template-tweak-closure/extra-states/`: one screenshot per case (or a failure screenshot), plus `results.json`. This separate namespace does not overwrite the peer controller's artifacts. Browser and per-case contexts close in `finally`. A failing assertion, page error, route exception or download sets a nonzero exit code. All successful cases still produce `DONE_WITH_CONCERNS`, never native acceptance.

## Prepared coverage and provenance

| Viewport | Cases | Canonical reference handling |
| --- | --- | --- |
| 1440×900 | Import success and partial failure, three sequential locally fulfilled requests each | `Library.import.done`, `Library.import.partial` |
| 1440×900 | Named slowroads, nightdrivefm, bilibili, lofilanes channel routes | `Library.kenh.slowroads`, `Library.kenh.nightdrivefm`, `Library.kenh.bilibili`, `Library.kenh.lofilanes` |
| 1440×900 | New downloading video card, 62% metadata progress | `Library.nguon.addvideo` |
| 390×844 | Import success and partial failure through the real mobile new-item sheet | Supplemental captures: desktop comparison IDs appear only in `comparisonTargetIds`, not mobile `referenceIds` |
| 390×844 | Actual mobile backdrop list with all four labelled channels and new video metadata | `MLibrary.tab.backdrops` only |

There are ten prepared cases. Every case and screenshot records `SYNTHETIC_LOCAL_FIXTURE_UI`, `nativeBackend: false` and its reference IDs. The canonical manifest is loaded at runtime; all referenced IDs must exist. Every viewport assertion checks the requested dimensions, no horizontal document overflow and no stacked interactive dialogs. Screenshots are comparison inputs, not pixel-parity verdicts.

The manifest has no `MLibrary.import.done/partial`, `MLibrary.nguon.addvideo` or `MLibrary.kenh.*` references. The mobile implementation uses source rows, not desktop ChannelPage/VideosPage; its generic list capture does **not** claim desktop selected-channel, pause, pin or downloading-progress parity. No invented mobile references are used. Wrong-extension import.zip is intentionally not repeated.

## Source contracts and selectors

- `web/src/api/library.ts:344–361`: JSDoc-linked `TemplateImportItem` / `TemplateImportResult`, multipart `file` and POST `/api/templates/import`.
- `web/src/screens/library/TemplateImportDialog.tsx:53–114,157–223,233–240`: one request per file, done/partial computation, media reuse summary, warnings and final actions. Assertions use current source copy (`mẫu video`), not older artboard `template` wording.
- `web/src/i18n/library.ts:125–158`: exact import button/title/status/media/warning/action copy.
- `web/src/api/backdrop.ts:43–100,137–152`: JSDoc-linked channel, catalogue page, catalogue item and standalone video types. The response bodies are arrays for channels/videos, and `{items, counts, total, unavailable}` for catalogue pages.
- `web/src/screens/library/backdrops/ChannelPage.tsx:139–218`: labelled channel region, heading, provider hyperlink (asserted, never opened), selected source rail, kind counts, usage metadata, four pinned Nightdrive cards, paused Bilibili refresh and empty Lofilane usage.
- `web/src/screens/library/backdrops/SourceRail.tsx:19–26,55–63,127–135`: actual current-source selector and Date.now-based pause predicate.
- `web/src/screens/library/backdrops/VideosPage.tsx:34–53`: addvideo is an already-downloading standalone card, **not** the parse/segment form. The script enters the real `nguon=videos` route with local downloading metadata; it does not parse or add a URL.
- `web/src/i18n/backdrop.ts:95–105`: exact Video lẻ heading and downloading group/text.
- `platform/ui/src/feedback.tsx:40–69`: actual progressbar role and `aria-valuenow` contract.
- `platform/ui/src/Dropzone.tsx:176–212`: file names and warning count control.
- `web/src/screens/library/mobile/MLibraryScreen.tsx:274–278,386–405,480–481,562–592`: actual mobile import action/dialog and source-list rendering.
- `platform/ui/src/mobile/sheets.tsx:134–148`: action descriptions contribute to accessible button names; the mobile import selector therefore matches the source label prefix rather than an incorrectly exact label.
- `Library.dc.html:1378–1392,1533–1561`: reference file identities, media counts/reuse, warning/error examples, named channels, usage modes, catalogue titles/durations/counts and new snow video. These values are adapted into typed, explicitly synthetic metadata, not discovered native records.

All displayed channel/video fixture titles have `SYNTHETIC UI metadata`; fixture filenames start `SYNTHETIC-`; warning/error text also identifies itself as synthetic. Provider URLs in canonical channel metadata are inert links: no link is clicked and all cross-origin requests are aborted. Avatars/thumbnails are null. Ready companion videos deliberately have no `file` object, preventing fabricated file paths or media loads.

## Safety boundary

- The only request continuation site is guarded same-origin GET. An explicit shell GET allowlist is retained; unlisted API GETs are aborted.
- **Every nonGET is aborted by default.** The only exception is POST `/api/templates/import` while the case explicitly enables done/partial fixtures. It checks multipart content type and the expected synthetic filename, then uses `route.fulfill`; it never continues/fetches the request. No other POST/PATCH/DELETE can be fulfilled successfully.
- All catalogue, template-list, prompt/overlay-list, platform and standalone-video fixtures use local `route.fulfill`. The template list stays intentionally empty even after fixture success: no persistence or native imported record is manufactured.
- Cross-origin network, WebSockets, SSE/eventsource, media/file/export/download/discovery/parse paths are blocked. Downloads are disabled and cancelled if emitted. Popups close. Service workers are blocked. No native discovery, inference, generation, trial, provider request, source download or real import is started.
- Request reports store method and pathname only. They do not store cookie/header values, request bodies, file bytes or query strings. External URLs are redacted.

## Missing prerequisites / limitations

1. **Native import success remains unproven.** Valid exported Viderse bundles, permission to perform a real import, actual backend ZIP parsing/media ingest and a native persisted template record would be needed. This assignment explicitly forbids that path. The selected files are empty ZIP fixture carriers, never valid-export claims.
2. **Native named-channel provenance remains unproven.** Real stored channel/catalogue/usage records for these identities and permission for discovery would be needed. No such records are created, updated or inferred.
3. **Native download remains unproven.** The new card's state/progress is GET metadata only. No add-video POST, parser POST, provider fetch or actual file transfer is allowed. Companion ready records omit files on purpose.
4. **Canonical visual parity remains unproven.** Neutral art areas, explicit fixture labels, changed source copy, synthetic timestamps and omitted real media are visible differences. Canonical counts represent the reference catalogue while fixture pages contain only the displayed reference subset; no pagination completeness is claimed. Parent must separately compare screenshots to the canonical boards.
5. **Bilibili pause uses a runtime future time** to reach the real date-based pause branch. It does not claim a real provider quota, historical pause or exact reference clock time of 02:10.
6. **Execution prerequisites:** current guarded build at port 8043, existing installed Playwright/Chrome, readable canonical manifest and (only if required by that build) parent-supplied authorized browser session. This worker has not exercised these prerequisites or any prepared assertion.
