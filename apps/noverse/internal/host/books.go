package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/datnt1998/noverse/internal/store"
)

var ErrChapterNotFound = errors.New("không tìm thấy chương")

type ChapterInfo struct {
	Chapter   int    `json:"chapter"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Words     int    `json:"words"`
}

type BookInfo struct {
	Title             string
	Phase             string
	CompletedChapters int
	TotalChapters     int
	TotalWords        int
	UpdatedAt         time.Time
}

// Chapters returns outline chapters and committed chapter metadata without modifying the store.
func (h *Host) Chapters() ([]ChapterInfo, error) {
	progress, err := h.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	outline, err := h.store.Outline.LoadOutline()
	if err != nil {
		return nil, err
	}
	chapters := make([]ChapterInfo, 0, len(outline))
	for _, entry := range outline {
		ch := ChapterInfo{Chapter: entry.Chapter, Title: entry.Title}
		if progress != nil {
			ch.Completed = slices.Contains(progress.CompletedChapters, entry.Chapter)
			ch.Words = progress.ChapterWordCounts[entry.Chapter]
		}
		chapters = append(chapters, ch)
	}
	if progress != nil {
		for _, n := range progress.CompletedChapters {
			found := false
			for _, ch := range chapters {
				if ch.Chapter == n {
					found = true
					break
				}
			}
			if !found {
				chapters = append(chapters, ChapterInfo{Chapter: n, Completed: true, Words: progress.ChapterWordCounts[n]})
			}
		}
	}
	slices.SortFunc(chapters, func(a, b ChapterInfo) int { return a.Chapter - b.Chapter })
	return chapters, nil
}

func (h *Host) ChapterText(n int) (title, text string, err error) {
	progress, err := h.store.Progress.Load()
	if err != nil {
		return "", "", err
	}
	if progress == nil || n <= 0 || !slices.Contains(progress.CompletedChapters, n) {
		return "", "", fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	outline, err := h.store.Outline.GetChapterOutline(n)
	if err != nil && !errors.Is(err, store.ErrOutlineChapterNotFound) {
		return "", "", err
	}
	if outline != nil {
		title = outline.Title
	}
	text, err = h.store.Drafts.LoadChapterText(n)
	if err != nil {
		return "", "", err
	}
	if text == "" {
		return "", "", fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	return title, text, nil
}

// PeekBook reads book facts without initializing the store or acquiring its lease.
func PeekBook(dir string) (BookInfo, error) {
	return peekBook(store.NewStore(dir), dir)
}

// PeekBookStore is PeekBook on a store the caller already opened (story list đọc
// cả card từ một store, không mở lại và đọc lại checkpoints cho từng phần).
func PeekBookStore(s *store.Store) (BookInfo, error) {
	return peekBook(s, s.Dir())
}

func peekBook(s *store.Store, dir string) (BookInfo, error) {
	// Hosted stores have no engine directory; existence and timestamps come
	// from rows, so no stat is attempted against "" or the process cwd.
	fileBacked := dir != ""
	var result BookInfo
	if fileBacked {
		stat, err := os.Stat(dir)
		if errors.Is(err, os.ErrNotExist) {
			return BookInfo{}, nil
		}
		if err != nil {
			return BookInfo{}, err
		}
		if !stat.IsDir() {
			return BookInfo{}, fmt.Errorf("%q không phải thư mục truyện", filepath.Base(dir))
		}
		result.UpdatedAt = stat.ModTime()
	}
	book, err := s.Book.Load()
	if err != nil {
		return BookInfo{}, err
	}
	if book != nil {
		result.Title = book.Title
	}
	p, err := s.Progress.Load()
	if err != nil {
		return BookInfo{}, err
	}
	if p != nil {
		result.Phase = string(p.Phase)
		result.CompletedChapters = len(p.CompletedChapters)
		result.TotalChapters = p.TotalChapters
		result.TotalWords = p.TotalWordCount
	}
	if fileBacked {
		if st, err := os.Stat(filepath.Join(dir, "meta", "progress.json")); err == nil {
			result.UpdatedAt = st.ModTime()
		} else if !errors.Is(err, os.ErrNotExist) {
			return BookInfo{}, err
		}
	}
	return result, nil
}
