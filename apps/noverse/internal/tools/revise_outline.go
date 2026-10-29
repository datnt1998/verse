package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/schema"
	"github.com/datnt1998/verse/core/llmcontract"
)

// ReviseOutlineTool sửa đại cương tương lai hoặc đối chiếu nhãn giữ chân của chương đã xong.
type ReviseOutlineTool struct {
	store *store.Store
	pack  RetentionPack
}

func NewReviseOutlineTool(store *store.Store, packs ...RetentionPack) *ReviseOutlineTool {
	t := &ReviseOutlineTool{store: store}
	if len(packs) > 0 {
		t.pack = packs[0]
	}
	return t
}

func (t *ReviseOutlineTool) Name() string  { return "revise_outline" }
func (t *ReviseOutlineTool) Label() string { return "Tu chỉnh đại cương" }
func (t *ReviseOutlineTool) Description() string {
	return "Tu chỉnh phần đại cương chưa diễn ra. Từ from_chapter, dùng replacement thay hoàn toàn phần kế hoạch phía sau: " +
		"đại cương phẳng thay đoạn cuối của cả sách, đại cương phân tầng thay đoạn cuối của cung chứa chương đó; chương đã xong hoặc đang viết không được di chuyển. " +
		"Các chương phía sau cần giữ lại phải được đưa vào replacement. " +
		"Riêng reconcile_completed=true: sau khi read_chapter(source=final) xác nhận chính văn, replacement phải có đúng một chương đã hoàn thành; " +
		"giữ nguyên title/core_event/hook/scenes, chỉ đồng bộ open_questions/promise/hook_type/midpoint_twist cho đúng tình tiết đã có. Không sửa chính văn hoặc đưa chương vào hàng đợi viết lại."
}

func (t *ReviseOutlineTool) ReadOnly(json.RawMessage) bool        { return false }
func (t *ReviseOutlineTool) ConcurrencySafe(json.RawMessage) bool { return false }
func (t *ReviseOutlineTool) StrictSchema() bool                   { return true }

func (t *ReviseOutlineTool) Schema() map[string]any {
	properties := []schema.Prop{
		schema.Property("title", schema.String("Tiêu đề chương")).Required(),
		schema.Property("core_event", schema.String("Sự kiện chính của chương này")).Required(),
		schema.Property("hook", schema.String("Điểm móc cuối chương")).Required(),
		schema.Property("scenes", schema.Array("Cảnh dự kiến; mảng rỗng nếu không có", schema.String(""))).Required(),
	}
	entry := schema.Object(append(properties, retentionProperties(true)...)...)
	return schema.Object(
		schema.Property("from_chapter", schema.Int("Chương bắt đầu thay đoạn tương lai; hoặc chương đã hoàn thành cần đối chiếu nhãn giữ chân")).Required(),
		schema.Property("reconcile_completed", llmcontract.Nullable(schema.Bool("true để đối chiếu nhãn giữ chân của đúng một chương đã hoàn thành; false/null để sửa đại cương tương lai"))).Required(),
		schema.Property("replacement", schema.Array("Đoạn cuối tương lai cần thay; riêng reconcile_completed=true có đúng một chương, giữ nguyên title/core_event/hook/scenes", entry)).Required(),
		schema.Property("reason", schema.String("Lý do của lần tu chỉnh này")).Required(),
	)
}

func (t *ReviseOutlineTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		FromChapter        int                   `json:"from_chapter"`
		ReconcileCompleted bool                  `json:"reconcile_completed"`
		Replacement        []domain.OutlineEntry `json:"replacement"`
		Reason             string                `json:"reason"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if input.FromChapter <= 0 {
		return nil, fmt.Errorf("from_chapter must be > 0: %w", errs.ErrToolArgs)
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, fmt.Errorf("reason không được rỗng: %w", errs.ErrToolArgs)
	}
	var raw struct {
		Replacement json.RawMessage `json:"replacement"`
	}
	if err := json.Unmarshal(args, &raw); err != nil {
		return nil, err
	}
	if err := validateRetention(t.store, t.pack, raw.Replacement, true); err != nil {
		return nil, err
	}

	total := 0
	var err error
	if input.ReconcileCompleted {
		if len(input.Replacement) != 1 {
			return nil, fmt.Errorf("Đối chiếu chương đã hoàn thành cần đúng một chương replacement: %w", errs.ErrToolArgs)
		}
		progress, loadErr := t.store.Progress.Load()
		if loadErr != nil {
			return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, loadErr)
		}
		if progress == nil || !slices.Contains(progress.CompletedChapters, input.FromChapter) ||
			progress.InProgressChapter == input.FromChapter || slices.Contains(progress.PendingRewrites, input.FromChapter) {
			return nil, fmt.Errorf("Chỉ đối chiếu chương đã hoàn thành, không đang viết hoặc chờ viết lại: %w", errs.ErrToolPrecondition)
		}
		prose, loadErr := t.store.Drafts.LoadChapterText(input.FromChapter)
		if loadErr != nil {
			return nil, fmt.Errorf("load accepted prose: %w: %w", errs.ErrStoreRead, loadErr)
		}
		if strings.TrimSpace(prose) == "" {
			return nil, fmt.Errorf("Chưa có chính văn bản cuối để đối chiếu: %w", errs.ErrToolPrecondition)
		}
		err = t.store.Outline.ReconcileChapterRetention(input.FromChapter, input.Replacement[0])
		total = progress.TotalChapters
	} else {
		total, err = t.store.ReviseOutline(input.FromChapter, input.Replacement)
	}
	if err != nil {
		return nil, fmt.Errorf("revise outline: %w", err)
	}
	artifact := "outline.json"
	result := map[string]any{
		"revised":      true,
		"from_chapter": input.FromChapter,
		"replacement":  len(input.Replacement),
		"reason":       strings.TrimSpace(input.Reason),
	}
	if input.ReconcileCompleted {
		result["reconciled_completed"] = true
	}
	progress, err := t.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("load progress after revise: %w: %w", errs.ErrStoreRead, err)
	}
	if progress != nil && progress.Layered {
		artifact = "layered_outline.json"
		outline, outlineErr := t.store.Outline.LoadOutline()
		if outlineErr != nil {
			return nil, fmt.Errorf("load outlined chapters after revise: %w: %w", errs.ErrStoreRead, outlineErr)
		}
		result["dynamic_planning"] = true
		result["outlined_chapters"] = len(outline)
	} else {
		result["total_chapters"] = total
	}
	if _, err := t.store.Checkpoints.AppendArtifact(domain.GlobalScope(), "revise_outline", artifact); err != nil {
		return nil, fmt.Errorf("checkpoint revise_outline: %w: %w", errs.ErrStoreWrite, err)
	}
	if !input.ReconcileCompleted {
		if err := t.store.Outline.ClearOutlineFeedback(); err != nil {
			return nil, fmt.Errorf("clear outline feedback: %w: %w", errs.ErrStoreWrite, err)
		}
	}

	return json.Marshal(result)
}
