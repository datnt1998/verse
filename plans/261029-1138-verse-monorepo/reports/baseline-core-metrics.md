# Baseline metric core

Ngày 2026-10-29. Binary `ainovel-cli` build từ commit `51708aa` (đã gồm các metric của phase 05), gọi bằng đường dẫn tuyệt đối. Model lấy từ `~/.ainovel/config.json`: architect và editor dùng `openai-codex/gpt-6.1-sol`, writer dùng `openai-codex/gpt-6-luna`.

## Phạm vi thực tế

Plan muốn chạy smoke ×3, bộ dev ×3 và held-out ×1. Mỗi lượt smoke mất khoảng 30 phút, nên cả loạt ước 14–20 giờ. Anh quyết định cắt ngay sau khi xong 3 lượt `architect_long`. Bộ dev, held-out, `architect_short` và `writer_first_chapter` đều **chưa chạy**. Vì vậy đây là baseline của một case, chưa đủ làm mốc A/B cho phase 06.

Vì chain bị dừng giữa chừng nên `report.json` chưa được ghi. Số liệu dưới đây được chấm lại từ artifact của 3 lượt đã xong, qua đúng `eval.Collect` + `eval.Grade` mà lệnh eval dùng. Kết quả chấm lại khớp với lúc chạy: PASS, WARN, PASS.

Lệnh tái tạo (từ `apps/noverse`):

```sh
<binary> eval --cases evals/cases/smoke/architect_long.json --repeat 3 --ci --timeout 60m \
  --out workspace/evals/baseline-core-51708aa-smoke
```

## Số đo (`architect_long`, N=3)

| Metric | Trung bình | Min | Max |
|---|---|---|---|
| `tool_calls` | 20.67 | 16 | 25 |
| `tool_errors` | 1.33 | 1 | 2 |
| `tool_errors_by_kind.business` | 1.00 | 1 | 1 |
| `tool_errors_by_kind.validation` | 0.33 | 0 | 1 |
| `retries` | 0.00 (thật ra 1.33, xem dưới) | 0 | 0 |
| `stream_idle` | 0.00 (thật ra 1.33, xem dưới) | 0 | 0 |
| `tool_blocked_events` | 0.00 | 0 | 0 |
| `circuit_open_events` | 0.00 | 0 | 0 |
| tổng `compactions_by_tool` | 0.00 | 0 | 0 |
| `subagent_exit_by_trigger.tool` | 2.00 | 2 | 2 |
| `duration_s` | 1889 | 1262 | 2789 |
| `usage.cost_usd` | 0.616 | 0.447 | 0.718 |
| `usage.input` | 860,373 | 491,771 | 1,170,194 |
| `usage.output` | 45,970 | 38,655 | 50,855 |
| `usage.cache_read` | 706,219 | 367,616 | 990,208 |

| Lượt | Kết quả | Tool call | Lỗi tool (theo loại) | Thời lượng (s) | Chi phí ($) | `usage_by_agent` (input/output, chi phí) |
|---|---|---|---|---|---|---|
| r1 | PASS | 25 | 1 (business 1) | 1617 | 0.684 | architect_long 846,629/34,743 $0.662; writer 322,952/13,048 $0.015 |
| r2 | WARN | 21 | 2 (business 1, validation 1) | 2789 | 0.718 | architect_long 747,627/40,737 $0.700; writer 170,915/9,518 $0.010 |
| r3 | PASS | 16 | 1 (business 1) | 1262 | 0.447 | architect_long 250,559/26,549 $0.428; writer 240,599/11,581 $0.013 |

- **Bất biến:** cả 3 lượt có tổng `tool_errors_by_kind` = `tool_errors`; không trường mới nào thiếu hay null.
- **r2 WARN:** luật `StreamIdleStorm` báo 4 lần ngắt vì chờ quá 5 phút (provider, 17:32–17:48). Mất thêm khoảng 21 phút, nhưng run vẫn hoàn tất.

## Lỗi đo phát hiện từ baseline

Ở r2, `retries` và `stream_idle` bằng 0, trong khi log runtime ghi 4 lần "Thử lại (n/7): stream idle timeout". Nguyên nhân: lỗi xảy ra trước sự kiện đầu tiên của stream do `core/llmretry.RetryingModel` bắt và retry, nằm dưới vòng lặp agentcore. Lớp này chỉ in dòng tiến độ, không phát sự kiện, nên session log không có gì để collector đếm. Test cầu nối của phase 05 chỉ giả lập lỗi giữa stream (vòng lặp tự retry), nên không bắt được trường hợp này.

Đã sửa ở `b1da1b8`:
- `RetryingModel` gửi `retry` (đánh dấu `Reason=model`) và `stream_idle` về run qua `agentcore.ReportEvent`.
- Subagent runner đếm các sự kiện đó nhưng không in thêm dòng tiến độ thứ hai.
- Có test `TestRetryingModelReportsPreStreamRetriesToTheRun`. Test này fail nếu thiếu bất kỳ nửa nào của bản sửa.

Số `retries`/`stream_idle` ở bảng trên lấy từ binary `51708aa`, tức trước bản sửa. Đếm theo log runtime thì r2 có 4 retry và 4 stream idle, nên trung bình N=3 là 1.33.
