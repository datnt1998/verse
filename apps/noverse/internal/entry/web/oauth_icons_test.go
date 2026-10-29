package web

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Trang callback OAuth nằm ở core/auth và được so với bản chép icon trong
// core/auth/testdata/icons. Test này giữ bản chép đó trùng từng byte với file
// thiết kế của app, để icon trang OAuth vẫn khớp docs/design/v1/icons.
func TestCoreOAuthIconsMatchDesignFiles(t *testing.T) {
	coreDir := filepath.Join("..", "..", "..", "..", "..", "core", "auth", "testdata", "icons")
	designDir := filepath.Join("..", "..", "..", "docs", "design", "v1", "icons")

	entries, err := os.ReadDir(coreDir)
	if err != nil {
		t.Fatalf("không đọc được icon của core: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s không có icon nào để so", coreDir)
	}
	for _, e := range entries {
		got, err := os.ReadFile(filepath.Join(coreDir, e.Name()))
		if err != nil {
			t.Errorf("đọc %s: %v", e.Name(), err)
			continue
		}
		want, err := os.ReadFile(filepath.Join(designDir, e.Name()))
		if err != nil {
			t.Errorf("icon %s không có trong docs/design/v1/icons: %v", e.Name(), err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("core/auth/testdata/icons/%s lệch docs/design/v1/icons/%s; chép lại file thiết kế", e.Name(), e.Name())
		}
	}
}
