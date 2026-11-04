# V38 frontend cutover checklist — Kênh, Thư viện, Template mặc định

Status: **DONE_WITH_CONCERNS** (report-only; no product source, artboard, git, build, test or formatter change made).
Scope owner: PR4 / V38 frontend. Backend/schema peer: `backend-cutover-checklist.md`.
Gate: **V38 implementation must not start until V37 is merged** (parent is accepting V37 at `93545e71cb6d3357a3147385c25db0e862995d53`). V38 is one atomic FE+BE+schema cutover; this file only maps the FE half.

Sources read as truth (line numbers below are current):
- `plans/.../phase-05-v38-channels-and-library.md:12,18-21,30-34,44`
- `apps/viderse/docs/design/v1/08-dev-prompt.md:1054-1090` (V38 package + nghiệm thu), `:1006` (create 3 rows), `:1079` (wire contract)
- `apps/viderse/docs/design/v1/01-ia-flows.md:475-500` (W11), `:504-506` (W12 default template)
- `artboards/{Workflow,Channel,Library,Template,CreateChannel,Onboard,MChannel,MCreate,MLibrary,MTemplate,Dialogs}.dc.html` (tweaks/actions; not edited)
- `reports/inventory.md`, `verification.md`, `backend-contracts.md:15-16,23-32`

---

## 1. Contract to consume — one version, no aliases

Backend target is `backend-cutover-checklist.md:244,258,268,288,303-307`. FE must switch in the same PR; do **not** read `old ?? new`, keep `position`, or keep a client-side rotation fallback.

### 1.1 Channel JSON (read)

| Current `api/channels.ts` anchor | Current | V38 wire | FE handling |
|---|---|---|---|
| `:43-69` | `autoPick: boolean` | `autoFetch: boolean` | rename all reads; `VideosFresh/VideoStats/Queue/Explore` consumers are cut below |
| `:54` | `pacePerDay: number` | `perDay: number` | 1–20; do not clamp a loaded out-of-range value silently |
| `:58` | `autonomy: Autonomy` | `autoApprove: boolean` | threshold fixed **85**; delete preset/mode/pick/onError reads |
| `:59` | `priority: Priority` | — | delete |
| `:55` | `templateCursor: number` | — | delete; never resume rotation |
| `:63` | `templates: ChannelTemplate[]` | `defaultTemplateId: string` (+ optional `defaultTemplateName?`) | exactly one default; no cursor/order |
| — | — | `autoFetch`, `autoApprove`, `perDay`, `defaultTemplateId` are required on read | `defaultTemplateId` non-empty for a valid channel |

`ChannelTemplate` keeps identity/label only (`ID, Name`) — remove `position` from the FE type too.

### 1.2 Channel writes

| Endpoint | Current | V38 | FE obligation |
|---|---|---|---|
| `POST /api/owner-channels` (`api/channels.ts:71-80`) | `autonomy?`, `priority?`, `pacePerDay?` | `autoFetch?`, `autoApprove?`, `perDay?`, `defaultTemplateId?` | `defaultTemplateId` = user's choice in create row; omit ⇒ backend creates standard template. Never PATCH after create to emulate choice |
| `PATCH /api/owner-channels/:id` (`:82-92`) | same old fields + `confirmed` | `defaultTemplateId?`, `autoFetch?`, `autoApprove?`, `perDay?` | body sends **only changed field**; no optimistic duplicate writes |
| `POST /api/owner-channels/:id/templates` (`:160-161`, `ChannelTemplateAction :94`) | add/remove/move/standard | **route removed** | delete `setChannelTemplates`, action type, all callers; default changes go through PATCH |
| `DELETE /api/owner-channels/:id/sources/:rivalId` (`:154-157`) | returns `{autoPickPaused}` | returns `{autoFetchPaused}` | rename read/type; last-unlink state still turns autoFetch off |

### 1.3 Template/library refs

- `api/library.ts:139-147` `ChannelRef.position` is rotation-only: delete. `TemplateSummary.channels`/`TemplateDetail.channels` mean **channels whose `defaultTemplateId` is this template**, not "in rotation". Shared templates are legal for 2+ channels (MT + QH).
- `api/library.ts:377-380` `removeTemplateFromChannel` and OutputSection detach are not V38 actions: replace with "Đổi ở Kênh" navigation; do not call a delete-template endpoint.
- `GET /api/owner-channels` (`api/library.ts:384`) stays as owner channel refs for the default picker; library no longer needs channel summaries/status/queue.
- Channel summary `today.cap` now derives from `perDay` (`backend-cutover-checklist.md:289`); FE type does not change unless the wire shape actually changes.
- `GET /api/channels/:id/performance` + insight-apply die with `VideosList` (`backend-cutover-checklist.md:311`). Keep `requestMarkPosted`/`requestDeleteExport` metadata only.

### 1.4 Score data must survive the "autonomy" cut

V38 removes **channel autonomy config**, not image scoring. `ScoreBadge`/`ScoreBanner`/`readImageScore`/`ScoreSummary` remain needed for Review and for "mọi ảnh ≥ 85". Therefore:
- **Do not delete `api/autonomy.ts` wholesale** (contrary to the peer FE note at `backend-cutover-checklist.md:318`). Split: keep `ImageScore`, `ScoreSummary`, `readImageScore`, `readScoreSummary`; delete `Autonomy`, `Priority`, `readChannelAutonomy`, `patchAutonomy`, `withRow`, `withThreshold`, presets.
- Same split for `i18n/autonomy.ts`: keep `score` copy; delete `headChip`, `presets`, `rows`, `priority`, `dialog`.
- `components/autonomy/ScoreParts.tsx`, `scoreSummary.ts`, `score.stylex.ts`, `scoreSummary.test.ts` stay; `AutonomyCard.tsx`, `AutonomyDialog.tsx`, `autonomy.stylex.ts`, `dialog.stylex.ts`, `AutonomyCard.test.tsx`, `screens/channel/mobile/AutonomyMobile.tsx` go.
- `TodayRow.tsx` stays (pace facts only); it may move/rename but must not depend on deleted autonomy copy.

---

## 2. File-by-file change map

### 2.1 Shell, router, nav

| Action | File | Exact change | Evidence |
|---|---|---|---|
| Modify | `router.ts:22-23` | `CHANNEL_TABS = ['sources','templates','settings']`; `ChannelTab` follows; old `videos` deep link falls back to sources in `ChannelScreen:38` | `08-dev-prompt.md:1060` |
| Modify | `router.ts:165` | `NavKey` gains `'library'`; **keep `'new'`** because `MobileShell` still uses it | `MainNav` 5 nouns; mobile keeps center + |
| Modify | `router.ts:166-171` | `navSection`: `library` + `template` → `'library'`; `channel`/`channel-new` stay `'channels'` | `library/template` currently return `'channels'`, wrong active item |
| Modify | `router.ts:151-154` | keep `paths.new` for PR4 button target; `paths.library('templates')` etc. unchanged | V39 owns dialog routing |
| Modify | `shell/MainNav.tsx:66-74` | replace item `{key:'new', label:t.nav.new, path:paths.new()}` with `{key:'library', label:t.nav.library, icon:'folder', path:paths.library()}`; items become Bàn dựng · Duyệt · Kênh · Khám phá · Thư viện | `08-dev-prompt.md:1059`; board `Main.dc.html:920-926` |
| Modify | `shell/AppHeader.tsx:151-153` | add `NewVideoButton` in `zoneRight` **before JobsPill**: icon `video-add`, label "Làm video", `<kbd>N</kbd>`, 32px; PR4 href `#${paths.new()}`, PR5 swaps to dialog handler | board `Main.dc.html:927-929`; V39 owns dialog |
| Modify | `shell/MobileShell.tsx:26` | leave tabs (Bàn dựng · Duyệt · + · Kênh · Khám phá); do **not** add mobile Library tab | V40 keeps bottom bar; MLibrary entered from Channel → Template |
| Modify | `i18n/vi.ts:41-49` | add `nav.library: 'Thư viện'`; keep `nav.new` until PR5 removes the `/new` route | V38 top nav |
| Modify | `shell/nav.test.tsx` | assert 5 nav items, Library active on `/library` and `/templates/:id`, NewVideoButton label/href present, no "Video mới" item | acceptance gate |

### 2.2 Channel desktop — `screens/channel/`

| Action | File | Exact change | Evidence |
|---|---|---|---|
| Modify | `ChannelScreen.tsx:34` | `TABS = ['sources','templates','settings']`; default tab `'sources'` even when old URL has `tab=videos`/absent | `08-dev-prompt.md:1060-1061`; board `Channel.dc.html:1149` |
| Modify | `ChannelScreen.tsx:38` | `route.tab` validation falls back to `'sources'`, not `'videos'` | no phantom video tab |
| Modify | `ChannelScreen.tsx:46-55` | keep `data`, `channels` (channel-only rail list) and `jobs` (4 stats); remove `rivals`, `templates`, `prompts`, `overlays`, `backdropChannels`, `backdropVideos`, `selecting`, `dockSlot` | side rail + VideosTab are gone |
| Modify | `ChannelScreen.tsx:86-87` | delete `referenceRival` derivation (only VideosTab) | dead after tab removal |
| Modify | `ChannelScreen.tsx:100-108` | replace `ChannelSideRail` with channel-only rail (my channels + create); no Library group | `01-ia-flows.md:477` |
| Modify | `ChannelScreen.tsx:131-138` | delete VideosTab branch + composer dock portal + `ChannelAside` | `08-dev-prompt.md:1061,1075` |
| Modify | `ChannelHeader.tsx` | replace `autonomy.headChip` with: `Tự lấy · {perDay} video/ngày` or `Tự lấy đang tắt`; add `Tự duyệt ảnh ≥ 85` **only when autoApprove**; keep language chip | `01-ia-flows.md:478` |
| Modify | `SourcesTab.tsx:31-33,101-121,226-250` | remove `AutonomyCard`, `AutonomyCardMobile`, `PrioritySeg*`, `ScheduleCard`, right column; add one `AutomationCard` (desktop) / same rows mobile; keep source list + `TodayRow` inside row 2 | `01-ia-flows.md:485-489`; board `Channel.dc.html:1180-1240` |
| Add | `screens/channel/AutomationCard.tsx` (or reuse/rename in place) | 3 rows: `Tự lấy video mới` switch → PATCH `{autoFetch}`; `Video mỗi ngày` stepper 1–20 + `TodayRow` + `Xem đối thủ ngay` → PATCH `{perDay}`; `Tự duyệt ảnh` switch + `≥ 85` → PATCH `{autoApprove}`. Disabled autoFetch with reason when no rivals. No confirm on either switch; toast | `08-dev-prompt.md:1064`; `01-ia-flows.md:486-489` |
| Modify | `PaceStepper.tsx:18,28-38` | min 1, max **20**; `disabled` at 1/20; never emit 0/21; no loaded-value clamp | `08-dev-prompt.md:1079`; `phase-05:42` |
| Rewrite | `TemplatesTab.tsx` | one `DefaultTemplateCard` (thumbnail, name, recipe, health). `Đổi template` floating list of Library templates + link `Xem trong Thư viện`, selection PATCH `{defaultTemplateId}` then toast; `Sửa template` → `paths.template(id, 'section')`; error health banner "video mới sẽ dừng ở bước dựng"; delete cursor/next/rotation rows/move up/down/remove | `01-ia-flows.md:490` |
| Modify | `SettingsTab.tsx:314,322,353-354` | `Xem N video đang làm` → `paths.workbench({channel: id})` (not `paths.channel(id,'videos')`); template link shows `defaultTemplateName` (not `templates.length`); no "Quy tắc đã học" row | `01-ia-flows.md:479,495-496` |
| Modify | `VideoStats.tsx:23` | `channelStats(... autoPick: channel.autoPick ...)` → `autoFetch` | four stats remain |
| Modify | `videosLogic.ts:125,147` | signature/body `autoPick` → `autoFetch` | stats only; keep wording queued auto on/off |
| Delete | `VideosTab.tsx`, `VideosFresh.tsx`, `VideosList.tsx`, `VideoRow.tsx`, `VideoBulkBar.tsx`, `ChannelAside.tsx`, `applied.ts`, `ScheduleCard.tsx` | after refs are zero; delete their tests: `VideosTab.test.tsx`, `VideosFresh.test.tsx`, `applied.test.ts`, `screens/channel/schedule.test.ts` | `08-dev-prompt.md:1075`; `phase-05:19` |
| Delete | `screens/channel/mobile/AutonomyMobile.tsx`, `components/autonomy/{AutonomyCard.tsx,AutonomyDialog.tsx,autonomy.stylex.ts,dialog.stylex.ts}` | after `SourcesTab` cutover; keep score/TodayRow files | only SourcesTab imports them |

**Reference display data:** MT/QH → `Mẫu chuẩn · Tách nền`; CL → `Truyện kể đêm` (`phase-05:34`). These are QA fixtures, never seeded names/IDs in production code.

### 2.3 Channel mobile — same implementation, narrow branches

There is **no `MChannelScreen.tsx`**. MChannel = narrow branches inside `ChannelScreen.tsx`, `SourcesTab.tsx`, `TemplatesTab.tsx`, `ChannelHeader.tsx`, plus `mobile/mchannel.stylex.ts`. Changes above already cover it; specifically:
- 3 tabs (`Nguồn & tự động` default, `Template`, `Hồ sơ kênh`), no Video tab/dock (`SourcesTab.tsx:118-121`, `TemplatesTab.tsx:53`).
- `AutomationCard` rows stack; stepper 1–20; chips same text.
- `DefaultTemplateCard` + bottom sheet template list; no link "Chọn template khác" (create row) / no link to Library for choosing default.
- Keep mobile back/hierarchy to Channel; do not add a mobile Library nav entry in PR4.
- `MChannel` board footer props already match: `tab`, `fresh`, `identity`, `sheet` (`MChannel.dc.html:1101`).

### 2.4 Library desktop — `screens/library/`

| Action | File | Exact change | Evidence |
|---|---|---|---|
| Modify | `LibraryScreen.tsx:126-186,271-278` | remove `<aside>` entirely + channel/library switcher; remove `.vlb-shell` side grid column / `side*` styles; content full width | `08-dev-prompt.md:1066`; board `Library.dc.html` has no rail |
| Modify | `LibraryScreen.tsx:48-58,263` | drop loads/imports used only by rail (`getChannelSummaries`, `queue`, `status`, `ChannelAvatar`, tints); keep templates/prompts/overlays and `ownerChannels` for default refs | no dead data fetches |
| Modify | `TemplatesTab.tsx:36-48,107-137` | card label `Mặc định của {names}` / `Chưa kênh nào dùng làm mặc định`; avatars = default channels; delete lock reason names default channels | `01-ia-flows.md:505-506`; board `Library.dc.html:1516,1521` |
| Modify | `TemplatesTab.tsx:126-137` | menu `Đặt làm mặc định cho kênh…` (replaces `Thêm vào kênh`); still opens `AttachChannelDialog` as a **channel chooser**; no direct write | W12 says "(mở Kênh)"; board `Library.dc.html:1401` navigates |
| Modify | `TemplatesTab.tsx:129-136` | remove duplicate Export menu item (export icon already in card row); menu = Đổi tên · Đặt làm mặc định cho kênh… · Xoá | board menu |
| Modify | `AttachChannelDialog.tsx:4-5,21-23,30,40` | copy/title/action: "Đặt làm mặc định cho kênh…" / "Chọn kênh để mở Kênh › Template"; target remains `paths.channel(id,'templates')`; close on success. Do not add a set-default API call | current navigation already matches W12's "(mở Kênh)" |
| Modify | `i18n/library.ts:111,195,366-367,546-552,565,597` | replace rotation wording: `Mặc định của…`, `Chưa kênh nào dùng làm mặc định`, `Kênh dùng làm mặc định`, `Đặt làm mặc định cho kênh…`, `Mở Kênh › Template`; delete `usersPosition`, `usersNote` rotation text, `deleteLocked` old wording | copy gate §7 |
| Keep | Rename/duplicate/export/import/prompt/backdrop/overlay actions | V38 only changes template default semantics; do not touch prompt/backdrop/overlay editors | 0 dead-button gate |

### 2.5 Template detail — desktop + mobile

| Action | File | Exact change | Evidence |
|---|---|---|---|
| Modify | `TemplateScreen.tsx:199` | chip `Mặc định của {n} kênh` (MT, QH) / `Chưa kênh nào dùng làm mặc định` (replace `Dùng ở n kênh` / `Chưa vào kênh`) | `08-dev-prompt.md:1067`; board `Template.dc.html:1220-1245` |
| Modify | `TemplateScreen.tsx:217` | delete menu locked reason must name default channels, not "đang dùng" | `01-ia-flows.md:506` |
| Modify | `screens/library/template/OutputSection.tsx:93-114` | card title `Kênh dùng làm mặc định`; per row `Đổi ở Kênh` navigates `paths.channel(id,'templates')` (no detach API); remove `channel.position` row; remove note about rotation; empty copy `Chưa kênh nào dùng làm mặc định.` | `01-ia-flows.md:505` |
| Modify | `OutputSection.tsx:95-96` | button `Đặt làm mặc định cho kênh…` → open `AttachChannelDialog`/channel picker, then navigate; no write | W12 |
| Modify | `lib/mobile/MTemplateScreen.tsx:176,202,313` | same OutputSection labels; `more` sheet unchanged; no new API | `MLibrary`/`MTemplate` contract gate |
| Modify | `api/library.ts:139-147,377-380` | remove `position`, remove `removeTemplateFromChannel` after no callers; `channels` = defaults | backend contract |
| Modify | `api/library.ts:384` | keep `listOwnerChannels` (channel refs only) for picker/empty states | no side rail needs full summaries |

### 2.6 Create / Onboard — desktop + mobile

| Action | File | Exact change | Evidence |
|---|---|---|---|
| Modify | `ChannelNewScreen.tsx` (create call + step props) | pass selected `defaultTemplateId` + `autoApprove`; `labels.template` selection now writes into `CreateStep`; create call includes chosen default (omit when Mẫu chuẩn untouched) | `08-dev-prompt.md:1006`; backend create supports `defaultTemplateId?` (`backend-cutover-checklist.md:244`) |
| Modify | `CreateStep.tsx:340-439` | desktop exactly 3 rows: Template (floating select, no layout push; options = library templates + `Mẫu chuẩn · gợi ý`), Tự duyệt ảnh switch, Video mỗi ngày 1–20. Remove "Mức tự chủ" row and separate "Chọn template khác" link | `01-ia-flows.md:330`; board `CreateChannel.dc.html:1050-1090` |
| Modify | `CreateStep.tsx:150-262` | mobile same 3 rows; template opens bottom sheet (functional in PR4, animation polish PR6); remove autonomy row/“Chọn template khác” link | `01-ia-flows.md:330`; `MCreate.dc.html` |
| Modify | `DoneStep.tsx` | read `perDay` not `pacePerDay`; summary template = `defaultTemplateName`/chosen label; no autonomy copy | new wire |
| Modify | `api/channels.ts:71-80` | `CreateChannelInput` new fields; delete `autonomy/priority/confirmed` | wire contract |
| Keep | identity ready/generating/failed/retry/cancel | do not change; real errors + continue after error | `phase-05:43` |

Onboard uses the same `ChannelNewScreen/CreateStep`; no separate file.

### 2.7 Cross-screen consumers forced by the contract (must be in PR4)

| Action | File | Current old read | V38 cutover | Later owner |
|---|---|---|---|---|
| Modify | `screens/review/ReviewScreen.tsx:111-112,460` | channel `autonomy` + presets | pass `{autoApprove}`/85 to `ScoreBanner`; banner when off: `"{kênh} chưa bật Tự duyệt ảnh nên vẫn hỏi bạn"` | PR6 final V40 copy/motion; **functional cutover PR4** |
| Modify | `components/autonomy/ScoreParts.tsx:22-40` | `Autonomy`, presetName, `approve.mode` | accept `{autoApprove: boolean; threshold: 85}`; keep `ScoreSummary`; delete preset labels | PR4 functional |
| Modify | `screens/review/useReview.ts:5,147-153` | it reads the **image** `summary` via `readScoreSummary(result.autonomy)`, not channel autonomy | keep per-image score summary; only remove channel-level autonomy reads. Do not delete `readScoreSummary` | PR4 functional |
| Modify | `screens/review/mobile/MReviewScreen.tsx` | same props through shared ScoreParts | same as review | PR6 layout only |
| Modify | `screens/explore/ExploreScreen.tsx:72` | `channel.templateCursor` / `templates[cursor]` | use `defaultTemplateName` or template list lookup by `defaultTemplateId` | PR5 (V39) owns picker UI; PR4 only contract |
| Modify | `screens/explore/mobile/MExploreScreen.tsx:60-62,567` | `pacePerDay` chip | `perDay` | PR6 layout; PR4 data |
| Modify | `screens/explore/mobile/MVideoScreen.tsx:50` | `templates[cursor]` | default template | PR5/6 |
| Modify | `screens/new/NewScreen.tsx:95,163` | `channel.templates`; label `Theo luân phiên` | options = chosen `defaultTemplateId` + `listTemplates()` (or default-only until PR5); remove rotation label | PR5 owns dialog rewrite; PR4 keeps route functional |
| Modify | `screens/queue/QueueScreen.tsx:110,371` | `autoPick` names | `autoFetch` names | PR5 owns V39 queue rules; PR4 field rename |
| Modify | `screens/queue/QueueScreen.tsx:192,408`, `api/clone.ts:98-103` | `no_template` wait/skip | remove case/union after FK guaranteed; update fixtures/tests | PR4 (producer dies with FK) |
| Modify | `screens/new/NewScreen.tsx:29,83,117` + `api/clone.ts:44` | `fromRotation` plan template | remove `fromRotation` from type/UI; backend plan returns default/explicit | PR5 final dialog |
| Modify | `screens/workbench/*` only where `channel.templates`/`templateCursor` is read | must be zero after PR4 | do not redesign workbench | V37 already accepted |
| Modify | `data/keys.ts:69-79` | `channelPerformance`, `channelSchedule` keys | delete after last consumer (VideosList/ScheduleCard deleted this PR) | PR4 |
| Keep | `components/performance/MarkPosted.tsx:32-45` | mark-posted/export | untouched; keep copy for mark | backend contract |
| Modify | `i18n/performance.ts` | full performance/insights copy | keep mark-posted/export strings only; delete insights/posted copy | PR4 |
| Modify | `i18n/all.ts` | exports `autonomy`, `performance` | keep trimmed score/autonomy copy + performance mark; delete deleted modules' exports | PR4 |

### 2.8 Tests to update/delete (do not run in this prep session)

- Rewrite: `screens/channel/ChannelControls.test.tsx` (currently asserts 4 tabs + posting-slots + autonomy PATCH at `:73-131`), `channels.test.tsx` (`:171` create body `pacePerDay`), `SettingsTab.test.tsx`, `library/library.test.tsx`, `library/mobile/mobile.test.tsx`, `review` fixtures, `new/NewScreen.test.tsx`, `explore` fixtures, `QueueScreen.test.tsx`/`MobileQueue.test.tsx` (`no_template`).
- Delete with owning components: `VideosTab.test.tsx`, `VideosFresh.test.tsx`, `applied.test.ts`, `screens/channel/schedule.test.ts`, `components/autonomy/AutonomyCard.test.tsx`.
- Add/keep: `PaceStepper` boundary tests 0/1/20/21; create 3 rows; default-template PATCH; chip visibility; no "Mức tự chủ/luân phiên" text; mobile 3 tabs.

---

## 3. Obsolete consumers — zero-reference gate before merge

Run these checks after implementation (expected zero non-test hits, exact pattern set):
- `pacePerDay`, `autoPick`, `autoPickPaused`, `templateCursor`, `autonomy.`, `priority`, `ChannelTemplateAction`, `setChannelTemplates`, `removeTemplateFromChannel`, `readChannelAutonomy`, `useAutonomySave`
- `VideosTab`, `VideosFresh`, `VideosList`, `VideoRow`, `VideoBulkBar`, `ChannelAside`, `ScheduleCard`, `appliedRows`, `channelSchedule`, `channelPerformance`
- `fromRotation` (UI/API type; backend plan response may still carry it only if peer keeps it — FE must not depend)
- `Theo luân phiên`, `Mức tự chủ`, `Ưu tiên kênh` in live desktop/mobile copy
- `tab=videos` deep link still resolves to Sources; no `ChannelTab` includes `videos`

Do not delete `VideoStats.tsx`, `videosLogic.ts` (4 stats remain), `TodayRow`, `ScoreParts`, `scoreSummary`, `MarkPosted`, `requestDeleteExport`, shared source pacer.

---

## 4. Artboard tweak × action acceptance matrix — desktop

Canonical artboards are targets, not editable here. "Pass" means the implemented screen reproduces board output and the action has the listed observable effect. Screenshot/control audit per `verification.md`.

### 4.1 `Channel.dc.html` (props: `tab`, `channel`, `autoApprove`, `tplMenu`, `identity`, `link`, `fresh`)

| Tweak / action | Expected observable | Fail if |
|---|---|---|
| default load / `tab=sources` | 3 tabs only; Sources selected; no Videos/Applied/Schedule/priority | 4 tabs, `videos` selected, right column, ScheduleCard |
| `channel=MT\|CL\|QH` | 4 stats, source list, default template, profile all swap atomically; left rail current item | mixed channel data, wrong default |
| click another channel in left rail | URL `/channels/:id?tab=sources`; loads that channel; current highlight moves; no full page flash | stale header/stats |
| `autoApprove=true` | header chip `Tự duyệt ảnh ≥ 85`; row switch on; no confirm; PATCH only `{autoApprove}`; toast 4s | chip absent/wrong, confirm dialog, extra body fields |
| `tplMenu=true` | floating list of Library templates + `Xem trong Thư viện`, anchored, **no layout shift**; Esc returns focus | pushing content, native select, navigation on open |
| choose template in list | PATCH `{defaultTemplateId}`; card name/health/thumb update; toast; URLs/jobs unchanged; running jobs unchanged | local-only change, PATCH other fields, list closes without feedback |
| `fresh=true` (AD) | empty-source/empty-video state, 4 zero stats, avatar, no crash/low-data assumptions | NaN stats, missing avatar |
| `link=none/picker/empty` | `Liên kết đối thủ` opens floating picker; `Liên kết N` adds N sources; last unlink pauses autoFetch with chip `Tự lấy đang tắt` | page navigation, inline list, no reload |
| `Tự lấy` on/off | PATCH `{autoFetch}`; chip `Tự lấy · {perDay} video/ngày` / `Tự lấy đang tắt`; disabled with reason when no rivals | chip stale, silent failure |
| `Video mỗi ngày` +/- | 1→20 only; at 20 + disabled, at 1 − disabled; PATCH `{perDay}` once; `Hôm nay` stays correct; no 0/21 request | 21/50, double PATCH, clamped on load |
| unlink last rival | confirm → DELETE; response `autoFetchPaused`; autoFetch switches off; prior videos/queue unchanged; next rival becomes reference | force-delete without confirm, autoFetch remains on |
| unlink non-last | confirm names source + real affected video/image counts; confirm → DELETE; **Cancel sends zero write**; no undo toast unless exact restore exists | invented counts, request on cancel, fake undo |
| open template / edit | `Sửa template` opens `/templates/:id`; error template shows "video mới sẽ dừng ở bước dựng" | disabled/dead link |
| stats footer | `Xem trong Bàn dựng` → workbench filtered by channel; avatar/4 stats match board | links to old videos tab or no-op |
| deep link `?tab=videos` | falls back to Sources (or redirects to sources), no error/blank | blank panel |

### 4.2 `Library.dc.html` (props: `tab`, `menu`, `prompt`, `nguon`, `import`, etc.)

| Tweak / action | Expected observable | Fail if |
|---|---|---|
| load | no left rail; full-width content; top nav `Thư viện` active | rail or channel switcher remains |
| `menu=true` | floating ⋯ menu: Đổi tên · Đặt làm mặc định cho kênh… · Xoá (disabled when default, reason names channels) | "Thêm vào kênh", rotation copy, direct delete |
| click `Đặt làm mặc định cho kênh…` | channel chooser dialog; choose → Channel › Template tab; no default write from Library | silently writes default, dead item |
| default template card | `Mặc định của MT, QH` or `Chưa kênh nào dùng làm mặc định`; avatars = defaults | "Dùng ở…", rotation count |
| delete default | blocked with names + "Đổi template mặc định ở Kênh › Template trước rồi xoá."; no 409 after click | deletes/FK error |
| `tab=templates`, rename, duplicate, export, import | existing actions still work; toasts 4s | regressed wiring |
| `tab=prompts/backdrops/overlays` | unchanged V38 scope; no dead buttons introduced | collateral edits |

### 4.3 `Template.dc.html` (props: `section`, `preview`, …)

| Tweak / action | Expected observable | Fail if |
|---|---|---|
| header | chip `Mặc định của 2 kênh` (MT, QH) or empty; no `Dùng ở`/`Chưa vào kênh` | wrong membership |
| `section=output` | card `Kênh dùng làm mặc định`; each row `Đổi ở Kênh` → channel template tab; button `Đặt làm mặc định cho kênh…` → picker → channel; no detach write; no position text | removes default, no navigation, rotation note |
| delete default | locked, names 2 channels; explanation exact | deletes or generic error |
| duplicate/export/preview | unchanged and working; `Mở trong editor` is a link | dead |
| `preview=fresh/stale/building` | no regression from V37 | — |

### 4.4 `CreateChannel.dc.html` / `Onboard.dc.html` (props include `tplMenu`, `identity`, `rivals`)

| Tweak / action | Expected observable | Fail if |
|---|---|---|
| create step | exactly 3 rows `Template` · `Tự duyệt ảnh` · `Video mỗi ngày`; no Mức tự chủ | 4 rows/old 3-row autonomy |
| `tplMenu=true` | floating template list incl `Mẫu chuẩn · gợi ý`; no push; phone bottom sheet; no link to Library | navigation, native select |
| choose template | row shows chosen name; POST includes `defaultTemplateId`; if untouched omit (server standard) | POST still creates standard; no field |
| autoApprove switch | default off; POST `autoApprove:true`; no threshold editor | old preset UI |
| perDay stepper | 1–20, default 5, boundaries disabled; POST `perDay`; 400 field error shows | 50/0, silent fallback |
| identity `generating/failed` + retry/cancel | input preserved, real error, can continue; mobile same | reset form |

### 4.5 `Dialogs.dc.html`

| Check | Pass |
|---|---|
| `dialog=unlink` | title/source name; body states kept videos/queue, next reference, last-source autoFetch pause; Cancel no write |
| no `autonomy` option | no "Chuyển sang Tự chạy"; no threshold/preset dialog |
| redraw-all/delete/etc. | unchanged V38 scope |

---

## 5. Mobile shared-contract acceptance matrix (PR4 functional, not final V40 layout)

The mobile screens must compile and work against the new wire **in PR4**. PR6 may change layout/animation/tabs polish, never reintroduce old fields.

| Screen | Source | PR4 pass | PR6 ownership |
|---|---|---|---|
| **MChannel** | `ChannelScreen` narrow + `SourcesTab` mobile + `TemplatesTab` mobile + `ChannelHeader` + `mchannel.stylex.ts` | 3 tabs Sources/Template/Profile; same chips; automation 3 rows with switch/stepper 1–20; default template card + bottom-sheet list selection PATCH; no Video tab/dock; `sheet=template/link/rival/unlink` behaviors same as desktop | final layout/motion, equal tab rhythm, sheet animation |
| **MCreate** | `ChannelNewScreen` + `CreateStep:150-262` `narrow` | 3 rows; template bottom sheet; identity error/retry/cancel with preserved input; POST `defaultTemplateId` when chosen | final sheet animation/layout |
| **MLibrary** | `screens/library/mobile/MLibraryScreen.tsx` | 4 tabs stay; template `⋯` action label `Đặt làm mặc định cho kênh…`; opens channel chooser; card text `Mặc định của…`/`Chưa kênh nào…`; empty state; `active.channels` interpreted as defaults | V40 equal tabs / counts-below / motion |
| **MTemplate** | `screens/library/mobile/MTemplateScreen.tsx` + shared `OutputSection` | section `Kênh dùng làm mặc định`; `Đổi ở Kênh` navigates; default chip; more sheet; delete lock names channels | V40 preview/motion |
| Deep links | `/library/:tab`, `/templates/:id/:section`, `/channels/:id?tab=template` | all reachable and 3-tab URLs accepted; old `tab=videos` → sources | no nav redesign in PR6 |

---

## 6. Overlay, focus, animation, feedback acceptance (V38 behavior)

- **Floating list/Popover** (template change, create template, link sources, Library menus, `Vẽ thử với`, `Sắp xếp`): absolutely anchored, overlays content, **does not push layout**; outside click/Esc closes; Esc returns focus to trigger; `aria-haspopup`/`aria-expanded` correct (`08-dev-prompt.md:1006`, `phase-05:44`).
- **Mobile sheets** (MChannel template/link/rival/unlink, MLibrary actions/new, MCreate template, MTemplate more): existing kit sheet; body scroll lock; back/Esc closes; no route change unless the action is a route (`Sửa template`, `Đổi ở Kênh`).
- **Async row state**: toggle/stepper disable only its own control; pending visible; no duplicate request from double click; error inline/toast and value reverts to server readback.
- **Toast**: kit `.vv-toast`, bottom center, auto-dismiss 4s; success after PATCH/default change/link/unlink; no success toast on failed write.
- **Destructive confirmation**: unlink only; autoApprove/autoFetch no confirm; delete template uses blocked reason, not a confirm; delete channel unchanged (typed name).
- **Animation**: PR4 only state continuity (no layout jump when chip/list changes; no image/avatar distortion; round avatars). Final reveal/slide motion belongs PR6; do not add bespoke animation here.
- **Dock check (V37 media bug regression)**: after removing VideosTab/composer portal, no horizontal/vertical dock element remains; `phase-05:44` checks avatar circular + no dock bug.

---

## 7. Vietnamese copy acceptance (exact; desktop gate)

Required:
- Nav: `Thư viện`; button `Làm video` + key hint `N`.
- Channel header: `Tự lấy · {n} video/ngày`; `Tự lấy đang tắt`; `Tự duyệt ảnh ≥ 85` (only on); language chip unchanged.
- Automation rows: `Tự lấy video mới`; `Video mỗi ngày`; `Tự duyệt ảnh`; `Hôm nay {done}/{cap} · lần xem gần nhất`; `Xem đối thủ ngay`; note: `Gặp lỗi máy tự thử sửa 1 lần rồi mới báo; không bao giờ tự đăng, xoá, đổi template, bỏ liên kết đối thủ.`
- Template tab: `Template mặc định`; `Đổi template`; `Xem trong Thư viện`; `Sửa template`; error: `Video mới sẽ dừng ở bước dựng` (board casing "video mới sẽ dừng ở bước dựng" is acceptable; pick one and use everywhere).
- Settings: `Xem {n} video đang làm`; template link label `Template mặc định`; no `Quy tắc đã học`.
- Library: `Mặc định của {names}`; `Chưa kênh nào dùng làm mặc định`; `Đặt làm mặc định cho kênh…`; `Đang là template mặc định của {n} kênh. Đổi template mặc định ở Kênh › Template trước rồi xoá.`
- Template detail: `Kênh dùng làm mặc định`; `Đổi ở Kênh`; `Đặt làm mặc định cho kênh…`.
- Create: `Cách kênh chạy`; `Template`; `Mẫu chuẩn · gợi ý`; `Tự duyệt ảnh`; `Video mỗi ngày`; range hint `1–20`.
- Review functional cutover: `{kênh} chưa bật Tự duyệt ảnh nên vẫn hỏi bạn` (V40 final wording may refine in PR6; it must not say "Mức tự chủ").
- Backend 400 surfaced: `Nhịp mỗi ngày phải trong khoảng 1–20.`

Forbidden in live desktop and mobile copy touched by this PR:
`Mức tự chủ`, `luân phiên` (template rotation), `Ưu tiên kênh`/`Ưu tiên cả N`, `Quy tắc đã học`, `Đang áp dụng cho kênh`, `Lịch đăng` (channel page), `Thêm vào kênh` (default action).
"Chờ tới lượt kênh này"/channel scheduling language in Queue is not template rotation and may remain until PR5.

---

## 8. Conflicts and gaps, by source line

### Conflicts (report; do not silently pick)
1. **perDay range 1–50 vs 1–20.**
   - Target 1–20: `08-dev-prompt.md:1079`, `01-ia-flows.md:487`, `phase-05:12,28,42`, `backend-cutover-checklist.md:391`.
   - Stale 1–50: `01-ia-flows.md:330`; `CreateChannel.dc.html:1271` and `MCreate.dc.html:1081` still clamp increments at 50. `Channel.dc.html:1337` and `MChannel.dc.html:1205` also contain 50 but are overridden by their V38 vals (`Channel.dc.html:1420`, `MChannel.dc.html:1244` clamp 20). **Visible create boards currently fail at 21+**; canonical artboards cannot be edited by this task. Owner: design/docs or parent before screenshots. FE follows 1–20.
2. **Create/Onboard autonomy text stale.** `01-ia-flows.md:328` still says create pre-fills "Mức tự chủ Cân bằng"; `:330` (r41b) supersedes it with 3 rows. Implement 3 rows; flag doc cleanup.
3. **MChannel status stale.** `01-ia-flows.md:499` says "MChannel chưa đổi (đợt 44)", but `:288,480` say 3 tabs equal on phone and the board footer already exposes `tab=sources|templates|settings` (`MChannel.dc.html:1101`). Treat the board + V38 contract as target in PR4; PR6 only final layout.
4. **Board dead code / latent fallbacks** (visual pass may hide them): `Channel.dc.html:1162` defaults `'videos'`; `Channel.dc.html:1337` 50 clamp; `MChannel.dc.html:1205` 50 clamp; `MChannel.dc.html:1152-1153,1179` reference undeclared `approve/autonomy` props; `Dialogs.dc.html` has an unreachable `autoApprove: k === 'autonomy'` branch. They are not product source, but control audit must not count hidden handlers.
5. **Library "attach" is navigation, not a write.** Board `Library.dc.html:1401` `setDef` navigates to Channel; W12 `01-ia-flows.md:505` says "(mở Kênh)". Current `AttachChannelDialog` already navigates to `paths.channel(id,'templates')`. Therefore change copy only; do **not** invent a library-side `set default` API. Channel PATCH remains the only default write.
6. **Review needs scores after autonomy removal.** Peer FE deletion note `backend-cutover-checklist.md:318` says delete `api/autonomy.ts`; `ScoreParts.tsx:1-40` and Review rely on `readImageScore`/`ScoreSummary`. Split the module instead (see §1.4). Values 84/85/missing remain a V38 backend gate; FE must render whatever score summary returns without inferring approval.

### Gaps requiring a decision before FE completion (not solvable from current sources)
1. **Unlink impact counts.** `phase-05:30` requires confirm "theo số video/ảnh ảnh hưởng thật, Huỷ không request ghi"; current `DELETE .../sources/:rivalId` returns only `{autoPickPaused}`→`{autoFetchPaused}` (`api/channels.ts:154-157`, `backend-cutover-checklist.md:272,306`). No FE endpoint/data provides affected video/image counts. Options for parent/backend (pick one, not a product decision): add counts to DELETE preflight/response, or add a read-only impact route. Until then FE must not fake counts; keep current generic confirm or defer exact copy. **Owner: Backend/PR4.**
2. **`defaultTemplateName` is optional** in the peer wire (`backend-cutover-checklist.md:288,307`). FE must resolve display name from `listTemplates()` when absent; no invented fallback name. Confirm the endpoint is available on Channel/Library initial load.
3. **Create default template.** Peer supports `POST /owner-channels` body `defaultTemplateId?` (`backend-cutover-checklist.md:244,303`); UI selection is required by `08-dev-prompt.md:1006`. FE must send it; if backend drops this field before implementation, create row must become display-only — a product change, escalate, do not PATCH-create a second template.
4. **Auto-approval score summary shape.** V38 changes channel config only; confirm `GET /api/clone/:id/images` summary still exposes enough to render "chưa bật Tự duyệt ảnh" without `channel.autonomy`. FE will take `{autoApprove, threshold:85}` from channel; summary only for scored/below lists.
5. **Old `?tab=videos` links.** Router validation drops it to sources; no redirect route. This is accepted from V38 "bỏ tab Video", but external bookmarks will land on Sources. No new redirect product decision.

---

## 9. PR5 / PR6 ownership boundaries

**PR4 (V38) must complete**
- Wire cutover: Channel JSON/writes, template default refs, autoFetch/perDay, remove rotation/autonomy/priority, delete template action route, `autoFetchPaused`.
- Top nav 5 items + active Library for `/library` and `/templates/*`; NewVideoButton link (PR5 swaps to dialog).
- Channel 3 tabs, 4 stats + workbench link, AutomationCard, DefaultTemplateCard, unlink confirm, no old right rail.
- Library full width, default labels/menu, Template default section, delete lock reason.
- Create 3 rows + POST default template.
- All desktop + mobile API consumers cut; obsolete files/tests/i18n gone; score module split.
- 0 dead buttons in Channel/Library/Template; no `Mức tự chủ/luân phiên` on desktop.

**PR5 (V39) owns — do not pre-build in PR4**
- `NewVideoDialog`, top button/`N` switching from route to dialog, route `/new` redirect to prior page.
- Explore multi-select dialog, filters/limit popovers, `Vì sao?` removal, preview template label.
- Queue run rules (`workers` only), priority removal, template default display.
- `api/clone` canonical `POST /api/jobs`, `templateId` override behavior, dedup mapping.
- Final removal of `NewScreen`/`ModeTabs` and `api/clone` old creation.
- PR4 may keep `/new` route and existing Explore/Queue UI working on the new read contract only.

**PR6 (V40) owns — layout/tweak/animation only**
- MChannel final layout/motion, MLibrary 4 equal tabs + counts below, MTemplate polish, MCreate sheet animation, MSettings/MReview/MExplore mobile details.
- No contract changes; no mobile old-field reads introduced.
- Review final copy/"Vì sao?" removal; motion/toast polish.
- Round 45 mobile image viewer remains excluded.

---

## 10. Acceptance gate for parent before starting V38 FE

Parent can hand this list to the implementer without rediscovering callers:
1. Confirm V37 merged; branch/revision for V38.
2. Confirm backend wire fields exist exactly as §1.1–1.3 (especially `perDay`, `autoFetch`, `autoFetchPaused`, `defaultTemplateId`, optional name, create support).
3. Resolve gap §8-1 (unlink counts) and §8-3 (create template) before writing UI copy/flow.
4. Implement §2 in the order: API types → shared score split → shell/router → Channel → Library → Template → Create → mobile actual files → cross-screen consumers → delete tests/i18n.
5. Run (post-implementation, not in this prep): `npm test -w apps/viderse/web -- screens/channel screens/library screens/review screens/queue screens/new screens/explore`; typecheck/build; artboard tweaks/screenshots/controls per `verification.md`; grep gate §3.
6. Verify acceptance from `08-dev-prompt.md:1084-1090`: channel switch updates all data; template change → floating list + toast; autoApprove on → header chip; no "luân phiên/Mức tự chủ/Ưu tiên kênh" on desktop; Library active from top nav and on Template; 0 dead buttons on Kênh/Thư viện/Template.
7. Mobile spot-check MChannel/MLibrary/MTemplate/MCreate against new contract before PR4 sign-off (PR6 polish may follow).

Acceptance evidence is only source-referenced; no build/test run performed for this document. Product source untouched; only this report was created.
