package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestWordDelta(t *testing.T) {
	cases := []struct {
		name           string
		before, after  string
		added, removed int
	}{
		{"unchanged", "Trời mưa to.", "Trời mưa to.", 0, 0},
		{"append sentence", "Trời mưa.", "Trời mưa. Cô ấy chạy về nhà.", 5, 0},
		{"replace word", "Anh ta cười lớn.", "Anh ta khóc lớn.", 1, 1},
		{"delete paragraph", "Một hai ba.\n\nBốn năm.", "Một hai ba.", 0, 2},
		{"punctuation only tokens ignored", "A — B", "A … B ***", 0, 0},
		{"repeated word counts multiplicity", "đi", "đi đi đi", 2, 0},
	}
	for _, c := range cases {
		added, removed := WordDelta(c.before, c.after)
		if added != c.added || removed != c.removed {
			t.Errorf("%s: got +%d −%d want +%d −%d", c.name, added, removed, c.added, c.removed)
		}
	}
}

// GAP-29：/sync --check trả {chapter,title,kind,added,removed,modifiedAt} mà không ghi chương.
func TestCheckRevisionChangesReportsWordDeltaAndKind(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(1); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu", CoreEvent: "e"}}); err != nil {
		t.Fatal(err)
	}
	original := "Trời mưa. Cô ấy chạy về nhà."
	if err := st.Drafts.SaveFinalChapter(1, original); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, original, domain.ChapterFacts{Title: "Mở đầu", Summary: "s", KeyEvents: []string{"k"}}, domain.StyleDelta{}); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.StartChapter(1); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.MarkChapterComplete(1, domain.WordCount(original), "", ""); err != nil {
		t.Fatal(err)
	}
	h := &Host{store: st}

	if changes, err := h.CheckRevisionChanges(); err != nil || len(changes) != 0 {
		t.Fatalf("untouched story: changes=%v err=%v", changes, err)
	}

	path := filepath.Join(st.Dir(), "chapters", "01.md")
	edited := "Trời mưa. Cô ấy chạy về nhà thật nhanh."
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := h.CheckRevisionChanges()
	if err != nil || len(changes) != 1 {
		t.Fatalf("edited chapter: changes=%v err=%v", changes, err)
	}
	c := changes[0]
	if c.Chapter != 1 || c.Title != "Mở đầu" || c.Kind != "edit" || c.Added != 3 || c.Removed != 1 {
		t.Fatalf("change = %+v", c)
	}
	info, _ := os.Stat(path)
	if c.ModifiedAt == nil || !c.ModifiedAt.Equal(info.ModTime()) {
		t.Fatalf("modifiedAt = %v, want %v", c.ModifiedAt, info.ModTime())
	}
	if raw, _ := os.ReadFile(path); string(raw) != edited {
		t.Fatal("check must not rewrite the chapter")
	}
	diff, err := h.RevisionDiffFor(1)
	if err != nil || diff.Before != original || diff.After != edited || diff.Title != "Mở đầu" {
		t.Fatalf("diff = %+v err=%v", diff, err)
	}
	if _, err := h.RevisionDiffFor(2); !errors.Is(err, ErrNoRevision) {
		t.Fatalf("chapter without revision: %v", err)
	}

	// Có bản ghi khôi phục dở → kind "resume" cho đúng các chương trong bản ghi.
	record, err := st.ChapterRecords.Load(1)
	if err != nil || record == nil {
		t.Fatalf("record: %v %v", record, err)
	}
	pending := domain.PendingRevision{Items: []domain.PendingRevisionItem{{Chapter: 1, Record: *record}}}
	if err := st.Revisions.SavePending(pending); err != nil {
		t.Fatal(err)
	}
	changes, err = h.CheckRevisionChanges()
	if err != nil || len(changes) != 1 || changes[0].Kind != "resume" || changes[0].Added != 3 || changes[0].Removed != 1 {
		t.Fatalf("pending resume: changes=%+v err=%v", changes, err)
	}
}
