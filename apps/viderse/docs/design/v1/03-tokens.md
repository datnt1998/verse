# Viderse — Tokens

Giá trị nằm ở [tokens.css](tokens.css) (tên `--vv-*`, để tra trên artboard). Trong verse không chép tệp này vào app: nền, mặt, chữ dùng token của `@noverse/ui`, sắc thương hiệu đặt qua `defineBrand` — bảng ánh xạ ở [02-stack-verse.md](02-stack-verse.md) §2. Tài liệu này chỉ ghi luật dùng. Lớp thành phần dùng các token này ở [kit.css](kit.css) ([05-kit.md](05-kit.md)); xem toàn bộ trên board `Kit` › Nền tảng.

## Màu <!-- r19-kit -->
Bộ màu là bộ của Noverse, chỉ đổi theme (đợt 19). Bảng ánh xạ đầy đủ ở [05-kit.md](05-kit.md).

- **Đỏ là màu nhấn** — đúng chỗ Noverse dùng tím:
  - `--vv-accent` (`#FF4D63`, 6:1 trên nền): hộp kiểm, radio, thanh trượt, dấu tick, link, icon tab đang chọn, vòng focus (`--vv-focus-outline`).
  - `--vv-accent-strong` (`#E6002E`): công tắc bật.
  - `--vv-accent-bg`, `-tint`, `-border`, `-focus`, `-ring`, `-mark`, `-soft`: nền và viền lựa chọn đang chọn, ô đang focus, thẻ trong ô chọn nhiều, phần khớp khi tìm.
  - `--vv-red` (`#FF0033`): logo và chấm “đang chạy”.
  - **Đỏ không làm nền nút**, trừ nút + tròn ở thanh dưới điện thoại.
- **Nút chính sáng:** `--vv-primary` (`#EDEDEF`), chữ `--vv-primary-fg`. Một màn một nút chính.
- **Cam là xoá và lỗi** — đúng chỗ Noverse dùng đỏ:
  - `--vv-danger` (`#FF9A5C`) nền nút xoá vĩnh viễn, chữ tối `--vv-danger-fg` (cao hơn 8:1).
  - `--vv-danger-soft` + `--vv-danger-border` + chữ `--vv-danger-text-2`: nút nguy hiểm làm lại được.
  - `--vv-danger-text`: chữ lỗi, mục menu nguy hiểm; `--vv-danger-focus` + `--vv-danger-ring`: viền ô lỗi.
- **Vàng** (`--vv-st-review`) cho “tới lượt bạn” (Chờ duyệt, Chờ bạn) và chỉ số nổi trội.
- **Xanh lá** (`--vv-st-ready`) cho Sẵn sàng, Xong, Đã kết nối. Công tắc bật không còn màu xanh lá.
- **Xanh dương nhạt** (`--vv-info`) cho banner thông tin.
- **Trợ lý:** màu chữ, cung logo và orb là cặp riêng của mỗi trợ lý; không dùng cho thứ khác.
- **Xám:**
  - Chữ phụ tối thiểu `--vv-muted` (5,8:1 trên nền, 5,4:1 trên card).
  - `--vv-faint` (3,9:1) chỉ cho chấm, icon, viền.
  - `--vv-disabled` chỉ cho chữ của thứ đang khoá.

## Chữ
- Be Vietnam Pro 400 / 500 / 600 / 700, subset `vietnamese`.
- JetBrains Mono cho thời lượng, mốc giờ, phím tắt.
- Cỡ:

  | Vai trò | Cỡ |
  |---|---|
  | Tiêu đề trang/video | 17px / 600 |
  | Hero Video mới | 30px |
  | Thân | 14px |
  | Nút, tab | 13px |
  | Dòng phụ | 12px |
  | Tiêu đề hộp thoại | 16px / 600 |
  | Nhãn nhóm | 11px / 600 VIẾT HOA, giãn chữ .08em (trong ô: 10,5px) |

- **Nút, tab, chip, kbd không xuống dòng** (`white-space: nowrap`). Chữ dài cắt bằng `…` và có `title`.

## Không gian
- Bo góc (Noverse): huy hiệu 6px · tooltip 7px · chip, nút nhỏ 8px · mục menu 9px · nút, ô nhập 10px · thẻ lựa chọn 11px · banner, khối trống 12px · menu, popover 14px · thẻ 16px · hộp thoại 18px · sheet 22px · thumbnail 7px · **avatar 50% (luôn tròn)**. Không có vạch màu một bên — xem [04-components › Luật hình](04-components.md#c-shape-rules). <!-- r26-rules -->
- Cao: nút, ô nhập, ô chọn 36px; nút nhỏ 30px; mục menu ≥ 34px.
- Vùng bấm: desktop ≥ 24px (nút thường 30–36px); điện thoại ≥ 44px.
- Cột desktop:

  | Cột | Rộng |
  |---|---|
  | Cột trái | 252–268px |
  | Cột phải | 316–340px |
  | Giữa | `minmax(0, 1fr)` |

- Breakpoint:

  | Bề rộng | Bố cục |
  |---|---|
  | ≥ 1380px | Đủ cột |
  | 1100–1379px | Bảng ẩn cột phụ |
  | < 1100px | Cột xếp dọc |
  | < 768px | Bố cục điện thoại (Gói V10) |

## Chuyển động
- 150–200ms, ease-out, chỉ `transform`, `opacity` và màu.
- Hiệu ứng lặp:

  | Hiệu ứng | Chu kỳ |
  |---|---|
  | Orb trợ lý đang chạy | thở 2,8s |
  | Chấm đỏ | 1,6s |
  | Logo ống kính | xoay 9s, chỉ khi có job chạy |
  | Chữ shimmer “đang vẽ” | 2,2s |

- `prefers-reduced-motion: reduce` tắt hết.

## Token của UI Kit <!-- r15-kit -->
Thêm ở đợt 15 để `kit.css` không phải viết cứng giá trị nào.

| Nhóm | Token | Dùng cho |
|---|---|---|
| Nút | `--vv-primary`, `--vv-primary-hover`, `--vv-primary-fg` (tên cũ `--vv-white-button` vẫn trỏ vào đây), `--vv-ghost-border` | Nút chính sáng; viền nút thường |
| Ô nhập | `--vv-field`, `--vv-field-border`, `--vv-field-border-hover`, `--vv-field-border-focus` | Nền và viền `.vv-field`, `.vv-fieldbox`; kèm vòng `--vv-focus` khi focus bằng phím |
| Lớp nổi | `--vv-popover`, `--vv-popover-border`, `--vv-popover-shadow`, `--vv-scrim`, `--vv-glass`, `--vv-hover` | Menu, select, tooltip; nền mờ sau hộp thoại/sheet; thanh dưới kính; hover hàng |
| Nền trạng thái | `--vv-st-review-bg`, `--vv-st-issue-bg`, `--vv-st-issue-border`, `--vv-st-ready-bg`, `--vv-st-working-bg`, `--vv-director-bg`, `--vv-ref-bg` | Banner, tag, chip Tham chiếu |
| Bo góc | xem mục Không gian (đợt 19 theo Noverse) | |
| Chữ điện thoại | `--vv-fs-title-m` 24px · `--vv-fs-nav-m` 16px · `--vv-fs-sheet` 18px | Tiêu đề màn gốc, màn con, sheet |
| Kích thước điện thoại | `--vv-m-btn` 44 · `--vv-m-btn-lg` 48 · `--vv-m-row` 56 · `--vv-m-bar` 52 · `--vv-m-tabbar` 64 | Nút, nút lớn, hàng cài đặt, đầu trang, thanh dưới |
| Focus | `--vv-focus` · `--vv-focus-outline` | Vòng 3px màu nhấn quanh ô đang focus; viền 2px màu nhấn cách 2px cho mọi phần tử bấm được (`:focus-visible`) |

## Token thêm ở đợt 19 <!-- r19-kit -->

| Nhóm | Token | Dùng cho |
|---|---|---|
| Nhấn | `--vv-accent`, `--vv-accent-strong`, `--vv-accent-soft`, `--vv-accent-bg`, `--vv-accent-tint`, `--vv-accent-border`, `--vv-accent-focus`, `--vv-accent-ring`, `--vv-accent-mark` | Chỗ Noverse dùng tím |
| Phá huỷ | `--vv-danger`, `--vv-danger-hover`, `--vv-danger-fg`, `--vv-danger-text`, `--vv-danger-text-2`, `--vv-danger-soft`, `--vv-danger-soft-hover`, `--vv-danger-tint`, `--vv-danger-border`, `--vv-danger-focus`, `--vv-danger-ring` | Chỗ Noverse dùng đỏ |
| Chữ | `--vv-text-soft`, `--vv-text-hint`, `--vv-disabled` | Thân hộp thoại, mô tả dưới lựa chọn, chữ đang khoá |
| Bề mặt | `--vv-stage`, `--vv-selected-2`, `--vv-dialog`, `--vv-toast`, `--vv-field-hover`, `--vv-hover-row`, `--vv-opt-hover`, `--vv-opt-active`, `--vv-skel-a/-b` | Khung minh hoạ, nút đang chọn trong nhóm chọn, hộp thoại, toast, hover |
| Thông tin | `--vv-info`, `--vv-info-bg`, `--vv-info-border`, `--vv-st-*-tint`, `--vv-st-*-border` | Banner |
| Cỡ | `--vv-h-btn` 36 · `--vv-h-btn-sm` 30 · `--vv-h-field` 36 · `--vv-h-opt` 34 · `--vv-fs-dialog` 16 · `--vv-fs-card` 14 · `--vv-fs-small` 11,5 · `--vv-r-*` | Theo Kit Noverse |
| Chuyển động | `--vv-ease`, `--vv-ease-spring`, `--vv-dur` 150ms, `--vv-dur-pop` 170ms, `--vv-dur-dialog` 300ms | Menu mở, công tắc nảy, hộp thoại |

## Token chốt ở đợt 31 (Gói V27) <!-- r31-v27 -->

| Token | Giá trị | Dùng cho | Thay chỗ dev đang dùng |
|---|---|---|---|
| `--vv-red-button-fill` (mới) | `radial-gradient(circle at 35% 28%, #FF6A80 0%, #FF1F45 45%, #C80028 100%)` | **Chỉ** cầu nút + giữa thanh dưới điện thoại (`.vv-tabbar__new`). Bóng giữ như kit. Nút thường vẫn không đỏ | `--vv-red-button` phẳng (#62) |
| `--vv-accent-soft` (có sẵn) | `#FFD3DA` | Chữ huy hiệu nhấn: “Quản trị viên”, “Đang mở” ở Tài khoản Viderse, nền `--vv-accent-bg` | `--vv-tabbar-on` (#51) |
| `--vv-info` (có sẵn) | `#9DB0F7` | Icon và viền chữ của băng thông tin (`.vv-banner--info`) | `--kit-info-fg` #9AB0D6 (#51) |
| `--vv-surface-3` (có sẵn) | `#18181A` | Ô tròn chứa logo app ở danh sách Ứng dụng | `#18181B` viết cứng |

Huy hiệu tím “Quản trị” ở tab Thêm của Noverse dùng token của Noverse, không phải token Viderse.

## Token thêm ở đợt 32 <!-- r32 -->

| Token | Giá trị | Dùng cho |
|---|---|---|
| `--vv-wide-content` | 1280px | Bề rộng tối đa của cột giữa ở Bàn dựng, Hàng chờ, Kênh, Việc nền (màn ≥ 1600px) |
| `--vv-wide-grid` | 1760px | Lưới thẻ (Khám phá, Thư viện) và khung xem ảnh (Duyệt) |
| `--vv-wide-read` | 880px | Form, trang đọc (Cài đặt, Template); nội dung hẹp hơn thì giữ bề rộng riêng, chỉ căn giữa |
| `--vv-wide-composer` | 880px | Ô Nhắn trợ lý và dòng phím tắt bên dưới |
| `--vv-badge-bg` | `rgba(246,203,140,.16)` | Nền badge số ở MainNav (Duyệt 3); chữ `--vv-st-review`. Lớp `.vv-badge` trong kit.css (#14) |
