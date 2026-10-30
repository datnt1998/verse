//go:build darwin

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Nguồn: yt-factory src/server/storage.ts (thông tin ổ: fs, chỗ trống, cùng ổ).

func TestDefaultVolumesInfoMatchesStatfs(t *testing.T) {
	home := t.TempDir()
	vols := DefaultVolumes(home)
	dir := t.TempDir()
	info, err := vols.Info(context.Background(), dir)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.FS == "" || info.FSType == "" {
		t.Fatalf("thiếu loại hệ tệp: %+v", info)
	}
	if info.FreeBytes <= 0 {
		t.Fatalf("freeBytes = %d", info.FreeBytes)
	}
	// So với ổ "/" (firmlink vào Data volume được báo là ổ trong máy).
	root, err := vols.Info(context.Background(), "/")
	if err != nil {
		t.Fatalf("Info /: %v", err)
	}
	if info.fsid() != root.fsid() {
		t.Fatalf("fsid = %v, muốn %v", info.fsid(), root.fsid())
	}
	// Một thư mục thường không phải gốc ổ; firmlink vào Data volume vẫn phải
	// báo là ổ trong máy ("/") với tên hiển thị.
	if info.Mount == filepath.Clean(dir) {
		t.Fatalf("thư mục thường bị coi là gốc ổ: %+v", info)
	}
	if info.Mount != "/" || info.Name == "" || info.Kind != VolumeInternal {
		t.Fatalf("thư mục trong máy phải thuộc ổ gốc: %+v", info)
	}
}

func TestDefaultVolumesListHasRoot(t *testing.T) {
	vols := DefaultVolumes(t.TempDir())
	list, err := vols.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) == 0 || list[0].Mount != "/" {
		t.Fatalf("list = %+v, muốn bắt đầu bằng /", list)
	}
	for _, v := range list {
		if v.Name == "" || v.Kind == "" {
			t.Fatalf("ổ thiếu tên/loại: %+v", v)
		}
	}
}

func TestVolumePresentRejectsUnmountedPath(t *testing.T) {
	if volumePresent("/Volumes/KhongCoODauNay/projects") {
		t.Fatalf("ổ chưa cắm mà báo có")
	}
	if !volumePresent(t.TempDir()) {
		t.Fatalf("đường dẫn thường phải được coi là có ổ")
	}
	if _, err := os.Stat("/"); err != nil {
		t.Fatalf("stat /: %v", err)
	}
}
