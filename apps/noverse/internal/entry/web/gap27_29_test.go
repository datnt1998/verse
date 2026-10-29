package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// GAP-27：POST S/exports nhận onExists = replace | keep | fail (mặc định fail).
func TestPostExportsOnExistsStrategies(t *testing.T) {
	g := &Gateway{Root: t.TempDir()}
	defer g.jobsHub().close()
	st, _ := storyFixture(t, []int{1, 2})
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		return storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports", body)
	}
	type okBody struct {
		Path string `json:"path"`
		Job  string `json:"job"`
	}
	decode := func(rec *httptest.ResponseRecorder) okBody {
		t.Helper()
		var v okBody
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
		return v
	}

	first := post(`{"format":"txt","from":1,"to":2}`)
	if first.Code != 200 {
		t.Fatalf("first export: %d %s", first.Code, first.Body)
	}
	orig := decode(first)
	origFull := filepath.Join(st.Dir, "xuat-ban", filepath.FromSlash(orig.Path))
	if err := os.WriteFile(origFull, []byte("bản cũ"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobsBefore := len(g.jobsHub().list(webauth.User{Role: webauth.RoleAdmin}))

	// Mặc định (không có onExists) và onExists=fail: 409, không ghi, không tạo việc thất bại.
	for _, body := range []string{`{"format":"txt","from":1,"to":2}`, `{"format":"txt","from":1,"to":2,"onExists":"fail"}`} {
		rec := post(body)
		if rec.Code != http.StatusConflict {
			t.Fatalf("%s: want 409, got %d %s", body, rec.Code, rec.Body)
		}
		var v struct {
			Error    string         `json:"error"`
			Existing map[string]any `json:"existing"`
			Incoming map[string]any `json:"incoming"`
			Result   struct {
				Kind   string         `json:"kind"`
				Exists map[string]any `json:"exists"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.Error == "" || v.Result.Kind != "exportExists" || v.Existing == nil || v.Incoming == nil || v.Result.Exists["format"] != "txt" {
			t.Fatalf("conflict body shape: %s", rec.Body)
		}
		if strings.Contains(rec.Body.String(), st.Dir) || filepath.IsAbs(v.Existing["path"].(string)) {
			t.Fatalf("conflict leaks absolute path: %s", rec.Body)
		}
		if raw, _ := os.ReadFile(origFull); string(raw) != "bản cũ" {
			t.Fatalf("fail strategy modified the existing file: %q", raw)
		}
	}
	if got := len(g.jobsHub().list(webauth.User{Role: webauth.RoleAdmin})); got != jobsBefore {
		t.Fatalf("409 conflict registered a job: before %d after %d", jobsBefore, got)
	}

	// keep: tệp mới có hậu tố -YYYYMMDD-HHmmss, tệp cũ còn nguyên.
	keep := post(`{"format":"txt","from":1,"to":2,"onExists":"keep"}`)
	if keep.Code != 200 {
		t.Fatalf("keep: %d %s", keep.Code, keep.Body)
	}
	kept := decode(keep)
	if !regexp.MustCompile(`-\d{8}-\d{6}(-\d+)?\.txt$`).MatchString(kept.Path) || kept.Path == orig.Path {
		t.Fatalf("keep path = %q (orig %q)", kept.Path, orig.Path)
	}
	if kept.Job == "" {
		t.Fatal("keep export must return a job id")
	}
	if raw, _ := os.ReadFile(origFull); string(raw) != "bản cũ" {
		t.Fatalf("keep modified the existing file: %q", raw)
	}

	// replace: ghi đè tại chỗ (cờ overwrite cũ đã bỏ — gửi lên sẽ bị 400).
	for _, body := range []string{`{"format":"txt","from":1,"to":2,"onExists":"replace"}`} {
		if err := os.WriteFile(origFull, []byte("bản cũ"), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := post(body)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body)
		}
		if decode(rec).Path != orig.Path {
			t.Fatalf("%s: replace must reuse the same path", body)
		}
		if raw, _ := os.ReadFile(origFull); string(raw) == "bản cũ" {
			t.Fatalf("%s: existing file was not replaced", body)
		}
	}

	// Giá trị lạ → 400.
	if rec := post(`{"format":"txt","onExists":"bogus"}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), "onExists") {
		t.Fatalf("invalid onExists: %d %s", rec.Code, rec.Body)
	}
	// GAP-40: `overwrite` đã bị bỏ khỏi API (thay bằng onExists) → trường lạ bị từ chối.
	if rec := post(`{"format":"txt","from":1,"to":2,"overwrite":true}`); rec.Code != 400 {
		t.Fatalf("overwrite should be rejected: %d %s", rec.Code, rec.Body)
	}
}

// controlProvider là nguồn việc ở gateway cho kiểm thử POST /api/jobs/:id/:op.
type controlProvider struct {
	jobs  []Job
	err   error
	calls []string
}

func (p *controlProvider) Kind() string                                    { return "audio" }
func (p *controlProvider) Jobs() []Job                                     { return p.jobs }
func (p *controlProvider) Watch(ctx context.Context, emit func(JobChange)) { <-ctx.Done() }
func (p *controlProvider) Control(_ context.Context, id, op string) error {
	p.calls = append(p.calls, op+" "+id)
	return p.err
}

// GAP-26：điều khiển việc chạy nền qua gateway — quyền, mã lỗi, việc đã xong bị bỏ khỏi danh sách.
func TestJobControlHTTPPermissionsAndErrors(t *testing.T) {
	tg := newTestGateway(t)
	hub := tg.g.jobsHub()
	provider := &controlProvider{}
	hub.addProvider(provider)
	hub.apply(JobChange{Job: Job{ID: "proj/s1/audio/1", Kind: "audio", Project: "proj", Story: "s1", Status: JobRunning}})
	hub.apply(JobChange{Job: Job{ID: "_/oldbook/audio/1", Kind: "audio", Project: "_", Story: "oldbook", Status: JobRunning}})
	hub.apply(JobChange{Job: Job{ID: "secret/x/audio/1", Kind: "audio", Project: "secret", Story: "x", Status: JobRunning}})
	hub.apply(JobChange{Job: Job{ID: "proj/s1/audio/2", Kind: "audio", Project: "proj", Story: "s1", Status: JobDoneS}})
	writer, reader := tg.login("writer", "writer-pass"), tg.login("reader", "reader-pass")

	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/pause", "", `{}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/pause", writer, `{}`); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"ok":true}` {
		t.Fatalf("writer pause: %d %s", w.Code, w.Body)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "pause proj/s1/audio/1" {
		t.Fatalf("provider calls = %v", provider.calls)
	}
	// Quyền xem không đủ để điều khiển việc; dự án chưa được gán cũng bị từ chối.
	if w := tg.do("POST", "/api/jobs/_/oldbook/audio/1/cancel", reader, `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("reader cancel: %d %s", w.Code, w.Body)
	}
	if w := tg.do("POST", "/api/jobs/secret/x/audio/1/cancel", writer, `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("unreadable story: %d %s", w.Code, w.Body)
	}
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/explode", writer, `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown op: %d", w.Code)
	}
	provider.err = errJobUnsupported
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/retry", writer, `{}`); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), errJobUnsupported.Error()) {
		t.Fatalf("unsupported op: %d %s", w.Code, w.Body)
	}
	provider.err = errJobNotFound
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/resume", writer, `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("source says not found: %d %s", w.Code, w.Body)
	}
	provider.err = errors.New("hỏng")
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/1/resume", writer, `{}`); w.Code != http.StatusConflict {
		t.Fatalf("source error: %d %s", w.Code, w.Body)
	}
	// Huỷ việc đã xong = bỏ khỏi danh sách, kể cả khi nguồn báo lỗi.
	if w := tg.do("POST", "/api/jobs/proj/s1/audio/2/cancel", writer, `{}`); w.Code != 200 {
		t.Fatalf("dismiss done job: %d %s", w.Code, w.Body)
	}
	if _, ok := hub.get("proj/s1/audio/2"); ok {
		t.Fatal("done job still listed after cancel")
	}

	// GET /api/jobs chỉ trả việc tài khoản được đọc.
	var listed struct {
		Jobs []Job `json:"jobs"`
	}
	w := tg.do("GET", "/api/jobs", reader, "")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Jobs) != 1 || listed.Jobs[0].ID != "_/oldbook/audio/1" {
		t.Fatalf("reader jobs = %+v", listed.Jobs)
	}
}
