package host

import "fmt"

// OperationErrorMessage uses only classified kinds and numeric story context.
// Raw tool results, summaries, arguments and provider errors are never copied.
func OperationErrorMessage(ev Event) string {
 context := ""
 switch {
 case ev.Chapter > 0:
  context = fmt.Sprintf(" ở chương %d", ev.Chapter)
 case ev.Scope == "arc" && ev.Arc > 0:
  context = fmt.Sprintf(" ở cung %d", ev.Arc)
 case ev.Scope == "volume" && ev.Volume > 0:
  context = fmt.Sprintf(" ở tập %d", ev.Volume)
 }
 switch ev.Kind {
 case "tool_validation", "validation":
  return "Thông tin thao tác" + context + " chưa hợp lệ. Trợ lý sẽ kiểm tra và thử lại."
 case "timeout", "stream_idle":
  return "Thao tác" + context + " đã quá thời gian chờ. Trợ lý sẽ kiểm tra và thử lại."
 case "network", "provider":
  return "Không kết nối được nhà cung cấp AI. Trợ lý sẽ kiểm tra và thử lại."
 case "auth":
  return "Nhà cung cấp AI từ chối xác thực. Hãy kiểm tra cấu hình nhà cung cấp trước khi thử lại."
 case "quota", "rate_limit", "overloaded":
  return "Nhà cung cấp AI chưa thể đáp ứng yêu cầu. Trợ lý sẽ thử lại sau."
 case "blocked", "circuit", "business", "tool", "tool_error":
  return "Thao tác" + context + " chưa thể thực hiện. Trợ lý sẽ kiểm tra và thử lại."
 default:
  return "Thao tác không thành công. Trợ lý tự xem lỗi và chạy lại ở thẻ kế tiếp."
 }
}
