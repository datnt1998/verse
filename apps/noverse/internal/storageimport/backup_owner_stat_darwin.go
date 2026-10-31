//go:build darwin

package storageimport

import (
	"os"
	"syscall"
)

func backupHandleUnchanged(before, after os.FileInfo) bool {
	a, okA := before.Sys().(*syscall.Stat_t)
	b, okB := after.Sys().(*syscall.Stat_t)
	return okA && okB && a.Dev == b.Dev && a.Ino == b.Ino &&
		a.Size == b.Size && a.Mtimespec == b.Mtimespec && a.Ctimespec == b.Ctimespec &&
		before.Mode() == after.Mode() && a.Uid == b.Uid && a.Gid == b.Gid
}
