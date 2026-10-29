// Package xai là lớp wire cho API xAI (api.x.ai/v1) khi xác thực bằng gói thuê bao
// SuperGrok / X Premium+ qua OAuth thay cho API key.
//
// API xAI nói giao thức OpenAI-compatible, nên ở đây tái dùng nguyên provider "grok"
// của litellm (Chat Completions, ánh xạ reasoning_effort, strict tools…) và chỉ thêm
// một http.RoundTripper lo phần xác thực:
//   - mỗi request mang "Authorization: Bearer <access token>" lấy từ auth.Manager
//     (tự làm mới trước hạn);
//   - HTTP 401 → làm mới ép buộc một lần rồi gửi lại;
//   - HTTP 403 → hạn chế gói (không phải token cũ): trả thông báo tiếng Việt, không refresh.
package xai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/auth"
	"github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/llm/provider/compat"
	"github.com/datnt1998/noverse/internal/llm/provider/grok"
	"github.com/datnt1998/noverse/internal/llm/wire"
)

const (
	// ProviderName là tên provider trong config và kho credential.
	ProviderName = auth.XAIOAuthID
	// DefaultBaseURL là gốc API xAI; endpoint chat là <base>/chat/completions.
	DefaultBaseURL = "https://api.x.ai/v1"
	userAgent      = "ainovel-cli"
)

// TokenSource cấp access token OAuth còn hạn. Invalidate được gọi khi API trả 401
// cho token stale và phải trả về token mới (đã refresh).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Invalidate(ctx context.Context, stale string) (string, error)
}

// ManagerTokens là TokenSource đọc credential "xai-oauth" từ auth.Manager.
func ManagerTokens(m *auth.Manager) TokenSource { return managerTokens{m: m} }

type managerTokens struct{ m *auth.Manager }

func (s managerTokens) Token(ctx context.Context) (string, error) {
	cred, err := s.m.Credential(ctx, auth.XAIOAuthID)
	return cred.Access, err
}

func (s managerTokens) Invalidate(ctx context.Context, stale string) (string, error) {
	cred, err := s.m.Invalidate(ctx, auth.XAIOAuthID, stale)
	return cred.Access, err
}

// ValidateBaseURL chỉ cho phép gửi bearer tới https trên miền x.ai: token OAuth chỉ
// có giá trị với API xAI, trỏ sang nơi khác sẽ làm lộ credential. Rỗng = mặc định.
func ValidateBaseURL(base string) error {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("base_url %q không hợp lệ", base)
	}
	if u.Scheme != "https" || !auth.IsXAIHost(u.Hostname()) {
		return fmt.Errorf("base_url %q phải là https trên miền x.ai — provider xai-oauth không gửi token OAuth tới nơi khác", base)
	}
	return nil
}

// Config cấu hình Provider.
type Config struct {
	// BaseURL ghi đè DefaultBaseURL.
	BaseURL string
	Tokens  TokenSource
	// Headers bổ sung vào mọi request (providers.<name>.extra.headers).
	Headers map[string]string
	// ExtraBody được trộn vào body JSON của POST (providers.<name>.extra_body).
	ExtraBody map[string]any
	// StreamIdleTimeout: ngắt stream khi không nhận chunk nào trong khoảng này; 0 = tắt.
	StreamIdleTimeout time.Duration
	// Transport là RoundTripper nền; nil dùng http.DefaultTransport.
	Transport http.RoundTripper
}

// New tạo litellm.Provider (provider "grok" của litellm) với transport OAuth.
func New(cfg Config) (*compat.Provider, error) {
	if cfg.Tokens == nil {
		return nil, fmt.Errorf("xai: thiếu nguồn token OAuth")
	}
	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return grok.New(compat.Config{
		// Authorization do transport sở hữu; provider chỉ cần một giá trị khác rỗng.
		APIKey:                      "xai-oauth",
		BaseURL:                     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		Transport:                   &transport{base: base, tokens: cfg.Tokens, extraBody: cfg.ExtraBody},
		UserAgent:                   userAgent,
		Headers:                     cfg.Headers,
		AllowUnknownProviderOptions: true,
	})
}

// NewChatModel tạo agentcore ChatModel cho một model xAI (vd. "grok-4.6").
func NewChatModel(model string, cfg Config) (*llm.ClientModel, error) {
	provider, err := New(cfg)
	if err != nil {
		return nil, err
	}
	var opts []litellm.ClientOption
	if cfg.StreamIdleTimeout > 0 {
		opts = append(opts, litellm.WithStreamIdleTimeout(cfg.StreamIdleTimeout))
	}
	client, err := litellm.New(provider, opts...)
	if err != nil {
		return nil, err
	}
	return llm.NewClientModel(model, client), nil
}

type transport struct {
	base      http.RoundTripper
	tokens    TokenSource
	extraBody map[string]any
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("xai: đọc body request: %w", err)
		}
		body = data
	}
	if req.Method == http.MethodPost && len(t.extraBody) > 0 {
		merged, err := mergeExtraBody(body, t.extraBody)
		if err != nil {
			return nil, err
		}
		body = merged
	}

	ctx := req.Context()
	token, err := t.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.send(req, body, token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		// 401: token bị thu hồi/hết hạn sớm — refresh một lần rồi gửi lại.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		token, err = t.tokens.Invalidate(ctx, token)
		if err != nil {
			return nil, err
		}
		if resp, err = t.send(req, body, token); err != nil {
			return nil, err
		}
	}
	if resp.StatusCode == http.StatusForbidden {
		return tierDenied(resp), nil
	}
	return resp, nil
}

func (t *transport) send(orig *http.Request, body []byte, token string) (*http.Response, error) {
	req := orig.Clone(orig.Context())
	if orig.Body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(req)
}

// tierDenied thay body 403 bằng lỗi tiếng Việt (giữ nguyên status) để litellm hiển thị
// đúng nguyên nhân; chi tiết gốc của xAI được giữ lại phía sau.
func tierDenied(resp *http.Response) *http.Response {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	message := auth.XAITierDeniedMessage
	if detail := strings.TrimSpace(string(raw)); detail != "" {
		if len(detail) > 300 {
			detail = detail[:300] + "…"
		}
		message += " (xAI: " + detail + ")"
	}
	out, _ := json.Marshal(map[string]any{"error": map[string]any{"code": "xai_oauth_tier_denied", "message": message}})
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header = resp.Header.Clone()
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", fmt.Sprint(len(out)))
	return resp
}

func mergeExtraBody(data []byte, extra map[string]any) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("xai: body request không phải JSON object: %w", err)
	}
	for key, value := range extra {
		body[key] = value
	}
	return json.Marshal(body)
}
