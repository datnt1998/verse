package arbiter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	storepkg "github.com/datnt1998/noverse/internal/store"
)

func TestInterventionReceivesDiagnosticContext(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	// Progress may retain an estimate; diagnostic scopes must use the actual outline.
	if err := st.Progress.Init(40); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	outline := make([]domain.OutlineEntry, 28)
	for i := range outline {
		outline[i] = domain.OutlineEntry{Chapter: i + 1, Title: "Chương", HookType: "lat_thong_tin"}
	}
	if err := st.Outline.SaveOutline(outline); err != nil {
		t.Fatal(err)
	}
	if err := st.World.SaveReviewDecision("review-16", storepkg.ReviewDecision{State: "kept", Ref: "Master §7", Text: "Móc lặp", Reason: "Giữ nhịp truyện", Chapter: 16}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Decisions.Append(storepkg.DecisionRecord{Kind: "intervention", Input: "Góp ý sửa §8: chỉ đồng bộ nhãn chương 16, không sửa chính văn"}); err != nil {
		t.Fatal(err)
	}
	facts, err := CollectInterventionFacts(st)
	if err != nil {
		t.Fatal(err)
	}
	model := &scriptedModel{outputs: []string{`{"answer":"Đã nhận","rules":null,"hold":null,"reopen":null,"ask":null,"dispatch":null,"reason":"phân luồng"}`}}
	if _, _, err := DecideIntervention(t.Context(), model, "sys", facts, "sửa theo quyết định ở chẩn đoán, xử lý cảnh báo §8"); err != nil {
		t.Fatal(err)
	}
	payload := model.lastMsgs[len(model.lastMsgs)-1].TextContent()
	var input struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal([]byte(payload), &input); err != nil {
		t.Fatal(err)
	}
	var summary string
	if err := json.Unmarshal(input.Facts["diagnostic_context"], &summary); err != nil {
		t.Fatalf("missing diagnostic context in coordinator turn: %v", err)
	}
	for _, want := range []string{"Chẩn đoán hiện tại", "§8", "MidpointTwistPosition", "13–16", "architect_long", "read_chapter", "source=final", "revise_outline", "reconcile_completed=true", "Giữ nguyên", "Master §7", "Giữ nhịp truyện", "review-16", "Góp ý sửa §8: chỉ đồng bộ nhãn chương 16, không sửa chính văn"} {
		if !strings.Contains(summary, want) {
			t.Errorf("diagnostic summary missing %q: %s", want, summary)
		}
	}
	// References are rule references, never positions in a severity-sorted list.
	facts2, err := CollectInterventionFacts(st)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(facts)
	second, _ := json.Marshal(facts2)
	if string(first) != string(second) {
		t.Fatal("diagnostic references changed without a story change")
	}
}
