package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestReviseOutlineReconcilesCompletedRetentionOnly(t *testing.T) {
	for _, scenario := range []string{"accepted", "changed_event", "multiple", "uncompleted", "missing_prose", "queued_rewrite", "active_rewrite"} {
		t.Run(scenario, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(s.Init())
			must(s.Progress.Init(2))
			original := []domain.OutlineEntry{{Chapter: 1, Title: "Đối chiếu", CoreEvent: "Ba nguồn trùng mã", Hook: "Ai thao tác?", Scenes: []string{"Kiểm tra"}}, {Chapter: 2, Title: "Kế tiếp"}}
			must(s.Outline.SaveOutline(original))
			if scenario != "uncompleted" {
				must(s.Progress.MarkChapterComplete(1, 100, "", ""))
			}
			if scenario == "queued_rewrite" {
				must(s.Progress.SetPendingRewrites([]int{1}, "Cần sửa", store.RewriteCauseUser))
			}
			if scenario == "active_rewrite" {
				progress, err := s.Progress.Load()
				must(err)
				progress.InProgressChapter = 1
				must(s.Progress.Save(progress))
			}
			if scenario != "missing_prose" {
				must(s.Drafts.SaveFinalChapter(1, "Ba nguồn ghi nhận cùng tác vụ trước sự cố."))
			}
			before, err := s.Progress.Load()
			must(err)
			entry := original[0]
			entry.MidpointTwist = true
			if scenario == "changed_event" {
				entry.CoreEvent = "Bịa thêm thủ phạm"
			}
			replacement := []domain.OutlineEntry{entry}
			if scenario == "multiple" {
				replacement = append(replacement, original[1])
			}
			args, err := json.Marshal(map[string]any{"from_chapter": 1, "reconcile_completed": true, "replacement": replacement, "reason": "Đã đối chiếu chính văn"})
			must(err)
			_, err = NewReviseOutlineTool(s).Execute(context.Background(), args)
			if scenario == "accepted" {
				must(err)
				original[0].MidpointTwist = true
			} else if err == nil {
				t.Fatal("unsafe reconciliation was allowed")
			}
			got, err := s.Outline.LoadOutline()
			must(err)
			after, err := s.Progress.Load()
			must(err)
			if !reflect.DeepEqual(original, got) || !reflect.DeepEqual(before, after) {
				t.Fatal("reconciliation altered narrative, tail, or progress")
			}
		})
	}
}
