# Channel / CreateChannel — native real-browser interaction QA (V38)

**Target:** `#/channels/<id>` (3-tab V38 shell) + `#/channels/new` on the live
native app `http://127.0.0.1:8043` (schema 40, restored QA data, real Chrome
headless via Playwright, per-context `**/api/**` mutation fence: every
non-GET/HEAD/OPTIONS request aborted client-side and logged).
**Surfaces:** `pc` 1440×900 · `m` 390×844 · `pc-rm` / `m-rm` with
`reducedMotion:'reduce'` — 4 isolated browser contexts.
**Data truth:** `GET /api/owner-channels` → 3 channels (Chuyện Lạ Đêm Khuya,
Midnight Tales, Quiet Hours; all `sources:0`, `autoFetch:false`, `perDay:5`,
language `vi`), `GET /api/templates` → 3 templates, `GET /api/rivals` → **0**
followed rivals.
**Result:** 73/73 actions passed · **0** blocked mutations · **0** console/page
errors · **0** horizontal overflow · **0** out-of-viewport elements · **0**
clipped labels on every probed surface.

---

## 1. Real, confirmed bug — PC profile "Template mặc định" nav icon is a dashed fallback

| | |
|---|---|
| Where | Kênh › Hồ sơ kênh › "Lối tắt trong kênh" nav, middle row **Template mặc định** |
| Symptom | The 16×16 icon renders `maskImage:none; border:1px dashed rgba(0,0,0,0)→currentColor; border-radius:50%` — the kit's **missing-icon fallback** (a dashed circle), not a real mask. Visible in `pc-channel-settings.png` / `pc-rm-channel-settings.png`. |
| Cause | The **served binary is stale relative to source**. The bundled chunk `assets/ChannelScreen-C2_A7LOy.js` still calls `Icon name="layers"`; `layers` is not in `ICON_NAMES` and no `layers.svg` ships (`/assets/layers.svg` → 404; only `layer.svg` + `layer-bulk.svg` are emitted). Source `SettingsTab.tsx:319` already uses `name="layer"` (fixed), but the deployed `apps/viderse/internal/web/dist` build (Nov 5 01:34) predates the fix. |
| Evidence | `channel-native-qa.json › pc settings-template-nav-icon`: `{maskImage:"none", borderStyle:"dashed", borderRadius:"50%", title:null}`; bundle grep `name:"layers"` ×1; `rendersRealMask:false`. |
| Fix | Rebuild `apps/viderse/internal/web/dist` + the serving binary — no code change needed. |
| Scope | PC only by design: both nav rows are `navHidden` on phone (mobile only shows the **Trợ lý AI** shortcut — confirmed `hiddenNavRows:2`, `assistantLinkVisible:true`). |

The other two dashed elements flagged by the probe are **by-design empty-state
placeholders** (identity "Phong cách nhận diện" box 338×92 PC / 215×59 mobile,
banner placeholder), not missing icons.

## 2. Controls exercised — action → feedback (all passes on both viewports unless noted)

| Control | Action → observed feedback |
|---|---|
| Tab strip (PC `Nguồn & tự động · Template · Hồ sơ kênh`, mobile `Nguồn · Template · Hồ sơ`) | Click each → `aria-selected=true`, URL gains `?tab=sources|templates|settings`, correct panel swaps. Labels match MChannel (mobile short) / Channel (PC full). |
| Channel switcher — PC top-nav menu (`Đổi kênh, đang chọn …`) | Click → `role=menu` bounds `{x:70,y:51,289×252}` inside viewport, current channel listed · Esc → menu hides. (pc-switcher-menu.png) |
| Channel switcher — mobile top-bar (`vv-switcher` button) | Tap → `role=dialog` bottom Sheet `{0,470,390×374}` inside viewport, channel rows visible · **Esc** → closes · second open + **scrim tap** → closes. (m-switcher-sheet.png) |
| Side rail switch (PC) | Click *Midnight Tales* row → URL+heading change; click back → restored. `andBack:true`. |
| "Liên kết đối thủ" picker (Sources tab) | Trigger click → `role=dialog` popover `{484,440,340×180}` PC / `{19,239,340×185}` mobile, inside viewport · 0 checkbox rows (empty rivals — correct empty state: `Chưa theo dõi đối thủ nào` + `Theo dõi đối thủ mới ở Khám phá` link + submit disabled: `submitDisabledWhenEmpty:true`) · **Esc** closes + focus returns to trigger · second open + **outside pointerdown** closes. (pc-link-picker.png, m-link-picker.png) |
| Template mặc định picker (`Đổi template`, ResponsiveSelect) | Focus + Enter → PC `role=listbox` popover `{300,648,346×102}` / mobile Sheet `{0,554,390×290}`, inside viewport · 3 options listed (`Kiểm thử QA…`, `Mẫu chuẩn · Tách nền`, `Truyện kể đêm`) with per-option descriptions · ArrowDown · **Esc** closes, focus back on trigger, `nothingSelected:true` (no PATCH) · mobile **scrim tap** closes. (pc-template-picker.png, m-template-picker.png) |
| Composer "Dặn trợ lý" (PC-only dock) | Type `QA draft scout` → draft stays; open recipient picker (`Bàn với Trinh sát`) → `role=listbox` `{1198,675,206×136}` in view (rm) · pick **Đạo diễn** → input clears (per-agent draft) · pick **Trinh sát** → `QA draft scout` **restored** (`draftBackOnFirst`) · `Gửi` disabled while empty · Enter→ArrowDown→Esc all work, focus returns to trigger. Send never clicked (would PATCH `agentInstructions`). |
| Profile fields (Hồ sơ kênh) | `#vch-name` fill `…QA-marker` → draft accepted, saveline reads `Tự lưu khi bạn rời ô` · page **reload** → original restored (`reverted:true`) — proves no-write edit-state; blur-commit PATCH path intentionally not exercised. |
| "Xoá kênh" confirm | Click → `alertdialog`/`dialog` `{498,295,444×329}` PC / settled sheet `{0,440,390×404}` mobile inside viewport, title `Xoá kênh "Chuyện Lạ Đêm Khuya"?`, typed-phrase gate present · **Huỷ** → closes, no mutation. (pc-delete-confirm.png, m-delete-confirm-settled.png) |
| CreateChannel — legitimate nav | PC side-rail **Tạo kênh** + mobile switcher-sheet **Tạo kênh** link → `#/channels/new`. PC: h1 `Tạo kênh` + `Đối thủ làm nguồn · bắt buộc` + `Thông tin kênh` + `Cách kênh chạy` headings. |
| CreateChannel fields | `#vch-new-name(-m)` + `#vch-new-url(-m)` fill fine · rival checkboxes: **0** (no followed rivals) → `Tạo kênh` stays gated (`Chọn ít nhất 1 đối thủ…` note), never clicked. |
| CreateChannel "Cách kênh chạy" | `Chọn template` opens PC popover / mobile Sheet (inside viewport), options lead with `Mẫu chuẩn · gợi ý` — default `templateId:null` matches artboard · `Tự duyệt ảnh` Switch present (unchecked = `Tắt · bạn duyệt…` default) · `Video mỗi ngày` stepper defaults **5** (matches `perDaySub`/artboard `{{perDay}}` 5). |
| Stepper local state (create page — the only safe place) | `Giảm` → 5→4, `Tăng` → 4→5, type `99` + blur → **clamps to 20** (`clampedTo20:true`). Local `onPerDay` only; live-channel stepper on Sources tab never clicked. |
| CreateChannel cancel | PC **Huỷ** → navigates back to previous channel; mobile `.m-back` (`Quay lại`) → same. `navigatedAway:true`. |
| Automation card (Sources) | 2 switches (`Tự lấy video mới`, `Tự duyệt ảnh`) + 1 `Video mỗi ngày` input present — presence/state read only; toggles never flipped (each commits a PATCH). |

## 3. Geometry / touch findings

| Finding | Severity | Detail |
|---|---|---|
| **No horizontal overflow** anywhere | — | `docScrollWidth == innerWidth` on all 4 surfaces × 5 probes; `outOfViewCount=0`. |
| **Overlay bounds** | — | Every opened overlay (PC menu, mobile sheets, both pickers, both confirms, create template sheet) settles **inside** the viewport. Two early reads flagged `withinViewport:false` (composer popover `y=861` PC, delete sheet `y=844` mobile) — re-measured after `document.getAnimations()` finishes: both settle in-view (`{1198,675,206×136}`, `{0,440,390×404}`). Animation-timing artifact, **not** a product bug. |
| Mobile touch targets < 44px | **low–med** | Top-bar icon links & channel-switcher `h=36`; tab buttons `h=40`; stepper `Giảm`/`Tăng` `44×42`, stepper input `21×42`; `Xoá kênh` `111×36`; language radios `h=40`. Note: `button.vv-switcher` carries a `.touchHalo` span that inflates the real hit area beyond the measured bounding box (source `ChannelSwitcher.tsx:92`), so some numbers understate effective touch area — measure with the halo expanded if you gate on 44px strictly. |
| PC controls < 44px | by design | 21–33 per page (nav links 30px, rail rows 38px, logo 26px). 44px rule applies to coarse pointers only; reported for completeness, not violations. |
| Tab accessible name | **low** | `Nguồn & tự động0` / `Nguồn0` — the source-count badge is concatenated into the tab's accessible name with no separator. Screen readers will say "Nguồn zero". |
| Mobile create title | **low** | `MobilePage` renders the title as a plain `<span>` (`mobile/page.tsx`); PC uses `<h1>`. Slightly weaker landmark/heading semantics on phone. |
| Keyboard focus into sheets (mobile) | observation | After opening the `Đổi template` sheet, `document.activeElement` = `<section>` (the focus trap container), not the first `role=option` — intended rAF focus-into-list exists in `MobileSelect`; with `defaultOpen:false` the timing landed on the frame. Esc + focus restore still verified. |

## 4. Unexercised / unverified (by design or by data)

- **Rival row menus** (`Bỏ liên kết` confirm, mobile row `⋯` ActionSheet, `Làm video mới nhất`, `Chọn video nổi trội`): **unexercised** — all 3 restored channels have `sources:0` and `/api/rivals` is empty; exercising unlink requires linked rivals (would mutate). The *picker* open/dismiss paths above were fully verified with the real empty state.
- **Automation toggles + live stepper** on Sources tab: read-only inspected; clicking commits a PATCH to the live channel — intentionally skipped.
- **Template selection commit** (`pick` → `PATCH defaultTemplateId`), **composer send**, **rival link submit**, **delete confirm**, **create submit**, **add-rival submit**: opened/gated/verified but never confirmed — each writes.
- **perDay save/reload/restoration**: not re-tested — parent already proved it; task says do not repeat mutation.
- **"Video mới cho kênh" / YouTube link buttons**: navigation-only targets, not clicked in this pass (route exists; window.open path unexercised).

## 5. Source gaps vs artboards

- `Channel.dc.html` `tabDefs` still lists a 4th **Video** tab + `Template luân phiên`/`Đang áp dụng` copy while V38 ships 3 tabs (`Nguồn & tự động · Template · Hồ sơ kênh`) — artboard is behind the V38 contract (dev-prompt §Gói V38 confirms removal). Not a product bug; flagging for the canonical-doc owner.
- Mobile create artboard file is `MCreate.dc.html`, not `MCreateChannel.dc.html` as named in the task brief.
- Source `SettingsTab.tsx` icon fix (`layers`→`layer`) already landed; the **deployed bundle is stale** — the only real runtime defect found.

## 6. Artifacts (this directory)

- `channel-qa.cjs` — the harness (reusable; rerun: `node channel-qa.cjs`)
- `channel-native-qa.json` — full structured evidence (actions, bounds, probes, blockedMutations)
- `{pc,m,pc-rm,m-rm}-*.png` — 30 screenshots incl. overlays and settled states
- `pc-composer-popover-settled.png`, `m-delete-confirm-settled.png` — post-animation bounds proofs
