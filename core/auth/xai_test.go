package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeXAI là auth.x.ai giả: discovery, device code, token endpoint (device + refresh), userinfo.
type fakeXAI struct {
	srv *httptest.Server

	mu          sync.Mutex
	tokenCalls  []url.Values
	pollScript  []fakePoll // trả lần lượt cho mỗi lần poll device_code
	refreshResp func(url.Values) (int, any)
	userinfo    any
	userinfoHit atomic.Int32
	deviceForms []url.Values
	discoveryTE string // token_endpoint trả về (rỗng = <srv>/oauth2/token)
}

type fakePoll struct {
	status int
	body   any
}

func newFakeXAI(t *testing.T) *fakeXAI {
	t.Helper()
	f := &fakeXAI{userinfo: map[string]any{"sub": "sub-1", "email": "Writer@X.AI", "name": "Writer"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		te := f.discoveryTE
		if te == "" {
			te = f.srv.URL + "/oauth2/token"
		}
		writeJSON(w, 200, map[string]any{"token_endpoint": te, "authorization_endpoint": f.srv.URL + "/authorize"})
	})
	mux.HandleFunc("/oauth2/device/code", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.deviceForms = append(f.deviceForms, r.PostForm)
		f.mu.Unlock()
		writeJSON(w, 200, map[string]any{
			"device_code": "dev-123", "user_code": "ABCD-EFGH",
			"verification_uri":          "https://accounts.x.ai/device",
			"verification_uri_complete": "https://accounts.x.ai/device?user_code=ABCD-EFGH",
			"expires_in":                600, "interval": 5,
		})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.tokenCalls = append(f.tokenCalls, r.PostForm)
		f.mu.Unlock()
		if r.PostForm.Get("grant_type") == "refresh_token" {
			status, body := f.refreshResp(r.PostForm)
			writeJSON(w, status, body)
			return
		}
		f.mu.Lock()
		step := f.pollScript[0]
		if len(f.pollScript) > 1 {
			f.pollScript = f.pollScript[1:]
		}
		f.mu.Unlock()
		writeJSON(w, step.status, step.body)
	})
	mux.HandleFunc("/oauth2/userinfo", func(w http.ResponseWriter, r *http.Request) {
		f.userinfoHit.Add(1)
		if r.Header.Get("Authorization") == "" {
			writeJSON(w, 401, map[string]any{"error": "no token"})
			return
		}
		writeJSON(w, 200, f.userinfo)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeXAI) provider() (*xaiOAuth, *[]time.Duration) {
	p := newXAIOAuth()
	p.issuer = f.srv.URL
	p.client = f.srv.Client()
	p.validateEndpoint = func(string) error { return nil }
	p.openBrowser = nil
	p.now = func() time.Time { return time.UnixMilli(1_000_000) }
	var waits []time.Duration
	p.wait = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	return p, &waits
}

func xaiJWT(exp int64, extra map[string]any) string {
	claims := map[string]any{"exp": exp}
	for k, v := range extra {
		claims[k] = v
	}
	return fakeJWT(claims)
}

func TestXAIDeviceLoginPollsUntilApproved(t *testing.T) {
	f := newFakeXAI(t)
	access := xaiJWT(5_000, map[string]any{"sub": "sub-from-token"})
	f.pollScript = []fakePoll{
		{400, map[string]any{"error": "authorization_pending"}},
		{400, map[string]any{"error": "slow_down"}},
		{400, map[string]any{"error": "authorization_pending"}},
		{200, map[string]any{"access_token": access, "refresh_token": "r1", "expires_in": 3600}},
	}
	p, waits := f.provider()

	var gotCode, gotURL, gotAuthURL string
	var progress []string
	cred, err := p.Login(context.Background(), Controller{
		OnAuth:       func(u, _ string) { gotAuthURL = u },
		OnDeviceCode: func(code, u string) { gotCode, gotURL = code, u },
		OnProgress:   func(m string) { progress = append(progress, m) },
	}, LoginOptions{})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if gotCode != "ABCD-EFGH" || gotURL != "https://accounts.x.ai/device?user_code=ABCD-EFGH" || gotAuthURL != gotURL {
		t.Fatalf("UI nhận code=%q url=%q auth=%q", gotCode, gotURL, gotAuthURL)
	}
	// 4 lần poll: chờ 5s mỗi lần, sau slow_down phải tăng thêm 5s (RFC 8628 §3.5).
	want := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 10 * time.Second}
	if len(*waits) != len(want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
	for i := range want {
		if (*waits)[i] != want[i] {
			t.Fatalf("waits = %v, want %v", *waits, want)
		}
	}
	if cred.Access != access || cred.Refresh != "r1" {
		t.Fatalf("cred tokens = %+v", cred)
	}
	if cred.Expires != 5_000*1000 {
		t.Fatalf("Expires = %d, phải lấy từ claim exp của JWT", cred.Expires)
	}
	// userinfo ghi đè sub/email để dựng nhãn.
	if cred.Email != "writer@x.ai" || cred.AccountID != "sub-1" || cred.Label() != "writer@x.ai" {
		t.Fatalf("profile = %+v label=%q", cred, cred.Label())
	}
	form := f.deviceForms[0]
	if form.Get("client_id") != xaiClientID || form.Get("scope") != xaiScope {
		t.Fatalf("device form = %v", form)
	}
	poll := f.tokenCalls[0]
	if poll.Get("grant_type") != xaiDeviceGrant || poll.Get("device_code") != "dev-123" || poll.Get("client_id") != xaiClientID {
		t.Fatalf("poll form = %v", poll)
	}
	if len(progress) == 0 {
		t.Fatal("thiếu progress")
	}
}

func TestXAIDeviceLoginFailureModes(t *testing.T) {
	cases := []struct {
		name string
		step fakePoll
		want string
	}{
		{"denied", fakePoll{400, map[string]any{"error": "access_denied"}}, "từ chối"},
		{"expired", fakePoll{400, map[string]any{"error": "expired_token"}}, "đã hết hạn"},
		{"tier", fakePoll{403, map[string]any{"error": "forbidden"}}, "chưa được xAI cấp quyền"},
		{"missing refresh", fakePoll{200, map[string]any{"access_token": "a", "expires_in": 60}}, "refresh_token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeXAI(t)
			f.pollScript = []fakePoll{tc.step}
			p, _ := f.provider()
			_, err := p.Login(context.Background(), Controller{}, LoginOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want chứa %q", err, tc.want)
			}
			if tc.name == "tier" && !errors.Is(err, ErrXAITierDenied) {
				t.Fatalf("403 phải là ErrXAITierDenied: %v", err)
			}
		})
	}
}

func TestXAIDeviceLoginIgnoresBrowserOptionAndOpensBrowserOnlyForCLI(t *testing.T) {
	access := xaiJWT(9_000, nil)
	for _, tc := range []struct {
		from      string
		wantOpens int
	}{{"", 1}, {"web", 0}} {
		f := newFakeXAI(t)
		f.pollScript = []fakePoll{{200, map[string]any{"access_token": access, "refresh_token": "r"}}}
		p, _ := f.provider()
		var opened []string
		p.openBrowser = func(u string) error { opened = append(opened, u); return nil }
		if _, err := p.Login(context.Background(), Controller{}, LoginOptions{Device: false, From: tc.from}); err != nil {
			t.Fatalf("from=%q: %v", tc.from, err)
		}
		if len(opened) != tc.wantOpens {
			t.Fatalf("from=%q mở trình duyệt %v lần, want %d", tc.from, opened, tc.wantOpens)
		}
	}
	if !DeviceOnly(XAIOAuthID) || DeviceOnly(OpenAICodexID) {
		t.Fatal("DeviceOnly: xai-oauth phải chỉ có device-code, codex thì không")
	}
}

func TestXAIRefreshRotatesTokensAndKeepsProfile(t *testing.T) {
	f := newFakeXAI(t)
	access2 := xaiJWT(8_000, nil)
	f.refreshResp = func(form url.Values) (int, any) {
		return 200, map[string]any{"access_token": access2, "refresh_token": "r2", "expires_in": 3600}
	}
	p, _ := f.provider()
	prev := Credential{Access: "old", Refresh: "r1", Expires: 1, AccountID: "sub-1", Email: "writer@x.ai"}
	next, err := p.Refresh(context.Background(), prev)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if next.Access != access2 || next.Refresh != "r2" || next.Expires != 8_000_000 {
		t.Fatalf("next = %+v", next)
	}
	if next.Email != "writer@x.ai" || next.AccountID != "sub-1" {
		t.Fatalf("profile phải giữ nguyên khi refresh: %+v", next)
	}
	form := f.tokenCalls[0]
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "r1" || form.Get("client_id") != xaiClientID {
		t.Fatalf("refresh form = %v", form)
	}

	// Server không xoay refresh token và token không phải JWT: giữ refresh cũ, dùng expires_in.
	f.refreshResp = func(url.Values) (int, any) {
		return 200, map[string]any{"access_token": "opaque", "expires_in": 120}
	}
	next, err = p.Refresh(context.Background(), prev)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if next.Refresh != "r1" || next.Expires != 1_000_000+120_000 {
		t.Fatalf("next = %+v", next)
	}
}

func TestXAIRefreshForbiddenIsTierDeniedAndManagerDoesNotAdviseRelogin(t *testing.T) {
	f := newFakeXAI(t)
	f.refreshResp = func(url.Values) (int, any) { return 403, map[string]any{"error": "forbidden"} }
	p, _ := f.provider()
	store := NewStore(t.TempDir() + "/auth.json")
	if err := store.Put(XAIOAuthID, Credential{Access: "stale", Refresh: "r1", Expires: 1}); err != nil {
		t.Fatal(err)
	}
	m := NewManager(store)
	m.lookup = func(id string) (Provider, bool) { return p, id == XAIOAuthID }

	_, err := m.Credential(context.Background(), XAIOAuthID)
	if !errors.Is(err, ErrXAITierDenied) {
		t.Fatalf("err = %v, want ErrXAITierDenied", err)
	}
	if strings.Contains(err.Error(), "login") || !strings.Contains(err.Error(), "API key xAI") {
		t.Fatalf("403 là giới hạn gói, không được khuyên đăng nhập lại: %q", err)
	}
	if n := len(f.tokenCalls); n != 1 {
		t.Fatalf("token endpoint bị gọi %d lần, want 1 (không lặp refresh)", n)
	}
}

func TestXAIRefreshInvalidGrantReportsOAuthError(t *testing.T) {
	f := newFakeXAI(t)
	f.refreshResp = func(url.Values) (int, any) {
		return 400, map[string]any{"error": "invalid_grant", "error_description": "revoked"}
	}
	p, _ := f.provider()
	_, err := p.Refresh(context.Background(), Credential{Refresh: "r1"})
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") || errors.Is(err, ErrXAITierDenied) {
		t.Fatalf("err = %v", err)
	}
}

func TestXAIManagerRefreshesBeforeJWTExpiryWithinSkew(t *testing.T) {
	f := newFakeXAI(t)
	now := time.Unix(100_000, 0)
	access2 := xaiJWT(now.Add(time.Hour).Unix(), nil)
	f.refreshResp = func(url.Values) (int, any) {
		return 200, map[string]any{"access_token": access2, "refresh_token": "r2"}
	}
	p, _ := f.provider()
	p.now = func() time.Time { return now }
	store := NewStore(t.TempDir() + "/auth.json")
	// Token còn 2 phút (< skew 5 phút của Manager) → phải refresh trước khi hết hạn.
	soon := xaiJWT(now.Add(2*time.Minute).Unix(), nil)
	if err := store.Put(XAIOAuthID, Credential{Access: soon, Refresh: "r1", Expires: now.Add(2 * time.Minute).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	m := NewManager(store)
	m.now = func() time.Time { return now }
	m.lookup = func(id string) (Provider, bool) { return p, true }
	cred, err := m.Credential(context.Background(), XAIOAuthID)
	if err != nil || cred.Access != access2 || cred.Refresh != "r2" {
		t.Fatalf("cred=%+v err=%v", cred, err)
	}
	// Token còn hạn xa: không refresh nữa.
	if _, err := m.Credential(context.Background(), XAIOAuthID); err != nil || len(f.tokenCalls) != 1 {
		t.Fatalf("không được refresh lần hai: calls=%d err=%v", len(f.tokenCalls), err)
	}
}

func TestXAITokenEndpointMustBeHTTPSOnXAIDomain(t *testing.T) {
	for _, bad := range []string{
		"http://auth.x.ai/oauth2/token",
		"https://evil.example/oauth2/token",
		"https://x.ai.evil.example/token",
		"https://notx.ai/token",
		"/relative",
	} {
		if err := validateXAIEndpoint(bad); err == nil {
			t.Errorf("validateXAIEndpoint(%q) = nil, want lỗi", bad)
		}
	}
	for _, good := range []string{"https://auth.x.ai/oauth2/token", "https://x.ai/token", "https://AUTH.X.AI./t"} {
		if err := validateXAIEndpoint(good); err != nil {
			t.Errorf("validateXAIEndpoint(%q) = %v", good, err)
		}
	}

	// Discovery bị đánh tráo: token_endpoint trỏ ra ngoài → từ chối, không gửi refresh token đi đâu.
	f := newFakeXAI(t)
	f.discoveryTE = "https://evil.example/oauth2/token"
	p, _ := f.provider()
	p.validateEndpoint = validateXAIEndpoint
	if _, err := p.Refresh(context.Background(), Credential{Refresh: "r1"}); err == nil || !strings.Contains(err.Error(), "không thuộc miền x.ai") {
		t.Fatalf("err = %v", err)
	}
	if len(f.tokenCalls) != 0 {
		t.Fatal("không được gọi token endpoint")
	}
}

func TestXAIProviderRegistered(t *testing.T) {
	p, ok := Lookup(XAIOAuthID)
	if !ok || p.Name() != "SuperGrok / X Premium+" {
		t.Fatalf("Lookup = %v %v", p, ok)
	}
}
