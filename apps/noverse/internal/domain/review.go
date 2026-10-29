package domain

// TimelineEvent 时间线事件。
type TimelineEvent struct {
	Chapter    int      `json:"chapter"`
	Time       string   `json:"time"`
	Event      string   `json:"event"`
	Characters []string `json:"characters,omitempty"`
	// Quote là trích nguyên văn câu/đoạn chứng minh sự kiện (tùy chọn); khi có
	// phải định vị được trong chính văn chương để làm bằng chứng truy hồi.
	Quote string `json:"quote,omitempty"`
}

// ForeshadowEntry 伏笔条目。
type ForeshadowEntry struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	PlantedAt   int    `json:"planted_at"`
	Status      string `json:"status"` // planted / advanced / resolved / abandoned
	ResolvedAt  int    `json:"resolved_at,omitempty"`
	// DueBy là chương dự kiến thu (tùy chọn). Chỉ là dự định của tác giả, không
	// phải hạn chót: quá mốc thì phục bút thành "quá hạn" để context/diag nhắc.
	DueBy int `json:"due_by,omitempty"`
	// LastTouched là chương gần nhất có plant hoặc advance. Tuổi "ngủ quên" đo
	// từ đây chứ không từ PlantedAt, để phục bút vừa được đẩy không bị coi là bỏ quên.
	// Sổ cái cũ chưa có trường này: đọc lên bằng 0, health lùi về PlantedAt.
	LastTouched int `json:"last_touched,omitempty"`
	// AbandonReason / AbandonedAt ghi lại việc tác giả chủ động bỏ tuyến.
	AbandonReason string `json:"abandon_reason,omitempty"`
	AbandonedAt   int    `json:"abandoned_at,omitempty"`
	// Evidence chứa trích nguyên văn chứng minh mỗi thao tác: mỗi update kèm
	// quote khi commit/sync thêm một EvidenceRef, chỉ tăng không sửa xoá.
	Evidence []EvidenceRef `json:"evidence,omitempty"`
}

// EvidenceRef 指向某章正文中支持一次伏笔操作的原文引文。刻意只存 quote +
// 章号不存偏移：正文可能被修订，偏移会漂移，quote 每次重定位，失效即丢弃。
type EvidenceRef struct {
	Chapter int    `json:"chapter"`
	Action  string `json:"action"` // plant / advance / resolve / abandon
	Quote   string `json:"quote"`
}

// AppendForeshadowEvidence 按严格相等去重追加证据，让伏笔账本重放（提交 Saga
// 崩溃重放、章节记录全量重放）产生同一账本。
func AppendForeshadowEvidence(e *ForeshadowEntry, ref EvidenceRef) {
	for _, existing := range e.Evidence {
		if existing == ref {
			return
		}
	}
	e.Evidence = append(e.Evidence, ref)
}

// ForeshadowUpdate 伏笔增量操作。
type ForeshadowUpdate struct {
	ID          string `json:"id"`
	Action      string `json:"action"` // plant / advance / resolve / abandon
	Description string `json:"description,omitempty"`
	Quote       string `json:"quote,omitempty"`  // 证明本次操作的原文引文，可选
	DueBy       int    `json:"due_by,omitempty"` // plant 时可选：预计回收章，必须晚于本章
	Reason      string `json:"reason,omitempty"` // abandon 时必填：为什么放弃这条线
}

// RestoreOwnPlants 把旧记录里本章种下、而新记录未再声明的伏笔 plant 补回队首。
// 一章埋过哪些伏笔是它自身的历史事实，重写正文不改变这一点；丢掉它，章节记录
// 全量重放时本章及后续章节的 advance/resolve 会找不到前置 plant，整条链报错。
func RestoreOwnPlants(prev, next []ForeshadowUpdate) []ForeshadowUpdate {
	declared := make(map[string]struct{}, len(next))
	for _, u := range next {
		if u.Action == "plant" {
			declared[u.ID] = struct{}{}
		}
	}
	var restored []ForeshadowUpdate
	for _, u := range prev {
		if u.Action != "plant" {
			continue
		}
		if _, ok := declared[u.ID]; ok {
			continue
		}
		declared[u.ID] = struct{}{}
		restored = append(restored, u)
	}
	if len(restored) == 0 {
		return next
	}
	// plant 必须排在同章 advance/resolve 之前，重放才能先建起条目。
	return append(restored, next...)
}

// RelationshipEntry 人物关系条目。
type RelationshipEntry struct {
	CharacterA string `json:"character_a"`
	CharacterB string `json:"character_b"`
	Relation   string `json:"relation"`
	// Type là kiểu quan hệ cho Não truyện (enemy/ally/work/own/secret/rel);
	// dữ kiện cũ hoặc model không phân loại được để trống → đọc là "rel".
	Type    string `json:"type,omitempty"`
	Chapter int    `json:"chapter"`
}

// 问题严重度三档。SeverityError 与 SeverityCritical 是「必改」档（Bắt sửa）：
// 它们本身就把章节推进返工队列，见 RequiresChangeFor。
const (
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// RequiresChangeFor 是「error 档问题 ⇒ 必改」这条规则的唯一定义：error/critical
// 一律要求返工，即使 Editor 把 requires_change 留成 false；warning 只在 Editor
// 明确要求时才算。requested=false 用来单独问「这个严重度本身是否构成必改」，
// 供 gói 的 LỖI 标签校验使用。
func RequiresChangeFor(severity string, requested bool) bool {
	switch severity {
	case SeverityError, SeverityCritical:
		return true
	default:
		return requested
	}
}

// ConsistencyIssue 一致性问题。
type ConsistencyIssue struct {
	Type        string `json:"type"`     // 模型依据 rubric 给出的具体问题维度
	Severity    string `json:"severity"` // critical / error / warning
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"` // 证据：原文片段、具体情节或状态数据
	RuleRef     string `json:"rule_ref,omitempty"`
	RuleText    string `json:"rule_text,omitempty"`
	Quote       string `json:"quote,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
	// Rule là mã phụ ổn định của mục (câu 4 §H). Hiện chỉ có s19-recap: tóm tắt
	// lại truyện trước dài quá hai câu hoặc không lồng vào hành động (Master §19).
	Rule           string `json:"rule,omitempty"`
	Chapters       []int  `json:"chapters,omitempty"` // 证据实际落在哪些章节
	RequiresChange bool   `json:"requires_change"`    // 是否应立即进入返工队列；error 档由 RequiresChangeFor 强制为 true
}

// DimensionScore 单维度评审评分。
type DimensionScore struct {
	Dimension string `json:"dimension"`         // 由评审 rubric 定义，可按任务扩展
	Score     int    `json:"score"`             // 0-100
	Verdict   string `json:"verdict,omitempty"` // 兼容旧审阅；运行时不再用阈值覆盖模型判断
	Comment   string `json:"comment,omitempty"` // 该维度的简要结论
}

// ReviewEntry Editor 的审阅条目。
type ReviewEntry struct {
	Chapter          int                `json:"chapter"`
	Scope            string             `json:"scope"` // chapter / global / arc
	Issues           []ConsistencyIssue `json:"issues"`
	Dimensions       []DimensionScore   `json:"dimensions,omitempty"`      // 分维度评分
	ContractStatus   string             `json:"contract_status,omitempty"` // met / partial / missed
	ContractMisses   []string           `json:"contract_misses,omitempty"` // 未达成的 contract 条目
	ContractNotes    string             `json:"contract_notes,omitempty"`  // 对 contract 履行情况的简述
	Verdict          string             `json:"verdict"`                   // accept / polish / rewrite
	Summary          string             `json:"summary"`
	AffectedChapters []int              `json:"affected_chapters,omitempty"` // 需要重写/打磨的章节号
	// RuleAware đánh dấu vòng chấm được sinh bởi schema save_review có mã phụ
	// `rule` (câu 4 §H). Nhờ đó "chưa được đánh giá" tách được khỏi "đã đạt".
	RuleAware bool `json:"rule_aware,omitempty"`
	// GoalChecks 冻结 scope=chapter 检查清单的逐项判定。kind/text 由
	// 保存时的 contract 快照回填；派生结果覆盖模型提交的
	// contract_status/contract_misses。
	GoalChecks     []GoalCheck     `json:"goal_checks,omitempty"` // 目标检查清单（rb/fm/cc 各项 pass/fail/unknown）
	Recheck        bool            `json:"recheck,omitempty"`
	RecheckResults []RecheckResult `json:"recheck_results,omitempty"`
}

// RecheckResult records a single targeted judgment against the archived source round.
type RecheckResult struct {
	FindingID string `json:"finding_id"`
	Status    string `json:"status"` // fixed / unfixed / unknown
	Quote     string `json:"quote,omitempty"`
	Note      string `json:"note"`
}

// CriticalCount 返回 critical 级别问题数量。
func (r *ReviewEntry) CriticalCount() int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityCritical {
			n++
		}
	}
	return n
}

// ErrorCount 返回 error 级别问题数量。
func (r *ReviewEntry) ErrorCount() int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			n++
		}
	}
	return n
}

// Dimension 返回指定维度的评分；不存在则返回 nil。
func (r *ReviewEntry) Dimension(name string) *DimensionScore {
	if r == nil {
		return nil
	}
	for i := range r.Dimensions {
		if r.Dimensions[i].Dimension == name {
			return &r.Dimensions[i]
		}
	}
	return nil
}
