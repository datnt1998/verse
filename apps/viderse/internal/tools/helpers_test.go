// Helper dùng chung cho test: cô lập Deps khỏi binary của máy.
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// isolatedDeps dựng Deps không thấy binary nào của máy: PATH trỏ vào thư mục
// rỗng và không có chỗ cài chuẩn, nên chỉ những đường dẫn test tự dựng mới
// được probe.
func isolatedDeps(t *testing.T, runner Runner, base string) Deps {
	t.Helper()
	return Deps{
		Runner:     runner,
		BaseDir:    base,
		Home:       base,
		PathEnv:    t.TempDir(),
		SystemDirs: []string{},
	}
}

// writeExecutable ghi một tệp thực thi giả vào dir và trả đường dẫn.
func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
