package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// DraftChapterTool 写入整章草稿，替代旧的 write_scene + polish_chapter 流水线。
// Agent 自主决定一次写完还是分批续写。
type DraftChapterTool struct {
	store *store.Store
}

func NewDraftChapterTool(store *store.Store) *DraftChapterTool {
	return &DraftChapterTool{store: store}
}

func (t *DraftChapterTool) Name() string { return "draft_chapter" }
func (t *DraftChapterTool) Description() string {
	return "Ghi nội dung chương. mode=write ghi đè toàn chương, mode=append nối vào bản nháp hiện có (viết tiếp/sửa). Khi append, không nhắc lại phần đuôi đã có trong bản nháp: đoạn lặp sẽ bị cắt tự động, còn nếu toàn bộ nội dung gửi lên đã có sẵn thì lệnh bị từ chối — hãy viết tiếp từ sau câu cuối"
}
func (t *DraftChapterTool) Label() string { return "Ghi chương" }

// 写工具，禁止并发（读-改-写竞态）。
func (t *DraftChapterTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *DraftChapterTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *DraftChapterTool) Schema() map[string]any {
	// mode 标 required 是为了兼容 OpenAI strict tool calling——strict 模式
	// 要求所有 properties 都在 required 列表中。原来的"省略 mode 走 write
	// 默认"行为现在需要模型显式传 mode="write"，Execute 的 default 分支不变。
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương")).Required(),
		schema.Property("content", schema.String("Chính văn chương")).Required(),
		schema.Property("mode", schema.Enum("Chế độ ghi", "write", "append")).Required(),
	)
}

// StrictSchema 要求 Provider 保证工具参数符合 schema。
func (t *DraftChapterTool) StrictSchema() bool { return true }

func (t *DraftChapterTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter int    `json:"chapter"`
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0: %w", errs.ErrToolArgs)
	}
	if a.Content == "" {
		return nil, fmt.Errorf("content must not be empty: %w", errs.ErrToolArgs)
	}
	if err := t.store.Progress.ValidateChapterWork(a.Chapter); err != nil {
		return nil, err
	}
	if err := EnsureChapterExpanded(t.store, a.Chapter); err != nil {
		return nil, err
	}
	completed, err := t.store.Progress.IsChapterCompleted(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if completed {
		// 打磨/重写路径：章节虽已完成，但仍在 pending_rewrites 中，允许覆盖草稿
		progress, err := t.store.Progress.Load()
		if err != nil {
			return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
		}
		inRewriteQueue := progress != nil && slices.Contains(progress.PendingRewrites, a.Chapter)
		if !inRewriteQueue {
			return json.Marshal(map[string]any{
				"chapter":   a.Chapter,
				"skipped":   true,
				"completed": true,
				"reason":    fmt.Sprintf("chương %d đã nộp hoàn tất, không thể ghi đè", a.Chapter),
			})
		}
	}
	content, removedHeadings := rules.StripStandaloneSceneHeadings(a.Content)
	if content == "" {
		return nil, fmt.Errorf("chính văn chỉ có dòng tiêu đề cảnh kiểu kịch bản, không còn nội dung để ghi: %w", errs.ErrToolArgs)
	}
	if vs := rules.Admission(content, rules.Draft); len(vs) > 0 {
		return nil, admissionError(vs)
	}
	if err := t.store.Progress.StartChapter(a.Chapter); err != nil {
		return nil, fmt.Errorf("mark chapter in progress: %w", err)
	}

	switch a.Mode {
	case "append":
		existing, err := t.store.Drafts.LoadDraft(a.Chapter)
		if err != nil {
			return nil, fmt.Errorf("load draft before append: %w", err)
		}
		// 防重叠：模型常把草稿结尾重抄一遍再续写，先按规整比对剪掉开头重复段。
		appended, trimmed := rules.TrimLeadingOverlap(draftTailRunes(existing, appendTailWindow), content, appendOverlapMinRunes)
		if trimmed > 0 && !hasLetterRune(appended) {
			return nil, fmt.Errorf("toàn bộ đoạn gửi lên đã có trong bản nháp; viết tiếp từ sau câu cuối: %w", errs.ErrToolArgs)
		}
		if err := t.store.Drafts.AppendDraft(a.Chapter, appended); err != nil {
			return nil, fmt.Errorf("append draft: %w", err)
		}
		full, err := t.store.Drafts.LoadDraft(a.Chapter)
		if err != nil {
			return nil, fmt.Errorf("load draft after append: %w", err)
		}
		afterChapterWrite(t.store, a.Chapter, full, nil)
		if _, err := t.store.Checkpoints.AppendArtifact(
			domain.ChapterScope(a.Chapter), "draft",
			fmt.Sprintf("drafts/%02d.draft.md", a.Chapter),
		); err != nil {
			return nil, fmt.Errorf("checkpoint draft: %w", err)
		}
		out := map[string]any{
			"written":    true,
			"chapter":    a.Chapter,
			"mode":       "append",
			"word_count": domain.WordCount(full),
			"next_step":  "Trước hết read_chapter(source=draft) để đọc lại bản nháp, rồi gọi check_consistency, cuối cùng commit_chapter",
		}
		if trimmed > 0 {
			out["trimmed_overlap_runes"] = trimmed
		}
		noteRemovedSceneHeadings(out, removedHeadings)
		return json.Marshal(out)
	default: // write
		if err := t.store.Drafts.SaveDraft(a.Chapter, content); err != nil {
			return nil, fmt.Errorf("save draft: %w", err)
		}
		afterChapterWrite(t.store, a.Chapter, content, nil)
		if _, err := t.store.Checkpoints.AppendArtifact(
			domain.ChapterScope(a.Chapter), "draft",
			fmt.Sprintf("drafts/%02d.draft.md", a.Chapter),
		); err != nil {
			return nil, fmt.Errorf("checkpoint draft: %w", err)
		}
		out := map[string]any{
			"written":    true,
			"chapter":    a.Chapter,
			"mode":       "write",
			"word_count": domain.WordCount(content),
			"next_step":  "Trước hết read_chapter(source=draft) để đọc lại bản nháp, rồi gọi check_consistency, cuối cùng commit_chapter",
		}
		noteRemovedSceneHeadings(out, removedHeadings)
		return json.Marshal(out)
	}
}

// noteRemovedSceneHeadings 把"刚被机械删掉的剧本场景标题"写进工具回执：writer
// 知道哪一行没了，也就不会以为是内容丢失；提示它把地点/时间写进叙述。
func noteRemovedSceneHeadings(out map[string]any, removed []string) {
	if len(removed) == 0 {
		return
	}
	out["removed_scene_headings"] = removed
	out["scene_heading_note"] = fmt.Sprintf(
		"Đã tự bỏ %d dòng tiêu đề cảnh kiểu kịch bản (%s) trước khi lưu; chính văn tiểu thuyết không dùng nhãn CẢNH/NỘI./NGOẠI. Nếu cần, hãy đưa bối cảnh và thời gian của dòng đó vào câu kể ở phần tới.",
		len(removed), strings.Join(removed, " | "),
	)
}

// appendTailWindow / appendOverlapMinRunes 是 append 防重叠的窗口与门槛。
// 只拿草稿末尾 1600 rune 比对：重复段若存在必落在衔接处；minRunes=48 约
// 一句话的规模，短引用与常用句式不会误剪。
const (
	appendTailWindow      = 1600
	appendOverlapMinRunes = 48
)

// draftTailRunes 取草稿末尾 n 个 rune 参与重叠比对。
func draftTailRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}

// hasLetterRune 判断剩余文本是否还含字母。trim 后只剩标点/空白说明模型
// 发来的内容整体是草稿结尾的重复，按参数错误拒绝而不写入。
func hasLetterRune(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

// admissionError 把准入检查的机械事故拼成参数错误：模型据此知道该改什么。
func admissionError(vs []rules.Violation) error {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		msg := v.Message
		if msg == "" {
			msg = v.Rule
		}
		parts = append(parts, msg)
	}
	return fmt.Errorf("%s: %w", strings.Join(parts, "; "), errs.ErrToolArgs)
}
