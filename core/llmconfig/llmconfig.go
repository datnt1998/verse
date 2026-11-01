// Package llmconfig chứa phần dùng chung của cấu hình provider/model: mỗi app
// có một bản đồ provider (tên → ProviderConfig) trong file cấu hình riêng, và
// gói này dựng llm.ChatModel từ nó — gồm cả các provider đăng nhập bằng OAuth
// subscription (openai-codex, xai-oauth, anthropic-oauth) đọc credential từ
// kho của app qua core/auth.
package llmconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	llmtypes "github.com/datnt1998/verse/core/llm"

	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/auth/anthropicoauth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	"github.com/datnt1998/verse/core/llm"
)

var (
	// ErrConfig marks invalid user configuration (missing/wrong provider,
	// malformed durations). Apps re-export or wrap it so callers can classify.
	ErrConfig = errors.New("lỗi cấu hình")
	// ErrProvider marks a failure building the provider client itself.
	ErrProvider = errors.New("lỗi khởi tạo nhà cung cấp")
)

// ProviderConfig 定义单个 LLM 提供商的凭证。
type ProviderConfig struct {
	Type    string        `json:"type,omitempty"`     // API 协议类型（openai/anthropic/gemini），自定义代理时指定
	API     string        `json:"api,omitempty"`      // OpenAI 协议 endpoint：chat（默认）/ responses
	APIKey  string        `json:"api_key,omitempty"`  // API Key
	BaseURL string        `json:"base_url,omitempty"` // API Base URL
	Models  []ModelConfig `json:"models,omitempty"`   // 可选模型列表，供 TUI 切换时展示
	// ExtraBody 透传给该 provider 每次请求的额外参数（如 temperature/top_p/min_p/
	// presence_penalty，或厂商特有键如 nvidia 开 think 的 chat_template_kwargs）。
	// OpenAI 兼容端逐字并入请求体（即 extra_body 约定）；值由用户自负其责。
	ExtraBody map[string]any `json:"extra_body,omitempty"`
	// Extra 透传给 provider 级配置（litellm.ProviderConfig.Extra），用于 HTTP
	// headers、user_agent、anthropic_beta 等客户端/传输层选项。
	Extra map[string]any `json:"extra,omitempty"`
	// StreamIdleTimeout 流式空闲看门狗：超过该时长收不到任何 chunk 即断流
	// （Go duration 字符串，如 "900s" / "15m"）。留空默认 5m——云端服务的合理上界；
	// LocalAI/ollama 等自建慢推理首块可远超 5 分钟，按 provider 放宽即可，
	// 不拖累其它通道的挂死检测（#79）。
	StreamIdleTimeout string `json:"stream_idle_timeout,omitempty"`
}

// ModelConfig 描述某个 provider 下可切换的模型及其可选上下文窗口。
// 为兼容旧配置，既可从 JSON 字符串（"model-name"）读取，也可从对象读取；
// 写回时始终规范化为对象形式。
type ModelConfig struct {
	Name          string `json:"name"`
	ContextWindow int    `json:"context_window,omitempty"`
	// JSONSchema 是原生结构化输出（response_format json_schema）的三态声明：
	// 未配置=按 provider adapter 的模型级能力判断；true=用户声明该 endpoint/模型
	// 支持（请求被拒绝时原样暴露，不静默降级）；false=强制走 prompt contract。
	// 自定义代理与聚合网关的能力以用户声明为准，程序不探测。
	JSONSchema *bool `json:"json_schema,omitempty"`
}

func (m *ModelConfig) UnmarshalJSON(data []byte) error {
	var legacy string
	if err := json.Unmarshal(data, &legacy); err == nil {
		m.Name = legacy
		m.ContextWindow = 0
		m.JSONSchema = nil
		return nil
	}
	type modelConfigAlias ModelConfig
	var decoded modelConfigAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("model phải là chuỗi hoặc đối tượng {name, context_window}: %w", err)
	}
	*m = ModelConfig(decoded)
	return nil
}

// ModelConfig 返回指定模型的显式配置。
func (pc ProviderConfig) ModelConfig(name string) (ModelConfig, bool) {
	name = strings.TrimSpace(name)
	for _, model := range pc.Models {
		if strings.TrimSpace(model.Name) == name {
			return model, true
		}
	}
	return ModelConfig{}, false
}

// defaultStreamIdleTimeout：长输出 + 长 ctx 场景下，reasoning-aware provider
// （mimo / deepseek-r1 等）思考阶段如果 server 端不流式发 reasoning delta，
// SSE 整段会保持沉默。litellm 默认 watchdog 是 2 分钟，对长输出写作经常
// 触发误杀；5 分钟覆盖绝大多数实测案例。
const defaultStreamIdleTimeout = 5 * time.Minute

// StreamIdleTimeoutValue 解析该 provider 的流式空闲超时；留空回落默认值。
func (pc ProviderConfig) StreamIdleTimeoutValue() (time.Duration, error) {
	s := strings.TrimSpace(pc.StreamIdleTimeout)
	if s == "" {
		return defaultStreamIdleTimeout, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("thời lượng %q không hợp lệ (dùng dạng Go như \"900s\" / \"15m\")", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("phải lớn hơn 0, nhận %q", s)
	}
	return d, nil
}

// RequiresAPIKey 返回该 provider 是否必须显式配置 api_key。
// 约定：
// 1. ollama / bedrock 允许无 key；openai-codex / xai-oauth / anthropic-oauth 走 OAuth 登录（auth.json），不用 key；
// 2. 显式指定 Type 的配置视为自定义代理，允许无 key；
// 3. 其他 provider 默认要求 key，保持对官方托管接口的保守校验。
func (pc ProviderConfig) RequiresAPIKey(name string) bool {
	switch name {
	case "ollama", "bedrock", codex.ProviderName, xai.ProviderName, anthropicoauth.ProviderName:
		return false
	}
	return pc.Type == ""
}

// UsesOAuth 返回该 provider 是否以订阅登录（OAuth）而非 api_key 鉴权。
func (pc ProviderConfig) UsesOAuth(name string) bool {
	return pc.OAuthID(name) != ""
}

// OAuthID 返回订阅 provider 在 auth.json 里的凭证 ID（即 provider 类型）；
// 非订阅 provider 返回 ""。自定义名只要 type 是订阅类型就共用同一份凭证。
func (pc ProviderConfig) OAuthID(name string) string {
	providerType, err := pc.ProviderType(name)
	if err != nil {
		return ""
	}
	switch providerType {
	case codex.ProviderName, xai.ProviderName, anthropicoauth.ProviderName:
		return providerType
	}
	return ""
}

// ProviderType 返回有效的 API 协议类型。
// 优先使用显式 Type；否则要求 provider 名本身已在 litellm 注册表中，
// 或是内置的订阅 provider（openai-codex、xai-oauth、anthropic-oauth）。
func (pc ProviderConfig) ProviderType(name string) (string, error) {
	if pc.Type != "" {
		return pc.Type, nil
	}
	if name == codex.ProviderName || name == xai.ProviderName || name == anthropicoauth.ProviderName || llm.IsProviderRegistered(name) {
		return name, nil
	}
	return "", fmt.Errorf("provider %q thiếu type và không nằm trong danh sách provider đã biết của litellm: %w", name, ErrConfig)
}

// NewModel 创建或复用 ChatModel 实例（cache 可为 nil，按 provider|model 去重）。
func NewModel(providerKey, model string, pc ProviderConfig, cache map[string]llmtypes.ChatModel) (llmtypes.ChatModel, error) {
	if cache != nil {
		if m, ok := cache[providerKey+"|"+model]; ok {
			return m, nil
		}
	}

	providerType, err := pc.ProviderType(providerKey)
	if err != nil {
		return nil, fmt.Errorf("phân tích loại provider thất bại: %w", err)
	}
	providerExtra := cloneMap(pc.Extra)
	if pc.API != "" {
		if providerExtra == nil {
			providerExtra = make(map[string]any, 1)
		}
		providerExtra["api"] = pc.API
	}

	streamIdle, err := pc.StreamIdleTimeoutValue()
	if err != nil {
		return nil, fmt.Errorf("provider %s: stream_idle_timeout: %w: %w", providerKey, ErrConfig, err)
	}

	var m llmtypes.ChatModel
	switch providerType {
	case codex.ProviderName:
		m, err = newCodexModel(model, pc, providerExtra, streamIdle)
	case xai.ProviderName:
		m, err = newXAIModel(model, pc, providerExtra, streamIdle)
	case anthropicoauth.ProviderName:
		m, err = newAnthropicOAuthModel(model, pc, providerExtra, streamIdle)
	default:
		m, err = llmtypes.NewModel(providerType, model,
			llmtypes.WithAPIKey(pc.APIKey),
			llmtypes.WithBaseURL(pc.BaseURL),
			llmtypes.WithStreamIdleTimeout(streamIdle),
			llmtypes.WithProviderExtra(providerExtra),
			llmtypes.WithExtra(pc.ExtraBody),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("provider %s (%s): %w: %w", providerKey, providerType, ErrProvider, err)
	}
	if cache != nil {
		cache[providerKey+"|"+model] = m
	}
	return m, nil
}

// newCodexModel 创建 ChatGPT 订阅（Codex 后端）模型：凭证取自 app 的 auth.json
// （由 `<app> login openai-codex` 写入，auth.Options.StoreDir 决定目录），请求时按需刷新。
func newCodexModel(model string, pc ProviderConfig, extra map[string]any, streamIdle time.Duration) (llmtypes.ChatModel, error) {
	if pc.API != "" && pc.API != "responses" {
		return nil, fmt.Errorf("openai-codex luôn dùng giao thức Responses, api chỉ có thể là \"responses\" hoặc để trống")
	}
	if err := codex.ValidateBaseURL(pc.BaseURL); err != nil {
		return nil, err
	}
	manager, err := auth.DefaultManager()
	if err != nil {
		return nil, err
	}
	headers, err := stringMapFromExtra(extra, "headers")
	if err != nil {
		return nil, err
	}
	return codex.NewChatModel(model, codex.Config{
		BaseURL:           pc.BaseURL,
		Tokens:            codex.ManagerTokens(manager),
		Headers:           headers,
		ExtraBody:         pc.ExtraBody,
		StreamIdleTimeout: streamIdle,
	})
}

// newXAIModel 创建 SuperGrok / X Premium+ 订阅模型：凭证来源与 codex 相同。
func newXAIModel(model string, pc ProviderConfig, extra map[string]any, streamIdle time.Duration) (llmtypes.ChatModel, error) {
	if pc.API != "" {
		return nil, fmt.Errorf("xai-oauth chỉ dùng giao thức Chat Completions, không hỗ trợ cấu hình api")
	}
	if err := xai.ValidateBaseURL(pc.BaseURL); err != nil {
		return nil, err
	}
	manager, err := auth.DefaultManager()
	if err != nil {
		return nil, err
	}
	headers, err := stringMapFromExtra(extra, "headers")
	if err != nil {
		return nil, err
	}
	return xai.NewChatModel(model, xai.Config{
		BaseURL:           pc.BaseURL,
		Tokens:            xai.ManagerTokens(manager),
		Headers:           headers,
		ExtraBody:         pc.ExtraBody,
		StreamIdleTimeout: streamIdle,
	})
}

// newAnthropicOAuthModel 创建 Claude Pro/Max 订阅模型：凭证来源与 codex 相同。
func newAnthropicOAuthModel(model string, pc ProviderConfig, extra map[string]any, streamIdle time.Duration) (llmtypes.ChatModel, error) {
	if pc.API != "" {
		return nil, fmt.Errorf("anthropic-oauth chỉ dùng giao thức Messages, không hỗ trợ cấu hình api")
	}
	if err := anthropicoauth.ValidateBaseURL(pc.BaseURL); err != nil {
		return nil, err
	}
	manager, err := auth.DefaultManager()
	if err != nil {
		return nil, err
	}
	headers, err := stringMapFromExtra(extra, "headers")
	if err != nil {
		return nil, err
	}
	return anthropicoauth.NewChatModel(model, anthropicoauth.Config{
		BaseURL:           pc.BaseURL,
		Tokens:            anthropicoauth.ManagerTokens(manager),
		Headers:           headers,
		ExtraBody:         pc.ExtraBody,
		StreamIdleTimeout: streamIdle,
	})
}

func stringMapFromExtra(extra map[string]any, key string) (map[string]string, error) {
	raw, ok := extra[key].(map[string]any)
	if !ok {
		if extra[key] == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("extra.%s phải là đối tượng khóa-giá trị chuỗi", key)
	}
	out := make(map[string]string, len(raw))
	for name, value := range raw {
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("extra.%s.%s phải là chuỗi", key, name)
		}
		out[name] = s
	}
	return out, nil
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
