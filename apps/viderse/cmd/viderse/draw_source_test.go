package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	imagesmodel "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// manifestStore trả manifest ảnh dựng sẵn cho mọi lần đọc.
type manifestStore struct{ raw json.RawMessage }

func (s manifestStore) Read(context.Context, string, domain.CloneStep) (jobedit.Snapshot, error) {
	return jobedit.Snapshot{
		Job:      domain.CloneJob{ID: "job", Generation: 1},
		Step:     domain.CloneStepRecord{Step: domain.CloneImages, Status: domain.CloneSucceeded, Checkpoint: s.raw},
		Manifest: s.raw,
	}, nil
}
func (s manifestStore) Commit(context.Context, jobedit.Snapshot, jobedit.Change) error { return nil }

func TestWithVideoDrawSourcePutsTheOverrideFirst(t *testing.T) {
	global := []domain.ModelRef{
		{Provider: "prov-chatgpt", Model: "gpt-image-2"},
		{Provider: "prov-grok", Model: "grok-imagine-image"},
	}
	manifest := imagesmodel.Manifest{Version: 1, Revision: 4, Overrides: &imagesmodel.Overrides{
		DrawProvider: "prov-grok", DrawModel: "grok-imagine-image"}}
	raw, _ := json.Marshal(manifest)
	edits := &jobedit.Service{Store: manifestStore{raw: raw}}
	got := withVideoDrawSource(context.Background(), edits, domain.CloneJob{ID: "job"}, global)
	if len(got) != 2 || got[0] != (domain.ModelRef{Provider: "prov-grok", Model: "grok-imagine-image"}) ||
		got[1] != (domain.ModelRef{Provider: "prov-chatgpt", Model: "gpt-image-2"}) {
		t.Fatalf("chuỗi vẽ = %+v", got)
	}
}

// Video không có nguồn riêng (hoặc video khác) giữ nguyên chuỗi chung.
func TestWithVideoDrawSourceLeavesTheGlobalChainAlone(t *testing.T) {
	global := []domain.ModelRef{{Provider: "prov-chatgpt", Model: "gpt-image-2"}}
	for name, raw := range map[string]json.RawMessage{
		"không có manifest": nil,
		"không có override": func() json.RawMessage {
			b, _ := json.Marshal(imagesmodel.Manifest{Version: 1, Revision: 2})
			return b
		}(),
	} {
		edits := &jobedit.Service{Store: manifestStore{raw: raw}}
		got := withVideoDrawSource(context.Background(), edits, domain.CloneJob{ID: "job"}, global)
		if len(got) != 1 || got[0] != global[0] {
			t.Fatalf("%s: chuỗi vẽ = %+v", name, got)
		}
	}
}
