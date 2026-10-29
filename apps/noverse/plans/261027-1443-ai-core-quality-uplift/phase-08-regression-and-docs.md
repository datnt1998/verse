---
title: "Phase 8: Regression, A/B cuối, docs"
status: todo
priority: P1
effort: "1.5 ngày"
depends_on: [1, 2, 3, 4, 5, 6, 7]
---

# Phase 8: Regression, A/B cuối, docs

## Overview

Phase này chứng minh toàn bộ gói thay đổi nâng chất lượng mà không gây hồi quy, chạy **held-out lần đầu tiên kể từ baseline**, rồi đồng bộ tài liệu. Không thêm tính năng mới.

## File ownership

| Sửa |
|-----|
| `plans/261027-1443-ai-core-quality-uplift/reports/final-ab.md` (tạo mới, chỉ chứa metric) |
| `docs/evaluation-system.md`, `docs/context-management.md`, `docs/architecture.md`, `docs/design/v1/06-data-contract.md`, `docs/design/v1/11-api-contract.md`, `docs/chapter-advance-gate.md` |
| `plans/reports/xia-pagekin-ai-core-compare.md` (đính chính) |
| `NOTICE` hoặc `docs/` (ghi nguồn tham khảo ý tưởng; xem bước 6) |

## Implementation Steps

1. **Regression code.** Chạy lần lượt:
   - `go vet ./...`
   - `go test -count=1 ./...`
   - `go test -race -count=1 ./internal/host ./internal/store ./internal/tools ./internal/flow`
   - `go generate ./internal/entry/web && git diff --exit-code web/src/api/types.gen.ts`
   - `cd web && npm run typecheck && npm test && npm run build`, sau đó `git diff --exit-code internal/entry/web/dist`. Nếu có diff thì commit bản build.
2. **Smoke pipeline cũ.** Chạy `go run ./cmd/ainovel-cli eval --cases evals/cases/smoke --config <cfg>`. Mọi case trước đây PASS vẫn phải PASS: không kẹt router, không chạm guard bế tắc, sách vẫn chuyển sang `complete` được.
3. **A/B cuối, trên cùng quần thể case.**
   - **HEAD:** chạy `eval --cases evals/cases/calibration --repeat 3`, gồm cả dev và held-out.
   - **Baseline:** tạo `git worktree add workspace/baseline-wt <baseline-sha>`, chép bộ case và fixture hiện tại vào worktree, chạy cùng lệnh. Case dùng tính năng chưa có ở baseline (`goal_status`, `recheck_status`, `context_contains`) được ghi `N/A ở baseline` và loại khỏi delta.
   - **Lưu `final-ab.md`**, chỉ chứa metric, không có văn bản hay lỗi thô:
     - delta của từng metric, tách dev và held-out;
     - tỉ lệ pass của từng case;
     - `worker_runs`, `tool_errors`, `quote_coverage`;
     - token và chi phí (ghi "không kết luận được" nếu `usage_missing > 0`).
   - Chạy secret scan trước khi commit, như phase 01.
   - **Đối chiếu tiêu chí thành công** trong `plan.md`. Tiêu chí nào trượt thì **không** đánh dấu xong; ghi nguyên nhân rồi báo user.
   - **Dọn dẹp:** `git worktree remove workspace/baseline-wt`.
4. **Smoke thật trên một sách mới.**
   - Chạy `serve` trên cổng cố định, kiểm `lsof -i :PORT` trước khi khởi động.
   - Tạo một sách auto khoảng 5 chương.
   - Quan sát:
     - chính văn không chứa `<think>` hay lời bình meta;
     - review có `goal_checks`;
     - phục bút có `quote`/`evidence` (đếm tỉ lệ);
     - recheck chạy sau mỗi lần viết lại theo review;
     - `worker_runs` của editor trên mỗi lần viết lại không vượt 1 lần recheck.
   - Chụp ảnh `ReviewPanel`. Dừng `serve`.
5. **Đính chính báo cáo xia.** Thêm mục "Đính chính sau khi lập plan" vào `plans/reports/xia-pagekin-ai-core-compare.md`:
   - ngân sách context là **mềm**, không hard fail (`internal/tools/novel_context.go:217-246`);
   - 2.9 và 2.11 bị loại (D1, D2);
   - span lưu dạng quote, không lưu offset;
   - recheck dùng `save_review`.
6. **Kiểm tra provenance và giấy phép.**
   - **Quét chuỗi:** lấy các cụm đặc trưng trong prompt mà Pagekin trích ở `plans/reports/xia-pagekin/01-prompt-pipeline.md` và `04-planning-import.md`, dịch sang tiếng Việt theo cách sát nghĩa nhất, rồi `rg -n` trên `assets/` và `internal/`. Ghi lại số kết quả.

     Lưu ý: quét chuỗi không chứng minh được việc không phái sinh. Bằng chứng chính là quy trình viết độc lập từ mô tả cơ chế (phase 02) và ghi chú nguồn gốc trong commit message.
   - **Ghi nguồn ý tưởng** trong `docs/` hoặc `NOTICE`: "Một số cơ chế kiểm chứng chất lượng được tham khảo ở mức ý tưởng từ Pagekin (EthanYoQ/AI-Novel-Writer, GPL-3.0); không sao chép mã hay văn bản." Cook hỏi user trước khi ghi vào `NOTICE`.
7. **Docs.** Đọc từng file trong ownership, sửa các đoạn lệch với hành vi mới, kiểm link tương đối. Nội dung cần có:
   - quy tắc ranh giới tin cậy;
   - thứ tự trim;
   - vòng đời recheck;
   - thay đổi no-op trên phục bút đã đóng.
8. **Dọn dẹp.**
   - Xóa `evals/variants/p*` còn sót.
   - `rg -n 'PreviousTailChars' internal/bootstrap internal/eval` không được còn kết quả.
   - Dừng mọi `serve`/eval đã khởi động.

## Success Criteria

- [ ] Mọi lệnh ở bước 1 xanh, smoke cũ PASS.
- [ ] `final-ab.md` chứng minh đạt các tiêu chí thành công của `plan.md` **trên cả held-out**, hoặc nêu rõ tiêu chí nào trượt.
- [ ] Đã ghi nguồn ý tưởng và kết quả quét chuỗi.
- [ ] Docs khớp với code; báo cáo xia đã được đính chính.

## Risk Assessment

- **Nhiễu model.** Với case nằm sát ngưỡng, chạy thêm 2 lần (tổng repeat 5), không chọn lọc kết quả. Nếu held-out xấu đi trong khi dev tốt lên, kết luận là overfit và báo user, không coi là xong.

## Rollback

Không áp dụng; phase này chỉ kiểm chứng và sửa tài liệu.
