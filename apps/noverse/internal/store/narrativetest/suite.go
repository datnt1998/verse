// Package narrativetest holds the cross-backend story-domain contract suite
// (phase 05): the same assertions run on the file backend (internal/store
// substores + comments.Store file repo) and on the Postgres repositories
// (internal/persistence/postgres). Ports are narrow — each is the subset of a
// phase-02 contract the domain tests actually exercise; file substores
// satisfy them structurally.
//
// File-only semantics (corrupt vs absent bytes, JSONL tail recovery, format
// version files) stay in internal/store/contract_test.go; everything here is
// backend-agnostic behavior: round-trips, immutable accept history,
// idempotent appends, comments lifecycle/stale anchors, review rounds.
package narrativetest

import (
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// BookPort — meta/book.json authority.
type BookPort interface {
	Load() (*domain.BookMetadata, error)
	Save(book domain.BookMetadata) error
}

// ProgressPort — meta/progress.json.
type ProgressPort interface {
	Load() (*domain.Progress, error)
	Save(p *domain.Progress) error
}

// OutlinePort — flat and layered outlines.
type OutlinePort interface {
	SaveOutline(entries []domain.OutlineEntry) error
	LoadOutline() ([]domain.OutlineEntry, error)
	SaveLayeredOutline(volumes []domain.VolumeOutline) error
	LoadLayeredOutline() ([]domain.VolumeOutline, error)
	ClearLayeredOutline() error
}

// DraftPort — chapter drafts.
type DraftPort interface {
	SaveDraft(chapter int, content string) error
	LoadDraft(chapter int) (string, error)
}

// RecordsPort — accepted chapter records. Accept must be idempotent on
// (content hash, origin, facts, style) and append immutable history.
type RecordsPort interface {
	Load(chapter int) (*domain.ChapterRecord, error)
	Accept(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error)
}

// CharsPort — whole-set character save/load.
type CharsPort interface {
	Save(chars []domain.Character) error
	Load() ([]domain.Character, error)
}

// CheckpointPort — append-only checkpoints with same-input idempotence.
type CheckpointPort interface {
	Append(scope domain.Scope, step, artifact, digest string) (*domain.Checkpoint, error)
	All() []domain.Checkpoint
}

// DecisionPort — append-only decision ledger, newest first.
type DecisionPort interface {
	Append(rec store.DecisionRecord) (store.DecisionRecord, error)
	Recent(n int) ([]store.DecisionRecord, error)
}

// RevisionPort — pending revision marker.
type RevisionPort interface {
	LoadPending() (*domain.PendingRevision, error)
	SavePending(pending domain.PendingRevision) error
	ClearPending() error
}

// ReviewPort — reviews with round archive and kept decisions.
type ReviewPort interface {
	SaveReview(r domain.ReviewEntry) error
	LoadReview(chapter int, scope store.ReviewScope) (*domain.ReviewEntry, error)
	LoadReviewRounds(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error)
	SaveReviewDecision(itemID string, d store.ReviewDecision) error
	LoadReviewDecisions() (map[string]store.ReviewDecision, error)
	DeleteReviewDecision(itemID string) (bool, error)
}

// Harness là toàn bộ port một backend phải cấp cho một story scope để chạy
// contract suite. Comments là *comments.Store: facade chung trên cả hai
// backend, nghiệp vụ không nhân đôi.
type Harness struct {
	Book        BookPort
	Progress    ProgressPort
	Outline     OutlinePort
	Drafts      DraftPort
	Records     RecordsPort
	Characters  CharsPort
	Checkpoints CheckpointPort
	Decisions   DecisionPort
	Revisions   RevisionPort
	Reviews     ReviewPort
	Comments    *comments.Store
}

// Run executes the full contract suite against one backend harness.
func Run(t *testing.T, h Harness) {
	t.Run("book_load_save_roundtrip", func(t *testing.T) {
		if book, err := h.Book.Load(); err != nil || book != nil {
			t.Fatalf("absent book must be (nil,nil): %v %+v", err, book)
		}
		want := domain.BookMetadata{Title: "Đêm Trắng", Synopsis: "Một đêm ở thư viện."}
		if err := h.Book.Save(want); err != nil {
			t.Fatal(err)
		}
		got, err := h.Book.Load()
		if err != nil || got == nil {
			t.Fatalf("saved book must load: %v %+v", err, got)
		}
		if *got != want.Normalized() {
			t.Fatalf("book roundtrip: got %+v want %+v", *got, want)
		}
	})

	t.Run("progress_roundtrip", func(t *testing.T) {
		if p, err := h.Progress.Load(); err != nil || p != nil {
			t.Fatalf("absent progress must be (nil,nil): %v %+v", err, p)
		}
		want := &domain.Progress{
			Phase:             domain.PhaseWriting,
			CurrentChapter:    2,
			TotalChapters:     12,
			CompletedChapters: []int{1},
		}
		if err := h.Progress.Save(want); err != nil {
			t.Fatal(err)
		}
		got, err := h.Progress.Load()
		if err != nil || got == nil {
			t.Fatalf("saved progress must load: %v %+v", err, got)
		}
		if got.Phase != want.Phase || got.CurrentChapter != 2 || got.TotalChapters != 12 ||
			len(got.CompletedChapters) != 1 || got.CompletedChapters[0] != 1 {
			t.Fatalf("progress roundtrip: %+v", got)
		}
	})

	t.Run("flat_outline_roundtrip", func(t *testing.T) {
		entries := []domain.OutlineEntry{
			{Chapter: 1, Title: "Mở đầu", CoreEvent: "vào thế giới"},
			{Chapter: 2, Title: "Đối đầu", Hook: "bí mật lộ"},
		}
		if err := h.Outline.SaveOutline(entries); err != nil {
			t.Fatal(err)
		}
		got, err := h.Outline.LoadOutline()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, entries) {
			t.Fatalf("flat outline: got %+v want %+v", got, entries)
		}
	})

	t.Run("layered_outline_roundtrip_and_clear", func(t *testing.T) {
		volumes := []domain.VolumeOutline{{
			Index: 1, Title: "Tập một", Theme: "khởi nguyên",
			Arcs: []domain.ArcOutline{{
				Index: 1, Title: "Arc một", Goal: "gặp gỡ",
				Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu"}},
			}},
		}}
		if err := h.Outline.SaveLayeredOutline(volumes); err != nil {
			t.Fatal(err)
		}
		got, err := h.Outline.LoadLayeredOutline()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, volumes) {
			t.Fatalf("layered outline: got %+v want %+v", got, volumes)
		}
		if err := h.Outline.ClearLayeredOutline(); err != nil {
			t.Fatal(err)
		}
		if got, err := h.Outline.LoadLayeredOutline(); err != nil || len(got) != 0 {
			t.Fatalf("cleared layered outline must be empty: %v %+v", err, got)
		}
	})

	t.Run("characters_replace_roundtrip", func(t *testing.T) {
		chars := []domain.Character{
			{ID: "linh", Name: "Linh", Role: "nữ chính", Kind: domain.CharacterKindLead},
			{ID: "bao", Name: "Bảo", Role: "phản diện", Kind: domain.CharacterKindAntagonist},
		}
		if err := h.Characters.Save(chars); err != nil {
			t.Fatal(err)
		}
		got, err := h.Characters.Load()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("characters: %d", len(got))
		}
		byID := map[string]domain.Character{}
		for _, c := range got {
			byID[c.ID] = c
		}
		if byID["linh"].Name != "Linh" || byID["bao"].Kind != domain.CharacterKindAntagonist {
			t.Fatalf("characters roundtrip: %+v", got)
		}
		// Replace semantics: saving a different set must not merge with the old.
		if err := h.Characters.Save(chars[:1]); err != nil {
			t.Fatal(err)
		}
		got, err = h.Characters.Load()
		if err != nil || len(got) != 1 {
			t.Fatalf("replace must drop removed characters: %v %+v", err, got)
		}
	})

	t.Run("draft_save_load", func(t *testing.T) {
		if text, err := h.Drafts.LoadDraft(3); err != nil || text != "" {
			t.Fatalf("absent draft must be empty: %q %v", text, err)
		}
		content := "Đoạn đầu.\n\nĐoạn hai."
		if err := h.Drafts.SaveDraft(3, content); err != nil {
			t.Fatal(err)
		}
		got, err := h.Drafts.LoadDraft(3)
		if err != nil || got != content {
			t.Fatalf("draft roundtrip: %q %v", got, err)
		}
	})

	t.Run("accept_immutable_revisions", func(t *testing.T) {
		facts := domain.ChapterFacts{Title: "Đêm một"}
		r1, err := h.Records.Accept(1, domain.ChapterOriginGenerated, "Nội dung v1.", facts, domain.StyleDelta{})
		if err != nil {
			t.Fatal(err)
		}
		if r1.Revision != 1 || r1.ContentSHA256 != domain.ChapterContentSHA256("Nội dung v1.") {
			t.Fatalf("first accept: %+v", r1)
		}
		// Same input must not append a duplicate revision.
		r1b, err := h.Records.Accept(1, domain.ChapterOriginGenerated, "Nội dung v1.", facts, domain.StyleDelta{})
		if err != nil || r1b.Revision != 1 {
			t.Fatalf("idempotent accept must keep revision 1: %+v %v", r1b, err)
		}
		r2, err := h.Records.Accept(1, domain.ChapterOriginUser, "Nội dung v2.", facts, domain.StyleDelta{})
		if err != nil || r2.Revision != 2 {
			t.Fatalf("second accept must bump revision: %+v %v", r2, err)
		}
		cur, err := h.Records.Load(1)
		if err != nil || cur == nil || cur.Revision != 2 || cur.ContentSHA256 != domain.ChapterContentSHA256("Nội dung v2.") {
			t.Fatalf("current record must be revision 2: %+v %v", cur, err)
		}
		if old, err := h.Records.Load(7); err != nil || old != nil {
			t.Fatalf("absent record must be (nil,nil): %v %+v", err, old)
		}
	})

	t.Run("checkpoints_idempotent", func(t *testing.T) {
		sc := domain.ChapterScope(1)
		c1, err := h.Checkpoints.Append(sc, "draft", "drafts/01.md", "d1")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := h.Checkpoints.Append(sc, "write", "chapters/01.md", "d2")
		if err != nil {
			t.Fatal(err)
		}
		if !(c1.Seq < c2.Seq) {
			t.Fatalf("checkpoint seq must ascend: %d %d", c1.Seq, c2.Seq)
		}
		dup, err := h.Checkpoints.Append(sc, "draft", "drafts/01.md", "d1")
		if err != nil || dup.Seq != c1.Seq {
			t.Fatalf("idempotent append must return seq %d: %+v %v", c1.Seq, dup, err)
		}
		if all := h.Checkpoints.All(); len(all) != 2 {
			t.Fatalf("idempotent append must not grow the log: %d", len(all))
		}
	})

	t.Run("decisions_append_order", func(t *testing.T) {
		r1, err := h.Decisions.Append(store.DecisionRecord{Kind: "intervention", Decider: "arbiter", Reason: "lý do một"})
		if err != nil || r1.ID == "" {
			t.Fatalf("append must stamp an id: %+v %v", r1, err)
		}
		if _, err := h.Decisions.Append(store.DecisionRecord{Kind: "plan_start", Decider: "architect"}); err != nil {
			t.Fatal(err)
		}
		recs, err := h.Decisions.Recent(0)
		if err != nil || len(recs) != 2 {
			t.Fatalf("two decisions: %v %d", err, len(recs))
		}
		if recs[0].Kind != "intervention" || recs[1].Kind != "plan_start" {
			t.Fatalf("order must be oldest first: %+v", recs)
		}
	})

	t.Run("pending_revision_lifecycle", func(t *testing.T) {
		if p, err := h.Revisions.LoadPending(); err != nil || p != nil {
			t.Fatalf("absent pending must be (nil,nil): %v %+v", err, p)
		}
		pending := domain.PendingRevision{
			Stage: domain.RevisionStagePrepared,
			Items: []domain.PendingRevisionItem{{Chapter: 1}},
		}
		if err := h.Revisions.SavePending(pending); err != nil {
			t.Fatal(err)
		}
		got, err := h.Revisions.LoadPending()
		if err != nil || got == nil || len(got.Items) != 1 {
			t.Fatalf("pending roundtrip: %v %+v", err, got)
		}
		if err := h.Revisions.ClearPending(); err != nil {
			t.Fatal(err)
		}
		if p, err := h.Revisions.LoadPending(); err != nil || p != nil {
			t.Fatalf("cleared pending must be (nil,nil): %v %+v", err, p)
		}
	})

	t.Run("comments_lifecycle", func(t *testing.T) {
		c, err := h.Comments.Add(1, comments.Input{Text: "Viết lại mượt hơn"}, "dat")
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != comments.StatusPending || c.ID == "" {
			t.Fatalf("new comment: %+v", c)
		}
		list, err := h.Comments.List(1)
		if err != nil || len(list) != 1 {
			t.Fatalf("list: %v %d", err, len(list))
		}
		done, err := h.Comments.Patch(1, c.ID, comments.PatchInput{Action: comments.ActionResolve})
		if err != nil || done.Status != comments.StatusDone {
			t.Fatalf("resolve: %+v %v", done, err)
		}
		reopened, err := h.Comments.Patch(1, c.ID, comments.PatchInput{Action: comments.ActionReopen})
		if err != nil || reopened.Status != comments.StatusPending || !reopened.Unfixed {
			t.Fatalf("reopen must mark unfixed pending: %+v %v", reopened, err)
		}
		if err := h.Comments.Delete(1, c.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.Comments.Delete(1, c.ID); err == nil {
			t.Fatal("deleting a missing comment must error")
		}
	})

	t.Run("comments_reanchor_after_write", func(t *testing.T) {
		// A sent comment anchored on old text must move or go lost when the
		// writer rewrites the chapter — the reanchor event reports it.
		text := "Anh mở cửa bước vào phòng.\n\nCô ấy ngồi yên ở cửa sổ."
		ev, err := h.Comments.OnChapterWritten(2, text)
		if err != nil {
			t.Fatal(err)
		}
		c, err := h.Comments.Add(2, comments.Input{Text: "câu này hơi gượng", Paragraph: new(0), Quote: "Anh mở cửa bước vào phòng."}, "dat")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.Comments.Apply(2, func(string) error { return nil }); err != nil {
			t.Fatal(err)
		}
		_ = c
		// Rewrite removes the quoted sentence entirely.
		ev, err = h.Comments.OnChapterWritten(2, "Cô ấy ngồi yên ở cửa sổ.\n\nGió thổi qua kẽ lá.")
		if err != nil {
			t.Fatal(err)
		}
		list, err := h.Comments.List(2)
		if err != nil || len(list) != 1 {
			t.Fatalf("list after rewrite: %v %d", err, len(list))
		}
		st := list[0].Status
		if st != comments.StatusLost && st != comments.StatusUnsure && st != comments.StatusFixed && st != comments.StatusSent {
			t.Fatalf("stale anchor must resolve to a tracked status, got %q (event %+v)", st, ev)
		}
		events, err := h.Comments.DrainEvents()
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			t.Fatal("reanchor must queue an SSE event")
		}
	})

	t.Run("review_rounds_archive", func(t *testing.T) {
		r1 := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Summary: "vòng 1"}
		if err := h.Reviews.SaveReview(r1); err != nil {
			t.Fatal(err)
		}
		got, err := h.Reviews.LoadReview(1, store.ReviewScopeChapter)
		if err != nil || got == nil || got.Summary != "vòng 1" {
			t.Fatalf("load: %+v %v", got, err)
		}
		// Saving the identical review must be idempotent (no archived round).
		if err := h.Reviews.SaveReview(r1); err != nil {
			t.Fatal(err)
		}
		r2 := r1
		r2.Summary = "vòng 2"
		r2.Verdict = "polish"
		if err := h.Reviews.SaveReview(r2); err != nil {
			t.Fatal(err)
		}
		got, err = h.Reviews.LoadReview(1, store.ReviewScopeChapter)
		if err != nil || got.Summary != "vòng 2" {
			t.Fatalf("latest must be round 2: %+v %v", got, err)
		}
		rounds, err := h.Reviews.LoadReviewRounds(1, store.ReviewScopeChapter)
		if err != nil {
			t.Fatal(err)
		}
		if len(rounds) != 1 || rounds[0].Summary != "vòng 1" {
			t.Fatalf("archived rounds must keep round 1 only: %+v", rounds)
		}
		// A non-chapter scope (global) must work the same way.
		g := domain.ReviewEntry{Chapter: 2, Scope: "global", Verdict: "accept", Summary: "tổng"}
		if err := h.Reviews.SaveReview(g); err != nil {
			t.Fatal(err)
		}
		if got, err := h.Reviews.LoadReview(2, store.ReviewScopeGlobal); err != nil || got == nil || got.Scope != "global" {
			t.Fatalf("global scope must round-trip: %+v %v", got, err)
		}
	})

	t.Run("review_decisions_kept", func(t *testing.T) {
		if err := h.Reviews.SaveReviewDecision("item-1", store.ReviewDecision{
			State: "kept", Reason: "giữ nguyên", By: "dat", At: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatal(err)
		}
		all, err := h.Reviews.LoadReviewDecisions()
		if err != nil {
			t.Fatal(err)
		}
		if all["item-1"].State != "kept" || all["item-1"].Reason != "giữ nguyên" {
			t.Fatalf("decision roundtrip: %+v", all)
		}
		ok, err := h.Reviews.DeleteReviewDecision("item-1")
		if err != nil || !ok {
			t.Fatalf("delete must report found: %v %v", ok, err)
		}
		all, err = h.Reviews.LoadReviewDecisions()
		if err != nil || len(all) != 0 {
			t.Fatalf("deleted decision must be gone: %v %+v", err, all)
		}
	})
}
