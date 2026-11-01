package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
)

// zeroBody streams n zero bytes without allocating them: the point is the
// request size the server sees, not the content.
type zeroBody struct{ left int64 }

func (z *zeroBody) Read(p []byte) (int, error) {
	if z.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > z.left {
		p = p[:z.left]
	}
	clear(p)
	z.left -= int64(len(p))
	return len(p), nil
}

// multipartBody builds a one-part multipart/form-data request whose file part
// is `size` zero bytes, streamed rather than buffered.
func multipartBody(size int64, filename, contentType string) (io.Reader, string) {
	const boundary = "harden"
	head := "--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"file\"; filename=\"" + filename + "\"\r\n" +
		"Content-Type: " + contentType + "\r\n\r\n"
	tail := "\r\n--" + boundary + "--\r\n"
	body := io.MultiReader(
		strings.NewReader(head),
		&zeroBody{left: size},
		strings.NewReader(tail),
	)
	return body, "multipart/form-data; boundary=" + boundary
}

func uploadRequest(t *testing.T, tg *testGateway, cookie, path string, size int64, filename, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentTypeHeader := multipartBody(size, filename, contentType)
	r := httptest.NewRequest(http.MethodPost, path, body)
	r.Header.Set("Content-Type", contentTypeHeader)
	r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
	r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

// TestOverlayUploadOf91MiBIsAppJSONError pins the hosted ceiling: a 91 MiB
// overlay never reaches Cloudflare's HTML 413 page, because the app refuses it
// first and says, in Vietnamese, what the limit is.
func TestOverlayUploadOf91MiBIsAppJSONError(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := library.OverlayLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("OverlayLimit() = %d khi hosted, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)

	w := uploadRequest(t, tg, cookie, "/api/overlays", 91<<20, "mua.mp4", "video/mp4")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error    string `json:"error"`
		Recovery string `json:"recovery"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body không phải JSON app: %s", w.Body.String())
	}
	if !strings.Contains(body.Error+body.Recovery, "Tải lên qua internet tối đa 90 MB") {
		t.Fatalf("lời nhắn không nêu trần: %+v", body)
	}
	if fake.uploadFilename != "" {
		t.Fatalf("service phải không được gọi, filename = %q", fake.uploadFilename)
	}
}

// TestTemplateImportOf91MiBIsAppJSONError does the same for a template bundle:
// the ceiling applies to the file and to the whole import.
func TestTemplateImportOf91MiBIsAppJSONError(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := library.TemplateBundleLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("TemplateBundleLimit() = %d khi hosted, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)

	w := uploadRequest(t, tg, cookie, "/api/templates/import", 91<<20, "bo.ytf-template.zip", "application/zip")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error    string `json:"error"`
		Recovery string `json:"recovery"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body không phải JSON app: %s", w.Body.String())
	}
	if !strings.Contains(body.Error+body.Recovery, "Tải lên qua internet tối đa 90 MB") {
		t.Fatalf("lời nhắn không nêu trần: %+v", body)
	}
}

// TestUploadLimitsEndpointReportsEffectiveCeilings keeps the SPA and the server
// on the same numbers: the endpoint reads the same functions the handlers use.
func TestUploadLimitsEndpointReportsEffectiveCeilings(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example")
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	MountUploadLimits(tg.g)

	w := tg.do(http.MethodGet, "/api/upload-limits", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body UploadLimitsWire
	decodeJSON(t, w.Body.Bytes(), &body)
	if !body.Hosted {
		t.Fatal("hosted = false khi có PUBLIC_URL")
	}
	for _, kind := range []string{"video", "audio", "overlay", "template"} {
		if body.Limits[kind] != uploadlimit.PublicMaxBytes {
			t.Fatalf("%s = %d, muốn %d", kind, body.Limits[kind], uploadlimit.PublicMaxBytes)
		}
	}
	if body.Limits["image"] != 50<<20 {
		t.Fatalf("image = %d, ảnh giữ 50 MiB", body.Limits["image"])
	}
	if !strings.Contains(body.Message, "90 MB") {
		t.Fatalf("message = %q", body.Message)
	}

	// Local (no PUBLIC_URL): the ceilings stay the desktop ones.
	t.Setenv("PUBLIC_URL", "")
	MountUploadLimits(tg.g)
	w = tg.do(http.MethodGet, "/api/upload-limits", cookie, "")
	decodeJSON(t, w.Body.Bytes(), &body)
	if body.Hosted || body.Limits["video"] != 20<<30 || body.Limits["overlay"] != library.MaxOverlayBytes {
		t.Fatalf("local limits = %+v", body)
	}
}
