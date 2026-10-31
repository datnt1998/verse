package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheUsageDoesNotInitializeCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-created")
	t.Setenv("BLOB_CACHE_DIR", root)
	t.Setenv("BLOB_CACHE_MAX_BYTES", "1048576")
	usage, err := CacheUsage()
	if err != nil {
		t.Fatal(err)
	}
	if usage.UsedBytes != 0 || usage.MaxBytes != 1048576 {
		t.Fatalf("unused cache metrics: %+v", usage)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("reading cache metrics created or touched the cache: %v", err)
	}
}
