// Package codex là lớp wire cho backend ChatGPT Codex (gói thuê bao Plus/Pro),
// tương đương provider "openai-codex-responses" của omp.
//
// Backend Codex nói giao thức OpenAI Responses nhưng có hợp đồng riêng:
//   - xác thực bằng access token OAuth của ChatGPT + header chatgpt-account-id;
//   - luôn stream (stream=true), không lưu (store=false), input phải là danh sách;
//   - từ chối mọi tham số sampling/giới hạn output (temperature, top_p,
//     max_output_tokens…) và cache breakpoint tường minh.
//
// Provider ở đây tái dùng provider OpenAI Responses của litellm và áp hợp đồng
// trên ở hai tầng: chuẩn hoá litellm.Request (trường có kiểu) và một
// http.RoundTripper viết lại body/header (trường chỉ có ở wire).
package codex

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/auth"
	"github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/llm/provider/openai"
	"github.com/datnt1998/noverse/internal/llm/wire"
)

const (
	// ProviderName là tên provider trong config và kho credential.
	ProviderName = "openai-codex"
	// DefaultBaseURL là gốc API của backend Codex; endpoint là <base>/responses.
	DefaultBaseURL = "https://chatgpt.com/backend-api/codex"
	// clientVersion là phiên bản Codex CLI khai báo qua header "version": backend
	// lọc model khả dụng theo giá trị này (đồng bộ với CODEX_CLIENT_VERSION của omp).
	clientVersion = "0.159.0"
	userAgent     = "ainovel-cli"
)

// Token là access token kèm workspace ChatGPT mà nó rút hạn mức.
type Token struct {
	Access    string
	AccountID string
}

// TokenSource cấp token OAuth còn hạn. Invalidate được gọi khi backend trả 401
// cho token stale và phải trả về token mới (đã refresh).
type TokenSource interface {
	Token(ctx context.Context) (Token, error)
	Invalidate(ctx context.Context, stale string) (Token, error)
}

// ManagerTokens là TokenSource đọc credential "openai-codex" từ auth.Manager.
func ManagerTokens(m *auth.Manager) TokenSource { return managerTokens{m: m} }

type managerTokens struct{ m *auth.Manager }

func (s managerTokens) Token(ctx context.Context) (Token, error) {
	cred, err := s.m.Credential(ctx, auth.OpenAICodexID)
	return tokenFrom(cred), err
}

func (s managerTokens) Invalidate(ctx context.Context, stale string) (Token, error) {
	cred, err := s.m.Invalidate(ctx, auth.OpenAICodexID, stale)
	return tokenFrom(cred), err
}

func tokenFrom(cred auth.Credential) Token {
	accountID := cred.AccountID
	if accountID == "" {
		accountID = auth.CodexAccountID(cred.Access)
	}
	return Token{Access: cred.Access, AccountID: accountID}
}

// Config cấu hình Provider.
type Config struct {
	// BaseURL ghi đè DefaultBaseURL (vd. proxy đứng trước backend Codex).
	BaseURL string
	Tokens  TokenSource
	// Headers bổ sung vào mọi request (providers.<name>.extra.headers).
	Headers map[string]string
	// ExtraBody được trộn vào body JSON (providers.<name>.extra_body); tham số
	// Codex từ chối (temperature, top_p…) vẫn bị gỡ sau khi trộn.
	ExtraBody map[string]any
	// StreamIdleTimeout: ngắt stream khi không nhận chunk nào trong khoảng này; 0 = tắt.
	StreamIdleTimeout time.Duration
	// Transport là RoundTripper nền; nil dùng http.DefaultTransport.
	Transport http.RoundTripper
}

// Provider là litellm.Provider cho backend Codex.
type Provider struct {
	inner *openai.Provider
}

// New tạo Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.Tokens == nil {
		return nil, fmt.Errorf("codex: thiếu nguồn token OAuth")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	inner, err := openai.New(openai.Config{
		API: openai.APIResponses,
		// Header Authorization do transport sở hữu (token và account id phải đến
		// từ cùng một lần đọc); provider OpenAI chỉ cần một giá trị khác rỗng.
		APIKey:            "chatgpt-oauth",
		BaseURL:           baseURL,
		Transport:         &transport{base: base, tokens: cfg.Tokens, headers: cfg.Headers, extraBody: cfg.ExtraBody},
		UserAgent:         userAgent,
		PromptCacheParams: true,
	})
	if err != nil {
		return nil, err
	}
	return &Provider{inner: inner}, nil
}

// NewChatModel tạo agentcore ChatModel cho một model Codex (vd. "gpt-5.5").
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

func (p *Provider) Name() string { return ProviderName }

// Chat dựng phản hồi đầy đủ bằng cách gom stream: backend Codex không có chế độ
// non-stream.
func (p *Provider) Chat(ctx context.Context, req *litellm.Request) (*litellm.Response, error) {
	ctx = prepareRequest(ctx, req)
	stream, err := p.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	return litellm.Collect(stream)
}

// Stream delegates Responses normalization to the owned OpenAI provider.
func (p *Provider) Stream(ctx context.Context, req *litellm.Request) (litellm.Stream, error) {
	ctx = prepareRequest(ctx, req)
	stream, err := p.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Capabilities là năng lực của OpenAI Responses, chỉnh theo hợp đồng Codex.
func (p *Provider) Capabilities(model string) litellm.Capabilities {
	caps := p.inner.Capabilities(model)
	caps.Provider = ProviderName
	// Backend chính chủ OpenAI: Structured Outputs (text.format json_schema) được
	// hỗ trợ như API gốc (codex exec --output-schema dùng đúng đường này).
	caps.Structured = litellm.StructuredCapabilities{
		JSONObject: litellm.SupportYes,
		JSONSchema: litellm.SupportYes,
		Strict:     litellm.SupportYes,
	}
	// Cache tự động theo prompt_cache_key/session; breakpoint và retention tường
	// minh không thuộc hợp đồng Codex nên bị gỡ trong prepareRequest.
	caps.Cache.Block = litellm.SupportNo
	caps.Cache.Retention = litellm.SupportNo
	caps.Cache.PromptKey = litellm.SupportYes
	caps.Tools.HostedProviderTools = litellm.SupportNo
	return caps
}

type sessionKey struct{}

// prepareRequest áp phần hợp đồng Codex biểu diễn được trên litellm.Request.
// req là bản sao do litellm.Client tạo cho riêng request này nên sửa tại chỗ an toàn.
func prepareRequest(ctx context.Context, req *litellm.Request) context.Context {
	// Codex từ chối giới hạn output và mọi tham số sampling.
	req.MaxTokens = nil
	req.Temperature = nil
	req.TopP = nil
	req.Stop = nil
	req.Cache = nil
	for i := range req.Messages {
		req.Messages[i].Blocks = stripCache(req.Messages[i].Blocks)
	}

	// session_id không phải tham số body của OpenAI: chuyển thành header phiên
	// (backend suy ra affinity của prompt cache từ đó). Thiếu thì dùng prompt_cache_key.
	session := ""
	if v, ok := req.ProviderOptions["session_id"].(string); ok {
		session = v
		delete(req.ProviderOptions, "session_id")
	}
	if session == "" {
		session, _ = req.ProviderOptions[openai.ProviderOptionPromptCacheKey].(string)
	}
	if session != "" {
		ctx = context.WithValue(ctx, sessionKey{}, session)
	}
	return ctx
}

func sessionFromContext(ctx context.Context) string {
	s, _ := ctx.Value(sessionKey{}).(string)
	return s
}

// stripCache gỡ cache breakpoint tường minh. Khi system message không còn
// breakpoint, litellm gộp nó vào "instructions" thay vì để thành developer item.
func stripCache(blocks []litellm.Block) []litellm.Block {
	for i, block := range blocks {
		switch b := block.(type) {
		case litellm.TextBlock:
			b.Cache = nil
			blocks[i] = b
		case litellm.ImageBlock:
			b.Cache = nil
			blocks[i] = b
		case litellm.ReasoningBlock:
			b.Cache = nil
			blocks[i] = b
		case litellm.ToolUseBlock:
			b.Cache = nil
			blocks[i] = b
		case litellm.ToolResultBlock:
			b.Cache = nil
			b.Content = stripCache(b.Content)
			blocks[i] = b
		case litellm.ToolReferenceBlock:
			b.Cache = nil
			blocks[i] = b
		}
	}
	return blocks
}
