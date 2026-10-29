---
title: "Phase 05 — Metric core trong eval"
status: chờ phase 02
---

# Bối cảnh

Eval đo kết quả đầu ra khá tốt: `critical_hit`, `false_positive`, `writer_violations`, `quote_coverage`, token và chi phí. Phần tín hiệu của vòng lặp core thì còn mỏng.

| Tín hiệu | Hiện trạng |
|---|---|
| Lỗi tool | Chỉ có một số `tool_errors` cho cả run (`internal/eval/collect.go:44-46`). Lỗi schema, lỗi nghiệp vụ và lỗi do guard chặn bị gộp chung |
| Retry, stream idle, lớp lỗi provider | Không đếm. Nguồn: `core/agentcore/loop.go:489-501`, `errors.go:152-268` |
| Guard lặp lỗi tool, circuit breaker | Không đếm. Nguồn: `agents/guard/repeat_failure.go`, `loop.go:875-884` |
| Compaction | Chỉ tính `novel_context` (`collect.go:47-55`); bỏ sót compaction của `read_chapter` |
| Kết thúc subagent | Không ghi lại vì sao subagent dừng: do tool kết thúc, do lỗi, hay do guard phủ quyết (`subagent.go:142,192`, `stop_guard.go:40`) |
| Thời lượng run, token theo agent | Không có |

Về độ nhiễu: mọi A/B trước đây đều chạy N=3, nên chênh dưới 2/3 không phân biệt được. Bộ held-out fail ở cả hai nhánh (`plans/261027-1443-ai-core-quality-uplift/reports/final-ab.md`).

# Các bước

1. **Kiểm nguồn dữ liệu.** Đối chiếu một session JSONL thật trong `workspace/evals/pre-verse-smoke-c724417/` với từng tín hiệu ở bảng trên, và ghi tín hiệu nào đã có trong log, tín hiệu nào chưa. Kết quả viết vào `reports/core-signal-audit.md` của plan này.
2. **Phát sự kiện ở core cho tín hiệu còn thiếu.** Sự kiện có cấu trúc, tên chung, không dính từ vựng tiểu thuyết: `retry`, `stream_idle`, `tool_blocked{reason}`, `circuit_open`, `compaction{tool}`, `subagent_exit{trigger}`. Sự kiện chỉ để quan sát, không đổi hành vi.
3. **Cầu nối từ sự kiện core vào session log của Noverse.** Hiện chưa có đường này:
   - `store/session.go:119-122` bỏ qua mọi giá trị không phải `llm.Message`;
   - session log được ghi qua `OnMessage` (`agents/build.go:238-240`);
   - `host/event_details.go:67-76` chỉ xử lý kết quả tool và phản hồi model.
   
   Cần định nghĩa một envelope sự kiện cố định (`{"_event": {"kind", "agent", "at", ...}}`) để collector phân biệt với message thường. Nối observer sự kiện của agent (`subagent.go:952-984`) vào bộ ghi session. File thuộc phạm vi: `apps/noverse/internal/store/session.go`, `apps/noverse/internal/agents/build.go`, `apps/noverse/internal/host/event_details.go`. Không có bước này thì các trường mới luôn bằng 0, dù sự kiện có thật.
4. **Collector.** Thêm các trường vào `report.json`:
   - `tool_errors_by_kind` gồm `validation`, `business`, `blocked`, `circuit`; tổng của chúng phải bằng `tool_errors`;
   - `retries`, `stream_idle`;
   - `compactions_by_tool`;
   - `subagent_exit_by_trigger`;
   - `duration_s`;
   - `usage_by_agent` (token và chi phí).
   
   Collector vẫn nằm trong `apps/noverse/internal/eval`, vì nó phụ thuộc `host`. Chỉ tách phần parse chung khi app thứ hai có eval.
5. **Sửa comment cũ** ở `core/llm/client.go:86-88`: comment ghi mặc định 120s, còn bootstrap đặt 5 phút (`bootstrap/config.go:130`).
6. **Baseline.** Mọi lần chạy dùng binary build từ commit ứng viên, gọi bằng đường dẫn tuyệt đối, ghi SHA của binary cùng run id. Chạy smoke 3 lần, cộng bộ dev của writer và editor với N=3. Ghi giá trị trung bình, min và max của từng metric vào `reports/baseline-core-metrics.md`. Ước lượng: smoke khoảng $1,5 và 67 phút mỗi lần; bộ dev khoảng $3 mỗi lượt. Chạy bộ held-out đúng một lần, làm mốc cuối cho phase 06.

# Gate

| Kiểm tra | Điều kiện qua |
|---|---|
| Đếm đúng | Test collector dùng fixture JSONL cắt từ run thật (bỏ hết secret và nội dung dài) và so số đếm chính xác cho từng trường. Có ít nhất một fixture chứa đủ 4 loại lỗi tool |
| Cầu nối thật | Test tích hợp: dựng agent với model giả lập lỗi stream (`retry`) và một tool luôn lỗi validation, chạy qua đúng đường ghi session của Noverse, rồi chạy collector trên file sinh ra. `retries` ≥ 1, `tool_errors_by_kind.validation` ≥ 1. Model giả chỉ dùng trong test này |
| Bất biến | Với mọi run trong baseline, tổng `tool_errors_by_kind` = `tool_errors` |
| Trường có mặt | `report.json` của một lần smoke có đủ các trường mới và không trường nào null |
| Không đổi hành vi | Smoke vẫn PASS 3/3. Chi phí mỗi lần smoke không vượt 1,25 lần số đã ghi ở phase 02 |
| Baseline | `reports/baseline-core-metrics.md` có số đo và lệnh tái tạo (commit, model, cờ) |

# Rollback

Revert commit. Sự kiện và trường mới chỉ là thêm vào; không thay đổi dữ liệu người dùng.

# Rủi ro

- **Session log phình to.** Mỗi sự kiện chỉ ghi vài trường ngắn, không chép nội dung prompt.
- **Gắn nhầm lỗi nghiệp vụ là lỗi validation.** Phân loại dựa vào kiểu lỗi (`ToolValidationError` tại `errors.go:96-99`), không dựa vào chuỗi thông báo.
