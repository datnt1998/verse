# 03 — Audit màn hình lõi Viderse mobile (MAuth · MMain · MNew · MQueue · MReview · MKit + shell)

> Phạm vi: artboard `apps/viderse/docs/design/v1/artboards/{MAuth,MMain,MNew,MQueue,MReview,MKit}.dc.html` + shell điện thoại (app bar, dock, JobsPill, bộ đổi kênh, menu tài khoản, hộp thư).
> Đối chiếu production `https://vid.aarverse.studio` (390×844, 360×780, 430×932) và code `apps/viderse/web/src`.
> Không sửa code sản phẩm. Chỉ đọc: navigate / scroll / mở–đóng sheet.

---

## 0. Phương pháp & giới hạn (đọc trước khi dùng số liệu)

**Cách đo artboard.** Artboard là file tự chứa, nhưng mở bằng `file://` thì icon mask (`mask-image` + `/icons/*.svg`) bị chặn — README của thư mục artboards đã cảnh báo. Vì vậy tôi phục vụ tĩnh:

```
cd apps/viderse/docs/design/v1 && python3 -m http.server 8099
→ http://127.0.0.1:8099/artboards/MMain.dc.html
```

Mỗi artboard khoá trạng thái bằng `data-props` (biến `screen`, `scope`, `stage`, `state`, `kicked`, `err`, `sheet`…). Runtime không đọc query string (chỉ có `?theme=`), nên tôi đăng ký một init-script vá `Element.prototype.getAttribute('data-props')` để lấy giá trị từ hash URL:

```
agent-browser --session MAuditA open --init-script /tmp/dc-override.js \
  "http://127.0.0.1:8099/artboards/MReview.dc.html#state=review"
```

→ mọi trạng thái trong báo cáo đều tái lập được bằng đúng hash đó. Số đo lấy từ `getBoundingClientRect()` + `getComputedStyle()` trên DOM thật của artboard, không đọc CSS bằng mắt.

**Cách đo app.** Chrome 390×844 (DPR 1), mỗi lần đo đều kiểm tra `location.host` để chắc chắn không đo nhầm artboard (artboard và app dùng **cùng tên class** `.vv-appbar`, `.vv-tabbar`, `.vv-jobsbtn` — rất dễ lẫn).

**env(safe-area-inset-\*).** Chrome **không** mô phỏng inset tai thỏ, kể cả khi chọn device "iPhone 14" (probe `padding-top: env(safe-area-inset-top)` trả về **0px**, `matchMedia('(display-mode: standalone)')` = false). Do đó mọi số "trên máy thật" dưới đây = số đo thực tế tại inset 0 **+ công thức CSS khai báo trong source**, rồi tính tại inset của iPhone 14/15 (top 47, bottom 34). Cách làm này được ghi rõ ở §9.

**Giới hạn dữ liệu production.** Tài khoản audit `qaaudit` (đăng nhập được, đã kiểm tra `#/account`) có **0 kênh, 0 video, 0 thông báo** (`/api` trả rỗng, Bàn dựng hiện empty state). Vì thế các trạng thái *có dữ liệu* của Bàn dựng / Duyệt / Hàng chờ / Hộp thư **không quan sát được** — chúng được đánh dấu `not observed` và chỉ kết luận ở mức cấu trúc + code + artboard. Mọi mutation (tạo kênh, duyệt, xếp hàng, đổi cài đặt) đều không thực hiện.

**Mức độ.** P0 = hỏng (không dùng được / sai hẳn mô hình màn hình) · P1 = lệch rõ so với artboard hoặc chậm · P2 = bóng bẩy/chi tiết. `[INFERENCE]` = suy luận, không quan sát trực tiếp.

---

## 1. Tóm tắt điều hành

| # | Mức | Phát hiện | Bằng chứng |
|---|-----|-----------|------------|
| 1 | **P0** | **Bàn dựng ở 390px vẫn là layout desktop 3 pane**: sidebar `aside` 248px + pane phụ 142px (360px → 112px; 430px → 182px). Artboard MMain là **một cột** 390px, có H1 “Bàn dựng” 24px, Bản tin, Đề xuất cuộn ngang, KÊNH. | `vmob-MAuditA-APP-workbench-390.png`, `-360.png`; đo: aside 248 / main 142; `WorkbenchScreen.tsx` **không có** nhánh mobile (không có `useMediaQuery`, không có `@media (max-width: 767px)`) |
| 2 | **P0** | **Duyệt (review) ở 390px = list desktop 390×290 + detail desktop 390×392 xếp dọc**, không có thanh hành động dưới, không có filmstrip/phiên bản/prompt như MReview. | `vmob-MAuditA-APP-review.png`; đo: `ASIDE 390×290 pad 12/10`, `MAIN 390×392 pad 24`; `ReviewScreen` không có nhánh mobile |
| 3 | **P1** | **Thanh dưới & dock dính đáy**: app dùng `bottom: calc(4px + env(safe-area-inset-bottom))` → tại inset 0 (Chrome mobile / Android không tai thỏ) dock cách đáy **4px**; artboard đặt **30px**. Bar hành động của “Video mới” cũng chỉ cách đáy 4px (artboard 26px). | đo dock `y=776..840` (390×844) vs artboard `750..814`; `MobileShell.tsx:147`; `kit.css:441` |
| 4 | **P1** | **Thiếu màn hình/hệ thống điện thoại mà artboard vẽ**: chi tiết video MMain (2 stage + 6 trạng thái meta + 2 trạng thái agent), Bản tin dạng sheet, Kế hoạch (plan), 8 trạng thái MReview, sheet hàng chờ (MQueue `sheet=row`). | §4, §6, §7; danh sách GAP §10 |
| 5 | **P1** | **Safe area không có nguồn sự thật duy nhất**: Viderse đọc `env()` ở **5 chỗ / 4 file**, không có token `--m-safe-*`, không có test; Noverse khai báo **1 lần** ở `:root` và mọi thứ nổi dùng `--m-tabbar-bottom`. | §9; `grep -rn safe-area-inset apps/viderse/web/src` = 5 hit |
| 6 | **P1** | **Sheet của kit mất đệm safe-area ở Viderse**: `kit.css` đọc `var(--m-safe-bottom)`/`var(--m-top)` nhưng hai biến này chỉ được định nghĩa trong `apps/noverse/web/src/mobile/mobile.css:6`. Đo sheet thật: `padding-bottom: 0px`. | §2 S16, §9.2 |
| 7 | **P2** | Hàng loạt lệch component nhỏ nhưng nhìn thấy được: nhãn dock 10.5/500 vs 10.5/**600**, FAB thiếu 2 inset bevel, tab sheet 12.5px vs 13.5px, thumb hàng chờ 64×36 vs **80×45**, card hàng chờ radius 14 vs **16**, nút “+ Video mới” mất viền, pill/chip 36px chưa có trong kit, title sheet kit 16px vs **18px**… | §2, §6, §8 |

**Điểm sáng (để không sửa nhầm):** MAuth đăng nhập **khớp pixel** với artboard (xem §3); shell (app bar 52, dock 64/radius 32/glass `rgba(26,26,29,.74)` blur 26, JobsPill vòng 36 dasharray 106.8, FAB gradient radial) **khớp**; Hàng chờ đã có nhánh mobile riêng và khớp gần như toàn bộ số đo.

---

## 2. Shell điện thoại (app bar · dock · JobsPill · bộ đổi kênh · tài khoản · hộp thư)

Artboard tham chiếu: `MMain.dc.html · screen=list&scope=all` (`.vv-appbar`, `.tabbar`, `.tcap`, `.tind`, `.tnew`), `MKit.dc.html · section=bars` (`.vv-appbar` + 7 biến thể thanh trên), `MAuth.dc.html · screen=account`.

| # | Phần tử | Artboard (đo) | App (đo) | Mức | Hướng sửa |
|---|---------|---------------|----------|-----|-----------|
| S1 | App bar | 52px, `padding: 0 12 0 16`, `gap: 10`, nền `#0B0B0D`, logo 24 | **Khớp**: 52px, `0 12 0 16`, gap 10, `MobileShell.tsx:136` | — | Giữ. Đưa thành `MobileTopBar` dùng chung (xem §8) |
| S2 | Avatar phải | 36×36, nền `#222225`, viền `rgba(255,255,255,.07)`, chữ 13/600, ls .02em, x=342 | **Khớp**: 36×36 @ x=342, `#222225`, border line-2, 13/600 ls .26px (`MobileShell.tsx:143`) | — | — |
| S3 | JobsPill | 36×36 @ x=298, vòng SVG r=17, stroke 2, dasharray 106.8, màu `--vv-red`; offline = icon cam, ẩn chấm | **Khớp**: `JobsPill.tsx:73-77` (r=17, stroke 2, dasharray 106.8), `:52` ẩn chấm khi offline, `:69` class theo kind | — | — |
| S4 | Dock khung | 362×64 @ (14, **750**) → cách đáy **30px**; radius 32; padding 5; kính `rgba(26,26,29,.74)` + `blur(26px) saturate(1.8)`; viền `--line-4` | 362×64 @ (14, **776**) → cách đáy **4px**; radius 32; padding 5; `--vv-glass` = `rgba(26,26,29,.74)`; blur 26 / sat 180% (`MobileShell.tsx:147`) | **P1** | Đặt sàn: `bottom: max(30px, calc(env(safe-area-inset-bottom) + 4px))` (hoặc token chung `--m-tabbar-bottom` như Noverse) |
| S5 | Dock — nhãn | 10.5px / **600**, màu `#FFE3E8` khi active | 10.5px / **500** (`MobileShell.tsx:155`) | P2 | `labelActive` phải 600 |
| S6 | Dock — indicator active | 70×52 (inset 0), radius 27 | Glow 72×54 (inset −1px) | P2 | Bỏ inset −1 hoặc cho artboard inset −1 |
| S7 | Dock — icon | active 25.4, thường 24 | active 25, thường 24 | P2 | Canh 25.4 |
| S8 | Dock — FAB | 46×46, gradient radial `#FF6A80 → #FF1F45 → #C80028` + `0 8px 20px -6px rgba(255,0,51,.7)` + 2 inset bevel | 46×46, **gradient khớp tuyệt đối**, nhưng shadow chỉ `0 6px 18px rgba(230,0,46,.4)` — **thiếu 2 inset** | P2 | Thêm `inset 0 1px 0 rgba(255,255,255,.35), inset 0 -2px 4px rgba(0,0,0,.25)` |
| S9 | Dock — badge Duyệt | 17px, nền `--color-warn`, viền 2px surface-4 | 17px, warn, `0 0 0 2px surface-4` (`:157`) | — | Không quan sát được (reviewCount = 0) nhưng code khớp |
| S10 | Nội dung sau dock | `main.vs` padding-bottom **118px** | `--vv-tabbar-pad: 110px` (`viderse.css:247`) | P2 | 118px (hoặc suy ra từ chiều cao dock + 24px khe) |
| S11 | Bộ đổi kênh (trigger) | 36px, radius 999, `#131315`, viền `.07`, 13.5/500, gap 7, **padding 0 10 0 4**, chồng 3 avatar 22px (`--7px`), chevron 14px | 36px, radius 999, cùng nền/viền/font/gap; nhưng **padding 0 12 0 10**; chồng avatar 22px có trong code (`ChannelSwitcher.tsx:59-63`) nhưng **không quan sát được** (0 kênh → app hiện biến thể “Tạo kênh” với icon `add`) | P2 (padding) / *not observed* | Padding trái 4px khi có stack; cần seed 1 kênh để xác nhận stack + chevron |
| S12 | Thanh trên — biến thể | 7 biến thể trong `MKit.dc.html · section=bars`: gốc / “Mọi kênh” + ring + badge / 1 kênh / tên dài cắt “…” / “Tạo kênh” / mất kết nối (icon cam, ẩn chấm) / Duyệt–Khám phá chỉ logo | App có đủ logic: `showSwitcher` chỉ ở workbench/channel; `kind==='offline'` dùng icon `danger`; biến thể “Tạo kênh” đã gặp thật | — (P2 truncation chưa xác nhận) | Cần test tên kênh dài (không quan sát được vì 0 kênh) |
| S13 | Menu tài khoản | Artboard không vẽ **dropdown**; avatar là link tới màn “Tài khoản” (`MAuth.dc.html · screen=account`), có sheet “Đổi mật khẩu” và “Thiết bị đang đăng nhập” | Avatar → `#/account`; màn tài khoản khớp copy + thứ tự + row 56px (`vmob-MAuditA-APP-account.png`) | — | — |
| S14 | Hộp thư / thông báo | `MMain.dc.html · screen=inbox`: sheet “Việc nền” + phụ đề “3 chưa đọc”, tab `Đang chạy 2` / `Thông báo 3`, section `HÔM NAY`, “Đánh dấu đã đọc”, row (tiêu đề + phụ + giờ), cuối sheet “Cài đặt thông báo” | Sheet có: title 18 ✓, grab 36×4 `.22` ✓, seg 48/pad 3 ✓, nhưng **tab không có số**, **không có phụ đề “N chưa đọc”** (phụ đề đang là “Không video nào chạy hay chờ”), **không có “Đánh dấu đã đọc”**, **không có “Cài đặt thông báo”**, **không có link “Mở cả hàng chờ / Xem cả 12 trong hàng chờ”** | **P1** | Bổ sung 4 phần tử trên; phụ đề theo tab (Đang chạy → tổng quan; Thông báo → “N chưa đọc”) |
| S15 | Sheet — độ cao/khoảng | `section` từ y=118 → 844 (726), radius 22 22 0 0, nền `#141416`; header padding `6 16 12`; title 18/600 | y=284 → 844 (560, theo nội dung), radius 22, `#141416` ✓, title 18/600 ✓ — nhưng đây là `ActivityPopover.tsx` **tự viết**, không dùng `Sheet` của kit | — | Chỉ khác do thiếu dữ liệu; nên chuyển sang `Sheet size="md"` sau khi sửa S16 |
| S16 | **Sheet của kit mất padding safe-area trong Viderse** | Artboard: sheet chạm đáy, nội dung dừng trên vạch 26px | `kit.css:157` khai báo `padding-bottom: max(8px, var(--m-safe-bottom))`, nhưng `--m-safe-bottom` **chỉ được định nghĩa ở `apps/noverse/web/src/mobile/mobile.css:6`**; Viderse không định nghĩa → đo trên sheet thật (`#/settings?section=connect` → “Thêm nhà cung cấp”): `padding-bottom: 0px`, `--m-safe-bottom: ""`. Cùng lỗi với `.m-sheet` (`padding-bottom: max(26px, …)` và `max-height: calc(100dvh - var(--m-top) - 12px)` — `--m-top` cũng không tồn tại ở Viderse) `[INFERENCE cho sheet không-md]` | **P1** | Khai báo `:root{--m-safe-top;--m-safe-bottom;--m-top}` ở cả hai app, hoặc cho kit fallback: `var(--m-safe-bottom, env(safe-area-inset-bottom, 0px))` |

---

## 3. MAuth — 6 màn (login · password · denied · setup ×3 · down ×2 · account) + sheet

Artboard: `MAuth.dc.html`. Trạng thái: `screen=login|password|denied|setup|down|account`, `kicked=none|self|other|password|admin|expired|unknown`, `err=none|wrong|limited`, `fail=none|network|copy|norecovery`, `busy`, `step=code|account|recovery`, `down=server|accounts`, `role=admin|member`, `other=yes|no`, `app=viderse|noverse`, `sheet=none|password|devices`.

### 3.1 Đăng nhập (quan sát trực tiếp, session riêng chưa đăng nhập)

| Phần tử | Artboard (`MAuth · screen=login`) | App (`vmob-MAuditA-APP-login-390.png`) | Mức |
|---|---|---|---|
| H1 “Đăng nhập” | 22px/600, y=168 (artboard có status bar 44 → tương đương y=124) | 22px/600, y=**124** | — |
| Phụ đề | 14px, `#A0A0AB`, rộng 295 | 14px, `#A0A0AB`, rộng 295 | — |
| Chip “Máy chủ đang hoạt động” | 182×28, radius 999, `#131315`, viền `rgba(255,255,255,.08)`, 12px | 182×28, radius 999, `#131315`, viền `rgba(255,255,255,.**07**)`, 12px | P2 |
| Ô nhập | 350×48, radius 10, nền `#0F0F11`, viền `.1`, 16px, pad 0 12 | **giống hệt** | — |
| Nhãn field | 11.5px, `#8E8E99` | 11.5px, `#8E8E99` | — |
| Nút chính | 350×48, radius 13, `#EDEDEF`, 15/600 | **giống hệt** | — |
| Ghi chú dưới nút | 13px, rộng 327, y=503 (↔ 459) | 13px, rộng 327, y=**459** | — |
| Footer | “Quên mật khẩu hay chưa có tài khoản?” + “Nhờ quản trị viên ở Noverse › Thành viên.” | Copy **khớp từng chữ** | — |

→ **Login đạt chuẩn pixel**; chỉ lệch alpha viền chip (.08 vs .07). Artboard vẽ thêm status bar 44px giả (mọi màn M* đều vậy) — app dựa vào `env(safe-area-inset-top)`.

### 3.2 Các màn còn lại — `not observed` (khoá theo trạng thái phiên)

`denied`, `password`, `down=server|accounts`, `setup` chỉ hiện khi server trả trạng thái tương ứng (`App.tsx:107-111`), không thể tạo trên production (read-only). Đối chiếu ở mức code:

| Màn | Artboard (nội dung/đo) | App (code) | Mức |
|---|---|---|---|
| `denied` (kicked=self/other/password/admin/expired/unknown) | 6 biến thể copy khác nhau (bị đăng xuất từ thiết bị khác / đổi mật khẩu / admin thu quyền / phiên hết hạn / không rõ lý do), có nút “Đăng nhập lại”, dòng “Thiết bị …” | `AccessDeniedScreen` + `auth.stylex.ts` có nhánh mobile (row radius 14, pad 12 14, copy 44/12) | *not observed* — rủi ro: 6 nhánh copy khó kiểm nếu không có test UI |
| `password` (bắt đổi) | “Đổi mật khẩu” + 3 field + nút 48 | `ChangePasswordScreen`, mobile input 48/10/16, submit 48/13 | *not observed* |
| `setup` (3 bước) | Bước 1/3 “Mã thiết lập”, 2/3 “Tài khoản”, 3/3 (recovery); navbar 52 `pad 0 6` | `SetupScreen`, 1 file có mobile | *not observed* |
| `down` | “Không kết nối được máy chủ” / “Tài khoản đang bảo trì”; nút “Thử lại”; “Tự thử lại sau 12 giây” | `ServerDownScreen` | *not observed* |
| `account` | Navbar 52 (`pad 0 6`) + `main 4 16 20` + spacer 26px; row 56px; nút “Đăng xuất” nền cam nhạt | **Khớp**: heading 16px trong navbar; row 356×56 `pad 6 14`; “Đăng xuất” 358×48 radius 12, nền `rgba(255,154,92,.12)`; copy khớp 100% (`vmob-MAuditA-APP-account.png`) | — |
| `sheet=password` / `sheet=devices` | Sheet đổi mật khẩu (3 field + note “mọi thiết bị phải đăng nhập lại”) / sheet 3 thiết bị | Có màn/sheet tương ứng; **không mở** (mutation risk) | *not observed* |

---

## 4. MMain — Bàn dựng (7 màn + trạng thái)

Artboard: `MMain.dc.html` — `screen=list|detail|jobs|inbox|switch|recap|plan`, `scope=all|channel`, `stage=images|review`, `meta=pending|writing|failed|ai|edited|rewrite|history`, `agent=learn|why`.

### 4.1 `screen=list` (màn gốc) — **P0**

Artboard đo được: status 44 · appbar 52 (44→96) · `main.vs` `padding: 6px 16px 118px`, gap 20 · **H1 “Bàn dựng” 24/600, ls −.02em, y=102** · phụ đề “6/13 video hôm nay” 13px · progress 4px · section head 12px uppercase ls .06em · card radius 16, pad 12 14 · “ĐỀ XUẤT” cuộn ngang (card 288px) · “KÊNH” danh sách card.

App ở 390×844 (`vmob-MAuditA-APP-workbench-390.png`):

| Đo | 390 | 360 | 430 |
|---|---|---|---|
| `aside` (sidebar desktop) | 248 | 248 | 248 |
| pane phụ (`main`) | **142** | **112** | **182** |

Nội dung thật trong 142px: `“Video 0” / “Bản tin” / “Đề xuất”` (tab panel desktop), `“Tự lấy video: bật ở 0/0 kênh”`, `“Hàng chờ chung: 0 video”`, H2 “Chưa có video nào” **rộng 54px** (xuống dòng từng chữ), nút “Làm video”.

| # | Lệch | Mức | Hướng sửa |
|---|---|---|---|
| M1 | Không có H1 24px “Bàn dựng” + phụ đề tiến độ ngày | P0 | Màn điện thoại mới: H1 24/600 + “6/13 video hôm nay” + progress 4px |
| M2 | Bố cục 2 cột desktop (aside + pane) thay vì 1 cột 390 | P0 | `useMediaQuery('(max-width: 767px)')` → `MobileWorkbench` một cột |
| M3 | Danh sách video dạng bảng/row desktop thay vì card radius 16 / VideoRow | P0 | Dùng `MobileVideoRow` (§8) |
| M4 | Không có Bản tin (RecapCard) đầu trang | P1 | `RecapCard` + mở sheet Bản tin |
| M5 | Không có dải “ĐỀ XUẤT” cuộn ngang 288px | P1 | `SuggestionCard` + `overflow-x: auto`, `scroll-snap` |
| M6 | Không có section “KÊNH” | P1 | Card kênh (avatar 26 + tên + số video + thanh tiến độ) |
| M7 | Tab panel desktop (“Video/Bản tin/Đề xuất”) hiện nguyên trên điện thoại | P0 | Bỏ ở mobile |

### 4.2 `screen=detail` (stage=images / stage=review + 7 trạng thái meta) — **P0 thiếu hẳn**

Artboard: navbar 52 (`pad 0 6`, nút back 44) + `main` + **thanh hành động dưới** (`abar`) + dock. Text artboard (stage=images): tiêu đề video, chip “Đang vẽ cảnh 5/6”, filmstrip ảnh, ảnh lớn, dải phiên bản (`v1 v2 v3`), khối prompt, hành động `Vẽ lại / Sửa prompt / Duyệt ảnh`, meta “Chờ duyệt · 7 ảnh”, lịch sử.

App: **không có màn chi tiết video trên điện thoại**. Ở 390, chi tiết là pane thứ hai của layout desktop (142px). Không quan sát được trạng thái có dữ liệu.

| # | Lệch | Mức |
|---|---|---|
| M8 | Không có route/màn chi tiết video cho điện thoại (artboard vẽ 9 biến thể) | P0 |
| M9 | Không có thanh hành động dưới (primary 48 + 2 nút 44) | P0 |
| M10 | Không có filmstrip / dải phiên bản / khối prompt trên điện thoại | P1 |

### 4.3 `screen=jobs` (Việc nền) — sheet, có nhưng thiếu

| Phần tử | Artboard | App | Mức |
|---|---|---|---|
| Sheet | y=118→844, radius 22, `#141416`, header `6 16 12`, title 18/600, grab 36×4 `.22` | y=284→844 (theo nội dung), radius 22 ✓, title 18/600 ✓, grab 36×4 `.22` ✓ | — |
| Phụ đề | “2 đang chạy · 12 chờ lượt · 3 kênh” | “Không video nào chạy hay chờ” (đúng khi rỗng) | — |
| Hành động header | “Tạm dừng” | **không có** | P1 |
| Tab | `Đang chạy 2` / `Thông báo 3` — item 174×40 radius 9, **label 13.5px**, seg 358×48 pad 3 | item 174×40 radius 9, **label 12.5px**, seg 48 pad 3 | P2 |
| Section | `ĐANG CHẠY` (row agent + progress), `TIẾP THEO · LUÂN PHIÊN KÊNH` (row số thứ tự) | không có (rỗng) | *not observed* |
| Footer | “Mở cả hàng chờ”, “Ảnh ChatGPT hôm nay 43/50 · đặt lại 07:00”, “Quy tắc chạy” | chỉ “Quy tắc chạy” + 2 dòng hạn mức | P1 |

### 4.4 `screen=inbox` — xem S14 (§2). Tóm lại: P1 (thiếu số đếm tab, “Đánh dấu đã đọc”, “Cài đặt thông báo”, section HÔM NAY, link “Xem cả 12 trong hàng chờ”).

### 4.5 `screen=switch` (bộ đổi kênh) — *not observed*

Artboard: sheet “Đổi kênh” với “Mọi kênh” (chồng avatar + “6/13 video hôm nay”), từng kênh (avatar, tên, “Hôm nay 3/5 · 2 chờ bạn”, chevron, tick đang chọn), “Tạo kênh mới”, “Quản lý kênh”.
App: tài khoản 0 kênh → trigger biến thành “Tạo kênh” và **điều hướng thẳng** `#/channels/new` (đã thử: click → màn “Tạo kênh”, `vmob-MAuditA-APP-channelnew-via-switcher.png`). Sheet `ChannelSwitcher` tồn tại nhưng **không mở được** khi 0 kênh.

### 4.6 `screen=recap` (Bản tin) — **P1 sai mô hình**

Artboard: **sheet toàn màn** “Bản tin · 23:40 → 08:15”, lead “Trong lúc bạn vắng…”, section `MÁY ĐÃ TỰ LÀM 3 việc`, số liệu `12 chờ lượt / 4 chờ bạn / 190/400 ảnh Grok / 0/50 ảnh trả phí`, banner “Phiên ChatGPT hết hạn lúc 23:58” + “Đăng nhập ChatGPT”, `THEO KÊNH` (3 kênh), “Nhắn trợ lý · mọi kênh”.
App: `Recap.tsx` là **panel desktop** (region “Bản tin”, nút “Vào việc ›”), nằm trong sidebar của Bàn dựng → trên điện thoại bị kẹp trong 248px hoặc không mở được. Không quan sát được với 0 kênh.

### 4.7 `screen=plan` (Kế hoạch) — **P1 thiếu**

Artboard: sheet “Kế hoạch hôm nay” với 3 nhóm việc, “Sửa kế hoạch”, “Chạy theo kế hoạch”. App: không có màn/sheet “Kế hoạch” nào trên điện thoại (không tìm thấy chuỗi “Kế hoạch” trong `screens/workbench`).

---

## 5. MNew — Video mới

Artboard `MNew.dc.html · input=empty|pasted|multi` (+ `prio=on`). Đo: navbar 52 `pad 0 6` (nút × 44×44), `main` `padding: 4px 16px 16px`, H1 22/600, seg 358×48 (pad 3, radius 12, item 174×40 radius 9, label 13.5), ô nhập 48, **thanh hành động 94px** `padding: 10 16 4` + **spacer 26px**.

| # | Lệch | Artboard | App | Mức |
|---|---|---|---|---|
| N1 | Đáy thanh hành động | Nút “Làm video” đáy ≈ **814** (cách đáy 30px) | Nút 358×48 đáy **840** (cách đáy 4px); `.plan` `bottom: env(safe-area-inset-bottom)` (`NewScreen.tsx`), đo `y=781 h=63 pad 10 16 4` | **P1** |
| N2 | Nút phụ “Dán” trong ô nhập | Không có ở trạng thái rỗng; trạng thái đã dán có nút **×** tròn | Có nút “Dán” 56×40 (nền sáng) bên phải input | P2 |
| N3 | Placeholder | “Link video YouTube” | “Dán link video YouTube” | P2 |
| N4 | Segmented | item cao 42 | item cao **40** | P2 |
| N5 | Hàng chọn Kênh / Template | Luôn hiện 2 row 56px (label + value + chevron) | Có trong DOM nhưng **0×0** khi chưa có kênh (`KênhChọn…`, `TemplateTheo luân ph…`) → không quan sát được | *not observed* |
| N6 | Dòng ghi chú trên nút | “Lần đầu hàng chờ · bắt đầu khoảng 00:40” | Không có (phụ thuộc dữ liệu) | P2 |
| N7 | 3 card “video nổi trội của đối thủ” | 3 card có thumb 16:9 + kênh + view | Chỉ còn dòng “Hoặc chọn video nổi trội của đối thủ” (rỗng) | P1 (dữ liệu/không vẽ) |
| N8 | Navbar | × 44 + “Video mới” 16/600 | ✓ khớp (`vv-mclose` 44, `newTitle` 16/600) | — |
| N9 | H1 / orbs | H1 22/600 + 4 orb 22px | ✓ khớp | — |

---

## 6. MQueue — Hàng chờ

Artboard `MQueue.dc.html · state=running|paused|limited|empty`, `sheet=rules|row`. Đo: header **56px** `pad 0 8 0 4`, H1 17/600 @ x=52, phụ đề 12.5, `main` `padding: 4px 16px 28px`, card radius 14 pad 12, “Quy tắc chạy” 64px, idle orb 48 + chấm 8, “+ Video mới” 126×44 radius 12 **viền `.12`**.

Đây là màn **khớp nhất** (đã có `queueMobile.stylex.ts`):

| # | Lệch | Artboard | App | Mức |
|---|---|---|---|---|
| Q1 | Header | 56, `0 8 0 4`, H1 17/600 x=52 | 56, `0 8 0 4`, H1 17/600 x=52 (`queueMobile.stylex.ts:7`) | — |
| Q2 | Nội dung | `4 16 28` | `4 16 28` (`:18`) | — |
| Q3 | Idle | orb 48 + chấm 8, “Máy rảnh” 17/600, mô tả 3 dòng, nút viền 44 | orb 48 ✓ chấm 8 ✓ (`:61-62`), nhưng nút “+ Video mới” đo **125×44, `border: 0`** → mất viền | P2 |
| Q4 | Phụ đề header | `● Máy rảnh` (có chấm trạng thái) | “Máy rảnh” (không chấm) | P2 |
| Q5 | Thumb hàng | VideoRow thumb **80×45** (MKit) | 64×36 (`:24`) | P2 |
| Q6 | Card | radius **16**, pad 12 14 (MMain card) / 14 (MQueue card) | radius 14, pad 12 (`:22`) | P2 |
| Q7 | Đáy trang | spacer 26px sau nội dung | Chỉ `padding-bottom: 28px`, **không** cộng `env()` → trên iPhone hàng cuối nằm dưới home indicator | P1 |
| Q8 | Sheet “Quy tắc chạy” | Sheet: grab 36×4, title 18/600, phụ đề, 3 stepper 160×44 **radius 10**, ghi chú, nút “Xong” 358×48 **đáy 814** | Sheet 422px, radius 22 ✓, stepper 44 **radius 12** (`:80`), “Xong” 48 nhưng **đáy 836** (sheet `paddingBottom: env()` = 0) | P1 (đáy) + P2 (radius) |
| Q9 | Hạn mức | “Hôm nay · đặt lại 07:00” / “Tuần này · đặt lại thứ Hai” | “đặt lại Ngày mai 00:00” (cùng 0/50) | P2 (copy) |
| Q10 | `state=limited` | Banner cam “Đã chạm hạn mức …” + nút | Banner dùng `--line-3` (viền trung tính), không dùng tông cam của artboard | P2 |
| Q11 | `sheet=row` | Sheet thao tác trên 1 video (Lên đầu hàng / Tạm dừng / Bỏ khỏi hàng / Mở chi tiết) | Không quan sát được (0 video) | *not observed* |

---

## 7. MReview — Duyệt ảnh

Artboard `MReview.dc.html · state=review|prompt|feedback|edit|redrawing|reveal|empty|learn`. Đo khung `state=review`: status 44 · appbar 52 · **row tiêu đề 48** (H1 “Duyệt ảnh” 24/600 y=100, đếm “Video 1 / 3” 13px bên phải) · `main` `padding: 0 16px 118px` · **`abar` 215px** `padding: 10 16 104` · dock 64 · spacer 26.
Trong `abar`: 2 nút phụ 175×44 radius 12 (14/500) “Vẽ lại”, “Bỏ ảnh này” + nút chính 358×48 radius 13 (15/600) “Duyệt cả video & dựng”.

App ở 390 (`vmob-MAuditA-APP-review.png`): `ASIDE 390×290 pad 12 10` + `MAIN 390×392 pad 24`, nội dung empty 334px, **không** có row tiêu đề, **không** có `abar`.

| # | Lệch | Mức |
|---|---|---|
| R1 | Layout desktop list+detail xếp dọc thay vì màn duyệt ảnh một cột | **P0** |
| R2 | Thiếu row tiêu đề 48px (H1 24 + “Video n / N”) | **P0** |
| R3 | Thiếu thanh hành động dưới (3 nút, đúng thứ tự 2 phụ + 1 chính) | **P0** |
| R4 | Thiếu filmstrip ảnh, dải phiên bản, khối prompt, sheet feedback / edit / redrawing / reveal / learn (7 trạng thái artboard) | P1 |
| R5 | Empty state dùng style desktop (pad 24, khối 334px giữa màn) thay vì empty của MReview | P1 |
| R6 | Footer “Duyệt xong hôm nay 0 video” nằm trong aside desktop (đúng artboard? không — artboard không vẽ footer này trên điện thoại) | P2 |

---

## 8. MKit — danh mục component (đối chiếu từng instance)

Artboard `MKit.dc.html · section=bars|controls|lists|sheets|agent`. Cột “Kit?” = có trong `platform/ui`; “Noverse?” = có bản riêng trong `apps/noverse/web/src/mobile`.

### 8.1 Bảng component

| Component (artboard) | Số đo artboard | App hiện tại | Kit? | Noverse? | Đề xuất |
|---|---|---|---|---|---|
| `.vv-appbar` (+ `.vv-title-m`) | 52, `0 12 0 16`, gap 10; nút 36, vùng chạm 44; biến thể “chỉ logo” | `MobileShell.tsx:136` (Viderse-only) | ✗ | ✗ (tự viết) | `MobileTopBar` — slots: `leading` (logo/back), `center` (switcher/title), `actions` (jobs, avatar); props `insetTop`, `variant` |
| `.vv-tabbar` (dock) | 362×64, side 14, **bottom 30**, radius 32, pad 5, glass `.74` + blur 26/sat 1.8; item 70×52; nhãn 10.5/600; badge 17 | `MobileShell.tsx:147-157` | ✗ | ✗ (`tabs.tsx`) | `MobileDock` — slots `items[4]`, `fab`; props `activeId`, `badge`, `bottomOffset` (`--m-tabbar-bottom`) |
| `.vv-jobsbtn` | 36, ring r17 stroke 2 dash 106.8, 4 kind (idle/run/paused/offline) | `JobsPill.tsx` (Viderse-only) | ✗ | ✗ | `JobsPill` — props `progress`, `unread`, `kind`, `compact` |
| `.vv-mswitch` | 36/999, stack avatar 22 (−7px), chevron 14 | `ChannelSwitcher.tsx` | ✗ | ✗ | `ChannelSwitcherTrigger` + `ChannelSwitcherSheet` — slots `avatars`, `label`, `hint` |
| `.vv-sheet` (+`__grab`,`__title`) | radius 22, max-h 86%, grab 36×4 `.22`, title **18/600** `6 16 10` | `Sheet` (kit) `size="md"`: radius 22 ✓, grab 36×4 `.22` ✓, nhưng **title 16px** (`kit.css:161`) và `padding-bottom` hỏng vì thiếu `--m-safe-bottom` (S16). Sheet “Việc nền” thì do `ActivityPopover.tsx` tự viết (title 18 ✓) | ✓ | ✓ (size `lg`) | `size="md"` phải = MKit: title **18px**, `padding-bottom: max(8px, var(--m-safe-bottom, env(safe-area-inset-bottom, 0px)))` + prop `homeIndicator` |
| `.vv-seg--m` | 358×48, pad 3, radius 12, item 40 radius 9, label 13.5 | `Segmented` + `size={40}` | ✓ (số) | ✓ | Thêm preset `size="m"` (root 48/btn 40/label 13.5) |
| `.vv-btn--m` / `--m-lg` | 44/12/14·500 · 48/13/15·600 | `Button size="lg"` / `"xl"` | ✓ | ✓ | Giữ; tài liệu hoá mapping |
| `.vv-field--m` | 44/12/15 `0 12` | `Field` 36, **không có size mobile** | ✓ nhưng thiếu size | ✓ | Thêm `size="m"` (44) / `"lg"` (48 — MAuth đang tự set 48) |
| `.vv-switch--m` | 50×30, track đỏ | `Switch` nhận tuple `[50,30]`, có `--kit-shadow-mobile-thumb` | ✓ (tuple) | ✓ | Thêm `size="m"` = [50,30] |
| `.vv-stepper--m` | 160×44 radius **10** | `NumberStepper` 148×36 radius 10; Hàng chờ tự viết 160×44 radius **12** | ✓ (size cũ) | ✗ | `size="m"` 160×44 radius 10 |
| `.vv-pill` | 36/999, `#222225`, viền `.14`, 12.5 | `Chip` max 28; Hàng chờ tự viết filter 36 | ✓ nhưng thiếu 36 | ✓ | `Chip size="lg"` = 36 |
| `.vv-row--m` | 356×56, `6 14`, gap 12 | tự viết ở settings (`0 14`), account (`6 14`) | ✗ | ✗ | `MobileRow` — slots `leading`, `title`, `sub`, `trailing`, `chevron`; prop `danger` |
| `VideoRow` | thumb **80×45**, tiêu đề 2 dòng, meta “Chờ duyệt · 7 ảnh”, nút 36 | Hàng chờ: thumb 64×36 | ✗ | ✗ | `MobileVideoRow` — slots `thumb`, `title`, `meta`, `action` |
| `.vv-choice` (role=radio) | 358×60, radius 14, pad 8 10, viền accent .4 + nền accent .06 | `RadioCard` (kit) | ✓ | ✓ | Dùng `RadioCard`, canh 60px |
| `.vv-banner--issue` | 358×112, radius 12, pad 10 12, **cam** `rgba(255,154,92,.22/.07)` | `Banner` có `tone="warning"/"error"`; Hàng chờ tự viết banner trung tính | ✓ | ✓ | Dùng `Banner tone="warning"` thay banner tự chế |
| `.vv-toast` | 40px cao, nằm **trên** thanh dưới | `ToastHost` (kit) + `toastAnchor` | ✓ | ✓ | Canh anchor theo `--m-tabbar-bottom` |
| `.vv-orb` | 26px, viền gradient | `AgentOrb` (kit) | ✓ | ✓ | — |
| Empty state | orb 48 + chấm 8 + title 17/600 + mô tả 13 + nút 44 | `EmptyState` (icon 19/22) | ✓ nhưng thiếu biến thể orb | ✓ | `EmptyState variant="phone"` (orb 48) |
| `RecapCard` | card radius 16, pad 12 14, orb 26, “Bản tin · 23:40 → 08:15” | `recap/Recap.tsx` (panel desktop) | ✗ | ✗ | `RecapCard` (tóm tắt) + `RecapSheet` (toàn màn) |
| `.vv-suggest--m` | card **288px**, cuộn ngang, 2 nút (Lên đầu hàng / Bỏ qua) | có card đề xuất nhưng desktop | ✗ | ✗ | `SuggestionCard` 288px + `scroll-snap` |

### 8.2 Ghi chú component
- **Kit đã đủ cho 60%**: Sheet, Segmented, Button, Switch, RadioCard, Banner, Toast, AgentOrb, EmptyState — nhưng thiếu **preset kích thước điện thoại** (44/48/50/56) và thiếu **6 component khung** (top bar, dock, jobs pill, switcher, mobile row, mobile video row) → hai app đang viết trùng.
- Noverse có `mobile/primitives.tsx` + `mobile/tabs.tsx` + `mobile.css` riêng; Viderse có `MobileShell.tsx` + `*.stylex` riêng. **Không có component nào của shell điện thoại nằm trong `platform/ui`.**

---

## 9. Safe area (đối chiếu Noverse làm chuẩn)

### 9.1 Artboard đặt gì
| Vị trí | Artboard | Ghi chú |
|---|---|---|
| Thanh trên | 44px status bar giả (0→44) rồi appbar 52 | Home indicator giả 134×5 tại `bottom: 8px` |
| Dock | `bottom: 30px` (`kit.css:441`) | = 26px vùng home indicator + 4px khe |
| Spacer đáy mọi màn | `<div style="height:26px">` xuất hiện ở **mọi** artboard M* (MAuth 11, MMain 18, MReview 9, MQueue 6, MNew 5, MKit 5 chỗ) | Mô hình: inset đáy ≈ **26px** |
| Thanh hành động | MNew: bar `10 16 4` + 26 spacer; MReview: `abar` `10 16 104` (104 = 64 dock + 40) | Nút chính cách đáy 30px |
| Sheet | MMain jobs: sheet chạm đáy 844, nội dung cuộn kết thúc 818; MQueue rules: nút “Xong” đáy 814 | Sheet **không** tự cộng inset; nội dung dừng trên vạch 26px |

### 9.2 App đặt gì (số đo tại inset 0)
| Vị trí | Code | Giá trị đo | Trên iPhone 14 (inset 34) |
|---|---|---|---|
| Shell | `paddingTop: env(safe-area-inset-top)` — `MobileShell.tsx:134` | 0 | 47 (đúng) |
| Dock | `bottom: calc(4px + env(safe-area-inset-bottom))` — `:147` | **4px** | 38px (artboard 30 → lệch +8) |
| Nội dung | `--vv-tabbar-pad: 110px` (`viderse.css:247`) | 110 | 110 (không cộng inset) |
| Video mới | `.plan { bottom: env(safe-area-inset-bottom); padding: 10 16 4 }` | **4px** (nút đáy 840) | 34px (artboard 30 → +4) |
| Hàng chờ — sheet | `paddingBottom: env(safe-area-inset-bottom, 0px)` — `queueMobile.stylex.ts:70` | 0 (nút “Xong” đáy 836) | 34 |
| Hàng chờ — trang | `padding: 4 16 28` (không env) | 28 | 28 → **hàng cuối nằm trong vùng home indicator** |
| Toast | `toastAnchor.ts` | theo dock | — |
| Sheet của kit | `kit.css:144/157` dùng `var(--m-safe-bottom)` / `var(--m-top)` | **không tồn tại ở Viderse → `padding-bottom` = 0px** (đo trên sheet “Thêm nhà cung cấp”), `max-height` của `.m-sheet` không-md thành invalid | 0 (lẽ ra 34) |

**Tổng: 5 điểm đọc `env()` trong 4 file + 2 biến kit không được định nghĩa, không có biến chung, không có test.** (Noverse: 1 khai báo `:root` + `--m-tabbar-bottom` + `safeArea.test.ts`.)

### 9.3 Quy tắc nên đưa vào chuẩn chung (đề xuất)
1. `:root { --m-safe-top: env(safe-area-inset-top, 0px); --m-safe-bottom: env(safe-area-inset-bottom, 0px); --m-top: var(--m-safe-top); --m-edge: 0px }` và `html[data-standalone] { --m-edge: 40px }` — **cả hai app** (Noverse đã có ở `mobile.css:6`; Viderse thiếu hoàn toàn, kể cả script `data-standalone` trong `index.html`). Việc này đồng thời sửa S16 (sheet kit đang `padding-bottom: 0`).
2. Shell cộng inset **một lần** (`padding-top: var(--m-safe-top)`), mọi thứ nổi dùng biến, **không** đọc `env()` rải rác.
3. Dock: ba công thức đang tồn tại — artboard `30px` cố định; app Viderse `4px + inset`; Noverse `max(20px, inset)`. Đề xuất chung: `bottom: max(30px, var(--m-safe-bottom))` → inset 0 = **30** (khớp artboard, thay vì 4), inset 34 = **34** (khớp Noverse). Hệ quả: dock cao thêm 26px so với hiện tại ở mọi điện thoại không tai thỏ.
4. Thanh hành động dưới (Video mới, Duyệt, footer sheet): `padding-bottom: max(30px, var(--m-safe-bottom))` — hiện `/new` để 4px (nút sát đáy), artboard 30px.
5. Trang không có dock (Hàng chờ, Tài khoản, Setup): `padding-bottom: calc(28px + max(26px, var(--m-safe-bottom)))` — artboard = 28px nội dung + spacer 26px = 54px ở inset 0.
6. Sheet: `padding-bottom: max(8px, var(--m-safe-bottom))` (đúng như `kit.css:157` mong đợi) + footer sheet cộng 4px; nút chính trong sheet không được sát đáy.
7. Test: một test kiểu `safeArea.test.ts` của Noverse, chạy trên các class khung (dock, abar, sheet footer) để chặn hồi quy.

---

## 10. GAP — màn hình điện thoại **không có artboard** (hoặc artboard không có bản trong app)

### 10.1 Có trong app, **không có artboard**
| Route | Nội dung | Trạng thái mobile | Mức |
|---|---|---|---|
| `#/library` | “Thư viện dùng chung” — 4 tab (Template 1 / Prompt ảnh 8 / Video nền / Lớp phủ 12), card template, nút “Nhập template”, “Template mới” | Có nhánh mobile (20 file), bố cục 1 cột 390 ổn (`vmob-MAuditA-APP-library.png`) | **GAP** — không có nguồn thiết kế; P1 |
| `#/templates/<id>` | Sửa template (4 section: images/cover/backdrop/output) | **0 nhánh mobile** | GAP + P1 |
| `#/editor/<projectId>` | Trình dựng video | **0 nhánh mobile** | GAP + P1 |
| `#/render/<requestId>`, `#/measure/<requestId>` | Màn tiến trình render / đo | **0 nhánh mobile** | GAP + P2 |
| `#/dev/kit?board=mkit` | Kit điện thoại trong app | Chỉ dev; production chuyển về `#/` (đã thử) | GAP (chấp nhận được) |

### 10.2 Có artboard, **app chưa có** (đã liệt kê ở §4–§7)
`MMain detail` (2 stage + 7 meta + 2 agent) · `MMain plan` · `MMain recap` (dạng sheet) · `MMain switch` (không mở được khi 0 kênh) · `MReview` **toàn bộ 8 trạng thái** · `MQueue sheet=row` · `MQueue state=limited` (banner cam) · `MAuth denied/password/down/setup` (không quan sát được, không kết luận).

### 10.3 Artboard vẽ trạng thái mà app chỉ có một nửa
- `MMain jobs`: thiếu “Tạm dừng”, “Mở cả hàng chờ”, section ĐANG CHẠY / TIẾP THEO.
- `MMain inbox`: thiếu 4 phần tử (S14).
- `MNew`: thiếu 3 card video đối thủ + dòng ghi chú.
- `MQueue`: thiếu chấm trạng thái ở phụ đề, viền nút “+ Video mới”, copy mốc đặt lại hạn mức.

---

## 11. Phụ lục

### 11.1 Ảnh (tất cả trong `/tmp/`)

**Cặp ảnh chính (artboard ↔ app):**

| Màn | Artboard | App |
|---|---|---|
| Đăng nhập | `vmob-MAuditA-AB-MAuth-login.png` | `vmob-MAuditA-APP-login-390.png` |
| Tài khoản | `vmob-MAuditA-AB-MAuth-account-admin.png` | `vmob-MAuditA-APP-account.png` |
| Bàn dựng (list) | `vmob-MAuditA-AB-MMain-list-all.png` | `vmob-MAuditA-APP-workbench-390.png` (+ `-360.png`) |
| Việc nền / Hộp thư | `vmob-MAuditA-AB-MMain-jobs.png`, `-inbox.png` | `vmob-MAuditA-APP-jobs-sheet.png` |
| Video mới | `vmob-MAuditA-AB-MNew-pasted.png`, `-empty.png` | `vmob-MAuditA-APP-new.png` |
| Hàng chờ | `vmob-MAuditA-AB-MQueue-empty.png`, `-running.png` | `vmob-MAuditA-APP-queue.png` |
| Quy tắc chạy | `vmob-MAuditA-AB-MQueue-sheet-rules.png` | `vmob-MAuditA-APP-queue-rules.png` |
| Duyệt ảnh | `vmob-MAuditA-AB-MReview-review.png`, `-empty.png` | `vmob-MAuditA-APP-review.png` |
| Kit component | `vmob-MAuditA-AB-MKit-{bars,controls,lists,sheets,agent}.png` | (production không có `#/dev/kit`; so với instance thật: `-APP-jobs-sheet`, `-APP-queue-rules`, `-APP-settings`) |
| Thư viện (GAP) | — (không có artboard) | `vmob-MAuditA-APP-library.png` |

Artboard: `vmob-MAuditA-AB-MAuth-{login,kicked-*,err-*,pwerr-mismatch,fail-network,busy,account-*,sheet-*,setup-*,down-*}.png` · `AB-MMain-{list-all,list-channel,list-reload,detail-images,detail-review,jobs,inbox,switch,recap,recap-open,plan,meta-*,agent-*}.png` · `AB-MNew-{empty,pasted,multi,multi-prio}.png` · `AB-MQueue-{running,paused,limited,empty,sheet-rules,sheet-row}.png` · `AB-MReview-{review,prompt,feedback,edit,redrawing,reveal,empty,learn}.png` · `AB-MKit-{bars,controls,lists,sheets,agent}.png`.

App: `vmob-MAuditA-APP-{login-390,workbench-390,workbench-360,review,queue,queue-rules,new,channelnew,channelnew-via-switcher,library,explore,settings,account,setup,jobs-sheet}.png`.

### 11.2 File đã đọc
`apps/viderse/web/src/shell/MobileShell.tsx` (:134,:136,:143,:147,:150,:155,:157) · `shell/JobsPill.tsx` (:52,:69,:73-77,:121-137) · `shell/ChannelSwitcher.tsx` (:59-63,:105-113) · `shell/AppShell.tsx:102` · `screens/workbench/WorkbenchScreen.tsx` (không có mobile) · `screens/queue/queueMobile.stylex.ts` (:7,:18,:24,:61,:70,:80,:84) · `screens/queue/QueueScreen.tsx:61,:278` · `screens/auth/auth.stylex.ts` (:10-19,:26-35) · `screens/auth/parts.tsx` (:19-31,:84-88) · `styles/viderse.css` (:76,:247,:310) · `styles/viderse.stylex.ts:31` · `apps/viderse/docs/design/v1/{kit.css,tokens.css}` · `platform/ui/src/{Sheet,Tabs,controls,Inputs,Chip,Banner,feedback,Button}.tsx` · `apps/noverse/web/src/mobile/{mobile.css,safeArea.test.ts}`.

### 11.3 Câu hỏi mở cho design (cần chốt trong chuẩn chung)
1. Dock cách đáy: artboard Viderse **30**, Noverse `max(20, inset)`, app Viderse `4 + inset`. Chọn **một** công thức.
2. Artboard giả định inset đáy **26px** (spacer 26) trong khi iPhone thật là 34 → có cần vẽ lại spacer thành `env()` trong artboard không?
3. Sheet: `size="md"` của kit ghi title 16px nhưng MKit và app đang 18px → sửa kit hay sửa artboard?
4. Card radius: MMain/MKit 16, MQueue 14, app 14 → chuẩn hoá 16?
5. Tab sheet label: 13.5 (artboard) vs 12.5 (app) — chọn 13.5?

---

**Status: DONE_WITH_CONCERNS** — đã audit đủ 6 artboard + shell với số đo hai phía; hai hạn chế: (a) tài khoản audit 0 kênh/0 video nên các trạng thái có dữ liệu của Bàn dựng/Duyệt/Hàng chờ/Hộp thư chỉ kết luận được ở mức cấu trúc + code (`not observed`), (b) Chrome không mô phỏng `env(safe-area-inset-*)` nên số "trên máy thật" là suy ra từ công thức CSS đã khai báo (đã ghi rõ cách tính).
