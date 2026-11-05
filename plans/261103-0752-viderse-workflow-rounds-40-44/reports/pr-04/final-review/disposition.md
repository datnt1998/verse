# Kết quả xử lý review V38

Review cùng họ model theo route task được người dùng cho phép, không phải independent-family certification. Parent trực tiếp sửa và chạy nghiệm thu; báo cáo gốc giữ nguyên.

| Phát hiện | Sửa nguyên nhân | Bằng chứng |
|---|---|---|
| Reload xung đột nhận rồi bỏ mất chỉnh sửa mới | Fieldset và ô tên bị khoá trong lúc GET; hook chặn nhận recipe/tên trong cùng cửa sổ đồng bộ. Thành công thay đúng draft đã được xác nhận bỏ; thất bại vẫn giữ draft. | Probe PC và390px đều fail trước/qua sau;19ca khôi phục khác tiếp tục qua. |
| Escape tên gửi PATCH của giá trị vừa huỷ | Cờ huỷ được ghi đồng bộ trước blur; bỏ commit đúng blur đó, giữ Enter/blur bình thường. | Regression bắt chính request tên bị huỷ: fail trước/qua sau. Probe Chrome thật không gửi PATCH khi Escape. |
| PC dựng template đã lưu thay cho draft đang lỗi | Eligibility gồm trạng thái edit đã lưu và chính nguồn đang chọn còn khả dụng; handler cũng kiểm cùng điều kiện. Không gửi draft vào API dựng. | Regression fail trước/qua sau; probe UI khoá hành động sau lỗi lưu. Không gọi dựng thật/provider. |

- `recovery-fail-before/results.json`: bốn ca UI lỗi (hai viewport cho reload),0mutation tới server.
- `recovery-pass-after.json`: prerequisite native và bốn ca sửa đều qua;0mutation tới server. PATCH trong probe là response fixture có nhãn, không phải lưu backend thật.
- Full frontend sau sửa:441file,3.661test qua/4skip. Focused Library19test qua; typecheck/Vite/native binary build qua. Zod annotation, dynamic/static audio import và chunk-size warnings còn được ghi, không coi là lỗi đã sửa.
- Ledger native vẫn3job/27step/2biên nhận; không gọi model, tạo ảnh hoặc tải ngoài.

Không có blocker backend mới trong phạm vi review đã đọc. Chưa dùng kết quả này để chứng nhận mọi tweak/pixel parity, Safari/iPhone, native generated preview, dữ liệu production hoặc deploy toàn bộ Viderse. V39/V40 còn riêng.
