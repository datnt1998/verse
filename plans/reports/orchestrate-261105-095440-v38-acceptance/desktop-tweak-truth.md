# Desktop tweak truth — current source snapshot

Scope: `Channel.dc.html`, `CreateChannel.dc.html`, `Onboard.dc.html`, `Template.dc.html`, `Library.dc.html` only. Static source/JSON audit; no screenshot inspection, browser, build, tests, API, download, or artboard edit. This report does not accept pixel parity.

## Evidence and immutable receipts

Artboard root: `/Users/dat/Projects/verse/apps/viderse/docs/design/v1/artboards`.
Run root: `/Users/dat/Projects/verse/plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs`.

| Evidence | SHA256 |
|---|---|
| `Channel.dc.html` | `5b4c984c85ad46405a66d01cd20f562a1768f08e432392b631cdc9847c5153ab` |
| `CreateChannel.dc.html` | `e4be3675fbe76208da1f6e36bbe7a6373c7c5232eedc437ca13f44f3607bebbf` |
| `Onboard.dc.html` | `2eb8bfd050a2d042a281dedab556c469152ff5cea6562451b72a846ac59785de` |
| `Template.dc.html` | `3e8107334d098f2dbbe1e5f9c637b945dc99acd1b074cdc9cfa09d4057904c24` |
| `Library.dc.html` | `56cf1f182e21590a5959ef263fd2ed168aab7e4c9a58b530feaae64f239f6841` |
| `artboards/README.md` | `fe102b5526e5066b84adb09e8b94b4fa324b507fbefd89aad3321f347599fed1` |
| `002-completed-canonical-reference/reference-captures.json` | `c4f872d80b7de3dd351b22d4943b0ae7aea06ed6d8b3bd5ad8c625ba01dc7d4d` |
| `017-final-canonical-comparisons/manifest.json` | `fcfaad27405da42d198a06ade532451fc92f53482cd5ca99878e3998784024b0` |
| `014-channel-tweak-closure/controller.cjs` | `e4a7b0caf5ae7ba9ca625602d70cf2035660e988b6b2a44488f15921aa7c039c` |
| `015-template-tweak-closure/controller.cjs` | `0c6f21ff51377db20b53247daf2bea1a49b6396fa48ecdd568e5bfbf156a0f83` |

The requested `002.../manifest.json` is absent; the actual capture inventory is `reference-captures.json`. Its 82 desktop IDs and props exactly match the desktop subset of 017. Both contain 124 rows overall; the other 42 are outside this desktop assignment. The reference inventory records capture time `2026-11-04T16:10:08.468Z` and revision `277b902b93cd7db1d771a418c789e8314802f722+canonical-design-worktree`; these labels do not prove byte identity with today’s files. No historical artboard byte snapshot was established in this audit, so **no source-change claim is made**.

## Current declared inventory

Authority: each board’s `data-props` (Channel:1149, CreateChannel/Onboard:1137, Template:1396, Library:1361). README rows 17–18 and 26–28 agree exactly with all five current property/value sets; README is not an alternative schema. `$preview` is viewport metadata, not a user tweak; all five declare 1440×900. Boolean values below are JSON booleans, not strings.

| Board | Property | Real declared values | Default |
|---|---|---|---|
| Channel | `tab` | `"sources"` · `"templates"` · `"settings"` | `"sources"` |
| Channel | `channel` | `"MT"` · `"CL"` · `"QH"` | `"MT"` |
| Channel | `autoApprove` | `false` · `true` | `false` |
| Channel | `tplMenu` | `false` · `true` | `false` |
| Channel | `identity` | `"ready"` · `"generating"` · `"failed"` | `"ready"` |
| Channel | `link` | `"none"` · `"picker"` · `"empty"` | `"none"` |
| Channel | `fresh` | `false` · `true` | `false` |
| CreateChannel | `step` | `"create"` · `"done"` | `"create"` |
| CreateChannel | `rivals` | `"some"` · `"none"` · `"empty"` | `"some"` |
| CreateChannel | `tplMenu` | `false` · `true` | `false` |
| CreateChannel | `identity` | `"running"` · `"failed"` · `"director"` | `"running"` |
| Onboard | `step` | `"setup"` · `"create"` · `"done"` | `"setup"` |
| Onboard | `setup` | `"running"` · `"login"` · `"failed"` | `"running"` |
| Onboard | `tplMenu` | `false` · `true` | `false` |
| Onboard | `identity` | `"running"` · `"failed"` · `"director"` | `"running"` |
| Onboard | `rivals` | `"some"` · `"none"` · `"empty"` | `"some"` |
| Template | `section` | `"images"` · `"cover"` · `"backdrop"` · `"output"` | `"images"` |
| Template | `preview` | `"fresh"` · `"stale"` · `"building"` | `"fresh"` |
| Template | `fewClips` | `false` · `true` | `false` |
| Template | `picker` | `"none"` · `"add"` · `"choose"` · `"video"` | `"none"` |
| Library | `tab` | `"templates"` · `"prompts"` · `"backdrops"` · `"overlays"` | `"templates"` |
| Library | `prompt` | `"list"` · `"mine"` · `"builtin"` · `"new"` | `"list"` |
| Library | `menu` | `false` · `true` | `false` |
| Library | `nguon` | `"channels"` · `"videos"` · `"adding"` · `"addvideo"` · `"unsupported"` | `"channels"` |
| Library | `import` | `"none"` · `"ready"` · `"drag"` · `"importing"` · `"done"` · `"partial"` · `"zip"` · `"offline"` | `"none"` |
| Library | `kenh` | `"slowroads"` · `"nightdrivefm"` · `"bilibili"` · `"lofilanes"` | `"slowroads"` |

There are 82 property/value obligations and 82 corresponding desktop reference IDs. They describe only 61 unique full prop combinations (Channel 12, CreateChannel 7, Onboard 10, Template 10, Library 22): defaults are intentionally repeated across per-property IDs. These are neither 82 independent visual states nor coverage of every Cartesian combination or every interaction.

## Every existing desktop reference ID

Classification is split deliberately: **declaration validity** answers whether the current board declares and consumes the target tweak; **comparison context** records independent evidence conflicts. `valid-declared` does not mean captured pixels or app functionality were accepted. There are zero existing desktop IDs classified `unsupported-ignored`, and zero existing IDs that are solely `valid-interaction-but-not-declared`. The prior assumption that these manifests contain `Template.picker.overlays`, `count`, or `composition` rows is disproved by their actual IDs/props; it was not evidence of any artboard change.

| Reference ID | Declaration class | Context / comparison qualification |
|---|---|---|
| `Template.section.images` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.section.cover` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.section.backdrop` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.section.output` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.preview.fresh` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.preview.stale` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.preview.building` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.fewClips.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.fewClips.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.picker.none` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Template.picker.add` | valid-declared | **conflicted** — images underlay vs backdrop app case; see C1. |
| `Template.picker.choose` | valid-declared | **conflicted** — images underlay vs backdrop app case; see C1 and C2. |
| `Template.picker.video` | valid-declared | **conflicted** — images underlay vs backdrop app case; see C1 and C2. |
| `Library.tab.templates` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.tab.prompts` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.tab.backdrops` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.tab.overlays` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.prompt.list` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.prompt.mine` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.prompt.builtin` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.prompt.new` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.menu.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.menu.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.nguon.channels` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.nguon.videos` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.nguon.adding` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.nguon.addvideo` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.nguon.unsupported` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.none` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.ready` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.drag` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.importing` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.done` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.partial` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.zip` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.import.offline` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.kenh.slowroads` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.kenh.nightdrivefm` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.kenh.bilibili` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Library.kenh.lofilanes` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.tab.sources` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.tab.templates` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.tab.settings` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.channel.MT` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.channel.CL` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.channel.QH` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.autoApprove.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.autoApprove.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.tplMenu.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.tplMenu.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.identity.ready` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.identity.generating` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.identity.failed` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.link.none` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.link.picker` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.link.empty` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.fresh.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Channel.fresh.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.step.create` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.step.done` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.rivals.some` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.rivals.none` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.rivals.empty` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.tplMenu.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.tplMenu.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.identity.running` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.identity.failed` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `CreateChannel.identity.director` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.step.setup` | valid-declared | **conflicted** — setup fixture differs from current canonical tool states/progress; C3. |
| `Onboard.step.create` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.step.done` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.setup.running` | valid-declared | **conflicted** — setup fixture differs from current canonical tool states/progress; C3. |
| `Onboard.setup.login` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.setup.failed` | valid-declared | **conflicted** — setup fixture differs; retry-success is a different interaction state; C3/C4. |
| `Onboard.tplMenu.false` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.tplMenu.true` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.identity.running` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.identity.failed` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.identity.director` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.rivals.some` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.rivals.none` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |
| `Onboard.rivals.empty` | valid-declared | No static declaration/context conflict identified; visual acceptance unreviewed. |

All 82 reference records say `reference-captured`, but **all 82 also say `visibleStateVerified:false`**. 017 is `VISUAL_REVIEW_PENDING`. Those recorded statuses are not new executed checks.

## Exact gaps, invalid contexts, and corrected compatible combinations

### Declared-value gaps

**Missing declared property/value IDs: none (0/82).** This is static inventory completeness only. No receipt here establishes that every old reference was captured from these current SHA256 bytes. A “current artboard” acceptance pass must pair references to these receipts (or re-capture from the same bytes); do not silently treat revision labels as byte receipts.

### C1 — backdrop picker has the wrong underlay in existing reference props

`Template.picker.add`, `.choose`, and `.video` all use `{section:"images",preview:"fresh",fewClips:false,picker:<value>}` in both inventories. The board permits this because the dialog renders independently (`Template:1567–1611`), so these are **not unsupported props**. But the real entry button and source-edit controls are in backdrop (`1126–1183`, `1531`, `1664`); 015 controller:296–301 explicitly navigates to backdrop, and its results:440 acknowledges this mismatch. Keeping an images-underlay canonical image opposite a backdrop app image cannot prove full composition parity.

| Recapture / pairing key | Compatible canonical props | Exact canonical meaning |
|---|---|---|
| `Template.picker.add@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"add"}` | lofilanes channel (actual name `lofilanes`), unlinked, random mode, Clip+Highlight; attach CTA |
| `Template.picker.choose@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"choose"}` | linked nightdrivefm; pick mode; Clip only; four selected videos; Save CTA |
| `Template.picker.video@backdrop` | `{section:"backdrop",preview:"fresh",fewClips:false,picker:"video"}` | unlinked ferry standalone video, segment 0:00–2:00, attach CTA; NOT preview/cover build video |

These are three context-corrected captures, not three newly discovered enum values. Retain the existing IDs/legacy context as explicit historical evidence if needed, not interchangeable acceptance images. `Template.picker.none` is already a valid images baseline; add a backdrop-none baseline when comparing picker transitions, using `{section:"backdrop",preview:"fresh",fewClips:false,picker:"none"}` (the exact combination already exists as `Template.section.backdrop` and `Template.fewClips.false`).

Do not combine `fewClips:true` with `picker:"choose"` expecting the same linked-four-video Save state: the fewClips fixture replaces the linked channels with lofilanes/8 clips, so nightdrivefm is then unlinked and defaults to random/attach (`1517–1519`, `1579–1605`). Use `fewClips:false` for the canonical choose state; for a few-clips source edit, start from backdrop/fewClips true and interact with the linked lofilanes edit control instead.

### C2 — picker chosen stage is not identified by selectedCase

017 maps both `.choose` and `.video` to the same selected case `synthetic-1440-backdrop-picker-choose-video` and offers four candidate files, but records no exact selected file for these rows. 015 controller:298 captures `-choose-empty.png` (zero selected, disabled attach), :299 captures `-choose.png` (one selected), :300 captures `-video.png` (standalone segment), then closes the modal; its generic final `.png` therefore represents a later closed state. Neither choose-empty nor one-selected unlinked channel matches canonical choose’s linked nightdrivefm/four-selected/Save state. Pair video specifically with the `-video` stage, and produce a linked-four-video Save stage for choose. Require `selectedFile`/stage evidence in the next comparison artifact; a case that exercised a state is not a filename receipt for that state. This is static controller evidence, not an image review.

The 002 `expectedVisible` description for `.video` incorrectly describes sample/cover-build video selection. Actual current `picker:"video"` selects standalone backdrop ferry video. `.add`/`.choose` are a channel/video modal, not proof of an arbitrary effects picker. Separate preview and cover video dropdowns are interaction state (`tPvO/tPv`, `tCvO/tCv` at Template:1436–1445); do not remap them to picker.video.

### C3 — Onboard running/failed fixture is not canonical tool context

Current Onboard:1174–1177 defines `running=[done,done,run,done,wait,user]` and `failed=[done,done,fail,done,done,focus]`; running progress is 62% with 806 MB/1.3 GB (`1189`), and failed keeps 806 MB and offers login as primary (`1191–1195`). 014 controller:228 instead supplies Chrome pending and image-tool pending for both running and failed, login needs_user, and 50% of 1 GB. These are declared running/failed values but non-equivalent fixtures. This affects `Onboard.setup.running`, its duplicate baseline `Onboard.step.setup`, and `Onboard.setup.failed`; do not call them invalid tweak values or delete setup functionality.

| Next exact setup comparison | Canonical props (other defaults unchanged) | App fixture correction |
|---|---|---|
| running / step.setup baseline | `{step:"setup",setup:"running",tplMenu:false,identity:"running",rivals:"some"}` | six tool states matching done/done/running/done/pending/needs-user; 62%, 806 MB/1.3 GB |
| failed initial stage | `{step:"setup",setup:"failed",tplMenu:false,identity:"running",rivals:"some"}` | done/done/failed/done/done/login-focus; retained-byte copy, retry, primary login |

### C4 — retry stage does not inherit failed visual identity

017 lists `setup-failed`, `setup-retry-503`, and `setup-retry-success` candidates under `Onboard.setup.failed`, selected only by the shared case. 014 controller:236 and :259–264 explicitly changes to running on success. Pin initial failed comparison to `SYNTHETIC-1440-onboard-setup-failed-setup-failed.png`; use retry-503 only for the additional error-feedback interaction; retry-success belongs to an interaction transition back to running, not canonical failed. Preserve both recovery scenarios as functionality coverage.

### Other incompatible combinations to prevent in the next matrix

- Channel: `tplMenu:true` needs `tab:"templates"`; identity variants need `tab:"settings"`; link picker/empty needs sources. The existing per-value rows already use these contexts. `fresh:true` selects After Dark Stories regardless of `channel` (1344–1345), `autoApprove:true` is MT-only (1414), and `link:"empty"` is MT-only (1397). Thus do not claim CL/QH autoApprove=true or fresh=true+channel=MT as independent requested channel states. Existing `Channel.fresh.true` correctly exercises After Dark, not MT.
- CreateChannel/Onboard: `tplMenu` and `rivals` are create-stage controls; identity is done-stage; Onboard `setup` is setup-stage. Existing rows set those contexts correctly. Hidden/default props are not independently visible states.
- Library: prompt variants require `tab:"prompts"`; menu requires templates; nguon/kenh require backdrops. Import is an independent modal whose normal trigger is templates. Existing rows use compatible contexts. Library tab=overlays is declared and supported; it must not be dropped just because Template has no picker=overlays enum.

## Interaction-only functionality: retain, capture through actions, do not invent tweaks

These states are supported by current board interaction wiring but are **not declared tweaks**. Setting extra props with these names does not reproduce them; Template only consumes section/preview/fewClips/picker. No existing desktop reference ID claims otherwise. Do not remove app functionality merely because the artboard exposes it through a click rather than `data-props`.

| Interaction inventory / defaults | Correct capture path and evidence owner |
|---|---|
| Composition full/cutout/close; default cutout | section=images, click composition card; `Template:1469–1471`; not composition or style props |
| Count fixed/per/none; default fixed, n=6, per=7; numeric stepper bounds 1–60 | images, click count option then stepper; `1400`, `1474–1479`, `1657–1658`; not count/n/per props |
| Motion zoom/wander (wander default); strength/speed 0–2 step .1 (1 default); opacity 0–100 (100 default); reference image boolean true default | images, change controls; `1055–1062`, `1434–1439`, `1480–1481`, `1659–1661`; distinguish these real controls from ignored props |
| Scene prompt moss / cover c-red defaults, prompt-viewer open/prev/next/use; cover trial; preview-video and cover-video dropdowns hotel/nurse/lastbus | images/cover, click real prompt/eye/trial/dropdown controls; `1400`, `1420–1445`, `1459–1461`, `1493`, `1663` |
| Backdrop random/pick, kinds, zero/one/multiple selected; attachment/removal, clip auto/manual (auto default), 60-second clip default with 10–600 bound | backdrop picker then actual radios/checks/save; backdrop clip disclosure; `1517–1611`, `1558`, `1665`; record source identity and exact selected count |
| Output compression true/false (true default), preview playback, menu | output switch, preview play and overflow; `1400`, `1411–1415`, `1620–1624`, `1666` |
| Channel source unlink/link selection, auto toggle, daily 1–20 (channel-specific default), default-template choice, identity retry, profile edits | actions in Channel `r42Vals` at `1395–1510`; no invented additional declared enum |
| Create/Onboard template choice, auto-approval, daily controls, rival selection/follow, director retry, setup login-wait/retry | create/done/setup actions at `1149–1156`, `1210–1269`; separate from declared baseline IDs |
| Library sort, source kind filters, prompt editor controls/viewer, import warning details, unsupported-platform request | actual actions at Library `1375–1411`, `1459–1510`, `1580–1601`; preserve beyond per-value enumeration |

**Effects correction:** canonical Template snow row and snow preview are static (`1197–1203`, `1281`); `tv.rmSnow` only displays feedback (`1442`), and “add from library” is an anchor to Library. There is no declared Template effect picker and no interaction-backed Template overlays modal in this snapshot. Use `{section:"backdrop",preview:"fresh",fewClips:false,picker:"none"}` to compare the existing snow layer; use Library `{tab:"overlays",prompt:"list",menu:false,nguon:"channels",import:"none",kenh:"slowroads"}` for the effect catalog. Keep an app’s additional effect picker/removal/edit controls as separately named interaction/product scenarios; do not claim canonical prop-state or removal persistence from a toast-only board handler. There is no compatible canonical `picker:"overlays"` combination.

## Status / Summary / Concerns

**Status:** Completed static desktop inventory and capture-matrix correction; only this report written. No design/source files changed.

**Summary:** 82/82 desktop IDs are valid declared property/value references, README matches current schemas, zero missing declared values, zero unsupported existing desktop rows; 61 unique prop combinations. Three picker comparisons need backdrop-context recaptures; setup running/failed fixtures and exact stage pairing need correction. All interaction-only functionality remains in scope under action-driven evidence.

**Concerns:** Current-file hashes do not retroactively authenticate old screenshots; all existing desktop reference records are visibly unverified. No screenshots were reviewed or created here. Parent/Main should run the authorized current-artboard capture and visual comparison with exact file/stage pairing and source-byte receipts; this report is not UI parity acceptance.
