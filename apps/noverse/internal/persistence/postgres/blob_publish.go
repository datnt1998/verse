package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBlobContentMismatch = errors.New("blob content or tenant does not match")
	ErrBlobDeleting        = errors.New("blob deletion is in progress")
)

// PublishInput describes bytes already verified in storage. JobID is optional.
type PublishInput struct {
	ProjectID pgtype.UUID
	// ID optionally preserves a deterministic import identity on first insert.
	ID                               pgtype.UUID
	Key, Driver, ContentType, SHA256 string
	Size                             int64
	JobID                            pgtype.UUID
}

// PublishBlob publishes verified bytes atomically. db must be a pgx transaction
// or pool; a supplied transaction retains its key lock until its caller commits.
func PublishBlob(ctx context.Context, db interface {
	gen.DBTX
	Begin(context.Context) (pgx.Tx, error)
}, in PublishInput) (Blob, error) {
	if !in.ProjectID.Valid || !sha256Re.MatchString(in.SHA256) || in.Size < 0 || in.Key == "" || (in.Driver != "local" && in.Driver != "s3") {
		return Blob{}, fmt.Errorf("publish blob: %w", errInvalidJobInput)
	}
	if tx, ok := db.(pgx.Tx); ok {
		return publishBlobTx(ctx, tx, in)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Blob{}, sanitizeErr(err)
	}
	defer tx.Rollback(ctx)
	blob, err := publishBlobTx(ctx, tx, in)
	if err != nil {
		return Blob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Blob{}, sanitizeErr(err)
	}
	return blob, nil
}

func publishBlobTx(ctx context.Context, tx pgx.Tx, in PublishInput) (Blob, error) {
	q := gen.New(tx)
	row, err := q.LockPublishBlob(ctx, in.Key)
	if noRows(err) {
		id := in.ID
		if !id.Valid {
			var e error
			id, e = randomUUID()
			if e != nil {
				return Blob{}, e
			}
		}
		_, err = q.InsertPublishedBlob(ctx, gen.InsertPublishedBlobParams{ID: id, ProjectID: in.ProjectID, ObjectKey: in.Key, StorageDriver: in.Driver, ContentType: in.ContentType, Sha256: in.SHA256, SizeBytes: in.Size, UploadJobID: in.JobID, UsageKind: blobkey.UsageKind(in.Key, in.ContentType)})
		if err != nil {
			return Blob{}, sanitizeErr(err)
		}
		row, err = q.LockPublishBlob(ctx, in.Key)
	}
	if err != nil {
		return Blob{}, sanitizeErr(err)
	}
	if row.State == "deleting" {
		return Blob{}, ErrBlobDeleting
	}
	if row.ProjectID != in.ProjectID || row.StorageDriver != in.Driver || row.Sha256 != in.SHA256 || row.SizeBytes != in.Size || row.ContentType != in.ContentType {
		return Blob{}, ErrBlobContentMismatch
	}
	if row.State == "ready" {
		return blobFromRow(row), nil
	}
	row, err = q.ResumePublishedBlob(ctx, gen.ResumePublishedBlobParams{ID: row.ID, Sha256: in.SHA256, SizeBytes: in.Size, UploadJobID: in.JobID, UsageKind: blobkey.UsageKind(in.Key, in.ContentType)})
	if err != nil {
		return Blob{}, sanitizeErr(err)
	}
	return blobFromRow(row), nil
}

// PublishBlob is the repository convenience entry point on its shared pool.
func (a *Audio) PublishBlob(ctx context.Context, in PublishInput) (Blob, error) {
	return PublishBlob(ctx, a.pool, in)
}

// BlobByObjectKey returns metadata after its caller authorizes the project.
func (a *Audio) BlobByObjectKey(ctx context.Context, key string) (Blob, error) {
	row, err := gen.New(a.pool).GetDeliveryBlob(ctx, key)
	if noRows(err) {
		return Blob{}, blobstore.ErrNotFound
	}
	if err != nil {
		return Blob{}, sanitizeErr(err)
	}
	return blobFromRow(row), nil
}

// UploadAndPublishBlob retains a rewindable source for bounded deletion retries.
// Every retry uploads again: bytes uploaded before a deletion are not reused.
func UploadAndPublishBlob(ctx context.Context, a *Audio, store blobstore.BlobStore, in PublishInput, source io.Reader) (Blob, error) {
	var db interface {
		Begin(context.Context) (pgx.Tx, error)
	} = a.pool
	if router, ok := store.(interface {
		PublicationStore(context.Context) (blobstore.BlobStore, *pgxpool.Conn, func(), error)
	}); ok {
		bound, conn, release, err := router.PublicationStore(ctx)
		if err != nil {
			return Blob{}, err
		}
		defer release()
		store = bound
		db = conn
		in.Driver = blobstore.DriverName(bound)
	}
	var seeker io.ReadSeeker
	if s, ok := source.(io.ReadSeeker); ok {
		seeker = s
	} else {
		f, err := os.CreateTemp("", "ainovel-publish-*")
		if err != nil {
			return Blob{}, err
		}
		defer os.Remove(f.Name())
		defer f.Close()
		if _, err := io.Copy(f, source); err != nil {
			return Blob{}, err
		}
		seeker = f
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return Blob{}, err
		}
	}
	origin, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return Blob{}, err
	}
	for attempt := range 6 {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<uint(attempt-1)) * 50 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Blob{}, ctx.Err()
			case <-timer.C:
			}
			if _, err := seeker.Seek(origin, io.SeekStart); err != nil {
				return Blob{}, err
			}
		}
		info, err := store.Put(ctx, in.Key, seeker, blobstore.PutOptions{ContentType: in.ContentType, ExpectedSHA256: in.SHA256})
		if err != nil {
			return Blob{}, err
		}
		if in.Size < 0 {
			in.Size = info.Size
		}
		if info.Size != in.Size || info.SHA256 != in.SHA256 {
			return Blob{}, ErrBlobContentMismatch
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return Blob{}, sanitizeErr(err)
		}
		blob, err := PublishBlob(ctx, tx, in)
		if errors.Is(err, ErrBlobDeleting) {
			tx.Rollback(ctx)
			continue
		}
		if err == nil {
			// GC deletes storage before its tombstone. The key lock protects the
			// replacement from every old GC action before this transaction commits.
			_, statErr := store.Stat(ctx, in.Key)
			if blobstore.IsNotFound(statErr) {
				if _, err = seeker.Seek(origin, io.SeekStart); err == nil {
					_, err = store.Put(ctx, in.Key, seeker, blobstore.PutOptions{ContentType: in.ContentType, ExpectedSHA256: in.SHA256})
				}
			} else if statErr != nil {
				err = statErr
			}
		}
		if err != nil {
			tx.Rollback(ctx)
			return Blob{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Blob{}, sanitizeErr(err)
		}
		return blob, nil
	}
	return Blob{}, ErrBlobDeleting
}
