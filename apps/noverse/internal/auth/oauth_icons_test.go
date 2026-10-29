package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var iconFileBody = regexp.MustCompile(`(?s)</metadata>(.*)</svg>\s*$`)

// Mọi icon inline trên trang OAuth phải trùng từng ký tự (path data, fill, opacity,
// transform) với docs/design/v1/icons/<file>.svg — đó là file mà `.i-<alias>` trong
// OAuth.dc.html trỏ tới. Chỉ #000 được đổi thành currentColor.
func TestCallbackIconsMatchDesignFiles(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller không trả về đường dẫn file test")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "design", "v1", "icons")

	wantFiles := []string{
		"arrow-down-2", "arrow-left-2", "arrow-right-2", "coin-1", "login", "mobile",
		"refresh-2", "setting-2", "shield-security", "tick-circle-bulk",
	}
	if len(callbackIconBodies) != len(wantFiles) {
		t.Errorf("callbackIconBodies có %d icon, artboard dùng %d", len(callbackIconBodies), len(wantFiles))
	}
	for _, name := range wantFiles {
		raw, err := os.ReadFile(filepath.Join(dir, name+".svg"))
		if err != nil {
			t.Errorf("không đọc được icon thiết kế %s: %v", name, err)
			continue
		}
		m := iconFileBody.FindSubmatch(raw)
		if m == nil {
			t.Errorf("%s.svg: không tách được phần thân SVG", name)
			continue
		}
		want := strings.ReplaceAll(string(m[1]), "#000", "currentColor")
		got, ok := callbackIconBodies[name]
		if !ok {
			t.Errorf("thiếu icon %s trong callbackIconBodies", name)
			continue
		}
		if got != want {
			t.Errorf("icon %s lệch file thiết kế:\n got: %s\nwant: %s", name, got, want)
		}
		if !strings.Contains(string(raw), `viewBox="0 0 24 24"`) {
			t.Errorf("%s.svg: viewBox không phải 0 0 24 24", name)
		}
	}
}
