package scan

// Câu hướng dẫn khôi phục hiển thị cho người dùng (RECOVERY_TEXT của bản TS,
// nguyên văn tiếng Việt).
const (
	// RecoveryBlocked: YouTube yêu cầu xác minh hoặc chặn truy cập.
	RecoveryBlocked = "YouTube yêu cầu xác minh hoặc chặn truy cập. Hãy thử lại sau ít phút rồi chọn Tiếp tục; ứng dụng không tự đổi tài khoản hay proxy."
	// RecoveryFailures: thử lại 5 lần liên tiếp không thành công.
	RecoveryFailures = "Đã thử lại 5 lần liên tiếp nhưng chưa thành công. Kiểm tra kết nối rồi chọn Tiếp tục; tiến độ đã lưu được giữ nguyên."
	// RecoveryRestarts: phiên duyệt hết hạn nhiều lần.
	RecoveryRestarts = "Phiên duyệt đã hết hạn nhiều lần. Chọn Tiếp tục để quét lại từ đầu; các video đã lưu được giữ nguyên."
	// RecoveryGap: không gặp video đã biết trong 10.000 video mới.
	RecoveryGap = "Chưa gặp video đã biết sau 10.000 video mới. Cần kiểm tra thủ công trước khi mở rộng giới hạn tìm kiếm gần đây."
	// RecoveryFatal: trang YouTube không đọc được.
	RecoveryFatal = "Dữ liệu trang YouTube không đọc được. Chọn Tiếp tục để thử lại hoặc huỷ."
)
