package bootstrap

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/auth/anthropicoauth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	"github.com/datnt1998/verse/core/llmcontract"
	"github.com/datnt1998/verse/core/llmretry"
	"github.com/datnt1998/verse/core/roles"
)

// FailoverEvent 表示一次显式 provider 切换。
// Reason 为短标签（rate_limit / timeout / stream_idle / network），用于结构化日志。
type FailoverEvent struct {
	Role         string
	Reason       string
	FromProvider string
	FromModel    string
	ToProvider   string
	ToModel      string
	Err          error
}

// FailoverReporter 在发生显式切换时被调用。
type FailoverReporter func(FailoverEvent)

type modelTarget struct {
	provider   string
	name       string
	model      llmtypes.ChatModel
	jsonSchema *bool
}

// SwappableModel 是可热切换的 ChatModel 包装器。
// 已开始的请求继续使用旧实例；后续请求自动切到新实例。
type SwappableModel struct {
	*llmtypes.SwappableModel
	mu       sync.RWMutex
	provider string
	name     string
	// jsonSchema 是当前选中模型的 config json_schema 三态声明，与 provider/name
	// 同锁原子切换；llmcontract.Resolve 经结构匹配接口每次现读。
	jsonSchema *bool
}

func NewSwappableModel(provider, name string, model llmtypes.ChatModel, jsonSchema *bool) *SwappableModel {
	return &SwappableModel{
		SwappableModel: llmtypes.NewSwappableModel(model),
		provider:       provider,
		name:           name,
		jsonSchema:     jsonSchema,
	}
}

func (m *SwappableModel) ProviderName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.provider
}

func (m *SwappableModel) Info() llmtypes.ModelInfo {
	return m.StructuredOutputFacts().Info
}

// StructuredOutputFacts 在同一把锁下读取模型实例、身份和配置覆盖，保证一次
// 结构化协议选择只观察到一个完整版本。
func (m *SwappableModel) StructuredOutputFacts() llmcontract.ModelFacts {
	m.mu.RLock()
	defer m.mu.RUnlock()
	current := m.SwappableModel.Current()
	facts := llmcontract.ModelFacts{
		Info:               llmtypes.ModelInfo{Name: m.name, Provider: m.provider},
		JSONSchemaOverride: cloneBoolPtr(m.jsonSchema),
	}
	if cp, ok := current.(llmtypes.CapabilityProvider); ok {
		facts.Capabilities = cp.Capabilities()
	}
	if info, ok := current.(interface{ Info() llmtypes.ModelInfo }); ok {
		modelInfo := info.Info()
		if modelInfo.Name == "" {
			modelInfo.Name = m.name
		}
		if modelInfo.Provider == "" {
			modelInfo.Provider = m.provider
		}
		facts.Info = modelInfo
	}
	return facts
}

func (m *SwappableModel) Capabilities() llmtypes.Capabilities {
	return m.StructuredOutputFacts().Capabilities
}

func (m *SwappableModel) Swap(provider, name string, model llmtypes.ChatModel, jsonSchema *bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SwappableModel.Swap(model)
	m.provider = provider
	m.name = name
	m.jsonSchema = jsonSchema
}

// JSONSchemaOverride 返回当前选中模型的 config json_schema 三态声明。
func (m *SwappableModel) JSONSchemaOverride() *bool {
	return m.StructuredOutputFacts().JSONSchemaOverride
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (m *SwappableModel) Current() (provider, name string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.provider, m.name
}

// ModelSet 持有按角色分配的模型实例，未配置的角色回退到默认模型。
type ModelSet struct {
	mu        sync.RWMutex
	Default   *SwappableModel
	models    map[string]*SwappableModel
	fallbacks map[string][]modelTarget
	config    Config
	// retryGate 可选：agent 模型包装器的可唤醒退避闸门（G4）。nil 时不包装。
	retryGate *llmretry.Gate
}

// SetRetryGate 让之后取出的 agent 模型带上可唤醒重试包装（G4）。
func (ms *ModelSet) SetRetryGate(gate *llmretry.Gate) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.retryGate = gate
}

// ForRole 返回指定角色的模型，未配置时返回默认模型。
func (ms *ModelSet) ForRole(role string) llmtypes.ChatModel {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if m, ok := ms.models[role]; ok {
		return m
	}
	return ms.Default
}

// ForRoleWithFailover 返回带有单次请求级 fallback 的角色模型。
// 仅当该角色显式配置了 fallbacks 时生效；未配置时退化为普通模型。
func (ms *ModelSet) ForRoleWithFailover(role string, report FailoverReporter) llmtypes.ChatModel {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	primary, ok := ms.models[role]
	if !ok {
		return ms.wrap(ms.Default, role)
	}
	targets := ms.fallbacks[role]
	if len(targets) == 0 {
		return ms.wrap(primary, role)
	}
	return ms.wrap(&failoverModel{
		role: role, primary: primary, set: ms, report: report,
	}, role)
}

// wrap 给 agent 模型套上开流前重试（G4）；未配置闸门时原样返回。
func (ms *ModelSet) wrap(m llmtypes.ChatModel, role string) llmtypes.ChatModel {
	if ms.retryGate == nil || m == nil {
		return m
	}
	return llmretry.NewRetryingModel(m, ms.retryGate, role, llmretry.DefaultMaxRetries)
}

// Summary 返回模型分配摘要（供日志使用）。
func (ms *ModelSet) Summary() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var parts []string
	for role, m := range ms.models {
		provider, name := m.Current()
		parts = append(parts, fmt.Sprintf("%s=%s/%s", role, provider, name))
	}
	if len(parts) == 0 {
		provider, name := ms.Default.Current()
		return fmt.Sprintf("default=%s/%s", provider, name)
	}
	provider, name := ms.Default.Current()
	return fmt.Sprintf("default=%s/%s %s", provider, name, strings.Join(parts, " "))
}

// CurrentSelection 返回角色当前生效的 provider/model。
// role 为空或 "default" 时返回默认模型。
func (ms *ModelSet) CurrentSelection(role string) (provider, model string, explicit bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if role == "" || role == "default" {
		provider, model = ms.Default.Current()
		return provider, model, true
	}
	if sw, ok := ms.models[role]; ok {
		provider, model = sw.Current()
		return provider, model, true
	}
	provider, model = ms.Default.Current()
	return provider, model, false
}

// Swap 切换默认模型或指定角色模型。
// role 为空或 "default" 时切换默认模型；其他角色切换为显式覆盖。
func (ms *ModelSet) Swap(role, provider, model string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	pc, ok := ms.config.Providers[provider]
	if !ok {
		return fmt.Errorf("provider %q chưa được cấu hình: %w", provider, errs.ErrConfig)
	}
	next, err := createModelFromConfig(provider, model, pc, make(map[string]llmtypes.ChatModel))
	if err != nil {
		return fmt.Errorf("đổi model thất bại: %w", err)
	}

	jsonSchema := ms.config.ModelJSONSchema(provider, model)
	if role == "" || role == "default" {
		ms.Default.Swap(provider, model, next, jsonSchema)
		ms.config.Provider = provider
		ms.config.ModelName = model
		return nil
	}

	if !knownRoles[role] {
		return fmt.Errorf("vai %q không tồn tại: %w", role, errs.ErrConfig)
	}

	if existing, ok := ms.models[role]; ok {
		existing.Swap(provider, model, next, jsonSchema)
	} else {
		ms.models[role] = NewSwappableModel(provider, model, next, jsonSchema)
	}
	if ms.config.Roles == nil {
		ms.config.Roles = make(map[string]roles.RoleConfig)
	}
	rc := ms.config.Roles[role]
	rc.Provider = provider
	rc.Model = model
	ms.config.Roles[role] = rc
	return nil
}

// ResolveContextWindow 使用 ModelSet 的最新配置解析窗口，供运行时热切换后的
// ContextManagerFactory 使用，避免捕获启动时的 Config 副本。
func (ms *ModelSet) ResolveContextWindow(provider, model string) (int, ContextWindowSource) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.config.ResolveContextWindow(provider, model)
}

// ApplyPrepared 提交一个已成功构建的候选 ModelSet。已有 SwappableModel 的地址
// 保持不变，因此已装配的 Worker/Arbiter 会在下一次请求自动使用新客户端。
func (ms *ModelSet) ApplyPrepared(candidate *ModelSet) {
	if candidate == nil {
		return
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()

	defaultProvider, defaultName := candidate.Default.Current()
	ms.Default.Swap(defaultProvider, defaultName, candidate.Default.SwappableModel.Current(), candidate.Default.JSONSchemaOverride())

	nextModels := make(map[string]*SwappableModel, len(candidate.models))
	for role, next := range candidate.models {
		provider, name := next.Current()
		if existing, ok := ms.models[role]; ok {
			existing.Swap(provider, name, next.SwappableModel.Current(), next.JSONSchemaOverride())
			nextModels[role] = existing
		} else {
			nextModels[role] = next
		}
	}
	ms.models = nextModels
	ms.fallbacks = candidate.fallbacks
	ms.config = CloneConfig(candidate.config)
}

func (ms *ModelSet) fallbackTargets(role string) []modelTarget {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return append([]modelTarget(nil), ms.fallbacks[role]...)
}

// ModelName 从 ChatModel 中提取当前模型名，失败返回空字符串。
// 支持 SwappableModel 的热切换：调用时总是返回最新值。
func ModelName(m llmtypes.ChatModel) string {
	if info, ok := m.(interface{ Info() llmtypes.ModelInfo }); ok {
		return info.Info().Name
	}
	return ""
}

// ModelProvider 从 ChatModel 中提取当前 provider 名称，失败返回空字符串。
func ModelProvider(m llmtypes.ChatModel) string {
	if info, ok := m.(interface{ Info() llmtypes.ModelInfo }); ok {
		return info.Info().Provider
	}
	if provider, ok := m.(interface{ ProviderName() string }); ok {
		return provider.ProviderName()
	}
	return ""
}

// NewModelSet 根据配置创建多模型集合。
// 相同 provider+model 组合复用同一个实例。
func NewModelSet(cfg Config) (*ModelSet, error) {
	cache := make(map[string]llmtypes.ChatModel)

	// 创建默认模型
	defaultPC := cfg.DefaultProviderConfig()
	defaultModel, err := createModelFromConfig(cfg.Provider, cfg.ModelName, defaultPC, cache)
	if err != nil {
		return nil, fmt.Errorf("model mặc định: %w", err)
	}

	ms := &ModelSet{
		Default:   NewSwappableModel(cfg.Provider, cfg.ModelName, defaultModel, cfg.ModelJSONSchema(cfg.Provider, cfg.ModelName)),
		models:    make(map[string]*SwappableModel),
		fallbacks: make(map[string][]modelTarget),
		config:    cfg,
	}

	// 创建角色覆盖模型
	for role, rc := range cfg.Roles {
		pc, ok := cfg.Providers[rc.Provider]
		if !ok {
			return nil, fmt.Errorf("vai %s trỏ tới provider %q chưa được cấu hình: %w", role, rc.Provider, errs.ErrConfig)
		}
		m, err := createModelFromConfig(rc.Provider, rc.Model, pc, cache)
		if err != nil {
			return nil, fmt.Errorf("model của vai %s: %w", role, err)
		}
		ms.models[role] = NewSwappableModel(rc.Provider, rc.Model, m, cfg.ModelJSONSchema(rc.Provider, rc.Model))
		slog.Info("gán model theo vai", "module", "config", "role", role, "provider", rc.Provider, "model", rc.Model)
		if len(rc.Fallbacks) == 0 {
			continue
		}

		targets := make([]modelTarget, 0, len(rc.Fallbacks))
		for _, fallback := range rc.Fallbacks {
			fpc, ok := cfg.Providers[fallback.Provider]
			if !ok {
				return nil, fmt.Errorf("fallback của vai %s trỏ tới provider %q chưa được cấu hình: %w", role, fallback.Provider, errs.ErrConfig)
			}
			fm, err := createModelFromConfig(fallback.Provider, fallback.Model, fpc, cache)
			if err != nil {
				return nil, fmt.Errorf("fallback %s/%s của vai %s: %w", fallback.Provider, fallback.Model, role, err)
			}
			targets = append(targets, modelTarget{
				provider:   fallback.Provider,
				name:       fallback.Model,
				model:      fm,
				jsonSchema: cfg.ModelJSONSchema(fallback.Provider, fallback.Model),
			})
		}
		ms.fallbacks[role] = targets
	}

	return ms, nil
}

// createModelFromConfig 创建或复用 ChatModel 实例。
func createModelFromConfig(providerKey, model string, pc ProviderConfig, cache map[string]llmtypes.ChatModel) (llmtypes.ChatModel, error) {
	cacheKey := providerKey + "|" + model
	if m, ok := cache[cacheKey]; ok {
		return m, nil
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
		return nil, fmt.Errorf("provider %s: stream_idle_timeout: %w: %w", providerKey, errs.ErrConfig, err)
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
		return nil, fmt.Errorf("provider %s (%s): %w: %w", providerKey, providerType, errs.ErrProvider, err)
	}
	cache[cacheKey] = m
	return m, nil
}

// newCodexModel 创建 ChatGPT 订阅（Codex 后端）模型：凭证取自 ~/.ainovel/auth.json，
// 由 `ainovel-cli login openai-codex` 写入，请求时按需刷新。
func newCodexModel(model string, pc ProviderConfig, extra map[string]any, streamIdle time.Duration) (llmtypes.ChatModel, error) {
	if pc.API != "" {
		return nil, fmt.Errorf("openai-codex luôn dùng giao thức Responses, không hỗ trợ cấu hình api")
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

// newXAIModel 创建 SuperGrok / X Premium+ 订阅模型：凭证取自 ~/.ainovel/auth.json，
// 由 `ainovel-cli login xai-oauth` 写入，请求时按需刷新。
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

// newAnthropicOAuthModel 创建 Claude Pro/Max 订阅模型：凭证取自 ~/.ainovel/auth.json，
// 由 `ainovel-cli login anthropic-oauth` 写入，请求时按需刷新。
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

type failoverModel struct {
	role    string
	primary *SwappableModel
	set     *ModelSet
	report  FailoverReporter
}

func (m *failoverModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	current := m.currentTarget()
	resp, err := current.model.Generate(ctx, messages, tools, opts...)
	if err == nil {
		return resp, nil
	}

	next, reason, ok := m.pickFallback(current, err, requestsJSONSchema(opts))
	if !ok {
		return nil, err
	}
	m.reportFailover(current, next, reason, err)
	return next.model.Generate(ctx, messages, tools, opts...)
}

func (m *failoverModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	out := make(chan llmtypes.StreamEvent, 100)

	go func() {
		defer close(out)

		current := m.currentTarget()
		fallbackUsed := false

	retry:
		source, resp, err := m.startAttempt(ctx, current, messages, tools, opts...)
		if err != nil {
			if !fallbackUsed {
				if next, reason, ok := m.pickFallback(current, err, requestsJSONSchema(opts)); ok {
					fallbackUsed = true
					m.reportFailover(current, next, reason, err)
					current = next
					goto retry
				}
			}
			out <- llmtypes.StreamEvent{Type: llmtypes.StreamEventError, Err: err}
			return
		}
		if resp != nil {
			out <- llmtypes.StreamEvent{
				Type:       llmtypes.StreamEventDone,
				Message:    resp.Message,
				StopReason: resp.Message.StopReason,
			}
			return
		}

		forwarded := false
		for ev := range source {
			switch ev.Type {
			case llmtypes.StreamEventError:
				if ev.Err != nil && !forwarded && !fallbackUsed {
					if next, reason, ok := m.pickFallback(current, ev.Err, requestsJSONSchema(opts)); ok {
						fallbackUsed = true
						m.reportFailover(current, next, reason, ev.Err)
						current = next
						goto retry
					}
				}
				out <- ev
				return
			case llmtypes.StreamEventDone:
				out <- ev
				return
			default:
				forwarded = true
				out <- ev
			}
		}
	}()

	return out, nil
}

func (m *failoverModel) SupportsTools() bool {
	return m.primary != nil && m.primary.SupportsTools()
}

func (m *failoverModel) ProviderName() string {
	if m.primary == nil {
		return ""
	}
	return m.primary.ProviderName()
}

func (m *failoverModel) Info() llmtypes.ModelInfo {
	if m.primary == nil {
		return llmtypes.ModelInfo{}
	}
	return m.primary.Info()
}

func (m *failoverModel) Capabilities() llmtypes.Capabilities {
	return m.StructuredOutputFacts().Capabilities
}

func (m *failoverModel) JSONSchemaOverride() *bool {
	return m.StructuredOutputFacts().JSONSchemaOverride
}

func (m *failoverModel) StructuredOutputFacts() llmcontract.ModelFacts {
	if m.primary == nil {
		return llmcontract.ModelFacts{}
	}
	return m.primary.StructuredOutputFacts()
}

func (m *failoverModel) currentTarget() modelTarget {
	if m.primary == nil {
		return modelTarget{}
	}
	provider, name := m.primary.Current()
	return modelTarget{
		provider:   provider,
		name:       name,
		model:      m.primary,
		jsonSchema: m.primary.JSONSchemaOverride(),
	}
}

func (m *failoverModel) pickFallback(current modelTarget, err error, requireJSONSchema bool) (modelTarget, string, bool) {
	if err == nil || current.model == nil {
		return modelTarget{}, "", false
	}
	if errors.Is(err, context.Canceled) {
		return modelTarget{}, "", false
	}

	if !agentcore.IsFailoverEligible(err) {
		return modelTarget{}, agentcore.FailoverReason(err), false
	}
	reason := agentcore.FailoverReason(err)
	var targets []modelTarget
	if m.set != nil {
		targets = m.set.fallbackTargets(m.role)
	}
	for _, target := range targets {
		if target.provider == current.provider && target.name == current.name {
			continue
		}
		if target.model == nil {
			continue
		}
		if requireJSONSchema && !supportsJSONSchema(target) {
			continue
		}
		return target, reason, true
	}
	return modelTarget{}, reason, false
}

func requestsJSONSchema(opts []llmtypes.CallOption) bool {
	format := llmtypes.ResolveCallConfig(opts).ResponseFormat
	return format != nil && format.Type == llmtypes.ResponseFormatJSONSchema
}

func supportsJSONSchema(target modelTarget) bool {
	if target.jsonSchema != nil {
		return *target.jsonSchema
	}
	cp, ok := target.model.(llmtypes.CapabilityProvider)
	return ok && cp.Capabilities().Structured.JSONSchema == llmtypes.SupportYes
}

func (m *failoverModel) reportFailover(from, to modelTarget, reason string, err error) {
	if m.report != nil {
		m.report(FailoverEvent{
			Role:         m.role,
			Reason:       reason,
			FromProvider: from.provider,
			FromModel:    from.name,
			ToProvider:   to.provider,
			ToModel:      to.name,
			Err:          err,
		})
	}
}

func (m *failoverModel) startAttempt(ctx context.Context, target modelTarget, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, *llmtypes.LLMResponse, error) {
	if target.model == nil {
		return nil, nil, fmt.Errorf("chưa cấu hình model")
	}

	streamCh, err := target.model.GenerateStream(ctx, messages, tools, opts...)
	if err == nil {
		return streamCh, nil, nil
	}

	resp, genErr := target.model.Generate(ctx, messages, tools, opts...)
	if genErr != nil {
		return nil, nil, genErr
	}
	return nil, resp, nil
}
