package audio

import (
	"errors"
	"fmt"
	"strings"
)

// 状态冲突：调用方请求的操作在当前状态下不合法（chưa chốt / chưa có giọng / đã xong…）。
// handler 把它映射成 409，并把 Msg 原样给界面（越南语、面向用户）。
type StateError struct{ Msg string }

func (e *StateError) Error() string { return e.Msg }

// conflict 构造一个 409 文案。
func conflict(format string, args ...any) error {
	return &StateError{Msg: fmt.Sprintf(format, args...)}
}

// ErrNoModel 表示故事没有可用的 AI 配置（soạn bản đọc需要模型）。
var ErrNoModel = fmt.Errorf("Chưa cấu hình mô hình AI cho truyện này — không soạn được bản đọc.")

func renderFailureReason(err error) string {
	var state *StateError
	if errors.As(err, &state) {
		return state.Msg
	}
	if errors.Is(err, ErrEngineMissing) {
		return "Máy đọc chưa sẵn sàng. Kiểm tra bộ đọc trên máy chủ rồi thử lại."
	}
	if errors.Is(err, ErrFFmpegMissing) {
		return "Máy chủ chưa có công cụ ghép âm thanh. Cài công cụ trên máy chủ rồi thử lại."
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "out of memory") || strings.Contains(message, "cuda") && strings.Contains(message, "memory") {
		return "Máy chủ hết bộ nhớ đồ hoạ khi đọc một đoạn dài. Phần đã tạo vẫn giữ."
	}
	return "Máy chủ chưa đọc được đoạn này. Phần đã tạo vẫn giữ; kiểm tra bộ đọc trên máy chủ rồi thử lại."
}
