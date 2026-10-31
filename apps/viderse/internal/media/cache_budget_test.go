package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestCacheBudgetUsesOwnedWorkspaceAndRecoversPartials(t *testing.T) {
	ctx := context.Background()
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	makeRef := func(data string) media.Ref {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
		key, err := media.ContentKey(sum)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(ctx, key, bytes.NewBufferString(data), blobstore.PutOptions{ExpectedSHA256: sum}); err != nil {
			t.Fatal(err)
		}
		return media.Ref{Key: key, SHA256: sum, Bytes: int64(len(data))}
	}
	a, b := makeRef("12345678"), makeRef("abcdefgh")
	dir := t.TempDir()
	cache, err := media.NewCache(dir, 8)
	if err != nil {
		t.Fatal(err)
	}
	partialDir := filepath.Join(dir, "sha256", a.SHA256[:2])
	if err := os.MkdirAll(partialDir, 0700); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(partialDir, ".hydrate-"+a.SHA256+"-crashed.part")
	if err := os.WriteFile(partial, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := cache.Acquire(ctx, store, a)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("abandoned part survived exclusive hydrate: %v", err)
	}
	second, err := cache.Acquire(ctx, store, b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Path, filepath.Join("work", "input-")) {
		t.Fatalf("pinned budget overflow entered evictable cache: %s", second.Path)
	}
	if used, err := cache.Used(); err != nil || used != 8 {
		t.Fatalf("cache limit exceeded: %d %v", used, err)
	}
	if _, err := media.NewCache(dir, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second.Path); err != nil {
		t.Fatal("startup cleanup removed live owned workspace")
	}
	secondPath := second.Path
	second.Release()
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Fatal("workspace did not release")
	}
	first.Release()
	third, err := cache.Acquire(ctx, store, b)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Release()
	if strings.Contains(third.Path, filepath.Join("work", "input-")) {
		t.Fatal("cold cache was not evicted to make room")
	}
}
