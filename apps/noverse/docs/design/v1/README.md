# Noverse Web — Design v1 (handoff cho Claude Code)

Gói này là bản thiết kế chính thức thay cho khung `web/src/skeleton/*` (khung test, sẽ được thay toàn bộ). Mục tiêu: dựng giao diện **Agentic Studio** cho `ainovel-cli serve` bằng stack đang có trong `web/` — React 19 + Vite + TypeScript + Tailwind CSS v4 — giao tiếp qua `/api/*` và SSE như `web/src/api/` hiện tại.

Canvas thiết kế gốc (xem tương tác, bấm Play): Claude Design artifact “Noverse Agentic Studio”. Bản sao mã nguồn từng màn nằm trong `artboards/` để tra cứu giá trị chính xác.

## Đọc theo thứ tự

| Tệp | Nội dung |
|---|---|
| `01-review.md` | Review UX/user flow, các lỗi đã sửa, quyết định thiết kế |
| `02-ia-flows.md` | Kiến trúc thông tin, route, các user flow từng bước kèm API |
| `03-tokens.md` + `tokens.css` | Màu, chữ, bo góc, khoảng cách, chuyển động — dán thẳng vào Tailwind v4 |
| `04-components.md` | Danh sách component, props, trạng thái, quy tắc a11y |
| `05-screens.md` | Bố cục từng màn, trạng thái, dữ liệu gắn với `Snapshot` |
| `06-data-contract.md` | Ánh xạ API/SSE → UI, parser luồng viết, bảng dịch nhãn, **các API còn thiếu** |
| `07-copy.md` | Toàn bộ chuỗi tiếng Việt dùng trong UI |
| `08-plan.md` | Kế hoạch implement theo pha + tiêu chí nghiệm thu |
| `09-auth-onboard.md` | Đăng nhập nhiều tài khoản, phiên 30 ngày, QR điện thoại, quản lý tài khoản, thiết lập lần đầu, trang xác thực OAuth nhà cung cấp |
| `10-audio.md` | **Audio (v1.1)**: bản đọc cho giọng đọc AI, phân vai nhiều giọng, tạo audio trên máy, nghe & đọc lại đoạn, ghép & xuất tập |
| `11-api-contract.md` | **Hợp đồng API đã chốt** cho mọi GAP (thư viện 3 cấp, đọc truyện, xuất, cấu hình, OAuth, lệnh live, tài khoản) |
| `12-implementation-plan.md` | Kế hoạch thi công: lát cắt, sở hữu tệp, thứ tự, nghiệm thu 1:1 |
| `13-commands.md` | **Lệnh**: popover gợi ý khi gõ `/` (mô tả, cú pháp, gợi ý tham số, kiểm tra lỗi trực tiếp) và trang Hướng dẫn lệnh chi tiết |
| `14-jobs-dock.md` | **Việc chạy nền**: dock góc dưới phải (ẩn · chấm · pill · bảng, morph & toast) + thứ hiện ra sau khi chọn từng lệnh |
| `15-audio-impl.md` | **Triển khai giọng đọc**: lớp `internal/voice` dùng chung (engine VieNeu-TTS qua API tương thích OpenAI, bản cài quản lý sẵn), dùng lại được cho mọi module |
| `16-feedback-refresh.md` | **Phản hồi thao tác & làm mới dữ liệu**: tầng dữ liệu `web/src/data`, luồng sự kiện chung `/api/events`, phiên bản + `If-Match`/409, bảng phản hồi cho từng hành động |
| `17-kit.md` | **Bộ thành phần**: 7 hộp xác nhận (thường · cảnh báo · đỏ · gõ để xác nhận · giữ để xác nhận), menu tài khoản, menu ⋯ có menu con, Select · Combobox · chọn nhiều, ô nhập, nút, tooltip, banner |
| `18-mobile.md` | **Bản điện thoại & máy tính bảng** (trang “Điện thoại” trên canvas): thanh tab nổi 4 tab, bottom sheet thay popover/hộp thoại, trạng thái mất mạng · hết phiên · bàn phím, kết quả lệnh, nhật ký, thông báo đẩy & liên kết sâu, đổi tài khoản/mật khẩu, hướng dẫn lệnh, máy tính bảng hai cột |
| `19-gap-closure.md` | **Bổ sung khoảng trống** G1–G13 (API còn thiếu so với artboard) + chính sách lõi tiếng Việt (§B) |
| `20-settings-rules.md` | **Tuỳ chỉnh & Quy tắc viết**: chỉnh theo 3 cấp (Truyện · Dự án · Mọi truyện), quy tắc viết bằng lời thường kèm “Noverse hiểu là…”, luật học từ chat, áp dụng từ chương sau; màn Dự án không còn cây thư mục; bảng đổi tên công cụ/mã kiểm tra sang lời thường |
| `21-dev-prompt.md` | **Prompt cho Dev**: danh sách việc đợt 24–25/10 (gói A–G, GAP-30…33, nghiệm thu) — dán cho Claude Code để bổ sung đầy đủ |
| `22-brief-2510.md` | **Trả lời brief design 25/10**: D1–D12 (phiên bản, gói thể loại, bản đọc TTS, thẻ thẩm duyệt có §, bản đồ giữ chân, số đo bằng máy, trang nhân vật & xưng hô, Não truyện, audio theo gói), trả lời câu hỏi treo §E, gói L cho dev; bổ sung lần chạy đầu trên web (A0), Não truyện điện thoại (M8), Chất lượng truyện, Não dự án, Não hệ thống (D13–D15) cho gói M |
| `23-brand.md` + `brand/` | **Thương hiệu Noverse**: biểu tượng 4 quả cầu, logo dài và đứng, app icon, favicon, màu, chữ (Plus Jakarta Sans), cách gắn vào web, phạm vi đổi tên |
| `icons/` | 142 icon Iconsax (Linear + Bulk, MIT) dạng SVG + `index.json` |
| `artboards/` | Mã nguồn `.dc.html` của 41 artboard + `canvas.json` (tham chiếu, không chạy trực tiếp) |

## Nguyên tắc bất biến

1. **Một màn hình, chat luôn neo.** Desktop (≥1100px): trang không cuộn; chỉ các panel cuộn bên trong. Ô nhập can thiệp luôn ở đáy cột giữa. Điện thoại (≤640px): **bố cục riêng** theo `18-mobile.md` (4 tab, bottom sheet), không xếp chồng cột của bản máy tính. Máy tính bảng (641–1099px): Viết + Đọc hai cột, thanh tab nổi giữa thanh trên.
2. **Liquid dark, gradient tiết chế.** Nền đen trung tính, card có vệt sáng mép trên. Gradient **chỉ** xuất hiện ở: orb của 4 agent, thanh tiến độ WRITER, viền chạy quanh ô nhập khi agent đang chạy, vệt chạy trên icon “Đang suy nghĩ”. Không dùng gradient cho nền, chữ hay nút.
3. **Agent có danh tính màu cố định**: ARCHITECT xanh dương, WRITER tím–hồng, EDITOR cam đào, ARBITER xanh mint (`03-tokens.md`).
4. **Trung thực dữ liệu.** Không bịa số. Trường chưa có dữ liệu hiển thị `—` hoặc trạng thái rỗng, không hiện giá trị mẫu. Các chỗ `[ngoặc vuông]` trong artboard là placeholder thiết kế, **không** đưa vào sản phẩm.
5. **Tiếng Việt thống nhất.** Host phát tiếng Việt trực tiếp (19-gap-closure §B); sự kiện/snapshot tiếng Trung cũ (`用户干预裁定`, `✻ 打磨`…) UI dịch theo bảng tương thích trong `06-data-contract.md`.
6. **Không phá engine.** `output/novel/` do engine quản lý, UI chỉ đọc. Mọi hành động đi qua API.
7. **Không lộ kỹ thuật.** Người dùng thấy Ý tưởng · Tư liệu · Truyện · Bản xuất, tên dự án/truyện, lời thường. Đường dẫn, tên tệp, khoá cấu hình, tên công cụ, mã lỗi chỉ ở ngăn **Nâng cao** (quản trị) — `20-settings-rules.md` §1, §5.

## Phạm vi v1

- **Làm được ngay với API hiện có**: Đăng nhập nhiều tài khoản + QR điện thoại + thiết bị + quản lý tài khoản (`platform/identity`), Workspace (live), Nhật ký tiến độ, Trình đọc + tải xuất TXT/EPUB, Tạo truyện (Bắt đầu nhanh), pause/resume/next/review/reopen, trạng thái kết nối & 401 & 409.
- **Cần backend bổ sung** (đánh dấu `GAP` trong spec): tầng Thư viện → Dự án → Truyện → Tập, `project.json`/`story.json`, xuất tự động vào `xuat-ban/`, đọc/ghi cấu hình, cờ thiết lập lần đầu & mật khẩu tạm, đăng nhập provider từ web, chi tiết thẩm duyệt, kế hoạch/hợp đồng chương, sổ phục bút, Đồng sáng tác, `/import`, `/sync`, `/diag`.
- **v1 · Lệnh** (GAP-22…25, `13-commands.md`): popover `/` và trang Hướng dẫn lệnh.
- **v1 · Việc chạy nền** (GAP-26…29, `14-jobs-dock.md`): dock tiến độ ở mọi màn, bảng `/model`, `/diag`, thẻ kết quả `/export`, `/sync`, `/simulate`.
- **v1 · Điện thoại & máy tính bảng** (GAP-30…31, `18-mobile.md`): thanh tab nổi, mất mạng/hết phiên/bàn phím, kết quả lệnh, nhật ký, thông báo đẩy & liên kết sâu, máy tính bảng hai cột.
- **v1 · Tuỳ chỉnh & Quy tắc viết** (GAP-32, `20-settings-rules.md`): 3 cấp áp dụng, quy tắc bằng lời thường, áp dụng giữa chừng, kiểm tra vi phạm.
- **v1.1 · Audio** (GAP-17…21, `10-audio.md`): bản đọc, phân vai, bộ đọc trên máy, tạo/nghe/xuất audio theo tập.
- **v1 · Não truyện W1–W2** (`22-brief-2510.md` D7–D10): trang nhân vật có bảng xưng hô, sửa xưng hô/quan hệ (khoá khi đang viết, xác nhận khi chạm chương đã viết), vi phạm xưng hô trong Nhật ký, màn Não truyện máy tính (`Brain.dc.html`). Điện thoại chỉ danh sách (`MBrain`).
- **v1 · Gói M** (`22-brief-2510.md` › Bổ sung 25/10, GAP-41…46): tạo quản trị đầu tiên trên web bằng mã thiết lập (`Setup`), Não truyện điện thoại (`MBrain`), Chất lượng truyện (`Quality`), Não dự án (`ProjBrain`), Não hệ thống (`SysBrain`, chỉ quản trị).
