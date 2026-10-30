// Package fsio là nguyên tắc đọc file chung của app nội bộ: đo kích thước
// cây và xoá có giới hạn/context. Mỗi package nghiệp vụ giữ quy tắc riêng
// (đo có ctx, có giới hạn mục…); chỉ việc đi cây lstat đơn giản là chung.
package fsio

import (
	"context"
	"os"
	"path/filepath"
)

// Bytes cộng kích thước một tệp hoặc cả cây (lstat — không theo symlink).
// Nhánh không đọc được (quyền, đã biến mất) tính 0 và không chặn nhánh khác.
func Bytes(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		total += Bytes(filepath.Join(path, e.Name()))
	}
	return total
}

// RemoveAll như os.RemoveAll nhưng dừng theo ctx (cho phép huỷ giữa một lần
// xoá lớn — ví dụ khi server đang tắt).
func RemoveAll(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	switch {
	case err != nil && os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0 || !info.IsDir():
		return os.Remove(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := RemoveAll(ctx, filepath.Join(path, e.Name())); err != nil {
			return err
		}
	}
	return os.Remove(path)
}
