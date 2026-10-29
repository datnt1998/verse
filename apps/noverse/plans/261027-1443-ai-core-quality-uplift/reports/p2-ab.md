# A/B — prompt authority pack

- Tập: `evals/cases/calibration/dev` (12 case), `--repeat 3`. Chỉ dùng tập dev, held-out không tham gia.
- Variant: `evals/variants/p2-authority` gồm `writer.md`, `editor.md`, `architect-long.md`, `architect-short.md`.
- Báo cáo thô: `workspace/evals/p2-ab/` (vòng 1) và `workspace/evals/p2-ab-r2/` (vòng 2), đều gitignore.

## Vòng 1

Chạy trên fixture gốc. Arm variant gắn `error` cho đúng những đoạn mà arm baseline cũng gắn, và các đoạn này lặp lại qua mọi lượt. Đối chiếu chính văn cho thấy đây là lỗi thật của bốn fixture control: lộ trước kế hoạch tương lai, mâu thuẫn vị trí đồ vật/không gian, và cơ chế báo động không khả thi. Kết luận: vòng 1 đo lỗi fixture, không đo prompt. Đã sửa fixture (xem `baseline.md`) và rebase `editor.md` của variant lên prompt hiện hành (goal checklist, kiểm lại) trước khi chạy vòng 2.

## Vòng 2 (quyết định)

| Metric | Baseline | Variant |
|---|---|---|
| pass (lượt không hard fail) | 23/36 | 30/36 |
| critical_hit | 9/9 | 9/9 |
| false_positive | 11 | 7 |
| writer_violations | 2 | 0 |
| tool_errors | 0 | 0 |
| chi phí | $2.35 | $2.44 |

| Case | Control | Baseline | Variant |
|---|:-:|:-:|:-:|
| ed-clean-01 | ✓ | 1/3 | 3/3 |
| ed-ctrl-future-plan | ✓ | 0/3 | 0/3 |
| ed-ctrl-lie | ✓ | 2/3 | 3/3 |
| ed-goal-all-met | ✓ | 1/3 | 0/3 |
| ed-goal-beat-missing | | 3/3 | 3/3 |
| ed-knowledge-leak | | 3/3 | 3/3 |
| ed-missing-history | ✓ | 0/3 | 3/3 |
| ed-recheck-unfixed | | 3/3 | 3/3 |
| ed-timeline-contradiction | | 3/3 | 3/3 |
| wr-intent-vs-arrival | | 1/3 | 3/3 |
| wr-predecessor-replay | | 3/3 | 3/3 |
| wr-relative-time | | 3/3 | 3/3 |

Quy tắc promote:
- critical_hit không giảm: đạt.
- false_positive không tăng: đạt (11 → 7).
- writer_violations không tăng: đạt (2 → 0).
- tool_errors tăng không quá 10%: đạt (0 → 0).
- Có case mục tiêu tăng ≥ 2/3: đạt, `wr-intent-vs-arrival` tăng từ 1/3 lên 3/3.

**Promote cả bốn file.** `simulationGuidanceVI` và mô tả property của `chapterfacts` được áp thẳng theo `p2-nonprompt-proposals.md`.

Hai case control không cải thiện là `ed-ctrl-future-plan` và `ed-goal-all-met`. Ở cả hai arm, issue `error` đều trỏ vào cùng một số đoạn chính văn, và mỗi đoạn là một lỗi thật còn sót trong fixture:
- vị trí di ảnh và hộp sắt lệch giữa chương 2 và chương 3;
- vị trí lọ quế mô tả lệch nhau;
- cần gạt phụ được tả là không ai chạm từ mùa nước trước, mâu thuẫn với lịch sử mở trộm ở chương 2;
- nhân vật định "chụp" chữ in trong khi thiết lập thế giới không có máy ảnh.

Đã sửa chính văn các fixture này sau vòng 2. Các case này sẽ được đo lại ở A/B kế tiếp và ở lần chạy cuối.

Thêm vào `writer.md` khi promote, không qua A/B: một câu mô tả hợp đồng `mode=append`. Câu này chỉ diễn đạt lại hành vi cơ học của tool, gồm cắt phần mở đầu trùng với cuối bản nháp và từ chối khi nội dung đã có sẵn.
