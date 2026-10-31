package serverstorage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

func saveGarage(t *testing.T, f fixture) {
	t.Helper()
	cfg := garageConfig(t)
	tested, err := f.manager.Test(context.Background(), "admin", cfg)
	if err != nil || tested.Error != nil {
		t.Fatalf("test: %+v %v", tested, err)
	}
	if _, err := f.manager.Save(context.Background(), "admin", cfg, tested.TestID); err != nil {
		t.Fatal(err)
	}
}
func copyForMigration(t *testing.T, f fixture, id string, ref media.Ref) {
	t.Helper()
	ctx := context.Background()
	record, err := f.repo.LatestMigration(ctx)
	if err != nil || record == nil {
		t.Fatalf("migration: %v", err)
	}
	source, _, resolved, release, err := f.media.ResolveAt(ctx, record.FromID, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	target, _, err := f.media.Store(ctx, record.ToID)
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := source.Open(ctx, resolved.Key, blobstore.FullRange)
	if err != nil {
		t.Fatal(err)
	}
	_, err = target.Put(ctx, ref.Key, body, blobstore.PutOptions{ExpectedSHA256: ref.SHA256, ContentType: ref.ContentType})
	body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blobstore.Verify(ctx, target, ref.Key, ref.SHA256, ref.Bytes); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.VerifyMigrationItem(ctx, id, ref, "", ""); err != nil {
		t.Fatal(err)
	}
}
func TestGarageCutoverRechecksLateRootsAndNeverBlocksPublisher(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	ctx := context.Background()
	first := f.publish(t, "first")
	saveGarage(t, f)
	record, err := f.repo.CreateMigration(ctx, "fence-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.ReconcileMigration(ctx, record.Migration.TaskID); err != nil {
		t.Fatal(err)
	}
	copyForMigration(t, f, record.Migration.TaskID, first)
	late := f.publish(t, "late root")
	if done, err := f.repo.TryCutover(ctx, record.Migration.TaskID); err != nil || done {
		t.Fatalf("cutover lost journal tail: done=%v err=%v", done, err)
	}
	if _, err := f.repo.ReconcileMigration(ctx, record.Migration.TaskID); err != nil {
		t.Fatal(err)
	}
	copyForMigration(t, f, record.Migration.TaskID, late)
	ready := make(chan struct{})
	release := make(chan struct{})
	committed := make(chan error, 1)
	go func() {
		committed <- f.media.Publish(ctx, nil, func(context.Context) error { close(ready); <-release; return nil })
	}()
	<-ready
	bounded, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	done, err := f.repo.TryCutover(bounded, record.Migration.TaskID)
	cancel()
	close(release)
	if pubErr := <-committed; pubErr != nil {
		t.Fatal(pubErr)
	}
	if err != nil || done {
		t.Fatalf("cutover waited on publisher or swapped: %v %v", done, err)
	}
	if done, err := f.repo.TryCutover(ctx, record.Migration.TaskID); err != nil || !done {
		t.Fatalf("complete graph did not swap: %v %v", done, err)
	}
	completedAfterSwap := f.publish(t, "prepared before swap, finalized afterwards")
	if completedAfterSwap.StoreID != record.ToID {
		t.Fatalf("producer used obsolete generation: %s", completedAfterSwap.StoreID)
	}
	if _, err := os.Stat(filepath.Join(f.root, completedAfterSwap.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("S3 writer touched old source tree: %v", err)
	}
}
func TestGarageCancelPreservesCopyAndNextRunDoesNotPutAgain(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	ctx := context.Background()
	ref := f.publish(t, "cancelled copy")
	saveGarage(t, f)
	record, err := f.repo.CreateMigration(ctx, "cancel-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.ReconcileMigration(ctx, record.Migration.TaskID); err != nil {
		t.Fatal(err)
	}
	copyForMigration(t, f, record.Migration.TaskID, ref)
	target, _, err := f.media.Store(ctx, record.ToID)
	if err != nil {
		t.Fatal(err)
	}
	countPut := func() uint64 {
		var n uint64
		for _, m := range target.(interface {
			Snapshot() []blobstore.OperationMetric
		}).Snapshot() {
			if m.Op == "put" {
				n += m.Count
			}
		}
		return n
	}
	before := countPut()
	if err := f.manager.MigrationAction(ctx, record.Migration.TaskID, "cancel"); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Stat(ctx, ref.Key); err != nil {
		t.Fatal("cancel deleted copied object")
	}
	state, err := f.repo.State(ctx)
	if err != nil || state.Active != "local" {
		t.Fatalf("cancel changed active: %+v %v", state, err)
	}
	next, err := f.manager.Migrate(ctx)
	if err != nil || next == record.Migration.TaskID {
		t.Fatalf("next run: %s %v", next, err)
	}
	waitMigration(t, f, "done")
	if after := countPut(); after != before {
		t.Fatalf("reused copied object was PUT again: before=%d after=%d", before, after)
	}
	if _, err := os.Stat(filepath.Join(f.root, ref.Path)); err != nil {
		t.Fatal("migration deleted old source")
	}
}
func TestGaragePauseResumeAndRetryMissingSource(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	ctx := context.Background()
	ref := f.publish(t, "retry source")
	original, err := os.ReadFile(filepath.Join(f.root, ref.Path))
	if err != nil {
		t.Fatal(err)
	}
	saveGarage(t, f)
	record, err := f.repo.CreateMigration(ctx, "pause-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.manager.MigrationAction(ctx, record.Migration.TaskID, "pause"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.root, ref.Path)); err != nil {
		t.Fatal(err)
	}
	if err := f.manager.MigrationAction(ctx, record.Migration.TaskID, "resume"); err != nil {
		t.Fatal(err)
	}
	waitMigration(t, f, "error")
	failed, err := f.repo.LatestMigration(ctx)
	if err != nil || len(failed.Migration.Errors) == 0 {
		t.Fatalf("failure not recorded: %+v %v", failed, err)
	}
	state, err := f.repo.State(ctx)
	if err != nil || state.Active != "local" {
		t.Fatal("failure changed active store")
	}
	if err := os.WriteFile(filepath.Join(f.root, ref.Path), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.manager.MigrationAction(ctx, record.Migration.TaskID, "retry"); err != nil {
		t.Fatal(err)
	}
	waitMigration(t, f, "done")
}
