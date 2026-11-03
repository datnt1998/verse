# V36 · Kết nối nhà cung cấp thật trong QA cô lập

Ngày: 04/11/2026. Chưa merge/deploy. Không gọi AI, không tải model, không dùng khoá mới.

## Dữ liệu và an toàn

- Dùng Ollama v0.40.11 đã cài trên máy, chạy ở `127.0.0.1:11434` với HOME và thư mục model QA riêng. GET `/api/tags` thực sự trả `{"models":[]}`.
- `127.0.0.1:11435` thực sự từ chối kết nối, không phải API bị mock. Không nới kiểm tra địa chỉ/SSRF.
- Mỗi ca thêm đúng một record Ollama bằng UI thật, kiểm GET/readback, rồi gỡ bằng xác nhận thật. `finally` chỉ dọn record mới thuộc đúng địa chỉ QA của ca; so lại toàn bộ ID provider ban đầu, job nguồn và cấu hình trợ lý.
- Chặn mọi browser API mutation ngoài POST tạo Ollama đúng hai địa chỉ QA và DELETE record Ollama. Auto setup-start bị chặn/ghi riêng khi phải đi qua link native Cài đặt nâng cao.
- Có receipt từng ca; không có credential/cookie/DSN trong report. Không thay production. Đã dừng các service Ollama do phiên này tạo sau lượt46 và lượt51; không để server model chạy thừa.

## Kết quả thực thi

| Lượt | Kết quả | Diễn giải |
|---|---|---|
| [45](runs/45-real-local-provider-catalog/interaction-results.json) | 4pass / 4fail | Ready/add/remove qua cả bốn project. Unreachable dừng do harness giả định form có inline alert; form thực dùng toast. Ảnh đồng thời bắt lỗi thật: thẻ local báo khoá401 và nút Đổi khoá. Cleanup đã chạy. |
| [46](runs/46-real-local-provider-catalog-fixed/interaction-results.json) | 8pass / 0fail | Ready/unreachable, PC1440×900 và phone390×844, normal/reduced. Nút trong thẻ lỗi mở đúng ô địa chỉ; model catalog rỗng phản ánh đúng máy không có model; gỡ có consequence/DELETE và readback. |
| [51](runs/51-provider-feedback-layout/interaction-results.json) | 8pass / 0fail | Thêm oracle hình học thật và đợi chuyển động hữu hạn kết thúc. Toast ở trong viewport, không đè CTA của mobile Sheet, cả normal/reduced. Ảnh lỗi sau Esc đợi Sheet unmount nên không chụp nhầm pha thoát. Hai ảnh mobile đã đọc trực tiếp: feedback nét trên Sheet, thẻ lỗi đúng Đổi địa chỉ và không bị che. |
| Focused ConnectSection | 16pass / 0fail | Có regression desktop/mobile: lỗi kết nối local sửa địa chỉ đã lưu, không đòi API key và không tự ghi khi mở sửa. Typecheck sau sửa setup responsive qua. |

## Sửa nguyên nhân

`ProviderCard` từng dùng cố định lời API-key và nút Đổi khoá cho mọi loại lỗi. Nay `unreachable` dùng lời không kết nối được; phiên subscription hết hạn dùng lời hết phiên; nút phụ lấy đúng loại kết nối. Bỏ mã401 khỏi lời API-key theo quy tắc copy, không lộ mã kỹ thuật ở UI thường. Không thêm API/backend field hoặc thay artboard.

## Giới hạn còn kiểm

- Phân tích lại video/DOM chứng minh blur lượt46 là frame vào `.5s`, **không phải toast nằm dưới scrim**. Lỗi thật là hai lớp cùng neo đáy làm pill phủ31,7px của CTA48px. Activity toast nay đo Sheet và chiều cao host để nhường vùng nút, giữ host trong viewport; dùng easing token, tắt chuyển động khi reduced. Lượt51 native8/8 đã có painted screenshot/không overlap, đóng giới hạn hình học này; không đổi thời lượng4s hoặc tăng z-index. [Chi tiết](toast-sheet-visibility.md).
- Empty catalog không chứng minh model/inference hoạt động. `conn=models/manual` với catalog12 model thật được kiểm trong harness Settings riêng khi có access QA chỉ đọc; không bịa model để lấy ảnh giống artboard.
- Ngân sách ảnh vẫn dùng đủ2/2 từ các lượt trước. Tất cả lượt ở đây thêm0 ảnh.
