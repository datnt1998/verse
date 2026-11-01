package images

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestRedrawProjectOwnershipSurvivesRestart(t *testing.T) {
	no, yes := false, true
	for _, tc := range []struct {
		name    string
		project *bool
		apply   bool
	}{{"review-default", nil, true}, {"editor", &no, false}, {"explicit-server", &yes, true}} {
		for _, restart := range []bool{false, true} {
			name := tc.name
			if restart {
				name += "/restart"
			}
			t.Run(name, func(t *testing.T) {
				s, _, _, _ := sceneFixture(t, "sc-0")
				m := loadManifest(t, s)
				item := &m.Items[0]
				item.Status = "pending"
				item.Draft = &PromptDraft{Project: tc.project}
				if restart {
					item.InFlight = &Flight{Project: tc.project, GenerationID: "ownership-restart", Version: 1, Prompt: "Giữ máy quay và chuyển động", PromptVersion: item.PromptVersion, SourceFingerprint: SourceFingerprint(*item), PNG: "images/flights/sc-0/ownership-restart.png", Parts: PromptParts{{Kind: PartScene, Text: "Giữ máy quay và chuyển động"}}, Provider: "codex", StartedAt: "2026-11-02T10:00:00Z", BaseStatus: "pending"}
					item.Draft = nil // Only the durable flight survives recovery.
					recordDrawing(item, *item.InFlight)
				}
				m.QueueOnly = []string{"sc-0"}
				if err := s.SaveCheckpoint(context.Background(), m); err != nil {
					t.Fatal(err)
				}
				var calls []RebindRequest
				h := handler(&recorderDrawer{}, func(d *Deps) {
					d.Built = func(context.Context, domain.CloneJob) (string, bool, error) { return "project", true, nil }
					d.Rebind = func(_ context.Context, _ domain.CloneJob, req RebindRequest) error {
						calls = append(calls, req)
						return nil
					}
				})
				if _, err := h.Run(context.Background(), s); err != nil {
					t.Fatal(err)
				}
				if len(calls) != 1 || calls[0].ApplyProject != tc.apply {
					t.Fatalf("project ownership lost: %+v", calls)
				}
				v := loadManifest(t, s).Items[0].Versions[0]
				if (v.Project == nil) != (tc.project == nil) || (v.Project != nil && *v.Project != *tc.project) {
					t.Fatalf("durable version ownership=%v want=%v", v.Project, tc.project)
				}
			})
		}
	}
}
