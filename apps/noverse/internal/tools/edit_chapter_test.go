package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func enterEditWritingPhase(t *testing.T, s *store.Store) {
	t.Helper()
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
}

func queueCompletedChapterForEdit(t *testing.T, s *store.Store, chapter int, wordCount int) {
	t.Helper()
	if err := s.Progress.MarkChapterComplete(chapter, wordCount, "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.SetPendingRewrites([]int{chapter}, "测试打磨", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.Progress.SetFlow(domain.FlowPolishing); err != nil {
		t.Fatalf("SetFlow: %v", err)
	}
}

// TestEditChapterAppliesEdit 正常路径：drafts 已有内容，唯一匹配替换成功。
func TestEditChapterAppliesEdit(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	if err := s.Drafts.SaveDraft(2, "他握紧了拳头，指节发白。"); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune("他握紧了拳头，指节发白。")))

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "指节发白",
		"new_string": "指节泛起青白",
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := s.Drafts.LoadDraft(2)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if !strings.Contains(got, "指节泛起青白") {
		t.Fatalf("expected draft to contain new text, got %q", got)
	}
	if strings.Contains(got, "指节发白") {
		t.Fatalf("old text should be replaced, got %q", got)
	}
}

func TestEditChapterRejectsIncompleteChapter(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "新章初稿必须整章覆盖。"
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "必须",
		"new_string": "应当",
	})
	_, err := NewEditChapterTool(s).Execute(context.Background(), args)
	if err == nil || !errors.Is(err, errs.ErrToolPrecondition) {
		t.Fatalf("未完成章节应被明确拒绝，got %v", err)
	}
	if !strings.Contains(err.Error(), `draft_chapter(mode="write"`) {
		t.Fatalf("错误应指向初稿整章覆盖路径，got %v", err)
	}
	got, loadErr := s.Drafts.LoadDraft(2)
	if loadErr != nil {
		t.Fatalf("LoadDraft: %v", loadErr)
	}
	if got != original {
		t.Fatalf("拒绝后不应修改草稿，got %q", got)
	}
}

// TestEditChapterSeedsFromFinalChapter drafts 不存在但 chapters 有 → 自动从 chapters 播种。
func TestEditChapterSeedsFromFinalChapter(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)

	// 模拟第 3 章已提交且进入打磨队列
	original := "风从窗缝里钻进来，带着潮湿的泥土气味。"
	if err := s.Drafts.SaveFinalChapter(3, original); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, len([]rune(original)), "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.SetPendingRewrites([]int{3}, "测试打磨", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.Progress.SetFlow(domain.FlowPolishing); err != nil {
		t.Fatalf("SetFlow: %v", err)
	}

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    3,
		"old_string": "潮湿的泥土气味",
		"new_string": "泥土和铁锈混杂的气味",
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// drafts 应被播种且包含新文本
	draft, err := s.Drafts.LoadDraft(3)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if !strings.Contains(draft, "泥土和铁锈混杂的气味") {
		t.Fatalf("expected draft seeded + edited, got %q", draft)
	}

	// chapters 保持原样（edit_chapter 不碰终稿）
	final, err := s.Drafts.LoadChapterText(3)
	if err != nil {
		t.Fatalf("LoadChapterText: %v", err)
	}
	if final != original {
		t.Fatalf("final chapter must stay untouched, got %q", final)
	}
}

// TestEditChapterRejectsCompletedWithoutQueue 已完成且不在重写队列中 → 拒绝。
func TestEditChapterRejectsCompletedWithoutQueue(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "第二章原始正文。"
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(2, original); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(2, len([]rune(original)), "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "原始正文",
		"new_string": "篡改内容",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected rejection for completed chapter not in PendingRewrites")
	}
	if !errors.Is(err, errs.ErrToolPrecondition) {
		t.Fatalf("expected ErrToolPrecondition, got %v", err)
	}
}

// TestEditChapterRejectsAmbiguousMatch 多处匹配且未开 replace_all → 报错。
func TestEditChapterRejectsAmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	if err := s.Drafts.SaveDraft(2, "他笑了。她也笑了。"); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune("他笑了。她也笑了。")))

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "笑了",
		"new_string": "沉默了",
	})
	if _, err := tool.Execute(context.Background(), args); err == nil {
		t.Fatal("expected rejection for ambiguous match")
	}
}

// TestEditChapterReplaceAll replace_all=true 时所有匹配均被替换。
func TestEditChapterReplaceAll(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	if err := s.Drafts.SaveDraft(2, "他笑了。她也笑了。"); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune("他笑了。她也笑了。")))

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":     2,
		"old_string":  "笑了",
		"new_string":  "沉默了",
		"replace_all": true,
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, _ := s.Drafts.LoadDraft(2)
	if strings.Contains(got, "笑了") {
		t.Fatalf("all occurrences should be replaced, got %q", got)
	}
	if strings.Count(got, "沉默了") != 2 {
		t.Fatalf("expected 2 replacements, got %q", got)
	}
}

// TestEditChapterRejectsEmptyOldString 空 old_string → 参数非法。
func TestEditChapterRejectsEmptyOldString(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "",
		"new_string": "xxx",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected rejection for empty old_string")
	}
	if !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("expected ErrToolArgs, got %v", err)
	}
}

// TestEditChapterRejectsNoDraftNoFinal drafts 与 chapters 都不存在 → 报错提示先 draft_chapter。
func TestEditChapterRejectsNoDraftNoFinal(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	queueCompletedChapterForEdit(t, s, 5, 0)

	tool := NewEditChapterTool(s)
	args, _ := json.Marshal(map[string]any{
		"chapter":    5,
		"old_string": "任何",
		"new_string": "替换",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected rejection when neither draft nor chapter exists")
	}
	if !errors.Is(err, errs.ErrToolPrecondition) {
		t.Fatalf("expected ErrToolPrecondition, got %v", err)
	}
}

// TestEditChapterWorksWithCommitValidation 整条链路：edit_chapter → commit_chapter 成功 drain 队列。
// 验证新工具与 commit_chapter 的 drafts≠chapters 硬校验配合良好。
func TestEditChapterWorksWithCommitValidation(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)

	original := "风从窗缝里钻进来，带着潮湿的泥土气味。"
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(2, original); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(2, len([]rune(original)), "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.SetPendingRewrites([]int{2}, "打磨", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.Progress.SetFlow(domain.FlowPolishing); err != nil {
		t.Fatalf("SetFlow: %v", err)
	}

	editTool := NewEditChapterTool(s)
	editArgs, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "潮湿的泥土气味",
		"new_string": "泥土和铁锈混杂的气味",
	})
	if _, err := editTool.Execute(context.Background(), editArgs); err != nil {
		t.Fatalf("edit_chapter: %v", err)
	}

	commitTool := newTestCommitChapterTool(s)
	commitArgs, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"title":      "第二章",
		"summary":    "打磨后摘要",
		"characters": []string{"主角"},
		"key_events": []string{"完成打磨"},
	})
	if _, err := commitTool.Execute(context.Background(), commitArgs); err != nil {
		t.Fatalf("commit_chapter after edit: %v", err)
	}

	progress, err := s.Progress.Load()
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if len(progress.PendingRewrites) != 0 {
		t.Fatalf("expected queue drained, got %v", progress.PendingRewrites)
	}
}

func TestEditChapterRejectsPlanningPhaseBeforeMutation(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	original := "规划期草稿不能被修改。"
	if err := s.Drafts.SaveDraft(1, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	args, err := json.Marshal(map[string]any{
		"chapter": 1, "old_string": "不能", "new_string": "已经",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if _, err := NewEditChapterTool(s).Execute(context.Background(), args); err == nil {
		t.Fatal("planning phase edit should be rejected")
	}
	got, err := s.Drafts.LoadDraft(1)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if got != original {
		t.Fatalf("planning phase edit mutated draft: %q", got)
	}
}

// TestEditChapterRejectsMechanicalResidue 锁死 new_string 的准入检查：
// 定点替换同样能注入思考块/代码栅栏/元叙述，必须以 ErrToolArgs 拒收
// 且不得改动草稿。
func TestEditChapterRejectsMechanicalResidue(t *testing.T) {
	for _, tc := range []struct {
		name      string
		newString string
	}{
		{name: "think tag", newString: "<think>ghép vào đây</think> đoạn mới"},
		{name: "code fence", newString: "đoạn mới\n```go\nfmt.Println()\n```"},
		{name: "meta note", newString: "đoạn mới. Hy vọng chương này đạt yêu cầu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if err := s.Progress.Init(10); err != nil {
				t.Fatalf("InitProgress: %v", err)
			}
			enterEditWritingPhase(t, s)
			original := "他握紧了拳头，指节发白。"
			if err := s.Drafts.SaveDraft(2, original); err != nil {
				t.Fatalf("SaveDraft: %v", err)
			}
			queueCompletedChapterForEdit(t, s, 2, len([]rune(original)))

			args, _ := json.Marshal(map[string]any{
				"chapter":    2,
				"old_string": "指节发白",
				"new_string": tc.newString,
			})
			_, err := NewEditChapterTool(s).Execute(context.Background(), args)
			if err == nil {
				t.Fatal("new_string 带机械残留必须被拒")
			}
			if !errors.Is(err, errs.ErrToolArgs) {
				t.Fatalf("lỗi phải là ErrToolArgs, got %v", err)
			}
			if got, _ := s.Drafts.LoadDraft(2); got != original {
				t.Fatalf("draft không được thay đổi khi bị từ chối: %q", got)
			}
		})
	}
}

// TestEditChapterAllowsMetaLikeMidChapter 片段单独看像元叙述（"hy vọng chương này"），
// 但替换发生在中段段落时整章首末段没变，不应被片段级准入误伤。
func TestEditChapterAllowsMetaLikeMidChapter(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "Mở đầu chương.\n\nĐoạn giữa một. Cô khép cửa.\n\nĐoạn kết chương."
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune(original)))

	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "Cô khép cửa.",
		"new_string": "Cô khép cửa rồi nói khẽ rằng hy vọng chương này sẽ an toàn.",
	})
	if _, err := NewEditChapterTool(s).Execute(context.Background(), args); err != nil {
		t.Fatalf("mid-chapter chỉnh sửa không được bị chặn: %v", err)
	}
	got, _ := s.Drafts.LoadDraft(2)
	if !strings.Contains(got, "hy vọng chương này sẽ an toàn") {
		t.Fatalf("đoạn mới phải được ghi: %q", got)
	}
}

// TestEditChapterRejectsIntroducedCodeFence 编辑若真的在整章里新增代码栅栏
// 行，仍然按 ErrToolArgs 拒收并回滚草稿。
func TestEditChapterRejectsIntroducedCodeFence(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "Mở đầu chương.\n\nHắn nhìn quanh.\n\nĐoạn kết chương."
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune(original)))

	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "Hắn nhìn quanh.",
		"new_string": "Hắn nhìn quanh.\n\n```go\nfmt.Println()\n```",
	})
	_, err := NewEditChapterTool(s).Execute(context.Background(), args)
	if err == nil || !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("code fence mới phải bị ErrToolArgs chặn, got %v", err)
	}
	if got, _ := s.Drafts.LoadDraft(2); got != original {
		t.Fatalf("bản nháp phải được khôi phục nguyên văn: %q", got)
	}
}

// TestEditChapterAllowsFixingExistingResidue 草稿里已有 think 残片时，把它
// 替换成正文的编辑必须放行——编辑修掉违规后整章更干净。
func TestEditChapterAllowsFixingExistingResidue(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "Mở đầu chương.\n\n<think>ghi chú còn sót</think> Hắn nhìn quanh.\n\nĐoạn kết chương."
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune(original)))

	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "<think>ghi chú còn sót</think> Hắn nhìn quanh.",
		"new_string": "Hắn nhìn quanh.",
	})
	if _, err := NewEditChapterTool(s).Execute(context.Background(), args); err != nil {
		t.Fatalf("sửa bỏ think tag phải được phép: %v", err)
	}
	got, _ := s.Drafts.LoadDraft(2)
	if strings.Contains(got, "<think") {
		t.Fatalf("think tag phải bị gỡ: %q", got)
	}
}
