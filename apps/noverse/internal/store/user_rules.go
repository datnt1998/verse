package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

const (
	userRulesPath        = "meta/user_rules.json"
	userRulesRuntimePath = "meta/user_rules_runtime.json"
	userRulesPinPath     = "meta/user_rules_pin.json"
	userRulesCachePath   = "meta/user_rules_cache.json"

	// RuntimeRuleSource là nhãn nguồn của luật học từ góp ý (Trọng tài) trong Snapshot.Sources.
	RuntimeRuleSource = "runtime_update"

	normCacheLimit = 500
)

// UserRulesStore 管理本书归一化后的用户规则快照（meta/user_rules.json）。
//
// 运行时唯一事实源：novel_context 注入与 commit_chapter 检查都只读这一份，
// 不再反复读 rules 文件（避免漂移与双读者发散）。快照由开书/导入/刷新时归一化生成。
//
// 除快照外还有三份旁路状态（都只增不改旧文件格式）：
//   - Runtime：Trọng tài 学到的运行时规则候选，让"重新应用"能保留它们而不是只剩合并后的快照；
//   - Pin：应用规则时若有章节在写，把旧快照钉给该章，Load 在该章写作期间返回钉住的版本；
//   - NormCache：按内容哈希缓存归一化结果，重新应用不必再调模型。
type fileUserRulesStore struct{ io *IO }

func newFileUserRulesStore(io *IO) *fileUserRulesStore { return &fileUserRulesStore{io: io} }

// RuntimeRule là một luật học từ góp ý của người dùng (Trọng tài). Candidate là kết quả chuẩn
// hoá đã lưu để dựng lại snapshot mà không gọi lại model. Legacy đánh dấu mục được suy ra từ
// snapshot cũ chưa có danh sách runtime: Candidate của nó chỉ là nguyên văn, cần chuẩn hoá lại
// ở lần áp dụng kế tiếp.
type RuntimeRule struct {
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Chapter   int             `json:"chapter"`
	CreatedAt time.Time       `json:"createdAt"`
	Candidate rules.Candidate `json:"candidate"`
	Legacy    bool            `json:"legacy,omitempty"`
}

type runtimeFile struct {
	Rules []RuntimeRule `json:"rules"`
}

type userRulesPin struct {
	Chapter  int            `json:"chapter"`
	Snapshot rules.Snapshot `json:"snapshot"`
}

// NormCacheEntry là kết quả chuẩn hoá một đoạn văn bản (khoá = băm nội dung do userrules đặt).
type NormCacheEntry struct {
	Structured    rules.Structured `json:"structured"`
	Preferences   string           `json:"preferences,omitempty"`
	Uncertain     []string         `json:"uncertain,omitempty"`
	UncertainNote string           `json:"uncertain_note,omitempty"`
	UncertainFlag bool             `json:"is_uncertain,omitempty"`
	Options       []string         `json:"options,omitempty"`
	At            time.Time        `json:"at"`
}

type normCacheFile struct {
	Entries map[string]NormCacheEntry `json:"entries"`
}

// NewRuntimeRuleID 生成运行时规则 id。
func NewRuntimeRuleID() string {
	buf := make([]byte, 5)
	_, _ = rand.Read(buf)
	return "rt-" + hex.EncodeToString(buf)
}

// ── 快照 ──

// Load 读取当前章节应当看到的快照。应用规则时若某章正在写，该章写作期间返回应用前
// 钉住的快照；章节结束后（in_progress_chapter 变化）钉住的版本自动作废。
// 不存在时返回 nil（调用方据此惰性生成）。
func (s *fileUserRulesStore) Load() (*rules.Snapshot, error) {
	s.io.mu.RLock()
	snap, err := s.loadCurrentUnlocked()
	pin := s.readPinUnlocked()
	inProgress := s.inProgressChapterUnlocked()
	s.io.mu.RUnlock()
	if pin != nil {
		if inProgress > 0 && pin.Chapter == inProgress {
			pinned := pin.Snapshot
			return &pinned, nil
		}
		s.dropStalePin()
	}
	return snap, err
}

// LoadCurrent 读取最新应用的快照，忽略钉住的版本（运行时叠加、重新应用、界面展示用）。
func (s *fileUserRulesStore) LoadCurrent() (*rules.Snapshot, error) {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	return s.loadCurrentUnlocked()
}

func (s *fileUserRulesStore) loadCurrentUnlocked() (*rules.Snapshot, error) {
	var snap rules.Snapshot
	if err := s.io.ReadJSONUnlocked(userRulesPath, &snap); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &snap, nil
}

// Save 保存快照。
func (s *fileUserRulesStore) Save(snap *rules.Snapshot) (err error) {
	defer s.io.lockWrite()(&err)
	return s.io.WriteJSONUnlocked(userRulesPath, snap)
}

// ── 钉住 ──

func (s *fileUserRulesStore) readPinUnlocked() *userRulesPin {
	var pin userRulesPin
	if err := s.io.ReadJSONUnlocked(userRulesPinPath, &pin); err != nil || pin.Chapter <= 0 {
		return nil
	}
	return &pin
}

func (s *fileUserRulesStore) inProgressChapterUnlocked() int {
	var p struct {
		InProgressChapter int `json:"in_progress_chapter"`
	}
	if err := s.io.ReadJSONUnlocked("meta/progress.json", &p); err != nil {
		return 0
	}
	return p.InProgressChapter
}

// dropStalePin is best effort, like the removal itself: a pin that survives
// is dropped again on the next load.
func (s *fileUserRulesStore) dropStalePin() {
	_ = s.io.WithWriteLock(func() error {
		if pin := s.readPinUnlocked(); pin != nil && pin.Chapter != s.inProgressChapterUnlocked() {
			return s.io.RemoveFileUnlocked(userRulesPinPath)
		}
		return nil
	})
}

// ApplySnapshot 把 next 设为当前快照，并返回有章节在写时该章的章号（0 = 没有）。
// 有章节在写：把应用前的快照钉给该章（该章已有钉住版本则保留最早那份）——
// "chương đang viết dở giữ luật cũ"。next 与钉住动作在同一把锁内完成，读者不会看到中间态。
func (s *fileUserRulesStore) ApplySnapshot(next *rules.Snapshot) (pinnedChapter int, err error) {
	defer s.io.lockWrite()(&err)
	inProgress := s.inProgressChapterUnlocked()
	if inProgress > 0 {
		pin := s.readPinUnlocked()
		if pin == nil || pin.Chapter != inProgress {
			old, lerr := s.loadCurrentUnlocked()
			if lerr != nil {
				return 0, lerr
			}
			if old != nil {
				if err := s.io.WriteJSONUnlocked(userRulesPinPath, userRulesPin{Chapter: inProgress, Snapshot: *old}); err != nil {
					return 0, err
				}
			}
		}
		pinnedChapter = inProgress
	} else {
		_ = s.io.RemoveFileUnlocked(userRulesPinPath)
	}
	return pinnedChapter, s.io.WriteJSONUnlocked(userRulesPath, next)
}

// UpdatePin 在有生效中的钉住快照时对它应用 fn（Trọng tài 中途追加的规则要同样作用于在写章节）。
// 没有生效的钉住快照时什么也不做。
func (s *fileUserRulesStore) UpdatePin(fn func(rules.Snapshot) rules.Snapshot) (err error) {
	defer s.io.lockWrite()(&err)
	pin := s.readPinUnlocked()
	if pin == nil || pin.Chapter != s.inProgressChapterUnlocked() {
		return nil
	}
	pin.Snapshot = fn(pin.Snapshot)
	return s.io.WriteJSONUnlocked(userRulesPinPath, pin)
}

// ── 运行时规则候选 ──

// Runtime 返回运行时规则列表（最早的在前）。没有列表文件时按旧快照迁移：
// Sources 中每个 runtime_update 算一个条目。迁移结果在下一次修改列表时才落盘。
func (s *fileUserRulesStore) Runtime() ([]RuntimeRule, error) {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	return s.runtimeUnlocked()
}

func (s *fileUserRulesStore) runtimeUnlocked() ([]RuntimeRule, error) {
	var f runtimeFile
	err := s.io.ReadJSONUnlocked(userRulesRuntimePath, &f)
	switch {
	case err == nil:
		return f.Rules, nil
	case !os.IsNotExist(err):
		return nil, err
	}
	snap, err := s.loadCurrentUnlocked()
	if err != nil || snap == nil {
		return nil, err
	}
	var at time.Time
	if info, serr := os.Stat(userRulesPath); serr == nil {
		at = info.ModTime()
	}
	return legacyRuntimeRules(*snap, at), nil
}

// legacyRuntimeTexts 按 "## [来源]" 标题切分 Preferences，取出所有 runtime_update 段的正文（保持出现顺序）。
func legacyRuntimeTexts(preferences string) []string {
	var texts []string
	for _, section := range strings.Split("\n"+preferences, "\n## [")[1:] {
		head, body, ok := strings.Cut(section, "]\n\n")
		if !ok || head != RuntimeRuleSource {
			continue
		}
		if t := strings.TrimSpace(body); t != "" {
			texts = append(texts, t)
		}
	}
	return texts
}

// legacyRuntimeRules 从没有 runtime 列表的旧快照推出条目：Sources 里每个 runtime_update 一条；
// 原文取 Preferences 中对应的 "## [runtime_update]" 段（按出现顺序）。纯结构化的规则
// （没有偏好正文）不留原文，只能给占位文案。
func legacyRuntimeRules(snap rules.Snapshot, at time.Time) []RuntimeRule {
	count := 0
	for _, src := range snap.Sources {
		if src == RuntimeRuleSource {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	texts := legacyRuntimeTexts(snap.Preferences)
	out := make([]RuntimeRule, 0, count)
	for i := range count {
		rule := RuntimeRule{
			ID:        fmt.Sprintf("legacy-%d", i+1),
			Text:      "Quy tắc học từ góp ý của bạn (không còn nguyên văn)",
			CreatedAt: at,
			Candidate: rules.Candidate{Source: RuntimeRuleSource},
			Legacy:    true,
		}
		if i < len(texts) {
			rule.Text = texts[i]
			rule.Candidate.Preferences = texts[i]
		}
		out = append(out, rule)
	}
	return out
}

// AddRuntime 追加一条运行时规则（先把旧快照迁移出来的条目一并落盘）。
func (s *fileUserRulesStore) AddRuntime(rule RuntimeRule) (err error) {
	defer s.io.lockWrite()(&err)
	list, err := s.runtimeUnlocked()
	if err != nil {
		return err
	}
	return s.io.WriteJSONUnlocked(userRulesRuntimePath, runtimeFile{Rules: append(list, rule)})
}

// RemoveRuntime 移除一条运行时规则并返回它；id 不存在时 found=false。
func (s *fileUserRulesStore) RemoveRuntime(id string) (rule RuntimeRule, found bool, err error) {
	defer s.io.lockWrite()(&err)
	list, err := s.runtimeUnlocked()
	if err != nil {
		return rule, false, err
	}
	idx := slices.IndexFunc(list, func(r RuntimeRule) bool { return r.ID == id })
	if idx < 0 {
		return rule, false, nil
	}
	rule = list[idx]
	list = slices.Delete(slices.Clone(list), idx, idx+1)
	return rule, true, s.io.WriteJSONUnlocked(userRulesRuntimePath, runtimeFile{Rules: list})
}

// ReplaceRuntime 整体写回运行时规则列表（apply 时把旧版条目换成已重新归一化的候选）。
func (s *fileUserRulesStore) ReplaceRuntime(list []RuntimeRule) (err error) {
	defer s.io.lockWrite()(&err)
	return s.io.WriteJSONUnlocked(userRulesRuntimePath, runtimeFile{Rules: list})
}

// ── 归一化缓存 ──

// NormCache 读取缓存；文件缺失或损坏都视为空缓存（缓存只是省调用，不是事实源）。
func (s *fileUserRulesStore) NormCache() map[string]NormCacheEntry {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	return s.normCacheUnlocked()
}

func (s *fileUserRulesStore) normCacheUnlocked() map[string]NormCacheEntry {
	var f normCacheFile
	if err := s.io.ReadJSONUnlocked(userRulesCachePath, &f); err != nil || f.Entries == nil {
		return map[string]NormCacheEntry{}
	}
	return f.Entries
}

// PutNormCache 合并写入缓存，超过上限时丢弃最久未写入的条目。
func (s *fileUserRulesStore) PutNormCache(entries map[string]NormCacheEntry) (err error) {
	if len(entries) == 0 {
		return nil
	}
	defer s.io.lockWrite()(&err)
	all := s.normCacheUnlocked()
	for k, v := range entries {
		all[k] = v
	}
	TrimNormCache(all)
	return s.io.WriteJSONUnlocked(userRulesCachePath, normCacheFile{Entries: all})
}

// TrimNormCache 丢弃最久未写入的条目，使 all 不超过上限（normCacheLimit）。
func TrimNormCache(all map[string]NormCacheEntry) {
	if len(all) <= normCacheLimit {
		return
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int { return all[b].At.Compare(all[a].At) })
	for _, k := range keys[normCacheLimit:] {
		delete(all, k)
	}
}

// LessonRules is frozen at the first planning/start of a story. Reapplying user
// rules never pulls later system lessons into a story already in progress.
func (s *fileUserRulesStore) LessonRules() ([]rules.Candidate, bool, error) {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	var saved struct {
		Rules []rules.Candidate `json:"rules"`
	}
	err := s.io.ReadJSONUnlocked("meta/lesson_rules.json", &saved)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return saved.Rules, err == nil, err
}

// CaptureLessonRules never overwrites an existing story's captured layer.
func (s *fileUserRulesStore) CaptureLessonRules(candidates []rules.Candidate) (_ []rules.Candidate, err error) {
	defer s.io.lockWrite()(&err)
	var saved struct {
		Rules []rules.Candidate `json:"rules"`
	}
	err = s.io.ReadJSONUnlocked("meta/lesson_rules.json", &saved)
	if err == nil {
		return saved.Rules, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if candidates == nil {
		candidates = []rules.Candidate{}
	}
	saved.Rules = candidates
	if err := s.io.WriteJSONUnlocked("meta/lesson_rules.json", saved); err != nil {
		return nil, err
	}
	return candidates, nil
}
