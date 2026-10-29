package bootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/notify"
)

func TestConfigResolveReasoningEffort(t *testing.T) {
	cfg := Config{
		ReasoningEffort: "low", // 顶层默认
		Roles: map[string]RoleConfig{
			"writer":    {Provider: "p", Model: "m", ReasoningEffort: "high"}, // 角色覆盖
			"architect": {Provider: "p", Model: "m"},                          // 无 reasoning_effort，应回落默认
		},
	}

	cases := []struct {
		role string
		want string
	}{
		{"writer", "high"},   // 角色覆盖优先
		{"architect", "low"}, // 角色未配 → 回落顶层默认
		{"editor", "low"},    // 角色不存在 → 顶层默认
		{"", "low"},          // 空 → 顶层默认
		{"default", "low"},   // default → 顶层默认
		{"arbiter", "low"},   // 非配置角色（裁定恒随顶层默认）
	}
	for _, c := range cases {
		if got := cfg.ResolveReasoningEffort(c.role); got != c.want {
			t.Errorf("ResolveReasoningEffort(%q) = %q, want %q", c.role, got, c.want)
		}
	}

	// 顶层默认也为空时，未覆盖角色返回 ""（不覆盖）。
	empty := Config{Roles: map[string]RoleConfig{"writer": {ReasoningEffort: "xhigh"}}}
	if got := empty.ResolveReasoningEffort("editor"); got != "" {
		t.Errorf("空默认下 editor 应返回 \"\"，得 %q", got)
	}
	if got := empty.ResolveReasoningEffort("writer"); got != "xhigh" {
		t.Errorf("空默认下 writer 覆盖应生效，得 %q", got)
	}
}

func TestValidateBaseRejectsNonConfigurableRoles(t *testing.T) {
	for _, role := range []string{"coordinator", "arbiter"} {
		t.Run(role, func(t *testing.T) {
			cfg := Config{
				Provider:  "openrouter",
				ModelName: "test-model",
				Providers: map[string]ProviderConfig{
					"openrouter": {APIKey: "sk-test-123456"},
				},
				Roles: map[string]RoleConfig{
					role: {Provider: "openrouter", Model: "test-model"},
				},
			}

			err := cfg.ValidateBase()
			if err == nil {
				t.Fatalf("roles.%s 应被拒绝", role)
			}
			if !errors.Is(err, errs.ErrConfig) {
				t.Fatalf("应包装 errs.ErrConfig，得到: %v", err)
			}
		})
	}
}

func TestValidateBaseNotifyEventsMatchRuntimeContract(t *testing.T) {
	validConfig := func(events []string) Config {
		return Config{
			Provider:  "openrouter",
			ModelName: "test-model",
			Providers: map[string]ProviderConfig{
				"openrouter": {APIKey: "sk-test-123456"},
			},
			Notify: NotifyConfig{Events: events},
		}
	}

	cfg := validConfig(notify.Kinds())
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("当前通知事件契约应全部通过配置校验: %v", err)
	}

	cfg = validConfig([]string{"repeat"})
	if err := cfg.ValidateBase(); !errors.Is(err, errs.ErrConfig) {
		t.Fatalf("旧 repeat 事件应被拒绝，得到: %v", err)
	}
}

func TestValidateBaseAcceptsCodexSubscriptionWithoutAPIKey(t *testing.T) {
	cfg := Config{
		Provider:  "openai-codex",
		ModelName: "gpt-5.6-terra",
		Providers: map[string]ProviderConfig{
			"openai-codex": {},
			"chatgpt":      {Type: "openai-codex", BaseURL: "https://proxy.example/codex"},
		},
		Roles: map[string]RoleConfig{"writer": {Provider: "chatgpt", Model: "gpt-6-sol"}},
	}
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("订阅 provider 不需要 api_key: %v", err)
	}
	// 凭证在请求时才读取：未登录也能构建模型，错误延迟到调用并提示 login。
	t.Setenv("HOME", t.TempDir())
	if _, err := NewModelSet(cfg); err != nil {
		t.Fatalf("NewModelSet: %v", err)
	}
	if !cfg.Providers["chatgpt"].UsesOAuth("chatgpt") || cfg.Providers["openai-codex"].RequiresAPIKey("openai-codex") {
		t.Fatal("type=openai-codex 的自定义名 provider 也应走 OAuth")
	}

	cfg.Providers["openai-codex"] = ProviderConfig{API: "chat"}
	if err := cfg.ValidateBase(); !errors.Is(err, errs.ErrConfig) {
		t.Fatalf("openai-codex 固定 Responses 协议，api 配置应被拒绝，得到: %v", err)
	}
}

func TestProviderStreamIdleTimeoutValue(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", defaultStreamIdleTimeout, false},
		{"900s", 15 * time.Minute, false},
		{"15m", 15 * time.Minute, false},
		{"abc", 0, true},
		{"-5s", 0, true},
		{"0", 0, true}, // 不提供"关闭看门狗"——真死流需要有限界
	}
	for _, c := range cases {
		got, err := ProviderConfig{StreamIdleTimeout: c.in}.StreamIdleTimeoutValue()
		if c.wantErr {
			if err == nil {
				t.Errorf("%q 应报错", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%q = (%v, %v), want %v", c.in, got, err, c.want)
		}
	}
}

func TestValidateBaseRejectsBadStreamIdleTimeout(t *testing.T) {
	cfg := Config{
		Provider:  "openrouter",
		ModelName: "test-model",
		Providers: map[string]ProviderConfig{
			"openrouter": {APIKey: "sk-test-123456", StreamIdleTimeout: "fast"},
		},
	}
	if err := cfg.ValidateBase(); !errors.Is(err, errs.ErrConfig) {
		t.Fatalf("非法 stream_idle_timeout 应拒绝并包装 ErrConfig，得到: %v", err)
	}
}

func TestXAIOAuthProviderIsSubscriptionWithoutAPIKey(t *testing.T) {
	cfg := Config{
		Provider:  "xai-oauth",
		ModelName: "grok-4.6",
		Providers: map[string]ProviderConfig{
			"xai-oauth": {},
			"supergrok": {Type: "xai-oauth"},
			"grok":      {APIKey: "xai-key"},
		},
		Roles: map[string]RoleConfig{"writer": {Provider: "supergrok", Model: "grok-4.3"}},
	}
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("xai-oauth không cần api_key: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	if _, err := NewModelSet(cfg); err != nil {
		t.Fatalf("NewModelSet: %v", err)
	}
	for _, name := range []string{"xai-oauth", "supergrok"} {
		pc := cfg.Providers[name]
		if !pc.UsesOAuth(name) || pc.RequiresAPIKey(name) || pc.OAuthID(name) != "xai-oauth" {
			t.Errorf("%s phải là provider OAuth xai-oauth", name)
		}
	}
	// "grok" vẫn là provider API key, không bị lẫn với xai-oauth.
	if pc := cfg.Providers["grok"]; pc.UsesOAuth("grok") || !pc.RequiresAPIKey("grok") || pc.OAuthID("grok") != "" {
		t.Error("grok là provider API key")
	}

	// Bearer OAuth không được gửi tới máy chủ ngoài x.ai; api (chat/responses) không cấu hình được.
	for _, pc := range []ProviderConfig{{BaseURL: "https://api.openai.com/v1"}, {BaseURL: "http://api.x.ai/v1"}, {API: "chat"}} {
		bad := cfg
		bad.Providers = map[string]ProviderConfig{"xai-oauth": pc}
		bad.Roles = nil
		if _, err := NewModelSet(bad); err == nil {
			t.Errorf("xai-oauth %+v phải bị từ chối", pc)
		}
	}
}

func TestAnthropicOAuthProviderIsSubscriptionWithoutAPIKey(t *testing.T) {
	cfg := Config{
		Provider:  "anthropic-oauth",
		ModelName: "claude-sonnet-4-6",
		Providers: map[string]ProviderConfig{
			"anthropic-oauth": {},
			"claude-max":      {Type: "anthropic-oauth"},
			"anthropic":       {APIKey: "sk-ant-key"},
		},
		Roles: map[string]RoleConfig{"writer": {Provider: "claude-max", Model: "claude-opus-4-8"}},
	}
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("anthropic-oauth không cần api_key: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	if _, err := NewModelSet(cfg); err != nil {
		t.Fatalf("NewModelSet: %v", err)
	}
	for _, name := range []string{"anthropic-oauth", "claude-max"} {
		pc := cfg.Providers[name]
		typ, err := pc.ProviderType(name)
		if err != nil || typ != "anthropic-oauth" || !pc.UsesOAuth(name) || pc.RequiresAPIKey(name) || pc.OAuthID(name) != "anthropic-oauth" {
			t.Errorf("%s phải là provider OAuth anthropic-oauth (type=%q err=%v)", name, typ, err)
		}
	}
	// "anthropic" (API key) giữ nguyên, không lẫn với anthropic-oauth.
	if pc := cfg.Providers["anthropic"]; pc.UsesOAuth("anthropic") || !pc.RequiresAPIKey("anthropic") || pc.OAuthID("anthropic") != "" {
		t.Error("anthropic là provider API key")
	}

	// Bearer OAuth không được gửi tới máy chủ ngoài anthropic.com; api không cấu hình được.
	for _, pc := range []ProviderConfig{{BaseURL: "https://api.openai.com/v1"}, {BaseURL: "http://api.anthropic.com"}, {API: "chat"}} {
		bad := cfg
		bad.Providers = map[string]ProviderConfig{"anthropic-oauth": pc}
		bad.Roles = nil
		if _, err := NewModelSet(bad); err == nil {
			t.Errorf("anthropic-oauth %+v phải bị từ chối", pc)
		}
	}
}
