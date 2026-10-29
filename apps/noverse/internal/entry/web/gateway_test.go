package web

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
	"github.com/gofrs/flock"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type testGateway struct {
	t    *testing.T
	g    *Gateway
	root string
}

// newTestGateway 建立带 admin / writer(_/* + proj/*) / reader(_/oldbook) 三个账号的网关。
// 根目录：proj（新项目）、cand（候选）、oldbook + secret（旧式故事目录）。
func newTestGateway(t *testing.T) *testGateway {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(mkdirAll(t, root, "proj"), "project.json"), []byte(`{"name":"Proj"}`), 0o644))
	mkdirAll(t, root, "proj", "kich_ban")
	mkdirAll(t, root, "proj", "y-tuong")
	mkdirAll(t, root, "proj", "tai-san")
	mkdirAll(t, root, "cand", "kich_ban")
	mkdirAll(t, root, "oldbook", "output", "novel")
	mkdirAll(t, root, "secret", "output", "novel")

	accounts := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	for _, u := range []webauth.NewUser{
		{Username: "admin", Role: webauth.RoleAdmin, Password: "admin-pass"},
		{Username: "writer", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"_": webauth.ProjectOwner, "proj": webauth.ProjectOwner}, Password: "writer-pass"},
		{Username: "reader", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"_": webauth.ProjectView}, Password: "reader-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGateway(root, accounts, version.Info{})
	t.Cleanup(g.Close)
	return &testGateway{t: t, g: g, root: root}
}

// newTestGatewayForBuild 与 newTestGateway 相同，但注入构建身份（P0-2 测试用）。
func newTestGatewayForBuild(t *testing.T, build version.Info) *testGateway {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(mkdirAll(t, root, "proj"), "project.json"), []byte(`{"name":"Proj"}`), 0o644))
	mkdirAll(t, root, "proj", "kich_ban")
	mkdirAll(t, root, "proj", "y-tuong")
	mkdirAll(t, root, "proj", "tai-san")
	mkdirAll(t, root, "cand", "kich_ban")
	mkdirAll(t, root, "oldbook", "output", "novel")
	mkdirAll(t, root, "secret", "output", "novel")

	accounts := webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	for _, u := range []webauth.NewUser{
		{Username: "admin", Role: webauth.RoleAdmin, Password: "admin-pass"},
		{Username: "writer", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"_": webauth.ProjectOwner, "proj": webauth.ProjectOwner}, Password: "writer-pass"},
		{Username: "reader", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"_": webauth.ProjectView}, Password: "reader-pass"},
	} {
		if _, err := accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGateway(root, accounts, build)
	t.Cleanup(g.Close)
	return &testGateway{t: t, g: g, root: root}
}

func mkdirAll(t *testing.T, parts ...string) string {
	t.Helper()
	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func (tg *testGateway) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	tg.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	if method == "POST" || method == "PUT" || body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func sessionFrom(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Fatalf("session cookie flags: %+v", c)
			}
			return c.Value
		}
	}
	t.Fatalf("no session cookie (status %d: %s)", w.Code, w.Body.String())
	return ""
}

func (tg *testGateway) login(username, password string) string {
	tg.t.Helper()
	return sessionFrom(tg.t, tg.do("POST", "/api/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)))
}

func decodeJSONBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return m
}

func projectIDs(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var v struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, p := range v.Projects {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestGatewayLibraryListing(t *testing.T) {
	tg := newTestGateway(t)
	if got := tg.do("GET", "/api/library", "", "").Code; got != 401 {
		t.Fatalf("no session: %d", got)
	}
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")
	writer := tg.login("writer", "writer-pass")

	w := tg.do("GET", "/api/library", admin, "")
	if w.Code != 200 {
		t.Fatalf("admin library: %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if ids := projectIDs(t, w); strings.Join(ids, ",") != "proj,_" {
		t.Fatalf("admin projects: %v", ids)
	}
	if cands, ok := body["candidates"].([]any); !ok || len(cands) != 1 || cands[0] != "cand" {
		t.Fatalf("admin candidates: %v", body["candidates"])
	}
	cfg := body["config"].(map[string]any)
	if cfg["dirs"].(map[string]any)["stories"] != "kich_ban" {
		t.Fatalf("default dirs: %v", cfg)
	}

	// reader 只看到 "_"（含 oldbook），看不到 proj，也看不到 candidates。
	w = tg.do("GET", "/api/library", reader, "")
	if ids := projectIDs(t, w); len(ids) != 1 || ids[0] != "_" {
		t.Fatalf("reader projects: %v", ids)
	}
	if cands := decodeJSONBody(t, w)["candidates"].([]any); len(cands) != 0 {
		t.Fatalf("reader sees candidates: %v", cands)
	}
	if ids := projectIDs(t, tg.do("GET", "/api/library", writer, "")); len(ids) != 2 {
		t.Fatalf("writer projects: %v", ids)
	}
}

func TestGatewayProjectLifecycle(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	// 非 admin 不能建项目。
	if code := tg.do("POST", "/api/projects", reader, `{"slug":"x"}`).Code; code != 403 {
		t.Fatalf("reader create project: %d", code)
	}
	if code := tg.do("POST", "/api/projects", admin, `{"slug":"Bad Slug"}`).Code; code != 400 {
		t.Fatalf("bad slug: %d", code)
	}
	w := tg.do("POST", "/api/projects", admin, `{"slug":"du-an","name":"Dự Án"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	p := decodeJSONBody(t, w)
	if p["id"] != "du-an" || p["name"] != "Dự Án" {
		t.Fatalf("project: %v", p)
	}
	if code := tg.do("POST", "/api/projects", admin, `{"slug":"du-an"}`).Code; code != 409 {
		t.Fatalf("dup: %d", code)
	}
	for _, sub := range []string{"y-tuong", "tai-san", "kich_ban", ".ainovel/rules"} {
		if st, err := os.Stat(filepath.Join(tg.root, "du-an", filepath.FromSlash(sub))); err != nil || !st.IsDir() {
			t.Fatalf("missing %s", sub)
		}
	}

	// adopt 候选目录。
	w = tg.do("POST", "/api/projects/adopt", admin, `{"dir":"cand","name":"Cand"}`)
	if w.Code != 201 {
		t.Fatalf("adopt: %d %s", w.Code, w.Body.String())
	}
	if code := tg.do("POST", "/api/projects/adopt", admin, `{"dir":"oldbook"}`).Code; code != 400 {
		t.Fatalf("adopt non-candidate: %d", code)
	}
	// adopt 后 cand 不在候选列表。
	body := decodeJSONBody(t, tg.do("GET", "/api/library", admin, ""))
	if cands := body["candidates"].([]any); len(cands) != 0 {
		t.Fatalf("candidates after adopt: %v", cands)
	}

	// GET 项目详情 + PUT。
	w = tg.do("GET", "/api/projects/du-an", admin, "")
	if w.Code != 200 {
		t.Fatalf("get project: %d", w.Code)
	}
	if code := tg.do("GET", "/api/projects/du-an", reader, "").Code; code != 403 {
		t.Fatalf("reader sees project: %d", code)
	}
	w = tg.do("PUT", "/api/projects/du-an", admin, `{"name":"Dự Án 2","description":"mô tả","notes":"ghi chú"}`)
	if w.Code != 200 || decodeJSONBody(t, w)["name"] != "Dự Án 2" {
		t.Fatalf("put project: %d %s", w.Code, w.Body.String())
	}
	if code := tg.do("PUT", "/api/projects/_", admin, `{"name":"x"}`).Code; code != 409 {
		t.Fatalf("put legacy: %d", code)
	}
}

func TestGatewayStoryLifecycle(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	// 建故事：无名字 → NN-truyen-moi + tempName。
	w := tg.do("POST", "/api/projects/proj/stories", admin, `{"name":""}`)
	if w.Code != 201 {
		t.Fatalf("create story: %d %s", w.Code, w.Body.String())
	}
	sum := decodeJSONBody(t, w)
	if sum["id"] != "proj/01-truyen-moi" || sum["tempName"] != true || sum["status"] != "idea" {
		t.Fatalf("story: %v", sum)
	}
	w = tg.do("POST", "/api/projects/proj/stories", admin, `{"name":"Truyện Kể Dài"}`)
	if w.Code != 201 || decodeJSONBody(t, w)["id"] != "proj/02-truyen-ke-dai" {
		t.Fatalf("named story: %d %s", w.Code, w.Body.String())
	}

	// 列表包含两则旧式 + 两个新故事（admin 可见全部）。
	w = tg.do("GET", "/api/projects/proj/stories", admin, "")
	var list struct {
		Stories []storySummary `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Stories) != 2 {
		t.Fatalf("stories: %d %s", w.Code, w.Body.String())
	}

	// meta GET/PUT。
	w = tg.do("GET", "/api/projects/proj/stories/02-truyen-ke-dai/meta", admin, "")
	if w.Code != 200 || decodeJSONBody(t, w)["title"] != "Truyện Kể Dài" {
		t.Fatalf("meta: %d %s", w.Code, w.Body.String())
	}
	w = tg.do("PUT", "/api/projects/proj/stories/02-truyen-ke-dai/meta", admin, `{"title":"Đổi Tên","status":"writing","tags":["a","b"],"idea":"x1"}`)
	if w.Code != 200 {
		t.Fatalf("put meta: %d %s", w.Code, w.Body.String())
	}
	w = tg.do("GET", "/api/projects/proj/stories/02-truyen-ke-dai/meta", admin, "")
	m := decodeJSONBody(t, w)
	if m["title"] != "Đổi Tên" || m["status"] != "writing" || m["createdAt"] == nil {
		t.Fatalf("meta after put: %v", m)
	}
	if code := tg.do("PUT", "/api/projects/proj/stories/02-truyen-ke-dai/meta", admin, `{"status":"bogus"}`).Code; code != 400 {
		t.Fatalf("bad status: %d", code)
	}

	// rename：保留 NN 前缀，accounts 授权更新。
	w = tg.do("POST", "/api/projects/proj/stories/01-truyen-moi/rename", admin, `{"slug":"ten-moi"}`)
	if w.Code != 200 || decodeJSONBody(t, w)["id"] != "proj/01-ten-moi" {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tg.root, "proj", "kich_ban", "01-ten-moi")); err != nil {
		t.Fatal("renamed dir missing")
	}
	if code := tg.do("POST", "/api/projects/proj/stories/01-ten-moi/rename", admin, `{"slug":"bad slug"}`).Code; code != 400 {
		t.Fatalf("bad slug: %d", code)
	}
}

func TestGatewayIdeasAndAssets(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	if code := tg.do("POST", "/api/projects/proj/ideas", reader, `{"text":"x"}`).Code; code != 403 {
		t.Fatalf("reader idea on unreadable project: %d", code)
	}
	if code := tg.do("POST", "/api/projects/proj/ideas", admin, `{"text":"   "}`).Code; code != 400 {
		t.Fatalf("empty idea: %d", code)
	}
	w := tg.do("POST", "/api/projects/proj/ideas", admin, `{"text":"Ý tưởng về nàng tiên cá\n\nChi tiết."}`)
	if w.Code != 201 {
		t.Fatalf("create idea: %d %s", w.Code, w.Body.String())
	}
	idea := decodeJSONBody(t, w)
	id := idea["id"].(string)
	if idea["title"] != "Ý tưởng về nàng tiên cá" || id == "" {
		t.Fatalf("idea: %v", idea)
	}
	w = tg.do("GET", "/api/projects/proj/ideas/"+id, admin, "")
	if w.Code != 200 || !strings.Contains(decodeJSONBody(t, w)["text"].(string), "Chi tiết") {
		t.Fatalf("get idea: %d %s", w.Code, w.Body.String())
	}
	if code := tg.do("GET", "/api/projects/proj/ideas/khong-co", admin, "").Code; code != 404 {
		t.Fatalf("missing idea: %d", code)
	}

	// assets：放一个文件再看列表。
	if err := os.WriteFile(filepath.Join(tg.root, "proj", "tai-san", "anh.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w = tg.do("GET", "/api/projects/proj/assets", admin, "")
	if w.Code != 200 {
		t.Fatalf("assets: %d", w.Code)
	}
	var al struct {
		Assets []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"assets"`
	}
	json.Unmarshal(w.Body.Bytes(), &al)
	if len(al.Assets) != 1 || al.Assets[0].Path != "anh.png" || al.Assets[0].Kind != "file" {
		t.Fatalf("assets: %+v", al.Assets)
	}
}

func TestGatewayValidateFix(t *testing.T) {
	tg := newTestGateway(t)
	// 手工造一个带旧规则的项目。
	dir := filepath.Join(tg.root, "dit")
	os.WriteFile(filepath.Join(mkdirAll(t, tg.root, "dit"), "project.json"), []byte(`{"name":"Dit"}`), 0o644)
	os.WriteFile(filepath.Join(mkdirAll(t, dir, "_chung"), "rule.md"), []byte("# r"), 0o644)
	story := mkdirAll(t, dir, "kich_ban", "01-x", ".ainovel", "rules")
	mkdirAll(t, dir, "kich_ban", "01-x", "output")
	os.Symlink(filepath.Join("..", "..", "..", "..", "_chung", "rule.md"), filepath.Join(story, "rule.md"))

	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	w := tg.do("GET", "/api/projects/dit/validate", admin, "")
	if w.Code != 200 {
		t.Fatalf("validate: %d", w.Code)
	}
	v := decodeJSONBody(t, w)
	if v["ok"] != false || v["tree"] == nil {
		t.Fatalf("validate: %v", v)
	}
	codes := map[string]bool{}
	for _, i := range v["issues"].([]any) {
		codes[i.(map[string]any)["code"].(string)] = true
	}
	if !codes["legacy-rules"] || !codes["rules-symlink"] || !codes["missing-story-json"] {
		t.Fatalf("codes: %v", codes)
	}

	if code := tg.do("POST", "/api/projects/dit/validate/fix", reader, `{}`).Code; code != 403 && code != 404 {
		t.Fatalf("reader fix: %d", code)
	}
	w = tg.do("POST", "/api/projects/dit/validate/fix", admin, `{}`)
	if w.Code != 200 {
		t.Fatalf("fix: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Lstat(filepath.Join(story, "rule.md")); !os.IsNotExist(err) {
		t.Fatal("symlink still there")
	}
	if _, err := os.Stat(filepath.Join(dir, ".ainovel", "rules", "rule.md")); err != nil {
		t.Fatal("rule not moved")
	}
	v2 := decodeJSONBody(t, w)
	codes2 := map[string]bool{}
	for _, i := range v2["issues"].([]any) {
		codes2[i.(map[string]any)["code"].(string)] = true
	}
	if codes2["legacy-rules"] || codes2["rules-symlink"] {
		t.Fatalf("unfixed: %v", codes2)
	}
}

func TestGatewayRolePermissions(t *testing.T) {
	tg := newTestGateway(t)
	launched := 0
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) {
		launched++
		return fakeWorker(make(chan struct{}), &sync.Once{}), nil
	}
	tg.g.transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})
	reader, writer := tg.login("reader", "reader-pass"), tg.login("writer", "writer-pass")

	// A view role covers every story in the assigned project.
	w := tg.do("GET", "/api/projects/_/stories", reader, "")
	var list struct {
		Stories []storySummary `json:"stories"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Stories) != 2 || list.Stories[0].ID != "_/oldbook" {
		t.Fatalf("reader stories: %+v", list.Stories)
	}
	if code := tg.do("GET", "/api/projects/_/stories/secret/meta", reader, "").Code; code != 200 {
		t.Fatalf("same-project story denied: %d", code)
	}
	if code := tg.do("GET", "/api/projects/proj/stories/anything/snapshot", reader, "").Code; code != 403 {
		t.Fatalf("ungranted project story: %d", code)
	}
	// reader 可读不可写。
	if code := tg.do("GET", "/api/projects/_/stories/oldbook/snapshot", reader, "").Code; code != 200 {
		t.Fatalf("reader read: %d", code)
	}
	for _, action := range []string{"input", "resume", "abort", "next", "review", "reopen", "close"} {
		if code := tg.do("POST", "/api/projects/_/stories/oldbook/"+action, reader, `{}`).Code; code != 403 {
			t.Errorf("reader %s: %d", action, code)
		}
	}
	// Owners can write and create stories in their projects.
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/abort", writer, `{}`).Code; code != 200 {
		t.Fatalf("writer write: %d", code)
	}
	// An unassigned member cannot create a story in a different project.
	if code := tg.do("POST", "/api/projects/proj/stories", reader, `{"name":"x"}`).Code; code != 403 {
		t.Fatalf("reader create story: %d", code)
	}
	if code := tg.do("POST", "/api/projects/proj/stories", writer, `{"name":"Của Tôi"}`).Code; code != 201 {
		t.Fatalf("writer create story: %d", code)
	}
	if launched != 1 {
		t.Fatalf("denied requests must not start workers: %d launches", launched)
	}
}

func TestGatewayRenamePreservesProjectAccess(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	// Renaming a story does not rewrite project permissions.
	w := tg.do("POST", "/api/projects/_/stories/oldbook/rename", admin, `{"slug":"ten-moi"}`)
	if w.Code != 200 {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	reader := tg.login("reader", "reader-pass")
	w = tg.do("GET", "/api/projects/_/stories", reader, "")
	var list struct {
		Stories []storySummary `json:"stories"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Stories) != 2 || list.Stories[0].ID != "_/01-ten-moi" {
		t.Fatalf("renamed story lost project access: %+v", list.Stories)
	}
}

func TestGatewayLockedStoryRenameConflict(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	// 外部进程持锁 → 摘要 locked=true，rename 409。
	lockDir := filepath.Join(tg.root, "oldbook", "output", "novel")
	fl := flock.New(filepath.Join(lockDir, ".ainovel.lock"))
	held, err := fl.TryLock()
	if err != nil || !held {
		t.Fatalf("hold lock: %v", err)
	}
	defer fl.Close()

	w := tg.do("GET", "/api/projects/_/stories", admin, "")
	var list struct {
		Stories []storySummary `json:"stories"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	found := false
	for _, s := range list.Stories {
		if s.ID == "_/oldbook" {
			found = true
			if !s.Locked {
				t.Fatalf("locked story not flagged: %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("oldbook missing")
	}
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/rename", admin, `{"slug":"x"}`).Code; code != 409 {
		t.Fatalf("rename locked: %d", code)
	}
}

func TestGatewayConcurrentRequestsLaunchOnce(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	var launches atomic.Int32
	done := make(chan struct{})
	var once sync.Once
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) {
		launches.Add(1)
		return &launchedWorker{addr: "127.0.0.1:1", stdin: io.NopCloser(strings.NewReader("")), done: done, kill: func() error { once.Do(func() { close(done) }); return nil }}, nil
	}
	tg.g.transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Ainovel-Worker-Token") != tg.g.workerToken || tg.g.workerToken == "" {
			t.Errorf("missing worker token")
		}
		if r.Header.Get("Cookie") != "" {
			t.Errorf("browser session cookie leaked to worker")
		}
		if r.URL.Path == "/api/events" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("event: jobs.reset\ndata: {\"jobs\":[]}\n\n"))}, nil
		}
		if r.URL.Path != "/api/snapshot" {
			t.Errorf("unexpected worker path %q", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":false}`))}, nil
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := tg.do("GET", "/api/projects/_/stories/oldbook/snapshot", admin, ""); w.Code != 200 {
				t.Errorf("proxy status %d", w.Code)
			}
		}()
	}
	wg.Wait()
	if got := launches.Load(); got != 1 {
		t.Fatalf("launches %d", got)
	}
	close(done)
}

func TestGatewayLockedBookIsConflict(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) {
		return nil, fmt.Errorf("%w: other process", errBookLocked)
	}
	w := tg.do("GET", "/api/projects/_/stories/oldbook/snapshot", admin, "")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "other process") {
		t.Fatalf("locked book: %d %s", w.Code, w.Body.String())
	}
}

func TestGatewayQRPairing(t *testing.T) {
	tg := newTestGateway(t)
	tg.g.PublicURLs = []string{"http://192.168.1.15:8686"}
	admin := tg.login("admin", "admin-pass")
	w := tg.do("POST", "/api/users/reader/pairings", admin, `{}`)
	var p struct {
		Code  string `json:"code"`
		Links []struct {
			URL string `json:"url"`
			SVG string `json:"svg"`
		} `json:"links"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || w.Code != 200 {
		t.Fatalf("pairing: %d %s", w.Code, w.Body.String())
	}
	if len(p.Links) != 1 || p.Links[0].URL != "http://192.168.1.15:8686/p/"+p.Code || !strings.HasPrefix(p.Links[0].SVG, "<svg") {
		t.Fatalf("links: %+v", p.Links)
	}
	// 短地址 ≤ 62 字节 ⇒ 二维码 M 级最高 4 版（33 模块）。
	if len(p.Links[0].URL) > 62 {
		t.Fatalf("pair link too long for QR v4-M: %d bytes", len(p.Links[0].URL))
	}
	redir := tg.do("GET", "/p/"+p.Code, "", "")
	if redir.Code != http.StatusFound || redir.Header().Get("Location") != "/#/pair/"+p.Code {
		t.Fatalf("short link redirect: %d %q", redir.Code, redir.Header().Get("Location"))
	}
	if bad := tg.do("GET", "/p/..%2f..%2fetc", "", ""); bad.Code == http.StatusFound {
		t.Fatalf("malformed short link redirected: %q", bad.Header().Get("Location"))
	}
	phone := sessionFrom(t, tg.do("POST", "/api/pair", "", `{"code":"`+p.Code+`"}`))
	me := tg.do("GET", "/api/me", phone, "")
	if me.Code != 200 || !strings.Contains(me.Body.String(), `"username":"reader"`) {
		t.Fatalf("paired session: %d %s", me.Code, me.Body.String())
	}
	if code := tg.do("POST", "/api/pair", "", `{"code":"`+p.Code+`"}`).Code; code != 401 {
		t.Fatalf("pairing code reused: %d", code)
	}
	if code := tg.do("POST", "/api/users/admin/pairings", phone, `{}`).Code; code != 403 {
		t.Fatalf("reader paired as admin: %d", code)
	}
}

func TestGatewayLoginRateLimit(t *testing.T) {
	tg := newTestGateway(t)
	for range 5 {
		tg.do("POST", "/api/login", "", `{"username":"admin","password":"bad-guess"}`)
	}
	w := tg.do("POST", "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected lockout even with the right password: %d %v", w.Code, w.Header())
	}
}

// TestGatewayLoginRateLimitIgnoresUsernameCase proves the per-account lock
// cannot be dodged by changing letter case: Postgres matches usernames
// case-insensitively, so every spelling must charge the same account bucket.
// Each try comes from a fresh address so only the account bucket can lock.
func TestGatewayLoginRateLimitIgnoresUsernameCase(t *testing.T) {
	tg := newTestGateway(t)
	login := func(i int, username, password string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
		r.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", i+1)
		r.Host = "127.0.0.1:8686"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		return w
	}
	for i, spelling := range []string{"Admin", "ADMIN", "aDmin", " AdMiN ", "admiN"} {
		login(i, spelling, "bad-guess")
	}
	w := login(10, "admin", "admin-pass")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("account must be locked after case-variant failures, got %d", w.Code)
	}
}

func TestGatewayLibraryConfigPut(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	if code := tg.do("PUT", "/api/library", reader, `{}`).Code; code != 403 {
		t.Fatalf("reader put library: %d", code)
	}
	if code := tg.do("PUT", "/api/library", admin, `{"dirs":{"ideas":"Y Tuong"}}`).Code; code != 400 {
		t.Fatalf("bad dir name: %d", code)
	}
	if code := tg.do("PUT", "/api/library", admin, `{"slugWords":20}`).Code; code != 400 {
		t.Fatalf("bad slugWords: %d", code)
	}
	w := tg.do("PUT", "/api/library", admin, `{"dirs":{"ideas":"y-tuong","assets":"tai-san","stories":"kich_ban","exports":"xuat-ban"},"slugWords":4,"autoExport":{"onVolume":true,"onBook":false,"onChapter":true,"formats":["txt"],"pattern":"tap-{tap}/{truyen}.{ext}","onExists":"keep"}}`)
	if w.Code != 200 {
		t.Fatalf("put library: %d %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if version, ok := body["version"].(string); !ok || version == "" {
		t.Fatalf("version: %v", body["version"])
	}
	config := body["config"].(map[string]any)
	if config["version"] != float64(1) || config["slugWords"] != float64(4) {
		t.Fatalf("config: %v", config)
	}
	got := decodeJSONBody(t, tg.do("GET", "/api/library", admin, ""))
	if got["config"].(map[string]any)["slugWords"].(float64) != 4 {
		t.Fatalf("config not persisted: %v", got["config"])
	}
}

func TestGatewayCreateStoryWriterScope(t *testing.T) {
	tg := newTestGateway(t)
	writer := tg.login("writer", "writer-pass")
	reader := tg.login("reader", "reader-pass")

	// Project roles cover newly created stories without an extra story grant.
	w := tg.do("POST", "/api/projects/proj/stories", writer, `{"name":"Truyện Của Writer"}`)
	if w.Code != 201 {
		t.Fatalf("writer create story: %d %s", w.Code, w.Body.String())
	}
	id := decodeJSONBody(t, w)["id"].(string)
	if id != "proj/01-truyen-cua-writer" {
		t.Fatalf("story id: %v", id)
	}
	me := decodeJSONBody(t, tg.do("GET", "/api/me", writer, ""))
	projects, _ := me["user"].(map[string]any)["projects"].(map[string]any)
	if len(projects) != 2 {
		t.Fatalf("projects should stay unchanged: %v", projects)
	}
	// 新建故事立即可写。
	if code := tg.do("PUT", "/api/projects/proj/stories/01-truyen-cua-writer/meta", writer, `{"title":"Truyện Của Writer","status":"writing","tags":[]}`).Code; code != 200 {
		t.Fatalf("writer put meta: %d", code)
	}
	// View-only and unassigned members both receive the explicit permission error.
	if code := tg.do("POST", "/api/projects/_/stories", reader, `{}`).Code; code != 403 {
		t.Fatalf("reader create story in readable project: %d", code)
	}
	if code := tg.do("POST", "/api/projects/proj/stories", reader, `{}`).Code; code != 403 {
		t.Fatalf("reader create story in unreadable project: %d", code)
	}
}

// Icon/script có mã băm phải được trình duyệt giữ vĩnh viễn, index.html luôn kiểm lại;
// tệp băm không tồn tại phải 404 (không trả index.html để bị cache nhầm thành icon).
func TestStaticAssetsCacheHeaders(t *testing.T) {
	tg := newTestGateway(t)
	entries, err := fs.ReadDir(ui, "dist/assets")
	if err != nil || len(entries) == 0 {
		t.Skip("dist/assets chưa build")
	}
	asset := "/assets/" + entries[0].Name()
	for _, method := range []string{"GET", "HEAD"} {
		w := tg.do(method, asset, "", "")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatalf("%s %s = %d cache=%q", method, asset, w.Code, w.Header().Get("Cache-Control"))
		}
	}
	if w := tg.do("GET", "/", "", ""); w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("index cache=%q", w.Header().Get("Cache-Control"))
	}
	if w := tg.do("GET", "/assets/khong-co-abc123.svg", "", ""); w.Code != 404 {
		t.Fatalf("missing asset = %d", w.Code)
	}
}
