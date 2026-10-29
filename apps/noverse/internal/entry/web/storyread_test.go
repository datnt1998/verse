package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

func TestChapterFactsArrayShape(t *testing.T) {
	res := httptest.NewRecorder()
	writeEngineJSON(res, &domain.ChapterFacts{Title: "Chương"}, nil)
	var body map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"characters", "key_events", "timeline_events", "foreshadow_updates", "relationship_changes", "state_changes", "cast_intros", "places", "objects"} {
		if string(body[key]) != "[]" {
			t.Errorf("%s must be [], got %s", key, body[key])
		}
	}
}

// storyFixture 在书库根下构造 <project>/<stories>/<story>/output/novel 的故事。
func storyFixture(t *testing.T, completed []int) (library.Story, *store.Store) {
	t.Helper()
	root := t.TempDir()
	storyDir := filepath.Join(root, "du-an", "kich_ban", "01-test-story")
	engineDir := filepath.Join(storyDir, "output", "novel")
	s := store.NewStore(engineDir)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := s.Book.Save(domain.BookMetadata{Title: "Truyện Kiểm", Synopsis: "x"}); err != nil {
		t.Fatalf("book: %v", err)
	}
	outline := make([]domain.OutlineEntry, 0, len(completed))
	for _, ch := range completed {
		outline = append(outline, domain.OutlineEntry{Chapter: ch, Title: "Ch", CoreEvent: "e"})
	}
	if err := s.Outline.SaveOutline(outline); err != nil {
		t.Fatalf("outline: %v", err)
	}
	if err := s.Progress.Init(len(outline)); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	for _, ch := range completed {
		if err := s.Drafts.SaveFinalChapter(ch, "nội dung chương"); err != nil {
			t.Fatalf("chapter: %v", err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 3, "", ""); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}
	// 锁文件模拟 TUI 持锁：直读必须照常工作。
	if err := os.WriteFile(filepath.Join(engineDir, ".ainovel.lock"), []byte("pid"), 0o644); err != nil {
		t.Fatalf("lock: %v", err)
	}
	st := library.Story{
		Project: "du-an", Story: "01-test-story", ID: "du-an/01-test-story",
		Dir: storyDir, ProjectDir: filepath.Join(root, "du-an"), Root: root,
	}
	return st, s
}

func adminUser() webauth.User {
	return webauth.User{Username: "admin", Role: webauth.RoleAdmin}
}

// storyReq 复刻 serveStories 的路由级门禁再调 serveStoryRead：未授权一律 404，
// 与 HTTP 层行为一致（授权不在各 handler 里）。
func storyReq(t *testing.T, g *Gateway, method, path string, user webauth.User, st library.Story, rest string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	if !storyAccessAllowed(user, st, method, rest) {
		jsonError(rec, http.StatusNotFound, "không tìm thấy truyện")
		return rec
	}
	if !g.serveStoryRead(rec, req, user, st, rest) {
		t.Fatalf("serveStoryRead returned false for %s %s (rest=%q)", method, path, rest)
	}
	return rec
}

func TestServeStoryRead_ChaptersAndText(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2})
	if err := s.Drafts.SaveDraft(1, "bản nháp"); err != nil {
		t.Fatalf("draft: %v", err)
	}

	rec := storyReq(t, g, "GET", "/x/chapters", adminUser(), st, "/chapters", "")
	if rec.Code != 200 {
		t.Fatalf("chapters: %d %s", rec.Code, rec.Body)
	}
	var list struct {
		Chapters []struct {
			Chapter  int  `json:"chapter"`
			HasDraft bool `json:"hasDraft"`
			Volume   int  `json:"volume"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Chapters) != 2 || !list.Chapters[0].HasDraft || list.Chapters[0].Volume != 1 {
		t.Fatalf("chapters = %+v", list.Chapters)
	}

	rec = storyReq(t, g, "GET", "/x/chapters/1", adminUser(), st, "/chapters/1", "")
	if rec.Code != 200 {
		t.Fatalf("chapter text: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nội dung chương") {
		t.Errorf("text missing: %s", rec.Body)
	}

	rec = storyReq(t, g, "GET", "/x/chapters/1?source=draft", adminUser(), st, "/chapters/1", "")
	if !strings.Contains(rec.Body.String(), "bản nháp") {
		t.Errorf("draft missing: %s", rec.Body)
	}

	rec = storyReq(t, g, "GET", "/x/chapters/9", adminUser(), st, "/chapters/9", "")
	if rec.Code != 404 {
		t.Errorf("missing chapter should be 404, got %d", rec.Code)
	}
}

func TestServeStoryRead_OutlineReviewsCharacters(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "global", Verdict: "accept", Summary: "ok"}); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{{ID: "f1", PlantedAt: 1, Status: "planted"}}); err != nil {
		t.Fatalf("foreshadow: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{{Name: "A"}}); err != nil {
		t.Fatalf("chars: %v", err)
	}

	for _, rest := range []string{"/outline", "/characters", "/foreshadow", "/reviews/latest", "/reviews/1"} {
		rec := storyReq(t, g, "GET", "/x"+rest, adminUser(), st, rest, "")
		if rec.Code != 200 {
			t.Errorf("%s → %d %s", rest, rec.Code, rec.Body)
		}
	}
	rec := storyReq(t, g, "GET", "/x/outline", adminUser(), st, "/outline", "")
	if !strings.Contains(rec.Body.String(), `"layered":false`) {
		t.Errorf("outline should be flat: %s", rec.Body)
	}
}

func TestServeStoryRead_ExportDownloadAndFile(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1, 2})

	rec := storyReq(t, g, "GET", "/x/export?format=txt&from=2", adminUser(), st, "/export", "")
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Chương 2") {
		t.Errorf("expected vi heading Chương 2, got:\n%s", body)
	}
	if strings.Contains(body, "Chương 1") {
		t.Errorf("from=2 should exclude ch1:\n%s", body)
	}

	rec = storyReq(t, g, "GET", "/x/export?format=bogus", adminUser(), st, "/export", "")
	if rec.Code != 400 {
		t.Errorf("bad format should be 400, got %d", rec.Code)
	}
}

// TestServeStoryRead_ExportNarration 钉死 GET /export 的 narration：
// 文件名 -narration.txt、text/plain、无标题无 Markdown；未知格式仍 400。
func TestServeStoryRead_ExportNarration(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2})
	if err := s.Drafts.SaveFinalChapter(1, "# Mở đầu\n\nCâu *đầu* tiên của 1984.\n\nĐoạn hai."); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, "GET", "/x/export?format=narration&readNumbers=1", adminUser(), st, "/export", "")
	if rec.Code != 200 {
		t.Fatalf("narration export: %d %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("content-type = %q", ct)
	}
	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "-narration.txt") {
		t.Errorf("Content-Disposition = %q, want -narration.txt", disp)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Chương") || strings.Contains(body, "Mở đầu") || strings.Contains(body, "*") {
		t.Errorf("narration must have no titles/markdown:\n%s", body)
	}
	if !strings.Contains(body, "một nghìn chín trăm tám mươi tư") {
		t.Errorf("readNumbers=1 should read digits:\n%s", body)
	}
	if strings.Contains(body, "Truyện Kiểm") {
		t.Errorf("narration must not contain book title:\n%s", body)
	}

	rec = storyReq(t, g, "GET", "/x/export?format=bogus", adminUser(), st, "/export", "")
	if rec.Code != 400 {
		t.Errorf("bogus format should be 400, got %d", rec.Code)
	}
}

// TestServeStoryRead_PostExportsNarration 钉死 POST /exports 的 narration 与 readNumbers。
func TestServeStoryRead_PostExportsNarration(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2})
	if err := s.Drafts.SaveFinalChapter(1, "# Mở đầu\n\nCâu *đầu* tiên."); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports",
		`{"format":"narration","from":1,"to":2,"readNumbers":true}`)
	if rec.Code != 200 {
		t.Fatalf("POST narration export: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Path     string `json:"path"`
		Chapters int    `json:"chapters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Chapters != 2 || !strings.HasSuffix(res.Path, ".txt") {
		t.Fatalf("result = %+v", res)
	}
	full := filepath.Join(st.Dir, "xuat-ban", filepath.FromSlash(res.Path))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("exported file missing: %v", err)
	}
	if strings.Contains(string(raw), "Chương") || strings.Contains(string(raw), "*") {
		t.Errorf("narration file must have no titles/markdown:\n%s", raw)
	}

	rec = storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports", `{"format":"bogus"}`)
	if rec.Code != 400 {
		t.Errorf("bogus format should be 400, got %d", rec.Code)
	}
}

func TestServeStoryRead_PostExportsAndFileDownload(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1, 2})

	rec := storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports",
		`{"format":"txt","from":1,"to":2}`)
	if rec.Code != 200 {
		t.Fatalf("POST exports: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Path     string `json:"path"`
		Chapters int    `json:"chapters"`
		Skipped  []int  `json:"skipped"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Chapters != 2 || res.Path == "" {
		t.Fatalf("result = %+v", res)
	}
	// 文件必须落在 <story>/xuat-ban 下（默认 dirs.exports + pattern）。
	full := filepath.Join(st.Dir, "xuat-ban", filepath.FromSlash(res.Path))
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("exported file missing: %v (rel=%q)", err, res.Path)
	}

	// GET /exports 列表应包含该文件。
	rec = storyReq(t, g, "GET", "/x/exports", adminUser(), st, "/exports", "")
	if rec.Code != 200 {
		t.Fatalf("GET exports: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "xuat-ban") {
		t.Errorf("exports list: %s", rec.Body)
	}

	// 文件下载。
	rec = storyReq(t, g, "GET", "/x/exports/file?path="+res.Path, adminUser(), st, "/exports/file", "")
	if rec.Code != 200 {
		t.Fatalf("exports/file: %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "Chương 1") {
		t.Errorf("downloaded content missing: %s", rec.Body)
	}

	// 路径穿越必须拒绝。
	for _, evil := range []string{"../story.json", "..%2fstory.json", "/etc/passwd", "sub/../../x"} {
		rec = storyReq(t, g, "GET", "/x/exports/file?path="+evil, adminUser(), st, "/exports/file", "")
		if rec.Code == 200 {
			t.Errorf("path %q must not be served", evil)
		}
	}
}

// 未授权的写请求在路由级门禁被 404 隐藏（storyReq 复刻该门禁）。
func TestServeStoryRead_PostExportsHiddenForReader(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	reader := webauth.User{Username: "doc-gia", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{st.Project: webauth.ProjectView}}
	rec := storyReq(t, g, "POST", "/x/exports", reader, st, "/exports", `{"format":"txt"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("reader POST should be 404, got %d", rec.Code)
	}
}

func TestServeStoryRead_FallsThroughToWorker(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	for _, rest := range []string{"/snapshot", "/events/live", "/cocreate", "/input", "/unknown"} {
		req := httptest.NewRequest("GET", "/x"+rest, nil)
		rec := httptest.NewRecorder()
		if g.serveStoryRead(rec, req, adminUser(), st, rest) {
			t.Errorf("rest %q should fall through to worker proxy", rest)
		}
	}
	// 非 GET 的只读端点也 fall through（worker 侧可能接管）。
	req := httptest.NewRequest("POST", "/x/chapters", strings.NewReader("{}"))
	if g.serveStoryRead(httptest.NewRecorder(), req, adminUser(), st, "/chapters") {
		t.Error("POST /chapters should fall through")
	}
}
