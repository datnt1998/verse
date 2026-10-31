package host

import (
	"context"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
)

// SimulateSource 与 Simulate 同一条链路，但允许指定语料目录（默认 ./simulate）。
// dir 为相对路径时相对 Host 工作目录（书目录）解析。
func (h *Host) SimulateSource(ctx context.Context, dir string) (<-chan sim.Event, error) {
	if err := h.acquireExclusive("tạo hồ sơ mô phỏng văn phong"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()

	if h.simulationSources != nil {
		// Hosted samples are scoped documents, never a caller-selected host path.
		dir = "simulate"
	} else if !filepath.IsAbs(dir) {
		wd, err := os.Getwd()
		if err != nil {
			h.releaseExclusive()
			return nil, err
		}
		if dir == "" {
			dir = "simulate"
		}
		dir = filepath.Join(wd, dir)
	}
	deps := sim.Deps{
		Store: h.store,
		LLM:   h.models.ForRole("architect"),
		Prompts: sim.Prompts{
			Source: h.bundle.Prompts.SimulationSource,
			Merge:  h.bundle.Prompts.SimulationMerge,
		},
	}
	ch, err := sim.Run(ctx, deps, sim.Options{SourceDir: dir, LoadSources: h.simulationSources})
	if err != nil {
		h.releaseExclusive()
		return nil, err
	}
	return superviseExclusive(h, ch), nil
}

// AvailableThinkingLevels 返回某角色当前模型支持的推理强度字符串列表（web /models 面板用）。
func (h *Host) AvailableThinkingLevels(role string) []string {
	levels := h.AvailableThinking(role)
	out := make([]string, 0, len(levels))
	for _, l := range levels {
		out = append(out, string(l))
	}
	return out
}

// webRoleSet 是 web 端允许切换模型/推理强度的角色集合（与 TUI modelRoleOptions 一致）。
var webRoleSet = map[string]bool{"default": true, "architect": true, "writer": true, "editor": true}

// ValidWebRole 校验 web 端传入的角色名。
func ValidWebRole(role string) bool { return webRoleSet[role] }
