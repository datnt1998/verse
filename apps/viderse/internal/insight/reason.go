package insight

import (
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"strings"
)

type Fact struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
}
type Reason struct {
	Agent string `json:"agent"`
	Text  string `json:"text"`
	Facts []Fact `json:"facts"`
}

// ScoreReason explains the actual stored score: total views divided by the
// channel median. It is deliberately absent when that basis is unavailable.
func ScoreReason(v domain.RivalVideo, c domain.RivalChannel) *Reason {
	if v.ViewCount == nil || v.Score == nil || c.MedianViews == nil || *c.MedianViews <= 0 {
		return nil
	}
	ratio := strings.ReplaceAll(fmt.Sprintf("%.1f", *v.Score), ".", ",")
	return &Reason{Agent: domain.AgentScout, Text: fmt.Sprintf("Lượt xem gấp %s lần mức thường của %s", ratio, c.Title), Facts: []Fact{
		{Label: "Lượt xem", Value: fmt.Sprint(*v.ViewCount), Source: "YouTube"},
		{Label: "Trung vị lượt xem", Value: fmt.Sprint(*c.MedianViews), Source: "150 video gần nhất"},
	}}
}
