//go:build unix

package storageimport

import (
	"errors"
	"os"
	"syscall"
)

// ownedByCurrentUser reports whether the file belongs to the effective user:
// a .v4.bak or dump planted by another account is never trusted as backup
// evidence.
func ownedByCurrentUser(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(st.Uid) == os.Geteuid()
}

// openNoFollow opens path for reading without following a symlink at the
// final component. Every backup-evidence file is opened through here so the
// fstat/byte checks that follow act on exactly the inode named.
func openNoFollow(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, errSymlinkRefused
	}
	return f, err
}

// symlinkOpenError reports whether an openNoFollow failure was the
// no-follow guard refusing a symlink.
func symlinkOpenError(err error) bool {
	return errors.Is(err, errSymlinkRefused)
}

// dumpRestorePath is the argument pg_restore receives for an archive: the
// inherited descriptor, so it reads exactly the open handle.
func dumpRestorePath(string) string {
	return "/dev/fd/3"
}
