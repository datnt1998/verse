# Ảnh bề mặt cảnh báo Kiểm định

Parent mở binary native đã rebuild ở 1440×900 và 390×844. Cảnh báo đọc được và nằm trong viewport, trang không tràn ngang, không request ghi. Chrome đóng sau lượt kiểm.

Đây là **response-override fixture ở trình duyệt**: dùng ảnh QA có sẵn và thêm trạng thái scoreIssue vào GET, không phải kết quả một lượt model thật hay một job được tạo mới. Không dùng bằng chứng này để khẳng định backend/provider đã trả trạng thái đó. Bốn nhóm regression riêng kiểm decoder, lưu checkpoint, dispatcher và PostgreSQL snapshot; hai regression UI lịch sử đã fail trước sửa và qua sau sửa, bao gồm chuyển sang chế độ so sánh.

Ảnh màu phẳng là media QA dựng sẵn, không phải ảnh do provider AI vẽ. Summary nền trong response không được tính lại bởi fixture; không dùng ảnh này nghiệm thu cổng tổng hợp điểm hoặc parity nội dung với artboard.
