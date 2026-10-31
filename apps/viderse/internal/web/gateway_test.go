package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/platform/identity"
)

const testCookie = "verse_session"

// fakeSettings là SettingsStore giả của status.Service cho test web.
type fakeSettings struct {
	paused bool
	err    error
}

func (f *fakeSettings) QueuePaused(context.Context) (bool, error) { return f.paused, f.err }

type testGateway struct {
	t   *testing.T
	g   *Gateway
	svc *status.Service
	set *fakeSettings
}

func newTestGateway(t *testing.T) *testGateway {
	t.Helper()
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), identity.Options{})
	if _, err := accounts.CreateUser(identity.NewUser{
		Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass",
	}); err != nil {
		t.Fatal(err)
	}
	hub := NewEventHub()
	set := &fakeSettings{}
	svc := status.New(set, PublishStatus(hub))
	t.Cleanup(hub.Close)
	return &testGateway{t: t, g: NewGateway(accounts, hub, svc), svc: svc, set: set}
}

func (tg *testGateway) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	tg.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func (tg *testGateway) sessionCookie() string {
	tg.t.Helper()
	w := tg.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		tg.t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.Value != "" {
			return c.Value
		}
	}
	tg.t.Fatalf("no session cookie (status %d)", w.Code)
	return ""
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body không phải JSON: %q", w.Body.String())
	}
	return m
}

// streamWriter là ResponseWriter+Flusher giả giữ toàn bộ body của luồng SSE.
type streamWriter struct {
	mu     sync.Mutex
	body   strings.Builder
	signal chan struct{}
	header http.Header
}

func newStreamWriter() *streamWriter {
	return &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
}

func (w *streamWriter) Header() http.Header { return w.header }
func (w *streamWriter) WriteHeader(int)     {}
func (w *streamWriter) Flush()              {}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, _ := w.body.Write(p)
	select {
	case w.signal <- struct{}{}:
	default:
	}
	return n, nil
}

func (w *streamWriter) until(t *testing.T, phrase string) string {
	t.Helper()
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		w.mu.Lock()
		body := w.body.String()
		w.mu.Unlock()
		if strings.Contains(body, phrase) {
			return body
		}
		select {
		case <-w.signal:
		case <-timeout.C:
			t.Fatalf("hết giờ chờ %q; body: %q", phrase, body)
		}
	}
}

func TestAPIRoutesRequireSession(t *testing.T) {
	tg := newTestGateway(t)
	for _, path := range []string{"/api/status", "/api/events"} {
		w := tg.do(http.MethodGet, path, "", "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s status %d, muốn 401", path, w.Code)
		}
	}
}

// Luồng SSE ping mỗi 2 s nhưng chỉ kiểm phiên (một truy vấn DB) sau mỗi
// sessionRecheck; phiên hết hiệu lực vẫn đóng luồng ở lần kiểm kế tiếp.
func TestEventSessionRecheckIsThrottled(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	calls, valid := 0, true
	tick := throttledCheck(15*time.Second, func() time.Time { return now }, func() bool { calls++; return valid })
	for i := range 7 { // 7 ping × 2 s = 14 s: chưa tới nhịp kiểm phiên
		now = now.Add(2 * time.Second)
		if !tick() {
			t.Fatalf("ping %d đóng luồng trước nhịp kiểm phiên", i)
		}
	}
	if calls != 0 {
		t.Fatalf("kiểm phiên %d lần trong 14 s, muốn 0", calls)
	}
	now = now.Add(2 * time.Second) // 16 s
	if !tick() || calls != 1 {
		t.Fatalf("sau 16 s: calls=%d, muốn 1 lần kiểm và giữ luồng", calls)
	}
	valid = false // phiên bị thu hồi
	now = now.Add(2 * time.Second)
	if !tick() || calls != 1 {
		t.Fatalf("2 s sau lần kiểm: calls=%d, muốn chưa kiểm lại", calls)
	}
	now = now.Add(14 * time.Second)
	if tick() {
		t.Fatal("phiên bị thu hồi mà luồng vẫn mở sau nhịp kiểm phiên")
	}
	if calls != 2 {
		t.Fatalf("calls=%d, muốn 2", calls)
	}
}

func TestGetStatusShapeAndETag(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/status", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if etag := w.Header().Get("ETag"); len(etag) < 4 || etag[0] != '"' {
		t.Fatalf("ETag phải là chuỗi trích dẫn băm nội dung, được %q", etag)
	}
	m := decode(t, w)
	if m["running"] != float64(0) || m["queued"] != float64(0) || m["paused"] != false || m["reviewCount"] != float64(0) {
		t.Fatalf("hình trạng thái sai: %v", m)
	}
	att, ok := m["attention"].([]any)
	if !ok || len(att) != 0 {
		t.Fatalf("attention phải là mảng rỗng: %v", m["attention"])
	}
}

// ETag suy từ nội dung: hai body khác nhau → hai ETag; nội dung y hệt sau khi
// tiến trình khởi động lại (revision đếm lại từ 0) vẫn cùng ETag.
func TestGetStatusETagFollowsContent(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	first := tg.do(http.MethodGet, "/api/status", cookie, "")
	etagIdle := first.Header().Get("ETag")
	same := tg.do(http.MethodGet, "/api/status", cookie, "")
	if got := same.Header().Get("ETag"); got != etagIdle {
		t.Fatalf("cùng nội dung phải cùng ETag: %q vs %q", etagIdle, got)
	}

	// Khởi động lại: gateway + service mới (revision về 0) cùng nội dung.
	restarted := newTestGateway(t)
	after := restarted.do(http.MethodGet, "/api/status", restarted.sessionCookie(), "")
	if got := after.Header().Get("ETag"); got != etagIdle {
		t.Fatalf("sau restart cùng nội dung phải cùng ETag: %q vs %q", etagIdle, got)
	}

	// Đổi nội dung mà KHÔNG Bump (cờ tạm dừng đọc từ store) → ETag phải đổi và
	// trả 200, không được 304 theo nội dung cũ.
	tg.set.paused = true
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	r.Header.Set("If-None-Match", etagIdle)
	changed := httptest.NewRecorder()
	tg.g.ServeHTTP(changed, r)
	if changed.Code != http.StatusOK {
		t.Fatalf("nội dung đổi phải trả 200, được %d", changed.Code)
	}
	etagBusy := changed.Header().Get("ETag")
	if etagBusy == etagIdle {
		t.Fatalf("nội dung đổi nhưng ETag giữ nguyên %q", etagBusy)
	}
	if m := decode(t, changed); m["paused"] != true {
		t.Fatalf("body mới phải có paused=true: %v", m)
	}
}

func TestGetStatusIfNoneMatchReturns304(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/status", cookie, "")
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("thiếu ETag")
	}

	do := func(inm string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		r.Header.Set("If-None-Match", inm)
		rec := httptest.NewRecorder()
		tg.g.ServeHTTP(rec, r)
		return rec
	}

	for _, inm := range []string{etag, "W/" + etag, `"khác", ` + etag, "*"} {
		w2 := do(inm)
		if w2.Code != http.StatusNotModified {
			t.Fatalf("If-None-Match %q → %d, muốn 304", inm, w2.Code)
		}
		if w2.Body.Len() != 0 {
			t.Fatalf("304 phải rỗng body, có %q", w2.Body.String())
		}
		if w2.Header().Get("ETag") != etag {
			t.Fatalf("304 ETag = %q", w2.Header().Get("ETag"))
		}
	}
	if w3 := do(`"không-khớp"`); w3.Code != http.StatusOK {
		t.Fatalf("ETag không khớp phải 200, được %d", w3.Code)
	}
}

func TestGetStatusReadsPausedSetting(t *testing.T) {
	tg := newTestGateway(t)
	tg.set.paused = true
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodGet, "/api/status", cookie, "")
	if decode(t, w)["paused"] != true {
		t.Fatalf("paused = %v", decode(t, w)["paused"])
	}
}

// Thay đổi trạng thái (Bump) phải chạm tới subscriber SSE dưới dạng frame
// "status" mang rev mới.
func TestStatusChangeReachesSSESubscriber(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	w := newStreamWriter()
	done := make(chan struct{})
	go func() { tg.g.ServeHTTP(w, r); close(done) }()

	body := w.until(t, "event: hello")
	if !strings.Contains(body, `"subscriber"`) {
		t.Fatalf("hello thiếu subscriber: %q", body)
	}
	tg.svc.Bump()
	body = w.until(t, "event: status")
	if !strings.Contains(body, `"rev":1`) {
		t.Fatalf("frame status thiếu rev mới: %q", body)
	}
	cancel()
	<-done
}

type storageHealth struct{ down bool }

func (s *storageHealth) StorageDown() (string, time.Time, bool) {
	return "Garage", time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC), s.down
}

func TestStatusStorageDownAndRoleScopedAttention(t *testing.T) {
	tg := newTestGateway(t)
	health := &storageHealth{down: true}
	tg.svc.SetStorageHealthSource(health)
	tg.svc.RegisterSource(statusSource{rep: status.Report{Attention: []status.Attention{
		{Kind: domain.AttentionToolFailed, Text: "private storage", Href: "#/settings?section=storage"},
		{Kind: domain.AttentionToolFailed, Text: "private task", AdminOnly: true},
		{Kind: domain.AttentionToolFailed, Text: "public tool"},
	}}})
	admin := tg.do(http.MethodGet, "/api/status", tg.sessionCookie(), "")
	if admin.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", admin.Code, admin.Body)
	}
	body := decode(t, admin)
	if down, ok := body["storageDown"].(map[string]any); !ok || down["label"] != "Garage" || down["since"] != "2026-10-31T10:00:00Z" {
		t.Fatalf("outage not serialized: %s", admin.Body)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.Header.Set("If-None-Match", admin.Header().Get("ETag"))
	member := httptest.NewRecorder()
	tg.g.getStatus(member, request, identity.User{Role: identity.RoleMember})
	if member.Code != http.StatusOK {
		t.Fatalf("role change reused private status: %d", member.Code)
	}
	body = decode(t, member)
	if attention := body["attention"].([]any); len(attention) != 1 || attention[0].(map[string]any)["text"] != "public tool" {
		t.Fatalf("member received administrative attention: %v", attention)
	}
	if body["storageDown"] == nil {
		t.Fatal("member safe outage information missing")
	}
	if member.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatal("status is not private")
	}
	health.down = false
	recovered := tg.do(http.MethodGet, "/api/status", tg.sessionCookie(), "")
	if recovered.Code != http.StatusOK {
		t.Fatalf("recovery=%d: %s", recovered.Code, recovered.Body)
	}
	if decode(t, recovered)["storageDown"] != nil {
		t.Fatalf("recovery retained outage: %s", recovered.Body)
	}
	if recovered.Header().Get("ETag") == admin.Header().Get("ETag") {
		t.Fatal("recovery did not invalidate status")
	}
}

type statusSource struct{ rep status.Report }

func (s statusSource) Report(context.Context) (status.Report, error) { return s.rep, nil }
