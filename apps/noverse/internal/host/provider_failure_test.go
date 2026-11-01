package host

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	llmtypes "github.com/datnt1998/verse/core/llm"
	litellm "github.com/datnt1998/verse/core/llm/wire"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

func TestClassifyProviderFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct {
		name string
		err  error
		kind string
		wait time.Duration
	}{
		{"402 subscription quota is expired", &litellm.LiteLLMError{StatusCode: 402, Message: "insufficient_quota"}, providerFailureExpired, 0},
		{"401 is key", &litellm.LiteLLMError{StatusCode: 401, Message: "invalid_api_key"}, providerFailureKey, 0},
		{"429 is quota with retry-after", &litellm.LiteLLMError{StatusCode: 429, RetryAfter: 240, Message: "rate limit"}, providerFailureQuota, 240 * time.Second},
		{"quota sentinel is expired, not a transient limit", errors.Join(llmtypes.ErrProviderQuota, errors.New("insufficient_quota")), providerFailureExpired, 0},
		{"rate-limit sentinel is quota", llmtypes.ErrProviderRateLimit, providerFailureQuota, quotaDefaultWait},
		{"auth sentinel is key", llmtypes.ErrProviderAuth, providerFailureKey, 0},
		{"subscription text is expired", fmt.Errorf("provider xai-oauth: 401 Unauthorized: subscription expired"), providerFailureExpired, 0},
		{"invalid key text is key", fmt.Errorf("invalid api key"), providerFailureKey, 0},
		{"usage limit text is quota", fmt.Errorf("usage_limit_reached"), providerFailureQuota, quotaDefaultWait},
		{"retry-after text is parsed", fmt.Errorf("rate limited; retry-after 180"), providerFailureQuota, 180 * time.Second},
		{"unclassified", errors.New("dial tcp: connection refused"), "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyProviderFailure(tc.err)
			if got.Kind != tc.kind {
				t.Fatalf("kind = %q, want %q", got.Kind, tc.kind)
			}
			if tc.wait != 0 && got.RetryAfter != tc.wait {
				t.Fatalf("retryAfter = %v, want %v", got.RetryAfter, tc.wait)
			}
		})
	}
}

// Bug 02/11: gói thuê bao hết hạn (402) và khoá hỏng (401) phải dừng ngay với
// câu Bắt sửa; quota tạm thời (429) không còn bị coi là lỗi vĩnh viễn.
func TestWorkerProviderFailuresPauseWithClassifiedMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct {
		name    string
		err     error
		want    []string
		notWant []string
	}{
		{
			name:    "expired",
			err:     fmt.Errorf("provider xai-oauth: 401 Unauthorized: subscription expired"),
			want:    []string{"SuperGrok", "grok-4.7", "đã hết hạn gói", "Gia hạn rồi đăng nhập lại ở Cài đặt › Kết nối AI", "Trợ lý AI"},
			notWant: []string{"401", "Unauthorized"},
		},
		{
			name: "key",
			err:  &litellm.LiteLLMError{StatusCode: 401, Message: "invalid_api_key"},
			want: []string{"SuperGrok", "khoá không dùng được", "Nhập khoá mới ở Cài đặt › Kết nối AI", "Trợ lý AI"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paused string
			var notices []string
			e := &engine{
				providerOf: func(role string) (string, string) {
					if role != "writer" {
						t.Errorf("unexpected role %q", role)
					}
					return "xai-oauth", "grok-4.7"
				},
				notify:  func(kind, level, title, body string) { notices = append(notices, body) },
				onPause: func(summary string) { paused = summary },
			}
			stop := e.handleWorkerError(context.Background(), &flow.Instruction{Agent: "writer", Task: "viết chương 3"}, tc.err)
			if !stop {
				t.Fatal("lỗi Bắt sửa phải dừng ngay, không thử lại")
			}
			if e.failedKey != "" {
				t.Fatalf("không được ghi nhận lượt thử lại: %q", e.failedKey)
			}
			for _, want := range tc.want {
				if !strings.Contains(paused, want) {
					t.Errorf("câu tạm dừng thiếu %q: %s", want, paused)
				}
			}
			for _, bad := range tc.notWant {
				if strings.Contains(paused, bad) {
					t.Errorf("câu tạm dừng còn chi tiết thô %q: %s", bad, paused)
				}
			}
			if len(notices) != 1 || notices[0] != paused {
				t.Fatalf("thông báo phải kèm cùng câu: %v", notices)
			}
		})
	}
}

func TestQuotaWaitsForRealRetryAt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var events []Event
	var recordedRetry time.Time
	e := &engine{
		providerOf: func(string) (string, string) { return "openai-codex", "gpt-6-luna" },
		emitEvent:  func(ev Event) { events = append(events, ev) },
		recordProviderFailure: func(provider, role, kind, message string, retryAt time.Time) {
			recordedRetry = retryAt
		},
	}
	start := time.Now()
	handled, stop := e.handleProviderFailure(context.Background(),
		&flow.Instruction{Agent: "writer", Task: "viết chương 4"},
		&litellm.LiteLLMError{StatusCode: 429, RetryAfter: 1, Message: "rate limit"})
	if !handled || stop {
		t.Fatalf("quota phải được xử lý và thử lại, handled=%v stop=%v", handled, stop)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("phải chờ retryAt thật, chỉ chờ %v", elapsed)
	}
	if recordedRetry.IsZero() || time.Until(recordedRetry) > time.Second {
		t.Fatalf("retryAt không thật: %v", recordedRetry)
	}
	if len(events) != 1 || events[0].Kind != "provider.retry" || events[0].RetryAt.IsZero() {
		t.Fatalf("thiếu provider.retry có retryAt: %+v", events)
	}
}

func TestQuotaBeyondTwoMinutesFallsBackToDifferentProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var fallbackRole, fallbackProvider, fallbackReason string
	e := &engine{
		providerOf: func(string) (string, string) { return "openai-codex", "gpt-6-luna" },
		emitEvent:  func(Event) {},
		providerFallback: func(role, badProvider, reason string) (fallbackSwap, bool) {
			fallbackRole, fallbackProvider, fallbackReason = role, badProvider, reason
			return fallbackSwap{Role: role, FromProvider: badProvider, ToProvider: "gemini", ToModel: "gemini-2.5-pro"}, true
		},
	}
	start := time.Now()
	handled, stop := e.handleProviderFailure(context.Background(),
		&flow.Instruction{Agent: "writer", Task: "viết chương 5"},
		&litellm.LiteLLMError{StatusCode: 429, RetryAfter: 240, Message: "rate limit"})
	if !handled || stop {
		t.Fatalf("quota >2 phút phải chuyển dự phòng, handled=%v stop=%v", handled, stop)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("không được chờ khi đã có dự phòng")
	}
	if fallbackRole != "writer" || fallbackProvider != "openai-codex" || fallbackReason != providerFailureQuota {
		t.Fatalf("fallback sai: role=%q provider=%q reason=%q", fallbackRole, fallbackProvider, fallbackReason)
	}
}

func TestExpiredWithFallbackKeepsStoryRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	e := &engine{
		providerOf: func(string) (string, string) { return "openai-codex", "gpt-6-luna" },
		providerFallback: func(role, badProvider, reason string) (fallbackSwap, bool) {
			calls++
			return fallbackSwap{Role: role, FromProvider: badProvider, ToProvider: "anthropic-oauth", ToModel: "claude-sonnet-4-6"}, true
		},
		recordProviderFailure: func(string, string, string, string, time.Time) {},
	}
	stop := e.handleWorkerError(context.Background(),
		&flow.Instruction{Agent: "writer", Task: "viết chương 6"},
		&litellm.LiteLLMError{StatusCode: 402, Message: "insufficient_quota"})
	if stop {
		t.Fatal("expired + dự phòng phải chạy tiếp, không dừng")
	}
	if calls != 1 {
		t.Fatalf("fallback gọi %d lần, want 1", calls)
	}
}

// ReloadModelSelection: lựa chọn provider/model dùng chung vừa lưu phải vào
// truyện đang chạy ngay lượt kế tiếp (không cần khởi động lại worker).
func TestReloadModelSelectionAppliesSharedProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldPC := llmconfig.ProviderConfig{Type: "openai", APIKey: "old", BaseURL: "https://old.example/v1", Models: []llmconfig.ModelConfig{{Name: "old-model"}}}
	oldCfg := bootstrap.Config{
		Provider: "old", ModelName: "old-model",
		Providers: map[string]llmconfig.ProviderConfig{"old": oldPC},
	}
	models, err := bootstrap.NewModelSet(oldCfg)
	if err != nil {
		t.Fatal(err)
	}
	newPC := llmconfig.ProviderConfig{Type: "openai", APIKey: "new", BaseURL: "https://new.example/v1", Models: []llmconfig.ModelConfig{{Name: "new-model"}}}
	newCfg := bootstrap.Config{
		Provider: "new", ModelName: "new-model",
		Providers: map[string]llmconfig.ProviderConfig{"new": newPC},
		Roles:     map[string]roles.RoleConfig{"writer": {Provider: "new", Model: "new-model", ReasoningEffort: "high"}},
	}
	h := &Host{cfg: oldCfg, models: models, configLoader: func() (bootstrap.Config, error) { return newCfg, nil }}

	if err := h.ReloadModelSelection(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if provider, model, _ := h.CurrentModelSelection("default"); provider != "new" || model != "new-model" {
		t.Fatalf("model mặc định chưa đổi: %s/%s", provider, model)
	}
	if provider, model, ok := h.CurrentModelSelection("writer"); !ok || provider != "new" || model != "new-model" {
		t.Fatalf("model của vai chưa đổi: %s/%s (%v)", provider, model, ok)
	}
	if h.cfg.Provider != "new" || h.cfg.ModelName != "new-model" {
		t.Fatalf("cấu hình host chưa đổi: %s/%s", h.cfg.Provider, h.cfg.ModelName)
	}

	// Cấu hình mới hỏng (provider chưa có khoá) → giữ nguyên model đang chạy,
	// trả lỗi để gateway báo người dùng thay vì âm thầm dùng tiếp provider cũ.
	h.configLoader = func() (bootstrap.Config, error) {
		return bootstrap.Config{Provider: "missing", ModelName: "m"}, nil
	}
	if err := h.ReloadModelSelection(); err == nil {
		t.Fatal("cấu hình mới không hợp lệ phải trả lỗi")
	}
	if provider, model, _ := h.CurrentModelSelection("default"); provider != "new" || model != "new-model" {
		t.Fatalf("lỗi reload làm hỏng model đang chạy: %s/%s", provider, model)
	}
}

func TestProviderDisplayName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for id, want := range map[string]string{
		"openai-codex":    "ChatGPT",
		"xai-oauth":       "SuperGrok / X Premium+",
		"anthropic-oauth": "Claude Pro/Max",
		"":                "",
		"custom-relay":    "custom-relay",
	} {
		if got := providerDisplayName(id); got != want {
			t.Errorf("providerDisplayName(%q) = %q, want %q", id, got, want)
		}
	}
}
