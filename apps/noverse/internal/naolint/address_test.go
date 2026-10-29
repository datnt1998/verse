package naolint

import (
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
)

func cast() []domain.Character {
	return []domain.Character{{ID: "nv:a", Name: "Kỷ Dương"}, {ID: "nv:b", Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}}}
}
func table() []domain.AddressTerm {
	return []domain.AddressTerm{{From: "nv:a", To: "nv:b", Calls: "chị", Self: "em", FromChapter: 1}}
}
func TestAddressCertainAndInferred(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		severity   rules.Severity
	}{{"certain", "Kỷ Dương nói: “Lâm Uyển, tôi nhờ chị giúp.”", rules.SeverityError}, {"certain_after", "“Lâm Uyển, tôi nhờ chị giúp,” Kỷ Dương nói.", rules.SeverityError}, {"inferred", "Kỷ Dương nhìn xuống.\n\n“Lâm Uyển, tôi nhờ chị giúp.”", rules.SeverityWarning}, {"paragraph", "Kỷ Dương mỉm cười: “Lâm Uyển, tôi nhờ chị giúp.”", rules.SeverityWarning}} {
		t.Run(tc.name, func(t *testing.T) {
			got := Address(1, tc.text, cast(), table())
			if len(got) != 1 || got[0].Severity != tc.severity || got[0].Actual != "tôi" {
				t.Fatalf("findings=%+v", got)
			}
		})
	}
}
func TestAddressPrecisionAndIntervals(t *testing.T) {
	for _, text := range []string{"Kỷ Dương nói: “Lâm Uyển, em nhờ chị giúp.”", "Kỷ Dương nói: “Lâm Uyển, anh ấy đang đợi cô ta.”", "Kỷ Dương nói: “Có người bảo tôi như thế.”", "“Tôi nhờ chị giúp.”"} {
		if got := Address(1, text, cast(), table()); len(got) != 0 {
			t.Fatalf("%s: %+v", text, got)
		}
	}
	terms := table()
	terms[0].UntilChapter = 1
	if got := Address(2, "Kỷ Dương nói: “Lâm Uyển, tôi nhờ chị giúp.”", cast(), terms); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestContradictions(t *testing.T) {
	terms := append(table(), domain.AddressTerm{From: "nv:a", To: "nv:b", Calls: "cô", Self: "tôi", FromChapter: 2})
	relations := []domain.RelationshipChange{{Chapter: 1, From: "nv:a", To: "nv:b", Relation: "bạn"}, {Chapter: 2, From: "nv:a", To: "nv:b", Relation: "thù"}}
	changes := []domain.StateChange{{Chapter: 1, Entity: "Kỷ Dương", Field: "status", NewValue: "dead"}}
	ledger := []domain.ForeshadowEntry{{ID: "f", PlantedAt: 3, ResolvedAt: 2}}
	events := []domain.TimelineEvent{{Chapter: 2, Characters: []string{"Kỷ Dương"}}}
	got := Contradictions(2, cast(), terms, relations, changes, ledger, events)
	if len(got) != 4 {
		t.Fatalf("findings=%+v", got)
	}
	terms[1].Reason = "cãi nhau"
	changes = append(changes, domain.StateChange{Chapter: 2, Entity: "Kỷ Dương", Field: "relation", Reason: "cãi nhau"})
	if got := Contradictions(2, cast(), terms, relations, changes, nil, nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestAddressDoesNotInventSceneOrCheckTitles(t *testing.T) {
	chars := append(cast(), domain.Character{ID: "nv:c", Name: "Người mới"})
	text := "Kỷ Dương nhìn xuống. “Tôi nhờ chị giúp.”\n\nNgười mới bước vào."
	if got := Address(1, text, chars, table()); len(got) != 0 {
		t.Fatalf("new scene was treated as addressee: %+v", got)
	}
	terms := table()
	terms[0].Self = "tôi"
	text = "Kỷ Dương nói: “Lâm Uyển, tôi không đề nghị giúp cho cô Tống.”"
	if got := Address(1, text, cast(), terms); len(got) != 0 {
		t.Fatalf("third-person title treated as address: %+v", got)
	}
}
