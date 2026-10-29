package host

import (
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/subagent"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func commentReviewStore(t *testing.T) *store.Store {
	t.Helper()
	st := store.NewStore(t.TempDir())
	for _, err := range []error{st.Init(), st.Progress.Init(3), st.Progress.UpdatePhase(domain.PhaseWriting), st.Progress.MarkChapterComplete(1, 3, "", ""), st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}, {Chapter: 3, Title: "Ba"}})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	content := "Nội dung chương một"
	if err := st.Drafts.SaveFinalChapter(1, content); err != nil {
		t.Fatal(err)
	}
	if err := st.ChapterRecords.Save(domain.ChapterRecord{Version: domain.ChapterRecordVersion, Chapter: 1, Revision: 1, Origin: domain.ChapterOriginUser, Content: content, ContentSHA256: domain.ChapterContentSHA256(content), AcceptedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCommentApprovalPersistsPermitAndDispatchBeforeSingleLaunch(t *testing.T) {
	st := commentReviewStore(t)
	e, _, done := newTestEngine(t, st, subagent.NewRunner(), nil)
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	e.refresh = func() { close(entered); <-release }
	h := &Host{store: st, engine: e, observer: e.observer, lifecycle: lifecyclePaused, events: make(chan Event, 16)}
	// No Steer/Continue call may start an engine before the /next running guard.
	if err := h.AdvanceOneChapterWithComments(1, "Sửa chương 1: thêm thoại"); err != nil {
		t.Fatalf("single approve action rejected (must not report running): %v", err)
	}
	defer func() { e.abort(); close(release); waitEngineDone(t, done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Writer instruction never reached dispatch precheck")
	}
	meta, err := st.RunMeta.Load()
	if err != nil || meta.AdvancePermitChapter != 2 || meta.PendingCommentReview == nil || meta.PendingCommentReview.Chapter != 1 {
		t.Fatalf("durable atomic approval = %+v, %v", meta, err)
	}
	if !e.isRunning() || h.sessionSeq != 1 {
		t.Fatalf("engine starts = %d running=%v", h.sessionSeq, e.isRunning())
	}
}

func TestCommentReviewRestoresWriterAndClearsOnlyMatchingDispatch(t *testing.T) {
	st := commentReviewStore(t)
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatal(err)
	}
	review := domain.CommentReview{Chapter: 1, Instruction: "Sửa chương 1: giữ giọng văn"}
	if err := st.RunMeta.GrantCommentReview(2, review); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatal(err)
	}
	e := &engine{store: st}
	inst, err := e.commentReviewInstruction()
	if err != nil || inst == nil || inst.Agent != "writer" || inst.Chapter != 1 || inst.Task != review.Instruction {
		t.Fatalf("restored Writer = %+v, %v", inst, err)
	}
	progress, _ := st.Progress.Load()
	if progress.Flow != domain.FlowRewriting || len(progress.PendingRewrites) != 1 || progress.PendingRewrites[0] != 1 {
		t.Fatalf("rewrite facts = %+v", progress)
	}
	if err := e.finishCommentReview(&flow.Instruction{Agent: "editor", Task: review.Instruction, Chapter: 1}); err != nil {
		t.Fatal(err)
	}
	meta, _ := st.RunMeta.Load()
	if meta.PendingCommentReview == nil {
		t.Fatal("unrelated dispatch cleared review")
	}
	if err := e.finishCommentReview(inst); err != nil {
		t.Fatal(err)
	}
	meta, _ = st.RunMeta.Load()
	if meta.PendingCommentReview != nil || meta.AdvancePermitChapter != 2 {
		t.Fatalf("completed review = %+v", meta)
	}
}

func TestCommentApprovalRunningGuardDoesNotPersist(t *testing.T) {
	st := commentReviewStore(t)
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatal(err)
	}
	h := &Host{store: st, engine: &engine{}, lifecycle: lifecycleRunning}
	if err := h.AdvanceOneChapterWithComments(1, "Sửa chương 1"); err == nil || !strings.Contains(err.Error(), "đang chạy") {
		t.Fatalf("guard = %v", err)
	}
	meta, _ := st.RunMeta.Load()
	if meta.PendingCommentReview != nil || meta.AdvancePermitChapter != 0 {
		t.Fatal("running rejection changed durable approval")
	}
}
