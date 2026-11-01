package main

// `viderse template import` — nhập template cũ vào Thư viện. Cùng đường
// nghiệp vụ với route POST /api/templates/import: cả hai gọi
// library.Service.ImportTemplateFiles, nên quy đổi, kiểm tệp và đặt tên chỉ có
// một bản.
//
// Ba dạng nguồn:
//   - một tệp `.ytf-template.zip` (bản xuất template);
//   - một thư mục chứa các tệp đó (nhập cả lô);
//   - thư mục `editor-workspace` cũ (có `projects/` + `media/`), nhập thẳng từ
//     dữ liệu cũ mà không cần chạy công cụ trước đây lần nào.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
)

// importFlags là cờ của lệnh nhập.
type importFlags struct {
	name string
	only []string
}

// parseImportFlags đọc --name/--only (dạng `--cờ giá-trị` và `--cờ=giá-trị`).
func parseImportFlags(args []string, stderr io.Writer) (importFlags, []string, int) {
	var flags importFlags
	positional := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		value := func() (string, bool) {
			if eq := strings.IndexByte(arg, '='); eq > 0 {
				return arg[eq+1:], true
			}
			if index+1 >= len(args) {
				return "", false
			}
			index++
			return args[index], true
		}
		switch {
		case arg == "--name" || strings.HasPrefix(arg, "--name="):
			name, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "thiếu giá trị cho --name")
			}
			flags.name = strings.TrimSpace(name)
		case arg == "--only" || strings.HasPrefix(arg, "--only="):
			text, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "thiếu giá trị cho --only")
			}
			if text = strings.TrimSpace(text); text != "" {
				flags.only = append(flags.only, text)
			}
		case strings.HasPrefix(arg, "-") && arg != "-":
			return flags, nil, libraryFlagError(stderr, "cờ không hợp lệ: "+arg)
		default:
			positional = append(positional, arg)
		}
	}
	return flags, positional, 0
}

// templateImportTargets liệt kê nguồn cần nhập từ một đường dẫn và bộ lọc
// --only (so chuỗi con, không phân biệt hoa thường, trên tên template).
func templateImportTargets(target string, only []string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, invalidTargetError("Không thấy đường dẫn " + target)
	}
	if !info.IsDir() {
		if !matchesOnly(filepath.Base(target), only) {
			return nil, invalidTargetError("Tệp không khớp --only")
		}
		return []string{target}, nil
	}
	if _, err := os.Stat(filepath.Join(target, "project.json")); err == nil {
		return []string{target}, nil
	}
	if projects := filepath.Join(target, "projects"); isDir(projects) {
		entries, err := os.ReadDir(projects)
		if err != nil {
			return nil, invalidTargetError("Không đọc được thư mục projects")
		}
		targets := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(projects, entry.Name())
			if !isFile(filepath.Join(dir, "project.json")) {
				continue
			}
			if !matchesOnly(projectNameOf(dir, entry.Name()), only) {
				continue
			}
			targets = append(targets, dir)
		}
		sort.Strings(targets)
		if len(targets) == 0 {
			return nil, invalidTargetError("Thư mục không có template nào khớp --only")
		}
		return targets, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, invalidTargetError("Không đọc được thư mục " + target)
	}
	targets := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		if !matchesOnly(entry.Name(), only) {
			continue
		}
		targets = append(targets, filepath.Join(target, entry.Name()))
	}
	sort.Strings(targets)
	if len(targets) == 0 {
		return nil, invalidTargetError("Thư mục không có tệp .ytf-template.zip nào khớp --only")
	}
	return targets, nil
}

// projectNameOf đọc tên template từ project.json (hỏng thì lấy tên thư mục).
func projectNameOf(dir, fallback string) string {
	data, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		return fallback
	}
	var project struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return fallback
	}
	if name := strings.TrimSpace(project.Name); name != "" {
		return name
	}
	return fallback
}

// matchesOnly báo tên khớp một trong các chuỗi lọc (rỗng = nhận hết).
func matchesOnly(name string, only []string) bool {
	if len(only) == 0 {
		return true
	}
	lowered := strings.ToLower(name)
	for _, text := range only {
		if strings.Contains(lowered, strings.ToLower(text)) {
			return true
		}
	}
	return false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// invalidTargetError là lỗi 400 của lệnh nhập (đường dẫn không có gì để nhập).
func invalidTargetError(message string) error {
	return &library.Error{
		Code:     library.CodeInvalidInput,
		Message:  message,
		Recovery: "Chỉ đường dẫn tới tệp .ytf-template.zip, thư mục chứa các tệp đó, hoặc thư mục editor-workspace cũ (có projects/ và media/)",
	}
}
