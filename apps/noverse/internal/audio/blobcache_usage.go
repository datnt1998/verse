package audio

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type BlobCacheUsage struct {
	UsedBytes int64 `json:"usedBytes"`
	MaxBytes  int64 `json:"maxBytes"`
}

var blobUsageSnapshot struct {
	sync.Mutex
	root  string
	at    time.Time
	usage BlobCacheUsage
}

// CacheUsage samples the local cache at most once per 30 seconds. It never
// lists remote objects and excludes working files outside the cache budget.
func CacheUsage() (BlobCacheUsage, error) {
	root, limit, err := blobCacheConfig()
	if err != nil {
		return BlobCacheUsage{}, err
	}
	blobUsageSnapshot.Lock()
	defer blobUsageSnapshot.Unlock()
	if blobUsageSnapshot.root == root && blobUsageSnapshot.usage.MaxBytes == limit && time.Since(blobUsageSnapshot.at) < 30*time.Second {
		return blobUsageSnapshot.usage, nil
	}
	usage := BlobCacheUsage{MaxBytes: limit}
	err = filepath.WalkDir(filepath.Join(root, "entries"), func(_ string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		info, err := d.Info()
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		usage.UsedBytes += info.Size()
		return nil
	})
	if err != nil {
		return usage, err
	}
	blobUsageSnapshot.root, blobUsageSnapshot.at, blobUsageSnapshot.usage = root, time.Now(), usage
	return usage, nil
}
