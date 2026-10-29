---
status: accepted
---

# Não truyện: JSON của engine là dữ liệu gốc, không chạy agentmemory, mã thực thể có phạm vi từ đầu

Não truyện (`docs/prd/nao-truyen.md`) chốt ba điều:

- **Dữ liệu gốc vẫn là các tệp JSON do engine ghi** trong `output/novel/`. Kho ghi chú kiểu Obsidian chỉ là bản sinh ra một chiều. Chỉ mục tìm kiếm và vector là dữ liệu phụ, xoá đi dựng lại được.
- **Không chạy agentmemory hay iii-engine.** Chỉ lấy lại mẫu thiết kế của nó và tự làm trong Go.
- **Từ đợt đầu, mọi dữ kiện mới đều mang mã thực thể ổn định có phạm vi**, kèm `scope` và `source`. Trong mỗi truyện lưu mã cục bộ như `nv:ky-duong`, cấp một lần và giữ nguyên khi đổi tên nhân vật. Mã đầy đủ `<cấp>:<chủ>:<loại>:<slug>` được tính từ vị trí hiện tại khi cần, không ghi vào tệp dữ kiện, để chuyển hoặc đổi tên thư mục truyện không phá liên kết. Mọi đường đọc lọc theo quyền truyện của tài khoản.

**Lý do:**

- Engine và các công cụ của agent đã ghi dữ liệu có cấu trúc. Nguyên tắc bất biến số 6 cũng giữ `output/novel/` cho engine.
- Nếu Markdown là gốc, sẽ phải có bộ phân tích chặt, và phải xử lý khi người dùng sửa trong Obsidian cùng lúc engine đang ghi.
- agentmemory cần Node ≥ 20 cộng iii-engine khoá phiên bản, và mở 4 cổng. Nó phá mô hình một tệp chạy Go `serve`.
- Embedding mặc định của agentmemory (`all-MiniLM-L6-v2`) thiên tiếng Anh. Cơ chế quên dần theo thời gian của nó cũng sai với truyện, vì chi tiết đã cài thì không được phai.
- Mã thực thể khoá theo tên (như `pairKey` hiện nay) sẽ va nhau khi não mở rộng lên cấp dự án và hệ thống. Đổi sau khi dữ liệu đã ghi ra đĩa thì phải chuyển đổi mọi truyện.

## Considered Options

- **Markdown/Obsidian là gốc, sửa hai chiều.** Bị loại vì rủi ro ghi trùng và phải viết bộ phân tích. Có thể xét lại khi đã có cơ chế khoá cho việc sửa ngoài app.
- **Nhúng agentmemory làm dịch vụ bên cạnh.** Bị loại vì lý do runtime và ngôn ngữ ở trên.
- **Hoãn khung cấp hệ thống tới khi làm tính năng.** Bị loại vì phải chuyển đổi dữ liệu.
