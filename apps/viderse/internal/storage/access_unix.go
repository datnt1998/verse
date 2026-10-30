//go:build darwin || linux

package storage

import "syscall"

// accessWOK là access(2) W_OK: đúng cả khi mount chỉ-đọc mà stat không nói.
func accessWOK(path string) bool {
	return syscall.Access(path, 0x02) == nil // W_OK
}
