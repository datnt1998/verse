package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// applyGraphContext is opt-in through W1 history; legacy stories keep their context policy.
func (t *ContextTool) applyGraphContext(result map[string]any, state contextBuildState, reads *contextReads) {
	log, err := t.store.World.LoadRelationshipLog()
	reads.require("relationship_log", err)
	terms, err := t.store.World.AddressAt(state.chapter)
	reads.require("address_terms", err)
	if len(log) == 0 && len(terms) == 0 {
		return
	}
	chars, err := t.store.Characters.Load()
	reads.require("characters", err)
	raw, _ := json.Marshal(struct {
		Outline *domain.OutlineEntry
		Plan    *domain.ChapterPlan
	}{state.currentEntry, state.chapterPlan})
	present := make(map[string]bool)
	var selected []domain.Character
	for _, c := range chars {
		if graphMentions(string(raw), c) {
			present[c.ID] = true
			selected = append(selected, c)
		}
	}
	// Do not guess a cast when no outline/plan names are available.
	if len(present) == 0 {
		return
	}
	result["_graph_context"] = true
	working := result["working_memory"].(map[string]any)
	episodic := result["episodic_memory"].(map[string]any)
	episodic["characters"] = selected
	delete(episodic, "character_snapshots")
	latest := make(map[string]domain.RelationshipChange)
	for _, e := range log {
		if e.Chapter > state.chapter {
			continue
		}
		a, b := e.From, e.To
		if a > b {
			a, b = b, a
		}
		key := a + "\x00" + b
		if old, ok := latest[key]; !ok || e.Chapter >= old.Chapter {
			latest[key] = e
		}
	}
	var relations []domain.RelationshipChange
	for _, e := range latest {
		if present[e.From] || present[e.To] {
			relations = append(relations, e)
		}
	}
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].Chapter != relations[j].Chapter {
			return relations[i].Chapter < relations[j].Chapter
		}
		return relations[i].From+relations[i].To < relations[j].From+relations[j].To
	})
	episodic["relationship_state"] = relations
	var active []domain.AddressTerm
	for _, e := range terms {
		if present[e.From] && present[e.To] {
			active = append(active, e)
		}
	}
	working["address_terms"] = active // Not a trimmable key, even when it alone exceeds budget.
	touches := func(text string) bool {
		for _, c := range selected {
			if graphMentions(text, c) {
				return true
			}
		}
		return false
	}
	// Read the full ledger to reconstruct openness at N, including later-closed entries.
	// 恢复第 N 章视角下的"仍未收束"：N 之前已 resolved/abandoned 的条目丢弃，
	// 但 N 之后才收束的保留——它们在写作当时仍悬而未决，属于有意为之的
	// as-of-N 历史召回；旧账本缺失收束章号的关闭条目视作早已关闭。
	ledger, err := t.store.World.LoadForeshadowLedger()
	reads.require("foreshadow_ledger", err)
	var openAtN []domain.ForeshadowEntry
	var foreshadows []domain.ForeshadowEntry
	for _, e := range ledger {
		if e.PlantedAt > state.chapter {
			continue
		}
		if e.Status == "resolved" && e.ResolvedAt < state.chapter {
			continue
		}
		if e.Status == "abandoned" && e.AbandonedAt < state.chapter {
			continue
		}
		// 未来的触及章号不能伪造成当前章发生过推进；缺少历史值时回到未知，
		// 由旧账本同样使用的 planted_at 回退规则处理。
		if e.LastTouched > state.chapter {
			e.LastTouched = 0
		}
		if !domain.ForeshadowOpen(e.Status) {
			e.Status = "planted"
		}
		e.ResolvedAt, e.AbandonedAt, e.AbandonReason = 0, 0, ""
		e.Evidence = nil
		openAtN = append(openAtN, e)
		if touches(e.Description) {
			foreshadows = append(foreshadows, e)
		}
	}
	episodic["foreshadow_ledger"] = foreshadows
	// story_threads 不能沿用 HEAD 视图算出的列表：那是当前账本快照，未来章
	// 的推进（LastTouched/Evidence）与"已收束→漏召回"都会污染 N 的视角。
	// 用 openAtN 重跑同一条召回管线，overdue/dueSoon/dormant 提示在重写 N 章
	// 时仍照常出现，且不含任何未来事实。
	graphState := state
	graphState.foreshadow = openAtN
	if threads := t.selectStoryThreads(graphState); len(threads) > 0 {
		selected, ok := result["selected_memory"].(map[string]any)
		if !ok {
			selected = map[string]any{}
			result["selected_memory"] = selected
		}
		selected["story_threads"] = threads
	} else if selectedMemory, ok := result["selected_memory"].(map[string]any); ok {
		delete(selectedMemory, "story_threads")
	}
	events, err := t.store.World.LoadRecentTimeline(state.chapter, state.profile.TimelineWindow)
	reads.require("timeline", err)
	var timeline []domain.TimelineEvent
	for _, e := range events {
		if e.Chapter < state.chapter && touches(strings.Join(e.Characters, " ")+" "+e.Event) {
			timeline = append(timeline, e)
		}
	}
	working["timeline"] = timeline
}

func graphMentions(text string, c domain.Character) bool {
	text = strings.ToLower(text)
	for n := range len(c.Aliases) + 2 {
		name := c.Name
		if n == 1 {
			name = c.ID
		} else if n > 1 {
			name = c.Aliases[n-2]
		}
		if name != "" && !graphPronoun(name) && strings.Contains(text, strings.ToLower(name)) {
			return true
		}
	}
	return false
}

func graphPronoun(s string) bool {
	return strings.Contains("|anh|chị|em|cậu|tớ|tôi|mình|bác|cô|chú|dì|con|cháu|mày|tao|ông|bà|", "|"+strings.ToLower(s)+"|")
}

type contextTrimItem struct {
	section, key, label      string
	index, chapter, priority int
}

// trimGraphItem removes one oldest item, never a whole collection. Scalar reference text
// is one item; maps use sorted keys for stable output. Address terms are never candidates.
func trimGraphItem(result map[string]any, order []string) (string, bool) {
	var candidates []contextTrimItem
	for _, section := range []string{"working_memory", "episodic_memory", "planning_memory", "foundation_memory", "reference_pack", "selected_memory"} {
		values, ok := result[section].(map[string]any)
		if !ok {
			continue
		}
		for priority, key := range order {
			value, ok := values[key]
			if !ok || value == nil {
				continue
			}
			v := reflect.ValueOf(value)
			switch v.Kind() {
			case reflect.Slice:
				for i := range v.Len() {
					chapter := 0
					label := fmt.Sprintf("%s.%s[%d]", section, key, i)
					raw, _ := json.Marshal(v.Index(i).Interface())
					var item map[string]any
					_ = json.Unmarshal(raw, &item)
					for _, field := range []string{"chapter", "planted_at", "from_chapter"} {
						if n, ok := item[field].(float64); ok {
							chapter = int(n)
							break
						}
					}
					if id, ok := item["id"].(string); ok {
						label = section + "." + key + "[" + id + "]"
					} else {
						label += fmt.Sprintf("@%d", chapter)
					}
					candidates = append(candidates, contextTrimItem{section, key, label, i, chapter, priority})
				}
			case reflect.Map:
				for _, k := range v.MapKeys() {
					if k.Kind() == reflect.String {
						candidates = append(candidates, contextTrimItem{section: section, key: key, label: section + "." + key + "." + k.String(), index: -1, priority: priority})
					}
				}
			default:
				candidates = append(candidates, contextTrimItem{section: section, key: key, label: section + "." + key, index: -2, priority: priority})
			}
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.chapter != b.chapter {
			return a.chapter < b.chapter
		}
		if a.priority != b.priority {
			return a.priority < b.priority
		}
		return a.label < b.label
	})
	c := candidates[0]
	values := result[c.section].(map[string]any)
	v := reflect.ValueOf(values[c.key])
	if c.index >= 0 {
		next := reflect.MakeSlice(v.Type(), 0, v.Len()-1)
		next = reflect.AppendSlice(next, v.Slice(0, c.index))
		next = reflect.AppendSlice(next, v.Slice(c.index+1, v.Len()))
		values[c.key] = next.Interface()
	} else if c.index == -1 {
		prefix := c.section + "." + c.key + "."
		v.SetMapIndex(reflect.ValueOf(strings.TrimPrefix(c.label, prefix)), reflect.Value{})
	} else {
		delete(values, c.key)
	}
	return c.label, true
}
