package web

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
)

// GAP-28：GET S/diag trả generatedAt, stats, findings (có fingerprint), actions (kèm sent) và sections.
func TestDiagJSONShape(t *testing.T) {
	h := newLiveFakeHost()
	finding := diag.Finding{Rule: "StaleForeshadow", Category: diag.CatPlanning, Severity: diag.SevWarning, Confidence: diag.ConfHigh, AutoLevel: diag.AutoSafe, Target: "runtime.planning", Title: "Phục bút trì trệ: 2 mục", Evidence: "Phục bút gieo ở chương 4, 7 chưa được đẩy tiếp.", Suggestion: "Cập nhật phục bút", Technical: "foreshadow_id=fs_secret"}
	fingerprint := diag.FindingFingerprint(finding)
	h.report = diag.Report{
		Stats:    diag.Stats{CompletedChapters: 2, TotalChapters: 10, TotalWords: 5000, AvgWordsPerCh: 2500, Phase: "writing", Flow: "writing", PlanningTier: "short", ReviewCount: 1, RewriteCount: 1, AvgReviewScore: 82.5, ForeshadowOpen: 3, ForeshadowStale: 2},
		Findings: []diag.Finding{finding},
		Actions:  []diag.Action{{SourceRule: finding.Rule, Kind: diag.ActionEnqueueFollowUp, Severity: finding.Severity, Summary: "Kiểm tra phục bút", Message: "Hãy cập nhật phục bút", Fingerprint: fingerprint}},
	}
	s := newLiveServer(h)

	v := decodeLiveBody(t, doReq(t, s, "GET", "/api/diag", ""))
	for _, key := range []string{"generatedAt", "stats", "findings", "actions", "sections"} {
		if _, ok := v[key]; !ok {
			t.Fatalf("missing %q: %v", key, v)
		}
	}
	stats := v["stats"].(map[string]any)
	for key, want := range map[string]any{"completedChapters": float64(2), "totalChapters": float64(10), "totalWords": float64(5000), "phase": "writing", "planningTier": "short", "reviewCount": float64(1), "foreshadowStale": float64(2)} {
		if stats[key] != want {
			t.Fatalf("stats[%s] = %v, want %v (%v)", key, stats[key], want, stats)
		}
	}
	for _, key := range []string{"startedAt", "currentVolume", "totalVolumes", "avgReviewScore", "foreshadowOpen", "rewriteCount", "flow", "avgWordsPerChapter"} {
		if _, ok := stats[key]; !ok {
			t.Fatalf("stats missing %q: %v", key, stats)
		}
	}
	f := v["findings"].([]any)[0].(map[string]any)
	if f["fingerprint"] != fingerprint || f["rule"] != "StaleForeshadow" || f["category"] != "planning" || f["severity"] != string(diag.SevWarning) || f["title"] != finding.Title || f["evidence"] != finding.Evidence || f["suggestion"] != "Cập nhật phục bút" || f["technical"] != finding.Technical {
		t.Fatalf("finding = %v", f)
	}
	for _, key := range []string{"confidence", "autoLevel", "target"} {
		if _, ok := f[key]; !ok {
			t.Fatalf("finding missing %q: %v", key, f)
		}
	}
	a := v["actions"].([]any)[0].(map[string]any)
	if a["fingerprint"] != fingerprint || a["sourceRule"] != "StaleForeshadow" || a["summary"] != "Kiểm tra phục bút" || a["message"] != "Hãy cập nhật phục bút" || a["sent"] != false {
		t.Fatalf("action = %v", a)
	}
	if _, ok := a["kind"]; !ok {
		t.Fatalf("action missing kind: %v", a)
	}
}
