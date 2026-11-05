package images

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/core/llm"
)

type qaPersistenceAnswer struct {
	body string
	err  error
}

func (m qaPersistenceAnswer) Generate(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (*llm.LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.LLMResponse{Message: llm.Message{
		Role:       llm.RoleAssistant,
		Content:    []llm.ContentBlock{llm.TextBlock(m.body)},
		StopReason: llm.StopReasonStop,
	}}, nil
}

func TestQAContractScoringFailuresKeepPaidVersionAndManualReview(t *testing.T) {
	const supported = `{"match":100,"artifacts":100,"identity":100,"duplicate":100,"reason":"Ảnh rõ và khớp cảnh.","facts":[],"instructionApplicable":true,"instructionIssue":""}`
	const unsupported = `{"match":100,"artifacts":100,"identity":100,"duplicate":100,"reason":"Ảnh rõ.","facts":[],"instructionApplicable":false,"instructionIssue":"Chưa áp dụng kiểm tra âm thanh vì chỉ có ảnh."}`
	const legacy = `{"match":60,"artifacts":100,"identity":100,"duplicate":100,"reason":"Ảnh khớp một phần lời kể.","facts":[]}`
	for _, tc := range []struct {
		name      string
		note      string
		body      string
		modelErr  error
		noScorer  bool
		wantScore int
		wantIssue bool
	}{
		{name: "music unapplied even at 100", note: "Kiểm tra nhạc nền", body: unsupported, wantIssue: true},
		{name: "missing applicability", note: "Giữ đúng nhân vật", body: legacy, wantIssue: true},
		{name: "malformed answer", note: "Giữ đúng nhân vật", body: `{"match":100`, wantIssue: true},
		{name: "scoring request error", note: "Giữ đúng nhân vật", modelErr: errors.New("local scoring fixture unavailable"), wantIssue: true},
		{name: "model unavailable", note: "Giữ đúng nhân vật", noScorer: true, wantIssue: true},
		{name: "supported instruction", note: "Giữ đúng nhân vật", body: supported, wantScore: 100},
		{name: "no note legacy score", body: legacy, wantScore: 86},
		{name: "no note scoring error", modelErr: errors.New("local scoring fixture unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, media, _, _ := sceneFixture(t, "sc-0")
			s.Job.Instructions = domain.AgentInstructions{"qa": tc.note}
			manifest := markUsed(t, s, media, "sc-0", Version{
				Version: 1, Path: "images/versions/sc-0/1-existing.webp", SHA256: "existing-image-hash",
				StateName: VersionReady, CostImages: 1, Score: &domain.ImageScore{Value: 93},
			})
			before := sceneItem(t, manifest, "sc-0").Versions[0]
			drawer := &recorderDrawer{}
			h := handler(drawer, func(d *Deps) {
				if !tc.noScorer {
					d.Scorer = autonomy.NewVisionScorer(qaPersistenceAnswer{body: tc.body, err: tc.modelErr})
				}
			})
			if err := h.runQueue(ctx, s, &manifest); err != nil {
				t.Fatalf("scoring must not discard successful paid draw: %v", err)
			}
			// Inspect the serialized checkpoint, not the local version under construction.
			persisted := loadManifest(t, s)
			item := sceneItem(t, persisted, "sc-0")
			if len(drawer.sent()) != 1 || len(item.Versions) != 2 || item.Status != "review" || item.InFlight != nil || item.Error != "" || item.CurrentVersion == nil || *item.CurrentVersion != 1 || item.ReviewVersion == nil || *item.ReviewVersion != 2 {
				t.Fatalf("paid draw/history not preserved: draws=%d item=%+v", len(drawer.sent()), item)
			}
			if !reflect.DeepEqual(item.Versions[0], before) {
				t.Fatalf("old version or its score changed: got=%+v want=%+v", item.Versions[0], before)
			}
			candidate := Candidate(item)
			if candidate == nil || candidate.State() != VersionReady || candidate.Path == "" || candidate.SHA256 == "" || candidate.Receipt == nil || candidate.Receipt.Provider != "codex" || candidate.CostImages != 1 || candidate.VersionError != nil {
				t.Fatalf("successful paid image became an error: %+v", candidate)
			}
			if (candidate.ScoreIssue != "") != tc.wantIssue {
				t.Fatalf("manual-review issue=%q wantIssue=%v", candidate.ScoreIssue, tc.wantIssue)
			}
			if tc.wantScore == 0 {
				if candidate.Score != nil {
					t.Fatalf("failed scoring fabricated a score: %+v", candidate.Score)
				}
			} else if candidate.Score == nil || candidate.Score.Value != tc.wantScore {
				t.Fatalf("score=%+v want=%d", candidate.Score, tc.wantScore)
			}
			artifact, err := media.Acquire(ctx, candidate.Path)
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := os.ReadFile(artifact.Path)
			artifact.Release()
			if readErr != nil || string(data) != "webp bytes" {
				t.Fatalf("reviewable bytes lost: %q %v", data, readErr)
			}
			if tc.wantIssue && !ImagesReady(persisted) {
				t.Fatal("unapplied QA must leave images available to owner review")
			}
			if tc.wantIssue && autonomy.Evaluate(domain.AutoApproveMin, []autonomy.ItemScore{{ID: item.ID, Score: candidate.Score}}).AllPass {
				t.Fatal("unverified candidate became auto-approvable")
			}
			// The model/scoring path must not mutate the snapshotted job preference.
			if s.Job.Instructions.InstructionFor("qa") != tc.note {
				t.Fatal("scoring rewrote immutable job instructions")
			}
		})
	}
}
