// Package bootstrap giữ danh tính và cấu hình hạ tầng của Viderse: thương hiệu
// trang OAuth của binary, thư mục dữ liệu ~/.viderse, và chuỗi kết nối
// Postgres lấy từ biến môi trường. Việc chọn model cho từng việc sẽ đọc từ cài
// đặt kết nối AI trong DB (xây trên core/llmconfig), không từ file role cục bộ.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/core/auth"
)

const configDirName = ".viderse"

// ConfigureIdentity cài danh tính app cho core/auth (đường dẫn kho credential,
// thương hiệu trang OAuth, lệnh login trong lỗi, env wire version). auth.Options
// được đọc lười tại lúc dùng, không cần init() — cmd gọi hàm này việc đầu tiên
// trong main() để thứ tự cài tường minh thay vì phụ thuộc vào import.
func ConfigureIdentity() {
	auth.Configure(auth.Options{
		AppName:              "Viderse",
		Purpose:              "để điều hành các trợ lý làm video",
		LoginCommand:         "viderse login",
		StoreDir:             configDirName,
		ClaudeCodeVersionEnv: "VIDERSE_CLAUDE_CODE_VERSION",
	})
}

// DefaultConfigDir trả thư mục dữ liệu của app: ~/.viderse.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// DSNFromEnv trả chuỗi kết nối Postgres của app (VIDERSE_DATABASE_URL) và của
// role migrate (VIDERSE_MIGRATION_DATABASE_URL, trống → dùng chung DSN app).
func DSNFromEnv() (appDSN, migrateDSN string) {
	appDSN = strings.TrimSpace(os.Getenv("VIDERSE_DATABASE_URL"))
	migrateDSN = strings.TrimSpace(os.Getenv("VIDERSE_MIGRATION_DATABASE_URL"))
	if migrateDSN == "" {
		migrateDSN = appDSN
	}
	return appDSN, migrateDSN
}
