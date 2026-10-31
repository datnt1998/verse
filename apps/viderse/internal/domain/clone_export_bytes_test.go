package domain

import "testing"

func TestCloneExportBytesUsesOnlyPresentExport(t *testing.T) {
 steps := []CloneStepRecord{{Step:CloneExport, Status:CloneSucceeded, Output:&CloneStepOutput{Data:map[string]any{"size":float64(4200)}}}}
 if got := CloneExportBytes(steps); got == nil || *got != 4200 { t.Fatalf("got %v", got) }
 steps[0].Output.Data["removedAt"] = "2026-01-01T00:00:00Z"
 if got := CloneExportBytes(steps); got != nil { t.Fatalf("removed export still has size %v", *got) }
}
