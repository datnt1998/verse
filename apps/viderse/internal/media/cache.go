package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/gofrs/flock"
)

type cacheIdentity struct {
	size     int64
	modified time.Time
	info     os.FileInfo
}
type Cache struct {
	dir      string
	limit    int64
	stripes  [64]sync.Mutex
	mu       sync.Mutex
	verified map[string]cacheIdentity
}

func NewCache(dir string, limit int64) (*Cache, error) {
	if !filepath.IsAbs(dir) || limit <= 0 {
		return nil, fmt.Errorf("bộ đệm cần đường dẫn tuyệt đối và giới hạn dương")
	}
	for _, sub := range []string{"sha256", "locks", "work"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			return nil, err
		}
	}
	cache := &Cache{dir: dir, limit: limit, verified: make(map[string]cacheIdentity)}
	if err := cache.recoverWork(); err != nil {
		return nil, err
	}
	return cache, nil
}
func (c *Cache) Limit() int64 { return c.limit }
func (c *Cache) Dir() string  { return c.dir }
func (c *Cache) Acquire(ctx context.Context, store blobstore.BlobStore, ref Ref) (*Handle, error) {
	key, err := ContentKey(ref.SHA256)
	if err != nil {
		return nil, err
	}
	if key != ref.Key {
		return nil, blobstore.ErrInvalidKey
	}
	if ref.Bytes < 0 {
		return nil, blobstore.ErrDigestMismatch
	}
	if ref.Bytes > c.limit {
		return c.workspace(ctx, store, ref)
	}
	stripe := &c.stripes[int(ref.SHA256[0])%len(c.stripes)]
	stripe.Lock()
	defer stripe.Unlock()
	lock := flock.New(filepath.Join(c.dir, "locks", ref.SHA256+".lock"))
	path := filepath.Join(c.dir, "sha256", ref.SHA256[:2], ref.SHA256)
	for {
		ok, err := lock.TryRLockContext(ctx, 20*time.Millisecond)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ctx.Err()
		}
		if c.valid(path, ref) {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
			c.remember(path)
			return &Handle{Path: path, Ref: ref, release: func() { _ = lock.Unlock() }}, nil
		}
		if err := lock.Unlock(); err != nil {
			return nil, err
		}
		ok, err = lock.TryLockContext(ctx, 20*time.Millisecond)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ctx.Err()
		}
		if !c.valid(path, ref) {
			budget := flock.New(filepath.Join(c.dir, "locks", "budget.lock"))
			ok, err := budget.TryLockContext(ctx, 20*time.Millisecond)
			if err != nil || !ok {
				lock.Unlock()
				if err == nil {
					err = ctx.Err()
				}
				return nil, err
			}
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				budget.Unlock()
				lock.Unlock()
				return nil, err
			}
			if _, err := c.evict(ctx, c.limit-ref.Bytes); err != nil {
				budget.Unlock()
				lock.Unlock()
				return nil, err
			}
			_, used, err := c.entries()
			if err != nil {
				budget.Unlock()
				lock.Unlock()
				return nil, err
			}
			if used > c.limit-ref.Bytes {
				budget.Unlock()
				lock.Unlock()
				return c.workspace(ctx, store, ref)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				lock.Unlock()
				budget.Unlock()
				return nil, err
			}
			if err := removePartials(filepath.Dir(path), ref.SHA256); err != nil {
				budget.Unlock()
				lock.Unlock()
				return nil, err
			}
			if err := download(ctx, store, ref, path); err != nil {
				lock.Unlock()
				budget.Unlock()
				return nil, err
			}
			c.remember(path)
			if err := budget.Unlock(); err != nil {
				lock.Unlock()
				return nil, err
			}
		}
		if err := lock.Unlock(); err != nil {
			return nil, err
		}
	}
}

func (c *Cache) valid(path string, ref Ref) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != ref.Bytes {
		return false
	}
	c.mu.Lock()
	known, ok := c.verified[path]
	c.mu.Unlock()
	if ok && known.size == info.Size() && known.modified.Equal(info.ModTime()) && os.SameFile(known.info, info) {
		return true
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	sum, n, err := blobstore.DigestAll(f)
	f.Close()
	if err != nil || sum != ref.SHA256 || n != ref.Bytes {
		return false
	}
	c.remember(path)
	return true
}
func (c *Cache) remember(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	c.mu.Lock()
	c.verified[path] = cacheIdentity{info.Size(), info.ModTime(), info}
	c.mu.Unlock()
}
func download(ctx context.Context, store blobstore.BlobStore, ref Ref, path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hydrate-"+ref.SHA256+"-*.part")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	body, _, err := store.Open(ctx, ref.Key, blobstore.FullRange)
	if err != nil {
		tmp.Close()
		return err
	}
	sum, n, copyErr := blobstore.DigestAll(io.TeeReader(io.LimitReader(body, ref.Bytes+1), tmp))
	closeErr := body.Close()
	if copyErr == nil {
		copyErr = ctx.Err()
	}
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil && (sum != ref.SHA256 || n != ref.Bytes) {
		copyErr = blobstore.ErrDigestMismatch
	}
	if copyErr == nil {
		copyErr = tmp.Sync()
	}
	fileErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if fileErr != nil {
		return fileErr
	}
	return os.Rename(name, path)
}

type cacheEntry struct {
	path, hash string
	size       int64
	at         time.Time
}

func (c *Cache) entries() ([]cacheEntry, int64, error) {
	var out []cacheEntry
	var total int64
	err := filepath.WalkDir(filepath.Join(c.dir, "sha256"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".part") {
			return nil
		}
		if _, err := ContentKey(name); err != nil {
			return fmt.Errorf("tệp bộ đệm không nhận diện: %s", name)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("bộ đệm chứa tệp không an toàn")
		}
		total += info.Size()
		out = append(out, cacheEntry{path, name, info.Size(), info.ModTime()})
		return nil
	})
	return out, total, err
}
func (c *Cache) Used() (int64, error) { _, total, err := c.entries(); return total, err }

// Clean drops only completed entries whose inter-process exclusive lock can
// be acquired immediately. Renderer/HTTP/CLI pins are never evicted.
func (c *Cache) Clean(ctx context.Context) (int64, error) { return c.evict(ctx, 0) }
func (c *Cache) Trim(ctx context.Context) (int64, error)  { return c.evict(ctx, c.limit) }
func (c *Cache) evict(ctx context.Context, target int64) (int64, error) {
	entries, total, err := c.entries()
	if err != nil {
		return 0, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].at.Before(entries[j].at) })
	var removed int64
	for _, entry := range entries {
		if total <= target {
			break
		}
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		lock := flock.New(filepath.Join(c.dir, "locks", entry.hash+".lock"))
		ok, err := lock.TryLock()
		if err != nil {
			return removed, err
		}
		if !ok {
			continue
		}
		err = os.Remove(entry.path)
		unlockErr := lock.Unlock()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		if unlockErr != nil {
			return removed, unlockErr
		}
		c.mu.Lock()
		delete(c.verified, entry.path)
		c.mu.Unlock()
		total -= entry.size
		removed += entry.size
	}
	return removed, nil
}
