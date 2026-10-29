package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func setupArcSummaryStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(4); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1,
		Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, Chapters: []domain.OutlineEntry{{Title: "三"}, {Title: "四"}}},
		},
	}}
	if err := s.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline(domain.FlattenOutline(volumes)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatal(err)
	}
	for chapter := 1; chapter <= 4; chapter++ {
		if err := s.Progress.MarkChapterComplete(chapter, 100, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 2, Scope: "arc", Verdict: "accept", Summary: "第一弧评审"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveArcSummary(domain.ArcSummary{Volume: 1, Arc: 1, Title: "第一弧", Summary: "完成", KeyEvents: []string{"事件"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 4, Scope: "arc", Verdict: "accept", Summary: "第二弧评审"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func validArcSummaryArgs(t *testing.T) []byte {
	t.Helper()
	args, err := json.Marshal(map[string]any{
		"volume":     1,
		"arc":        2,
		"title":      "入山",
		"summary":    "主角完成入山试炼，确认后续追索方向。",
		"key_events": []string{"通过试炼", "发现旧案线索"},
		"character_snapshots": []map[string]any{
			{"name": "沈渊", "status": "存活", "motivation": "追查旧案"},
		},
		"style_rules": map[string]any{
			"prose": []string{"环境描写优先触觉和嗅觉", "动作戏用短句推进", "心理描写不解释结论"},
			"dialogue": []map[string]any{
				{"name": "沈渊", "rules": []string{"对话极简", "少用疑问句"}},
			},
			"taboos": []string{"避免章末长独白"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return args
}

func TestSaveArcSummaryPersistsStyleRulesDialogueObjects(t *testing.T) {
	s := setupArcSummaryStore(t)

	tool := NewSaveArcSummaryTool(s)
	if _, err := tool.Execute(context.Background(), validArcSummaryArgs(t)); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	rules, err := s.World.LoadStyleRules()
	if err != nil {
		t.Fatalf("LoadStyleRules: %v", err)
	}
	if rules == nil || len(rules.Dialogue) != 1 {
		t.Fatalf("expected one dialogue rule, got %+v", rules)
	}
	if rules.Dialogue[0].Name != "沈渊" || len(rules.Dialogue[0].Rules) != 2 {
		t.Fatalf("unexpected dialogue rule: %+v", rules.Dialogue[0])
	}
}

func TestSaveArcSummaryWritesCompletionMarkerLast(t *testing.T) {
	s := setupArcSummaryStore(t)
	snapshotPath := filepath.Join(s.Dir(), "meta", "snapshots", "v01a02.json")
	if err := os.MkdirAll(snapshotPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := NewSaveArcSummaryTool(s).Execute(context.Background(), validArcSummaryArgs(t)); err == nil || !strings.Contains(err.Error(), "save character snapshots") {
		t.Fatalf("expected snapshot write failure, got %v", err)
	}
	if summary, err := s.Summaries.LoadArcSummary(1, 2); err != nil || summary != nil {
		t.Fatalf("arc summary must remain absent after partial failure, summary=%+v err=%v", summary, err)
	}
}

func TestSaveArcSummaryRetriesCheckpointWithoutOverwriting(t *testing.T) {
	s := setupArcSummaryStore(t)
	checkpointPath := filepath.Join(s.Dir(), "meta", "checkpoints.jsonl")
	if err := os.MkdirAll(checkpointPath, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := NewSaveArcSummaryTool(s)
	args := validArcSummaryArgs(t)

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "checkpoint arc summary") {
		t.Fatalf("expected checkpoint failure, got %v", err)
	}
	if summary, err := s.Summaries.LoadArcSummary(1, 2); err != nil || summary == nil {
		t.Fatalf("semantic summary should already be persisted, summary=%+v err=%v", summary, err)
	}
	if err := os.Remove(checkpointPath); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("idempotent checkpoint retry: %v", err)
	}
	if cp := s.Checkpoints.LatestByStep(domain.ArcScope(1, 2), "arc_summary"); cp == nil {
		t.Fatal("checkpoint retry must finish the aggregate write")
	}
}

func TestSaveArcSummaryRejectsDialogueStringArray(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	tool := NewSaveArcSummaryTool(s)
	args, err := json.Marshal(map[string]any{
		"volume":              1,
		"arc":                 2,
		"title":               "入山",
		"summary":             "主角完成入山试炼，确认后续追索方向。",
		"key_events":          []string{"通过试炼"},
		"character_snapshots": []map[string]any{},
		"style_rules": map[string]any{
			"prose":    []string{"环境描写优先触觉和嗅觉"},
			"dialogue": []string{"沈渊对话极简"},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "style_rules.dialogue") {
		t.Fatalf("expected style_rules.dialogue validation error, got %v", err)
	}
}

func TestSaveArcSummaryRequiresStyleRules(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	args := json.RawMessage(`{
		"volume": 1,
		"arc": 2,
		"title": "入山",
		"summary": "主角完成入山试炼。",
		"key_events": ["通过试炼"],
		"character_snapshots": []
	}`)
	if _, err := NewSaveArcSummaryTool(s).Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "style_rules is required") {
		t.Fatalf("expected missing style_rules error, got %v", err)
	}
}

func TestSaveArcSummaryRejectsUncommittedChapters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		completed []int
		missing   string
	}{
		{name: "future arc", completed: []int{1, 2}, missing: "chương 3"},
		{name: "gap before committed endpoint", completed: []int{1, 2, 4}, missing: "chương 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := setupArcSummaryStore(t)
			progress, err := st.Progress.Load()
			if err != nil {
				t.Fatal(err)
			}
			progress.CompletedChapters = tc.completed
			if err := st.Progress.Save(progress); err != nil {
				t.Fatal(err)
			}
			_, err = NewSaveArcSummaryTool(st).Execute(context.Background(), validArcSummaryArgs(t))
			for _, want := range []string{"Chưa thể lưu tóm tắt cung 2 của tập 1", tc.missing, "chưa được nộp", "Không lưu tóm tắt dự kiến"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("expected clear uncommitted-arc error containing %q, got %v", want, err)
				}
			}
			if summary, err := st.Summaries.LoadArcSummary(1, 2); err != nil || summary != nil {
				t.Fatalf("uncommitted arc must not persist a summary: summary=%+v err=%v", summary, err)
			}
			if snapshots, err := st.Characters.LoadSnapshots(1, 2); err != nil || len(snapshots) != 0 {
				t.Fatalf("uncommitted arc must not persist snapshots: snapshots=%+v err=%v", snapshots, err)
			}
			if rules, err := st.World.LoadStyleRules(); err != nil || rules != nil {
				t.Fatalf("uncommitted arc must not persist style rules: rules=%+v err=%v", rules, err)
			}
			if cp := st.Checkpoints.LatestByStep(domain.ArcScope(1, 2), "arc_summary"); cp != nil {
				t.Fatal("uncommitted arc must not append completion checkpoint")
			}
		})
	}
}

func TestSaveArcSummaryIdenticalReplayReportsUnchanged(t *testing.T) {
	st := setupArcSummaryStore(t)
	tool := NewSaveArcSummaryTool(st)
	args := validArcSummaryArgs(t)
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("identical replay: %v", err)
	}
	var reply struct {
		Saved     bool   `json:"saved"`
		Unchanged bool   `json:"unchanged"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(result, &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.Saved || !reply.Unchanged || reply.Message != "đã có, không đổi" {
		t.Fatalf("expected successful unchanged save, got %s", result)
	}
	var changed map[string]any
	if err := json.Unmarshal(args, &changed); err != nil {
		t.Fatal(err)
	}
	changed["style_rules"].(map[string]any)["prose"] = []string{"Luật khác cho cùng cung"}
	conflictingArgs, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Execute(context.Background(), conflictingArgs); err == nil || !strings.Contains(err.Error(), "tool conflict") {
		t.Fatalf("same arc rule changes must still conflict, got %v", err)
	}
}

func TestSaveArcSummaryReplayDoesNotCompareLaterArcStyleRules(t *testing.T) {
	st := setupArcSummaryStore(t)
	tool := NewSaveArcSummaryTool(st)
	args := validArcSummaryArgs(t)
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	// This file is a rolling book-wide view, not part of the old arc's identity.
	laterRules := domain.WritingStyleRules{
		Volume: 2, Arc: 1, Prose: []string{"Luật của cung sau"},
		Dialogue:  []domain.CharacterVoice{{Name: "An", Rules: []string{"Nói ngắn"}}},
		UpdatedAt: "2026-10-27T13:41:31+07:00",
	}
	if err := st.World.SaveStyleRules(laterRules); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(st.Dir(), "summaries", "arc-v01a02.json"),
		filepath.Join(st.Dir(), "meta", "snapshots", "v01a02.json"),
		filepath.Join(st.Dir(), "meta", "style_rules.json"),
	}
	before := make([]string, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[i] = string(data)
	}
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("identical historical replay must ignore later arc rules: %v", err)
	}
	for _, field := range []string{"summary", "character_snapshots"} {
		var changed map[string]any
		if err := json.Unmarshal(args, &changed); err != nil {
			t.Fatal(err)
		}
		switch field {
		case "summary":
			changed[field] = "Nội dung khác"
		case "character_snapshots":
			changed[field].([]any)[0].(map[string]any)["status"] = "Mất tích"
		}
		conflictingArgs, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tool.Execute(context.Background(), conflictingArgs); err == nil || !strings.Contains(err.Error(), "tool conflict") {
			t.Errorf("historical %s changes must still conflict, got %v", field, err)
		}
	}
	for i, path := range paths {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != before[i] {
			t.Errorf("historical replay overwrote %s", path)
		}
	}
}
