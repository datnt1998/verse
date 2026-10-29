package naolint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
	"github.com/datnt1998/noverse/internal/store"
)

// Contradictions checks only explicit facts, never infers death from prose. Relationship
// history has no reason field in W1: a matching relation state-change supplies that reason.
func Contradictions(chapter int, chars []domain.Character, terms []domain.AddressTerm, relations []domain.RelationshipChange, changes []domain.StateChange, ledger []domain.ForeshadowEntry, events []domain.TimelineEvent) []Finding {
	var out []Finding
	add := func(rule, target, message string) {
		out = append(out, Finding{Violation: rules.Violation{Rule: rule, Target: target, Actual: 1, Severity: rules.SeverityError, Message: "[LỖI] " + message}, Chapter: chapter})
	}
	terms = append([]domain.AddressTerm(nil), terms...)
	sort.SliceStable(terms, func(i, j int) bool { return terms[i].FromChapter < terms[j].FromChapter })
	last := map[string]domain.AddressTerm{}
	for _, t := range terms {
		key := t.From + "\x00" + t.To
		if old, ok := last[key]; ok && t.FromChapter == chapter && (old.Calls != t.Calls || old.Self != t.Self) && strings.TrimSpace(t.Reason) == "" {
			add("address_change_reason", t.From+" → "+t.To, fmt.Sprintf("chương %d đổi xưng hô %s → %s nhưng thiếu reason", chapter, t.From, t.To))
		}
		last[key] = t
	}
	relations = append([]domain.RelationshipChange(nil), relations...)
	sort.SliceStable(relations, func(i, j int) bool { return relations[i].Chapter < relations[j].Chapter })
	previous := map[string]string{}
	for _, r := range relations {
		a, b := r.From, r.To
		if a > b {
			a, b = b, a
		}
		key := a + "\x00" + b
		if old, ok := previous[key]; ok && old != r.Relation && r.Chapter == chapter {
			reason := false
			for _, c := range changes {
				if c.Chapter != chapter || (c.Field != "relation" && c.Field != "relationship" && c.Field != "quan_he") || strings.TrimSpace(c.Reason) == "" {
					continue
				}
				for _, ch := range chars {
					if ch.ID == r.From || ch.ID == r.To {
						if c.Entity == ch.ID || c.Entity == ch.Name {
							reason = true
						}
					}
				}
			}
			if !reason {
				add("relationship_change_reason", r.From+" ↔ "+r.To, fmt.Sprintf("chương %d đổi quan hệ nhưng không có lý do trong state_changes (W1 không lưu reason trong relationship_log)", chapter))
			}
		}
		previous[key] = r.Relation
	}
	for _, f := range ledger {
		if f.ResolvedAt == chapter && f.ResolvedAt < f.PlantedAt {
			add("foreshadow_order", f.ID, fmt.Sprintf("phục bút %s được giải chương %d trước khi cài chương %d", f.ID, f.ResolvedAt, f.PlantedAt))
		}
	}
	statuses := map[string]domain.StateChange{}
	for _, c := range changes {
		if c.Chapter < chapter && (c.Field == "status" || c.Field == "trang_thai") {
			if old, ok := statuses[c.Entity]; !ok || c.Chapter >= old.Chapter {
				statuses[c.Entity] = c
			}
		}
	}
	for _, c := range chars {
		state := statuses[c.ID]
		if named := statuses[c.Name]; named.Chapter > state.Chapter {
			state = named
		}
		status := strings.ToLower(strings.TrimSpace(state.NewValue))
		if status != "dead" && status != "deceased" && status != "đã chết" && status != "chết" && status != "tử vong" {
			continue
		}
		for _, e := range events {
			if e.Chapter != chapter {
				continue
			}
			for _, name := range e.Characters {
				if name == c.ID || name == c.Name {
					add("dead_character", c.Name, fmt.Sprintf("chương %d ghi nhân vật %s xuất hiện sau trạng thái đã chết; cần kiểm tra hồi tưởng", chapter, c.Name))
				}
			}
		}
	}
	return out
}

// Chapter rechecks a draft/final text against persisted W1 facts. Read errors are explicit.
func Chapter(s *store.Store, chapter int, text string) ([]Finding, error) {
	chars, err := s.Characters.Load()
	if err != nil {
		return nil, err
	}
	terms, err := s.World.LoadAddressTerms()
	if err != nil {
		return nil, err
	}
	relations, err := s.World.LoadRelationshipLog()
	if err != nil {
		return nil, err
	}
	changes, err := s.World.LoadStateChanges()
	if err != nil {
		return nil, err
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil {
		return nil, err
	}
	events, err := s.World.LoadTimeline()
	if err != nil {
		return nil, err
	}
	findings := Address(chapter, text, chars, terms)
	return append(findings, Contradictions(chapter, chars, terms, relations, changes, ledger, events)...), nil
}

// All rechecks every committed chapter in progress, with stable chapter/line order.
func All(s *store.Store) ([]Finding, error) {
	progress, err := s.Progress.Load()
	if err != nil {
		return nil, err
	}
	if progress == nil {
		return nil, nil
	}
	chapters := append([]int(nil), progress.CompletedChapters...)
	sort.Ints(chapters)
	var out []Finding
	for _, chapter := range chapters {
		text, err := s.Drafts.LoadChapterText(chapter)
		if err != nil {
			return nil, err
		}
		findings, err := Chapter(s, chapter, text)
		if err != nil {
			return nil, err
		}
		out = append(out, findings...)
	}
	return out, nil
}
