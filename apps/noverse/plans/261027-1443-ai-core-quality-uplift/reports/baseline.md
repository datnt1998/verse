# Baseline — eval calibration

- Commit: `334d7aa` (worktree `workspace/baseline-wt`)
- Provider: `openai-codex`
- Model: editor/architect `gpt-6.1-sol`, writer `gpt-6-luna`
- Lệnh: `eval --cases evals/cases/calibration --repeat 3 --ci --timeout 20m`
- Run ID: `20261027-233648`. Báo cáo thô: `workspace/evals/baseline-20261027/` (gitignore)
- Quy ước pass của một lượt: không có hard fail. Lượt WARN vẫn tính là pass vì mọi assertion đều đạt; warning của lượt đó là chẩn đoán phụ, ví dụ "chương chờ viết lại" sau khi editor chấm đúng `rewrite`.

## Metric tổng hợp (17 case × 3)

| Metric | Giá trị |
|---|---|
| mechanical_valid | 34/36 (0.94) |
| critical_hit | 13/15 (0.87) |
| false_positive | 30 |
| unknown_correct | N/A ở baseline |
| writer_violations | 3 |
| tool_errors | 1 |
| worker_runs | editor 36, writer 15. Mọi lượt có đúng 1 worker |
| lỗi runtime | 2, đều do mất mạng tới provider (dial timeout) ở `ed-ctrl-lie` |
| token | input 6 949 557 · output 205 273 · cache read 5 468 288 |
| chi phí | $3.10 |

## Tỉ lệ pass từng case

| Tập | Case | Control | Pass |
|---|---|:-:|:-:|
| dev | ed-clean-01 | ✓ | 0/3 |
| dev | ed-ctrl-future-plan | ✓ | 0/3 |
| dev | ed-ctrl-lie | ✓ | 0/3 |
| dev | ed-knowledge-leak | | 3/3 |
| dev | ed-missing-history | ✓ | 0/3 |
| dev | ed-timeline-contradiction | | 3/3 |
| dev | wr-intent-vs-arrival | | 3/3 |
| dev | wr-predecessor-replay | | 3/3 |
| dev | wr-relative-time | | 3/3 |
| heldout | ed-clean-02 | ✓ | 0/3 |
| heldout | ed-ctrl-flashback | ✓ | 0/3 |
| heldout | ed-ctrl-indirect-transfer | ✓ | 2/3 |
| heldout | ed-dead-returns | | 3/3 |
| heldout | ed-injection-in-prose | | 1/3 |
| heldout | ed-timeline-contradiction-2 | | 3/3 |
| heldout | wr-dedupe-events | | 0/3 |
| heldout | wr-premature-future | | 3/3 |

Gate: FAIL. Mọi case trap đều bắt được lỗi được cài. Fail tập trung ở case control: editor gắn `error` cho đoạn văn không có lỗi được cài, và `wr-dedupe-events`.

## Sửa fixture sau baseline

Đối chiếu issue `error` lặp lại giữa các lượt cho thấy bốn fixture control ở tập dev có lỗi thật trong chính văn. Editor bắt đúng các lỗi này, nên false positive của bốn case trên phản ánh lỗi fixture chứ không phản ánh prompt:

- `ed-ctrl-future-plan`: chương 3 thực hiện sớm móc của chương 4. Vị trí di ảnh lệch với chương 1.
- `ed-ctrl-lie`: chương 3 kể trước sự kiện cốt lõi của chương 5. Mẹ chỉ vào ống quần đã thay. Cụm "im lặng" vượt `fatigue_words`.
- `ed-missing-history`: nhân vật nhìn tấm ảnh đã cất vào ví đóng.
- `ed-clean-01`: hướng cửa lớn/đồi chùa mâu thuẫn với chương 2. Cơ chế báo động gõ lan can không làm dây giật.

Đã sửa chính văn của bốn fixture này. Từ đây, số liệu so sánh cho tập dev lấy từ arm baseline của A/B chạy trên fixture đã sửa. Fixture held-out giữ nguyên tới phase 08.
