# Cold entry cause — PR2 / V36

Investigation only; no source, build config, threshold, or measurement was changed. Evidence source: the local production `apps/viderse/internal/web/dist/.vite/manifest.json` plus the emitted chunks whose gzip sizes match the latest `npm run size` run (`index` 29.7, `responsive-overlays` 59.9, `shared-controls` 36.9, `client` 63.7 KiB). Chunk hashes below are from that dist; the stable identity is the chunk `name` in the manifest.

## Gate state

| Metric | Current | Limit | Baseline a734 |
|---|---:|---:|---:|
| entryJS | 190.2 KiB | 150 | 189.2 |
| initialJS | 203.4 KiB | 200 | 202.5 |
| initialCSS | 34.7 KiB | 40 | — |

This is pre-existing (the merged baseline already fails entry and initial), not a PR2 regression. The current increase is about 1 KiB. `scripts/size.mjs` still counts the phone-workbench chunk and the deferred `react-dom/client` boot chunk; nothing here proposes changing that accounting.

## What is actually in the cold closure

The manifest entry `index.html` imports exactly two chunks, then `size.mjs` adds the boot chunk:

| Chunk | gzip | Role |
|---|---:|---|
| `index` | 29.7 | App shell, router, session/data wiring, base i18n |
| `responsive-overlays` | 59.9 | Shared kit primitives, inlined cold-icon SVG data, app data/router modules, `Selects.tsx`, app `responsive-overlays.tsx` |
| `shared-controls` | 36.9 | Manual shared partition: `ConfirmDialog`, kit controls, `Tabs`, phone rows, screen i18n, `api/workbench`, assistant plan logic, etc. |
| `client` | 63.7 | `react-dom` loaded by `main.tsx`; genuinely required to render |

`zod`/editor code is **not** in this closure. Zod lives in `schemas-*.js` (98.93 kB raw / 27.99 KiB gzip) and is only reachable from editor/render chunks; the manifest entry has no import path to it. `editor-budget.json`/`size.mjs` also assert no editor module appears in a cold chunk. The leak is dialog/menu and shared-app co-location, not zod or editor code.

## Cause A — `RouteScreen` imports the whole component barrel

Chain:

```
src/main.tsx
  -> src/App.tsx
    -> src/screens/RouteScreen.tsx:8   import { SystemLoading } from '../components'
      -> src/components/index.ts:19    export { ResponsiveSelect, ... } from './responsive-overlays'
        -> src/components/responsive-overlays.tsx
          -> @noverse/ui Selects.tsx (KitSelect/ChipSelect/Combobox), Menus.tsx, Sheet.tsx ...
```

`SystemLoading` is defined in `src/components/states.tsx:60`; the barrel is not needed by the eager graph. The barrel also re-exports `./confirm` and `./DevelopEffect` (`src/components/index.ts:10-18`), but those already resolve to their own lazy chunks in the current dist and are not the cold problem. The cold problem is that the barrel module sits in the entry chunk and keeps its `responsive-overlays` re-export edge live.

All value importers of `ResponsiveSelect` / `ResponsiveChipSelect` / `ResponsiveActionMenu` are lazy route code:

- `src/components/suggestions/SuggestionCard.tsx`
- `src/screens/channel/TemplatesTab.tsx`, `VideoBulkBar.tsx`, `VideoRow.tsx`
- `src/screens/explore/ScanCard.tsx`, `screens/explore/mobile/MExploreScreen.tsx`
- `src/screens/new/NewScreen.tsx`

No eager module uses that adapter, and no eager module uses `KitSelect`/`ChipSelect` directly (the other direct users are `AutonomyCard`, `explore/BulkBar`, `FilterBar`, `RivalCard`, `YouTubeLogin`, all lazy).

Measured chunk evidence: the `responsive-overlays` chunk contains the `Selects` module (`listbox`/`combobox` markers, roughly bytes 170,000–188,000) and the app adapter at the tail (`role="combobox"`, `listbox`, `menuitem`, bytes 232,000–241,636). Gzip slices of those two regions are about 6.37 + 3.68 KiB, i.e. about **10.0 KiB** of cold JS that has no eager consumer. [INFERENCE: slice gzip is a close upper-bound estimate, not the exact chunk delta.]

**Smallest fix:** in `apps/viderse/web/src/screens/RouteScreen.tsx:8`, import `SystemLoading` from `../components/states` instead of `../components`. Keep `src/components/index.ts` as the lazy-screen barrel; editing it would break lazy callers and is not required.

**Must remain eager after this fix:** `Menus.tsx`/`Sheet.tsx` are still genuinely cold because `src/shell/ChannelSwitcher.tsx:6` imports `ActionMenu, Sheet` and is reached eagerly from `src/shell/AppHeader.tsx:11` and `src/shell/MobileShell.tsx:10`. `Button`, `Icon`, `Spinner`, `EmptyState`, `ToastHost`, `mobile/safeArea`, `mobile/tabs`, `mobile/keepAlive`, kit labels/tokens and the inlined cold-icon SVGs are also genuine shell needs. The fix only makes the *unused* Selects adapter/menu variants lazy.

## Cause B — the confirm *store* lives inside the confirm *dialog* module

The entry chunk's only import from `shared-controls` is:

```
import { ai as Qn, aj as Xn } from "./shared-controls-*.js"
```

The chunk exports `ai` and `aj` as `Si` and `Ti`; in the emitted code `Si` is `dismissConfirmations` (`function Si(){ qe++; for (const e of [...Xe]) e() }`) and `Ti` is `setConfirmLayout` (`const Ti = e => { Fe = e }`). Both are defined in `platform/ui/src/ConfirmDialog.tsx`:

- `src/App.tsx:9` imports `dismissConfirmations` and calls it at `src/App.tsx:36` on every session/user boundary.
- `src/main.tsx:4` imports `setConfirmLayout` and calls it at `src/main.tsx:19` before first render.
- `platform/ui/src/ConfirmDialog.tsx:22` defines `setConfirmLayout`; `:343` defines `dismissConfirmations`; `:348` defines `confirm()`; the `ConfirmDialog` component itself starts at `:202`.

Rollup splits only at module boundaries. Because the two boot API functions are in the same file as the dialog component, the entry must load the whole `ConfirmDialog.tsx` module and all of its static imports (`controls.tsx`, `useAsyncAction.ts`, `Button`, `Icon`, `Overlays`, `Toast`, `labels`, `Sheet`, `mobile/safeArea`, etc.).

`scripts/mobile-route-chunks.ts` then amplifies this into the whole 36.9 KiB chunk. Its `phoneControls` list (`/platform/ui/src/ConfirmDialog.tsx`, `controls.tsx`, `useAsyncAction.ts`, `Tabs.tsx`, `mobile/rows.tsx`, `components/DevelopEffect.tsx`, `components/confirm.tsx`) keeps those modules in the phone closure and the `shared` partition even when nothing eager needs them. The chunk's own comment says these controls are "rendered by lazy UI, not the entry" — true for the ConfirmDialog *component*, false for the confirm *store functions*. Because entry imports one module from the chunk, it downloads the whole partition:

- `Tabs` (`tablist` marker around 4 KiB)
- kit `controls` (radio/checkbox, about 4–18 KiB)
- `ConfirmDialog` (about 18–32 KiB)
- phone `rows` (`m-row`, about 32–40 KiB)
- screen i18n dictionaries (about 40–69 KiB)
- `api/workbench` (`/api/`, `thumbnailUrl`, `stageGroup`, about 69–82 KiB)
- assistant plan/i18n and other screen-shared logic (about 82–105 KiB)

None of those extra modules are imported by the entry chunk; they are in the chunk only because of the manual `shared` partition.

**Smallest fix:** split `platform/ui/src/ConfirmDialog.tsx` into a small eager store module and the lazily mounted component:

- New `platform/ui/src/confirmStore.ts` (name is a suggestion): module-level `confirmLayout`, `setConfirmLayout`, `confirmationGeneration`, `activeConfirmations`, `dismissConfirmations`, `rememberKey`, `isConfirmationRemembered`, and `confirm()`. `confirm()` already dynamically imports `react-dom/client`; it should also dynamically import `./ConfirmDialog` before `root.render(...)`. Keep `ConfirmOptions` type-only (erased) to avoid a runtime cycle.
- Keep `ConfirmDialog`, `HOLD_DURATION`, `matchesConfirmation`, `scheduleConfirmationHold` in `ConfirmDialog.tsx`; import the layout getter from `confirmStore.ts` for `useSheetConfirmation`.
- Update `platform/ui/src/index.ts` to export `confirm`, `setConfirmLayout`, `dismissConfirmations`, `isConfirmationRemembered` from `confirmStore.ts`, and keep `ConfirmDialog`, the hold helpers and types from `ConfirmDialog.tsx`. The public API is unchanged.

The new store is in the entry closure and is not in `phoneControls`; the existing eager-prune loop in `scripts/mobile-route-chunks.ts` (`for (const module of eager) if (!phoneControls.some(...)) phone.delete(module)`) keeps it out of the phone/shared partition, so it lands in the default eager chunk. The component, `controls`, `useAsyncAction`, `Tabs`, `rows`, i18n and API modules remain in the lazily imported `shared-controls` chunk, which is correct for phone/desktop lazy routes. No manual-chunk repartition is needed.

**Functionality note:** `confirm()` is already asynchronous (it dynamically imports `react-dom/client` and mounts a new root). The split adds one component-chunk fetch before the first dialog mount; it does not change the synchronous store API, `dismissConfirmations` at session boundaries, or the responsive layout decision. If the requirement is that dialog bytes must be resident before any user action, this fix is disallowed — but then the current eager graph cannot reach the 150 KiB entry budget by removing only lazy code.

## What must remain eager

- `App.tsx`/`main.tsx` shell, router, `api/session`, `api/client`, `data/events`, `i18n/vi` and the `client` (`react-dom`) chunk.
- `ConfirmDialog` store API: `setConfirmLayout`, `dismissConfirmations`, `confirm` signature (the dialog view itself can be lazy).
- `ChannelSwitcher`'s `ActionMenu`/`Sheet` and their kit dependencies (`Menus.tsx`, `Sheet.tsx`, `Overlays.tsx`, `Button`, `Icon`, tokens/labels/safeArea).
- `ToastHost`/`toast`/`dismissToasts`, `Spinner`/`EmptyState`, `mobile/tabs`, `mobile/keepAlive`.
- `SystemLoading` and the small `states.tsx` module it lives in.
- The curated inlined cold-icon SVG data. The icon-inlining experiment saved only about 0.5 KiB and was reverted; it is not the cause and must not be re-proposed.

## Expected result and non-fixes

Projected after both fixes (one-line direct import + confirm store split), using the measured chunk slices:

| Metric | Current | Projected | Limit |
|---|---:|---:|---:|
| entryJS | 190.2 | ~143–144 KiB | 150 |
| initialJS | 203.4 | ~193 KiB | 200 |
| initialCSS | 34.7 | 34.7 KiB | 40 |

[INFERENCE: projections are from the measured 36.9 KiB `shared-controls` import removal and ~10.0 KiB `Selects`/adapter slices; they require a real `npm run build && npm run size` to confirm.]

This is **not** byte-accounting evasion because:

- `RouteScreen` genuinely does not execute `ResponsiveSelect`/`Selects` before a lazy screen opens.
- `shared-controls` genuinely has no eager importer for its i18n/API/rows/Tabs/controls content once the store functions move to their own module.
- The size script still walks the real manifest import graph, still includes inline CSS, the phone chunk and the deferred `react-dom/client` chunk.

Do not: change `150`/`200`/`40`, move JS into HTML or external assets to dodge the JS sum, change icon inlining, wrap eager code in `import()` on boot, or merely rename/split chunks without removing an eager import. A manual-chunk-only repartition is also insufficient: it removes the lazy-only shared fraction (~26.7 KiB) but leaves the `ConfirmDialog`/`controls` module cold, so the projected entry is still about 153.5 KiB, over 150.

## V37–V40 natural reduction

The workflow cutover removes lazy consumers, which shrinks the `shared-controls` lazy payload, but it does not remove the eager bindings that make that chunk cold today:

- V37 removes (per `phase-04-v37-compact-studio.md`): `apps/viderse/web/src/screens/workbench/recap/Recap.tsx`, `apps/viderse/web/src/components/assistant/AssistantComposer.tsx`, `apps/viderse/web/src/api/assistant.ts`, `apps/viderse/web/src/components/learn/*`, `apps/viderse/web/src/api/learnedRules.ts`, and related `src/i18n/assistant.ts` / `src/i18n/learn.ts` text.
- V38 removes (per `phase-05-v38-channels-and-library.md`): autonomy desktop/mobile components, `src/api/autonomy.ts`, `src/screens/channel/ScheduleCard.tsx`, `ChannelAside.tsx`, and related i18n.
- V39/V40 rewrite New/Explore/Queue and mobile layouts; they keep `api/workbench.ts`, `api/activity.ts`, channel/library i18n, `Tabs`, phone `rows`, and the confirm primitive.

Exact remaining dependencies after those removals: `src/App.tsx:9/36` (`dismissConfirmations`), `src/main.tsx:4/19` (`setConfirmLayout`), `src/screens/RouteScreen.tsx:8` (barrel), and `src/shell/ChannelSwitcher.tsx:6` (`ActionMenu`, `Sheet`). None of V37–V40 removes those consumers; V40 depends on the responsive `setConfirmLayout` behavior. Therefore the cutover alone must **not** be asserted to pass the gate. The store split and direct import are the cause-aligned fixes; V37–V38 removals only reduce lazy payload (and become irrelevant to entry once the store split lands).

## Residual risks to verify on implementation

- Confirm `index.html`'s manifest `imports` no longer contains `_shared-controls-*.js` after the store split; if it does, the new store was swept into the `shared` partition and the eager-prune rule needs to exclude it explicitly.
- Confirm the `ConfirmDialog` component chunk appears only in `dynamicImports`, and the first `confirm()` flow still opens/awaits/returns the same result (including `dismissConfirmations` during an open dialog and the remember-me path).
- Confirm `responsive-overlays` remains cold and still lacks `Selects`/`KitSelect`/`ChipSelect`/the adapter after the direct import; `Menus`/`Sheet` are expected to remain in that cold chunk because of `ChannelSwitcher`.
- Re-run size only after those source changes; do not treat this report's projected numbers as a passing gate.
