package blobstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestLocalConfinement proves the local driver cannot be pushed outside its
// root: absolute-path keys, .. traversal, backslash traversal and — the case
// key validation alone can't cover — a symlink planted inside the root that
// points outside it.
func TestLocalConfinement(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("host-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := NewLocal(filepath.Join(root, "blobs"))
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	// Plant a symlink inside the root pointing outside it.
	if err := os.Symlink(outside, filepath.Join(root, "blobs", "escape.txt")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}
	if _, _, err := st.Open(ctx, "escape.txt", FullRange); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open through escaping symlink err = %v, want ErrNotFound", err)
	}
	if _, err := st.Stat(ctx, "escape.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat through escaping symlink err = %v, want ErrNotFound", err)
	}
	if err := st.Delete(ctx, "escape.txt"); err != nil {
		t.Fatalf("Delete of escaping symlink should be a no-op success: %v", err)
	}
	// The outside file must be untouched.
	if b, _ := os.ReadFile(outside); string(b) != "host-secret" {
		t.Fatal("outside file was modified")
	}

	// Path-based escapes are rejected at the key layer.
	for _, key := range []string{"../../etc/passwd", "/etc/passwd", "a/../../b", `win\..\..\x`} {
		if _, err := st.Put(ctx, key, strings.NewReader("x"), PutOptions{}); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("Put(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 || entries[0].Name() != "blobs" {
		t.Fatalf("root polluted: %v", entries)
	}
}

func TestLocalRelativeRootRejected(t *testing.T) {
	if _, err := NewLocal("relative/path"); err == nil {
		t.Fatal("relative root must be rejected")
	}
	if _, err := NewLocal(""); err == nil {
		t.Fatal("empty root must be rejected")
	}
}

// TestLocalConcurrentPut hammers the same key from many goroutines: every
// read must see a complete object (one of the two payloads), never a partial
// temp file, and no .tmp- file may leak.
func TestLocalConcurrentPut(t *testing.T) {
	st, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	a := bytes.Repeat([]byte("A"), 4096)
	b := bytes.Repeat([]byte("B"), 4096)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload := a
			if i%2 == 1 {
				payload = b
			}
			_, _ = st.Put(ctx, "hot/key", bytes.NewReader(payload), PutOptions{})
		}()
	}
	wg.Wait()
	rc, _, err := st.Open(ctx, "hot/key", FullRange)
	if err != nil {
		t.Fatalf("Open after concurrent puts: %v", err)
	}
	body, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(body, a) && !bytes.Equal(body, b) {
		t.Fatalf("concurrent put left torn object (len=%d)", len(body))
	}
	entries, _ := os.ReadDir(filepath.Join(st.Path(), "hot"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

// TestLocalNoPartialVisibility: a source that fails mid-stream must leave no
// object at the key.
func TestLocalNoPartialVisibility(t *testing.T) {
	st, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.Put(context.Background(), "p/half", io.MultiReader(
		strings.NewReader("partial-data"),
		failReader{},
	), PutOptions{})
	if err == nil {
		t.Fatal("Put of failing reader must fail")
	}
	if _, err := st.Stat(context.Background(), "p/half"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial object visible, stat err = %v", err)
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
