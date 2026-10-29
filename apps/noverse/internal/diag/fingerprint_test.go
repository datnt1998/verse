package diag

import (
	"regexp"
	"testing"
)

// GAP-28：指纹是 POST …/diag/actions/:fingerprint 的稳定键。
func TestFindingFingerprintStableAndCarriedByActions(t *testing.T) {
	f := Finding{Rule: "OrphanedSteer", Target: "runtime.flow", Title: "Có chỉ thị chuyển hướng chưa được xử lý", Evidence: "steer=abc", Severity: SevWarning, Confidence: ConfHigh, AutoLevel: AutoSafe}
	key := FindingFingerprint(f)
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(key) {
		t.Fatalf("fingerprint %q is not 12 hex chars (URL-safe)", key)
	}
	if again := FindingFingerprint(f); again != key {
		t.Fatalf("fingerprint not stable: %q vs %q", key, again)
	}
	other := f
	other.Evidence = "steer=xyz"
	if FindingFingerprint(other) == key {
		t.Fatal("different evidence must give a different fingerprint")
	}
	other = f
	other.Technical = "pending_steer=xyz"
	if FindingFingerprint(other) == key {
		t.Fatal("different technical evidence must give a different fingerprint")
	}
	actions := PlanActions([]Finding{f})
	if len(actions) == 0 {
		t.Fatal("expected an action")
	}
	for _, a := range actions {
		if a.Fingerprint != key {
			t.Fatalf("action fingerprint %q != finding fingerprint %q", a.Fingerprint, key)
		}
	}
}
