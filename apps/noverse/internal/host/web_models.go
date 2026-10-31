package host

import (
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/agents"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/core/models"
)

// CatalogModel 是 /model 面板中一个可选模型：上下文窗口与该模型支持的推理强度。
type CatalogModel struct {
	Name           string
	ContextWindow  int
	ContextSource  bootstrap.ContextWindowSource
	RegistryWindow int
	// Thinking 为空串 "" 表示“沿用默认”（ThinkingAuto）；按 agentcore 顺序排列。
	Thinking []string
}

// CatalogProvider 是 /model 面板中的一个 provider；Ready=凭证可用（有 API Key、
// 无需 Key，或 OAuth 已登录）。
type CatalogProvider struct {
	Name   string
	Type   string
	Ready  bool
	Models []CatalogModel
}

// ModelCatalog 列出已配置 provider 及其候选模型的能力，供 web /model 面板在保存前
// 预览每个模型可选的推理强度（与运行时钳制使用同一能力来源）。
func (h *Host) ModelCatalog() []CatalogProvider {
	snapshot := h.ModelConfiguration()
	h.mu.Lock()
	cfg := bootstrap.CloneConfig(h.cfg)
	h.mu.Unlock()

	out := make([]CatalogProvider, 0, len(snapshot.Providers))
	for _, p := range snapshot.Providers {
		ready := p.HasAPIKey || (!p.RequiresAPIKey && !p.OAuth) || (p.OAuth && p.Account != "")
		item := CatalogProvider{Name: p.Name, Type: p.Type, Ready: ready, Models: make([]CatalogModel, 0, len(p.Models))}
		for _, m := range p.Models {
			window, source := cfg.ResolveContextWindow(p.Name, m.Name)
			item.Models = append(item.Models, CatalogModel{
				Name: m.Name, ContextWindow: window, ContextSource: source,
				RegistryWindow: models.DefaultRegistry().ResolveContextWindow(m.Name), Thinking: modelThinkingLevels(cfg, p.Name, m.Name),
			})
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RefreshModelContexts reloads context metadata without changing active model
// selections or interrupting an in-flight request.
func (h *Host) RefreshModelContexts() error {
	var cfg bootstrap.Config
	var err error
	if h.configLoader != nil {
		cfg, err = h.configLoader()
	} else {
		cfg, err = bootstrap.LoadConfigAt(h.Dir())
	}
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg.ApplyContextWindows(cfg)
	if h.models == nil {
		return nil
	}
	h.models.ApplyContextWindows(cfg)
	for _, role := range []string{"default", "architect", "writer", "editor"} {
		provider, model, ok := h.models.CurrentSelection(role)
		if !ok {
			continue
		}
		window, source := h.cfg.ResolveContextWindow(provider, model)
		bootstrap.LogContextWindowChoice(role, model, window, source)
	}
	return nil
}

func modelThinkingLevels(cfg bootstrap.Config, provider, model string) []string {
	probe, err := cfg.ProbeModel(provider, model)
	if err != nil {
		return []string{""}
	}
	levels := agents.AvailableThinkingForModel(probe)
	out := make([]string, 0, len(levels))
	for _, l := range levels {
		out = append(out, strings.TrimSpace(string(l)))
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}
