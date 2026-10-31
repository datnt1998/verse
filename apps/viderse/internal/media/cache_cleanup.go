package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type CacheCandidate struct {
	entry    cacheEntry
	identity os.FileInfo
}

func (c CacheCandidate) Bytes() int64 { return c.entry.size }
func (c *Cache) PreviewTrim(ctx context.Context) ([]CacheCandidate, error) {
	entries, _, err := c.entries()
	if err != nil {
		return nil, err
	}
	out := []CacheCandidate{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lock := flock.New(filepath.Join(c.dir, "locks", entry.hash+".lock"))
		ok, err := lock.TryLock()
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		info, statErr := os.Lstat(entry.path)
		unlockErr := lock.Unlock()
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return nil, statErr
		}
		if unlockErr != nil {
			return nil, unlockErr
		}
		out = append(out, CacheCandidate{entry, info})
	}
	return out, nil
}
func (c *Cache) TrimPreview(ctx context.Context, entries []CacheCandidate) (int64, int, error) {
	var bytes int64
	var count int
	for _, candidate := range entries {
		if err := ctx.Err(); err != nil {
			return bytes, count, err
		}
		entry := candidate.entry
		lock := flock.New(filepath.Join(c.dir, "locks", entry.hash+".lock"))
		ok, err := lock.TryLock()
		if err != nil {
			return bytes, count, err
		}
		if !ok {
			continue
		}
		info, err := os.Lstat(entry.path)
		if err == nil && os.SameFile(info, candidate.identity) && info.ModTime().Equal(candidate.identity.ModTime()) && info.Size() == candidate.identity.Size() {
			err = os.Remove(entry.path)
			if err == nil {
				bytes += info.Size()
				count++
				c.mu.Lock()
				delete(c.verified, entry.path)
				c.mu.Unlock()
			}
		}
		unlockErr := lock.Unlock()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return bytes, count, err
		}
		if unlockErr != nil {
			return bytes, count, unlockErr
		}
	}
	return bytes, count, nil
}
