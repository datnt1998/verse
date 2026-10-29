package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/llmcontract"
)

// reviewArgs là chỗ duy nhất dựng tham số save_review cho test: fixture là giá
// trị domain.ReviewEntry có kiểu, rồi được mã hoá theo đúng hình dạng schema
// strict (trường tùy chọn bỏ trống thành null) để khớp hợp đồng với LLM.
func reviewArgs(t *testing.T, entry domain.ReviewEntry) json.RawMessage {
	t.Helper()
	args, err := llmcontract.EncodeArgs(NewSaveReviewTool(nil, nil).Schema(), entry)
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	return args
}

func TestSaveReviewPersistsContractAssessment(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	if !tool.StrictSchema() {
		t.Fatal("save_review must use strict schema")
	}
	if err := llmcontract.ValidateStrictReady(tool.Schema()); err != nil {
		t.Fatalf("save_review schema is not strict-ready: %v", err)
	}
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Verdict: "pass", Comment: "基本一致"},
			{Dimension: "character", Score: 82, Verdict: "pass", Comment: "人设稳定"},
			{Dimension: "pacing", Score: 78, Verdict: "warning", Comment: "略慢"},
			{Dimension: "continuity", Score: 84, Verdict: "pass", Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Verdict: "pass", Comment: "正常"},
			{Dimension: "hook", Score: 76, Verdict: "warning", Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Verdict: "pass", Comment: "语言基本成立"},
		},
		Issues: []domain.ConsistencyIssue{{
			Type: "contract", Severity: domain.SeverityError, Description: "契约漏项", Evidence: "未出现试炼邀请",
			Suggestion: "补入邀请", Chapters: []int{3}, RequiresChange: true,
		}},
		ContractStatus: "partial",
		ContractMisses: []string{"未明确埋下内门试炼邀请"},
		ContractNotes:  "主线推进达成，但 contract 中的第二个推进项没有落地。",
		Verdict:        "polish",
		Summary:        "本章基本完成目标，但 contract 仍有漏项。",
	})

	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	review, err := s.World.LoadReview(3)
	if err != nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if review == nil {
		t.Fatal("expected review saved, got nil")
	}
	if review.ContractStatus != "partial" {
		t.Fatalf("unexpected contract status: %q", review.ContractStatus)
	}
	if len(review.ContractMisses) != 1 || review.ContractMisses[0] != "未明确埋下内门试炼邀请" {
		t.Fatalf("unexpected contract misses: %+v", review.ContractMisses)
	}
	if review.Dimension("aesthetic") == nil {
		t.Fatalf("expected aesthetic dimension persisted, got %+v", review.Dimensions)
	}
}

// TestSaveReviewPersistsRuleCode pin câu 4 §H: schema nhận `rule` (mã phụ ổn
// định), lưu nguyên vào bài thẩm duyệt và đánh dấu vòng chấm là rule-aware.
func TestSaveReviewPersistsRuleCode(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(3, "Ba câu tóm tắt liên tiếp. Rồi câu chuyện tiếp tục với hành động mới."); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "continuity", Score: 78, Comment: "Nhắc lại phần trước"},
		},
		Issues: []domain.ConsistencyIssue{{
			Type: "continuity", Severity: domain.SeverityWarning, Description: "Nhắc lại truyện trước quá dài",
			Evidence: "Ba câu tóm tắt liên tiếp.", RuleRef: "Master §19",
			Quote: "Ba câu tóm tắt liên tiếp.", Suggestion: "Lồng vào hành động",
			Rule: "s19-recap", Chapters: []int{3},
		}},
		Verdict: "accept", Summary: "Chỉ còn cảnh báo",
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	review, err := s.World.LoadReview(3)
	if err != nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if review == nil || len(review.Issues) != 1 {
		t.Fatalf("review = %+v", review)
	}
	if review.Issues[0].Rule != "s19-recap" {
		t.Fatalf("rule = %q, muốn s19-recap", review.Issues[0].Rule)
	}
	if !review.RuleAware {
		t.Fatal("vòng chấm mới phải được đánh dấu rule-aware")
	}
}

func TestSaveReviewRejectsMissingDimensions(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "accept", Summary: "ok",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "dimensions must contain at least one") {
		t.Fatalf("expected dimensions validation error, got %v", err)
	}
}

func TestSaveReviewRejectsDimensionWithoutComment(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Comment: "基本一致"},
			{Dimension: "character", Score: 82, Comment: "人设稳定"},
			{Dimension: "pacing", Score: 78},
			{Dimension: "continuity", Score: 84, Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Comment: "正常"},
			{Dimension: "hook", Score: 76, Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Comment: "语言基本成立"},
		},
		Verdict: "accept",
		Summary: "ok",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "dimension comment is required: pacing") {
		t.Fatalf("expected dimension comment validation error, got %v", err)
	}
}

func TestSaveReviewRejectsIssueOutsideChapterScope(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(80); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	for ch := 1; ch <= 58; ch++ {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 58,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Comment: "基本一致"},
			{Dimension: "character", Score: 82, Comment: "人设稳定"},
			{Dimension: "pacing", Score: 58, Comment: "节奏需要重写"},
			{Dimension: "continuity", Score: 84, Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Comment: "正常"},
			{Dimension: "hook", Score: 76, Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Comment: "语言基本成立"},
		},
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: domain.SeverityError, Description: "节奏问题", Evidence: "第65章",
			Suggestion: "调整", Chapters: []int{65}, RequiresChange: true,
		}},
		ContractStatus: "partial",
		ContractMisses: []string{"节奏超出本章职责"},
		ContractNotes:  "应只处理已完成章节。",
		Verdict:        "polish",
		Summary:        "需要打磨第 58 章，不能把未完成章节入队。",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "must reference chapter 58") {
		t.Fatalf("expected out-of-scope affected chapter rejection, got %v", err)
	}
	review, err := s.World.LoadReview(58)
	if err != nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if review != nil {
		t.Fatalf("review should not be saved when pending rewrite validation fails: %+v", review)
	}
	p, _ := s.Progress.Load()
	if p.Flow != domain.FlowWriting && p.Flow != "" {
		t.Fatalf("flow should not enter rewrite/polish, got %s", p.Flow)
	}
	if len(p.PendingRewrites) != 0 {
		t.Fatalf("pending_rewrites should remain empty, got %v", p.PendingRewrites)
	}
}

// TestSaveReviewKeepsModelDefinedDimension 验证工具不再把文学评价维度和分数阈值
// 写死在 Go 中；Editor 可以按当前任务补充更准确的评价面。
func TestSaveReviewKeepsModelDefinedDimension(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "dialogue_subtext", Score: 85, Verdict: "warning", Comment: "潜台词仍可加强"},
		},
		Verdict: "accept",
		Summary: "ok",
	})

	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute should accept model-defined dimension, got %v", err)
	}

	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if d := review.Dimension("dialogue_subtext"); d == nil || d.Verdict != "warning" {
		t.Fatalf("model-defined assessment should be preserved, got %+v", d)
	}
}

func TestSaveReviewRejectsRewriteWithoutActionableIssue(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Verdict: "pass", Comment: "基本一致"},
			{Dimension: "character", Score: 82, Verdict: "pass", Comment: "人设稳定"},
			{Dimension: "pacing", Score: 78, Verdict: "warning", Comment: "略慢"},
			{Dimension: "continuity", Score: 84, Verdict: "pass", Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Verdict: "pass", Comment: "正常"},
			{Dimension: "hook", Score: 76, Verdict: "warning", Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Verdict: "pass", Comment: "语言基本成立"},
		},
		Verdict: "rewrite",
		Summary: "需要重写",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "requires at least one issue") {
		t.Fatalf("expected actionable issue validation error, got %v", err)
	}
}

func TestSaveReviewRejectsIssueWithoutEvidence(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Verdict: "pass", Comment: "基本一致"},
			{Dimension: "character", Score: 82, Verdict: "pass", Comment: "人设稳定"},
			{Dimension: "pacing", Score: 78, Verdict: "warning", Comment: "略慢"},
			{Dimension: "continuity", Score: 84, Verdict: "pass", Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Verdict: "pass", Comment: "正常"},
			{Dimension: "hook", Score: 76, Verdict: "warning", Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Verdict: "pass", Comment: "语言基本成立"},
		},
		Issues: []domain.ConsistencyIssue{
			{Type: "hook", Severity: domain.SeverityWarning, Description: "章末钩子偏弱"},
		},
		Verdict: "polish",
		Summary: "需要补强钩子。",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "issue evidence is required") {
		t.Fatalf("expected issue evidence validation error, got %v", err)
	}
}

// TestSaveReviewDoesNotDirtyQueueOnIllegalFlowTransition 防回归：返工排空中途
// （Flow=rewriting、PendingRewrites=[8,9]）对已重写章复审得到 polish 时，
// Flow=polishing 与 rewriting 构成非法迁移。ApplyReviewOutcome 必须在同一次写锁中
// 完成校验和写入，非法迁移时队列保持不变。
func TestSaveReviewDoesNotDirtyQueueOnIllegalFlowTransition(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	for _, ch := range []int{8, 9} {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}
	if err := s.Progress.SetPendingRewrites([]int{8, 9}, "返工", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.Progress.SetFlow(domain.FlowRewriting); err != nil {
		t.Fatalf("SetFlow rewriting: %v", err)
	}

	tool := NewSaveReviewTool(s, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 8,
		Scope:   "chapter",
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Verdict: "pass", Comment: "基本一致"},
			{Dimension: "character", Score: 82, Verdict: "pass", Comment: "人设稳定"},
			{Dimension: "pacing", Score: 78, Verdict: "warning", Comment: "略慢"},
			{Dimension: "continuity", Score: 84, Verdict: "pass", Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Verdict: "pass", Comment: "正常"},
			{Dimension: "hook", Score: 76, Verdict: "warning", Comment: "钩子一般"},
			{Dimension: "aesthetic", Score: 81, Verdict: "pass", Comment: "语言基本成立"},
		},
		Issues: []domain.ConsistencyIssue{{
			Type: "contract", Severity: domain.SeverityError, Description: "漏项", Evidence: "契约未完成",
			Suggestion: "补齐", Chapters: []int{8}, RequiresChange: true,
		}},
		ContractStatus: "partial",
		ContractMisses: []string{"漏项"},
		ContractNotes:  "复审仍有漏项。",
		Verdict:        "polish",
		Summary:        "复审第 8 章需打磨。",
	})

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "apply review outcome") {
		t.Fatalf("expected illegal flow transition error, got %v", err)
	}

	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 2 || p.PendingRewrites[0] != 8 || p.PendingRewrites[1] != 9 {
		t.Fatalf("PendingRewrites 不应被脏写，期望 [8 9]，got %v", p.PendingRewrites)
	}
	if p.Flow != domain.FlowRewriting {
		t.Fatalf("Flow 应保持 rewriting，got %s", p.Flow)
	}
}

func TestSaveReviewKeepsOutcomeWhenReviewArtifactWriteFails(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(3); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	// 让目标文件路径成为目录，稳定触发原子 rename 失败。
	if err := os.MkdirAll(filepath.Join(dir, "reviews", "03.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "polish", Summary: "需要补足衔接",
		Issues: []domain.ConsistencyIssue{{
			Type: "continuity", Severity: domain.SeverityError, Description: "衔接不足", Evidence: "开篇缺少承接",
			Suggestion: "补足衔接", Chapters: []int{3}, RequiresChange: true,
		}},
		Dimensions: []domain.DimensionScore{
			{Dimension: "consistency", Score: 85, Comment: "一致"},
			{Dimension: "character", Score: 82, Comment: "稳定"},
			{Dimension: "pacing", Score: 78, Comment: "略快"},
			{Dimension: "continuity", Score: 84, Comment: "连贯"},
			{Dimension: "foreshadow", Score: 80, Comment: "正常"},
			{Dimension: "hook", Score: 76, Comment: "可加强"},
			{Dimension: "aesthetic", Score: 81, Comment: "语言成立"},
		},
	})
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "save review") {
		t.Fatalf("expected review write failure, got %v", err)
	}

	p, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Flow != domain.FlowPolishing || len(p.PendingRewrites) != 1 || p.PendingRewrites[0] != 3 {
		t.Fatalf("审阅工件失败后返工意图必须保持可恢复，got flow=%s queue=%v", p.Flow, p.PendingRewrites)
	}
}

func setupArcReviewStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1,
		Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, Chapters: []domain.OutlineEntry{{Title: "三"}, {Title: "四"}}},
		},
	}}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline(domain.FlattenOutline(volumes)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatal(err)
	}
	for chapter := 1; chapter <= 4; chapter++ {
		if err := s.Progress.MarkChapterComplete(chapter, 100, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	// 第一弧已经完整收尾，因此 Router 当前唯一待补工件是第二弧评审。
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 2, Scope: "arc", Verdict: "accept", Summary: "第一弧评审"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveArcSummary(domain.ArcSummary{Volume: 1, Arc: 1, Title: "第一弧", Summary: "完成", KeyEvents: []string{"事件"}}); err != nil {
		t.Fatal(err)
	}
	return s
}

func arcReviewArgs(t *testing.T, issueChapter int) []byte {
	t.Helper()
	return reviewArgs(t, domain.ReviewEntry{
		Chapter: 4,
		Scope:   "arc",
		Dimensions: []domain.DimensionScore{
			{Dimension: "pacing", Score: 70, Comment: "第三章节奏拖沓"},
		},
		Issues: []domain.ConsistencyIssue{{
			Type: "pacing", Severity: domain.SeverityError, Description: "冲突进入过晚", Evidence: "第3章前半没有推进",
			Suggestion: "压缩铺垫", Chapters: []int{issueChapter}, RequiresChange: true,
		}},
		Verdict: "polish",
		Summary: "第二弧需要压缩一处铺垫",
	})
}

func TestSaveReviewRejectsIssueOutsideArcSpan(t *testing.T) {
	s := setupArcReviewStore(t)
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), arcReviewArgs(t, 2)); err == nil || !strings.Contains(err.Error(), "outside 3-4") {
		t.Fatalf("expected arc range rejection, got %v", err)
	}
	if p, _ := s.Progress.Load(); len(p.PendingRewrites) != 0 {
		t.Fatalf("invalid review must not enqueue rewrites: %v", p.PendingRewrites)
	}
}

func TestSaveReviewDerivesAffectedChaptersFromIssues(t *testing.T) {
	s := setupArcReviewStore(t)
	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), arcReviewArgs(t, 3)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	review, err := s.World.LoadReview(4)
	if err != nil || review == nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if !slices.Equal(review.AffectedChapters, []int{3}) {
		t.Fatalf("affected chapters must be derived from issues, got %v", review.AffectedChapters)
	}
	if p, _ := s.Progress.Load(); !slices.Equal(p.PendingRewrites, []int{3}) {
		t.Fatalf("rewrite queue = %v, want [3]", p.PendingRewrites)
	}
}

func TestSaveReviewRetriesCheckpointWithoutReapplyingOutcome(t *testing.T) {
	s := setupArcReviewStore(t)
	checkpointPath := filepath.Join(s.Dir(), "meta", "checkpoints.jsonl")
	if err := os.MkdirAll(checkpointPath, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := NewSaveReviewTool(s, nil)
	args := arcReviewArgs(t, 3)

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "checkpoint review") {
		t.Fatalf("expected checkpoint failure, got %v", err)
	}
	if err := os.Remove(checkpointPath); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("idempotent checkpoint retry: %v", err)
	}
	progress, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(progress.PendingRewrites, []int{3}) {
		t.Fatalf("review outcome must not be duplicated or changed, queue=%v", progress.PendingRewrites)
	}
	if cp := s.Checkpoints.LatestByStep(domain.ArcScope(1, 2), "review"); cp == nil {
		t.Fatal("checkpoint retry must finish the review write")
	}
}

func TestSaveArcReviewCanReplaceChapterReviewAtEndpoint(t *testing.T) {
	s := setupArcReviewStore(t)
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 4, Scope: "chapter", Verdict: "accept", Summary: "末章单章评审"}); err != nil {
		t.Fatal(err)
	}

	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), arcReviewArgs(t, 3)); err != nil {
		t.Fatalf("arc review should replace chapter-scope artifact at the same endpoint: %v", err)
	}
	review, err := s.World.LoadReview(4)
	if err != nil {
		t.Fatal(err)
	}
	if review == nil || review.Scope != "arc" {
		t.Fatalf("stored review = %+v, want scope=arc", review)
	}
}

func TestSaveReviewRejectsFutureArc(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1,
		Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, Chapters: []domain.OutlineEntry{{Title: "三"}, {Title: "四"}}},
		},
	}}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline(domain.FlattenOutline(volumes)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatal(err)
	}
	for chapter := 1; chapter <= 2; chapter++ {
		if err := s.Progress.MarkChapterComplete(chapter, 100, "", ""); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := NewSaveReviewTool(s, nil).Execute(context.Background(), arcReviewArgs(t, 3)); err == nil || !strings.Contains(err.Error(), "must be completed") {
		t.Fatalf("expected future arc rejection, got %v", err)
	}
	if review, err := s.World.LoadReview(4); err != nil || review != nil {
		t.Fatalf("future review must not be persisted, review=%+v err=%v", review, err)
	}
}

// goalReviewFixture: chương 3 đã nộp với chính văn chứa các đoạn trích của mục
// tiêu, và một plan có đủ rb/fm/cc để sinh checklist rb1..cc1.
func goalReviewFixture(t *testing.T) (*store.Store, *SaveReviewTool) {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(3, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(3,
		"Anh ta vượt qua cửa thứ nhất và nhận lời mời thử nội môn. "+
			"Cánh tay trái vẫn còn đau. Đêm khuya rồi. Tôi đi ngủ. Thân phận sư tôn vẫn kín."); err != nil {
		t.Fatalf("SaveFinalChapter: %v", err)
	}
	if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{
		Chapter: 3,
		Contract: domain.ChapterContract{
			RequiredBeats:    []string{"主角通过第一关", "埋下内门试炼邀请"},
			ForbiddenMoves:   []string{"不得揭露师尊身份"},
			ContinuityChecks: []string{"左臂旧伤未愈"},
		},
	}); err != nil {
		t.Fatalf("SaveChapterPlan: %v", err)
	}
	return s, NewSaveReviewTool(s, nil)
}

// goalEntry là bài thẩm duyệt chapter-scope tối thiểu hợp lệ; caller chỉnh
// GoalChecks/Issues/Verdict cho từng trường hợp.
func goalEntry(checks []domain.GoalCheck) domain.ReviewEntry {
	return domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "accept", Summary: "Chương đọc ổn",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 80, Comment: "Đã đọc chương"}},
		GoalChecks: checks,
	}
}

func passAllGoalChecks() []domain.GoalCheck {
	return []domain.GoalCheck{
		{ID: "rb1", Status: "pass", Quote: "vượt qua cửa thứ nhất"},
		{ID: "rb2", Status: "pass", Quote: "nhận lời mời thử nội môn"},
		{ID: "fm1", Status: "pass"},
		{ID: "cc1", Status: "pass", Quote: "Cánh tay trái vẫn còn đau"},
	}
}

// TestSaveReviewGoalChecksCoverExactSet pin phạm vi bắt buộc: thiếu, trùng hoặc
// lạ ID đều bị ErrToolArgs từ chối với đúng ID trong thông điệp.
func TestSaveReviewGoalChecksCoverExactSet(t *testing.T) {
	for _, tc := range []struct {
		name   string
		checks []domain.GoalCheck
		want   string
	}{
		{"missing id", []domain.GoalCheck{
			{ID: "rb1", Status: "pass", Quote: "vượt qua cửa thứ nhất"},
			{ID: "rb2", Status: "pass", Quote: "nhận lời mời thử nội môn"},
			{ID: "fm1", Status: "pass"},
		}, "thiếu cc1"},
		{"duplicate id", []domain.GoalCheck{
			{ID: "rb1", Status: "pass", Quote: "vượt qua cửa thứ nhất"},
			{ID: "rb1", Status: "pass", Quote: "vượt qua cửa thứ nhất"},
			{ID: "rb2", Status: "pass", Quote: "nhận lời mời thử nội môn"},
			{ID: "fm1", Status: "pass"},
			{ID: "cc1", Status: "pass"},
		}, "trùng rb1"},
		{"foreign id", append(passAllGoalChecks(), domain.GoalCheck{ID: "zz9", Status: "pass"}), "lạ zz9"},
		{"empty set", []domain.GoalCheck{}, "thiếu rb1,rb2,fm1,cc1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tool := goalReviewFixture(t)
			err := executeReview(t, tool, goalEntry(tc.checks))
			if !errors.Is(err, errs.ErrToolArgs) {
				t.Fatalf("phải là ErrToolArgs, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("lỗi phải nêu %q, got %v", tc.want, err)
			}
		})
	}
}

// TestSaveReviewGoalCheckQuoteAndNoteRules pin bảng quy tắc trích dẫn/ghi chú
// theo kind+status, và việc quote phải định vị được trong chính văn.
func TestSaveReviewGoalCheckQuoteAndNoteRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]domain.GoalCheck)
		want   string
	}{
		{"rb pass without quote", func(c []domain.GoalCheck) { c[0].Quote = "" }, "rb1 pass: cần quote"},
		{"rb fail without note", func(c []domain.GoalCheck) {
			c[0].Status, c[0].Quote, c[0].Note = "fail", "", ""
		}, "rb1 fail: cần note"},
		{"unknown without note", func(c []domain.GoalCheck) {
			c[3].Status, c[3].Quote, c[3].Note = "unknown", "", ""
		}, "cc1 unknown: cần note"},
		{"fm fail without quote", func(c []domain.GoalCheck) {
			c[2].Status, c[2].Note = "fail", "đã xảy ra"
		}, "fm1 fail: cần quote"},
		{"quote not in chapter", func(c []domain.GoalCheck) {
			c[0].Quote = "một đoạn chưa hề được viết ra"
		}, "không tìm thấy trong chính văn"},
		{"quote too short", func(c []domain.GoalCheck) { c[0].Quote = "vượt" }, "ít nhất 6 ký tự"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tool := goalReviewFixture(t)
			checks := passAllGoalChecks()
			tc.mutate(checks)
			err := executeReview(t, tool, goalEntry(checks))
			if !errors.Is(err, errs.ErrToolArgs) {
				t.Fatalf("phải là ErrToolArgs, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("lỗi phải chứa %q, got %v", tc.want, err)
			}
		})
	}
}

// TestSaveReviewGoalQuoteErrorShowsNearestSnippet pin phần Risk: khi quote lệch
// một từ, thông điệp lỗi kèm đoạn gần nhất trong chính văn để model chép đúng.
func TestSaveReviewGoalQuoteErrorShowsNearestSnippet(t *testing.T) {
	_, tool := goalReviewFixture(t)
	checks := passAllGoalChecks()
	checks[0].Quote = "vượt qua cửa thứ nhất vào nhận toàn bộ"
	err := executeReview(t, tool, goalEntry(checks))
	if err == nil || !strings.Contains(err.Error(), "đoạn gần nhất") {
		t.Fatalf("lỗi phải kèm đoạn gần nhất, got %v", err)
	}
	if !strings.Contains(err.Error(), "vượt qua cửa thứ nhất") {
		t.Fatalf("đoạn gợi ý phải là nguyên văn chương, got %v", err)
	}
}

// TestSaveReviewRejectsGoalChecksOnArcScope pin: chỉ scope=chapter có checklist;
// arc/global phải gửi goal_checks rỗng.
func TestSaveReviewRejectsGoalChecksOnArcScope(t *testing.T) {
	s := setupArcReviewStore(t)
	entry := domain.ReviewEntry{
		Chapter: 4, Scope: "arc", Verdict: "accept", Summary: "Tổng kết hồi hai",
		Dimensions: []domain.DimensionScore{{Dimension: "pacing", Score: 80, Comment: "Ổn"}},
		GoalChecks: []domain.GoalCheck{{ID: "rb1", Status: "pass"}},
	}
	err := func() error {
		_, err := NewSaveReviewTool(s, nil).Execute(context.Background(), reviewArgs(t, entry))
		return err
	}()
	if !errors.Is(err, errs.ErrToolArgs) || !strings.Contains(err.Error(), "scope=chapter") {
		t.Fatalf("arc scope phải từ chối goal_checks bằng ErrToolArgs, got %v", err)
	}
}

// TestSaveReviewDerivesContractFromGoalChecks pin: khi có goal_checks, hệ thống
// tự suy ra contract_status/contract_misses và ghi đè giá trị model gửi; các mục
// được lưu kèm kind/text từ snapshot contract.
func TestSaveReviewDerivesContractFromGoalChecks(t *testing.T) {
	s, tool := goalReviewFixture(t)
	entry := goalEntry([]domain.GoalCheck{
		{ID: "rb1", Status: "pass", Quote: "vượt qua cửa thứ nhất"},
		{ID: "rb2", Status: "fail", Note: "thiếu lời mời nội môn"},
		{ID: "fm1", Status: "fail", Quote: "Thân phận sư tôn vẫn kín", Note: "lộ danh tính"},
		{ID: "cc1", Status: "unknown", Note: "không thấy vết thương cũ"},
	})
	// Model cố gửi sai: phải bị ghi đè bởi kết quả suy ra (2/3 rb+fm fail → missed).
	entry.ContractStatus = "met"
	entry.ContractMisses = []string{"mục do model tự ghi"}
	if err := executeReview(t, tool, entry); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if review.ContractStatus != "missed" {
		t.Fatalf("contract_status = %q, muốn missed do suy ra", review.ContractStatus)
	}
	wantMisses := []string{
		"[rb2|fail] 埋下内门试炼邀请",
		"[fm1|fail] 不得揭露师尊身份",
		"[cc1|unknown] 左臂旧伤未愈",
	}
	if !slices.Equal(review.ContractMisses, wantMisses) {
		t.Fatalf("contract_misses = %v, want %v", review.ContractMisses, wantMisses)
	}
	if len(review.GoalChecks) != 4 {
		t.Fatalf("goal_checks = %+v", review.GoalChecks)
	}
	for _, c := range review.GoalChecks {
		if c.Kind == "" || c.Text == "" {
			t.Fatalf("kind/text phải được điền từ contract snapshot: %+v", c)
		}
	}
	if review.GoalChecks[0].Kind != "rb" || review.GoalChecks[3].Kind != "cc" {
		t.Fatalf("thứ tự/kind sai: %+v", review.GoalChecks)
	}
}

// TestSaveReviewGoalChecksIdempotentRetry pin sameStoredReview: ghi lại cùng
// một vòng chấm có goal_checks không sinh xung đột hay round mới.
func TestSaveReviewGoalChecksIdempotentRetry(t *testing.T) {
	s, tool := goalReviewFixture(t)
	entry := goalEntry(passAllGoalChecks())
	if err := executeReview(t, tool, entry); err != nil {
		t.Fatalf("round 1: %v", err)
	}
	if err := executeReview(t, tool, entry); err != nil {
		t.Fatalf("retry y hệt phải được chấp nhận: %v", err)
	}
	review, err := s.World.LoadReview(3)
	if err != nil || review == nil {
		t.Fatalf("LoadReview: %v", err)
	}
	if review.ContractStatus != "met" || len(review.GoalChecks) != 4 {
		t.Fatalf("review = %+v", review)
	}
	rounds, err := s.World.LoadReviewRounds(3, store.ReviewScopeChapter)
	if err != nil || len(rounds) != 0 {
		t.Fatalf("retry không được tạo round mới: %+v, %v", rounds, err)
	}
}

// TestSaveReviewOldEntryWithoutGoalChecksLoads pin ngược tương thích: bản ghi
// cũ không có trường goal_checks vẫn decode và đọc được.
func TestSaveReviewOldEntryWithoutGoalChecksLoads(t *testing.T) {
	var r domain.ReviewEntry
	legacy := `{"chapter":3,"scope":"chapter","issues":[],"verdict":"accept","summary":"cũ","contract_status":"met"}`
	if err := json.Unmarshal([]byte(legacy), &r); err != nil {
		t.Fatalf("unmarshal legacy review: %v", err)
	}
	if r.GoalChecks != nil {
		t.Fatalf("goal_checks phải vắng mặt trên bản ghi cũ: %+v", r.GoalChecks)
	}
	s, _ := goalReviewFixture(t)
	if err := s.World.SaveReview(r); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}
	loaded, err := s.World.LoadReview(3)
	if err != nil || loaded == nil || loaded.ContractStatus != "met" || len(loaded.GoalChecks) != 0 {
		t.Fatalf("bản ghi cũ phải đọc được nguyên vẹn: %+v, %v", loaded, err)
	}
}

// TestSaveReviewIssueQuoteMustExistInChapter pin: mọi issue.quote khác null
// phải định vị được trong chính văn của chương được nêu; issue không quote thì
// không phải định vị.
func TestSaveReviewIssueQuoteMustExistInChapter(t *testing.T) {
	_, tool := goalReviewFixture(t)

	bad := goalEntry(passAllGoalChecks())
	bad.Verdict = "polish"
	bad.Issues = []domain.ConsistencyIssue{{
		Type: "pacing", Severity: domain.SeverityError, Description: "Nhịp chậm",
		Evidence: "đoạn giữa", Quote: "câu văn không tồn tại trong chương",
		Chapters: []int{3}, RequiresChange: true,
	}}
	if err := executeReview(t, tool, bad); !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("quote không có trong chương phải là ErrToolArgs, got %v", err)
	}

	good := goalEntry(passAllGoalChecks())
	good.Verdict = "polish"
	good.Issues = []domain.ConsistencyIssue{{
		Type: "pacing", Severity: domain.SeverityError, Description: "Nhịp chậm",
		Evidence: "đoạn giữa", Quote: "Tôi đi ngủ.",
		Chapters: []int{3}, RequiresChange: true,
	}}
	if err := executeReview(t, tool, good); err != nil {
		t.Fatalf("quote có trong chính văn phải được chấp nhận: %v", err)
	}
}
