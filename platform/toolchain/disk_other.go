//go:build !unix

package toolchain

// freeDiskBytes không có cách portable ngoài unix; báo không đọc được.
func freeDiskBytes(dir string) int64 { return -1 }

// FreeDiskBytes trả số byte trống của filesystem chứa dir; -1 khi không đọc được.
func FreeDiskBytes(dir string) int64 { return freeDiskBytes(dir) }
