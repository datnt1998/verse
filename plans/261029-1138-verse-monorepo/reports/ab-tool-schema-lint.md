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
- Thay đổi (ghi sau khi đăng ký trước, trước khi chạy eval):
  `save_foundation.nguoi_nghe.items.enum` đổi `[]int` → `[]any`
  (`save_foundation.go:59`). Lint phát hiện validator lúc gọi chỉ đọc enum
  dạng `[]any`/`[]string`, nên enum `[]int` bị im lặng bỏ qua — giá trị ngoài
  {0..4} trước đây lọt vào tool không qua kiểm tra. Sau sửa enum được ép thật.
  JSON schema gửi cho model không đổi (`[0,1,2,3,4]` ở cả hai dạng).

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

**Chưa chạy A/B.** Ngày 2026-10-29, anh quyết định cắt toàn bộ eval (cả loạt ước 14–20 giờ) và merge mà không đo. Baseline phase 05 chỉ có `architect_long` ×3, `tool_errors_by_kind.validation` trung bình 0,33/run (0–1), nên ngưỡng ở mục 2 chưa đánh giá được.

Merge vào `main` ở `61424cc` dựa trên:
- Lint chạy trong suite với tool thật của mọi worker và qua; suite đầy đủ 4152 test pass.
- Thay đổi duy nhất lên tool là kiểu Go của `enum`. JSON gửi cho model vẫn là `[0,1,2,3,4]`, nên prompt và schema mà model thấy không đổi.

Rủi ro còn lại: validator giờ ép enum `nguoi_nghe.items` thật. Model gửi giá trị ngoài 0–4 sẽ nhận lỗi validation và phải gọi lại, thay vì giá trị sai lọt vào dữ liệu. Nếu muốn xác nhận, chạy lại mục 3 trên `main` và so với baseline.
