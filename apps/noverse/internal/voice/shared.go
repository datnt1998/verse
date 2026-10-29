package voice

import (
	"sync"
)

var (
	sharedMu       sync.Mutex
	shared         *Service
	configProvider func() Config
)

// Shared trả Service dùng chung của tiến trình (gateway, TUI, CLI đều dùng một).
// Cấu hình lấy qua hàm đã đăng ký bằng SetConfigProvider (mặc định: gói bootcfg,
// import trống ở nơi dựng binary, đọc khoá `tts` trong config.json toàn cục).
// Không có provider nào thì dùng mặc định hoàn toàn — giọng đọc luôn khởi động được.
func Shared() *Service {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared == nil {
		provider := configProvider
		if provider == nil {
			provider = func() Config { return Config{}.WithDefaults() }
		}
		shared = New(provider())
	}
	return shared
}

// SetShared thay Service dùng chung (test hoặc khi cấu hình đổi cần dựng lại).
// Service cũ không bị đóng — người gọi tự quyết định.
func SetShared(s *Service) {
	sharedMu.Lock()
	shared = s
	sharedMu.Unlock()
}

// SetConfigProvider đổi nguồn cấu hình cho Shared(). Nên gọi lúc khởi động,
// trước khi mã nào đó gọi Shared() đầu tiên.
func SetConfigProvider(fn func() Config) {
	sharedMu.Lock()
	configProvider = fn
	sharedMu.Unlock()
}

// CloseShared đóng Service dùng chung nếu đã từng được dựng (tắt tiến trình máy đọc).
// Gọi khi tiến trình chủ (serve, TUI) thoát; chưa dựng thì không làm gì.
func CloseShared() error {
	sharedMu.Lock()
	s := shared
	shared = nil
	sharedMu.Unlock()
	if s == nil {
		return nil
	}
	return s.Close()
}
