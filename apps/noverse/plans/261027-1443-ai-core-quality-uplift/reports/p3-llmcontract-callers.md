# Rà caller `llmcontract.Execute` khi hết giới hạn tự sửa

Bối cảnh: `Execute` trước đây lặp vô hạn tới khi ctx kết thúc. Nay `Request.MaxCorrections`
(mặc định `defaultMaxCorrections = 3`) giới hạn số vòng sửa schema/nghiệp vụ; hết giới hạn
trả `&Failure{Kind: FailureContract, Raw: <raw cuối>, Err: …}`. Bảng dưới kiểm chứng từng
caller đi theo đường thất bại sẵn có — không panic, không treo.

| Caller | Đường thất bại khi nhận `FailureContract` | Kết luận |
|--------|--------------------------------------------|----------|
| `internal/arbiter/arbiter.go` (`decide`) | Lỗi được bọc `arbiter: %w` rồi trả nguyên. Các hàm `Decide*` truyền error lên; `DecideIntervention` (`intervention.go`) tuyên bố rõ: thất bại → trả error, không ghi gì ("宁可不动，不可误动"). | OK |
| `internal/host/imp/call.go` (`callStructured`) | `errors.As(&Failure)` → `FailureContract` (có `Raw`) → bọc `errSemantic{Raw, Err}` → `runner.saveFailure` (`runner.go:250-256`) ghi artifact vào `failures/` → `fail()` → sự kiện `StageError`. Nhánh chia đôi batch chỉ áp cho `errTruncated` (`segment.go:357-371`): lỗi semantic không chia đôi mà lan truyền + lưu artifact — đúng thiết kế sẵn có. | OK |
| `internal/host/sim/runner.go` (`generateStructured`) | Bọc `sinh kết quả có cấu trúc thất bại: %w`, trả nguyên cho `MergeSynthesis` (bọc thêm `đọc kết quả tổng hợp văn phong: %w`). | OK |
| `internal/ledger/architect.go` (`Analyze`) | Trả `err` nguyên cho caller. | OK |
| `internal/revision/analyze.go` (`Analyze`) | Bọc `phân tích bản sửa chương %d: %w`. | OK |
| `internal/userrules/normalize.go` (`Normalize`) | Bọc `Chuẩn hoá thất bại: %w`; `service.go:104-107` và `understand.go:29-35` đón lỗi bằng `degraded()` — giữ nguyên văn làm sở thích, đánh dấu `Degraded`. | OK |

Ghi chú:

- Không caller nào cần sửa: mọi đường đều trả error hoặc artifact, không retry vô hạn phía trên.
- `hooks.Correction` chỉ bắn khi thực sự gửi feedback (attempt ≤ MaxCorrections); lần cuối thất bại
  đi qua `Failure` trả về, không qua hook — log ở các caller vẫn nhất quán.
- Test pinning hành vi cũ được cập nhật: `arbiter_test.go` (`TestDecide_InvalidOutputStopsAfterCorrectionCap`)
  và `normalize_test.go` (`TestNormalize_FeedbackRetryStopsAfterCorrectionCap`).
