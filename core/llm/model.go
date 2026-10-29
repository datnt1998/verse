package llm

// ModelPricing defines per-token cost rates in USD.
// Set rates to 0 for categories that don't apply.
type ModelPricing struct {
	InputPerToken      float64 `json:"input_per_token"`
	OutputPerToken     float64 `json:"output_per_token"`
	CacheReadPerToken  float64 `json:"cache_read_per_token"`
	CacheWritePerToken float64 `json:"cache_write_per_token"`
}

// CalculateCost computes the monetary cost from pricing rates and token usage.
// Returns nil if pricing or usage is nil.
//
// Pricing semantic: usage.Input already includes usage.CacheRead per the
// underlying litellm convention. The cached portion must only be billed at
// the cache-read rate; charging full Input at input rate AND CacheRead at
// cache-read rate would double-bill.
func CalculateCost(pricing *ModelPricing, usage *Usage) *Cost {
	if pricing == nil || usage == nil {
		return nil
	}
	nonCachedInput := usage.Input - usage.CacheRead
	if nonCachedInput < 0 {
		nonCachedInput = usage.Input
	}
	c := &Cost{
		Input:      float64(nonCachedInput) * pricing.InputPerToken,
		Output:     float64(usage.Output) * pricing.OutputPerToken,
		CacheRead:  float64(usage.CacheRead) * pricing.CacheReadPerToken,
		CacheWrite: float64(usage.CacheWrite) * pricing.CacheWritePerToken,
	}
	c.Total = c.Input + c.Output + c.CacheRead + c.CacheWrite
	return c
}

// ModelInfo contains basic model metadata.
type ModelInfo struct {
	Name         string        `json:"name"`
	Provider     string        `json:"provider"`
	Version      string        `json:"version"`
	MaxTokens    int           `json:"max_tokens"`
	ContextSize  int           `json:"context_size"`
	Capabilities []string      `json:"capabilities"`
	Pricing      *ModelPricing `json:"pricing,omitempty"`
}

// ModelCapability defines capability identifiers.
type ModelCapability string

const (
	CapabilityChat         ModelCapability = "chat"
	CapabilityCompletion   ModelCapability = "completion"
	CapabilityToolCalling  ModelCapability = "tool_calling"
	CapabilityStreaming    ModelCapability = "streaming"
	CapabilityMultimodal   ModelCapability = "multimodal"
	CapabilityFunctionCall ModelCapability = "function_call"
)

// GenerationConfig defines sampling and length control parameters.
type GenerationConfig struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

var DefaultGenerationConfig = &GenerationConfig{
	Temperature: 0.7,
	MaxTokens:   65536,
}

// BaseModel provides common model metadata and capability checks.
type BaseModel struct {
	info   ModelInfo
	config *GenerationConfig
}

func NewBaseModel(info ModelInfo, config *GenerationConfig) *BaseModel {
	if config == nil {
		config = DefaultGenerationConfig
	}
	copy := *config
	return &BaseModel{info: info, config: &copy}
}

func (m *BaseModel) Info() ModelInfo              { return m.info }
func (m *BaseModel) GetConfig() *GenerationConfig { return m.config }

// ModelName implements agentcore.ModelNamer.
func (m *BaseModel) ModelName() string { return m.info.Name }

func (m *BaseModel) SupportsCapability(capability ModelCapability) bool {
	for _, c := range m.info.Capabilities {
		if c == string(capability) {
			return true
		}
	}
	return false
}

func (m *BaseModel) SupportsTools() bool {
	return m.SupportsCapability(CapabilityToolCalling) || m.SupportsCapability(CapabilityFunctionCall)
}

func (m *BaseModel) SupportsStreaming() bool {
	return m.SupportsCapability(CapabilityStreaming)
}
