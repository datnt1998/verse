// Ca test chuyển từ src/server/backdrop/bilibili-qr-login.test.ts và
// bilibili-login.test.ts của yt-factory: tạo mã kèm buvid3/4, hỏi mã theo
// từng mã lỗi, đăng nhập xong ghi cookie + refresh token một lần, chuỗi gia
// hạn (cookie/info → correspond → refresh → confirm), ngưỡng 24 giờ, và khoá
// chung cho mọi thao tác đăng nhập.
package accounts

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/gofrs/flock"
)

const loginURL = "https://passport.bilibili.com/h5-app/passport/login/scan?key=test-key"

const oldCookie = "SESSDATA=old-session; bili_jct=old-csrf; buvid3=device3; buvid4=device4"

// qrEnv là môi trường test: server giả, đồng hồ giả, service trỏ vào server.
type qrEnv struct {
	t      *testing.T
	svc    *Service
	dir    string
	now    *time.Time
	mu     sync.Mutex
	calls  []string
	handle func(path string, r *http.Request, w http.ResponseWriter) bool
}

func newQREnv(t *testing.T) *qrEnv {
	t.Helper()
	e := &qrEnv{t: t, dir: t.TempDir(), now: new(time.Time)}
	*e.now = time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.mu.Lock()
		e.calls = append(e.calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		handle := e.handle
		e.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if handle != nil && handle(r.URL.Path, r, w) {
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	e.svc = New(Deps{
		Dir:   e.dir,
		Hosts: BilibiliHosts{API: srv.URL, Passport: srv.URL, Site: srv.URL},
		Now:   func() time.Time { return *e.now },
	})
	return e
}

func (e *qrEnv) setHandle(fn func(path string, r *http.Request, w http.ResponseWriter) bool) {
	e.mu.Lock()
	e.handle = fn
	e.mu.Unlock()
}

func (e *qrEnv) advance(d time.Duration) { *e.now = e.now.Add(d) }

func (e *qrEnv) count(prefix string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// cookie returns a request's Cookie header (helper for handlers).
func cookie(r *http.Request) string { return r.Header.Get("Cookie") }

func writeJSON(w http.ResponseWriter, body string) { _, _ = io.WriteString(w, body) }

// generation trả hai lời gọi tạo mã như bản TS.
func generation(path string, w http.ResponseWriter) bool {
	switch path {
	case "/x/frontend/finger/spi":
		writeJSON(w, `{"code":0,"data":{"b_3":"device3","b_4":"device4"}}`)
		return true
	case "/qrcode/generate":
		writeJSON(w, `{"code":0,"data":{"url":"`+loginURL+`","qrcode_key":"test-key"}}`)
		return true
	}
	return false
}

func seedQRState(t *testing.T, svc *Service, extra bilibiliLoginState) {
	t.Helper()
	st := bilibiliLoginState{Cookie: oldCookie, Method: "qr", Account: new("Cũ"), RefreshToken: new("old-token"), CSRF: new("old-csrf")}
	if extra.Account == nil {
		st.Account = extra.Account
	}
	if extra.LastCheckedAt != nil {
		st.LastCheckedAt = extra.LastCheckedAt
	}
	if err := svc.writeLoginState(&st); err != nil {
		t.Fatal(err)
	}
	if err := svc.setCookieMirror(oldCookie); err != nil {
		t.Fatal(err)
	}
}

func TestStartQRSendsDeviceCookiesAndExpiresIn180s(t *testing.T) {
	e := newQREnv(t)
	e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
		if path == "/qrcode/generate" {
			if cookie(r) != "buvid3=device3; buvid4=device4" {
				t.Fatalf("cookie generate = %q", cookie(r))
			}
		}
		return generation(path, w)
	})
	session, err := e.svc.StartQR(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if session.QRCodeKey != "test-key" || session.LoginURL != loginURL {
		t.Fatalf("session = %+v", session)
	}
	if !session.ExpiresAt.Equal(e.now.Add(180 * time.Second)) {
		t.Fatalf("expiresAt = %v", session.ExpiresAt)
	}
	if !strings.HasPrefix(session.QRImage, "data:image/svg+xml;base64,") {
		t.Fatalf("qrImage = %q", session.QRImage[:32])
	}
	info, err := os.Stat(filepath.Join(e.dir, "bilibili-qr-session.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("phiên QR phải 0600: %v", err)
	}
}

func TestStartQRDeviceFailureIsBestEffortAndRejectsForeignURL(t *testing.T) {
	e := newQREnv(t)
	e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
		if path == "/x/frontend/finger/spi" {
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		if path == "/qrcode/generate" {
			if cookie(r) != "" {
				t.Fatalf("không có buvid thì cookie phải rỗng, được %q", cookie(r))
			}
			writeJSON(w, `{"code":0,"data":{"url":"https://evil.example/qr","qrcode_key":"test-key"}}`)
			return true
		}
		return false
	})
	_, err := e.svc.StartQR(context.Background())
	var le *LoginError
	if !errors.As(err, &le) || le.Reason != ReasonInvalid {
		t.Fatalf("URL lạ phải invalid, được %v", err)
	}
}

func TestPollMapsStatusesAndLocalExpiryDoesNotRequest(t *testing.T) {
	e := newQREnv(t)
	code := 86101
	e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
		if generation(path, w) {
			return true
		}
		if path == "/qrcode/poll" {
			if r.URL.Query().Get("source") != "main-fe-header" || r.URL.Query().Get("qrcode_key") != "test-key" {
				t.Fatalf("poll query = %q", r.URL.RawQuery)
			}
			writeJSON(w, `{"code":0,"data":{"code":`+strconv.Itoa(code)+`}}`)
			return true
		}
		return false
	})
	if _, err := e.svc.StartQR(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		code int
		want domain.QRPollStatus
	}{{86101, domain.QRWaiting}, {86090, domain.QRScanned}, {86038, domain.QRExpired}, {42, domain.QRRejected}} {
		code = c.code
		poll, err := e.svc.PollQR(context.Background(), "test-key")
		if err != nil {
			t.Fatalf("poll %d: %v", c.code, err)
		}
		if poll.Status != c.want {
			t.Fatalf("poll %d = %q, muốn %q", c.code, poll.Status, c.want)
		}
	}
	e.advance(180 * time.Second)
	before := e.count("GET /qrcode/poll")
	poll, err := e.svc.PollQR(context.Background(), "test-key")
	if err != nil || poll.Status != domain.QRExpired {
		t.Fatalf("hết hạn cục bộ = %+v %v", poll, err)
	}
	if after := e.count("GET /qrcode/poll"); after != before {
		t.Fatal("hết hạn cục bộ không được gọi mạng")
	}
}

func TestPollSuccessMergesCookiesAndStoresNoSecretsInView(t *testing.T) {
	e := newQREnv(t)
	e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
		if generation(path, w) {
			return true
		}
		if path == "/qrcode/poll" {
			http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "header-session", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "bili_jct", Value: "header-csrf", Path: "/"})
			writeJSON(w, `{"code":0,"data":{"code":0,"url":"https://passport.bilibili.com/crossDomain?SESSDATA=url-session&bili_jct=url-csrf&DedeUserID=42","refresh_token":"refresh-secret"}}`)
			return true
		}
		if path == "/x/web-interface/nav" {
			for _, want := range []string{"SESSDATA=header-session", "bili_jct=header-csrf", "DedeUserID=42", "buvid3=device3"} {
				if !strings.Contains(cookie(r), want) {
					t.Fatalf("nav cookie %q thiếu %q", cookie(r), want)
				}
			}
			writeJSON(w, `{"code":0,"data":{"isLogin":true,"uname":"Tài khoản phụ"}}`)
			return true
		}
		return false
	})
	if _, err := e.svc.StartQR(context.Background()); err != nil {
		t.Fatal(err)
	}
	poll, err := e.svc.PollQR(context.Background(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if poll.Status != domain.QRSuccess || poll.Account == nil || *poll.Account != "Tài khoản phụ" {
		t.Fatalf("poll = %+v", poll)
	}
	for _, f := range []string{"bilibili-cookie.txt", "bilibili-login.json"} {
		info, err := os.Stat(filepath.Join(e.dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s quyền = %o", f, info.Mode().Perm())
		}
	}
	state, err := e.svc.readLoginState()
	if err != nil || state == nil {
		t.Fatal(err)
	}
	if state.RefreshToken == nil || *state.RefreshToken != "refresh-secret" {
		t.Fatalf("refresh token = %v", state.RefreshToken)
	}
	if state.CSRF == nil || *state.CSRF != "header-csrf" {
		t.Fatalf("csrf = %v", state.CSRF)
	}
	view, err := e.svc.bilibiliLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !view.Renewable || view.Method != "qr" {
		t.Fatalf("view = %+v", view)
	}
	serialized := strings.Join([]string{strOf(view.Masked), strOf(view.Account), strOf(view.RefreshError)}, "")
	for _, secret := range []string{"refresh-secret", "header-csrf", "header-session"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("API lộ bí mật %q trong %q", secret, serialized)
		}
	}
	if _, err := os.Stat(filepath.Join(e.dir, "bilibili-qr-session.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("phiên QR phải bị xoá sau đăng nhập: %v", err)
	}
}

func strOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestPollEdgeCasesDoNotAcceptInvalidLogin(t *testing.T) {
	t.Run("missing csrf", func(t *testing.T) {
		e := newQREnv(t)
		e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
			if generation(path, w) {
				return true
			}
			writeJSON(w, `{"code":0,"data":{"code":0,"url":"https://passport.bilibili.com/crossDomain?SESSDATA=session","refresh_token":"token"}}`)
			return path == "/qrcode/poll"
		})
		if _, err := e.svc.StartQR(context.Background()); err != nil {
			t.Fatal(err)
		}
		var le *LoginError
		if _, err := e.svc.PollQR(context.Background(), "test-key"); !errors.As(err, &le) || le.Reason != ReasonRejected {
			t.Fatalf("thiếu bili_jct phải rejected, được %v", err)
		}
		if _, err := os.Stat(filepath.Join(e.dir, "bilibili-cookie.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("không được lưu cookie thiếu thông tin")
		}
	})
	t.Run("rejected", func(t *testing.T) {
		e := newQREnv(t)
		e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
			if generation(path, w) {
				return true
			}
			if path == "/qrcode/poll" {
				writeJSON(w, `{"code":0,"data":{"code":0,"url":"https://passport.bilibili.com/crossDomain?SESSDATA=session&bili_jct=csrf","refresh_token":"token"}}`)
				return true
			}
			writeJSON(w, `{"code":-101,"data":{"isLogin":false}}`)
			return path == "/x/web-interface/nav"
		})
		if _, err := e.svc.StartQR(context.Background()); err != nil {
			t.Fatal(err)
		}
		poll, err := e.svc.PollQR(context.Background(), "test-key")
		if err != nil || poll.Status != domain.QRRejected {
			t.Fatalf("poll = %+v %v", poll, err)
		}
		if _, err := os.Stat(filepath.Join(e.dir, "bilibili-cookie.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cookie bị từ chối không được lưu")
		}
	})
	t.Run("network", func(t *testing.T) {
		e := newQREnv(t)
		e.setHandle(func(path string, r *http.Request, w http.ResponseWriter) bool {
			if generation(path, w) {
				return true
			}
			if path == "/qrcode/poll" {
				writeJSON(w, `{"code":0,"data":{"code":0,"url":"https://passport.bilibili.com/crossDomain?SESSDATA=session&bili_jct=csrf","refresh_token":"token"}}`)
				return true
			}
			if path == "/x/web-interface/nav" {
				w.WriteHeader(http.StatusBadGateway)
				return true
			}
			return false
		})
		if _, err := e.svc.StartQR(context.Background()); err != nil {
			t.Fatal(err)
		}
		poll, err := e.svc.PollQR(context.Background(), "test-key")
		if err != nil {
			t.Fatal(err)
		}
		if poll.Status != domain.QRSuccess || poll.Account != nil {
			t.Fatalf("poll = %+v", poll)
		}
		view, err := e.svc.bilibiliLogin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if view.RefreshError == nil || !strings.Contains(*view.RefreshError, "chưa kiểm tra") {
			t.Fatalf("refreshError = %v", view.RefreshError)
		}
	})
}

var hex256 = regexp.MustCompile(`^[0-9a-f]{256}$`)

func TestRenewalChainCommitsTogether(t *testing.T) {
	path, err := bilibiliCorrespondPath(1_800_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if !hex256.MatchString(path) {
		t.Fatalf("correspond path = %q", path)
	}

	e := newQREnv(t)
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		switch {
		case p == "/cookie/info":
			if cookie(r) != oldCookie {
				t.Fatalf("cookie info = %q", cookie(r))
			}
			writeJSON(w, `{"code":0,"data":{"refresh":true,"timestamp":1800000000000}}`)
		case strings.HasPrefix(p, "/correspond/1/"):
			if !hex256.MatchString(strings.TrimPrefix(p, "/correspond/1/")) {
				t.Fatalf("correspond = %q", p)
			}
			_, _ = io.WriteString(w, `<html><div id="1-name">refresh-csrf</div></html>`)
		case p == "/cookie/refresh":
			if cookie(r) != oldCookie {
				t.Fatalf("refresh cookie = %q", cookie(r))
			}
			_ = r.ParseForm()
			if r.PostForm.Get("csrf") != "old-csrf" || r.PostForm.Get("refresh_csrf") != "refresh-csrf" ||
				r.PostForm.Get("source") != "main_web" || r.PostForm.Get("refresh_token") != "old-token" {
				t.Fatalf("form refresh = %v", r.PostForm)
			}
			http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "new-session", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "bili_jct", Value: "new-csrf", Path: "/"})
			writeJSON(w, `{"code":0,"data":{"refresh_token":"new-token"}}`)
		case p == "/confirm/refresh":
			_ = r.ParseForm()
			if r.PostForm.Get("csrf") != "new-csrf" || r.PostForm.Get("refresh_token") != "old-token" {
				t.Fatalf("form confirm = %v", r.PostForm)
			}
			if !strings.Contains(cookie(r), "SESSDATA=new-session") {
				t.Fatalf("confirm cookie = %q", cookie(r))
			}
			writeJSON(w, `{"code":0,"data":null}`)
		case p == "/x/web-interface/nav":
			writeJSON(w, `{"code":0,"data":{"isLogin":true,"uname":"Mới"}}`)
		default:
			return false
		}
		return true
	})
	seedQRState(t, e.svc, bilibiliLoginState{})
	outcome, view, err := e.svc.Refresh(context.Background())
	if err != nil || outcome != domain.RefreshRefreshed {
		t.Fatalf("refresh = %q %v", outcome, err)
	}
	if view.Account == nil || *view.Account != "Mới" || !view.Renewable {
		t.Fatalf("view = %+v", view)
	}
	if n := len(e.calls); n != 5 {
		t.Fatalf("số lời gọi = %d, muốn 5 (%v)", n, e.calls)
	}
	state, err := e.svc.readLoginState()
	if err != nil || state == nil {
		t.Fatal(err)
	}
	if *state.RefreshToken != "new-token" || *state.CSRF != "new-csrf" {
		t.Fatalf("state = %+v", state)
	}
	if state.Cookie != "SESSDATA=new-session; bili_jct=new-csrf; buvid3=device3; buvid4=device4" {
		t.Fatalf("cookie = %q", state.Cookie)
	}
	if got, _ := e.svc.CookieHeader(context.Background()); got != state.Cookie {
		t.Fatalf("CookieHeader = %q", got)
	}
	info, _ := os.Stat(filepath.Join(e.dir, "bilibili-login.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("quyền = %o", info.Mode().Perm())
	}
	// Snapshot thắng gương khi gương lệch (crash giữa hai lần ghi).
	if err := os.WriteFile(filepath.Join(e.dir, "bilibili-cookie.txt"), []byte("SESSDATA=uncommitted; bili_jct=wrong"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.svc.CookieHeader(context.Background()); got != state.Cookie {
		t.Fatalf("gương lệch phải bị bỏ qua, được %q", got)
	}
}

func TestRefreshSkipsWhenCurrentOrWithinADay(t *testing.T) {
	e := newQREnv(t)
	calls := 0
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		calls++
		writeJSON(w, `{"code":0,"data":{"refresh":false,"timestamp":123}}`)
		return p == "/cookie/info"
	})
	seedQRState(t, e.svc, bilibiliLoginState{})

	if outcome, _, err := e.svc.refresh(context.Background(), false); err != nil || outcome != domain.RefreshCurrent {
		t.Fatalf("lần đầu = %q %v", outcome, err)
	}
	if outcome, _, _ := e.svc.refresh(context.Background(), false); outcome != domain.RefreshSkipped {
		t.Fatalf("lần hai = %q", outcome)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, muốn 1", calls)
	}
	if outcome, _, _ := e.svc.refresh(context.Background(), true); outcome != domain.RefreshCurrent {
		t.Fatalf("force = %q", outcome)
	}
	e.advance(24 * time.Hour)
	if outcome, _, _ := e.svc.refresh(context.Background(), false); outcome != domain.RefreshCurrent {
		t.Fatalf("sau 24h = %q", outcome)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, muốn 3", calls)
	}
}

func TestRefreshFailureKeepsCommittedLogin(t *testing.T) {
	e := newQREnv(t)
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		if p == "/cookie/info" {
			writeJSON(w, `{"code":-101,"data":null}`)
			return true
		}
		w.WriteHeader(http.StatusInternalServerError)
		return true
	})
	seedQRState(t, e.svc, bilibiliLoginState{})

	outcome, view, err := e.svc.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if outcome != domain.RefreshFailed {
		t.Fatalf("outcome = %q", outcome)
	}
	if view.RefreshError == nil || !strings.Contains(*view.RefreshError, "quét mã QR lại") {
		t.Fatalf("refreshError = %v", view.RefreshError)
	}
	if got, _ := e.svc.CookieHeader(context.Background()); got != oldCookie {
		t.Fatalf("cookie cũ phải giữ, được %q", got)
	}
	state, _ := e.svc.readLoginState()
	if state == nil || *state.RefreshToken != "old-token" {
		t.Fatalf("token cũ phải giữ, được %+v", state)
	}
	before := len(e.calls)
	if got := e.svc.EnsureFresh(context.Background()); got != oldCookie {
		t.Fatalf("EnsureFresh = %q", got)
	}
	if len(e.calls) != before {
		t.Fatal("EnsureFresh trong 24 giờ không được gọi lại mạng")
	}
}

func TestManualLoginReplacesRenewalAndClearRemovesBoth(t *testing.T) {
	e := newQREnv(t)
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		if p == "/x/web-interface/nav" {
			writeJSON(w, `{"code":0,"data":{"isLogin":true,"uname":"Thủ công"}}`)
			return true
		}
		return false
	})
	seedQRState(t, e.svc, bilibiliLoginState{})

	view, err := e.svc.SaveManualCookie(context.Background(), "manual-session")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Saved || view.Method != "manual" || view.Renewable {
		t.Fatalf("view = %+v", view)
	}
	if view.Masked == nil || *view.Masked != "SESSDATA=••••sion" || view.Account == nil || *view.Account != "Thủ công" {
		t.Fatalf("view = %+v", view)
	}
	if outcome, _, err := e.svc.Refresh(context.Background()); err != nil || outcome != domain.RefreshSkipped {
		t.Fatalf("refresh manual = %q %v", outcome, err)
	}
	state, _ := e.svc.readLoginState()
	if state == nil || state.RefreshToken != nil {
		t.Fatalf("manual phải bỏ refresh token, được %+v", state)
	}

	if _, err := e.svc.ClearLogin(context.Background(), domain.PlatformBilibili); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"bilibili-cookie.txt", "bilibili-login.json"} {
		if _, err := os.Stat(filepath.Join(e.dir, f)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s còn sau khi xoá: %v", f, err)
		}
	}
	view, err = e.svc.bilibiliLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Saved || view.Method != "" || view.Masked != nil || view.Renewable {
		t.Fatalf("view sau xoá = %+v", view)
	}
}

func TestRenewalFailureKeepsSnapshot(t *testing.T) {
	for _, failure := range []string{"missing-new-csrf", "confirm", "nav"} {
		t.Run(failure, func(t *testing.T) {
			e := newQREnv(t)
			e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
				switch {
				case p == "/cookie/info":
					writeJSON(w, `{"code":0,"data":{"refresh":true,"timestamp":1800000000000}}`)
				case strings.HasPrefix(p, "/correspond/1/"):
					_, _ = io.WriteString(w, `<div id="1-name">refresh-csrf</div>`)
				case p == "/cookie/refresh":
					http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "new-session", Path: "/"})
					if failure != "missing-new-csrf" {
						http.SetCookie(w, &http.Cookie{Name: "bili_jct", Value: "new-csrf", Path: "/"})
					}
					writeJSON(w, `{"code":0,"data":{"refresh_token":"new-token"}}`)
				case p == "/confirm/refresh":
					if failure == "confirm" {
						writeJSON(w, `{"code":-400,"data":null}`)
						return true
					}
					writeJSON(w, `{"code":0,"data":null}`)
				case p == "/x/web-interface/nav":
					if failure == "nav" {
						writeJSON(w, `{"code":-101,"data":{"isLogin":false}}`)
						return true
					}
					writeJSON(w, `{"code":0,"data":{"isLogin":true,"uname":"Mới"}}`)
				default:
					return false
				}
				return true
			})
			seedQRState(t, e.svc, bilibiliLoginState{})
			outcome, _, err := e.svc.Refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if outcome != domain.RefreshFailed {
				t.Fatalf("outcome = %q", outcome)
			}
			if got, _ := e.svc.CookieHeader(context.Background()); got != oldCookie {
				t.Fatalf("cookie = %q", got)
			}
			state, _ := e.svc.readLoginState()
			if state == nil || *state.RefreshToken != "old-token" {
				t.Fatalf("token = %+v", state)
			}
			mirror, _ := os.ReadFile(filepath.Join(e.dir, "bilibili-cookie.txt"))
			if string(mirror) != oldCookie {
				t.Fatalf("gương = %q", mirror)
			}
		})
	}
}

func TestActiveLoginLockRefusesButScanStillGetsCookie(t *testing.T) {
	e := newQREnv(t)
	seedQRState(t, e.svc, bilibiliLoginState{})

	held := flock.New(e.svc.lockFile())
	ok, err := held.TryLock()
	if err != nil || !ok {
		t.Fatalf("giữ khoá: %v %v", ok, err)
	}
	defer func() { _ = held.Unlock() }()

	_, _, err = e.svc.Refresh(context.Background())
	var le *LoginError
	if !errors.As(err, &le) || le.Reason != ReasonBusy {
		t.Fatalf("refresh khi khoá bận phải busy, được %v", err)
	}
	if got := e.svc.EnsureFresh(context.Background()); got != oldCookie {
		t.Fatalf("EnsureFresh = %q", got)
	}
	if len(e.calls) != 0 {
		t.Fatalf("không được gọi mạng, được %v", e.calls)
	}
}
