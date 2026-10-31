package metadata

import (
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"strings"
)

type BasedOn struct {
	SourceTitle string   `json:"sourceTitle"`
	Rules       []string `json:"rules"`
}

func metadataReasons(b *BasedOn, sourceTitle, channel string) map[string]insight.Reason {
	if b == nil || strings.TrimSpace(sourceTitle) == "" || b.SourceTitle != sourceTitle {
		return nil
	}
	facts := []insight.Fact{{Label: "Tiêu đề nguồn", Value: sourceTitle, Source: "Video nguồn"}}
	if strings.TrimSpace(channel) != "" {
		facts = append(facts, insight.Fact{Label: "Kênh", Value: channel, Source: "Kênh của bạn"})
	}
	reason := insight.Reason{Agent: "director", Text: "Dựa trên tiêu đề video nguồn và ngữ cảnh kênh.", Facts: facts}
	if strings.TrimSpace(channel) == "" {
		reason.Text = "Dựa trên tiêu đề video nguồn."
	}
	return map[string]insight.Reason{"title": reason, "description": reason, "tags": reason}
}
