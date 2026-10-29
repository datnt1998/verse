# 01 · Review UX & user flow

Review lần cuối trước khi implement. Mỗi mục: vấn đề → quyết định → trạng thái (**Đã sửa trong artboard** / **Quy định trong spec** / **Backlog**).

## A. Luồng & trạng thái

| # | Vấn đề | Quyết định | Trạng thái |
|---|---|---|---|
| A1 | Màn “Dàn ý” không thể hiện cấp tổ chức thật (Thư viện → Dự án → Truyện → Tập → Chương) | Thay bằng màn **Dự án**: cây bên trái, chi tiết giữa, cây thư mục chuẩn bên phải | Đã sửa |
| A2 | Thiếu chuyển dự án | Breadcrumb `dự án / truyện / tập` trên mọi header; menu chuyển dự án ở màn Dự án | Đã sửa |
| A3 | Tạo truyện: tên thư mục `{stt}-{slug}` lấy từ tên truyện, nhưng tên truyện chỉ có sau khi ARCHITECT `save_book` | Tạo thư mục tạm `NN-truyen-moi` (hoặc slug từ tên làm việc người dùng nhập); khi có tên thật và truyện đang rảnh → đề xuất “Đổi tên thư mục theo tên truyện” | Quy định trong spec (`02-ia-flows.md` F3) |
| A4 | `/reopen` cần “hướng viết tiếp” nhưng nút chỉ bấm một phát | Nút mở hộp thoại **Viết tiếp tập mới** có ô hướng viết (tùy chọn) | Đã sửa (Dialogs › reopen) |
| A5 | Thiếu bảng lệnh khi gõ `/` | **Command palette** neo trên ô nhập; lệnh chưa có API hiện nhãn “Chưa có API”, lệnh cần rảnh hiện “Cần rảnh” | Đã sửa (Dialogs › palette) |
| A6 | Thiếu trạng thái mất kết nối SSE, phiên hết hạn (401), truyện bị khóa (409 `errBookLocked`) | Banner kết nối lại có đếm ngược; 401 → màn Đăng nhập “Phiên đã hết hạn”; hộp thoại “Truyện đang mở ở nơi khác” | Đã sửa (Dialogs › offline/locked, Login › expired) |
| A7 | Workspace khi chưa chọn truyện | Route `#/` → màn Dự án; nav “Workspace” disabled tới khi có truyện | Quy định trong spec |
| A8 | Tạm dừng vs dừng hẳn | `Esc`/nút Tạm dừng → `POST /abort`; Tiếp tục → `POST /resume`; gõ bất kỳ khi đang dừng → `POST /input` (action `continue`) | Quy định trong spec |
| A9 | Nghiệm thu từng chương | Trạng thái `advanceMode=review` + đang chờ: thẻ vàng trên cột giữa + nút `/next` | Đã sửa (Workspace tweak `gate`) |
| A10 | Truyện hoàn thành | Không có agent chạy: cột giữa vẫn hiện nhật ký; luồng viết hiện trạng thái rỗng; nút `/export` + `/reopen` | Đã sửa (Workspace tweak `done`) |
| A11 | `/import`, `/cocreate`, `/sync`, `/diag`, `/simulate` | Thiết kế sẵn (Dialogs › import; NewStory › cocreate), API là GAP | Backlog |
| A12 | Xuất tự động theo cấu trúc `xuat-ban/` | Quy tắc trong Cài đặt › Thư viện; ma trận trạng thái theo tập ở màn Dự án › Xuất bản | Đã sửa (thiết kế); API GAP |
| A13 | `serve --lan` cho điện thoại | Mobile layout bắt buộc (`05-screens.md` › Mobile) | Quy định trong spec |

## B. Kiến trúc thông tin & bố cục

| # | Vấn đề | Quyết định | Trạng thái |
|---|---|---|---|
| B1 | Hai cột bên quá nhiều khối, bị khuất | Tổng quan → hàng chip đầu cột giữa; trái chỉ còn Đại cương; phải = panel 3 tab (Agent · Chi phí · Truyện) | Đã sửa |
| B2 | Trang cuộn làm mất ô chat | Shell `100vh`, panel cuộn nội bộ, ô nhập neo đáy | Đã sửa |
| B3 | Nhật ký và luồng viết tranh chiều cao | Tỉ lệ 4:6 như TUI; mỗi khung tự cuộn và bám đáy (`flex-direction: column-reverse`) | Đã sửa |
| B4 | Màn hình hẹp hơn 1440 | Hai cột bên có nút ẩn/hiện; dưới 1100px xếp chồng | Đã sửa |
| B5 | Phím tắt `[` `]` ẩn/hiện panel mới ghi chú, chưa hoạt động | Implement trong app (không phải trong artboard) | Quy định trong spec |
| B6 | Tab mặc định panel phải | Đang chạy → Agent; Hoàn thành → Truyện; có chấm báo ở tab Agent khi agent chạy mà đang xem tab khác | Đã sửa |

## C. Nhất quán hình ảnh

| # | Vấn đề | Quyết định |
|---|---|---|
| C1 | Nhiều kiểu nút | Chỉ 3 kiểu: `primary` (nền #EDEDEF chữ đen), `ghost` (viền mảnh), `icon` (32×32). Không nút gradient |
| C2 | Trạng thái màu | ok/warn/err/info cố định trong token; agent dùng màu riêng, không trộn với trạng thái |
| C3 | Icon | Chỉ Iconsax. Linear cho thao tác thường; Bulk cho mục đang chọn & trạng thái quan trọng |
| C4 | Số liệu | Luôn `font-variant-numeric: tabular-nums`; token/chi phí dùng định dạng ngắn `3.6M`, `76.1k`, `$2.61` |
| C5 | Font đọc | Literata chỉ ở Trình đọc & cài đặt xem trước; Be Vietnam Pro cho UI; JetBrains Mono cho lệnh, tên tool, đường dẫn |

## D. Khả năng tiếp cận

- Mọi điều khiển là `<button>`/`<a>`/`<input>` thật; nhóm lựa chọn dùng `role="radiogroup"`/`role="tablist"`; công tắc `role="switch"`.
- Focus ring `2px #C9A6F5`, offset 2px. Không bỏ outline.
- Tương phản: chữ phụ thấp nhất `#7E7E8A` trên nền ≤ `#171719` (≥4.5:1). Không dùng `#7E7E8A` trên `#1D2130`+.
- Màu trạng thái luôn đi kèm icon/chữ (không chỉ dựa vào màu).
- `prefers-reduced-motion`: tắt mọi animation (orb thở, viền chạy, shimmer).
- Vùng chạm ≥ 32px desktop, ≥ 44px mobile.
- Nhật ký & luồng viết: `aria-live="polite"` ở mức tóm tắt (không đọc từng token).

## E. Hiệu năng

- Luồng viết: gom `delta` theo `requestAnimationFrame`, không re-render mỗi ký tự; giới hạn 32 round gần nhất (như TUI).
- Nhật ký: giữ tối đa 1000 sự kiện (client hiện tại đã `slice(-1000)`), virtualize khi > 200 dòng.
- Snapshot đến mỗi 1–5s: so sánh nông trước khi set state.

## F. Những gì còn chưa chắc (cần anh xác nhận khi implement)

1. Tên trường thật của `project.json` / `story.json` (spec đề xuất ở `06-data-contract.md`).
2. Thư mục con theo tập `xuat-ban/tap-01/…` là đề xuất; có thể để phẳng bằng mẫu tên tệp.
3. Audio là công cụ ngoài, chỉ đặt kết quả vào `xuat-ban/audio/`.
4. `gpt-6-luna` xuất hiện trong usage nhưng không có trong config mẫu — UI chỉ hiển thị theo dữ liệu `cachePerModel`.
