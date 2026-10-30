# Viderse — Theo dõi gói đã xong

Dev ghi mỗi gói khi xong: ngày, PR/commit, chỗ lệch so với artboard (nếu có) và lý do. Không xoá dòng cũ.

| Gói | Ngày xong | PR / commit | Lệch so với thiết kế | Ghi chú |
|---|---|---|---|---|
| V0 — Nền tảng | | | Khung điện thoại dưới 768px (hỏi đáp #16). Chữ badge Duyệt tạm dùng `--vv-badge-ink` (hỏi đáp #14). | Đang làm. Đã có: thanh trên, nav, `JobsPill`, badge Duyệt và chấm bánh răng đọc `/api/status` qua sự kiện đẩy; chấm bánh răng theo `attention` (đăng nhập hết hạn, công cụ lỗi/có bản mới, hết hạn mức, thiếu thư mục dự án) mở đúng mục Cài đặt; brand, logo, favicon; đứng yên khi giảm chuyển động. Còn: `JobsPill` theo job thật và “Đã tạm dừng” (cần hàng chờ). |
| V1 — Bàn dựng | | | | |
| V2 — Làm video | | | | |
| V3 — Duyệt | | | | |
| V4 — Kênh | | | | |
| V5 — Khám phá + Video đối thủ | | | | |
| V6 — Hàng chờ | | | | |
| V7 — Cài đặt | | | Thẻ tổng không có số luồng (#18); chưa có hàng “Giới hạn từng kênh” (#19); thẻ phiên bản không có ngày phát hành (#20); dải mất kết nối đẩy nội dung, tông chấm theo `kind`, nhãn pha cài do máy chủ trả (#30). | Đang làm. Đã có: khung Cài đặt (cột trái 3 nhóm, chấm cảnh báo theo mục) và `MSettings`; Công cụ (`tools=ok|update|failed|installing`, cài/cập nhật/tải tiếp, Nâng cao, kích thước đo sau khi cài); Nhật ký (lọc, lỗi 7 ngày, xuất log đã che bí mật, kiểm tra bản mới). QA ảnh cặp 5 vòng xong 30/10/2026, còn 0 lỗi lớn / 0 lỗi nhỏ. Còn: hàng “Giới hạn từng kênh” khi phase Kênh có route (#19). |
| V8 — Thư viện + Template | | | | |
| V9 — Lần đầu mở + Tạo kênh | | | Chuẩn bị máy là route `#/setup` sau đăng nhập (#17). | Đang làm. Đã có: bước `step=setup` (`running|login|failed`) cài thật 6 việc, tiến độ qua SSE, đăng nhập ChatGPT, kiểm tra dung lượng; điện thoại hiện “Mở trên máy tính”. Còn: Tạo kênh. |
| V10 — Điện thoại | | | | |
| V11 — Nhắn trợ lý | | | | |
| V12 — Hộp thoại + trạng thái hệ thống | | | Dải mất kết nối trong Cài đặt đẩy nội dung xuống thay vì che (#30). | Đang làm. Đã có: 7 loại `ConfirmDialog` (khoá kèm lý do, cỡ `size` cho hộp thoại rộng), dải mất kết nối (hiện ~0,1 giây sau khi tắt máy chủ, nội dung khoá `inert`, tự tắt khi nối lại), trang Không tìm thấy, Bàn dựng trống. Tắt máy chủ khi còn tab mở thoát ngay, mã 0. Còn: khoá Xoá video đang chạy trên menu ⋯ (cần Bàn dựng có job). |
| V13 — Editor theo Viderse | | | | | <!-- r2-gap -->
| V14 — Việc nền & thông báo | | | | |
| V17 — Kết nối AI và Trợ lý AI | | | Dữ liệu artboard chưa có nguồn (#26); thứ tự thẻ, thẻ trùng loại (#27); giá mỗi video = 7 ảnh, lưới 4 nguồn vẽ (#28). | Đang làm. Đã có: §15 đủ API; ChatGPT (một credential trong `core/auth`), Grok mã thiết bị, khoá API niêm phong AES-GCM, Ollama; danh mục model phân trang, thêm model tay; Đạo diễn/Hoạ sĩ chọn model và nguồn vẽ dự phòng; trần ảnh trả phí mỗi ngày (D7). QA ảnh cặp 5 vòng xong 30/10/2026, còn 0 lỗi lớn / 0 lỗi nhỏ. Còn: chủ máy thử đăng nhập Grok và thêm khoá API thật. |
| V22 — Thư mục lưu dự án | 30/10/2026 | `00fd26e` | Chọn thư mục bằng danh sách của máy chủ (#21); “Ổ ngoài USB” theo `diskutil` (#22); banner đếm việc nền, không đếm video (#23). | Xong. §23 (`here|pick|lowspace|cloud|moving|done|donenew|offline`), chuyển thư mục chạy nền có tiến độ và thử lại, phát hiện ổ mất/chỉ đọc, dọn theo chính sách giữ có xem trước. QA ảnh cặp 5 vòng, còn 0 lỗi lớn / 0 lỗi nhỏ. |
| V23 — Video nền và Tài khoản nền tảng | | | Mốc “từ HH:MM” và “một số video” (#25); state điện thoại chưa có artboard, giãn dòng 1,5 (#29). | Đang làm. Đã có: phần Tài khoản nền tảng (YouTube cookie từ trình duyệt hoặc tệp, Bilibili, Twitch, hồ sơ, trạng thái tạm chặn từ `platform_state`). Còn: Thư viện › Video nền. |
| V24 — Đăng nhập Bilibili bằng mã QR | | | Mở app 哔哩哔哩 trên chính điện thoại (#6). | Đang làm. Đã có: `qr=loading|waiting|scanned|expired|rejected|error|success` trên máy tính và điện thoại, API trả cả `loginUrl` (#7), poll 2 giây, tự gia hạn cookie. Còn: quét thật bằng tài khoản Bilibili. |
