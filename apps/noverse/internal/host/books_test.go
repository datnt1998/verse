package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func TestPeekBookDoesNotCreateDirectory(t *testing.T) {
	root := t.TempDir()
	absent := filepath.Join(root, "missing")
	info, err := PeekBook(absent)
	if err != nil || info != (BookInfo{}) {
		t.Fatalf("missing book: %+v %v", info, err)
	}
	if _, err = os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("peek created directory: %v", err)
	}
	data := filepath.Join(root, "existing")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := PeekBook(data); err != nil {
		t.Fatal(err)
	}
	children, err := os.ReadDir(data)
	if err != nil || len(children) != 0 {
		t.Fatalf("peek created files: %v %v", children, err)
	}
}

func TestChapterTextIncompleteIsNotFound(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Progress.Save(&domain.Progress{CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	h := &Host{store: s}
	if _, _, err := h.ChapterText(2); !errors.Is(err, ErrChapterNotFound) {
		t.Fatalf("incomplete chapter: %v", err)
	}
}
