package host

import "testing"

func TestResultHintFromLongChapterResult(t *testing.T) {
	raw := []byte(`{"chapter":5,"word_count":1546,"content":"Bữa trưa hôm ấy bàn dài đặt giữa phòng ăn…"}`)
	if got := resultHint(raw); got != `{"word_count":1546}` {
		t.Fatalf("hint: %q", got)
	}
}

func TestResultHintKeepsIssuesAndCount(t *testing.T) {
	review := []byte(`{"affected_chapters":[2,4,8],"chapter":8,"issues":5,"saved":true}`)
	if got := resultHint(review); got != `{"issues":5}` {
		t.Fatalf("review hint: %q", got)
	}
	foundation := []byte(`{"count":7,"foundation_ready":false,"saved":true,"type":"characters"}`)
	if got := resultHint(foundation); got != `{"count":7}` {
		t.Fatalf("foundation hint: %q", got)
	}
}

func TestResultHintEmptyWhenNoNumbers(t *testing.T) {
	if got := resultHint([]byte(`{"saved":true,"type":"premise"}`)); got != "" {
		t.Fatalf("expected empty hint, got %q", got)
	}
	// `"issues":[]` (audit sạch) không phải con số — không được biến thành 0.
	if got := resultHint([]byte(`{"foundation_ready":true,"issues":[]}`)); got != "" {
		t.Fatalf("expected empty hint for empty issues, got %q", got)
	}
}
