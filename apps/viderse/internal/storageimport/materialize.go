package storageimport

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

type MaterializeResult struct {
	Files   int64 `json:"files"`
	Bytes   int64 `json:"bytes"`
	Applied bool  `json:"applied"`
}

// Materialize exports the complete current live manifest, including writes
// received after an S3 cutover. Operators must stop writers for rollback; this
// function does not silently switch the active backend or overwrite conflicts.
func Materialize(ctx context.Context, service *media.Service, target string, apply bool) (MaterializeResult, error) {
	out := MaterializeResult{Applied: apply}
	if !filepath.IsAbs(target) {
		return out, errors.New("thư mục khôi phục phải là đường dẫn tuyệt đối")
	}
	roots, err := service.Repository().Roots(ctx)
	if err != nil {
		return out, err
	}
	if !apply {
		for _, root := range roots {
			_, _, ref, release, err := service.Resolve(ctx, root.Path)
			if err != nil {
				return out, err
			}
			release()
			out.Files++
			out.Bytes += ref.Bytes
		}
		return out, nil
	}
	if err := os.MkdirAll(target, 0700); err != nil {
		return out, err
	}
	destination, err := os.OpenRoot(target)
	if err != nil {
		return out, err
	}
	defer destination.Close()
	for _, root := range roots {
		handle, err := service.Acquire(ctx, root.Path)
		if err != nil {
			return out, err
		}
		err = materializeFile(ctx, destination, root.Path, handle.Path, handle.Ref)
		handle.Release()
		if err != nil {
			return out, err
		}
		out.Files++
		out.Bytes += handle.Ref.Bytes
	}
	return out, nil
}
func materializeFile(ctx context.Context, destination *os.Root, rel, source string, ref media.Ref) error {
	if err := media.ValidatePath(rel); err != nil {
		return err
	}
	if existing, err := destination.Open(rel); err == nil {
		sum, n, err := blobstore.DigestAll(existing)
		existing.Close()
		if err != nil {
			return err
		}
		if sum != ref.SHA256 || n != ref.Bytes {
			return fmt.Errorf("tệp đích khác nội dung: %s", rel)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := destination.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0755); err != nil {
		return err
	}
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	tmp := filepath.ToSlash(filepath.Dir(rel)) + fmt.Sprintf("/.restore-%x.part", id)
	out, err := destination.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer destination.Remove(tmp)
	in, err := os.Open(source)
	if err != nil {
		out.Close()
		return err
	}
	sum, n, copyErr := blobstore.DigestAll(io.TeeReader(in, out))
	closeErr := in.Close()
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
		copyErr = out.Sync()
	}
	closeErr = out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	// The target is operator-owned and writers must be stopped. Existing files
	// are checked above; Root confines every component including symlinks.
	return destination.Link(tmp, rel)
}
