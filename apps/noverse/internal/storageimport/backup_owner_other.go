//go:build !unix

package storageimport

import (
	"errors"
	"os"
)

// Ownership and atomic no-follow opening cannot be established on this
// platform. Backup evidence therefore fails closed rather than using a
// path-based fallback.
func ownedByCurrentUser(os.FileInfo) bool { return false }

func openNoFollow(string) (*os.File, error) {
	return nil, errors.New("nền tảng này không hỗ trợ kiểm chứng backup bằng O_NOFOLLOW và euid")
}

func symlinkOpenError(err error) bool { return errors.Is(err, errSymlinkRefused) }

func dumpRestorePath(string) string { return "/dev/fd/3" }
