# V38 · Khép tweak Kênh/Tạo kênh/Chuẩn bị máy

Parent thực thi controller trên native Viderse localhost:8043, ngày05/11/2026. Base source `acbc8c5192365520f56447756b2b2c868ffd95e3`; các sửa bổ sung chưa commit lúc chạy, fingerprint trong `source-fingerprints.json`.

- 44/44 ca component UI qua;105 liên kết ảnh ứng với51reference ID. Hai viewport1440×900 và390×844 theo từng artboard.
- 0page error,0mutation tiếp tục tới backend,0request ngoài origin tiếp tục,0inference,0download. Browser đã đóng.
- Thử lại Setup: lỗi503 hiển thị feedback đã dịch; thử lại thành công cập nhật snapshot ngay. Không cài/tải công cụ thật.
- Mobile Source: ActionSheet thoát xong mới mở xác nhận; tối đa một lớp modal nhìn thấy. Huỷ không gửi bỏ liên kết.
- `attempt-1/results.json` giữ41qua/3fail trước sửa: hai lỗi chồng sheet thật và một locator caption sai. Không xoá lịch sử lỗi.

## Giới hạn bằng chứng

Các trạng thái khó dựng dùng response fixture gắn nhãn SYNTHETIC ngay trên ảnh; owner/template ID lấy qua GET native. Đây là React thật trong binary native với response cục bộ, **không phải** chứng minh tạo kênh, lưu cấu hình, unlink, provider hay persistence thành công. `results.json` ghi provenance từng ca và mọi request. Chưa thay nghiệm thu mọi control, pixel parity, Safari thiết bị thật hoặc gate deploy. Dùng cùng các bằng chứng native persistence đã có, không lấy fixture thay chúng.
