//go:build unix

package storageimport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupOpenRefusesSymlinkSwap(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(dir, "accounts.v4.bak")
	target := filepath.Join(dir, "other")
	for _, path := range []string{backup, target} {
		if err := os.WriteFile(path, []byte("identical private bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(backup); err != nil {
		t.Fatal(err)
	}
	// Swap after the path-level check used by the former implementation.
	if err := os.Remove(backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, backup); err != nil {
		t.Fatal(err)
	}
	f, err := openNoFollow(backup)
	if f != nil {
		f.Close()
	}
	if !symlinkOpenError(err) {
		t.Fatalf("O_NOFOLLOW phải chặn symlink đổi sau lstat: %v", err)
	}
}

func TestBackupHandleDetectsCtimeAndInodeChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.dump")
	if err := os.WriteFile(path, []byte("private archive bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openNoFollow(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	// Reading changes atime on some filesystems, but is not a mutation.
	if _, err := f.Read(make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	stable, err := f.Stat()
	if err != nil || !backupHandleUnchanged(before, stable) {
		t.Fatalf("đọc không được làm bằng chứng mất ổn định: %v", err)
	}
	if err := f.Chmod(0o400); err != nil {
		t.Fatal(err)
	}
	if err := f.Chmod(0o600); err != nil {
		t.Fatal(err)
	}
	after, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("chmod không được thay size/mtime trong kiểm thử ctime")
	}
	if backupHandleUnchanged(before, after) {
		t.Fatal("ctime đổi phải bị từ chối dù size/mtime và mode cuối giống nhau")
	}
	replacement := filepath.Join(filepath.Dir(path), "other.dump")
	if err := os.WriteFile(replacement, []byte("private archive bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	other, err := os.Stat(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if backupHandleUnchanged(before, other) {
		t.Fatal("inode khác phải bị từ chối")
	}
}
