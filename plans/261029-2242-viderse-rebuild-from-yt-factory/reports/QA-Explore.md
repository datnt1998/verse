# QA · Explore (Khám phá, desktop 1440×900)

Independent artboard-fidelity review, 30/10/2026. The reviewer did not build this screen. Everything below is measured from a real Chrome render of the app against a real render of `artboards/Explore.dc.html` tweaks, never inferred from source.

Status: DONE_WITH_CONCERNS — **4 majors / 13 minors**. Grid, columns, cards, rail cards, bulk bar, heat map and slot rows are pixel-accurate; the majors are two clipped segmented labels, a wrong empty-state title, a missing empty-state sidebar column, and an invisible focus target on the two filter chips.

## How it was run

- Build: `go build ./apps/viderse/cmd/viderse` (embedded web dist current, web not rebuilt), served on `127.0.0.1:18797` against a dedicated DB `viderse_smoke_qaexplore`.
- Artboard: a copy of `docs/design/v1` under `/tmp` served read-only on `127.0.0.1:18798`; every state is a temp copy with only the `data-props` default and the `this.state` seed edited (outside the repo). No design or app file was modified.
- 1440×900, Chrome headless, `allowed_domains: 127.0.0.1, localhost`, `delete window.PublicKeyCredential`. The harness reports `devicePixelRatio` 1.25, so PNGs are 1800×1125; all numbers below are CSS px.
- Google Fonts is outside the allowlist, so the design copy loaded the app's own Be Vietnam Pro 400/500/600 and JetBrains Mono woff2 into the reference page — a system-font fallback is therefore not being mistaken for a type defect.
- Thumbnails from `i.ytimg.com` are blocked by the harness; empty thumbs are ignored, not counted.
- Seed (`rivals.sql` schema, SQL as `dat`): 5 rivals / 2.140 videos matching the artboard sample — Nocturne Stories median 286.000, the 8 sample videos with the artboard's exact titles, durations, view counts (1,2 Tr / 1,08 Tr / 944 N / 812 N / 780 N / 766 N / 512 N) and the artboard's slot pattern (30 videos: 12 in T3+T5 06:00–08:00, 6 in T7 21:00–22:00, 5 in T2 10:00–11:00 “mới”, 4 in CN 08:00–09:00 “hai tuần một lần”, 3 off-slot), `newOutliers` 12 for the rail badge.
- `scan_state.head_checked_at`/`scanned_at`/`platform_state.scanner_lease_until` were set to now/ahead and the history job was written directly with a QA-held channel lease, so nothing was due. **No YouTube network attempt appears in the server log**; scan states were produced by changing only `scan_jobs.state`.
- Incident, disclosed: right after login the app routed to the first-run machine setup and began installing tools before the reviewer reached `#/explore`. It wrote ~1,9 GB into the sandbox `HOME` (`/tmp/viderse-qa-explore/home/.viderse/...`) only; no repo file, DB schema or screen state was affected.
- Evidence: `/tmp/viderse-qa-explore/shots/` — `art-<state>.png` ↔ `app-<state>.png` plus `<state>.json` (every node's box and computed font/colour/border/gap).

## Per-state pairs

| State | Files | Verdict |
|---|---|---|
| default, Tất cả đối thủ | `art-default.png` ↔ `app-default.png` | Columns 252/872/316 and the 268×256,1 card are exact. Filter block collapses to one row (see m2), sort labels clip (M1). |
| one rival selected | `art-rival.png` ↔ `app-rival.png` | Same deltas as default; rail correct. |
| empty | `art-empty.png` ↔ `app-empty.png` | **M2** wrong H1, **M3** no sidebar column; orb 72 vs 40 (m10). |
| preview | `art-preview.png` ↔ `app-preview.png` | Block/thumb/typography match; missing “Tự lấy” note and chapters are known gaps; back-button height m9. |
| preview + why | `art-previewwhy.png` ↔ `app-previewwhy.png` | Panel position/width match; fact set, missing threshold link and two muted colours m8. |
| select (bulk bar) | `art-select.png` ↔ `app-select.png` | Bar 54 h / r16 / internals exact; primary label is the locked variant, chevron m13. |
| rivalMenu | `art-menu.png` ↔ `app-menu.png` | 4 items, order, disabled reason and danger item correct; width/gap/hint m7. |
| scan=running | `art-default.png` (scan default running) ↔ `app-scanrunning.png` | 1.284 / 2.000, 64 % progress, copy and buttons match; card 171,9 px identical. |
| scan=paused | `art-scanpaused.png` ↔ `app-scanpaused.png` | “Đã tạm dừng …” + “Tiếp tục / Huỷ” match, card 153,2 px identical. |
| scan=limited | `art-scanlimited.png` ↔ `app-scanlimited.png` | Copy and “thử lại lúc HH:MM” match (time value is data). |
| scan=done | `art-scandone.png` ↔ `app-scandone.png` | Button “Quét lại” + 2.000/2.000 match; copy “định kỳ” m5. |
| posting VN, slot clicked | `art-vnslot.png` ↔ `app-posting-vn-slot.png` | Card 553,4 px, slot row 59,1 px, bar/chip/rhythm widths (48/65,2/105,7) identical; ring on the 2 cells of T3+T5; text format m6. |
| posting New York, slot clicked | `art-nyslot.png` ↔ `app-posting-ny-slot.png` | Zone switch shifts slots correctly (T2+T4 19:00–21:00, T7 10:00–11:00, CN 23:00–00:00, T7 21:00–22:00) and the ring follows the picked block; “New…” clips (M1). |
| add-rival error | design has no such state (07 contracts the copy) ↔ `app-add-error.png` | Correct: `invalid_input` copy under the field, warn border, field keeps input. No defect. |
| unfollow confirm | `Dialogs.dc.html` confirm (kit) ↔ `app-unfollow-confirm.png` | Dialog, danger title, body, “Huỷ / Bỏ theo dõi” match the kit confirm; button role swap is the kit's. |
| keyboard focus | — ↔ `app-focus-select.png` | Sort radiogroup: roving Tab + ArrowLeft/Right works; “Dài” chip received focus; **M4** the two select-backed chips show no focus ring. |
| 20.000-video rival, scrolled | — ↔ `app-grid20k.png` | **24 `article.vx-card` nodes max**, 8 rows × 3 cols, 594 DOM nodes total for the page, 0 horizontal overflow (`scrollWidth == clientWidth == 1440`), counter “400 / 20.000 video”. |

## Majors

**M1 — Both segmented controls clip their labels.** `app-default.png` / `app-posting-ny-slot.png` ↔ `art-default.png` / `art-nyslot.png`.
Sort group: artboard radios are content-sized 69,2 / 77 / 80,8 px and read “Nổi trội · Mới nhất · Lượt xem”; the app splits the 168,8 px track into three equal 51,6 px radios and shows “Nổi trội · Mới nh… · Lượt x…”. Zone group: artboard 32,5 / 68,5 (“VN / New York”), app 37,5 / 37,5 with “New…”. Root cause is the kit segmented splitting width equally and clipping, used at `FilterBar` `[role=radiogroup]` and `PostingCard` `.vx-posthead`; `web/src/screens/explore/explore.css` only sizes the container. Fix: let segments size to content (`width:max-content` per segment, or pass per-option intrinsic width) instead of `flex:1 1 0` + `text-overflow`, as the artboard does. This is also the reason the sort group is 70 px narrower than the artboard, which cascades into m2.

**M2 — Empty state prints the screen title instead of its own heading.** `art-empty.png` ↔ `app-empty.png`. Artboard H1 “Chưa theo dõi đối thủ nào”; the app renders “Khám phá”. `ExploreScreen.tsx` empty branch uses `{tt.title}` while `screens.explore.emptyTitle` already holds the right string (unused). Wrong copy on the first screen a new user sees.

**M3 — Empty state drops the left sidebar column.** `art-empty.png` ↔ `app-empty.png`. The artboard keeps the 252 px sidebar with the “Đối thủ 0” group plus “Đối thủ bạn theo dõi sẽ hiện ở đây, kèm số video nổi trội mới.”; the app returns `<main class="vx-empty">` only, so the “Thêm đối thủ” block centres at x=720 instead of x=846 (−126 px) and the whole rival-list affordance disappears. `ExploreScreen.tsx` empty early-return should render the shell with `RivalSidebar` (empty list) and centre only the middle column.

**M4 — The two select-backed filter chips have no visible focus indicator.** `app-focus-select.png`. Tab order from the sort group is: checked sort radio → “Dài ≥ 8 phút” (ring visible) → “Đăng trong 30 ngày” → “Trạng thái” → first card. The last two chips wrap a fully transparent `<select style="opacity:0">`; when it takes focus the 1 px outline is painted on the 0-opacity element, so nothing changes on screen, and the focus ring of the label wrapper is never triggered (`:focus-within` is not styled). Additionally the locked “Trạng thái” chip (label `aria-disabled`, all options disabled) is still tabbable. Fix: `.vx-chip:focus-within { outline: 2px solid var(--vv-accent); outline-offset: 2px }`, and either remove the status chip from the tab order or make the locked state non-focusable while keeping the reason in a tooltip.

## Minors

- **m1 — “Dài ≥ 8 phút” lost its chevron.** Artboard chip is `clock + “Dài ≥ 8 phút” + arrow-d`, 134,8 px; the app's is 116,8 px with no trailing icon (art markup line 790). `FilterBar` renders `Icon name="clock"` for this chip only; add the same `arrow-d` the two other chips have (and note the chip is not a menu, so if the chevron exists only as an affordance keep it static `aria-hidden`).
- **m2 — Filter block occupies one row instead of two.** Artboard: chips end at x=1022, the 99 px counter wraps to a second line (y=161, block 117→180,4) and the grid starts at **194,4**. App: controls are 71 px narrower (M1 + m1 + m3), the counter stays on the same line (y=139) and the grid starts at **171** (−23,4 px). Layout only; resolves itself once M1/m1/m3 are fixed.
- **m3 — “Chọn nhiều” lost its button chrome and is 4 px too tall.** Artboard: 32 px high, `border:1px solid`, label `#EDEDEF` (line 784). App: 36 px (`ActionButton` default), no border (`variant="quiet"`), label `#A0A0AB`, which also makes the head row 36 instead of 32 and pushes the filter row 4 px.
- **m4 — Sidebar group starts 6 px low.** “ĐỐI THỦ” label y=115 (art) vs 121 (app); rival rows are 54 px in both, so all six later rows inherit the +6. `.vx-add` `padding-bottom` + `.vx-grp` `padding-top`.
- **m5 — Scan-done copy differs.** App “Đã quét đủ giới hạn. Video mới vẫn được kiểm tra **định kỳ**.” vs artboard “**mỗi giờ**” (`vi.ts` line 281). Artboard wording matches the 1 giờ rescan cycle shown in the ⋯ menu hint; “định kỳ” is defensible but is not verbatim.
- **m6 — Slot line formatting.** App “06:00 – 08:00” (spaces around the dash) and “12 / 30”; artboard “06:00–08:00” and “11/30”. `PostingCard`/`posting.ts` should emit an en dash without spaces and `n/30` without spaces.
- **m7 — Rival menu geometry and copy.** 264 px wide vs 272; items 67,8 / 50 / 67,8 / 67,8 tall with 0 gap vs 70,5 / 53,8 / 70,5 / 88,2 with 2 px gap. The “Bỏ theo dõi” hint is much shorter than the artboard's (“Gỡ khỏi Khám phá và khỏi Midnight Tales. Video đã làm giữ nguyên; 2 video trong hàng chờ bị bỏ.”), and item 3 reads “Liên kết với kênh của tôi” where the artboard says “Liên kết với kênh…”. Popover width/gap come from the kit; the copy is `vi.ts`.
- **m8 — “Vì sao?” panel content.** App shows 4 facts (lượt xem, mức thường, “Ngưỡng nổi trội ×3”, giờ đăng); the artboard has 3 and then the action link “Đổi ngưỡng ở Kênh › Nguồn & tự động”, which the app omits entirely. Muted tones also differ: source `#7E7E8A` vs `#8B8B97`, reason `#C9C9D1` vs `#B4B4BE`. Facts are contract-driven; the missing link is a missing element (no threshold settings screen yet) — flagged, not counted major because the destination does not exist.
- **m9 — Preview header spacing and queue hint.** The artboard renders “← Thông tin đối thủ” as a 19,4 px bare text button, the app as a 30 px control, shifting the whole preview block 10,6 px down. Queue hint: app “Lên đầu hàng chờ khi bấm Làm video.” vs artboard “Lên đầu hàng chờ · bắt đầu khoảng 00:40, 1 chỗ đang trống.” (no queue timing data exists yet). The artboard's own “× lượt xem gấp lần mức thường” line is missing its numbers via a template bug; the app's “lượt xem gấp 4,2 lần mức thường” is correct — not a defect.
- **m10 — Empty-state orb.** `Orb size={72}` vs artboard 40×40. `ExploreScreen.tsx` line 161.
- **m11 — Empty-state examples.** Artboard `@nocturnestories`, `youtube.com/@thelatearchive`, `UCx9…Qw`; app `@motphim`, `@vnreview`, `youtube.com/@kenhda`. Copy fixture in `screens.explore`, same count and shape.
- **m12 — No breadcrumb in the header.** Artboard header leads with “Khám phá / ‹scope›”; the app header is logo + centred nav + right status. Shared shell (`MainNav`), so out of the Explore component but visible on every Explore capture; the scope is only communicated by the sidebar selection.
- **m13 — Bulk primary.** Label “Làm 3 video” vs artboard “Làm 3 video cho Midnight Tales”, and the artboard's trailing `arrow-d` is absent; disabled styling itself is correct per the locked-control contract. Bar width 614,6 vs 756,1 follows from this.

## Verified equal (no finding)

252 / 872 / 316 columns; card 268 × 256,1 with 16:9 thumb 150,8 and r10; row gap 14 / column gap 12; score badge 20 h / r6 / 11,5 px 600; duration badge mono 10,5; title 13,5/500 lh 1,4, meta 12; rail card 283–284 wide, scan card 171,9 and paused 153,2, posting card 553,4, slot 59,1, heat cell 12 h with 6 px column stride and 1,5 px ring on slot blocks; chips 28 h / r999 / 12,5 px; bulk bar 54 h / r16 with 26 px quick buttons, 36 px copy/primary, 34 px close; selection checkbox 20 × 20 r6 with the accent fill; toast/tooltip for the locked “Làm video” button resolves to “Chưa có kênh của bạn để làm video — tạo kênh ở mục Kênh.”; locked buttons are disabled and explainable, as the contract requires.

## Excluded as data / harness, not defects

Thumbnails blank (domains blocked); “200 / 1.274 video” vs “24 / 1.284” (page size and seeded dates); “Mỗi tuần 5 video”, “Nổi trội 4” vs 3/11 (seeded counts — the artboard's own 11 outliers with only 8 cards is internally inconsistent); rival sublines show `@handle` instead of a linked channel name and the rail shows “Chưa liên kết kênh nào” instead of “Liên kết với Midnight Tales · Đổi ›” (no owner channel, locked by `local://p03-contract.md`); “Làm video” instead of “Làm cho Midnight Tales” and the used-video state line (same reason); missing preview “Tự lấy” note and chapter list (features not built, documented in `P3-ExploreWeb`); the NY slot “c” rhythm differs from the artboard because the Sunday 23:00 NY block belongs to the previous week — the app's computation is correct; the 20.000-video rival's own counter “22.140 video · 12 mới” (seed artefact of adding a 6th rival for the DOM test).

## Counts

**4 majors / 13 minors**, counted per finding (M1 covers both segmented controls, m1/m9/m13 group their duplicated occurrences).

Status: DONE_WITH_CONCERNS — Explore matches the artboard on every measured box of the grid, cards, rail cards, bulk bar, scan states, posting slots and heat map, and caps the DOM at 24 cards on a 20.000-video rival; the open majors are the two clipped segmented labels, the empty-state title/sidebar, and the invisible focus ring on the select-backed filter chips.

---

# Round 2 — re-review after `Fix-Explore.md` (30/10/2026)

Same setup and rules as round 1: rebuilt `go build` binary on `127.0.0.1:18797`, fresh `viderse_smoke_qaexplore` with the identical seed, artboard tweaks rendered from a `/tmp` copy on `127.0.0.1:18798` with the app's Be Vietnam Pro injected locally, 1440×900 Chrome headless, loopback-only domains, WebAuthn deleted, **plus** `sessionStorage.setItem('viderse:setup-checked','1')` in `init_scripts`. No setup screen opened, no tool install ran this round (server log clean, no `/api/tools/*` needed). Evidence: `/tmp/viderse-qa-explore/shots/r2/` — `art-*` re-captured and `app-*` fresh; measurements are CSS px.

## Round-1 findings, re-checked

| # | Round-1 finding | Result |
|---|---|---|
| M1 | Segmented clips labels | **PASS** — sort radios 69,2 / 77 / 80,6 px (art 69,2 / 77 / 80,8), full “Mới nhất / Lượt xem”; zone radios 33 / 69 (art 32,5 / 68,5), “New York” full. |
| M2 | Empty H1 wrong | **PASS** — “Chưa theo dõi đối thủ nào”. |
| M3 | Empty without sidebar | **PASS** — 252 px sidebar with “ĐỐI THỦ 0” + guidance line; empty box centred at x=846 exactly; exactly 1 form. |
| M4 | No focus ring / locked chip tabbable | **PASS with a caveat** — `:focus-within` ring verified on both chips (2 px); locked “Trạng thái” chip removed from Tab order. Caveat: the fix's own change caused regression **R1** below. |
| m1 | “Dài” chip chevron | **PASS** — chevron present, chip 134,8 px = art. |
| m2 | Count wraps in art / one row in app | **PASS** — counter now wraps to y=161 on the second row (block 117→180,4, grid 194,4 = art). |
| m3 | “Chọn nhiều” chrome | **PASS** — button 118,3×32 px, 1 px border (`rgba(255,255,255,.12)`), label `#EDEDEF`, head row 32 px. |
| m4 | Sidebar group +6 | **PASS** — “ĐỐI THỦ” y=115 = art; rows aligned. |
| m5 | “định kỳ” vs “mỗi giờ” | **PASS** — card reads “… kiểm tra mỗi giờ.” |
| m6 | `06:00 – 08:00` / `12 / 30` | **CLOSED — r1 false positive.** The round-1 diff script joined React's split text nodes with spaces; the rendered `textContent` was already `06:00–08:00` and `12/30` in r1 and stays so. My measurement bug, not an app defect. |
| m7 | Menu width/copy | **PASS** — 272 px wide; “Liên kết với kênh…” verbatim; unfollow hint now covers both halves (item 85,6 px). Row padding/gap remains 6 px/0 vs art 8/2 → R3. |
| m8 | Why panel | **PASS** — 3 facts, `#B4B4BE`/`#8B8B97` verified on screen; missing “Đổi ngưỡng…” link is an accepted deviation (target screen does not exist). |
| m9 | Back button / queue hint | **PASS** — back renders 19,4 px, preview block starts y=104,4 = art; shorter queue hint accepted (queue timing data does not exist yet). |
| m10 | Orb 72 | **PASS** — 40×40. |
| m11 | Empty examples | **PASS** — `@nocturnestories / youtube.com/@thelatearchive / UCx9…Qw` verbatim. |
| m12 | Breadcrumb | **PASS** — header renders “Khám phá / Tất cả đối thủ”, “Khám phá / Nocturne Stories” after selection, and plain “Khám phá” when empty. |
| m13 | Bulk primary | **PASS** — trailing chevron present; locked label “Làm 3 video” accepted (no owner channel). |

## Regressions found in round 2

**R1 — MAJOR: the “Đăng trong” chip's invisible `<select>` covers the entire viewport and swallows every click.**
`FilterBar.tsx` puts `position:absolute; inset:0` on the select; round 2 removed `position: relative` from the chip wrapper (`.vx-chip--select` has none; `.vx-chip` has none), so the containing block becomes the viewport: measured rect **[0,0,1440,900]**, `elementFromPoint` at the centre of every rival row, card and button returns the `select`. Any real click opens the native “Tất cả / 7 ngày / 30 ngày / 90 ngày” dropdown — the page is effectively not clickable. Round-1 rect was 171,2×26 over the chip. Fix: `position: relative` on `.vx-chip--select` (or the `.vx-chip` shared rule). This also sits one line below the new `.vx-chip:focus-within` rule the same fix added, so it was likely dropped while editing that block. All other round-2 captures bypassed the overlay only with `pointer-events:none` on the select (a JS interaction override; screenshots are unaffected).

**R2 — MINOR: “Đăng trong 30 ngày” chip 8 px too wide.** 191,2 px vs artboard 183,2. Cause: `margin-left: 8px` still on the calendar `Icon` inside the padded chip (the padding/gap fix made the margin pure addition). Shifts the “Trạng thái” chip x by +8; the row still ends in time for the wrap. `FilterBar.tsx` — drop the icon's `marginLeft`.

**R3 — MINOR (also Fix-Explore deviation #1, counted per the instruction): menu item rows 6 px padding / 0 gap vs artboard 8/2.** Measured again: items 67,8 / 50 / 67,8 / 85,6 px vs 70,5 / 53,8 / 70,5 / 88,2. The fix argues ActionMenu metrics are shared and match the Kit artboard — reasonable as a temporary deviation, but it is a visual-density choice, not missing data, so it stays on the list until the owner decides on `rowPadding`/`rowGap`.

**R4 — MINOR: empty-state content block sits 6 px high.** Box y=339 vs artboard 345,1; orb/h1/body/form all inherit −6 while the sidebar and tags are pixel-exact. Vertically centred content, cosmetic only.

## Deliberate deviations judged

Accepted (data/screen truly absent): menu row padding **not** accepted (see R3 — it is a component metric, not absent data); “Đổi ngưỡng ở Kênh › Nguồn & tự động” link (target screen unbuilt) accepted; queue ETA hint (no queue data) accepted; “Làm 3 video” without channel name (no owner channel) accepted; shorter unfollow hint (no queue count) accepted; English example handles copied verbatim from the artboard accepted; empty sidebar without the add box matches the artboard itself (deviation #7) — no finding.

## State pairs (all re-captured)

`art-default`↔`app-default`, `art-rival`↔`app-rival`, `art-empty`↔`app-empty`, `art-preview`↔`app-preview`, `art-previewwhy`↔`app-previewwhy`, `art-select`↔`app-select`, `art-menu`↔`app-menu`, `art-scanpaused`↔`app-scanpaused`, `art-scanlimited`↔`app-scanlimited`, `art-scandone`↔`app-scandone`, `art-vnslot`↔`app-posting-vn-slot`, `art-nyslot`↔`app-posting-ny-slot`, plus `app-add-error`, `app-unfollow-confirm`, `app-focus-select`, `app-scanrunning`, `app-grid20k` without artboard counterparts.

20.000-video rival again: **24 card nodes** max (18 at top), 594–660 DOM nodes total, 3 columns, 0 horizontal overflow.

**Round 2: 1 major / 3 minors** — every round-1 defect verified fixed (m6 retracted as my own diff bug); the residual is the viewport-covering select (R1), the 8 px chip width (R2), menu row metrics (R3) and a 6 px empty offset (R4).

Status: DONE_WITH_CONCERNS — all 4 majors and 12 of 13 minors closed; one new major regression (R1) must land before this build is used interactively.

---

# Round 3 — regression pass after the hover-token fix (30/10/2026)

Rebuilt `go build` binary (embedded dist current), same DB/seed/artboard harness as rounds 1–2 (fresh `viderse_smoke_qaexplore`, `/tmp` design copy with local Be Vietnam Pro, 1440×900 Chrome, loopback-only, `setup-checked` init script). Two phase-2 checks were added: the `Settings → Kết nối AI` provider tiles at **1440×900** and **390×844**. Evidence: `/tmp/viderse-qa-explore/shots/r3/`.

## Round-2 residual, re-checked

| # | R2 finding | Result |
|---|---|---|
| R1 | Invisible `<select>` covering the viewport | **PASS** — select rect back to **181 × 26** inside the chip (was 1440 × 900); `.vx-chip` now carries `position: relative`. `elementFromPoint` at the centre returns the intended node for the rival row, card thumb, “Chọn nhiều” and the chip itself (`reaches: true` ×4). Real clicks verified end-to-end: rival row → `#/explore?rival=@nocturnestories` and rail swaps; card thumb → preview panel with its `h2`; “Chọn nhiều” → 3 checkbox states; chip → focus lands on the native `select`. |
| R2 | “Đăng trong” 8 px too wide | **PASS** — chip row is now pixel-identical: “Dài” 134,8 @ x520,8, “Đăng trong” **183,2** @ x663,6, “Trạng thái” 167,2 @ x854,8 (artboard 134,8 / 183,2 / 167,2 @ 521 / 663,8 / 855); sort group 238,8 vs 239; counter still wraps to y=161 and the grid still starts at **194,4** = artboard. |
| R3 | Menu row metrics | **Not re-raised, as instructed.** Numbers for the record: menu 272 wide ✓; items padding `6px 10px` gap `0` vs artboard `8px 10px` gap `2px` → 67,8 / 50 / 67,8 / 85,6 vs 70,5 / 53,8 / 70,5 / 88,2 (menu 284,1 vs 302,9). |
| R4 | Empty block 6 px high | **PASS** — empty box at **[546, 345, 600 × 241]** = artboard 546 / 345,1 / 600; sidebar 252, exactly one form, tags on one line. |

## Hover states — every hover changes paint, no dead hover

Measured with a real pointer move, reading computed style before/after, artboard vs app:

| Element | Artboard hover | App hover | Match |
|---|---|---|---|
| Rival row | `rgba(255,255,255,.04)` | `rgba(255,255,255,.04)` | ✓ |
| Filter chip | bg `.07`, text unchanged | bg `.07`, text unchanged | ✓ (rest border `.09` vs `.10` — imperceptible) |
| Rail ⋯ icon button | bg `.07`, text stays `#A0A0AB` | bg `.07`, text → `#EDEDEF` | bg ✓, text brightens (m2) |
| Preview back button | bg `.07`, text stays `#A0A0AB` | bg `.07`, text → `#EDEDEF` | bg ✓, text brightens (m2) |
| Posting slot row | `rgba(255,255,255,.07)` (`.ghost`) | `rgba(255,255,255,.04)` (`--vv-hover-row`) | **mismatch (m1)** |
| “Mở trên YouTube” link | bg `.07` | bg `.07` | ✓ |
| Bulk quick-pick buttons | bg `.07` | bg `.07` | ✓ |
| Card thumb | — | `scale(1.03)` on `.vx-card:hover` | present, unchanged |

## Remaining minors (3)

- **m1 — Posting slot row hovers one step too dark.** `.vx-slot:hover { background: var(--vv-hover-row) }` = `.04`; the artboard slot is a `.ghost` control and hovers at `.07` (same as the chips/icon buttons it sits next to). One-token change in `explore.css`.
- **m2 — Icon button and back button brighten their label on hover.** `.vx-iconbtn:hover` / `.vx-back:hover` set `color: var(--color-fg)` (`#EDEDEF`); the artboard's `.ghost:hover` changes only the background, leaving `#A0A0AB`. Observed on both elements; the background half of both rules is correct.
- **m3 — Channel avatar initials are slightly larger and lighter than the artboard.** Sidebar 30 px avatar: app `10.8px / 600 / lh 10.8` vs artboard `10.5px / 700 / lh 16.275`; rail 40 px avatar: app `14.4px / 600` vs artboard `13px / 700`. `ChannelAvatar` (shared component). Present since round 1 — my earlier text-diff did not surface 2-character nodes; now recorded. Also noted, inside tolerance and not counted: rival-row name text sits 2,0 px higher in the app (artboard name span is inline with a 16 px line box, the app uses the 20,15 px line box), which pulls the sub-line up by the same 2 px.

## Phase-2 check — `Settings → Kết nối AI` provider tiles

Desktop 1440×900 (`art-settings-connect-add` ↔ `app-settings-connect-add`): 8 tiles, grid columns 175,5 px, heights **77,3 × 4** (two-line subtitle) and **64 × 4** (one-line) in both; `min-height 64`, `padding 10`, `border-radius 11`, `border 1px`; name `13px/600/lh 20,15`, subtitle `11,5px/400/lh 16,1` — identical.

Phone 390×844 (`art-msettings-connect-add` ↔ `app-msettings-connect-add`, sheet open): 8 tiles, width **175**, heights **58,3** (art) vs **58,1** (app) → within 0,2 px; `min-height 58`, `padding 8px 12px`, `border-radius 12`; name `14px/600/lh 21`, subtitle `11,5px/400/lh 16,1` — identical. The `--vv-s-58` token and the mobile tile padding are correct; no regression from the hover/padding edits.

## Other states re-checked (unchanged or better)

Default/rival/preview/preview+why/select/rivalMenu/scan running-paused-limited-done/posting VN+slot/posting NY+slot/add-error/unfollow-confirm/focus: no visual change from round 2 apart from the fixes above. Focus order still sort radio → “Dài” chip (2 px ring) → zone select (2 px ring) → card; locked “Trạng thái” chip still out of the tab order. 20.000-video rival: 18 card nodes at rest, **24** while scrolling, 660 DOM nodes, 3 columns, `scrollWidth == 1440`.

**Round 3: 0 majors / 3 minors.**

Status: DONE — R1/R2/R4 verified fixed, the hover token now paints on every hoverable element, and the connect tiles match at 1440 and 390; the remaining three minors are one hover-alpha token, one hover-label colour, and the shared avatar initials.
