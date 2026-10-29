package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AnthropicOAuthID là ID provider cho gói Claude Pro/Max (đăng nhập OAuth, không cần API key).
const AnthropicOAuthID = "anthropic-oauth"

// Hằng số luồng OAuth của Claude Code (khớp omp: catalog/compat/rules/auth/anthropic.kdl).
const (
	// anthropicClientIDB64 là client id công khai của Claude Code, lưu base64 như omp.
	anthropicClientIDB64   = "OWQxYzI1MGEtZTYxYi00NGQ5LTg4ZWQtNTk0NGQxOTYyZjVl"
	anthropicAuthorizeURL  = "https://claude.ai/oauth/authorize"
	anthropicTokenURL      = "https://api.anthropic.com/v1/oauth/token"
	anthropicBootstrapURL  = "https://api.anthropic.com/api/claude_cli/bootstrap"
	anthropicBootstrapMdl  = "claude-opus-4-8"
	anthropicCallbackAddr  = "127.0.0.1:54545"
	anthropicCallbackPath  = "/callback"
	anthropicRedirectURI   = "http://localhost:54545/callback"
	anthropicOAuthBeta     = "oauth-2025-04-20"
	anthropicRequestTO     = 30 * time.Second
	anthropicPKCEBytes     = 96
	anthropicAuthorizeHint = "Hoàn tất đăng nhập trong trình duyệt. Nếu trình duyệt không truy cập được máy này, hãy dán URL chuyển hướng cuối cùng hoặc mã xác thực (code#state)."
)

// anthropicScopes: user:inference cho suy luận bằng OAuth token, phần còn lại cho quản lý
// tài khoản/phiên. platform.claude.com chỉ cấp token console; suy luận trực tiếp cần claude.ai.
var anthropicScopes = []string{
	"org:create_api_key", "user:profile", "user:inference",
	"user:sessions:claude_code", "user:mcp_servers", "user:file_upload",
}

func anthropicClientID() string {
	raw, _ := base64.StdEncoding.DecodeString(anthropicClientIDB64)
	return string(raw)
}

type anthropicOAuth struct {
	authorizeURL string
	tokenURL     string
	bootstrapURL string
	callbackAddr string
	redirectURI  string
	client       *http.Client
	openBrowser  func(string) error
	now          func() time.Time
}

func newAnthropicOAuth() *anthropicOAuth {
	return &anthropicOAuth{
		authorizeURL: anthropicAuthorizeURL,
		tokenURL:     anthropicTokenURL,
		bootstrapURL: anthropicBootstrapURL,
		callbackAddr: anthropicCallbackAddr,
		redirectURI:  anthropicRedirectURI,
		client:       &http.Client{Timeout: anthropicRequestTO},
		openBrowser:  OpenBrowser,
		now:          time.Now,
	}
}

func init() { register(newAnthropicOAuth()) }

func (p *anthropicOAuth) ID() string   { return AnthropicOAuthID }
func (p *anthropicOAuth) Name() string { return "Claude Pro/Max" }

// PasteCode: luồng nhận mã dán tay (LoginOptions.Manual).
func (p *anthropicOAuth) PasteCode() bool { return true }

// authorizationURL dựng URL authorize (PKCE S256); code=true bắt claude.ai hiện mã
// `code#state` để người dùng dán lại khi không tới được callback.
func (p *anthropicOAuth) authorizationURL(state, challenge string) string {
	q := url.Values{
		"code":                  {"true"},
		"client_id":             {anthropicClientID()},
		"response_type":         {"code"},
		"redirect_uri":          {p.redirectURI},
		"scope":                 {strings.Join(anthropicScopes, " ")},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	return p.authorizeURL + "?" + q.Encode()
}

// ParseAuthorizationInput tách code/state từ thứ người dùng dán: URL chuyển hướng đầy đủ,
// chuỗi query (`code=…&state=…`) hoặc mã thô `code#state` (giống omp parseCallbackInput).
func ParseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}
	if u, err := url.Parse(value); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Query().Get("code"), u.Query().Get("state")
	}
	if strings.Contains(value, "code=") {
		q, _ := url.ParseQuery(strings.TrimLeft(value, "?#"))
		return q.Get("code"), q.Get("state")
	}
	// Mã thô, có thể kèm state sau '#' (giữ ngữ nghĩa split("#", 2) của JS: bỏ phần thừa).
	parts := strings.Split(value, "#")
	code = parts[0]
	if len(parts) > 1 {
		state = parts[1]
	}
	return code, state
}

func (p *anthropicOAuth) Login(ctx context.Context, ctrl Controller, opts LoginOptions) (Credential, error) {
	if opts.Device && opts.Manual == nil {
		return Credential{}, errors.New("đăng nhập bằng mã cần nơi nhập mã dán từ trình duyệt")
	}
	verifier, challenge, err := pkceBytes(anthropicPKCEBytes)
	if err != nil {
		return Credential{}, err
	}
	state, err := randomState()
	if err != nil {
		return Credential{}, err
	}
	var server *callbackServer
	var serverResult <-chan callbackResult
	if !opts.Device {
		server, err = startCallbackServer(p.callbackAddr, anthropicCallbackPath, state, opts.From, claudeBrand)
		switch {
		case err == nil:
			defer server.close()
			serverResult = server.result
		case opts.Manual == nil:
			return Credential{}, err
		default:
			// Cổng 54545 bận nhưng UI có ô dán mã: vẫn đăng nhập được bằng cách dán.
			server = nil
			ctrl.progress("Không mở được cổng callback localhost:54545 — hãy dán mã xác thực vào ô nhập.")
		}
	}

	authURL := p.authorizationURL(state, challenge)
	ctrl.auth(authURL, anthropicAuthorizeHint)
	// Luồng web: tab do app mở bằng window.open — server không tự mở trình duyệt.
	if server != nil && p.openBrowser != nil && opts.From != "web" {
		if err := p.openBrowser(authURL); err != nil {
			ctrl.progress("Không tự mở được trình duyệt, hãy mở URL ở trên bằng tay.")
		}
	}
	ctrl.progress("Đang chờ trình duyệt hoàn tất đăng nhập…")
	code, err := waitAnthropicCode(ctx, serverResult, opts.Manual, state)
	if err != nil {
		return Credential{}, err
	}
	ctrl.progress("Đang đổi mã xác thực lấy token…")
	cred, err := p.exchangeCode(ctx, code, state, verifier)
	if err != nil {
		return Credential{}, err
	}
	return p.fillIdentity(ctx, cred, true), nil
}

// waitAnthropicCode đua callback localhost với mã dán tay. Mã dán kèm state khác state của
// phiên bị từ chối (báo lại cho UI) và luồng vẫn chờ; state trống (mã thô) được chấp nhận.
func waitAnthropicCode(ctx context.Context, server <-chan callbackResult, manual <-chan ManualSubmission, state string) (string, error) {
	for {
		select {
		case res := <-server:
			return res.code, res.err
		case sub := <-manual:
			code, got := ParseAuthorizationInput(sub.Input)
			var reject error
			switch {
			case code == "":
				reject = errors.New("không tìm thấy mã xác thực trong nội dung đã dán")
			case got != "" && got != state:
				reject = errors.New("mã dán thuộc một phiên đăng nhập khác (state không khớp) — hãy mở lại URL đăng nhập mới nhất")
			}
			if sub.Result != nil {
				sub.Result <- reject
			}
			if reject == nil {
				return code, nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

type anthropicTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    *float64 `json:"expires_in"`
	Account      *struct {
		UUID  string `json:"uuid"`
		Email string `json:"email_address"`
	} `json:"account"`
	Organization *struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"organization"`
}

// exchangeCode đổi code lấy token: POST JSON, kèm state trong body (như omp).
func (p *anthropicOAuth) exchangeCode(ctx context.Context, code, state, verifier string) (Credential, error) {
	tok, err := p.tokenRequest(ctx, map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID(),
		"code":          code,
		"redirect_uri":  p.redirectURI,
		"code_verifier": verifier,
		"state":         state,
	}, nil)
	if err != nil {
		return Credential{}, fmt.Errorf("đổi mã lấy token thất bại: %w", err)
	}
	cred := p.credentialFrom(tok, Credential{})
	if tok.Organization != nil {
		cred.OrgID, cred.OrgName = tok.Organization.UUID, tok.Organization.Name
	}
	return cred, nil
}

// Refresh làm mới token. Claude Code gửi anthropic-beta và User-Agent của SDK ở bước refresh
// (không gửi ở bước đổi mã). Tổ chức cố định từ lúc đăng nhập nên lấy lại từ credential cũ.
func (p *anthropicOAuth) Refresh(ctx context.Context, cred Credential) (Credential, error) {
	tok, err := p.tokenRequest(ctx, map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     anthropicClientID(),
		"refresh_token": cred.Refresh,
	}, map[string]string{
		"anthropic-beta": anthropicOAuthBeta,
		"User-Agent":     "anthropic-sdk-typescript/" + ClaudeCodeSDKVersion + " userOAuthProvider",
	})
	if err != nil {
		return Credential{}, err
	}
	next := p.credentialFrom(tok, cred)
	next.OrgID, next.OrgName = cred.OrgID, cred.OrgName
	return p.fillIdentity(ctx, next, false), nil
}

// credentialFrom dựng credential từ phản hồi token. Hạn token = now + expires_in: omp trừ
// thêm 5 phút ngay trong giá trị lưu, ở đây Manager đã refresh sớm 5 phút (defaultRefreshSkew)
// nên giữ hạn thật — hành vi hiệu dụng giống nhau. Trường thiếu giữ giá trị của prev.
func (p *anthropicOAuth) credentialFrom(tok anthropicTokenResponse, prev Credential) Credential {
	cred := Credential{
		Access:    tok.AccessToken,
		Refresh:   tok.RefreshToken,
		Expires:   p.now().Add(time.Duration(*tok.ExpiresIn * float64(time.Second))).UnixMilli(),
		AccountID: prev.AccountID,
		Email:     prev.Email,
		Plan:      prev.Plan,
	}
	if cred.Refresh == "" {
		cred.Refresh = prev.Refresh
	}
	if tok.Account != nil {
		if tok.Account.UUID != "" {
			cred.AccountID = tok.Account.UUID
		}
		if tok.Account.Email != "" {
			cred.Email = tok.Account.Email
		}
	}
	return cred
}

// tokenRequest POST JSON tới token endpoint (đổi mã và refresh dùng chung).
func (p *anthropicOAuth) tokenRequest(ctx context.Context, params, headers map[string]string) (anthropicTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, anthropicRequestTO)
	defer cancel()
	data, err := json.Marshal(params)
	if err != nil {
		return anthropicTokenResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, bytes.NewReader(data))
	if err != nil {
		return anthropicTokenResponse{}, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return anthropicTokenResponse{}, fmt.Errorf("gọi máy chủ token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return anthropicTokenResponse{}, fmt.Errorf("máy chủ token trả %s", formatTokenEndpointError(resp.StatusCode, body))
	}
	var tok anthropicTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return anthropicTokenResponse{}, fmt.Errorf("giải mã phản hồi token: %w", err)
	}
	if tok.AccessToken == "" {
		return anthropicTokenResponse{}, errors.New("phản hồi token thiếu access_token")
	}
	if tok.ExpiresIn == nil {
		return anthropicTokenResponse{}, errors.New("phản hồi token thiếu expires_in")
	}
	return tok, nil
}

// fillIdentity bổ sung tài khoản/tổ chức từ /api/claude_cli/bootstrap khi phản hồi token bỏ
// trống (hook anthropic-identity của omp). Tổ chức chỉ lấy lúc đăng nhập: ghi lại danh tính
// trong lúc refresh nền có thể âm thầm đổi khoá credential. Mọi lỗi bootstrap bị nuốt.
func (p *anthropicOAuth) fillIdentity(ctx context.Context, cred Credential, includeOrg bool) Credential {
	orgSatisfied := !includeOrg || cred.OrgID != ""
	if cred.AccountID != "" && cred.Email != "" && orgSatisfied {
		return cred
	}
	id, err := p.fetchBootstrap(ctx, cred.Access)
	if err != nil {
		return cred
	}
	if cred.AccountID == "" {
		cred.AccountID = id.AccountID
	}
	if cred.Email == "" {
		cred.Email = id.Email
	}
	if includeOrg {
		if cred.OrgID == "" {
			cred.OrgID = id.OrgID
		}
		if cred.OrgName == "" {
			cred.OrgName = id.OrgName
		}
	}
	return cred
}

type anthropicIdentity struct{ AccountID, Email, OrgID, OrgName string }

func (p *anthropicOAuth) fetchBootstrap(ctx context.Context, accessToken string) (anthropicIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, anthropicRequestTO)
	defer cancel()
	target := p.bootstrapURL + "?entrypoint=cli&model=" + url.QueryEscape(anthropicBootstrapMdl)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return anthropicIdentity{}, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-code/"+ClaudeCodeVersion())
	req.Header.Set("anthropic-beta", anthropicOAuthBeta)
	resp, err := p.client.Do(req)
	if err != nil {
		return anthropicIdentity{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return anthropicIdentity{}, fmt.Errorf("bootstrap trả %d", resp.StatusCode)
	}
	var data struct {
		OAuthAccount struct {
			AccountUUID      string `json:"account_uuid"`
			AccountEmail     string `json:"account_email"`
			OrganizationUUID string `json:"organization_uuid"`
			OrganizationName string `json:"organization_name"`
		} `json:"oauth_account"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return anthropicIdentity{}, err
	}
	a := data.OAuthAccount
	return anthropicIdentity{AccountID: a.AccountUUID, Email: a.AccountEmail, OrgID: a.OrganizationUUID, OrgName: a.OrganizationName}, nil
}
