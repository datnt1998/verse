# Native — chỉ dẫn theo kênh

Trạng thái: qua luồng lưu/đọc lại, chưa nghiệm thu toàn V38.

- Native binary build từ source hiện hành bằng SQLC model overlay riêng; không ghi đè model người dùng.
- 1440×900 và 390×844: bốn người nhận Trinh sát, Đạo diễn, Hoạ sĩ, Kiểm định. Tám lượt lưu trả HTTP 200, GET xác nhận merge giữ nguyên người nhận khác, reload nạp lại đúng lời dặn.
- Điện thoại vào Hồ sơ → Dặn trợ lý về kênh; chỉ một Sheet hiện ở từng lần chọn người nhận. Lưu đóng Sheet; mở lại và Esc đóng được.
- Chặn request ghi ngoài PATCH chỉ dẫn được phép; không có request ghi ngoài danh sách. Lời dặn ban đầu được khôi phục sau mỗi viewport, từ chối ghi đè nếu phát hiện thay đổi đồng thời.
- DB QA trước/sau giữ số lượng 3 job, 27 step, 2 biên nhận ảnh. Worker guard vẫn giữ; không gọi AI hay vẽ ảnh.
- Auth nguồn và model source được bảo vệ giữ nguyên fingerprint.

Bằng chứng: `native-instruction-save-smoke.json`, `native-instruction-state-preserved.json` và hai ảnh `native-*-channel-instructions-readback.png`.

Không suy rộng kết quả này thành mọi tweak, mọi điều khiển, motion, độ lệch viên trượt, parity ảnh artboard hoặc Safari thật đã qua. Hai task phụ đang chuẩn bị kiểm sâu và regression; parent chạy, tích hợp và nghiệm thu riêng.
