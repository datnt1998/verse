package host

import (
	"log/slog"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
)

// DiagView 是 web 端 /diag 面板所需的一次完整诊断（GAP-28）：诊断报告、
// 概览补充（开写时间、卷进度）与已渲染的脱敏导出 Markdown。
type DiagView struct {
	Report        diag.Report
	StartedAt     *time.Time
	CurrentVolume int
	TotalVolumes  int
	Export        []byte
}

// DiagnoseView 运行一次诊断（创作规则 + 运行时抓取），同时把脱敏导出落盘到
// meta/diag-export.md（与 TUI /diag 一致）并随结果返回其内容。
func (h *Host) DiagnoseView() DiagView {
	rep, rc := diag.Diagnose(h.store)
	if _, err := diag.WriteExport(h.store, rep, rc); err != nil {
		slog.Warn("Ghi file chẩn đoán đã che giấu thất bại", "module", "host", "err", err)
	}
	v := DiagView{Report: rep, Export: diag.RenderExport(rep, rc)}
	if meta, err := h.store.RunMeta.Load(); err == nil && meta != nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(meta.StartedAt)); err == nil {
			v.StartedAt = &t
		}
	}
	if progress, err := h.store.Progress.Load(); err == nil && progress != nil && progress.Layered {
		v.CurrentVolume = progress.CurrentVolume
		if volumes, err := h.store.Outline.LoadLayeredOutline(); err == nil {
			v.TotalVolumes = len(volumes)
		}
	}
	return v
}

func (h *Host) Diagnose() diag.Report { return h.DiagnoseView().Report }
