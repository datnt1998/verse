package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// TestCommitChapterRejectsDueByAtPlantingChapter 守护 due_by 预检：
// due_by 不晚于种植章属于 ErrToolArgs，必须在 PendingCommit 冻结、终稿与章节记录
// 落盘之前拦下。此前约束只在 UpdateForeshadow 落账时触发——那时 PendingCommit
// 已冻结，重试只回放同一载荷，模型永远无法修正参数（死循环）。
// 失败后模型修正 due_by 重新提交必须成功，且账本记录的 DueBy 与修正一致。
func TestCommitChapterRejectsDueByAtPlantingChapter(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(1); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := s.Drafts.SaveDraft(1, "Uyển Uyển nhặt được chiếc chìa khóa rồi lặng lẽ cất vào túi áo."); err != nil {
		t.Fatal(err)
	}
	args := func(dueBy int) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{
			"chapter": 1, "title": "Chương 1", "summary": "Nhặt được chìa khóa",
			"characters": []string{"Lâm Uyển"}, "key_events": []string{"nhặt chìa khóa"},
			"foreshadow_updates": []map[string]any{
				{"id": "chia-khoa", "action": "plant", "description": "Chiếc chìa khóa lạ", "due_by": dueBy},
			},
		})
		return raw
	}

	_, err := newTestCommitChapterTool(s).Execute(context.Background(), args(1))
	if err == nil || !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("due_by=chương phải bị từ chối ErrToolArgs, nhận %v", err)
	}
	if pending, perr := s.Signals.LoadPendingCommit(); perr != nil || pending != nil {
		t.Fatalf("tham số sai không được tạo pending commit: pending=%+v err=%v", pending, perr)
	}
	if text, terr := s.Drafts.LoadChapterText(1); terr != nil || text != "" {
		t.Fatalf("bản nộp bị từ chối không được ghi bản cuối: %q err=%v", text, terr)
	}
	if rec, rerr := s.ChapterRecords.Load(1); rerr != nil || rec != nil {
		t.Fatalf("bản nộp bị từ chối không được ghi chapter record: %+v err=%v", rec, rerr)
	}

	if _, err := newTestCommitChapterTool(s).Execute(context.Background(), args(4)); err != nil {
		t.Fatalf("due_by đã sửa phải commit thành công: %v", err)
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 1 || ledger[0].ID != "chia-khoa" || ledger[0].DueBy != 4 || ledger[0].Status != "planted" {
		t.Fatalf("ledger sau khi sửa = %+v", ledger)
	}
}
