---
title: Plan AI core quality uplift (Pagekin lessons)
date: 2026-10-27
summary: "8-phase plan, red-team 16 findings accepted, 3 user decisions"
---

# Plan AI core quality uplift (Pagekin lessons)

## Chuyện gì đã xảy ra
- Plan `plans/261027-1443-ai-core-quality-uplift/` gồm 8 phase, input là `plans/reports/xia-pagekin-ai-core-compare.md`.
- 4 scout map seam local: eval, prompt/context, review/rewrite, LLM runtime. Sau đó 4 reviewer red-team đọc plan.
- Red-team và lượt rà toàn plan tìm ra 16 nhóm lỗi, chấp nhận cả 16. Có 7 lỗi Critical:
  - `start` + `PauseAfterStep` có race;
  - vòng recheck tự mâu thuẫn;
  - quote bắt buộc làm hỏng import/`/sync`;
  - từ chối thao tác lên phục bút đã đóng làm vỡ replay;
  - gate ở `validateCommitArgs` kẹt pending;
  - đường dẫn sai;
  - import publish qua `commit_chapter`.

## Quyết định
- D7: gate chỉ chặn tai nạn cơ học, còn lại chỉ báo qua `internal/rules`.
- D8: recheck dùng `save_review` (re-review), cận trên suy ra từ dữ liệu.
- D9: quote của fact là tùy chọn, có thì kiểm.
- Bỏ 2.9 và 2.11 (có bằng chứng). Giới hạn vòng sửa của `llmcontract` là 3.
- Eval tách dev/held-out, gom kết quả theo tỉ lệ pass. Báo cáo chỉ commit metric.

## Bài học
- Report xia có claim sai ("hard fail" context budget). Phải kiểm lại bằng code trước khi đưa vào plan.
- Import publish qua tool `commit_chapter`, nên mọi gate ở commit đều ảnh hưởng import.

## Next steps
- `/ak:cook plans/261027-1443-ai-core-quality-uplift/plan.md`, bắt đầu phase 01 (harness + baseline).

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
