//go:build unix

package toolchain

import (
	"os"
	"path/filepath"
	"syscall"
)

// freeDiskBytes trả số byte trống của filesystem chứa dir (leo lên tổ tiên
// gần nhất còn tồn tại). -1 khi không đọc được.
func freeDiskBytes(dir string) int64 {
	probe := dir
	for range 10 {
		if _, err := os.Stat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(probe, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

// FreeDiskBytes trả số byte trống của filesystem chứa dir (leo lên tổ tiên
// gần nhất còn tồn tại). -1 khi không đọc được.
func FreeDiskBytes(dir string) int64 { return freeDiskBytes(dir) }
