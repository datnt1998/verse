package metadata

import (
	"reflect"
	"testing"
)

func TestRewriteOnlySelectedParts(t *testing.T) {
	old := Content{Title: "old", Description: "untouched\nexact", Tags: []string{"one", "two"}}
	fresh := Content{Title: "new", Description: "different", Tags: []string{"other"}}
	got := merge(old, fresh, []string{"title"})
	if got.Title != "new" || got.Description != old.Description || !reflect.DeepEqual(got.Tags, old.Tags) {
		t.Fatalf("unexpected merge: %#v", got)
	}
}

func TestPatchStoresPreviousAndRejectsStaleRevision(t *testing.T) {
	s := State{Revision: 4, Current: Content{Title: "old", Description: "desc", Tags: []string{"tag"}}}
	title := "new"
	if err := s.patch(Patch{ExpectedRevision: new(3), Title: &title}); err == nil {
		t.Fatal("accepted stale revision")
	}
	if err := s.patch(Patch{ExpectedRevision: new(4), Title: &title}); err != nil {
		t.Fatal(err)
	}
	if len(s.History) != 1 || s.History[0].Title != "old" || s.Current.Title != "new" || s.Revision != 5 {
		t.Fatalf("bad history: %#v", s)
	}
}
