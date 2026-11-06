# Bàn dựng kênh — kiểm tra bản đang phát hành

- URL người dùng: `https://vid.aarverse.studio/#/?channel=aa72b6a1-7114-4a46-9ed5-539216fb81fb` (Billionaire Battles).
- Bản production sau vá có phạm vi hẹp: `520d4ba3946ad458bbfa1b023a31f9087c1b2d3e`. **Không phải phát hành toàn bộ giao diện workflow gọn.**
- Chuẩn: [Workflow](../../../../apps/viderse/docs/design/v1/artboards/Workflow.dc.html), [Main](../../../../apps/viderse/docs/design/v1/artboards/Main.dc.html), V37/V40 trong [dev prompt](../../../../apps/viderse/docs/design/v1/08-dev-prompt.md).
- Phương pháp: đối chiếu source chính xác của binary với thiết kế, chạy compiled UI với dữ liệu QA có chủ đích ở1440×900/1024×900/390×844. Không có phiên browser production đăng nhập hoặc Safari iPhone thật. Không gọi phần này là so sánh ảnh production1:1 hay nghiệm thu đủ tweak.

| Vùng | Bản đang chạy | Theo thiết kế / việc còn thiếu |
|---|---|---|
| Cột danh sách | Vẫn có Bản tin và nút Đề xuất | Bỏ Bản tin; ba loại đề xuất thành dòng trong Chờ bạn, không giữ dock/thẻ riêng |
| Trung tâm | Còn ô Nhắn trợ lý, dải agent và bố cục thao tác cũ | Hoàn tất Bàn dựng gọn V37; không coi việc thêm shimmer là đã đổi workflow |
| Nhật ký tiến độ | Vị trí/cấu trúc còn theo luồng cũ | Khớp khối Tiến độ, điều hướng và bố cục của Main; kiểm đủ tải nguồn, ảnh, dựng, lỗi, tạm dừng |
| Cột phải | Cấu trúc inspector cũ | Chỉ hai tab Nội dung đăng / Thông tin theo thiết kế; không thêm tab để giữ chức năng đã bỏ |
| Cuối danh sách | Còn template luân phiên / Tự lấy theo mô hình cũ | Một template mặc định quản lý ở Kênh; chỉ đổi nhãn không đủ, cần cutover backend V38 đúng nguồn dữ liệu |
| Điện thoại | Còn thẻ Bản tin và Đề xuất cũ | V40 đủ cùng chức năng, nhãn và mặc định với PC; không nghiệm thu toàn bộ dựa trên hai bản vá avatar/chữ chạy |
| Avatar | Trước vá hàng thiếu nguồn ảnh hoặc không có chỗ riêng | Đã nối ChannelSummary.avatarUrl vào avatar tròn; mobile có lớp avatar không bị thumbnail cắt. Ảnh production chưa được xác minh qua browser có đăng nhập |
| Chữ đang chạy | Có chỗ không dùng Shimmer hoặc chạy cả khi không hoạt động | Vá hẹp dùng Shimmer của kit cho trạng thái running thật, reduced motion giữ chữ đọc được; trạng thái paused không có shimmer |

## Bằng chứng và giới hạn

- [Receipt, hình và kết quả tương tác](template-channel-hotfix/receipt.json): avatar/trạng thái đang tải nguồn; normal motion `kit-shim`2.4s, reduced motion tắt, paused không shimmer; không tràn ngang390px.
- Explore: nút Làm cho… đã được giới hạn co/ellipsis, icon/copy không bị đẩy; aria-label giữ đầy đủ tên. Bấm copy nhận đúng link. Đây không phải nghiệm thu toàn bộ Explore.
- Mẫu Joker: lỗi cấu hình ảnh không hoạt động đã được tái hiện và sửa riêng; mở được/lưu được không có nghĩa phần Xuất & kênh đã khớp thiết kế. Xem [audit Template](template-output-audit.md).
- Source chính/PR7 có thay đổi workflow mới nhưng **source có không đồng nghĩa production đã phát hành**. Gate Safari iPhone#93 vẫn KEEP theo quyết định người dùng; không tự gỡ hoặc tuyên bố fullV40 đã deploy.

## Điều kiện nghiệm thu lần tiếp theo

Mỗi phần phải có source/binary đang chạy xác định rõ; ảnh code và artboard cùng trạng thái ở1440×900 và390×844; mọi nút/tab/switch có phản hồi; kiểm normal/reduced motion, chạy/tạm dừng/lỗi/xong; dữ liệu backend của template mặc định và đề xuất đã cutover cùng UI. Không dùng bản vá hẹp này thay cho nghiệm thu đó.
