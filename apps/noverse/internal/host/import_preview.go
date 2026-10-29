package host

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/noverse/internal/host/imp"
	"github.com/datnt1998/noverse/internal/tools"
)

// ImportPreviewTimeout bounds the synchronous HTTP preview and its model calls.
const ImportPreviewTimeout = 20 * time.Second

func (h *Host) previewCaller() imp.Caller {
	role := h.importRole("segment")
	// ForRole deliberately omits ForRoleWithFailover's retry gate and fallbacks;
	// each preview call is tried once. Structured-output correction shares the deadline.
	model := newUsageTrackedModel(h.models.ForRole(role), role, h.usage.Record)
	return imp.Caller{Model: interactiveModel{ChatModel: model, singleAttempt: true}, Runtime: h.importModelRuntime(role, model)}
}

func (h *Host) importDeps() imp.Deps {
	return imp.Deps{
		Store:         h.store,
		CommitChapter: tools.NewCommitChapterTool(h.store, h.styleStats, h.packRules).WithAuthorSource(),
		Segment:       h.importCaller("segment"),
		Analyze:       h.importCaller("analyze"),
		Synthesize:    h.importCaller("synthesize"),
		RetryGate:     h.retryGate,
		Prompts:       imp.Prompts{Segment: h.bundle.Prompts.ImportSegment, Analyze: h.bundle.Prompts.ImportAnalyze, Synthesize: h.bundle.Prompts.ImportSynthesize, Range: h.bundle.Prompts.ImportRange},
	}
}

// ImportPreview shares the import exclusive slot and model configuration. It
// stages only the source snapshot and segmentation, never published chapters.
func (h *Host) ImportPreview(ctx context.Context, opts imp.Options) (imp.Preview, error) {
	if h == nil || !h.ModelReady() || h.engine == nil {
		return imp.Preview{}, fmt.Errorf("Chưa có cấu hình mô hình — thiết lập ở Cài đặt › Nhà cung cấp")
	}
	if err := h.budget.Refuse(); err != nil {
		return imp.Preview{}, err
	}
	if err := h.acquireExclusive("chia chương nhập"); err != nil {
		return imp.Preview{}, err
	}
	defer h.releaseExclusive()
	ctx, cancel := context.WithTimeout(ctx, ImportPreviewTimeout)
	defer cancel()
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()
	// Preview only ingests and segments; the background import still gets failover/retry deps.
	preview, err := imp.PreparePreview(ctx, imp.Deps{
		Store:   h.store,
		Segment: h.previewCaller(),
		Prompts: imp.Prompts{Segment: h.bundle.Prompts.ImportSegment},
	}, opts)
	if errors.Is(err, context.DeadlineExceeded) {
		return imp.Preview{}, fmt.Errorf("Không kết nối được nhà cung cấp AI — xem trước quá thời gian chờ 20 giây; hãy kiểm tra nhà cung cấp và thử lại: %w", err)
	}
	return preview, userFacingProviderError(err)
}

func (h *Host) ImportState() (imp.PreviewState, error) {
	return imp.LoadPreviewState(h.store.Dir())
}
