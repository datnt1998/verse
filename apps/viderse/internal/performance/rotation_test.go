package performance

import "testing"

import "github.com/datnt1998/verse/apps/viderse/internal/domain"

func TestRotationWeightsViewsPriority(t *testing.T) {
	views := map[string][]int64{"a": {3000, 2800, 3200, 2900}, "b": {1500, 1400, 1600, 1300, 1450}}
	if got := RotationWeights(nil, domain.PrioritySteady, views); len(got) != 0 {
		t.Fatalf("steady changed rotation: %v", got)
	}
	got := RotationWeights(nil, domain.PriorityViews, views)
	if got["a"] != 2 || got["b"] != 0 {
		t.Fatalf("views priority weights = %v", got)
	}
	if got := RotationWeights(nil, domain.PriorityViews, map[string][]int64{"a": {1, 2, 3}}); len(got) != 0 {
		t.Fatalf("too few videos still reweighted: %v", got)
	}
	if got := RotationWeights(map[string]int{"b": 2, "c": 1}, domain.PrioritySteady, nil); got["b"] != 2 || got["c"] != 0 {
		t.Fatalf("manual weights = %v", got)
	}
	templates := []domain.ChannelTemplate{{ID: "a"}, {ID: "b"}}
	seq := RotationSequence(templates, map[string]int{"a": 2})
	if len(seq) != 3 || seq[0].ID != "a" || seq[1].ID != "b" || seq[2].ID != "a" {
		t.Fatalf("sequence = %+v", seq)
	}
}

func TestOrderByScoreViewsPriorityChangesAutopickOrder(t *testing.T) {
	ids := []string{"v1", "v2", "v3", "v4"}
	got := OrderByScore(ids, map[string]float64{"v2": 9.5, "v3": 2.0, "v4": 4.0})
	want := []string{"v2", "v4", "v3", "v1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if ids[0] != "v1" {
		t.Fatal("input mutated")
	}
}
