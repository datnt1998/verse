package images

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestHistoryExposesExactReferenceArtifactsWithoutArtistQuota(t *testing.T) {
	service, store := versionService(t)
	service.Quotas = func(context.Context) ([]ProviderQuota, error) {
		return nil, errors.New("configured artist unavailable")
	}
	at := 1.25
	store.m.Items[0].Versions[0].Refs = []model.ImageRef{
		{ID: "upload:one", Role: model.RefCharacter, Path: "images/references/upload.png", SHA256: "upload-sha", Source: model.RefSource{Type: "upload", Label: "Upload", ImageID: "one"}},
		{ID: "rival:source:1.25", Role: model.RefLayout, Path: "images/references/frame.png", SHA256: "frame-sha", Source: model.RefSource{Type: "rival_frame", Label: "Frame", VideoID: "source", AtSec: &at}},
	}
	store.artifacts = []domain.CloneArtifact{
		{ID: "wrong-upload", Path: "clone/job-1/g/images/references/upload.png", SHA256: "old-sha"},
		{ID: "upload", Path: "clone/job-1/g/images/references/upload.png", SHA256: "upload-sha"},
		{ID: "frame", Path: "clone/job-1/g/images/references/frame.png", SHA256: "frame-sha"},
	}
	view, err := service.Read(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Quota != nil || len(view.Images) != 2 || len(view.Images[0].Versions) != 1 {
		t.Fatalf("history lost without quota: %+v", view)
	}
	refs := view.Images[0].Versions[0].Refs
	if len(refs) != 2 || refs[0].URL != "/api/clone/job-1/artifacts/upload" || refs[1].URL != "/api/clone/job-1/artifacts/frame" {
		t.Fatalf("reference URLs=%+v", refs)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Quota json.RawMessage `json:"quota"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || string(wire.Quota) != "null" {
		t.Fatalf("quota=%s err=%v", wire.Quota, err)
	}
}
