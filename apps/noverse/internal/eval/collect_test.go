package eval

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func TestCollectReadsStyleUsageAndToolCalls(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	progress := &domain.Progress{
		Phase:             domain.PhaseWriting,
		Flow:              domain.FlowWriting,
		TotalChapters:     5,
		CompletedChapters: []int{3, 1, 5, 2, 4},
	}
	if err := s.Progress.Save(progress); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Chương 1 Gió nổi"},
		{Chapter: 2, Title: "Phá cục"},
		{Chapter: 3, Title: "Chương 3 Nhập cuộc"},
		{Chapter: 4, Title: "Truy vấn"},
		{Chapter: 5, Title: "Chương 5 Dư âm"},
	}); err != nil {
		t.Fatalf("save outline: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Mặc", Aliases: []string{"dược đồng"}}}); err != nil {
		t.Fatalf("save characters: %v", err)
	}
	for ch := 1; ch <= 5; ch++ {
		text := "# Tiêu đề\n\nKhông phải gió đã ngừng, mà là tất cả đều nín thở. Im lặng một thoáng, Lâm Mặc siết chặt túi thuốc."
		if err := s.Drafts.SaveFinalChapter(ch, text); err != nil {
			t.Fatalf("save chapter %d: %v", ch, err)
		}
	}
	if err := s.Usage.Save(domain.UsageState{
		Overall:      domain.AgentUsageTotals{Input: 100, Output: 40, Cost: 0.12},
		MissingUsage: 1,
	}); err != nil {
		t.Fatalf("save usage: %v", err)
	}
	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch01.jsonl", llmtypes.Message{
		Role: llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{
			llmtypes.ToolCallBlock(llmtypes.ToolCall{Name: "dispatch"}),
			llmtypes.ToolCallBlock(llmtypes.ToolCall{Name: "novel_context"}),
		},
	})

	col := Collect(dir, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("不应有读取错误: %v", col.LoadErrors)
	}
	if col.Style.Status != "ok" || col.Style.Stats == nil {
		t.Fatalf("stylestat 应可计算，得到 status=%s stats=%v", col.Style.Status, col.Style.Stats)
	}
	if col.Style.Stats.TitleFormats == nil {
		t.Fatal("标题混用应被 stylestat 捕获")
	}
	if !col.Usage.UsageRecorded || col.Usage.Input != 100 || col.Usage.Output != 40 || col.Usage.CostUSD != 0.12 {
		t.Fatalf("usage 读取不正确: %+v", col.Usage)
	}
	if col.ToolCalls != 2 {
		t.Fatalf("tool calls = %d want 2", col.ToolCalls)
	}
}

func TestCollectStyleInsufficientSample(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Progress.Save(&domain.Progress{CompletedChapters: []int{1}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	if err := s.Drafts.SaveFinalChapter(1, "只有一章"); err != nil {
		t.Fatalf("save chapter: %v", err)
	}
	col := Collect(dir, nil)
	if col.Style.Status != "insufficient_sample" {
		t.Fatalf("一章样本应 insufficient_sample，得到 %s", col.Style.Status)
	}
}

func TestCollectFailsLoudWhenCompletedChapterMissing(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Progress.Save(&domain.Progress{CompletedChapters: []int{1}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	col := Collect(dir, nil)
	if !containsString(col.LoadErrors, "progress đánh dấu đã hoàn thành nhưng bản cuối rỗng") {
		t.Fatalf("缺终稿应进入 LoadErrors，实际 %v", col.LoadErrors)
	}
}

func TestChapterTitleUsesLayeredEntryChapter(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Outline.SaveLayeredOutline([]domain.VolumeOutline{
		{
			Index: 1,
			Arcs: []domain.ArcOutline{
				{Index: 1}, // 未展开 arc 不应让后续章节位置漂移
				{
					Index: 2,
					Chapters: []domain.OutlineEntry{
						{Chapter: 7, Title: "第七章 真标题"},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("save layered outline: %v", err)
	}

	got := chapterTitle(s, 7, "# 正文兜底标题\n\n内容", func(string, error) {})
	if got != "第七章 真标题" {
		t.Fatalf("应按 entry.Chapter 匹配分层标题，得到 %q", got)
	}
}

func TestChapterTitleUsesCommittedTitle(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "计划标题"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "终稿标题"}); err != nil {
		t.Fatal(err)
	}
	got := chapterTitle(s, 1, "# 正文标题\n\n内容", func(string, error) {})
	if got != "终稿标题" {
		t.Fatalf("chapterTitle = %q, want committed title", got)
	}
}

func containsString(items []string, sub string) bool {
	for _, item := range items {
		if strings.Contains(item, sub) {
			return true
		}
	}
	return false
}

func writeSessionLine(t *testing.T, root, rel string, msg llmtypes.Message) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir session: %v", err)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
}

// writeSessionLines 往同一 jsonl 追加多条消息（writeSessionLine 每次覆盖重写，
// 只适合单条）；顺序即模型收到的顺序——commit 对账依赖调用先于结果。
func writeSessionLines(t *testing.T, root, rel string, msgs ...llmtypes.Message) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir session: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer f.Close()
	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal session: %v", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			t.Fatalf("write session: %v", err)
		}
	}
}

// Collect 必须读到评审正本、被覆盖的旧轮次、章节正文与事实，以及 session 里的
// worker 运行数与工具错误数。
func TestCollectReviewsChaptersAndSessions(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, CompletedChapters: []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	texts := map[int]string{1: "# Một\n\nNội dung chương một.", 2: "# Hai\n\nNội dung chương hai."}
	for chapter, text := range texts {
		if err := s.Drafts.SaveFinalChapter(chapter, text); err != nil {
			t.Fatal(err)
		}
		record := domain.ChapterRecord{
			Version:       domain.ChapterRecordVersion,
			Chapter:       chapter,
			Revision:      1,
			Origin:        domain.ChapterOriginGenerated,
			Content:       text,
			ContentSHA256: domain.ChapterContentSHA256(text),
			Facts:         domain.ChapterFacts{Title: "Chương", Summary: "Tóm tắt", KeyEvents: []string{"sự kiện"}},
			AcceptedAt:    time.Now(),
		}
		if err := s.ChapterRecords.Save(record); err != nil {
			t.Fatal(err)
		}
	}

	first := domain.ReviewEntry{Chapter: 2, Scope: "chapter", Verdict: "polish", Issues: []domain.ConsistencyIssue{{Type: "timeline", Severity: "error", Quote: "câu cũ"}}}
	if err := s.World.SaveReview(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Verdict = "accept"
	second.Issues = nil
	if err := s.World.SaveReview(second); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 2, Scope: "global", Verdict: "accept"}); err != nil {
		t.Fatal(err)
	}

	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch01.jsonl", llmtypes.Message{
		Role:    llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{Name: "novel_context"})},
	})
	writeSessionLine(t, dir, "meta/sessions/agents/editor-001.jsonl", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock("lỗi công cụ")},
		Metadata: map[string]any{"is_error": true},
	})
	writeSessionLine(t, dir, "meta/sessions/cocreate.jsonl", llmtypes.Message{Role: llmtypes.RoleUser})

	col := Collect(dir, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("không được có lỗi đọc: %v", col.LoadErrors)
	}
	if len(col.Reviews) != 2 {
		t.Fatalf("reviews = %d want 2 (chapter + global)", len(col.Reviews))
	}
	if len(col.ReviewRounds) != 1 {
		t.Fatalf("review rounds = %d want 1", len(col.ReviewRounds))
	}
	if col.Reviews[0].Chapter != 2 || col.Reviews[1].Scope != "global" {
		t.Fatalf("reviews chưa được sắp xếp ổn định: %+v", col.Reviews)
	}
	for chapter, want := range texts {
		if col.ChapterTexts[chapter] != want {
			t.Fatalf("chapter %d text = %q want %q", chapter, col.ChapterTexts[chapter], want)
		}
		if col.ChapterFacts[chapter].Title == "" {
			t.Fatalf("chapter %d thiếu facts", chapter)
		}
	}
	if col.WorkerRuns["writer"] != 1 || col.WorkerRuns["editor"] != 1 || len(col.WorkerRuns) != 2 {
		t.Fatalf("worker runs = %+v want writer:1 editor:1", col.WorkerRuns)
	}
	if col.ToolCalls != 1 {
		t.Fatalf("tool calls = %d want 1", col.ToolCalls)
	}
	if col.ToolErrors != 1 {
		t.Fatalf("tool errors = %d want 1", col.ToolErrors)
	}
}

// 损坏的评审文件不能当成"没有评审"：必须进 LoadErrors。
func TestCollectReviewReadErrorsGoToLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if err := store.NewStore(dir).Init(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "reviews", "02.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	col := Collect(dir, nil)
	if !containsString(col.LoadErrors, "02.json") {
		t.Fatalf("review hỏng phải vào LoadErrors, nhận %v", col.LoadErrors)
	}
}

// context_trims 数 novel_context 结果里 previous_tail 被裁的次数；
// quote_coverage 只统计本次 run commit_chapter 成功的章节事实——
// seed 的历史章节（无工具调用）与失败的提交不计入。
func TestCollectContextTrimsAndQuoteCoverage(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, CompletedChapters: []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	saveFacts := func(chapter int, facts domain.ChapterFacts) {
		t.Helper()
		text := fmt.Sprintf("Nội dung chương %d.", chapter)
		if err := s.Drafts.SaveFinalChapter(chapter, text); err != nil {
			t.Fatal(err)
		}
		if err := s.ChapterRecords.Save(domain.ChapterRecord{
			Version: domain.ChapterRecordVersion, Chapter: chapter, Revision: 1,
			Origin: domain.ChapterOriginGenerated, Content: text,
			ContentSHA256: domain.ChapterContentSHA256(text),
			Facts:         facts,
			AcceptedAt:    time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 第 1 章：本次 run 里 commit_chapter 成功的章节，事实计入 quote_coverage。
	saveFacts(1, domain.ChapterFacts{
		Title: "Chương", Summary: "x", KeyEvents: []string{"e"},
		ForeshadowUpdates: []domain.ForeshadowUpdate{
			{ID: "fs1", Action: "plant", Description: "d", Quote: "đoạn trích"},
			{ID: "fs2", Action: "advance"},
		},
		StateChanges: []domain.StateChange{
			{Entity: "A", Field: "f", NewValue: "v", Quote: "trích"},
			{Entity: "B", Field: "f", NewValue: "v", Quote: "  "},
		},
	})
	// 第 2 章：fixture seed 的历史章节事实——没有本次 commit 调用，不能计入。
	saveFacts(2, domain.ChapterFacts{
		Title: "Chương cũ", Summary: "y", KeyEvents: []string{"e"},
		ForeshadowUpdates: []domain.ForeshadowUpdate{
			{ID: "fs9", Action: "plant", Description: "d", Quote: "seed"},
			{ID: "fs10", Action: "advance", Quote: "seed"},
		},
	})

	// commit_chapter：第 1 章成功提交（调参+成功结果对账），第 2 章是 seed 事实没有调用，
	// 另有一个失败提交不得计入。
	writeSessionLines(t, dir, "meta/sessions/agents/writer-ch01.jsonl",
		llmtypes.Message{
			Role: llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{
				ID: "call_commit1", Name: "commit_chapter", Args: json.RawMessage(`{"chapter":1}`),
			})},
		},
		llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`{"chapter":1,"committed":true,"word_count":120}`)},
			Metadata: map[string]any{"tool_call_id": "call_commit1", "tool_name": "commit_chapter"},
		},
		llmtypes.Message{
			Role: llmtypes.RoleAssistant,
			Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{
				ID: "call_bad", Name: "commit_chapter", Args: json.RawMessage(`{"chapter":2}`),
			})},
		},
		llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`"chapter facts invalid"`)},
			Metadata: map[string]any{"tool_call_id": "call_bad", "tool_name": "commit_chapter", "is_error": true},
		},
	)

	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch01b.jsonl", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`{"_trimmed":["previous_tail","timeline"]}`)},
		Metadata: map[string]any{"tool_name": "novel_context"},
	})
	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch02.jsonl", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`{"_trimmed":["references"]}`)},
		Metadata: map[string]any{"tool_name": "novel_context"},
	})
	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch03.jsonl", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`{"_trimmed":["previous_tail"]}`)},
		Metadata: map[string]any{"tool_name": "read_chapter"},
	})

	col := Collect(dir, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("không được có lỗi đọc: %v", col.LoadErrors)
	}
	if col.ContextTrims != 1 {
		t.Fatalf("context_trims = %d want 1", col.ContextTrims)
	}
	if col.ContextMissing != 0 {
		t.Fatalf("context_missing = %d want 0（本次结果全是原文）", col.ContextMissing)
	}
	if col.QuoteCoverage.Hit != 2 || col.QuoteCoverage.Total != 4 || col.QuoteCoverage.Rate != 0.5 {
		t.Fatalf("quote_coverage = %+v want 2/4 (0.5)，seed 与失败提交不得计入", col.QuoteCoverage)
	}
}

// 评测录制通路：KeepToolResults 登记的 novel_context 结果原文（>4096B）不经压缩
// 落盘，Collect 必须读到模型真正拿到的 payload——不是 [session_compact: ...] 占位符。
func TestCollectRecordedNovelContextResult(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.Sessions.KeepToolResults("novel_context")
	big := `{"_loading_summary":"9 keys","selected_memory":{"story_threads":[{"key":"loi_hoi_1979","reason":"` +
		strings.Repeat("phục bút đã quá hạn thu ", 400) + `"}]}}`
	if len(big) < 4096 {
		t.Fatalf("测试载荷不足 4096B: %d", len(big))
	}
	s.Sessions.SubAgentLogger(nil)("writer", "Viết chương 3", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(big)},
		Metadata: map[string]any{"tool_name": "novel_context"},
	})

	col := Collect(dir, nil)
	if len(col.ContextResults) != 1 || col.ContextResults[0] != big {
		t.Fatalf("录制的 novel_context 原文没被采到（%d 条）: %+v", len(col.ContextResults), col.ContextResults)
	}
	if col.ContextMissing != 0 {
		t.Fatalf("保真结果不应计为 missing: %d", col.ContextMissing)
	}
}

// 旧工件里 novel_context 被压缩成占位符或调用出错：ContextResults 拒收、
// ContextMissing 计数，断言报"没有可对照结果"而不是对着摘要/错误串自证。
func TestCollectSkipsCompactedContextResults(t *testing.T) {
	dir := t.TempDir()
	if err := store.NewStore(dir).Init(); err != nil {
		t.Fatal(err)
	}
	writeSessionLines(t, dir, "meta/sessions/agents/writer-ch03.jsonl",
		llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock("[session_compact: novel_context 18234B | 9 keys]")},
			Metadata: map[string]any{"tool_name": "novel_context"},
		},
		llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`"invalid args"`)},
			Metadata: map[string]any{"tool_name": "novel_context", "is_error": true},
		},
	)

	col := Collect(dir, nil)
	if len(col.ContextResults) != 0 {
		t.Fatalf("压缩/出错结果不得进 ContextResults: %+v", col.ContextResults)
	}
	if col.ContextMissing != 2 {
		t.Fatalf("context_missing = %d want 2（1 压缩 + 1 出错）", col.ContextMissing)
	}
	if col.ContextTrims != 0 {
		t.Fatalf("context_trims = %d want 0（不可测不得谎报）", col.ContextTrims)
	}
	res := Result{}
	gradeContextExpect(contextCase("loi_hoi_1979"), col, &res)
	if !hasIssue(res.HardFails, "contract:context", "không đo được") {
		t.Fatalf("压缩结果应报不可测 hard fail: %+v", res.HardFails)
	}
}

// 没有事实的 run：quote_coverage 必须是 N/A（Total=0），不是 0%。
func TestCollectQuoteCoverageNAWithoutCommits(t *testing.T) {
	dir := t.TempDir()
	if err := store.NewStore(dir).Init(); err != nil {
		t.Fatal(err)
	}
	col := Collect(dir, nil)
	if col.QuoteCoverage.Total != 0 || col.QuoteCoverage.Hit != 0 {
		t.Fatalf("no commits must yield Total=0: %+v", col.QuoteCoverage)
	}
	if got := renderRate(col.QuoteCoverage); got != unknownCorrectNA {
		t.Fatalf("empty coverage should render N/A, got %q", got)
	}
}
