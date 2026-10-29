package arbiter

import "testing"

func TestWorkerQuestionCannotRemainFreeText(t *testing.T) {
	facts := InterventionFacts{Phase: "writing", WorkerQuestion: true, AskUserEnabled: true}
	decision := InterventionDecision{Answer: "Bạn muốn tôi giữ hay cắt?", Reason: "Cần chọn hướng đi"}
	if err := decision.ValidateAgainst(facts); err == nil {
		t.Fatal("worker ambiguity must produce structured ask choices, not another free-text answer")
	}
}
