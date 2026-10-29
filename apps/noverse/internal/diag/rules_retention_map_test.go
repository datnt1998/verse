package diag

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// TestBuildRetentionMap: vị trí lời hứa/cú lật, chuỗi móc thứ ba, câu hỏi treo
// ngoài 2–4 (chương cuối được miễn) và cờ "đã khai" của đại cương cũ.
func TestBuildRetentionMap(t *testing.T) {
	entries := make([]domain.OutlineEntry, 0, 8)
	for i := range 8 {
		entry := domain.OutlineEntry{Chapter: i + 1, HookType: "lat_thong_tin"}
		entry.OpenQuestions = []string{"Ai?", "Gì?"}
		entries = append(entries, entry)
	}
	entries[2].Promise = "da_tra: cảnh lời hứa"
	entries[3].MidpointTwist = true
	entries[1].HookType = "cau_thoai_no"
	for i := 4; i <= 6; i++ {
		entries[i].HookType = "de_doa_moi"
	}
	entries[0].OpenQuestions = []string{"Ai?"}
	entries[2].OpenQuestions = []string{"a?", "b?", "c?", "d?", "e?"}
	entries[7].OpenQuestions = []string{"Ai?"}

	m := BuildRetentionMap(&Snapshot{Outline: entries})
	if m.Total != 8 {
		t.Errorf("Total = %d, muốn 8", m.Total)
	}
	if m.PromiseChapter != 3 || m.TwistChapter != 4 {
		t.Errorf("promise/twist = %d/%d, muốn 3/4", m.PromiseChapter, m.TwistChapter)
	}
	if len(m.HookChains) != 1 || m.HookChains[0].Type != "de_doa_moi" {
		t.Fatalf("HookChains = %+v", m.HookChains)
	}
	if got := m.HookChains[0].Chapters; len(got) != 3 || got[0] != 5 || got[2] != 7 {
		t.Errorf("chương của chuỗi = %v, muốn [5 6 7] (báo chương thứ ba)", got)
	}
	if len(m.FewQuestions) != 1 || m.FewQuestions[0] != 1 {
		t.Errorf("FewQuestions = %v, muốn [1]", m.FewQuestions)
	}
	if len(m.ManyQuestions) != 1 || m.ManyQuestions[0] != 3 {
		t.Errorf("ManyQuestions = %v, muốn [3]", m.ManyQuestions)
	}
	if !m.PromiseDeclared || !m.TwistDeclared || !m.HooksDeclared || !m.QuestionsDeclared {
		t.Errorf("cờ khai = %+v", m)
	}
}

// TestBuildRetentionMapOldOutline: đại cương không có trường giữ chân vẫn trả
// Total theo số dòng và không báo câu hỏi (không coi "chưa khai" là 0 câu hỏi).
func TestBuildRetentionMapOldOutline(t *testing.T) {
	entries := make([]domain.OutlineEntry, 0, 4)
	for i := range 4 {
		entries = append(entries, domain.OutlineEntry{Chapter: i + 1})
	}
	m := BuildRetentionMap(&Snapshot{Outline: entries})
	if m.Total != 4 || m.PromiseChapter != 0 || m.TwistChapter != 0 {
		t.Errorf("map = %+v", m)
	}
	if len(m.FewQuestions) != 0 || len(m.ManyQuestions) != 0 || len(m.HookChains) != 0 {
		t.Errorf("đại cương cũ không được báo: %+v", m)
	}
	if m.PromiseDeclared || m.TwistDeclared || m.HooksDeclared || m.QuestionsDeclared {
		t.Errorf("cờ khai phải false: %+v", m)
	}
}

// TestBuildRetentionMapUnexpandedLayered: cung chưa mở rộng thì mẫu số vị trí
// rơi về số dòng đại cương hiện có (không giả vờ biết tổng số chương).
func TestBuildRetentionMapUnexpandedLayered(t *testing.T) {
	snap := &Snapshot{
		Outline: []domain.OutlineEntry{{Chapter: 1}, {Chapter: 2}},
		Volumes: []domain.VolumeOutline{{Arcs: []domain.ArcOutline{{EstimatedChapters: 8}}}},
	}
	if m := BuildRetentionMap(snap); m.Total != 2 {
		t.Errorf("Total = %d, muốn 2", m.Total)
	}
	if m := BuildRetentionMap(nil); m.Total != 0 {
		t.Errorf("nil snapshot: %+v", m)
	}
}
