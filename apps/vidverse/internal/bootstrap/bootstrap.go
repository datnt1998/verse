// Package bootstrap giữ danh tính lúc chạy của vidverse: thư mục dữ liệu riêng
// (~/.vidverse), không dùng chung với Noverse (~/.ainovel) hay Viderse (~/.viderse).
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

const configDirName = ".vidverse"

// DefaultConfigDir trả thư mục dữ liệu của app: ~/.vidverse.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}
