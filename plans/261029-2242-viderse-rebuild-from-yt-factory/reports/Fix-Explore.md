# Fix — Explore (hậu QA)

Nguồn: `reports/QA-Explore.md` (4 major, 13 minor). Ảnh đối chiếu: `/tmp/viderse-fx/*.png`
(màn thật, 1440×900, dpr 1, build `ExploreScreen-BFK2rmRu.js` + kit 0.2.x mới) so với
artboard `Explore.dc.html` đo trực tiếp bằng DevTools protocol ở cùng viewport.

## Major

### M1 — Nhãn nhóm Sắp xếp bị cắt ("Mới nh…", "Lượt x…")

**Nguyên nhân:** `Segmented` (kit) ở chế độ một dòng luôn `flex: 1` cho mỗi mục nên
chia đều track (168,8 px cho ba mục 51,6 px) rồi cắt nhãn bằng `text-overflow: ellipsis`.
Artboard để mục rộng theo nhãn: 66,6 / 74,7 / 78,1 px, cả nhóm 231,4 px.

**Sửa:** thêm prop **opt-in** `fit?: 'equal' | 'content'` cho `Segmented`
(`platform/ui/src/Tabs.tsx`). `'content'` ⇒ mục `flex: 0 0 auto`, `padding-inline: 12`
(đè được bằng `options[].style.paddingInline`), nhóm vẫn một dòng. Mặc định `'equal'`
giữ nguyên hành vi cũ. Dùng ở `FilterBar` (nhóm Sắp xếp) và `PostingCard` (nhóm
VN / New York, `paddingInline 8`, `font-size 11.5` như artboard). Nhóm lọc còn nhận
nền `surface-1`, viền `line-1`, `gap 2`, thumb `surface-6` đúng token artboard.

**Bằng chứng:** ảnh `app-default.png` — "Nổi trội · Mới nhất · Lượt xem" đủ chữ; đo
màn thật: nhóm x 274, w **238,8**, ba mục **69,2 / 77 / 80,6** px — QA đo artboard
69,2 / 77 / 80,8 (nhóm 238). Kit mặc định không đổi: Noverse `tsc` sạch,
**187 file / 1514 test pass**.

### M2 — H1 trạng thái trống lặp tên màn

**Sửa:** `tt.emptyTitle` = "Chưa theo dõi đối thủ nào" (06-copy, nguyên văn);
trước đây dùng `tt.title`.
**Bằng chứng:** `app-empty.png` — H1 đúng câu; test
`ExploreScreen.test.tsx` "trạng thái trống: tiêu đề riêng …" so cả hai chuỗi và
`not.toBe(tt.title)`.

### M3 — Khối trống căn giữa cả trang (720) thay vì giữa cột giữa (846)

**Sửa:** trạng thái trống vẫn dựng `vx-shell` đủ: cột đối thủ 252 px
(`RivalSidebar list={list.data} showAdd={false}` — nhóm "Đối thủ 0" + câu hướng dẫn,
không lặp ô thêm) + `<main class="vx-empty">` là `flex: 1` (thêm `min-width: 0`).
**Bằng chứng:** đo `app-empty.png`: hộp 600×240,9 tại x 546 ⇒ **tâm x = 846**
(= artboard); cột trái `x 0 w 252`; chỉ **1** `<form>` trong cây (test khẳng định).

### M4 — Chip lọc không có vòng focus; chip "Trạng thái" khoá vẫn là điểm dừng Tab

**Nguyên nhân:** chip bọc `<select>` trong suốt (`opacity: 0`) — viền focus của
`<select>` vẽ trên chính nó nên vô hình; chip "Trạng thái" (`aria-disabled`) vẫn chứa
`<select>` nên Tab dừng vào một control chết.

**Sửa:**
- `.vx-chip:focus-within { outline: 2px solid var(--color-accent); outline-offset: 2px }`
  — vòng vẽ trên chip, đúng như gợi ý của QA (token, không hex).
- Chip "Trạng thái" **bỏ hẳn `<select>`**, đổi thành `<span class="vx-chip--locked"
  aria-disabled="true" aria-describedby="vx-status-locked">`: không vào thứ tự Tab,
  lý do khoá vừa ở `title` vừa ở phần mô tả ẩn (đọc được bằng trình đọc màn hình).
  CSS `.vx-chip--locked { cursor: default }` + triệt hover của span.
**Bằng chứng:** test "chip lọc: <select> nằm trong chip …" (select là con trực tiếp của
`.vx-chip--select`, đúng điều kiện để `:focus-within` chạy) và test khoá
(`aria-disabled`, không có `select`, `aria-describedby` trỏ `#vx-status-locked`,
không `tabindex`). Chip dài/ngắn vẫn hover + bấm được.

## Minor

| # | Tìm thấy | Sửa |
|---|---|---|
| m1 | Chip "Dài ≥ 8 phút" thiếu mũi tên (134,8 px ở artboard) | thêm `<Icon name="arrow-d" size={12} aria-hidden>` sau phần đậm — cả ba chip một hàng đều có chevron như artboard |
| m2 | Counter nằm cùng hàng vì hàng lọc hẹp hơn 71 px | hệ quả M1+m1+m3: chip cuối kết thúc ở x **1030** (artboard 1022), counter rộng 98,9 px không còn chỗ ⇒ xuống hàng 2 (`app-default.png`; đo: counter x 274, y 161) |
| m3 | "Chọn nhiều" mất khung nút, chữ `#A0A0AB` | `variant="outline"` (kit ghost) + `size="compact"` (32) — đúng artboard `height:32; border:1px; color:#EDEDEF`; trạng thái bật dùng token mới `--vv-line-30` + `--vv-wash-8`; icon luôn `tick` 15 px, màu `fg-3` → `fg` khi bật (artboard `selIc`) |
| m4 | "ĐỐI THỦ" thấp hơn 6 px | `.vx-grp` padding-top `s-6` (artboard ghi đè inline 6 trên 12); trạng thái trống `[data-empty] .vx-grp` = `s-4`. Đo: nhãn ở **y 109** ở cả app và artboard |
| m5 | Bản quét xong dùng "định kỳ" | copy theo dữ liệu: `scanStates.capped/complete(every)` + `everyHours(rescanHours)` ⇒ chu kỳ 1 giờ in đúng câu artboard "… kiểm tra mỗi giờ."; chu kỳ khác in "mỗi 6 giờ" (đúng dữ liệu thay vì câu cứng) |
| m6 | Slot "06:00 – 08:00" / "12 / 30" có dấu cách | **không còn trong bản dựng QA đo**: mã hiện tại in `{start}–{end}` và `{count}/{sample}`, bundle `ExploreScreen-*.js` chứa `[p.start,"–",p.end]`, `[p.count,"/",m]`. Không đổi mã; ghi nhận là ảnh QA chụp bản dựng cũ |
| m7 | Menu đối thủ 264 px, hàng 67,8/50/67,8, hint ngắn, item 3 sai chữ | `ActionMenu` thêm prop opt-in `width` (mặc định 264) — RivalCard truyền **272**; `menuLink` → "Liên kết với kênh…" (nguyên văn); `menuUnfollowHint` → "Gỡ kênh này khỏi Khám phá; video đã quét bị xoá cùng, video đã làm giữ nguyên." (dài hơn, đủ hai vế artboard). Đệm hàng/`gap` xem Deviations |
| m8 | Panel Vì sao: 4 fact, thiếu link sửa, màu nhạt sai | còn **3 fact** (bỏ "Ngưỡng nổi trội ×3" — ngưỡng đã nằm trong câu lý do; IA cho 2–4 dòng); thân panel `--vv-fg-soft` `#B4B4BE`, nguồn `--vv-fg-note` `#8B8B97`. Đo thật: `rgb(180,180,190)` / `rgb(139,139,151)` = artboard. Link "Đổi ngưỡng…" xem Deviations |
| m9 | Nút "← Thông tin đối thủ" cao 30 px, đẩy khối xem trước xuống 10,6 px | bỏ `height: 30` khỏi `.vx-back` ⇒ chữ trần **19,4 px** (artboard cũng render 19,4). Đo: back `y 71 h 19.4`, khối `.vx-pv` `y 104.4` — trùng artboard từng 0,1 px. Hint hàng chờ xem Deviations |
| m10 | Orb 72 px | `Orb size={40}` — đo 40×40. |
| m11 | Ba chip ví dụ sai nội dung | `emptyExamples` = `@nocturnestories`, `youtube.com/@thelatearchive`, `UCx9…Qw` (nguyên văn), render từ mảng (test duyệt cả ba) |
| m12 | Thiếu breadcrumb "Khám phá / <đối thủ>" | `shell/ExploreCrumb.tsx`: đọc cùng khoá cache `rivals` (không thêm request), hiện "Khám phá / Tất cả đối thủ" hoặc tên đối thủ theo `?rival=`; chưa có đối thủ thì chỉ "Khám phá". Ảnh: header trái `Khám phá / Tất cả đối thủ`; trạng thái trống chỉ `Khám phá` |
| m13 | Nút chính bulk thiếu mũi tên | thêm `<Icon name="arrow-d" size={13}>` sau nhãn (nhãn "cho <kênh>" xem Deviations) |

## Deviations (hỏi-đáp)

1. **Hàng menu đối thủ vẫn đệm 6 px, cách nhau 0 px** (artboard 8 px / 2 px ⇒ mỗi
   hàng thấp hơn ~2,7 px). Đệm hàng là metric chung của `ActionMenu` và khớp artboard
   Kit (`.vv-menu__item` 6/10, `--vv-r-opt` 9); đổi sẽ đổi menu của Noverse. Đã mở
   riêng `width` (272) nên chênh còn lại chỉ là đệm dọc. *Cần quyết: thêm prop
   `rowPadding`/`rowGap` cho kit, hay coi artboard Explore là ngoại lệ?*
2. **Panel Vì sao không có link "Đổi ngưỡng ở Kênh › Nguồn & tự động"**: chưa có màn
   Kênh › Nguồn & tự động (gói sau). `Reason.fix` là trường tuỳ chọn trong
   `07-data-contract`; thà thiếu link còn hơn dẫn tới màn không tồn tại. *Khi màn Kênh
   có mục ngưỡng thì thêm link + copy nguyên văn.*
3. **Hint hàng chờ** giữ "Lên đầu hàng chờ khi bấm Làm video." Artboard cần ETA
   ("bắt đầu khoảng 00:40") và số video đang chạy — dữ liệu hàng chờ đợi gói 05.
4. **Nhãn nút bulk** "Làm 3 video" (artboard "… cho Midnight Tales") — chưa có kênh
   của bạn; hợp đồng khoá nút ở dạng "cho <kênh>". Đã thêm mũi tên để đúng hình thức.
5. **Hint "Bỏ theo dõi"** ngắn hơn artboard vì không có tên kênh của bạn và số video
   trong hàng chờ ("2 video trong hàng chờ bị bỏ").
6. **Ví dụ trạng thái trống là kênh tiếng Anh** (@nocturnestories, thelatearchive) —
   chép nguyên văn artboard; app tiếng Việt nên đây là chỗ nên hỏi lại thiết kế.
7. **Cột trái ở trạng thái trống không có ô thêm** (artboard ẩn form, giữ nhóm + câu
   hướng dẫn) — giữ đúng artboard, ô thêm chỉ còn một chỗ ở giữa.
8. **m6 (dấu cách trong slot)** không tái hiện được trên bản dựng hiện tại — mã và
   bundle đều in liền; coi như đã đúng, chờ ảnh QA vòng sau xác nhận.

## Kiểm tra

- `apps/viderse/web`: `tsc --noEmit` sạch; `vitest run --maxWorkers=2` **19 file / 215
  test pass** (gồm `src/checks/design-rules.test.ts`); `npm run size` **129,0 kB gzip**
  (ngưỡng 250).
- `platform/ui`: `npm run build` + `vitest run` pass (thêm test `Segmented fit="content"`);
  README ghi `fit`, `width` của `ActionMenu`.
- `apps/noverse/web`: `tsc --noEmit` sạch; **187 file / 1514 test pass** — render
  Noverse không đổi (mọi prop mới đều opt-in).
- Go: `go build ./apps/viderse/cmd/viderse` OK (không sửa tệp Go nào).

**Status:** Done — 4/4 major và 10/13 minor sửa bằng mã; m6 không tái hiện (đã đúng
trong bản dựng hiện tại); m7 còn chênh đệm dọc hàng menu + m9/m13/m8-link nằm ở
Deviations vì thiếu dữ liệu/màn đích. Đo lại bằng màn thật 1440×900: nhóm Sắp xếp
đủ nhãn, counter xuống hàng 2, khối trống tâm x=846 với cột đối thủ 252 px, back
19,4 px và khối xem trước y=104,4, panel Vì sao 3 fact đúng màu token, orb 40 px.
