package diag

import (
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
)

func foreshadowSnapshot(foreshadow []domain.ForeshadowEntry, completed int) *Snapshot {
	chapters := make([]int, 0, completed)
	for ch := 1; ch <= completed; ch++ {
		chapters = append(chapters, ch)
	}
	return &Snapshot{
		Progress:   &domain.Progress{Phase: domain.PhaseWriting, CompletedChapters: chapters, TotalChapters: completed + 5},
		Foreshadow: foreshadow,
	}
}

// 已 bỏ tuyến 的伏笔不算 "đang mở"，也不算 đình trệ；quá hạn 与 ngủ quên 都计入 Đình trệ。
func TestBuildStatsForeshadowHealth(t *testing.T) {
	snap := foreshadowSnapshot([]domain.ForeshadowEntry{
		{ID: "open", Status: "planted", PlantedAt: 1},
		{ID: "advanced_short", Status: "advanced", PlantedAt: 1, LastTouched: 10},
		{ID: "overdue", Status: "planted", PlantedAt: 1, DueBy: 5},
		{ID: "abandoned", Status: "abandoned", PlantedAt: 1, DueBy: 2, AbandonReason: "cắt"},
		{ID: "resolved", Status: "resolved", PlantedAt: 1, ResolvedAt: 3},
	}, 10)

	stats := buildStats(snap)
	if stats.ForeshadowOpen != 3 {
		t.Fatalf("ForeshadowOpen = %d, muốn 3 (bỏ abandoned/resolved)", stats.ForeshadowOpen)
	}
	// open 与 overdue 都睡过头；advanced_short 第 10 章刚动过。
	if stats.ForeshadowStale != 2 {
		t.Fatalf("ForeshadowStale = %d, muốn 2", stats.ForeshadowStale)
	}
}

func TestStaleForeshadowIncludesOverdueAndDormant(t *testing.T) {
	snap := foreshadowSnapshot([]domain.ForeshadowEntry{
		{ID: "overdue", Status: "planted", PlantedAt: 2, DueBy: 4},
		{ID: "dormant", Status: "planted", PlantedAt: 1},
		{ID: "fresh", Status: "advanced", PlantedAt: 1, LastTouched: 9},
		{ID: "abandoned", Status: "abandoned", PlantedAt: 1, DueBy: 2, AbandonReason: "cắt"},
	}, 10)

	findings := StaleForeshadow(snap)
	if len(findings) != 1 {
		t.Fatalf("muốn 1 finding, nhận %d", len(findings))
	}
	f := findings[0]
	if f.Rule != "StaleForeshadow" || f.Severity != SevWarning {
		t.Fatalf("unexpected finding: %+v", f)
	}
	if !strings.Contains(f.Evidence, "chương 2 đã quá hạn thu (dự kiến chương 4)") {
		t.Fatalf("phải nêu tuyến quá hạn: %s", f.Evidence)
	}
	if !strings.Contains(f.Evidence, "chương 1 đã qua 9 chương chưa được đẩy tiếp") {
		t.Fatalf("phải nêu tuyến ngủ quên: %s", f.Evidence)
	}
	if strings.Contains(f.Evidence, "abandoned") || strings.Contains(f.Technical, "abandoned") {
		t.Fatalf("tuyến đã bỏ không được vào finding: %+v", f)
	}
	if !strings.Contains(f.Technical, "due_by=4") {
		t.Fatalf("technical phải ghi due_by: %s", f.Technical)
	}
}

func TestStaleForeshadowQuietWhenAllThreadsHealthy(t *testing.T) {
	snap := foreshadowSnapshot([]domain.ForeshadowEntry{
		{ID: "fresh", Status: "advanced", PlantedAt: 9, LastTouched: 9},
		{ID: "abandoned", Status: "abandoned", PlantedAt: 1, DueBy: 2, AbandonReason: "cắt"},
	}, 10)
	if findings := StaleForeshadow(snap); len(findings) != 0 {
		t.Fatalf("muốn không finding, nhận %+v", findings)
	}
}
