package revision

import (
	"fmt"
	"slices"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

type Change struct {
	Chapter       int
	BaseSHA256    string
	CurrentSHA256 string
	Before        string
	After         string
}

func Scan(st *store.Store) ([]Change, error) {
	progress, err := st.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc tiến độ: %w", err)
	}
	if progress == nil || len(progress.CompletedChapters) == 0 {
		return nil, nil
	}
	chapters := slices.Clone(progress.CompletedChapters)
	slices.Sort(chapters)
	changes := make([]Change, 0)
	for _, chapter := range chapters {
		record, err := st.ChapterRecords.Load(chapter)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return nil, fmt.Errorf("chương %d thiếu bản ghi chấp nhận, dự án hiện tại không thể nhận diện bản sửa bên ngoài một cách an toàn", chapter)
		}
		text, err := st.Drafts.LoadChapterText(chapter)
		if err != nil {
			return nil, fmt.Errorf("đọc nội dung chương %d trong vùng làm việc: %w", chapter, err)
		}
		content := domain.NormalizeChapterContent(text)
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("nội dung chương %d trong vùng làm việc rỗng, từ chối chấp nhận", chapter)
		}
		digest := domain.ChapterContentSHA256(content)
		if digest == record.ContentSHA256 {
			continue
		}
		changes = append(changes, Change{
			Chapter: chapter, BaseSHA256: record.ContentSHA256, CurrentSHA256: digest,
			Before: record.Content, After: content,
		})
	}
	return changes, nil
}

func ChangedChapters(changes []Change) []int {
	out := make([]int, len(changes))
	for i, change := range changes {
		out[i] = change.Chapter
	}
	return out
}
