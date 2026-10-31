package host

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
	"github.com/datnt1998/verse/apps/noverse/internal/userrules"
	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/agentcore/subagent"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

// Runs real simulation → snapshot → novel_context → plan/draft/check/commit
// against a lease-fenced PG store. No provider calls or story files are used.
func TestPostgresDocumentSourcesReachScriptedEngine(t *testing.T) {
	work := isolateCwd(t)
	st, _, pool := pgEngineStoreDocuments(t, 2)
	docs := postgres.NewTextDocuments(pool)
	ctx := context.Background()
	const storyRule = "Luật riêng của truyện: chim giấy chỉ bay về phía biển."
	const sampleText = "Văn mẫu từ kho hosted: giọt nắng nằm yên trên bậc cửa."
	const styleMarker = "Nhịp văn hosted: câu ngắn rồi một khoảng lặng."
	for _, layer := range []struct{ story, text string }{{"", "Luật dự án bị cấp truyện thay thế."}, {"pg-story", storyRule}} {
		if _, err := docs.Mutate(ctx, nil, "pg-engine", layer.story, "rules", "tone.md", layer.text, "", false, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := docs.Mutate(ctx, nil, "pg-engine", "pg-story", "style_sample", "Giọng mẫu.txt", sampleText, "", false, 20); err != nil {
		t.Fatal(err)
	}
	opts, err := docs.RuleOptions(ctx, nil, "pg-engine", "pg-story")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := userrules.NewService(st, nil, opts).Build(ctx, ""); err != nil {
		t.Fatal(err)
	}
	var rulesSeen, styleSeen, sampleSeen atomic.Bool
	base := scriptedWriterModel()
	writer := &scriptedChatModel{fn: func(messages []llmtypes.Message) llmtypes.Message {
		raw, _ := json.Marshal(messages)
		if strings.Contains(string(raw), storyRule) {
			rulesSeen.Store(true)
		}
		if strings.Contains(string(raw), styleMarker) {
			styleSeen.Store(true)
		}
		results := 0
		filtered := make([]llmtypes.Message, 0, len(messages))
		for _, message := range messages {
			if message.Role == llmtypes.RoleTool {
				results++
				if results == 1 {
					continue
				}
			}
			filtered = append(filtered, message)
		}
		if results == 0 {
			return testToolCallMsg("novel_context", map[string]any{"chapter": 1})
		}
		return base.fn(filtered)
	}}
	h, e := onceTestHost(t, st, writer)
	h.runCtx = context.Background()
	stats := tools.NewStyleStatsIndex(st)
	e.workers = subagent.NewRunner(subagent.Config{Name: "writer", Description: "hosted documents writer", Model: writer, SystemPrompt: "test", Tools: []agentcore.Tool{
		tools.NewContextTool(st, tools.References{}, "default", stats), tools.NewPlanChapterTool(st), tools.NewDraftChapterTool(st), tools.NewCheckConsistencyTool(st), tools.NewCommitChapterTool(st, stats),
	}, MaxTurns: 12, StopAfterTools: []string{"commit_chapter"}})
	var calls atomic.Int32
	simulation := &scriptedChatModel{fn: func(messages []llmtypes.Message) llmtypes.Message {
		raw, _ := json.Marshal(messages)
		if strings.Contains(string(raw), sampleText) {
			sampleSeen.Store(true)
		}
		if calls.Add(1) == 1 {
			return testTextMsg(`{"title":null,"summary":"Giọng mẫu hosted","style_observations":[],"common_words":[],"plot_patterns":[],"hook_patterns":[],"pacing_notes":[],"reader_appeal":[],"reusable_techniques":[],"warnings":[]}`)
		}
		return testTextMsg(`{"style":{"narrative_voice":[],"sentence_rhythm":[],"prose_texture":["` + styleMarker + `"],"perspective":[],"mood":[],"do_not_copy":[]},"lexicon":{"common_words":[],"emotion_words":[],"scene_words":[],"transition_words":[],"signature_phrases":[]},"plot_design":{"opening_patterns":[],"escalation_patterns":[],"turning_point_patterns":[],"payoff_patterns":[]},"hook_design":{"hook_types":[],"placement":[],"cliffhanger_patterns":[],"payoff_rules":[]},"pacing_density":{"scene_density":[],"information_release":[],"dialogue_action_ratio":[],"compression_rules":[]},"reader_engagement":{"methods":[],"emotional_drivers":[],"progression_rewards":[],"anti_patterns":[]},"role_guidance":{"architect":[],"writer":["` + styleMarker + `"],"editor":[]}}`)
	}}
	h.models = &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("scripted", "scripted", simulation, nil)}
	h.bundle.Prompts.SimulationSource = "Phân tích văn mẫu"
	h.bundle.Prompts.SimulationMerge = "Hợp nhất giọng văn"
	var options newOptions
	WithDocumentSources(opts, func(ctx context.Context) ([]sim.Source, error) {
		rows, err := docs.List(ctx, nil, "pg-engine", "pg-story", "style_sample")
		if err != nil {
			return nil, err
		}
		out := []sim.Source{}
		for _, row := range rows {
			out = append(out, sim.Source{Name: row.Name, Data: []byte(row.Text), UpdatedAt: row.UpdatedAt})
		}
		return out, nil
	})(&options)
	h.ruleOptions = *options.ruleOptions
	h.simulationSources = options.simulationSources
	events, err := h.SimulateSource(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for event := range events {
		if event.Err != nil {
			t.Fatal(event.Err)
		}
	}
	if !sampleSeen.Load() {
		t.Fatal("PG sample did not reach simulation prompt context")
	}
	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatal(err)
	}
	waitHostDone(t, h)
	e.wait()
	if !rulesSeen.Load() {
		t.Fatal("story-level PG rule did not reach writer prompt context")
	}
	if !styleSeen.Load() {
		t.Fatal("PG sample's learned style did not reach writer prompt context")
	}
	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("scripted engine failed to commit: %+v", progress)
	}
	assertEmptyDir(t, work)
}
