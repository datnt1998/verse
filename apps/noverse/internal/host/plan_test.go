package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// seedPlanStore tạo một truyện đang lập kế hoạch với mốc thời gian cố định:
// khởi động 10:00 → book 10:02 → premise 10:07 → characters 10:11 →
// outline (layered, 1 arc mở + 1 arc khung) 10:30; còn thiếu world_rules + compass.
// checkpoints.jsonl/decisions.jsonl phải ghi trước NewStore vì cache nạp lúc dựng.
func seedPlanStore(t *testing.T) *storepkg.Store {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) time.Time {
		return time.Date(2026, 10, 26, h, m, 0, 0, time.UTC)
	}
	checkpoint := func(seq int, step string, when time.Time) string {
		rec, err := json.Marshal(domain.Checkpoint{
			Seq: int64(seq), Scope: domain.GlobalScope(), Step: step,
			Artifact: "x", Digest: fmt.Sprintf("sha256:d%d", seq), OccurredAt: when,
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(rec) + "\n"
	}
	cps := checkpoint(1, "book", at(10, 2)) +
		checkpoint(2, "premise", at(10, 7)) +
		checkpoint(3, "characters", at(10, 11)) +
		checkpoint(4, "expand_arc", at(10, 30))
	if err := os.WriteFile(filepath.Join(dir, "meta", "checkpoints.jsonl"), []byte(cps), 0o644); err != nil {
		t.Fatal(err)
	}
	dec := `{"schema_version":1,"id":"d1","at":"2026-10-26T10:00:00Z","kind":"plan_start","decider":"arbiter","input":"truyện test"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "meta", "decisions.jsonl"), []byte(dec), 0o644); err != nil {
		t.Fatal(err)
	}

	st := storepkg.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Save(&domain.Progress{Phase: domain.PhaseOutline}); err != nil {
		t.Fatal(err)
	}
	if err := st.Book.Save(domain.BookMetadata{Title: "Tuổi Sáng Phản Bộn", Synopsis: "Thư ký pháp án đoạt lại kịch bản của mình."}); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SavePremise("Cô thư ký chứng kiến tác giả kịch bản bị thay tên."); err != nil {
		t.Fatal(err)
	}
	if err := st.Characters.Save([]domain.Character{
		{Name: "Lâm Tri Hạ", Role: "Nữ chính"},
		{Name: "Tống Nhược Vy", Role: "Phản diện"},
	}); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1, Title: "Tập 1",
		Arcs: []domain.ArcOutline{
			{Index: 1, Title: "Cung 1", Chapters: []domain.OutlineEntry{
				{Chapter: 1, Title: "Chữ ký sai hợp đồng", Hook: "Tri Hạ phát hiện chữ ký bị thay"},
				{Chapter: 2, Title: "Phòng quy hoạch vắng chủ", Hook: "Người giữ khoá mất tích"},
			}},
			{Index: 2, Title: "Cung 2", EstimatedChapters: 6},
		},
	}}
	if err := st.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	return st
}

func stepByKey(t *testing.T, plan *PlanSnapshot, key string) PlanStep {
	t.Helper()
	for _, s := range plan.Steps {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("step %q missing in plan %+v", key, plan.Steps)
	return PlanStep{}
}

// P1: khung “Kế hoạch truyện” — 5 bước đúng thứ tự, trạng thái theo FoundationMissing,
// thời lượng theo mốc checkpoint, và dàn ý đếm “đã lưu n/N chương” gồm cung khung.
func TestFillPlanStepsStatusesDurationsAndOutlineCount(t *testing.T) {
	st := seedPlanStore(t)
	h := &Host{store: st}
	snap := &UISnapshot{
		IsRunning: true,
		Agents: []AgentSnapshot{{
			Name: "architect_long", State: "working", Tool: "save_foundation",
			Summary: "architect_long → save_foundation[outline]",
		}},
	}

	h.fillPlan(snap, &domain.Progress{Phase: domain.PhaseOutline})
	plan := snap.Plan
	if plan == nil {
		t.Fatal("planning story must emit plan")
	}
	if plan.Total != 5 || len(plan.Steps) != 5 {
		t.Fatalf("steps = %+v", plan.Steps)
	}
	wantKeys := []string{"book", "premise", "characters", "worldRules", "outline"}
	for i, s := range plan.Steps {
		if s.Key != wantKeys[i] {
			t.Fatalf("step %d = %q, want %q", i, s.Key, wantKeys[i])
		}
	}
	// book·premise·characters xong; outline đang chạy (agent đang save_foundation[outline]);
	// worldRules còn thiếu → pending.
	statuses := map[string]string{}
	for _, s := range plan.Steps {
		statuses[s.Key] = s.Status
	}
	if statuses["book"] != "done" || statuses["premise"] != "done" || statuses["characters"] != "done" {
		t.Fatalf("done steps wrong: %v", statuses)
	}
	if statuses["outline"] != "running" {
		t.Fatalf("outline should be running (agent variant): %v", statuses)
	}
	if statuses["worldRules"] != "pending" {
		t.Fatalf("worldRules should be pending: %v", statuses)
	}
	if plan.RunningStep != "outline" || plan.Done != 3 {
		t.Fatalf("running=%q done=%d", plan.RunningStep, plan.Done)
	}
	// Thời lượng theo mốc: book 2p, premise 5p, characters 4p, outline 19p.
	want := map[string]int64{"book": 120000, "premise": 300000, "characters": 240000, "outline": 1140000}
	for key, ms := range want {
		if got := stepByKey(t, plan, key).DurationMs; got != ms {
			t.Fatalf("%s duration = %dms, want %dms", key, got, ms)
		}
	}
	if s := stepByKey(t, plan, "outline"); s.StartedAt.IsZero() {
		t.Fatal("running step must carry startedAt")
	}
	// Dàn ý: 2 chương đã lưu + 1 cung khung ước 6 = tổng 8 → “đã lưu 2/8 chương”.
	var outline PlanOutlineDetail
	if err := json.Unmarshal(stepByKey(t, plan, "outline").Detail, &outline); err != nil {
		t.Fatal(err)
	}
	if outline.Saved != 2 || outline.Total != 8 || len(outline.Chapters) != 2 {
		t.Fatalf("outline detail = %+v", outline)
	}
	if outline.Chapters[0].Title != "Chữ ký sai hợp đồng" || outline.Chapters[0].Hook == "" {
		t.Fatalf("chapter detail = %+v", outline.Chapters[0])
	}
	if got := stepByKey(t, plan, "outline").Summary; got != "đã lưu 2/8 chương" {
		t.Fatalf("outline summary = %q", got)
	}
	// Nội dung từng bước xong.
	var book PlanBookDetail
	if err := json.Unmarshal(stepByKey(t, plan, "book").Detail, &book); err != nil || book.Title != "Tuổi Sáng Phản Bộn" || book.Synopsis == "" {
		t.Fatalf("book detail = %+v err=%v", book, err)
	}
	var chars PlanCharactersDetail
	if err := json.Unmarshal(stepByKey(t, plan, "characters").Detail, &chars); err != nil || len(chars.Characters) != 2 || chars.Characters[0].Name != "Lâm Tri Hạ" {
		t.Fatalf("characters detail = %+v err=%v", chars, err)
	}
	var premise PlanPremiseDetail
	if err := json.Unmarshal(stepByKey(t, plan, "premise").Detail, &premise); err != nil || premise.Text == "" {
		t.Fatalf("premise detail = %+v err=%v", premise, err)
	}
}

// Tạm dừng giữa chừng: bước kế tiếp mang trạng thái paused thay vì running.
func TestFillPlanPausedMarksNextStepPaused(t *testing.T) {
	st := seedPlanStore(t)
	h := &Host{store: st}
	snap := &UISnapshot{IsRunning: false}

	h.fillPlan(snap, &domain.Progress{Phase: domain.PhaseOutline})
	if snap.Plan == nil {
		t.Fatal("paused planning must still emit plan")
	}
	if got := stepByKey(t, snap.Plan, "worldRules").Status; got != "paused" {
		t.Fatalf("first missing step should be paused, got %q", got)
	}
	if got := stepByKey(t, snap.Plan, "outline").Status; got != "pending" {
		t.Fatalf("next missing step should be pending, got %q", got)
	}
}

// Đã sang giai đoạn viết thì không phát khung kế hoạch nữa.
func TestFillPlanOmittedOnceWriting(t *testing.T) {
	st := seedPlanStore(t)
	h := &Host{store: st}
	snap := &UISnapshot{}
	h.fillPlan(snap, &domain.Progress{Phase: domain.PhaseWriting})
	if snap.Plan != nil {
		t.Fatalf("writing phase must not emit plan: %+v", snap.Plan)
	}
	snap2 := &UISnapshot{WaitingFor: "cocreate"}
	h.fillPlan(snap2, &domain.Progress{Phase: domain.PhaseOutline})
	if snap2.Plan != nil {
		t.Fatalf("cocreate hold must not emit plan: %+v", snap2.Plan)
	}
}
