package userrules

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// Service 编排用户规则快照的生成与更新：归一化各来源 → 确定性合并 → 落盘。
//
// 调用方共用同一套逻辑：
//   - 开书/刷新：Build / GetOrBuild，由 Host 确定性调用。
//   - 运行中更新：Arbiter 提取 rules 后，Host 调用 AddRuntimeRule（同时记入运行时候选列表）。
//   - 界面"应用"：Apply 从文件 + 启动 prompt + 运行时候选重新合并，下一章起生效。
type Service struct {
	store     *store.Store
	norm      *Normalizer
	rulesOpts rules.LoadOptions
	pack      rules.Candidate

	// mu 串行化会改写快照/运行时列表的操作（Apply、AddRuntimeRule、Keep/Drop）。
	mu sync.Mutex
}

// NewService 构造服务。model 用于归一化（应为能力较强的模型）；model 为 nil 时
// 所有来源降级为 raw preferences（仍可产出快照，机械检查由 system_defaults 兜底）。
func NewService(st *store.Store, model llmtypes.ChatModel, opts rules.LoadOptions, pack ...rules.Candidate) *Service {
	s := &Service{store: st, norm: NewNormalizer(model), rulesOpts: opts}
	if len(pack) > 0 {
		s.pack = pack[0]
	}
	return s
}

// normalizerCacheVersion 随归一化提示词/契约的语义变化而加一，使旧缓存整体失效。
const normalizerCacheVersion = "v2" // v2: kết quả chuẩn hoá có thêm options

// normConcurrency 是同一个规则文件逐条归一化时的并发上限。
const normConcurrency = 4

// cacheKey 以内容哈希作键：空白折叠后相同的文本复用同一份归一化结果。
func cacheKey(text string) string {
	sum := sha256.Sum256([]byte(normalizerCacheVersion + "\x00" + collapseSpace(text)))
	return hex.EncodeToString(sum[:])
}

func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// normBatch 是一次操作（Build/Apply/Effective）内共享的归一化上下文：读缓存快照、
// 收集新结果，结束时 flush 一次落盘。成功的结果才入缓存；降级结果不缓存，下次会重试。
type normBatch struct {
	norm  *Normalizer
	opts  rules.LoadOptions
	sink  normCache
	cache map[string]store.NormCacheEntry

	mu    sync.Mutex
	fresh map[string]store.NormCacheEntry
}

// normCache là nơi lưu bộ nhớ đệm chuẩn hoá: của truyện (store) hoặc của cấp toàn cục/dự án (tệp).
type normCache interface {
	NormCache() map[string]store.NormCacheEntry
	PutNormCache(map[string]store.NormCacheEntry) error
}

func newBatch(norm *Normalizer, opts rules.LoadOptions, sink normCache) *normBatch {
	return &normBatch{norm: norm, opts: opts, sink: sink, cache: sink.NormCache(), fresh: map[string]store.NormCacheEntry{}}
}

func (s *Service) newBatch() *normBatch {
	return newBatch(s.norm, s.rulesOpts, s.store.UserRules)
}

func (b *normBatch) lookup(key string) (store.NormCacheEntry, bool) {
	if e, ok := b.cache[key]; ok {
		return e, true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.fresh[key]
	return e, ok
}

// normalize 归一化一段文本为候选；失败时记录真实错误并降级为 raw preferences。
func (b *normBatch) normalize(ctx context.Context, source, text string) rules.Candidate {
	text = strings.TrimSpace(text)
	if text == "" {
		return rules.Candidate{Source: source}
	}
	key := cacheKey(text)
	if e, ok := b.lookup(key); ok {
		return rules.Candidate{Source: source, Structured: e.Structured, Preferences: e.Preferences, Uncertain: e.Uncertain, UncertainNote: e.UncertainNote, UncertainFlag: e.UncertainFlag, Options: e.Options}
	}
	cand, err := b.norm.Normalize(ctx, source, text)
	if err != nil {
		slog.Warn("Chuẩn hoá luật thất bại, hạ cấp thành sở thích nguyên văn", "module", "rules", "source", source, "err", err)
		return degraded(source, text)
	}
	b.mu.Lock()
	b.fresh[key] = store.NormCacheEntry{Structured: cand.Structured, Preferences: cand.Preferences, Uncertain: cand.Uncertain, UncertainNote: cand.UncertainNote, UncertainFlag: cand.UncertainFlag, Options: cand.Options, At: time.Now()}
	b.mu.Unlock()
	return cand
}

// flush 把本次新增的缓存条目写盘；缓存只是省模型调用，写失败只记日志。
func (b *normBatch) flush() {
	b.mu.Lock()
	fresh := b.fresh
	b.fresh = map[string]store.NormCacheEntry{}
	b.mu.Unlock()
	if err := b.sink.PutNormCache(fresh); err != nil {
		slog.Warn("Ghi bộ nhớ đệm chuẩn hoá luật thất bại", "module", "rules", "err", err)
	}
}

// normalizeLines 逐行归一化，保持行序；最多 normConcurrency 路并发。
func (b *normBatch) normalizeLines(ctx context.Context, source string, lines []string) []rules.Candidate {
	out := make([]rules.Candidate, len(lines))
	sem := make(chan struct{}, normConcurrency)
	var wg sync.WaitGroup
	for i, line := range lines {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = b.normalize(ctx, source, line)
		}()
	}
	wg.Wait()
	return out
}

// fileCandidate 归一化一个规则文件。quy-tac.md 逐行归一化（界面需要每条规则各自的"hiểu là"，
// 快照与卡片因此永远同源），再按文件合并成一个候选；其它 .md 整段归一化。
// items 仅对 quy-tac.md 返回，按行序。
func (b *normBatch) fileCandidate(ctx context.Context, rs rules.RawSource) (rules.Candidate, []RuleItem) {
	if rs.Name != rules.ManagedRulesFile {
		return b.normalize(ctx, rs.Label, rs.Text), nil
	}
	lines := rules.ParseRuleLines(rs.Text)
	parts := b.normalizeLines(ctx, rs.Label, lines)
	items := make([]RuleItem, len(lines))
	for i, line := range lines {
		items[i] = newRuleItem(line, rs.Kind.String(), parts[i])
	}
	return rules.MergeCandidates(rs.Label, parts), items
}

// runtimeCandidates 把运行时规则转为候选。旧快照迁移来的条目（Legacy）按原文重新归一化
// （命中缓存则不调模型）；原文已丢失的条目没有可重建的内容，跳过。
// 返回的 updated 非空表示有条目已被重新归一化，调用方应写回。
func (b *normBatch) runtimeCandidates(ctx context.Context, list []store.RuntimeRule) (cands []rules.Candidate, updated []store.RuntimeRule) {
	changed := false
	updated = make([]store.RuntimeRule, len(list))
	copy(updated, list)
	for i, r := range updated {
		cand := r.Candidate
		if r.Legacy {
			if strings.TrimSpace(r.Candidate.Preferences) == "" {
				continue
			}
			cand = b.normalize(ctx, store.RuntimeRuleSource, r.Text)
			if !cand.Degraded {
				updated[i].Candidate, updated[i].Legacy = cand, false
				changed = true
			}
		}
		cand.Source = store.RuntimeRuleSource
		cands = append(cands, cand)
	}
	if !changed {
		updated = nil
	}
	return cands, updated
}

// staticCandidates 按优先级（低→高）列出全部来源候选：system_defaults → 规则文件
// （全局→项目→故事）→ 启动 prompt → 运行时候选。items 是 quy-tac.md 逐条的理解。
func (b *normBatch) staticCandidates(ctx context.Context, startupPrompt string, runtime []store.RuntimeRule) (cands []rules.Candidate, items []RuleItem, updatedRuntime []store.RuntimeRule) {
	cands = []rules.Candidate{rules.SystemDefaults()}
	for _, rs := range rules.RawFileSources(b.opts) {
		cand, fileItems := b.fileCandidate(ctx, rs)
		cands = append(cands, cand)
		items = append(items, fileItems...)
	}
	if strings.TrimSpace(startupPrompt) != "" {
		cands = append(cands, b.normalize(ctx, "startup_prompt", startupPrompt))
	}
	runtimeCands, updatedRuntime := b.runtimeCandidates(ctx, runtime)
	return append(cands, runtimeCands...), items, updatedRuntime
}

// Build 从全部来源（system_defaults + rules 文件 + 启动 prompt + 运行时候选）归一化生成快照并落盘。
// 开书/刷新时调用。startupPrompt 可空。
func (s *Service) Build(ctx context.Context, startupPrompt string) (*rules.Snapshot, error) {
	runtime, err := s.store.UserRules.Runtime()
	if err != nil {
		return nil, err
	}
	b := s.newBatch()
	cands, _, _ := b.staticCandidates(ctx, startupPrompt, runtime)
	lessonRules, err := CaptureLessons(s.store, "")
	if err != nil {
		return nil, err
	}
	cands = append(cands, lessonRules...)
	b.flush()
	snap := rules.BuildSnapshot(cands)
	if err := s.store.UserRules.Save(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// GetOrBuild 返回当前快照；缺失时按 system_defaults + rules 文件初始化。
// 运行时读取路径统一走这里（遵守钉住的快照：写作中的章节看到的是它开始时的版本）。
func (s *Service) GetOrBuild(ctx context.Context) (*rules.Snapshot, error) {
	cur, err := s.store.UserRules.Load()
	if err != nil {
		return nil, err
	}
	if cur != nil {
		return cur, nil
	}
	return s.Build(ctx, "")
}

// AddRuntimeRule 归一化一条运行中长期规则，以最高优先级叠加到当前快照并落盘，
// 同时把候选记入运行时列表（界面"Học từ góp ý"、重新应用时保留）。
// 永不因归一化失败而报错——失败时该条降级为 raw preferences。
// 返回叠加后的快照与本次的归一化候选。
func (s *Service) AddRuntimeRule(ctx context.Context, text string) (*rules.Snapshot, rules.Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.store.UserRules.LoadCurrent()
	if err != nil {
		return nil, rules.Candidate{}, err
	}
	if cur == nil {
		if cur, err = s.Build(ctx, ""); err != nil {
			return nil, rules.Candidate{}, err
		}
	}
	b := s.newBatch()
	cand := b.normalize(ctx, store.RuntimeRuleSource, text)
	b.flush()
	rule := store.RuntimeRule{
		ID:        store.NewRuntimeRuleID(),
		Text:      strings.TrimSpace(text),
		Chapter:   s.currentChapter(),
		CreatedAt: time.Now(),
		Candidate: cand,
	}
	if err := s.store.UserRules.AddRuntime(rule); err != nil {
		return nil, cand, err
	}
	merged := rules.OverlaySnapshot(*cur, cand)
	if err := s.store.UserRules.Save(&merged); err != nil {
		return nil, cand, err
	}
	// Chương đang viết (nếu đã bị ghim bởi một lần Áp dụng) cũng nhận luật Trọng tài vừa học,
	// đúng như trước khi có tính năng ghim: luật học từ chat có hiệu lực ngay.
	if err := s.store.UserRules.UpdatePin(func(pinned rules.Snapshot) rules.Snapshot {
		return rules.OverlaySnapshot(pinned, cand)
	}); err != nil {
		return nil, cand, err
	}
	return &merged, cand, nil
}

// currentChapter 是此刻"正在写/刚写完"的章号：在写的章优先，否则最近完成的章。
func (s *Service) currentChapter() int {
	p, err := s.store.Progress.Load()
	if err != nil || p == nil {
		return 0
	}
	if p.InProgressChapter > 0 {
		return p.InProgressChapter
	}
	last := 0
	for _, ch := range p.CompletedChapters {
		last = max(last, ch)
	}
	return last
}
