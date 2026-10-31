package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

// Reconstruct the persisted pre-adapter ready export, not a mock inventory.
// The real startup path must verify/adopt its binary BEFORE strict ImportExisting.
func TestEditorStartupAdoptsVerifiedReadyBinaryBeforeInventory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("STORAGE_DRIVER", "local")
	fx := producerMedia(t, "local")
	ctx := context.Background()
	repo := postgres.NewEditorRepository(fx.pool)
	staging := t.TempDir()
	workspace := &editor.Service{Store: editor.Store{Media: fx.media, Repository: repo, StagingDir: staging}, Media: fx.media, Repository: repo, StagingDir: staging}
	project, _, err := workspace.CreateProject(ctx, editor.CreateProjectInput{Name: "Restart"})
	if err != nil {
		t.Fatal(err)
	}
	pid := project["id"].(string)
	eid, err := workspace.BeginExport(ctx, pid, editor.BeginExportOptions{FileName: "Kiểm tra.mp4", MIME: "video/mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if err = workspace.UploadExport(ctx, pid, eid, bytes.NewReader([]byte("committed export binary")), nil); err != nil {
		t.Fatal(err)
	}
	if _, err = workspace.CommitExport(ctx, pid, eid, editor.Object{}); err != nil {
		t.Fatal(err)
	}
	key := "exports/" + pid + "/" + eid + "/file"
	if _, err = fx.pool.Exec(ctx, "DELETE FROM editor_resources WHERE resource_key=$1", key); err != nil {
		t.Fatal(err)
	}
	entries, err := storageimport.Inventory(ctx, fx.config.LocalRoot)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := fx.repo.Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageimport.RequireKnown(entries, roots); err == nil {
		t.Fatal("legacy ready bytes should reproduce the missing-root startup failure")
	}
	for restart := range 2 {
		runtime, err := newMediaRuntime(ctx, fx.pool, func() string { return fx.config.LocalRoot }, nil, true, nil, true)
		if err != nil {
			t.Fatalf("restart%d: %v", restart, err)
		}
		if err = runtime.Close(ctx); err != nil {
			t.Fatal(err)
		}
		binary, err := repo.Get(ctx, key)
		if err != nil || binary.Version != 1 {
			t.Fatalf("restart%d not idempotent: %v %v", restart, binary, err)
		}
	}
	h, record, err := workspace.AcquireExport(ctx, pid, eid)
	if err != nil {
		t.Fatal(err)
	}
	h.Release()
	if record.Status != "ready" {
		t.Fatalf("ready metadata changed: %v", record)
	}
}
