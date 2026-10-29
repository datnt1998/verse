// Thư mục tạm mà app tạo dưới os.TempDir(). Mỗi người tạo tự xoá trong defer;
// bộ quét này tồn tại cho những cái mà owner chết trước (server bị giết, sập,
// mất điện) — mỗi profile Chrome rò hàng chục MB và không gì khác xoá chúng.
package tools

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ChromeProbePrefix là tiền tố thư mục profile của probe Chrome.
const ChromeProbePrefix = "viderse-chrome-probe-"

// appTempPrefixes là các tiền tố thư mục tạm mà mã app (không phải test) tạo
// trong os.TempDir.
var appTempPrefixes = []string{
	ChromeProbePrefix,
	"viderse-measure-chrome-",
	"viderse-codex-iso-",
	"viderse-image-call-",
	"viderse-story-call-",
	"viderse-metadata-call-",
	"viderse-branding-ref-",
	"viderse-ffprobe-",
	"viderse-cutout-",
	"viderse-dl-",
	"viderse-vision-probe-",
}

// StaleTempDirAge xa hơn lời gọi app dài nhất (một lời gọi ảnh codex, một lần
// đo), nên thư mục của owner còn sống không bao giờ bị lấy.
const StaleTempDirAge = 6 * time.Hour

// ListStaleTempDirs liệt kê thư mục dưới root có tên <prefix>… và mtime cũ
// hơn maxAge.
func ListStaleTempDirs(root string, prefixes []string, maxAge time.Duration, now time.Time) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var stale []string
	for _, e := range entries {
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(e.Name(), prefix) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		dir := filepath.Join(root, e.Name())
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue // đua với việc dọn của chính owner
		}
		if now.Sub(info.ModTime()) > maxAge {
			stale = append(stale, dir)
		}
	}
	return stale
}

// SweepStaleTempDirs xoá các thư mục ListStaleTempDirs tìm thấy; trả những
// gì đã xoá.
func SweepStaleTempDirs(root string, prefixes []string, maxAge time.Duration) []string {
	stale := ListStaleTempDirs(root, prefixes, maxAge, time.Now())
	for _, dir := range stale {
		_ = os.RemoveAll(dir)
	}
	return stale
}

// AppTempPrefixes trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func AppTempPrefixes() []string { return append([]string(nil), appTempPrefixes...) }
