# A/B cuối — kết quả đầy đủ (chỉ metric)

Trạng thái: **chưa đạt; đã đo xong**. Held-out editor không cải thiện. Writer không hồi quy trên cả hai tập. Smoke pipeline không kết luận được: cả bốn lần chạy architect ở hai arm đều chạm guard 30 phút. Với architect_long là do lỗi stream của provider; với architect_short là do mỗi bước sinh mất 4–8 phút, không có lỗi provider.

## Thiết lập

| Mục | Baseline | HEAD |
|---|---|---|
| Mã | `334d7aa` (worktree baseline) | `ec7baab` (editor); `627a0e4` (writer + smoke rerun — chỉ khác dòng prompt `hook_type` và validator architect, prompt writer y hệt) |
| Bộ case | bản đông lạnh của các case dùng được ở baseline (17 case) | `evals/cases/calibration` (22 case) |
| Fixture | `evals/fixtures` hiện hành (cả hai arm dùng chung) | như baseline |
| Model | editor/architect `gpt-6.1-sol`, writer `gpt-6-luna` (openai-codex) | như baseline |
| Repeat | 3 | 3 |

- Baseline chạy xong trọn vẹn.
- HEAD: 16 case editor chạy xong ở `ec7baab`. Lần chạy đầu, từ khoảng 11:16 provider trả HTTP 429 (usage limit), nên toàn bộ 6 case writer đều hỏng vì quota, không phải vì mã. Sau khi quota hồi, 6 case writer (repeat 3) và smoke pipeline đã chạy lại xong bằng binary `627a0e4`, không còn 429 nào.
- Tiến trình HEAD đã bị dừng trước khi ghi `report.json`. Kết quả được chấm lại offline bằng `eval.Collect` + `eval.Grade` trên artifact đã lưu. Outcome chấm lại khớp 48/48 run với outcome in ra lúc chạy.
- Lần chạy HEAD trước đó bằng binary cũ (trước các sửa evidence, `due_by`, recheck) đã bị huỷ và **không dùng**. Lần đó đã chạy 12 case, trong đó có 4 case held-out editor. Kết quả của nó không được đọc để tinh chỉnh gì.

## Editor — 12 case so được (loại 4 case chỉ có ở HEAD)

| Metric | dev baseline | dev HEAD | held-out baseline | held-out HEAD |
|---|---|---|---|---|
| Run pass (PASS+WARN) | 12/18 | 18/18 | 7/18 | 7/18 |
| `critical_hit` | 6/6 | 6/6 | 8/9 | 7/9 |
| `false_positive` (tổng) | 8 | 0 | 16 | 21 |
| `false_positive` trên case đối chứng | 4 | 0 | 12 | 17 |
| `tool_errors` | 0 | 0 | 0 | 0 |
| `worker_runs` editor | 18 | 18 | 18 | 18 |
| tool call / run | 5.00 | 4.89 | 5.00 | 4.94 |
| token (in+out) / run | 97,769 | 104,621 (+7.0%) | 94,864 | 102,619 (+8.2%) |
| chi phí / run (USD) | 0.0798 | 0.0843 (+5.6%) | 0.0883 | 0.0926 (+4.8%) |
| `usage_missing` | 0 | 0 | 0 | 0 |

### Tỉ lệ pass từng case

| Tập | Case | Baseline | HEAD | Kết luận |
|---|---|---|---|---|
| dev | ed-clean-01 | 0/3 | 3/3 | cải thiện |
| dev | ed-ctrl-future-plan | 3/3 | 3/3 | — |
| dev | ed-ctrl-lie | 3/3 | 3/3 | — |
| dev | ed-knowledge-leak | 3/3 | 3/3 | — |
| dev | ed-missing-history | 0/3 | 3/3 | cải thiện |
| dev | ed-timeline-contradiction | 3/3 | 3/3 | — |
| held-out | ed-clean-02 | 0/3 | 0/3 | — |
| held-out | ed-ctrl-flashback | 0/3 | 0/3 | — |
| held-out | ed-ctrl-indirect-transfer | 0/3 | 1/3 | không phân biệt được |
| held-out | ed-dead-returns | 3/3 | 3/3 | — |
| held-out | ed-injection-in-prose | 1/3 | 0/3 | không phân biệt được |
| held-out | ed-timeline-contradiction-2 | 3/3 | 3/3 | — |

### Case chỉ có ở HEAD (N/A ở baseline, loại khỏi delta)

| Tập | Case | HEAD |
|---|---|---|
| dev | ed-goal-all-met | 3/3 |
| dev | ed-goal-beat-missing | 3/3 |
| dev | ed-recheck-unfixed | 3/3 |
| held-out | ed-goal-unverifiable | 0/3 (`unknown_correct` 0/3) |

## Writer — 5 case so được (HEAD-only `wr-thread-overdue` tách riêng)

`quote_coverage` không tồn tại ở binary baseline `334d7aa` (metric mới) nên cột baseline ghi N/A. Tỉ lệ pass = PASS+WARN trên 3 run. Delta token/chi phí tính trên các run dùng được ở cả hai arm (dev 9 run, held-out 6 run).

| Metric | dev baseline | dev HEAD | held-out baseline | held-out HEAD |
|---|---|---|---|---|
| Run pass (PASS+WARN) | 8/9 | 9/9 | 3/6 | 3/6 |
| `writer_violations` | 1 | 0 | 3 | 3 |
| `quote_coverage` | N/A | 41/41 (100%) | N/A | 39/40 (98%) |
| `worker_runs` writer | 9 | 9 | 6 | 6 |
| `tool_errors` | 0 | 0 | 0 | 0 |
| token (in+out) / run | 172,150 | 206,863 (+20.2%) | 194,847 | 242,492 (+24.5%) |
| chi phí / run (USD) | 0.0076 | 0.0100 (+31.5%) | 0.0084 | 0.0095 (+13.6%) |
| `usage_missing` | 0 | 0 | 0 | 0 |

### Tỉ lệ pass từng case

| Tập | Case | Baseline | HEAD | Kết luận |
|---|---|---|---|---|
| dev | wr-intent-vs-arrival | 2/3 | 3/3 | cải thiện |
| dev | wr-predecessor-replay | 3/3 | 3/3 | — |
| dev | wr-relative-time | 3/3 | 3/3 | — |
| held-out | wr-dedupe-events | 0/3 | 0/3 | — |
| held-out | wr-premature-future | 3/3 | 3/3 | — |

### Case chỉ có ở HEAD (N/A ở baseline, loại khỏi delta)

| Tập | Case | HEAD |
|---|---|---|
| dev | wr-thread-overdue | 3/3 (3 × WARN), `writer_violations` 0, `quote_coverage` 22/22 |

## Smoke pipeline

| Case | HEAD `ec7baab` | HEAD `627a0e4` (chạy lại) | Baseline `334d7aa` (chạy lại) |
|---|---|---|---|
| architect_long | PASS | FAIL: chạm guard 30 phút ở phase `premise` (đã lưu premise, characters, world_rules, compass); mất ≥10 phút vì 2 lần stream idle timeout 5 phút + 1 lỗi `INTERNAL_ERROR` của provider | FAIL: chạm guard 30 phút ở phase `outline` (layered_outline lưu ở phút 28,6); 2 lỗi `INTERNAL_ERROR` của provider, trước đó ~12 phút không có sự kiện |
| architect_short | FAIL: quá 30 phút; nền tảng đã lưu đủ, đang sửa theo audit | FAIL: chạm guard 30 phút ở phase `outline`; 0 lỗi provider; đại cương lưu ở phút 25,8, audit yêu cầu sửa, đang sửa lượt 2 thì hết giờ (cùng dạng với `ec7baab`) | FAIL: chạm guard 30 phút ở phase `outline`; 0 lỗi provider; đại cương lưu ở phút 26,7 và lưu lại ở phút 29,8; 1 tool error |
| writer_first_chapter | PASS | PASS (18 tool call, 0 tool error) | WARN: 1 warning về vị trí cảnh lời hứa |

Ở `ec7baab`, architect_short mất một lượt sinh đại cương (~7 phút) vì prompt cho phép một giá trị `hook_type` mà validator từ chối; `627a0e4` thống nhất hợp đồng này. Ở lần chạy lại, mỗi lời gọi `gpt-6.1-sol` mất 4–8 phút, nên một lượt lập nền tảng (book → premise → characters → world_rules → outline) đã tốn ~26 phút ở cả hai arm. architect_short hỏng vì chậm, không vì lỗi provider. Riêng architect_long còn mất thêm thời gian vì stream lỗi ở cả hai arm. Cả hai arm cùng không vượt được 30 phút, nên không quy cho thay đổi mã. Việc sửa `hook_type` chỉ được kiểm chứng trên `final-architect-short-627a0e4` (đại cương nhận ngay lần đầu, vẫn quá 30 phút).

## Đối chiếu tiêu chí thành công

| Tiêu chí | dev | held-out |
|---|---|---|
| `critical_hit` tăng | không đổi (trần 6/6) | **trượt** (8/9 → 7/9) |
| `false_positive` không tăng | đạt (8 → 0) | **trượt** (16 → 21) |
| `writer_violations` không tăng | đạt (1 → 0) | đạt (3 → 3) |
| token mỗi chương ≤ +25% | đạt (editor +7.0%, writer +20.2%) | đạt (editor +8.2%, writer +24.5%) |
| `tool_errors` ≤ +10% | đạt (0 → 0) | đạt (0 → 0) |
| không case PASS cũ chuyển FAIL | đạt (writer: 3 case PASS giữ nguyên) | đạt theo ngưỡng 2/3 |
| smoke cũ PASS | writer_first_chapter PASS; architect_* không kết luận được (cả hai arm chạm guard 30 phút: architect_long do lỗi stream của provider, architect_short do model chậm) | — |

Dev cải thiện rõ, còn held-out không cải thiện và báo nhầm nhiều hơn. Theo quy tắc rủi ro của phase 08, đây là dấu hiệu overfit vào dev, nên **không đánh dấu phase 08 hoàn tất**. Thay đổi trên từng case held-out đều dưới 2/3 (không phân biệt được). Nhưng gộp lại, cả hai tiêu chí held-out về `critical_hit` và `false_positive` đều trượt.

## Quét provenance

- Đã quét 58 cụm đặc trưng (32 bản gốc, 26 bản dịch tiếng Việt) trên `assets/` và `internal/`. Bước trích cụm do một agent tách biệt làm, để người triển khai không đọc văn bản nguồn.
- Không có cụm nào trùng ở mức câu hay cụm từ.
- Có 5 neo trùng **cấu trúc quy tắc**: thứ bậc nguồn sự thật, tách sự thật khỏi ý định, quy tắc không chép văn mẫu.
  - Thứ bậc nguồn sự thật là cơ chế mà plan chủ đích học.
  - `assets/prompts/simulation-source.md` có từ trước plan (upstream 2026-08-31).
- 28 neo không phải test và 12 neo trong test chỉ dùng từ vựng chung của lĩnh vực.
- Ghi nguồn ý tưởng nằm ở `docs/architecture.md`.

## Việc còn lại

1. Writer đã đo đủ. Smoke architect_* cần chạy lại khi `gpt-6.1-sol` nhanh lại như buổi sáng (lúc đó architect_long PASS) hoặc với `--timeout` dài hơn cho cả hai arm, để đối chiếu `hook_type`. Hiện cả hai arm cùng hết giờ nên không phân biệt được mã.
2. Held-out editor: các báo nhầm tập trung ở mức `error` trên chương sạch hoặc chương đối chứng (`consistency`/`continuity`/`pacing`). Mục tiêu không kiểm chứng được bị chấm `pass` thay vì `unknown`. Muốn sửa phải tune lại trên dev. Held-out chỉ được đo lại sau khi đã chốt thay đổi.
