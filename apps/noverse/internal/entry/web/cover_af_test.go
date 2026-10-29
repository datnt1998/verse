package web

// Gói AF / A7: cover 端点、shortTitle API、Content-Disposition 编码。

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"net/http/httptest"
)

// coverReq 只走 serveStoryRead 的读路由；rest 只含 path，query 进 request URL。
func coverReq(t *testing.T, g *Gateway, st library.Story, path string) *httptest.ResponseRecorder {
	t.Helper()
	rest := path
	if i := strings.Index(rest, "?"); i >= 0 {
		rest = rest[:i]
	}
	return storyReq(t, g, "GET", "/x"+path, adminUser(), st, rest, "")
}

func TestStoryCoverGenerated(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	if err := library.WriteStoryFile(st.Dir, library.StoryFile{Title: "Gia tộc bị bỏ rơi", Status: "idea", Tags: []string{}}); err != nil {
		t.Fatal(err)
	}
	for _, size := range []string{"", "96", "192", "512", "1400"} {
		path := "/cover"
		if size != "" {
			path += "?size=" + size
		}
		rec := coverReq(t, g, st, path)
		if rec.Code != 200 {
			t.Fatalf("size=%q → %d %s", size, rec.Code, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("size=%q Content-Type=%q", size, ct)
		}
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("size=%q không phải PNG: %v", size, err)
		}
		want := 512
		if size != "" {
			want = mustAtoi(size)
		}
		if img.Bounds().Dx() != want || img.Bounds().Dy() != want {
			t.Fatalf("size=%q → %dx%d, muốn %d", size, img.Bounds().Dx(), img.Bounds().Dy(), want)
		}
	}
	// 确定性：同请求 → 同字节。
	r1 := coverReq(t, g, st, "/cover?size=256")
	r2 := coverReq(t, g, st, "/cover?size=256")
	if !bytes.Equal(r1.Body.Bytes(), r2.Body.Bytes()) {
		t.Fatal("generated cover không xác định")
	}
	for _, bad := range []string{"32", "513", "abc", "0"} {
		rec := coverReq(t, g, st, "/cover?size="+bad)
		if rec.Code != 400 {
			t.Fatalf("size=%s → %d, muốn 400", bad, rec.Code)
		}
	}
}

func TestStoryCoverUploaded(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, []int{1})
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	layout, err := audio.ResolveLayout(st)
	if err != nil {
		t.Fatal(err)
	}
	bia := layout.CoverDir()
	if err := os.MkdirAll(bia, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bia, "bia.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := coverReq(t, g, st, "/cover?size=96")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	img, _ := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if img.Bounds().Dx() != 96 || img.Bounds().Dy() != 96 {
		t.Fatalf("uploaded cover = %dx%d, muốn 96x96", img.Bounds().Dx(), img.Bounds().Dy())
	}
	// 红图 → 中心像素红；生成图是紫渐变。
	px := color.RGBAModel.Convert(img.At(48, 48)).(color.RGBA)
	if px.R < 150 {
		t.Fatalf("không phải ảnh tải lên: center px = %+v", px)
	}
}

func TestShortTitleAPI(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	// 建故事：用现成 PUT meta 端点（proj 项目下先建故事目录）。
	mustWriteStoryForAF(t, tg.root)
	base := "/api/projects/proj/stories/af-story"
	body := `{"title":"Truyện","status":"idea","idea":"","tags":[],"shortTitle":"Trà Xanh"}`
	rec := tg.do("PUT", base+"/meta", admin, body)
	if rec.Code != 200 {
		t.Fatalf("PUT meta: %d %s", rec.Code, rec.Body)
	}
	var saved map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["shortTitle"] != "Trà Xanh" {
		t.Fatalf("shortTitle = %v", saved["shortTitle"])
	}
	rec = tg.do("GET", base+"/meta", admin, "")
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["shortTitle"] != "Trà Xanh" {
		t.Fatalf("GET meta shortTitle = %v", got["shortTitle"])
	}
	long := strings.Repeat("a", 33)
	if rec := tg.do("PUT", base+"/meta", admin, `{"title":"x","shortTitle":"`+long+`"}`); rec.Code != 400 {
		t.Fatalf("shortTitle 33 ký tự → %d", rec.Code)
	}
	if rec := tg.do("PUT", base+"/meta", admin, `{"title":"x","shortTitle":"a/b"}`); rec.Code != 400 {
		t.Fatalf("shortTitle / → %d", rec.Code)
	}
}

// mustWriteStoryForAF: proj/kich_ban/af-story/{story.json,output/novel}。
func mustWriteStoryForAF(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "proj", "kich_ban", "af-story")
	if err := os.MkdirAll(filepath.Join(dir, "output", "novel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "story.json"),
		[]byte(`{"title":"AF Story","status":"idea","tags":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContentDispositionASCII(t *testing.T) {
	got := contentDisposition("Truyện · Tập 1 · Chương 02 · Bảy (bản thử).mp3")
	if !strings.HasPrefix(got, `attachment; filename="`) {
		t.Fatalf("thiếu filename fallback: %q", got)
	}
	end := strings.Index(got[len(`attachment; filename="`):], `"`)
	ascii := got[len(`attachment; filename="`):][:end]
	for _, r := range ascii {
		if r > 0x7E {
			t.Fatalf("filename fallback không ASCII: %q", ascii)
		}
	}
	if !strings.Contains(ascii, "Truyen") || strings.Contains(ascii, "Chương") {
		t.Fatalf("fallback giữ nguyên dấu: %q", ascii)
	}
	if !strings.Contains(got, "filename*=UTF-8''") {
		t.Fatalf("thiếu filename*: %q", got)
	}
	dec, err := url.PathUnescape(got[strings.Index(got, "filename*=UTF-8''")+len("filename*=UTF-8''"):])
	if err != nil || dec != "Truyện · Tập 1 · Chương 02 · Bảy (bản thử).mp3" {
		t.Fatalf("filename* decode = %q err=%v", dec, err)
	}
	inj := contentDisposition(`x"; y.mp3`)
	// 引号被剥 → ASCII fallback 只有一个完整 quoted-string。
	if strings.Count(inj, `"`) != 2 {
		t.Fatalf("quote 注入: %q", inj)
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
