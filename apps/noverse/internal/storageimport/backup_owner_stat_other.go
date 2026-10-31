//go:build !darwin && !linux

package storageimport

import "os"

// No supported ctime/inode verification is available on this platform.
func backupHandleUnchanged(os.FileInfo, os.FileInfo) bool { return false }
