package serverstorage_test

import (
	"context"
	"os"
	"testing"
)

func TestGarageLostKeyShowsCorruptWithoutRegenerationAndStillAllowsTest(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	saveGarage(t, f)
	ctx := context.Background()
	original, err := os.ReadFile(f.box.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.box.Path()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.manager.Snapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Storage.EncryptionKey.OK || snapshot.Storage.EncryptionKey.Issue != "corrupt" {
		t.Fatalf("lost key hidden behind healthy replacement: %+v", snapshot.Storage.EncryptionKey)
	}
	if _, err := os.Stat(f.box.Path()); !os.IsNotExist(err) {
		t.Fatal("missing key silently regenerated despite retained encrypted descriptor")
	}
	config := garageConfig(t)
	tested, err := f.manager.Test(ctx, "admin", config)
	if err != nil || tested.Error != nil {
		t.Fatalf("explicit candidate could not be tested with lost key: %+v %v", tested, err)
	}
	if _, err := f.manager.Save(ctx, "admin", config, tested.TestID); err == nil {
		t.Fatal("save replaced unreadable retained credentials")
	}
	if err := os.WriteFile(f.box.Path(), original, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = f.manager.Snapshot(ctx, true)
	if err != nil || !snapshot.Storage.EncryptionKey.OK {
		t.Fatalf("restored key not recognized: %+v %v", snapshot, err)
	}
}
