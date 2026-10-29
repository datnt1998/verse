// Package anthropicoauth là lớp wire cho API Anthropic Messages khi xác thực bằng gói thuê bao
// Claude Pro/Max qua OAuth thay cho API key.
//
// Tái dùng nguyên provider "anthropic" của litellm (Messages API, thinking, tools, cache…) và
// chỉ thêm một http.RoundTripper làm đúng những gì omp (oh-my-pi) áp lên nhánh OAuth để
// Anthropic chấp nhận request từ token thuê bao:
//   - Authorization: Bearer <access token> lấy từ auth.Manager (tự làm mới trước hạn), bỏ
//     x-api-key; header/UA/Stainless của Claude Code, bộ anthropic-beta, `?beta=true`;
//   - viết lại body: khối system billing (chữ ký cch) + nhận dạng Claude Code, tiền tố `_` cho
//     tên tool, tools:[] khi trống, metadata.user_id, cache 1 giờ;
//   - gỡ tiền tố tên tool khỏi phản hồi, kể cả SSE;
//   - 401 → làm mới ép buộc một lần rồi gửi lại; 403/429 → thông báo tiếng Việt.
package anthropicoauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/auth"
	"github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/llm/provider/anthropic"
	"github.com/datnt1998/noverse/internal/llm/wire"
)

const (
	// ProviderName là tên provider trong config và kho credential.
	ProviderName = auth.AnthropicOAuthID
	// DefaultBaseURL là gốc API Anthropic; endpoint chat là <base>/v1/messages.
	DefaultBaseURL = "https://api.anthropic.com"
)

// TokenSource cấp credential OAuth còn hạn. Invalidate được gọi khi API trả 401 cho token
// stale và phải trả về credential mới (đã refresh).
type TokenSource interface {
	Credential(ctx context.Context) (auth.Credential, error)
	Invalidate(ctx context.Context, stale string) (auth.Credential, error)
	// InstallID là mã cài đặt ổn định (suy ra device_id trong metadata.user_id).
	InstallID() string
}

// ManagerTokens là TokenSource đọc credential "anthropic-oauth" từ auth.Manager.
func ManagerTokens(m *auth.Manager) TokenSource {
	return managerTokens{m: m, dir: filepath.Dir(m.Store().Path())}
}

type managerTokens struct {
	m   *auth.Manager
	dir string
}

func (s managerTokens) Credential(ctx context.Context) (auth.Credential, error) {
	return s.m.Credential(ctx, auth.AnthropicOAuthID)
}

func (s managerTokens) Invalidate(ctx context.Context, stale string) (auth.Credential, error) {
	return s.m.Invalidate(ctx, auth.AnthropicOAuthID, stale)
}

func (s managerTokens) InstallID() string { return installID(s.dir) }

// installID đọc (hoặc tạo) mã cài đặt ngẫu nhiên nằm cạnh kho credential; lỗi ghi/đọc thì
// rơi về hostname để vẫn ổn định trong tiến trình.
func installID(dir string) string {
	path := filepath.Join(dir, "install_id")
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		id := hex.EncodeToString(buf)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			if f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
				_, _ = f.WriteString(id + "\n")
				f.Close()
				return id
			}
			// Tiến trình khác vừa tạo trước: dùng giá trị của nó.
			if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
				return strings.TrimSpace(string(data))
			}
		}
	}
	host, _ := os.Hostname()
	return "host-" + host
}

// ValidateBaseURL chỉ cho phép gửi bearer tới https trên miền anthropic.com: token OAuth chỉ
// có giá trị với API Anthropic, trỏ sang nơi khác sẽ làm lộ credential. Rỗng = mặc định.
func ValidateBaseURL(base string) error {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("base_url %q không hợp lệ", base)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" || (host != "anthropic.com" && !strings.HasSuffix(host, ".anthropic.com")) {
		return fmt.Errorf("base_url %q phải là https trên miền anthropic.com — provider anthropic-oauth không gửi token OAuth tới nơi khác", base)
	}
	return nil
}

// Config cấu hình Provider.
type Config struct {
	// BaseURL ghi đè DefaultBaseURL.
	BaseURL string
	Tokens  TokenSource
	// Headers bổ sung vào mọi request (providers.<name>.extra.headers); header của Claude Code
	// do transport sở hữu nên không bị ghi đè.
	Headers map[string]string
	// ExtraBody được trộn vào body JSON của POST /v1/messages (providers.<name>.extra_body).
	ExtraBody map[string]any
	// StreamIdleTimeout: ngắt stream khi không nhận chunk nào trong khoảng này; 0 = tắt.
	StreamIdleTimeout time.Duration
	// Transport là RoundTripper nền; nil dùng http.DefaultTransport.
	Transport http.RoundTripper
}

// New tạo litellm.Provider (provider "anthropic" của litellm) với transport OAuth.
func New(cfg Config) (*anthropic.Provider, error) {
	if cfg.Tokens == nil {
		return nil, fmt.Errorf("anthropic-oauth: thiếu nguồn token OAuth")
	}
	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return anthropic.New(anthropic.Config{
		// Authorization do transport sở hữu; provider chỉ cần một giá trị khác rỗng.
		APIKey:    "anthropic-oauth",
		BaseURL:   strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		Transport: newTransport(base, cfg.Tokens, cfg.ExtraBody),
		UserAgent: auth.ClaudeCodeUserAgent(),
		Headers:   cfg.Headers,
	})
}

// NewChatModel tạo agentcore ChatModel cho một model Claude (vd. "claude-sonnet-4-6").
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
	// session là UUID phiên Claude Code của transport này (header X-Claude-Code-Session-Id
	// và session_id trong metadata.user_id).
	session string
}

func newTransport(base http.RoundTripper, tokens TokenSource, extraBody map[string]any) *transport {
	return &transport{base: base, tokens: tokens, extraBody: extraBody, session: newSessionID()}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("anthropic-oauth: đọc body request: %w", err)
		}
		body = data
	}
	messages := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/v1/messages")
	if messages && len(t.extraBody) > 0 {
		merged, err := mergeExtraBody(body, t.extraBody)
		if err != nil {
			return nil, err
		}
		body = merged
	}

	ctx := req.Context()
	cred, err := t.tokens.Credential(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.send(req, body, cred, messages)
	if err != nil {
		return nil, err
	}
	refreshed, adopted := false, false
	for {
		if resp.StatusCode == http.StatusUnauthorized && !refreshed {
			// 401: token bị thu hồi/hết hạn sớm — refresh một lần rồi gửi lại.
			refreshed = true
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if cred, err = t.tokens.Invalidate(ctx, cred.Access); err != nil {
				return nil, err
			}
			if resp, err = t.send(req, body, cred, messages); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			// Bản Claude Code ghim đã lỗi thời: học version tối thiểu từ lỗi và thử lại một lần.
			if !adopted && auth.AdoptRequiredClaudeCodeVersion(string(raw)) {
				adopted = true
				if resp, err = t.send(req, body, cred, messages); err != nil {
					return nil, err
				}
				continue
			}
			return apiError(resp, raw), nil
		}
		if messages {
			unprefixResponse(resp)
		}
		return resp, nil
	}
}

// send dựng và gửi một request nhánh OAuth từ body gốc (viết lại body + header theo cred).
func (t *transport) send(orig *http.Request, body []byte, cred auth.Credential, messages bool) (*http.Response, error) {
	req := orig.Clone(orig.Context())
	out := body
	// Request không phải /v1/messages (vd. /v1/models) dùng bộ beta agent đầy đủ như omp.
	shape := requestShape{HasTools: true, Thinking: true, SessionID: t.session}
	if messages {
		var err error
		out, shape, err = rewriteMessages(body, rewriteOptions{AccountID: cred.AccountID, InstallID: t.tokens.InstallID(), SessionID: t.session})
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("beta", "true")
		req.URL.RawQuery = q.Encode()
	}
	if orig.Body != nil {
		req.Body = io.NopCloser(bytes.NewReader(out))
		req.ContentLength = int64(len(out))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(out)), nil }
	}
	applyOAuthHeaders(req.Header, cred.Access, shape.SessionID, claudeCodeBetas(shape))
	return t.base.RoundTrip(req)
}

func mergeExtraBody(data []byte, extra map[string]any) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("anthropic-oauth: body request không phải JSON object: %w", err)
	}
	for key, value := range extra {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("anthropic-oauth: extra_body.%s: %w", key, err)
		}
		body[key] = raw
	}
	return json.Marshal(body)
}

// newSessionID sinh UUID v4 cho phiên Claude Code.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
