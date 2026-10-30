// Ca test HTTP cho /api/platforms*: hình wire theo §24/§24.1, phân giải
// route, và map lỗi nghiệp vụ sang status + reason.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// fakePlatforms là PlatformService giả ghi lại lời gọi.
type fakePlatforms struct {
	err      error
	lastID   domain.PlatformID
	lastIn   platforms.LoginInput
	lastKey  string
	lastRefr bool
}

func (f *fakePlatforms) Status(context.Context) ([]domain.PlatformStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	refreshErr := "Đăng nhập Bilibili đã hết hạn — quét mã QR lại"
	rivals := 2
	at := time.Date(2026, 10, 29, 2, 0, 0, 0, time.UTC)
	return []domain.PlatformStatus{
		{
			ID: domain.PlatformYouTube, Label: "YouTube",
			Uses: []domain.PlatformUse{domain.PlatformUseCompetitor}, Kinds: []domain.PlatformKind{domain.PlatformKindVideo},
			TargetHeight: 360,
			Login:        domain.PlatformLogin{Type: domain.LoginBrowserCookie, Saved: true, Browser: "chrome", Profile: "Default"},
			State:        domain.PlatformNeedsLogin,
			LastError:    &domain.PlatformLastError{Code: "cookies_failed", At: at},
			Usage:        domain.PlatformUsage{Channels: 1, Videos: 4, Rivals: &rivals},
		},
		{
			ID: domain.PlatformBilibili, Label: "Bilibili",
			Uses: []domain.PlatformUse{domain.PlatformUseBackdrop}, Kinds: []domain.PlatformKind{domain.PlatformKindVideo},
			TargetHeight: 1080,
			Login:        domain.PlatformLogin{Type: domain.LoginQR, Saved: true, Method: "qr", Renewable: true, Masked: new("SESSDATA=••••abcd"), RefreshError: &refreshErr},
			State:        domain.PlatformPaused,
			Usage:        domain.PlatformUsage{},
		},
	}, nil
}

func (f *fakePlatforms) Profiles(context.Context, domain.PlatformID, bool) ([]domain.BrowserProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.BrowserProfile{{Browser: "chrome", Profile: "Default", Name: "Cá nhân", Email: "a@b.c"}}, nil
}

func (f *fakePlatforms) SetLogin(_ context.Context, id domain.PlatformID, in platforms.LoginInput) (domain.PlatformLogin, error) {
	f.lastID, f.lastIn = id, in
	if f.err != nil {
		return domain.PlatformLogin{}, f.err
	}
	return domain.PlatformLogin{Type: domain.LoginBrowserCookie, Saved: true, Browser: in.Browser, Profile: in.Profile, Method: "manual"}, nil
}

func (f *fakePlatforms) ClearLogin(_ context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	f.lastID = id
	return domain.PlatformLogin{}, nil
}

func (f *fakePlatforms) Check(_ context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	f.lastID = id
	return domain.PlatformLogin{}, f.err
}

func (f *fakePlatforms) StartQR(context.Context, domain.PlatformID) (domain.QRSession, error) {
	if f.err != nil {
		return domain.QRSession{}, f.err
	}
	return domain.QRSession{QRCodeKey: "key", QRImage: "data:image/svg+xml;base64,abc", LoginURL: "https://passport.bilibili.com/x", ExpiresAt: time.Date(2026, 10, 29, 8, 3, 0, 0, time.UTC)}, nil
}

func (f *fakePlatforms) PollQR(_ context.Context, _ domain.PlatformID, key string) (domain.QRPoll, error) {
	f.lastKey = key
	return domain.QRPoll{Status: domain.QRWaiting}, f.err
}

func (f *fakePlatforms) Refresh(context.Context, domain.PlatformID) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	return domain.RefreshRefreshed, domain.PlatformLogin{Type: domain.LoginQR, Saved: true, Method: "qr"}, f.err
}

// loginErr là lỗi đăng nhập mang reason như accounts.LoginError.
type loginErr struct{ reason string }

func (e loginErr) Error() string       { return "lỗi " + e.reason }
func (e loginErr) LoginReason() string { return e.reason }

func TestPlatformsWireShape(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakePlatforms{}
	MountPlatforms(tg.g, svc)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/platforms", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d", len(list))
	}
	yt := list[0]
	if yt["id"] != "youtube" || yt["targetHeight"].(float64) != 360 || yt["state"] != "needs_login" {
		t.Fatalf("youtube = %v", yt)
	}
	login := yt["login"].(map[string]any)
	if login["type"] != "browser_cookie" || login["saved"] != true || login["browserProfile"] != "chrome:Default" {
		t.Fatalf("login = %v", login)
	}
	if v, ok := login["method"]; !ok || v != nil {
		t.Fatalf("method phải luôn có và là null, được %v", v)
	}
	errObj := yt["lastError"].(map[string]any)
	if errObj["code"] != "cookies_failed" || errObj["at"] != "2026-10-29T02:00:00Z" {
		t.Fatalf("lastError = %v", errObj)
	}
	usage := yt["usage"].(map[string]any)
	if usage["channels"].(float64) != 1 || usage["videos"].(float64) != 4 || usage["rivals"].(float64) != 2 {
		t.Fatalf("usage = %v", usage)
	}
	bili := list[1]
	bl := bili["login"].(map[string]any)
	if bl["method"] != "qr" || bl["renewable"] != true || bl["masked"] != "SESSDATA=••••abcd" || bl["refreshError"] == nil {
		t.Fatalf("bilibili login = %v", bl)
	}
	if _, ok := bili["lastError"]; ok {
		t.Fatalf("bilibili không được có lastError: %v", bili)
	}
}

func TestPlatformLoginRoutes(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakePlatforms{}
	MountPlatforms(tg.g, svc)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodPut, "/api/platforms/youtube/login", cookie, `{"browser":"chrome","profile":"Default"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put status %d: %s", w.Code, w.Body.String())
	}
	if svc.lastID != "youtube" || svc.lastIn.Browser != "chrome" || svc.lastIn.Profile != "Default" {
		t.Fatalf("last = %v %+v", svc.lastID, svc.lastIn)
	}

	w = tg.do(http.MethodGet, "/api/platforms/youtube/login/profiles?refresh=1", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("profiles status %d: %s", w.Code, w.Body.String())
	}
	var profiles []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0]["browser"] != "chrome" || profiles[0]["email"] != "a@b.c" {
		t.Fatalf("profiles = %v", profiles)
	}

	w = tg.do(http.MethodPost, "/api/platforms/bilibili/login/qr", cookie, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("qr status %d: %s", w.Code, w.Body.String())
	}
	var session map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &session)
	if session["qrcodeKey"] != "key" || session["loginUrl"] != "https://passport.bilibili.com/x" || session["expiresAt"] != "2026-10-29T08:03:00Z" {
		t.Fatalf("session = %v", session)
	}

	w = tg.do(http.MethodGet, "/api/platforms/bilibili/login/qr/key", cookie, "")
	if w.Code != http.StatusOK || svc.lastKey != "key" {
		t.Fatalf("poll = %d %q", w.Code, svc.lastKey)
	}

	w = tg.do(http.MethodPost, "/api/platforms/bilibili/login/refresh", cookie, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("refresh status %d: %s", w.Code, w.Body.String())
	}
	var refreshed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &refreshed)
	if refreshed["status"] != "refreshed" || refreshed["method"] != "qr" {
		t.Fatalf("refresh = %v", refreshed)
	}

	w = tg.do(http.MethodPost, "/api/platforms/youtube/login/check", cookie, "{}")
	if w.Code != http.StatusOK || svc.lastID != "youtube" {
		t.Fatalf("check = %d %v", w.Code, svc.lastID)
	}

	if w := tg.do(http.MethodDelete, "/api/platforms/bilibili/login", cookie, ""); w.Code != http.StatusOK || svc.lastID != "bilibili" {
		t.Fatalf("delete = %d %v", w.Code, svc.lastID)
	}
	if w := tg.do(http.MethodDelete, "/api/platforms/youtube", cookie, ""); w.Code != http.StatusNotFound {
		t.Fatalf("route lạ = %d", w.Code)
	}
}

func TestPlatformErrorsMapToStatusAndReason(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakePlatforms{}
	MountPlatforms(tg.g, svc)
	cookie := tg.sessionCookie()

	cases := []struct {
		err    error
		status int
		reason string
	}{
		{platforms.ErrPlatformUnknown, http.StatusNotFound, ""},
		{platforms.ErrInvalidLoginBody, http.StatusBadRequest, "invalid"},
		{platforms.ErrLoginUnsupported, http.StatusBadRequest, "unsupported"},
		{loginErr{"rejected"}, http.StatusUnprocessableEntity, "rejected"},
		{loginErr{"invalid"}, http.StatusBadRequest, "invalid"},
		{loginErr{"unavailable"}, http.StatusBadGateway, "unavailable"},
		{loginErr{"busy"}, http.StatusConflict, "busy"},
	}
	for _, c := range cases {
		svc.err = c.err
		w := tg.do(http.MethodPut, "/api/platforms/youtube/login", cookie, `{"browser":"chrome","profile":"Default"}`)
		if w.Code != c.status {
			t.Fatalf("%v → %d, muốn %d (%s)", c.err, w.Code, c.status, w.Body.String())
		}
		if c.reason != "" {
			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if body["reason"] != c.reason {
				t.Fatalf("%v reason = %v, muốn %s", c.err, body["reason"], c.reason)
			}
		}
	}
}

// rawInternalErr là lỗi chứa đường dẫn home + DSN + token để kiểm vỏ 500.
type rawInternalErr struct{}

func (rawInternalErr) Error() string {
	return "open /Users/dathome/.viderse/bilibili-login.json: permission denied postgres://u:p@db/viderse token=SECRETTOKEN"
}

func TestInternalErrorsReturnNeutralEnvelope(t *testing.T) {
	tg := newTestGateway(t)
	svc := &fakePlatforms{}
	var diagged string
	MountPlatformsDiag(tg.g, svc, func(msg string) { diagged = msg })
	cookie := tg.sessionCookie()

	svc.err = rawInternalErr{}
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/platforms", ""},
		{http.MethodPut, "/api/platforms/youtube/login", `{"browser":"chrome","profile":"Default"}`},
		{http.MethodPost, "/api/platforms/bilibili/login/refresh", `{}`},
	} {
		w := tg.do(tc.method, tc.path, cookie, tc.body)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("%s %s = %d", tc.method, tc.path, w.Code)
		}
		body := w.Body.String()
		for _, secret := range []string{"/Users/dathome", "postgres://", "SECRETTOKEN", "bilibili-login.json"} {
			if strings.Contains(body, secret) {
				t.Fatalf("API lộ %q trong %q", secret, body)
			}
		}
		if diagged == "" || !strings.Contains(diagged, "SECRETTOKEN") {
			t.Fatalf("diag phải nhận nguyên nhân thô, được %q", diagged)
		}
	}
}

func TestRedactedDiagHidesSecrets(t *testing.T) {
	// Redactor chung của logs phải che bí mật trong nguyên nhân lỗi trước khi
	// chạm đĩa — path home cũng thu gọn về ~.
	dir := t.TempDir()
	store := logs.New(dir, "/Users/dathome")
	t.Cleanup(func() { _ = store.Close() })
	redacted := store.Redact(rawInternalErr{}.Error())
	for _, secret := range []string{"/Users/dathome", "SECRETTOKEN"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redact còn %q trong %q", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "~") {
		t.Fatalf("home phải thu về ~, được %q", redacted)
	}
}
