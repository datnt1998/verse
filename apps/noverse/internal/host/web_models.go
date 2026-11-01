package host

import (
	"os"
	"sort"
	"strings"
	"time"

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

// ReloadModelSelection applies the app-wide provider/model/thinking selection
// saved by the settings screen to this running story: the provider choice no
// longer belongs to a story, so a story that is running or paused must adopt
// the shared one on the next turn instead of keeping the provider it booted
// with (e.g. a subscription that has just expired). In-flight requests finish
// on the old client; SwappableModel addresses stay stable for assembled
// workers and the Arbiter.
func (h *Host) ReloadModelSelection() error {
	h.modelSelectionMu.Lock()
	defer h.modelSelectionMu.Unlock()
	// Capture before loading: a save racing this load remains detectable at the
	// following boundary rather than stamping newer data we did not read.
	fileInfo, _ := os.Stat(bootstrap.DefaultConfigPath())
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
	if h.models == nil {
		return nil
	}
	// OutputDir là trường runtime (json:"-"), không có trong tệp cấu hình.
	cfg.OutputDir = h.cfg.OutputDir
	if err := cfg.ValidateModelReady(); err != nil {
		return err
	}
	prepared, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return err
	}
	before := selectionSummary(h.models)
	h.models.ApplyPrepared(prepared)
	h.cfg = cfg
	h.selectionFileInfo = fileInfo
	// Lựa chọn mới từ cài đặt thắng fallback tạm thời: người dùng vừa chọn
	// provider/model thì không giữ model dự phòng cũ.
	h.fallbacks = nil
	h.lastFallback = nil
	// Model client vừa đổi →下发 lại mức suy luận theo năng lực model mới
	// (ý định đã lưu giữ nguyên, chỉ giá trị hiệu lực bị kẹp lại).
	h.applyThinkingLocked("default")
	if after := selectionSummary(h.models); after != before {
		h.emitEvent(Event{
			Time: time.Now(), Category: "SYSTEM", Level: "info",
			Summary: "Đã áp dụng nhà cung cấp và model dùng chung mới: " + after,
		})
	}
	return nil
}

// refreshModelSelectionForTurn is the recovery path when an immediate gateway
// reload failed. A cheap stat at each worker boundary detects a committed
// global save; clients are rebuilt only after the file actually changed.
func (h *Host) refreshModelSelectionForTurn() error {
	info, err := os.Stat(bootstrap.DefaultConfigPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	h.mu.Lock()
	prior := h.selectionFileInfo
	unchanged := prior != nil && prior.Size() == info.Size() && prior.ModTime().Equal(info.ModTime())
	h.mu.Unlock()
	if unchanged {
		return nil
	}
	return h.ReloadModelSelection()
}

func selectionSummary(models *bootstrap.ModelSet) string {
	parts := make([]string, 0, len(concreteThinkingRoles)+1)
	for _, role := range append([]string{"default"}, concreteThinkingRoles...) {
		if provider, model, ok := models.CurrentSelection(role); ok {
			parts = append(parts, role+"="+provider+"/"+model)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
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
