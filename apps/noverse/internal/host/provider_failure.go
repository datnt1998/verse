package host

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	llmtypes "github.com/datnt1998/verse/core/llm"
	litellm "github.com/datnt1998/verse/core/llm/wire"
)

// providerFailureInfo is the classified result AU3 acts on. The three kinds
// match the settings/writing artboards:
//
//	expired → Bắt sửa (renew/login), key → Bắt sửa (new key),
//	quota → Cảnh báo, wait for RetryAt or fall back when it is beyond 2 minutes.
type providerFailureInfo struct {
	Kind       string
	RetryAfter time.Duration
}

const (
	providerFailureExpired = bootstrap.ProviderFailureExpired
	providerFailureKey     = bootstrap.ProviderFailureKey
	providerFailureQuota   = bootstrap.ProviderFailureQuota

	// quotaFallbackAfter is the owner's threshold: a temporary limit longer
	// than this switches the assistant to a different provider instead of
	// leaving the story idle.
	quotaFallbackAfter = 2 * time.Minute
	// quotaDefaultWait is used when the provider gives no Retry-After hint.
	quotaDefaultWait = time.Minute
)

// ClassifyProviderFailure inspects a worker error and returns the AU3 kind.
// It deliberately reads the structured provider facts (HTTP status / litellm
// type / Retry-After) before falling back to message patterns, so a 402
// subscription quota is never mistaken for a transient 429 limit.
func ClassifyProviderFailure(err error) providerFailureInfo {
	if err == nil {
		return providerFailureInfo{}
	}

	var le *litellm.LiteLLMError
	if errors.As(err, &le) {
		retryAfter := time.Duration(le.RetryAfter) * time.Second
		// Explicit vendor codes outrank proxy HTTP statuses. In particular,
		// OpenAI reports exhausted billing as 429 insufficient_quota.
		switch strings.ToLower(strings.TrimSpace(le.Code)) {
		case "invalid_api_key", "incorrect_api_key", "api_key_expired", "key_expired",
			"token_expired", "expired_token", "invalid_token", "token_revoked", "key_revoked",
			"authentication_error":
			return providerFailureInfo{Kind: providerFailureKey}
		case "insufficient_quota", "billing_hard_limit_reached", "billing_not_active",
			"billing_limit_reached", "subscription_expired", "subscription_ended",
			"payment_required", "credit_balance_too_low", "insufficient_credits":
			return providerFailureInfo{Kind: providerFailureExpired}
		}
		if isBillingExhaustionText(strings.ToLower(le.Message)) {
			return providerFailureInfo{Kind: providerFailureExpired}
		}
		switch {
		case le.StatusCode == 402 || le.Type == litellm.ErrorTypeQuota:
			return providerFailureInfo{Kind: providerFailureExpired}
		case le.StatusCode == 401 || le.StatusCode == 403 || le.Type == litellm.ErrorTypeAuth:
			return providerFailureInfo{Kind: providerFailureKey}
		case le.StatusCode == 429 || le.Type == litellm.ErrorTypeRateLimit:
			if retryAfter <= 0 {
				retryAfter = quotaDefaultWait
			}
			return providerFailureInfo{Kind: providerFailureQuota, RetryAfter: retryAfter}
		}
	}

	// Kernel sentinels (also cover flattened sub-agent JSON strings).
	switch {
	case errors.Is(err, llmtypes.ErrProviderQuota):
		return providerFailureInfo{Kind: providerFailureExpired}
	case errors.Is(err, llmtypes.ErrProviderAuth):
		return providerFailureInfo{Kind: providerFailureKey}
	case errors.Is(err, llmtypes.ErrProviderRateLimit):
		return providerFailureInfo{Kind: providerFailureQuota, RetryAfter: quotaDefaultWait}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case isBillingExhaustionText(msg):
		return providerFailureInfo{Kind: providerFailureExpired}
	case containsAny(msg,
		"invalid api key", "invalid_api_key", "incorrect api key", "unauthorized",
		"authentication failed", "key revoked", "api_key_expired", "token_expired", "expired token"):
		return providerFailureInfo{Kind: providerFailureKey}
	case containsAny(msg,
		"rate limit", "rate_limit", "too many requests", "usage_limit_reached",
		"quota exceeded", "retry-after"):
		retry := parseRetryAfterMessage(msg)
		if retry <= 0 {
			retry = quotaDefaultWait
		}
		return providerFailureInfo{Kind: providerFailureQuota, RetryAfter: retry}
	}
	return providerFailureInfo{}
}

func isBillingExhaustionText(msg string) bool {
	return containsAny(msg, "insufficient_quota", "plan expired", "subscription expired",
		"subscription ended", "payment required", "billing_hard_limit_reached",
		"billing limit", "billing not active", "credit balance is too low",
		"insufficient credits", "gói đã hết hạn", "hết hạn gói")
}

func containsAny(s string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

// parseRetryAfterMessage reads the common "retry-after 240s" rendering that
// provider adapters append to the error text.
func parseRetryAfterMessage(msg string) time.Duration {
	idx := strings.Index(msg, "retry-after")
	if idx < 0 {
		return 0
	}
	rest := strings.TrimSpace(msg[idx+len("retry-after"):])
	rest = strings.TrimLeft(rest, ": ")
	var value int
	if _, err := fmt.Sscanf(rest, "%d", &value); err != nil || value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}

// providerFailureMessage turns a classified failure into what the user can do
// right now. It never repeats raw provider strings.
func providerFailureMessage(provider, model, kind string) string {
	name := providerDisplayName(provider)
	if name == "" {
		name = "Nhà cung cấp AI"
	}
	who := name
	if model != "" {
		who = fmt.Sprintf("%s (%s)", name, model)
	}
	switch kind {
	case providerFailureExpired:
		return fmt.Sprintf("%s đã hết hạn gói. Gia hạn rồi đăng nhập lại ở Cài đặt › Kết nối AI, hoặc chọn nhà cung cấp khác ở Cài đặt › Trợ lý AI, rồi bấm Viết tiếp.", who)
	case providerFailureKey:
		return fmt.Sprintf("%s báo khoá không dùng được. Nhập khoá mới ở Cài đặt › Kết nối AI, hoặc chọn nhà cung cấp khác ở Cài đặt › Trợ lý AI, rồi bấm Viết tiếp.", who)
	case providerFailureQuota:
		return fmt.Sprintf("%s tạm hết lượt. Noverse tự thử lại sau ít phút; không muốn chờ thì chọn nhà cung cấp khác ở Cài đặt › Trợ lý AI.", who)
	default:
		return fmt.Sprintf("%s không phản hồi được. Kiểm tra kết nối rồi bấm Viết tiếp.", who)
	}
}

// providerDisplayName: tên người dùng thấy (“ChatGPT”, “SuperGrok / X Premium+”),
// không phải id kỹ thuật (openai-codex, xai-oauth).
func providerDisplayName(id string) string {
	if id == "" {
		return ""
	}
	for _, preset := range bootstrap.ProviderPresets() {
		if preset.Name != id {
			continue
		}
		if id == "openai-codex" {
			return "ChatGPT"
		}
		if label := strings.TrimSpace(strings.SplitN(preset.Label, " (", 2)[0]); label != "" {
			return label
		}
	}
	return id
}
