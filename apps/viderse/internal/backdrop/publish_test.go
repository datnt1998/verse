package backdrop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestPublishFileCrossVolumeKeepsOldUntilRename: khác ổ đĩa thì chép vào tệp
// tạm cùng thư mục đích rồi rename nguyên tử; lỗi giữa chừng không đụng tệp cũ
// và không để lại tệp .part.
func TestPublishFileCrossVolumeKeepsOldUntilRename(t *testing.T) {
	original := renameFile
	defer func() { renameFile = original }()

	sourceDir := t.TempDir()
	destDir := t.TempDir()
	from := filepath.Join(sourceDir, "asset.mp4")
	to := filepath.Join(destDir, "twitch-Abc.mp4")
	if err := os.WriteFile(from, []byte("mới"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, []byte("cũ"), 0o644); err != nil {
		t.Fatal(err)
	}

	stagingRename := false
	renameFile = func(src, dst string) error {
		if strings.HasPrefix(filepath.Base(src), ".backdrop-") {
			stagingRename = true
			return os.Rename(src, dst)
		}
		return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EXDEV}
	}
	if err := publishFile(from, to); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !stagingRename {
		t.Fatal("phải rename từ tệp tạm cùng thư mục đích")
	}
	data, err := os.ReadFile(to)
	if err != nil || string(data) != "mới" {
		t.Fatalf("tệp đích = %q err=%v", data, err)
	}
	entries, _ := os.ReadDir(destDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".part") {
			t.Fatalf("còn tệp tạm %q", entry.Name())
		}
	}
	if _, err := os.Stat(from); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("phải dọn tệp nguồn sau khi publish")
	}
}

// TestPublishFileFailureLeavesOldAndCleansStaging: rename cuối lỗi thì tệp cũ
// nguyên vẹn, không còn tệp tạm.
func TestPublishFileFailureLeavesOldAndCleansStaging(t *testing.T) {
	original := renameFile
	defer func() { renameFile = original }()

	sourceDir := t.TempDir()
	destDir := t.TempDir()
	from := filepath.Join(sourceDir, "asset.mp4")
	to := filepath.Join(destDir, "twitch-Abc.mp4")
	if err := os.WriteFile(from, []byte("mới"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, []byte("cũ"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := errors.New("đĩa đầy")
	renameFile = func(src, dst string) error {
		if strings.HasPrefix(filepath.Base(src), ".backdrop-") {
			return want
		}
		return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EXDEV}
	}
	if err := publishFile(from, to); !errors.Is(err, want) {
		t.Fatalf("lỗi = %v, muốn %v", err, want)
	}
	data, _ := os.ReadFile(to)
	if string(data) != "cũ" {
		t.Fatalf("tệp cũ bị đụng: %q", data)
	}
	entries, _ := os.ReadDir(destDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".part") {
			t.Fatalf("còn tệp tạm %q", entry.Name())
		}
	}
}
