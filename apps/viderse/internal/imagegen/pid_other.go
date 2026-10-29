//go:build !unix

package imagegen

// pidAlive trên nền tảng không-unix: coi như không chứng minh được pid sống —
// đường phục hồi rơi về kiểm tra output.
func pidAlive(pid int) bool { return false }
