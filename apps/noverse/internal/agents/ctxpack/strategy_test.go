package ctxpack

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corecontext "github.com/datnt1998/verse/apps/noverse/internal/agent/context"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestStoreSummaryCompactApplyUsesPersistentStoreData(t *testing.T) {
	s := seededWriterStore(t)
	strategy := NewStoreSummaryCompact(StoreSummaryCompactConfig{
		Store:              s,
		KeepRecentTokens:   80,
		SummaryTokenBudget: 2000,
	})

	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg("original request"), llmtypes.UserMsg(strings.Repeat("Ngữ cảnh cũ. ", 300)),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("Phản hồi cũ. ", 300))},
		},
		llmtypes.UserMsg("Tiếp tục viết chương ba, chú ý nối tiếp kết thúc chương hai."),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock("Đã hiểu, tôi sắp xếp lại cảnh hiện tại trước.")},
		},
	}

	out, result, err := strategy.Apply(context.Background(), msgs, msgs, corecontext.Budget{
		Tokens:    corecontext.EstimateTotal(msgs),
		Window:    128,
		Threshold: 32,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected store summary strategy to apply")
	}
	if result.Name != storeSummaryStrategyName {
		t.Fatalf("unexpected strategy name: %q", result.Name)
	}
	if len(out) < 2 {
		t.Fatalf("expected summary + kept messages, got %d", len(out))
	}
	summary, ok := out[1].(corecontext.ContextSummary)
	if !ok {
		t.Fatalf("expected ContextSummary, got %T", out[1])
	}
	if !strings.Contains(summary.Summary, "Tóm tắt chương gần đây") {
		t.Fatalf("expected persistent summaries in checkpoint, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "Kế hoạch chương hiện tại") {
		t.Fatalf("expected chapter plan in checkpoint, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "Phục bút đang hoạt động") {
		t.Fatalf("expected foreshadow data in checkpoint, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "Vấn đề thẩm duyệt cần sửa") {
		t.Fatalf("expected pending review section in checkpoint, got %q", summary.Summary)
	}
	if !strings.Contains(summary.Summary, "manh mối kho cần nén thêm một nhịp") {
		t.Fatalf("expected pending review details in checkpoint, got %q", summary.Summary)
	}
	if result.Info == nil || result.Info.CompactedCount <= 0 {
		t.Fatalf("expected compaction info, got %+v", result.Info)
	}
}

func TestWriterRestoreIncludesOptionalDataWarnings(t *testing.T) {
	s := seededWriterStore(t)
	if err := os.WriteFile(filepath.Join(s.Dir(), "meta", "style_rules.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	text, ok, err := buildWriterRestoreText(s, restoreBudgetTokens)
	if err != nil {
		t.Fatalf("辅助数据损坏不应阻止恢复上下文: %v", err)
	}
	if !ok || !strings.Contains(text, "Cảnh báo dữ liệu") || !strings.Contains(text, "style_rules") {
		t.Fatalf("恢复上下文应向模型暴露读取告警: %q", text)
	}
}

func TestStoreSummaryCompactApplyFallsBackWhenStoreDataInsufficient(t *testing.T) {
	dir := t.TempDir()
	s := storepkg.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    1,
		TotalChapters:     3,
		CompletedChapters: nil,
	}); err != nil {
		t.Fatalf("Save progress: %v", err)
	}

	strategy := NewStoreSummaryCompact(StoreSummaryCompactConfig{Store: s, KeepRecentTokens: 20})
	msgs := []llmtypes.AgentMessage{
		llmtypes.UserMsg(strings.Repeat("Ngữ cảnh cũ. ", 40)),
		llmtypes.Message{
			Role:    llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.TextBlock(strings.Repeat("Phản hồi cũ. ", 40))},
		},
	}

	out, result, err := strategy.Apply(context.Background(), msgs, msgs, corecontext.Budget{
		Tokens:    corecontext.EstimateTotal(msgs),
		Window:    64,
		Threshold: 16,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Applied {
		t.Fatal("expected no-op when persistent memory is insufficient")
	}
	if len(out) != len(msgs) {
		t.Fatalf("expected messages unchanged, got %d", len(out))
	}
}

func TestWriterRestorePackRefreshReusesStoreBuilder(t *testing.T) {
	s := seededWriterStore(t)
	pack := &WriterRestorePack{}
	pack.Refresh(s)

	msg, ok, err := pack.buildMessage(restoreBudgetTokens)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	if !ok {
		t.Fatal("expected restore pack message")
	}
	text := msg.TextContent()
	if !strings.Contains(text, "<post-compact-context>") {
		t.Fatalf("expected wrapped restore context, got %q", text)
	}
	if !strings.Contains(text, "Vấn đề thẩm duyệt cần sửa") {
		t.Fatalf("expected pending review section, got %q", text)
	}
	if !strings.Contains(text, "Kế hoạch chương hiện tại") {
		t.Fatalf("expected chapter plan section, got %q", text)
	}

	if _, _, err := pack.buildMessage(0); err == nil {
		t.Fatal("expected an explicit error when the restore pack does not fit")
	}
}

func seededWriterStore(t *testing.T) *storepkg.Store {
	t.Helper()

	s := storepkg.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    3,
		TotalChapters:     6,
		CompletedChapters: []int{1, 2},
		Flow:              domain.FlowWriting,
	}); err != nil {
		t.Fatalf("Save progress: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Chương 1", CoreEvent: "Mở màn"},
		{Chapter: 2, Title: "Chương 2", CoreEvent: "Xung đột leo thang"},
		{Chapter: 3, Title: "Chương 3", CoreEvent: "Truy tìm manh mối", Scenes: []string{"Nhân vật chính điều tra vụ mất tích", "Phát hiện manh mối ở kho cũ"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{
		Chapter:    3,
		Title:      "Chương 3",
		Goal:       "Đẩy mạnh điều tra vụ mất tích",
		Conflict:   "Nhân vật chính và đồng đội bất đồng hướng điều tra",
		Hook:       "Phát hiện đoạn ghi âm khả nghi trong kho",
		EmotionArc: "Từ nghi ngờ đến căng thẳng",
	}); err != nil {
		t.Fatalf("SaveChapterPlan: %v", err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter:    1,
		Summary:    "Nhân vật chính nhận uỷ thác, nhận ra vụ mất tích không đơn giản.",
		Characters: []string{"Lâm Lam", "Chu Sách"},
		KeyEvents:  []string{"Uỷ thác được lập"},
	}); err != nil {
		t.Fatalf("SaveSummary 1: %v", err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter:    2,
		Summary:    "Hai người truy tìm bến tàu cũ, manh mối dẫn tới kho bỏ hoang.",
		Characters: []string{"Lâm Lam", "Chu Sách", "Thẩm thúc"},
		KeyEvents:  []string{"Xung đột ở bến tàu cũ", "Manh mối kho xuất hiện"},
	}); err != nil {
		t.Fatalf("SaveSummary 2: %v", err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "tape", Description: "Băng ghi âm người mất tích để lại", PlantedAt: 2, Status: "planted"},
	}); err != nil {
		t.Fatalf("SaveForeshadowLedger: %v", err)
	}
	if err := s.World.SaveTimeline([]domain.TimelineEvent{
		{Chapter: 2, Time: "Ban đêm", Event: "Giao chiến ở bến tàu cũ", Characters: []string{"Lâm Lam", "Chu Sách"}},
	}); err != nil {
		t.Fatalf("SaveTimeline: %v", err)
	}
	if err := s.World.SaveStyleRules(domain.WritingStyleRules{
		Prose:  []string{"Câu ngắn, giữ áp lực"},
		Taboos: []string{"Tránh giải thích trực diện điều bí ẩn"},
	}); err != nil {
		t.Fatalf("SaveStyleRules: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 2,
		Scope:   "chapter",
		Verdict: "polish",
		Summary: "Kết chương hai dựng nền hơi gấp, cần thêm một nhịp áp lực trước kho.",
		Issues: []domain.ConsistencyIssue{
			{
				Type:        "pacing",
				Severity:    "warning",
				Description: "Manh mối kho xuất hiện quá nhanh, chưa đủ nén nghi vấn.",
				Suggestion:  "Thêm một đoạn do dự và miêu tả áp lực môi trường trước khi vào kho.",
			},
		},
		ContractMisses: []string{"Điểm móc cuối chương chưa đủ mạnh"},
	}); err != nil {
		t.Fatalf("Save chapter review: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 2,
		Scope:   "global",
		Verdict: "polish",
		Summary: "Cuối chương hai nhịp hơi nhanh, manh mối kho cần nén thêm một nhịp.",
	}); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}
	return s
}
