package store

import (
	"fmt"
	"os"

	"github.com/datnt1998/noverse/internal/domain"
)

// BookStore 管理作品对外信息，meta/book.json 是唯一事实源，book.md 是可读投影。
type fileBookStore struct{ io *IO }

func newFileBookStore(io *IO) *fileBookStore { return &fileBookStore{io: io} }

// Load 读取作品信息；尚未生成时返回 nil。
func (s *fileBookStore) Load() (*domain.BookMetadata, error) {
	var book domain.BookMetadata
	if err := s.io.ReadJSON("meta/book.json", &book); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	book = book.Normalized()
	if err := book.Validate(); err != nil {
		return nil, err
	}
	return &book, nil
}

// Save 保存规范化的作品信息及其可读投影。
func (s *fileBookStore) Save(book domain.BookMetadata) error {
	book = book.Normalized()
	if err := book.Validate(); err != nil {
		return err
	}
	return s.io.WithWriteLock(func() error {
		if err := s.io.WriteJSONUnlocked("meta/book.json", book); err != nil {
			return err
		}
		return s.io.WriteMarkdownUnlocked("book.md", renderBook(book, s.io.labels()))
	})
}

func renderBook(book domain.BookMetadata, l mdLabels) string {
	return fmt.Sprintf("# "+l.bookTitleFmt+"\n\n## %s\n\n%s\n", book.Title, l.synopsis, book.Synopsis)
}
