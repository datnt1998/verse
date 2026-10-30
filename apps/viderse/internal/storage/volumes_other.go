//go:build !darwin

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// fallbackVolumes là backend tối thiểu cho hệ điều hành khác darwin: một "ổ"
// duy nhất là gốc "/" (statfs), mọi kiểm tra cloud/fs vẫn chạy theo đường dẫn.
type fallbackVolumes struct{ home string }

// DefaultVolumes dựng backend ổ đĩa (bản rút gọn ngoài darwin).
func DefaultVolumes(home string) Volumes { return &fallbackVolumes{home: home} }

func (v *fallbackVolumes) stat(path string) (Volume, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Volume{}, err
	}
	return Volume{
		Name:      "/",
		Kind:      VolumeInternal,
		FS:        "ext4",
		FSType:    fmt.Sprintf("%x", st.Type),
		FreeBytes: int64(st.Bavail) * int64(st.Bsize),
		Mount:     "/",
		id:        st.Fsid.Val,
	}, nil
}

func (v *fallbackVolumes) List(ctx context.Context) ([]Volume, error) {
	vol, err := v.stat("/")
	if err != nil {
		return nil, err
	}
	return []Volume{vol}, nil
}

func (v *fallbackVolumes) Info(ctx context.Context, path string) (Volume, error) {
	abs, err := canonicalPath(path, v.home)
	if err != nil {
		return Volume{}, err
	}
	base, err := firstExisting(abs)
	if err != nil {
		return Volume{}, err
	}
	if st, err := os.Stat(base); err == nil && !st.IsDir() {
		base = filepath.Dir(base)
	}
	return v.stat(base)
}

// CachedVolumes giữ chữ ký với bản darwin (ngoài darwin mọi đọc đã là statfs
// một lần nên không cần snapshot).
func CachedVolumes(inner Volumes) Volumes { return inner }
