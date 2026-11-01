package editor

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// Long editor work (fetching a clip from a link, decoding a voice source) takes
// minutes, and a request that sends no byte for 100 s is cut in front of the
// origin (Cloudflare HTTP 524). Handlers queue a job, answer 202 with the job,
// and the client polls /api/editor/jobs/{id}. The work runs on a context of its
// own: a closed tab never cancels it. A per-job timeout bounds it and the queue
// runs at most DefaultJobConcurrency jobs at once so a burst of imports cannot
// saturate the machine.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// Job timeouts sit above the tools' own limits (yt-dlp download 20 min, ffmpeg
// decode 30 min) so the job reports the tool's error instead of a bare timeout.
const (
	LinkJobTimeout        = 30 * time.Minute
	WaveformJobTimeout    = 40 * time.Minute
	DefaultJobConcurrency = 2
	jobRetention          = time.Hour
	jobHistory            = 64
)

// JobError keeps the wire shape of an editor API error so the client shows the
// same Vietnamese copy a synchronous request would have returned.
type JobError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

// Job is the polling resource of detached work: state, and on success the
// payload the synchronous handler used to return.
type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	State      string    `json:"state"`
	CreatedAt  int64     `json:"createdAt"`
	StartedAt  int64     `json:"startedAt,omitempty"`
	FinishedAt int64     `json:"finishedAt,omitempty"`
	Result     Object    `json:"result,omitempty"`
	Error      *JobError `json:"error,omitempty"`
}

// JobQueue is the process-wide registry of detached editor jobs. It is created
// once by the HTTP mount; finished jobs are pruned by age and count.
type JobQueue struct {
	mu        sync.Mutex
	runs      map[string]*jobRun
	slots     chan struct{}
	now       func() time.Time
	retention time.Duration
	history   int
}

type jobRun struct {
	job    Job
	cancel context.CancelFunc
}

func NewJobQueue(concurrency int) *JobQueue {
	if concurrency <= 0 {
		concurrency = DefaultJobConcurrency
	}
	return &JobQueue{
		runs:      map[string]*jobRun{},
		slots:     make(chan struct{}, concurrency),
		now:       time.Now,
		retention: jobRetention,
		history:   jobHistory,
	}
}

// Start queues work and returns its initial view without waiting. The work gets
// a fresh context with the job timeout, never the request's.
func (q *JobQueue) Start(kind string, timeout time.Duration, work func(context.Context) (Object, error)) Job {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	run := &jobRun{job: Job{ID: ID(), Kind: kind, State: JobQueued, CreatedAt: q.now().UnixMilli()}, cancel: cancel}
	q.mu.Lock()
	q.pruneLocked()
	q.runs[run.job.ID] = run
	view := run.job
	q.mu.Unlock()
	go q.run(ctx, run, work)
	return view
}

func (q *JobQueue) run(ctx context.Context, run *jobRun, work func(context.Context) (Object, error)) {
	defer run.cancel()
	select {
	case q.slots <- struct{}{}:
		defer func() { <-q.slots }()
	case <-ctx.Done():
		q.finish(run, nil, jobFailure(ctx, nil))
		return
	}
	q.mu.Lock()
	if run.job.State == JobQueued {
		run.job.State = JobRunning
		run.job.StartedAt = q.now().UnixMilli()
	}
	q.mu.Unlock()
	result, err := work(ctx)
	if err != nil {
		q.finish(run, nil, jobFailure(ctx, err))
		return
	}
	q.finish(run, result, nil)
}

// finish records a terminal state once. A job cancelled while its tool was
// still winding down keeps the cancelled state and its partial result is
// dropped.
func (q *JobQueue) finish(run *jobRun, result Object, failure *JobError) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if run.job.State != JobQueued && run.job.State != JobRunning {
		return
	}
	run.job.FinishedAt = q.now().UnixMilli()
	if failure == nil {
		run.job.State = JobSucceeded
		run.job.Result = result
		return
	}
	run.job.Error = failure
	if failure.Code == "cancelled" {
		run.job.State = JobCancelled
		return
	}
	run.job.State = JobFailed
}

func (q *JobQueue) Get(id string) (Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pruneLocked()
	run, ok := q.runs[id]
	if !ok {
		return Job{}, apiError("not_found", "Không tìm thấy tác vụ")
	}
	return run.job, nil
}

// Cancel stops queued or running work. The state changes at once; the tool may
// take a moment to die, and its eventual result is ignored.
func (q *JobQueue) Cancel(id string) (Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pruneLocked()
	run, ok := q.runs[id]
	if !ok {
		return Job{}, apiError("not_found", "Không tìm thấy tác vụ")
	}
	if run.job.State == JobQueued || run.job.State == JobRunning {
		run.cancel()
		run.job.State = JobCancelled
		run.job.FinishedAt = q.now().UnixMilli()
		run.job.Error = &JobError{Code: "cancelled", Message: "Đã huỷ tác vụ"}
	}
	return run.job, nil
}

func (q *JobQueue) pruneLocked() {
	cutoff := q.now().Add(-q.retention).UnixMilli()
	for id, run := range q.runs {
		if run.job.FinishedAt > 0 && run.job.FinishedAt < cutoff {
			delete(q.runs, id)
		}
	}
	if len(q.runs) <= q.history {
		return
	}
	finished := make([]*jobRun, 0, len(q.runs))
	for _, run := range q.runs {
		if run.job.FinishedAt > 0 {
			finished = append(finished, run)
		}
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].job.FinishedAt < finished[j].job.FinishedAt })
	for _, run := range finished {
		if len(q.runs) <= q.history {
			return
		}
		delete(q.runs, run.job.ID)
	}
}

// jobFailure maps a job's error onto the API error vocabulary: an editor error
// keeps its code and Vietnamese message, a cancelled or expired context is
// reported as such, and anything else stays a generic internal failure.
func jobFailure(ctx context.Context, err error) *JobError {
	var api *Error
	if errors.As(err, &api) {
		return &JobError{Code: api.Code, Message: api.Message}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &JobError{Code: "timeout", Message: "Tác vụ chạy quá lâu và đã bị dừng — thử lại sau"}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return &JobError{Code: "cancelled", Message: "Đã huỷ tác vụ"}
	}
	return &JobError{Code: "internal_error", Message: "Không xử lý được yêu cầu editor"}
}
