package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var iconFileBody = regexp.MustCompile(`(?s)</metadata>(.*)</svg>\s*$`)

// Mọi icon inline trên trang OAuth phải trùng từng ký tự (path data, fill, opacity,
// transform) với file thiết kế mà `.i-<alias>` trong OAuth.dc.html trỏ tới. Chỉ #000
// được đổi thành currentColor. testdata/icons là bản chép của các file thiết kế đó;
// phía app có test so từng byte bản chép với docs/design/v1/icons của app.
func TestCallbackIconsMatchDesignFiles(t *testing.T) {
	dir := filepath.Join("testdata", "icons")

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
