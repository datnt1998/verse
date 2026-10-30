package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
)

// RootMarker là tệp đánh dấu thư mục gốc của Viderse trong một thư mục do
// người dùng chọn — để nhận ra thư mục đã từng chứa dự án.
const RootMarker = ".viderse-root.json"

// ProjectsFolderName là thư mục con tự tạo khi người dùng chọn gốc một ổ.
const ProjectsFolderName = "Viderse Projects"

// canonicalPath chuẩn hoá một đường dẫn người dùng nhập: bung `~`, đổi sang
// tuyệt đối và resolve symlink (phần tồn tại). home rỗng = os.UserHomeDir.
func canonicalPath(path, home string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", codeError(ErrInvalidPath, "đường dẫn trống")
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		if home == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("không xác định được thư mục home: %w", err)
			}
		}
		trimmed = filepath.Join(home, trimmed[1:])
	}
	if !filepath.IsAbs(trimmed) {
		return "", codeError(ErrInvalidPath, "đường dẫn phải là tuyệt đối: %q", path)
	}
	cleaned := filepath.Clean(trimmed)
	// Resolve symlink của phần tồn tại (đích chưa tạo vẫn canonical hoá được).
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved, nil
	}
	probe := cleaned
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(probe)
		if err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return cleaned, nil
		}
		tail = append([]string{filepath.Base(probe)}, tail...)
		probe = parent
	}
}

// cloudRoots là các gốc đồng bộ đám mây mà thư mục dự án không được nằm trong
// (tệp đang dựng có thể bị khoá hoặc tải lên giữa chừng). So sánh trên đường
// dẫn đã canonical.
func cloudRoots(home string) []string {
	cloud := filepath.Join(home, "Library", "CloudStorage")
	roots := []string{
		cloud,
		filepath.Join(home, "Library", "Mobile Documents"), // iCloud Drive
		filepath.Join(home, "Dropbox"),
		filepath.Join(home, "Google Drive"),
	}
	if entries, err := os.ReadDir(cloud); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				roots = append(roots, filepath.Join(cloud, e.Name()))
			}
		}
	}
	return roots
}

// isCloudSyncPath báo path nằm trong một gốc đồng bộ đám mây.
func isCloudSyncPath(path, home string) bool {
	for _, root := range cloudRoots(home) {
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		}
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// hiddenSystemDirs là các thư mục hệ thống mà hộp chọn không hiện (tên cấp
// một so với gốc "/"); dotfiles bị bỏ riêng theo tên.
var hiddenSystemDirs = map[string]bool{
	"bin": true, "boot": true, "cores": true, "dev": true, "etc": true,
	"lib": true, "opt": true, "private": true, "sbin": true, "System": true,
	"tmp": true, "usr": true, "var": true, "VM": true, "swapfile": true,
	"net": true, "home": true, "Network": true,
}

// hiddenHomeDirs là các thư mục trong home mà hộp chọn không hiện.
var hiddenHomeDirs = map[string]bool{
	"Library":      true,
	"Applications": true,
	"Public":       true,
}

// dirWritable báo thư mục có ghi được (access(2) W_OK trên tổ tiên gần nhất).
func dirWritable(path string) bool {
	probe := path
	for {
		st, err := os.Stat(probe)
		if err == nil && st.IsDir() {
			return writable(probe)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return false
		}
		probe = parent
	}
}

// writable là access(2) W_OK — syscall.Access tồn tại trên darwin/linux.
func writable(path string) bool { return accessWOK(path) }

// dirUsable báo thư mục tồn tại, là thư mục và ghi được — điều kiện "online"
// của watch (một stat + một access mỗi nhịp, không đi cây).
func dirUsable(path string) bool {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return false
	}
	return writable(path)
}

// pathBytes cộng kích thước một tệp hoặc cây (lstat — không theo symlink):
// chung logic với retention qua fsio — một chỗ cho phép đo đệ quy.
func pathBytes(path string) int64 { return fsio.Bytes(path) }

// volumePresent báo path không nằm dưới một ổ chưa cắm: /Volumes/<tên> không
// tồn tại nghĩa là ổ đó đang rút — không được MkdirAll (sẽ tạo thư mục che
// mất ổ thật khi cắm lại).
func volumePresent(path string) bool {
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/")
	if len(parts) < 2 || parts[0] != "Volumes" {
		return true
	}
	_, err := os.Stat(filepath.Join("/Volumes", parts[1]))
	return err == nil
}

// onMount báo path nằm trên mount (kể cả chính nó); mount "/" đặc biệt vì
// không có tiền tố "//".
func onMount(path, mount string) bool {
	if mount == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == mount || strings.HasPrefix(path, mount+string(filepath.Separator))
}

// firstExisting trả path nếu tồn tại, ngược lại tổ tiên gần nhất còn tồn tại.
func firstExisting(path string) (string, error) {
	probe := path
	for {
		if _, err := os.Lstat(probe); err == nil {
			return probe, nil
		} else if !isNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("không có tổ tiên nào tồn tại của %q: %w", path, fs.ErrNotExist)
		}
		probe = parent
	}
}

func isNotExist(err error) bool { return os.IsNotExist(err) }

// dirEmpty báo thư mục tồn tại và rỗng; không tồn tại → true (xem như trống).
func dirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if isNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

// writeTestFile ghi và xoá một tệp thử trong dir (kiểm tra quyền ghi thật).
func writeTestFile(dir string) error {
	f, err := os.CreateTemp(dir, ".viderse-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if _, err := f.WriteString("ok"); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}
