# Editor panels shared-kit implementation — 2026-11-02

Status: DONE_WITH_CONCERNS

## Outcome and source

Completed the assigned V33 editor controls migration without changing APIs, project persistence, clip replacement commands, scene versions, or undo behavior. The genuine vertical workspace rail remains native and retains vertical roving navigation and panel relationships. No popup menu or native select exists in the assigned consumers; no artificial ActionMenu or KitSelect was introduced.

Work context: `/Users/dat/Projects/verse-controls-editor`, branch `fix/viderse-editor-panels-kit`, base `d89a52e3a6982020d860d726732dc39c0f5341e9`.

Source commits, in order:

1. `2ba65787` — `refactor(viderse): use kit controls in editor panels`
2. `bfa3cf44` — `refactor(viderse): use kit subtitle search field`
3. `646158ef` — `fix(viderse): keep media picker actions within viewport`
4. `86bfe090` — `fix(viderse): let media filters fill the editor panel`

All commits used a fresh private index initialized with `read-tree HEAD` and explicit owned paths. No main merge was performed. The parent integrated these changes independently. Final browser proof ran against the parent's main-backed server, with the parent's generic Dialog focus repair `54296785` and filter integration `e45fa229`; that shared Dialog repair is not duplicated in this branch.

## Files changed

Paths below are relative to `apps/viderse/web/src/editor/v13/`; counts are final file lines, not diff sizes.

| File | Lines | Change |
| --- | ---: | --- |
| `ContentPanels.tsx` | 504 | Media kind filtering uses Segmented; reroll uses icon Button; subtitle search uses TextField; MediaPicker content is scroll-bounded. Exported helpers and mutation behavior remain intact. |
| `ContentPanels.stylex.ts` | 47 | Removed obsolete filter/button/search styles; retained native visual selectors; added picker content containment; allowed the filter to fill its layout container. |
| `ContentPanels.test.tsx` | 56 | Two real-component filter tests cover image/overlay separation, empty kinds, keyboard selection, and roving focus without mocking kit components or insertion behavior. |
| `Editor.tsx` | 273 | Header, history commands, notices, narrow-screen actions, conflict actions, and scene-swap undo use Button variants without button style overrides. |
| `editor.stylex.ts` | 89 | Removed obsolete command styles; retained genuine workspace/cue styles and a dedicated native download-link style. |
| `FixPanel.tsx` | 60 | Alignment, disclosure, and measurement commands use Button; timeline cue navigation stays native. |
| `Shortcuts.tsx` | 12 | Trigger and dismissal use Button; shortcut registration/bindings remain unchanged. |
| `ExportDialogs.tsx` | 41 | Cancel/back commands use Button; native video overlay and actual download anchor remain intact. |
| `ExportGateDialog.tsx` | 33 | Confirmation commands use Button; acceptance and continuation behavior remain unchanged. |

The parent explicitly authorized the lowercase `editor.stylex.ts` and the four adjacent consumers beyond the original two TSX files. `rail.tsx` and uppercase `Editor.stylex.ts` do not exist. Inspector, SceneImagePanel, scene-images, APIs, engine, storage, and palette were not edited.

## Remaining genuine native controls

| Surface / selector family | Reason for retaining native semantics |
| --- | --- |
| `Editor`: vertical `nav[role=tablist]`, `railButton`, `railSelected`, `railIcon`, `railLabel` | Icon-over-label workspace navigation is not a horizontal text filter or underline tab strip. It preserves ArrowUp/ArrowDown/Home/End, one tab stop, `aria-selected`, `aria-controls=ed-left`, and the panel's `aria-labelledby`. |
| `MediaPicker` and `MediaPanel`: `mediaCard`, `thumbnail`, `mediaThumbnail` | These are visual media-card grids containing actual thumbnails, not ordinary text choice groups. Selection and insertion behavior are unchanged. |
| `BackgroundList`: `backgroundPick` | A clip-thumbnail row selects a concrete timeline clip and playback position; the separate reroll command now uses kit Button. |
| Scene list: `row` with thumbnails and timing | A visual timeline clip browser, not a finite text filter. |
| Subtitle list: `row`, `subRow`, `time`, `subText` | Transcript cue navigation seeks a specific timeline position; arbitrary multi-line cue content is not a regular text choice group. |
| `FixPanel`: `cueButton`, `selectedCue`, `cueTime`, `cueText`, `cueStatus` | Timeline problem-range navigation preserves cue timing/status and existing selection behavior. |
| Audio panel: native `input[type=range]` / `slider` | Native continuous editor gain control retains min/max/step and the existing undoable audio command. |
| Hidden native file input | Required browser file-selection boundary; the visible import command is kit Button. |
| Export preview: native video controls and canvas-overlay play button | Media playback surface with actual video play/rejection handling, not an app command group. |
| Export download: `a[href][download]` / `downloadLink` | Real download navigation remains a native anchor. The installed Button API is button-only; no window.open or synthetic navigation replaced it. |

No application height, padding, background, border, or font overrides are applied to migrated kit Buttons. Segmented uses the kit's sliding thumb and reduced-motion rules. Source `platform/ui/src/kit.css:113-127` restricts transform transitions to no-preference and removes transform animation under reduce; the browser also exercised reduced-motion emulation without adding application motion rules.

## Exercised gates

- Focused editor consumer/V29 suite: **32/32 passed**, covering ContentPanels (2), ExportDialogs (5), ExportGateDialog (3), scene-images (15), and quality (7).
- After the authorized export changes: affected export consumers **8/8 passed**, followed by full `npm run typecheck` passing.
- After search, picker containment, and final filter layout changes: the affected ContentPanels consumers **2/2 passed** and full `npm run typecheck` passed for each coherent source change. The last such run tested `86bfe090`'s source before its commit.
- Design rules: **15/16 passed**. The sole failure was `screens/review/mobile/mreview.stylex.ts: metaSep` under the fg-5 separator-name rule. That source and the check are outside this assignment and were not modified or repinned. The unchanged failure was not rerun.
- No install, build, formatter, or broad engine test sweep was run. Canonical installed dependencies were linked from the main worktree at the three authorized node_modules paths.

## Actual browser evidence

Both batches used one owned Chrome tab/context, the existing parent-owned API `8053` and main Vite `8059`, an isolated `editor-kit.localhost` origin, and the provided real API fixture. Network behavior was not mocked. Authentication material was read privately and is not included in these artifacts.

Initial granted batch, 1440×1000:

- Actual media records: 12 images, 1 background video, 0 overlays, 2 audio records. Keyboard selection changed the rendered media choices from 12 images to 1 video; click selection reached the empty overlay kind.
- Sliding thumb reported a transform transition of 0.3 seconds. Reduced-motion emulation reported only the kit's three 0.2-second color transitions, consistent with the source's transform suppression.
- Vertical workspace rail ArrowUp/Home/End changed panels and focus; exactly one rail tab stop remained, with the real controls/labelledby relationship.
- Undo and redo were disabled on the fresh project session. Shortcuts opened and Escape dismissed the real dialog.
- The bounded 12-file MediaPicker allowed the last row and footer close button to be reached.
- No page errors occurred.
- Found two real issues rather than masking them: cramped filter layout, corrected by `86bfe090`; missing close/Escape focus restoration, corrected centrally by the parent in shared Dialog. No local focus shim remains.

Final granted batch against the integrated parent source:

- Filter bounds: **287×36**, using the available panel width; labels were readable and separated. See `04-media-filters-final.png`.
- At **1440×1000**, all 12 real picker assets remained available. Dialog bounds were **x=440, y=19, width=560, height=962**. The last row and footer were reachable; footer close restored focus to the exact original Thay ảnh trigger.
- At **1440×900**, dialog bounds were **x=440, y=19, width=560, height=862**. The last row and footer were reachable; Escape restored focus to the exact original trigger.
- No page errors occurred. Both batches closed the owned browser/context in finally and released the browser slot. Parent services were left untouched.

Evidence directory: [`261102-editor-panels-kit/`](./261102-editor-panels-kit/).

- [`browser-evidence.json`](./261102-editor-panels-kit/browser-evidence.json): initial functional proof and the honest pre-fix focus finding.
- [`browser-final-evidence.json`](./261102-editor-panels-kit/browser-final-evidence.json): successful final integrated proof.
- `01-media-filters.png`, `02-picker-last-row.png`, `03-editor-after-picker.png`: initial native screenshots, retained as historical evidence rather than presented as the final UI.
- `04-media-filters-final.png`, `05-picker-final-1000.png`, `06-picker-final-900.png`: final native screenshots, visually inspected.

## Concerns and limits

The out-of-scope design-rule failure remains a parent integration concern. This assignment contains no popup menu, so no menu keyboard smoke is claimed; the exercised keyboard surface here was the actual shortcuts Dialog. Final focus restoration depends on the parent's shared Dialog repair, intentionally not cherry-picked into this stale isolated branch. No project mutations, API behavior changes, source-version swaps, or synthetic fake media behavior were introduced by these browser checks.
