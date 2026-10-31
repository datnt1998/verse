// Package blobstore provides the single blob-storage contract used by both
// drivers (local filesystem and S3-compatible). The plan locks one interface:
// object keys are immutable tenant-scoped identifiers (UUID prefixes), never
// derived from titles, slugs or user filenames. Drivers never get request
// context outside the documented capability model — the local driver reports
// ErrPresignUnsupported instead of faking URLs, and the S3 driver is one code
// path configured per provider recipe (AWS, R2, GCS interop, MinIO, Wasabi,
// B2, Spaces), not seven drivers.
package blobstore

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

// Contract errors. Callers use errors.Is — message text is not part of the
// interface.
var (
	// ErrNotFound: key absent. Delete is idempotent and does NOT return this.
	ErrNotFound = errors.New("blob không tồn tại")
	// ErrInvalidKey: key violates the shared key rules (see ValidateKey).
	ErrInvalidKey = errors.New("khóa blob không hợp lệ")
	// ErrInvalidRange: requested byte range cannot be satisfied (HTTP 416).
	ErrInvalidRange = errors.New("khoảng byte không hợp lệ")
	// ErrPresignUnsupported: driver cannot mint signed URLs (normal capability
	// of the local driver; the HTTP seam falls back to authenticated streaming).
	ErrPresignUnsupported = errors.New("driver không hỗ trợ presigned URL")
	// ErrTooLarge: object exceeds the driver bound (local read cap or the S3
	// multipart part-count ceiling). Never silently truncated.
	ErrTooLarge = errors.New("blob vượt giới hạn kích thước")
	// ErrDigestMismatch: streamed content does not match the expected digest
	// (PutOptions.ExpectedSHA256) or a transport was corrupted mid-flight.
	ErrDigestMismatch = errors.New("sha256 không khớp")
)

// MaxKeyBytes bounds one object key. S3 allows 1024; we keep the same bound so
// keys stay portable across drivers.
const MaxKeyBytes = 1024

var keySegmentBad = regexp.MustCompile(`[\\<>"|?*\x00-\x1f]`)

// ValidateKey enforces the shared key contract every driver applies:
// non-empty, relative, forward-slashes only, no dot/dotdot segments, no
// control or shell-special characters. Immutability and tenant scoping are
// caller concerns (UUID prefixes); this guards the storage layer itself.
func ValidateKey(key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	if len(key) > MaxKeyBytes {
		return ErrInvalidKey
	}
	if strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
		return ErrInvalidKey
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return ErrInvalidKey
		}
	}
	if keySegmentBad.MatchString(key) {
		return ErrInvalidKey
	}
	return nil
}

// ByteRange selects a half-open [Offset, Offset+Length) window of an object.
// Length < 0 means "to end of object". Offset == Size with a non-empty object
// is unsatisfiable (ErrInvalidRange); Offset beyond Size is always invalid.
type ByteRange struct {
	Offset int64
	Length int64 // <0 → to end
}

// FullRange addresses the whole object.
var FullRange = ByteRange{Offset: 0, Length: -1}

// PutOptions accompanies Put.
type PutOptions struct {
	// ContentType is stored as the object's declared type; empty means
	// application/octet-stream.
	ContentType string
	// CacheControl is the stored response policy; empty means private, no-store.
	// Public caching is always an explicit caller decision.
	CacheControl string
	// ContentDisposition is an optional HTTP disposition value. Use
	// ContentDisposition(kind, filename) for a sanitized UTF-8 filename.
	// Empty means inline, so audio playback is not forced into a download.
	ContentDisposition string
	// ExpectedSHA256, when set (lowercase hex, 64 chars), is verified against
	// the computed digest before publishing bytes. A mismatch leaves any
	// existing object untouched.
	ExpectedSHA256 string
}

// DownloadOptions accompanies PresignGet.
type DownloadOptions struct {
	// Filename sets Content-Disposition on the signed response (download name
	// only — never part of the object key). Sanitized by the driver.
	Filename string
	// Disposition selects "inline" or "attachment". Empty keeps attachment
	// when Filename is supplied, and otherwise uses the stored disposition.
	Disposition string
}

// BlobInfo describes one object. SHA256 is the canonical content digest:
// always populated by Put; populated by Stat/Open only when the driver can
// read it cheaply (all S3 Put calls store it as user metadata).
// ETag is provider bookkeeping and is NEVER the digest —
// multipart ETags are not MD5 and must not be treated as content hashes.
type BlobInfo struct {
	Key         string
	Size        int64
	ContentType string
	SHA256      string // lowercase hex, "" when unknown
	ETag        string // provider value; informational only
	ModTime     time.Time
}

// ListPage is one page of a key listing. NextCursor is opaque to callers and
// empty when the listing is complete.
type ListPage struct {
	Infos      []BlobInfo
	NextCursor string
}

// BlobStore is the locked storage contract. Implementations: local (atomic
// file driver, confined root) and s3 (one aws-sdk-go-v2 driver for every
// S3-compatible provider recipe).
type BlobStore interface {
	// Put streams r into the object at key, atomically: no partial object is
	// ever visible at key. The returned info always carries the canonical
	// SHA-256 of the stored bytes.
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (BlobInfo, error)
	// Open returns a stream of the requested range. The caller must Close.
	Open(ctx context.Context, key string, rng ByteRange) (io.ReadCloser, BlobInfo, error)
	// Stat returns metadata without opening the object.
	Stat(ctx context.Context, key string) (BlobInfo, error)
	// Delete removes key; deleting an absent key succeeds (idempotent).
	Delete(ctx context.Context, key string) error
	// List pages objects under prefix, ordered by key. cursor="" starts the
	// listing; limit<=0 uses a driver default. Pages stream — the driver never
	// materializes the whole bucket.
	List(ctx context.Context, prefix, cursor string, limit int) (ListPage, error)
	// PresignGet returns a short-lived signed GET URL. ttl<=0 defaults to 60s;
	// drivers clamp to their maximum. ErrPresignUnsupported is a normal
	// capability answer (local driver), not a failure.
	PresignGet(ctx context.Context, key string, ttl time.Duration, opts DownloadOptions) (string, error)
}

// DigestAll streams r to EOF and returns the lowercase-hex SHA-256 and byte
// count. Shared by drivers, copy verification and tests — the canonical digest
// is computed by us, never trusted from provider metadata.
func DigestAll(r io.Reader) (string, int64, error) {
	return digestReader(r)
}

// IsNotFound reports whether err denotes an absent blob.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// DriverName reports the registry label ('local' or 's3') of a store, so a
// blobs row always records the driver the bytes actually live on instead of a
// hardcoded guess. It returns "" for a store that does not declare itself —
// callers that persist the label must treat that as a configuration error.
func DriverName(store BlobStore) string {
	named, ok := store.(interface{ DriverName() string })
	if !ok {
		return ""
	}
	return named.DriverName()
}
