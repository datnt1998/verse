package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/host/exp"
	"github.com/datnt1998/noverse/internal/store"
)

// newFlatStory 构造一个扁平故事目录：<story>/output/novel 下 1..n 章终稿、
// 大纲、进度，返回读取器与故事目录。
func newFlatStory(t *testing.T, completed []int) (*StoryReader, string) {
	t.Helper()
	storyDir := t.TempDir()
	s := store.NewStore(filepath.Join(storyDir, engineSubdir))
	if err := s.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := s.Book.Save(domain.BookMetadata{Title: "Truyện Thử", Synopsis: "tóm tắt"}); err != nil {
		t.Fatalf("save book: %v", err)
	}
	outline := make([]domain.OutlineEntry, 0, len(completed)+1)
	for _, ch := range completed {
		outline = append(outline, domain.OutlineEntry{Chapter: ch, Title: fmt.Sprintf("Ch %d", ch), CoreEvent: "sự kiện"})
	}
	outline = append(outline, domain.OutlineEntry{Chapter: len(completed) + 1, Title: "Ch sau", CoreEvent: "chưa viết"})
	if err := s.Outline.SaveOutline(outline); err != nil {
		t.Fatalf("save outline: %v", err)
	}
	if err := s.Progress.Init(len(outline)); err != nil {
		t.Fatalf("init progress: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	for _, ch := range completed {
		if err := s.Drafts.SaveFinalChapter(ch, "nội dung chương"); err != nil {
			t.Fatalf("save chapter %d: %v", ch, err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatalf("start %d: %v", ch, err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 3, "cliff", "main"); err != nil {
			t.Fatalf("complete %d: %v", ch, err)
		}
	}
	return &StoryReader{storyDir: storyDir, store: s}, storyDir
}

func TestStoryReaderChapters_FlatHasDraftVolumeArc(t *testing.T) {
	r, _ := newFlatStory(t, []int{1, 2})
	if err := r.store.Drafts.SaveDraft(3, "bản nháp chương 3"); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	chapters, err := r.Chapters()
	if err != nil {
		t.Fatalf("Chapters: %v", err)
	}
	if len(chapters) != 3 {
		t.Fatalf("chapters = %d, want 3", len(chapters))
	}
	var ch3 *StoryChapter
	for i := range chapters {
		if chapters[i].Chapter == 3 {
			ch3 = &chapters[i]
		}
	}
	if ch3 == nil {
		t.Fatal("outline ch3 missing")
	}
	if !ch3.HasDraft {
		t.Error("ch3 should report hasDraft")
	}
	if ch3.Completed {
		t.Error("ch3 should not be completed")
	}
	if ch3.Volume != 1 || ch3.Arc != 0 {
		t.Errorf("flat story should map to volume 1 arc 0, got v%d a%d", ch3.Volume, ch3.Arc)
	}
	if chapters[0].Volume != 1 {
		t.Errorf("ch1 volume = %d, want 1", chapters[0].Volume)
	}
	if !chapters[0].Completed || chapters[0].Words == 0 {
		t.Errorf("ch1 should be completed with words, got %+v", chapters[0])
	}
}

func TestStoryReaderChapterText_CommittedVsDraft(t *testing.T) {
	r, _ := newFlatStory(t, []int{1})
	if err := r.store.Drafts.SaveDraft(1, "bản nháp khác"); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	committed, err := r.ChapterText(1, "")
	if err != nil {
		t.Fatalf("committed: %v", err)
	}
	if committed.Source != "committed" || committed.Text != "nội dung chương" {
		t.Errorf("committed = %+v", committed)
	}
	// "nội dung chương" = 3 chữ (domain.WordCount), không phải 15 ký tự.
	if committed.Words != 3 {
		t.Errorf("committed.Words = %d, want 3", committed.Words)
	}
	draft, err := r.ChapterText(1, "draft")
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if draft.Source != "draft" || draft.Text != "bản nháp khác" {
		t.Errorf("draft = %+v", draft)
	}
	if _, err := r.ChapterText(2, "draft"); err == nil {
		t.Error("expect error for missing draft")
	}
	if _, err := r.ChapterText(9, ""); err == nil {
		t.Error("expect error for missing chapter")
	}
}

func TestStoryReaderChapterPlanFactsRevisions(t *testing.T) {
	r, _ := newFlatStory(t, []int{1})
	if err := r.store.Drafts.SaveChapterPlan(domain.ChapterPlan{Chapter: 1, Title: "P1", Goal: "g"}); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	plan, err := r.ChapterPlan(1)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Title != "P1" {
		t.Errorf("plan title = %q", plan.Title)
	}
	if _, err := r.ChapterPlan(2); err == nil {
		t.Error("expect missing plan error")
	}
	if _, err := r.ChapterFacts(1); err == nil {
		t.Error("expect missing facts error")
	}
	revs, err := r.ChapterRevisions(1)
	if err != nil {
		t.Fatalf("revisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("revisions should be empty, got %d", len(revs))
	}
}

func TestStoryReaderOutline_FlatSingleVolume(t *testing.T) {
	r, _ := newFlatStory(t, []int{1, 2})
	if err := r.store.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "Mở đầu", Summary: "tóm"}); err != nil {
		t.Fatalf("save summary: %v", err)
	}
	if err := r.store.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "ok"}); err != nil {
		t.Fatalf("save review: %v", err)
	}
	if err := r.store.Drafts.SaveChapterPlan(domain.ChapterPlan{Chapter: 1, Title: "P"}); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	outline, err := r.Outline(OutlineAllVolumes)
	if err != nil {
		t.Fatalf("Outline: %v", err)
	}
	if outline.Layered {
		t.Error("flat story should not be layered")
	}
	if len(outline.Volumes) != 1 {
		t.Fatalf("volumes = %d, want 1", len(outline.Volumes))
	}
	vol := outline.Volumes[0]
	if vol.Index != 1 || vol.Title != "Truyện Thử" {
		t.Errorf("volume = %+v", vol)
	}
	if len(vol.Chapters) != 3 {
		t.Fatalf("volume chapters = %d, want 3", len(vol.Chapters))
	}
	c1 := vol.Chapters[0]
	if c1.Status != "done" || !c1.HasPlan || !c1.HasReview {
		t.Errorf("ch1 = %+v, want done+plan+review", c1)
	}
	if c1.Summary != "tóm" {
		t.Errorf("ch1 summary = %q", c1.Summary)
	}
	if vol.Chapters[2].Status != "pending" {
		t.Errorf("ch3 status = %q, want pending", vol.Chapters[2].Status)
	}
}

func TestStoryReaderOutline_RewriteStatuses(t *testing.T) {
	r, _ := newFlatStory(t, []int{1, 2, 3})
	if err := r.store.Progress.SetPendingRewrites([]int{1, 2}, "sửa", store.RewriteCauseReview); err != nil {
		t.Fatalf("set rewrites: %v", err)
	}
	if err := r.store.Progress.StartChapter(2); err != nil {
		t.Fatalf("start rewrite ch2: %v", err)
	}
	outline, err := r.Outline(OutlineAllVolumes)
	if err != nil {
		t.Fatalf("Outline: %v", err)
	}
	chapters := outline.Volumes[0].Chapters
	if chapters[0].Status != "rewrite" {
		t.Errorf("ch1 = %q, want rewrite", chapters[0].Status)
	}
	if chapters[1].Status != "rewriting" {
		t.Errorf("ch2 = %q, want rewriting", chapters[1].Status)
	}
	if chapters[2].Status != "done" {
		t.Errorf("ch3 = %q, want done", chapters[2].Status)
	}
}

// newLayeredStory 构造两卷分层故事：v1 两弧各 1 章（全局 1..2），v2 一个展开弧
// 1 章（全局 3）+ 一个骨架弧（不计章）。
func newLayeredStory(t *testing.T, completed []int) (*StoryReader, string) {
	t.Helper()
	r, storyDir := newFlatStory(t, nil)
	s := r.store
	volumes := []domain.VolumeOutline{
		{
			Index: 1, Title: "Tập một",
			Arcs: []domain.ArcOutline{
				{Index: 1, Title: "Hồi 1", Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "A1", CoreEvent: "e1"}}},
				{Index: 2, Title: "Hồi 2", Chapters: []domain.OutlineEntry{{Chapter: 2, Title: "A2", CoreEvent: "e2"}}},
			},
		},
		{
			Index: 2, Title: "Tập hai",
			Arcs: []domain.ArcOutline{
				{Index: 1, Title: "Hồi 3", Chapters: []domain.OutlineEntry{{Chapter: 3, Title: "B1", CoreEvent: "e3"}}},
				{Index: 2, Title: "Hồi khung", EstimatedChapters: 4},
			},
		},
	}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatalf("save layered: %v", err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatalf("set layered: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	for _, ch := range completed {
		if err := s.Drafts.SaveFinalChapter(ch, "nội dung"); err != nil {
			t.Fatalf("save chapter: %v", err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 2, "", ""); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}
	return r, storyDir
}

func TestStoryReaderOutline_Layered(t *testing.T) {
	r, _ := newLayeredStory(t, []int{1, 2})
	if err := r.store.Summaries.SaveVolumeSummary(domain.VolumeSummary{Volume: 1, Title: "Tập một", Summary: "tóm tập 1"}); err != nil {
		t.Fatalf("save vol summary: %v", err)
	}
	outline, err := r.Outline(OutlineAllVolumes)
	if err != nil {
		t.Fatalf("Outline: %v", err)
	}
	if !outline.Layered {
		t.Fatal("expected layered outline")
	}
	if len(outline.Volumes) != 2 {
		t.Fatalf("volumes = %d, want 2", len(outline.Volumes))
	}
	v1 := outline.Volumes[0]
	if v1.Summary != "tóm tập 1" {
		t.Errorf("vol summary = %q", v1.Summary)
	}
	if len(v1.Arcs) != 2 {
		t.Fatalf("v1 arcs = %d, want 2", len(v1.Arcs))
	}
	if got := v1.Arcs[0].Chapters; len(got) != 1 || got[0] != 1 {
		t.Errorf("arc1 chapters = %v, want [1]", got)
	}
	if got := v1.Arcs[1].Chapters; len(got) != 1 || got[0] != 2 {
		t.Errorf("arc2 chapters = %v, want [2]", got)
	}
	v2 := outline.Volumes[1]
	if len(v2.Chapters) != 1 || v2.Chapters[0].Chapter != 3 {
		t.Fatalf("v2 chapters = %+v, want global ch3", v2.Chapters)
	}
	if v2.Chapters[0].Status != "pending" {
		t.Errorf("v2 ch3 status = %q, want pending", v2.Chapters[0].Status)
	}
	// 分层章号映射：弧内顺序累加为全局章号。
	chapters, err := r.Chapters()
	if err != nil {
		t.Fatalf("Chapters: %v", err)
	}
	var c2, c3 *StoryChapter
	for i := range chapters {
		switch chapters[i].Chapter {
		case 2:
			c2 = &chapters[i]
		case 3:
			c3 = &chapters[i]
		}
	}
	if c2 == nil || c2.Volume != 1 || c2.Arc != 2 {
		t.Errorf("ch2 should be v1 a2, got %+v", c2)
	}
	if c3 == nil || c3.Volume != 2 || c3.Arc != 1 {
		t.Errorf("ch3 should be v2 a1, got %+v", c3)
	}
}

// ?volume=N chỉ trả chương của một tập; ?volume=0 chỉ header (hợp đồng AG, mục lục lazy >200 ch).
func TestStoryReaderOutline_VolumeFilter(t *testing.T) {
	r, _ := newLayeredStory(t, []int{1, 2})

	full, err := r.Outline(OutlineAllVolumes)
	if err != nil {
		t.Fatalf("Outline all: %v", err)
	}
	if len(full.Volumes[0].Chapters) != 2 || len(full.Volumes[1].Chapters) != 1 {
		t.Fatalf("full outline chapters = %d/%d, want 2/1", len(full.Volumes[0].Chapters), len(full.Volumes[1].Chapters))
	}

	vol2, err := r.Outline(2)
	if err != nil {
		t.Fatalf("Outline(2): %v", err)
	}
	if len(vol2.Volumes) != 2 {
		t.Fatalf("filtered outline volumes = %d, want 2 (headers giữ nguyên)", len(vol2.Volumes))
	}
	if len(vol2.Volumes[0].Chapters) != 0 {
		t.Errorf("vol1 chapters = %d, want 0 when volume=2", len(vol2.Volumes[0].Chapters))
	}
	if len(vol2.Volumes[1].Chapters) != 1 || vol2.Volumes[1].Chapters[0].Chapter != 3 {
		t.Errorf("vol2 chapters = %+v, want [ch3]", vol2.Volumes[1].Chapters)
	}
	if got := vol2.Volumes[0].Arcs[0].Chapters; len(got) != 1 {
		t.Errorf("vol1 arc1 numbers = %v, want [1]", got)
	}

	headers, err := r.Outline(0)
	if err != nil {
		t.Fatalf("Outline(0): %v", err)
	}
	for i, v := range headers.Volumes {
		if len(v.Chapters) != 0 {
			t.Errorf("volume=0: vol%d chapters = %d, want 0", i+1, len(v.Chapters))
		}
	}
	if headers.Volumes[1].Arcs[1].EstimatedChapters != 4 {
		t.Errorf("skeleton arc estimatedChapters = %d, want 4", headers.Volumes[1].Arcs[1].EstimatedChapters)
	}
}

func TestStoryReaderReviews(t *testing.T) {
	r, _ := newFlatStory(t, []int{1, 2})
	if err := r.store.World.SaveReview(domain.ReviewEntry{Chapter: 2, Scope: "global", Verdict: "polish", Summary: "toàn cục"}); err != nil {
		t.Fatalf("save global: %v", err)
	}
	latest, err := r.LatestReview()
	if err != nil {
		t.Fatalf("LatestReview: %v", err)
	}
	if latest == nil || latest.Scope != "global" {
		t.Fatalf("latest = %+v", latest)
	}
	both, err := r.Reviews(2)
	if err != nil {
		t.Fatalf("Reviews: %v", err)
	}
	if both.Chapter != nil {
		t.Error("chapter review should be nil")
	}
	if both.Global == nil || both.Global.Verdict != "polish" {
		t.Errorf("global = %+v", both.Global)
	}
}

func TestStoryReaderLatestReview_PrefersGlobalOnTie(t *testing.T) {
	r, _ := newFlatStory(t, []int{1})
	if err := r.store.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "c"}); err != nil {
		t.Fatalf("save chapter review: %v", err)
	}
	if err := r.store.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "global", Verdict: "accept", Summary: "g"}); err != nil {
		t.Fatalf("save global: %v", err)
	}
	latest, err := r.LatestReview()
	if err != nil {
		t.Fatalf("LatestReview: %v", err)
	}
	if latest == nil || latest.Scope != "global" {
		t.Fatalf("latest = %+v, want global", latest)
	}
}

func TestStoryReaderCharactersForeshadow(t *testing.T) {
	r, _ := newFlatStory(t, []int{1})
	if err := r.store.Characters.Save([]domain.Character{{Name: "Tri Hạ", Role: "chính"}}); err != nil {
		t.Fatalf("save characters: %v", err)
	}
	if err := r.store.Cast.Save([]domain.CastEntry{{Name: "Tinh Mang", FirstSeenChapter: 1}}); err != nil {
		t.Fatalf("save cast: %v", err)
	}
	if err := r.store.World.SaveForeshadowLedger([]domain.ForeshadowEntry{{ID: "f1", Description: "bằng chứng", PlantedAt: 1, Status: "planted"}}); err != nil {
		t.Fatalf("save foreshadow: %v", err)
	}
	chars, cast, err := r.Characters()
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	if len(chars) != 1 || chars[0].Name != "Tri Hạ" {
		t.Errorf("characters = %+v", chars)
	}
	if len(cast) != 1 || cast[0].Name != "Tinh Mang" {
		t.Errorf("cast = %+v", cast)
	}
	entries, err := r.Foreshadow()
	if err != nil {
		t.Fatalf("Foreshadow: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "f1" {
		t.Errorf("foreshadow = %+v", entries)
	}
}

func TestStoryReaderVolumeRanges(t *testing.T) {
	flat, _ := newFlatStory(t, []int{1, 2})
	ranges, err := flat.VolumeRanges()
	if err != nil {
		t.Fatalf("VolumeRanges: %v", err)
	}
	if len(ranges) != 1 || ranges[0].Volume != 1 || ranges[0].From != 1 || ranges[0].To != 3 {
		t.Errorf("flat ranges = %+v, want [1..3] in one volume", ranges)
	}

	layered, _ := newLayeredStory(t, []int{1})
	ranges, err = layered.VolumeRanges()
	if err != nil {
		t.Fatalf("VolumeRanges: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("layered ranges = %d, want 2", len(ranges))
	}
	if ranges[0].From != 1 || ranges[0].To != 2 {
		t.Errorf("v1 range = %+v, want 1..2", ranges[0])
	}
	if ranges[1].From != 3 || ranges[1].To != 3 {
		t.Errorf("v2 range = %+v, want 3..3 (cung khung không tính chương)", ranges[1])
	}
}

func TestStoryReaderExport_Range(t *testing.T) {
	r, storyDir := newFlatStory(t, []int{1, 2, 3})
	out := filepath.Join(storyDir, "xuat-ban", "tap-1.txt")
	res, err := r.Export(context.Background(), exp.Options{
		Format: exp.FormatTXT, OutPath: out, From: 2, To: 3,
		Overwrite: true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Chapters != 2 || res.Path != out {
		t.Errorf("result = %+v", res)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Chương 2") || !strings.Contains(text, "Chương 3") {
		t.Errorf("vi headings missing:\n%s", text)
	}
	if strings.Contains(text, "Chương 1") || strings.Contains(text, "第") {
		t.Errorf("export should contain only range 2..3 with vi headings:\n%s", text)
	}
}

// TestStoryReaderWorksAlongsideLock 模拟"TUI 持锁"：引擎目录里已有
// .ainovel.lock 时读取器照常工作（读取不取租约）。
func TestStoryReaderWorksAlongsideLock(t *testing.T) {
	r, storyDir := newFlatStory(t, []int{1})
	lock := filepath.Join(storyDir, engineSubdir, ".ainovel.lock")
	if err := os.WriteFile(lock, []byte("pid"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	chapters, err := r.Chapters()
	if err != nil {
		t.Fatalf("Chapters with lock: %v", err)
	}
	if len(chapters) == 0 {
		t.Error("chapters should be readable while locked")
	}
	text, err := r.ChapterText(1, "")
	if err != nil {
		t.Fatalf("ChapterText with lock: %v", err)
	}
	if text.Text == "" {
		t.Error("text should be readable while locked")
	}
}
