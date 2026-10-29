package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// target=config: chọn đúng tệp config.json của cấp được chọn (Settings › Nâng cao › “Tệp cấu hình” › Mở).
func TestRevealConfigFile(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	reader := tg.login("reader", "reader-pass")

	var folder, file string
	oldFolder, oldFile := openFolder, revealFile
	openFolder = func(p string) error { folder = p; return nil }
	revealFile = func(p string) error { file = p; return nil }
	defer func() { openFolder, revealFile = oldFolder, oldFile }()

	globalPath := filepath.Join(tg.home, ".ainovel", "config.json")
	projectPath := filepath.Join(tg.project, ".ainovel", "config.json")
	storyPath := filepath.Join(tg.story, ".ainovel", "config.json")

	// GET chỉ phân giải đường dẫn (không cần loopback, không mở gì).
	for _, c := range []struct{ query, want string }{
		{"target=config&scope=global", globalPath},
		{"target=config&scope=project&project=proj", projectPath},
		{"target=config&scope=story&project=proj&story=01-story", storyPath},
		{"target=config&project=proj&story=01-story", storyPath}, // scope bỏ trống → suy ra từ project/story
	} {
		w := tg.do("GET", "/api/reveal?"+c.query, admin, "")
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d %s", c.query, w.Code, w.Body.String())
		}
		m := decodeResp(t, w)
		if m["path"] != c.want || m["canReveal"] != false {
			t.Fatalf("GET %s: %v want path %s", c.query, m, c.want)
		}
	}
	if folder != "" || file != "" {
		t.Fatal("GET không được mở gì")
	}

	// Quyền: toàn cục chỉ admin; project/story đọc được thì GET được nhưng POST chỉ admin.
	if code := tg.do("GET", "/api/reveal?target=config&scope=global", writer, "").Code; code != 403 {
		t.Fatalf("writer global config: %d", code)
	}
	if code := tg.do("GET", "/api/reveal?target=config&scope=project&project=proj", reader, "").Code; code != 200 {
		t.Fatalf("reader project config: %d", code)
	}
	w := doFrom(t, tg.g, "POST", "/api/reveal", reader, `{"target":"config","scope":"project","project":"proj"}`, "127.0.0.1:5555", nil)
	if w.Code != 403 || file != "" || folder != "" {
		t.Fatalf("reader POST -> %d file=%q folder=%q", w.Code, file, folder)
	}

	// Lỗi tham số.
	if code := tg.do("GET", "/api/reveal?target=config&scope=project&project=nope", admin, "").Code; code != 404 {
		t.Fatalf("project không có: %d", code)
	}
	if code := tg.do("GET", "/api/reveal?target=config&scope=story&project=proj", admin, "").Code; code != 400 {
		t.Fatalf("thiếu story: %d", code)
	}
	if code := tg.do("GET", "/api/reveal?target=config&scope=galaxy", admin, "").Code; code != 400 {
		t.Fatalf("scope sai: %d", code)
	}

	// POST ngoài loopback → 403, không mở.
	w = tg.do("POST", "/api/reveal", admin, `{"target":"config","scope":"global"}`)
	if w.Code != 403 || !strings.Contains(w.Body.String(), revealLoopbackError) || file != "" || folder != "" {
		t.Fatalf("non-loopback -> %d %s", w.Code, w.Body.String())
	}

	// Tệp chưa có → mở thư mục chứa nó (và thư mục được tạo).
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, `{"target":"config","scope":"global"}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || folder != filepath.Dir(globalPath) || file != "" {
		t.Fatalf("missing file -> %d %s folder=%q file=%q", w.Code, w.Body.String(), folder, file)
	}
	if st, err := os.Stat(filepath.Dir(globalPath)); err != nil || !st.IsDir() {
		t.Fatalf("thư mục config chưa được tạo: %v", err)
	}

	// Tệp đã có → chọn đúng tệp, không mở thư mục.
	folder = ""
	tg.globalConfig(t, `{"provider":"openai"}`)
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, `{"target":"config","scope":"global"}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || file != globalPath || folder != "" {
		t.Fatalf("existing file -> %d %s folder=%q file=%q", w.Code, w.Body.String(), folder, file)
	}

	if err := os.MkdirAll(filepath.Dir(storyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storyPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	file = ""
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, `{"target":"config","scope":"story","project":"proj","story":"01-story"}`, "127.0.0.1:5555", nil)
	if w.Code != 200 || file != storyPath {
		t.Fatalf("story config -> %d %s file=%q", w.Code, w.Body.String(), file)
	}
}
