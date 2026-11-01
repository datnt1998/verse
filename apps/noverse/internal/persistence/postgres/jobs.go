package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// Durable job queue on Postgres (phase 07).
//
// Claim is one short transaction with `FOR UPDATE SKIP LOCKED`; it bumps the
// row's fencing token and grants a bounded lease on **server time**. Every
// later write by a worker (heartbeat/complete/fail/release) must present the
// same (worker_id, fencing_token) while the lease is still live, so a worker
// that was killed, partitioned or superseded can never publish. Cancel and
// retry also bump the fence, which invalidates an in-flight worker
// immediately instead of waiting for the lease to lapse.

const (
	// DefaultLease is the job lease granted by Claim.
	DefaultLease = 120 * time.Second
	// DefaultHeartbeat is how often a healthy worker renews its lease.
	DefaultHeartbeat = 30 * time.Second
)

// JobKind is the workflow a job row belongs to.
type JobKind string

const (
	JobTTS         JobKind = "tts"
	JobAudioExport JobKind = "audio_export"
	JobTextExport  JobKind = "text_export"
	JobEngine      JobKind = "engine"
	JobStoryImport JobKind = "story_import"
	JobSimulate    JobKind = "simulate"
	JobSync        JobKind = "sync"
)

// JobState is the durable lifecycle state. It maps onto the public job dock
// states (queued/ running/ done/ error) at the HTTP layer.
type JobState string

const (
	JobQueued    JobState = "queued"
	JobRunning   JobState = "running"
	JobPaused    JobState = "paused"
	JobWaiting   JobState = "waiting"
	JobFailed    JobState = "failed"
	JobCompleted JobState = "completed"
	JobCancelled JobState = "cancelled"
)

var (
	// ErrNoClaimableJob means the queue is empty (or every row is paused or
	// leased by a live worker).
	ErrNoClaimableJob = errors.New("không có việc nào đang chờ")
	// ErrJobLeaseLost means the caller's fence/lease no longer owns the job:
	// the job was cancelled, retried, reclaimed after expiry, or the caller is
	// a stale worker. It is a hard failure — never retry the same publish.
	ErrJobLeaseLost = errors.New("quyền xử lý việc này đã mất")
	// ErrJobNotFound is a missing job inside the caller's project/story scope.
	ErrJobNotFound = errors.New("không tìm thấy việc này")
	// ErrJobState means the operation is not legal for the current state.
	ErrJobState = errors.New("việc này không ở trạng thái cho phép thao tác")
	// errInvalidJobInput is a caller bug (missing key/scope), not a storage
	// failure; it must never be confused with a lease loss.
	errInvalidJobInput = errors.New("dữ liệu job không hợp lệ")
)

// Job is a durable queue row.
type Job struct {
	ProjectID      pgtype.UUID
	StoryID        pgtype.UUID
	ID             pgtype.UUID
	Kind           JobKind
	State          JobState
	IdempotencyKey string
	Input          []byte
	Result         []byte
	Error          []byte
	ProgressDone   int32
	ProgressTotal  *int32
	RequestedBy    pgtype.UUID
	WorkerID       pgtype.UUID
	LeaseUntil     *time.Time
	FencingToken   int64
	CancelledAt    *time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

// JobKey identifies one job inside its project (the composite primary key).
type JobKey struct {
	ProjectID pgtype.UUID
	ID        pgtype.UUID
}

// WorkerLease is the capability a worker holds over a claimed job. It is the
// fence every publish path checks.
type WorkerLease struct {
	JobKey
	WorkerID pgtype.UUID
	Fence    int64
}

// Worker is a least-privilege service credential (today: 'tts' only).
//
// State/StateUpdatedAt carry the reader's latest published provision state and
// CommandSeq/Command the operator's queued command (the only way the server can
// press a button on the Mac). Both are opaque JSON: the server reads the fields
// the API exposes and stores the rest verbatim.
type Worker struct {
	ID           pgtype.UUID
	Name         string
	Capabilities []string
	RevokedAt    *time.Time
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	// State is the worker's last voice.ProvisionState ({} before the first post).
	State          json.RawMessage
	StateUpdatedAt *time.Time
	// CommandSeq is monotonic per credential; Command is {op: install|reinstall}.
	CommandSeq int64
	Command    json.RawMessage
}

// Jobs is the durable job/worker/event repository.
type Jobs struct {
	pool *pgxpool.Pool
}

// NewJobs builds the jobs repository on the shared pool.
func NewJobs(p *Pool) *Jobs { return &Jobs{pool: p.Pool()} }

func (j *Jobs) tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return RunInTx(ctx, j.pool, fn)
}

func jobFromRow(r gen.Job) Job {
	return Job{
		ProjectID:      r.ProjectID,
		StoryID:        r.StoryID,
		ID:             r.ID,
		Kind:           JobKind(r.Kind),
		State:          JobState(r.State),
		IdempotencyKey: r.IdempotencyKey,
		Input:          r.Input,
		Result:         r.Result,
		Error:          r.Error,
		ProgressDone:   r.ProgressDone,
		ProgressTotal:  int32Ptr(r.ProgressTotal),
		RequestedBy:    r.RequestedBy,
		WorkerID:       r.WorkerID,
		LeaseUntil:     timePtr(r.LeaseUntil),
		FencingToken:   r.FencingToken,
		CancelledAt:    timePtr(r.CancelledAt),
		StartedAt:      timePtr(r.StartedAt),
		FinishedAt:     timePtr(r.FinishedAt),
		CreatedAt:      r.CreatedAt.Time.UTC(),
		UpdatedAt:      r.UpdatedAt.Time.UTC(),
		Version:        r.Version,
	}
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func int32Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	n := v.Int32
	return &n
}

func jobKeyUUID(k JobKey) (gen.GetJobParams, error) {
	if !k.ProjectID.Valid || !k.ID.Valid {
		return gen.GetJobParams{}, fmt.Errorf("job key chưa đủ: %w", errInvalidJobInput)
	}
	return gen.GetJobParams{ProjectID: k.ProjectID, ID: k.ID}, nil
}

// Enqueue creates a queued job, or returns the existing row when the same
// (story, kind, idempotency key) was already enqueued. Re-locking a chapter
// must not create two parallel renders.
func (j *Jobs) Enqueue(ctx context.Context, scope ScopeRef, kind JobKind, idempotencyKey string, input any, progressTotal *int32, requestedBy pgtype.UUID) (Job, error) {
	if idempotencyKey == "" {
		return Job{}, fmt.Errorf("job cần idempotency key: %w", errInvalidJobInput)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return Job{}, fmt.Errorf("marshal job input: %w", errInvalidJobInput)
	}
	var total pgtype.Int4
	if progressTotal != nil {
		total = pgtype.Int4{Int32: *progressTotal, Valid: true}
	}
	var row gen.Job
	err = j.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		id, err := randomUUID()
		if err != nil {
			return fmt.Errorf("job id: %w", errInvalidJobInput)
		}
		row, err = q.InsertJob(ctx, gen.InsertJobParams{
			ProjectID:      scope.ProjectID,
			StoryID:        scope.StoryID,
			ID:             id,
			Kind:           string(kind),
			IdempotencyKey: idempotencyKey,
			Input:          payload,
			ProgressTotal:  total,
			RequestedBy:    requestedBy,
		})
		if noRows(err) {
			// Same (story, kind, idempotency key) already queued: resolve it.
			row, err = q.GetJobByIdempotency(ctx, gen.GetJobByIdempotencyParams{
				ProjectID:      scope.ProjectID,
				StoryID:        scope.StoryID,
				Kind:           string(kind),
				IdempotencyKey: idempotencyKey,
			})
		}
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("job %s: %w", kind, ErrJobNotFound)
			}
			return sanitizeErr(err)
		}
		return nil
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// Get returns one job inside its project scope.
func (j *Jobs) Get(ctx context.Context, key JobKey) (Job, error) {
	arg, err := jobKeyUUID(key)
	if err != nil {
		return Job{}, err
	}
	row, err := gen.New(j.pool).GetJob(ctx, arg)
	if err != nil {
		if noRows(err) {
			return Job{}, ErrJobNotFound
		}
		return Job{}, sanitizeErr(err)
	}
	return jobFromRow(row), nil
}

// List returns every job of a story in enqueue order. The HTTP layer derives
// the display `position` among queued jobs from this order.
func (j *Jobs) List(ctx context.Context, scope ScopeRef) ([]Job, error) {
	rows, err := gen.New(j.pool).ListStoryJobs(ctx, gen.ListStoryJobsParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, jobFromRow(r))
	}
	return out, nil
}

// Claim takes the oldest claimable job of the given kind for one worker.
// projectFilter may be zero to claim across the whole installation (the TTS
// worker does this); any other value limits the claim to that project.
func (j *Jobs) Claim(ctx context.Context, kind JobKind, workerID pgtype.UUID, lease time.Duration, projectFilter pgtype.UUID) (Job, error) {
	if lease <= 0 {
		lease = DefaultLease
	}
	var row gen.Job
	err := j.tx(ctx, func(tx pgx.Tx) error {
		var err error
		row, err = gen.New(tx).ClaimJob(ctx, gen.ClaimJobParams{
			Kind:     string(kind),
			Column2:  projectFilter,
			WorkerID: workerID,
			Column4:  int32(lease / time.Second),
		})
		if noRows(err) {
			return ErrNoClaimableJob
		}
		return sanitizeErr(err)
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// Heartbeat renews the lease. It returns ErrJobLeaseLost when the worker's
// fence is stale, the job was cancelled/retried, or the lease already expired.
func (j *Jobs) Heartbeat(ctx context.Context, lease WorkerLease, ttl time.Duration) (Job, error) {
	if ttl <= 0 {
		ttl = DefaultLease
	}
	return j.leaseUpdate(ctx, lease, func(q *gen.Queries) (gen.Job, error) {
		return q.HeartbeatJob(ctx, gen.HeartbeatJobParams{
			ProjectID:    lease.ProjectID,
			ID:           lease.ID,
			WorkerID:     lease.WorkerID,
			FencingToken: lease.Fence,
			Column5:      int32(ttl / time.Second),
		})
	})
}

func (j *Jobs) leaseUpdate(ctx context.Context, lease WorkerLease, fn func(*gen.Queries) (gen.Job, error)) (Job, error) {
	if !lease.ID.Valid || !lease.ProjectID.Valid {
		return Job{}, ErrJobLeaseLost
	}
	var row gen.Job
	err := j.tx(ctx, func(tx pgx.Tx) error {
		var err error
		row, err = fn(gen.New(tx))
		if noRows(err) {
			return ErrJobLeaseLost
		}
		return sanitizeErr(err)
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// Complete marks a claimed job done. Replaying an ACK returns
// ErrJobLeaseLost (the job is already terminal); the HTTP layer answers the
// replay with the stored result, so a retried ACK never appends a second
// event and immutable blob keys make a re-upload harmless.
func (j *Jobs) Complete(ctx context.Context, lease WorkerLease, result any, progressDone int32, progressTotal *int32) (Job, error) {
	return j.finish(ctx, lease, nil, result, JobCompleted, progressDone, progressTotal)
}

// Fail records a terminal error for a claimed job.
func (j *Jobs) Fail(ctx context.Context, lease WorkerLease, cause any, progressTotal *int32) (Job, error) {
	return j.finish(ctx, lease, cause, nil, JobFailed, 0, progressTotal)
}

func (j *Jobs) finish(ctx context.Context, lease WorkerLease, cause, result any, state JobState, progressDone int32, progressTotal *int32) (Job, error) {
	var errPayload, resPayload []byte
	var err error
	if cause != nil {
		if errPayload, err = json.Marshal(cause); err != nil {
			return Job{}, fmt.Errorf("marshal job error: %w", errInvalidJobInput)
		}
	}
	if result != nil {
		if resPayload, err = json.Marshal(result); err != nil {
			return Job{}, fmt.Errorf("marshal job result: %w", errInvalidJobInput)
		}
	}
	var total pgtype.Int4
	if progressTotal != nil {
		total = pgtype.Int4{Int32: *progressTotal, Valid: true}
	}
	return j.leaseUpdate(ctx, lease, func(q *gen.Queries) (gen.Job, error) {
		if state == JobCompleted {
			return q.CompleteJob(ctx, gen.CompleteJobParams{
				ProjectID:     lease.ProjectID,
				ID:            lease.ID,
				WorkerID:      lease.WorkerID,
				FencingToken:  lease.Fence,
				Result:        resPayload,
				ProgressDone:  progressDone,
				ProgressTotal: total,
			})
		}
		return q.FailJob(ctx, gen.FailJobParams{
			ProjectID:     lease.ProjectID,
			ID:            lease.ID,
			WorkerID:      lease.WorkerID,
			FencingToken:  lease.Fence,
			Error:         errPayload,
			ProgressTotal: total,
		})
	})
}

// CompleteWith runs a caller-supplied publish step in the same transaction as
// the fence-validated completion: either the attached metadata (render rows,
// export manifest) and the job result both land, or neither does. The fencing
// check happens first, so a stale worker never reaches fn.
func (j *Jobs) CompleteWith(ctx context.Context, lease WorkerLease, result any, progressDone int32, fn func(ctx context.Context, tx pgx.Tx, job Job) error) (Job, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return Job{}, fmt.Errorf("marshal job result: %w", errInvalidJobInput)
	}
	var row gen.Job
	err = j.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		var err error
		row, err = q.CompleteJob(ctx, gen.CompleteJobParams{
			ProjectID:    lease.ProjectID,
			ID:           lease.ID,
			WorkerID:     lease.WorkerID,
			FencingToken: lease.Fence,
			Result:       payload,
			ProgressDone: progressDone,
		})
		if noRows(err) {
			return ErrJobLeaseLost
		}
		if err != nil {
			return sanitizeErr(err)
		}
		if fn != nil {
			if err := fn(ctx, tx, jobFromRow(row)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// Release hands a job back to the queue without finishing it (graceful worker
// stop). Progress already reported stays on the row.
func (j *Jobs) Release(ctx context.Context, lease WorkerLease) (Job, error) {
	return j.leaseUpdate(ctx, lease, func(q *gen.Queries) (gen.Job, error) {
		return q.ReleaseJob(ctx, gen.ReleaseJobParams{
			ProjectID:    lease.ProjectID,
			ID:           lease.ID,
			WorkerID:     lease.WorkerID,
			FencingToken: lease.Fence,
		})
	})
}

// Cancel stops a job from any non-terminal state and invalidates any worker
// currently holding it by bumping the fence.
func (j *Jobs) Cancel(ctx context.Context, key JobKey) (Job, error) {
	return j.control(ctx, key, func(q *gen.Queries) (gen.Job, error) {
		return q.CancelJob(ctx, gen.CancelJobParams{ProjectID: key.ProjectID, ID: key.ID})
	})
}

// Pause stops claims on the job. A running worker's next heartbeat/publish
// fails because the fence moved.
func (j *Jobs) Pause(ctx context.Context, key JobKey) (Job, error) {
	return j.control(ctx, key, func(q *gen.Queries) (gen.Job, error) {
		return q.PauseJob(ctx, gen.PauseJobParams{ProjectID: key.ProjectID, ID: key.ID})
	})
}

// Resume puts a paused job back in the queue.
func (j *Jobs) Resume(ctx context.Context, key JobKey) (Job, error) {
	return j.control(ctx, key, func(q *gen.Queries) (gen.Job, error) {
		return q.ResumeJob(ctx, gen.ResumeJobParams{ProjectID: key.ProjectID, ID: key.ID})
	})
}

// Retry re-queues a finished/failed/cancelled job. newInput, when non-nil, is
// marshalled into the job input (retry options); the fence is bumped so late
// output from the previous attempt is rejected.
func (j *Jobs) Retry(ctx context.Context, key JobKey, newInput any) (Job, error) {
	var payload []byte
	if newInput != nil {
		var err error
		if payload, err = json.Marshal(newInput); err != nil {
			return Job{}, fmt.Errorf("marshal retry input: %w", errInvalidJobInput)
		}
	}
	return j.control(ctx, key, func(q *gen.Queries) (gen.Job, error) {
		return q.RetryJob(ctx, gen.RetryJobParams{ProjectID: key.ProjectID, ID: key.ID, Column3: payload})
	})
}

func (j *Jobs) control(ctx context.Context, key JobKey, fn func(*gen.Queries) (gen.Job, error)) (Job, error) {
	arg, err := jobKeyUUID(key)
	if err != nil {
		return Job{}, err
	}
	var row gen.Job
	err = j.tx(ctx, func(tx pgx.Tx) error {
		var err error
		row, err = fn(gen.New(tx))
		if noRows(err) {
			// Distinguish "missing" from "wrong state" for honest HTTP codes.
			if _, getErr := gen.New(tx).GetJob(ctx, arg); getErr != nil {
				if noRows(getErr) {
					return ErrJobNotFound
				}
				return sanitizeErr(getErr)
			}
			return ErrJobState
		}
		return sanitizeErr(err)
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// Progress records segment counts without touching the fence (the job owner
// reports progress; the state must still be live).
func (j *Jobs) Progress(ctx context.Context, key JobKey, done int32, total *int32) (Job, error) {
	arg, err := jobKeyUUID(key)
	if err != nil {
		return Job{}, err
	}
	var tv pgtype.Int4
	if total != nil {
		tv = pgtype.Int4{Int32: *total, Valid: true}
	}
	row, err := gen.New(j.pool).UpdateJobProgress(ctx, gen.UpdateJobProgressParams{
		ProjectID:     arg.ProjectID,
		ID:            arg.ID,
		ProgressDone:  done,
		ProgressTotal: tv,
	})
	if err != nil {
		if noRows(err) {
			return Job{}, ErrJobState
		}
		return Job{}, sanitizeErr(err)
	}
	return jobFromRow(row), nil
}

// RequeueExpired normalises running jobs whose lease is gone back to queued.
// It never touches a live lease, so a healthy worker is not disturbed; a
// process that died mid-job leaves recoverable work behind after restart.
func (j *Jobs) RequeueExpired(ctx context.Context) (int64, error) {
	n, err := gen.New(j.pool).RequeueExpiredJobs(ctx)
	if err != nil {
		return 0, sanitizeErr(err)
	}
	return n, nil
}

// ── events ───────────────────────────────────────────────────────────────────

// JobEvent is one append-only job history entry.
type JobEvent struct {
	Seq       int64
	Type      string
	Payload   []byte
	CreatedAt time.Time
}

// AppendEvent adds an event to the job's stream. The job row is locked first
// so concurrent appenders cannot compute the same seq.
func (j *Jobs) AppendEvent(ctx context.Context, key JobKey, eventType string, payload any) (JobEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return JobEvent{}, fmt.Errorf("marshal job event: %w", errInvalidJobInput)
	}
	var ev gen.JobEvent
	err = j.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		if _, err := q.LockJob(ctx, gen.LockJobParams{ProjectID: key.ProjectID, ID: key.ID}); err != nil {
			if noRows(err) {
				return ErrJobNotFound
			}
			return sanitizeErr(err)
		}
		seq, err := q.NextJobEventSeq(ctx, gen.NextJobEventSeqParams{ProjectID: key.ProjectID, JobID: key.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		ev, err = q.InsertJobEvent(ctx, gen.InsertJobEventParams{
			ProjectID: key.ProjectID,
			JobID:     key.ID,
			Seq:       seq,
			EventType: eventType,
			Payload:   body,
		})
		return sanitizeErr(err)
	})
	if err != nil {
		return JobEvent{}, err
	}
	return JobEvent{Seq: ev.Seq, Type: ev.EventType, Payload: ev.Payload, CreatedAt: ev.CreatedAt.Time.UTC()}, nil
}

// Events returns the job's events after seq (0 = all), oldest first.
func (j *Jobs) Events(ctx context.Context, key JobKey, afterSeq int64) ([]JobEvent, error) {
	rows, err := gen.New(j.pool).ListJobEvents(ctx, gen.ListJobEventsParams{
		ProjectID: key.ProjectID,
		JobID:     key.ID,
		Seq:       afterSeq,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]JobEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, JobEvent{Seq: r.Seq, Type: r.EventType, Payload: r.Payload, CreatedAt: r.CreatedAt.Time.UTC()})
	}
	return out, nil
}

// ── workers ──────────────────────────────────────────────────────────────────

// RegisterWorker stores a new service credential. Only the SHA-256 of the
// bearer token is persisted; the plaintext token is returned exactly once.
func (j *Jobs) RegisterWorker(ctx context.Context, name string, tokenHash []byte, capabilities []string) (Worker, error) {
	id, err := randomUUID()
	if err != nil {
		return Worker{}, fmt.Errorf("worker id: %w", errInvalidJobInput)
	}
	row, err := gen.New(j.pool).InsertWorker(ctx, gen.InsertWorkerParams{
		ID:           id,
		Name:         name,
		TokenHash:    tokenHash,
		Capabilities: capabilities,
		LastSeenAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return Worker{}, sanitizeErr(err)
	}
	return workerFromRow(&row), nil
}

// AuthenticateWorker resolves a bearer token hash to an active worker.
func (j *Jobs) AuthenticateWorker(ctx context.Context, tokenHash []byte) (Worker, error) {
	row, err := gen.New(j.pool).GetWorkerByTokenHash(ctx, tokenHash)
	if err != nil {
		if noRows(err) {
			return Worker{}, ErrJobNotFound
		}
		return Worker{}, sanitizeErr(err)
	}
	if row.RevokedAt.Valid {
		return Worker{}, ErrJobLeaseLost
	}
	return workerFromRow(&row), nil
}

// TouchWorker records worker liveness (server clock).
func (j *Jobs) TouchWorker(ctx context.Context, id pgtype.UUID) error {
	_, err := gen.New(j.pool).TouchWorker(ctx, gen.TouchWorkerParams{
		ID:         id,
		LastSeenAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	return sanitizeErr(err)
}

// RevokeWorker stops further claims/heartbeats/publishes for the credential.
func (j *Jobs) RevokeWorker(ctx context.Context, id pgtype.UUID) error {
	n, err := gen.New(j.pool).RevokeWorker(ctx, id)
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrJobNotFound
	}
	return nil
}

// ListWorkers returns every credential row (active and revoked) for the admin
// CLI; token hashes are never exported.
func (j *Jobs) ListWorkers(ctx context.Context) ([]Worker, error) {
	rows, err := gen.New(j.pool).ListWorkers(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]Worker, 0, len(rows))
	for i := range rows {
		out = append(out, workerFromRow(&rows[i]))
	}
	return out, nil
}

// WorkerByID resolves one credential row (state + queued command included).
func (j *Jobs) WorkerByID(ctx context.Context, id pgtype.UUID) (Worker, error) {
	row, err := gen.New(j.pool).GetWorker(ctx, id)
	if err != nil {
		if noRows(err) {
			return Worker{}, ErrJobNotFound
		}
		return Worker{}, sanitizeErr(err)
	}
	return workerFromRow(&row), nil
}

// SetWorkerState records the reader's latest provision state (a JSON object from
// the shared voice.ProvisionState contract). A revoked credential matches no row
// and reports ErrJobNotFound, so a stolen token cannot keep a machine "fresh".
func (j *Jobs) SetWorkerState(ctx context.Context, id pgtype.UUID, state []byte) error {
	if len(state) == 0 {
		state = []byte(`{}`)
	}
	n, err := gen.New(j.pool).SetWorkerState(ctx, gen.SetWorkerStateParams{ID: id, State: state})
	if err != nil {
		return sanitizeErr(err)
	}
	if n == 0 {
		return ErrJobNotFound
	}
	return nil
}

// BumpWorkerCommand queues one operator command (install|reinstall) for the
// worker and returns the row with its new, monotonic seq.
func (j *Jobs) BumpWorkerCommand(ctx context.Context, id pgtype.UUID, op string) (Worker, error) {
	command, err := json.Marshal(map[string]string{"op": op})
	if err != nil {
		return Worker{}, fmt.Errorf("worker command: %w", errInvalidJobInput)
	}
	row, err := gen.New(j.pool).BumpWorkerCommand(ctx, gen.BumpWorkerCommandParams{ID: id, Command: command})
	if err != nil {
		if noRows(err) {
			return Worker{}, ErrJobNotFound
		}
		return Worker{}, sanitizeErr(err)
	}
	return workerFromRow(&row), nil
}

func workerFromRow(r *gen.Worker) Worker {
	return Worker{
		ID:             r.ID,
		Name:           r.Name,
		Capabilities:   r.Capabilities,
		RevokedAt:      timePtr(r.RevokedAt),
		LastSeenAt:     timePtr(r.LastSeenAt),
		CreatedAt:      r.CreatedAt.Time.UTC(),
		State:          append(json.RawMessage(nil), r.State...),
		StateUpdatedAt: timePtr(r.StateUpdatedAt),
		CommandSeq:     r.CommandSeq,
		Command:        append(json.RawMessage(nil), r.Command...),
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// GetForWorker resolves a job by id for the worker currently holding it. The
// worker API only ever learns ids from a claim it was granted, so there is no
// cross-project lookup path here.
func (j *Jobs) GetForWorker(ctx context.Context, jobID, workerID pgtype.UUID) (Job, error) {
	row, err := gen.New(j.pool).GetWorkerJob(ctx, gen.GetWorkerJobParams{ID: jobID, WorkerID: workerID})
	if err != nil {
		if noRows(err) {
			return Job{}, ErrJobNotFound
		}
		return Job{}, sanitizeErr(err)
	}
	return jobFromRow(row), nil
}

// UUIDString renders a UUID as canonical text ("" for the zero/invalid value).
func UUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	const hexd = "0123456789abcdef"
	var buf [36]byte
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf[j] = '-'
			j++
		}
		buf[j] = hexd[v>>4]
		buf[j+1] = hexd[v&0x0f]
		j += 2
	}
	return string(buf[:])
}

// ParseUUID parses canonical UUID text into the driver type.
func ParseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(strings.TrimSpace(s)); err != nil {
		return pgtype.UUID{}, fmt.Errorf("uuid %q: %w", s, errInvalidJobInput)
	}
	if !id.Valid {
		return pgtype.UUID{}, fmt.Errorf("uuid %q: %w", s, errInvalidJobInput)
	}
	return id, nil
}
