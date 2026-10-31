package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type approvalCountsStore struct { ServiceStore; err error }
func (s approvalCountsStore) CountBoardGroups(context.Context) (map[[2]string]int64, error) { return nil, nil }
func (s approvalCountsStore) CountBoardGroupsToday(context.Context, time.Time) (map[string]int64, error) { return nil, nil }
func (s approvalCountsStore) ApprovedToday(context.Context, string) (int, error) { return 6, s.err }

func TestCloneCountsIncludeApprovalTotal(t *testing.T) {
	s := Service{Store: approvalCountsStore{}}
	counts, err := s.Counts(context.Background())
	if err != nil { t.Fatal(err) }
	raw, _ := json.Marshal(counts)
	var result struct { ApprovedToday *int `json:"approvedToday"` }
	if err := json.Unmarshal(raw, &result); err != nil { t.Fatal(err) }
	if result.ApprovedToday == nil || *result.ApprovedToday != 6 { t.Fatalf("missing approval total: %s", raw) }
	s.Store = approvalCountsStore{err: errors.New("db unavailable")}
	if _, err := s.Counts(context.Background()); err == nil { t.Fatal("ignored approval count error") }
}
