//go:build !windows

package library

import (
	"runtime"
	"syscall"
)

const goosName = runtime.GOOS

// writableDir hỏi hệ điều hành (access W_OK) mà không tạo gì trên đĩa; dùng khi duyệt hàng loạt.
func writableDir(path string) bool { return syscall.Access(path, 2) == nil }
