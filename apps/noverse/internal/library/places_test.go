package library

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mk(t *testing.T, parts ...string) string {
	t.Helper()
	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func realTemp(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// macFixture dựng một HOME kiểu macOS + một thư mục ổ ngoài.
func macFixture(t *testing.T) (placeEnv, Places) {
	t.Helper()
	base := realTemp(t)
	home := mk(t, base, "home")
	mk(t, home, "Documents")
	mk(t, home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	mk(t, home, "Library", "CloudStorage", "GoogleDrive-ten@gmail.com")
	mk(t, home, "Library", "CloudStorage", "OneDrive-Personal")
	volumes := mk(t, base, "Volumes")
	mk(t, volumes, "Samsung T7")
	mk(t, volumes, ".Spotlight-V100")
	env := placeEnv{GOOS: "darwin", Home: home, VolumeDirs: []string{volumes}}
	return env, discoverPlaces(env)
}

func TestDiscoverPlacesLabelsAndOnlyExisting(t *testing.T) {
	_, ps := macFixture(t)
	var got []string
	for _, p := range ps {
		got = append(got, p.Label)
	}
	want := "Thư mục nhà|Tài liệu|iCloud Drive|Google Drive|OneDrive|Samsung T7"
	if strings.Join(got, "|") != want {
		t.Fatalf("labels = %q, want %q (Màn hình không tồn tại phải bị bỏ; email không được vào nhãn)", strings.Join(got, "|"), want)
	}
	for _, p := range ps {
		if p.ID == "" || p.Icon == "" || !filepath.IsAbs(p.Path) {
			t.Fatalf("place thiếu trường: %+v", p)
		}
	}
}

func TestDiscoverPlacesSkipsBootVolume(t *testing.T) {
	base := realTemp(t)
	home := mk(t, base, "home")
	volumes := mk(t, base, "Volumes")
	if err := os.Symlink("/", filepath.Join(volumes, "Macintosh HD")); err != nil {
		t.Skip("symlink không khả dụng")
	}
	ps := discoverPlaces(placeEnv{GOOS: "darwin", Home: home, VolumeDirs: []string{volumes}})
	for _, p := range ps {
		if p.Label == "Macintosh HD" {
			t.Fatalf("ổ hệ thống (trỏ về /) không được là một Nơi")
		}
	}
}

func TestDiscoverPlacesLinuxXDG(t *testing.T) {
	base := realTemp(t)
	home := mk(t, base, "home")
	docs := mk(t, base, "tai-lieu-cua-toi")
	xdg := filepath.Join(base, "user-dirs.dirs")
	if err := os.WriteFile(xdg, []byte("XDG_DOCUMENTS_DIR=\""+docs+"\"\n# comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps := discoverPlaces(placeEnv{GOOS: "linux", Home: home, XDGFile: xdg})
	for _, p := range ps {
		if p.ID == "documents" && p.Path == docs {
			return
		}
	}
	t.Fatalf("XDG_DOCUMENTS_DIR không được dùng: %+v", ps)
}

func TestDisplayFormatting(t *testing.T) {
	env, ps := macFixture(t)
	cases := []struct{ path, want string }{
		{env.Home, "Thư mục nhà"},
		{filepath.Join(env.Home, "Truyen"), "Thư mục nhà › Truyen"},
		{filepath.Join(env.Home, "Documents", "Truyện Noverse"), "Tài liệu › Truyện Noverse"},
		{filepath.Join(env.Home, "Library", "Mobile Documents", "com~apple~CloudDocs", "Truyện của Dat"), "iCloud Drive › Truyện của Dat"},
		{filepath.Join(env.VolumeDirs[0], "Samsung T7", "a", "b", "c", "d"), "Samsung T7 › a › … › d"},
		{filepath.Join(env.Home, "x", "y", "z"), "Thư mục nhà › x › y › z"},
	}
	for _, c := range cases {
		got := ps.Display(c.path)
		if got != c.want {
			t.Errorf("Display(%s) = %q, want %q", c.path, got, c.want)
		}
		if strings.Contains(got, env.Home) || strings.HasPrefix(got, "/") {
			t.Errorf("Display lộ đường dẫn tuyệt đối: %q", got)
		}
	}
	outside := ps.Display(filepath.Join(realTemp(t), "opt", "sach"))
	if !strings.HasPrefix(outside, ElsewhereLabel+" › ") || strings.HasPrefix(outside, "/") {
		t.Errorf("ngoài mọi Nơi phải là %q › …, got %q", ElsewhereLabel, outside)
	}
	if ps.Display("") != "" {
		t.Error("đường dẫn rỗng phải ra rỗng")
	}
}

func TestDisplayFollowsSymlinkedPlace(t *testing.T) {
	env, ps := macFixture(t)
	link := filepath.Join(realTemp(t), "tat")
	if err := os.Symlink(filepath.Join(env.Home, "Documents"), link); err != nil {
		t.Skip("symlink không khả dụng")
	}
	if got := ps.Display(filepath.Join(link, "Moi")); got != "Tài liệu › Moi" {
		t.Fatalf("Display qua symlink = %q", got)
	}
}

func TestConfineBlocksTraversalAndSymlinkEscape(t *testing.T) {
	env, ps := macFixture(t)
	outside := mk(t, realTemp(t), "ngoai")
	inside := mk(t, env.Home, "Documents", "ok")

	if real, err := ps.Confine(inside); err != nil || real != inside {
		t.Fatalf("thư mục trong Nơi phải qua: %q %v", real, err)
	}
	if _, err := ps.Confine(outside); !errors.Is(err, ErrOutsidePlaces) {
		t.Fatalf("ngoài mọi Nơi phải ErrOutsidePlaces, got %v", err)
	}
	// ".." thoát khỏi Nơi.
	if _, err := ps.Confine(filepath.Join(env.Home, "Documents", "..", "..", "..")); !errors.Is(err, ErrOutsidePlaces) {
		t.Fatalf("traversal bằng .. phải bị chặn, got %v", err)
	}
	// Symlink nằm TRONG Nơi nhưng trỏ ra ngoài.
	link := filepath.Join(env.Home, "Documents", "loi-thoat")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink không khả dụng")
	}
	if _, err := ps.Confine(link); !errors.Is(err, ErrOutsidePlaces) {
		t.Fatalf("symlink thoát Nơi phải bị chặn, got %v", err)
	}
	if _, err := ps.Confine(filepath.Join(link, "con")); err == nil {
		t.Fatal("đường dẫn đi qua symlink thoát phải bị chặn")
	}
	if _, err := ps.Confine(filepath.Join(env.Home, "khong-co")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("không tồn tại phải ErrNotExist, got %v", err)
	}
}

func TestListFoldersHidesDotSystemAndEscapingLinks(t *testing.T) {
	env, ps := macFixture(t)
	docs := filepath.Join(env.Home, "Documents")
	mk(t, docs, "Bản thảo")
	mk(t, docs, ".git")
	mk(t, docs, "lost+found")
	mk(t, docs, "Truyện Noverse")
	if err := os.WriteFile(filepath.Join(docs, "ghi-chu.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(mk(t, realTemp(t), "ngoai"), filepath.Join(docs, "thoat"))
	mk(t, env.Home, "Applications")
	lib := mk(t, docs, "Kho")
	if err := os.WriteFile(filepath.Join(mk(t, lib, "du-an"), ProjectFileName), []byte(`{"name":"D"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ListFolders(docs, ps)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	if strings.Join(names, "|") != "Bản thảo|Kho|Truyện Noverse" {
		t.Fatalf("danh sách = %v (phải ẩn .git, lost+found, tệp, symlink thoát)", names)
	}
	for _, f := range got {
		if f.Name == "Kho" && (f.Kind != "library" || f.Projects != 1) {
			t.Fatalf("Kho phải là thư viện 1 dự án: %+v", f)
		}
		if f.Name == "Bản thảo" && (f.Kind != "dir" || !f.Writable) {
			t.Fatalf("Bản thảo: %+v", f)
		}
	}
	if runtime.GOOS != "darwin" {
		return // Library/Applications chỉ là thư mục hệ thống trên macOS (hiddenDir dùng GOOS thật)
	}
	home, _ := ListFolders(env.Home, ps)
	for _, f := range home {
		if f.Name == "Applications" || f.Name == "Library" {
			t.Fatalf("thư mục hệ thống trong Thư mục nhà phải bị ẩn: %s", f.Name)
		}
	}
}

func TestListFoldersMarksReadonly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ghi được mọi nơi")
	}
	env, ps := macFixture(t)
	docs := filepath.Join(env.Home, "Documents")
	ro := mk(t, docs, "Chi doc")
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	got, _ := ListFolders(docs, ps)
	if len(got) != 1 || got[0].Kind != "readonly" || got[0].Writable {
		t.Fatalf("thư mục chỉ đọc: %+v", got)
	}
}

func TestInspectKinds(t *testing.T) {
	env, ps := macFixture(t)
	docs := filepath.Join(env.Home, "Documents")

	missing := Inspect(filepath.Join(docs, "Truyện Noverse"), ps)
	if missing.Kind != InspectMissing || missing.Display != "Tài liệu › Truyện Noverse" || missing.Place != "Tài liệu" {
		t.Fatalf("missing: %+v", missing)
	}

	empty := mk(t, docs, "Trong")
	if err := os.WriteFile(filepath.Join(empty, ".DS_Store"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(empty, ps); got.Kind != InspectEmpty || got.Items != 0 {
		t.Fatalf("empty (tệp chấm không tính): %+v", got)
	}

	other := mk(t, docs, "Khac")
	if err := os.WriteFile(filepath.Join(other, "hoa-don.pdf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(other, ps); got.Kind != InspectOther || got.Items != 1 {
		t.Fatalf("other: %+v", got)
	}

	lib := mk(t, docs, "Kho")
	for _, p := range []string{"a", "b"} {
		dir := mk(t, lib, p)
		if err := os.WriteFile(filepath.Join(dir, ProjectFileName), []byte(`{"name":"`+p+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		mk(t, dir, "kich_ban", "01-x")
	}
	mk(t, lib, "b", "kich_ban", "02-y")
	found := Inspect(lib, ps)
	if found.Kind != InspectFound || found.Projects != 2 || found.Stories != 3 {
		t.Fatalf("found: %+v", found)
	}
	if found.Display != "Tài liệu › Kho" {
		t.Fatalf("display: %q", found.Display)
	}

	file := filepath.Join(docs, "tep.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(file, ps); got.Kind != InspectMissing {
		t.Fatalf("tệp thường không phải thư mục: %+v", got)
	}
}

func TestInspectReadonlyUsesCheckRootProbe(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ghi được mọi nơi")
	}
	env, ps := macFixture(t)
	ro := mk(t, env.Home, "Documents", "Chi doc")
	if err := os.WriteFile(filepath.Join(ro, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	got := Inspect(ro, ps)
	if got.Kind != InspectReadonly || got.Items != 1 {
		t.Fatalf("readonly: %+v", got)
	}
	if _, err := CheckRoot(ro); err == nil {
		t.Fatal("CheckRoot và Inspect phải cùng kết luận")
	}
}

// 23-brand §5: nhận ra thư viện theo thư mục .ainovel/ bên trong, không theo tên —
// nên thư viện cũ "Truyện ainovel" vẫn dùng được, và tên gợi ý mới "Truyện Noverse"
// cũng được nhận y như vậy.
func TestInspectDetectsLibraryByNameIndependently(t *testing.T) {
	env, ps := macFixture(t)
	docs := filepath.Join(env.Home, "Documents")
	for _, name := range []string{"Truyện ainovel", "Truyện Noverse"} {
		// Bố cục thư viện hiện tại: <root>/.ainovel/library.json + dự án có project.json.
		root := mk(t, docs, name+" (thư viện)")
		if err := os.WriteFile(filepath.Join(mk(t, root, ".ainovel"), "library.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		proj := mk(t, root, "du-an")
		if err := os.WriteFile(filepath.Join(proj, ProjectFileName), []byte(`{"name":"D"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		mk(t, proj, "kich_ban", "truyen-1")
		got := Inspect(root, ps)
		if got.Kind != InspectFound || got.Projects != 1 || got.Stories != 1 {
			t.Fatalf("%s (bố cục mới): %+v", name, got)
		}
		if got.Name != name+" (thư viện)" || got.Display != "Tài liệu › "+name+" (thư viện)" {
			t.Fatalf("%s (bố cục mới): name/display = %q / %q", name, got.Name, got.Display)
		}

		// Bố cục cũ: mỗi truyện là thư mục con giữ .ainovel/ của riêng nó.
		legacy := mk(t, docs, name)
		mk(t, legacy, "truyen-cu", ".ainovel")
		got = Inspect(legacy, ps)
		if got.Kind != InspectFound || got.Projects != 1 || got.Stories != 1 {
			t.Fatalf("%s (bố cục cũ): %+v", name, got)
		}
		if got.Name != name || got.Display != "Tài liệu › "+name {
			t.Fatalf("%s (bố cục cũ): name/display = %q / %q", name, got.Name, got.Display)
		}
	}
}
