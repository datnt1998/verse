package arbiter

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// InterventionFacts 干预分诊的事实包(Collect 时刻快照)。
// Engine 在边界执行 Dispatch 前用 Phase/QueueHead 做对账(咨询与执行之间隔着
// worker 运行,事实可能已推进;不符 → 丢弃并以新事实重询)。
type InterventionFacts struct {
	Phase                    string   `json:"phase,omitempty"`
	Flow                     string   `json:"flow,omitempty"`
	Title                    string   `json:"title,omitempty"`
	CompletedChapters        int      `json:"completed_chapters"`
	OutlinedChapters         int      `json:"outlined_chapters,omitempty"`
	DynamicPlanning          bool     `json:"dynamic_planning"`
	NextChapter              int      `json:"next_chapter,omitempty"`
	PendingRewrites          []int    `json:"pending_rewrites,omitempty"`
	ReopenCount              int      `json:"reopen_count,omitempty"` // 用户显式 /reopen 重开完结书的累计次数
	FoundationMissing        []string `json:"foundation_missing,omitempty"`
	PlanningTier             string   `json:"planning_tier,omitempty"`
	AdvanceMode              string   `json:"advance_mode,omitempty"`
	HasAdvanceHold           bool     `json:"has_advance_hold"`
	AdvanceHoldAfter         string   `json:"advance_hold_after,omitempty"`
	AdvanceHoldTargetChapter int      `json:"advance_hold_target_chapter,omitempty"`
	AdvanceHoldReason        string   `json:"advance_hold_reason,omitempty"`
	Running                  bool     `json:"running"` // 干预到达时是否有 run 在进行
	// PendingAsk 是引擎停下后仍在等待用户回答的仲裁提问；新一轮干预默认就是对它的回答。
	PendingAsk *domain.PendingAsk `json:"pending_ask,omitempty"`
	// AskUserEnabled 对应 cấu hình ask_user：tắt thì DecideIntervention tự chọn
	// phương án is_default thay vì đăng ký câu hỏi chờ (xem autoPickReply).
	AskUserEnabled bool `json:"ask_user_enabled"`
	// WorkerQuestion requires a structured ask before any action on a worker's
	// unresolved handoff. Disabled asking still goes through the auto-pick loop.
	WorkerQuestion  bool             `json:"worker_question,omitempty"`
	CheckpointSeq   int64            `json:"checkpoint_seq,omitempty"` // Collect 时刻最新 checkpoint;Engine 对账用
	RecentDecisions []RecentDecision `json:"recent_decisions,omitempty"`
	// NoActionStreak 只统计连续未提出操作的裁定，不代表操作执行结果，
	// 也不用于截取授权对话。失败记录不参与计数。
	NoActionStreak    int    `json:"no_action_streak,omitempty"`
	DiagnosticContext string `json:"diagnostic_context,omitempty"`
}

// RecentDecision 保留最近干预的原文（旧→新），不截掉末尾的范围/禁令。
// PlannedAction 只是审计里的操作意图，不能用作已执行或已完成的证据。
type RecentDecision struct {
	At             string `json:"at"`
	Input          string `json:"input"`
	InputTruncated bool   `json:"input_truncated,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Answer         string `json:"answer,omitempty"`
	PlannedAction  string `json:"planned_action,omitempty"`
}

// QueueHead 返回重写队列头(无则 0),Engine 对账用。
func (f InterventionFacts) QueueHead() int {
	if len(f.PendingRewrites) > 0 {
		return f.PendingRewrites[0]
	}
	return 0
}

// CollectInterventionFacts 从 store 读齐分诊事实。任何控制事实读取失败都显式
// 返回错误，禁止 Arbiter 在零值拼成的不完整快照上做语义决策。
func CollectInterventionFacts(st *storepkg.Store) (InterventionFacts, error) {
	var f InterventionFacts
	if st == nil {
		return f, fmt.Errorf("store không được để trống")
	}
	missing, err := st.FoundationMissing()
	if err != nil {
		return f, fmt.Errorf("đọc trạng thái thiết lập nền: %w", err)
	}
	f.FoundationMissing = missing
	book, err := st.Book.Load()
	if err != nil {
		return f, fmt.Errorf("đọc thông tin tác phẩm: %w", err)
	}
	if book != nil {
		f.Title = book.Title
	}
	p, err := st.Progress.Load()
	if err != nil {
		return f, fmt.Errorf("đọc tiến độ: %w", err)
	}
	if p != nil {
		f.Phase = string(p.Phase)
		f.Flow = string(p.Flow)
		f.CompletedChapters = len(p.CompletedChapters)
		f.DynamicPlanning = p.Layered
		if p.Layered {
			outline, outlineErr := st.Outline.LoadOutline()
			if outlineErr != nil {
				return f, fmt.Errorf("đọc đại cương chi tiết hiện tại: %w", outlineErr)
			}
			f.OutlinedChapters = len(outline)
		} else {
			f.OutlinedChapters = p.TotalChapters
		}
		f.NextChapter = p.NextChapter()
		f.PendingRewrites = append([]int(nil), p.PendingRewrites...)
		f.ReopenCount = p.ReopenCount
	}
	meta, err := st.RunMeta.Load()
	if err != nil {
		return f, fmt.Errorf("đọc siêu dữ liệu chạy: %w", err)
	}
	if meta != nil {
		f.PlanningTier = string(meta.PlanningTier)
		f.AdvanceMode = string(meta.AdvanceMode)
		if meta.AdvanceHold != nil {
			f.HasAdvanceHold = true
			f.AdvanceHoldAfter = string(meta.AdvanceHold.After)
			f.AdvanceHoldTargetChapter = meta.AdvanceHold.TargetChapter
			f.AdvanceHoldReason = meta.AdvanceHold.Reason
		}
		if meta.PendingAsk != nil {
			ask := *meta.PendingAsk
			f.PendingAsk = &ask
		}
	}
	if cp := st.Checkpoints.LatestGlobal(); cp != nil {
		f.CheckpointSeq = cp.Seq
	}
	// 多读几条:失败裁定、其它 kind 会混在中间,过滤后仍要凑满最近 5 次干预。
	recent, err := st.Decisions.Recent(recentDecisionScan)
	if err != nil {
		return f, fmt.Errorf("đọc các phán định gần đây: %w", err)
	}
	var memory []RecentDecision
	for _, r := range recent {
		if r.Kind != "intervention" {
			continue
		}
		var d InterventionDecision
		if r.Error == "" && len(r.Decision) > 0 {
			if err := json.Unmarshal(r.Decision, &d); err != nil {
				return f, fmt.Errorf("đọc nội dung phán định gần đây: %w", err)
			}
		}
		action := d.actionSummary()
		if r.Error == "" {
			// 失败裁定什么都没对用户说,不算"只说不做",也不打断计数。
			if action == "" {
				f.NoActionStreak++
			} else {
				f.NoActionStreak = 0
			}
		}
		memory = append(memory, RecentDecision{
			At: r.At, Input: r.Input, InputTruncated: r.InputTruncated, Reason: r.Reason,
			Answer: d.Answer, PlannedAction: action,
		})
	}
	if len(memory) > recentDecisionKeep {
		memory = memory[len(memory)-recentDecisionKeep:]
	}
	f.RecentDecisions = memory
	f.DiagnosticContext, err = collectDiagnosticContext(st, f.PlanningTier, f.RecentDecisions)
	if err != nil {
		return f, err
	}
	return f, nil
}

const (
	recentDecisionScan = 12
	recentDecisionKeep = 5
)

// actionSummary 把裁定计划的操作压成一行；执行结果必须从其它记录核实。
func (d InterventionDecision) actionSummary() string {
	var parts []string
	if d.Dispatch != nil {
		parts = append(parts, fmt.Sprintf("giao %s: %s", d.Dispatch.Agent, truncateRunes(d.Dispatch.Task, 200)))
	}
	if d.Rules != "" {
		parts = append(parts, "lưu luật viết: "+truncateRunes(d.Rules, 120))
	}
	if d.Hold != nil {
		if d.Hold.Cancel {
			parts = append(parts, "huỷ tạm dừng")
		} else {
			parts = append(parts, "đặt tạm dừng")
		}
	}
	if d.Reopen != nil {
		parts = append(parts, fmt.Sprintf("mở lại chương %v", d.Reopen.Chapters))
	}
	if d.Ask != nil {
		parts = append(parts, "hỏi người dùng: "+truncateRunes(d.Ask.Question, 160))
	}
	return strings.Join(parts, "; ")
}

// AdvanceHoldOp 一次性暂停动作：在工作边界、返工排空或目标章节完成后暂停，也可取消。
type AdvanceHoldOp struct {
	Cancel        bool                    `json:"cancel,omitempty"`
	After         domain.AdvanceHoldAfter `json:"after,omitempty"`
	TargetChapter int                     `json:"target_chapter,omitempty"`
	Reason        string                  `json:"reason,omitempty"`
}

// ReopenOp 完本返工:把全书重开进返工态并把目标章入队(仅 phase=complete 合法)。
type ReopenOp struct {
	Chapters []int  `json:"chapters"`
	Reason   string `json:"reason,omitempty"`
}

// AskOptionOp 是仲裁给用户的候选项；reply 为空时界面以 label 作答。
type AskOptionOp struct {
	Label     string `json:"label"`
	Reply     string `json:"reply,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// AskOp 是“停下，等用户选/答”动作：引擎在下一工作边界暂停并记住提问；
// 用户下一次干预（点选项或自由输入）走同一条裁定通道把它结清。
type AskOp struct {
	Question string        `json:"question"`
	Options  []AskOptionOp `json:"options,omitempty"`
	Chapter  int           `json:"chapter,omitempty"`
}

// InterventionDecision 干预裁定。动作组合自由,执行顺序由 Engine 固定:
// answer → rules → hold → reopen → ask → dispatch;至多一个 dispatch(类型事实)。
type InterventionDecision struct {
	Answer   string         `json:"answer,omitempty"`
	Rules    string         `json:"rules,omitempty"`
	Hold     *AdvanceHoldOp `json:"hold,omitempty"`
	Reopen   *ReopenOp      `json:"reopen,omitempty"`
	Dispatch *DispatchOp    `json:"dispatch,omitempty"`
	Ask      *AskOp         `json:"ask,omitempty"`
	Reason   string         `json:"reason"`
}

var interventionContract = llmcontract.Contract{
	Name:        "arbiter_intervention",
	Description: "Phán định can thiệp người dùng: trả lời, luật, tạm dừng, mở lại và điều phối",
	Schema: schema.Object(
		schema.Property("answer", llmcontract.Nullable(schema.String("Văn bản hiển thị lại cho người dùng; không có thì null"))).Required(),
		schema.Property("rules", llmcontract.Nullable(schema.String("Nguyên văn luật viết dài hạn cần lưu; không có thì null"))).Required(),
		schema.Property("hold", llmcontract.Nullable(schema.Object(
			schema.Property("cancel", schema.Bool("Có huỷ tạm dừng một lần hiện có không")).Required(),
			schema.Property("after", llmcontract.Nullable(schema.Enum("Điểm kích hoạt tạm dừng; khi huỷ thì null", string(domain.AdvanceHoldAtBoundary), string(domain.AdvanceHoldAfterRewritesDrained), string(domain.AdvanceHoldAtChapter)))).Required(),
			schema.Property("target_chapter", llmcontract.Nullable(schema.Int("Chương đích khi after=chapter; trường hợp khác là null"))).Required(),
			schema.Property("reason", llmcontract.Nullable(schema.String("Tóm tắt yêu cầu của người dùng; khi huỷ có thể null"))).Required(),
		))).Required(),
		schema.Property("reopen", llmcontract.Nullable(schema.Object(
			schema.Property("chapters", schema.Array("Số chương cần mở lại", schema.Int("Số chương"))).Required(),
			schema.Property("reason", llmcontract.Nullable(schema.String("Lý do mở lại"))).Required(),
		))).Required(),
		schema.Property("ask", llmcontract.Nullable(schema.Object(
			schema.Property("question", schema.String("Câu hỏi đặt ra cho người dùng (tiếng Việt, 1–2 câu)")).Required(),
			schema.Property("options", llmcontract.Nullable(schema.Array("Phương án gợi ý; tối đa một mặc định", schema.Object(
				schema.Property("label", schema.String("Nhãn ngắn trên nút")).Required(),
				schema.Property("reply", llmcontract.Nullable(schema.String("Văn bản đầy đủ gửi lại; null = dùng label"))).Required(),
				schema.Property("is_default", schema.Bool("Phương án Trọng tài khuyên dùng")).Required(),
			)))).Required(),
			schema.Property("chapter", llmcontract.Nullable(schema.Int("Chương liên quan; không có thì null"))).Required(),
		))).Required(),
		schema.Property("dispatch", dispatchSchema("Mục tiêu điều phối; không cần điều phối thì null")).Required(),
		schema.Property("reason", schema.String("Lý do phán định trong một câu")).Required(),
	),
}

// ValidateAgainst 按事实做机械校验(场景内合法性;类型已排除跨场景动作)。
func (d *InterventionDecision) ValidateAgainst(f InterventionFacts) error {
	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("reason không được để trống")
	}
	if d.Answer == "" && d.Rules == "" && d.Hold == nil && d.Reopen == nil && d.Ask == nil && d.Dispatch == nil {
		return fmt.Errorf("quyết định rỗng: phải có ít nhất một hành động hoặc answer")
	}
	if f.WorkerQuestion && d.Ask == nil {
		return fmt.Errorf("phân vân của trợ lý cần ask có lựa chọn, không được trả lời bằng câu hỏi tự do hoặc tự thực hiện trước khi chọn")
	}
	complete := f.Phase == string(domain.PhaseComplete)
	if d.Ask != nil {
		// Lưu ý: ask_user tắt KHÔNG làm ask bất hợp lệ ở đây — DecideIntervention
		// tự chọn phương án mặc định rồi phán định lại (27/10), tránh vòng hỏi lại
		// khi model xuất ask ngoài ý muốn.
		ask := domain.PendingAsk{Question: d.Ask.Question, Chapter: d.Ask.Chapter}
		for _, o := range d.Ask.Options {
			ask.Options = append(ask.Options, domain.PendingAskOption{Label: o.Label, Reply: o.Reply, IsDefault: o.IsDefault})
		}
		if err := ask.Validate(); err != nil {
			return fmt.Errorf("ask không hợp lệ: %w", err)
		}
		if len(d.Ask.Options) < 2 {
			return fmt.Errorf("ask cần ít nhất hai phương án")
		}
		if d.Hold != nil || d.Reopen != nil || d.Dispatch != nil {
			return fmt.Errorf("ask không kết hợp được với hold/reopen/dispatch — dừng hỏi người dùng chỉ đứng một mình")
		}
		if complete {
			return fmt.Errorf("giai đoạn đã kết thúc cấm ask; viết lại dùng reopen")
		}
		if f.PendingAsk != nil && f.PendingAsk.Question != "" && f.PendingAsk.Question != d.Ask.Question {
			return fmt.Errorf("đã có câu hỏi đang chờ (%s), trả lời nó trước", f.PendingAsk.Question)
		}
	}
	if err := d.Dispatch.validate(); err != nil {
		return err
	}
	if err := validateDispatchAgainst(d.Dispatch, f.Phase); err != nil {
		return err
	}
	if d.Reopen != nil {
		if !complete {
			return fmt.Errorf("reopen chỉ dành cho giai đoạn đã kết thúc (phase hiện tại=%s)", f.Phase)
		}
		if len(d.Reopen.Chapters) == 0 {
			return fmt.Errorf("reopen.chapters không được để trống")
		}
		for _, ch := range d.Reopen.Chapters {
			if ch < 1 || ch > f.CompletedChapters {
				return fmt.Errorf("reopen chương %d vượt giới hạn (đã hoàn thành %d chương)", ch, f.CompletedChapters)
			}
		}
	}
	if complete && d.Dispatch != nil {
		return fmt.Errorf("giai đoạn đã kết thúc cấm điều phối trực tiếp; viết lại dùng reopen (vào hàng đợi rồi Router tự điều phối)")
	}
	if d.Hold != nil && !d.Hold.Cancel {
		if f.Phase != string(domain.PhaseWriting) {
			return fmt.Errorf("tạm dừng một lần chỉ dành cho giai đoạn viết (phase hiện tại=%s)", f.Phase)
		}
		hold := domain.AdvanceHold{After: d.Hold.After, TargetChapter: d.Hold.TargetChapter, Reason: d.Hold.Reason}
		if err := hold.Validate(); err != nil {
			return fmt.Errorf("hold không hợp lệ: %w", err)
		}
		nextChapter := f.NextChapter
		if nextChapter == 0 {
			nextChapter = f.CompletedChapters + 1
		}
		if hold.After == domain.AdvanceHoldAtChapter && hold.TargetChapter < nextChapter {
			return fmt.Errorf("chương đích %d sớm hơn chương kế tiếp hiện tại %d", hold.TargetChapter, nextChapter)
		}
	}
	return nil
}

// validateDispatchAgainst 把提示词中的阶段纪律落实为机械防线。Architect 可在规划期
// 与写作期维护结构；Writer/Editor 只能消费已经完整且进入 writing 的作品事实。
func validateDispatchAgainst(dispatch *DispatchOp, phase string) error {
	if dispatch == nil {
		return nil
	}
	if phase == "" {
		return fmt.Errorf("thiếu phase, cấm thực hiện điều phối")
	}
	if phase == string(domain.PhaseComplete) {
		return fmt.Errorf("giai đoạn đã kết thúc cấm điều phối trực tiếp")
	}
	switch dispatch.Agent {
	case "writer", "editor":
		if phase != string(domain.PhaseWriting) {
			return fmt.Errorf("%s chỉ được điều phối ở giai đoạn writing (phase hiện tại=%s)", dispatch.Agent, phase)
		}
	}
	return nil
}

// AskAutoPick là vết của một lần máy chủ tự trả lời ask khi ask_user tắt:
// ghi vào nhật ký hoạt động (dòng "Trọng tài tự chọn: … (vì bạn đã tắt Hỏi khi phân vân)").
type AskAutoPick struct {
	Question string `json:"question"`
	Choice   string `json:"choice"`
}

// maxAskAutoPicks 封顶自答次数：模型连续两轮仍硬发 ask 即放弃并回显失败，
// 绝不把 ask 静默吞成空操作。
const maxAskAutoPicks = 2

// autoPickReply 复刻界面回退逻辑（IdleCard）：优先 is_default 项，无默认取第一项；
// reply 空则用 label 当整句回答。
func autoPickReply(ask *AskOp) string {
	if len(ask.Options) == 0 {
		return ""
	}
	pick := ask.Options[0]
	for _, o := range ask.Options {
		if o.IsDefault {
			pick = o
			break
		}
	}
	if reply := strings.TrimSpace(pick.Reply); reply != "" {
		return reply
	}
	return strings.TrimSpace(pick.Label)
}

// DecideIntervention 干预分诊。失败语义:返回 error → 调用方显式回显
// 真实失败原因,且不产生任何写入(宁可不动,不可误动)。
//
// ask_user tắt（27/10）: model 仍硬发 ask 时服务器 tự chọn 而不问人——按界面
// 同款回退挑出 phương án（is_default，否则第一项），把它的 reply 当作下一句
// 干预原样重过分诊（与用户在 “Đang chờ bạn” 卡上点击同一通道），最多
// maxAskAutoPicks 次。autoPicks 记录每次自答供调用方写 nhật ký。
func DecideIntervention(ctx context.Context, model llmtypes.ChatModel, systemPrompt string, facts InterventionFacts, text string) (decision InterventionDecision, autoPicks []AskAutoPick, err error) {
	intervention := text
	// ask 单独成立（ValidateAgainst 禁与 hold/reopen/dispatch 共存）但可带
	// answer/rules；它们合并进最终裁定，语义等价于“模型先答完再问，人再答”。
	var carriedAnswer, carriedRules string
	for {
		payload, err := marshalPayload(struct {
			Intervention string            `json:"intervention"`
			Facts        InterventionFacts `json:"facts"`
		}{Intervention: intervention, Facts: facts})
		if err != nil {
			return InterventionDecision{}, nil, err
		}
		d, err := decide(ctx, model, interventionContract, systemPrompt, payload, func(d *InterventionDecision) error {
			return d.ValidateAgainst(facts)
		})
		if err != nil {
			return InterventionDecision{}, nil, err
		}
		if facts.AskUserEnabled || d.Ask == nil {
			if d.Answer == "" {
				d.Answer = carriedAnswer
			}
			if d.Rules == "" {
				d.Rules = carriedRules
			}
			return d, autoPicks, nil
		}
		reply := autoPickReply(d.Ask)
		if reply == "" || len(autoPicks) >= maxAskAutoPicks {
			return InterventionDecision{}, autoPicks, fmt.Errorf("Trọng tài vẫn đòi hỏi người dùng dù Hỏi khi phân vân đang tắt; can thiệp chưa được áp dụng")
		}
		autoPicks = append(autoPicks, AskAutoPick{Question: d.Ask.Question, Choice: reply})
		if d.Answer != "" {
			carriedAnswer = d.Answer
		}
		if d.Rules != "" {
			carriedRules = d.Rules
		}
		intervention = reply
		facts.WorkerQuestion = false // the automatic reply resolves the initial handoff
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
