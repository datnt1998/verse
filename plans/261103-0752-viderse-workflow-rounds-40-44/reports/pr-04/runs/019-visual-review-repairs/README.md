# V38 · Sửa theo review đối chiếu artboard

Parent tích hợp proposal task, không có child ghi source/worktree. Source sửa bố cục `ac3d40c7bbb835209921cee9332744ff8cffacaa` và loại video `4292b33ff3ba248c1e49310777fc7d7c0a3dd3ba` đã push trên PR#5.

## Source đã sửa

- Template PC: header/hint xếp dọc, heading18px theo token; header spacing, main content cap700px và aside372px theo artboard. Bỏ card thừa quanh khối Xem thử; vẫn dùng dữ liệu/placeholder native, không vẽ nội dung giả.
- Output: nút Biên tập đi cùng hàng tiêu đề Bố cục trên PC; mobile không mở Editor, giữ hướng dẫn dùng máy tính theo IA§6.2 và MTemplate. Kích thước lựa chọn nén theo token.
- Import thành công: hiển thị tên mẫu video server trả về, không dùng tên ZIP làm tên mẫu; tệp lỗi vẫn giữ tên tệp để nhận biết.
- Kênh PC: template card nằm ngang, giữa theo trục, giới hạn860px bằng semantic token; mobile card riêng không bị đổi.
- Tạo kênh mobile: một row-button borderless có label, tên mẫu/description thực, chevron; radio action-menu thay combobox cố định44px bị tràn. Desktop select giữ nguyên.
- Nhận diện: caption banner và điều khiển safe-area nằm trên lớp generation, không bị che mất.
- Bỏ liên kết mobile: Sheet gọn, nội dung đầy bề ngang, hai action cân nhau; giữ callback mutation/invalidate/toast dùng chung với confirm PC. Khoá dismiss/action khi đang gửi, lỗi giữ Sheet cho thử lại và không lộ lỗi thô. Giữ handoff sau ActionSheet onExited.
- Menu đối thủ mobile: phục hồi hướng dẫn phụ và icon đúng nghĩa.

## Bằng chứng hiện có

- Editor boundary: regression phone fail trước/desktop qua; sửa xong cả hai qua. Native fail-before receipt trong `editor-fail-before/results.json`; native sau sửa2/2viewport qua,0page error/request ghi.
- Tên import: regression fail trước vì chỉ thấy a.zip; sau sửa qua cùng ca partial/error-identification.
- Mobile unlink: test cancel không DELETE; pending khoá nút/Esc;503 giữ dialog và ẩn raw marker; retry thành công đóng. Một lần fixture trả nhầm RivalList cho scan đã fail, sửa fixture exact-path, test qua; không chỉnh product để chiều fixture.
- Typecheck/Vite/native build qua. Full frontend cuối441file/3667test qua,4skip; Library42test qua. Cảnh báo cũ empty image src, Vite annotation/dynamic-static import/chunk size vẫn ghi nhận.
- Controller ghi41qua/3fail do mong combobox trong khi row-button mobile dùng radio menu; giữ receipt `create-role-transition-before-harness/results.json`, sửa oracle theo semantics thật. Native sau sửa oracle Kênh44/44, Template58/58, extra11/11qua; source-gallery mobile thêm fail-before/qua-sau2viewport. Không lấy source test xanh thay ảnh UI.
- Kind picker có2regression fail trước/qua sau: nguồn mới không chọn loại rỗng, nguồn cũ giữ nhưng có thể bỏ chọn; disabled/aria-disabled nhất quán. [4context picker và Setup đúng trạng thái](../022-matched-contexts/README.md) đã chạy, không force-click nút không khả dụng.

## Giới hạn và nghiệm thu cuối

124reference ID hợp lệ/93vector duy nhất, không phải124pixel-parity pass. Các ảnh sai underlay, failed/running và stage mặc định đã được tách; bộ [canonical byte/font](../020-authenticated-canonical-reference/README.md), [bề mặt hiện tại](../021-delivery-surfaces/README.md), [context ghép đúng](../022-matched-contexts/README.md) là evidence bàn giao. Picker chụp sau finite animation; lớp đã opaque nhưng mobile vẫn sai Dialog, nên parent sửa Sheet và giữ fail trước/qua sau. Không đổi artboard, không sinh ảnh/download/inference mới. PR5chưa merge/deploy; V39/V40, Safari vật lý/release#83 chưa hoàn tất. Mobile final layout/tweak/motion thuộc V40 theo phase05§7, không bỏ contract mobile sang PR6.
