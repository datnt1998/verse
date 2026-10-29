package imp

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

// TestPublishChapterAcceptsAuthorTruncatedEnd 锁死作者原文豁免：导入的是
// 外部书稿，正文断在半行也是原稿事实。走 WithAuthorSource 的 commit 实例
// 必须照常发布，而不是因 truncated_end 退回作者改稿。
func TestPublishChapterAcceptsAuthorTruncatedEnd(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	commit := tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)).WithAuthorSource()
	f := ImportedChapterFacts{Chapter: 1, Title: "Chương 1", Summary: "s", CoreEvent: "c"}
	content := "Chương nhập của tác giả kết thúc giữa chừng, đúng như nguyên bản"

	if err := publishChapter(context.Background(), st, commit, 1, content, f); err != nil {
		t.Fatalf("chương tác giả thiếu dấu câu cuối vẫn phải xuất bản: %v", err)
	}
	prog, err := st.Progress.Load()
	if err != nil || prog == nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if len(prog.CompletedChapters) != 1 || prog.CompletedChapters[0] != 1 {
		t.Fatalf("chương 1 phải được đánh dấu hoàn thành: %+v", prog.CompletedChapters)
	}
}

// TestPublishChapterCommitToolBlocksTruncatedEnd 反向锚点：同一实例不带
// WithAuthorSource 时准入检查生效——豁免来自构造选项而非导入路径本身。
func TestPublishChapterCommitToolBlocksTruncatedEnd(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	commit := tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st))
	f := ImportedChapterFacts{Chapter: 1, Title: "Chương 1", Summary: "s", CoreEvent: "c"}
	content := "Chương nhập của tác giả kết thúc giữa chừng, đúng như nguyên bản"

	if err := publishChapter(context.Background(), st, commit, 1, content, f); err == nil {
		t.Fatal("không bật author-source thì truncated_end phải bị chặn")
	}
}
