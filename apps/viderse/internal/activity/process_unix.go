//go:build unix

package activity

import "syscall"

// processAlive báo process còn sống (kill 0 không gửi tín hiệu).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
