# 04 — Audit nội dung: MChannel · MCompVideo · MCreate · MExplore · MSettings (+ Thư viện & Template)

**Người audit:** MAuditB · **Ngày:** 2026-11-01
**Đối tượng:** `apps/viderse/web/src` (production `https://vid.aarverse.studio`) ↔ artboard điện thoại `apps/viderse/docs/design/v1/artboards/M*.dc.html`
**Tham chiếu phụ:** `apps/noverse/web/src/mobile/*` (production `https://nov.aarverse.studio`), `platform/ui/src`

## 0. Phương pháp (đọc trước khi dùng số liệu)

| Việc | Cách làm |
|---|---|
| Artboard | Render qua `python3 -m http.server 8931` tại `apps/viderse/docs/design/v1` → `http://127.0.0.1:8931/artboards/<File>.dc.html`. **Không dùng `file://`**: `file://` chặn `mask-image` nên toàn bộ icon (`span.ic.*`, logo) biến mất → so sánh icon sẽ sai. Đổi trạng thái bằng `window.__dcSetProps('<Board>', {...})` (hàm do `support.js` gắn vào `window`), viewport 390×844. |
| App | `agent-browser --session MAuditB`, viewport 390×844 (thêm 360×780 khi layout có nguy cơ vỡ), đăng nhập `qaaudit` (mật khẩu đọc từ file, không ghi vào báo cáo). |
| Safe area | Emulate notch thật bằng CDP: `Emulation.setSafeAreaInsetsOverride {top:59,bottom:34}` (script `/tmp/vmob-cdp.mjs`), sau đó kiểm tra lại `env(safe-area-inset-*)` = `59px/34px` trong trang. Override **mất sau mỗi lần điều hướng** nên phải set lại mỗi lần (`/tmp/vmob-insets.sh`). |
| Đo | Dump cây DOM kèm `getBoundingClientRect` + `getComputedStyle` (font-size, weight, màu, radius, background, padding) ở cả artboard và app, cùng selector/thứ tự. |
| Ràng buộc | Production **chỉ đọc**: không tạo/sửa/xoá/enqueue/duyệt/render, không bấm nút submit, không mở luồng đăng nhập nền tảng (Bilibili QR). Trạng thái nào chỉ thấy được khi mutate → ghi **"không quan sát được"**. |

**Dữ liệu tài khoản `qaaudit` (quyết định phạm vi quan sát được):** 0 kênh (`/api/owner-channels` → rỗng), 0 template ở `/api/templates` lúc đầu (sau đó 1 template `Mẫu chuẩn · a — Bản sao`), 2 đối thủ (34 video), 0 hàng chờ, 0 video đã làm.
⇒ **MChannel (F1) không có dữ liệu kênh** → chỉ quan sát được trạng thái "không tìm thấy kênh"; các tab có dữ liệu phải đối chiếu bằng mã nguồn + artboard (đánh dấu `[CODE]`).
⇒ **MCreate bước `done`** chỉ quan sát được fallback; **sheet QR Bilibili** và **Template** phải đánh dấu "không quan sát được".

**Quy ước cột "Lệch":** `AB` = artboard, `APP` = app production, số đo px thật ở 390×844.
**Mức độ:** **P0** = vỡ/không dùng được · **P1** = lệch rõ so với artboard hoặc chậm · **P2** = bóng bẩy/chi tiết.
`[INFERENCE]` = suy luận, không đo trực tiếp.

---

## 1. Tóm tắt điều hành

| # | Phát hiện | Mức | Ảnh |
|---|---|---|---|
| 1 | **Toàn bộ Cài đặt (G3) render dưới notch**: `.vv-mset` là `position: fixed; top: 0` nên `header.vv-mset__head` nằm ở `y=0..52` trong khi inset trên = 59px. Tiêu đề + nút back chui vào thanh trạng thái. | **P0** | `vmob-MAuditB-app-settings-connect.png`, `…-360-conn.png` |
| 2 | **Sheet của kit mất safe area dưới**: `.m-sheet` có `padding-bottom: max(26px, var(--m-safe-bottom))` nhưng Viderse **không định nghĩa** `--m-safe-bottom`/`--m-top` → `padding-bottom` = **0px**; nút cuối của sheet kết thúc ở `y=832` (vùng home indicator bắt đầu 810). | **P0** | `vmob-MAuditB-app-sheet-jobs.png` |
| 3 | **Explore trên điện thoại là layout 3 cột desktop xếp dọc**: `aside.vx-side` (204px) + `main.vx-center.vv-wide` + `aside.vx-rail` (bắt đầu `y=922`, **ngoài màn hình**), trang cao 1867px, `.vx-grid` cao 11042px, cuộn lồng nhau. | **P0** | `vmob-MAuditB-app-explore.png` |
| 4 | **Chi tiết video (B2) không tồn tại trên điện thoại**: bấm thumbnail chỉ làm `aside.vx-rail` ở `y=922` đổi nội dung — không cuộn, không phản hồi thị giác. Artboard B2 là **màn riêng** có header riêng, hero, danh sách chương, CTA dính đáy. | **P0** | `vmob-MAuditB-app-compvideo.png`, `…-rail.png` |
| 5 | **Bulk bar (chọn nhiều) nằm ngoài màn hình**: `.vx-bulk` ở `y=820..908` (màn 844) và nằm dưới tabbar → chỉ thấy 24px đầu. Artboard thay tabbar bằng thanh chọn. | **P0** | `vmob-MAuditB-app-explore-select-bulk.png` |
| 6 | **Không có tiêu đề màn "Khám phá"** và không có nút "Chọn"/"Thông tin đối thủ" 44px; thay bằng h1 20px "Video nổi trội" + ô tìm kiếm 32px + "Chọn nhiều" 32px (dưới 44px tap target). | **P1** | như trên |
| 7 | **MCreate (A2) mang 3 tầng chrome desktop** (mobile bar + onboard head "Viderse · Lần đầu mở/Cài đặt nâng cao" + breadcrumb + h1 22px) và **giữ tabbar** dù artboard là trang con; footer dính đáy **không có nền** nên chữ/CTA trôi trên nội dung cuộn. | **P1** | `vmob-MAuditB-app-mcreate.png`, `…-360-new.png` |
| 8 | **Thư viện (G1) và Cài đặt template (G2) không có artboard điện thoại** → GAP; Thư viện **không có lối vào** trên điện thoại (tabbar 5 mục không có Thư viện, AccountMenu không có). | **P1** | `vmob-MAuditB-app-library.png`, `…-app-template.png` |
| 9 | **Danh sách chương trong B2 không được cài** (client chỉ có `chapterCount`, không có `chapters`) → thiếu hẳn một khối nội dung của artboard. | **P1** | `vmob-MAuditB-arte-MCompVideo-none.png` |
| 10 | **Card YouTube Data API vỡ ở 390/360** trong Kết nối AI (tiêu đề 3 dòng, pill xuống dòng, nút "Thêm khoá" chen vào tiêu đề). | **P1** | `vmob-MAuditB-app-360-conn.png` |

**Điểm khớp tốt (không cần sửa):** Cài đặt › Trợ lý AI (`agents`), Tài khoản nền tảng (`accounts`), Lưu trữ ở biến thể kho S3 đang hoạt động (`store=s3active`) khớp artboard gần như 1:1 về cấu trúc và kích thước; tabbar dưới (`362×64`, item `70×52`, pill `r27`, nhãn `10.5px`) khớp chính xác artboard.

---

## 2. Safe area (mục riêng — so Viderse ↔ artboard ↔ Noverse)

### 2.1 Số đo (viewport 390×844, inset emulated: top 59 / bottom 34)

| Mốc | Artboard M* | Viderse app | Noverse app (chuẩn) |
|---|---|---|---|
| Dải trên | `.phone` con đầu = khung status minh hoạ **44px** (MExplore: appbar ở `y=44`), sau đó `.vv-appbar` 52px | `.vv-mobile-shell { padding-top: env(safe-area-inset-top) }` → appbar ở `y=59`, cao 52 (**đúng**) | `.m-app { padding-top: var(--m-top) }`, `--m-top: max(8px, calc(--m-safe-top + --m-edge))` → `59px`; `.m-topbar` ở `y=59` |
| Dải dưới | dải home minh hoạ **26px** (`y=818..844`); tabbar đáy ở `y=814` | `.vv-tabbar { bottom: calc(4px + env(safe-area-inset-bottom)) }` → đáy **806** (cách mép 38) | `.m-tabs { bottom: var(--m-tabbar-bottom) }` với `--m-tabbar-bottom: max(20px, --m-safe-bottom)` → đáy **810** (cách mép 34) |
| Trang con / overlay | header 52px nằm **dưới** khung status (`y=44`) | `.vv-mset { position: fixed; top: 0 }` → header `y=0..52` ⇒ **dưới notch** (P0) | `.m-page` nằm trong `.m-app` đã có `padding-top`; mọi mốc trên dùng `--m-top` |
| Sheet | sheet chiếm hết đáy khung, có dải home 26px riêng (`.m-sheet-home`, `kit.css:152`) | `.m-sheet { padding-bottom: max(26px, var(--m-safe-bottom)) }` → **0px** vì `--m-safe-bottom` không tồn tại; nút cuối `y=832` | `.m-sheet` cùng công thức nhưng Noverse định nghĩa `--m-safe-bottom` → **34px** |
| Toast | — | `Toast.tsx:226` `calc(80px + env(safe-area-inset-bottom,0px))` (mobile) — có fallback, **đúng** | `.m-toast { bottom: var(--m-toast-bottom, calc(var(--kb,0px) + 92px)) }` (nâng theo bàn phím) |
| Bàn phím | — | **không có** `visualViewport`/`--kb` trong toàn repo Viderse | `mobile/runtime.tsx:80` đo `visualViewport` → đặt `--kb`; sticky CTA/toast né bàn phím |
| PWA standalone | — | `index.html:5` có `viewport-fit=cover`, `apple-mobile-web-app-status-bar-style=black-translucent`, **không** có `data-standalone` | `index.html` có script `navigator.standalone → html[data-standalone]`, `--m-edge: 40px` (mobile.css:7-12) |

### 2.2 Quy tắc rút ra cho chuẩn dùng chung (đề xuất)

1. **Một nguồn duy nhất cho safe area**, đặt ở `:root` của app: `--safe-top: env(safe-area-inset-top,0px)`, `--safe-bottom: env(safe-area-inset-bottom,0px)`, `--top: max(8px, calc(var(--safe-top) + var(--edge)))`, `--tabbar-bottom: max(20px, var(--safe-bottom))`. Cấm dùng thẳng `env(safe-area-inset-top)` trong component (Noverse đã có test canh: `mobile/safeArea.test.ts:44-56`).
2. **Mọi lớp phủ toàn màn (trang con, overlay, sheet) phải thừa hưởng `--top`** — không được `position: fixed; top: 0` (đúng lỗi `.vv-mset`). Nếu buộc phải `fixed`, dùng `padding-top: var(--top)` và `padding-bottom: max(26px, var(--safe-bottom))`.
3. **Tabbar dưới**: `bottom: var(--tabbar-bottom)` (Noverse = 34 ở iPhone notch). Viderse đang cộng thêm 4px → cao hơn chuẩn 4px; artboard vẽ 26px dải home (thấp hơn chuẩn 8px). Chốt **34px** (= `max(20px, --safe-bottom)`), artboard cần vẽ lại dải home thành 34px.
4. **Artboard không được coi khung status 44px là thật**: `noverse/docs/design/v1/18-mobile.md:496` ghi rõ "khoảng 40px đầu mọi board điện thoại là chỗ status bar minh hoạ; app dùng safe-area thật". Viderse M* vẽ 44px (không phải 40px) — nên thống nhất ghi chú này trong `apps/viderse/docs/design/v1/05-kit.md` và dùng 44px cho mọi board M (hoặc đổi sang 59px như iPhone 14/15).
5. **Nội dung cuộn phải chừa đáy** `calc(tabbar-h + tabbar-bottom + 12px)` = 110px (Viderse đang dùng 110px ✓); khi có thanh CTA dính đáy thì cộng thêm chiều cao thanh đó.
6. **Bàn phím**: mọi thanh dính đáy và sheet phải nâng theo `--kb` (Noverse `runtime.tsx:80`, `18-mobile.md:507`). Viderse chưa có → thêm vào `MobileShell`/`Sheet`.
7. **Test**: port `safeArea.test.ts` của Noverse sang Viderse (cấm `env(safe-area-inset-top` ngoài 1 chỗ; `--m-top` có `--edge`; tabbar dùng `--tabbar-bottom`).

---

## 3. MExplore — B1 · Khám phá

**Artboard:** `MExplore.dc.html` · tweak `mode` = `browse` | `select` | `info` | `add` (đủ 4 trạng thái đã chụp).
**App:** `#/explore` → `screens/explore/ExploreScreen.tsx` + `explore.stylex.ts` + `videos.stylex.ts`.

### 3.1 Trạng thái artboard (liệt kê đủ)
| `mode` | Nội dung vẽ |
|---|---|
| `browse` | header `.vv-appbar` (logo + jobs ring "2" + avatar "D") → h1 "Khám phá" 24/600 + nút info 44 + nút "Chọn" 93×44 → dải chip đối thủ `.hs` (chip "Tất cả" + chip đối thủ có avatar 30 + badge số video) → segment sắp xếp 252×46 + chip lọc 44 ("Dài ≥ 8 phút", "Chưa làm cho kênh nào") → dòng đếm 12.5 → danh sách card |
| `select` | nút header đổi thành "Thoát chọn"; mỗi card có checkbox; **tabbar bị thay** bằng thanh chọn: "Đã chọn 2" 14/600 + chip "Top 10" 40 + "Sao chép link" 40, nút chính 358×48 r13 15/600 + caption 12 |
| `info` | **sheet toàn màn** (r28 trên, tay nắm) chứa: header đối thủ (avatar 44, tên 18/600, ⋯), hàng "Liên kết với Midnight Tales … Đổi ở Kênh ›", 3 ô số 119×64, thẻ "Quét lịch sử" (tiến trình + Tạm dừng), thẻ "Lịch đăng" (segment vùng VN/New York), "30 video gần nhất · 4 khung lặp lại", các hàng khung giờ |
| `add` | sheet "Theo dõi đối thủ mới" (`.sheet-t` 18/600) + ô dán link/handle + nút theo dõi + tuỳ chọn liên kết luôn vào kênh |

### 3.2 Bảng lệch

| Thành phần | AB (đo) | APP (đo) | Lệch | Mức |
|---|---|---|---|---|
| Khung tổng | 1 cột, vùng cuộn `.vs` `[0,96 390×722]`, `padding: 4px 16px 118px` | `aside.vx-side` 390×204 + `main.vx-center.vv-wide` `[0,315 390×608]` + `aside.vx-rail` `[0,922 390×654]`; `documentElement.scrollHeight` = **1867**; `.vx-grid` cao **11042** | Layout desktop 3 cột xếp dọc; rail ngoài màn hình; 2 vùng cuộn lồng nhau | **P0** |
| Tiêu đề màn | h1 "Khám phá" `205×36` fs **24/600** ở `y=104` | h1 "Video nổi trội" fs **20/600**; không có tiêu đề "Khám phá" | Sai chữ + sai cỡ + sai thứ bậc | **P1** |
| Nút header | info `44×44 r12` + "Chọn" `93×44 r12` fs14/500 (icon 16) | không có; chỉ "Chọn nhiều" `118×32` fs13 | Mất 2 control 44px, thay bằng control 32px (dưới chuẩn tap 44) | **P1** |
| Ô tìm kiếm | **không có** trong `browse` (artboard B1 desktop có `Tìm tiêu đề`, mobile không) | `input` cao **32**, placeholder "Tìm theo tiêu đề video", kbd "/" | Thêm control không có trong AB + 32px | **P2** |
| Chip đối thủ | dải ngang `.hs` cao **44**, cuộn ngang, chip `r999`, avatar **30**, tên 13.5/400, badge `24×20 r999` nền warn 14% chữ 11.5/600, chip cuối "Theo dõi" | danh sách dọc, hàng cao **53**, 2 cột wrap, hiện `@handle`, không badge, có nhóm "ĐỐI THỦ 2" | Đổi mô hình điều hướng (dải ngang → danh sách dọc chiếm 204px) | **P1** |
| Sắp xếp | segment `252×46`, `padding 3`, nút `38 r9` fs13.5 | hàng tab `28` fs12.5 r7 (không có khung segment) | Mất khung segment; 28px | **P2** |
| Chip lọc | cao **44**, cuộn ngang 1 hàng | cao **28**, wrap thành **2 hàng** (108px: "Dài ≥ 8 phút" + "Trạng thái Tất cả") | 28 vs 44 + wrap làm đẩy nội dung | **P1** |
| Dòng đếm | 12.5 `#8B8B97` | có | ✓ | — |
| Card video | `358` rộng, thumb `358×201 r12`, score `42×22 r6` 12/600, dur `45×22` mono 11, title **14.5/500** clamp2, meta 12.5, dòng "Làm cho … · Chờ lượt" 24 + dot 6, nút copy `44×44` | thumb `362×204 r10`, score `48×20` 11.5, dur `42×20` 10.5, title **13.5/500**, meta 12, nút copy 32 | Bán kính/màu/cỡ chữ nhỏ hơn 1 bậc; tap target copy 32 | **P2** |
| Thanh chọn nhiều | thay tabbar, nằm trong khung `[0,680 390×138]`, nút chính 48px, có caption | `.vx-bulk` `[820..908]` cao 88, `position: sticky; bottom: 14px` trong khung cuộn sai → **ngoài màn hình**, chồng tabbar | **P0** |
| Sheet thông tin đối thủ | sheet toàn màn từ nút info | **không có**; nội dung nằm inline trong `aside.vx-rail` ở `y=922` (sau lưới video) | Màn/thao tác không tới được | **P0** |
| Form thêm đối thủ | sheet `.sheet-t` "Theo dõi đối thủ mới" | form inline luôn hiện đầu sidebar (`addbox` 34px, `addInput`) | Sai mô hình + input 34px | **P1** |
| Tabbar | `nav.tabbar` `362×64 r32` p5, item `70×52 r27`, nhãn 10.5 | `nav.vv-tabbar` `362×64 r32` p5, item `70×52 r27`, nhãn 10.5 | ✓ khớp | — |
| Trạng thái rỗng/không khớp | AB không vẽ trạng thái "chưa theo dõi đối thủ" cho mobile (B1 desktop có) | `nomatchTitle`/`nomatchSub` tồn tại (`explore.stylex.ts:48-49`) | AB thiếu trạng thái | **P2** (GAP artboard) |

**Ảnh cặp:** `vmob-MAuditB-arte-MExplore-browse.png` ↔ `vmob-MAuditB-app-explore.png`; `…-arte-MExplore-select.png` ↔ `…-app-explore-select-bulk.png`; `…-arte-MExplore-info.png` ↔ (không có tương đương); `…-arte-MExplore-add.png` ↔ (không có tương đương).

---

## 4. MCompVideo — B2 · Chi tiết video

**Artboard:** `MCompVideo.dc.html` · tweak `sheet` = `none` | `picker` | `why`.
**App:** không có route riêng; dùng `#/explore?video=<id>` → `screens/explore/PreviewPanel.tsx` render trong `aside.vx-rail`.

### 4.1 Trạng thái artboard
| `sheet` | Nội dung |
|---|---|
| `none` | Trang riêng: header 52 (back `.ib.ghost 44×44 r12`, avatar đối thủ 24 + tên 16/600, copy 44, ⋯ 44) → hero `358×201 r12` (score `42×22` 12/600 góc dưới trái, dur `45×22` mono 11 góc phải) → h1 **18/600** 2 dòng + meta 12.5 → hàng điểm (`.vv-score 41×20` 11.5/600 + text 13 + `.vv-why 64×36` 12) → card "Đang làm cho …" `358×66 r14 p12` (orb 25, 14/500, phụ 12.5 tint info, mũi tên 16) → `details.card` "Chương 5" `358×276`, summary 48, **5 hàng 44** (giờ mono 12 + tên 13.5) → thanh đáy dính `[0,731 390×87]` nền `#0D0D0F` viền trên: `.btn-red 358×48` 15/600 + caption 12 "Lên đầu hàng chờ · chọn kênh ở bước sau" |
| `picker` | sheet chọn kênh trước khi Làm video |
| `why` | sheet "Vì sao nổi trội?" |

### 4.2 Bảng lệch

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Vỏ màn | trang riêng, không tabbar, có header riêng + thanh CTA dính đáy | `aside.vx-rail` `[0,922 390×654]` — **dưới màn hình**, không có header riêng, CTA nằm trong dòng chảy | Không có màn B2 trên điện thoại | **P0** |
| Điều hướng tới | back 44×44 | `button.vx-back` `139×30` fs12.5 "Thông tin đối thủ" | Sai control, sai chữ, 30px | **P1** |
| Hero | 358×201 r12 | 357×201, `img 355×199` | ✓ | — |
| Badge điểm | `42×22 r6` 12/600, chữ nền tối trên warn | `48×20` 11.5 | Nhỏ hơn | **P2** |
| Thời lượng | `45×22` mono 11 | `42×20` 10.5 | Nhỏ hơn | **P2** |
| Tiêu đề | **18/600**, 2 dòng | **15/600**, 2 dòng (42 cao) | Nhỏ hơn 3px | **P1** |
| Meta | 12.5 | 12 | −0.5 | **P2** |
| Hàng điểm + Vì sao? | score 41×20 11.5 + text 13 + nút **64×36** | score 48×20 11.5 + text 12 + nút **64×22** | Nút 22px (dưới 44) | **P1** |
| "Đang làm cho kênh" | card `358×66 r14 p12`, orb 25, 2 dòng, mũi tên | `vx-makegroup 357×60`: head 12.5 "Làm cho kênh" + 1 hàng chọn kênh 36 | Khác cấu trúc; không có orb/trạng thái tiến trình | **P1** |
| **Danh sách chương** | `details.card` 5 hàng 44, giờ mono 12 | **không có** (client chỉ có `chapterCount` — `WorkbenchPanels.tsx:110`; không có `chapters` trong `apps/viderse/web/src`) | Thiếu hẳn khối nội dung | **P1** |
| CTA | `.btn-red 358×48` 15/600, **dính đáy** + caption | `vx-preview-make 357×46` 13.5, trong dòng chảy | Không dính, nhỏ hơn, thiếu caption | **P1** |
| Thêm ngoài AB | — | `p.vx-locknote` 36 (khoá khi chưa có kênh) + `a.vx-linkbtn 357×34` "Mở trên YouTube" | Control 34px | **P2** |

**Ảnh cặp:** `vmob-MAuditB-arte-MCompVideo-none.png` ↔ `vmob-MAuditB-app-compvideo.png` (không thấy gì) / `…-app-compvideo-rail.png` (cuộn xuống = trắng); `…-arte-MCompVideo-picker.png`, `…-arte-MCompVideo-why.png` ↔ không có tương đương.

---

## 5. MCreate — A2 · Tạo kênh

**Artboard:** `MCreate.dc.html` · tweak `step` = `create`|`done`, `rivals` = `some`|`none`, `identity` = `running`|`failed`.
**App:** `#/channels/new` → `screens/channel/CreateStep.tsx`, `DoneStep.tsx`, `channel.stylex.ts`.

### 5.1 Trạng thái artboard
| Trạng thái | Nội dung |
|---|---|
| `create` + `rivals=some` | header 52: back 44 + "Tạo kênh" 16/600 (**không tabbar**) → lead 13.5 `#A0A0AB` → mục "Đối thủ làm nguồn" 12/600 + "· bắt buộc" cam + "Đã chọn 2 / 5" 12.5 → **5 hàng đối thủ 60px** (checkbox 22×22 r6 đỏ, avatar 36, tên, badge "Tham chiếu nhận diện" 11 `#FFD08A`) → form thêm: input `256×44 r12` + nút "Theo dõi" `94×44` → mục "Thông tin kênh" (Tên kênh, Link kênh YouTube: input 44; card ngôn ngữ: icon globe, "Ngôn ngữ tiêu đề, mô tả" 13.5 + giá trị 13.5/500, hint 12) → card "Cách kênh chạy" (Template + Mức tự chủ: segment `48` cao, 3 nút `105×40 r9` fs14; Nhịp mỗi ngày: stepper `146×44`) → **thanh đáy dính `[0,730 390×88]` nền `#0D0D0F` viền trên**: dòng hint (icon 14 + 12.5) + nút chính `358×48` 15/600 |
| `create` + `rivals=none` | danh sách rỗng + lời dẫn theo dõi đối thủ mới |
| `done` + `identity=running` | avatar 60, "Kênh đã sẵn sàng" 21/600, meta 13, hint 12.5, card "Đang chạy nền" (dot đỏ 8 nhấp nháy, 14.5/600, "không cần đợi" 12), **3 hàng orb 20** (tên 13.5/600 có màu, hành động 13.5, trạng thái 12 xanh/hổ phách), hint 12, card gợi ý (thumb `104×59 r9`, tiêu đề 13.5/500, meta 12) + `.btn-red 332×48` "Làm video mới nhất của đối thủ" |
| `done` + `identity=failed` | hàng nhận diện lỗi + nút thử lại |

### 5.2 Bảng lệch

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Chrome trên | 1 header 52 (back + "Tạo kênh"), **không tabbar** | mobile bar 52 + onboard head 47 ("Viderse · Lần đầu mở / Cài đặt nâng cao") + breadcrumb "‹ Kênh của tôi / Tạo kênh" + h1 22/600 "Tạo kênh đầu tiên" = **~220px** trước nội dung; **có tabbar** | 3 tầng chrome desktop lọt vào điện thoại; tabbar che nội dung | **P1** |
| Lead | 13.5 | 13.5 nhưng dài hơn (4 dòng) | Nội dung | **P2** |
| Khung danh sách đối thủ | danh sách phẳng, hàng 60, không card | `section.card` r14 chứa danh sách | Sai kiểu khung | **P2** |
| Hàng đối thủ | cao **60**, checkbox **22×22 r6** (đỏ), avatar **36**, badge "Tham chiếu nhận diện" 11 | cao **56**, checkbox **18** r5, avatar **24**, không badge | Nhỏ hơn 4/6/12px | **P1** |
| Dòng đếm | "Đã chọn 2 / 5" 12.5 | "Đã chọn 0 · trong 2 đối thủ đang theo dõi" | Sai chữ | **P2** |
| Form thêm | input `256×44` + nút `94×44` | `vch-add` cao 34, 2 nút nhỏ | 34 < 44 | **P1** |
| "Thông tin kênh" | input 44, card ngôn ngữ 12px | `.vch-field` 61 cao (input bên trong), trong card r14 | Khung/cỡ lệch | **P2** |
| "Cách kênh chạy" | segment 48 (nút 105×40 fs14) + stepper 146×44; **không có** hàng "Chọn template" | `Segmented size="sm"` (nút thấp hơn), stepper `.vch-runrow` 47, **thêm hàng "Chọn template"** | Thiếu/ thừa 1 hàng | **P2** |
| Thanh đáy | nền `#0D0D0F` + viền trên, 1 dòng hint + nút chính full-width 48 | `.vch-foot` cao **103**, **không nền** (nội dung cuộn xuyên qua), hint + "Huỷ" (outline 36) + "Tạo kênh" (36) trên 1 hàng | Mất nền, nút 36, thêm "Huỷ" (AB dùng back) | **P1** |
| Bước `done` | như 5.1 | fallback khi `channelId=null`: chỉ h1 20 + "Tạo kênh" — **không quan sát được** trạng thái thật (production read-only) | Cần audit lại bằng dữ liệu thật | — |

**Ảnh cặp:** `vmob-MAuditB-arte-MCreate-create.png` ↔ `vmob-MAuditB-app-mcreate.png`, `…-app-360-new.png`; `…-arte-MCreate-done.png` ↔ `…-app-mcreate-done.png` (fallback).

---

## 6. MSettings — G3 · Cài đặt

**Artboard:** `MSettings.dc.html` · tweak `screen` = `list`|`tools`|`connect`|`agents`|`accounts`|`storage`; `sheet` = `none`|`qr`|`folder`|`model`|`store`; `bili` = 5 trạng thái; `qr` = 7 trạng thái; `store` = **10** trạng thái; `usage` = 2.
**App:** `#/settings?section=` → `screens/settings/*` + `settings.stylex.ts` (có media query 767px).

### 6.1 Vỏ màn (áp dụng cho **mọi** section)

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Định vị | trang con trong khung, header ở `y=44` (dưới status minh hoạ) | `.vv-mset { position: fixed; inset 0 }` → header `y=0..52` | **Nằm dưới notch** (inset 59) | **P0** |
| Header | back `44×44 r12` + "Cài đặt"/tên mục 16/600 | `button.vv-mset__back 44×44 r12` + `span.vv-mset__title` 16/600 ✓ | ✓ (trừ định vị) | — |
| Tabbar | **không có** | `nav.vv-tabbar` vẫn render (bị `.vv-mset` che) | Không thấy nhưng vẫn trong DOM/focus | **P2** |
| Đáy | dải home 26px; nội dung dừng ở `y=818` | `.vv-mset__body` `padding: 4px 16px 20px`, cuộn tới `y=844`; phần tử cuối của Lưu trữ kết thúc ở **824** (vùng home bắt đầu 810) | Mất safe area dưới | **P1** |
| Danh sách (list) | card "Máy sẵn sàng" `358×71 r16 p14` (vòng icon 36, 15/600, 12.5) + nav card `358×338 r16` 6 hàng **56** (icon 19, nhãn 14.5, giá trị 13, chevron 15) + card "Luồng chạy, nhịp, tạm dừng" `358×83` + dòng phiên bản 12 | `358×71` ✓, nav card ✓ hàng 56 ✓, card luồng chạy ✓ | ✓ khớp | — |
| Hàng "Thành viên" | **không có** trong AB | có, và **lệch grid**: icon/nhãn ở `y=588`, giá trị xuống `y=630` | Hàng lệch + không có trong AB | **P1** |
| Dòng phiên bản | "Viderse 1.4.0 · cài đặt áp dụng cho máy này" 12 | "Viderse.dev · cài đặt áp dụng cho máy này" | Sai chữ (`.dev` lộ ra) | **P2** |

### 6.2 `tools` — Công cụ

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Hàng | `356×85`, icon 19, tiêu đề 14.5/500, hint 12.5 clamp2, pill `53×24 r999` 11.5/500 | hàng 85/86 ✓, `ibox` 19×19, tiêu đề 14.5/500 ✓, pill `53×24 r999` 11.5 ✓ | ✓ (alpha pill 0.12 vs 0.10 AB) | **P2** |
| Hàng có bản cập nhật | cao **137**: hint màu warn, pill "Có bản mới" `81×24`, **nút đỏ "Cập nhật · 125 MB" `297×44`** thụt lề dưới tiêu đề | không quan sát được (API báo đã cài); code có nhánh `ToolsSection.tsx` pill `update` | Không kết luận — cần môi trường có bản mới | — |
| Khối "Chi tiết · phiên bản và thư mục" | không có trong AB | có (50px) — chứa đường dẫn (đúng Invariant #7) | AB thiếu trạng thái | **P2** (GAP AB) |

### 6.3 `connect` — Kết nối AI

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Card nhà cung cấp | `358×159 r16`, avatar **34**, tên 15/600, pill "Đăng nhập gói" `92×20` 11, phụ 12.5, thanh quota `328×4` + dòng "Ảnh hôm nay 43/50" 12 + "đặt lại 07:00", 2 nút `160×44` fs14/500 | `358×160` ✓, avatar 34 ✓, pill ✓, bar `328×4` ✓, nút `160×44` ✓ | ✓ khớp (dữ liệu khác: 0/50) | — |
| Card **YouTube Data API** | **không có trong AB** | `358×132`, tiêu đề wrap **2–3 dòng**, pill "Khoá API"/"Không bắt buộc" xuống dòng, nút "Thêm khoá" `96×32` chen vào cột tiêu đề | Vỡ layout ở 390, vỡ nặng ở 360 | **P1** |
| Nút "Thêm nhà cung cấp" | không có | có (dashed) | AB thiếu | **P2** |

**Ảnh:** `vmob-MAuditB-arte-MSettings-connect.png` ↔ `…-app-settings-connect.png`, `…-app-360-conn.png`.

### 6.4 `agents` — Trợ lý AI

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Hàng trợ lý | card nhóm, hàng orb 20 + tên 13.5/600 + mô tả 12 + trạng thái màu | ✓ tương đương | ✓ | — |
| Nhóm "HOẠ SĨ VẼ ẢNH BẰNG" | 3 lựa chọn (ChatGPT, Grok, **Gemini** kèm khoá API + giá `$0.039/ảnh`) | 2 lựa chọn (ChatGPT, Grok) | Thiếu lựa chọn Gemini (dữ liệu/cấu hình) | **P2** |
| Card "Vẽ lại bằng nguồn khác khi lỗi" + toggle | có | có | ✓ | — |

### 6.5 `accounts` — Tài khoản nền tảng

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Card nền tảng | avatar 34, tên 15/600, pill trạng thái, phụ 12.5 (khả năng + hạn mức), hàng chân (nguồn cookie / "Không cần đăng nhập") | ✓ cùng cấu trúc | ✓ | — |
| Bilibili bị chặn | pill cam "Tạm chặn tới 02:10" + **hộp thông báo cam** "Bilibili đang giới hạn yêu cầu…" + hàng "nenvideo_02 · mã QR / Từ giờ hạn mỗi ngày · lần cuối 08:12" + nút "Gia hạn" 44 | pill "Sẵn sàng" + "Chưa đăng nhập" + nút "Đăng nhập" (trạng thái rỗng) | Không quan sát được trạng thái bị chặn | — |
| Ghi chú chân trang | không có | "Thêm kênh hoặc video lẻ ở Thư viện › Video nền trên máy tính…" | Thêm khối không có trong AB | **P2** |
| Sheet QR | 7 trạng thái (`qr`): sheet r28, tiêu đề 18/600, QR 176, thanh tiến trình, "Mã còn hiệu lực 2:41", hàng "Đang chờ quét", 3 bước đánh số, card "Bilibili trên chính máy này?" + nút "Mở app" | **không quan sát được** (mở QR = tạo yêu cầu đăng nhập → mutate, bị cấm) | Cần audit riêng trên môi trường tạm | — |

**Ảnh:** `vmob-MAuditB-arte-MSettings-accounts.png` ↔ `…-app-settings-accounts.png`; `…-arte-MSettings-qr-waiting.png`, `…-qr-expired.png` ↔ (không có).

### 6.6 `storage` — Lưu trữ

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Chế độ **kho S3 đang hoạt động** (`store=s3active`) | card `358×302`: avatar "GA" `40×40` tròn, "Garage" 15/600, tag "Hoạt động tốt" `22 r6` nền ok 10%, 4 hàng `40` (nhãn 14 `#8B8B97` + giá trị mono 14, "Tên kho/viderse-media", "Vùng/garage"), nút `328×44` "Kiểm tra kết nối"; card "Dung lượng theo nhóm" (`h2` 14/600 + "Đã đếm 25 tệp · 12,7 MB" 12.5) 5 hàng `44`/`64`; ghi chú 12.5 | **giống hệt về cấu trúc** (GA 40×40, "Garage" 15/600, tag "Hoạt động tốt", hàng 40, nút 328×44, `h2` 14/600, 5 hàng 44/64, ghi chú 12.5); card cao `300` (AB `302`) | ✓ khớp (chỉ khác dữ liệu: "viderse" vs "viderse-media") | — |
| 9 biến thể còn lại (`here`, `pick`, `lowspace`, `moving`, `done`, `offline`, `server`, `s3moving`, `s3down`) | có vẽ đủ (thư mục máy tính, đổi thư mục, ổ đầy, đang chuyển, xong, mất kết nối, ổ máy chủ, đang chuyển S3, S3 hỏng) | **chỉ có `s3active`** — bản web không có thư mục cục bộ và không có màn "đang chuyển/mất kết nối" | 8 trạng thái thiết kế chưa dùng tới | **P2** (AB/không áp dụng) |

**Ảnh:** `vmob-MAuditB-arte-MSettings-storage.png` (biến thể `here` — thư mục máy tính, không có trên web) ↔ `…-app-settings-storage.png` = `vmob-MAuditB-arte-MSettings-storage-s3active.png` (biến thể `s3active`, khớp).
**Bằng chứng safe area dưới (nhìn thấy bằng mắt):** trong `…-app-settings-storage.png`, dòng ghi chú cuối "Đổi kho, đổi khoá và chuyển dữ liệu làm trên máy tính: Cài đặt › Lưu trữ › Đổi kho…" **bị cắt** ở mép dưới màn hình (đo: phần tử cuối `y=824`, vùng home indicator bắt đầu `810`) — đúng lỗi `.vv-mset` không có `padding-bottom: max(20px, env(safe-area-inset-bottom))`.

---

## 7. MChannel — F1 · Kênh (không có dữ liệu kênh ⇒ audit một phần)

**Artboard:** `MChannel.dc.html` · tweak `tab` = `videos`|`sources`|`profile`|`templates`, `sheet`.
**App:** `#/channels/<id>` → `screens/channel/ChannelScreen.tsx` (có 37 media query 767px).

### 7.1 Artboard (đủ 4 tab)
| Tab | Nội dung đo được |
|---|---|
| `videos` | appbar: logo 24 + switcher `166×36` (avatar 26, tên 13.5/500, chevron) + jobs + avatar → đầu kênh (avatar **60**, tên **21/600**, meta 12.5, chip "Tự chủ · Cân bằng" 11.5 xanh + "Tiếng Anh") → nút chính `302×48` "Video mới cho kênh" + nút icon `48×48` → card số `358×68 r14` 4 ô `89×66` (số 19/600, nhãn 11.5) → 4 tab `40` cao (nhãn 14 + badge đếm 12/600) → segment lọc `358×38` (6 nút, dot 6px, nhãn 12.5 + đếm) → card gợi ý `358×86` → **hàng video**: thumb `80×45 r8`, tiêu đề 13.5/500 clamp2, dòng trạng thái (dot 6 + nhãn), nút hành động nhỏ `32` |
| `sources` | card "Nhịp và ưu tiên" (sub 13, "Nhịp mỗi ngày" + stepper `146×44`, hint 2 dòng, "Ưu tiên kênh" segment `358×48` Đều đặn/Lượt xem, footnote) |
| `profile` | hồ sơ nhận diện + quy tắc viết + Quy tắc đã học (không chụp lại do hết ngân sách — xem `…-arte-MChannel-profile.png`) |
| `templates` | danh sách template luân phiên (xem `…-arte-MChannel-templates.png`) |

### 7.2 Bảng lệch

| Thành phần | AB | APP | Lệch | Mức |
|---|---|---|---|---|
| Trạng thái kênh rỗng | **AB không vẽ** (desktop F1 có tweak `Kênh mới`); M* chỉ có kênh có dữ liệu | `#/channels/<id>` không tồn tại → thẻ dashed giữa màn "Kênh không tồn tại" (kiểu desktop) + switcher biến thành nút "+ Tạo kênh" | Thiếu trạng thái trong AB; app dùng empty-state desktop | **P1** (GAP AB) |
| Có dữ liệu | như 7.1 | **không quan sát được** (0 kênh) | Cần audit lại với tài khoản có kênh | — |
| Code-level (so AB) `[CODE]` | tab `40` cao; lọc `38`; hàng video thumb `80×45` | `ChannelScreen.tsx` dùng `tabsShort`, `dockSlot`, media query 767 — cấu trúc tab/lọc khớp thiết kế; **không kiểm chứng được số đo runtime** | — | — |

**Ảnh:** `vmob-MAuditB-arte-MChannel-videos.png`, `…-sources.png`, `…-profile.png`, `…-templates.png` ↔ `vmob-MAuditB-app-channel-empty.png`.

---

## 8. GAP — Thư viện (G1) và Cài đặt template (G2)

### 8.1 Xác nhận không có artboard điện thoại
`apps/viderse/docs/design/v1/artboards/canvas.json` → `pages[].mobile` chỉ có: MAuth (A0), MCreate (A1/A2), MExplore (B1), MCompVideo (B2), MNew (C1), MMain (D1), MQueue (D2), MReview (E1), MChannel (F1), MSettings (G3), MKit (H4). **Không có M*** cho **G1 Thư viện** và **G2 Cài đặt template**, dù tiêu đề trang mobile ghi "Điện thoại — cùng hành trình **B → G**" (`notes.mtitle`) và G1/G2 thuộc G.

### 8.2 So với artboard desktop
| | `Library.dc.html` (G1) | `Template.dc.html` (G2) |
|---|---|---|
| Khung | header 55 + `aside.side 252` + `main.center 1188` | header 55 + `aside.side 236` (nút "Tất cả template" + 4 hàng mục) + vùng chính 1204 (head 59 + body 786) |
| Nội dung | rail: nhóm kênh, 4 mục thư viện (hàng 57), ghost "Tạo kênh"; center: head 59, toolbar 38, lưới 670 | rail: 4 mục (Ảnh bìa / Ảnh cảnh / Nền / Xuất) hàng 58 + khối chân 154 |

### 8.3 App trên điện thoại (đo được)
| Màn | App 390×844 | Lệch so với tinh thần desktop + Noverse | Mức |
|---|---|---|---|
| `#/library` | `vlb-head` 140 (h1 **18/600** "Thư viện dùng chung" + hint 13.5 + 2 nút `36`), `vlb-tabs` 38 (4 tab 12.5 + đếm), `vlb-grid` 1 cột, card `358×306 r14`; tabbar highlight **"Kênh"** dù không thuộc kênh | 1) **Không có lối vào**: tabbar 5 mục không có Thư viện (`MobileShell.tsx`), AccountMenu không có link → chỉ vào được từ chip "Chọn template" ở A2 hoặc link ở Cài đặt › Tài khoản. 2) Nút `36` < 44. 3) Rail 4 mục → hàng tab 38 (hợp lý) nhưng nhãn 12.5 nhỏ hơn chuẩn 14.5 của AB. 4) Không có trạng thái rỗng riêng cho tab "Video nền"/"Lớp phủ". | **P1** |
| `#/templates/<id>` | `vt-head` 136: tên template là **`input` 33px** (không phải h1), chip `23` cao, "Đã lưu · 22:14" 12 + nút `36`; `vt-row2` 2538px cao, 3 card `r12`; **không có rail mục** (Ảnh bìa/Ảnh cảnh/Nền/Xuất biến mất, phải cuộn); **không có nút back** | 1) Mất điều hướng mục → người dùng phải cuộn 2538px để tới "Ảnh cảnh". 2) Không có back: chỉ thoát được bằng tabbar (sang màn khác) → mất ngữ cảnh. 3) Tên màn là input 33px thay vì tiêu đề 16–18px như mọi màn M*. 4) `kit-action-trigger 30×30` < 44. | **P1** |

**Ảnh:** `vmob-MAuditB-app-library.png`, `vmob-MAuditB-app-template.png` (không có ảnh artboard tương ứng — chính là GAP).
**Đề xuất hướng (không code):** vẽ **MLibrary** (G1) và **MTemplate** (G2) theo mẫu MChannel: header 52 (back + tên 16/600), dải tab ngang 40 cho 4 mục thay rail, lưới card 1 cột `358`, sheet cho "Nhập template"/"Gắn kênh hoặc video"; với G2 dùng **thanh mục ngang sticky** (Ảnh bìa · Ảnh cảnh · Nền · Xuất) + CTA dính đáy như B2; thêm mục "Thư viện" vào AccountMenu (hoặc tab thứ 6 dạng "Thêm").

---

## 9. Kiểm kê component: kit ↔ app-local ↔ Noverse (cho chuẩn dùng chung)

| Component gặp trong audit | Có trong `platform/ui`? | Chỉ Viderse | Chỉ Noverse | Nên thành component hệ thống (tên + slot) |
|---|---|---|---|---|
| Tabbar dưới | **Không** (`Tabs.tsx` chỉ có `Tabs` panel + `Segmented`) | `shell/MobileShell.tsx` `.vv-tabbar` (StyleX) | `mobile/tabs.tsx` + `mobile/mobile.css` `.m-tabs` | **`MobileTabBar`** — slots: `items[{key,label,icon,href,onSelect,badge,centerAction}]`, `activeKey`, `hidden`, `bottomOffset`; dùng `--tabbar-bottom` |
| Thanh trên (logo + jobs + avatar) | Không | `.vv-mbar` (MobileShell) | `.m-topbar`/`.m-title` | **`MobileTopBar`** — slots: `leading`, `title`, `subtitle`, `actions`, `onBack` |
| Trang con có header/back | Không | `.vv-mset__head`, `.vt-head`, `.vch-*` (mỗi màn tự dựng) | `MobilePage` (`primitives.tsx:20`) | **`MobilePage`** — slots: `title`, `backTitle`, `onBack`, `actions`, `footer` (dính đáy + nâng theo `--kb`), `scroll` |
| Sheet đáy | **Có**: `Sheet.tsx` (`.m-sheet`) | — | `BottomSheet` (bọc lại) | `Sheet` (đã có) — **bổ sung** `--m-safe-bottom`/`--top` + `homeIndicator` mặc định |
| Toast | **Có**: `Toast.tsx` | `toastAnchor.ts` | `.m-toast` | `Toast` (đã có) — thống nhất `--toast-bottom` = `calc(--kb + 92px)` |
| Dải mất kết nối | Không | `shell/OfflineStrip.tsx` (đỏ, `err`) | `.m-offline` (cam, có nút Thử lại + làm mờ nội dung) | **`OfflineBanner`** — slots: `state`, `retry`, `dimContent` |
| Pill viên jobs | Không | `shell/JobsPill.tsx` (ring SVG + số + dot) | — | **`JobsPill`** (đưa vào kit, theme qua token) |
| Chip lọc / chip đối thủ | **Có**: `Chip.tsx` (chưa dùng cho mobile: AB 44px) | `.vv-btn--m`, `chip` trong explore | `.m-chip` 34px + `::before` mở rộng tap | **`MobileChip`** — height 44 (AB) / 34 + hit-area 44 (Noverse) → chốt **44px hiển thị** |
| Nút | **Có**: `Button.tsx` | `vv-btn--m`, `btn-red`, `.vch-*` 36 | `.m-button` 50 | `Button` — thêm size `m` = **48** cho CTA dính đáy |
| Segment | **Có**: `Segmented` (kit) | `vch-seg` | `.m-seg` | `Segmented` — thêm size `lg` (48/40, fs14) theo AB |
| Stepper số | Không | `.vch-runrow` (stepper 47) | — | **`Stepper`** — slots: `value`, `min`, `max`, `onChange`, `unit`; cao 44 |
| Ô nhập | **Có**: `Field.tsx`, `Inputs.tsx` | `.vch-field` 61, `addbox` 34 | `.m-field` 46 | `Field` — mobile: **min-height 46**, `inputmode`, `font-size ≥16px` (tránh zoom iOS) |
| Hàng danh sách (icon + nhãn + giá trị + chevron) | Không | `vv-mset` nav rows, `vv-tools__*` | `.m-row` (`min-height 54`) | **`ListRow`** — slots: `icon`, `label`, `value`, `hint`, `chevron`, `onPress`, `tone` |
| Card | **Có** (tokens + `kit.css` `.m-card`) | `section.card` r14 | `.m-card` r20 | `Card` — chốt radius **16** (AB dùng 14–16; Noverse 20) |
| Badge trạng thái (Đã cài / Tạm chặn / Sẵn sàng) | Không | `vv-tools__pill`, `accounts` pill | `.m-pill` | **`StatusPill`** — slots: `tone` (ok/warn/danger/neutral), `label`, `size` (20/24) |
| Thanh tiến trình mảnh (quota, dung lượng) | Không (chỉ `ProgressRing`) | `.conn-m__bar` 4px, storage bar 8px | — | **`MeterBar`** — slots: `segments[]`, `height` (4/8), `label` |
| QR + sheet QR | **Có**: `QrCode.tsx` + `Sheet` | — | — | dùng lại; thêm `QrLoginSheet` (slots: `state`, `qr`, `expiresAt`, `steps`) |
| Bulk bar (chọn nhiều) | Không | `screens/explore/BulkBar.tsx` | — | **`SelectionBar`** — slots: `count`, `actions[]`, `primary{label,onPress}`, `hint`; tự ẩn tabbar |
| Tab màn (Video/Nguồn/Template/Hồ sơ) | **Có** `Tabs` (panel) | `tabsShort` | `.m-tabs-inline` | `Tabs` + variant `mobileInline` (cao 40, fs14, badge) |
| Orb trợ lý | **Có**: `AgentOrb.tsx` | `components/Orb.tsx` | `.m-orb` | `AgentOrb` (đã có) — dùng chung, xoá `Orb.tsx` trùng |
| Empty state | Không | `components/ScreenEmpty.tsx` | `.m-empty` | **`EmptyState`** — slots: `illustration`, `title`, `body`, `actions[]` |
| Avatar kênh/đối thủ | **Có**: `BrandMark.tsx` (logo) — không có avatar initials | `ChannelAvatar.tsx`, `.av` | `.m-orb` initials | **`InitialsAvatar`** — slots: `initials`, `src`, `size`, `ring` |
| Hàng video (thumb 80×45 + tiêu đề + trạng thái) | Không | `screens/channel/*` | — | **`VideoRow`** — slots: `thumb`, `title`, `status{tone,label}`, `action`, `meta` |

---

## 10. Danh sách GAP (artboard thiếu / app thiếu)

**Artboard thiếu (phải vẽ thêm trước khi code):**
1. **G1 Thư viện** và **G2 Cài đặt template** — không có M* (mục 8).
2. **MChannel trạng thái "kênh mới chưa có video"** — desktop F1 có tweak `Kênh mới`, mobile không (mục 7.2).
3. **MExplore trạng thái "chưa theo dõi đối thủ nào"** (desktop B1 có) và **trạng thái lỗi tải/không có video** (H2 có cho desktop, mobile không).
4. **MSettings › Công cụ: hàng "Chi tiết · phiên bản và thư mục"** không có trong AB nhưng app có.
5. **MSettings › Kết nối AI: card YouTube Data API** (Khoá API, "Không bắt buộc", "Thêm khoá") — không có trong AB, đây chính là card vỡ layout.
6. **MSettings › Tài khoản: ghi chú chân trang** ("Thêm kênh hoặc video lẻ ở Thư viện…") — app có, AB không.
7. **MCreate bước `done` khi `identity=failed`** và trạng thái "đang chạy nền" có nhiều hơn 3 hàng — chưa vẽ.
8. **B2 danh sách chương**: AB vẽ nhưng **app chưa có dữ liệu** (`chapters` không tồn tại) → cần chốt: bỏ khỏi AB hay bổ sung API.

**App thiếu / hỏng (phải sửa):**
9. Màn **B2 (chi tiết video)** cho điện thoại (mục 4).
10. **Safe area**: `.vv-mset` (P0), `.m-sheet` padding (P0), thiếu `--m-safe-*`/`--m-top`/`--kb` (mục 2).
11. **Bulk bar** ngoài màn hình (P0) + tabbar không tự ẩn khi chọn nhiều.
12. **Sheet thông tin đối thủ** (B1 `info`) và **sheet thêm đối thủ** (B1 `add`) chưa có trên điện thoại.
13. **Lối vào Thư viện** trên điện thoại.
14. **Back** trên màn Template (và mọi trang con mở từ tabbar).
15. **Keyboard lift**: không có `visualViewport` → CTA dính đáy/sheet bị bàn phím che.
16. **Offline banner**: có `OfflineStrip` nhưng chỉ hiện khi SSE mất kết nối; không có trạng thái "offline" trong artboard M* và **không quan sát được** (CDP offline không lan tới `navigator.onLine` của trang).

**Không quan sát được (cần môi trường/tài khoản khác):**
17. MChannel 4 tab có dữ liệu (0 kênh).
18. MCreate `done` thật + `identity=running/failed`.
19. MSettings `qr` 7 trạng thái, `bili` bị chặn, `tools` có bản cập nhật, `store` 10 trạng thái.
20. Trạng thái loading/skeleton của mọi màn (production phản hồi < 200ms, không bắt được).

---

## 11. Đề xuất thứ tự sửa (không code)

1. **Safe area** (P0, ảnh hưởng mọi màn): thêm token `--safe-top/--safe-bottom/--top/--tabbar-bottom/--kb` vào `:root` Viderse; sửa `.vv-mset` (bỏ `fixed; top:0` hoặc thêm `padding-top: var(--top)`), sửa `.m-sheet` bằng cách định nghĩa `--m-safe-bottom`; port `safeArea.test.ts`.
2. **Màn B2 riêng** + route `#/explore/video/<id>` (hoặc `#/videos/<id>`): header 52 + hero + chương + CTA dính đáy 48px; xoá việc render PreviewPanel trong rail trên mobile.
3. **Explore 1 cột**: bỏ `aside.vx-side`/`aside.vx-rail` khỏi mobile; dựng dải chip ngang 44px, segment 46, chip lọc 44, h1 24/600 + 2 nút 44; `SelectionBar` thay tabbar.
4. **A2 tạo kênh**: bỏ onboard head + breadcrumb trên mobile, footer có nền `#0D0D0F` + nút 48 full-width, hàng đối thủ 60/checkbox 22/avatar 36, thêm badge "Tham chiếu nhận diện".
5. **Vẽ MLibrary/MTemplate** rồi làm màn tương ứng; thêm lối vào Thư viện.
6. **Component hoá** theo mục 9 (MobileTabBar, MobilePage, ListRow, StatusPill, MeterBar, SelectionBar, VideoRow) để Noverse và Viderse dùng chung.

---

## 12. Tệp bằng chứng

Ảnh trong `/tmp` (tiền tố `vmob-MAuditB-`):
`arte-MExplore-browse|select|info|add.png`, `app-explore.png`, `app-explore-select.png`, `app-explore-select-bulk.png`,
`arte-MCompVideo-none|picker|why.png`, `app-compvideo.png`, `app-compvideo-rail.png`, `app-compvideo-clicked.png`,
`arte-MCreate-create|done.png`, `app-mcreate.png`, `app-mcreate-done.png`, `app-360-new.png`,
`arte-MSettings-list|tools|connect|agents|accounts|storage|storage-server|qr-waiting|qr-expired.png`,
`app-settings-list|tools|connect|agents|accounts|storage.png`, `app-360-conn.png`, `app-sheet-jobs.png`,
`arte-MChannel-videos|sources|profile|templates.png`, `app-channel-empty.png`,
`app-library.png`, `app-template.png`, `hdr-zoom.png`, `app-workbench.png`, `app-offline2.png`.

Script đo: `/tmp/vmob-dump.js` (dump cây DOM), `/tmp/vmob-probe*.js`, `/tmp/vmob-cdp.mjs` (CDP), `/tmp/vmob-insets.sh` (set inset 59/34), `/tmp/vmob-go.sh` (open + inset + dump).
