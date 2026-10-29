package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fakeJWT(claims map[string]any) string {
	enc := func(v any) string {
		data, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	return enc(map[string]string{"alg": "none"}) + "." + enc(claims) + ".sig"
}

func codexAccessToken(account string) string {
	return fakeJWT(map[string]any{
		codexJWTAuthClaim:    map[string]any{"chatgpt_account_id": account},
		codexJWTProfileClaim: map[string]any{"email": "Writer@Example.com"},
	})
}

func codexIDToken(plan string) string {
	return fakeJWT(map[string]any{codexJWTAuthClaim: map[string]any{"chatgpt_plan_type": plan}})
}

func newTestCodex(t *testing.T, tokenHandler http.HandlerFunc) *openAICodex {
	t.Helper()
	srv := httptest.NewServer(tokenHandler)
	t.Cleanup(srv.Close)
	p := newOpenAICodex()
	p.tokenURL = srv.URL + "/oauth/token"
	p.deviceUserCodeURL = srv.URL + "/deviceauth/usercode"
	p.deviceTokenURL = srv.URL + "/deviceauth/token"
	p.client = srv.Client()
	p.openBrowser = nil
	p.now = func() time.Time { return time.UnixMilli(1_000_000) }
	p.devicePollFloor = time.Millisecond
	return p
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestCodexBrowserLoginExchangesPKCECode(t *testing.T) {
	var gotForm url.Values
	p := newTestCodex(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		writeJSON(w, 200, map[string]any{
			"access_token": codexAccessToken("acct-1"), "refresh_token": "r1",
			"id_token": codexIDToken("Pro"), "expires_in": 3600,
		})
	})
	addr := freeLoopbackAddr(t)
	p.callbackAddr = addr
	p.redirectURI = "http://" + addr + codexCallbackPath

	var challenge string
	p.openBrowser = func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		challenge = q.Get("code_challenge")
		// Trình duyệt giả: máy chủ OAuth redirect về callback với code + state.
		go http.Get(q.Get("redirect_uri") + "?code=auth-code&state=" + url.QueryEscape(q.Get("state")))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cred, err := p.Login(ctx, Controller{}, LoginOptions{})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if gotForm.Get("grant_type") != "authorization_code" || gotForm.Get("code") != "auth-code" ||
		gotForm.Get("redirect_uri") != p.redirectURI || gotForm.Get("client_id") != codexClientID {
		t.Fatalf("form đổi mã sai: %v", gotForm)
	}
	if verifier := gotForm.Get("code_verifier"); verifier == "" || pkceChallenge(verifier) != challenge {
		t.Fatalf("code_verifier không khớp code_challenge đã gửi")
	}
	want := Credential{Access: cred.Access, Refresh: "r1", Expires: 1_000_000 + 3600_000,
		AccountID: "acct-1", Email: "writer@example.com", Plan: "pro"}
	if cred != want {
		t.Fatalf("credential = %+v, want %+v", cred, want)
	}
}

func pkceChallenge(verifier string) string {
	// Tính lại S256 độc lập với pkce() để không kiểm tra hàm bằng chính nó.
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestCodexCallbackRejectsStateMismatch(t *testing.T) {
	addr := freeLoopbackAddr(t)
	cs, err := startCallbackServer(addr, codexCallbackPath, "expected", "cli", chatGPTBrand)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.close()
	resp, err := http.Get("http://" + addr + codexCallbackPath + "?code=x&state=forged")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := cs.wait(ctx); err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("wait err = %v, want state mismatch", err)
	}
}

func TestCodexDeviceLoginPollsUntilAuthorized(t *testing.T) {
	var polls atomic.Int32
	var exchange url.Values
	p := newTestCodex(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deviceauth/usercode":
			writeJSON(w, 200, map[string]any{"device_auth_id": "dev-1", "user_code": "ABCD-1234", "interval": "0"})
		case "/deviceauth/token":
			if polls.Add(1) < 3 {
				writeJSON(w, 403, map[string]any{"error": "authorization_pending"})
				return
			}
			writeJSON(w, 200, map[string]any{"authorization_code": "dev-code", "code_verifier": "dev-verifier"})
		case "/oauth/token":
			_ = r.ParseForm()
			exchange = r.PostForm
			writeJSON(w, 200, map[string]any{"access_token": codexAccessToken("acct-2"), "refresh_token": "r2", "expires_in": 60})
		}
	})
	// interval dạng chuỗi như server thật; "0" giữ test nhanh (sàn poll là 1ms).
	var shownCode string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cred, err := p.Login(ctx, Controller{OnAuth: func(_, instructions string) { shownCode = instructions }}, LoginOptions{Device: true})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !strings.Contains(shownCode, "ABCD-1234") {
		t.Fatalf("người dùng không được hiển thị mã thiết bị: %q", shownCode)
	}
	if polls.Load() != 3 {
		t.Fatalf("polls = %d, want 3 (2 pending + 1 thành công)", polls.Load())
	}
	if exchange.Get("code") != "dev-code" || exchange.Get("code_verifier") != "dev-verifier" ||
		exchange.Get("redirect_uri") != codexDeviceRedirect {
		t.Fatalf("form đổi mã device sai: %v", exchange)
	}
	if cred.AccountID != "acct-2" || cred.Refresh != "r2" {
		t.Fatalf("credential = %+v", cred)
	}
}

func TestCodexRefreshKeepsUnrotatedFields(t *testing.T) {
	p := newTestCodex(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != "old-refresh" {
			writeJSON(w, 400, map[string]any{"error": "invalid_grant"})
			return
		}
		// Không xoay refresh token, access token không mang claim định danh.
		writeJSON(w, 200, map[string]any{"access_token": "opaque", "expires_in": 120})
	})
	prev := Credential{Access: "a", Refresh: "old-refresh", AccountID: "acct", Email: "e@x", Plan: "plus"}
	got, err := p.Refresh(context.Background(), prev)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	want := Credential{Access: "opaque", Refresh: "old-refresh", Expires: 1_000_000 + 120_000, AccountID: "acct", Email: "e@x", Plan: "plus"}
	if got != want {
		t.Fatalf("refresh = %+v, want %+v", got, want)
	}
}

func TestCodexRefreshSurfacesOAuthError(t *testing.T) {
	p := newTestCodex(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 401, map[string]any{"error": map[string]any{"code": "refresh_token_reused", "message": "Already used"}})
	})
	_, err := p.Refresh(context.Background(), Credential{Refresh: "r"})
	if err == nil || !strings.Contains(err.Error(), "401 refresh_token_reused: Already used") {
		t.Fatalf("err = %v", err)
	}
}

func TestCodexExchangeRejectsMissingExpiry(t *testing.T) {
	p := newTestCodex(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"access_token": codexAccessToken("a"), "refresh_token": "r"})
	})
	if _, err := p.exchangeCode(context.Background(), "c", "v", "u"); err == nil {
		t.Fatal("thiếu expires_in phải báo lỗi thay vì tạo credential không có hạn")
	}
}

func TestCodexAccountIDFromAccessToken(t *testing.T) {
	if got := CodexAccountID(codexAccessToken("acct-9")); got != "acct-9" {
		t.Fatalf("CodexAccountID = %q", got)
	}
	for _, token := range []string{"", "not-a-jwt", "a.b.c"} {
		if got := CodexAccountID(token); got != "" {
			t.Fatalf("CodexAccountID(%q) = %q, want rỗng", token, got)
		}
	}
}
