package diag

import (
	"sort"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// ── 诊断阈值 ─────────────────────────────────────────────

const (
	ThresholdDimScoreLow      = 70  // ChronicLowDimension: 维度均分低于此值告警
	ThresholdContractMissRate = 0.3 // ContractMissPattern: 合同未达成率上限
	ThresholdRewriteRate      = 0.5 // ExcessiveRewrites: 改写率上限
	ThresholdWordShortRatio   = 0.4 // WordCountAnomaly: 字数低于均值此比例视为异常
	ThresholdWordLongRatio    = 2.5 // WordCountAnomaly: 字数高于均值此比例视为异常
	ThresholdHookWeakScore    = 75  // HookWeakChain: hook 低于此分视为偏弱
	ThresholdHookWeakChain    = 3   // HookWeakChain: 连续偏弱章数阈值
	ThresholdPayoffMissRate   = 0.4 // PayoffMissPattern: payoff 未兑现率上限
	ThresholdCompassDrift     = 15  // CompassDrift: 指南针未更新章数上限
	ThresholdTimelineGapRate  = 0.3 // TimelineGaps: 缺失率容忍上限
)

// allRules 按 flow → quality → planning → context 排列。
var allRules = []RuleFunc{
	// Flow
	InvalidPendingRewrites,
	RewritePendingPressure,
	OrphanedSteer,
	PhaseFlowMismatch,
	ChapterGaps,
	// Quality
	ChronicLowDimension,
	ContractMissPattern,
	HookWeakChain,
	PayoffMissPattern,
	ExcessiveRewrites,
	WordCountAnomaly,
	// Planning
	StaleForeshadow,
	CompassDrift,
	OutlineExhausted,
	MissingSummaries,
	HookTypeChain,
	MidpointTwistPosition,
	PromiseScenePosition,
	// Context
	GhostCharacter,
	TimelineGaps,
	RelationshipStagnation,
}

// Analyze 是诊断系统的唯一入口。
func Analyze(s *store.Store) Report {
	snap := Load(s)

	var findings []Finding
	for _, e := range snap.LoadErrors {
		findings = append(findings, Finding{
			Rule:       "LoadError",
			Category:   CatFlow,
			Severity:   SevWarning,
			Confidence: ConfHigh,
			AutoLevel:  AutoNone,
			Target:     "runtime.flow",
			Title:      "Không đọc được một phần dữ liệu truyện",
			Evidence:   "Một phần dữ liệu không đọc được nên kết quả chẩn đoán có thể chưa đầy đủ.",
			Suggestion: "Thử phân tích lại. Nếu vẫn gặp lỗi, hãy xuất chẩn đoán ẩn danh để gửi hỗ trợ; không sửa hoặc xoá dữ liệu truyện khi chưa có bản sao lưu.",
			Technical:  e,
		})
	}
	for _, rule := range allRules {
		findings = append(findings, rule(&snap)...)
	}
	sortFindings(findings)

	return Report{
		Stats:    buildStats(&snap),
		Findings: findings,
		Actions:  PlanActions(findings),
	}
}

func buildStats(snap *Snapshot) Stats {
	st := Stats{}
	if snap.Progress == nil {
		return st
	}
	p := snap.Progress
	st.CompletedChapters = len(p.CompletedChapters)
	st.TotalChapters = p.TotalChapters
	st.TotalWords = p.TotalWordCount
	st.Phase = string(p.Phase)
	st.Flow = string(p.Flow)

	if st.CompletedChapters > 0 {
		st.AvgWordsPerCh = st.TotalWords / st.CompletedChapters
	}

	if snap.RunMeta != nil {
		st.PlanningTier = string(snap.RunMeta.PlanningTier)
	}

	// 评审统计
	st.ReviewCount = len(snap.Reviews)
	var totalScore float64
	var dimCount int
	for _, r := range snap.Reviews {
		if r.Verdict == "rewrite" {
			st.RewriteCount++
		}
		for _, d := range r.Dimensions {
			totalScore += float64(d.Score)
			dimCount++
		}
	}
	if dimCount > 0 {
		st.AvgReviewScore = totalScore / float64(dimCount)
	}

	// 伏笔统计：已收束（resolved / abandoned）不算"đang mở"；睡太久的按
	// ThreadHealth 判定，和上下文召回共用同一条口径。
	latest := snap.LatestCompleted()
	completed := snap.CompletedCount()
	for _, f := range snap.Foreshadow {
		if !domain.ForeshadowOpen(f.Status) {
			continue
		}
		st.ForeshadowOpen++
		if _, dormant := domain.ThreadHealth(f, latest, completed); dormant {
			st.ForeshadowStale++
		}
	}
	return st
}

// sortFindings 按严重程度排序：critical > warning > info。
func sortFindings(findings []Finding) {
	order := map[Severity]int{SevCritical: 0, SevWarning: 1, SevInfo: 2}
	sort.SliceStable(findings, func(i, j int) bool {
		return order[findings[i].Severity] < order[findings[j].Severity]
	})
}
