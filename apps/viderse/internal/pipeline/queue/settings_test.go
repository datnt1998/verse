package queue

import "testing"

func TestStoredRuleNumberRejectsMalformedAndOutOfRange(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `"4"`, `1.5`, `0`, `999`, `{}`} {
		if got := ruleNumber([]byte(raw), 3, 1, 6); got != 3 {
			t.Errorf("raw=%s got=%d want default=3", raw, got)
		}
	}
	if got := ruleNumber([]byte(`5`), 3, 1, 6); got != 5 {
		t.Fatalf("valid stored integer ignored: %d", got)
	}
}
