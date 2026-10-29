# P2 — Đề xuất văn bản ngoài variant (promote thẳng)

Các khối dưới đây không nằm trong file prompt `*.md` nên không đi qua
`evals/variants/p2-authority/` (variant loader chỉ nhận `*.md`,
`internal/eval/eval.go:188-208`). Khi promote, áp trực tiếp vào code.
Văn bản tự soạn tiếng Việt; tham chiếu cơ chế:
`plans/reports/xia-pagekin-ai-core-compare.md` (P1 1.4, 1.6).

## Req 1.6 — `simulationGuidanceVI` (`assets/load.go:303-307`)

Thêm một câu vào cuối đoạn "Nguyên tắc sử dụng":

```
Hồ sơ mô phỏng chỉ chi phối cách diễn đạt; khi xung đột, thứ tự ưu tiên là fact và thiết lập của tác giả > nhân quả của chương > độ dài > văn phong.
```

Đoạn đầy đủ sau khi thêm:

```
Nguyên tắc sử dụng: Học hỏi cấu trúc, nhịp điệu, móc câu, cách giải phóng thông tin và thủ pháp cuốn hút độc giả; tuyệt đối không sao chép câu văn nguyên văn, tên nhân vật, địa danh, thiết lập độc quyền hay phân đoạn cố định. Nếu simulation_profile xung đột với yêu cầu rõ ràng của người dùng, ưu tiên tuân thủ yêu cầu của người dùng. Hồ sơ mô phỏng chỉ chi phối cách diễn đạt; khi xung đột, thứ tự ưu tiên là fact và thiết lập của tác giả > nhân quả của chương > độ dài > văn phong.
```

## Req 1.4 — mô tả property trong `internal/chapterfacts/facts.go`

Chỉ đổi chuỗi mô tả, không đổi cấu trúc schema:

- `summary` (dòng 56):
  `"Tóm tắt chương"` →
  `"Tóm tắt chương: chỉ kể việc đã xảy ra trong chương, không ghi dự định hay suy đoán"`

- `key_events` (dòng 58):
  `"Sự kiện then chốt"` →
  `"Sự kiện then chốt đã xảy ra trong chương; dự định, nghi ngờ, phủ nhận hay sự kiện bị hủy không ghi vào đây"`

- `timeline_events` (dòng 59):
  `"Sự kiện dòng thời gian"` →
  `"Sự kiện dòng thời gian đã thực sự xảy ra trong chương; `time` quy đổi từ chỉ thời gian tương đối theo mốc đã rõ trong chính văn; không ghi dự định, nghi ngờ, phủ nhận hay sự kiện bị hủy"`

- `state_changes` (dòng 62):
  `"Thay đổi trạng thái"` →
  `"Thay đổi trạng thái đã thực sự xảy ra trong chương; có đính chính trong cùng chương thì ghi bản sau cùng; ghi rõ ai biết thông tin, ai sở hữu vật"`
