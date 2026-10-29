package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestDraftChapterRejectsUnfinishedPendingRewrite(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(80); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	for ch := 1; ch <= 58; ch++ {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}

	p, _ := s.Progress.Load()
	p.Flow = domain.FlowPolishing
	p.PendingRewrites = []int{65}
	if err := s.Progress.Save(p); err != nil {
		t.Fatalf("Save corrupt progress: %v", err)
	}

	tool := NewDraftChapterTool(s)
	args, err := json.Marshal(map[string]any{
		"chapter": 65,
		"content": "错误写入未来章节。",
		"mode":    "write",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "pending_rewrites chỉ được chứa chương đã hoàn thành") {
		t.Fatalf("expected invalid pending_rewrites rejection, got %v", err)
	}
	progress, _ := s.Progress.Load()
	if progress.InProgressChapter == 65 {
		t.Fatalf("future chapter should not become in progress")
	}
}

func TestDraftChapterRejectsUnexpandedLayeredChapter(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(5); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
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
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatalf("SetLayered: %v", err)
	}

	tool := NewDraftChapterTool(s)
	args, err := json.Marshal(map[string]any{
		"chapter": 3,
		"content": "越界正文。",
		"mode":    "write",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "expand_arc") {
		t.Fatalf("expected unexpanded chapter rejection, got %v", err)
	}
	progress, _ := s.Progress.Load()
	if progress.InProgressChapter == 3 {
		t.Fatalf("unexpanded chapter should not become in progress")
	}
}

// TestDraftChapterRejectsMechanicalResidue 锁死准入检查：思考块、代码栅栏、
// 首末段元叙述都是模型侧的机械事故，写/续两种模式都必须以 ErrToolArgs 拒收，
// 且不得落盘。
func TestDraftChapterRejectsMechanicalResidue(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "think tag", content: "<think>phải mở đầu chậm</think>\n\nNgọc mở cửa tiệm."},
		{name: "code fence", content: "Đoạn mở.\n\n```\nnội dung\n```"},
		{name: "meta opener", content: "Dưới đây là chương 2 theo đại cương.\n\nNgọc mở cửa tiệm."},
		{name: "meta closer", content: "Ngọc mở cửa tiệm.\n\nHy vọng chương này đạt yêu cầu."},
	}
	for _, mode := range []string{"write", "append"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				s := store.NewStore(t.TempDir())
				if err := s.Init(); err != nil {
					t.Fatalf("Init: %v", err)
				}
				if err := s.Progress.Init(10); err != nil {
					t.Fatalf("InitProgress: %v", err)
				}
				if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
					t.Fatalf("UpdatePhase: %v", err)
				}
				args, _ := json.Marshal(map[string]any{"chapter": 1, "content": tc.content, "mode": mode})
				if _, err := NewDraftChapterTool(s).Execute(context.Background(), args); err == nil {
					t.Fatal("mechanical residue phải bị từ chối")
				} else if !errors.Is(err, errs.ErrToolArgs) {
					t.Fatalf("lỗi phải là ErrToolArgs, got %v", err)
				}
				if draft, _ := s.Drafts.LoadDraft(1); draft != "" {
					t.Fatalf("bản nháp không được ghi khi bị từ chối: %q", draft)
				}
			})
		}
	}
}

// TestDraftChapterAppendTrimsRepeatedTail 模型常把草稿结尾重抄一遍再接新段；
// append 须剪掉开头重复部分再写盘，结果里回执被剪的 rune 数。
func TestDraftChapterAppendTrimsRepeatedTail(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	tail := "Mưa rơi trên mái ngói cũ, Ngọc khép cửa tiệm rồi đi vào trong. Lá thư cô giấu trong túi áo bỗng nặng như một cục đá."
	if err := s.Drafts.SaveDraft(2, "Phần mở đầu chương.\n\n"+tail); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	args, _ := json.Marshal(map[string]any{
		"chapter": 2,
		"content": tail + " Sáng hôm sau trời đã tạnh.",
		"mode":    "append",
	})
	result, err := NewDraftChapterTool(s).Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var payload struct {
		Trimmed int `json:"trimmed_overlap_runes"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if payload.Trimmed == 0 {
		t.Fatal("muốn trimmed_overlap_runes > 0 khi đoạn đầu bị cắt")
	}
	got, err := s.Drafts.LoadDraft(2)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if !strings.Contains(got, "Sáng hôm sau trời đã tạnh.") {
		t.Fatalf("phần mới phải được nối vào: %q", got)
	}
	if strings.Count(got, "cục đá") != 1 {
		t.Fatalf("đuôi bản nháp bị lặp lại: %q", got)
	}
}

// TestDraftChapterAppendRejectsFullDuplicate 整个追加段都是草稿已有内容时
// 必须拒绝且不写盘——这是模型复读事故，不是续写。
func TestDraftChapterAppendRejectsFullDuplicate(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	tail := "Mưa rơi trên mái ngói cũ, Ngọc khép cửa tiệm rồi đi vào trong. Lá thư cô giấu trong túi áo bỗng nặng như một cục đá."
	before := "Phần mở đầu chương.\n\n" + tail
	if err := s.Drafts.SaveDraft(2, before); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	args, _ := json.Marshal(map[string]any{"chapter": 2, "content": tail, "mode": "append"})
	if _, err := NewDraftChapterTool(s).Execute(context.Background(), args); err == nil {
		t.Fatal("đoạn trùng toàn bộ phải bị từ chối")
	} else if !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("lỗi phải là ErrToolArgs, got %v", err)
	} else if !strings.Contains(err.Error(), "viết tiếp từ sau câu cuối") {
		t.Fatalf("lỗi phải gợi ý viết tiếp, got %v", err)
	}
	got, _ := s.Drafts.LoadDraft(2)
	if got != before {
		t.Fatalf("bản nháp không được thay đổi khi bị từ chối: %q", got)
	}
}

// TestDraftChapterAppendKeepsSharedStemDifferentWord 续写开头与草稿结尾共享词干
// 但最后一个词不同（"trước bà" vs "trước bàn đá"）时不得把内容剪进半个词，
// 整段必须原样追加落盘。
func TestDraftChapterAppendKeepsSharedStemDifferentWord(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	tail := "Nàng khẽ nói rằng chiều nay hôm nay bà sẽ đến trước bà."
	content := "Nàng khẽ nói rằng chiều nay hôm nay bà sẽ đến trước bàn đá, gió thổi qua hành lang tối."
	before := "Phần mở đầu chương.\n\n" + tail
	if err := s.Drafts.SaveDraft(2, before); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	args, _ := json.Marshal(map[string]any{"chapter": 2, "content": content, "mode": "append"})
	if _, err := NewDraftChapterTool(s).Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := s.Drafts.LoadDraft(2)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if !strings.Contains(got, content) {
		t.Fatalf("đoạn nối phải giữ nguyên từng chữ: %q", got)
	}
	if !strings.HasSuffix(got, "hành lang tối.") {
		t.Fatalf("đoạn nối bị cắt giữa từ: %q", got)
	}
}
