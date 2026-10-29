package web

import (
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

type catalogModelJSON struct {
	Name           string                        `json:"name"`
	ContextWindow  int                           `json:"contextWindow"`
	ContextSource  bootstrap.ContextWindowSource `json:"contextSource"`
	RegistryWindow int                           `json:"registryWindow"`
	Thinking       []string                      `json:"thinking"`
}

type catalogProviderJSON struct {
	Provider string             `json:"provider"`
	Label    string             `json:"label"`
	Ready    bool               `json:"ready"`
	Models   []catalogModelJSON `json:"models"`
}

// modelCatalogJSON 把 Host.ModelCatalog 转成 GET S/models 的 `providers`（11 §5 /model 面板）。
func modelCatalogJSON(catalog []host.CatalogProvider) []catalogProviderJSON {
	out := make([]catalogProviderJSON, 0, len(catalog))
	for _, p := range catalog {
		item := catalogProviderJSON{
			Provider: p.Name, Label: providerShortLabel(p.Name, p.Type), Ready: p.Ready,
			Models: make([]catalogModelJSON, 0, len(p.Models)),
		}
		for _, m := range p.Models {
			thinking := m.Thinking
			if thinking == nil {
				thinking = []string{}
			}
			item.Models = append(item.Models, catalogModelJSON{Name: m.Name, ContextWindow: m.ContextWindow, ContextSource: m.ContextSource, RegistryWindow: m.RegistryWindow, Thinking: thinking})
		}
		out = append(out, item)
	}
	return out
}

// providerShortLabel 取 preset 名称中括号前的部分（“OpenRouter (Claude, …)” → “OpenRouter”）；
// 非 preset 的自定义 provider 按 type 再找一次，最后退回配置键名。
func providerShortLabel(name, typ string) string {
	for _, key := range []string{name, typ} {
		if key == "" {
			continue
		}
		for _, preset := range bootstrap.ProviderPresets() {
			if preset.Name != key || key == "custom" {
				continue
			}
			label := preset.Label
			if i := strings.Index(label, " ("); i > 0 {
				label = label[:i]
			}
			return label
		}
	}
	return name
}
