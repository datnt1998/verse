package autonomy_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/core/llm"
)

// Fixed remote-boundary responses exercise the real schema, parser, applicability
// decision and weighted score. They neither inspect nor echo the request.
type qaContractAnswer struct {
	body string
	err  error
}

func (m qaContractAnswer) Generate(context.Context, []llm.Message, []llm.ToolSpec, ...llm.CallOption) (*llm.LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.LLMResponse{Message: llm.Message{
		Role:       llm.RoleAssistant,
		Content:    []llm.ContentBlock{llm.TextBlock(m.body)},
		StopReason: llm.StopReasonStop,
	}}, nil
}

func qaContractBody(t *testing.T, changes map[string]any, omit ...string) string {
	t.Helper()
	answer := map[string]any{
		"match": 100, "artifacts": 100, "identity": 100, "duplicate": 100,
		"reason":                "Nhân vật rõ, khớp cảnh và không có lỗi hình.",
		"facts":                 []map[string]string{{"label": "Nhân vật", "value": "Đứng giữa cảnh"}},
		"instructionApplicable": true, "instructionIssue": "",
	}
	for key, value := range changes {
		answer[key] = value
	}
	for _, key := range omit {
		delete(answer, key)
	}
	raw, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestQAContractRejectsUnappliedOrUnverifiedInstructionEvenAt100(t *testing.T) {
	for _, tc := range []struct {
		name    string
		note    string
		changes map[string]any
		omit    []string
		body    string
	}{
		{name: "music false with 100", note: "Kiểm tra nhạc nền không lấn lời kể", changes: map[string]any{"instructionApplicable": false, "instructionIssue": "Chưa áp dụng phần kiểm nhạc nền vì chỉ có ảnh."}},
		{name: "false empty issue", note: "Kiểm tra giọng đọc", changes: map[string]any{"instructionApplicable": false}},
		{name: "true with outstanding issue", note: "Giữ đúng nhân vật", changes: map[string]any{"instructionIssue": "Chưa xác minh được nhân vật."}},
		{name: "missing applicability", note: "Giữ đúng nhân vật", omit: []string{"instructionApplicable"}},
		{name: "null applicability", note: "Giữ đúng nhân vật", changes: map[string]any{"instructionApplicable": nil}},
		{name: "string applicability", note: "Giữ đúng nhân vật", changes: map[string]any{"instructionApplicable": "true"}},
		{name: "numeric applicability", note: "Giữ đúng nhân vật", changes: map[string]any{"instructionApplicable": 1}},
		{name: "missing issue field", note: "Giữ đúng nhân vật", omit: []string{"instructionIssue"}},
		{name: "null issue field", note: "Giữ đúng nhân vật", changes: map[string]any{"instructionIssue": nil}},
		{name: "missing numeric part", note: "Giữ đúng nhân vật", omit: []string{"match"}},
		{name: "numeric out of range", note: "Giữ đúng nhân vật", changes: map[string]any{"match": 101}},
		{name: "missing reason", note: "Giữ đúng nhân vật", omit: []string{"reason"}},
		{name: "malformed JSON", note: "Giữ đúng nhân vật", body: `{"match":100,"instructionApplicable":true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if body == "" {
				body = qaContractBody(t, tc.changes, tc.omit...)
			}
			s := autonomy.NewVisionScorer(qaContractAnswer{body: body})
			got, err := s.Score(context.Background(), autonomy.ScoreInput{ItemID: "scene-1", Instruction: tc.note})
			var issue *autonomy.InstructionIssue
			if got != nil || !errors.As(err, &issue) || issue.Error() == "" {
				t.Fatalf("unsafe result: score=%+v err=%v", got, err)
			}
			verdict := autonomy.Evaluate(domain.AutoApproveMin, []autonomy.ItemScore{{ID: "scene-1", Score: got}})
			if verdict.AllPass || verdict.Passed != 0 || len(verdict.Below) != 1 || verdict.Below[0].Value != -1 {
				t.Fatalf("unapplied instruction entered approval: %+v", verdict)
			}
		})
	}
}

func TestQAContractSupportedInstructionRetainsRealWeightedScore(t *testing.T) {
	fixed := time.Date(2026, 11, 4, 1, 2, 3, 0, time.UTC)
	s := autonomy.NewVisionScorer(qaContractAnswer{body: qaContractBody(t, map[string]any{"match": 60})})
	s.Now = func() time.Time { return fixed }
	got, err := s.Score(context.Background(), autonomy.ScoreInput{ItemID: "scene-1", Kind: "scene", Instruction: "Giữ đúng nhân vật và bối cảnh"})
	if err != nil || got == nil {
		t.Fatalf("score=%+v err=%v", got, err)
	}
	if got.Value != 86 || got.Parts != (domain.ImageScoreParts{Match: 60, Artifacts: 100, Identity: 100, Duplicate: 100}) || got.At != fixed.Format(time.RFC3339) || got.Reason.Agent != domain.AgentQA {
		t.Fatalf("supported instruction changed scoring rules: %+v", got)
	}
	if !autonomy.Evaluate(domain.AutoApproveMin, []autonomy.ItemScore{{Score: got}}).AllPass {
		t.Fatal("supported 86-point image should pass image auto-approval")
	}
}

func TestQAContractNoNotePreservesLegacyScore(t *testing.T) {
	body := qaContractBody(t, map[string]any{"match": 60}, "instructionApplicable", "instructionIssue")
	fixed := time.Date(2026, 11, 4, 1, 2, 3, 0, time.UTC)
	s := autonomy.NewVisionScorer(qaContractAnswer{body: body})
	s.Now = func() time.Time { return fixed }
	baseline, err := s.Score(context.Background(), autonomy.ScoreInput{ItemID: "scene-1"})
	if err != nil || baseline == nil || baseline.Value != 86 {
		t.Fatalf("legacy result=%+v err=%v", baseline, err)
	}
	for _, note := range []string{"", " \n\t "} {
		got, err := s.Score(context.Background(), autonomy.ScoreInput{ItemID: "scene-1", Instruction: note})
		if err != nil || !reflect.DeepEqual(got, baseline) {
			t.Fatalf("no-note score changed: got=%+v want=%+v err=%v", got, baseline, err)
		}
	}
}
