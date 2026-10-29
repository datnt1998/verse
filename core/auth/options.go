package auth

import "sync/atomic"

// Options là định danh của app dùng gói auth chung: tên hiển thị trên trang
// callback OAuth, lệnh đăng nhập gợi ý trong thông báo lỗi, và thư mục dữ
// liệu chứa auth.json. Mỗi app cài một lần ở đầu chương trình qua Configure.
type Options struct {
	// AppName là thương hiệu trên trang callback (vd. "Viderse").
	// Trống → trang dùng từ trung lập "ứng dụng".
	AppName string
	// Purpose là vế tùy chọn nối sau "dùng gói X của bạn" trên trang thành công
	// (vd. "để viết truyện"). Trống → bỏ vế đó.
	Purpose string
	// LoginCommand là lệnh đăng nhập gợi ý trong lỗi và trong mục "Chi tiết kỹ
	// thuật" của trang callback (vd. "viderse login"). Trống → gợi ý trung
	// lập không kèm lệnh.
	LoginCommand string
	// StoreDir là thư mục dưới home chứa auth.json (vd. ".viderse"). Được ghép
	// với os.UserHomeDir() tại thời điểm gọi, không cache, nên test đổi HOME
	// bằng t.Setenv vẫn đúng. Trống → DefaultStorePath/DefaultStore báo lỗi
	// thay vì lặng lẽ ghi vào một đường mặc định.
	StoreDir string
	// ClaudeCodeVersionEnv là biến môi trường ghim version Claude Code trên
	// wire. Trống → không ghim qua env.
	ClaudeCodeVersionEnv string
}

var appOptions atomic.Pointer[Options]

// Configure cài định danh app cho toàn tiến trình. Mọi chuỗi hiển thị và đường
// dẫn kho credential đều suy ra từ đây, tại thời điểm render/báo lỗi — không
// chụp lúc init package.
func Configure(o Options) { appOptions.Store(&o) }

// options trả cấu hình hiện hành; chưa cài thì nhận Options{} trung lập.
func options() Options {
	if o := appOptions.Load(); o != nil {
		return *o
	}
	return Options{}
}

// appName trả thương hiệu đã cài, hoặc từ trung lập khi chưa cấu hình.
func appName() string {
	if name := options().AppName; name != "" {
		return name
	}
	return "ứng dụng"
}
