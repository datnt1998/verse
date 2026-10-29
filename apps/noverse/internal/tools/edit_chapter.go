package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	agentcoretools "github.com/datnt1998/verse/apps/noverse/internal/agent/tools"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// EditChapterTool 对章节草稿做定点字符串替换，适用于打磨场景。
// 相比 draft_chapter 整章重写，token 节省 10x+。
//
// 落盘契约：只改 drafts/{ch:02d}.draft.md，禁止直接改 chapters/（终稿由 commit_chapter 独占）。
// Seed 语义：drafts 不存在但 chapters 有 → 自动把 chapters 复制到 drafts 作为起点。
// 归属检查：仅允许编辑已完成且位于 PendingRewrites 队列中的章节。
//
// 本工具复用 agentcore 的机械匹配算法（ApplyProseEdit）但**在内存里**完成替换，
// 再通过 DraftStore 落盘：章节正文归 store 所有，没有文件系统编辑路径，DB 模式
// 也不会碰引擎目录。
type EditChapterTool struct {
	store *store.Store
}

func NewEditChapterTool(s *store.Store) *EditChapterTool {
	return &EditChapterTool{store: s}
}

func (t *EditChapterTool) Name() string  { return "edit_chapter" }
func (t *EditChapterTool) Label() string { return "Sửa chương" }

// ReadOnly 明确声明写工具（配合 ConcurrencySafeTool 防止被并发调度）。
func (t *EditChapterTool) ReadOnly(_ json.RawMessage) bool { return false }

// ConcurrencySafe 显式禁止并发：同章节多次 edit_chapter 并行会读-改-写竞态，
// 即使不同章节并行也会穿插 checkpoint 顺序。统一串行最稳。
func (t *EditChapterTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

// ActivityDescription 供 UI/日志展示当前工具的活动描述。
func (t *EditChapterTool) ActivityDescription(_ json.RawMessage) string {
	return "Đang sửa bản nháp chương"
}

func (t *EditChapterTool) Description() string {
	return "Chỉ sửa một chỗ chuỗi cụ thể trong bản nháp của chương đã hoàn thành và đang nằm trong hàng đợi PendingRewrites (lựa chọn ưu tiên khi gọt giũa, tiết kiệm token hơn viết lại cả chương bằng draft_chapter)." +
		"Bản thảo sơ khởi của chương mới không được dùng công cụ này; bản thảo sơ khởi có lỗi nghiêm trọng thì gọi draft_chapter(mode=\"write\") để ghi đè toàn chương." +
		"Tìm old_string rồi thay bằng new_string; chỉ dung sai khác biệt Unicode NFC, dấu ngoặc kép, dấu ba chấm và khoảng trắng, không tự đoán lại nội dung; nhiều chỗ tương đương phải dùng replace_all=true." +
		"old_string phải chép nguyên văn từ kết quả read_chapter(source=\"draft\") gần nhất, không được dựng lại theo trí nhớ; " +
		"lưu ý giá trị trả về là chuỗi JSON, \\n phải khôi phục thành xuống dòng thật. Sau khi draft_chapter ghi lại bản nháp, phải read_chapter lại rồi mới sửa." +
		"Lỗi khớp thất bại sẽ kèm đoạn gần nhất trong bản nháp, hãy chép nguyên văn từ đoạn đó rồi thử lại." +
		"Ghi vào drafts/{ch}.draft.md; nếu drafts chưa có sẽ tự gieo từ chapters." +
		"Từ chối thực thi khi chương đã hoàn thành mà không nằm trong PendingRewrites. Mỗi lần gọi chỉ sửa một chỗ, nhiều chỗ thì gọi nhiều lần."
}

func (t *EditChapterTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương")).Required(),
		schema.Property("old_string", schema.String("Đoạn văn gốc chép từ bản nháp hiện tại; dung sai khác biệt Unicode, dấu ngoặc kép, dấu ba chấm và khoảng trắng; khi không dùng replace_all thì phải khớp duy nhất")).Required(),
		schema.Property("new_string", schema.String("Văn bản mới sau khi thay")).Required(),
		schema.Property("replace_all", schema.Bool("Thay mọi chỗ khớp (mặc định false)")),
	)
}

func (t *EditChapterTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter    int    `json:"chapter"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0: %w", errs.ErrToolArgs)
	}
	if a.OldString == "" {
		return nil, fmt.Errorf("old_string không được rỗng: %w", errs.ErrToolArgs)
	}
	if a.OldString == a.NewString {
		return nil, fmt.Errorf("old_string giống new_string, không cần sửa: %w", errs.ErrToolArgs)
	}
	// 准入按"编辑引入的新违规"判定：片段本身不是整章，mid-chapter 替换不应背
	// 首末段的元叙述规则；先通过前置检查，再在写出后与改动前做差集。
	if err := t.store.Progress.ValidateChapterWork(a.Chapter); err != nil {
		return nil, err
	}

	// 归属检查：机械落实 writer 协议。新章初稿只能整章覆盖，不能依赖
	// 模型自行遵守提示词后仍把脆弱的精确编辑暴露为可执行路径。
	completed, err := t.store.Progress.IsChapterCompleted(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if !completed {
		return nil, fmt.Errorf("chương %d chưa hoàn thành, bản thảo sơ khởi không được dùng edit_chapter; nếu có lỗi nghiêm trọng hãy gọi draft_chapter(mode=\"write\", chapter=%d) để ghi đè toàn chương: %w", a.Chapter, a.Chapter, errs.ErrToolPrecondition)
	}
	progress, err := t.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if progress == nil || !slices.Contains(progress.PendingRewrites, a.Chapter) {
		return nil, fmt.Errorf("chương %d đã hoàn thành và không nằm trong PendingRewrites, không thể sửa; muốn sửa hãy để editor thẩm duyệt kích hoạt viết lại/gọt giũa trước: %w", a.Chapter, errs.ErrToolPrecondition)
	}
	if err := EnsureChapterExpanded(t.store, a.Chapter); err != nil {
		return nil, err
	}

	// Seed：drafts 不存在时从 chapters 复制一份作为起点
	if err := t.ensureDraft(a.Chapter); err != nil {
		return nil, err
	}

	before, err := t.store.Drafts.LoadDraft(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load draft: %w: %w", errs.ErrStoreRead, err)
	}
	beforeViolations := rules.Admission(before, rules.Draft)

	// 机械匹配 + 内存替换：与 agentcore 的 prose edit 同一算法，不写文件。
	applied, err := agentcoretools.ApplyProseEdit(before, a.OldString, a.NewString, a.ReplaceAll)
	if err != nil {
		return nil, fmt.Errorf("apply edit: %w: %w", errs.ErrToolPrecondition, err)
	}
	if err := t.store.Drafts.SaveDraft(a.Chapter, applied.Content); err != nil {
		return nil, fmt.Errorf("save draft: %w: %w", errs.ErrStoreWrite, err)
	}

	after, err := t.store.Drafts.LoadDraft(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load draft: %w: %w", errs.ErrStoreRead, err)
	}
	// 剧本场景标题就地删掉，不退回给 writer：整章重发代价远大于漏一处标题。
	// 落盘后草稿里不再有这类整行，Admission 的 scene_heading 只兜底漏网内容。
	cleaned, removedHeadings := rules.StripStandaloneSceneHeadings(after)
	if len(removedHeadings) > 0 {
		if err := t.store.Drafts.SaveDraft(a.Chapter, cleaned); err != nil {
			return nil, fmt.Errorf("save cleaned draft: %w: %w", errs.ErrStoreWrite, err)
		}
		after = cleaned
	}
	if vs := introducedViolations(beforeViolations, rules.Admission(after, rules.Draft)); len(vs) > 0 {
		if revertErr := t.store.Drafts.SaveDraft(a.Chapter, before); revertErr != nil {
			slog.ErrorContext(ctx, "Rollback draft thất bại sau khi chặn viết lại", "chapter", a.Chapter, "err", revertErr)
		}
		afterChapterWrite(t.store, a.Chapter, before, nil)
		return nil, admissionError(vs)
	}

	if _, err := t.store.Checkpoints.AppendArtifact(
		domain.ChapterScope(a.Chapter), "edit",
		fmt.Sprintf("drafts/%02d.draft.md", a.Chapter),
	); err != nil {
		return nil, fmt.Errorf("checkpoint edit: %w: %w", errs.ErrStoreWrite, err)
	}
	afterChapterWrite(t.store, a.Chapter, after, nil)

	// 附加指引：让 writer 知道后续步骤，避免遗漏 check_consistency / commit_chapter
	passthrough := map[string]any{
		"message":            fmt.Sprintf("Successfully replaced text in drafts/%02d.draft.md.", a.Chapter),
		"diff":               applied.Diff,
		"first_changed_line": applied.FirstLine,
		"chapter":            a.Chapter,
		"next_step":          "edit đã ghi vào bản nháp của truyện. Còn lỗi nghiêm trọng thì có thể edit_chapter tiếp; nếu không thì check_consistency rồi commit_chapter",
	}
	noteRemovedSceneHeadings(passthrough, removedHeadings)
	return json.Marshal(passthrough)
}

// ensureDraft 保证 drafts/{ch}.draft.md 存在：
//   - 已有草稿 → 直接返回
//   - 无草稿但有终稿 → 把终稿复制到 drafts 作为修改起点（常见于打磨场景）
//   - 都没有 → 报错，提示先用 draft_chapter 创建初稿
func (t *EditChapterTool) ensureDraft(chapter int) error {
	draft, err := t.store.Drafts.LoadDraft(chapter)
	if err != nil {
		return fmt.Errorf("load draft: %w: %w", errs.ErrStoreRead, err)
	}
	if draft != "" {
		return nil
	}
	text, err := t.store.Drafts.LoadChapterText(chapter)
	if err != nil {
		return fmt.Errorf("load chapter: %w: %w", errs.ErrStoreRead, err)
	}
	if text == "" {
		return fmt.Errorf("chương %d không có bản nháp cũng không có bản cuối, hãy gọi draft_chapter(mode=write, chapter=%d) để tạo bản thảo sơ khởi: %w", chapter, chapter, errs.ErrToolPrecondition)
	}
	if err := t.store.Drafts.SaveDraft(chapter, text); err != nil {
		return fmt.Errorf("seed draft from chapter: %w: %w", errs.ErrStoreWrite, err)
	}
	return nil
}

// introducedViolations 返回 after 中新增、before 里不存在的违规（按规则/对象/
// 文案做多重集差）。edit_chapter 只应拦截本次替换引入的机械残片；草稿里既有
// 的残片归 draft_chapter/commit 链路管，反而允许编辑去修掉它。
func introducedViolations(before, after []rules.Violation) []rules.Violation {
	type key struct {
		rule, target, message string
		severity              rules.Severity
	}
	remaining := make(map[key]int, len(before))
	for _, v := range before {
		remaining[key{v.Rule, v.Target, v.Message, v.Severity}]++
	}
	var out []rules.Violation
	for _, v := range after {
		k := key{v.Rule, v.Target, v.Message, v.Severity}
		if remaining[k] > 0 {
			remaining[k]--
			continue
		}
		out = append(out, v)
	}
	return out
}
