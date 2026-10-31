package audio

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
)

const defaultBlobCacheBytes int64 = 2 << 30
const blobCacheHeadroom uint64 = 64 << 20

// A process shares one cache for each configured root. Entries remain private
// (0700 root, 0400 content); their namespace includes the storage driver and
// tenant key prefix, so reuse never changes storage's tenant addressing.
var audioBlobCaches = struct {
	sync.Mutex
	roots map[string]*verifiedBlobCache
}{roots: make(map[string]*verifiedBlobCache)}

type verifiedBlobCache struct {
	root      string
	maxBytes  int64
	mu        sync.Mutex
	flights   map[string]chan struct{}
	pins      map[string]int
	reserved  int64
	fetches   chan struct{}
	freeBytes func(string) (uint64, error)
}

func blobCacheConfig() (string, int64, error) {
	root := strings.TrimSpace(os.Getenv("BLOB_CACHE_DIR"))
	if root == "" {
		root = filepath.Join(os.TempDir(), "ainovel-blob-cache")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", 0, err
	}
	limit := defaultBlobCacheBytes
	if value := strings.TrimSpace(os.Getenv("BLOB_CACHE_MAX_BYTES")); value != "" {
		limit, err = strconv.ParseInt(value, 10, 64)
		if err != nil || limit <= 0 {
			return "", 0, errors.New("BLOB_CACHE_MAX_BYTES must be a positive byte count")
		}
	}
	return root, limit, nil
}

func configuredBlobCache() (*verifiedBlobCache, error) {
	root, limit, err := blobCacheConfig()
	if err != nil {
		return nil, err
	}
	audioBlobCaches.Lock()
	defer audioBlobCaches.Unlock()
	if cache := audioBlobCaches.roots[root]; cache != nil {
		if cache.maxBytes != limit {
			return nil, errors.New("blob cache budget changed while cache is active")
		}
		return cache, nil
	}
	cache, err := newVerifiedBlobCache(root, limit)
	if err != nil {
		return nil, err
	}
	audioBlobCaches.roots[root] = cache
	return cache, nil
}

func newVerifiedBlobCache(root string, limit int64) (*verifiedBlobCache, error) {
	if limit <= 0 {
		return nil, errors.New("blob cache budget must be positive")
	}
	for _, dir := range []string{root, filepath.Join(root, "entries"), filepath.Join(root, "work")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	c := &verifiedBlobCache{root: root, maxBytes: limit, flights: map[string]chan struct{}{}, pins: map[string]int{}, fetches: make(chan struct{}, 4), freeBytes: diskAvailable}
	if err := c.cleanupWork(); err != nil {
		return nil, err
	}
	// A crashed fetch leaves only an uncommitted temp; exclude recent temps
	// because another process sharing the volume may still be writing them.
	entries, err := os.ReadDir(filepath.Join(root, "entries"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.Remove(filepath.Join(root, "entries", entry.Name()))
			}
		}
	}
	return c, nil
}

// Namespace at least the project, and the story when present. Include the
// environment; no title/path rename or cross-tenant/global dedupe is involved.
func cacheNamespace(driver, key string) string {
	parts := strings.Split(key, "/")
	end := len(parts)
	for i, p := range parts {
		if p == "p" && i+1 < len(parts) {
			end = i + 2
			break
		}
	}
	for i, p := range parts {
		if p == "s" && i+1 < len(parts) {
			end = i + 2
			break
		}
	}
	sum := sha256.Sum256([]byte(driver + "\x00" + strings.Join(parts[:end], "/")))
	return hex.EncodeToString(sum[:16])
}

func (c *verifiedBlobCache) entry(driver, key, digest string) (string, error) {
	digest = strings.ToLower(digest)
	raw, err := hex.DecodeString(digest)
	if err != nil || len(raw) != sha256.Size {
		return "", errors.New("blob cache requires a SHA-256 digest")
	}
	if driver == "" {
		return "", errors.New("blob cache requires a storage driver namespace")
	}
	return filepath.Join(c.root, "entries", cacheNamespace(driver, key)+"-"+digest), nil
}

// Per-key single flight also serializes verification and linking. A cancelled
// waiter does not cancel another job's fetch; a failed owner lets it retry.
func (c *verifiedBlobCache) lockEntry(ctx context.Context, path string) (func(), error) {
	for {
		c.mu.Lock()
		if ch := c.flights[path]; ch != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ch:
				continue
			}
		}
		if err := ctx.Err(); err != nil {
			c.mu.Unlock()
			return nil, err
		}
		ch := make(chan struct{})
		c.flights[path] = ch
		c.mu.Unlock()
		return func() { c.mu.Lock(); delete(c.flights, path); close(ch); c.mu.Unlock() }, nil
	}
}
func (c *verifiedBlobCache) pin(path string) func() {
	c.mu.Lock()
	c.pins[path]++
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.pins[path]--
		if c.pins[path] == 0 {
			delete(c.pins, path)
		}
		c.mu.Unlock()
	}
}

// Reserve both cache bytes and real filesystem space before opening storage.
// LRU timestamps are access timestamps we explicitly maintain, not filesystem
// atime (often disabled on container volumes). Pinned/in-flight entries survive.
func (c *verifiedBlobCache) reserve(size int64) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if size < 0 || size > c.maxBytes {
		return nil, fmt.Errorf("blob size %d exceeds cache budget %d", size, c.maxBytes)
	}
	type candidate struct {
		path string
		size int64
		used time.Time
	}
	var entries []candidate
	var used int64
	err := filepath.WalkDir(filepath.Join(c.root, "entries"), func(path string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil // A concurrent corruption repair removed this entry.
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
		used += info.Size()
		entries = append(entries, candidate{path, info.Size(), info.ModTime()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].used.Before(entries[j].used) })
	for _, entry := range entries {
		if used+c.reserved+size <= c.maxBytes {
			break
		}
		if c.pins[entry.path] > 0 {
			continue
		}
		if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		used -= entry.size
	}
	if used+c.reserved+size > c.maxBytes {
		return nil, errors.New("blob cache budget is occupied by live materializations")
	}
	free, err := c.freeBytes(c.root)
	if err != nil {
		return nil, err
	}
	if uint64(size+c.reserved)+blobCacheHeadroom > free {
		return nil, errors.New("insufficient free space for audio materialization")
	}
	c.reserved += size
	return func() { c.mu.Lock(); c.reserved -= size; c.mu.Unlock() }, nil
}

type cacheContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r cacheContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func verifiedFile(ctx context.Context, path, digest string, size int64) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() != size {
		return false
	}
	h := sha256.New()
	n, err := io.Copy(h, cacheContextReader{ctx, file})
	return err == nil && n == size && hex.EncodeToString(h.Sum(nil)) == strings.ToLower(digest)
}

func (c *verifiedBlobCache) materialize(ctx context.Context, store blobstore.BlobStore, driver, key, digest string, size int64, dest string) error {
	path, err := c.entry(driver, key, digest)
	if err != nil {
		return err
	}
	unpin := c.pin(path)
	defer unpin()
	unlock, err := c.lockEntry(ctx, path)
	if err != nil {
		return err
	}
	defer unlock()
	if !verifiedFile(ctx, path, digest, size) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := c.fetch(ctx, store, key, digest, size, path); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	// Destination is private per job and never written in place by the pipeline.
	if err := os.Link(path, dest); err == nil {
		return nil
	} else if errors.Is(err, os.ErrExist) {
		if verifiedFile(ctx, dest, digest, size) {
			return nil
		}
		return err
	}
	// A cross-filesystem copy consumes scratch space, not reusable cache quota.
	free, err := c.freeBytes(filepath.Dir(dest))
	if err != nil {
		return err
	}
	if uint64(size)+blobCacheHeadroom > free {
		return errors.New("insufficient space for blob scratch copy")
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".blob-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, copyErr := io.Copy(tmp, cacheContextReader{ctx, src})
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(tmp.Name(), 0400); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}
func (c *verifiedBlobCache) fetch(ctx context.Context, store blobstore.BlobStore, key, digest string, size int64, path string) error {
	select {
	case c.fetches <- struct{}{}:
		defer func() { <-c.fetches }()
	case <-ctx.Done():
		return ctx.Err()
	}
	release, err := c.reserve(size)
	if err != nil {
		return err
	}
	defer release()
	body, _, err := store.Open(ctx, key, blobstore.FullRange)
	if err != nil {
		return err
	}
	defer body.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(cacheContextReader{ctx, body}, size+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != size || hex.EncodeToString(h.Sum(nil)) != strings.ToLower(digest) {
		return fmt.Errorf("blob %s content does not match registry", key)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0400); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Work directories hold an advisory lock for their entire lifetime. The same
// root lock serializes creation with startup sweeping across server processes;
// a live job's lock is never acquired by the sweep, regardless of directory age.
func (c *verifiedBlobCache) workRootLock() (*flock.Flock, error) {
	lock := flock.New(filepath.Join(c.root, ".work-lock"), flock.SetPermissions(0600))
	if err := lock.Lock(); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}
func (c *verifiedBlobCache) cleanupWork() error {
	lock, err := c.workRootLock()
	if err != nil {
		return err
	}
	defer lock.Close()
	entries, err := os.ReadDir(filepath.Join(c.root, "work"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "job-") {
			continue
		}
		dir := filepath.Join(c.root, "work", entry.Name())
		fileLock := flock.New(filepath.Join(dir, ".lock"), flock.SetPermissions(0600))
		locked, err := fileLock.TryLock()
		if os.IsNotExist(err) {
			_ = fileLock.Close()
			continue // A live job finished between listing and opening its lock.
		}
		if err != nil {
			_ = fileLock.Close()
			return err
		}
		// Windows cannot unlink the open lock file. Holding the root lock
		// prevents a new creator/sweeper while this unlocked directory is removed.
		if err := fileLock.Close(); err != nil {
			return err
		}
		if locked {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *verifiedBlobCache) workDir() (string, func(), error) {
	free, err := c.freeBytes(c.root)
	if err != nil {
		return "", nil, err
	}
	if free < blobCacheHeadroom {
		return "", nil, errors.New("insufficient audio scratch space")
	}
	rootLock, err := c.workRootLock()
	if err != nil {
		return "", nil, err
	}
	defer rootLock.Close()
	dir, err := os.MkdirTemp(filepath.Join(c.root, "work"), "job-")
	if err != nil {
		return "", nil, err
	}
	lock := flock.New(filepath.Join(dir, ".lock"), flock.SetPermissions(0600))
	if err := lock.Lock(); err != nil {
		_ = lock.Close()
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return dir, func() { _ = lock.Close(); _ = os.RemoveAll(dir) }, nil
}

const peaksAlgorithm = "pcm-s16-v1"

type cachedPeaks struct {
	Peaks    []float64 `json:"peaks"`
	Duration float64   `json:"duration"`
}
type peaksEnvelope struct {
	Payload json.RawMessage `json:"payload"`
	SHA256  string          `json:"sha256"`
}

func (c *verifiedBlobCache) peaks(ctx context.Context, obj Object, bucket time.Duration, decode func(context.Context, string, string) error) ([]float64, float64, error) {
	if bucket <= 0 {
		return nil, 0, errors.New("peaks bucket must be positive")
	}
	path, err := c.entry(blobstore.DriverName(obj.Store), obj.Key, obj.SHA256)
	if err != nil {
		return nil, 0, err
	}
	path += "-" + peaksAlgorithm + "-" + strconv.FormatInt(int64(bucket), 10) + ".json"
	unpin := c.pin(path)
	defer unpin()
	unlock, err := c.lockEntry(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer unlock()
	var result cachedPeaks
	if body, err := os.ReadFile(path); err == nil {
		var env peaksEnvelope
		if json.Unmarshal(body, &env) == nil {
			sum := sha256.Sum256(env.Payload)
			if hex.EncodeToString(sum[:]) == env.SHA256 && json.Unmarshal(env.Payload, &result) == nil && result.Duration >= 0 {
				now := time.Now()
				_ = os.Chtimes(path, now, now)
				return result.Peaks, result.Duration, nil
			}
		}
	}
	_ = os.Remove(path)
	dir, cleanup, err := c.workDir()
	if err != nil {
		return nil, 0, err
	}
	defer cleanup()
	src := filepath.Join(dir, "source")
	if err := c.materialize(ctx, obj.Store, blobstore.DriverName(obj.Store), obj.Key, obj.SHA256, obj.Size, src); err != nil {
		return nil, 0, err
	}
	pcm := filepath.Join(dir, "peaks.wav")
	if err := decode(ctx, src, pcm); err != nil {
		return nil, 0, err
	}
	result.Peaks, result.Duration, err = streamWAVPeaks(ctx, pcm, bucket)
	if err != nil {
		return nil, 0, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, 0, err
	}
	sum := sha256.Sum256(payload)
	body, err := json.Marshal(peaksEnvelope{payload, hex.EncodeToString(sum[:])})
	if err != nil {
		return nil, 0, err
	}
	release, err := c.reserve(int64(len(body)))
	if err != nil {
		return nil, 0, err
	}
	defer release()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(body)
	closeErr := tmp.Close()
	if writeErr != nil {
		return nil, 0, writeErr
	}
	if closeErr != nil {
		return nil, 0, closeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, 0, err
	}
	return result.Peaks, result.Duration, nil
}

// Scan RIFF chunks and PCM incrementally: memory is one bucket plus the final
// waveform, not the entire decoded chapter. toPCM guarantees mono 48 kHz s16;
// accept any valid PCM s16 channel count/sample rate for parity with WAVPeaks.
func streamWAVPeaks(ctx context.Context, path string, bucket time.Duration) ([]float64, float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return nil, 0, err
	}
	if string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return nil, 0, errors.New("invalid PCM WAV")
	}
	var channels, rate, bits, format int
	for {
		var chunk [8]byte
		if _, err := io.ReadFull(file, chunk[:]); err != nil {
			return nil, 0, err
		}
		size := int64(binary.LittleEndian.Uint32(chunk[4:]))
		switch string(chunk[:4]) {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("invalid WAV format chunk")
			}
			var fmtHeader [16]byte
			if _, err := io.ReadFull(file, fmtHeader[:]); err != nil {
				return nil, 0, err
			}
			format = int(binary.LittleEndian.Uint16(fmtHeader[:2]))
			channels = int(binary.LittleEndian.Uint16(fmtHeader[2:4]))
			rate = int(binary.LittleEndian.Uint32(fmtHeader[4:8]))
			bits = int(binary.LittleEndian.Uint16(fmtHeader[14:16]))
			if _, err := file.Seek(size-16+size%2, io.SeekCurrent); err != nil {
				return nil, 0, err
			}
		case "data":
			if format != 1 || bits != 16 || channels < 1 || channels > 32 || rate < 1 || rate > 768000 {
				return nil, 0, errors.New("peaks requires valid 16-bit PCM WAV")
			}
			frameBytes := channels * 2
			if size%int64(frameBytes) != 0 {
				return nil, 0, errors.New("incomplete WAV sample frame")
			}
			frames := size / int64(frameBytes)
			window := int64(math.Round(float64(rate) * bucket.Seconds()))
			if window < 1 {
				window = 1
			}
			result := make([]float64, 0)
			var maxAbs int32
			var inWindow int64
			// Fixed-size I/O buffer rather than one Read call per sample.
			buf := make([]byte, 32*1024/frameBytes*frameBytes)
			remaining := size
			for remaining > 0 {
				if err := ctx.Err(); err != nil {
					return nil, 0, err
				}
				n := int64(len(buf))
				if n > remaining {
					n = remaining
				}
				if _, err := io.ReadFull(file, buf[:n]); err != nil {
					return nil, 0, err
				}
				for off := 0; off < int(n); off += frameBytes {
					for ch := range channels {
						v := int32(int16(binary.LittleEndian.Uint16(buf[off+ch*2:])))
						if v < 0 {
							v = -v
						}
						if v > maxAbs {
							maxAbs = v
						}
					}
					inWindow++
					if inWindow == window {
						result = append(result, math.Min(1, float64(maxAbs)/32768))
						maxAbs = 0
						inWindow = 0
					}
				}
				remaining -= n
			}
			if inWindow > 0 {
				result = append(result, math.Min(1, float64(maxAbs)/32768))
			}
			return result, float64(frames) / float64(rate), nil
		default:
			if _, err := file.Seek(size+size%2, io.SeekCurrent); err != nil {
				return nil, 0, err
			}
		}
	}
}
