package identity

import (
	"errors"
	"path/filepath"
	"testing"
)

type noSystemAuditRepository struct{ Repository }
type systemAuditRepository struct {
	Repository
	calls int
}

func (r *systemAuditRepository) RecordSystemAudit(action, target, reason string) error {
	r.calls++
	if action != "Chuyển quy tắc" || target != "migration" || reason != "Đã chuyển" {
		return errors.New("wrong audit payload")
	}
	return nil
}

func TestRecordSystemAuditIsIdempotentAndDurable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	s := NewStore(path, Options{})
	for range 2 {
		if err := s.RecordSystemAudit("Chuyển quy tắc", "migration", "Đã chuyển"); err != nil {
			t.Fatal(err)
		}
	}
	s = NewStore(path, Options{})
	page, err := s.Audit(AuditQuery{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("audit=%+v err=%v", page, err)
	}
	entry := page.Items[0]
	if entry.Actor != nil || entry.Type != "perm" || entry.Action != "Chuyển quy tắc" || entry.Target != "migration" || entry.Reason != "Đã chuyển" {
		t.Fatalf("lost maintenance audit payload: %+v", entry)
	}
}

func TestRecordSystemAuditRequiresExplicitRepositoryCapability(t *testing.T) {
	s := NewFromRepository(noSystemAuditRepository{}, Options{})
	if err := s.RecordSystemAudit("x", "y", "z"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported repository: %v", err)
	}
	r := &systemAuditRepository{}
	s = NewFromRepository(r, Options{})
	if err := s.RecordSystemAudit("Chuyển quy tắc", "migration", "Đã chuyển"); err != nil || r.calls != 1 {
		t.Fatalf("delegation calls=%d err=%v", r.calls, err)
	}
}
