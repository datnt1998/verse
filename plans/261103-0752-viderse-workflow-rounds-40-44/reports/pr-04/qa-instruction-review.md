# Kiểm định theo lời dặn — rà soát độc lập

Hai task cùng họ model với controller; không tuyên bố độc lập khác họ model. Source do parent tích hợp, task chỉ đọc và bàn giao artifact riêng.

## Đã thực thi

- Bốn nhóm regression mới qua: decoder/applicability, giữ ảnh/biên nhận sau chấm lỗi, dispatcher tự duyệt thật với manifest có scoreIssue + điểm 100, snapshot chỉ dẫn QA qua sửa/xoá kênh trên PostgreSQL riêng.
- Parent bỏ các dòng unsupported lặp cùng đường và ma trận 85/90 đã có coverage, không thêm test lời văn hay canary vào suite chỉ để tạo màu xanh.
- Full backend suite qua trước phần sửa lỗi xem phiên bản lịch sử; vet được chạy lại sau DTO cuối và qua. Sau sửa, package projection ảnh và 65 test liên quan Review/imageView qua; kiểm bổ sung cả so sánh ở PC/mobile qua. [Ảnh cảnh báo ở PC/mobile](runs/006-qa-warning-visual/README.md) đã được parent xem, là fixture response override và không thay bằng chứng model thật.

## Phát hiện đã sửa

Xem phiên bản ảnh cũ đổi ảnh/prompt nhưng cột điểm/cảnh báo vẫn lấy ở cấp item của bản candidate/current. Hai regression UI PC/mobile đều fail trước sửa, qua sau sửa. API hiện trả score/issue ở từng phiên bản; FE lấy đúng bản đang xem/so sánh, không mượn điểm đạt từ bản khác. Tóm tắt video và cổng tự duyệt vẫn theo candidate/current, không theo bản người dùng tạm xem. Giữ đầy đủ lịch sử ảnh và quyền duyệt thủ công.

## Giới hạn model

Tính áp dụng của chỉ dẫn ngôn ngữ tự nhiên do model kiểm ảnh diễn giải. Schema và nhánh fail-closed chặn false, issue chưa giải quyết, thiếu/malformed hoặc lỗi chấm; không chứng minh model luôn hiểu đúng ngữ nghĩa. Canary giả model nói dối true/điểm100 là giới hạn chất lượng model, không phải bằng chứng đã có một lượt model thật phân loại sai. Không dùng keyword denylist hoặc âm thầm đổi mọi lời dặn thành duyệt thủ công để né vấn đề. Chưa chạy model thật vì QA không được tiêu thêm hạn mức; không tuyên bố đã kiểm âm thanh hay bảo đảm tuyệt đối về ngữ nghĩa.
