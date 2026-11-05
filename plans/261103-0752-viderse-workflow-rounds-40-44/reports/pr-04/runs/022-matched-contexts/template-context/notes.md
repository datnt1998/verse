# Template comparison-context capture handoff

## Status

**PREPARED, NOT EXECUTED.** Only this private `controller.cjs` and this note were written. No repository/source/artboard/worktree/git/build/test/browser/API operation was executed by this worker. No screenshots or runtime safety counts are claimed yet.

Parent entry point after review:

```sh
node /var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v38-acceptance-xxkx9151/parallel-acceptance-20261104/template-context/controller.cjs --execute
```

Without `--execute`, the script refuses to start a browser. The existing cookie file is consumed directly without printing or copying its contents.

## Summary: exact pairing, not new declared values

Executable mapping owner: `controller.cjs` → `mappings`, `mobileMapping`. Runtime receipt owner: sibling `results.json` after parent execution. These keys correct comparison contexts; they do **not** add enum values or missing declared-value obligations.

| Pairing key | Canonical props for parent recapture | Exact selected app filename / open stage |
|---|---|---|
| `Template.picker.add@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"add"}` | `fixture-1440-template-picker-add-backdrop-open.png` — lofilanes, unlinked, random, Clip+Highlight, zero selected, attach enabled |
| `Template.picker.choose@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"choose"}` | `fixture-1440-template-picker-choose-backdrop-open.png` — linked nightdrivefm, pick, Clip only, first four canonical clips checked, Save enabled |
| `Template.picker.video@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"video"}` | `fixture-1440-template-picker-video-backdrop-open.png` — unlinked ferry / Phà đêm trong sương, 0:00–2:00, attach enabled |
| `MTemplate.screen.backdrop@attach-source` | `{screen:"backdrop",preview:"fresh",sheet:"none"}`, then click **Gắn kênh hoặc video** (`sS=true`) | `fixture-390-template-backdrop-attach-source-open.png` — actual native source Sheet, before selection; interaction-only comparison with explicit structural qualification |

Desktop viewports are 1440×900; mobile is 390×844. Existing legacy desktop reference IDs remain recorded with their old `section:"images"` props in the controller. They are historical references, not interchangeable with these corrected backdrop-underlay combinations. The default `picker:"none"` images baseline is intentionally unchanged.

No generic final/closed screenshots are written. A case must pass its exact open-state assertions after document fonts and finite animations settle before its filename becomes `selectedFile`. Failed or blocked cases have `selectedFile:null`; there is no fallback screenshot masquerading as the reference. Parent captures corresponding corrected canonical combinations separately.

## Data provenance and evidence boundary

- Canonical data owner: `apps/viderse/docs/design/v1/artboards/Template.dc.html`, `LIB` / `DEF` / picker rendering; canonical mobile owner: `MTemplate.dc.html`, `tx.sOpen`, `tx.sList` and `tx.sShow`.
- Source owner: `BackdropSection.tsx` imports and mounts the actual `BackdropPicker.tsx`; capture entry uses the real attach button or linked nightdrivefm row's **Chọn video** button. No component is injected or reimplemented.
- Type owners: `api/backdrop.ts` and `api/library.ts`; controller fixture declarations reference the actual TypeScript wire types. Native template detail/identity/revision and owner IDs are read at execution time. Missing native prerequisites stop discovery; none are created.
- Named channels, catalogue subset, healthy mix and recipe backdrop links are clearly labelled **local metadata fixtures**. `fixture()` clones a native detail and changes only its private GET-response reference graph. This is not a native link, attachment, saved revision or persistence claim. Capture never submits attach/save.
- The canonical non-few source underlay is locally populated with slowroads, linked nightdrivefm/four picks and the two fixed pass/glass segments. Other native recipe settings remain native, not silently represented as canonical defaults. A native snow overlay is reused only if present; absence is recorded rather than manufacturing an asset ID.
- Every image has null fixture thumbnail/avatar metadata; existing native neutral slots are retained. Fixture videos have no file and no media URL. Fresh preview is metadata without projectId or prior rendered frame. No real imagery, player footage or native preview persistence is fabricated.
- Screenshots carry a visible, pointer-transparent **UI FIXTURE · metadata only · 0 server writes** badge. `results.json` includes fixture payloads, exact stage/props/file mappings, observed assertions, native ID receipts, source-file SHA256 receipts, safety counters and request decisions. Cookie values are excluded.

## Safety contract

The sole request continuation site is guarded by exact origin `http://127.0.0.1:8043`, method `GET`, and an API allowlist. Non-GETs are aborted by default without any locally fulfilled successful mutation. External origins, provider discovery/parse/follow, media/image acquisition, file/export/download paths, unknown API GETs, service workers and WebSockets are fenced. Downloads are also disabled/cancelled. Browser and per-case contexts close in `finally`.

Expected successful capture receipt: `realWrites=0`, `nonGETContinued=0`, `externalContinued=0`, `localMutationFulfillments=0`, `downloads=0`, no route errors, and no non-GET requests in any capture case. These are future execution assertions, not observed counts from this preparation.

## Concerns requiring parent interpretation

1. **Unlinked add is currently source-blocked unless the native default is corrected.** `BackdropPicker.tsx` initializes kinds with `link?.kinds ?? platformKinds` (line 41); Twitch includes VOD even when its count is zero. The zero-count selected chip then has `aria-disabled=true` via lines 113–118, despite its native button and toggle handler remaining live. Canonical add requires Clip+Highlight only. Per parent direction, the controller performs **normal clicks only**: if this chip is selected and disabled it records `BLOCKED_CANONICAL_CONTEXT`, does not capture add, and continues choose/video/mobile. It does not use force, evaluate-click, DOM/state rewriting or a fake linked lofi fixture. Source-backed proposal sent to parent: use positive-count kinds for a listed **unlinked** channel's initial default; preserve explicit existing linked kinds, and distinguish a selected removable zero-count kind from an unavailable unselected kind in disabled semantics. This worker does not change source.
2. **Mobile has no canonical detailed add/choose/video counterparts.** Mobile truth and current MTemplate contain one simple attach-source list with four channels, slowroads/nightdrivefm already attached, and direct local attach on selection. The app's actual shared BackdropPicker Sheet includes detailed channel controls/standalone sources. The single mobile capture records that mismatch rather than pretending desktop picker enums exist on MTemplate. No mobile choose/video files are scheduled and no `sheet=source|video` value is invented.
3. **Ferry media framing remains unproven.** Visual report identifies canonical reserved player area versus actual metadata-only detail. This script fixes selection identity/segment/stage, not media-region design or playback. A neutral/no-file screenshot cannot prove player parity.
4. These are context-corrected UI-consumer receipts, not pixel acceptance, backend generation, native source-library records, persisted linking or new feature coverage. Existing reports and source remain untouched.

## Source-backed inputs consulted

- Existing private `template-tweak-closure/controller.cjs`, especially guard/native helpers and original mixed choose/video case.
- `plans/reports/orchestrate-261105-095440-v38-acceptance/desktop-tweak-truth.md` — C1/C2.
- Sibling `mobile-tweak-truth.md` — MTemplate declared values and attach-source interaction distinction.
- Sibling `template-visual-acceptance.md` — picker selection and metadata-only media caveats.
- Native `BackdropSection.tsx`, its imported `BackdropPicker.tsx`, `SourceRail.tsx`, catalogue pagination helper, API types, localization/formatters, and the two current canonical boards.
