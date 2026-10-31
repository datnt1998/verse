package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type sourceViewStore struct {
	ServiceStore
	job   domain.CloneJob
	steps []domain.CloneStepRecord
}

func (s *sourceViewStore) GetJob(context.Context, string) (*domain.CloneJob, error) {
	return &s.job, nil
}
func (s *sourceViewStore) ListSteps(context.Context, string) ([]domain.CloneStepRecord, error) {
	return s.steps, nil
}
func (s *sourceViewStore) SourceRival(_ context.Context, url string) (*domain.CloneSourceSnapshot, error) {
	if url != "https://www.youtube.com/watch?v=abcdefghijk" {
		return nil, nil
	}
	return &domain.CloneSourceSnapshot{Outlier: new(4.2), ViewCount: new(int64(1200000)), PublishedAt: new(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))}, nil
}
func TestJobSourceEnrichment(t *testing.T) {
	store := &sourceViewStore{job: domain.CloneJob{Source: domain.CloneSourceSnapshot{URL: "https://www.youtube.com/watch?v=abcdefghijk"}}}
	view, err := (&Service{Store: store}).Get(context.Background(), "job")
	if err != nil {
		t.Fatal(err)
	}
	if view.Source.Outlier == nil || *view.Source.Outlier != 4.2 || view.Source.ViewCount == nil || *view.Source.ViewCount != 1200000 || view.Source.PublishedAt == nil {
		t.Fatalf("tracked rival metrics missing: %+v", view.Source)
	}
	store.job.Source.URL = "https://example.com/untracked"
	view, err = (&Service{Store: store}).Get(context.Background(), "job")
	if err != nil || view.Source.Outlier != nil || view.Source.ViewCount != nil {
		t.Fatalf("untracked source gained metrics: %+v %v", view.Source, err)
	}
}
