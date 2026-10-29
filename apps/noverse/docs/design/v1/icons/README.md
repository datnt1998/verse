# Icons

142 SVG từ **Iconsax** (gói `iconsax-react@0.0.8`, giấy phép MIT; bộ icon Iconsax của Vuesax), lấy nguyên path gốc.

- `<tên>.svg`: biến thể **Linear** (stroke 1.5, viewBox 24×24).
- `<tên>-bulk.svg`: biến thể **Bulk** (hai tông, lớp phụ opacity .4) — dùng cho mục đang chọn và trạng thái quan trọng.
- Màu trong tệp là `#000`. Trong app, dùng màu theo chữ (`currentColor`): thay `stroke="#000"`/`fill="#000"` bằng `currentColor` khi import, hoặc dùng làm `mask-image` như artboard (`.ic { background-color: currentColor; mask: url(icon.svg) center/contain no-repeat }`).
- `index.json` › `artboardClasses`: class `.i-*` trong artboard → tệp SVG (vd `i-folder` → `folder-open`, `i-tick-b` → `tick-circle-bulk`).

Kích thước dùng: 13–16px trong chip/dòng, 18px mặc định, 20–26px tiêu đề/hộp thoại.
