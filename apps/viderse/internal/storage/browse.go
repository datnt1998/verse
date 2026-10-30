package storage

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// statusSource.go holds the status.Source adapter — see Report at bottom.

// Browse liệt kê thư mục con của path cho hộp chọn phía server (hỏi đáp #10:
// trình duyệt không trả được đường dẫn thật). path rỗng → gốc chọn (thư mục
// nhà + các ổ đang cắm). Chỉ trả thư mục: bỏ ẩn, hệ thống và không đọc được.
func (s *Service) Browse(ctx context.Context, path string) (BrowseResult, error) {
	roots, err := s.browseRoots(ctx)
	if err != nil {
		return BrowseResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return BrowseResult{Path: "", Roots: roots, Dirs: roots}, nil
	}
	abs, err := canonicalPath(path, s.deps.Home)
	if err != nil {
		return BrowseResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return BrowseResult{}, codeError(ErrNotFound, "không phải thư mục: %s", abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return BrowseResult{}, codeError(ErrNotFound, "không đọc được thư mục: %s", abs)
	}
	var dirs []BrowseEntry
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if hiddenAt(abs, name, s.deps.Home) {
			continue
		}
		child := filepath.Join(abs, name)
		if st, err := os.Stat(child); err != nil || !st.IsDir() {
			continue // symlink hỏng hoặc không phải thư mục
		}
		dirs = append(dirs, BrowseEntry{Name: name, Path: child})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	if len(dirs) > 500 {
		dirs = dirs[:500]
	}
	return BrowseResult{Path: abs, Roots: roots, Dirs: dirs}, nil
}

// hiddenAt báo một mục con của dir thuộc danh sách ẩn hệ thống.
func hiddenAt(dir, name, home string) bool {
	switch {
	case dir == "/":
		return hiddenSystemDirs[name]
	case home != "" && filepath.Clean(dir) == filepath.Clean(home):
		return hiddenHomeDirs[name]
	default:
		return false
	}
}

// browseRoots là các điểm bắt đầu của hộp chọn: thư mục nhà rồi các ổ đang
// gắm (không gồm ổ đồng bộ đám mây — check sẽ từ chối, nhưng vẫn cho duyệt
// để người dùng tự chọn sai và thấy lý do).
func (s *Service) browseRoots(ctx context.Context) ([]BrowseEntry, error) {
	roots := []BrowseEntry{}
	if s.deps.Home != "" {
		roots = append(roots, BrowseEntry{Name: "Thư mục nhà", Path: s.deps.Home})
	}
	vols, err := s.deps.Volumes.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range vols {
		name := v.Name
		if name == "" {
			name = filepath.Base(v.Mount)
		}
		roots = append(roots, BrowseEntry{Name: name, Path: v.Mount})
	}
	return roots, nil
}

// Open mở thư mục dự án hiện tại trong trình quản lý tệp của máy chạy
// Viderse (Finder trên darwin) qua Opener đã tiêm.
func (s *Service) Open(ctx context.Context) error {
	root := s.Root()
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return codeError(ErrVolumeGone, "thư mục dự án hiện không truy cập được")
	}
	return s.deps.Opener.Open(ctx, root)
}
