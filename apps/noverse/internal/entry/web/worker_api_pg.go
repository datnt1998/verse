package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"io"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PGWorkerQueue wires the private worker API to the durable Postgres queue.
// Only the postgres backend exposes a worker listener: the file/offline mode
// keeps rendering locally through audio.Service and has no claim loop.
type PGWorkerQueue struct {
	Jobs  *postgres.Jobs
	Audio *postgres.Audio
}

func (q *PGWorkerQueue) mustJobs() error {
	if q == nil || q.Jobs == nil {
		return errWorkerBadRequest
	}
	return nil
}

// Authenticate resolves the hashed bearer token to an active worker.
func (q *PGWorkerQueue) Authenticate(ctx context.Context, tokenHash []byte) (WorkerIdentity, error) {
	if err := q.mustJobs(); err != nil {
		return WorkerIdentity{}, err
	}
	w, err := q.Jobs.AuthenticateWorker(ctx, tokenHash)
	if err != nil {
		return WorkerIdentity{}, err
	}
	return WorkerIdentity{ID: postgres.UUIDString(w.ID), Name: w.Name, Capabilities: w.Capabilities}, nil
}

// Touch records worker liveness.
func (q *PGWorkerQueue) Touch(ctx context.Context, workerID string) error {
	id, err := postgres.ParseUUID(workerID)
	if err != nil {
		return errWorkerBadRequest
	}
	return q.Jobs.TouchWorker(ctx, id)
}

// Claim takes one queued (or lease-expired) TTS job.
func (q *PGWorkerQueue) Claim(ctx context.Context, workerID, projectID string, lease time.Duration) (WorkerJob, error) {
	if err := q.mustJobs(); err != nil {
		return WorkerJob{}, err
	}
	worker, err := postgres.ParseUUID(workerID)
	if err != nil {
		return WorkerJob{}, errWorkerBadRequest
	}
	// Zero UUID = no project filter (claim across the installation).
	var filter pgtype.UUID
	if projectID != "" {
		id, err := postgres.ParseUUID(projectID)
		if err != nil {
			return WorkerJob{}, errWorkerBadRequest
		}
		filter = id
	}
	job, err := q.Jobs.Claim(ctx, postgres.JobTTS, worker, lease, filter)
	if errors.Is(err, postgres.ErrNoClaimableJob) {
		return WorkerJob{}, errNoWorkerJob
	}
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(job), nil
}

func workerJob(j postgres.Job) WorkerJob {
	out := WorkerJob{
		ID:            postgres.UUIDString(j.ID),
		ProjectID:     postgres.UUIDString(j.ProjectID),
		StoryID:       postgres.UUIDString(j.StoryID),
		Kind:          string(j.Kind),
		State:         string(j.State),
		ProgressDone:  j.ProgressDone,
		ProgressTotal: j.ProgressTotal,
		Fence:         j.FencingToken,
	}
	if len(j.Input) > 0 {
		out.Input = json.RawMessage(j.Input)
	}
	if j.LeaseUntil != nil {
		out.LeaseUntil = *j.LeaseUntil
	}
	return out
}

func (q *PGWorkerQueue) resolve(ctx context.Context, workerID, jobID string) (postgres.Job, postgres.WorkerLease, error) {
	if err := q.mustJobs(); err != nil {
		return postgres.Job{}, postgres.WorkerLease{}, err
	}
	worker, err := postgres.ParseUUID(workerID)
	if err != nil {
		return postgres.Job{}, postgres.WorkerLease{}, errWorkerBadRequest
	}
	job, err := postgres.ParseUUID(jobID)
	if err != nil {
		return postgres.Job{}, postgres.WorkerLease{}, errWorkerBadRequest
	}
	row, err := q.Jobs.GetForWorker(ctx, job, worker)
	switch {
	case errors.Is(err, postgres.ErrJobNotFound):
		return postgres.Job{}, postgres.WorkerLease{}, errWorkerNotFound
	case err != nil:
		return postgres.Job{}, postgres.WorkerLease{}, err
	}
	return row, postgres.WorkerLease{
		JobKey:   postgres.JobKey{ProjectID: row.ProjectID, ID: row.ID},
		WorkerID: worker,
	}, nil
}

// Heartbeat renews the lease; fence mismatch/expiry is a hard 409.
func (q *PGWorkerQueue) Heartbeat(ctx context.Context, workerID, jobID string, fence int64, lease time.Duration) (WorkerJob, error) {
	_, key, err := q.resolve(ctx, workerID, jobID)
	if err != nil {
		return WorkerJob{}, err
	}
	key.Fence = fence
	job, err := q.Jobs.Heartbeat(ctx, key, lease)
	if errors.Is(err, postgres.ErrJobLeaseLost) {
		return WorkerJob{}, errWorkerFencedOut
	}
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(job), nil
}

// JobInput returns the frozen snapshot the worker was granted.
func (q *PGWorkerQueue) JobInput(ctx context.Context, workerID, jobID string) (WorkerJob, error) {
	row, _, err := q.resolve(ctx, workerID, jobID)
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(row), nil
}

// ttsSnapshot is the minimal part of the frozen job input the ACK path needs:
// which chapter script this job was enqueued for. The web layer writes it at
// enqueue time; a job without it cannot attach renders.
type ttsSnapshot struct {
	ChapterID      string `json:"chapterId"`
	ScriptRevision int32  `json:"scriptRevision"`
}

// Complete validates the uploaded blobs and attaches them atomically with the
// job result: either the render rows and the completion both land, or neither.
//
// A job id the worker does not currently hold (cancelled, reclaimed, another
// worker's job) resolves to 404: the API never reveals that a foreign job
// exists, and the publish effect is simply refused.
func (q *PGWorkerQueue) Complete(ctx context.Context, workerID, jobID string, fence int64, progressDone int32, result any, attachments []WorkerAttachment) (WorkerJob, error) {
	row, key, err := q.resolve(ctx, workerID, jobID)
	if err != nil {
		return WorkerJob{}, err
	}
	if q.Audio == nil {
		return WorkerJob{}, fmt.Errorf("worker audio repo chưa cấu hình: %w", errWorkerBadRequest)
	}
	var snap ttsSnapshot
	if len(row.Input) > 0 {
		if err := json.Unmarshal(row.Input, &snap); err != nil {
			return WorkerJob{}, fmt.Errorf("job input không đọc được: %w", errWorkerBadRequest)
		}
	}
	key.Fence = fence

	job, err := q.Jobs.CompleteWith(ctx, key, result, progressDone, func(ctx context.Context, tx pgx.Tx, job postgres.Job) error {
		if len(attachments) == 0 {
			return nil
		}
		if snap.ChapterID == "" || snap.ScriptRevision <= 0 {
			return fmt.Errorf("job không có snapshot chương: %w", errWorkerBadRequest)
		}
		chapterID, err := postgres.ParseUUID(snap.ChapterID)
		if err != nil {
			return errWorkerBadRequest
		}
		for _, att := range attachments {
			if att.ChapterID != "" && att.ChapterID != snap.ChapterID {
				return fmt.Errorf("đính kèm thuộc chương khác: %w", errWorkerBadRequest)
			}
			if att.ScriptRevision != 0 && att.ScriptRevision != snap.ScriptRevision {
				return fmt.Errorf("đính kèm thuộc bản đọc khác: %w", errWorkerBadRequest)
			}
			blob, err := q.Audio.BlobByKeyTx(ctx, tx, job.ProjectID, att.Key)
			if errors.Is(err, postgres.ErrJobNotFound) {
				return errWorkerNotFound
			}
			if err != nil {
				return err
			}
			if blob.State != "ready" || blob.SHA256 != att.SHA256 {
				return fmt.Errorf("blob %s chưa sẵn sàng hoặc sai hash: %w", att.Key, errWorkerBadRequest)
			}
			render, err := q.Audio.AddRenderTx(ctx, tx, job.ProjectID, chapterID, snap.ScriptRevision, postgres.Render{
				SegmentID:   att.SegmentID,
				SynthKey:    att.SynthKey,
				BlobID:      blob.ID,
				DurationSec: att.DurationSec,
				SampleRate:  att.SampleRate,
				Parameters:  att.Parameters,
			})
			if err != nil {
				return err
			}
			if att.Select {
				if _, err := q.Audio.SelectRenderTx(ctx, tx, job.ProjectID, chapterID, snap.ScriptRevision, att.SegmentID, render.SynthKey); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if errors.Is(err, postgres.ErrJobLeaseLost) {
		return WorkerJob{}, errWorkerFencedOut
	}
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(job), nil
}

// Release hands a job back to the queue on graceful shutdown. The job keeps
// its progress; the fence stays with the old worker, so a release from a stale
// worker matches nothing.
func (q *PGWorkerQueue) Release(ctx context.Context, workerID, jobID string, fence int64) (WorkerJob, error) {
	_, key, err := q.resolve(ctx, workerID, jobID)
	if err != nil {
		return WorkerJob{}, err
	}
	key.Fence = fence
	job, err := q.Jobs.Release(ctx, key)
	if errors.Is(err, postgres.ErrJobLeaseLost) {
		return WorkerJob{}, errWorkerFencedOut
	}
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(job), nil
}

// Fail records a terminal failure for the claimed job.
func (q *PGWorkerQueue) Fail(ctx context.Context, workerID, jobID string, fence int64, cause any) (WorkerJob, error) {
	_, key, err := q.resolve(ctx, workerID, jobID)
	if err != nil {
		return WorkerJob{}, err
	}
	key.Fence = fence
	job, err := q.Jobs.Fail(ctx, key, cause, nil)
	if errors.Is(err, postgres.ErrJobLeaseLost) {
		return WorkerJob{}, errWorkerFencedOut
	}
	if err != nil {
		return WorkerJob{}, err
	}
	return workerJob(job), nil
}

// PGAudioBlobs records worker uploads in the blobs registry through the audio
// repository. Registration is idempotent: retrying the same (key, sha, size)
// succeeds, while different bytes under the same key are refused (objects are
// immutable and content-addressed, so that can only be a bug or an attack).
type PGAudioBlobs struct {
	Audio *postgres.Audio
}

// RegisterBlob atomically publishes verified uploaded bytes.
func (b *PGAudioBlobs) RegisterBlob(ctx context.Context, projectID, jobID, key, driver, contentType, sha256Hex string, size int64) error {
	if b == nil || b.Audio == nil {
		return fmt.Errorf("blob registry chưa cấu hình: %w", errWorkerBadRequest)
	}
	project, err := postgres.ParseUUID(projectID)
	if err != nil {
		return errWorkerBadRequest
	}
	job, err := postgres.ParseUUID(jobID)
	if err != nil {
		return errWorkerBadRequest
	}
	_, err = b.Audio.PublishBlob(ctx, postgres.PublishInput{ProjectID: project, JobID: job, Key: key, Driver: driver, ContentType: contentType, SHA256: sha256Hex, Size: size})
	return err
}

// UploadBlob owns the retryable source so deletion retries always re-upload.
func (b *PGAudioBlobs) UploadBlob(ctx context.Context, store blobstore.BlobStore, projectID, jobID, key, driver, contentType, sha string, source io.Reader) (blobstore.BlobInfo, error) {
	project, err := postgres.ParseUUID(projectID)
	if err != nil {
		return blobstore.BlobInfo{}, errWorkerBadRequest
	}
	job, err := postgres.ParseUUID(jobID)
	if err != nil {
		return blobstore.BlobInfo{}, errWorkerBadRequest
	}
	blob, err := postgres.UploadAndPublishBlob(ctx, b.Audio, store, postgres.PublishInput{ProjectID: project, JobID: job, Key: key, Driver: driver, ContentType: contentType, SHA256: sha, Size: -1}, source)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	return blobstore.BlobInfo{Key: blob.ObjectKey, Size: blob.SizeBytes, SHA256: blob.SHA256, ContentType: blob.ContentType}, nil
}

// ReadyBlobSize answers "is this exact content already committed under this
// key?" for the upload dedup path. The key is content-addressed, so a ready row
// whose digest equals the digest embedded in the key means the object is
// already stored; the size comes from that row.
func (b *PGAudioBlobs) ReadyBlobSize(ctx context.Context, projectID, key string) (int64, bool, error) {
	if b == nil || b.Audio == nil {
		return 0, false, fmt.Errorf("blob registry chưa cấu hình: %w", errWorkerBadRequest)
	}
	want, err := blobkey.Parse(key)
	if err != nil || want.SHA256 == "" {
		// Not a content-addressed key: no dedup claim can be made.
		return 0, false, nil
	}
	project, err := postgres.ParseUUID(projectID)
	if err != nil {
		return 0, false, errWorkerBadRequest
	}
	row, err := b.Audio.BlobByKey(ctx, project, key)
	if errors.Is(err, postgres.ErrJobNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if row.State != "ready" || row.SHA256 != want.SHA256 {
		return 0, false, nil
	}
	return row.SizeBytes, true, nil
}
