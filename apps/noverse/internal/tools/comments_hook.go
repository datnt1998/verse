package tools

import (
	"fmt"
	"log/slog"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// afterChapterWrite chạy vòng đời góp ý §AI sau mỗi lần một chapter text được
// ghi: id writer báo đã sửa (fixed_comments), gắn lại mọi mốc neo trên nội
// dung mới, rồi xếp sự kiện comments.reanchored để gateway phát.
//
// The comment store comes from the backend (file directory or scoped rows), so
// this never derives a directory from Dir() — hosted runs have "". A backend
// without a comment surface is a capability bug, reported and skipped: comments
// never block the chapter write path.
func afterChapterWrite(s *store.Store, chapter int, content string, fixedCommentIDs []string) {
	cs := s.Comments()
	if cs == nil {
		slog.Error("backend không cung cấp kho góp ý; bỏ qua gắn lại mốc neo",
			"module", "tools", "chapter", chapter, "backend", s.BackendKind())
		return
	}
	if _, err := cs.AfterWriteAt(chapter, content, fixedCommentIDs); err != nil {
		slog.Warn("gắn lại góp ý sau khi ghi chương thất bại", "module", "tools", "chapter", chapter, "err", err)
	}
}

// chapterRef returns the human-readable location of a chapter's draft for tool
// results. It names a document, not a host path.
func chapterRef(chapter int) string { return fmt.Sprintf("drafts/%02d.draft.md", chapter) }
