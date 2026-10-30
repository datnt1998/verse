package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type anthropicCall struct {
	method string
	path   string
	header http.Header
	body   map[string]string
}

type fakeAnthropic struct {
	srv       *httptest.Server
	mu        sync.Mutex
	calls     []anthropicCall
	tokenResp map[string]any
	status    int
	bootstrap map[string]any
}

func newFakeAnthropic(t *testing.T) *fakeAnthropic {
	t.Helper()
	f := &fakeAnthropic{
		status: 200,
		tokenResp: map[string]any{
			"access_token": "acc-1", "refresh_token": "ref-1", "expires_in": 3600,
			"account":      map[string]any{"uuid": "acct-1", "email_address": "writer@example.com"},
			"organization": map[string]any{"uuid": "org-1", "name": "Truyện Việt"},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body := map[string]string{}
		_ = json.Unmarshal(data, &body)
		f.mu.Lock()
		f.calls = append(f.calls, anthropicCall{r.Method, r.URL.Path, r.Header.Clone(), body})
		status, resp := f.status, f.tokenResp
		f.mu.Unlock()
		writeJSON(w, status, resp)
	})
	mux.HandleFunc("/api/claude_cli/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, anthropicCall{r.Method, r.URL.Path + "?" + r.URL.RawQuery, r.Header.Clone(), nil})
		resp := f.bootstrap
		f.mu.Unlock()
		if resp == nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, resp)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAnthropic) provider() *anthropicOAuth {
	p := newAnthropicOAuth()
	p.tokenURL = f.srv.URL + "/v1/oauth/token"
	p.bootstrapURL = f.srv.URL + "/api/claude_cli/bootstrap"
	p.client = f.srv.Client()
	p.openBrowser = nil
	p.callbackAddr = "127.0.0.1:0"
	p.now = func() time.Time { return time.UnixMilli(2_000_000) }
	return p
}

func TestAnthropicAuthorizationURLHasPKCEStateAndCodeTrue(t *testing.T) {
	p := newAnthropicOAuth()
	u, err := url.Parse(p.authorizationURL("st-1", "chal-1"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != "https://claude.ai/oauth/authorize" {
		t.Fatalf("url = %s", u)
	}
	for k, want := range map[string]string{
		"code": "true", "client_id": "9d1c250a-e61b-44d9-88ed-5944d1962f5e", "response_type": "code",
		"redirect_uri": "http://localhost:54545/callback", "code_challenge": "chal-1",
		"code_challenge_method": "S256", "state": "st-1",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, muốn %q", k, q.Get(k), want)
		}
	}
	if !strings.Contains(q.Get("scope"), "user:inference") || !strings.Contains(q.Get("scope"), "user:profile") {
		t.Errorf("scope = %q", q.Get("scope"))
	}
}

func TestAnthropicExchangeSendsJSONWithStateAndVerifier(t *testing.T) {
	f := newFakeAnthropic(t)
	p := f.provider()
	cred, err := p.exchangeCode(context.Background(), "the-code", "the-state", "the-verifier")
	if err != nil {
		t.Fatal(err)
	}
	c := f.calls[0]
	if c.method != "POST" || !strings.HasPrefix(c.header.Get("Content-Type"), "application/json") {
		t.Fatalf("call = %+v", c)
	}
	for k, want := range map[string]string{
		"grant_type": "authorization_code", "client_id": anthropicClientID(), "code": "the-code",
		"state": "the-state", "code_verifier": "the-verifier", "redirect_uri": "http://localhost:54545/callback",
	} {
		if c.body[k] != want {
			t.Errorf("body[%s] = %q, muốn %q", k, c.body[k], want)
		}
	}
	if c.header.Get("anthropic-beta") != "" {
		t.Errorf("bước đổi mã không gửi anthropic-beta, có %q", c.header.Get("anthropic-beta"))
	}
	if cred.Access != "acc-1" || cred.Refresh != "ref-1" || cred.AccountID != "acct-1" || cred.Email != "writer@example.com" || cred.OrgID != "org-1" || cred.OrgName != "Truyện Việt" {
		t.Fatalf("cred = %+v", cred)
	}
	if want := int64(2_000_000 + 3600*1000); cred.Expires != want {
		t.Errorf("Expires = %d, muốn %d (now + expires_in)", cred.Expires, want)
	}
	if cred.Label() != "writer@example.com · Truyện Việt" {
		t.Errorf("Label = %q", cred.Label())
	}
}

func TestAnthropicRefreshHeadersAndKeepsOrg(t *testing.T) {
	f := newFakeAnthropic(t)
	f.tokenResp = map[string]any{"access_token": "acc-2", "refresh_token": "ref-2", "expires_in": 100}
	p := f.provider()
	prev := Credential{Access: "acc-1", Refresh: "ref-1", AccountID: "acct-1", Email: "writer@example.com", OrgID: "org-1", OrgName: "Truyện Việt"}
	next, err := p.Refresh(context.Background(), prev)
	if err != nil {
		t.Fatal(err)
	}
	c := f.calls[0]
	if c.body["grant_type"] != "refresh_token" || c.body["refresh_token"] != "ref-1" || c.body["client_id"] != anthropicClientID() {
		t.Errorf("body = %v", c.body)
	}
	if c.header.Get("anthropic-beta") != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q", c.header.Get("anthropic-beta"))
	}
	if ua := c.header.Get("User-Agent"); ua != "anthropic-sdk-typescript/"+ClaudeCodeSDKVersion+" userOAuthProvider" {
		t.Errorf("User-Agent = %q", ua)
	}
	if next.Access != "acc-2" || next.Refresh != "ref-2" || next.OrgID != "org-1" || next.OrgName != "Truyện Việt" || next.Email != "writer@example.com" || next.AccountID != "acct-1" {
		t.Errorf("next = %+v", next)
	}
}

func TestAnthropicIdentityFallsBackToBootstrap(t *testing.T) {
	f := newFakeAnthropic(t)
	f.tokenResp = map[string]any{"access_token": "acc-1", "refresh_token": "ref-1", "expires_in": 60}
	f.bootstrap = map[string]any{"oauth_account": map[string]any{
		"account_uuid": "acct-b", "account_email": "b@example.com", "organization_uuid": "org-b", "organization_name": "Org B",
	}}
	p := f.provider()
	cred, err := p.exchangeCode(context.Background(), "c", "s", "v")
	if err != nil {
		t.Fatal(err)
	}
	cred = p.fillIdentity(context.Background(), cred, true)
	if cred.AccountID != "acct-b" || cred.Email != "b@example.com" || cred.OrgID != "org-b" || cred.OrgName != "Org B" {
		t.Fatalf("cred = %+v", cred)
	}
	var boot anthropicCall
	for _, c := range f.calls {
		if strings.HasPrefix(c.path, "/api/claude_cli/bootstrap") {
			boot = c
		}
	}
	if boot.header.Get("Authorization") != "Bearer acc-1" || boot.header.Get("anthropic-beta") != "oauth-2025-04-20" || !strings.Contains(boot.path, "entrypoint=cli") {
		t.Errorf("bootstrap call = %+v", boot)
	}
}

func TestAnthropicRefreshDoesNotOverwriteOrgFromBootstrap(t *testing.T) {
	f := newFakeAnthropic(t)
	f.tokenResp = map[string]any{"access_token": "acc-2", "refresh_token": "ref-2", "expires_in": 60}
	f.bootstrap = map[string]any{"oauth_account": map[string]any{
		"account_uuid": "acct-b", "account_email": "b@example.com", "organization_uuid": "org-OTHER", "organization_name": "Other",
	}}
	next, err := f.provider().Refresh(context.Background(), Credential{Refresh: "ref-1"})
	if err != nil {
		t.Fatal(err)
	}
	if next.OrgID != "" || next.AccountID != "acct-b" {
		t.Fatalf("refresh không được ghi tổ chức từ bootstrap: %+v", next)
	}
}

func TestAnthropicTokenErrorsAreVietnameseAndDescriptive(t *testing.T) {
	f := newFakeAnthropic(t)
	f.status = 400
	f.tokenResp = map[string]any{"error": map[string]any{"type": "invalid_grant", "message": "Authorization code expired"}}
	_, err := f.provider().exchangeCode(context.Background(), "c", "s", "v")
	if err == nil || !strings.Contains(err.Error(), "đổi mã lấy token thất bại") || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseAuthorizationInput(t *testing.T) {
	for _, tc := range []struct{ in, code, state string }{
		{"abc#xyz", "abc", "xyz"},
		{"  abc#xyz\n", "abc", "xyz"},
		{"abc", "abc", ""},
		{"abc#xyz#extra", "abc", "xyz"},
		{"http://localhost:54545/callback?code=abc&state=xyz", "abc", "xyz"},
		{"https://platform.claude.com/oauth/code/callback?code=abc&state=xyz#frag", "abc", "xyz"},
		{"code=abc&state=xyz", "abc", "xyz"},
		{"?code=abc&state=xyz", "abc", "xyz"},
		{"", "", ""},
	} {
		code, state := ParseAuthorizationInput(tc.in)
		if code != tc.code || state != tc.state {
			t.Errorf("Parse(%q) = (%q,%q), muốn (%q,%q)", tc.in, code, state, tc.code, tc.state)
		}
	}
}

func submit(ch chan<- ManualSubmission, input string) error {
	res := make(chan error, 1)
	select {
	case ch <- ManualSubmission{Input: input, Result: res}:
	case <-time.After(2 * time.Second):
		return errors.New("không có ai nhận mã dán (luồng đăng nhập đã kết thúc)")
	}
	return <-res
}

// offerURL chuyển URL từ goroutine Login sang test qua kênh (không chia sẻ biến).
func offerURL(ch chan<- string, u string) {
	select {
	case ch <- u:
	default:
	}
}

// awaitURL đợi URL đầu tiên tối đa 2 giây; hết hạn trả chuỗi rỗng để test báo lỗi.
func awaitURL(ch <-chan string) string {
	select {
	case u := <-ch:
		return u
	case <-time.After(2 * time.Second):
		return ""
	}
}

func TestAnthropicLoginPasteCodeRejectsWrongStateThenAccepts(t *testing.T) {
	f := newFakeAnthropic(t)
	p := f.provider()
	manual := make(chan ManualSubmission)
	authURLs := make(chan string, 1)
	done := make(chan struct{})
	var cred Credential
	var loginErr error
	go func() {
		defer close(done)
		cred, loginErr = p.Login(context.Background(), Controller{OnAuth: func(u, _ string) { offerURL(authURLs, u) }}, LoginOptions{Device: true, From: "web", Manual: manual})
	}()
	// Chờ OnAuth chạy (Login bắt đầu nhận manual sau OnAuth).
	authURL := awaitURL(authURLs)
	u, _ := url.Parse(authURL)
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("không có state trong URL")
	}
	if err := submit(manual, "   "); err == nil {
		t.Error("nội dung trống phải bị từ chối")
	}
	if err := submit(manual, "thecode#other-state"); err == nil || !strings.Contains(err.Error(), "state không khớp") {
		t.Errorf("state sai: err = %v", err)
	}
	if err := submit(manual, "thecode#"+state); err != nil {
		t.Fatalf("mã hợp lệ bị từ chối: %v", err)
	}
	<-done
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	exch := f.calls[0].body
	if exch["code"] != "thecode" || exch["state"] != state || exch["code_verifier"] == "" {
		t.Errorf("exchange body = %v", exch)
	}
	if len(exch["code_verifier"]) < 43 || len(exch["code_verifier"]) > 128 {
		t.Errorf("code_verifier dài %d, RFC 7636 cho 43–128", len(exch["code_verifier"]))
	}
	if cred.Access != "acc-1" {
		t.Errorf("cred = %+v", cred)
	}
}

func TestAnthropicLoginBrowserCallbackAndFullURLPaste(t *testing.T) {
	f := newFakeAnthropic(t)
	p := f.provider()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	p.callbackAddr = addr

	authURLs := make(chan string, 1)
	done := make(chan struct{})
	var loginErr error
	go func() {
		defer close(done)
		_, loginErr = p.Login(context.Background(), Controller{OnAuth: func(u, _ string) { offerURL(authURLs, u) }}, LoginOptions{From: "cli"})
	}()
	authURL := awaitURL(authURLs)
	u, _ := url.Parse(authURL)
	state := u.Query().Get("state")
	resp, err := http.Get("http://" + addr + anthropicCallbackPath + "?code=browser-code&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(page), "Claude") || strings.Contains(string(page), "ChatGPT") {
		t.Errorf("trang callback status=%d, phải mang thương hiệu Claude", resp.StatusCode)
	}
	<-done
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	if f.calls[0].body["code"] != "browser-code" || f.calls[0].body["state"] != state {
		t.Errorf("exchange = %v", f.calls[0].body)
	}
}

func TestAnthropicDeviceWithoutManualFails(t *testing.T) {
	_, err := newAnthropicOAuth().Login(context.Background(), Controller{}, LoginOptions{Device: true})
	if err == nil {
		t.Fatal("Device không có Manual phải lỗi")
	}
}

func TestAnthropicPasteCodeCapability(t *testing.T) {
	if !PasteCode(AnthropicOAuthID) || PasteCode("openai-codex") || PasteCode("nope") {
		t.Fatal("chỉ anthropic-oauth hỗ trợ dán mã")
	}
}

func TestCredentialLabelOmitsOrgContainingEmail(t *testing.T) {
	c := Credential{Email: "a@b.com", OrgName: "A@B.com's Organization"}
	if c.Label() != "a@b.com" {
		t.Errorf("Label = %q", c.Label())
	}
}

func TestClaudeCodeVersionAdoption(t *testing.T) {
	useTestApp(t)
	t.Setenv(testApp.ClaudeCodeVersionEnv, "")
	adoptedVersion.Store(nil)
	defer adoptedVersion.Store(nil)
	if AdoptRequiredClaudeCodeVersion("claude_code_version_too_old: version 1.0.0 or newer is required") {
		t.Error("version thấp hơn không được nhận")
	}
	if !AdoptRequiredClaudeCodeVersion("claude_code_version_too_old: version 99.1.2 or newer is required") || ClaudeCodeVersion() != "99.1.2" {
		t.Errorf("version = %s", ClaudeCodeVersion())
	}
	if AdoptRequiredClaudeCodeVersion("claude_code_version_too_old: version 99.1.2 or newer is required") {
		t.Error("cùng version không được thử lại lần hai")
	}
	t.Setenv(testApp.ClaudeCodeVersionEnv, "5.0.0")
	if ClaudeCodeVersion() != "5.0.0" || AdoptRequiredClaudeCodeVersion("claude_code_version_too_old: version 99.9.9 or newer is required") {
		t.Error("biến môi trường phải ghim version")
	}
}
