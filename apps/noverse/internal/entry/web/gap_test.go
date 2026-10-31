package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/core/auth"
)

// closerFunc 让测试 worker 的 stdin 关闭时立刻结束 done，避免 stopWorker 等 10s。
type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// doFrom 发一个请求并指定 RemoteAddr / 额外头（loopback 与代理场景）。
func doFrom(t *testing.T, g *Gateway, method, path, cookie, body, remote string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = remote
	r.Host = "127.0.0.1:8686"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: cookie})
	}
	if method == "POST" || method == "PUT" || body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}

// fakeWorker 返回一个可由 stdin 关闭立即结束的 worker。
func fakeWorker(done chan struct{}, once *sync.Once) *launchedWorker {
	finish := func() { once.Do(func() { close(done) }) }
	return &launchedWorker{
		addr:  "127.0.0.1:1",
		stdin: closerFunc(func() error { finish(); return nil }),
		done:  done,
		kill:  func() error { finish(); return nil },
	}
}

func TestRevealTargetsAndLoopback(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	opened := ""
	oldOpen := openFolder
	openFolder = func(p string) error { opened = p; return nil }
	defer func() { openFolder = oldOpen }()

	// GET 只解析路径（非 loopback 也可以），不打开。
	w := tg.do("GET", "/api/reveal?target=story&project=proj&story=01-story", admin, "")
	if w.Code != 200 {
		t.Fatalf("GET reveal -> %d %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["path"] != tg.story || m["canReveal"] != false {
		t.Fatalf("reveal: %v", m)
	}
	if opened != "" {
		t.Fatal("GET không được mở thư mục")
	}

	// POST 非 loopback → 403 + đúng thông báo, không gọi open.
	w = tg.do("POST", "/api/reveal", admin, `{"target":"story","project":"proj","story":"01-story"}`)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "Chỉ mở được thư mục khi dùng ngay trên máy chủ") {
		t.Fatalf("non-loopback reveal -> %d %s", w.Code, w.Body.String())
	}
	if opened != "" {
		t.Fatal("không được mở khi ngoài loopback")
	}

	// POST loopback → mở.
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, `{"target":"story","project":"proj","story":"01-story"}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || opened != tg.story {
		t.Fatalf("loopback reveal -> %d %s opened=%q", w.Code, w.Body.String(), opened)
	}

	// Có X-Forwarded-For thì coi như qua proxy → 403.
	opened = ""
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, `{"target":"story","project":"proj","story":"01-story"}`, "127.0.0.1:5555", map[string]string{"X-Forwarded-For": "10.0.0.1"})
	if w.Code != 403 || opened != "" {
		t.Fatalf("proxied reveal -> %d opened=%q", w.Code, opened)
	}

	// library: chỉ admin.
	if code := tg.do("GET", "/api/reveal?target=library", reader, "").Code; code != 403 {
		t.Fatalf("reader library reveal: %d", code)
	}
	m = decodeResp(t, tg.do("GET", "/api/reveal?target=library", admin, ""))
	if m["path"] != tg.root {
		t.Fatalf("library path: %v", m["path"])
	}

	// activity: tạo <engine>/meta nếu chưa có.
	m = decodeResp(t, tg.do("GET", "/api/reveal?target=activity&project=proj&story=01-story", admin, ""))
	wantMeta := filepath.Join(tg.story, "output", "novel", "meta")
	if m["path"] != wantMeta {
		t.Fatalf("activity path: %v", m["path"])
	}
	if st, err := os.Stat(wantMeta); err != nil || !st.IsDir() {
		t.Fatalf("activity dir chưa được tạo: %v", err)
	}

	// exports + audio: thư mục lá được tạo.
	for target, want := range map[string]string{
		"exports": filepath.Join(tg.story, "xuat-ban"),
		"audio":   filepath.Join(tg.story, "audio"),
	} {
		m = decodeResp(t, tg.do("GET", "/api/reveal?target="+target+"&project=proj&story=01-story", admin, ""))
		if m["path"] != want {
			t.Fatalf("%s path: %v", target, m["path"])
		}
	}

	// rules: global cần admin; story/project chỉ cần đọc.
	if code := tg.do("GET", "/api/reveal?target=rules&scope=global", reader, "").Code; code != 403 {
		t.Fatalf("reader global rules: %d", code)
	}
	m = decodeResp(t, tg.do("GET", "/api/reveal?target=rules&scope=global", admin, ""))
	if !strings.HasSuffix(m["path"].(string), filepath.Join(".ainovel", "rules")) {
		t.Fatalf("global rules path: %v", m["path"])
	}
	m = decodeResp(t, tg.do("GET", "/api/reveal?target=rules&project=proj", admin, ""))
	if m["path"] != filepath.Join(tg.project, ".ainovel", "rules") {
		t.Fatalf("project rules path: %v", m["path"])
	}

	// Lỗi tham số.
	if code := tg.do("GET", "/api/reveal?target=bogus&project=proj", admin, "").Code; code != 400 {
		t.Fatalf("bogus target: %d", code)
	}
	if code := tg.do("GET", "/api/reveal?target=project&project=khong-co", admin, "").Code; code != 404 {
		t.Fatalf("missing project: %d", code)
	}
	if code := tg.do("GET", "/api/reveal?target=story&project=proj", admin, "").Code; code != 400 {
		t.Fatalf("missing story: %d", code)
	}
}

func TestSystemCanReveal(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	if m := decodeResp(t, tg.do("GET", "/api/system", admin, "")); m["canReveal"] != false {
		t.Fatalf("non-loopback canReveal: %v", m["canReveal"])
	}
	if m := decodeResp(t, doFrom(t, tg.g, "GET", "/api/system", admin, "", "127.0.0.1:5555", nil)); m["canReveal"] != true {
		t.Fatalf("loopback canReveal: %v", m["canReveal"])
	}
	if m := decodeResp(t, doFrom(t, tg.g, "GET", "/api/system", admin, "", "127.0.0.1:5555", map[string]string{"Forwarded": "for=10.0.0.1"})); m["canReveal"] != false {
		t.Fatalf("forwarded canReveal: %v", m["canReveal"])
	}
}

func TestPickFolderLoopbackAndOutcomes(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	oldPick := pickFolder
	defer func() { pickFolder = oldPick }()
	pickFolder = func(string) (string, bool, error) { return "/tmp/da-chon", false, nil }

	if code := tg.do("POST", "/api/system/pick-folder", admin, `{}`).Code; code != 403 {
		t.Fatalf("non-loopback pick: %d", code)
	}
	w := doFrom(t, tg.g, "POST", "/api/system/pick-folder", admin, `{"start":"/tmp"}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || decodeResp(t, w)["path"] != "/tmp/da-chon" {
		t.Fatalf("pick -> %d %s", w.Code, w.Body.String())
	}

	pickFolder = func(string) (string, bool, error) { return "", true, nil }
	w = doFrom(t, tg.g, "POST", "/api/system/pick-folder", admin, `{}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || decodeResp(t, w)["cancelled"] != true {
		t.Fatalf("cancel -> %d %s", w.Code, w.Body.String())
	}

	pickFolder = func(string) (string, bool, error) { return "", false, errPickUnsupported }
	w = doFrom(t, tg.g, "POST", "/api/system/pick-folder", admin, `{}`, "127.0.0.1:5555", nil)
	if w.Code != 501 {
		t.Fatalf("unsupported -> %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryRootSwitch(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	newRoot := filepath.Join(t.TempDir(), "thu-vien-2")
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := tg.do("PUT", "/api/library", reader, fmt.Sprintf(`{"root":%q}`, newRoot)).Code; code != 403 {
		t.Fatalf("reader switch: %d", code)
	}
	if code := tg.do("PUT", "/api/library", admin, `{"root":"/khong/ton/tai"}`).Code; code != 400 {
		t.Fatalf("bad root: %d", code)
	}

	w := tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, newRoot))
	if w.Code != 200 {
		t.Fatalf("switch -> %d %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["ok"] != true || m["root"] != newRoot || m["config"] == nil {
		t.Fatalf("switch body: %v", m)
	}
	if root, err := library.LoadServeRoot(); err != nil || root != newRoot {
		t.Fatalf("serve.json: %q %v", root, err)
	}
	if m := decodeResp(t, tg.do("GET", "/api/library", admin, "")); m["rootPath"] != newRoot {
		t.Fatalf("GET library rootPath: %v", m["rootPath"])
	}

	// Có engine đang chạy → 409; worker phải còn sống.
	done := make(chan struct{})
	var once sync.Once
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) {
		return fakeWorker(done, &once), nil
	}
	tg.g.setTransport(transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":true}`))}, nil
	}))
	if _, err := tg.g.startDir("proj/01-story", tg.story); err != nil {
		t.Fatal(err)
	}
	third := filepath.Join(t.TempDir(), "thu-vien-3")
	if err := os.MkdirAll(third, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, third)).Code; code != 409 {
		t.Fatalf("running switch: %d", code)
	}
	if root, _ := library.LoadServeRoot(); root != newRoot {
		t.Fatalf("409 không được đổi serve.json: %q", root)
	}

	// Rảnh → đổi được và worker bị dừng.
	tg.g.setTransport(transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":false}`))}, nil
	}))
	if w := tg.do("PUT", "/api/library", admin, fmt.Sprintf(`{"root":%q}`, third)); w.Code != 200 {
		t.Fatalf("idle switch -> %d %s", w.Code, w.Body.String())
	}
	select {
	case <-done:
	default:
		t.Fatal("worker nhàn rỗi phải bị dừng khi đổi thư viện")
	}
	if root, _ := library.LoadServeRoot(); root != third {
		t.Fatalf("serve.json sau đổi: %q", root)
	}
}

func TestConfigTestUsesPresetDefaultModel(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"fake","choices":[{"message":{"content":"OK"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	body := fmt.Sprintf(`{"provider":"openai","config":{"provider":"openai","providers":{"openai":{"type":"openai","api_key":"sk-fake","base_url":%q}}}}`, srv.URL)
	w := tg.do("POST", "/api/config/test", admin, body)
	if w.Code != 200 {
		t.Fatalf("test -> %d %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	if m["ok"] != true || m["model"] != "gpt-4o-mini" {
		t.Fatalf("test result: %v", m)
	}
	if !strings.Contains(gotBody, "gpt-4o-mini") {
		t.Fatalf("provider không nhận model mặc định: %s", gotBody)
	}

	presets := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))
	found := false
	for _, item := range presets["providers"].([]any) {
		p := item.(map[string]any)
		if p["id"] == "openai" {
			found = p["defaultModel"] == "gpt-4o-mini"
		}
	}
	if !found {
		t.Fatalf("presets thiếu defaultModel: %v", presets["providers"])
	}
}

func TestConfigExposesPersistedOAuthAccount(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(auth.OpenAICodexID, auth.Credential{
		Access: "access", Refresh: "refresh",
		Expires: time.Now().Add(time.Hour).UnixMilli(),
		Email:   "me@example.com", Plan: "plus",
	}); err != nil {
		t.Fatal(err)
	}

	accounts, ok := decodeResp(t, tg.do("GET", "/api/config", admin, ""))["accounts"].(map[string]any)
	if !ok {
		t.Fatal("GET /api/config thiếu accounts")
	}
	acc, ok := accounts[auth.OpenAICodexID].(map[string]any)
	if !ok || acc["connected"] != true || acc["label"] != "me@example.com (plus)" {
		t.Fatalf("account: %v", accounts)
	}

	// Người không phải admin không thấy tài khoản.
	readerAccounts, _ := decodeResp(t, tg.do("GET", "/api/config", reader, ""))["accounts"].(map[string]any)
	if len(readerAccounts) != 0 {
		t.Fatalf("reader accounts: %v", readerAccounts)
	}
}

func TestChaptersRevisionTracksChapterDraftAndReview(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	engine := filepath.Join(tg.story, "output", "novel")

	revision := func() string {
		t.Helper()
		w := tg.do("GET", "/api/projects/proj/stories/01-story/chapters", admin, "")
		if w.Code != 200 {
			t.Fatalf("chapters -> %d %s", w.Code, w.Body.String())
		}
		rev, _ := decodeResp(t, w)["revision"].(string)
		if rev == "" {
			t.Fatal("thiếu revision")
		}
		return rev
	}

	mustWriteFile(t, filepath.Join(engine, "chapters", "01.md"), "Chương một")
	rev1 := revision()

	mustWriteFile(t, filepath.Join(engine, "drafts", "02.draft.md"), "Bản nháp hai")
	rev2 := revision()
	if rev2 == rev1 {
		t.Fatal("đổi bản nháp phải đổi revision")
	}

	mustWriteFile(t, filepath.Join(engine, "reviews", "01.json"), `{"chapter":1}`)
	rev3 := revision()
	if rev3 == rev2 {
		t.Fatal("đổi thẩm duyệt phải đổi revision")
	}

	// Tệp ngoài chapters/drafts/reviews không ảnh hưởng.
	mustWriteFile(t, filepath.Join(engine, "meta", "khac.json"), `{}`)
	if rev4 := revision(); rev4 != rev3 {
		t.Fatalf("revision đổi vì tệp không liên quan: %s -> %s", rev3, rev4)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectCoCreateLifecycle(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	done := make(chan struct{})
	var once sync.Once
	var launched []string
	tg.g.launch = func(id, dir, _, _ string) (*launchedWorker, error) {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Errorf("worker cwd không tồn tại: %s (%v)", dir, err)
		}
		launched = append(launched, id)
		return fakeWorker(done, &once), nil
	}
	inputText := ""
	tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
		payload := `{"ok":true}`
		switch r.URL.Path {
		case "/api/cocreate":
			payload = `{"active":true,"messages":[{"role":"user","content":"Truyện trinh thám về nữ giám định pháp y"}],"draft":"## Chủ đề\nTrinh thám","ready":true,"suggestions":[]}`
		case "/api/cocreate/message":
			payload = `{"reply":"Đã hiểu","draft":"## Chủ đề\nTrinh thám","ready":true,"suggestions":["Thêm bối cảnh"]}`
		case "/api/cocreate/cancel":
			payload = `{"ok":true}`
		case "/api/cocreate/commit":
			data, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(data, &body)
			inputText, _ = body["draft"].(string)
			payload = `{"ok":true,"action":"start"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
	}))

	// Reader (chỉ đọc) không được đồng sáng tác.
	if code := tg.do("POST", "/api/projects/proj/cocreate/message", reader, `{"message":"x"}`).Code; code != 403 {
		t.Fatalf("reader cocreate: %d", code)
	}

	// Không có draft → state rỗng.
	m := decodeResp(t, tg.do("GET", "/api/projects/proj/cocreate", admin, ""))
	if m["active"] != false || m["draftId"] != "" {
		t.Fatalf("empty state: %v", m)
	}

	// Tin nhắn đầu tiên tạo nháp trong <dự án>/.ainovel/drafts/<id>/.
	w := tg.do("POST", "/api/projects/proj/cocreate/message", admin, `{"message":"Truyện trinh thám về nữ giám định pháp y"}`)
	if w.Code != 200 {
		t.Fatalf("message -> %d %s", w.Code, w.Body.String())
	}
	m = decodeResp(t, w)
	id, _ := m["draftId"].(string)
	if id == "" || m["reply"] != "Đã hiểu" {
		t.Fatalf("message body: %v", m)
	}
	draftDir := filepath.Join(tg.project, ".ainovel", "drafts", id)
	if st, err := os.Stat(draftDir); err != nil || !st.IsDir() {
		t.Fatalf("thư mục nháp: %v", err)
	}
	if len(launched) == 0 || launched[0] != draftKey("proj", id) {
		t.Fatalf("worker key: %v", launched)
	}

	// GET theo id trả trạng thái + draftId.
	m = decodeResp(t, tg.do("GET", "/api/projects/proj/cocreate?draft="+id, admin, ""))
	if m["draftId"] != id || m["active"] != true {
		t.Fatalf("state: %v", m)
	}

	// Commit: chuyển thư mục, ghi story.json, bắt đầu viết. Không có name → thư mục tạm
	// NN-truyen-moi và tiêu đề trống: nội dung bản nháp/tin nhắn là yêu cầu, không phải tên.
	w = tg.do("POST", "/api/projects/proj/cocreate/commit", admin, fmt.Sprintf(`{"draft":%q}`, id))
	if w.Code != 201 {
		t.Fatalf("commit -> %d %s", w.Code, w.Body.String())
	}
	story, _ := decodeResp(t, w)["story"].(map[string]any)
	sid, _ := story["id"].(string)
	if !strings.HasPrefix(sid, "proj/02-truyen-moi") {
		t.Fatalf("story id: %v", sid)
	}
	storyDir := filepath.Join(tg.project, "kich_ban", strings.TrimPrefix(sid, "proj/"))
	if _, err := os.Stat(storyDir); err != nil {
		t.Fatalf("thư mục truyện: %v", err)
	}
	if _, err := os.Stat(draftDir); !os.IsNotExist(err) {
		t.Fatal("thư mục nháp phải biến mất")
	}
	f, err := library.LoadStoryFile(storyDir)
	if err != nil || f == nil || f.Status != "writing" || f.Title != "" {
		t.Fatalf("story.json: %+v %v", f, err)
	}
	if inputText != "## Chủ đề\nTrinh thám" {
		t.Fatalf("draft prompt gửi lúc bắt đầu: %q", inputText)
	}

	// Cancel xoá thư mục nháp.
	w = tg.do("POST", "/api/projects/proj/cocreate/message", admin, `{"message":"Truyện khác"}`)
	id2, _ := decodeResp(t, w)["draftId"].(string)
	if id2 == "" {
		t.Fatal("không tạo được nháp thứ hai")
	}
	if w := tg.do("POST", "/api/projects/proj/cocreate/cancel", admin, fmt.Sprintf(`{"draft":%q}`, id2)); w.Code != 200 {
		t.Fatalf("cancel -> %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tg.project, ".ainovel", "drafts", id2)); !os.IsNotExist(err) {
		t.Fatal("cancel phải xoá thư mục nháp")
	}
}

// 14 §B.4：language đã bị xoá. Client/tệp cũ vẫn gửi khoá này → chấp nhận im lặng
// (không 400 vì DisallowUnknownFields), không ghi lại vào project.json/config.json,
// và không còn xuất hiện trong phản hồi.
func TestLegacyLanguageAcceptedButNeverWritten(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	if err := os.WriteFile(filepath.Join(tg.project, "project.json"), []byte(`{"name":"P","language":"zh"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	w := tg.do("PUT", "/api/projects/proj", admin, `{"name":"P2","description":"mô tả","language":"zh"}`)
	if w.Code != 200 {
		t.Fatalf("put project with legacy language -> %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"language"`) {
		t.Fatalf("phản hồi dự án không được còn language: %s", w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(tg.project, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "language") || !strings.Contains(string(raw), `"P2"`) {
		t.Fatalf("project.json phải bỏ language khi ghi lại: %s", raw)
	}

	if w := tg.do("POST", "/api/projects", admin, `{"slug":"moi","name":"Mới","language":"vi"}`); w.Code != 201 {
		t.Fatalf("create project with legacy language -> %d %s", w.Code, w.Body.String())
	}
	created, err := os.ReadFile(filepath.Join(filepath.Dir(tg.project), "moi", "project.json"))
	if err != nil || strings.Contains(string(created), "language") {
		t.Fatalf("project.json mới không được có language: %s %v", created, err)
	}

	w = tg.do("PUT", "/api/config?scope=global", admin, `{"style":"default","language":"zh"}`)
	if w.Code != 400 {
		t.Fatalf("PUT config language=zh phải 400, nhận %d %s", w.Code, w.Body.String())
	}
	w = tg.do("PUT", "/api/config?scope=global", admin, `{"style":"default","language":"vi"}`)
	if w.Code != 200 {
		t.Fatalf("PUT config language=vi -> %d %s", w.Code, w.Body.String())
	}

	presets := tg.do("GET", "/api/config/presets", admin, "")
	if presets.Code != 200 || strings.Contains(presets.Body.String(), `"languages"`) {
		t.Fatalf("presets không được còn languages: %d %s", presets.Code, presets.Body.String())
	}
}

func TestProjectCoCreateCommitReportsStartFailure(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	done := make(chan struct{})
	var once sync.Once
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) { return fakeWorker(done, &once), nil }
	tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/cocreate/commit" {
			return &http.Response{StatusCode: 500, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"thiếu khoá API"}`))}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"active":true,"messages":[{"role":"user","content":"Truyện thử"}],"draft":"## Chủ đề\nThử","ready":true,"suggestions":[]}`))}, nil
	}))

	w := tg.do("POST", "/api/projects/proj/cocreate/message", admin, `{"message":"Truyện thử"}`)
	id, _ := decodeResp(t, w)["draftId"].(string)
	if id == "" {
		t.Fatalf("message -> %d %s", w.Code, w.Body.String())
	}
	w = tg.do("POST", "/api/projects/proj/cocreate/commit", admin, fmt.Sprintf(`{"draft":%q}`, id))
	if w.Code != 201 {
		t.Fatalf("commit -> %d %s", w.Code, w.Body.String())
	}
	m := decodeResp(t, w)
	warning, _ := m["warning"].(string)
	if !strings.Contains(warning, "chưa bắt đầu viết") || !strings.Contains(warning, "thiếu khoá API") {
		t.Fatalf("thiếu cảnh báo khởi động: %v", m)
	}
	story, _ := m["story"].(map[string]any)
	sid, _ := story["id"].(string)
	storyDir := filepath.Join(tg.project, "kich_ban", strings.TrimPrefix(sid, "proj/"))
	if f, err := library.LoadStoryFile(storyDir); err != nil || f == nil || f.Status != "idea" {
		t.Fatalf("start lỗi thì story.json phải giữ idea: %+v %v", f, err)
	}
}

func TestProjectCoCreateCommitWithoutSession(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	done := make(chan struct{})
	var once sync.Once
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) { return fakeWorker(done, &once), nil }
	tg.g.setTransport(transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"active":false,"messages":[],"draft":"","ready":false,"suggestions":[]}`))}, nil
	}))

	lib, err := library.Open(tg.root)
	if err != nil {
		t.Fatal(err)
	}
	const id = "20260101-101010-abcdef"
	if _, err := lib.CreateDraft("proj", id); err != nil {
		t.Fatal(err)
	}
	w := tg.do("POST", "/api/projects/proj/cocreate/commit", admin, fmt.Sprintf(`{"draft":%q}`, id))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "Chưa có phiên đồng sáng tác") {
		t.Fatalf("commit without session -> %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tg.project, ".ainovel", "drafts", id)); err != nil {
		t.Fatal("409 không được xoá nháp")
	}
	// draft id sai định dạng → 400.
	if code := tg.do("GET", "/api/projects/proj/cocreate?draft=../x", admin, "").Code; code != 400 {
		t.Fatalf("bad draft id: %d", code)
	}
}

// 只有 POST /api/projects/<p>/voices 的 multipart 绕过 JSON 守卫。
func TestJSONGuardAllowsOnlyVoiceUploadMultipart(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	const multipart = "multipart/form-data; boundary=x"
	guarded := func(method, path, ct string) bool {
		w := doFrom(t, tg.g, method, path, admin, "--x--\r\n", "127.0.0.1:1", map[string]string{"Content-Type": ct})
		return w.Code == 400 && strings.Contains(w.Body.String(), "Content-Type phải là application/json")
	}
	if guarded("POST", "/api/projects/proj/voices", multipart) {
		t.Fatal("multipart POST voices bị chặn")
	}
	for _, c := range []struct{ method, path, ct string }{
		{"PUT", "/api/projects/proj/voices", multipart},
		{"POST", "/api/projects/proj/pronunciations", multipart},
		{"POST", "/api/projects/proj/stories/01-story/voices", multipart},
		{"POST", "/api/projects/proj/voices", "text/plain"},
	} {
		if !guarded(c.method, c.path, c.ct) {
			t.Fatalf("%s %s (%s) phải bị chặn bởi JSON guard", c.method, c.path, c.ct)
		}
	}
}
