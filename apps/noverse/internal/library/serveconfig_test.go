package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServeRootRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if root, err := LoadServeRoot(); err != nil || root != "" {
		t.Fatalf("chưa có serve.json: %q %v", root, err)
	}
	dir := filepath.Join(home, "thu-vien")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveServeRoot(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ainovel", "serve.json")); err != nil {
		t.Fatal("serve.json chưa được ghi")
	}
	root, err := LoadServeRoot()
	if err != nil || root != dir {
		t.Fatalf("root: %q %v", root, err)
	}
}

func TestCheckRootValidation(t *testing.T) {
	base := t.TempDir()
	good := filepath.Join(base, "good")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := CheckRoot(good)
	if err != nil || abs != good {
		t.Fatalf("good root: %q %v", abs, err)
	}
	if _, err := CheckRoot(""); err == nil {
		t.Fatal("root rỗng phải lỗi")
	}
	if _, err := CheckRoot(filepath.Join(base, "khong-co")); err == nil {
		t.Fatal("root không tồn tại phải lỗi")
	}
	file := filepath.Join(base, "tep.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckRoot(file); err == nil {
		t.Fatal("root là tệp phải lỗi")
	}
}

func TestResolveServeRootNoImplicitDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	// Cài mới: không cờ, không serve.json, không ~/Truyen → chưa chọn (không phải thư mục hiện tại).
	if got, err := ResolveServeRoot(""); err != nil || got != "" {
		t.Fatalf("cài mới phải không có root: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(home, "Truyen")); err == nil {
		t.Fatal("không được tự tạo ~/Truyen")
	}
	if _, err := os.Stat(filepath.Join(home, ".ainovel", "serve.json")); err == nil {
		t.Fatal("serve.json chỉ được ghi khi PUT thành công")
	}

	// ~/Truyen có sẵn → dùng tiếp.
	legacy := filepath.Join(home, "Truyen")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _ := ResolveServeRoot(""); got != legacy {
		t.Fatalf("~/Truyen có sẵn phải được giữ: %q", got)
	}

	// serve.json thắng ~/Truyen, cờ --root thắng serve.json.
	saved := filepath.Join(home, "Documents", "Kho")
	if err := os.MkdirAll(saved, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveServeRoot(saved); err != nil {
		t.Fatal(err)
	}
	if got, _ := ResolveServeRoot(""); got != saved {
		t.Fatalf("serve.json: %q", got)
	}
	flagDir := t.TempDir()
	if got, _ := ResolveServeRoot(flagDir); got != flagDir {
		t.Fatalf("--root: %q", got)
	}
	if _, err := ResolveServeRoot(filepath.Join(home, "khong-co")); err == nil {
		t.Fatal("--root không tồn tại phải lỗi")
	}
}

func TestResolveServeRootContainerVolume(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	volume := filepath.Join(t.TempDir(), "library")
	t.Setenv("NOVERSE_LIBRARY_ROOT", volume)
	if got, err := ResolveServeRoot(""); err != nil || got != volume {
		t.Fatalf("container library = %q, %v; want %q", got, err, volume)
	}
	if info, err := os.Stat(volume); err != nil || !info.IsDir() {
		t.Fatalf("container library was not provisioned: %v", err)
	}
	explicit := t.TempDir()
	if got, err := ResolveServeRoot(explicit); err != nil || got != explicit {
		t.Fatalf("explicit root = %q, %v; want %q", got, err, explicit)
	}
}

func TestBrowseRootsOverrideContainerDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	first, second := t.TempDir(), t.TempDir()
	t.Setenv("NOVERSE_LIBRARY_ROOT", filepath.Join(home, "managed"))
	t.Setenv("LIBRARY_BROWSE_ROOTS", first+","+second)
	firstReal, err := filepath.EvalSymlinks(first)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveServeRoot(""); err != nil || got != firstReal {
		t.Fatalf("first browse root = %q, %v; want %q", got, err, firstReal)
	}
	places := DiscoverPlaces()
	if len(places) != 2 || places[0].Path != first || places[1].Path != second {
		t.Fatalf("browse allowlist = %+v, want only the two mounted roots", places)
	}
	if _, err := places.Confine(first + "/../" + filepath.Base(first)); err == nil {
		t.Fatal("configured roots must reject parent traversal")
	}
	outside := t.TempDir()
	link := filepath.Join(first, "outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := places.Confine(link); err == nil {
		t.Fatal("configured roots must reject escaping symlinks")
	}
}

func TestContainerNeverBrowsesOrSelectsHome(t *testing.T) {
	home, volume := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("NOVERSE_CONTAINER", "1")
	t.Setenv("NOVERSE_LIBRARY_ROOT", volume)
	t.Setenv("LIBRARY_BROWSE_ROOTS", "")
	places := DiscoverPlaces()
	if len(places) != 1 || places[0].Path != volume {
		t.Fatalf("container places must contain only the declared volume, got %+v", places)
	}
	if _, err := ResolveServeRoot(home); err == nil {
		t.Fatal("container must reject a library root in its ephemeral HOME")
	}
}
