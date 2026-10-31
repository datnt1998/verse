package insight

import (
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"testing"
)

func TestScoreReasonRequiresNumericalBasis(t *testing.T) {
	views, median, score := int64(4200), int64(1000), 4.2
	v := domain.RivalVideo{ViewCount: &views, Score: &score}
	c := domain.RivalChannel{Title: "Nocturne", MedianViews: &median}
	got := ScoreReason(v, c)
	if got == nil || len(got.Facts) < 2 || got.Facts[1].Value != "1000" {
		t.Fatalf("reason: %+v", got)
	}
	c.MedianViews = nil
	if ScoreReason(v, c) != nil {
		t.Fatal("missing median must not produce reason")
	}
}
