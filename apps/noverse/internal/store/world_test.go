package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

// TestLoadEmpty 统一验证所有领域的空读取行为。
func TestLoadEmpty(t *testing.T) {
	s := newTestStore(t)

	if v, err := s.World.LoadTimeline(); err != nil || v != nil {
		t.Errorf("Timeline: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadForeshadowLedger(); err != nil || v != nil {
		t.Errorf("Foreshadow: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadRelationships(); err != nil || v != nil {
		t.Errorf("Relationships: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadStateChanges(); err != nil || v != nil {
		t.Errorf("StateChanges: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadStyleRules(); err != nil || v != nil {
		t.Errorf("StyleRules: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadWorldRules(); err != nil || v != nil {
		t.Errorf("WorldRules: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadReview(99); err != nil || v != nil {
		t.Errorf("Review: want (nil, nil), got (%v, %v)", v, err)
	}
	if v, err := s.World.LoadLastReview(10); err != nil || v != nil {
		t.Errorf("LastReview: want (nil, nil), got (%v, %v)", v, err)
	}
}

// ── Timeline ──

func TestTimeline_Append(t *testing.T) {
	s := newTestStore(t)

	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{
		{Chapter: 1, Time: "清晨", Event: "事件一"},
	}); err != nil {
		t.Fatalf("batch1: %v", err)
	}
	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{
		{Chapter: 2, Time: "午后", Event: "事件二"},
		{Chapter: 3, Time: "傍晚", Event: "事件三"},
	}); err != nil {
		t.Fatalf("batch2: %v", err)
	}

	loaded, err := s.World.LoadTimeline()
	if err != nil {
		t.Fatalf("LoadTimeline: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("want 3, got %d", len(loaded))
	}
	if loaded[2].Event != "事件三" {
		t.Errorf("third event: %+v", loaded[2])
	}
}

func TestTimeline_AppendIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	event := domain.TimelineEvent{
		Chapter:    1,
		Time:       "清晨",
		Event:      "林墨入住客栈",
		Characters: []string{"林墨", "老周"},
	}
	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{event}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	event.Characters = []string{"老周", "林墨"} // 角色顺序不应影响同一事件判定
	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{event}); err != nil {
		t.Fatalf("append duplicate: %v", err)
	}

	loaded, err := s.World.LoadTimeline()
	if err != nil {
		t.Fatalf("LoadTimeline: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("duplicate timeline event should be ignored, got %d: %+v", len(loaded), loaded)
	}

	// 跨重启仍从 JSONL 重建去重索引，commit Saga 重放不能产生重复记录。
	s2 := NewStore(s.Dir())
	if err := s2.World.AppendTimelineEvents([]domain.TimelineEvent{event}); err != nil {
		t.Fatalf("append duplicate after restart: %v", err)
	}
	loaded, err = s2.World.LoadTimeline()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("restart idempotency: got (%+v, %v)", loaded, err)
	}
}

func TestTimeline_DedupKeyDoesNotCollideOnContent(t *testing.T) {
	s := newTestStore(t)
	events := []domain.TimelineEvent{
		{Chapter: 1, Time: "a|b", Event: "c"},
		{Chapter: 1, Time: "a", Event: "b|c"},
	}
	if err := s.World.AppendTimelineEvents(events); err != nil {
		t.Fatalf("append: %v", err)
	}
	loaded, err := s.World.LoadTimeline()
	if err != nil || len(loaded) != 2 {
		t.Fatalf("delimiter-bearing events must remain distinct: got (%+v, %v)", loaded, err)
	}
}

func TestTimeline_MigratesLegacyAndAppendsProjection(t *testing.T) {
	dir := t.TempDir()
	// legacy layout seeded directly via IO below (was written through the store internals)
	legacy := []domain.TimelineEvent{{Chapter: 1, Time: "清晨", Event: "旧事件"}}
	legacyIO := newIO(dir)
	if err := legacyIO.WriteJSON("timeline.json", legacy); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := legacyIO.WriteMarkdown("timeline.md", "stale projection"); err != nil {
		t.Fatalf("write stale projection: %v", err)
	}

	s := NewStore(dir)
	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{
		{Chapter: 2, Time: "午后", Event: "新事件"},
	}); err != nil {
		t.Fatalf("append with migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "timeline.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy timeline.json should be removed, err=%v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "timeline.jsonl"))
	if err != nil {
		t.Fatalf("read timeline.jsonl: %v", err)
	}

	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{
		{Chapter: 3, Time: "傍晚", Event: "追加事件"},
	}); err != nil {
		t.Fatalf("append jsonl: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "timeline.jsonl"))
	if err != nil {
		t.Fatalf("read appended timeline.jsonl: %v", err)
	}
	if !strings.HasPrefix(string(after), string(before)) || len(after) <= len(before) {
		t.Fatal("timeline.jsonl should preserve old bytes and append new records")
	}

	loaded, err := s.World.LoadTimeline()
	if err != nil || len(loaded) != 3 {
		t.Fatalf("load migrated timeline: got (%+v, %v)", loaded, err)
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "timeline.md"))
	if err != nil {
		t.Fatalf("read timeline.md: %v", err)
	}
	if string(markdown) != renderTimeline(loaded, mdLabelsVI) {
		t.Fatalf("timeline projection not synchronized:\n%s", markdown)
	}
}

func TestTimeline_RepairsUncommittedJSONLTail(t *testing.T) {
	s := newTestStore(t)
	if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{{Chapter: 1, Event: "完整记录"}}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	path := filepath.Join(s.Dir(), "timeline.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	if _, err := f.WriteString(`{"chapter":2,"event":"未提交`); err != nil {
		_ = f.Close()
		t.Fatalf("write partial tail: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close jsonl: %v", err)
	}

	s2 := NewStore(s.Dir())
	if err := s2.World.AppendTimelineEvents([]domain.TimelineEvent{{Chapter: 2, Event: "重放记录"}}); err != nil {
		t.Fatalf("append after partial tail: %v", err)
	}
	loaded, err := s2.World.LoadTimeline()
	if err != nil || len(loaded) != 2 || loaded[1].Event != "重放记录" {
		t.Fatalf("tail recovery: got (%+v, %v)", loaded, err)
	}
}

func TestTimeline_RejectsCorruptCommittedJSONLRecord(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "timeline.jsonl"), []byte("{broken}\n"), 0o644); err != nil {
		t.Fatalf("write corrupt jsonl: %v", err)
	}
	s := NewStore(dir)
	if _, err := s.World.LoadTimeline(); err == nil {
		t.Fatal("committed corrupt record must fail loudly")
	}
}

func TestTimeline_LoadRecent(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.SaveTimeline([]domain.TimelineEvent{
		{Chapter: 1}, {Chapter: 3}, {Chapter: 5}, {Chapter: 7},
	})

	for _, tt := range []struct {
		current, window, want int
	}{
		{7, 10, 4}, // 全部
		{7, 3, 2},  // ch5,ch7
		{5, 2, 3},  // ch3,ch5,ch7
	} {
		got, _ := s.World.LoadRecentTimeline(tt.current, tt.window)
		if len(got) != tt.want {
			t.Errorf("LoadRecent(%d,%d): want %d, got %d", tt.current, tt.window, tt.want, len(got))
		}
	}
}

// ── Foreshadow ──

func TestForeshadow_UpdateLifecycle(t *testing.T) {
	s := newTestStore(t)

	// plant
	_ = s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影"},
		{ID: "f2", Action: "plant", Description: "断剑"},
	})
	// advance f1, resolve f2
	_ = s.World.UpdateForeshadow(3, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "advance"},
		{ID: "f2", Action: "resolve"},
	})

	all, _ := s.World.LoadForeshadowLedger()
	if len(all) != 2 {
		t.Fatalf("want 2, got %d", len(all))
	}
	if all[0].Status != "advanced" {
		t.Errorf("f1: want advanced, got %s", all[0].Status)
	}
	if all[1].Status != "resolved" || all[1].ResolvedAt != 3 {
		t.Errorf("f2: want resolved@3, got %s@%d", all[1].Status, all[1].ResolvedAt)
	}

	// LoadActive 应排除 resolved
	active, _ := s.World.LoadActiveForeshadow()
	if len(active) != 1 || active[0].ID != "f1" {
		t.Errorf("active: want [f1], got %v", active)
	}
}

func TestForeshadow_RejectsUnknownAndInvalidOperations(t *testing.T) {
	s := newTestStore(t)
	for _, update := range []domain.ForeshadowUpdate{
		{ID: "missing", Action: "advance"},
		{ID: "missing", Action: "resolve"},
		{ID: "f1", Action: "unknown"},
		{ID: "f1", Action: "plant"},
	} {
		if err := s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{update}); err == nil {
			t.Fatalf("expected rejection for %+v", update)
		}
	}
	if ledger, err := s.World.LoadForeshadowLedger(); err != nil || len(ledger) != 0 {
		t.Fatalf("invalid operations must not mutate ledger: %+v err=%v", ledger, err)
	}
}

func TestForeshadow_PlantIsIdempotent(t *testing.T) {
	s := newTestStore(t)

	_ = s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影"},
	})
	_ = s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影"},
	})
	_ = s.World.UpdateForeshadow(3, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "advance"},
	})
	_ = s.World.UpdateForeshadow(3, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影"},
	})

	all, _ := s.World.LoadForeshadowLedger()
	if len(all) != 1 {
		t.Fatalf("duplicate plant should not append entries, got %d: %+v", len(all), all)
	}
	if all[0].Status != "advanced" {
		t.Fatalf("duplicate plant should not downgrade status, got %s", all[0].Status)
	}
}

// ── Relationships ──

func TestRelationships_UpdateMerge(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.SaveRelationships([]domain.RelationshipEntry{
		{CharacterA: "张三", CharacterB: "李四", Relation: "师徒", Chapter: 1},
	})

	// 更新已有 + 新增
	_ = s.World.UpdateRelationships([]domain.RelationshipEntry{
		{CharacterA: "张三", CharacterB: "李四", Relation: "挚友", Chapter: 5},
		{CharacterA: "王五", CharacterB: "赵六", Relation: "同门", Chapter: 5},
	})

	loaded, _ := s.World.LoadRelationships()
	if len(loaded) != 2 {
		t.Fatalf("want 2, got %d", len(loaded))
	}
	if loaded[0].Relation != "挚友" {
		t.Errorf("update failed: %+v", loaded[0])
	}
}

func TestRelationships_PairKeySymmetry(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.SaveRelationships([]domain.RelationshipEntry{
		{CharacterA: "张三", CharacterB: "李四", Relation: "师徒", Chapter: 1},
	})
	// B-A 顺序更新，应匹配同一条
	_ = s.World.UpdateRelationships([]domain.RelationshipEntry{
		{CharacterA: "李四", CharacterB: "张三", Relation: "反目", Chapter: 3},
	})

	loaded, _ := s.World.LoadRelationships()
	if len(loaded) != 1 {
		t.Fatalf("want 1 (merged), got %d", len(loaded))
	}
	if loaded[0].Relation != "反目" {
		t.Errorf("not updated: %+v", loaded[0])
	}
}

// ── StateChanges ──

func TestStateChanges_Append(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.AppendStateChanges([]domain.StateChange{
		{Chapter: 1, Entity: "张三", Field: "realm", NewValue: "练气期"},
	})
	_ = s.World.AppendStateChanges([]domain.StateChange{
		{Chapter: 3, Entity: "张三", Field: "realm", OldValue: "练气期", NewValue: "筑基期"},
	})

	loaded, _ := s.World.LoadStateChanges()
	if len(loaded) != 2 {
		t.Fatalf("want 2, got %d", len(loaded))
	}
	if loaded[1].NewValue != "筑基期" {
		t.Errorf("second: %+v", loaded[1])
	}
}

func TestStateChanges_AppendIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	change := domain.StateChange{
		Chapter:  1,
		Entity:   "张三",
		Field:    "realm",
		OldValue: "凡人",
		NewValue: "练气期",
	}
	_ = s.World.AppendStateChanges([]domain.StateChange{change})
	_ = s.World.AppendStateChanges([]domain.StateChange{change})

	loaded, _ := s.World.LoadStateChanges()
	if len(loaded) != 1 {
		t.Fatalf("duplicate state change should be ignored, got %d: %+v", len(loaded), loaded)
	}
}

func TestStateChanges_DedupKeyDoesNotCollideOnContent(t *testing.T) {
	s := newTestStore(t)
	changes := []domain.StateChange{
		{Chapter: 1, Entity: "a|b", Field: "c", OldValue: "d", NewValue: "e"},
		{Chapter: 1, Entity: "a", Field: "b|c", OldValue: "d", NewValue: "e"},
	}
	if err := s.World.AppendStateChanges(changes); err != nil {
		t.Fatalf("append: %v", err)
	}
	loaded, err := s.World.LoadStateChanges()
	if err != nil || len(loaded) != 2 {
		t.Fatalf("delimiter-bearing changes must remain distinct: got (%+v, %v)", loaded, err)
	}
}

func TestStateChanges_MigratesLegacyAndRemainsIdempotent(t *testing.T) {
	dir := t.TempDir()
	// legacy layout seeded directly via IO below (was written through the store internals)
	legacy := []domain.StateChange{{
		Chapter: 1, Entity: "张三", Field: "realm", NewValue: "练气期",
	}}
	if err := newIO(dir).WriteJSON("meta/state_changes.json", legacy); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	change := domain.StateChange{
		Chapter: 2, Entity: "张三", Field: "realm", OldValue: "练气期", NewValue: "筑基期",
	}
	s := NewStore(dir)
	if err := s.World.AppendStateChanges([]domain.StateChange{change}); err != nil {
		t.Fatalf("append with migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta", "state_changes.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy state_changes.json should be removed, err=%v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "meta", "state_changes.jsonl"))
	if err != nil {
		t.Fatalf("read state_changes.jsonl: %v", err)
	}

	next := domain.StateChange{
		Chapter: 3, Entity: "张三", Field: "realm", OldValue: "筑基期", NewValue: "金丹期",
	}
	if err := s.World.AppendStateChanges([]domain.StateChange{next}); err != nil {
		t.Fatalf("append jsonl: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "meta", "state_changes.jsonl"))
	if err != nil {
		t.Fatalf("read appended state_changes.jsonl: %v", err)
	}
	if !strings.HasPrefix(string(after), string(before)) || len(after) <= len(before) {
		t.Fatal("state_changes.jsonl should preserve old bytes and append new records")
	}

	// 新 Store 从日志恢复索引后重放相同 change，条目数仍保持不变。
	s2 := NewStore(dir)
	if err := s2.World.AppendStateChanges([]domain.StateChange{next}); err != nil {
		t.Fatalf("restart duplicate: %v", err)
	}
	loaded, err := s2.World.LoadStateChanges()
	if err != nil || len(loaded) != 3 {
		t.Fatalf("load migrated state changes: got (%+v, %v)", loaded, err)
	}
}

// ── StyleRules ──

func TestStyleRules_SaveAndLoad(t *testing.T) {
	s := newTestStore(t)
	rules := domain.WritingStyleRules{
		Volume: 1, Arc: 2,
		Prose:    []string{"短句为主"},
		Dialogue: []domain.CharacterVoice{{Name: "张三", Rules: []string{"粗犷"}}},
		Taboos:   []string{"不用网络用语"},
	}
	_ = s.World.SaveStyleRules(rules)

	loaded, _ := s.World.LoadStyleRules()
	if loaded == nil || loaded.Volume != 1 || len(loaded.Dialogue) != 1 {
		t.Errorf("roundtrip failed: %+v", loaded)
	}
}

// ── Reviews ──

func TestReview_SaveAndLoad(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.SaveReview(domain.ReviewEntry{Chapter: 3, Scope: "chapter", Verdict: "polish"})

	loaded, _ := s.World.LoadReview(3)
	if loaded == nil || loaded.Verdict != "polish" {
		t.Errorf("chapter review: %+v", loaded)
	}
}

func TestReview_GlobalScopeIsolation(t *testing.T) {
	s := newTestStore(t)
	_ = s.World.SaveReview(domain.ReviewEntry{Chapter: 5, Scope: "global", Verdict: "accept"})

	// chapter-scoped load 不应找到 global review
	if got, _ := s.World.LoadReview(5); got != nil {
		t.Errorf("chapter load should not find global: %+v", got)
	}
}

func TestReview_LoadLastReview(t *testing.T) {
	s := newTestStore(t)
	for _, ch := range []int{2, 5, 8} {
		_ = s.World.SaveReview(domain.ReviewEntry{Chapter: ch, Scope: "global", Verdict: "accept"})
	}

	for _, tt := range []struct {
		from, want int
	}{
		{10, 8}, {5, 5}, {3, 2},
	} {
		got, _ := s.World.LoadLastReview(tt.from)
		if got == nil || got.Chapter != tt.want {
			t.Errorf("LoadLastReview(%d): want ch%d, got %+v", tt.from, tt.want, got)
		}
	}
	// from=1 找不到
	if got, _ := s.World.LoadLastReview(1); got != nil {
		t.Errorf("from=1 should be nil, got %+v", got)
	}
}

// ── WorldRules ──

func TestWorldRules_SaveAndLoad(t *testing.T) {
	s := newTestStore(t)
	rules := []domain.WorldRule{
		{Category: "magic", Rule: "法术消耗精神力", Boundary: "精神力耗尽会昏迷"},
		{Category: "society", Rule: "贵族拥有裁判权", Boundary: "不得越权"},
	}
	_ = s.World.SaveWorldRules(rules)

	if _, err := os.Stat(filepath.Join(s.Dir(), "world_rules.json")); err != nil {
		t.Fatalf("json not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "world_rules.md")); err != nil {
		t.Fatalf("md not created: %v", err)
	}

	loaded, _ := s.World.LoadWorldRules()
	if len(loaded) != 2 || loaded[0].Rule != "法术消耗精神力" {
		t.Errorf("roundtrip: %+v", loaded)
	}
}

func TestRenderWorldRules(t *testing.T) {
	md := renderWorldRules([]domain.WorldRule{
		{Category: "magic", Rule: "法术消耗精神力", Boundary: "精神力耗尽会昏迷"},
		{Category: "society", Rule: "贵族有裁判权"},
		{Category: "magic", Rule: "禁咒需三人", Boundary: "单人施放会死"},
	}, mdLabelsVI)

	// magic 分组应在 society 之前
	if strings.Index(md, "## magic") >= strings.Index(md, "## society") {
		t.Error("magic should appear before society")
	}
	if !strings.Contains(md, "Ranh giới: 精神力耗尽会昏迷") {
		t.Error("missing boundary")
	}
	// 无 boundary 不应输出空边界行
	if strings.Contains(md, "Ranh giới: \n") {
		t.Error("empty boundary rendered")
	}
}

// TestRuleViolationsContract 违规事实存储契约(第五轮评审):
// 同章最新覆盖旧记录;重写后空列表视为已清;跨重启可读。
func TestRuleViolationsContract(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.World.SaveRuleViolations(3, []rules.Violation{
		{Rule: "fatigue_words", Target: "不禁", Actual: 9, Severity: rules.SeverityWarning},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := s.World.LoadRuleViolations(3); len(got) != 1 || got[0].Target != "不禁" {
		t.Fatalf("首次读取: %+v", got)
	}

	// 同章重写:最新记录(空列表=已清)覆盖旧违规
	if err := s.World.SaveRuleViolations(3, nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if got := s.World.LoadRuleViolations(3); len(got) != 0 {
		t.Fatalf("重写后旧违规应被清除: %+v", got)
	}

	// 其他章不受影响 + 跨重启(新 Store 实例)可读
	if err := s.World.SaveRuleViolations(5, []rules.Violation{{Rule: "forbidden_phrases", Target: "某种程度上", Actual: 2, Severity: rules.SeverityWarning}}); err != nil {
		t.Fatalf("save ch5: %v", err)
	}
	s2 := NewStore(dir)
	if got := s2.World.LoadRuleViolations(5); len(got) != 1 || got[0].Rule != "forbidden_phrases" {
		t.Fatalf("跨重启读取: %+v", got)
	}
	if got := s2.World.LoadRuleViolations(99); got != nil {
		t.Fatalf("无记录章节应返回 nil: %+v", got)
	}
}

// TestReview_LoadLastReviewBoundedScan: fromChapter khổng lồ không được biến
// thành vòng quét lùi hàng tỷ lần đọc tệp.
//
// Trước vá: LoadLastReview(2147483647) đọc reviews/NN-global.json từng chương
// một từ 2147483647 xuống 1 — production 2026-10-26 22:31, novel_context(
// chapter=2147483647) treo agent vĩnh viễn, không có kết quả trả về.
func TestReview_LoadLastReviewBoundedScan(t *testing.T) {
	s := newTestStore(t)
	for _, ch := range []int{2, 5, 8} {
		_ = s.World.SaveReview(domain.ReviewEntry{Chapter: ch, Scope: "global", Verdict: "accept"})
	}

	done := make(chan *domain.ReviewEntry, 1)
	go func() {
		r, _ := s.World.LoadLastReview(2147483647)
		done <- r
	}()
	select {
	case r := <-done:
		if r == nil || r.Chapter != 8 {
			t.Fatalf("LoadLastReview(2147483647) = %+v, want review chương 8", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("LoadLastReview(2147483647) không trả về sau 2s — vẫn quét lùi từng chương")
	}
}

// 带 quote 的伏笔操作在账本上留下 EvidenceRef 证据；重放同一章不产生重复证据。
func TestForeshadow_UpdateRecordsEvidence(t *testing.T) {
	s := newTestStore(t)
	if err := s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "buc-thu", Action: "plant", Description: "Bức thư chưa mở", Quote: "Lâm Uyển nhìn thấy bức thư cũ trên bàn"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateForeshadow(3, []domain.ForeshadowUpdate{
		{ID: "buc-thu", Action: "advance", Quote: "Chữ ký cuối thư khiến tay anh run"},
		{ID: "khac", Action: "plant", Description: "Không có quote"},
	}); err != nil {
		t.Fatal(err)
	}
	// 同章重放：payload 不变，证据必须幂等。
	if err := s.World.UpdateForeshadow(3, []domain.ForeshadowUpdate{
		{ID: "buc-thu", Action: "advance", Quote: "Chữ ký cuối thư khiến tay anh run"},
		{ID: "khac", Action: "plant", Description: "Không có quote"},
	}); err != nil {
		t.Fatal(err)
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger: %+v", ledger)
	}
	want := []domain.EvidenceRef{
		{Chapter: 1, Action: "plant", Quote: "Lâm Uyển nhìn thấy bức thư cũ trên bàn"},
		{Chapter: 3, Action: "advance", Quote: "Chữ ký cuối thư khiến tay anh run"},
	}
	if got := ledger[0].Evidence; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("evidence = %+v want %+v", got, want)
	}
	if len(ledger[1].Evidence) != 0 {
		t.Fatalf("no-quote plant must not record evidence: %+v", ledger[1].Evidence)
	}
}

// 旧账本没有 evidence 字段仍能正常加载并继续追加。
func TestForeshadow_LegacyLedgerWithoutEvidenceLoads(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(s.Dir(), "foreshadow_ledger.json")
	if err := os.WriteFile(path, []byte(`[{"id":"fs-cu","description":"Phục bút cũ","planted_at":1,"status":"planted"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := s.World.LoadForeshadowLedger()
	if err != nil || len(entries) != 1 {
		t.Fatalf("legacy ledger: %+v err=%v", entries, err)
	}
	if len(entries[0].Evidence) != 0 {
		t.Fatalf("legacy entry must have no evidence: %+v", entries[0])
	}
	if err := s.World.UpdateForeshadow(4, []domain.ForeshadowUpdate{
		{ID: "fs-cu", Action: "advance", Quote: "đoạn trích chứng minh"},
	}); err != nil {
		t.Fatal(err)
	}
	entries, err = s.World.LoadForeshadowLedger()
	if err != nil || len(entries) != 1 || len(entries[0].Evidence) != 1 || entries[0].Evidence[0].Chapter != 4 {
		t.Fatalf("legacy ledger after update: %+v err=%v", entries, err)
	}
}

// 已 đóng 的伏笔（resolved / abandoned）上再 đẩy / thu / bỏ đều là no-op：
// 章节记录全量重放会照原样重演这些操作，任何报错都会弄断恢复，任何改写都会让账本漂移。
func TestForeshadow_ClosedLedgerReplayIsNoOp(t *testing.T) {
	s := newTestStore(t)
	for _, step := range []struct {
		chapter int
		updates []domain.ForeshadowUpdate
	}{
		{1, []domain.ForeshadowUpdate{
			{ID: "f_resolve", Action: "plant", Description: "Tuyến thu bằng resolve"},
			{ID: "f_abandon", Action: "plant", Description: "Tuyến bỏ có chủ đích"},
		}},
		{2, []domain.ForeshadowUpdate{
			{ID: "f_resolve", Action: "resolve"},
			{ID: "f_abandon", Action: "abandon", Reason: "tuyến phụ bị cắt"},
		}},
		{3, []domain.ForeshadowUpdate{
			{ID: "f_resolve", Action: "advance"},
			{ID: "f_resolve", Action: "resolve"},
			{ID: "f_resolve", Action: "abandon", Reason: "muộn"},
			{ID: "f_abandon", Action: "advance"},
			{ID: "f_abandon", Action: "resolve"},
		}},
	} {
		if err := s.World.UpdateForeshadow(step.chapter, step.updates); err != nil {
			t.Fatalf("chương %d: %v", step.chapter, err)
		}
	}

	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger: %+v", ledger)
	}
	resolved, abandoned := ledger[0], ledger[1]
	if resolved.ID != "f_resolve" || resolved.Status != "resolved" || resolved.ResolvedAt != 2 {
		t.Fatalf("resolve replay must keep closed state: %+v", resolved)
	}
	if abandoned.ID != "f_abandon" || abandoned.Status != "abandoned" || abandoned.AbandonedAt != 2 ||
		abandoned.AbandonReason != "tuyến phụ bị cắt" {
		t.Fatalf("abandon replay must keep closed state: %+v", abandoned)
	}

	active, err := s.World.LoadActiveForeshadow()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("đã bỏ tuyến không được coi là đang mở: %+v", active)
	}
}

func TestForeshadow_AbandonRequiresReason(t *testing.T) {
	s := newTestStore(t)
	if err := s.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateForeshadow(2, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "abandon"},
	}); err == nil {
		t.Fatal("abandon thiếu reason phải bị từ chối")
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil || len(ledger) != 1 || ledger[0].Status != "planted" {
		t.Fatalf("bị từ chối thì账本 không đổi: %+v err=%v", ledger, err)
	}
}

func TestForeshadow_DueByMustBeAfterPlantChapter(t *testing.T) {
	s := newTestStore(t)
	if err := s.World.UpdateForeshadow(5, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影", DueBy: 5},
	}); err == nil {
		t.Fatal("due_by bằng chương gieo phải bị từ chối")
	}
	if err := s.World.UpdateForeshadow(5, []domain.ForeshadowUpdate{
		{ID: "f1", Action: "plant", Description: "黑影", DueBy: 6},
	}); err != nil {
		t.Fatal(err)
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil || len(ledger) != 1 || ledger[0].DueBy != 6 || ledger[0].LastTouched != 5 {
		t.Fatalf("plant phải ghi due_by và last_touched: %+v err=%v", ledger, err)
	}
}

// 旧账本没有 last_touched / due_by：加载后继续 đẩy / thu 必须得到与全新账本
// 逐字段相同的结果（老数据不因升级而漂移）。
func TestForeshadow_LegacyLedgerReplaysIdentically(t *testing.T) {
	legacy := newTestStore(t)
	path := filepath.Join(legacy.Dir(), "foreshadow_ledger.json")
	if err := os.WriteFile(path, []byte(`[{"id":"fs-cu","description":"Phục bút cũ","planted_at":1,"status":"planted"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	fresh := newTestStore(t)
	if err := fresh.World.UpdateForeshadow(1, []domain.ForeshadowUpdate{
		{ID: "fs-cu", Action: "plant", Description: "Phục bút cũ"},
	}); err != nil {
		t.Fatal(err)
	}

	for _, step := range []struct {
		chapter int
		updates []domain.ForeshadowUpdate
	}{
		{4, []domain.ForeshadowUpdate{{ID: "fs-cu", Action: "advance"}}},
		{5, []domain.ForeshadowUpdate{{ID: "fs-cu", Action: "resolve"}}},
	} {
		if err := legacy.World.UpdateForeshadow(step.chapter, step.updates); err != nil {
			t.Fatal(err)
		}
		if err := fresh.World.UpdateForeshadow(step.chapter, step.updates); err != nil {
			t.Fatal(err)
		}
	}

	got, err := legacy.World.LoadForeshadowLedger()
	if err != nil {
		t.Fatal(err)
	}
	want, err := fresh.World.LoadForeshadowLedger()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("旧账本重放结果不一致\nlegacy: %+v\nfresh:  %+v", got, want)
	}
	if want[0].LastTouched != 4 {
		t.Fatalf("advance phải cập nhật last_touched: %+v", want[0])
	}
}
