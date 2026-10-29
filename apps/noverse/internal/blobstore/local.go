package blobstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalMaxObjectBytes bounds one local object. The file driver is a scratch
// store for CLI/offline and test fixtures — a bound keeps a runaway upload
// from filling the disk silently.
const LocalMaxObjectBytes = 8 << 30 // 8 GiB

const localListLimit = 1000

// Local is the atomic-file driver. Every write goes temp-file → fsync → rename
// → dir fsync, so a crash never leaves a truncated object visible at its key.
// Confinement is enforced by os.Root: keys are relative paths inside the root
// and symlinks that point outside it fail open/stat/delete.
type Local struct {
	root *os.Root
	path string
}

var _ BlobStore = (*Local)(nil)

// DriverName is the registry label stored in blobs.storage_driver.
func (s *Local) DriverName() string { return "local" }

// NewLocal opens (creating if needed) a local store rooted at root. The root
// must be absolute — a relative root would silently follow the caller's
// working directory.
func NewLocal(root string) (*Local, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("BLOB_LOCAL_ROOT phải là đường dẫn tuyệt đối: %w", ErrInvalidKey)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục blob: %w", err)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("mở thư mục blob: %w", err)
	}
	return &Local{root: r, path: root}, nil
}

// Path returns the confined root (for diagnostics; never a blob URL).
func (l *Local) Path() string { return l.path }

// Close releases the root handle.
func (l *Local) Close() error { return l.root.Close() }

func (l *Local) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return BlobInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}
	dir := path.Dir(key)
	if dir != "." {
		if err := l.root.MkdirAll(dir, 0o700); err != nil {
			return BlobInfo{}, fmt.Errorf("tạo thư mục key: %w", err)
		}
	}
	tmpName := fmt.Sprintf(".tmp-%s-%s", path.Base(key), randSuffix())
	tmpPath := path.Join(dir, tmpName)
	tmp, err := l.root.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return BlobInfo{}, fmt.Errorf("tạo tệp tạm: %w", err)
	}
	cleanup := func(e error) (BlobInfo, error) {
		_ = tmp.Close()
		_ = l.root.Remove(tmpPath)
		return BlobInfo{}, e
	}
	sum := newWriteSum(tmp)
	n, err := io.Copy(sum, capReader(r, LocalMaxObjectBytes))
	if err != nil {
		return cleanup(fmt.Errorf("đọc nguồn blob: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return cleanup(err)
	}
	sha := sum.Digest()
	if opts.ExpectedSHA256 != "" && !strings.EqualFold(opts.ExpectedSHA256, sha) {
		return cleanup(fmt.Errorf("ghi được %s, kỳ vọng %s: %w", sha, opts.ExpectedSHA256, ErrDigestMismatch))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("fsync tệp tạm: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return cleanup(fmt.Errorf("đóng tệp tạm: %w", err))
	}
	if err := l.root.Rename(tmpPath, key); err != nil {
		return cleanup(fmt.Errorf("commit blob: %w", err))
	}
	if err := fsyncDir(l.root, dir); err != nil {
		return BlobInfo{}, fmt.Errorf("fsync thư mục: %w", err)
	}
	return BlobInfo{
		Key:         key,
		Size:        n,
		ContentType: defaultType(opts.ContentType),
		SHA256:      sha,
		ModTime:     time.Now(),
	}, nil
}

func (l *Local) Open(ctx context.Context, key string, rng ByteRange) (io.ReadCloser, BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return nil, BlobInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, BlobInfo{}, err
	}
	f, err := l.root.Open(key)
	if err != nil {
		return nil, BlobInfo{}, mapNotFound(err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, BlobInfo{}, err
	}
	if info.IsDir() {
		f.Close()
		return nil, BlobInfo{}, ErrNotFound
	}
	start, length, err := resolveRange(rng, info.Size())
	if err != nil {
		f.Close()
		return nil, BlobInfo{}, err
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		f.Close()
		return nil, BlobInfo{}, err
	}
	body := struct {
		io.Reader
		io.Closer
	}{io.LimitReader(f, length), f}
	return body, BlobInfo{Key: key, Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (l *Local) Stat(ctx context.Context, key string) (BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return BlobInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}
	info, err := l.root.Stat(key)
	if err != nil {
		return BlobInfo{}, mapNotFound(err)
	}
	if info.IsDir() {
		return BlobInfo{}, ErrNotFound
	}
	return BlobInfo{Key: key, Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := l.root.Remove(key)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("xóa blob: %w", err)
}

func (l *Local) List(ctx context.Context, prefix, cursor string, limit int) (ListPage, error) {
	if limit <= 0 {
		limit = localListLimit
	}
	if err := ctx.Err(); err != nil {
		return ListPage{}, err
	}
	var keys []string
	err := fs.WalkDir(l.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".tmp-") {
			return nil // in-flight temp object; never a visible blob
		}
		rel := p
		if rel == "." {
			return nil
		}
		if prefix != "" && !strings.HasPrefix(rel, prefix) {
			return nil
		}
		if cursor != "" && rel <= cursor {
			return nil
		}
		keys = append(keys, rel)
		return nil
	})
	if err != nil {
		return ListPage{}, fmt.Errorf("liệt kê blob: %w", err)
	}
	sort.Strings(keys)
	page := ListPage{}
	if len(keys) > limit {
		keys = keys[:limit]
		page.NextCursor = keys[len(keys)-1]
	}
	for _, k := range keys {
		info, err := l.root.Stat(k)
		if err != nil {
			continue // vanished between walk and stat
		}
		page.Infos = append(page.Infos, BlobInfo{Key: k, Size: info.Size(), ModTime: info.ModTime()})
	}
	return page, nil
}

func (l *Local) PresignGet(ctx context.Context, key string, ttl time.Duration, opts DownloadOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	// Capability answer, not a failure: a filesystem has no URLs. The HTTP
	// seam falls back to authenticated streaming.
	return "", ErrPresignUnsupported
}

// resolveRange converts a ByteRange into absolute [start,length) against a
// known size, enforcing satisfiability.
func resolveRange(rng ByteRange, size int64) (start, length int64, err error) {
	if rng.Offset < 0 {
		return 0, 0, ErrInvalidRange
	}
	if rng.Offset > size {
		return 0, 0, ErrInvalidRange
	}
	if rng.Offset == size && size > 0 {
		return 0, 0, ErrInvalidRange
	}
	length = size - rng.Offset
	if rng.Length >= 0 && rng.Length < length {
		length = rng.Length
	}
	return rng.Offset, length, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "escapes") {
		return ErrNotFound
	}
	return err
}

// fsyncDir forces the directory entry that carries a rename onto disk.
func fsyncDir(root *os.Root, dir string) error {
	if dir == "" {
		dir = "."
	}
	d, err := root.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func randSuffix() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func defaultType(ct string) string {
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}
