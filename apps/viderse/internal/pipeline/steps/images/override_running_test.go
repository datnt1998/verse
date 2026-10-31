package images

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

type recordingDrawer struct {
	p provider
	prompts []string
}
func (d *recordingDrawer) Draw(ctx context.Context, r imagegen.Request) (imagegen.Receipt, error) {
	d.prompts = append(d.prompts, r.Prompt)
	return d.p.Draw(ctx, r, nil)
}

func TestWorkerAppliesOverrideBeforeStartingRemainingImages(t *testing.T) {
	s, _ := fixture(t)
	var m Manifest
	if err := json.Unmarshal(s.Checkpoint, &m); err != nil { t.Fatal(err) }
	first := m.Items[0]
	first.ID, first.Kind = "a", "scene"
	second := first
	second.ID = "b"
	drawn := first
	drawn.ID, drawn.Status = "drawn", "review"
	drawn.Versions = []Version{{Version:1, Prompt:"original prompt"}}
	m.Items = []Item{first, second, drawn}
	m.TemplatePrompt, m.ImagePromptMode = "old visual style", "custom"
	m.Story = nil
	if err := s.SaveCheckpoint(context.Background(), m); err != nil { t.Fatal(err) }
	sem := make(chan struct{}, 1)
	s.ImageSlot = func(ctx context.Context, run func(context.Context) error) error {
		sem <- struct{}{}
		defer func(){ <-sem }()
		return run(ctx)
	}
	drawer := &recordingDrawer{}
	originalWrite := s.WriteCheckpoint
	pending, merged := false, false
	s.WriteCheckpoint = func(ctx context.Context, raw json.RawMessage) error {
		var next Manifest
		if err := json.Unmarshal(raw, &next); err != nil { return err }
		if drawer.p.calls == 1 && !pending && !merged {
			for _, item := range next.Items {
				if item.InFlight != nil { pending = true; return pipeline.ErrImageOverridePending }
			}
		}
		return originalWrite(ctx, raw)
	}
	s.DrainImageIntents = func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		if !pending || merged { return raw, nil }
		var next Manifest
		if err := json.Unmarshal(raw, &next); err != nil { return nil, err }
		next.TemplatePrompt = "new visual style"
		for i := range next.Items {
			if next.Items[i].ID == "drawn" { next.Items[i].Status = "pending" }
		}
		next.Revision++
		merged = true
		return json.Marshal(next)
	}
	analyse := func(context.Context, string, []string, []string, string, string, string) (Story, error) {
		return Story{Look:"", Shots: map[string]string{"a":"shot a", "b":"shot b", "drawn":"shot c"}}, nil
	}
	h := Handler{Deps: Deps{Drawer:drawer, Analyse:analyse, Encode:encode}}
	if _, err := h.Run(context.Background(), s); err != nil { t.Fatal(err) }
	if len(drawer.prompts) != 3 || !strings.Contains(drawer.prompts[0], "old visual style") { t.Fatalf("unexpected initial draws: %+v", drawer.prompts) }
	for _, prompt := range drawer.prompts[1:] {
		if !strings.Contains(prompt, "new visual style") { t.Fatalf("remaining image used stale prompt: %s", prompt) }
	}
	if err := json.Unmarshal(s.Checkpoint, &m); err != nil { t.Fatal(err) }
	if len(m.Items[2].Versions) != 2 || m.Items[2].Versions[0].Prompt != "original prompt" { t.Fatalf("prior drawn version changed: %+v", m.Items[2].Versions) }
}
