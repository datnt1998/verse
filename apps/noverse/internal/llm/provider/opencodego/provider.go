// Package opencodego 接入 OpenCode Go 订阅（API key 鉴权）。所有文本模型都能走
// OpenAI Chat Completions 端点，网关按模型翻译到上游原生协议。
package opencodego

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/provider/compat"
	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

const (
	ProviderName   = "opencode-go"
	defaultBaseURL = "https://opencode.ai/inference/go/v1"

	// sessionHeader 是网关的路由与缓存亲和键；缺少时推理请求直接 400 MissingSessionID。
	sessionHeader = "x-opencode-session"

	ProviderOptionSessionID      = "session_id"
	ProviderOptionPromptCacheKey = "prompt_cache_key"
)

type Config = compat.Config

var allowedProviderOptions = map[string]struct{}{
	ProviderOptionSessionID:      {},
	ProviderOptionPromptCacheKey: {},
}

var reasoningEfforts = []string{"minimal", "low", "medium", "high", "xhigh", "max"}

func New(cfg Config) (*compat.Provider, error) {
	fallbackSession, err := newSessionID()
	if err != nil {
		return nil, err
	}
	return compat.New(cfg, compat.Spec{
		Name: ProviderName,
		Endpoint: compat.EndpointSpec{
			BaseURL: defaultBaseURL,
		},
		Auth: compat.AuthSpec{APIKeyRequired: true},
		Headers: compat.HeaderSpec{
			Request: func(headers http.Header, req *litellm.Request) {
				headers.Set(sessionHeader, sessionID(req, fallbackSession))
			},
		},
		Request: compat.RequestSpec{
			SupportsJSONSchema:     true,
			Thinking:               mapThinking,
			ProviderOptions:        mapProviderOptions,
			AllowedProviderOptions: allowedProviderOptions,
		},
		Response: compat.ResponseSpec{
			ModelFromResponse:         true,
			ReasoningFields:           []string{"reasoning_content"},
			HasCompletionTokenDetails: true,
		},
		Stream: compat.StreamSpec{
			ReasoningFields: []string{"reasoning_content"},
		},
		Capabilities: func(_ string, caps litellm.Capabilities) litellm.Capabilities {
			// Kimi K2.7 Code 开思考时拒绝强制 tool_choice；Kimi、GLM 不允许关闭思考。
			caps.Tools.Choice = litellm.SupportPartial
			caps.Thinking.Disable = litellm.SupportPartial
			caps.Thinking.Efforts = append([]string(nil), reasoningEfforts...)
			caps.Thinking.BudgetTokens = litellm.SupportNo
			caps.Thinking.Notes = []string{"per-model effort and disable limits are enforced by the OpenCode Go API"}
			// 网关逐模型翻译 response_format；默认走提示词契约，需要原生 schema 时在配置里显式开启。
			caps.Structured.JSONSchema = litellm.SupportPartial
			caps.Cache.PromptKey = litellm.SupportYes
			return caps
		},
	})
}

func Factory(cfg Config) (litellm.Provider, error) {
	return New(cfg)
}

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("%s: session id: %w", ProviderName, err)
	}
	return "noverse-" + hex.EncodeToString(b[:]), nil
}

// sessionID 优先用调用方的会话/缓存键，让同一 agent 的多轮请求落到同一路由；
// 一次性调用退回本 provider 实例的固定 ID。
func sessionID(req *litellm.Request, fallback string) string {
	for _, key := range []string{ProviderOptionSessionID, ProviderOptionPromptCacheKey} {
		if value, ok := req.ProviderOptions[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fallback
}

func mapThinking(thinking *litellm.Thinking, _ string) (map[string]any, error) {
	if thinking == nil || thinking.Mode == litellm.ThinkingUnspecified {
		return nil, nil
	}
	switch thinking.Mode {
	case litellm.ThinkingDisabled:
		return map[string]any{"reasoning_effort": "none"}, nil
	case litellm.ThinkingEnabled:
		if thinking.BudgetTokens != nil {
			return nil, fmt.Errorf("%s: thinking budget_tokens is not supported; use effort", ProviderName)
		}
		effort := strings.ToLower(strings.TrimSpace(thinking.Effort))
		if effort == "" {
			return nil, nil
		}
		for _, allowed := range reasoningEfforts {
			if effort == allowed {
				return map[string]any{"reasoning_effort": effort}, nil
			}
		}
		return nil, fmt.Errorf("%s: unsupported reasoning effort %q", ProviderName, thinking.Effort)
	default:
		return nil, fmt.Errorf("%s: unsupported thinking mode %d", ProviderName, thinking.Mode)
	}
}

// mapProviderOptions 只校验会话类选项（它们走请求头，不进 body）。MiniMax 经 Chat
// Completions 默认把思考以 <think> 混进正文，reasoning_split 让网关拆到 reasoning_content。
func mapProviderOptions(options litellm.ProviderOptions, body map[string]any, req *litellm.Request) error {
	for key, value := range options {
		switch key {
		case ProviderOptionSessionID, ProviderOptionPromptCacheKey:
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s: provider option %q must be string", ProviderName, key)
			}
		default:
			return fmt.Errorf("%s: unsupported provider option %q", ProviderName, key)
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Model)), "minimax-") {
		body["reasoning_split"] = true
	}
	return nil
}
