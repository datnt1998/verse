package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 09 §10.4 · GAP-34: kết luận kiểm tra một thư mục trước khi dùng làm nơi lưu truyện.
const (
	InspectFound    = "found"    // đã là thư viện Noverse có dự án
	InspectEmpty    = "empty"    // trống, ghi được
	InspectOther    = "other"    // có tệp khác, ghi được
	InspectReadonly = "readonly" // không ghi được (cùng phép thử với CheckRoot)
	InspectMissing  = "missing"  // chưa tồn tại (hoặc không phải thư mục)
)

// Inspection là kết quả của Inspect. Display đã dịch sẵn "Nơi › … › Thư mục".
type Inspection struct {
	Kind     string `json:"kind"`
	Projects int    `json:"projects"`
	Stories  int    `json:"stories"`
	Items    int    `json:"items"`
	Place    string `json:"place"`
	Display  string `json:"display"`
	Name     string `json:"name"`
	// ProjectName là tên dự án đầu tiên khi kind = found.
	ProjectName    string `json:"projectName,omitempty"`
	StorageWarning string `json:"storageWarning,omitempty"`
}

// visibleEntries đếm mục không bắt đầu bằng "." (tệp hệ thống như .DS_Store không tính).
func visibleEntries(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			n++
		}
	}
	return n, nil
}

// LibraryCounts đếm dự án và truyện của một thư viện Noverse; ok=false nếu không phải thư viện.
// firstName là tên dự án đầu tiên (để câu “Đã có 1 dự án Noverse (“{tên}”)” gọi đúng tên).
func LibraryCounts(dir string) (projects, stories int, firstName string, ok bool) {
	lib, err := Open(dir)
	if err != nil {
		return 0, 0, "", false
	}
	scan, err := lib.Scan()
	if err != nil || len(scan.Projects) == 0 {
		return 0, 0, "", false
	}
	for i, p := range scan.Projects {
		if list, err := lib.Stories(p.ID); err == nil {
			stories += len(list)
		}
		if i == 0 && !p.Legacy {
			if pf, err := LoadProjectFile(p.Dir); err == nil && pf != nil {
				firstName = pf.Name
			}
		}
	}
	return len(scan.Projects), stories, firstName, true
}

// Inspect phân loại path. Thử ghi bằng CheckRoot (tạo rồi xoá thư mục tạm), nên chỉ gọi cho thư mục
// người dùng thực sự định dùng, không gọi hàng loạt khi duyệt.
func Inspect(path string, places Places) Inspection {
	res := Inspection{Display: places.Display(path), Place: places.PlaceLabel(path), Name: filepath.Base(path), StorageWarning: LibraryMountWarning(path)}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		res.Kind = InspectMissing
		return res
	}
	res.Items, _ = visibleEntries(path)
	if _, err := CheckRoot(path); err != nil {
		res.Kind = InspectReadonly
		return res
	}
	if projects, stories, name, ok := LibraryCounts(path); ok {
		res.Kind, res.Projects, res.Stories, res.ProjectName = InspectFound, projects, stories, name
		return res
	}
	if res.Items == 0 {
		res.Kind = InspectEmpty
		return res
	}
	res.Kind = InspectOther
	return res
}

// Folder là một dòng của danh sách thư mục trong bộ duyệt.
type Folder struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"` // dir | library | readonly
	Items    int    `json:"items"`
	Projects int    `json:"projects,omitempty"`
	Writable bool   `json:"writable"`
}

// maxListed chặn thư mục khổng lồ; Items của từng thư mục con dừng ở maxItems.
const (
	maxListed = 1000
	maxItems  = 999
)

var systemDirs = map[string]bool{
	"lost+found": true, "$RECYCLE.BIN": true, "System Volume Information": true, "Windows": true,
	"Program Files": true, "Program Files (x86)": true, "ProgramData": true, "Recovery": true,
}

// hiddenDir báo thư mục không đưa vào bộ duyệt: tên chấm và thư mục hệ thống.
func hiddenDir(parentIsHome bool, goos, name string) bool {
	if strings.HasPrefix(name, ".") || systemDirs[name] {
		return true
	}
	return parentIsHome && goos == "darwin" && (name == "Library" || name == "Applications")
}

// ListFolders liệt kê thư mục con của dir (đã Confine). Chỉ thư mục; ẩn thư mục chấm, thư mục hệ thống
// và symlink trỏ ra ngoài mọi Nơi. Không ghi gì lên đĩa.
func ListFolders(dir string, places Places) ([]Folder, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	isHome := false
	for _, p := range places {
		if p.ID == "home" && p.real == dir {
			isHome = true
		}
	}
	out := []Folder{}
	for _, e := range entries {
		if len(out) >= maxListed {
			break
		}
		name := e.Name()
		if hiddenDir(isHome, goosName, name) {
			continue
		}
		child := filepath.Join(dir, name)
		real, err := places.Confine(child)
		if err != nil {
			continue
		}
		if st, err := os.Stat(real); err != nil || !st.IsDir() {
			continue
		}
		f := Folder{Name: name, Path: child, Kind: "dir", Writable: writableDir(real)}
		f.Items = countCapped(real)
		if f.Items < 500 {
			if projects, _, _, ok := LibraryCounts(real); ok {
				f.Kind, f.Projects = "library", projects
			}
		}
		if !f.Writable {
			f.Kind = "readonly"
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func countCapped(dir string) int {
	f, err := os.Open(dir)
	if err != nil {
		return 0
	}
	defer f.Close()
	names, _ := f.Readdirnames(maxItems + 200)
	n := 0
	for _, name := range names {
		if !strings.HasPrefix(name, ".") {
			n++
		}
	}
	if n > maxItems {
		n = maxItems
	}
	return n
}
