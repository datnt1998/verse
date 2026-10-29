package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/store/narrativetest"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

// backendFixture provisions a story and returns the full store facade on the
// postgres backend plus the raw scope.
func backendFixture(t *testing.T, projectSlug, storySlug string) (*store.Store, *postgres.Backend, pgFixture) {
	t.Helper()
	f := provisionNarrative(t, projectSlug, storySlug)
	b := postgres.NewBackend(context.Background(), f.pool, f.nar, f.scope)
	s, err := store.NewFromBackend(b)
	if err != nil {
		t.Fatalf("NewFromBackend refuses a complete postgres backend: %v", err)
	}
	return s, b, f
}

// reviewPort adapts WorldStore to the suite's explicit-scope review port.
type reviewPort struct{ w store.WorldStore }

func (p reviewPort) SaveReview(r domain.ReviewEntry) error { return p.w.SaveReview(r) }
func (p reviewPort) LoadReview(chapter int, scope store.ReviewScope) (*domain.ReviewEntry, error) {
	if scope == store.ReviewScopeGlobal {
		return p.w.LoadGlobalReview(chapter)
	}
	return p.w.LoadReview(chapter)
}
func (p reviewPort) LoadReviewRounds(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error) {
	return p.w.LoadReviewRounds(chapter, scope)
}
func (p reviewPort) SaveReviewDecision(itemID string, d store.ReviewDecision) error {
	return p.w.SaveReviewDecision(itemID, d)
}
func (p reviewPort) LoadReviewDecisions() (map[string]store.ReviewDecision, error) {
	return p.w.LoadReviewDecisions()
}
func (p reviewPort) DeleteReviewDecision(itemID string) (bool, error) {
	return p.w.DeleteReviewDecision(itemID)
}

// TestPostgresBackendNarrativeContract runs the shared domain suite through
// the postgres substores (not the repository adapters): the same assertions
// that gate the file backend must pass on rows.
func TestPostgresBackendNarrativeContract(t *testing.T) {
	s, _, f := backendFixture(t, "project-backend", "truyen-backend")
	if got := s.BackendKind(); got != store.BackendPostgres {
		t.Fatalf("backend kind = %q, want postgres", got)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	narrativetest.Run(t, narrativetest.Harness{
		Book:        s.Book,
		Progress:    s.Progress,
		Outline:     s.Outline,
		Drafts:      s.Drafts,
		Records:     s.ChapterRecords,
		Characters:  s.Characters,
		Checkpoints: s.Checkpoints,
		Decisions:   s.Decisions,
		Revisions:   s.Revisions,
		Reviews:     reviewPort{w: s.World},
		Comments:    f.nar.CommentsStore(f.scope),
	})
}

// TestPostgresBackendRemainingSubstores covers every substore the shared suite
// does not touch, so "all 19 domains work" is an executed claim, not a list.
func TestPostgresBackendRemainingSubstores(t *testing.T) {
	s, b, _ := backendFixture(t, "project-subs", "truyen-subs")
	ctx := context.Background()

	t.Run("run_meta", func(t *testing.T) {
		if err := s.RunMeta.Init("truyen", "provider", "model"); err != nil {
			t.Fatal(err)
		}
		if err := s.RunMeta.SetPendingSteer("viết chậm lại"); err != nil {
			t.Fatal(err)
		}
		if err := s.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
			t.Fatal(err)
		}
		if err := s.RunMeta.GrantAdvancePermit(3); err != nil {
			t.Fatal(err)
		}
		meta, err := s.RunMeta.Load()
		if err != nil || meta == nil || meta.PendingSteer != "viết chậm lại" || meta.AdvancePermitChapter != 3 {
			t.Fatalf("run meta roundtrip: %+v %v", meta, err)
		}
		if err := s.RunMeta.SetAdvanceMode(domain.ChapterAdvanceAuto); err != nil {
			t.Fatal(err)
		}
		if err := s.RunMeta.ClearPendingSteer(); err != nil {
			t.Fatal(err)
		}
		meta, _ = s.RunMeta.Load()
		if meta.PendingSteer != "" {
			t.Fatalf("steer must be cleared: %+v", meta)
		}
	})

	t.Run("user_rules", func(t *testing.T) {
		if _, err := s.UserRules.Load(); err != nil {
			t.Fatal(err)
		}
		if err := s.UserRules.AddRuntime(store.RuntimeRule{ID: "r1", Text: "không dùng từ lóng"}); err != nil {
			t.Fatal(err)
		}
		list, err := s.UserRules.Runtime()
		if err != nil || len(list) != 1 || list[0].ID != "r1" {
			t.Fatalf("runtime rules: %+v %v", list, err)
		}
		if _, found, err := s.UserRules.RemoveRuntime("r1"); err != nil || !found {
			t.Fatalf("remove runtime: %v %v", found, err)
		}
	})

	t.Run("signals", func(t *testing.T) {
		if err := s.Signals.SavePendingCommit(domain.PendingCommit{Chapter: 2}); err != nil {
			t.Fatal(err)
		}
		p, err := s.Signals.LoadPendingCommit()
		if err != nil || p == nil || p.Chapter != 2 {
			t.Fatalf("pending commit: %+v %v", p, err)
		}
		if err := s.Signals.ClearPendingCommit(); err != nil {
			t.Fatal(err)
		}
		if p, err := s.Signals.LoadPendingCommit(); err != nil || p != nil {
			t.Fatalf("cleared pending commit: %+v %v", p, err)
		}
	})

	t.Run("runtime_queue_and_logs", func(t *testing.T) {
		item, err := s.Runtime.AppendQueue(domain.RuntimeQueueItem{Agent: "writer", Summary: "chương 1"})
		if err != nil || item.Seq == 0 {
			t.Fatalf("append queue: %+v %v", item, err)
		}
		if err := s.Runtime.AppendTaskLog("task-1", domain.RuntimeTaskLogEntry{TaskID: "task-1", Event: "start", Summary: "bắt đầu"}); err != nil {
			t.Fatal(err)
		}
		log, err := s.Runtime.LoadTaskLog("task-1")
		if err != nil || len(log) != 1 || log[0].Event != "start" {
			t.Fatalf("task log: %+v %v", log, err)
		}
		queue, err := s.Runtime.LoadQueueAfter(0)
		if err != nil || len(queue) != 1 {
			t.Fatalf("queue: %+v %v", queue, err)
		}
		if err := s.Runtime.Reset(); err != nil {
			t.Fatal(err)
		}
		if queue, _ := s.Runtime.LoadQueue(); len(queue) != 0 {
			t.Fatalf("reset must clear queue: %+v", queue)
		}
		if log, _ := s.Runtime.LoadTaskLog("task-1"); len(log) != 0 {
			t.Fatalf("reset must clear task logs: %+v", log)
		}
	})

	t.Run("cast_usage_simulation", func(t *testing.T) {
		if err := s.Cast.Save([]domain.CastEntry{{Name: "Nga", BriefRole: "chính"}}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Cast.Load(); err != nil || len(got) != 1 {
			t.Fatalf("cast: %+v %v", got, err)
		}
		if err := s.Usage.Save(domain.UsageState{Overall: domain.AgentUsageTotals{Input: 10, Output: 5}}); err != nil {
			t.Fatal(err)
		}
		if u, err := s.Usage.Load(); err != nil || u == nil || u.Overall.Input != 10 {
			t.Fatalf("usage: %+v %v", u, err)
		}
		if err := s.Simulation.Save(domain.SimulationProfile{Version: domain.SimulationProfileVersion}); err != nil {
			t.Fatal(err)
		}
		if p, err := s.Simulation.Load(); err != nil || p == nil {
			t.Fatalf("simulation: %+v %v", p, err)
		}
	})

	t.Run("sessions", func(t *testing.T) {
		if err := s.Sessions.LogCoCreate(map[string]string{"text": "chào"}); err != nil {
			t.Fatal(err)
		}
		logger := s.Sessions.SubAgentLogger(nil)
		if logger == nil {
			t.Fatal("sub agent logger must exist")
		}
	})

	t.Run("summaries", func(t *testing.T) {
		if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "Mở đầu", Summary: "tóm tắt"}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Summaries.LoadSummary(1); err != nil || got == nil || got.Summary != "tóm tắt" {
			t.Fatalf("summary: %+v %v", got, err)
		}
		if err := s.Summaries.SaveVolumeSummary(domain.VolumeSummary{Volume: 1, Summary: "tập"}); err != nil {
			t.Fatal(err)
		}
		if ok, err := s.Summaries.HasVolumeSummary(1); err != nil || !ok {
			t.Fatalf("has volume summary: %v %v", ok, err)
		}
	})

	t.Run("world_documents", func(t *testing.T) {
		if err := s.World.SaveWorldRules([]domain.WorldRule{{Category: "magic", Rule: "ma thuật"}}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.World.LoadWorldRules(); err != nil || len(got) != 1 {
			t.Fatalf("world rules: %+v %v", got, err)
		}
		if err := s.World.AppendTimelineEvents([]domain.TimelineEvent{{Chapter: 1, Event: "bắt đầu"}}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.World.LoadTimeline(); err != nil || len(got) != 1 {
			t.Fatalf("timeline: %+v %v", got, err)
		}
		if err := s.World.SaveRuleViolations(1, []rules.Violation{{Rule: "xưng hô"}}); err != nil {
			t.Fatal(err)
		}
		if got := s.World.LoadRuleViolations(1); len(got) != 1 {
			t.Fatalf("rule violations: %+v", got)
		}
	})

	t.Run("drafts_plans_and_text", func(t *testing.T) {
		if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{Chapter: 1, Goal: "kế hoạch"}); err != nil {
			t.Fatal(err)
		}
		if plan, err := s.Drafts.LoadChapterPlan(1); err != nil || plan == nil || plan.Goal != "kế hoạch" {
			t.Fatalf("plan: %+v %v", plan, err)
		}
		if err := s.Drafts.SaveFinalChapter(2, "Chương hai đã xong."); err != nil {
			t.Fatal(err)
		}
		if text, err := s.Drafts.LoadChapterText(2); err != nil || !strings.Contains(text, "Chương hai") {
			t.Fatalf("chapter text: %q %v", text, err)
		}
	})

	t.Run("format_version_and_fingerprint", func(t *testing.T) {
		if v, err := b.LoadProjectFormatVersion(); err != nil || v != store.LegacyProjectFormatVersion {
			t.Fatalf("absent format must read legacy: %d %v", v, err)
		}
		if err := b.SaveProjectFormatVersion(3); err != nil {
			t.Fatal(err)
		}
		if v, err := b.LoadProjectFormatVersion(); err != nil || v != 3 {
			t.Fatalf("format roundtrip: %d %v", v, err)
		}
		if err := s.Outline.SavePremise("Tiền đề."); err != nil {
			t.Fatal(err)
		}
		if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Mở"}}); err != nil {
			t.Fatal(err)
		}
		if err := s.Characters.Save([]domain.Character{{ID: "c1", Name: "Nga"}}); err != nil {
			t.Fatal(err)
		}
		if err := s.World.SaveWorldRules([]domain.WorldRule{{Category: "magic", Rule: "ma thuật"}}); err != nil {
			t.Fatal(err)
		}
		if err := s.Book.Save(domain.BookMetadata{Title: "Tên đầu", Synopsis: "mở"}); err != nil {
			t.Fatal(err)
		}
		first, err := b.FoundationFingerprint()
		if err != nil || first == "" {
			t.Fatalf("fingerprint: %q %v", first, err)
		}
		if err := s.Book.Save(domain.BookMetadata{Title: "Tên mới", Synopsis: "đổi"}); err != nil {
			t.Fatal(err)
		}
		second, err := b.FoundationFingerprint()
		if err != nil || second == first {
			t.Fatalf("foundation change must move the fingerprint: %q %v", second, err)
		}
	})

	t.Run("compound_ops", func(t *testing.T) {
		volumes := []domain.VolumeOutline{{
			Index: 1, Title: "Tập 1",
			Arcs: []domain.ArcOutline{{Index: 1, Title: "Cung khung", Goal: "mở"}},
		}}
		if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
			t.Fatal(err)
		}
		expansion := domain.ArcExpansion{
			Title: "Cung một", Goal: "khai triển",
			Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu"}, {Chapter: 2, Title: "Tiếp"}},
		}
		if err := s.ExpandArc(1, 1, expansion); err != nil {
			t.Fatal(err)
		}
		if p, err := s.Progress.Load(); err != nil || p == nil || p.TotalChapters != 2 {
			t.Fatalf("expand must update progress in the same op: %+v %v", p, err)
		}
		if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, TotalChapters: 2, Flow: domain.FlowSteering}); err != nil {
			t.Fatal(err)
		}
		if err := s.RunMeta.SetPendingSteer("dừng"); err != nil {
			t.Fatal(err)
		}
		if err := s.ClearHandledSteer(); err != nil {
			t.Fatal(err)
		}
		if p, _ := s.Progress.Load(); p.Flow != domain.FlowWriting {
			t.Fatalf("flow must reset: %+v", p)
		}
		if m, _ := s.RunMeta.Load(); m.PendingSteer != "" {
			t.Fatalf("steer must clear: %+v", m)
		}
		if _, err := s.ReviseOutline(2, []domain.OutlineEntry{{Title: "Khác"}}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("review_invalidation_on_rewrite", func(t *testing.T) {
		facts := domain.ChapterFacts{Title: "Đêm"}
		if _, err := s.ChapterRecords.Accept(4, domain.ChapterOriginGenerated, "Bản một.", facts, domain.StyleDelta{}); err != nil {
			t.Fatal(err)
		}
		if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 4, Scope: "chapter", Verdict: "rewrite", Summary: "cần sửa"}); err != nil {
			t.Fatal(err)
		}
		if err := s.World.SaveReviewDecision("item-x", store.ReviewDecision{State: "kept", Reason: "giữ"}); err != nil {
			t.Fatal(err)
		}
		rec, err := s.ChapterRecords.Accept(4, domain.ChapterOriginUser, "Bản hai.", facts, domain.StyleDelta{})
		if err != nil {
			t.Fatal(err)
		}
		if rec.Revision != 2 {
			t.Fatalf("rewrite must bump revision: %+v", rec)
		}
		if rec.Build != version.Build() {
			t.Fatalf("accepted record must carry the build stamp: %q want %q", rec.Build, version.Build())
		}
		if got, err := s.World.LoadReview(4); err != nil || got != nil {
			t.Fatalf("review of the rewritten chapter must be invalidated: %+v %v", got, err)
		}
		if got, err := s.World.LoadReviewDecisions(); err != nil || len(got) != 0 {
			t.Fatalf("kept decisions of invalidated reviews must go: %+v %v", got, err)
		}
	})

	t.Run("immutable_history_conflict", func(t *testing.T) {
		rec, err := s.ChapterRecords.Load(4)
		if err != nil || rec == nil {
			t.Fatalf("load: %+v %v", rec, err)
		}
		rec.Revision = 9
		if err := s.ChapterRecords.Save(*rec); err == nil {
			t.Fatal("saving a non-next revision must conflict")
		} else if !isErr(err, errs.ErrStorageConflict) {
			t.Fatalf("conflict error class: %v", err)
		}
	})

	t.Run("chapter_record_document_view", func(t *testing.T) {
		// commit_chapter digests the accepted record by its file-backend path;
		// on rows that path is a read-only view over the revision table, and a
		// document write there must be refused (revisions only append through
		// the typed store).
		rel := store.ChapterRecordPath(4)
		data, err := b.Docs().ReadFile(rel)
		if err != nil {
			t.Fatalf("read accepted record %s: %v", rel, err)
		}
		var rec domain.ChapterRecord
		if err := json.Unmarshal(data, &rec); err != nil || rec.Chapter != 4 || rec.Revision == 0 {
			t.Fatalf("record view: %+v %v", rec, err)
		}
		if err := s.Drafts.SaveFinalChapter(4, rec.Content); err != nil {
			t.Fatal(err)
		}
		if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 4, Title: "Đêm", Summary: "tóm tắt"}); err != nil {
			t.Fatal(err)
		}
		cp, err := s.Checkpoints.AppendArtifacts(domain.ChapterScope(4), "commit",
			fmt.Sprintf("chapters/%02d.md", 4), fmt.Sprintf("summaries/%02d.json", 4), rel)
		if err != nil {
			t.Fatalf("commit checkpoint over the record view: %v", err)
		}
		if cp == nil || cp.Digest == "" {
			t.Fatalf("checkpoint: %+v", cp)
		}
		if exists, err := b.Docs().ExistsUnlocked(rel); err != nil || !exists {
			t.Fatalf("record view must exist: %v %v", exists, err)
		}
		if err := b.Docs().WriteFileUnlocked(rel, []byte(`{}`)); err == nil {
			t.Fatal("a document write to an accepted record must be refused")
		}
	})

	_ = ctx
	_ = time.Now
}

func isErr(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
