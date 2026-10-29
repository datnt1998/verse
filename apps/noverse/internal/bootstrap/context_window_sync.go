package bootstrap

import "github.com/datnt1998/verse/core/llmconfig"

// ApplyContextWindows updates only context metadata. Model selections, provider
// credentials and already-running requests remain unchanged.
func (c *Config) ApplyContextWindows(source Config) {
	c.ContextWindow = source.ContextWindow
	// Config values may share a provider map at startup. Detach the changed
	// metadata before publishing it under the caller's own lock.
	providers := make(map[string]llmconfig.ProviderConfig, len(c.Providers))
	for provider, current := range c.Providers {
		desired := source.Providers[provider]
		models := make([]llmconfig.ModelConfig, len(current.Models), len(current.Models)+len(desired.Models))
		copy(models, current.Models)
		current.Models = models
		for i := range current.Models {
			model, _ := desired.ModelConfig(current.Models[i].Name)
			current.Models[i].ContextWindow = model.ContextWindow
		}
		for _, model := range desired.Models {
			if _, exists := current.ModelConfig(model.Name); !exists {
				current.Models = append(current.Models, llmconfig.ModelConfig{Name: model.Name, ContextWindow: model.ContextWindow})
			}
		}
		providers[provider] = current
	}
	c.Providers = providers
}

func (ms *ModelSet) ApplyContextWindows(source Config) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.config.ApplyContextWindows(source)
}
