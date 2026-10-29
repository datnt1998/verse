package notify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCloseRejectsLateProducerWrites(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(Options{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// A cancelled provider can still publish one final event during shutdown.
	if err = s.Record("reader", PushNotification{Type: "security", Title: "Thiết bị mới"}); err == nil {
		t.Fatal("late producer wrote after Close")
	}
	if _, err = os.Stat(filepath.Join(dir, "state.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("late producer created a state file: %v", err)
	}
}
