package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
)

func TestGraphSelectionAndProtectedBudget(t *testing.T) {
	s := naoToolStore(t)
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}}, {Name: "Kỷ Dương"}, {Name: "Đỗ Mẫn"}, {Name: "Người xa"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateAddressTerms(1, []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateRelationships([]domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Đỗ Mẫn", Relation: "bạn", Chapter: 1}, {CharacterA: "Đỗ Mẫn", CharacterB: "Người xa", Relation: "bạn", Chapter: 1}, {CharacterA: "Kỷ Dương", CharacterB: "Lâm Uyển", Relation: "thù", Chapter: 8}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveTimeline([]domain.TimelineEvent{{Chapter: 1, Event: "Uyển Uyển đến"}, {Chapter: 2, Event: "Người xa đi"}, {Chapter: 4, Event: "Kỷ Dương đến tương lai"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{{ID: "open", Description: "Uyển Uyển giữ chìa khóa", PlantedAt: 1, Status: "resolved", ResolvedAt: 5}, {ID: "closed", Description: "Kỷ Dương tìm thư", PlantedAt: 1, Status: "resolved", ResolvedAt: 2}, {ID: "unrelated", Description: "Người xa", PlantedAt: 1, Status: "planted"}}); err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"working_memory": map[string]any{}, "episodic_memory": map[string]any{}}
	tool := NewContextTool(s, References{}, "", NewStyleStatsIndex(s))
	reads := &contextReads{}
	tool.applyGraphContext(result, contextBuildState{chapter: 3, profile: domain.NewContextProfile(10), currentEntry: &domain.OutlineEntry{Scenes: []string{"Uyển Uyển gặp Kỷ Dương"}}}, reads)
	if reads.err != nil {
		t.Fatal(reads.err)
	}
	episodic := result["episodic_memory"].(map[string]any)
	if len(episodic["characters"].([]domain.Character)) != 2 {
		t.Fatal(episodic)
	}
	relations := episodic["relationship_state"].([]domain.RelationshipChange)
	if len(relations) != 1 || relations[0].To != "nv:do-man" {
		t.Fatal(relations)
	}
	working := result["working_memory"].(map[string]any)
	timeline := working["timeline"].([]domain.TimelineEvent)
	foreshadows := episodic["foreshadow_ledger"].([]domain.ForeshadowEntry)
	if len(timeline) != 1 || timeline[0].Chapter != 1 || len(foreshadows) != 1 || foreshadows[0].ID != "open" {
		t.Fatalf("timeline=%+v foreshadows=%+v", timeline, foreshadows)
	}
	if len(working["address_terms"].([]domain.AddressTerm)) != 1 {
		t.Fatal(working)
	}
	// Deliberately unsorted events: oldest is removed, not the first or the whole block.
	working["timeline"] = []domain.TimelineEvent{{Chapter: 2, Event: strings.Repeat("new", 200)}, {Chapter: 1, Event: strings.Repeat("old", 200)}}
	full, _ := json.Marshal(result)
	if err := trimByBudget(result, len(full)-400); err != nil {
		t.Fatal(err)
	}
	remaining := working["timeline"].([]domain.TimelineEvent)
	if len(remaining) != 1 || remaining[0].Chapter != 2 {
		t.Fatalf("remaining=%+v", remaining)
	}
	if len(working["address_terms"].([]domain.AddressTerm)) != 1 {
		t.Fatal("address terms trimmed")
	}
	trimmed := result["_trimmed"].([]string)
	if len(trimmed) != 1 || !strings.Contains(trimmed[0], "timeline") || !strings.Contains(trimmed[0], "@1") {
		t.Fatal(trimmed)
	}
	if err := trimByBudget(result, 1); err == nil {
		t.Fatal("protected context must report impossible budget")
	}
	if len(working["address_terms"].([]domain.AddressTerm)) != 1 {
		t.Fatal("protected terms lost")
	}
}

// TestGraphForeshadowRespectsAbandonAtChapter 守护 as-of-N 召回语义：
// 账本按"第 N 章写作当时是否仍未收束"过滤——N 之前已 abandon/resolve 的
// 条目不得再出现（此前 abandon 条目会被全量账本原样带回），而 N 之后才
// 收束的条目保留：重写早期章节时，那条线在当时确实还悬着。
// N 之后才收束的条目出现在账本块时，收束事实必须被截回"当时仍开着"。
func TestGraphForeshadowRespectsAbandonAtChapter(t *testing.T) {
	s := naoToolStore(t)
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateRelationships([]domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "bạn", Chapter: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "mo", Description: "Uyển Uyển giữ chìa khóa", PlantedAt: 1, Status: "planted"},
		{ID: "bo-truoc", Description: "Uyển Uyển ôm hộp thư", PlantedAt: 1, Status: "abandoned", AbandonedAt: 2, AbandonReason: "cắt tuyến"},
		{ID: "bo-sau", Description: "Uyển Uyển giữ chiếc khăn", PlantedAt: 1, Status: "abandoned", AbandonedAt: 5, AbandonReason: "sau này bỏ"},
		{ID: "thu-truoc", Description: "Uyển Uyển tìm lá thư", PlantedAt: 1, Status: "resolved", ResolvedAt: 2},
		{ID: "thu-sau", Description: "Uyển Uyển giữ con dấu", PlantedAt: 1, Status: "resolved", ResolvedAt: 4, LastTouched: 6},
	}); err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"working_memory": map[string]any{}, "episodic_memory": map[string]any{}}
	tool := NewContextTool(s, References{}, "", NewStyleStatsIndex(s))
	reads := &contextReads{}
	tool.applyGraphContext(result, contextBuildState{chapter: 3, profile: domain.NewContextProfile(10), currentEntry: &domain.OutlineEntry{Scenes: []string{"Uyển Uyển vào phòng"}}}, reads)
	if reads.err != nil {
		t.Fatal(reads.err)
	}
	got := map[string]domain.ForeshadowEntry{}
	for _, e := range result["episodic_memory"].(map[string]any)["foreshadow_ledger"].([]domain.ForeshadowEntry) {
		got[e.ID] = e
	}
	for _, id := range []string{"bo-truoc", "thu-truoc"} {
		if _, ok := got[id]; ok {
			t.Fatalf("phục bút %q đã đóng trước chương 3 không được nhớ lại: %v", id, got)
		}
	}
	for _, id := range []string{"mo", "bo-sau", "thu-sau"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("phục bút %q còn mở tại chương 3 phải được nhớ lại: %v", id, got)
		}
	}
	// as-of-N 截断：thu-sau 在第 4 章才收束、第 6 章才推进——写第 3 章时
	// 这些事实都还不存在，账本块里的条目必须退回"当时仍开着"。
	if e := got["thu-sau"]; e.Status != "planted" || e.ResolvedAt != 0 || e.LastTouched != 0 {
		t.Fatalf("thu-sau phải被截回第 3 章视角: %+v", e)
	}
	if e := got["bo-sau"]; e.Status != "planted" || e.AbandonedAt != 0 || e.AbandonReason != "" {
		t.Fatalf("bo-sau phải被截回第 3 章视角: %+v", e)
	}
}

// TestGraphStoryThreadsRecomputedAsOfN 守护图模式下的 overdue/dueSoon 提示：
// story_threads 以前被整块删掉，等于关掉了到期/沉睡提醒。现在按
// openAtN 重跑同一条召回管线——N 章视角下已到期/快到期的线必须带原因回来，
// 而 N 之后才种下的线与 N 之前已收束的线都不得出现。
func TestGraphStoryThreadsRecomputedAsOfN(t *testing.T) {
	s := naoToolStore(t)
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateRelationships([]domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "bạn", Chapter: 1}}); err != nil {
		t.Fatal(err)
	}
	// 7 条 ≥ storyThreadRecallThreshold；关键词召回只命中含 Uyển Uyển 的两条，
	// 其余靠 overdue/dueSoon 回填——恰好覆盖被删掉的 P07 提示路径。
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: "mo", Description: "Uyển Uyển giữ chìa khóa", PlantedAt: 1, Status: "planted"},
		{ID: "bo-sau", Description: "Uyển Uyển giữ chiếc khăn", PlantedAt: 1, Status: "abandoned", AbandonedAt: 5, AbandonReason: "sau này bỏ"},
		{ID: "qua-han", Description: "vương miện cổ", PlantedAt: 1, Status: "planted", DueBy: 2},
		{ID: "sap-han", Description: "bức thư chưa mở", PlantedAt: 1, Status: "planted", DueBy: 4},
		{ID: "binh-thuong", Description: "cây đèn cũ", PlantedAt: 1, Status: "planted"},
		{ID: "bo-truoc", Description: "hộp thư gỉ", PlantedAt: 1, Status: "abandoned", AbandonedAt: 2, AbandonReason: "cắt tuyến"},
		{ID: "chua-gieo", Description: "vật phẩm tương lai", PlantedAt: 9, Status: "planted", DueBy: 2},
	}); err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"working_memory": map[string]any{}, "episodic_memory": map[string]any{}}
	tool := NewContextTool(s, References{}, "", NewStyleStatsIndex(s))
	reads := &contextReads{}
	tool.applyGraphContext(result, contextBuildState{chapter: 3, profile: domain.NewContextProfile(10), currentEntry: &domain.OutlineEntry{Scenes: []string{"Uyển Uyển vào phòng"}}}, reads)
	if reads.err != nil {
		t.Fatal(reads.err)
	}
	selected, ok := result["selected_memory"].(map[string]any)
	if !ok {
		t.Fatal("graph path phải giữ lại selected_memory.story_threads")
	}
	rawThreads, ok := selected["story_threads"].([]domain.RecallItem)
	if !ok || len(rawThreads) == 0 {
		t.Fatalf("story_threads phải được as-of-N 重算, nhận %v", selected["story_threads"])
	}
	byKey := map[string]domain.RecallItem{}
	for _, item := range rawThreads {
		byKey[item.Key] = item
	}
	for _, id := range []string{"qua-han", "sap-han"} {
		item, ok := byKey[id]
		if !ok {
			t.Fatalf("phục bút %q đã/sắp đến hạn tại chương 3 phải được nhắc: %+v", id, rawThreads)
		}
		want := map[string]string{"qua-han": "quá hạn", "sap-han": "sắp đến hạn"}[id]
		if !strings.Contains(item.Reason, want) {
			t.Fatalf("phục bút %q phải nêu lý do %q, nhận %+v", id, want, item)
		}
	}
	for _, id := range []string{"bo-truoc", "chua-gieo"} {
		if _, ok := byKey[id]; ok {
			t.Fatalf("phục bút %q không tồn tại ở chương 3 không được nhắc: %+v", id, byKey[id])
		}
	}
}

func TestConsistencyActiveAddressPayload(t *testing.T) {
	s := naoToolStore(t)
	if err := s.World.UpdateAddressTerms(1, []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateAddressTerms(2, []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "cô", Self: "tôi", Reason: "cãi nhau"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := NewCheckConsistencyTool(s).Execute(context.Background(), json.RawMessage(`{"chapter":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Terms []domain.AddressTerm `json:"address_terms"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Terms) != 1 || result.Terms[0].Calls != "chị" {
		t.Fatalf("payload=%s", raw)
	}
}

func TestCommitIncludesAddressViolation(t *testing.T) {
	s := naoToolStore(t)
	if err := s.World.UpdateAddressTerms(1, []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Drafts.SaveDraft(1, "Kỷ Dương nói: “Lâm Uyển, tôi nhờ chị giúp.”"); err != nil {
		t.Fatal(err)
	}
	raw, err := newTestCommitChapterTool(s).Execute(context.Background(), json.RawMessage(`{"chapter":1,"title":"Gặp gỡ","summary":"Hai người gặp nhau","characters":["Lâm Uyển","Kỷ Dương"],"key_events":["Hợp tác"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"rule":"address_terms"`) || !strings.Contains(string(raw), `"severity":"error"`) {
		t.Fatalf("payload=%s", raw)
	}
}

func TestGraphLeavesLegacyContextUnchanged(t *testing.T) {
	s := naoToolStore(t)
	result := map[string]any{"working_memory": map[string]any{}, "episodic_memory": map[string]any{"legacy": true}}
	before, _ := json.Marshal(result)
	tool := NewContextTool(s, References{}, "", NewStyleStatsIndex(s))
	tool.applyGraphContext(result, contextBuildState{chapter: 1, currentEntry: &domain.OutlineEntry{Scenes: []string{"Kỷ Dương gặp Lâm Uyển"}}}, &contextReads{})
	after, _ := json.Marshal(result)
	if string(before) != string(after) {
		t.Fatalf("legacy context changed: %s", after)
	}
}

func TestSingleOverdueThreadSurvivesGraphContext(t *testing.T) {
	s := naoToolStore(t)
	if err := s.Characters.Save([]domain.Character{{Name: "Lan"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateRelationships([]domain.RelationshipEntry{{CharacterA: "Lan", CharacterB: "Mẹ", Relation: "gia đình", Chapter: 1}}); err != nil {
		t.Fatal(err)
	}
	thread := domain.ForeshadowEntry{ID: "old-letter", Description: "Lá thư hẹn ở bến xa", PlantedAt: 1, DueBy: 2, Status: "planted"}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{thread}); err != nil {
		t.Fatal(err)
	}
	tool := newTestContextTool(s, References{}, "")
	state := contextBuildState{chapter: 3, profile: domain.NewContextProfile(5),
		currentEntry: &domain.OutlineEntry{CoreEvent: "Lan sửa ổ khóa"}, foreshadow: []domain.ForeshadowEntry{thread}}
	assertRecall := func(items []domain.RecallItem) {
		t.Helper()
		if len(items) != 1 || items[0].Key != thread.ID || !strings.Contains(items[0].Reason, "quá hạn") {
			t.Fatalf("single overdue thread disappeared: %+v", items)
		}
	}
	assertRecall(tool.selectStoryThreads(state))
	result := map[string]any{"working_memory": map[string]any{}, "episodic_memory": map[string]any{}}
	reads := &contextReads{}
	tool.applyGraphContext(result, state, reads)
	if reads.err != nil {
		t.Fatal(reads.err)
	}
	selected, ok := result["selected_memory"].(map[string]any)
	if !ok {
		t.Fatal("graph context omitted selected memory")
	}
	items, _ := selected["story_threads"].([]domain.RecallItem)
	assertRecall(items)
}
