package library

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// 09 §10.2 · GAP-34: "Nơi" là tên thân thiện của các thư mục gốc trên máy chủ
// (Thư mục nhà, Tài liệu, Màn hình, iCloud Drive, Google Drive…, ổ ngoài).
// UI không bao giờ hiện tên thư mục trần hay đường dẫn tuyệt đối: luôn "Nơi › … › Thư mục".

// ErrOutsidePlaces: đường dẫn nằm ngoài các Nơi cho phép duyệt (kể cả sau khi theo symlink).
var ErrOutsidePlaces = errors.New("thư mục nằm ngoài các nơi được phép duyệt")

// ElsewhereLabel là nhãn cho thư mục không thuộc Nơi nào (vd chọn bằng hộp chọn của máy).
const ElsewhereLabel = "Máy chủ"

// Place là một Nơi tồn tại trên máy chủ. Path chỉ dành cho quản trị.
type Place struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon"`
	real  string // Path đã EvalSymlinks, dùng để giới hạn và so khớp
}

// Places là danh sách Nơi theo thứ tự hiển thị.
type Places []Place

// placeEnv gom mọi thứ phụ thuộc máy để test được trên bất kỳ hệ điều hành nào.
type placeEnv struct {
	GOOS       string
	Home       string
	User       string
	VolumeDirs []string // mỗi thư mục con là một ổ ngoài (/Volumes, /media/$USER…)
	Drives     []string // gốc ổ đĩa dùng nguyên (Windows: C:\)
	XDGFile    string   // user-dirs.dirs (Linux)
}

// BrowseRootsConfigured restricts discovery to explicit container mounts.
func BrowseRootsConfigured() bool {
	return strings.TrimSpace(os.Getenv("LIBRARY_BROWSE_ROOTS")) != ""
}

// BrowsePlacesFromEnv validates the complete allowlist before serve starts.
// Invalid or missing mounts never fall back to desktop discovery.
func BrowsePlacesFromEnv() (Places, error) {
	if !BrowseRootsConfigured() {
		return nil, nil
	}
	var places Places
	for i, raw := range strings.Split(os.Getenv("LIBRARY_BROWSE_ROOTS"), ",") {
		path := strings.TrimSpace(raw)
		if !filepath.IsAbs(path) || hasParentSegment(path) {
			return nil, fmt.Errorf("LIBRARY_BROWSE_ROOTS: gốc %d phải là đường dẫn tuyệt đối không chứa ..", i+1)
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("LIBRARY_BROWSE_ROOTS: gốc %d không truy cập được: %w", i+1, err)
		}
		if st, err := os.Stat(real); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("LIBRARY_BROWSE_ROOTS: gốc %d không phải thư mục", i+1)
		}
		dir, err := os.Open(real)
		if err != nil {
			return nil, fmt.Errorf("LIBRARY_BROWSE_ROOTS: không đọc được gốc %d: %w", i+1, err)
		}
		_, readErr := dir.ReadDir(1)
		_ = dir.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("LIBRARY_BROWSE_ROOTS: không đọc được gốc %d: %w", i+1, readErr)
		}
		places = append(places, Place{ID: fmt.Sprintf("mount:%d", i+1), Label: fmt.Sprintf("Kho truyện %d", i+1),
			Path: filepath.Clean(path), Icon: "folder-open", real: real})
	}
	return places, nil
}

func hasParentSegment(path string) bool {
	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// DiscoverPlaces liệt kê các Nơi hiện có trên máy chủ này; Nơi không tồn tại bị bỏ.
func DiscoverPlaces() Places {
	if BrowseRootsConfigured() {
		places, _ := BrowsePlacesFromEnv() // startup validates; errors remain fail-closed
		return places
	}
	if IsContainer() {
		root := ContainerLibraryRoot()
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil
		}
		return Places{{ID: "volume:library", Label: "Vùng dữ liệu Noverse", Path: root, Icon: "folder-open", real: real}}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	env := placeEnv{GOOS: runtime.GOOS, Home: home, User: os.Getenv("USER")}
	if env.User == "" {
		env.User = os.Getenv("USERNAME")
	}
	switch runtime.GOOS {
	case "darwin":
		env.VolumeDirs = []string{"/Volumes"}
	case "windows":
		for c := 'C'; c <= 'Z'; c++ {
			env.Drives = append(env.Drives, string(c)+`:\`)
		}
	default:
		env.XDGFile = filepath.Join(home, ".config", "user-dirs.dirs")
		env.VolumeDirs = []string{filepath.Join("/media", env.User), filepath.Join("/run/media", env.User), "/mnt"}
	}
	return discoverPlaces(env)
}

func discoverPlaces(env placeEnv) Places {
	var out Places
	add := func(id, label, path, icon string, skipRoot bool) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return
		}
		if st, err := os.Stat(real); err != nil || !st.IsDir() {
			return
		}
		if skipRoot && filepath.Dir(real) == real { // ổ hệ thống (/Volumes/Macintosh HD → /)
			return
		}
		out = append(out, Place{ID: id, Label: label, Path: path, Icon: icon, real: real})
	}
	xdg := parseUserDirs(env.XDGFile, env.Home)
	userDir := func(key, fallback string) string {
		if p := xdg[key]; p != "" {
			return p
		}
		return filepath.Join(env.Home, fallback)
	}
	add("home", "Thư mục nhà", env.Home, "user", false)
	add("documents", "Tài liệu", userDir("XDG_DOCUMENTS_DIR", "Documents"), "document-text", false)
	add("desktop", "Màn hình", userDir("XDG_DESKTOP_DIR", "Desktop"), "monitor", false)
	if env.GOOS == "darwin" {
		add("icloud", "iCloud Drive", filepath.Join(env.Home, "Library", "Mobile Documents", "com~apple~CloudDocs"), "cloud-connection", false)
		cloud := filepath.Join(env.Home, "Library", "CloudStorage")
		labels := map[string]int{}
		for _, name := range subDirNames(cloud) {
			label := cloudLabel(name)
			labels[label]++
			if n := labels[label]; n > 1 {
				label = fmt.Sprintf("%s (%d)", label, n)
			}
			add("cloud:"+name, label, filepath.Join(cloud, name), "cloud-connection", false)
		}
	}
	for _, dir := range env.VolumeDirs {
		for _, name := range subDirNames(dir) {
			add("volume:"+name, name, filepath.Join(dir, name), "box", true)
		}
	}
	for _, drive := range env.Drives {
		add("drive:"+drive, "Ổ đĩa "+strings.TrimSuffix(drive, `\`), drive, "box", false)
	}
	return out
}

// subDirNames trả tên các thư mục con (theo thứ tự tên), bỏ tên bắt đầu bằng "." và tệp thường.
func subDirNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && st.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// cloudLabel: "GoogleDrive-ten@gmail.com" → "Google Drive"; "OneDrive-Personal" → "OneDrive".
// Không đưa email hay tên tài khoản vào nhãn.
func cloudLabel(dir string) string {
	base, _, _ := strings.Cut(dir, "-")
	if base == "GoogleDrive" {
		return "Google Drive"
	}
	return base
}

var userDirLine = regexp.MustCompile(`^\s*(XDG_[A-Z]+_DIR)="(.*)"\s*$`)

// parseUserDirs đọc ~/.config/user-dirs.dirs; thiếu tệp trả map rỗng.
func parseUserDirs(file, home string) map[string]string {
	out := map[string]string{}
	if file == "" {
		return out
	}
	f, err := os.Open(file)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := userDirLine.FindStringSubmatch(sc.Text()); m != nil {
			p := strings.ReplaceAll(m[2], "$HOME", home)
			if filepath.IsAbs(p) && p != home {
				out[m[1]] = p
			}
		}
	}
	return out
}

// within báo target có nằm trong base không (kể cả bằng base) và phần tương đối.
func within(base, target string) (string, bool) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// ResolvePath chuẩn hoá path thành đường dẫn tuyệt đối đã theo symlink. Phần cuối chưa tồn tại
// được giữ nguyên (để hiển thị thư mục sắp tạo).
func ResolvePath(path string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	rest := ""
	cur := abs
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(real, rest), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// best trả Nơi cụ thể nhất chứa real (đường dẫn đã resolve) cùng phần tương đối.
func (ps Places) best(real string) (Place, string, bool) {
	var (
		found Place
		rel   string
		ok    bool
	)
	for _, p := range ps {
		r, in := within(p.real, real)
		if in && (!ok || len(p.real) > len(found.real)) {
			found, rel, ok = p, r, true
		}
	}
	return found, rel, ok
}

// Confine xác thực path nằm trong một Nơi sau khi theo symlink (không có đường vòng bằng "..")
// và trả đường dẫn thật. Thư mục không tồn tại trả lỗi os.ErrNotExist.
func (ps Places) Confine(path string) (string, error) {
	if BrowseRootsConfigured() && hasParentSegment(path) {
		return "", ErrOutsidePlaces
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%w: thiếu đường dẫn", ErrInvalidRoot)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if _, _, ok := ps.best(real); !ok {
		return "", ErrOutsidePlaces
	}
	return real, nil
}

// ConfineCandidate permits a missing last directory for onboarding suggestions,
// resolving existing ancestors before checking the configured boundary.
func (ps Places) ConfineCandidate(path string) (string, error) {
	if !filepath.IsAbs(path) || hasParentSegment(path) {
		return "", ErrOutsidePlaces
	}
	real, err := ResolvePath(path)
	if err != nil {
		return "", err
	}
	if _, _, ok := ps.best(real); !ok {
		return "", ErrOutsidePlaces
	}
	return real, nil
}

// Display trả "Nơi › … › Thư mục" bằng tiếng Việt; không bao giờ lộ đường dẫn tuyệt đối khi
// thư mục thuộc một Nơi. Thư mục ngoài mọi Nơi hiện là "Máy chủ › …". path rỗng → "".
func (ps Places) Display(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	real, err := ResolvePath(path)
	if err != nil {
		return ""
	}
	label := ElsewhereLabel
	rel := ""
	if p, r, ok := ps.best(real); ok {
		label, rel = p.Label, r
	} else {
		rel = strings.TrimPrefix(real, filepath.VolumeName(real))
		rel = strings.Trim(rel, string(filepath.Separator))
	}
	return joinDisplay(label, splitSegments(rel))
}

// PlaceLabel trả nhãn Nơi chứa path ("" nếu ngoài mọi Nơi).
func (ps Places) PlaceLabel(path string) string {
	real, err := ResolvePath(path)
	if err != nil {
		return ""
	}
	if p, _, ok := ps.best(real); ok {
		return p.Label
	}
	return ""
}

func splitSegments(rel string) []string {
	if rel == "" {
		return nil
	}
	return strings.Split(filepath.ToSlash(rel), "/")
}

// joinDisplay rút gọn đường dẫn dài: Nơi › đầu › … › cuối.
func joinDisplay(label string, segs []string) string {
	if len(segs) > 3 {
		segs = []string{segs[0], "…", segs[len(segs)-1]}
	}
	return strings.Join(append([]string{label}, segs...), " › ")
}
