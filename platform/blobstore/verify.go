package blobstore

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// Verify reads the stored bytes, never trusting provider metadata or ETags for
// integrity. The read is bounded to expectedSize+1 to reject oversized objects.
func Verify(ctx context.Context, store BlobStore, key, expectedSHA256 string, expectedSize int64) (BlobInfo, error) {
	digest, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(digest) != 32 || expectedSize < 0 || expectedSize == math.MaxInt64 {
		return BlobInfo{}, fmt.Errorf("invalid verification digest or size: %w", ErrDigestMismatch)
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}
	rc, info, err := store.Open(ctx, key, FullRange)
	if err != nil {
		return BlobInfo{}, err
	}
	sha, size, readErr := DigestAll(io.LimitReader(s3ContextReader{ctx: ctx, r: rc}, expectedSize+1))
	if err := errors.Join(readErr, rc.Close()); err != nil {
		return BlobInfo{}, err
	}
	if size != expectedSize || !strings.EqualFold(sha, expectedSHA256) {
		return BlobInfo{}, fmt.Errorf("stored bytes do not match expected digest and size: %w", ErrDigestMismatch)
	}
	info.SHA256 = sha
	info.Size = size
	return info, nil
}
