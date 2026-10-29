package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func naoToolStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := s.Characters.Save([]domain.Character{{Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}}, {Name: "Kỷ Dương", Aliases: []string{"Dương"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Drafts.SaveDraft(1, "Kỷ Dương nói: Chị chờ em nhé. Lâm Uyển gật đầu."); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNaoCommitAddressAndRelationshipNames(t *testing.T) {
	s := naoToolStore(t)
	args := json.RawMessage(`{"chapter":1,"title":"Gặp gỡ","summary":"Hai người gặp nhau","characters":["Lâm Uyển","Kỷ Dương"],"key_events":["Hợp tác"],"relationship_changes":[{"character_a":"Uyển Uyển","character_b":"Dương","relation":"đồng nghiệp","type":"ally"}],"address_terms":[{"from":"DƯƠNG","to":"Uyển Uyển","calls":"chị","self":"em"}]}`)
	if _, err := newTestCommitChapterTool(s).Execute(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	terms, err := s.World.AddressAt(1)
	if err != nil || len(terms) != 1 || terms[0].From != "nv:ky-duong" || terms[0].To != "nv:lam-uyen" || terms[0].Calls != "chị" {
		t.Fatalf("committed address terms = %+v, %v", terms, err)
	}
	log, err := s.World.LoadRelationshipLog()
	if err != nil || len(log) != 1 || log[0].From != "nv:lam-uyen" || log[0].To != "nv:ky-duong" || log[0].Chapter != 1 {
		t.Fatalf("committed relationship history = %+v, %v", log, err)
	}
	if log[0].Type != "ally" {
		t.Fatalf("committed relationship type = %q, want ally", log[0].Type)
	}
}

func TestNaoCommitRejectsUnknownRelationshipType(t *testing.T) {
	s := naoToolStore(t)
	args := json.RawMessage(`{"chapter":1,"title":"Gặp gỡ","summary":"Hai người gặp nhau","characters":["Lâm Uyển","Kỷ Dương"],"key_events":["Hợp tác"],"relationship_changes":[{"character_a":"Lâm Uyển","character_b":"Kỷ Dương","relation":"đồng nghiệp","type":"bogus"}]}`)
	_, err := newTestCommitChapterTool(s).Execute(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "type invalid") {
		t.Fatalf("unknown relationship type = %v", err)
	}
	if pending, err := s.Signals.LoadPendingCommit(); err != nil || pending != nil {
		t.Fatalf("invalid facts must not freeze a pending commit: %+v, %v", pending, err)
	}
}

func TestNaoCommitRejectsBadNamesAndReasonBeforeFreeze(t *testing.T) {
	for _, unknown := range []bool{true, false} {
		t.Run(map[bool]string{true: "unknown_name", false: "missing_reason"}[unknown], func(t *testing.T) {
			s := naoToolStore(t)
			from := "Dương"
			want := "phải điền reason"
			if unknown {
				from = "Không có"
				want = "Tên đã biết: Lâm Uyển, Kỷ Dương"
			} else if err := s.World.UpdateAddressTerms(1, []domain.AddressTermInput{{From: "Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}); err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]any{
				"chapter": 1, "title": "Gặp gỡ", "summary": "Hai người gặp nhau", "key_events": []string{"Hợp tác"},
				"address_terms": []domain.AddressTermInput{{From: from, To: "Lâm Uyển", Calls: "cô", Self: "tôi"}},
			})
			_, err := newTestCommitChapterTool(s).Execute(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("actionable rejection = %v, want %q", err, want)
			}
			if pending, err := s.Signals.LoadPendingCommit(); err != nil || pending != nil {
				t.Fatalf("invalid facts must not freeze a pending commit: %+v, %v", pending, err)
			}
		})
	}
}

func TestNaoFoundationInitialAddressTerms(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(`{"type":"characters","axes":null,"content":[{"name":"Lâm Uyển","aliases":["Uyển Uyển"]},{"name":"Kỷ Dương"}],"address_terms":[{"from":"Kỷ Dương","to":"Uyển Uyển","calls":"chị","self":"em"}]}`)
	if _, err := NewSaveFoundationTool(s).Execute(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	terms, err := s.World.AddressAt(1)
	if err != nil || len(terms) != 1 || terms[0].FromChapter != 1 || terms[0].From != "nv:ky-duong" || terms[0].To != "nv:lam-uyen" {
		t.Fatalf("foundation terms = %+v, %v", terms, err)
	}
}
