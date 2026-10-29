---
title: "Phase 06 — Nâng cấp core có A/B"
status: merge không qua A/B (quyết định của anh, 2026-10-29)
---

# Nguyên tắc

Mỗi nâng cấp đi trên một nhánh riêng và có một file `reports/ab-<tên>.md` ghi lại các mục sau. **Ba mục đầu phải viết xong trước khi chạy eval:**

1. Giả thuyết, và metric mục tiêu lấy từ phase 05.
2. Ngưỡng cải thiện, so với baseline phase 05.
3. Bộ case dùng để đo. Mỗi nâng cấp chỉ chạy smoke cộng tối đa 6 case dev có liên quan, mỗi case N=3.
4. Kết quả của cả hai nhánh, kèm commit và run id.

Một nâng cấp chỉ được merge khi thoả cả bốn điều:

- metric mục tiêu đạt ngưỡng;
- không case nào từ pass chuyển sang fail vượt mức nhiễu đã ghi ở baseline;
- smoke vẫn PASS 3/3;
- token và chi phí tăng không quá 10% trên cùng bộ case.

Bộ held-out chỉ chạy một lần ở cuối phase, so với mốc của phase 05.

# Danh sách theo thứ tự

| # | Nâng cấp | Hiện trạng | Metric mục tiêu | Làm khi nào |
|---|---|---|---|---|
| 1 | Lint schema tool tĩnh: một test duyệt mọi tool đã đăng ký của Noverse (và Viderse nếu đã có), bắt các schema mà kiểm tra lúc gọi (`core/agentcore/tool.go:370-400`) hoặc chế độ strict của provider sẽ từ chối | Chưa có lint tĩnh. Bằng chứng thật: `save_foundation` thiếu `axes` ở run `final-architect-short-627a0e4` | `tool_errors_by_kind.validation` giảm | Luôn làm. Không tốn chi phí runtime |
| 2 | Đưa guard lặp lỗi tool từ `apps/noverse/internal/agents/guard/repeat_failure.go` lên `core/agentcore` thành một option | Guard đã chạy, ngưỡng 2, nhớ tối đa 256 khoá. Chưa rõ phạm vi sống (theo run hay theo process) | `tool_errors_by_kind.blocked` đo được; không tăng số case fail | Luôn làm. Trước khi dời, kiểm phạm vi sống của guard và thêm test cho phạm vi đó |
| 3 | Subagent trả `Status` và `OutputSchema` qua tool `submit_result`, xây trên `StopAfterToolResult`/`TerminalResult` | Đường kết thúc đã có. Chưa có contract kết quả có kiểu | `subagent_exit_by_trigger` không còn lần thoát nào thiếu kết quả kết thúc | Khi phase 05 cho thấy có lần thoát như vậy, hoặc khi Viderse cần kết quả có kiểu |
| 4 | Ghim message khi compaction | Có placeholder `[session_compact: …]` trong log thật | Giảm số case fail có `compactions_by_tool` > 0 | Chỉ khi phase 05 cho thấy compaction đi kèm case fail |
| 5 | `SessionStore` tuỳ chọn | Noverse resume dựa vào dữ kiện (`host/resume.go:134`) | Đúng trạng thái sau khi resume | Chỉ khi có app cần. Noverse giữ cách resume hiện tại |

# Gate của phase

- Mỗi nâng cấp đã merge có file A/B đủ 4 mục và thoả điều kiện merge.
- Nâng cấp không đạt ngưỡng thì không merge. Ghi kết quả âm tính vào file A/B.
- Chạy held-out cuối phase. So với mốc phase 05, `critical_hit` và `false_positive` không xấu đi quá mức nhiễu.
- Test ranh giới vẫn qua. Mọi thứ thêm vào `core/` không có từ vựng tiểu thuyết trong code runtime.

## Kết quả (2026-10-29)

Gate của phase **không đạt theo nghĩa gốc**: chưa có A/B. Anh quyết định cắt eval (cả loạt ước 14–20 giờ) và merge cả hai nâng cấp "Luôn làm" (#1, #2) mà không đo. Nhánh `core-upgrades` merge ở `61424cc`. Bằng chứng thay cho eval và rủi ro còn lại nằm ở mục 4 của [ab-tool-schema-lint.md](./reports/ab-tool-schema-lint.md) và [ab-repeat-failure-guard.md](./reports/ab-repeat-failure-guard.md).

#3–#5 chưa làm vì điều kiện kích hoạt chưa xảy ra trên baseline đã có (`architect_long` ×3, xem [baseline-core-metrics.md](./reports/baseline-core-metrics.md)):
- **#3:** mọi lần thoát subagent đều là `tool`, tức có kết quả kết thúc. Writer của Viderse đã trả kịch bản có kiểu qua `save_script`, nên chưa cần thêm.
- **#4:** `compactions_by_tool` bằng 0.
- **#5:** chưa app nào cần.

Baseline này nhỏ, nên kết luận chỉ đúng với case đó.

# Rollback

Mỗi nâng cấp là một nhánh và một lần merge riêng; revert từng cái độc lập được.

# Rủi ro

- **Overfit vào bộ dev.** Held-out đã fail ở cả hai nhánh trước đây. Giảm rủi ro bằng cách đặt ngưỡng trước khi đo và chỉ chạy held-out ở cuối.
- **Chi phí eval.** Mỗi nhánh tốn khoảng $1,5 cho smoke, cộng khoảng $1,1 cho 18 lượt case. Con số $1,1 suy từ baseline: 51 lượt case hết $3,10. Nhân 2 nhánh và 5 nâng cấp thì tối đa khoảng $26, chưa kể held-out, và mất khoảng 15–25 giờ máy (mỗi smoke 67 phút). Thứ tự ở bảng trên ưu tiên các nâng cấp rẻ và chắc chắn.
