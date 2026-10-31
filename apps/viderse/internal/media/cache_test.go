package media_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestContentKeyRejectsTraversalAndInvalidDigest(t *testing.T) {
	for _, hash := range []string{"", "../file", "ABCD", fmt.Sprintf("%064d", 0) + "x"} {
		if _, err := media.ContentKey(hash); err == nil {
			t.Fatalf("accepted %q", hash)
		}
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte("data")))
	key, err := media.ContentKey(sum)
	if err != nil || key != "v1/sha256/"+sum[:2]+"/"+sum {
		t.Fatalf("%q %v", key, err)
	}
}

func TestCachePinsSurviveEvictionAndCorruptRestart(t *testing.T) {
	ctx := context.Background()
	remote, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source")
	data := []byte("verified durable bytes")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	key, _ := media.ContentKey(sum)
	f, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = remote.Put(ctx, key, f, blobstore.PutOptions{ExpectedSHA256: sum})
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Key: key, SHA256: sum, Bytes: int64(len(data)), Path: "library/overlays/one.mp4"}
	dir := t.TempDir()
	cache, err := media.NewCache(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := cache.Acquire(ctx, remote, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Clean(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(handle.Path); err != nil {
		t.Fatal("evicted pinned input")
	}
	handle.Release()
	handle.Release()
	if err := os.WriteFile(handle.Path, []byte("corrupt durable bytes!"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted, err := media.NewCache(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	other, err := restarted.Acquire(ctx, remote, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	got, err := os.ReadFile(other.Path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("corrupt cache trusted: %q %v", got, err)
	}
}
