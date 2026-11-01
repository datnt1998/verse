# MMain — mobile workbench and desktop kit convergence

## Slice handoff before controller integration

- Assigned source is complete on `feat/viderse-mobile-main`, worktree `/Users/dat/Projects/verse-mobile-main`, final source `591d3e52`.
- Final source gates passed: full web typecheck; **16 focused files / 113 tests**; production build to `/tmp/mmain-final-dist`.
- Managed tab `MMainFinal` was **closed and released**. No MMain browser context remains open.
- Main explicitly retains final integrated **mobile More-menu** browser proof. No further browser grant was given; the old More screenshot was marked stale, not recycled as current proof.
- Main owns integration, root/shared-brand reconciliation, durable gap entries, final dist and deployment. This report and evidence were written under `plans/reports`; they were not committed from the controller's concurrently changing main worktree.

## Evidence inventory

Durable manifest: `plans/reports/mmain-evidence-final/manifest.json`.

- **137 native PNGs**: 127 actual-application captures and 10 clearly identified canonical references.
- Original matrix: `/tmp/mmain-final-shots/manifest.json`.
- Every PNG has exact viewport dimensions, byte count and SHA-256 in the manifest. Validation found **zero dimension mismatches and zero safe-area filename mismatches**.
- Images were captured using native `page.screenshot`, DPR 1, not normalized browser-tool thumbnails or reconstructed images.
- Phone matrix: 390×844 and 360×780, each with real CDP safe-area env overrides 0/0 and 59/34. No fake OS status text was added.
- Matrix includes all/scoped list, switcher, review detail, image grid, v1/v2 filmstrip, scene/cover prompt estimates, metadata pending/writing/failed/AI/edited, why, edit-bottom, rewrite, history, learner, actual plan, recap, jobs and inbox.
- Captures were collected incrementally. Changed list/progress/recap and pending/failed surfaces were rebuilt and retaken at final source. Earlier unchanged surface snapshots retain their actual provenance; no claim that all 137 were taken at the same instant.

Manifest SHA-256:

- Durable: `d380ce115b300691e4a205ef66778be35270253574ef330a3b35ad68f5a0c133`
- Original `/tmp`: `135f5883174af86072b72fdea4bfa4587199eaa6f6688356ec72a0ccb9e1e591`

### Critical final native paths

All filenames below are in `plans/reports/mmain-evidence-final/`:

| Proof | Native file |
|---|---|
| Final built inline daily progress and real channel cards/mode | `360-safe0-list-all.png`, `390-safe59-list-all.png` |
| Desktop selected all-avatar stack retained with trailing tick | `desktop-switch-all.png` |
| Desktop selected individual channel avatar and tick | `desktop-switch-channel.png` |
| Actual desktop workbench and Phòng vẽ | `desktop-workbench-live.png` |
| Actual kit metadata panel / actions / prompt picker | `desktop-content-panel.png`, `desktop-workbench-actions.png`, `desktop-prompt-selector.png` |
| Final safe fullscreen recap | `390-safe59-recap.png`, `360-safe0-recap.png` |
| Safe sticky heading and reachable bottom composer after scrolling | `390-safe59-recap-bottom.png`, `360-safe59-recap-bottom.png` |
| Real saved rule with still-reachable Undo | `360-safe59-meta-learn-saved.png` |
| Native Undo completed and persisted rule removed | `360-safe59-meta-learn-undone.png` |
| Genuine classifier plan and real completed operation | `360-safe59-plan-native.png`, `360-safe59-plan-done.png` |
| Actual failed metadata recovery | `390-safe59-meta-failed-sheet.png`, `390-safe59-meta-retry-success.png` |

These critical files were decoded through `read` and inspected, rather than trusting their filenames.

## Phòng vẽ: exact claim and limit

`desktop-workbench-live.png` is an **actual application capture**, not an artboard replacement. It shows three real published fixture images in review, the canonical 2px artist progress, Orb/title, true **3/3** count, tile badges, narration/timing, real tile-action buttons, and resolved kit panels/composer.

The old statement that this pane remains a legacy `!image && running` renderer is no longer source-correct. `LiveStudio.tsx` now renders the shared `DevelopEffect` for **running, pending or queued even when a prior review/current image exists**. The old version is passed as `baseUrl`, not discarded. No duplicate screen-local effect was created.

- `LiveStudio.test.tsx`: all three retained-image redraw statuses are regression-covered; a single tile redraw is checked against the real endpoint shape and manifest revision.
- `DevelopEffect.test.tsx`: the canonical component retains its base image, compact state and reduced-motion safeguards.
- **Do not describe the desktop PNG as a separate native running-redraw capture.** Its native state is ready/review 3/3. Retained-version drawing behavior in this slice is consumer-regression verified; live upstream image-generation quality was not tested.

## Source changes

- Phone per-scope scroll memory restores after cached switching, detail/back and tab hops, including delayed placeholder data.
- Real image filmstrip/version prompts, roving keyboard focus and scene/cover selection; internal `sc-f…` identifiers are not displayed as scene numbers. Cover has the canonical full-row hierarchy.
- Metadata readiness states, real persisted reasons, owner-edited fields, locked footer/tags, edit/rewrite/history/copy and actual Learner interaction.
- Native metadata description editing strips the locked footer from the editable body and reattaches it exactly once; locked tags are preserved.
- Desktop workbench toolbar, prompt options, overflow menu, right panels, live pane, recap, ActivityPopover and desktop ChannelSwitcher use existing kit Button/Tabs/Segmented/RadioCard/ActionMenu contracts. True navigation/download links and genuine card/list selections remain semantic native elements.
- Corrected the real snapshot boundary to `library.Snapshot.templateName`, not nonexistent `templateSnapshot.name`; no alias shim was retained.
- Mobile switcher no longer uses a non-portal outside-click hook to close its portalled Sheet before a row's pointer click. Sheet owns scrim/Escape/focus; pointerdown→selection is regression-tested.
- Composer targets the actual pipeline agent: Scout for download/backdrop/transcript, Director for metadata/scenes, Artist for images, QA for build/score/export; review and paused-image guidance are retained. A data-driven consumer regression covers the step mapping.
- Learned-rule confirmation is scoped with required `ownerKey` at every caller. A next suggestion cannot erase the accepted action's Undo; changing channel/job/image cannot inherit an old confirmation. The new-prop regression failed before repair and passed afterward.
- Canonical DevelopEffect was integrated at its original shared application paths, with exact colors/noise in the existing palette and StyleX rendering. Main's selected-avatar fix `be924edc` is included; shared fullscreen Sheet cap fix `df3e5fd3` was cherry-picked before the consumer override.
- Fullscreen recap uses the existing native Sheet max-height contract and a safe-aware sticky content header. No inline sizing exception, specificity hack, fake status bar or new Sheet API was added.
- AppShell's explicitly delegated `waitingYou` channel projection is included. No root route/runtime bug was reintroduced or claimed against Main's prior fixes.

## Actual API and persistence smoke

All mutations below used the isolated real backend and normal public APIs. No frontend interception, seeded plan ID, product stub or fake fallback was used.

1. **Director classifier → plan → execute:** native composer created plan `b3b552b1-0fe7-4a34-bb83-b673405366e9`; native execution created run `e37efdb0-8c44-4c6f-af88-8598a40f5184`. The selected job changed from waiting/review version 23 to paused version 24. Native resume subsequently restored waiting/review version 27. Zero-selection disabled execution was captured.
2. **Metadata edit/history:** native owner edit persisted revision 6 with reserved footer and tags preserved; native history restore persisted revision 7 through the actual restore endpoint (200).
3. **Selective rewrite:** native title-only rewrite returned revision 8 and preserved description/tags. The approved external upstream is deterministic, so this is adapter/persistence integration, not a claim of live LLM quality.
4. **Failed recovery:** native retry on a genuinely expired writer returned metadata `ai`, revision 1, through the real rewrite endpoint.
5. **Learner:** actual three-job edits generated real suggestions. Native accept returned 200 and created a persisted rule. A distinct next suggestion arrived while the saved confirmation/Undo stayed visible. Native `/api/undo` returned 200 and the learned-rules read returned an empty list again. The pre-fix lost-Undo failure and normal API cleanup are preserved in manifest history.
6. **Clipboard:** native copy-package write was followed by a real clipboard read in the correct isolated browser context; expected title and tags were present.
7. **Notifications:** native mark-all-read returned 200 and changed the actual unread count from 17 to 0.
8. **Prompt estimates:** authored prompts were created through the real library API. Native selection called real estimate endpoints: two scene images or one cover image. Paid generation was not requested merely to obtain a screenshot.

## Measured navigation and safe areas

- Independent actual list scroll positions: all channels **650px**, selected channel **320px**; both restored correctly after native switcher selection.
- Actual list→detail started at 0, and native back restored **650px**.
- Actual Review-tab→Workbench-tab roundtrip restored **650px**; 60 sampled cached-transition frames contained **zero workbench loading frames**.
- The forced-bottom last job cleared the bottom navigation.
- Final built recap frame is exactly **0…844** or **0…780**, not the former 86% cap.
- At native 59/34, title top is **76px initially and 68px after scrolling**; composer bottom is **742px at 390×844** or **678px at 360×780**, above the home safe area.
- At safe0, title top is **17px initially and 9px after scrolling**. All four final recap clearance assertions passed.

## Build and bundle evidence

Final standalone slice build:

- Entry: `/assets/index-0KqkSeCT.js`
- Raw: **431,899 bytes**
- gzip: **138,707 bytes**, below the tightened **150 KiB / 153,600-byte** root gate.
- Output: `/tmp/mmain-final-dist`
- Final source test/build output: `artifact://70273` (`bg_1066`). Existing third-party zod PURE annotations and the existing time-stretch static/dynamic-import warning remain visible, not suppressed.
- Built phone preview used an explicit `VITE_API_TARGET=http://127.0.0.1:8041`. Its actual resource list requested **no WorkbenchDesktop, WorkbenchPanels or AssistantComposer chunk**.

## Fixture boundaries, cleanup and controller follow-through

- Only `/tmp/mmain-home`, `mmain_data`, `mmain_identity`, backend 8041 and assigned local frontend/fixture ports were used. No production keys or personal HOME/profile were retrieved.
- Director smoke uses the controller-approved external HTTP fixture upstream through the real provider adapter/classifier and normal in-memory plan/run APIs. It does **not** establish live-provider response quality.
- Image/video inputs are authored deterministic media published through the real isolated render fixture machinery, not injected frontend UI state.
- The first genuine 30-minute writing hold expired before capture. Replacement jobs were admitted through StartJob and Runner; no old job was relabelled or SQL-reset. All final held writers were released normally and exited 0.
- The temporary repository Go fixture test was removed. No permanent mock wiring or fixture scaffolding was committed.
- Fixtures/services are retained at Main's explicit request for final integrated More-menu evidence. MMain browser is closed.
- Canonical MMain has fixed drawn 44px status/30px home geometry and no safeTop/safeBottom props. It has no image-sheet screen; unsupported blank reference captures were removed instead of being called proof. The stage=images reference retained a metadata/log scroll position and was honestly named `reference-390-meta-ai-log.png`.
- The final built image showed purple selected-card/nav accents under current shared token aliases, unlike earlier pink dev captures. This concrete integration observation was reported to Main, who owns root/shared-brand reconciliation; no root/theme files were changed to conceal it.
- Preserve Main's newer review renderer/styles on integration; this slice's review-file changes are only the required LearnSlot owner-key caller props.

## Controller integration receipt

- Final integrated production bundle `index-9ZyA7gxg.js` passed the 150 KiB entry gate; no editor code occurs in its static entry graph.
- Native final-built 390×844 and 360×780 More sheets were exercised after integration: every row remains within the viewport, each row is at least 44 px high, Escape restores the exact opener, and the channel anchor navigates to the real channel route. The controller corrected the missing title/body inset by using the shared Sheet title and 16 px body inset.
- The compiled brand now uses `:root[data-brand="viderse"]`, so appended StyleX defaults cannot replace the app accent. Actual 360 px Agents capture and computed `--color-accent: #FF4D63` confirm pink selected controls, not the stale purple build.
- Final-built model sheets were opened at both widths; options remain inside the 16 px body inset and the last option is reachable. Labels are clipped with ellipsis beside bounded capability/reason metadata; full accessible text is retained.
- Controller native proof: [`../261102-final-built-mobile/`](../261102-final-built-mobile/), including JSON geometry, native PNGs and SHA-256 hashes. All contexts closed after proof. This receipt does not certify live-provider inference, device Safari or final production performance.

