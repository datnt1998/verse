package web

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/identity"
)

// fsHome trả HOME của gateway thử nghiệm đã theo symlink (macOS: /var → /private/var).
func fsHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func decodeList(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return out
}

func q(path string) string { return url.QueryEscape(path) }

func TestFSAdminOnly(t *testing.T) {
	tg := newTestGateway(t)
	home := fsHome(t)
	for _, name := range []string{"writer", "reader"} {
		cookie := tg.login(name, name+"-pass")
		for _, c := range []struct{ method, path, body string }{
			{"GET", "/api/fs/places", ""},
			{"GET", "/api/fs/list?path=" + q(home), ""},
			{"POST", "/api/fs/mkdir", fmt.Sprintf(`{"parent":%q,"name":"x"}`, home)},
			{"GET", "/api/library/inspect?path=" + q(home), ""},
		} {
			if w := tg.do(c.method, c.path, cookie, c.body); w.Code != 403 {
				t.Errorf("%s %s %s → %d, want 403", name, c.method, c.path, w.Code)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(home, "x")); err == nil {
		t.Fatal("non-admin mkdir đã tạo thư mục")
	}
}

func TestFSPlaces(t *testing.T) {
	tg := newTestGateway(t)
	home := fsHome(t)
	mkdirAll(t, home, "Documents")
	mkdirAll(t, home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	admin := tg.login("admin", "admin-pass")
	w := tg.do("GET", "/api/fs/places", admin, "")
	if w.Code != 200 {
		t.Fatalf("places %d %s", w.Code, w.Body.String())
	}
	labels := map[string]string{}
	for _, p := range decodeList(t, w) {
		for _, k := range []string{"id", "label", "path", "icon"} {
			if s, _ := p[k].(string); s == "" {
				t.Fatalf("place thiếu %s: %v", k, p)
			}
		}
		labels[p["label"].(string)] = p["path"].(string)
	}
	if labels["Thư mục nhà"] == "" || labels["Tài liệu"] == "" {
		t.Fatalf("thiếu Nơi cơ bản: %v", labels)
	}
	if _, ok := labels["Màn hình"]; ok {
		t.Fatal("Màn hình không tồn tại phải bị bỏ")
	}
	if runtimeIsDarwin() && labels["iCloud Drive"] == "" {
		t.Fatalf("thiếu iCloud Drive: %v", labels)
	}
}

func TestFSListConfinement(t *testing.T) {
	tg := newTestGateway(t)
	home := fsHome(t)
	docs := mkdirAll(t, home, "Documents")
	mkdirAll(t, docs, "Bản thảo")
	mkdirAll(t, docs, ".git")
	outside := mkdirAll(t, t.TempDir(), "ngoai")
	mkdirAll(t, outside, "bi-mat")
	if err := os.Symlink(outside, filepath.Join(docs, "loi-thoat")); err != nil {
		t.Skip("symlink không khả dụng")
	}
	admin := tg.login("admin", "admin-pass")

	w := tg.do("GET", "/api/fs/list?path="+q(docs), admin, "")
	if w.Code != 200 {
		t.Fatalf("list %d %s", w.Code, w.Body.String())
	}
	rows := decodeList(t, w)
	if len(rows) != 1 || rows[0]["name"] != "Bản thảo" || rows[0]["kind"] != "dir" || rows[0]["writable"] != true {
		t.Fatalf("danh sách phải chỉ có Bản thảo (ẩn .git và symlink thoát): %v", rows)
	}
	for path, want := range map[string]int{
		outside:                          403, // ngoài mọi Nơi
		filepath.Join(docs, "loi-thoat"): 403, // symlink thoát Nơi
		filepath.Join(docs, "loi-thoat", "bi-mat"): 403,
		filepath.Join(docs, "..", "..", ".."):      403, // traversal
		filepath.Join(docs, "khong-co"):            404,
	} {
		if got := tg.do("GET", "/api/fs/list?path="+q(path), admin, "").Code; got != want {
			t.Errorf("list %s → %d, want %d", path, got, want)
		}
	}
	if got := tg.do("GET", "/api/fs/list", admin, "").Code; got != 400 {
		t.Errorf("thiếu path → %d", got)
	}
}

func TestFSMkdir(t *testing.T) {
	tg := newTestGateway(t)
	home := fsHome(t)
	docs := mkdirAll(t, home, "Documents")
	outside := mkdirAll(t, t.TempDir(), "ngoai")
	_ = os.Symlink(outside, filepath.Join(docs, "loi-thoat"))
	admin := tg.login("admin", "admin-pass")
	mk := func(parent, name string) *httptest.ResponseRecorder {
		return tg.do("POST", "/api/fs/mkdir", admin, fmt.Sprintf(`{"parent":%q,"name":%q}`, parent, name))
	}

	w := mk(docs, "Truyện Noverse")
	if w.Code != 201 {
		t.Fatalf("mkdir %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["path"] != filepath.Join(docs, "Truyện Noverse") || body["display"] != "Tài liệu › Truyện Noverse" {
		t.Fatalf("mkdir body: %v", body)
	}
	if st, err := os.Stat(filepath.Join(docs, "Truyện Noverse")); err != nil || !st.IsDir() {
		t.Fatal("thư mục chưa được tạo")
	}
	if w := mk(docs, "Truyện Noverse"); w.Code != 409 {
		t.Fatalf("tồn tại phải 409, got %d %s", w.Code, w.Body.String())
	}
	for _, bad := range []string{"", "  ", "..", ".", "a/b", `a\b`, ".an", "x:y", strings.Repeat("a", 101)} {
		if w := mk(docs, bad); w.Code != 400 {
			t.Errorf("tên %q → %d, want 400", bad, w.Code)
		}
	}
	if w := mk(outside, "x"); w.Code != 403 {
		t.Errorf("parent ngoài Nơi → %d", w.Code)
	}
	if w := mk(filepath.Join(docs, "loi-thoat"), "x"); w.Code != 403 {
		t.Errorf("parent là symlink thoát → %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(outside, "x")); err == nil {
		t.Fatal("mkdir đã ghi ra ngoài Nơi")
	}
	if w := mk(filepath.Join(docs, "khong-co"), "x"); w.Code != 404 {
		t.Errorf("parent không tồn tại → %d", w.Code)
	}
}

func TestLibraryInspectEndpoint(t *testing.T) {
	tg := newTestGateway(t)
	home := fsHome(t)
	docs := mkdirAll(t, home, "Documents")
	admin := tg.login("admin", "admin-pass")
	inspect := func(path string) map[string]any {
		w := tg.do("GET", "/api/library/inspect?path="+q(path), admin, "")
		if w.Code != 200 {
			t.Fatalf("inspect %s → %d %s", path, w.Code, w.Body.String())
		}
		return decodeJSONBody(t, w)
	}
	m := inspect(filepath.Join(docs, "Truyện Noverse"))
	if m["kind"] != "missing" || m["display"] != "Tài liệu › Truyện Noverse" || m["place"] != "Tài liệu" {
		t.Fatalf("missing: %v", m)
	}
	m = inspect(mkdirAll(t, docs, "Trong"))
	if m["kind"] != "empty" {
		t.Fatalf("empty: %v", m)
	}
	other := mkdirAll(t, docs, "Khac")
	_ = os.WriteFile(filepath.Join(other, "a.pdf"), []byte("x"), 0o644)
	if m = inspect(other); m["kind"] != "other" || m["items"] != float64(1) {
		t.Fatalf("other: %v", m)
	}
	if m = inspect(tg.root); m["kind"] != "found" || m["projects"].(float64) < 1 {
		t.Fatalf("found: %v", m)
	}
	if strings.Contains(fmt.Sprint(m["display"]), tg.root) {
		t.Fatalf("display lộ đường dẫn tuyệt đối: %v", m["display"])
	}
	if os.Geteuid() != 0 {
		ro := mkdirAll(t, docs, "Chi doc")
		if err := os.Chmod(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
		if m = inspect(ro); m["kind"] != "readonly" {
			t.Fatalf("readonly: %v", m)
		}
	}
	if got := tg.do("GET", "/api/library/inspect", admin, "").Code; got != 400 {
		t.Fatalf("thiếu path → %d", got)
	}
}

// newUnconfiguredGateway dựng gateway cài mới: HOME trống, không có thư viện nào.
func newUnconfiguredGateway(t *testing.T) *testGateway {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), library.IdentityOptions())
	for _, u := range []identity.NewUser{
		{Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass"},
		{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"_": identity.ProjectView}, Password: "reader-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGateway("", accounts, version.Info{})
	t.Cleanup(g.Close)
	return &testGateway{t: t, g: g}
}

func TestUnconfiguredLibraryNoPanicAnd409(t *testing.T) {
	tg := newUnconfiguredGateway(t)
	// Thư mục hiện tại có dự án: gateway chưa cấu hình tuyệt đối không được mở nó.
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	trap := t.TempDir()
	_ = os.WriteFile(filepath.Join(mkdirAll(t, trap, "bay"), "project.json"), []byte(`{"name":"Bẫy"}`), 0o644)
	if err := os.Chdir(trap); err != nil {
		t.Fatal(err)
	}
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	for _, cookie := range []string{admin, reader} {
		m := decodeResp(t, tg.do("GET", "/api/library", cookie, ""))
		if m["configured"] != false || m["rootPath"] != "" || m["rootDisplay"] != "" {
			t.Fatalf("GET /api/library chưa cấu hình: %v", m)
		}
		if ps, _ := m["projects"].([]any); len(ps) != 0 {
			t.Fatalf("không được thấy dự án của thư mục hiện tại: %v", m["projects"])
		}
		if m["config"] == nil {
			t.Fatalf("thiếu config mặc định: %v", m)
		}
	}
	const msg = "Chưa chọn nơi lưu truyện"
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/projects/bay", ""},
		{"GET", "/api/projects/bay/stories/x/snapshot", ""},
		{"POST", "/api/projects", `{"name":"A"}`},
		{"POST", "/api/projects/adopt", `{"dir":"a"}`},
		{"GET", "/api/config?scope=project&project=bay", ""},
		{"GET", "/api/rules?scope=story&project=bay&story=x", ""},
		{"GET", "/api/reveal?target=library", ""},
		{"POST", "/api/reveal", `{"target":"project","project":"bay"}`},
		{"PUT", "/api/library", `{"version":1,"dirs":{},"slugWords":3,"autoExport":{}}`},
	} {
		w := tg.do(c.method, c.path, admin, c.body)
		if w.Code != 409 || !strings.Contains(w.Body.String(), msg) {
			t.Errorf("%s %s → %d %s, want 409 %q", c.method, c.path, w.Code, w.Body.String(), msg)
		}
	}
	// Đường không phụ thuộc thư viện vẫn chạy.
	if w := tg.do("GET", "/api/system", admin, ""); w.Code != 200 {
		t.Fatalf("/api/system %d", w.Code)
	} else if m := decodeJSONBody(t, w); m["rootDisplay"] != "" || m["root"] != "" {
		t.Fatalf("system: %v", m)
	}
	if w := tg.do("GET", "/api/jobs", admin, ""); w.Code != 200 {
		t.Fatalf("/api/jobs %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(fsHome(t), ".ainovel", "serve.json")); err == nil {
		t.Fatal("serve.json không được ghi khi chưa PUT")
	}
}

// TestHostedRepositorySkipsLibraryGuard pins the hosted split: with a library
// repository installed (STORE_DRIVER=postgres) the filesystem root is empty by
// design, so the "chưa chọn nơi lưu truyện" guard must not answer 409 for
// every project/library route.
func TestHostedRepositorySkipsLibraryGuard(t *testing.T) {
	tg := newUnconfiguredGateway(t)
	lib, err := library.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tg.g.SetLibraryRepository(library.FileRepository(lib))
	admin := tg.login("admin", "admin-pass")
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/projects", `{"name":"Du an hosted"}`},
		{"GET", "/api/library", ""},
	} {
		w := tg.do(tc.method, tc.path, admin, tc.body)
		if w.Code == 409 && strings.Contains(w.Body.String(), "Chưa chọn nơi lưu truyện") {
			t.Fatalf("%s %s hit the file-root guard with a hosted repository: %s", tc.method, tc.path, w.Body.String())
		}
	}
}

func TestServeJSONWrittenOnlyOnSuccessfulPut(t *testing.T) {
	tg := newUnconfiguredGateway(t)
	home := fsHome(t)
	docs := mkdirAll(t, home, "Documents")
	admin := tg.login("admin", "admin-pass")
	serveJSON := filepath.Join(home, ".ainovel", "serve.json")
	put := func(root string) int {
		return tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, root)).Code
	}
	missing := filepath.Join(docs, "khong-co")
	if code := put(missing); code != 400 {
		t.Fatalf("PUT thư mục không tồn tại → %d", code)
	}
	if os.Geteuid() != 0 {
		ro := mkdirAll(t, docs, "Chi doc")
		if err := os.Chmod(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
		w := tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, ro))
		if w.Code != 400 {
			t.Fatalf("PUT thư mục chỉ đọc → %d %s", w.Code, w.Body.String())
		}
	}
	if _, err := os.Stat(serveJSON); err == nil {
		b, _ := os.ReadFile(serveJSON)
		t.Fatalf("serve.json đã bị ghi sau PUT thất bại: %s", b)
	}
	if tg.g.RootPath() != "" {
		t.Fatalf("root đổi dù PUT thất bại: %q", tg.g.RootPath())
	}

	target := filepath.Join(docs, "Truyện Noverse")
	if w := tg.do("POST", "/api/fs/mkdir", admin, fmt.Sprintf(`{"parent":%q,"name":"Truyện Noverse"}`, docs)); w.Code != 201 {
		t.Fatalf("mkdir %d", w.Code)
	}
	w := tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, target))
	if w.Code != 200 {
		t.Fatalf("PUT hợp lệ → %d %s", w.Code, w.Body.String())
	}
	if m := decodeJSONBody(t, w); m["rootDisplay"] != "Tài liệu › Truyện Noverse" {
		t.Fatalf("rootDisplay: %v", m)
	}
	if root, err := library.LoadServeRoot(); err != nil || root != target {
		t.Fatalf("serve.json: %q %v", root, err)
	}
	m := decodeResp(t, tg.do("GET", "/api/library", admin, ""))
	if m["configured"] != true || m["rootDisplay"] != "Tài liệu › Truyện Noverse" || m["rootPath"] != target {
		t.Fatalf("GET sau khi chọn: %v", m)
	}
	if _, has := m["root"]; has {
		t.Fatalf("không còn trả tên thư mục trần: %v", m["root"])
	}
	// Đã cấu hình thì các đường dự án chạy lại (404 vì dự án không có, không phải 409).
	if code := tg.do("GET", "/api/projects/bay", admin, "").Code; code != 404 {
		t.Fatalf("sau khi cấu hình: %d", code)
	}
}

func TestLibraryResponseHidesRootFromNonAdmin(t *testing.T) {
	tg := newTestGateway(t)
	reader := tg.login("reader", "reader-pass")
	w := tg.do("GET", "/api/library", reader, "")
	if strings.Contains(w.Body.String(), tg.root) || strings.Contains(w.Body.String(), `"root"`) {
		t.Fatalf("reader thấy thư mục gốc: %s", w.Body.String())
	}
}

func runtimeIsDarwin() bool { return runtime.GOOS == "darwin" }
