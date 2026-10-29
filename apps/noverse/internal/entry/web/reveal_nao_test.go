package web

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// D12 — Nâng cao › Não truyện: hai nút “Mở bằng Obsidian”. Chỉ quản trị, chỉ mở trên
// máy chủ, mở `obsidian://open?path=<kho>`; câu trả không chứa đường dẫn; kho chưa
// sinh thì 404 (kho là bản sinh một chiều, không tự tạo).
func TestRevealNaoVaultObsidian(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	var opened []string
	oldOpen := openURL
	openURL = func(raw string) error { opened = append(opened, raw); return nil }
	oldFolder, oldFile := openFolder, revealFile
	openFolder = func(path string) error {
		t.Fatalf("không được mở thư mục: %s", path)
		return nil
	}
	revealFile = func(path string) error {
		t.Fatalf("không được chọn tệp: %s", path)
		return nil
	}
	defer func() { openURL, openFolder, revealFile = oldOpen, oldFolder, oldFile }()

	storyVault := filepath.Join(tg.root, "oldbook", "output", "novel", "nao")
	projectVault := filepath.Join(tg.root, "proj", "nao")
	storyBody := `{"target":"nao-story","project":"_","story":"oldbook"}`
	projectBody := `{"target":"nao-project","project":"proj"}`
	loopback := "127.0.0.1:5555"

	// Kho chưa sinh → 404 kèm câu, không lộ đường dẫn, không mở gì.
	w := doFrom(t, tg.g, "POST", "/api/reveal", admin, storyBody, loopback, nil)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "Chưa có kho ghi chú của truyện") || len(opened) != 0 {
		t.Fatalf("kho truyện chưa có: %d %s opened=%v", w.Code, w.Body.String(), opened)
	}
	if strings.Contains(w.Body.String(), tg.root) {
		t.Fatalf("câu 404 lộ đường dẫn: %s", w.Body.String())
	}

	// Có kho → mở bằng Obsidian, câu trả 200 không có đường dẫn.
	if err := os.MkdirAll(storyVault, 0o755); err != nil {
		t.Fatal(err)
	}
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, storyBody, loopback, nil)
	if w.Code != 200 {
		t.Fatalf("mở kho truyện: %d %s", w.Code, w.Body.String())
	}
	wantStory := "obsidian://open?path=" + strings.ReplaceAll(url.QueryEscape(storyVault), "+", "%20")
	if len(opened) != 1 || opened[0] != wantStory {
		t.Fatalf("opened = %v, want %q", opened, wantStory)
	}
	if body := w.Body.String(); strings.Contains(body, "path") || strings.Contains(body, tg.root) {
		t.Fatalf("câu trả lộ đường dẫn: %s", body)
	}

	// Kho dự án: chưa có → 404 câu riêng; có → mở đúng kho dự án.
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, projectBody, loopback, nil)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "Chưa có kho ghi chú của dự án") || len(opened) != 1 {
		t.Fatalf("kho dự án chưa có: %d %s opened=%v", w.Code, w.Body.String(), opened)
	}
	if err := os.MkdirAll(projectVault, 0o755); err != nil {
		t.Fatal(err)
	}
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, projectBody, loopback, nil)
	wantProject := "obsidian://open?path=" + strings.ReplaceAll(url.QueryEscape(projectVault), "+", "%20")
	if w.Code != 200 || len(opened) != 2 || opened[1] != wantProject {
		t.Fatalf("kho dự án: %d %s opened=%v", w.Code, w.Body.String(), opened)
	}

	// Quyền: chỉ quản trị, cả đọc lẫn mở (nhóm Nâng cao).
	if code := tg.do("GET", "/api/reveal?target=nao-story&project=_&story=oldbook", reader, "").Code; code != 403 {
		t.Fatalf("reader GET: %d", code)
	}
	if w := tg.do("POST", "/api/reveal", reader, storyBody); w.Code != 403 || len(opened) != 2 {
		t.Fatalf("reader POST: %d opened=%v", w.Code, opened)
	}

	// GET của quản trị: chỉ cho biết mở được hay không, không trả đường dẫn.
	w = doFrom(t, tg.g, "GET", "/api/reveal?target=nao-story&project=_&story=oldbook", admin, "", loopback, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), tg.root) || !strings.Contains(w.Body.String(), `"canReveal":true`) {
		t.Fatalf("admin GET: %d %s", w.Code, w.Body.String())
	}

	// Ngoài loopback: không mở (như mọi target khác).
	w = doFrom(t, tg.g, "POST", "/api/reveal", admin, storyBody, "192.0.2.1:1234", nil)
	if w.Code != 403 || !strings.Contains(w.Body.String(), revealLoopbackError) || len(opened) != 2 {
		t.Fatalf("non-loopback: %d %s opened=%v", w.Code, w.Body.String(), opened)
	}

	// Tham số thiếu/sai.
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"target":"nao-story","story":"oldbook"}`, 400},
		{`{"target":"nao-story","project":"_"}`, 400},
		{`{"target":"nao-story","project":"nope","story":"oldbook"}`, 404},
		{`{"target":"nao-story","project":"_","story":"nope"}`, 404},
		{`{"target":"nao-project"}`, 400},
	} {
		if w := doFrom(t, tg.g, "POST", "/api/reveal", admin, c.body, loopback, nil); w.Code != c.want {
			t.Fatalf("%s: %d %s", c.body, w.Code, w.Body.String())
		}
	}
	if len(opened) != 2 {
		t.Fatalf("tham số sai vẫn mở: %v", opened)
	}
}

// obsidianURL mã hoá đường dẫn tuyệt đối; dấu cách phải là %20 (không phải dấu +).
func TestObsidianURLEncodesPath(t *testing.T) {
	got := obsidianURL("/tmp/kho ghi chú/nao")
	want := "obsidian://open?path=%2Ftmp%2Fkho%20ghi%20ch%C3%BA%2Fnao"
	if got != want {
		t.Fatalf("obsidianURL = %q, want %q", got, want)
	}
}

// GET cho target kho ghi chú không trả `path` (invariant #7) — chỉ `canReveal`.
func TestRevealNaoVaultGetHasNoPath(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	if err := os.MkdirAll(filepath.Join(tg.root, "proj", "nao"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := doFrom(t, tg.g, "GET", "/api/reveal?target=nao-project&project=proj", admin, "", "192.0.2.1:1234", nil)
	if w.Code != 200 {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "path") || strings.Contains(body, tg.root) {
		t.Fatalf("GET lộ đường dẫn: %s", body)
	}
	if !strings.Contains(body, `"canReveal":false`) {
		t.Fatalf("GET ngoài loopback: %s", body)
	}
}
