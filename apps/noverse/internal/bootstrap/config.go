package bootstrap

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/notify"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
	"github.com/datnt1998/verse/core/auth/anthropicoauth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/models"
	"github.com/datnt1998/verse/core/roles"
)

// DefaultContextWindow 模型未在 registry 登记时的兜底窗口大小。
const DefaultContextWindow = 200000

// CompactRatio 触发上下文压缩的相对阈值：tokens >= window * CompactRatio 时压缩。
// 0.85 是经验值，给"下一轮 prompt + 大工具结果"留 15% 头部空间，同时让大窗口
// 模型也能在 85% 主动压缩，避免在 1M 名义窗口下吃满才压（注意力衰退区）。
//
// 压缩比例不暴露给用户配置；用户只配置每个模型的真实 context_window。
const CompactRatio = 0.85

// MinCompactReserve 是 ReserveTokens 的下限。小窗口模型（如 32k 本地 qwen3:8b）
// 按 0.15 比例算 reserve 仅 4800，单次 commit_chapter 工具响应就能塞 5-8k，
// 一章正文 8-15k——会出现"压完立刻又超"。8000 兜底保证最坏场景下还有半轮缓冲。
const MinCompactReserve = 8000

// CompactReserveTokens 按 CompactRatio 反算 ReserveTokens 并应用 MinCompactReserve floor：
//
//	threshold = window - reserve = window * CompactRatio
//	reserve   = max(MinCompactReserve, window * (1 - CompactRatio))
//
// 给 agentcore.context.Engine 的 EngineConfig.ReserveTokens 用。
func CompactReserveTokens(window int) int {
	if window <= 0 {
		return 0
	}
	reserve := window - int(float64(window)*CompactRatio)
	if reserve < MinCompactReserve {
		return MinCompactReserve
	}
	return reserve
}

// ModelJSONSchema 返回模型的 json_schema 三态声明；未列入 models 或未配置时
// 返回 nil（按 adapter 能力判断）。
func (c Config) ModelJSONSchema(provider, model string) *bool {
	if pc, ok := c.Providers[provider]; ok {
		if mc, ok := pc.ModelConfig(model); ok {
			return mc.JSONSchema
		}
	}
	return nil
}

// roles.RoleConfig/roles.ModelRef 与按角色解析的规则已提取到 core/roles（跨应用共享）；
// 下方 knownRoles 是本应用允许配置的具体角色名。

// knownRoles 支持的可配置角色名。Arbiter 当前不开放角色级配置，
// 统一使用顶层默认模型（host.arbiterModel 用 models.Default）。
// import_* 是导入语义函数的模型档位旋钮（docs/import-pipeline.md §13.1）：
// 未配置时落 architect，配置后可把机械性更强的函数指到更便宜档位。
var knownRoles = map[string]bool{
	"architect":         true,
	"writer":            true,
	"editor":            true,
	"import_segment":    true,
	"import_analyze":    true,
	"import_synthesize": true,
}

// Config 小说应用配置。
type Config struct {
	// 运行时字段（不序列化到 JSON）
	OutputDir string `json:"-"` // 输出根目录

	// 默认 LLM 配置
	Provider  string `json:"provider,omitempty"` // 默认 provider（Providers map 中的 key）；空则不写盘（网页引导会分步保存）
	ModelName string `json:"model,omitempty"`    // 默认模型名
	// ReasoningEffort 顶层默认推理强度（off/low/medium/high/xhigh/max），空=不覆盖（沿用模型/provider 默认）。
	// 角色未单独配置 reasoning_effort 时回落到此值。
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// Provider 凭证库
	Providers map[string]llmconfig.ProviderConfig `json:"providers,omitempty"`

	// 角色级模型覆盖
	Roles map[string]roles.RoleConfig `json:"roles,omitempty"`

	// 创作参数
	Style string `json:"style,omitempty"`
	// Language là ngôn ngữ sáng tác ("vi" mặc định | "zh"); bước Ngôn ngữ của Onboard ghi khoá này.
	Language string `json:"language,omitempty"`
	// ContextWindow là cửa sổ ngữ cảnh toàn cục phiên bản cũ, giữ lại để tương thích.
	ContextWindow int `json:"context_window,omitempty"`

	// AdvanceMode 是新建书籍的章节推进模式（"auto" 默认 / "review" 逐章验收）。
	// 仅在建书时初始化 RunMeta；已存在的书保留其持久化模式，不被配置覆盖。
	AdvanceMode string `json:"advance_mode,omitempty"`

	// AskUser 允许 Trọng tài tạm dừng và hỏi người dùng 2–3 lựa chọn (mặc định tắt).
	// Kế thừa 3 cấp như mọi khoá khác; lớp gần bật/tắt được độc lập.
	AskUser *bool `json:"ask_user,omitempty"`

	// Budget 单本书的成本预算政策；book_usd > 0 才启用。
	Budget BudgetConfig `json:"budget,omitzero"`

	// Notify 无人值守告警配置；缺省启用（system 通道兜底）。
	Notify NotifyConfig `json:"notify,omitzero"`

	// TTS là cấu hình giọng đọc trên máy (GAP-21). Kế thừa 3 cấp như mọi khoá khác;
	// api_key chỉ nên đặt ở cấp toàn cục.
	TTS TTSConfig `json:"tts,omitzero"`
}

// TTSConfig là cấu hình bộ đọc trên máy (internal/voice).
type TTSConfig struct {
	Engine   string `json:"engine,omitempty"`   // vieneu (mặc định) | openai-speech
	Model    string `json:"model,omitempty"`    // model gửi lên máy đọc (vieneu: vieneu-v3-turbo)
	Device   string `json:"device,omitempty"`   // cpu | gpu
	Quality  string `json:"quality,omitempty"`  // fast | standard | high
	Endpoint string `json:"endpoint,omitempty"` // engine openai-speech: địa chỉ máy chủ TTS
	APIKey   string `json:"api_key,omitempty"`  // Bearer token của máy chủ (chỉ cấp toàn cục)
}

// WithDefaults 填充缺省值：本地 VieNeu、48 kHz v3 Turbo、CPU、chuẩn。
func (t TTSConfig) WithDefaults() TTSConfig {
	if strings.TrimSpace(t.Engine) == "" {
		t.Engine = "vieneu"
	}
	if strings.TrimSpace(t.Model) == "" {
		t.Model = "vieneu-v3-turbo"
	}
	t.Device = strings.ToLower(strings.TrimSpace(t.Device))
	if t.Device == "" {
		t.Device = "cpu"
	}
	switch strings.ToLower(strings.TrimSpace(t.Quality)) {
	case "fast", "standard", "high":
		t.Quality = strings.ToLower(strings.TrimSpace(t.Quality))
	default:
		t.Quality = "standard"
	}
	t.Endpoint = strings.TrimSpace(t.Endpoint)
	return t
}

// BudgetConfig 是用户对单本书钱包的政策声明。越线停机等同于用户在那一刻
// 手动 Abort——Host 只代为执行，不评估模型行为（架构 §10 合宪边界）。
type BudgetConfig struct {
	BookUSD   float64 `json:"book_usd,omitempty"`   // 必填才启用；0/缺省 = 不限
	WarnRatio float64 `json:"warn_ratio,omitempty"` // 告警水位，默认 0.8
	HardStop  bool    `json:"hard_stop,omitempty"`  // true=越线立即停；默认等当前子代理任务结束
}

// Enabled 返回预算政策是否启用。
func (b BudgetConfig) Enabled() bool { return b.BookUSD > 0 }

// NotifyConfig 无人值守告警通道配置。
type NotifyConfig struct {
	Enabled *bool    `json:"enabled,omitempty"` // 缺省 true（system 通道零配置可用）
	Command string   `json:"command,omitempty"` // 可选，配置后替代 system 通道（手机推送走这里）
	Events  []string `json:"events,omitempty"`  // 可选，按 notify.Kinds 过滤；缺省全开
}

// IsEnabled 返回告警是否启用（缺省 true）。
func (n NotifyConfig) IsEnabled() bool { return n.Enabled == nil || *n.Enabled }

// ArbiterAskUser 报告 Trọng tài có được hỏi người dùng hay không (mặc định tắt).
func (c Config) ArbiterAskUser() bool { return c.AskUser != nil && *c.AskUser }

// ValidLanguage báo language có thể ghi vào cấu hình: rỗng (mặc định vi) hoặc "vi".
// "zh" chưa hỗ trợ viết (bước Ngôn ngữ của Onboard khoá thẻ Tiếng Trung).
func ValidLanguage(language string) bool {
	return language == "" || language == "vi"
}

// ValidateBase 校验基础配置。
func (c *Config) ValidateBase() error {
	if err := validateConfigText("provider", c.Provider); err != nil {
		return err
	}
	if err := validateConfigText("model", c.ModelName); err != nil {
		return err
	}

	if c.Provider == "" {
		return fmt.Errorf("thiếu provider: %w", errs.ErrConfig)
	}
	if c.ModelName == "" {
		return fmt.Errorf("thiếu model: %w", errs.ErrConfig)
	}

	if c.AdvanceMode != "" && c.AdvanceMode != "auto" && c.AdvanceMode != "review" {
		return fmt.Errorf("advance_mode chỉ nhận \"auto\" hoặc \"review\": %w", errs.ErrConfig)
	}

	// 默认 provider 必须有凭证
	pc, ok := c.Providers[c.Provider]
	if !ok {
		return fmt.Errorf("provider %q chưa cấu hình thông tin xác thực trong providers; nếu đã ghi đè provider trong ./.ainovel/config.json thì phải khai báo cả providers.%s (gồm api_key/base_url), không chỉ sửa provider ở cấp cao nhất: %w", c.Provider, c.Provider, errs.ErrConfig)
	}
	if pc.RequiresAPIKey(c.Provider) && pc.APIKey == "" {
		return fmt.Errorf("provider %q chưa có khoá API (api_key): %w", c.Provider, errs.ErrConfig)
	}
	if err := validateProviderConfigText(c.Provider, pc); err != nil {
		return err
	}
	if err := c.validateProviderAPI("model mặc định", c.Provider, pc); err != nil {
		return err
	}
	for name, provider := range c.Providers {
		if err := ValidateProvider(name, provider); err != nil {
			return err
		}
	}

	// 校验角色覆盖
	for role, rc := range c.Roles {
		if err := validateConfigText("tên vai", role); err != nil {
			return err
		}
		if err := validateConfigText(fmt.Sprintf("provider của vai %q", role), rc.Provider); err != nil {
			return err
		}
		if err := validateConfigText(fmt.Sprintf("model của vai %q", role), rc.Model); err != nil {
			return err
		}
		if !knownRoles[role] {
			return fmt.Errorf("vai %q không có trong roles (hợp lệ: architect/writer/editor/import_segment/import_analyze/import_synthesize): %w", role, errs.ErrConfig)
		}
		if rc.Provider == "" || rc.Model == "" {
			return fmt.Errorf("vai %q phải có đủ provider và model: %w", role, errs.ErrConfig)
		}
		if err := c.validateModelRef(
			fmt.Sprintf("vai %q", role),
			roles.ModelRef{Provider: rc.Provider, Model: rc.Model},
		); err != nil {
			return err
		}
		for i, fallback := range rc.Fallbacks {
			if err := validateConfigText(fmt.Sprintf("provider của vai %q fallback[%d]", role, i), fallback.Provider); err != nil {
				return err
			}
			if err := validateConfigText(fmt.Sprintf("model của vai %q fallback[%d]", role, i), fallback.Model); err != nil {
				return err
			}
			if err := c.validateModelRef(
				fmt.Sprintf("vai %q fallback[%d]", role, i),
				fallback,
			); err != nil {
				return err
			}
		}
	}

	// 校验预算政策
	if c.Budget.BookUSD < 0 {
		return fmt.Errorf("budget.book_usd phải >= 0: %w", errs.ErrConfig)
	}
	if c.Budget.Enabled() && (c.Budget.WarnRatio <= 0 || c.Budget.WarnRatio >= 1) {
		return fmt.Errorf("budget.warn_ratio phải nằm trong khoảng (0, 1): %w", errs.ErrConfig)
	}

	// 校验告警配置
	if err := validateConfigText("notify.command", c.Notify.Command); err != nil {
		return err
	}
	for _, ev := range c.Notify.Events {
		if !notify.IsKnownKind(ev) {
			return fmt.Errorf("sự kiện notify %q không tồn tại (hợp lệ: %s): %w", ev, strings.Join(notify.Kinds(), "/"), errs.ErrConfig)
		}
	}

	return nil
}

// ValidateModelReady checks only providers reachable by the default, roles and
// fallbacks. ValidateBase remains strict for full configuration validation.
func (c Config) ValidateModelReady() error {
	providers := make(map[string]llmconfig.ProviderConfig)
	add := func(name string) {
		if pc, ok := c.Providers[name]; ok {
			providers[name] = pc
		}
	}
	add(c.Provider)
	for _, role := range c.Roles {
		add(role.Provider)
		for _, fallback := range role.Fallbacks {
			add(fallback.Provider)
		}
	}
	c.Providers = providers
	return c.ValidateBase()
}

// ValidateProvider 只校验单个 provider 的结构（models、api、stream_idle_timeout、控制字符），
// 不要求它是默认 provider，也不检查 key——供网关“Thử kết nối”在建模型前给出越南语错误。
func ValidateProvider(name string, pc llmconfig.ProviderConfig) error {
	if err := validateConfigText("tên provider", name); err != nil {
		return err
	}
	typ, err := pc.ProviderType(name)
	if err != nil {
		return err
	}
	switch typ {
	case codex.ProviderName, xai.ProviderName, anthropicoauth.ProviderName:
	default:
		if !llm.IsProviderRegistered(typ) {
			return fmt.Errorf("provider %q: giao thức %q không được hỗ trợ: %w", name, typ, errs.ErrConfig)
		}
	}
	if err := validateProviderConfigText(name, pc); err != nil {
		return err
	}
	return Config{}.validateProviderAPI(fmt.Sprintf("provider %q", name), name, pc)
}

func validateProviderConfigText(name string, pc llmconfig.ProviderConfig) error {
	fields := []struct {
		label string
		value string
	}{
		{label: fmt.Sprintf("provider %q type", name), value: pc.Type},
		{label: fmt.Sprintf("provider %q api", name), value: pc.API},
		{label: fmt.Sprintf("provider %q api_key", name), value: pc.APIKey},
		{label: fmt.Sprintf("provider %q base_url", name), value: pc.BaseURL},
	}
	for _, field := range fields {
		if err := validateConfigText(field.label, field.value); err != nil {
			return err
		}
	}
	seenModels := make(map[string]bool, len(pc.Models))
	for i, model := range pc.Models {
		modelName := strings.TrimSpace(model.Name)
		if err := validateConfigText(fmt.Sprintf("provider %q models[%d].name", name, i), model.Name); err != nil {
			return err
		}
		if modelName == "" {
			return fmt.Errorf("provider %q: models[%d].name không được trống: %w", name, i, errs.ErrConfig)
		}
		if seenModels[modelName] {
			return fmt.Errorf("provider %q: model %q bị trùng: %w", name, modelName, errs.ErrConfig)
		}
		seenModels[modelName] = true
		if model.ContextWindow < 0 {
			return fmt.Errorf("provider %q: context_window của model %q phải >= 0: %w", name, modelName, errs.ErrConfig)
		}
	}
	switch pc.API {
	case "", "chat", "responses":
	default:
		return fmt.Errorf("provider %q: api chỉ nhận \"chat\" hoặc \"responses\": %w", name, errs.ErrConfig)
	}
	if _, err := pc.StreamIdleTimeoutValue(); err != nil {
		return fmt.Errorf("provider %q stream_idle_timeout: %w: %w", name, err, errs.ErrConfig)
	}
	return nil
}

func validateConfigText(name, value string) error {
	if utils.ContainsControl(value) {
		return fmt.Errorf("%s chứa ký tự điều khiển: %w", name, errs.ErrConfig)
	}
	return nil
}

// DefaultProviderConfig 返回默认 provider 的凭证配置。
func (c *Config) DefaultProviderConfig() llmconfig.ProviderConfig {
	if c.Providers == nil {
		return llmconfig.ProviderConfig{}
	}
	return c.Providers[c.Provider]
}

// FillDefaults 填充默认值。
func (c *Config) FillDefaults() {
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join("output", "novel")
	}
	if c.Providers == nil {
		c.Providers = make(map[string]llmconfig.ProviderConfig)
	}
	if c.Roles == nil {
		c.Roles = make(map[string]roles.RoleConfig)
	}
	if c.Style == "" {
		c.Style = "default"
	}
	if c.Budget.Enabled() && c.Budget.WarnRatio == 0 {
		c.Budget.WarnRatio = 0.8
	}
	c.TTS = c.TTS.WithDefaults()
}

// ContextWindowSource 标记窗口取值的来源，供日志/诊断使用。
type ContextWindowSource string

const (
	CtxWindowModelConfig ContextWindowSource = "model_config" // provider 模型项显式指定
	CtxWindowConfig      ContextWindowSource = "config"       // 旧顶层 context_window 显式指定
	CtxWindowRegistry    ContextWindowSource = "registry"     // OpenRouter 基线命中
	CtxWindowDefault     ContextWindowSource = "default"      // 兜底（自定义代理/未知模型）
)

// ResolveContextWindow 解析上下文压缩使用的有效窗口，按优先级：
//  1. providers.<provider>.models[].context_window
//  2. 旧顶层 ContextWindow（兼容已有配置）
//  3. models.DefaultRegistry 按模型名查询（OpenRouter 基线 + 24h 刷新）
//  4. 兜底 DefaultContextWindow（自定义代理 / 未知模型）
//
// 注意：返回值仅用于压缩阈值计算，不会缩小 LLM API 真实可发请求长度。
func (c Config) ResolveContextWindow(provider, modelName string) (int, ContextWindowSource) {
	if pc, ok := c.Providers[strings.TrimSpace(provider)]; ok {
		if model, found := pc.ModelConfig(modelName); found && model.ContextWindow > 0 {
			return model.ContextWindow, CtxWindowModelConfig
		}
	}
	if c.ContextWindow > 0 {
		return c.ContextWindow, CtxWindowConfig
	}
	if rw := models.DefaultRegistry().ResolveContextWindow(modelName); rw > 0 {
		return rw, CtxWindowRegistry
	}
	return DefaultContextWindow, CtxWindowDefault
}

// ResolveReasoningEffort 返回某角色生效的推理强度原始串（off/low/medium/high/xhigh/max 或空）。
// 优先级：角色级 Roles[role].ReasoningEffort → 顶层默认 ReasoningEffort → ""（不覆盖，沿用模型/provider 默认）。
// role 为空或 "default" 时直接取顶层默认。值的合法性由 llm.ParseThinkingLevel 把关。
func (c Config) ResolveReasoningEffort(role string) string {
	return roles.ResolveReasoningEffort(c.Roles, role, c.ReasoningEffort)
}

// ResolveRole 返回某角色当前生效的 provider/model；未配置的角色回落默认对，
// explicit=false。解析规则在 core/roles。
func (c Config) ResolveRole(role string) (provider, model string, explicit bool) {
	return roles.Resolve(c.Roles, role, c.Provider, c.ModelName)
}

// LogContextWindowChoice 打印某个角色的窗口决策。source=default 时发 Warn 提示
// 该模型未在 registry 命中（OpenRouter 也未收录），后续上下文压缩会按兜底窗口
// 触发——若模型实际窗口更大，可在配置文件用 context_window 显式指定，避免被提前压缩、丢史。
func LogContextWindowChoice(role, model string, window int, source ContextWindowSource) {
	attrs := []any{"module", "context", "role", role, "model", model, "window", window, "source", source}
	switch source {
	case CtxWindowModelConfig:
		slog.Info("cửa sổ ngữ cảnh (từ cấu hình model của provider)", attrs...)
	case CtxWindowDefault:
		slog.Warn("model chưa nhận diện, dùng cửa sổ dự phòng (có thể chỉ định rõ ở providers.<name>.models[].context_window)", attrs...)
	case CtxWindowConfig:
		slog.Info("cửa sổ ngữ cảnh (từ context_window trong tệp cấu hình)", attrs...)
	default:
		slog.Info("cửa sổ ngữ cảnh", attrs...)
	}
}

// CandidateModels 返回某个 provider 下可供切换的模型列表。
// 优先使用 provider 显式声明的 models；同时补充当前配置中已出现过的该 provider 模型。
func (c Config) CandidateModels(provider string) []string {
	if provider == "" {
		return nil
	}

	seen := make(map[string]bool)
	models := make([]string, 0, 4)
	add := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			return
		}
		seen[model] = true
		models = append(models, model)
	}

	if pc, ok := c.Providers[provider]; ok {
		for _, model := range pc.Models {
			add(model.Name)
		}
	}
	if c.Provider == provider {
		add(c.ModelName)
	}
	for _, rc := range c.Roles {
		if rc.Provider == provider {
			add(rc.Model)
		}
		for _, fallback := range rc.Fallbacks {
			if fallback.Provider == provider {
				add(fallback.Model)
			}
		}
	}
	return models
}

func (c Config) validateModelRef(owner string, ref roles.ModelRef) error {
	if ref.Provider == "" || ref.Model == "" {
		return fmt.Errorf("%s phải có đủ provider và model: %w", owner, errs.ErrConfig)
	}

	pc, ok := c.Providers[ref.Provider]
	if !ok {
		return fmt.Errorf("%s trỏ tới provider %q chưa được cấu hình: %w", owner, ref.Provider, errs.ErrConfig)
	}
	if pc.RequiresAPIKey(ref.Provider) && pc.APIKey == "" {
		return fmt.Errorf("%s trỏ tới provider %q chưa có khoá API (api_key): %w", owner, ref.Provider, errs.ErrConfig)
	}
	if err := c.validateProviderAPI(owner, ref.Provider, pc); err != nil {
		return err
	}
	return nil
}

func (c Config) validateProviderAPI(owner, providerName string, pc llmconfig.ProviderConfig) error {
	if pc.API == "" {
		return nil
	}
	providerType, err := pc.ProviderType(providerName)
	if err != nil {
		return fmt.Errorf("%s provider %q: không phân tích được loại giao thức từ cấu hình api: %w", owner, providerName, err)
	}
	if providerType == codex.ProviderName {
		if pc.API == "responses" {
			return nil
		}
		return fmt.Errorf("%s provider %q: luôn dùng giao thức Responses, api chỉ có thể là \"responses\" hoặc để trống: %w", owner, providerName, errs.ErrConfig)
	}
	if strings.ToLower(strings.TrimSpace(providerType)) != "openai" {
		return fmt.Errorf("%s provider %q: cấu hình api chỉ hỗ trợ provider giao thức OpenAI: %w", owner, providerName, errs.ErrConfig)
	}
	return nil
}
