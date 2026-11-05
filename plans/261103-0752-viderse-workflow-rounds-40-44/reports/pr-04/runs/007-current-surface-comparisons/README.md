# V38 — 24 cặp ảnh artboard / ứng dụng thật

Mỗi PNG: **artboard bên trái, native QA bên phải**. Có 12 cặp ở1440×900 và12 cặp ở390×844: ba tab Kênh, bốn tab Thư viện, Tạo kênh và bốn mục Template. `manifest.json` ghi nguồn tham chiếu, ảnh ứng dụng và hash source.

Native runner đã chụp36màn (gồm ba kênh), không tràn ngang, không page error, không mutation. Bộ ảnh này được chụp trước bản sửa cuối của trạng thái lỗi tải chi tiết Template; không dùng chúng làm bằng chứng cho nút thử lại mới.

## Giới hạn

- Dữ liệu native khác ví dụ artboard: kênh chưa liên kết đối thủ/ảnh nhận diện, số lượng video và tên mẫu khác. Không biến khác biệt dữ liệu thành tuyên bố khớp từng pixel hoặc tự gọi AI để điền ảnh.
- Có ảnh mẫu thật thì renderer lấy đúng chỉ dẫn đang gắn. Thiếu/tải lỗi dùng hình chờ; người dùng đã chốt phương án A tại câu hỏi#90.
- Bộ ảnh không thay thế kiểm mọi tweak, hành động, bàn phím hoặc nghiệm thu Safari/iPhone thật.
- Các controller trạng thái lỗi/khôi phục tiếp tục chạy riêng; V38 chưa được nghiệm thu/merge/deploy chỉ nhờ bộ ảnh này.
