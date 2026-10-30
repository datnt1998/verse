package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// OpenAICodexID là ID provider cho gói ChatGPT Plus/Pro (Codex subscription).
const OpenAICodexID = "openai-codex"

// Hằng số luồng OAuth của Codex CLI (khớp omp: catalog/compat/rules/auth/openai-codex.kdl).
const (
	codexClientID        = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexScope           = "openid profile email offline_access"
	codexCallbackAddr    = "127.0.0.1:1455"
	codexCallbackPath    = "/auth/callback"
	codexRedirectURI     = "http://localhost:1455/auth/callback"
	codexDeviceRedirect  = "https://auth.openai.com/deviceauth/callback"
	codexDeviceVerifyURL = "https://auth.openai.com/codex/device"
	// CodexOriginator định danh client với backend Codex (header originator và tham số authorize).
	CodexOriginator = "ainovel"
	// codexJWTAuthClaim chứa chatgpt_account_id / chatgpt_plan_type.
	codexJWTAuthClaim    = "https://api.openai.com/auth"
	codexJWTProfileClaim = "https://api.openai.com/profile"
	codexTokenTimeout    = 15 * time.Second
	codexDeviceMaxPolls  = 120
)

type openAICodex struct {
	authorizeURL      string
	tokenURL          string
	deviceUserCodeURL string
	deviceTokenURL    string
	callbackAddr      string
	redirectURI       string
	client            *http.Client
	openBrowser       func(string) error
	now               func() time.Time
	// devicePollFloor là khoảng chờ tối thiểu giữa các lần poll device-code.
	devicePollFloor time.Duration
}

func newOpenAICodex() *openAICodex {
	return &openAICodex{
		authorizeURL:      "https://auth.openai.com/oauth/authorize",
		tokenURL:          "https://auth.openai.com/oauth/token",
		deviceUserCodeURL: "https://auth.openai.com/api/accounts/deviceauth/usercode",
		deviceTokenURL:    "https://auth.openai.com/api/accounts/deviceauth/token",
		callbackAddr:      codexCallbackAddr,
		redirectURI:       codexRedirectURI,
		client:            &http.Client{Timeout: codexTokenTimeout},
		openBrowser:       OpenBrowser,
		now:               time.Now,
		devicePollFloor:   5 * time.Second,
	}
}

func init() { register(newOpenAICodex()) }

func (p *openAICodex) ID() string   { return OpenAICodexID }
func (p *openAICodex) Name() string { return "ChatGPT Plus/Pro (Codex Subscription)" }

func (p *openAICodex) Login(ctx context.Context, ctrl Controller, opts LoginOptions) (Credential, error) {
	if opts.Device {
		return p.loginDevice(ctx, ctrl)
	}
	return p.loginBrowser(ctx, ctrl, opts.From, opts.Manual)
}

// authorizationURL dựng URL authorize (PKCE S256) cho luồng trình duyệt.
func (p *openAICodex) authorizationURL(state, challenge string) string {
	q := url.Values{
		"response_type":              {"code"},
		"client_id":                  {codexClientID},
		"redirect_uri":               {p.redirectURI},
		"scope":                      {codexScope},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"state":                      {state},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"originator":                 {CodexOriginator},
	}
	return p.authorizeURL + "?" + q.Encode()
}

func (p *openAICodex) loginBrowser(ctx context.Context, ctrl Controller, from string, manual <-chan ManualSubmission) (Credential, error) {
	verifier, challenge, err := pkce()
	if err != nil {
		return Credential{}, err
	}
	state, err := randomState()
	if err != nil {
		return Credential{}, err
	}
	server, err := startCallbackServer(p.callbackAddr, codexCallbackPath, state, from, chatGPTBrand)
	if err != nil {
		return Credential{}, err
	}
	defer server.close()

	authURL := p.authorizationURL(state, challenge)
	ctrl.auth(authURL, "Trình duyệt sẽ mở trang đăng nhập ChatGPT; hoàn tất đăng nhập để tiếp tục.")
	// Luồng web: tab do app mở bằng window.open — server không tự mở trình duyệt.
	if p.openBrowser != nil && from != "web" {
		if err := p.openBrowser(authURL); err != nil {
			ctrl.progress("Không tự mở được trình duyệt, hãy mở URL ở trên bằng tay.")
		}
	}
	ctrl.progress("Đang chờ trình duyệt hoàn tất đăng nhập…")
	code, err := waitCodexCode(ctx, server.result, manual, state)
	if err != nil {
		return Credential{}, err
	}
	ctrl.progress("Đang đổi mã xác thực lấy token…")
	return p.exchangeCode(ctx, code, verifier, p.redirectURI)
}

// waitCodexCode đua callback localhost với mã dán tay (dùng khi trình duyệt
// khác máy hoặc cổng callback bận). Mã dán kèm state khác state của phiên bị
// từ chối và luồng vẫn chờ; mã thô/URL thiếu state được chấp nhận.
func waitCodexCode(ctx context.Context, server <-chan callbackResult, manual <-chan ManualSubmission, state string) (string, error) {
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

func (p *openAICodex) loginDevice(ctx context.Context, ctrl Controller) (Credential, error) {
	ctrl.progress("Đang khởi tạo xác thực thiết bị…")
	var init struct {
		DeviceAuthID string          `json:"device_auth_id"`
		UserCode     string          `json:"user_code"`
		Interval     json.RawMessage `json:"interval"`
	}
	status, body, err := p.postJSON(ctx, p.deviceUserCodeURL, map[string]string{"client_id": codexClientID})
	if err != nil {
		return Credential{}, err
	}
	if status != http.StatusOK {
		return Credential{}, fmt.Errorf("khởi tạo device-code thất bại: %s", formatTokenEndpointError(status, body))
	}
	if err := json.Unmarshal(body, &init); err != nil || init.DeviceAuthID == "" || init.UserCode == "" {
		return Credential{}, errors.New("phản hồi device-code thiếu trường bắt buộc")
	}
	interval := parseInterval(init.Interval)
	if interval < p.devicePollFloor {
		interval = p.devicePollFloor
	}

	ctrl.auth(codexDeviceVerifyURL, "Nhập mã: "+init.UserCode)
	ctrl.deviceCode(init.UserCode, codexDeviceVerifyURL)

	for range codexDeviceMaxPolls {
		select {
		case <-ctx.Done():
			return Credential{}, ctx.Err()
		case <-time.After(interval):
		}
		status, body, err := p.postJSON(ctx, p.deviceTokenURL, map[string]string{
			"device_auth_id": init.DeviceAuthID,
			"user_code":      init.UserCode,
		})
		if err != nil {
			return Credential{}, err
		}
		// 403/404 = người dùng chưa xác nhận, tiếp tục chờ.
		if status == http.StatusForbidden || status == http.StatusNotFound {
			continue
		}
		if status != http.StatusOK {
			return Credential{}, fmt.Errorf("poll device-code thất bại: %s", formatTokenEndpointError(status, body))
		}
		var granted struct {
			AuthorizationCode string `json:"authorization_code"`
			CodeVerifier      string `json:"code_verifier"`
		}
		if err := json.Unmarshal(body, &granted); err != nil || granted.AuthorizationCode == "" || granted.CodeVerifier == "" {
			return Credential{}, errors.New("phản hồi device-code thiếu authorization_code hoặc code_verifier")
		}
		ctrl.progress("Đang đổi mã xác thực lấy token…")
		return p.exchangeCode(ctx, granted.AuthorizationCode, granted.CodeVerifier, codexDeviceRedirect)
	}
	return Credential{}, errors.New("hết thời gian chờ xác nhận device-code")
}

func parseInterval(raw json.RawMessage) time.Duration {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

type codexTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    *int64 `json:"expires_in"`
}

func (p *openAICodex) exchangeCode(ctx context.Context, code, verifier, redirectURI string) (Credential, error) {
	tok, err := p.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {codexClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		return Credential{}, fmt.Errorf("đổi mã lấy token thất bại: %w", err)
	}
	if tok.RefreshToken == "" {
		return Credential{}, errors.New("phản hồi token thiếu refresh_token")
	}
	cred := p.credentialFrom(tok, Credential{})
	if cred.AccountID == "" && cred.Email == "" {
		return Credential{}, errors.New("không đọc được định danh tài khoản ChatGPT từ token")
	}
	return cred, nil
}

func (p *openAICodex) Refresh(ctx context.Context, cred Credential) (Credential, error) {
	tok, err := p.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {codexClientID},
		"refresh_token": {cred.Refresh},
	})
	if err != nil {
		return Credential{}, err
	}
	return p.credentialFrom(tok, cred), nil
}

// credentialFrom dựng credential từ phản hồi token; trường thiếu (refresh token
// không xoay vòng, claim vắng trên token refresh) giữ giá trị của prev.
func (p *openAICodex) credentialFrom(tok codexTokenResponse, prev Credential) Credential {
	cred := Credential{
		Access:    tok.AccessToken,
		Refresh:   tok.RefreshToken,
		Expires:   p.now().Add(time.Duration(*tok.ExpiresIn) * time.Second).UnixMilli(),
		AccountID: prev.AccountID,
		Email:     prev.Email,
		Plan:      prev.Plan,
	}
	if cred.Refresh == "" {
		cred.Refresh = prev.Refresh
	}
	accountID, email, plan := codexTokenProfile(tok.AccessToken, tok.IDToken)
	if accountID != "" {
		cred.AccountID = accountID
	}
	if email != "" {
		cred.Email = email
	}
	if plan != "" {
		cred.Plan = plan
	}
	return cred
}

// CodexAccountID đọc chatgpt_account_id từ access token (rỗng nếu không phải JWT Codex).
func CodexAccountID(accessToken string) string {
	accountID, _, _ := codexTokenProfile(accessToken, "")
	return accountID
}

// codexTokenProfile đọc định danh từ claim của access token, rơi về id_token.
// chatgpt_plan_type có thể chỉ xuất hiện trên id_token.
func codexTokenProfile(accessToken, idToken string) (accountID, email, plan string) {
	for _, token := range []string{accessToken, idToken} {
		if token == "" {
			continue
		}
		claims := decodeJWTClaims(token)
		if claims == nil {
			continue
		}
		authClaim := claimObject(claims, codexJWTAuthClaim)
		if accountID == "" {
			accountID = claimString(authClaim, "chatgpt_account_id")
		}
		if plan == "" {
			plan = strings.ToLower(claimString(authClaim, "chatgpt_plan_type"))
		}
		if email == "" {
			email = strings.ToLower(claimString(claimObject(claims, codexJWTProfileClaim), "email"))
		}
		if email == "" {
			email = strings.ToLower(claimString(claims, "email"))
		}
	}
	return accountID, email, plan
}

func (p *openAICodex) tokenRequest(ctx context.Context, form url.Values) (codexTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, codexTokenTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return codexTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return codexTokenResponse{}, fmt.Errorf("gọi máy chủ token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return codexTokenResponse{}, fmt.Errorf("máy chủ token trả %s", formatTokenEndpointError(resp.StatusCode, body))
	}
	var tok codexTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return codexTokenResponse{}, fmt.Errorf("giải mã phản hồi token: %w", err)
	}
	if tok.AccessToken == "" || tok.ExpiresIn == nil {
		return codexTokenResponse{}, errors.New("phản hồi token thiếu access_token hoặc expires_in")
	}
	return tok, nil
}

func (p *openAICodex) postJSON(ctx context.Context, endpoint string, payload any) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, codexTokenTimeout)
	defer cancel()
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("gọi %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, body, nil
}

// formatTokenEndpointError rút gọn lỗi OAuth {"error","error_description"} thành một dòng.
func formatTokenEndpointError(status int, body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return strconv.Itoa(status)
	}
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		code := describeOAuthValue(parsed["error"])
		desc := describeOAuthValue(parsed["error_description"])
		switch {
		case code != "" && desc != "" && code != desc:
			return fmt.Sprintf("%d %s: %s", status, code, desc)
		case code != "":
			return fmt.Sprintf("%d %s", status, code)
		case desc != "":
			return fmt.Sprintf("%d %s", status, desc)
		}
	}
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return fmt.Sprintf("%d %s", status, text)
}

func describeOAuthValue(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		code := describeOAuthValue(value["code"])
		if code == "" {
			code = describeOAuthValue(value["type"]) // Anthropic: {"error":{"type":"invalid_grant","message":…}}
		}
		msg := describeOAuthValue(value["message"])
		if code != "" && msg != "" && code != msg {
			return code + ": " + msg
		}
		if code != "" {
			return code
		}
		return msg
	}
	return ""
}
