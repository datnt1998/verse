# A/B — Lint schema tool tĩnh

## 1. Giả thuyết

Một test tĩnh duyệt mọi tool mà mọi worker Noverse đăng ký (lấy từ
`subagent.Runner.AgentConfig(...).Tools`, không duy trì danh sách thủ công) bắt
được schema mà validator lúc gọi (`agentcore.validateToolArgs`) hoặc chế độ
strict của provider (`openai.normalizeStrictSchema`) sẽ từ chối hoặc im lặng bỏ
qua — ví dụ `required` chỉ tới thuộc tính không khai báo trong `properties`,
mảng `enum` lệch kiểu `type`, `items`/`properties` không phải object schema,
`required`/`additionalProperties` sai kiểu, `const`, và với tool
`StrictSchemaTool` thì các ràng buộc strict (mọi property phải required,
`additionalProperties` không được ghi giá trị khác `false`, combinator bị từ
chối). Schema hợp lệ giúp model không mất lượt vào lỗi validate có thể phát
hiện tĩnh (tiền lệ thật: `save_foundation` thiếu `axes` ở run
`final-architect-short-627a0e4` — đã được sửa trước nhánh này).

- Metric mục tiêu (phase 05): `tool_errors_by_kind.validation` — đo được trong
  report.json; thứ cấp: `tool_errors` tổng không tăng.
- Thay đổi: (ghi ở đây nếu lint phát hiện schema hỏng cần sửa — sẽ cập nhật
  trước khi chạy eval).

## 2. Ngưỡng cải thiện (vs baseline phase 05)

- Nếu baseline có `tool_errors_by_kind.validation` trung bình ≥ 1/run trên bộ
  case đã chọn: trung bình nhánh mới phải giảm ≥ 1 lỗi/run.
- Nếu baseline = 0: nhánh mới phải giữ = 0 (không có lỗi validation mới), và
  không case nào từ pass sang fail quá mức nhiễu baseline (chênh < 2/3 theo N=3).
- smoke PASS 3/3; token + chi phí trên cùng bộ case không tăng quá 10%.

## 3. Bộ case

Mỗi case N=3. Tổng 9 case = 3 smoke + 6 dev.

- smoke (3): `architect_short`, `architect_long`, `writer_first_chapter` —
  đủ cả ba vai, đường ngắn nhất chạm mọi tool của architect/writer/editor.
- dev (6):
  - `wr-intent-vs-arrival`, `wr-predecessor-replay`, `wr-relative-time` —
    writer chạy chuỗi `novel_context` → `plan_chapter` → `draft_chapter` →
    `check_consistency` → `commit_chapter` (schema strict phức tạp nhất,
    `commit_chapter` ~10 thuộc tính nested).
  - `ed-clean-01` — editor đường sạch (`novel_context`, `save_review` strict
    ~12 thuộc tính nested 4 mảng object).
  - `ed-timeline-contradiction` — editor phải ghi `issues[]` (subschema
    `issueSchema` nhiều trường nullable-required).
  - `ed-recheck-unfixed` — editor chạm `recheck_results[]` (subschema đặc biệt,
    thường ít được exercise).

Phần còn lại của bộ dev (`ed-ctrl-*`, `ed-goal-*`, `ed-knowledge-leak`,
`ed-missing-history`, `wr-thread-overdue`) dùng cùng tập tool, không thêm thông
tin về schema nên không chọn.

## 4. Kết quả

_(để trống — controller điền sau khi chạy eval hai nhánh)_
