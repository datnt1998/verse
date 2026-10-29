---
title: "Phase 4: Review goal checklist"
status: todo
priority: P1
effort: "2.5 ngày"
depends_on: [2]
---

# Phase 4: Review goal checklist

## Overview

Hiện editor chỉ trả một nhãn gộp `contract_status` (`met|partial|missed`) cộng danh sách `contract_misses` dạng văn bản tự do. Không ai biết từng mục tiêu của chương đã đạt hay chưa, và không có bằng chứng.

Phase này làm bốn việc:

1. **Code đóng băng danh sách mục tiêu có ID** từ contract của chương.
2. Editor trả `pass|fail|unknown` cho **từng ID**, kèm trích dẫn.
3. Code kiểm tra phủ đủ, không trùng, không lạ, và mọi trích dẫn có thật trong chính văn. Sau đó code **suy ra** `contract_status`/`contract_misses`.
4. Mọi `issue.quote` (không chỉ mục `[LỖI]`) phải định vị được trong chính văn. Thứ tự property của issue được đổi để model viết bằng chứng trước khi chọn severity.

Hiện trạng đã kiểm chứng:

- Schema `save_review` nằm ở `internal/tools/save_review.go:76-106`. `issue.quote` chỉ bắt buộc với mục `[LỖI]` (`:439-440`) và **không** được kiểm tra có tồn tại trong chương hay không.
- `ReviewEntry` nằm ở `internal/domain/review.go:113-127`.
- Contract được đưa vào `working_memory.chapter_contract` ở `internal/tools/novel_context_builders.go:256-268`. Brief viết lại có dùng `contract_misses` của review scope `chapter` (`:311-313`).
- Consumer của `contract_status`/`contract_misses` (danh sách đầy đủ ở `agent://ScoutReviewRewrite`, đã tóm tắt dưới đây): `internal/diag/rules_quality.go:51-77,144-148`, `internal/agents/ctxpack/builder.go:44-47,354-366`, `internal/tools/novel_context.go:929-933`, `internal/entry/web/api_dto.go:128-131`, `web/src/screens/reader/ReviewPanel.tsx:298,358-368`, `docs/design/v1/11-api-contract.md:160`.
- Schema golden: `internal/tools/testdata/save_review_schema.json`, kiểm bởi `save_review_schema_test.go:15-33` (cập nhật bằng `-update`).
- `web/src/api/types.gen.ts` sinh từ `go generate ./internal/entry/web`. Có test drift ở `internal/entry/web/gentypes/drift_test.go`. Thư mục `internal/entry/web/dist` được track trong git.

## File ownership

| Tạo mới | Sửa |
|---------|-----|
| `internal/domain/goals.go` (`ChapterGoal`, `GoalCheck`, `GoalsFromContract`, `DeriveContractStatus`) + `goals_test.go` | `internal/domain/review.go` (`ReviewEntry.GoalChecks`) |
| `internal/domain/quote.go` (`LocateQuote(text, quote string) (start, end int, ok bool)`, `NormalizeQuote`) + test. Phase 05 và 06 dùng lại | `internal/tools/save_review.go`, `save_review_test.go`, `save_review_schema_test.go`, `testdata/save_review_schema.json` |
| | `internal/tools/novel_context_builders.go` (chỉ thêm `chapter_goals` cạnh `chapter_contract`), `novel_context_test.go` |
| | `assets/prompts/editor.md` (§1 và hướng dẫn `goal_checks`) |
| | `internal/entry/web/api_dto.go`, `web/src/api/types.gen.ts` (sinh tự động), `web/src/screens/reader/ReviewPanel.tsx` (+ test nếu có), `internal/entry/web/dist/**` (build lại) |
| | `internal/eval/grade.go` (bật `goal_status`, `unknown_correct`), `evals/cases/calibration/dev/ed-goal-{all-met,beat-missing}.json`, `evals/cases/calibration/heldout/ed-goal-unverifiable.json` + fixture |
| | `docs/design/v1/11-api-contract.md` |

## Requirements

1. **ID mục tiêu.** `GoalsFromContract(chapter int, c ChapterContract) []ChapterGoal` sinh ID tất định theo thứ tự trong contract:
   - `rb1..rbN` cho `required_beats`;
   - `fm1..` cho `forbidden_moves`;
   - `cc1..` cho `continuity_checks`.

   Bỏ qua phần tử rỗng sau khi `TrimSpace`. Mỗi goal gồm `{id, kind, text}`. `evaluation_focus`/`emotion_target`/`payoff_points`/`hook_goal` **không** phải mục tiêu nhị phân, nên không có trong checklist; chúng vẫn được chấm ở 7 chiều.
2. **Context.** Khi chương có ít nhất một goal, `working_memory.chapter_goals = GoalsFromContract(...)`. Đặt cạnh `chapter_contract`, cùng điều kiện.
3. **Schema `save_review`.**
   - Thêm `goal_checks: array<{id: string, status: enum[pass, fail, unknown], quote: nullable string, note: string}>`, bắt buộc. Trả mảng rỗng khi không áp dụng.
   - Đổi thứ tự property của issue thành `type, chapters, quote, evidence, description, severity, requires_change, rule_ref, rule_text, rule, suggestion`.
   - Cập nhật mô tả `contract_status`: "scope=chapter có chapter_goals thì hệ thống tự suy ra từ goal_checks; điền null".
4. **Validation** trong `normalizeReviewEntry`. Mọi lỗi trả về `ErrToolArgs`, kèm danh sách ID cụ thể.
   - **Phạm vi bắt buộc.** `scope == "chapter"` và chương có goal thì bắt buộc nộp đúng tập ID, không thiếu, không trùng, không lạ. Các scope khác thì `goal_checks` phải rỗng.
   - **Quy tắc trích dẫn:**

     | Kind | `pass` | `fail` | `unknown` |
     |------|--------|--------|-----------|
     | `rb` (bước bắt buộc) | cần `quote` | `note` nói thiếu gì | cần `note` (vì sao không xác định được) |
     | `fm` (điều cấm) | không cần `quote` | cần `quote` | cần `note` |
     | `cc` (điểm liên tục) | `quote` tùy chọn | cần `quote` | cần `note` |

   - **Định vị.** Mọi `quote` khác null trong `goal_checks` và `issues` phải được `domain.LocateQuote` tìm thấy trong chính văn của ít nhất một chương của nó. Với issue là `issue.chapters`, rỗng thì lấy `r.Chapter`. Với goal là `r.Chapter`. Issue không có quote (ví dụ "thiếu cảnh X") thì không phải định vị.
   - **Chuẩn hóa khi so khớp:** gộp khoảng trắng, đưa ngoặc kép và ngoặc đơn cong/thẳng về một dạng, chuẩn hóa `…`/`...` và gạch ngang `—`/`–`/`-`. **Không** bỏ dấu tiếng Việt. Quote ngắn hơn 6 rune thì từ chối vì không đủ định danh.
5. **Suy ra trạng thái** bằng `DeriveContractStatus(goals, checks)`. Khi có `goal_checks`, giá trị này ghi đè `contract_status`/`contract_misses`:
   - mọi mục `pass` → `met`;
   - có `fail` ở `rb`/`fm` chiếm ≥ 50% số mục `rb+fm` → `missed`;
   - còn lại, nếu có `fail` hoặc `unknown` → `partial`;
   - `contract_misses` lấy từng mục `fail`/`unknown` theo dạng `"[rb2|fail] <text>"`.

   `unknown` **không bao giờ** được tính là đạt.
6. **Lưu trữ.** `ReviewEntry.GoalChecks []GoalCheck \`json:"goal_checks,omitempty"\``. Mỗi `GoalCheck` gồm `{id, kind, text, status, quote, note}`, trong đó `kind`/`text` do code điền từ snapshot contract lúc lưu. Review cũ không có trường này vẫn đọc được. `sameStoredReview` vẫn đúng với retry idempotent.
7. **Prompt `editor.md`.** Bổ sung §1:
   - đọc `chapter_goals`, chấm từng ID;
   - trích dẫn nguyên văn từ `read_chapter`;
   - không xác định được thì dùng `unknown`, không đoán;
   - không tự điền `contract_status` khi đã có goal.
8. **Web.**
   - Thêm `GoalCheck` vào DTO, chạy `go generate ./internal/entry/web`.
   - `ReviewPanel` hiển thị danh sách goal: ID, chip trạng thái, text, quote. Đặt ngay dưới khối contract hiện có.
   - Chạy `npm run typecheck && npm test && npm run build`, rồi commit `dist`.
9. **Eval.**
   - Bật `expect.review.goal_status` và metric `unknown_correct`.
   - Thêm 3 case kèm fixture:
     - `ed-goal-all-met`;
     - `ed-goal-beat-missing` (gold `rb2: fail`);
     - `ed-goal-unverifiable`: một `cc` cần lịch sử không có trong context, gold `unknown`.

## Implementation Steps

1. Viết `domain/goals.go` cùng test bảng: sinh ID có phần tử rỗng; các trường hợp suy ra `met`/`partial`/`missed`/`unknown`.
2. Viết `domain/quote.go` cùng test bảng: khớp sau chuẩn hóa ngoặc và khoảng trắng; không khớp khi khác dấu tiếng Việt; quote quá ngắn.
3. Context: thêm `chapter_goals`, cập nhật pin trong `novel_context_test.go` (`:293-309`).
4. Sửa schema, validation và suy ra trạng thái trong `save_review`. Cập nhật golden schema bằng `go test ./internal/tools -run TestSaveReviewSchemaGolden -update`, rồi review diff. Viết test hành vi:
   - thiếu, trùng hoặc lạ ID → lỗi nêu đúng ID;
   - quote không có trong chương → lỗi;
   - `rb` `pass` không có quote → lỗi;
   - scope `arc` mà gửi `goal_checks` → lỗi;
   - suy ra ghi đè giá trị model gửi;
   - review cũ đọc được.

   Sửa các fixture test hiện có đang dùng quote không có trong chương (`save_review_test.go`, `save_review_pack_test.go`, `save_review_address_test.go`) để chúng phản ánh hợp đồng mới. Không nới lỏng quy tắc để test cũ qua.
5. Rà consumer: `diag`, `ctxpack`, `novel_context` lesson. Chúng đọc `contract_status`/`contract_misses` đã suy ra, nên ngữ nghĩa giữ nguyên. Chạy test của các package này. Nếu test `vi_output_test.go:41-44,126-132` vỡ thì chỉ sửa khi do thay đổi định dạng `contract_misses`, và phải ghi lý do.
6. Sửa `editor.md`. Chạy A/B **tập dev** (repeat 3). Lưu bản tóm tắt an toàn (chỉ metric) vào `reports/p4-ab.md`. Chạy bổ sung case `ed-goal-*` trên worktree baseline; trước phase 04 tính năng này không tồn tại nên ghi "N/A ở baseline".
7. Web: DTO, sinh type, `ReviewPanel`, build. Kiểm tra trực quan bằng `serve` cộng trình duyệt trên **sách fixture tổng hợp** (không dùng bản thảo thật), chụp ảnh lưu vào `reports/`.
8. Cập nhật `docs/design/v1/11-api-contract.md` cho trường mới, quy tắc suy ra và quy tắc định vị quote.

## Test Matrix

| Lớp | Lệnh |
|-----|------|
| Domain | `go test ./internal/domain/...` |
| Tool | `go test ./internal/tools -run 'SaveReview\|ContextTool'` |
| Consumer | `go test ./internal/diag/... ./internal/agents/ctxpack/... ./internal/entry/web/...` |
| Kiểu sinh | `go generate ./internal/entry/web && git diff --exit-code web/src/api/types.gen.ts` (sau khi commit) |
| Web | `cd web && npm run typecheck && npm test && npm run build` |
| A/B | `eval --cases evals/cases/calibration/dev --repeat 3` |

## Success Criteria

- [x] Tập dev: `ed-goal-beat-missing` đạt `rb2: fail` ≥2/3 lần; `ed-goal-all-met` không có `fail` ≥2/3 lần. `ed-goal-unverifiable` (held-out, đáp án `unknown`) chỉ được kiểm ở phase 08.
- [x] `mechanical_valid` ≥ baseline. Nếu model hay gửi thiếu ID, thông điệp lỗi phải đủ để lần gọi sau sửa được: ≤ 1 lần lỗi trên mỗi review, tính trung bình.
- [x] `false_positive` trên các case đối chứng không tăng so với `p2-ab`.
- [x] `ReviewPanel` hiển thị goal checks, có ảnh chụp làm bằng chứng.

## Risk Assessment

- **Quy tắc định vị quote làm tăng số lần gọi tool lỗi.** Model hay trích dẫn sai một từ. Giảm thiểu bằng cách chuẩn hóa vừa đủ và trả thông điệp lỗi kèm đoạn gần nhất (nếu tìm được tiền tố ≥ 12 rune) để model chép lại đúng. Đo trong A/B.
- **Arc/global review không có checklist.** Đây là chủ đích: goal là hợp đồng của một chương. Ghi rõ trong docs.
- **Định dạng `contract_misses` đổi** (thêm tiền tố `[id|status]`). `vi_output_test` dùng số đếm chương chứ không dùng chuỗi, nên ảnh hưởng thấp. Kiểm bằng test.

## Rollback

Revert commit. `goal_checks` là `omitempty`, nên review mới vẫn đọc được bằng code cũ, chỉ là trường bị bỏ qua. Bản `dist` cũ được khôi phục cùng commit.
