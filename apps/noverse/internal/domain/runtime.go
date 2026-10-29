package domain

import (
	"fmt"
	"strings"
	"time"
)

// Phase 表示小说创作阶段。
type Phase string

const (
	PhaseInit     Phase = "init"
	PhasePremise  Phase = "premise"
	PhaseOutline  Phase = "outline"
	PhaseWriting  Phase = "writing"
	PhaseComplete Phase = "complete"
)

// FlowState 当前活动流程类型，用于 checkpoint 恢复。
type FlowState string

const (
	FlowWriting   FlowState = "writing"
	FlowReviewing FlowState = "reviewing"
	FlowRewriting FlowState = "rewriting"
	FlowPolishing FlowState = "polishing"
	FlowSteering  FlowState = "steering"
)

// PlanningTier 表示作品规划的长度级别。
type PlanningTier string

const (
	PlanningTierShort PlanningTier = "short"
	PlanningTierMid   PlanningTier = "mid"
	PlanningTierLong  PlanningTier = "long"
)

// Progress 进度追踪，持久化到 meta/progress.json。
type Progress struct {
	Phase          Phase `json:"phase"`
	CurrentChapter int   `json:"current_chapter"`
	// TotalChapters 在非分层模式是详细大纲章数；在分层模式仅是包含骨架估算的
	// 内部容量值，用于上下文策略，不代表全书固定总章数。
	TotalChapters     int         `json:"total_chapters"`
	CompletedChapters []int       `json:"completed_chapters"`
	TotalWordCount    int         `json:"total_word_count"`
	ChapterWordCounts map[int]int `json:"chapter_word_counts,omitempty"` // 每章字数，支持重写时修正总字数
	InProgressChapter int         `json:"in_progress_chapter,omitempty"` // 正在写作的章节（场景级恢复）
	CompletedScenes   []int       `json:"completed_scenes,omitempty"`    // 当前章节已完成的场景编号
	Flow              FlowState   `json:"flow,omitempty"`                // 当前流程
	PendingRewrites   []int       `json:"pending_rewrites,omitempty"`    // 待重写章节队列
	PendingRechecks   []int       `json:"pending_rechecks,omitempty"`
	RewriteReason     string      `json:"rewrite_reason,omitempty"` // 重写原因
	StrandHistory     []string    `json:"strand_history,omitempty"` // 按章节顺序记录 dominant_strand
	HookHistory       []string    `json:"hook_history,omitempty"`   // 按章节顺序记录 hook_type
	// 长篇分层追踪（仅长篇模式使用，短篇/中篇为零值）
	CurrentVolume int  `json:"current_volume,omitempty"`
	CurrentArc    int  `json:"current_arc,omitempty"`
	Layered       bool `json:"layered,omitempty"`
	// ReopenedFromComplete 标记本书是经 reopen 从完结态重开进入返工的。返工只改已有章、
	// 不增减结构，故排空后应按"结构完整即重新完结"放行（避免终卷末伏笔被返工扰动后卡在
	// writing → 越界续写死循环）；正向写作不置此标记，完结判定保持线索收束的保守语义。
	ReopenedFromComplete bool `json:"reopened_from_complete,omitempty"`
	// ReopenCount 记录本书从完结态被重开的累计次数（/reopen 审计事实）。它同时保证
	// 重开后的再完结与上次完结的 progress.json 内容不同：checkpoint 对同 digest 幂等
	// 去重，字节相同的再完结不会产生新 checkpoint，StopGuard 会把成功的 complete_book
	// 误判为空转并升级终止。
	ReopenCount int `json:"reopen_count,omitempty"`
	// CompletionVersion versions the persisted completion metadata independently of legacy progress.
	CompletionVersion int        `json:"completion_version,omitempty"`
	LockedAt          *time.Time `json:"locked_at,omitempty"`
}

// ReviewWorkDrained is shared by routing, holds and completion.
func (p *Progress) ReviewWorkDrained() bool {
	return p != nil && len(p.PendingRewrites) == 0 && len(p.PendingRechecks) == 0
}

// RewriteEntry là một lần chương thực sự bước vào viết lại, ghi ở
// meta/rewrites.jsonl (chỉ thêm, không sửa). Cause: review = Editor chấm ra
// Bắt sửa; user = người dùng tự yêu cầu (góp ý đã duyệt, sửa xưng hô/quan hệ,
// mở lại sách để viết lại). Chỉ ghi từ bản này trở đi, không dựng lại lịch sử.
type RewriteEntry struct {
	Chapter int    `json:"chapter"`
	At      string `json:"at"`
	Cause   string `json:"cause"`
}

// IsResumable 判断是否可以从断点恢复。
func (p *Progress) IsResumable() bool {
	return p.Phase == PhaseWriting && p.CurrentChapter > 0
}

// NextChapter 返回下一个要写的章节号。
func (p *Progress) NextChapter() int {
	return p.LatestCompleted() + 1
}

// LatestCompleted 返回最大已完成章节号；无已完成章节时返回 0。
func (p *Progress) LatestCompleted() int {
	max := 0
	for _, ch := range p.CompletedChapters {
		if ch > max {
			max = ch
		}
	}
	return max
}

// ContextProfile 上下文加载策略，根据总章节数自适应。
type ContextProfile struct {
	SummaryWindow  int  // 加载最近 N 章摘要
	TimelineWindow int  // 加载最近 N 章时间线
	Layered        bool // true = 启用分层摘要加载（卷摘要+弧摘要+章摘要）
}

// MemoryPolicy 表示运行时共享的记忆使用策略。
// 它既用于上下文输出，也用于宿主层的 handoff / reminder 决策。
type MemoryPolicy struct {
	Mode                string `json:"mode,omitempty"`
	SummaryWindow       int    `json:"summary_window,omitempty"`
	TimelineWindow      int    `json:"timeline_window,omitempty"`
	LayeredSummaries    bool   `json:"layered_summaries,omitempty"`
	SummaryStrategy     string `json:"summary_strategy,omitempty"`
	WorkingRefresh      string `json:"working_refresh,omitempty"`
	EpisodicRefresh     string `json:"episodic_refresh,omitempty"`
	PlanningRefresh     string `json:"planning_refresh,omitempty"`
	FoundationRefresh   string `json:"foundation_refresh,omitempty"`
	PlanningFocus       string `json:"planning_focus,omitempty"`
	FoundationFocus     string `json:"foundation_focus,omitempty"`
	PreviousTailChars   int    `json:"previous_tail_chars,omitempty"`
	ChapterPlanEnabled  bool   `json:"chapter_plan_enabled,omitempty"`
	RelatedLookup       bool   `json:"related_chapter_lookup,omitempty"`
	CurrentOutlineBound bool   `json:"current_outline_bound,omitempty"`
	HandoffPreferred    bool   `json:"handoff_preferred,omitempty"`
	ReadOnlyThreshold   int    `json:"read_only_threshold,omitempty"`
}

// DefaultPreviousTailChars 是上一章正文尾部注入上下文的默认长度（rune）。
const DefaultPreviousTailChars = 800

// NewContextProfile 根据总章节数计算上下文策略。
func NewContextProfile(totalChapters int) ContextProfile {
	switch {
	case totalChapters <= 15:
		return ContextProfile{SummaryWindow: 10, TimelineWindow: 10}
	case totalChapters <= 50:
		return ContextProfile{SummaryWindow: 5, TimelineWindow: 8}
	default:
		return ContextProfile{SummaryWindow: 3, TimelineWindow: 5, Layered: true}
	}
}

// NewChapterMemoryPolicy 根据进度与上下文策略生成章节运行时记忆策略。
func NewChapterMemoryPolicy(progress *Progress, profile ContextProfile, currentOutlineBound bool) MemoryPolicy {
	policy := MemoryPolicy{
		Mode:                "chapter",
		SummaryWindow:       profile.SummaryWindow,
		TimelineWindow:      profile.TimelineWindow,
		LayeredSummaries:    profile.Layered,
		WorkingRefresh:      "làm mới mỗi lần nạp theo chương",
		EpisodicRefresh:     "làm mới khi nộp chương, thẩm duyệt và trạng thái truyện dài thay đổi",
		PreviousTailChars:   DefaultPreviousTailChars,
		ChapterPlanEnabled:  true,
		CurrentOutlineBound: currentOutlineBound,
		ReadOnlyThreshold:   5,
	}
	if profile.Layered {
		policy.SummaryStrategy = "tóm tắt tập + tóm tắt cung + tóm tắt chương gần nhất"
	} else {
		policy.SummaryStrategy = "tóm tắt chương gần nhất"
	}
	if progress != nil {
		if progress.TotalChapters > 30 {
			policy.RelatedLookup = true
		}
		if progress.Flow == FlowReviewing || progress.Flow == FlowRewriting || progress.Flow == FlowPolishing {
			policy.HandoffPreferred = true
		}
		if progress.Layered && len(progress.CompletedChapters) >= 6 {
			policy.HandoffPreferred = true
		}
		if len(progress.CompletedChapters) >= 12 {
			policy.HandoffPreferred = true
		}
		if progress.Layered && len(progress.CompletedChapters) >= 6 {
			policy.ReadOnlyThreshold = 4
		}
		if len(progress.CompletedChapters) >= 12 {
			policy.ReadOnlyThreshold = 4
		}
	}
	return policy
}

// NewArchitectMemoryPolicy 返回规划阶段使用的记忆策略。
func NewArchitectMemoryPolicy() MemoryPolicy {
	return MemoryPolicy{
		Mode:               "architect",
		PlanningRefresh:    "làm mới khi cấu trúc tập/cung, compass hoặc tóm tắt thay đổi",
		FoundationRefresh:  "làm mới khi nhân vật, phục bút, thiết lập thay đổi",
		PlanningFocus:      "đại cương phân tầng, compass, tóm tắt tập",
		FoundationFocus:    "thiết lập nhân vật, ảnh chụp nhân vật, sổ phục bút",
		HandoffPreferred:   true,
		ChapterPlanEnabled: false,
		ReadOnlyThreshold:  4,
	}
}

// RunMeta 运行元信息，持久化到 meta/run.json。
type RunMeta struct {
	StartedAt            string             `json:"started_at"`
	Provider             string             `json:"provider,omitempty"`
	Style                string             `json:"style"`
	Model                string             `json:"model"`
	PlanningTier         PlanningTier       `json:"planning_tier,omitempty"`
	StartPrompt          string             `json:"start_prompt,omitempty"`           // 用户原始创作需求（输入事实，先于启动裁定落盘；裁定失败后据此补裁）
	PlanStart            *PlanStartRecord   `json:"plan_start,omitempty"`             // 启动裁定事实，规划期崩溃恢复的唯一依据
	PendingSteer         string             `json:"pending_steer,omitempty"`          // 未完成的 Steer 指令，中断恢复时重新注入
	AdvanceMode          ChapterAdvanceMode `json:"advance_mode"`                     // 章节推进模式：auto / review
	AdvancePermitChapter int                `json:"advance_permit_chapter,omitempty"` // review 模式下一次性许可的正向章节
	AdvanceHold          *AdvanceHold       `json:"advance_hold,omitempty"`           // 当前干预签署的一次性暂停意图
	PendingCommentReview *CommentReview     `json:"pending_comment_review,omitempty"`
	PendingAsk           *PendingAsk        `json:"pending_ask,omitempty"` // 当前等待用户回答的仲裁提问（engine 已停下等回答）
}

// CommentReview is an explicit Writer dispatch approved together with the next
// chapter permit. Both facts live in one atomic run metadata write.
type CommentReview struct {
	Chapter     int    `json:"chapter"`
	Instruction string `json:"instruction"`
}

// ChapterAdvanceMode 决定新章节是否需要逐章许可。
type ChapterAdvanceMode string

const (
	ChapterAdvanceAuto   ChapterAdvanceMode = "auto"
	ChapterAdvanceReview ChapterAdvanceMode = "review"
)

// Valid 报告章节推进模式是否受当前版本支持。
func (m ChapterAdvanceMode) Valid() bool {
	return m == ChapterAdvanceAuto || m == ChapterAdvanceReview
}

// UnsupportedAdvanceModeError 表示书的控制模式不受当前二进制支持。
// 调用方必须停止构造可写 Host，并提示用户使用匹配版本；禁止猜测降级。
type UnsupportedAdvanceModeError struct {
	Mode ChapterAdvanceMode
}

func (e *UnsupportedAdvanceModeError) Error() string {
	return fmt.Sprintf("Chế độ đi tiếp chương %q không được hỗ trợ, hãy dùng bản Noverse mới hơn — bản đã tạo dự án này", e.Mode)
}

// AdvanceHoldAfter 是一次性暂停的确定性触发条件。
type AdvanceHoldAfter string

const (
	AdvanceHoldAtBoundary           AdvanceHoldAfter = "boundary"
	AdvanceHoldAfterRewritesDrained AdvanceHoldAfter = "rewrites_drained"
	AdvanceHoldAtChapter            AdvanceHoldAfter = "chapter"
)

// Valid 报告暂停条件是否受当前版本支持。
func (a AdvanceHoldAfter) Valid() bool {
	return a == AdvanceHoldAtBoundary || a == AdvanceHoldAfterRewritesDrained || a == AdvanceHoldAtChapter
}

// AdvanceHold 是当前干预签署的一次性暂停意图，由 Host 边界消费。
type AdvanceHold struct {
	After         AdvanceHoldAfter `json:"after"`
	TargetChapter int              `json:"target_chapter,omitempty"`
	Reason        string           `json:"reason"`
}

// Validate 校验一次性暂停意图自身的结构约束。
func (h AdvanceHold) Validate() error {
	if !h.After.Valid() {
		return fmt.Errorf("Điều kiện tạm dừng một lần %q không được hỗ trợ", h.After)
	}
	if h.After == AdvanceHoldAtChapter {
		if h.TargetChapter <= 0 {
			return fmt.Errorf("Chương đích phải lớn hơn 0")
		}
	} else if h.TargetChapter != 0 {
		return fmt.Errorf("Điều kiện tạm dừng %q không được đặt chương đích", h.After)
	}
	if strings.TrimSpace(h.Reason) == "" {
		return fmt.Errorf("Lý do tạm dừng một lần không được để trống")
	}
	return nil
}

// PlanStartRecord 启动裁定的持久化事实(裁定先落事实,再起执行;恢复不重新裁定)。
// 首个 save_foundation 落盘 scale 后,规划期恢复改由 PlanningTier 推导,本记录
// 只覆盖"裁定完成到首次落盘之间"的窗口。DecisionID 关联 decisions.jsonl 审计。
type PlanStartRecord struct {
	RawPrompt   string `json:"raw_prompt"`
	Planner     string `json:"planner"`
	PlannerTask string `json:"planner_task"`
	DecisionID  string `json:"decision_id,omitempty"`
}

// PendingAskOption 是仲裁给用户的候选项。Reply 是发回干预通道的完整文本；
// 为空时 UI 回退用 Label 作答。至多一个 IsDefault（界面上的主按钮）。
type PendingAskOption struct {
	Label     string `json:"label"`
	Reply     string `json:"reply,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// PendingAsk 是仲裁签署、由 Host 在引擎停下后展示的待回答问题。与
// AdvanceHold 的区别：hold 是一次性暂停“意图”，在引擎停下的边界被消费；
// ask 是已经生效的“停着等回答”状态，只有新的用户干预把它结清。
type PendingAsk struct {
	Question string             `json:"question"`
	Options  []PendingAskOption `json:"options,omitempty"`
	Chapter  int                `json:"chapter,omitempty"`
}

// Validate 校验待回答问题自身的结构约束。
func (a PendingAsk) Validate() error {
	if strings.TrimSpace(a.Question) == "" {
		return fmt.Errorf("câu hỏi của Trọng tài không được để trống")
	}
	if a.Chapter < 0 {
		return fmt.Errorf("chương chờ không được âm: %d", a.Chapter)
	}
	defaults := 0
	for i, o := range a.Options {
		if strings.TrimSpace(o.Label) == "" {
			return fmt.Errorf("phương án %d không có nhãn", i+1)
		}
		if o.IsDefault {
			defaults++
		}
	}
	if defaults > 1 {
		return fmt.Errorf("chỉ được có một phương án mặc định")
	}
	return nil
}
