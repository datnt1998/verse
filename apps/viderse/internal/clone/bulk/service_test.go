package bulk

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

type bulkJobs struct {
	*pipeline.Service
	calls []string
}

func (j *bulkJobs) Resume(_ context.Context, id string) (*domain.CloneJob, error) {
	j.calls = append(j.calls, id)
	if id == "broken" {
		return nil, errors.New("Không thể thử lại")
	}
	return &domain.CloneJob{ID: id}, nil
}
func TestBulkKeepsPerJobErrorsAndInputOrder(t *testing.T) {
	jobs := &bulkJobs{}
	service := Service{Jobs: jobs}
	result, err := service.Run(context.Background(), Input{Action: "retry", JobIDs: []string{"first", "broken", "last"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 3 || !result.Results[0].OK || result.Results[1].OK || result.Results[1].Error != "Không thể thử lại" || !result.Results[2].OK || len(jobs.calls) != 3 {
		t.Fatalf("result: %+v calls:%v", result, jobs.calls)
	}
}
func TestBulkRejectsUnboundedOrUnknownActions(t *testing.T) {
	service := Service{}
	for _, input := range []Input{{Action: "retry"}, {Action: "retry", JobIDs: make([]string, 101)}, {Action: "unknown", JobIDs: []string{"job"}}} {
		if _, err := service.Run(context.Background(), input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
}
