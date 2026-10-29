package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func planArgs(chapter int) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"chapter":     chapter,
		"title":       "测试章",
		"goal":        "推进剧情",
		"conflict":    "外部阻力",
		"hook":        "留下悬念",
		"emotion_arc": "紧张到期待",
	})
	return b
}

func planArgsWithOpeningState(chapter int) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"chapter":       chapter,
		"title":         "测试章",
		"goal":          "推进剧情",
		"conflict":      "外部阻力",
		"hook":          "留下悬念",
		"opening_state": "上一章已定案：主角被逐出矿队，独自留在山脚，时间仍是同一夜",
	})
	return b
}

func TestPlanChapterRejectsUnexpandedLayeredChapter(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(5); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1,
		Title: "第一卷",
		Arcs: []domain.ArcOutline{{
			Index: 1,
			Title: "第一弧",
			Chapters: []domain.OutlineEntry{
				{Chapter: 1, Title: "一"},
				{Chapter: 2, Title: "二"},
			},
		}, {
			Index:             2,
			Title:             "第二弧",
			EstimatedChapters: 3,
		}},
	}}); err != nil {
		t.Fatalf("SaveLayeredOutline: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := st.Progress.SetLayered(true); err != nil {
		t.Fatalf("SetLayered: %v", err)
	}

	tool := NewPlanChapterTool(st)
	if _, err := tool.Execute(context.Background(), planArgs(3)); err == nil || !strings.Contains(err.Error(), "expand_arc") {
		t.Fatalf("expected unexpanded chapter rejection, got %v", err)
	}
	if p, _ := st.Progress.Load(); p != nil && p.InProgressChapter == 3 {
		t.Fatal("unexpanded chapter should not become in-progress")
	}
}

func TestPlanChapterAllowsExpandedLayeredChapter(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1,
		Title: "第一卷",
		Arcs: []domain.ArcOutline{{
			Index: 1,
			Title: "第一弧",
			Chapters: []domain.OutlineEntry{
				{Chapter: 1, Title: "一"},
				{Chapter: 2, Title: "二"},
			},
		}},
	}}); err != nil {
		t.Fatalf("SaveLayeredOutline: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := st.Progress.SetLayered(true); err != nil {
		t.Fatalf("SetLayered: %v", err)
	}

	tool := NewPlanChapterTool(st)
	if _, err := tool.Execute(context.Background(), planArgs(2)); err != nil {
		t.Fatalf("expected expanded chapter to plan, got %v", err)
	}
}

// Chương đã viết xong nhưng đang nằm trong hàng chờ viết lại phải lập kế hoạch được:
// đây là chỗ duy nhất ghi được "viết lại thành cái gì". Trước đây nó bị từ chối, khiến
// revise_outline / save_foundation(outline) / plan_chapter bịt cả ba đường, và kiến trúc
// sư quay vòng 4 lượt rồi bị ngắt mạch.
func TestPlanChapterAllowsChapterQueuedForRewrite(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := st.Progress.StartChapter(1); err != nil {
		t.Fatalf("StartChapter: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(1, 1000, "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := st.Progress.SetPendingRewrites([]int{1}, "đoạn mở đầu trùng chương trước", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}

	raw, err := NewPlanChapterTool(st).Execute(context.Background(), planArgs(1))
	if err != nil {
		t.Fatalf("chương trong hàng viết lại phải lập kế hoạch được, nhận: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if skipped, _ := got["skipped"].(bool); skipped {
		t.Fatalf("không được bỏ qua, nhận: %v", got)
	}
	if plan, err := st.Drafts.LoadChapterPlan(1); err != nil || plan == nil {
		t.Fatalf("kế hoạch phải được ghi xuống, err=%v plan=%v", err, plan)
	}
}

// Chương đã xong và KHÔNG chờ viết lại thì vẫn phải bị chặn — hành vi cũ giữ nguyên.
func TestPlanChapterStillSkipsSettledCompletedChapter(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := st.Progress.StartChapter(1); err != nil {
		t.Fatalf("StartChapter: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(1, 1000, "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	raw, err := NewPlanChapterTool(st).Execute(context.Background(), planArgs(1))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if skipped, _ := got["skipped"].(bool); !skipped {
		t.Fatalf("chương đã chốt phải bị bỏ qua, nhận: %v", got)
	}
}

// opening_state 落盘后必须经 LoadChapterPlan 原样读回，并出现在
// working_memory.chapter_plan 里——writer 的开章指引来源。
func TestPlanChapterOpeningStateRoundTrips(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, TotalChapters: 4}); err != nil {
		t.Fatalf("Save progress: %v", err)
	}

	if _, err := NewPlanChapterTool(st).Execute(context.Background(), planArgsWithOpeningState(2)); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	plan, err := st.Drafts.LoadChapterPlan(2)
	if err != nil {
		t.Fatalf("LoadChapterPlan: %v", err)
	}
	if plan == nil || plan.OpeningState == "" {
		t.Fatalf("opening_state must round-trip through the store, got %+v", plan)
	}

	raw, err := newTestContextTool(st, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":2}`))
	if err != nil {
		t.Fatalf("novel_context: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	planInCtx, ok := payload["working_memory"].(map[string]any)["chapter_plan"].(map[string]any)
	if !ok {
		t.Fatalf("expected working_memory.chapter_plan, got %v", payload["working_memory"])
	}
	if planInCtx["opening_state"] == "" || planInCtx["opening_state"] == nil {
		t.Fatalf("working_memory.chapter_plan.opening_state missing: %v", planInCtx)
	}
}

// 旧版 plan 文件没有 opening_state 字段，升级后仍要能读出来（omitempty 向后兼容）。
func TestLoadChapterPlanWithoutOpeningState(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	legacy := `{"chapter":5,"title":"测试章","goal":"推进剧情","conflict":"外部阻力","hook":"留下悬念","emotion_arc":"紧张到期待"}`
	if err := os.WriteFile(filepath.Join(dir, "drafts", "05.plan.json"), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy plan: %v", err)
	}

	plan, err := st.Drafts.LoadChapterPlan(5)
	if err != nil {
		t.Fatalf("LoadChapterPlan: %v", err)
	}
	if plan == nil || plan.Chapter != 5 || plan.Goal == "" {
		t.Fatalf("legacy plan must still load, got %+v", plan)
	}
	if plan.OpeningState != "" {
		t.Fatalf("legacy plan phải có opening_state rỗng, got %q", plan.OpeningState)
	}
}
