---
title: "Phase 3: Runtime text guards"
status: done
priority: P1
effort: "2.5 ngày"
depends_on: [1]
---

# Phase 3: Runtime text guards

## Overview

Phase này bắt bằng code những lỗi cơ học mà prompt không bảo đảm được.

**Quyết định (user đã chọn): chỉ chặn lỗi rõ ràng, còn lại chỉ báo.**

- Mọi detector nằm trong `internal/rules`. Đây là nơi duy nhất sở hữu detector văn bản, đã chạy lúc commit ở `internal/tools/commit_chapter.go:551-566`.
- Chỉ những **tai nạn cơ học không có cách hiểu văn chương nào khác** mới bị chặn ngay khi tool nhận đầu vào (trả `ErrToolArgs`):
  - thẻ thinking;
  - code fence;
  - lời bình meta ở đoạn đầu hoặc đoạn cuối;
  - văn bản bị cắt cụt (chỉ ở bản cuối);
  - bản viết lại co ngắn quá mức (chỉ khi viết lại theo review);
  - append không có chữ mới.
- **Lặp lại đoạn kết chương trước** (`opening_replay`) chỉ là violation mức `error`, không chặn. Editor và user vẫn thấy nó qua `rule_violations`. Cách này đúng nguyên tắc "công cụ chỉ báo sự thật" (`docs/architecture.md:62`, `internal/rules/lint.go:12-13`).
- **Lặp đoạn văn trong chương** dùng rule `self_duplication` đã có (`internal/rules/lint.go:141-198`, ngưỡng 10% chừa chỗ cho điệp khúc có chủ đích). Phase này **không** thêm detector lặp đoạn thứ hai.

Phase này cũng giới hạn vòng sửa của `llmcontract` và gỡ `<think>` khỏi raw trước khi decode.

Hiện trạng đã kiểm chứng:

- `draft_chapter` chỉ kiểm content rỗng (`internal/tools/draft_chapter.go:61-63`). Append nối cứng bằng `"\n\n"` (`internal/store/drafts.go:41-56`). Có test append ngắn hợp lệ (`internal/tools/read_draft_test.go:239-247`); hành vi này phải được giữ.
- Thứ tự xử lý trong commit: nạp payload pending nếu có (`commit_chapter.go:108-120`) → `validateCommitArgs` chạy cho cả bản nộp mới lẫn pending ở stage `started` (`:136-150`) → nhánh viết lại mới gọi `validateRewriteDraft` **trước khi** `SavePendingCommit` (`:158-180`) → pending viết lại thì đi thẳng vào phục hồi (`:152-157`) và dùng `pending.DraftContent` đã đóng băng.
- Vì vậy gate đặt trong `validateCommitArgs` sẽ làm kẹt các pending commit cũ. Gate phải chỉ chạy **khi nhận bản nộp mới**, tức nhánh `existingPending == nil`.
- `llmcontract.Execute` lặp không giới hạn (`core/llmcontract/execute.go:107`). Test `internal/arbiter/arbiter_test.go:145` đang pin hành vi này.
- Nguyên nhân viết lại được ghi trong `meta/rewrites.jsonl` với cause là `review` hoặc `user` (`internal/store/rewrites.go:17-57`).

## File ownership

| Tạo mới | Sửa |
|---------|-----|
| `internal/rules/integrity.go`, `integrity_test.go` (`Admission(text, stage) []Violation`, `OpeningReplay`) | `internal/rules/lint.go` (gọi thêm `appendOpeningReplay` qua `CheckChapter` khi có chương trước) |
| | `internal/tools/draft_chapter.go`, `read_draft_test.go` |
| | `internal/tools/commit_chapter.go` (nhánh nhận bản nộp mới và `validateRewriteDraft`), `commit_chapter_test.go` |
| | `internal/host/import_preview.go:27` (instance commit của import bật option bỏ admission gate) |
| | `internal/tools/edit_chapter.go`, `edit_chapter_test.go` |
| | `core/llmcontract/execute.go`, `contract_test.go`; `internal/arbiter/arbiter_test.go:145` |
| | `assets/prompts/writer.md` (dòng nói về append: hợp đồng chống chồng lấn), `assets/testdata/writer-golden.md` |
| | `core/llm/provider/compat/stream.go`: chỉ khi bước 8 có bằng chứng |
| | `docs/import-pipeline.md:652` (nếu mô tả vòng sửa của llmcontract) |

## Requirements

1. **`rules.Admission(text string, stage Stage) []Violation`**, với `Stage ∈ {Draft, Final}`. Mọi violation có `Severity: error` và `Message` hướng dẫn sửa bằng tiếng Việt.

   | Rule | Draft | Final | Điều kiện |
   |------|:-----:|:-----:|-----------|
   | `think_tag` | ✔ | ✔ | Chứa `<think`, `</think>`, `<thinking`, `</thinking>` (không phân biệt hoa thường) |
   | `code_fence` | ✔ | ✔ | Có dòng bắt đầu bằng <code>```</code> |
   | `meta_commentary` | ✔ | ✔ | Đoạn đầu hoặc đoạn cuối khớp danh sách cụm cố định tiếng Việt/Anh, ví dụ "Dưới đây là chương", "Hy vọng chương này", "Here is chapter", "As an AI". Danh sách nằm trong code và được mở rộng qua test bảng |
   | `truncated_end` | — | ✔ | Ký tự cuối (sau khi bỏ khoảng trắng) không thuộc `.!?…"”’»)」』—~` |
2. **Co ngắn bất thường khi viết lại.** Trong `validateRewriteDraft`, tức trước khi đóng băng: nếu `WordCount(next)/WordCount(orig) < 0.6` **và** cause viết lại gần nhất là `review`, trả `ErrToolPrecondition`. Thông điệp gợi ý hai cách: khôi phục phần đã mất, hoặc dùng `edit_chapter`. Viết lại do user hoặc polish chỉ đổi tiêu đề thì không bị ảnh hưởng.
3. **Vị trí gắn gate.**
   - `draft_chapter` (`write`/`append`): `Admission(content, Draft)`.
   - `commit_chapter`, chỉ trên đường nhận bản nộp mới:
     - chương mới: gọi `Admission(final, Final)` sau `ValidateChapterWork` (`:187-201`) và **trước khi lưu pending**;
     - viết lại: gọi trong `validateRewriteDraft`.

     Đường phục hồi pending (`:109-120`, `:152-157`) **không** chạy gate. Payload đã đóng băng được coi là đã qua kiểm theo hợp đồng lúc nó được nhận.
   - `edit_chapter`: `Admission(new_string, Draft)`.
   - **Import được miễn.** Import publish **qua chính `commit_chapter`** (`internal/host/imp/publish.go:161-233`), dùng một instance riêng tạo ở `internal/host/import_preview.go:27`. Chính văn của tác giả là sự thật gốc, nên không được bị chặn vì `truncated_end`/`code_fence`.
     - Thêm option `tools.WithAuthorSource()` vào `NewCommitChapterTool` để tắt `Admission`. Instance import bật option này. `rules.Lint` vẫn chạy và vẫn báo như hiện tại.
     - Test: import một chương kết thúc không có dấu câu → publish thành công (`go test ./internal/host/imp/... ./internal/host -run Import`).
4. **`opening_replay`.** Trong `rules.CheckChapter`, khi có chương N-1: lấy `OpeningOverlap(tail(N-1, 1200 rune), head(N, 600 rune))`. Kết quả ≥ `replayLimitRunes` (khởi điểm 80, hiệu chỉnh bằng `wr-predecessor-replay`) thì thêm `Violation{Rule: "opening_replay", Severity: error}`. **Không chặn.** Violation đi vào `rule_violations` của kết quả commit và được lưu bằng `SaveRuleViolations`.
5. **Append chống chồng lấn** trong `draft_chapter`, trước khi gọi `AppendDraft`:
   - `TrimLeadingOverlap(tail(draft, 1600), content, 48)`;
   - nếu sau khi cắt **không còn ký tự chữ nào**, trả `ErrToolArgs` ("toàn bộ đoạn gửi lên đã có trong bản nháp; viết tiếp từ sau câu cuối") và không ghi;
   - đoạn ngắn nhưng có chữ mới thì vẫn được nhận, giữ test `read_draft_test.go:239-247`;
   - kết quả trả về thêm `trimmed_overlap_runes` khi có cắt;
   - cập nhật mô tả tool và `writer.md` dòng 8.
6. **(1.9) Giới hạn vòng sửa của `llmcontract`.**
   - Thêm `Request.MaxCorrections`, mặc định `defaultMaxCorrections = 3`. Chỉ đếm vòng sửa schema/nghiệp vụ; retry do lỗi request vẫn thuộc `llmretry`.
   - Hết giới hạn thì trả `&Failure{Kind: FailureContract, Raw: lastRaw, Err: …}`.
   - Đổi test arbiter `:145` thành `TestDecide_InvalidOutputStopsAfterCorrectionCap`: tổng cộng 4 lần gọi, không có lần thứ 5.
   - Rà từng caller, ghi kết quả vào PR:
     - `arbiter/arbiter.go:34`: đọc thêm `intervention.go:346-366`;
     - `host/imp/call.go:159`: đọc thêm `imp/runner.go:251-252`;
     - `host/sim/runner.go:125`;
     - `ledger/architect.go:46`;
     - `revision/analyze.go:64`;
     - `userrules/normalize.go:80`.

     Mỗi caller phải đi theo đường thất bại sẵn có, không panic, không treo.
7. **(2.5) Thẻ thinking.**
   - (a) Trong `Execute`, trước khi decode, gỡ các khối `<think>…</think>` / `<thinking>…</thinking>` **ở đầu** raw. Chỉ có thẻ mở mà không có thẻ đóng thì coi là lỗi protocol, cho model sửa.
   - (b) **Có điều kiện.** Probe provider thật: gọi một lượt chat thường với model reasoning qua Ollama hoặc compat, nếu cấu hình có.
     - Content có `<think>`: thêm bộ lọc tăng dần vào nhánh content của `compat/stream.go:251` (giữ prefix thẻ bị cắt giữa các chunk, chuyển phần bên trong sang `ReasoningDelta`) và fixture SSE trong `testdata/compat/`.
     - Không có: ghi `reports/p3-think-probe.md` (chỉ tên model và kết quả) rồi bỏ qua bước này.

## Implementation Steps

1. Viết `rules/integrity.go` cùng test bảng. Mỗi rule cần một case dương và một case âm. Bắt buộc có các case **không** được chặn nhầm: lời thoại kết bằng `”`, câu kết bằng `—` hoặc `…`, nhân vật nói "Tôi sẽ viết thư cho anh" ở giữa truyện.
2. Viết `opening_replay` trong `CheckChapter`, kèm test bảng.
3. `draft_chapter`: gate Draft và append chống chồng lấn. Test: chồng lấn một phần → cắt; trùng toàn bộ → lỗi; câu kết ngắn → nhận.
4. `commit_chapter`:
   - Gate Final ở nhánh nộp mới; co ngắn trong `validateRewriteDraft`.
   - Test viết lại do review mà co còn 50% → bị chặn, **không** có pending được tạo.
   - Test viết lại do user mà co còn 50% → qua.
   - Test polish chỉ đổi tiêu đề (`commit_chapter_test.go:1326-1330`) → qua.
   - Test **pending cũ ở stage `started`** có văn bản chứa `**` hoặc thiếu dấu chấm cuối → phục hồi thành công, không bị gate chặn.
5. `edit_chapter`: gate và test.
6. `llmcontract`: giới hạn vòng sửa, gỡ think. Sửa `contract_test.go` (`:291`, `:314`, `:328`) và test arbiter. Rà caller theo Requirement 6.
7. Hiệu chỉnh `replayLimitRunes` và danh sách cụm meta: test bảng cộng `eval --cases evals/cases/calibration/dev --repeat 3`.
8. Probe `<think>` theo Requirement 7b.
9. A/B tập dev (repeat 3), lưu bản tóm tắt an toàn `reports/p3-ab.md`. Theo dõi `tool_errors` và `writer_violations`.

## Test Matrix

| Lớp | Lệnh |
|-----|------|
| Rules | `go test ./internal/rules/...` |
| Tools | `go test ./internal/tools -run 'Draft\|Commit\|EditChapter\|Rewrite'` |
| Contract | `go test ./core/llmcontract/... ./apps/noverse/internal/arbiter/...` (ở gốc repo) |
| Caller | `go test ./internal/host/imp/... ./internal/host/sim/... ./internal/ledger/... ./internal/revision/... ./internal/userrules/...` |
| Golden | `go test ./assets/...` |
| Race | `go test -race ./internal/tools ./internal/store ./internal/host` |

## Success Criteria

- [x] `wr-predecessor-replay`: tỉ lệ pass không thấp hơn baseline, và violation `opening_replay` xuất hiện trong `rule_violations` khi lặp ≥ ngưỡng.
- [x] `false_positive` và `writer_violations` không tăng so với baseline. `tool_errors` tăng ≤ 10%.
- [x] Mọi đường trong `llmcontract.Execute` đều có giới hạn; test arbiter mới xanh.
- [x] Pending cũ phục hồi được dù văn bản không qua gate mới (có test). Import chương "cụt" của tác giả vẫn publish được (có test).
- [x] Có kết quả probe `<think>`, kèm bộ lọc stream nếu probe dương tính.

## Risk Assessment

- **Chặn nhầm.** Chỉ 4 rule được chặn, đều là tai nạn cơ học. Danh sách cụm meta chỉ khớp ở đoạn đầu/cuối. `truncated_end` chỉ áp dụng cho bản cuối.
- **Giới hạn vòng sửa làm import thất bại sớm hơn.** Rà `imp/runner.go:251-252` để chắc rằng batch lỗi đi theo đường retry/chia đôi sẵn có.

## Rollback

Mỗi gate là một lời gọi trong tool, revert từng commit độc lập được. `opening_replay` chỉ là violation. Không có dữ liệu lưu trên đĩa nào đổi hình dạng.
