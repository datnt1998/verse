package images

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

// /provider: nguồn vẽ riêng nằm trong manifest của CHÍNH video, ảnh đã vẽ giữ
// nguyên và không bước nào bị chạy lại.
func TestSetDrawSourceKeepsDrawnImagesAndStoresTheOverride(t *testing.T) {
	s, store := testService()
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	if err := s.SetDrawSource(context.Background(), "job", "prov-grok", "grok-imagine-image"); err != nil {
		t.Fatalf("SetDrawSource: %v", err)
	}
	if store.m.Overrides == nil || store.m.Overrides.DrawProvider != "prov-grok" || store.m.Overrides.DrawModel != "grok-imagine-image" {
		t.Fatalf("overrides = %+v", store.m.Overrides)
	}
	if store.change.InvalidateFrom != "" || store.change.Rerun {
		t.Fatalf("đổi nguồn vẽ đã chạy lại bước: %+v", store.change)
	}
	if len(store.m.Items) != 2 || len(store.m.Items[0].Versions) != 1 {
		t.Fatalf("ảnh đã vẽ bị đụng: %+v", store.m.Items)
	}
	// Đổi lần nữa ghi đè, không nhân bản.
	if err := s.SetDrawSource(context.Background(), "job", "prov-gemini", "gemini-2.5-flash-image"); err != nil {
		t.Fatalf("SetDrawSource lần hai: %v", err)
	}
	if store.m.Overrides.DrawProvider != "prov-gemini" || store.m.Overrides.DrawModel != "gemini-2.5-flash-image" {
		t.Fatalf("overrides = %+v", store.m.Overrides)
	}
}

// pendingStore là video chưa chạy bước ảnh: chưa có manifest nào.
type pendingStore struct{ change jobedit.Change }

func (s *pendingStore) Read(context.Context, string, domain.CloneStep) (jobedit.Snapshot, error) {
	return jobedit.Snapshot{
		Job:  domain.CloneJob{ID: "job", Generation: 1},
		Step: domain.CloneStepRecord{Step: domain.CloneImages, Status: domain.ClonePending},
	}, nil
}

func (s *pendingStore) Commit(_ context.Context, _ jobedit.Snapshot, c jobedit.Change) error {
	s.change = c
	return nil
}

func TestSetDrawSourceWorksBeforeTheImagesStepRan(t *testing.T) {
	store := &pendingStore{}
	s := &Service{Edits: &jobedit.Service{Store: store, Media: &stepstest.FakeMedia{Root: t.TempDir()}}}
	if err := s.SetDrawSource(context.Background(), "job", "prov-grok", "grok-imagine-image"); err != nil {
		t.Fatalf("SetDrawSource: %v", err)
	}
	if store.change.InvalidateFrom != "" || store.change.Rerun {
		t.Fatalf("đổi nguồn vẽ đã chạy lại bước: %+v", store.change)
	}
	var saved model.Manifest
	if err := json.Unmarshal(store.change.Checkpoint, &saved); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if saved.SceneRevision != -1 || saved.Overrides == nil || saved.Overrides.DrawProvider != "prov-grok" {
		t.Fatalf("manifest = %+v overrides = %+v", saved, saved.Overrides)
	}
}
