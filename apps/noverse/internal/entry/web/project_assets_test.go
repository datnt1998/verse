package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (tg *testGateway) uploadAsset(cookie, project, kind, filename string, data []byte) *httptest.ResponseRecorder {
	tg.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if kind != "" {
		_ = mw.WriteField("kind", kind)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		tg.t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/projects/"+project+"/assets", &body)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func TestUploadProjectAssets(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")
	assets := filepath.Join(tg.root, "proj", "tai-san")

	// Bìa → <assets>/bia/, nhạc → <assets>/nhac/; đường dẫn trả về tương đối thư mục tư liệu.
	w := tg.uploadAsset(admin, "proj", "cover", "Bìa Tập 1.PNG", []byte("png-1"))
	if w.Code != 201 {
		t.Fatalf("cover: %d %s", w.Code, w.Body.String())
	}
	if got := decodeJSONBody(t, w)["path"]; got != "bia/Bìa Tập 1.png" {
		t.Fatalf("cover path = %v", got)
	}
	if b, err := os.ReadFile(filepath.Join(assets, "bia", "Bìa Tập 1.png")); err != nil || string(b) != "png-1" {
		t.Fatalf("cover file: %q %v", b, err)
	}
	w = tg.uploadAsset(admin, "proj", "music", "mo-dau.mp3", []byte("mp3"))
	if w.Code != 201 || decodeJSONBody(t, w)["path"] != "nhac/mo-dau.mp3" {
		t.Fatalf("music: %d %s", w.Code, w.Body.String())
	}

	// Trùng tên: giữ bản cũ, bản mới thêm hậu tố.
	w = tg.uploadAsset(admin, "proj", "music", "mo-dau.mp3", []byte("mp3-new"))
	if w.Code != 201 || decodeJSONBody(t, w)["path"] != "nhac/mo-dau-2.mp3" {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(assets, "nhac", "mo-dau.mp3")); string(b) != "mp3" {
		t.Fatalf("original overwritten: %q", b)
	}

	// Tên có dấu gạch chéo không thoát khỏi thư mục.
	w = tg.uploadAsset(admin, "proj", "cover", "../../x.jpg", []byte("x"))
	if w.Code != 201 || decodeJSONBody(t, w)["path"] != "bia/x.jpg" {
		t.Fatalf("traversal: %d %s", w.Code, w.Body.String())
	}

	// Sai định dạng / sai loại / không có quyền → báo bằng lời.
	if w := tg.uploadAsset(admin, "proj", "cover", "a.exe", []byte("x")); w.Code != 400 || !strings.Contains(w.Body.String(), "Định dạng chưa hỗ trợ") {
		t.Fatalf("ext: %d %s", w.Code, w.Body.String())
	}
	if w := tg.uploadAsset(admin, "proj", "cover", "a.mp3", []byte("x")); w.Code != 400 {
		t.Fatalf("music file as cover: %d", w.Code)
	}
	if w := tg.uploadAsset(admin, "proj", "voice", "a.wav", []byte("x")); w.Code != 400 {
		t.Fatalf("kind: %d", w.Code)
	}
	if w := tg.uploadAsset(admin, "proj", "", "a.png", []byte("x")); w.Code != 400 {
		t.Fatalf("missing kind: %d", w.Code)
	}
	if w := tg.uploadAsset(reader, "proj", "cover", "a.png", []byte("x")); w.Code == 201 {
		t.Fatalf("reader uploaded: %d", w.Code)
	}
	if w := tg.uploadAsset("", "proj", "cover", "a.png", []byte("x")); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}

	// Danh sách tư liệu thấy các tệp mới.
	list := tg.do("GET", "/api/projects/proj/assets", admin, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), "nhac/mo-dau-2.mp3") {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
}
