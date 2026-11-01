package bootstrap

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/llmconfig"
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
	generation uint64
	epoch      *atomic.Uint64
}

func NewSwappableModel(provider, name string, model llmtypes.ChatModel, jsonSchema *bool) *SwappableModel {
	return &SwappableModel{
		SwappableModel: llmtypes.NewSwappableModel(model),
		provider:       provider,
		name:           name,
		jsonSchema:     jsonSchema,
	}
}

// ModelRequestError identifies the stable slot and selection that actually
// served a failed request, including errors delivered asynchronously in a stream.
// The original chain is retained for provider classification and retry policy.
type ModelRequestError struct {
	Provider   string
	Model      string
	Generation uint64
	source     *SwappableModel
	cause      error
}

func (e *ModelRequestError) Error() string { return e.cause.Error() }
func (e *ModelRequestError) Unwrap() error { return e.cause }

func (e ModelRequestError) withCause(cause error) error {
	e.cause = cause
	return &e
}

// IsCurrent reports whether this request still belongs to its selected client.
// A swap back to the same provider/model is a new selection, not the old request.
func (e *ModelRequestError) IsCurrent() bool {
	e.source.mu.RLock()
	defer e.source.mu.RUnlock()
	return e.Generation == e.source.selectionGeneration()
}

func (m *SwappableModel) requestModel() (llmtypes.ChatModel, ModelRequestError) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.SwappableModel.Current(), ModelRequestError{
		Provider: m.provider, Model: m.name, Generation: m.selectionGeneration(), source: m,
	}
}

// Caller holds m.mu. Shared epochs invalidate request-level fallback clients
// as well as role slots when the selection changes.
func (m *SwappableModel) selectionGeneration() uint64 {
	if m.epoch != nil {
		return m.epoch.Load()
	}
	return m.generation
}

func (m *SwappableModel) setEpoch(epoch *atomic.Uint64) {
	m.mu.Lock()
	m.epoch = epoch
	m.mu.Unlock()
}

func (m *SwappableModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	model, identity := m.requestModel()
	if model == nil {
		return nil, llmtypes.ErrNoModel
	}
	response, err := model.Generate(ctx, messages, tools, opts...)
	if err != nil {
		return response, identity.withCause(err)
	}
	return response, nil
}

func (m *SwappableModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	model, identity := m.requestModel()
	if model == nil {
		return nil, llmtypes.ErrNoModel
	}
	stream, err := model.GenerateStream(ctx, messages, tools, opts...)
	if err != nil {
		return nil, identity.withCause(err)
	}
	out := make(chan llmtypes.StreamEvent)
	go func() {
		defer close(out)
		for {
			var event llmtypes.StreamEvent
			select {
			case <-ctx.Done():
				return
			case next, ok := <-stream:
				if !ok {
					return
				}
				event = next
			}
			if event.Err != nil {
				failure := identity
				failure.cause = event.Err
				event.Err = &failure
			}
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
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
	if m.epoch != nil {
		m.epoch.Add(1)
	} else {
		m.generation++
	}
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
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.roleModelLocked(role)
}

// Each assembled role retains its own stable slot, including roles initially
// inheriting the default. A later explicit override must update the model a
// running worker already holds, not allocate a disconnected replacement.
func (ms *ModelSet) roleModelLocked(role string) *SwappableModel {
	if role == "" || role == "default" || !knownRoles[role] {
		return ms.Default
	}
	if model, ok := ms.models[role]; ok {
		return model
	}
	provider, name := ms.Default.Current()
	model := NewSwappableModel(provider, name, ms.Default.SwappableModel.Current(), ms.Default.JSONSchemaOverride())
	model.setEpoch(ms.Default.epoch)
	if ms.models == nil {
		ms.models = make(map[string]*SwappableModel)
	}
	ms.models[role] = model
	return model
}

// ForRoleWithFailover 返回带有单次请求级 fallback 的角色模型。
// 仅当该角色显式配置了 fallbacks 时生效；未配置时退化为普通模型。
func (ms *ModelSet) ForRoleWithFailover(role string, report FailoverReporter) llmtypes.ChatModel {
	return ms.forRoleWithFailover(role, report, false)
}

// ForWorkerRole leaves temporary provider limits to the engine's AU policy
// (real retryAt and cross-provider fallback after two minutes). Direct import
// and revision calls keep their existing request-level retry/failover policy.
func (ms *ModelSet) ForWorkerRole(role string, report FailoverReporter) llmtypes.ChatModel {
	return ms.forRoleWithFailover(role, report, true)
}

func (ms *ModelSet) forRoleWithFailover(role string, report FailoverReporter, worker bool) llmtypes.ChatModel {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	primary := ms.roleModelLocked(role)
	if len(ms.fallbacks[role]) == 0 {
		return ms.wrap(primary, role, worker)
	}
	return ms.wrap(&failoverModel{
		role: role, primary: primary, set: ms, report: report, deferProviderLimits: worker,
	}, role, worker)
}

// Worker errors must also bypass agentcore's own retries when no shared gate
// was installed (e.g. an isolated runner). Other callers retain nil-gate behavior.
func (ms *ModelSet) wrap(m llmtypes.ChatModel, role string, worker bool) llmtypes.ChatModel {
	if m == nil || ms.retryGate == nil && !worker {
		return m
	}
	wrapped := llmretry.NewRetryingModel(m, ms.retryGate, role, llmretry.DefaultMaxRetries)
	if worker {
		wrapped.DeferError = isProviderLimit
	}
	return wrapped
}

func isProviderLimit(err error) bool {
	return errors.Is(err, llmtypes.ErrProviderRateLimit)
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
		_, explicit = ms.config.Roles[role]
		return provider, model, explicit
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
	next, err := llmconfig.NewModel(provider, model, pc, make(map[string]llmtypes.ChatModel))
	if err != nil {
		return fmt.Errorf("đổi model thất bại: %w", err)
	}

	jsonSchema := ms.config.ModelJSONSchema(provider, model)
	if role == "" || role == "default" {
		ms.Default.Swap(provider, model, next, jsonSchema)
		for inheritedRole, modelSlot := range ms.models {
			if _, explicit := ms.config.Roles[inheritedRole]; !explicit {
				modelSlot.Swap(provider, model, next, jsonSchema)
			}
		}
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
	ms.models[role].setEpoch(ms.Default.epoch)
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
			next.setEpoch(ms.Default.epoch)
		}
	}
	// Roles whose explicit override was removed still have assembled workers.
	// Reset those same slots to the new default instead of dropping the slots.
	for role, existing := range ms.models {
		if _, retained := nextModels[role]; !retained {
			existing.Swap(defaultProvider, defaultName, candidate.Default.SwappableModel.Current(), candidate.Default.JSONSchemaOverride())
			nextModels[role] = existing
		}
	}
	ms.models = nextModels
	ms.fallbacks = candidate.fallbacks
	for _, targets := range ms.fallbacks {
		for _, target := range targets {
			if slot, ok := target.model.(*SwappableModel); ok {
				slot.setEpoch(ms.Default.epoch)
			}
		}
	}
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
	defaultModel, err := llmconfig.NewModel(cfg.Provider, cfg.ModelName, defaultPC, cache)
	if err != nil {
		return nil, fmt.Errorf("model mặc định: %w", err)
	}

	ms := &ModelSet{
		Default:   NewSwappableModel(cfg.Provider, cfg.ModelName, defaultModel, cfg.ModelJSONSchema(cfg.Provider, cfg.ModelName)),
		models:    make(map[string]*SwappableModel),
		fallbacks: make(map[string][]modelTarget),
		config:    cfg,
	}
	ms.Default.setEpoch(&atomic.Uint64{})

	// 创建角色覆盖模型
	for role, rc := range cfg.Roles {
		pc, ok := cfg.Providers[rc.Provider]
		if !ok {
			return nil, fmt.Errorf("vai %s trỏ tới provider %q chưa được cấu hình: %w", role, rc.Provider, errs.ErrConfig)
		}
		m, err := llmconfig.NewModel(rc.Provider, rc.Model, pc, cache)
		if err != nil {
			return nil, fmt.Errorf("model của vai %s: %w", role, err)
		}
		ms.models[role] = NewSwappableModel(rc.Provider, rc.Model, m, cfg.ModelJSONSchema(rc.Provider, rc.Model))
		ms.models[role].setEpoch(ms.Default.epoch)
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
			fm, err := llmconfig.NewModel(fallback.Provider, fallback.Model, fpc, cache)
			if err != nil {
				return nil, fmt.Errorf("fallback %s/%s của vai %s: %w", fallback.Provider, fallback.Model, role, err)
			}
			fallbackModel := NewSwappableModel(fallback.Provider, fallback.Model, fm, cfg.ModelJSONSchema(fallback.Provider, fallback.Model))
			fallbackModel.setEpoch(ms.Default.epoch)
			targets = append(targets, modelTarget{
				provider:   fallback.Provider,
				name:       fallback.Model,
				model:      fallbackModel,
				jsonSchema: cfg.ModelJSONSchema(fallback.Provider, fallback.Model),
			})
		}
		ms.fallbacks[role] = targets
	}

	return ms, nil
}

// createModelFromConfig 已由 core/llmconfig.NewModel 承担（跨应用共享的
// provider → ChatModel 构造）；本文件只保留 ModelSet/SwappableModel/failover
// 这些 Noverse 装配层逻辑。
type failoverModel struct {
	role                string
	primary             *SwappableModel
	set                 *ModelSet
	report              FailoverReporter
	deferProviderLimits bool
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
	var request *ModelRequestError
	if errors.As(err, &request) && !request.IsCurrent() {
		return modelTarget{}, "", false
	}
	if errors.Is(err, context.Canceled) {
		return modelTarget{}, "", false
	}
	// The worker engine owns the quota countdown and two-minute threshold;
	// request-level failover must not silently switch first.
	if m.deferProviderLimits && isProviderLimit(err) {
		return modelTarget{}, "quota", false
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
	// An explicit provider rejection is not "streaming unsupported": repeating
	// it as a non-streaming call only duplicates an expired/key/quota request.
	if errors.Is(err, llmtypes.ErrProviderAuth) || errors.Is(err, llmtypes.ErrProviderQuota) || isProviderLimit(err) {
		return nil, nil, err
	}

	resp, genErr := target.model.Generate(ctx, messages, tools, opts...)
	if genErr != nil {
		return nil, nil, genErr
	}
	return nil, resp, nil
}
