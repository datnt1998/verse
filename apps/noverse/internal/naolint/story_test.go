package naolint

import (
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func TestAllRechecksCommittedChapters(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Characters.Save(cast()); err != nil {
		t.Fatal(err)
	}
	if err := s.World.UpdateAddressTerms(1, []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{CompletedChapters: []int{2, 1}}); err != nil {
		t.Fatal(err)
	}
	for _, chapter := range []int{1, 2} {
		if err := s.Drafts.SaveFinalChapter(chapter, "Kỷ Dương nói: “Lâm Uyển, tôi nhờ chị giúp.”"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := All(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Chapter != 1 || got[1].Chapter != 2 || got[0].Line != 1 {
		t.Fatalf("findings=%+v", got)
	}
}
