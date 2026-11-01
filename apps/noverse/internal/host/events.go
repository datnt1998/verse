package host

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// Event 是 TUI 消费的结构化事件。
//
// 对于 TOOL / DISPATCH / DECISION 三类调用事件，同一次调用的开始与结束共用一个 ID：
// 开始时先发 FinishedAt 为零值的事件（TUI 渲染为"进行中"样式）；
// 结束时再发一条同 ID 的事件，填入 FinishedAt + Duration（+ Failed），
// TUI 按 ID 定位原行原地更新，避免"开始一行、完成又一行"的冗余。
//
// SYSTEM / ERROR / CONTEXT 等非调用类事件 ID 为空，每条独立追加。
type Event struct {
	ID         string        `json:"id"`
	Time       time.Time     `json:"time"`
	FinishedAt time.Time     `json:"finishedAt"`
	Failed     bool          `json:"failed"`
	Category   string        `json:"category"`
	Agent      string        `json:"agent"`
	Summary    string        `json:"summary"`
	Detail     string        `json:"-"`
	Kind       string        `json:"kind"`
	Level      string        `json:"level"`
	Depth      int           `json:"depth"`
	Duration   time.Duration `json:"durationNs"`
	RetryAt    time.Time     `json:"retryAt"`
	// SessionID 标识事件所属的引擎会话（G3）：每次引擎开跑（start/resume/continue/
	// reopen/import）自增。空串表示会话开始之前的事件。
	SessionID string `json:"sessionId"`
	// CanRetryNow 表示该退避等待可被 POST /retry 立即放行（G4）：模型包装器的
	// 开流前重试为 true；agentcore 流中途重试无法唤醒，为 false。
	CanRetryNow    bool       `json:"canRetryNow"`
	Chapter        int        `json:"chapter"`
	Scope          string     `json:"scope,omitempty"`
	Volume         int        `json:"volume,omitempty"`
	Arc            int        `json:"arc,omitempty"`
	ChapterFrom    int        `json:"chapterFrom,omitempty"`
	ChapterTo      int        `json:"chapterTo,omitempty"`
	Username       string     `json:"username,omitempty"`
	Command        string     `json:"command,omitempty"`
	Input          string     `json:"input,omitempty"`
	Tool           string     `json:"tool,omitempty"`
	RoundTitle     string     `json:"roundTitle,omitempty"`
	RoundID        string     `json:"roundId,omitempty"`
	RoundStartedAt *time.Time `json:"roundStartedAt,omitempty"`
	Role           string     `json:"role,omitempty"`
	Error          string     `json:"error,omitempty"`
	Args           string     `json:"args,omitempty"`
	Result         string     `json:"result,omitempty"`
	// ResultHint giữ vài con số người đọc cần (số chữ, số vấn đề) từ kết quả gốc — `Result`
	// bị cắt 800 ký tự nên với chương dài phần đuôi JSON mất số liệu.
	ResultHint string       `json:"resultHint,omitempty"`
	DurationMs int64        `json:"durationMs,omitempty"`
	Status     string       `json:"status,omitempty"`
	Retries    []EventRetry `json:"retries,omitempty"`
	// Gói AU: nhà cung cấp/model của sự kiện fallback.active hoặc provider.retry.
	// Kind = "fallback.active" → FromProvider/FromModel là model chính vừa lỗi,
	// Provider/Model là model dự phòng đang dùng. Kind = "provider.retry" →
	// RetryAt là mốc tự thử lại, Reason là expired|key|quota.
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	FromProvider string `json:"fromProvider,omitempty"`
	FromModel    string `json:"fromModel,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// Internal marks reasoning that belongs in audit logs, never in user chat.
// Older logs predate Kind; recognize only their system-generated prefixes.
func (e Event) Internal() bool {
	if e.Kind == "internal" {
		return true
	}
	if !strings.EqualFold(e.Category, "SYSTEM") {
		return false
	}
	text := LegacyEventText(strings.TrimSpace(e.Summary))
	return strings.HasPrefix(text, "Phán định:")
}

// EventRetry is one retry attempt reported by the agent kernel.
type EventRetry struct {
	AtSec   float64 `json:"atSec"`
	Error   string  `json:"error"`
	WaitSec float64 `json:"waitSec"`
	OK      bool    `json:"ok"`
}

// FallbackSnapshot is the "đang viết bằng model dự phòng" banner state. It
// survives a page reload because it travels in the worker snapshot, not only
// in the event stream.
type FallbackSnapshot struct {
	Active       bool      `json:"active"`
	Role         string    `json:"role"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	FromProvider string    `json:"fromProvider"`
	FromModel    string    `json:"fromModel"`
	Reason       string    `json:"reason"`
	Since        time.Time `json:"since"`
}

// ProviderErrorSnapshot is the classified provider error strip (AU3).
type ProviderErrorSnapshot struct {
	Provider string     `json:"provider"`
	Kind     string     `json:"kind"`
	Message  string     `json:"message"`
	RetryAt  *time.Time `json:"retryAt,omitempty"`
}

// Running 返回事件是否处于进行中。
// 仅调用类事件（有 ID 的 TOOL / DISPATCH / DECISION）可能进行中；其它类型总是返回 false。
func (e Event) Running() bool {
	return e.hasLifecycle() && e.FinishedAt.IsZero()
}

func (e Event) hasLifecycle() bool {
	if e.ID == "" {
		return false
	}
	switch e.Category {
	case "TOOL", "DISPATCH", "DECISION":
		return true
	default:
		return false
	}
}

// UISnapshot 是 TUI 渲染所需的聚合状态快照。
type UISnapshot struct {
	StreamText         string `json:"streamText,omitempty"`
	ReadOnlyReason     string `json:"readOnlyReason,omitempty"`
	Provider           string `json:"provider"`
	BookTitle          string `json:"bookTitle"`
	ModelName          string `json:"modelName"`
	ModelContextWindow int    `json:"modelContextWindow"`
	ThinkingLevel      string `json:"thinkingLevel"`
	Style              string `json:"style"`
	RuntimeState       string `json:"runtimeState"`
	StatusLabel        string `json:"statusLabel"`
	// WaitingFor 表示工作台状态芯片的等待原因（G5）："" · "gate" · "cocreate" ·
	// "confirm"（导入待确认，由 worker 叠加）· "input"（空闲待命令）。
	WaitingFor           string `json:"waitingFor"`
	Phase                string `json:"phase"`
	Flow                 string `json:"flow"`
	CurrentChapter       int    `json:"currentChapter"`
	TotalChapters        int    `json:"totalChapters"`
	CompletedCount       int    `json:"completedCount"`
	TotalWordCount       int    `json:"totalWordCount"`
	InProgressChapter    int    `json:"inProgressChapter"`
	PendingRewrites      []int  `json:"pendingRewrites"`
	RewriteReason        string `json:"rewriteReason"`
	PendingSteer         string `json:"pendingSteer"`
	AdvanceMode          string `json:"advanceMode"`
	AdvancePermitChapter int    `json:"advancePermitChapter"`
	HasAdvanceHold       bool   `json:"hasAdvanceHold"`
	AdvanceHoldReason    string `json:"advanceHoldReason"`
	// AdvanceHoldChapter 是一次性暂停意图的章节目标（after=chapter），供 Đại cương 行加色。
	AdvanceHoldChapter int `json:"advanceHoldChapter"`
	// PendingAsk 是 Trọng tài 正在等用户回答的提问（Gói P · P4）；无则 null。
	PendingAsk *PendingAskSnapshot `json:"pendingAsk,omitempty"`
	// Gói AU: banner "đang viết bằng model dự phòng" và băng lỗi nhà cung cấp
	// đã phân loại (expired/key/quota) — sống qua lần tải lại trang.
	Fallback               *FallbackSnapshot      `json:"fallback,omitempty"`
	ProviderError          *ProviderErrorSnapshot `json:"providerError,omitempty"`
	RecoveryLabel          string                 `json:"recoveryLabel"`
	IsRunning              bool                   `json:"isRunning"`
	Agents                 []AgentSnapshot        `json:"agents"`
	TotalInputTokens       int                    `json:"totalInputTokens"`
	TotalOutputTokens      int                    `json:"totalOutputTokens"`
	TotalCacheReadTokens   int                    `json:"totalCacheReadTokens"`
	TotalCacheWriteTokens  int                    `json:"totalCacheWriteTokens"`
	TotalCostUSD           float64                `json:"totalCostUSD"`
	TotalSavedUSD          float64                `json:"totalSavedUSD"`
	BudgetLimitUSD         float64                `json:"budgetLimitUSD"`
	OverallCacheCapable    bool                   `json:"overallCacheCapable"`
	OverallRecentCacheRead int                    `json:"overallRecentCacheRead"`
	OverallRecentInput     int                    `json:"overallRecentInput"`
	OverallRecentSamples   int                    `json:"overallRecentSamples"`
	TotalCacheBreaks       int                    `json:"totalCacheBreaks"`
	MissingAssistantUsage  int                    `json:"missingAssistantUsage"`
	CachePerAgent          []AgentCacheStat       `json:"cachePerAgent"`
	CachePerModel          []AgentCacheStat       `json:"cachePerModel"`
	Synopsis               string                 `json:"synopsis"`
	Premise                string                 `json:"premise"`
	Outline                []OutlineSnapshot      `json:"outline"`
	Characters             []string               `json:"characters"`
	SupportingCount        int                    `json:"supportingCount"`
	RecentSupporting       []string               `json:"recentSupporting"`
	Layered                bool                   `json:"layered"`
	CurrentVolumeArc       string                 `json:"currentVolumeArc"`
	NextVolumeTitle        string                 `json:"nextVolumeTitle"`
	CompassDirection       string                 `json:"compassDirection"`
	CompassScale           string                 `json:"compassScale"`
	OpenThreads            []string               `json:"openThreads"`
	PlanningTier           domain.PlanningTier    `json:"planningTier"`
	LastCommitSummary      string                 `json:"lastCommitSummary"`
	LastReviewSummary      string                 `json:"lastReviewSummary"`
	LastCheckpointName     string                 `json:"lastCheckpointName"`
	RecentSummaries        []string               `json:"recentSummaries"`
	// Plan 是规划期“Kế hoạch truyện”面板数据（Gói P），写作/完结期为 nil。
	Plan *PlanSnapshot `json:"plan,omitempty"`
	// OnceResult 是"单指令运行"(RunInstructionOnce)唯一 Worker 的回收结果：
	// worker 返回后记录，供快照消费者读取；正常 run 恒为 nil。
	OnceResult *OnceResult `json:"onceResult,omitempty"`
}

// OnceResult 记录一次单指令运行的派发结果。
// Finished=true 表示那个 Worker 已经返回；Err 是 worker 错误文本(成功为空)。
type OnceResult struct {
	Agent    string `json:"agent"`
	Finished bool   `json:"finished"`
	Err      string `json:"err,omitempty"`
}

// PendingAskSnapshot 是等待用户回答的仲裁提问投影（Gói P · P4）：
// 卡片按钮=options（isDefault 是主按钮），chapter 是 Đại cương 需要加色的等待章。
type PendingAskSnapshot struct {
	Question string          `json:"question"`
	Options  []AskOptionItem `json:"options,omitempty"`
	Chapter  int             `json:"chapter,omitempty"`
}

// AskOptionItem 是一个候选项；label 为按钮文字，reply 为发回干预通道的完整文本。
type AskOptionItem struct {
	Label     string `json:"label"`
	Reply     string `json:"reply,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// PlanSnapshot 是“Kế hoạch truyện”面板的聚合投影（Gói P · 21-dev-prompt.md
// P1）：规划期五步的事实视图，由 save_book / save_foundation 已落盘工件 +
// checkpoints.jsonl 推导。仅在仍有基础设定缺项且未进入写作时输出。
type PlanSnapshot struct {
	// RunningStep 是面板推定正在执行的步骤 key（book|premise|characters|worldRules|outline）；
	// 全部落盘后（仅剩 foundation_audit）为空。
	RunningStep string     `json:"runningStep,omitempty"`
	Done        int        `json:"done"`
	Total       int        `json:"total"`
	Steps       []PlanStep `json:"steps"`
}

// PlanStep 是规划面板的一行：状态、单行摘要、时长与展开内容（Detail）。
// Status: done | running | paused | pending。
type PlanStep struct {
	Key        string          `json:"key"`
	Status     string          `json:"status"`
	Summary    string          `json:"summary,omitempty"`
	DurationMs int64           `json:"durationMs,omitempty"`
	StartedAt  time.Time       `json:"startedAt,omitempty"`
	Detail     json.RawMessage `json:"detail,omitempty"`
}

// PlanBookDetail 是“Thông tin truyện”步的展开内容：界面渲染为键值格。
type PlanBookDetail struct {
	Kind     string `json:"kind"` // "book"
	Title    string `json:"title"`
	Synopsis string `json:"synopsis"`
}

// PlanPremiseDetail 是“Tiền đề”步的展开内容：界面渲染为段落。
type PlanPremiseDetail struct {
	Kind string `json:"kind"` // "premise"
	Text string `json:"text"`
}

// PlanCharacter 是“Nhân vật”步展开内容里的一个人物 chip。
type PlanCharacter struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
	// Kind là loại vai (06-data-contract › Bổ sung §S) — chip dùng màu avatar theo loại.
	Kind string `json:"kind,omitempty"`
}

// PlanCharactersDetail 是“Nhân vật”步的展开内容：chip 列表。
type PlanCharactersDetail struct {
	Kind       string          `json:"kind"` // "characters"
	Characters []PlanCharacter `json:"characters"`
}

// PlanRule 是“Quy tắc thế giới”步展开内容里的一条规则。
type PlanRule struct {
	Rule     string `json:"rule"`
	Boundary string `json:"boundary,omitempty"`
}

// PlanRulesDetail 是“Quy tắc thế giới”步的展开内容：规则列表。
type PlanRulesDetail struct {
	Kind  string     `json:"kind"` // "worldRules"
	Rules []PlanRule `json:"rules"`
}

// PlanChapter 是“Dàn ý”步网格里已落盘的一章：标题 + móc（hook）。
type PlanChapter struct {
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	Hook    string `json:"hook,omitempty"`
}

// PlanOutlineDetail 是“Dàn ý”步的展开内容：Saved 已细化章数，
// Total 为含骨架估算的总规模，界面为未细化章节画等待条。
type PlanOutlineDetail struct {
	Kind     string        `json:"kind"` // "outline"
	Saved    int           `json:"saved"`
	Total    int           `json:"total"`
	Chapters []PlanChapter `json:"chapters"`
}

// OutlineSnapshot 是大纲条目的展示摘要。
type OutlineSnapshot struct {
	Chapter   int    `json:"chapter"`
	Title     string `json:"title"`
	CoreEvent string `json:"coreEvent"`
}

// AgentSnapshot 是 Agent 状态的展示投影。
type AgentSnapshot struct {
	Name      string               `json:"name"`
	State     string               `json:"state"`
	TaskID    string               `json:"taskId"`
	TaskKind  string               `json:"taskKind"`
	Summary   string               `json:"summary"`
	Tool      string               `json:"tool"`
	Turn      int                  `json:"turn"`
	Context   AgentContextSnapshot `json:"context"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// AgentCacheStat 是单个 agent 的缓存命中累计（投影到左栏）。
// HitRate = CacheRead / Input；Input 在 litellm 层已统一为"含 CacheRead"语义。
//
// CacheCapable 用来区分两种 0% 命中：
//   - true  → 模型支持 prompt cache，0% 是 prompt 设计差或前缀不稳定，需要优化
//   - false → 模型/provider 不支持 prompt cache，0% 是预期，不必排查
//
// Recent* 是滑动窗（最近 N 次调用）的命中数据，对比累计可识别"前期拖累"vs"稳态低命中"。
type AgentCacheStat struct {
	Role            string  `json:"role"`
	Model           string  `json:"model"`
	Input           int     `json:"input"`
	Output          int     `json:"output"`
	CacheRead       int     `json:"cacheRead"`
	CacheWrite      int     `json:"cacheWrite"`
	Cost            float64 `json:"cost"`
	Saved           float64 `json:"saved"`
	CacheCapable    bool    `json:"cacheCapable"`
	RecentCacheRead int     `json:"recentCacheRead"`
	RecentInput     int     `json:"recentInput"`
	RecentSamples   int     `json:"recentSamples"`
}

// AgentContextSnapshot 是 Agent 上下文使用情况。
type AgentContextSnapshot struct {
	Tokens          int     `json:"tokens"`
	ContextWindow   int     `json:"contextWindow"`
	Percent         float64 `json:"percent"`
	Scope           string  `json:"scope"`
	Strategy        string  `json:"strategy"`
	ActiveMessages  int     `json:"activeMessages"`
	SummaryMessages int     `json:"summaryMessages"`
	CompactedCount  int     `json:"compactedCount"`
	KeptCount       int     `json:"keptCount"`
}

// CoCreateMessage 是共创对话的消息。
type CoCreateMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CoCreateReply 是共创对话的 LLM 回复。Raw 保留模型完整四段原文，
// 用于写回 history 让下一轮模型看到自己上一轮的 [DRAFT]，从而真正在
// 已有草稿上累积更新（仅 Message 不含 [DRAFT]，会导致模型每轮凭对话重新归纳）。
// Suggestions 是 AI 主动给的"接下来你可能想说"，用户卡壳时按数字键一键填入输入框。
type CoCreateReply struct {
	Message     string
	Prompt      string
	Ready       bool
	Suggestions []string
	Proposal    *CoCreateProposal
	Raw         string
}
