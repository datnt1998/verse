package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/assistant"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmretry"
)

type cannedDirector struct{ answer string }

func (c cannedDirector) Generate(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(c.answer)}}}, nil
}

// sceneRevisionStore chỉ thay phần đọc revision cảnh (bước scenes đã xong và đã
// xoá checkpoint trong fixture này); mọi đọc/ghi của bước images vẫn đi qua
// CloneStore PostgreSQL thật.
type sceneRevisionStore struct{ *postgres.CloneStore }

func (s sceneRevisionStore) Read(ctx context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	snap, err := s.CloneStore.Read(ctx, id, step)
	if err == nil && step == domain.CloneScenes {
		snap.Manifest = json.RawMessage(`{"version":1,"revision":1}`)
	}
	return snap, err
}

type noChannels struct{ assistant.Channels }

func (noChannels) Get(context.Context, string) (channels.ChannelView, error) {
	return channels.ChannelView{}, errors.New("không dùng trong test")
}

// Câu "vẽ lại cảnh 5" đi parse → run qua images.Service + jobedit + CloneStore
// thật trên PostgreSQL: cảnh 5 sang trạng thái đang vẽ, các cảnh khác giữ nguyên.
func TestAssistantParseRunRedrawSceneFiveOnRealJob(t *testing.T) {
	ctx := context.Background()
	store := cloneStore(t)
	job := cloneTestJob(t, store, "assistant-redraw")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	var claim *pipeline.ClaimedStep
	for {
		var err error
		if claim, err = store.ClaimNextStep(ctx, job.ID, "worker"); err != nil || claim == nil {
			t.Fatalf("claim: %v %v", claim, err)
		}
		if claim.Step.Step == domain.CloneImages {
			break
		}
		if err = store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil {
			t.Fatal(err)
		}
	}
	one := 1
	m := images.Manifest{Version: 1, Revision: 4, SceneRevision: 1, Style: "full"}
	for _, id := range []string{"scene-1", "scene-2", "scene-3", "scene-4", "scene-5"} {
		it := images.Item{ID: id, Kind: "scene", SceneID: id, Narration: id, Status: "review", ReviewVersion: &one}
		it.Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(it)}}
		m.Items = append(m.Items, it)
	}
	raw, _ := json.Marshal(m)
	if err := store.WriteStepCheckpoint(ctx, job.ID, domain.CloneImages, claim.Fence, claim.Step.Attempt, raw); err != nil {
		t.Fatal(err)
	}
	edits := &jobedit.Service{Store: sceneRevisionStore{store}, Media: &stepstest.FakeMedia{Root: t.TempDir()}}
	svc := assistant.New(assistant.Deps{
		Director: func(context.Context) (llmretry.Generator, error) {
			return cannedDirector{`{"outcome":"steps","steps":[{"intent":"redraw_image","sceneNo":5,"feedback":"cho y tá cầm đèn pin","parts":null,"count":null,"minDurationMinutes":null,"rival":null,"channel":null,"preset":null,"rule":null,"pick":null,"videoTitle":null,"drawSource":null}],"ask":null,"unsupportedKind":null}`}, nil
		},
		Jobs:     &pipeline.Service{Store: store},
		Images:   &cloneimages.Service{Edits: edits},
		Channels: noChannels{},
	})
	t.Cleanup(svc.Close)

	parsed, err := svc.Parse(ctx, assistant.Scope{JobID: job.ID}, "vẽ lại cảnh 5, cho y tá cầm đèn pin")
	if err != nil || parsed.Reply != nil || parsed.Intent != assistant.RedrawImage || parsed.Confirm.Title != "Vẽ lại cảnh 5" || parsed.Confirm.Cost.Images != 1 {
		t.Fatalf("parse = %+v %v", parsed, err)
	}
	status := func() map[string]string {
		snap, err := edits.Read(ctx, job.ID, domain.CloneImages)
		if err != nil {
			t.Fatal(err)
		}
		var cur images.Manifest
		if err := json.Unmarshal(snap.Manifest, &cur); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, it := range cur.Items {
			out[it.ID] = it.Status
		}
		return out
	}
	if status()["scene-5"] != "review" {
		t.Fatalf("parse đã đổi dữ liệu: %v", status())
	}
	if _, err = svc.Run(ctx, parsed.Intent, parsed.Params); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := status()
	if got["scene-5"] != "pending" || got["scene-4"] != "review" || got["scene-1"] != "review" {
		t.Fatalf("sau run: %v, muốn chỉ cảnh 5 đang vẽ", got)
	}
}
