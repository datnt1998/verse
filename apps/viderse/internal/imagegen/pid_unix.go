//go:build unix

package imagegen

import "syscall"

// pidAlive: pid còn sống (ESRCH = chết; EPERM = sống nhưng không signal được).
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
