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

func TestEvaluateFixed85Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []autonomy.ItemScore
		pass  bool
	}{
		{"empty", nil, false},
		{"all skipped", []autonomy.ItemScore{{Skipped: true}}, false},
		{"84", []autonomy.ItemScore{{Score: score(84)}}, false},
		{"85", []autonomy.ItemScore{{Score: score(85)}}, true},
		{"unscored", []autonomy.ItemScore{{Score: score(99)}, {}}, false},
		{"mixed", []autonomy.ItemScore{{Score: score(85)}, {Score: score(84)}}, false},
		{"skipped missing score", []autonomy.ItemScore{{Score: score(85)}, {Skipped: true}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if v := autonomy.Evaluate(domain.AutoApproveMin, tc.items); v.AllPass != tc.pass {
				t.Fatalf("verdict=%+v want pass=%v", v, tc.pass)
			}
		})
	}
}
