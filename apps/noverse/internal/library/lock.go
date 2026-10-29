package library

import (
	"path/filepath"

	"github.com/gofrs/flock"
)

// lockFileName 与 host/book_lock.go 的 bookLockFile 保持一致：Host 在输出目录
// 持有该文件的 flock 租约（worker/TUI 运行期间）。
const lockFileName = ".ainovel.lock"

// Locked 探测某输出目录是否被另一个进程持有书锁：尝试取锁，取不到即被占用。
// 不创建目录、不启动 worker；锁文件可能因此被创建，这与 Host 的行为一致。
func Locked(outputDir string) bool {
	fl := flock.New(filepath.Join(outputDir, lockFileName))
	ok, err := fl.TryLock()
	if err != nil {
		return false // 无法判断（目录不存在等）时按未占用处理
	}
	if !ok {
		_ = fl.Close()
		return true
	}
	_ = fl.Close()
	return false
}
