// Package domain chứa từ vựng dùng chung của Viderse: kiểu giá trị và khoá
// cài đặt mà nhiều mảng nghiệp vụ tham chiếu. Không có I/O, không phụ thuộc
// vào web hay persistence — mọi gói khác được phép import domain.
package domain

// Khoá của bảng settings. Tên giữ không gian tên của yt-factory để khi port
// từng mảng (queue, scan, hạn mức ảnh…) khỏi phải dịch lại khoá.
const (
	// KeyQueuePaused là cờ tạm dừng hàng chờ làm video (boolean JSON; thiếu
	// hoặc sai kiểu đọc là false), tương đương "clone.queuePaused" của bản TS.
	KeyQueuePaused = "clone.queuePaused"
)

// AttentionKind là loại mục cần xử lý hiện trên chấm bánh răng — tập giá trị
// cố định theo hợp đồng của GET /api/status.
type AttentionKind = string

const (
	AttentionToolUpdate   AttentionKind = "tool_update"
	AttentionLoginExpired AttentionKind = "login_expired"
	AttentionQuotaLow     AttentionKind = "quota_low"
	AttentionToolFailed   AttentionKind = "tool_failed"
)
