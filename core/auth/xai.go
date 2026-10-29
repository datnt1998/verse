package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// XAIOAuthID là ID provider cho gói SuperGrok / X Premium+ (đăng nhập OAuth, không cần API key).
const XAIOAuthID = "xai-oauth"

// Hằng số luồng OAuth của Grok CLI (khớp Hermes Agent, được xAI công nhận: https://x.ai/news/grok-hermes).
const (
	xaiIssuer         = "https://auth.x.ai"
	xaiClientID       = "b1a00492-073a-47ea-816f-4c329264a828"
	xaiScope          = "openid profile email offline_access grok-cli:access api:access"
	xaiDeviceGrant    = "urn:ietf:params:oauth:grant-type:device_code"
	xaiHTTPTimeout    = 20 * time.Second
	xaiDefaultPoll    = 5 * time.Second
	xaiSlowDownStep   = 5 * time.Second
	xaiDefaultExpiry  = 10 * time.Minute
	xaiFallbackExpiry = 30 * time.Minute
)

// XAITierDeniedMessage là thông báo khi xAI trả 403 sau khi đăng nhập thành công:
// đó là giới hạn gói (tier gating) chứ không phải token cũ, nên làm mới/đăng nhập lại vô ích.
const XAITierDeniedMessage = "Gói SuperGrok / X Premium+ của bạn chưa được xAI cấp quyền gọi API qua đăng nhập OAuth " +
	"(đăng nhập lại hay làm mới token cũng không đổi được điều này). " +
	"Hãy dùng API key xAI với provider \"grok\", hoặc nâng cấp gói tại https://x.ai/grok"

// ErrXAITierDenied báo tài khoản xAI không có quyền API qua OAuth (HTTP 403).
var ErrXAITierDenied = errors.New(XAITierDeniedMessage)

type xaiOAuth struct {
	issuer string
	client *http.Client
	// validateEndpoint kiểm tra endpoint do discovery trả về trước khi gửi token tới đó.
	validateEndpoint func(string) error
	openBrowser      func(string) error
	now              func() time.Time
	// wait chờ d hoặc ctx kết thúc; test thay để không phải ngủ thật.
	wait func(ctx context.Context, d time.Duration) error
}

func newXAIOAuth() *xaiOAuth {
	return &xaiOAuth{
		issuer:           xaiIssuer,
		client:           &http.Client{Timeout: xaiHTTPTimeout},
		validateEndpoint: validateXAIEndpoint,
		openBrowser:      OpenBrowser,
		now:              time.Now,
		wait:             sleepCtx,
	}
}

func init() { register(newXAIOAuth()) }

func (p *xaiOAuth) ID() string   { return XAIOAuthID }
func (p *xaiOAuth) Name() string { return "SuperGrok / X Premium+" }

// DeviceOnly: xAI không có luồng callback localhost, chỉ có device-code (RFC 8628).
func (p *xaiOAuth) DeviceOnly() bool { return true }

// IsXAIHost báo host thuộc x.ai hoặc *.x.ai.
func IsXAIHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "x.ai" || strings.HasSuffix(host, ".x.ai")
}

// validateXAIEndpoint chỉ chấp nhận https trên miền x.ai: token_endpoint nhận refresh
// token nên không được để discovery bị đánh tráo trỏ sang máy chủ khác.
func validateXAIEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("endpoint xAI không hợp lệ: %q", raw)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("endpoint xAI phải dùng https: %q", raw)
	}
	if !IsXAIHost(u.Hostname()) {
		return fmt.Errorf("endpoint xAI %q không thuộc miền x.ai — từ chối gửi token tới đó", raw)
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type xaiTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    *int64 `json:"expires_in"`
}

type xaiDeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

// tokenEndpoint đọc token_endpoint từ OIDC discovery và kiểm tra miền.
func (p *xaiOAuth) tokenEndpoint(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, xaiHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lấy cấu hình OIDC của xAI: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cấu hình OIDC của xAI trả HTTP %d", resp.StatusCode)
	}
	var doc struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("giải mã cấu hình OIDC của xAI: %w", err)
	}
	endpoint := strings.TrimSpace(doc.TokenEndpoint)
	if endpoint == "" {
		return "", errors.New("cấu hình OIDC của xAI thiếu token_endpoint")
	}
	if err := p.validateEndpoint(endpoint); err != nil {
		return "", err
	}
	return endpoint, nil
}

// Login luôn dùng device-code (opts.Device bị bỏ qua vì xAI không có luồng khác).
func (p *xaiOAuth) Login(ctx context.Context, ctrl Controller, opts LoginOptions) (Credential, error) {
	ctrl.progress("Đang khởi tạo xác thực thiết bị xAI…")
	tokenURL, err := p.tokenEndpoint(ctx)
	if err != nil {
		return Credential{}, err
	}
	dc, err := p.requestDeviceCode(ctx)
	if err != nil {
		return Credential{}, err
	}
	verifyURL := dc.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dc.VerificationURI
	}
	ctrl.auth(verifyURL, "Nhập mã: "+dc.UserCode)
	ctrl.deviceCode(dc.UserCode, verifyURL)
	// Luồng web: người dùng tự mở link trên thiết bị của họ; server không mở trình duyệt.
	if p.openBrowser != nil && opts.From != "web" {
		if err := p.openBrowser(verifyURL); err != nil {
			ctrl.progress("Không tự mở được trình duyệt, hãy mở URL ở trên bằng tay.")
		}
	}
	ctrl.progress("Đang chờ bạn xác nhận trên trình duyệt…")
	tok, err := p.pollDeviceToken(ctx, tokenURL, dc)
	if err != nil {
		return Credential{}, err
	}
	cred := p.credentialFrom(tok, Credential{})
	p.fillProfile(ctx, &cred)
	return cred, nil
}

func (p *xaiOAuth) requestDeviceCode(ctx context.Context) (xaiDeviceCode, error) {
	status, body, err := p.postForm(ctx, p.issuer+"/oauth2/device/code", url.Values{
		"client_id": {xaiClientID},
		"scope":     {xaiScope},
	})
	if err != nil {
		return xaiDeviceCode{}, err
	}
	if status != http.StatusOK {
		return xaiDeviceCode{}, fmt.Errorf("khởi tạo device-code xAI thất bại: %s", formatTokenEndpointError(status, body))
	}
	var dc xaiDeviceCode
	if err := json.Unmarshal(body, &dc); err != nil || dc.DeviceCode == "" || dc.UserCode == "" || (dc.VerificationURI == "" && dc.VerificationURIComplete == "") {
		return xaiDeviceCode{}, errors.New("phản hồi device-code của xAI thiếu trường bắt buộc")
	}
	return dc, nil
}

// pollDeviceToken hỏi token_endpoint theo RFC 8628 §3.5 tới khi người dùng xác nhận,
// từ chối, hoặc mã hết hạn.
func (p *xaiOAuth) pollDeviceToken(ctx context.Context, tokenURL string, dc xaiDeviceCode) (xaiTokenResponse, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval <= 0 {
		interval = xaiDefaultPoll
	}
	lifetime := time.Duration(dc.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = xaiDefaultExpiry
	}
	ctx, cancel := context.WithTimeout(ctx, lifetime)
	defer cancel()

	for {
		if err := p.wait(ctx, interval); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
				return xaiTokenResponse{}, errors.New("hết thời gian chờ xác nhận đăng nhập xAI — mã thiết bị đã hết hạn")
			}
			return xaiTokenResponse{}, err
		}
		status, body, err := p.postForm(ctx, tokenURL, url.Values{
			"grant_type":  {xaiDeviceGrant},
			"client_id":   {xaiClientID},
			"device_code": {dc.DeviceCode},
		})
		if err != nil {
			if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return xaiTokenResponse{}, errors.New("hết thời gian chờ xác nhận đăng nhập xAI — mã thiết bị đã hết hạn")
			}
			return xaiTokenResponse{}, err
		}
		if status == http.StatusOK {
			var tok xaiTokenResponse
			if err := json.Unmarshal(body, &tok); err != nil {
				return xaiTokenResponse{}, fmt.Errorf("giải mã phản hồi token xAI: %w", err)
			}
			if tok.AccessToken == "" || tok.RefreshToken == "" {
				return xaiTokenResponse{}, errors.New("phản hồi token xAI thiếu access_token hoặc refresh_token")
			}
			return tok, nil
		}
		var oauthErr struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &oauthErr)
		switch oauthErr.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += xaiSlowDownStep
			continue
		case "access_denied":
			return xaiTokenResponse{}, errors.New("bạn đã từ chối đăng nhập trên trang xAI")
		case "expired_token":
			return xaiTokenResponse{}, errors.New("mã thiết bị xAI đã hết hạn — hãy đăng nhập lại để lấy mã mới")
		}
		if status == http.StatusForbidden {
			return xaiTokenResponse{}, ErrXAITierDenied
		}
		return xaiTokenResponse{}, fmt.Errorf("poll device-code xAI thất bại: %s", formatTokenEndpointError(status, body))
	}
}

func (p *xaiOAuth) Refresh(ctx context.Context, cred Credential) (Credential, error) {
	tokenURL, err := p.tokenEndpoint(ctx)
	if err != nil {
		return Credential{}, err
	}
	status, body, err := p.postForm(ctx, tokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {xaiClientID},
		"refresh_token": {cred.Refresh},
	})
	if err != nil {
		return Credential{}, err
	}
	if status == http.StatusForbidden {
		return Credential{}, ErrXAITierDenied
	}
	if status != http.StatusOK {
		return Credential{}, fmt.Errorf("máy chủ token xAI trả %s", formatTokenEndpointError(status, body))
	}
	var tok xaiTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return Credential{}, fmt.Errorf("giải mã phản hồi token xAI: %w", err)
	}
	if tok.AccessToken == "" {
		return Credential{}, errors.New("phản hồi token xAI thiếu access_token")
	}
	return p.credentialFrom(tok, cred), nil
}

// credentialFrom dựng credential từ phản hồi token. Hạn token lấy từ claim exp của
// JWT (nguồn chính xác nhất), rơi về expires_in. Trường thiếu giữ giá trị của prev
// (refresh không nhất thiết xoay vòng refresh token hay trả lại id_token).
func (p *xaiOAuth) credentialFrom(tok xaiTokenResponse, prev Credential) Credential {
	cred := Credential{
		Access:    tok.AccessToken,
		Refresh:   tok.RefreshToken,
		AccountID: prev.AccountID,
		Email:     prev.Email,
		Plan:      prev.Plan,
	}
	if cred.Refresh == "" {
		cred.Refresh = prev.Refresh
	}
	switch exp := jwtExpiry(tok.AccessToken); {
	case exp > 0:
		cred.Expires = exp
	case tok.ExpiresIn != nil && *tok.ExpiresIn > 0:
		cred.Expires = p.now().Add(time.Duration(*tok.ExpiresIn) * time.Second).UnixMilli()
	default:
		cred.Expires = p.now().Add(xaiFallbackExpiry).UnixMilli()
	}
	for _, token := range []string{tok.IDToken, tok.AccessToken} {
		claims := decodeJWTClaims(token)
		if claims == nil {
			continue
		}
		if sub := claimString(claims, "sub"); sub != "" && cred.AccountID == "" {
			cred.AccountID = sub
		}
		if email := strings.ToLower(claimString(claims, "email")); email != "" {
			cred.Email = email
		}
	}
	return cred
}

// jwtExpiry trả về claim exp (Unix mili-giây) của JWT; 0 nếu không phải JWT/không có exp.
func jwtExpiry(token string) int64 {
	claims := decodeJWTClaims(token)
	if claims == nil {
		return 0
	}
	exp, ok := claims["exp"].(float64)
	if !ok || exp <= 0 {
		return 0
	}
	return int64(exp) * 1000
}

// fillProfile lấy email/sub từ userinfo để dựng nhãn tài khoản; lỗi chỉ làm nhãn nghèo đi.
func (p *xaiOAuth) fillProfile(ctx context.Context, cred *Credential) {
	ctx, cancel := context.WithTimeout(ctx, xaiHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.issuer+"/oauth2/userinfo", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+cred.Access)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info) != nil {
		return
	}
	if sub := strings.TrimSpace(info.Sub); sub != "" {
		cred.AccountID = sub
	}
	switch email := strings.ToLower(strings.TrimSpace(info.Email)); {
	case email != "":
		cred.Email = email
	case cred.Email == "":
		cred.Email = strings.TrimSpace(info.Name)
	}
}

func (p *xaiOAuth) postForm(ctx context.Context, endpoint string, form url.Values) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, xaiHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("gọi %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, body, nil
}
