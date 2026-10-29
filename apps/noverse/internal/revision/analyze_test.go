package revision

import (
	"context"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// 事实引文是可选证据：不带 quote 照常通过；带了就必须能定位到修订后的正文，
// 定位失败走 llmcontract 纠错重问（同一回答耗尽重试后报错）。
func TestAnalyzeValidatesQuotesAgainstRevisedText(t *testing.T) {
	change := Change{Chapter: 1, Before: "林墨留在城中。", After: "林墨连夜离开城市，顺手拿走了桌上的信。"}
	prev := domain.ChapterRecord{Facts: domain.ChapterFacts{Title: "第一章", Summary: "旧摘要"}}

	analysisJSON := func(quote string) string {
		q := `"quote":null`
		if quote != "" {
			q = `"quote":"` + quote + `"`
		}
		return `{
  "change_summary":"修订",
  "story_changed":true,
  "facts":{
    "title":"第一章","summary":"林墨连夜离城","characters":["林墨"],"key_events":["离城"],
    "timeline_events":[{"time":"当夜","event":"林墨离开城市","characters":["林墨"],` + q + `}],
    "foreshadow_updates":[],"relationship_changes":[],"state_changes":[],
    "cast_intros":[],"hook_type":null,"dominant_strand":null,"places":null,"objects":null
  },
  "style_delta":{"prose":[],"dialogue":[],"taboos":[]},
  "outline_impact":{"deviation":"提前离城","suggestion":"后续从城外承接"},
  "downstream_issues":[]
}`
	}

	t.Run("quote 能在修订正文里定位则通过", func(t *testing.T) {
		model := &revisionModel{response: analysisJSON("顺手拿走了桌上的信")}
		analysis, err := Analyze(context.Background(), model, "分析", change, prev, nil)
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		if analysis.Facts.TimelineEvents[0].Quote != "顺手拿走了桌上的信" {
			t.Fatalf("quote not carried: %+v", analysis.Facts.TimelineEvents)
		}
	})

	t.Run("quote 定位不到修订正文则返回纠错失败", func(t *testing.T) {
		model := &revisionModel{response: analysisJSON("不存在的引文内容")}
		_, err := Analyze(context.Background(), model, "分析", change, prev, nil)
		if err == nil || !strings.Contains(err.Error(), "không tìm thấy") {
			t.Fatalf("expected quote correction failure, got %v", err)
		}
	})

	t.Run("不带 quote 不检查照常通过", func(t *testing.T) {
		model := &revisionModel{response: analysisJSON("")}
		if _, err := Analyze(context.Background(), model, "分析", change, prev, nil); err != nil {
			t.Fatalf("Analyze without quote must pass: %v", err)
		}
	})
}
