package images

import (
	"context"
	"encoding/json"
	"testing"
)

type todayStore struct{ *editStore }

func (s *todayStore) ApprovedToday(_ context.Context, channelID string) (int, error) {
	if channelID != "" {
		panic("review total must cover all channels")
	}
	return 6, nil
}

func TestReviewViewExposesApprovedToday(t *testing.T) {
	s, store := testService()
	s.Edits.Store = &todayStore{store}
	view, err := s.Read(context.Background(), "job")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(view)
	var wire struct {
		ApprovedToday *int `json:"approvedToday"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.ApprovedToday == nil || *wire.ApprovedToday != 6 {
		t.Fatalf("wrong approval count: %s", raw)
	}
}
