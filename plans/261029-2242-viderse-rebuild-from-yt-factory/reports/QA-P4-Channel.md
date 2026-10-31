# QA P4 · Kênh (Channel)

Artboard-fidelity QA of the rebuilt **Channel** area against `docs/design/v1`, run independently
(no repo edits). Every state was rendered twice — real Chrome at `127.0.0.1:18751` and the artboard
at `127.0.0.1:18752` — at **1440×900** (desktop pairs) and **390×844** (usability pass only).

---

## 1. How it was run

| Step | Detail |
|---|---|
| Build | `npm run build` in `apps/viderse/web` (lock-serialised), then `go build -o $HOME/viderse ./apps/viderse/cmd/viderse` |
| DB | `viderse_qa_p4ch`; `migrate up`; admin `qa` / `qa-pass-2026-xyz` through `/api/setup/verify` → `/api/setup/admin` |
| Serve | `viderse serve` on `:18751` (named service), artboards `npx serve -l 18752` over a copy of `docs/design/v1` in `/tmp/viderse-qa-p4ch/design` |
| Artboard state switching | runtime `window.__dcSetProps('<Root>', {…})` on one loaded file per artboard (root names are `Channel.dc`, `Dialogs.dc`, `CreateChannel.dc`, `Onboard.dc`; `serve` strips `.html`) |
| Fonts | app's Be Vietnam Pro (400/500/600) + JetBrains Mono woff2 copied into the copy and injected as `@font-face` (Google Fonts blocked); verified `document.fonts.size = 11` |
| Chrome | dedicated `--user-data-dir=/tmp/viderse-qa-p4ch/chrome`, headless new, `PublicKeyCredential` removed, `viderse:setup-checked=1` seeded; login via `fetch('/api/login')` |
| Seed | 5 rivals + 74 videos inserted as `dat` (no YouTube/Twitch/Bilibili requests, no AI calls): slot geometry reproduces the artboard's VN rail exactly — T3+T5 06–08 = 17, T2 10–11 = 9, T7 21–22 = 7, T6 19–20 = 6, T4 20–21 = 5, CN 08–09 = 4 (sample 90 videos, 6 slots); `scan_state`/`scan_jobs`/`scan_schedule`/`platform_state` faked so no scanner runs |
| Channels | `Midnight Tales` (3 rivals, 3 templates, branding assets), `Chuyện Lạ Đêm Khuya`, `Quiet Hours`, two `After Dark Stories`; identity `generating`/`failed` set purely via SQL (`identity_state`, `identity_error*`, `identity_retry_at`, future `identity_lease_until` + one `background_tasks` row) — nothing executed |
| Measurement | `getBoundingClientRect`, `getComputedStyle`, `scrollingElement.scrollWidth` vs `innerWidth`, DOM row counts, `aria-*`/`disabled`, focusable target heights |
| Evidence | `/tmp/viderse-qa-p4ch/shots/` (kept) |

Server and artboard stopped, Chrome killed, DB dropped after the pass.

---

## 2. State pairs

| # | State | App shot | Artboard shot |
|---|---|---|---|
| 1 | Channel › Video (fresh) | `app-channel-videos.png` | `ref-channel-videos.png` |
| 2 | Channel › Nguồn, `link=none` | `app-channel-sources.png` | `ref-channel-sources.png` |
| 3 | Channel › Nguồn, picker open | `app-channel-picker.png` | `ref-channel-picker.png` |
| 4 | Channel › Nguồn, unlink confirm → empty | `app-channel-unlink-confirm.png`, `app-channel-empty.png` | `ref-channel-empty.png` (artboard has no desktop unlink sheet; mobile `MChannel`) |
| 5 | Channel › Template luân phiên | `app-channel-templates.png` | `ref-channel-templates.png` |
| 6 | Channel › Hồ sơ kênh, identity=ready | `app-channel-settings.png` | `ref-channel-settings.png` |
| 7 | … identity=generating | `app-channel-id-gen.png` | `ref-channel-id-gen.png` |
| 8 | … identity=failed | `app-channel-id-failed.png` | `ref-channel-id-failed.png` |
| 9 | Delete-channel dialog | `app-channel-delete.png` | `ref-delete-channel.png` |
| 10 | Channel switcher popover | `app-channel-switcher.png` | `ref-channel-switch.png` |
| 11 | CreateChannel `step=create`, rivals=some / none / empty | `app-create-some.png`, `app-create-none.png`, `app-create-empty.png` | `ref-create-some.png`, `ref-create-none.png`, `ref-create-empty.png` |
| 12 | CreateChannel `step=done`, identity running / failed / ready | `app-done-running.png`, `app-done-failed.png`, `app-done-ready.png` | `ref-done-running.png`, `ref-done-failed.png`, `ref-done-ready.png` |
| 13 | Onboard `step=create` | `app-onboard-create.png`, `app-onboard-create-some.png` | `ref-onboard-create.png` |
| 14 | Onboard `step=done` (same shell as #12) | `app-done-ready.png` | `ref-onboard-done.png` |
| 15 | Explore: PreviewPanel channel picker unlocked, rival menu, queue lock | `app-explore-preview.png`, `app-explore-rival-menu.png`, `app-explore-link-menu.png`, `app-explore-link-picker.png` | Explore.dc.html (no dedicated shot; picker + menu markup read) |
| 16 | Mobile 390×844: Video / Nguồn / Hồ sơ / Tạo kênh | `app-channel-m-videos.png`, `-m-sources.png`, `-m-settings.png`, `-m-create.png` | desktop `Channel.dc.html` ≤700 breakpoint |

Confirmed **exact matches** worth recording: VN rail counts `17 / 9 / 7 / 6 / 5 / 4`, summary
`90 video gần nhất của 3 đối thủ · 6 khung lặp lại`, `T3 · T5` day chips, `×2,7` + `48:40` on the
fresh latest-video card, sources/templates badges `2`/`3`/`1`, picker rows correctly blocked
(`Đã liên kết`, `đang dùng cho …`), delete-channel typed-name gate, and the done-step pipeline card
(`Đang chạy nền` + 3 orb rows + `Việc tiếp theo`) — the artboard's done step *is* that card
(`CreateChannel.dc.html:299-320`), so the app matches it.

---

## 3. Major

### M1 · "New York" time-zone tab wraps onto two lines (user-reported)
- **App**: `button.vch-zone__btn` "New York" = **44.4 × 24 px**, `white-space: normal`; the label
  wraps inside a 24 px-tall box (text overflows/clips). See `app-channel-sources.png` and the user's
  crop. The `radiogroup` is 85 px wide, two buttons.
- **Artboard**: pill is **52.5 × 15 px** with `white-space: nowrap`
  (`Channel.dc.html:1338` — `{ id: 'ny', label: 'New York' }`).
- **Cause**: `web/src/screens/channel/channel.css:353-357` — `.vch-zone__btn` sets
  `height: var(--vv-s-24); padding: 0 var(--vv-s-8); font-size: 11.5px` and never `nowrap`, so the
  flex item shrinks below the label width.
- **Fix**: in `channel.css` add `white-space: nowrap; flex: 0 0 auto;` to `.vch-zone__btn`
  (optionally `padding: 0 var(--vv-s-10)`); no markup change needed.

### M2 · 390 px touch targets: 18/27 (Video), 30/42 (Hồ sơ), 11/21 (Tạo kênh) are under 40 px tall
- **Measured at 390×844** (no horizontal overflow in any state: `scrollWidth === innerWidth === 390`;
  page heights 1559 / 2663 / 3564 / 1537).
- Interactive elements with height < 40 px: **Video 18/27**, **Hồ sơ kênh 30/42**, **Tạo kênh 11/21**
  (chips, `Tuỳ chỉnh` links, zone pills, template/per-day steppers, icon buttons). The one-hand rule
  in the brief is 44 px.
- **Artboard**: desktop artboard ≤700 breakpoint keeps 30–36 px controls
  (e.g. `Channel.dc.html:1019` ghost buttons `height: 30px`), so this is a *design* gap too, not only
  implementation — but the brief's mobile gate is explicit about 44 px.
- **Fix**: add a ≤700 px rule in `channel.css` bumping `.vch-chip`, `.vch-applied__link`,
  `.vch-zone__btn`, `.vch-ic`, steppers to `min-height: 44px` (and 44 px min width for icon-only
  buttons); or rule the gate as "links exempt" and record it.

---

## 4. Minor

### m1 · Slot rail order is weekday-first, artboard is count-first
- **App** (`app-channel-sources.png`): `T2 10:00–11:00 (9)`, `T3 · T5 (17)`, `T5 20:00–21:00 (5)`,
  `T6 (6)`, `T7 (7)`, `CN (4)` — sorted by weekday then time (`schedule.ts:35` comment, sort at
  `schedule.ts:62`).
- **Artboard** (`Channel.dc.html:1331-1338`, `cslots`): `T3·T5 17`, `T2 9`, `T7 7`, `T6 6`, `T4 5`,
  `CN 4` — biggest block first.
- **Effect**: the "which window matters most" read is lost; the first row is not the densest.
- **Fix**: sort merged slots by `count` desc (then weekday/time) in
  `web/src/screens/channel/schedule.ts:62`.

### m2 · "nhiều nhất" badge appears on two rows
- **App**: badge shown on `T3 · T5` **and** `T6` (`app-channel-sources.png` text dump). The flag is
  per-rival (`schedule.ts:23` — "khung này là nhiều nhất ở ít nhất một đối thủ", rendered via
  `postingTop`, `vi.ts:332`), so several rows qualify.
- **Artboard**: no such badge exists anywhere in `Channel.dc.html`; the design marks nothing.
- **Effect**: a singular label reads as "the biggest", but two rows carry it.
- **Fix**: either drop the badge (match artboard) in `ScheduleCard.tsx`, or only set `top` on the
  global maximum row.

### m3 · Sources row meta drops scan state and cadence
- **App** (`app-channel-sources.png`): `1,24 Tr người theo dõi · 30 video đã quét · xem mới đăng trước`
  (`SourcesTab.tsx:99-120`).
- **Artboard** (`Channel.dc.html:1283`): `1,24 Tr người theo dõi · 3 video/tuần · đang quét lịch sử
  1.284/2.000`, plus a `Đang quét lịch sử` chip for the scanning rival.
- **Effect**: the reference rival shows no scan progress and no cadence; `scan_state.indexed_videos`
  exists in the API but is unused on this screen.
- **Fix**: compose meta as `subs · <perWeek> video/tuần · <scan state>` in
  `web/src/screens/channel/SourcesTab.tsx:99-120` (per-week from `median` cadence or the schedule
  model; scan text from `scanState`).

### m4 · Link-rival picker is a centred modal, artboard is an in-card popover
- **App** (`app-channel-picker.png`): page-dimming modal (`Dialog`, `SourcesTab.tsx:285-351`) with 5
  rows, `Đã liên kết` / `đang dùng cho …` correctly disabled.
- **Artboard** (`Channel.dc.html:901-907`): `aria-haspopup="dialog"` + `position: relative` card
  anchored under `Liên kết đối thủ`, `role="checkbox"` rows, inline footer with the `Liên kết N`
  button — no scrim, channel context stays visible.
- **Fix**: render `LinkPicker` as an anchored popover (or accept the modal and record it).

---

## 5. Artboard issues to reconcile (not app defects)

1. `tabSettings` is **never defined** — `Channel.dc.html:1355` sets `tabIdentity` for
   `tab === 'settings'` and line `1033` gates the whole "Thông tin kênh" + writing block on
   `{{tabSettings}}`, which stays undefined → the settings artboard can only ever show the identity
   grid. The app shows identity **and** channel info/writing; it follows the block's evident intent,
   so treat the artboard as stale rather than the app as over-built.
2. `fresh` is referenced (`if (fresh)` at `Channel.dc.html:1379`) but **not declared** in `data-props`,
   so the "kênh mới" Video variant is unreachable in the design tool (only via runtime override).
3. `Onboard.dc.html` `step=create` renders the full app nav header, while the app uses the slim
   `Viderse · Lần đầu mở` shell for first run (`app-onboard-create.png`). The app matches the comment
   at `AppShell.tsx:74-80`; the artboard is out of date.
4. Applied table mismatch is **accepted**: artboard lists `Mức tự chủ` (autonomy) and labels row 1
   `Lấy video`; app lists `Tự lấy video` and omits autonomy — autonomy blocks are an accepted
   deviation.

---

## 6. Accepted deviations verified (no action)

- Queue-dependent controls locked with a reason: `Video mới cho kênh`, `Làm video`,
  `Làm video mới nhất`, Explore `Làm video` — all show `Cần hàng chờ để làm video. Hàng chờ chưa mở.`
- No `Hôm nay x/y` counts, no channel chat composer, no Library projects tab, template
  `preview=building` unreachable — app simply does not render them.
- Video fresh lead's second card is `Tự lấy video` instead of the artboard's `Hàng chờ` (documented
  substitution in `VideosTab.tsx:1-5`).
- Channel rail shows 4–5 channels vs the artboard's 3 — data, not layout.
- Done-step identity `failed` shows the pipeline row `Họa sĩ … Tạo nhận diện hỏng.` instead of the
  artboard's `chưa vẽ được avatar và ảnh bìa: ChatGPT hết hạn mức` + `Tự thử lại 07:00`
  (`CreateChannel.dc.html` `bgTasks`) — same structure, different fixture text; not counted.

## 7. Not verified / out of scope

- `Escape` on the delete dialog did not close it in my session, but the cancel control is labelled
  `Huỷ` (`vi.ts:23`) and my first finder looked for `Hủy`; I never actually clicked cancel, so this is
  **unconfirmed** — re-test before filing.
- Identity `generating` percentage is data-driven (`DoneStep.tsx:64-66` reads `task.progress`); my
  synthetic task row produced `0%`, so progress rendering is unverified with real runs.
- Explore PreviewPanel picker/menu were captured but not pixel-diffed (belongs to the Explore pair
  set): picker unlocked with 4 channels, rival menu `Liên kết với kênh…` opened the channel dialog and
  linked successfully, queue reason present.

---

## Status

**Major: 2** (M1 New York toggle wrap — 44.4×24 vs 52.5×15 nowrap; M2 mobile touch targets
18/27 · 30/42 · 11/21 under 40 px at 390).
**Minor: 4** (m1 slot order weekday-first vs count-first; m2 `nhiều nhất` on two rows; m3 sources meta
missing scan/cadence; m4 picker modal vs in-card popover).
**Artboard issues: 4** (dead `tabSettings`, undeclared `fresh`, stale `Onboard` header, accepted
applied-table drift). **Blocked checks: 2** (delete-dialog escape, generating %).
