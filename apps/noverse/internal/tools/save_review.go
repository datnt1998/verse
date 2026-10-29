package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/naolint"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// PackRuleSource cung cấp các mục luật đã phân tích mà không phụ thuộc assets.
type PackRuleSource interface {
	RuleItems() ([]domain.PackRuleItem, error)
}

// SaveReviewTool 保存 Editor 的审阅结果。
type SaveReviewTool struct {
	store       *store.Store
	packRules   map[string][]domain.PackRuleItem
	ruleContext string
	ruleErr     error
}

func NewSaveReviewTool(store *store.Store, source PackRuleSource) *SaveReviewTool {
	t := &SaveReviewTool{store: store}
	if source == nil {
		return t
	}
	items, err := source.RuleItems()
	t.ruleErr = err
	if len(items) == 0 {
		return t
	}
	t.packRules = make(map[string][]domain.PackRuleItem)
	var context strings.Builder
	context.WriteString("\n\n## Chỉ mục nhãn luật gói thể loại\n\nDùng đúng rule_ref dưới đây; mục có cả hai nhãn cần rule_text để xác định câu luật bị vi phạm.\n")
	for _, item := range items {
		if item.Label != "" {
			fmt.Fprintf(&context, "\n- %s [%s]: %s", item.Ref, item.Label, item.Text)
		}
		item.Text = normalizeRuleText(item.Text)
		t.packRules[item.Ref] = append(t.packRules[item.Ref], item)
	}
	t.ruleContext = context.String()
	return t
}

// RuleContext cung cấp nhãn tham chiếu cho Editor, không thay văn bản luật gốc.
func (t *SaveReviewTool) RuleContext() string { return t.ruleContext }

func (t *SaveReviewTool) Name() string { return "save_review" }
func (t *SaveReviewTool) Description() string {
	return "Lưu kết quả thẩm duyệt và cập nhật trạng thái luồng. verdict là một trong accept/polish/rewrite." +
		"Editor đưa ra verdict dựa trên ngữ cảnh đầy đủ, công cụ chỉ kiểm tra sự thật và cập nhật Progress nguyên tử." +
		"scope=arc/global chỉ dùng khi hệ thống giao đúng nhiệm vụ thẩm duyệt cung/toàn cục; yêu cầu sửa của người dùng cho nhiều chương thì gọi scope=chapter lần lượt cho từng chương, các chương đã đưa vào hàng đợi được giữ nguyên." +
		"Trả về sự thật có cấu trúc: verdict / affected_chapters / next_flow / next_chapter"
}

// unscheduledAggregateReviewHint 附在 arc/global 目标不符的错误后：这两种 scope 绑定
// Router 排期的聚合工件，用户干预要求改多章时应逐章走 scope=chapter。
const unscheduledAggregateReviewHint = "scope=arc/global chỉ dùng cho thẩm duyệt tổng hợp do hệ thống lên lịch. Để đưa các chương người dùng yêu cầu sửa vào hàng đợi, gọi save_review(scope=chapter, chapter=N) cho từng chương, mỗi lần chỉ issue của chương đó"

func (t *SaveReviewTool) Label() string { return "Lưu thẩm duyệt" }

// 写工具（同时更新 reviews/ 与 Progress 的 PendingRewrites/Flow），禁止并发。
func (t *SaveReviewTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *SaveReviewTool) ConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *SaveReviewTool) StrictSchema() bool                     { return true }

func (t *SaveReviewTool) Schema() map[string]any {
	// 属性顺序即契约顺序：chapters 与 quote（证据锚点）先写，severity 与
	// requires_change 在证据之后判断，luật 引用与建议殿后。
	issueSchema := schema.Object(
		schema.Property("type", schema.String("Chiều của vấn đề; có thể dùng chiều cơ bản trong gợi ý thẩm duyệt, hoặc ghi chiều cụ thể chính xác hơn")).Required(),
		schema.Property("chapters", schema.Array("Các chương thực sự chứa bằng chứng của vấn đề; thẩm duyệt cung phải nằm trong khoảng nhiệm vụ giao", schema.Int("Số chương"))).Required(),
		schema.Property("quote", llmcontract.Nullable(schema.String("Đoạn trích nguyên văn trong chương chứng minh vi phạm; bắt buộc với luật gói nhãn LỖI, phải xuất hiện nguyên văn trong chính văn của chương đã nêu, null khi không áp dụng"))).Required(),
		schema.Property("evidence", schema.String("Bằng chứng: đoạn nguyên văn, tình tiết cụ thể hoặc dữ liệu trạng thái")).Required(),
		schema.Property("description", schema.String("Mô tả vấn đề")).Required(),
		schema.Property("severity", schema.Enum("Mức độ nghiêm trọng", "critical", "error", "warning")).Required(),
		schema.Property("requires_change", schema.Bool("Vấn đề này có nên kích hoạt viết lại các chương đã nêu ngay không, do Editor đánh giá theo trải nghiệm đọc tổng thể")).Required(),
		schema.Property("rule_ref", llmcontract.Nullable(schema.String("Mục luật gói bị vi phạm, đúng dạng Nhãn tệp §số mục, ví dụ Master §7; null nếu không dẫn luật gói"))).Required(),
		schema.Property("rule_text", llmcontract.Nullable(schema.String("Chép nguyên một đoạn ≥ 15 ký tự của chính câu luật bị vi phạm; bắt buộc khi mục chứa cả LỖI và CẢNH BÁO, null khi không cần"))).Required(),
		schema.Property("rule", llmcontract.Nullable(schema.String("Mã phụ ổn định của mục; dùng s19-recap khi tóm tắt lại truyện trước dài hơn hai câu hoặc không lồng vào hành động (Master §19); null khi không áp dụng"))).Required(),
		schema.Property("suggestion", llmcontract.Nullable(schema.String("Đề xuất chỉnh sửa; null khi không cần đề xuất"))).Required(),
	)
	goalCheckSchema := schema.Object(
		schema.Property("id", schema.String("ID mục tiêu trong working_memory.chapter_goals, ví dụ rb1, fm1, cc1")).Required(),
		schema.Property("status", schema.Enum("Kết quả chấm mục tiêu: pass đã đạt, fail thiếu/vi phạm, unknown không xác định được từ nguyên văn", "pass", "fail", "unknown")).Required(),
		schema.Property("quote", llmcontract.Nullable(schema.String("Đoạn trích nguyên văn trong chính văn chứng minh kết quả; bắt buộc khi rb pass hoặc fm/cc fail, phải xuất hiện nguyên văn trong chương; null khi không áp dụng"))).Required(),
		schema.Property("note", schema.String("Giải thích ngắn; bắt buộc không rỗng khi rb fail hoặc mọi unknown (thiếu gì / vì sao không xác định được)")).Required(),
	)
	dimensionSchema := schema.Object(
		schema.Property("dimension", schema.String("Chiều đánh giá; do nhiệm vụ thẩm duyệt hiện tại và rubric quyết định")).Required(),
		schema.Property("score", schema.Int("Điểm (0-100)")).Required(),
		schema.Property("comment", schema.String("Kết luận ngắn và bằng chứng cho chiều này; bắt buộc với mọi chiều")).Required(),
	)
	recheckSchema := schema.Object(
		schema.Property("finding_id", schema.String("ID nguyên văn trong recheck_findings")).Required(),
		schema.Property("status", schema.Enum("Kết quả kiểm lại", "fixed", "unfixed", "unknown")).Required(),
		schema.Property("quote", llmcontract.Nullable(schema.String("Trích nguyên văn trong chương mới; unfixed bắt buộc; fixed cần khi đoạn lỗi cũ còn nguyên"))).Required(),
		schema.Property("note", schema.String("Giải thích kết quả; fixed/unknown bắt buộc không rỗng")).Required(),
	)
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương được thẩm duyệt (thẩm duyệt toàn cục điền số chương mới nhất)")).Required(),
		schema.Property("scope", schema.Enum("Phạm vi thẩm duyệt", "chapter", "global", "arc")).Required(),
		schema.Property("recheck", schema.Bool("true chỉ khi kiểm lại chương trong PendingRechecks; review thường dùng false")).Required(),
		schema.Property("recheck_results", schema.Array("Phủ đúng bộ finding_id trong recheck_findings; review thường gửi []", recheckSchema)).Required(),
		schema.Property("dimensions", schema.Array("Điểm theo từng chiều; rubric cơ bản do gợi ý Editor cung cấp, có thể bổ sung chiều cụ thể hơn theo nhiệm vụ", dimensionSchema)).Required(),
		schema.Property("issues", schema.Array("Các vấn đề phát hiện được", issueSchema)).Required(),
		schema.Property("goal_checks", schema.Array("Kết quả chấm từng ID trong chapter_goals; bắt buộc đủ bộ khi scope=chapter và chương có mục tiêu, gửi mảng rỗng khi không áp dụng", goalCheckSchema)).Required(),
		schema.Property("contract_status", llmcontract.Nullable(schema.Enum("Mức hoàn thành hợp đồng chương; khi scope=chapter có chapter_goals hệ thống tự suy ra từ goal_checks — điền null; null khi không áp dụng", "met", "partial", "missed"))).Required(),
		schema.Property("contract_misses", schema.Array("Các mục contract chưa hoàn thành hoặc bị vi phạm; mảng rỗng nếu không có; khi có goal_checks hệ thống tự suy ra, gửi mảng rỗng", schema.String(""))).Required(),
		schema.Property("contract_notes", llmcontract.Nullable(schema.String("Mô tả ngắn về việc thực hiện contract; null nếu không có"))).Required(),
		schema.Property("verdict", schema.Enum("Kết luận thẩm duyệt", "accept", "polish", "rewrite")).Required(),
		schema.Property("summary", schema.String("Tóm tắt thẩm duyệt")).Required(),
	)
}

func (t *SaveReviewTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	if t.ruleErr != nil {
		return nil, fmt.Errorf("không đọc được nhãn luật gói thể loại: %w", t.ruleErr)
	}
	var r domain.ReviewEntry
	if err := json.Unmarshal(args, &r); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if r.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0")
	}
	if r.Recheck && r.Scope != "chapter" {
		return nil, fmt.Errorf("recheck chỉ áp dụng scope=chapter: %w", errs.ErrToolPrecondition)
	}
	// Câu 13 / D9: vi phạm xưng hô đã ghi cho chương được gộp vào vòng chấm như
	// một mục Bắt sửa bình thường, không phụ thuộc việc Editor có chép
	// rule_violations vào issues[] hay không. Mục đó đi đúng một đường
	// RequiresChange → AffectedChapters → Progress.PendingRewrites với mọi Bắt sửa.
	if t.mergeAddressFindings(&r) && r.Verdict == "accept" {
		// Editor kết luận Đạt nhưng chương còn mục Bắt sửa xưng hô: "Đạt" không
		// duyệt được chương còn Bắt sửa, nên chuyển verdict sang rewrite và ghi
		// lý do vào summary để hàng đợi viết lại có ghi chú đúng.
		r.Verdict = "rewrite"
		note := fmt.Sprintf("Chương còn mục Bắt sửa “%s” nên vẫn vào hàng đợi viết lại.", naolint.IssueRef)
		if summary := strings.TrimSpace(r.Summary); summary != "" {
			r.Summary = summary + " " + note
		} else {
			r.Summary = note
		}
	}
	boundary, err := t.normalizeReviewEntry(&r)
	if err != nil {
		return nil, err
	}
	reviewOutcome, err := reviewFlow(r.Verdict)
	if err != nil {
		return nil, err
	}

	affected := r.AffectedChapters

	progress, err := t.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("load progress: %w", err)
	}
	if progress == nil || !slices.Contains(progress.CompletedChapters, r.Chapter) {
		return nil, fmt.Errorf("review chapter %d must be completed", r.Chapter)
	}
	if err := t.validateRecheck(&r, progress); err != nil {
		return nil, err
	}
	if r.Recheck && !slices.Contains(progress.PendingRechecks, r.Chapter) {
		// An acknowledged retry must not resurrect the second rewrite after drain.
		return t.finishReview(r, progress, domain.ChapterScope(r.Chapter), store.ReviewScopeChapter.Artifact(r.Chapter))
	}
	scope := domain.ChapterScope(r.Chapter)
	artifact := store.ReviewScopeOf(r.Scope).Artifact(r.Chapter)
	var existing *domain.ReviewEntry
	switch r.Scope {
	case "arc":
		scope = domain.ArcScope(boundary.Volume, boundary.Arc)
		existing, err = t.store.World.LoadReview(r.Chapter)
		if err != nil {
			return nil, fmt.Errorf("load arc review: %w", err)
		}
		if existing != nil && existing.Scope != "arc" {
			existing = nil
		}
	case "global":
		existing, err = t.store.World.LoadGlobalReview(r.Chapter)
		if err != nil {
			return nil, fmt.Errorf("load global review: %w", err)
		}
	}
	// Vòng chấm do schema có mã phụ `rule` sinh ra (câu 4 §H). Ghi lại y hệt một
	// vòng cũ thì giữ cờ của vòng cũ để phép so sánh idempotent không biến retry
	// thành xung đột.
	r.RuleAware = true
	if existing != nil {
		r.RuleAware = existing.RuleAware
		same, err := sameStoredReview(*existing, r)
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, fmt.Errorf("bài thẩm duyệt tổng hợp của chương %d đã tồn tại và nội dung khác, từ chối ghi đè: %w", r.Chapter, errs.ErrToolConflict)
		}
		return t.finishReview(r, progress, scope, artifact)
	}
	switch r.Scope {
	case "arc":
		if err := requireAggregateTarget(t.store, flow.AggregateArcReview, boundary.Volume, boundary.Arc, r.Chapter); err != nil {
			return nil, fmt.Errorf("%w. %s", err, unscheduledAggregateReviewHint)
		}
	case "global":
		if err := requireAggregateTarget(t.store, flow.AggregateGlobalReview, 0, 0, r.Chapter); err != nil {
			return nil, fmt.Errorf("%w. %s", err, unscheduledAggregateReviewHint)
		}
	}

	// Rechecks persist the artifact first. A failed progress write leaves the
	// queue dispatchable, and the archived source round validates the retry.
	recheckedChapter := 0
	if r.Recheck {
		recheckedChapter = r.Chapter
		if err := t.store.World.SaveReview(r); err != nil {
			return nil, fmt.Errorf("save recheck: %w", err)
		}
	}
	latest, err := t.store.Progress.ApplyReviewOutcome(reviewOutcome, affected, r.Summary, store.RewriteCauseReview, recheckedChapter)
	if err != nil {
		return nil, fmt.Errorf("apply review outcome: %w", err)
	}
	if !r.Recheck {
		if err := t.store.World.SaveReview(r); err != nil {
			return nil, fmt.Errorf("save review: %w", err)
		}
	}

	return t.finishReview(r, latest, scope, artifact)
}

// sameStoredReview so sánh hai vòng chấm theo đúng dạng ghi xuống đĩa: trường
// omitempty bị bỏ, nên "mảng/chuỗi rỗng" và "không có" là một. DeepEqual coi
// []int{} và nil là khác nhau, biến một lần retry y hệt (sau lỗi checkpoint)
// thành xung đột.
func sameStoredReview(stored, next domain.ReviewEntry) (bool, error) {
	written, err := json.Marshal(stored)
	if err != nil {
		return false, fmt.Errorf("marshal stored review: %w", err)
	}
	submitted, err := json.Marshal(next)
	if err != nil {
		return false, fmt.Errorf("marshal review: %w", err)
	}
	return bytes.Equal(written, submitted), nil
}

func (t *SaveReviewTool) finishReview(
	r domain.ReviewEntry,
	progress *domain.Progress,
	scope domain.Scope,
	artifact string,
) (json.RawMessage, error) {
	if r.Recheck {
		if _, err := evaluateCompletionAfterDrain(t.store, progress); err != nil {
			return nil, err
		}
	}
	if _, err := t.store.Checkpoints.AppendArtifact(scope, "review", artifact); err != nil {
		return nil, fmt.Errorf("checkpoint review: %w", err)
	}

	// 使用原子更新返回的 Progress 快照作为事实，避免二次读取产生新的失败窗口。
	nextFlow := string(domain.FlowWriting)
	nextChapter := 0
	if progress != nil {
		nextFlow = string(progress.Flow)
		nextChapter = progress.NextChapter()
	}

	return json.Marshal(map[string]any{
		"saved":             true,
		"chapter":           r.Chapter,
		"scope":             r.Scope,
		"verdict":           r.Verdict,
		"affected_chapters": r.AffectedChapters,
		"issues":            len(r.Issues),
		"next_flow":         nextFlow,
		"next_chapter":      nextChapter,
	})
}

func (t *SaveReviewTool) normalizeReviewEntry(r *domain.ReviewEntry) (*store.ArcBoundary, error) {
	switch r.Scope {
	case "chapter", "global", "arc":
	default:
		return nil, fmt.Errorf("invalid review scope: %q", r.Scope)
	}
	if len(r.AffectedChapters) > 0 {
		return nil, fmt.Errorf("affected_chapters is derived from issues[].chapters; do not submit it")
	}
	if strings.TrimSpace(r.Summary) == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if r.ContractStatus != "" && r.ContractStatus != "met" && r.ContractStatus != "partial" && r.ContractStatus != "missed" {
		return nil, fmt.Errorf("invalid contract_status: %q", r.ContractStatus)
	}
	for _, miss := range r.ContractMisses {
		if strings.TrimSpace(miss) == "" {
			return nil, fmt.Errorf("contract_misses cannot contain empty entries")
		}
	}
	var boundary *store.ArcBoundary
	if r.Scope == "arc" {
		var err error
		boundary, err = t.store.Outline.CheckArcBoundary(r.Chapter)
		if err != nil {
			return nil, fmt.Errorf("check arc scope: %w", err)
		}
		if boundary == nil || !boundary.IsArcEnd || boundary.EndChapter != r.Chapter {
			return nil, fmt.Errorf("arc review chapter must be an arc endpoint")
		}
	}

	// Checklist mục tiêu: chỉ scope=chapter có contract mới được chấm; vòng chấm
	// phải nộp đúng bộ ID của contract snapshot, không thiếu/trùng/lạ.
	var goals []domain.ChapterGoal
	if r.Scope == "chapter" {
		plan, planErr := t.store.Drafts.LoadChapterPlan(r.Chapter)
		if planErr != nil {
			return nil, fmt.Errorf("load chapter plan: %w", planErr)
		}
		if plan != nil {
			goals = domain.GoalsFromContract(r.Chapter, plan.Contract)
		}
		if len(goals) == 0 && len(r.GoalChecks) > 0 {
			return nil, fmt.Errorf("chương %d không có chapter_goals; gửi goal_checks rỗng: %w", r.Chapter, errs.ErrToolArgs)
		}
	} else if len(r.GoalChecks) > 0 {
		return nil, fmt.Errorf("goal_checks chỉ áp dụng cho scope=chapter, gửi mảng rỗng: %w", errs.ErrToolArgs)
	}

	// Văn bản các chương được tham chiếu bởi quote, nạp lười theo nhu cầu.
	chapterTexts := make(map[int]string)
	chapterText := func(ch int) (string, error) {
		if text, ok := chapterTexts[ch]; ok {
			return text, nil
		}
		text, err := t.store.Drafts.LoadChapterText(ch)
		if err != nil {
			return "", err
		}
		chapterTexts[ch] = text
		return text, nil
	}
	locateInChapters := func(quote string, chapters []int, label string) error {
		if utf8.RuneCountInString(strings.TrimSpace(quote)) < domain.MinQuoteRunes {
			return fmt.Errorf("%s: quote phải có ít nhất %d ký tự để định vị được: %w", label, domain.MinQuoteRunes, errs.ErrToolArgs)
		}
		var hint string
		for _, ch := range chapters {
			text, err := chapterText(ch)
			if err != nil {
				return fmt.Errorf("load chapter %d text: %w", ch, err)
			}
			if _, _, ok := domain.LocateQuote(text, quote); ok {
				return nil
			}
			if hint == "" {
				if snippet, found := domain.NearestQuotePrefix(text, quote); found {
					hint = snippet
				}
			}
		}
		if hint != "" {
			return fmt.Errorf("%s: quote không tìm thấy trong chính văn chương %v; đoạn gần nhất trong chính văn: %q — hãy chép nguyên văn: %w", label, chapters, hint, errs.ErrToolArgs)
		}
		return fmt.Errorf("%s: quote không tìm thấy trong chính văn chương %v: %w", label, chapters, errs.ErrToolArgs)
	}

	if len(goals) > 0 {
		want := make(map[string]domain.ChapterGoal, len(goals))
		for _, g := range goals {
			want[g.ID] = g
		}
		seen := make(map[string]bool, len(r.GoalChecks))
		var missing, duplicate, unknown []string
		for _, c := range r.GoalChecks {
			if _, ok := want[c.ID]; !ok {
				unknown = append(unknown, c.ID)
				continue
			}
			if seen[c.ID] {
				duplicate = append(duplicate, c.ID)
				continue
			}
			seen[c.ID] = true
		}
		for _, g := range goals {
			if !seen[g.ID] {
				missing = append(missing, g.ID)
			}
		}
		slices.Sort(duplicate)
		slices.Sort(unknown)
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, "thiếu "+strings.Join(missing, ","))
		}
		if len(duplicate) > 0 {
			parts = append(parts, "trùng "+strings.Join(duplicate, ","))
		}
		if len(unknown) > 0 {
			parts = append(parts, "lạ "+strings.Join(unknown, ","))
		}
		if len(parts) > 0 {
			return nil, fmt.Errorf("goal_checks phải phủ đúng bộ ID chương (%s): %w", strings.Join(parts, "; "), errs.ErrToolArgs)
		}

		ordered := make([]domain.GoalCheck, 0, len(goals))
		byID := make(map[string]domain.GoalCheck, len(r.GoalChecks))
		for _, c := range r.GoalChecks {
			byID[c.ID] = c
		}
		for _, g := range goals {
			c := byID[g.ID]
			c.Kind = g.Kind
			c.Text = g.Text
			quote := strings.TrimSpace(c.Quote)
			switch c.Status {
			case "pass", "fail", "unknown":
			default:
				return nil, fmt.Errorf("goal_checks %s: status không hợp lệ %q: %w", c.ID, c.Status, errs.ErrToolArgs)
			}
			note := strings.TrimSpace(c.Note)
			needQuote := (g.Kind == "rb" && c.Status == "pass") || ((g.Kind == "fm" || g.Kind == "cc") && c.Status == "fail")
			needNote := (g.Kind == "rb" && c.Status == "fail") || c.Status == "unknown"
			if needQuote && quote == "" {
				return nil, fmt.Errorf("goal_checks %s %s: cần quote nguyên văn từ chính văn: %w", c.ID, c.Status, errs.ErrToolArgs)
			}
			if needNote && note == "" {
				return nil, fmt.Errorf("goal_checks %s %s: cần note giải thích: %w", c.ID, c.Status, errs.ErrToolArgs)
			}
			if quote != "" {
				if err := locateInChapters(quote, []int{r.Chapter}, "goal_checks "+c.ID); err != nil {
					return nil, err
				}
			}
			ordered = append(ordered, c)
		}
		r.GoalChecks = ordered

		// Trạng thái hợp đồng do hệ thống suy ra; giá trị model gửi bị ghi đè.
		r.ContractStatus, r.ContractMisses = domain.DeriveContractStatus(goals, ordered)
	} else {
		r.GoalChecks = nil
	}

	affectedSet := make(map[int]struct{})
	for i := range r.Issues {
		issue := &r.Issues[i]
		if strings.TrimSpace(issue.Description) == "" {
			return nil, fmt.Errorf("issue description is required")
		}
		if strings.TrimSpace(issue.Evidence) == "" {
			return nil, fmt.Errorf("issue evidence is required")
		}
		switch issue.Severity {
		case domain.SeverityCritical, domain.SeverityError, domain.SeverityWarning:
		default:
			return nil, fmt.Errorf("invalid issue severity: %q", issue.Severity)
		}
		// D4: mục Bắt sửa (critical/error) luôn đưa chương vào hàng chờ sửa, kể cả khi
		// Editor quên bật requires_change. Đây là mắt nối duy nhất giữa "Bắt sửa" và
		// PendingRewrites: affectedSet bên dưới chỉ đọc RequiresChange, rồi
		// Progress.ApplyReviewOutcome ghi hàng chờ + flow nguyên tử.
		// Luật nằm ở domain.RequiresChangeFor, dùng chung với naolint.
		issue.RequiresChange = domain.RequiresChangeFor(issue.Severity, issue.RequiresChange)
		if err := t.validatePackIssue(issue); err != nil {
			return nil, err
		}
		if len(issue.Chapters) == 0 && r.Scope == "chapter" {
			issue.Chapters = []int{r.Chapter}
		}
		if len(issue.Chapters) == 0 {
			return nil, fmt.Errorf("issue chapters are required when scope=%s", r.Scope)
		}
		issue.Chapters = uniqueSortedChapters(issue.Chapters)
		for _, chapter := range issue.Chapters {
			switch r.Scope {
			case "chapter":
				if chapter != r.Chapter {
					return nil, fmt.Errorf("chapter review issue must reference chapter %d, got %d. Mỗi save_review(scope=chapter) chỉ chứa issue của đúng chương đó: bỏ issue chương %d khỏi lần gọi này và gọi riêng save_review(scope=chapter, chapter=%d) cho nó", r.Chapter, chapter, chapter, chapter)
				}
			case "global":
				if chapter <= 0 || chapter > r.Chapter {
					return nil, fmt.Errorf("global review issue chapter %d outside 1-%d", chapter, r.Chapter)
				}
			case "arc":
				if chapter < boundary.StartChapter || chapter > boundary.EndChapter {
					return nil, fmt.Errorf("arc review issue chapter %d outside %d-%d", chapter, boundary.StartChapter, boundary.EndChapter)
				}
			}
			if domain.RequiresChangeFor(issue.Severity, issue.RequiresChange) {
				affectedSet[chapter] = struct{}{}
			}
		}
		if strings.TrimSpace(issue.Quote) != "" {
			if err := locateInChapters(issue.Quote, issue.Chapters, fmt.Sprintf("issues[%d]", i)); err != nil {
				return nil, err
			}
		}
	}
	if err := validateDimensions(r.Dimensions); err != nil {
		return nil, err
	}
	derived := make([]int, 0, len(affectedSet))
	for chapter := range affectedSet {
		derived = append(derived, chapter)
	}
	slices.Sort(derived)
	if r.Verdict == "accept" && len(derived) > 0 {
		return nil, fmt.Errorf("accept review cannot contain issues with requires_change=true")
	}
	if (r.Verdict == "rewrite" || r.Verdict == "polish") && len(derived) == 0 {
		return nil, fmt.Errorf("verdict=%s requires at least one issue with requires_change=true", r.Verdict)
	}
	r.AffectedChapters = derived
	return boundary, nil
}

// mergeAddressFindings gộp vi phạm xưng hô đã ghi cho chương vào issues[] của
// vòng chấm (câu 13 / D9). Nguồn là meta/rule_violations.jsonl do commit_chapter
// ghi, nên vòng chấm, cửa duyệt và hàng đợi viết lại cùng một sự thật. Trả về
// true nếu có ít nhất một mục mức Bắt sửa.
func (t *SaveReviewTool) mergeAddressFindings(r *domain.ReviewEntry) bool {
	if t.store == nil || r.Chapter < 1 {
		return false
	}
	seen := make(map[string]bool, len(r.Issues))
	blocking := false
	for _, issue := range r.Issues {
		if issue.RuleRef == naolint.IssueRef && domain.RequiresChangeFor(issue.Severity, issue.RequiresChange) {
			blocking = true
		}
		seen[issue.Description] = true
	}
	for _, v := range t.store.World.LoadRuleViolations(r.Chapter) {
		if v.Rule != naolint.AddressRule {
			continue
		}
		issue := naolint.Issue(v, r.Chapter)
		if seen[issue.Description] {
			continue
		}
		seen[issue.Description] = true
		if domain.RequiresChangeFor(issue.Severity, issue.RequiresChange) {
			blocking = true
		}
		r.Issues = append(r.Issues, issue)
	}
	return blocking
}

func normalizeRuleText(text string) string {
	text = norm.NFC.String(strings.ToLower(strings.ReplaceAll(text, "**", "")))
	return strings.Join(strings.Fields(text), " ")
}

func (t *SaveReviewTool) validatePackIssue(issue *domain.ConsistencyIssue) error {
	if t.packRules == nil || issue.RuleRef == "" || issue.RuleRef == naolint.IssueRef {
		return nil
	}
	items, ok := t.packRules[issue.RuleRef]
	if !ok {
		refs := make([]string, 0, len(t.packRules))
		for ref := range t.packRules {
			refs = append(refs, ref)
		}
		slices.Sort(refs)
		if len(refs) > 3 {
			refs = refs[:3]
		}
		return fmt.Errorf("rule_ref %q không tồn tại trong gói; dùng đúng dạng <nhãn tệp> §<số mục>, ví dụ: %s", issue.RuleRef, strings.Join(refs, ", "))
	}
	label := ""
	mixed := false
	for _, item := range items {
		if item.Label == "" {
			continue
		}
		if label != "" && label != item.Label {
			mixed = true
		}
		label = item.Label
	}
	if mixed {
		fragment := normalizeRuleText(issue.RuleText)
		matches := 0
		if utf8.RuneCountInString(fragment) >= 15 {
			for _, item := range items {
				if strings.Contains(item.Text, fragment) {
					matches++
					label = item.Label
				}
			}
		}
		if matches != 1 || label == "" {
			return fmt.Errorf("mục %s có cả LỖI và CẢNH BÁO; hãy điền rule_text bằng đoạn chép nguyên chính câu luật bị vi phạm (ít nhất 15 ký tự); nếu không khớp hoặc khớp nhiều câu, chép đoạn dài hơn và chính xác", issue.RuleRef)
		}
	}
	// Nhãn LỖI chỉ hợp lệ khi chính mức của mục đã là Bắt sửa và có trích dẫn;
	// requires_change đi kèm tự động nhờ domain.RequiresChangeFor.
	if label == "LỖI" && (!domain.RequiresChangeFor(issue.Severity, false) || strings.TrimSpace(issue.Quote) == "") {
		return fmt.Errorf("vi phạm %s nhãn LỖI: sửa severity thành error hoặc critical, đặt requires_change=true và điền quote bằng đoạn trích nguyên văn không rỗng trong chương", issue.RuleRef)
	}
	return nil
}

func uniqueSortedChapters(chapters []int) []int {
	seen := make(map[int]struct{}, len(chapters))
	for _, chapter := range chapters {
		seen[chapter] = struct{}{}
	}
	result := make([]int, 0, len(seen))
	for chapter := range seen {
		result = append(result, chapter)
	}
	slices.Sort(result)
	return result
}

// reviewFlow 是文学裁定与持久化协议之间唯一的映射点。verdict 由 Editor 决定；
// 这里只接受 Router 能恢复的三种控制结果。
func reviewFlow(verdict string) (domain.FlowState, error) {
	switch verdict {
	case "accept":
		return domain.FlowWriting, nil
	case "polish":
		return domain.FlowPolishing, nil
	case "rewrite":
		return domain.FlowRewriting, nil
	default:
		return "", fmt.Errorf("invalid review verdict: %q", verdict)
	}
}

func validateDimensions(dimensions []domain.DimensionScore) error {
	if len(dimensions) == 0 {
		return fmt.Errorf("dimensions must contain at least one evidence-based assessment")
	}

	seen := make(map[string]struct{}, len(dimensions))
	for _, dim := range dimensions {
		name := strings.TrimSpace(dim.Dimension)
		if name == "" {
			return fmt.Errorf("dimension name is required")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate dimension: %s", name)
		}
		seen[name] = struct{}{}
		if dim.Score < 0 || dim.Score > 100 {
			return fmt.Errorf("invalid score for %s: %d", dim.Dimension, dim.Score)
		}
		if strings.TrimSpace(dim.Comment) == "" {
			return fmt.Errorf("dimension comment is required: %s", dim.Dimension)
		}
	}
	return nil
}

// recheckSource reads the original round even after an artifact-first crash.
func recheckSource(s *store.Store, chapter int) (*domain.ReviewEntry, error) {
	current, err := s.World.LoadReview(chapter)
	if err != nil {
		return nil, err
	}
	if current != nil && current.Recheck {
		rounds, err := s.World.LoadReviewRounds(chapter, store.ReviewScopeChapter)
		if err != nil {
			return nil, err
		}
		if len(rounds) == 0 {
			return nil, fmt.Errorf("recheck thiếu vòng nguồn: %w", errs.ErrToolPrecondition)
		}
		current = &rounds[len(rounds)-1]
	}
	if current == nil || current.Scope != "chapter" || current.Recheck {
		return nil, fmt.Errorf("không có review nguồn cho kiểm lại: %w", errs.ErrToolPrecondition)
	}
	return current, nil
}

func (t *SaveReviewTool) validateRecheck(r *domain.ReviewEntry, progress *domain.Progress) error {
	if !r.Recheck {
		if len(r.RecheckResults) > 0 {
			return fmt.Errorf("review thường cần recheck_results rỗng: %w", errs.ErrToolArgs)
		}
		if slices.Contains(progress.PendingRechecks, r.Chapter) && r.Scope == "chapter" {
			return fmt.Errorf("chương đang chờ kiểm lại, cần recheck=true: %w", errs.ErrToolPrecondition)
		}
		return nil
	}
	if r.Scope != "chapter" {
		return fmt.Errorf("recheck chỉ áp dụng scope=chapter: %w", errs.ErrToolPrecondition)
	}
	current, err := t.store.World.LoadReview(r.Chapter)
	if err != nil {
		return fmt.Errorf("load current recheck: %w", err)
	}
	if !slices.Contains(progress.PendingRechecks, r.Chapter) {
		if current != nil && current.Recheck {
			r.RuleAware = current.RuleAware
			same, err := sameStoredReview(*current, *r)
			if err != nil {
				return err
			}
			if same {
				return nil
			}
		}
		return fmt.Errorf("chương không nằm trong PendingRechecks: %w", errs.ErrToolPrecondition)
	}
	source, err := recheckSource(t.store, r.Chapter)
	if err != nil {
		return err
	}
	text, err := t.store.Drafts.LoadChapterText(r.Chapter)
	if err != nil {
		return fmt.Errorf("load recheck text: %w", err)
	}
	ids := domain.ReviewEntryItemIDs(*source)
	want := make(map[string]domain.ConsistencyIssue)
	for i, issue := range source.Issues {
		if domain.RequiresChangeFor(issue.Severity, issue.RequiresChange) && domain.ReviewIssueTargets(*source, issue, r.Chapter) {
			want[ids[i]] = issue
		}
	}
	seen := make(map[string]bool)
	for _, result := range r.RecheckResults {
		issue, ok := want[result.FindingID]
		if !ok || seen[result.FindingID] {
			return fmt.Errorf("recheck_results ID lạ/trùng %q: %w", result.FindingID, errs.ErrToolArgs)
		}
		seen[result.FindingID] = true
		quote := strings.TrimSpace(result.Quote)
		located := false
		if quote != "" {
			_, _, located = domain.LocateQuote(text, quote)
			if !located {
				return fmt.Errorf("recheck_results quote không tìm thấy: %w", errs.ErrToolArgs)
			}
		}
		switch result.Status {
		case "fixed":
			if strings.TrimSpace(result.Note) == "" {
				return fmt.Errorf("fixed cần note: %w", errs.ErrToolArgs)
			}
			oldQuote := issue.Quote
			if strings.TrimSpace(oldQuote) == "" {
				oldQuote = issue.Evidence
			}
			if _, _, present := domain.LocateQuote(text, oldQuote); present {
				if !located || domain.NormalizeQuote(quote) == domain.NormalizeQuote(oldQuote) {
					return fmt.Errorf("đoạn lỗi còn nguyên; fixed cần quote mới cho ngữ cảnh đã đổi: %w", errs.ErrToolArgs)
				}
			}
		case "unfixed":
			matched := false
			for _, next := range r.Issues {
				if next.RequiresChange && domain.ReviewIssueTargets(*r, next, r.Chapter) && domain.NormalizeQuote(next.Quote) == domain.NormalizeQuote(quote) {
					matched = true
					break
				}
			}
			if !located || !matched {
				return fmt.Errorf("unfixed cần quote và issue requires_change trùng quote: %w", errs.ErrToolArgs)
			}
		case "unknown":
			if strings.TrimSpace(result.Note) == "" {
				return fmt.Errorf("unknown cần note: %w", errs.ErrToolArgs)
			}
		default:
			return fmt.Errorf("recheck status không hợp lệ %q: %w", result.Status, errs.ErrToolArgs)
		}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("recheck_results thiếu finding_id, phải phủ đúng bộ: %w", errs.ErrToolArgs)
	}
	if current != nil && current.Recheck {
		r.RuleAware = current.RuleAware
		same, err := sameStoredReview(*current, *r)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("recheck đã lưu nội dung khác: %w", errs.ErrToolConflict)
		}
	}
	return nil
}
