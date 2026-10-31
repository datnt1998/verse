package blobstore

import (
	"context"
	"fmt"
	"time"
)

// DeleteBatchSize is the S3 DeleteObjects maximum. Callers may use smaller batches.
const DeleteBatchSize = 1000

// DeleteResult records the outcome for one input key, including provider per-key errors.
type DeleteResult struct {
	Key string
	Err error
}

// BatchDeleter is an optional capability; BlobStore itself remains unchanged.
// Results are returned in input order. A request error may accompany partial results.
type BatchDeleter interface {
	DeleteMany(context.Context, []string) ([]DeleteResult, error)
}

// DeleteMany uses bounded batches or the sequential fallback. Only failed keys
// receive one retry; successful deletions are never repeated. Request-level
// retries remain the provider SDK's responsibility.
func DeleteMany(ctx context.Context, store BlobStore, keys []string) ([]DeleteResult, error) {
	results := make([]DeleteResult, 0, len(keys))
	for start := 0; start < len(keys); start += DeleteBatchSize {
		end := min(start+DeleteBatchSize, len(keys))
		batch := keys[start:end]
		run := func(keys []string) ([]DeleteResult, error) {
			if d, ok := store.(BatchDeleter); ok {
				return d.DeleteMany(ctx, keys)
			}
			return deleteSequential(ctx, store, keys), nil
		}
		part, err := run(batch)
		if err != nil {
			return append(results, part...), err
		}
		if len(part) != len(batch) {
			return results, fmt.Errorf("batch delete returned %d results for %d keys", len(part), len(batch))
		}
		retry := make([]string, 0)
		positions := make([]int, 0)
		for i, r := range part {
			if r.Key != batch[i] {
				return results, fmt.Errorf("batch delete returned an out-of-order key")
			}
			if r.Err != nil {
				retry = append(retry, r.Key)
				positions = append(positions, i)
			}
		}
		if len(retry) > 0 {
			retried, retryErr := run(retry)
			if retryErr != nil {
				return append(results, part...), retryErr
			}
			if len(retried) != len(retry) {
				return results, fmt.Errorf("batch retry returned incomplete results")
			}
			for i, r := range retried {
				if r.Key != retry[i] {
					return results, fmt.Errorf("batch retry returned an out-of-order key")
				}
				part[positions[i]] = r
			}
		}
		results = append(results, part...)
	}
	return results, nil
}

func deleteSequential(ctx context.Context, store BlobStore, keys []string) []DeleteResult {
	out := make([]DeleteResult, 0, len(keys))
	for _, key := range keys {
		out = append(out, DeleteResult{Key: key, Err: store.Delete(ctx, key)})
	}
	return out
}

// MultipartUpload identifies unfinished bytes, which never appear in List.
type MultipartUpload struct {
	Key       string
	UploadID  string
	Initiated time.Time
}
type MultipartPage struct {
	Uploads    []MultipartUpload
	NextCursor string
}

// MultipartReaper is optional (S3-compatible stores only). Its cursor contains
// both the key and upload-id markers; multiple uploads may share a key.
type MultipartReaper interface {
	ListMultipartUploads(context.Context, string, string, int) (MultipartPage, error)
	AbortMultipartUpload(context.Context, MultipartUpload) error
}
