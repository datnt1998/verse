//go:build windows

package library

import (
	"os"
	"runtime"
)

const goosName = runtime.GOOS

// writableDir trên Windows không có access(2): dựa vào bit ghi của chủ sở hữu.
func writableDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().Perm()&0o200 != 0
}
