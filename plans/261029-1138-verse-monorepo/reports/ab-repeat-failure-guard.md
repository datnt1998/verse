# A/B — Guard lặp lỗi tool lên core

## 1. Giả thuyết

Đưa middleware chặn "gọi lại y nguyên lời gọi vừa lỗi" từ
`apps/noverse/internal/agents/guard/repeat_failure.go` lên `core/agentcore`
thành option opt-in, giữ nguyên ngưỡng 2, trần 256 khoá, phạm vi sống và thông
báo chặn của Noverse. Đây là refactor thuần + tách contract: hành vi đúng vẫn
chặn đúng, kết quả bị chặn vẫn phát `tool_blocked{reason:"repeat_failure"}` và
tool result `is_error` với `error_kind=blocked`.

- Metric mục tiêu (phase 05): `tool_errors_by_kind.blocked` — phải tiếp tục đo
  được trong report.json (không mất sự kiện `tool_blocked`, không mất
  `error_kind`), đồng thời không tăng số case fail.
- Phạm vi sống của guard (đã kiểm): một instance middleware per
  `agents.BuildWorkers`, dùng chung cho `architect_short` và `architect_long`;
  sống suốt vòng đời runner = hiệu quả theo process, vượt qua từng lần
  `Runner.Run`. Test pin: bộ đếm tồn tại qua hai lần spawn liên tiếp của cùng
  runner.

## 2. Ngưỡng cải thiện (vs baseline phase 05)

Refactor thuần, không kỳ vọng cải thiện:

- `tool_errors_by_kind.blocked` trên bộ case phải bằng baseline ± nhiễu (chênh
  < 2/3 theo N=3), và phải > 0 ở ít nhất một lần chạy nếu baseline cũng > 0 để
  chứng minh kênh đo còn sống.
- Không case nào từ pass sang fail quá mức nhiễu baseline.
- smoke PASS 3/3; token + chi phí trên cùng bộ case không tăng quá 10%.

## 3. Bộ case

Mỗi case N=3. Tổng 9 case = 3 smoke + 6 dev.

- smoke (3): `architect_short`, `architect_long`, `writer_first_chapter` —
  cả ba vai đều gắn guard; architect nhiều lượt `novel_context`/`read_chapter`
  là nơi guard chặn thật trong production (vụ 10 lượt `novel_context`).
- dev (6):
  - `wr-intent-vs-arrival`, `wr-predecessor-replay`, `wr-relative-time`,
    `wr-thread-overdue` — writer tự chữa lỗi qua nhiều lượt đọc/ghi; đây là
    đường dễ sinh "gọi lại cùng tham số sau lỗi" nhất (đọc lại cùng chương,
    commit lại sau lỗi validate).
  - `ed-recheck-unfixed` — editor đọc `novel_context` cho chương kiểm lại;
    đường dễ lặp lời gọi đọc ngữ cảnh.
  - `ed-missing-history` — fixture thiếu lịch sử, khả năng tool báo lỗi dữ
    liệu và model thử lại cao nhất trong bộ editor.

Phần còn lại (`ed-clean-01`, `ed-ctrl-*`, `ed-goal-*`, `ed-knowledge-leak`,
`ed-timeline-contradiction`) cùng vai editor, đường tool giống nhau và ít khả
năng sinh lặp lỗi hơn các case đã chọn.

## 4. Kết quả

_(để trống — controller điền sau khi chạy eval hai nhánh)_
