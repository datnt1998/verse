//go:build unix

package imagegen

import (
	"os"
	"syscall"
)

// openNoFollow mở tệp chỉ-đọc và từ chối symlink ở cấp syscall.
func openNoFollow(file string) (*os.File, error) {
	fd, err := syscall.Open(file, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), file), nil
}
