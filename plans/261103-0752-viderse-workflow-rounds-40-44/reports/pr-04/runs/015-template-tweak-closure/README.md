# V38 · Thư viện/Template: khép trạng thái tương tác

Parent thực thi controller trên native QA localhost:8043 ở source `7daef5fc7a453c9f277ba7e895c76f7100379222`, ngày05/11/2026.

- Controller chính:58/58ca qua,0page error,0mutation/network ngoài origin tiếp tục,0download. Browser đóng trong finally.
- `extra-states/results.json`:11/11ca bổ sung qua; import done/partial, danh mục nguồn và unsupported có nhãn fixture. Unsupported POST parse chỉ được trả lời cục bộ, không gửi URL ra ngoài hoặc tạo nguồn backend. Ghi chú chuẩn bị của task giữ nguyên lịch sử10ca trước khi parent bổ sung ca này.
- Lỗi catalogue thật đã sửa:503 không giả thành nguồn bị xoá, retry phục hồi giữ recipe và không PATCH. Nguồn/lớp phủ thực sự mất chỉ hiện lời văn người dùng, không lộ ID.
- Menu đặt mặc định đi đúng kênh native; menu mobile dùng accessible name thực, không locator đoán. Sửa harness `.vbd-subnav` thành rail thực `nav.vbd-rail`; không sửa source để làm locator qua.
- Ready cover history dùng ảnh native đã có, decode được, không gọi sinh ảnh. Discovery/provenance nằm trong `results.json`; envelope có thể là fixture và không được gọi là trial mới/persistence.
- `attempt-1/results.json` giữ46qua/10fail/2chưa chứng minh trước sửa. Bốn failure là lỗi product catalogue/copy; sáu là locator/helper. Lượt tiếp theo57qua/1fail do selector rail sai, sau sửa58qua. Không bỏ assertion product.

## Giới hạn

Đây là component UI thật chạy trong native binary, có GET/PATCH response fixture có nhãn và provenance; không chứng minh import ZIP thật, download nguồn, provider inference, preview rendered/playback hoặc lưu các PATCH mô phỏng. Ca native thật được phân loại riêng. Không thay pixel parity, control ledger đầy đủ, Safari thiết bị thật hay release gate. Không làm viewer mobile đợt45.
