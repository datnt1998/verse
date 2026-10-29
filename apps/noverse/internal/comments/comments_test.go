package comments

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func ptr(n int) *int { return &n }
func TestCRUDAndIndependentLease(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ainovel.lock"), []byte("TUI"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(dir)
	empty, err := s.List(1)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %v, %v", empty, err)
	}
	c, err := s.Add(1, Input{Paragraph: ptr(2), Quote: "Trích đoạn", Chips: []string{Chips[0]}, Text: "Giữ ý chính"}, "dat")
	if err != nil {
		t.Fatal(err)
	}
	list, err := New(dir).List(1)
	if err != nil || len(list) != 1 || list[0].ID != c.ID || list[0].Author != "dat" || list[0].Status != StatusPending || list[0].SentAt != nil {
		t.Fatalf("persisted = %+v, %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ainovel", "gop-y", "1.json")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(1, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(1, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing = %v", err)
	}
	list, err = s.List(1)
	if err != nil || len(list) != 0 {
		t.Fatalf("deleted = %v, %v", list, err)
	}
	for _, in := range []Input{{}, {Paragraph: ptr(-1), Quote: "x", Text: "x"}, {Paragraph: ptr(0), Text: "x"}, {Text: "x", Chips: []string{"khác"}}} {
		if _, err := s.Add(1, in, "dat"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid %+v = %v", in, err)
		}
	}
	if _, err := s.List(0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("chapter validation = %v", err)
	}
}

func TestComposeOrdering(t *testing.T) {
	now := time.Now()
	list := []Comment{
		{ID: "whole", Text: "Sửa cả chương", CreatedAt: now, Status: StatusPending},
		{ID: "later", Paragraph: ptr(4), Quote: "Bốn", Chips: []string{Chips[1]}, CreatedAt: now, Status: StatusPending},
		{ID: "second", Paragraph: ptr(1), Quote: "Hai sau", Text: "Yêu cầu sau", CreatedAt: now.Add(time.Second), Status: StatusPending},
		{ID: "first", Paragraph: ptr(1), Quote: "Hai trước", Text: "Yêu cầu trước", CreatedAt: now, Status: StatusPending},
		{ID: "applied", Paragraph: ptr(0), Text: "Không lặp", Status: StatusSent, SentAt: &now, CreatedAt: now},
	}
	got := Compose(8, list)
	last := -1
	for _, text := range []string{"sửa chương 8", "Hai trước", "Yêu cầu trước", "Hai sau", "Yêu cầu sau", "Bốn", "Thêm thoại", "cả chương", "Sửa cả chương"} {
		at := strings.Index(got, text)
		if at <= last {
			t.Fatalf("compose order %q at %d after %d: %s", text, at, last, got)
		}
		last = at
	}
	for _, id := range []string{"first", "second", "later", "whole"} {
		if !strings.Contains(got, "Góp ý "+id+":") {
			t.Fatalf("compose missing comment id %q: %s", id, got)
		}
	}
	if !strings.Contains(got, "fixed_comments") {
		t.Fatal("compose must instruct the writer to report fixed_comments")
	}
	if strings.Contains(got, "Không lặp") {
		t.Fatal("already sent comment repeated")
	}
	if list[0].ID != "whole" {
		t.Fatal("Compose mutated caller order")
	}
}

func TestApplyMarksOnlyAfterSuccessfulDispatch(t *testing.T) {
	s := New(t.TempDir())
	c, err := s.Add(1, Input{Text: "Giữ giọng văn"}, "dat")
	if err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("cổng chưa sẵn sàng")
	count, err := s.Apply(1, func(instruction string) error {
		if !strings.Contains(instruction, "Giữ giọng văn") {
			t.Fatalf("dispatch = %q", instruction)
		}
		return rejected
	})
	if !errors.Is(err, rejected) || count != 0 {
		t.Fatalf("failed apply = %d, %v", count, err)
	}
	list, _ := s.List(1)
	if list[0].Status != StatusPending || list[0].SentAt != nil {
		t.Fatal("failed dispatch marked feedback sent")
	}
	count, err = s.Apply(1, func(string) error { return nil })
	if err != nil || count != 1 {
		t.Fatalf("apply = %d,%v", count, err)
	}
	list, _ = s.List(1)
	if list[0].ID != c.ID || list[0].Status != StatusSent || list[0].SentAt == nil {
		t.Fatal("successful dispatch did not mark feedback sent")
	}
	count, err = s.Apply(1, func(instruction string) error {
		if instruction != "" {
			t.Fatal("repeated dispatch")
		}
		return nil
	})
	if err != nil || count != 0 {
		t.Fatalf("repeat apply = %d,%v", count, err)
	}
}

func TestConcurrentAddsPreserveEveryComment(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := New(dir).Add(2, Input{Text: "Thêm thoại"}, "dat"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	list, err := New(dir).List(2)
	if err != nil || len(list) != 12 {
		t.Fatalf("concurrent comments = %d, %v", len(list), err)
	}
}
