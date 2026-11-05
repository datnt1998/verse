# V38 · Lưu Template thật và đọc lại

Parent chạy UI native QA source `7daef5fc7a453c9f277ba7e895c76f7100379222`, ngày05/11/2026.

Hai viewport1440×900 và390×844: đổi Nén video sau khi xuất bằng radio thật → PATCH native HTTP200 → GET đúng giá trị/revision tăng → reload UI vẫn chọn đúng. Tổng2PATCH thật, không response fixture. Lượt PC đổi sang false, mobile đổi lại true; recipe cuối có hash đúng bản ban đầu. `results.json` ghi revision/hash, không lưu body recipe/credential.

Job/step/receipt trước và sau đều3/27/2.0page error, browser đóng. Route chỉ cho PATCH đúng một template QA; mọi nonGET khác, inference/download và request ngoài origin bị chặn. Worker QA có advisory guard. Không đụng auth hoặc dữ liệu production.

Đây là bằng chứng native persistence của lựa chọn output trên cả hai bố cục; không suy thành mọi trường/template mutation đã kiểm, không chứng minh preview hay tạo ảnh. Revision tăng hợp lệ, không sửa lùi revision để giả chưa từng có mutation. Recipe được giữ nguyên sau smoke.
