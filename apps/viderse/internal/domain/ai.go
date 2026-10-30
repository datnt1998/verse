package domain

// Kiểu giá trị của mảng kết nối AI (hợp đồng dữ liệu §15): nhà cung cấp,
// danh mục model và gán model cho trợ lý. Chỉ kiểu + hằng — không I/O.

import "time"

// Capability là một việc model làm được.
type Capability = string

const (
	CapText   Capability = "text"
	CapVision Capability = "vision"
	CapImage  Capability = "image"
)

// ProviderKind là loại kết nối của một nhà cung cấp.
type ProviderKind = string

const (
	KindChatGPT    ProviderKind = "chatgpt" // gói ChatGPT (Codex subscription)
	KindGrok       ProviderKind = "grok"    // gói SuperGrok / X Premium+ (xAI OAuth)
	KindOpenAI     ProviderKind = "openai"
	KindGemini     ProviderKind = "gemini"
	KindOpenRouter ProviderKind = "openrouter"

	KindDeepSeek     ProviderKind = "deepseek"
	KindXAI          ProviderKind = "xai"
	KindAnthropic    ProviderKind = "anthropic"
	KindOllama       ProviderKind = "ollama"
	KindOpenAICompat ProviderKind = "openai_compat"
)

// Quota là hạn mức đã biết của một thẻ đăng nhập gói — nguồn duy nhất hiện
// có là trần ảnh trả phí ngày (D7): used = ảnh đã charge hôm nay.
type Quota struct {
	Used    int64
	Limit   int64
	Period  string // "day" | "week"
	ResetAt time.Time
}

// AIAuth là cách một nhà cung cấp xác thực.
type AIAuth = string

const (
	AuthSubscription AIAuth = "subscription"
	AuthAPIKey       AIAuth = "api_key"
	AuthLocal        AIAuth = "local"
)

// AIStatus là trạng thái hiện trên thẻ nhà cung cấp.
type AIStatus = string

const (
	StatusReady        AIStatus = "ready"
	StatusLoggedOut    AIStatus = "logged_out"
	StatusLoginPending AIStatus = "login_pending"
	StatusAuthFailed   AIStatus = "auth_failed"
	StatusUnreachable  AIStatus = "unreachable"
)

// Mã reason trong body lỗi của API kết nối AI (theo §15; các mã mở rộng
// được ghi trong report của aiconn).
const (
	ReasonAuthFailed        = "auth_failed"
	ReasonInUse             = "in_use"
	ReasonCapabilityMissing = "capability_missing"
	ReasonUnreachable       = "unreachable"
	ReasonInvalidKind       = "invalid_kind"
	ReasonManaged           = "managed"
	ReasonMissingKey        = "missing_key"
	ReasonMissingBaseURL    = "missing_base_url"
	ReasonInvalidBaseURL    = "invalid_base_url"
	ReasonUnknownModel      = "unknown_model"
	ReasonNoExecutor        = "no_executor" // model hợp lệ nhưng không có đường chạy cho vai trò
	ReasonLoginInProgress   = "login_in_progress"
	ReasonNotSubscription   = "not_subscription"
	ReasonInvalidMode       = "invalid_mode"
	ReasonInvalidCode       = "invalid_code"
)

// Provider là một dòng ai_providers — khoá API đã được mã hoá (EncKey), chỉ
// tầng persistence/service nhìn thấy; tầng web không bao giờ nhận EncKey.
type Provider struct {
	ID        string
	Kind      ProviderKind
	Auth      AIAuth
	Label     string
	EncKey    []byte // ciphertext secretbox; rỗng với local/subscription
	KeyHint   string // "••••7c2e"
	BaseURL   string // ollama, openai_compat
	Status    AIStatus
	CatalogAt int64 // Unix mili-giây lần lấy danh mục gần nhất; 0 = chưa lấy
	Selected  []string
	CreatedAt int64
	UpdatedAt int64
}

// AIModel là một model trong danh mục đã lưu của một nhà cung cấp.
type AIModel struct {
	ID          string
	Caps        []Capability
	ContextK    int
	InPerM      float64 // USD / 1M token vào
	OutPerM     float64 // USD / 1M token ra
	PerImage    float64 // USD / ảnh
	Recommended bool
	Manual      bool // thêm tay qua models/check — sống sót qua lần lấy danh mục
}

// ModelRef trỏ một model cụ thể trên một nhà cung cấp.
type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// AgentModels gán model cho bốn trợ lý (Trinh sát và Kiểm định không dùng
// model). Đạo diễn cần text (+vision cho metadata có ảnh); Hoạ sĩ cần vision
// để viết mô tả ảnh và image để vẽ, có chuỗi dự phòng.
type AgentModels struct {
	Director DirectorModels `json:"director"`
	Artist   ArtistModels   `json:"artist"`
}

// DirectorModels là lựa chọn model của Đạo diễn.
type DirectorModels struct {
	Model    ModelRef   `json:"model"`
	Effort   string     `json:"effort"` // auto | low | medium | high
	Fallback []ModelRef `json:"fallback"`
}

// ArtistModels là lựa chọn model của Hoạ sĩ.
type ArtistModels struct {
	PromptModel  ModelRef   `json:"promptModel"`
	Draw         ModelRef   `json:"draw"`
	FallbackOn   bool       `json:"fallbackOn"`
	DrawFallback []ModelRef `json:"drawFallback"`
}

// KeyAIAgents là khoá settings lưu AgentModels (JSON).
const KeyAIAgents = "ai.agents"

// DefaultAgentModels là gán mặc định theo "Đối chiếu code hiện tại" của §15:
// Đạo diễn gpt-6.1-sol (text+vision), Hoạ sĩ viết mô tả cũng gpt-6.1-sol, vẽ
// bằng gpt-image-2; dự phòng vẽ mặc định là nguồn còn lại (Grok).
func DefaultAgentModels() AgentModels {
	return AgentModels{
		Director: DirectorModels{
			Model:    ModelRef{Provider: "chatgpt", Model: "gpt-6.1-sol"},
			Effort:   "auto",
			Fallback: []ModelRef{},
		},
		Artist: ArtistModels{
			PromptModel:  ModelRef{Provider: "chatgpt", Model: "gpt-6.1-sol"},
			Draw:         ModelRef{Provider: "chatgpt", Model: "gpt-image-2"},
			FallbackOn:   true,
			DrawFallback: []ModelRef{{Provider: "grok", Model: "grok-imagine-image"}},
		},
	}
}
