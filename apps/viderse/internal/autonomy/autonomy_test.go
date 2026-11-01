package autonomy_test

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func score(v int) *domain.ImageScore { return &domain.ImageScore{Value: v} }

func TestEvaluateAllPassAtThreshold(t *testing.T) {
	items := []autonomy.ItemScore{{ID: "a", Label: "cảnh 1", Score: score(85)}, {ID: "b", Label: "cảnh 2", Score: score(97)}, {ID: "x", Label: "cảnh 3", Skipped: true}}
	v := autonomy.Evaluate(85, items)
	if !v.AllPass || v.Total != 2 || v.Passed != 2 || v.Lowest != 85 {
		t.Fatalf("%+v", v)
	}
}

func TestEvaluateSummaryNamesTheScene(t *testing.T) {
	v := autonomy.Evaluate(85, []autonomy.ItemScore{{ID: "a", Label: "cảnh 3", Score: score(90)}, {ID: "b", Label: "cảnh 4", Score: score(72)}, {ID: "c", Label: "ảnh bìa"}})
	if v.AllPass || v.Summary() != "cảnh 4 được 72 · ảnh bìa chưa có điểm" {
		t.Fatalf("%+v %q", v, v.Summary())
	}
}

func TestCombineWeights(t *testing.T) {
	if got := autonomy.Combine(domain.ImageScoreParts{Match: 100, Artifacts: 100, Identity: 100, Duplicate: 100}); got != 100 {
		t.Fatal(got)
	}
	if got := autonomy.Combine(domain.ImageScoreParts{Match: 60, Artifacts: 100, Identity: 100, Duplicate: 100}); got != 86 {
		t.Fatalf("match 60 → %d", got)
	}
}
