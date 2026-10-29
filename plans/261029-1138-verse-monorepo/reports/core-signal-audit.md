# Kiểm nguồn dữ liệu — metric core trong eval

Nguồn đối chiếu: `workspace/evals/pre-verse-smoke-c49b37d/` (run thật, file backend; spec ghi tên cũ `c724417`). Thư mục `verse-core-smoke-da2b431` trên worktree chính còn đang chạy — chỉ đọc file đã ghi xong, hình dạng giống hệt.

Đối tượng chính: `artifacts/<case>/meta/sessions/agents/<agent>-*.jsonl` — mỗi dòng là một `llm.Message` đã serialize (`role`, `content`, `metadata`, `timestamp`, `usage`), kèm `_meta:{provider,model}` trên assistant có usage. Quan sát run `writer_first_chapter`: writer 12 assistant + 12 tool, architect_long 11 + 11; **0 tool error trong cả run**.

| Tín hiệu | Đã có trong session log? | Ở đâu / dạng gì | Thiếu gì |
|---|---|---|---|
| `tool_errors` tổng | Có | `metadata.is_error=true` trên message `role:"tool"` (đã có `tool_name`, `tool_call_id`) | Không phân loại validation / business / blocked / circuit — cần `error_kind` trên tool message |
| Retry LLM | Không | `EventRetry` chỉ tồn tại trong event stream / queue runtime (`meta/runtime/queue.jsonl` không có bản ghi retry nào trong run này) | Cần cầu nối sự kiện `retry` vào session log |
| Stream idle | Không | Chỉ nằm trong chuỗi error (`ErrProviderStreamIdle` / chuỗi "stream idle timeout") | Cần sự kiện riêng `stream_idle` |
| Guard chặn tool (repeat-failure, ToolGate) | Không | Middleware/gate trả lỗi → chỉ là tool result `is_error` không phân biệt được với lỗi nghiệp vụ | Cần `tool_blocked{reason}` + phân loại theo kiểu lỗi (`ToolBlockedError`), không theo chuỗi |
| Circuit breaker (MaxToolErrors) | Không | `failToolCall` sinh tool result `is_error` giống lỗi thường | Cần `circuit_open` + `error_kind=circuit` |
| Compaction `novel_context` | Có (cũ) | Đếm gián tiếp qua `_trimmed` trên result nguyên văn + placeholder `[session_compact: ...]` | Chỉ phản ánh session-log compact, không phải context compaction |
| Compaction `read_chapter` và tool khác | Không | `ToolResultMicrocompact` (corecontext) gắn `metadata.compacted_tool_name` vào view nội bộ, không vào session log; `RewriteEvent` chỉ đi slog | Cần `compaction{tool}` phát từ chiến lược qua kênh sự kiện |
| Kết thúc subagent (vì sao dừng) | Không | `EndReason`/`ErrStopGuard` chỉ ở `RunSummary`/error stream; session log không ghi | Cần `subagent_exit{trigger}` |
| Thời lượng run | Một phần | `meta/run.json` `started_at`; `meta/usage.json` `updated_at`; `timestamp` trên mỗi message | Không có trường `duration_s` trong report — suy ra từ các timestamp trên |
| Token + chi phí tổng | Có | `meta/usage.json` `overall` + `per_agent` (đã gồm `cost_usd`, `saved_usd`) | `per_agent` ở mức role (architect_* gộp), không nằm trong report |
| Token theo agent | Có | `usage` + `_meta` trên từng assistant message của từng file session; `usage.json` `per_agent` | Không có `usage_by_agent` trong report |

## Kết luận

- Dữ liệu message đã đủ để đếm `tool_errors`, usage theo agent và suy thời lượng.
- Toàn bộ tín hiệu vòng lặp (retry, stream idle, blocked, circuit, compaction, subagent exit) đều **đã phát sinh trong core** nhưng chưa có đường vào session log → cần envelope `_event` cố định và observer nối vào `Sessions`.
- Phân loại lỗi tool phải đặt `error_kind` ngay trên tool message tại chỗ phát sinh (core phân loại bằng kiểu lỗi `ToolValidationError`/`ToolBlockedError`), để bất biến `sum(tool_errors_by_kind) == tool_errors` đúng kể cả với log cũ (thiếu kind → `business`).
