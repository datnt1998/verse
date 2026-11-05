# Mobile canonical tweak truth — rounds 40–44

## Scope and authority

Read-only audit of the current `MChannel.dc.html`, `MCreate.dc.html`, `MTemplate.dc.html`, `MLibrary.dc.html`, artboards `README.md`, and `01-ia-flows.md` §6.2. Current DC declarations/render code are the board-state authority; README is corroborating inventory, not a substitute schema. No artboard, application source, capture, or existing report was changed. No browser, build, test, API, download, or inference service was run.

The requested `002-completed-canonical-reference/manifest.json` does **not** exist in that directory. Its actual manifest is `002-completed-canonical-reference/reference-captures.json`; it contains 124 captures. `017-final-canonical-comparisons/manifest.json` contains 124 rows. Their 42 mobile IDs and props match exactly. All mobile rows specify 390×844. Onboard appears only as desktop 1440×900; there is no mobile Onboard capture to audit. IA §6.2 line 281 explicitly keeps first-run onboarding desktop-only. No round-45 mobile viewer scope is added.

Evidence below is static declaration/render-code analysis, not fresh screenshot parity acceptance. The reference metadata says `visibleStateVerified:false`, `productAcceptance:false`; final mobile rows remain `REVIEW_PENDING`.

## Real canonical declarations

`$preview` is viewport metadata, not another user-visible tweak. README rows 33 and 40–42 agree with every current mobile declaration; no mobile declaration/README conflict was found.

| Board / declaration line | Prop | Every declared value | Default |
|---|---|---|---|
| MChannel:1101 | `tab` | `sources`, `templates`, `settings` | `sources` |
| MChannel:1101 | `fresh` | `false`, `true` | `false` |
| MChannel:1101 | `identity` | `ready`, `generating`, `failed` | `ready` |
| MChannel:1101 | `sheet` | `none`, `rival`, `unlink`, `template`, `link` | `none` |
| MCreate:1014 | `step` | `create`, `done` | `create` |
| MCreate:1014 | `rivals` | `some`, `none`, `empty` | `some` |
| MCreate:1014 | `tplMenu` | `false`, `true` | `false` |
| MCreate:1014 | `identity` | `running`, `failed`, `director` | `running` |
| MTemplate:926 | `screen` | `list`, `images`, `cover`, `backdrop`, `output` | `list` |
| MTemplate:926 | `preview` | `fresh`, `stale`, `building` | `fresh` |
| MTemplate:926 | `sheet` | `none`, `more` | `none` |
| MLibrary:927 | `tab` | `templates`, `prompts`, `backdrops`, `overlays` | `templates` |
| MLibrary:927 | `sheet` | `none`, `actions`, `new` | `none` |
| MLibrary:927 | `empty` | `false`, `true` | `false` |

There are **42 declared prop/value entries**, all explicitly represented by existing mobile refIDs. **Real missing declared values: none.** Those 42 labels represent **32 distinct board+prop vectors** (MChannel 10, MCreate 7, MTemplate 8, MLibrary 7), because defaults are deliberately recaptured under several labels. This is single-prop-value coverage, not coverage of all combinations, interactions, or passing pixel comparisons.

## Existing mobile refID classification and correct contexts

Classification vocabulary:
- **valid-declared**: requested value is declared, with a compatible manifest description.
- **conflicted**: requested value is declared and must remain covered, but the old `expectedVisible` description names something different from the current board. This is not an unsupported prop and is not a passing state.
- **unsupported-ignored** and **interaction-only-notdeclared**: neither occurs among these 42 mobile refIDs. Undeclared mobile features are listed separately below; they must not be converted into invented tweak passes.

Context abbreviations show exact bindings; omitted props stay at the defaults in the declaration table. All 42 reference props are declared and in range, including the conflicted-description rows. There are 31 valid-declared rows and 11 conflicted-description rows under this definition.

### MChannel

Current final rendering is `r44Vals`, lines 1218–1276, overriding obsolete earlier video-tab/autonomy rendering. Three live tabs are Nguồn & tự động, Template mặc định, Hồ sơ kênh. The historical `videos` implementation is not a current fourth tab.

| Existing refID | Classification | Correct binding / actual visible meaning |
|---|---|---|
| `MChannel.tab.sources` | valid-declared | `tab=sources`; automation and linked rivals. |
| `MChannel.tab.templates` | conflicted | `tab=templates,sheet=none`; default-template tab, **not an open Đổi template sheet** as `expectedVisible` says. Open sheet is a separate ref below. |
| `MChannel.tab.settings` | valid-declared | `tab=settings`; profile, identity, language, tags, delete control. |
| `MChannel.fresh.false` | valid-declared | `tab=sources,fresh=false`; existing Midnight Tales context. |
| `MChannel.fresh.true` | conflicted | `tab=sources,fresh=true`; After Dark Stories identity/header, unlinked-YouTube wording, today 0/5. Current code explicitly sets `isFresh=false` in final rendering; **not a dedicated empty-video-list screen**. Sources/settings remain available. |
| `MChannel.identity.ready` | valid-declared | `tab=settings,identity=ready`. |
| `MChannel.identity.generating` | valid-declared | `tab=settings,identity=generating`; generating status/locked art controls. |
| `MChannel.identity.failed` | valid-declared | `tab=settings,identity=failed`; failure banner and visible Thử lại control. Static board does not establish a working backend retry. |
| `MChannel.sheet.none` | valid-declared | `tab=sources,sheet=none`. Same vector as sources/default-fresh ref. |
| `MChannel.sheet.rival` | conflicted | `tab=sources,sheet=rival`; **actions for first linked rival Nocturne Stories**, not a competitors picker/list. Actions: select prominent videos, open Explore, unlink. |
| `MChannel.sheet.unlink` | valid-declared | `tab=sources,sheet=unlink`; confirmation for first/reference rival. Existing/completed/queued videos stay; no further auto-fetch; next rival becomes identity reference without redrawing. Board provides consequence text, not live affected-record counting. |
| `MChannel.sheet.template` | conflicted | `tab=templates,sheet=template`; **five-template chooser**, selected standard template. Not the tab's one-template card plus Đổi/Sửa controls as old description suggests. |
| `MChannel.sheet.link` | conflicted | `tab=sources,sheet=link`; choose from followed rivals; already-linked entries disabled. **No handle/URL input or validation**; following new rivals belongs to Explore. |

MChannel context defaults from final rendering: auto-fetch on; 5 videos/day, stepper 1–20; auto-approve off; default template `std`. Fresh is not permission to remove normal channel features. `identity` must be bound to `settings`; `template` sheet to `templates`; rival/unlink/link sheets to `sources` for meaningful comparison. Sources are not a video-list screen.

### MLibrary

| Existing refID | Classification | Correct binding / actual visible meaning |
|---|---|---|
| `MLibrary.tab.templates` | valid-declared | `tab=templates,sheet=none,empty=false`. |
| `MLibrary.tab.prompts` | valid-declared | `tab=prompts`; prompt list, default local filter All. |
| `MLibrary.tab.backdrops` | valid-declared | `tab=backdrops`; sources list. |
| `MLibrary.tab.overlays` | valid-declared | `tab=overlays`; 12 overlay assets. |
| `MLibrary.sheet.none` | valid-declared | `tab=templates,sheet=none,empty=false`; no sheet. |
| `MLibrary.sheet.actions` | conflicted | `tab=templates,sheet=actions,empty=false`; **template actions**, default first template `std` (used by two channels, deletion blocked). Rename/duplicate/set-default/export/delete, not a generic asset/new editor. |
| `MLibrary.sheet.new` | conflicted | `tab=templates,sheet=new`; **Thêm vào thư viện chooser**: new template, import template, new prompt, channel/video backdrop. Not a prompt/asset editor with a save form. Those are subsequent interactions. |
| `MLibrary.empty.false` | valid-declared | `tab=templates,empty=false`. |
| `MLibrary.empty.true` | valid-declared | `tab=templates,empty=true`; template empty state with create/import entry points. Does not empty prompts/backdrops/overlays. |

Detail sheets are interaction state, not extra declared `sheet` values: prompt (`prI`), backdrop source (`srI`), overlay (`ovI`), rename (`ren`), add source (`as`). `sheet=actions` starts on `act=0`; other template action targets require a row interaction, not a made-up prop.

### MTemplate

| Existing refID | Classification | Correct binding / actual visible meaning |
|---|---|---|
| `MTemplate.screen.list` | valid-declared | `screen=list,preview=fresh,sheet=none`; editor overview with preview and four child entries; not Library's template list. |
| `MTemplate.screen.images` | valid-declared | `screen=images`; composition, scene prompt, count controls. |
| `MTemplate.screen.cover` | valid-declared | `screen=cover`; cover prompt and sample-cover action. |
| `MTemplate.screen.backdrop` | valid-declared | `screen=backdrop`; source mix, attach action, inline overlay chips. |
| `MTemplate.screen.output` | valid-declared | `screen=output`; compression and channels using as default. |
| `MTemplate.preview.fresh` | valid-declared | `screen=list,preview=fresh`; current/green preview label. |
| `MTemplate.preview.stale` | valid-declared | `screen=list,preview=stale`; changed-settings/rebuild label. |
| `MTemplate.preview.building` | valid-declared | `screen=list,preview=building`; building label, disabled/busy rebuild state, illustrative 1:12/3:00 progress. This static artboard is not evidence of a real builder job. |
| `MTemplate.sheet.none` | valid-declared | `screen=list,sheet=none`. |
| `MTemplate.sheet.more` | conflicted | `screen=list,sheet=more`; **rename, duplicate, export, blocked delete**. Old `expectedVisible` incorrectly puts rebuild/sample actions in this sheet; those live on overview/cover respectively. |

Preview-state comparisons must use `screen=list`: preview block is inside the `isList` conditional. A child screen is not a substitute for seeing the preview-state difference. In the manifest these bindings are already correct.

### MCreate

| Existing refID | Classification | Correct binding / actual visible meaning |
|---|---|---|
| `MCreate.step.create` | valid-declared | `step=create,rivals=some,tplMenu=false,identity=running`. |
| `MCreate.step.done` | valid-declared | `step=done,identity=running`; background identity tasks. |
| `MCreate.rivals.some` | valid-declared | `step=create,rivals=some`; followed list, NS and LA preselected. |
| `MCreate.rivals.none` | conflicted | `step=create,rivals=none`; **followed rivals remain listed, none selected**. Not “No rivals.” Create disabled until selection. |
| `MCreate.rivals.empty` | conflicted | `step=create,rivals=empty`; **no followed rivals initially**, follow entry shown; not an empty search-result state (no search tweak). Newly followed local rows can appear. |
| `MCreate.tplMenu.false` | valid-declared | `step=create,tplMenu=false`; closed template chooser. |
| `MCreate.tplMenu.true` | valid-declared | `step=create,tplMenu=true`; bottom sheet with five template choices. |
| `MCreate.identity.running` | valid-declared | `step=done,identity=running`; director/standard template done, artist running. |
| `MCreate.identity.failed` | conflicted | `step=done,identity=failed`; **artist quota failure, automatic retry at 07:00**, not a manual retry button as expectedVisible says. Manual retry belongs to director failure. |
| `MCreate.identity.director` | valid-declared | `step=done,identity=director`; temporary channel name, artist waits for director; Thử lại changes director to retrying. |

`rivals` and `tplMenu` bind to `step=create`; `identity` binds to `step=done`. All existing capture props use those correct contexts. Default create settings are standard/recommended template, auto-fetch on, auto-approve off, 5/day. The current MCreate stepper is 1–50, whereas final MChannel is 1–20; that is an existing board-level constraint discrepancy, not license to silently rewrite either board or claim uniform bounds.

## Undeclared features retained; not counted as missing declared values

These are **interaction-only-notdeclared** capabilities, not unsupported features to delete and not new canonical knob values:

- **MChannel:** switch auto-fetch/approval, daily limit, select default template, link an available followed rival, open selected rival actions/confirmation, profile language and tags, identity/profile controls. Final `r44Vals` overrides legacy autonomy props/controls; do not resurrect the old fourth Video tab or autonomy presets merely because earlier dead rendering code still mentions them. `approve`, `autonomy`, `sheet=row` are not canonical declared knobs.
- **MCreate:** selection and first-reference identity, follow-new validation/auto-selection, five templates (`std`, `doc`, `close`, `sleep`, `night`), auto-approval, quota, create transition, director retry state. Follow validation is here (and Explore), not `MChannel.sheet.link`.
- **MTemplate images:** local composition `full/cutout/close`, default `cutout`; scene prompts `def/moss/doc35/drama`, default `moss`; count mode `fixed/every`, default fixed 6, switching to every gives 7, count bounds 1–30. These are state driven by controls, not declared `composition`, `count`, `countMode`, or `scenePrompt` props.
- **MTemplate cover/output:** cover choices `c-clean/c-hook/c-face/c-split/c-red/c-yellow`, default `c-red`; compression on (`zip=true`) versus original; channels using template as default. No declared cover/zip knobs exist.
- **MTemplate flows:** preview video selection (`hotel/nurse/snow`, default hotel), attach-source sheet, remove/add source, rebuild, try-cover, rename, duplicate, export. Open/closed video/source picker state is `vS/sS`, not declared `sheet=video|source` or `picker=...` values.
- **MTemplate effects:** snow/rain/dust/film/fog local toggle chips; snow initially on, the rest off. There is no declared `overlay`/`effect`/`picker=overlays` tweak. Absence of a knob does not negate overlay persistence or real custom assets in the product.
- **MLibrary:** prompt filters All/Scene/Cover; 12 prompt entries; prompt detail/new form, built-in read-only state, source detail/add/validation, overlay detail, template rename/action target; 12 overlay entries (Tuyết, Mưa, Bụi, Phim cũ, Loé sáng, Bokeh, Tàn lửa, Sương mù, Đom đóm, Lá rơi, Hoa anh đào, Sao). These require actual row/action interactions for coverage.

Unsupported invented props passed to the four mobile boards would not establish distinct states. This finding does **not** say the 124-row desktop/mobile manifest is wholly valid: desktop props are outside this report, and its mobile descriptions already contain conflicts.

## Effect / overlay picker container authority

1. **Exact current MTemplate design:** `MTemplate.dc.html:923` renders the backdrop's Lớp phủ as an **inline wrapping chip row**, not an overlay picker dialog or sheet. `FX` at 946–947 and initial state at 928 establish five toggles with snow selected. There is no canonical mobile effect-picker modal to screenshot under an invented knob.
2. **Actual picker containers on this board:** preview-video (`tx.vShow`) and attach-source (`tx.sShow`) both render a scrim plus `vv-sheet vv-sheet-in`, `role=dialog`, bottom:0, left/right:0. The semantic dialog role does not make them centered dialogs. Their state/actions are at 960–966. More/rename are bottom sheets too.
3. **Library overlay detail:** `MLibrary.dc.html:924`, `lx.ov.show`, is a bottom `vv-sheet vv-sheet-in`, not a centered dialog. IA §6.2 line293 explicitly requires prompt/backdrop/overlay details as sheets.
4. **Global mobile rule:** IA §6.2 lines282/285 requires same data/options with mobile layout; dropdown lists and ⋯ menus are bottom sheets. Line294 explicitly makes preview-video and backdrop attachment pickers sheets. MCreate template picker (`977–982`) and MChannel template/link pickers (`1071–1072`) corroborate that pattern.
5. **Current app implementation evidence:** `web/src/screens/library/template/BackdropSection.tsx:206` computes mobile at max-width 767px, but 409–429 renders the overlay-candidate selection through unconditional `<Dialog ... width={520}>`, with no mobile container branch. This supports the parent's observed centered dialog at 390px as an implementation choice, not a canonical-board exception.

**Authority decision:** exact board parity is the inline chip interaction. Do not invent a centered canonical mobile effect-picker state. If the product retains its additional asset-list picker to support all real/custom overlay assets, **[INFERENCE]** IA's mobile list-selection rule supports adapting that list to a bottom sheet, while retaining selection/removal/persistence and supported assets. That inference specifies container family only: this audit provides no invented effect-picker geometry, artwork, item layout, or new tweak. Do not remove supported features because they lack a declared knob, and do not count the existing app Dialog as a passed declared-tweak comparison.

## Current SHA-256 receipt

Hashes were computed from file bytes read for this static audit, not from git or a previous report. Paths below are relative to `/Users/dat/Projects/verse`.

| File | SHA-256 |
|---|---|
| `apps/viderse/docs/design/v1/artboards/MChannel.dc.html` | `0bbd689ace43af3085709e3c79251e40d9c599fa4b90d304670233acfae487ba` |
| `apps/viderse/docs/design/v1/artboards/MCreate.dc.html` | `03af265e0edf09d1417b25c8e7d9910a2c6f967cce5cae4492305f0327a7eff7` |
| `apps/viderse/docs/design/v1/artboards/MTemplate.dc.html` | `2d351bd5fcafafd47fd1c794ad018f03aa27a0d03b69437ab61e91af4fc157c2` |
| `apps/viderse/docs/design/v1/artboards/MLibrary.dc.html` | `97ac9595e64fc06d659aa963ce8f1a99b9e6e0bbcfd230062e69fc9dbe98f7a8` |
| `apps/viderse/docs/design/v1/artboards/README.md` | `fe102b5526e5066b84adb09e8b94b4fa324b507fbefd89aad3321f347599fed1` |
| `apps/viderse/docs/design/v1/01-ia-flows.md` | `1bec656cc5a82d9ff4d07d897918389a491e36781847169046b86fc1c3634614` |
| `plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/002-completed-canonical-reference/reference-captures.json` | `c4f872d80b7de3dd351b22d4943b0ae7aea06ed6d8b3bd5ad8c625ba01dc7d4d` |
| `plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/017-final-canonical-comparisons/manifest.json` | `fcfaad27405da42d198a06ade532451fc92f53482cd5ca99878e3998784024b0` |
| `apps/viderse/web/src/screens/library/template/BackdropSection.tsx` | `6ef63407db9d7d9325da6184e1325f363f01980fbbd7ada2b3c9ac2d5404f4ad` |

## Status

Complete: current mobile declared inventory, all 42 existing mobile refIDs, context corrections, undeclared interaction distinctions, container authority, and file receipts recorded. Only this assigned report was written.

## Summary

No missing mobile declared values and no unsupported mobile capture props. All 42 prop/value labels are valid declarations, but 11 existing manifest descriptions conflict with actual board meaning; 42 labels collapse to 32 unique prop vectors. Supported interaction features remain required independently of knob coverage. Current board overlay controls are inline chips; real mobile pickers/details use bottom sheets, not a canonical centered effect-picker Dialog.

## Concerns

Existing capture metadata is not state/pixel acceptance. Old `expectedVisible` prose must not override actual board behavior. Static sample progress/toast-only board feedback is not proof of backend integration. Some board controls have no explicit local handler, so this report does not certify their runtime execution. MCreate/MChannel daily-limit bounds differ in current artboards; retain the discrepancy as evidence rather than inventing a normalized design. No round45, mobile onboarding, source/artboard edits, or verification runs were introduced.
