package editor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// waitJob polls a job until it leaves the queued/running states.
func waitJob(t *testing.T, q *JobQueue, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		job, e := q.Get(id)
		if e != nil {
			t.Fatal(e)
		}
		if job.State != JobQueued && job.State != JobRunning {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s stuck in %s", id, job.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Start must not wait for the work: the caller is an HTTP handler that has to
// answer 202 while the download is still running.
func TestJobsStartReturnsBeforeWorkFinishes(t *testing.T) {
	q := NewJobQueue(1)
	blocked := make(chan struct{})
	release := make(chan struct{})
	started := make(chan struct{})
	began := time.Now()
	job := q.Start("media.link", time.Minute, func(context.Context) (Object, error) {
		close(started)
		<-blocked
		<-release
		return Object{"media": "media-1"}, nil
	})
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("Start blocked %v", elapsed)
	}
	if job.State != JobQueued || job.ID == "" || job.Kind != "media.link" {
		t.Fatalf("initial job: %+v", job)
	}
	<-started
	running, e := q.Get(job.ID)
	if e != nil || running.State != JobRunning || running.StartedAt == 0 {
		t.Fatalf("running: %+v %v", running, e)
	}
	close(blocked)
	close(release)
	done := waitJob(t, q, job.ID)
	if done.State != JobSucceeded || done.Result["media"] != "media-1" || done.FinishedAt == 0 || done.Error != nil {
		t.Fatalf("finished: %+v", done)
	}
	if _, e = q.Get("missing"); e == nil {
		t.Fatal("unknown job resolved")
	}
	var api *Error
	if !errors.As(e, &api) || api.Code != "not_found" {
		t.Fatalf("unknown job error: %v", e)
	}
}

// The queue runs at most `concurrency` jobs; the rest wait their turn and a
// cancelled waiter never reaches the work.
func TestJobsCapConcurrencyAndCancelQueuedWork(t *testing.T) {
	q := NewJobQueue(1)
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	ran := make(chan string, 2)
	work := func(name string) func(context.Context) (Object, error) {
		return func(context.Context) (Object, error) {
			started <- struct{}{}
			<-release
			ran <- name
			return Object{"name": name}, nil
		}
	}
	first := q.Start("media.link", time.Minute, work("first"))
	<-started
	second := q.Start("media.link", time.Minute, work("second"))
	time.Sleep(20 * time.Millisecond)
	queued, e := q.Get(second.ID)
	if e != nil || queued.State != JobQueued || queued.StartedAt != 0 {
		t.Fatalf("second should wait for the slot: %+v %v", queued, e)
	}
	cancelled, e := q.Cancel(second.ID)
	if e != nil || cancelled.State != JobCancelled || cancelled.Error == nil || cancelled.Error.Message != "Đã huỷ tác vụ" {
		t.Fatalf("cancel: %+v %v", cancelled, e)
	}
	close(release)
	if done := waitJob(t, q, first.ID); done.State != JobSucceeded {
		t.Fatalf("first: %+v", done)
	}
	select {
	case name := <-ran:
		if name != "first" {
			t.Fatalf("cancelled work ran: %s", name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first job never ran")
	}
	if len(ran) != 0 {
		t.Fatalf("cancelled work ran: %s", <-ran)
	}
	if done, e := q.Get(second.ID); e != nil || done.State != JobCancelled {
		t.Fatalf("cancelled job changed state: %+v %v", done, e)
	}
}

// Cancelling a running job stops its context and keeps the cancelled state
// even if the tool returns something afterwards.
func TestJobsCancelRunningWorkCancelsContext(t *testing.T) {
	q := NewJobQueue(1)
	observed := make(chan error, 1)
	job := q.Start("media.waveform", time.Minute, func(ctx context.Context) (Object, error) {
		<-ctx.Done()
		observed <- ctx.Err()
		return Object{"late": true}, nil
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		current, e := q.Get(job.ID)
		if e != nil {
			t.Fatal(e)
		}
		if current.State == JobRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, e := q.Cancel(job.ID); e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("work context: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("work context was not cancelled")
	}
	time.Sleep(20 * time.Millisecond)
	done, e := q.Get(job.ID)
	if e != nil || done.State != JobCancelled || done.Result != nil {
		t.Fatalf("cancelled job: %+v %v", done, e)
	}
}

// A job that outlives its timeout fails with the Vietnamese copy the client
// shows, never as a silent success.
func TestJobsTimeoutFailsWithMessage(t *testing.T) {
	q := NewJobQueue(1)
	job := q.Start("media.link", 20*time.Millisecond, func(ctx context.Context) (Object, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := waitJob(t, q, job.ID)
	if done.State != JobFailed || done.Error == nil || done.Error.Code != "timeout" {
		t.Fatalf("timeout job: %+v", done)
	}
	if done.Error.Message != "Tác vụ chạy quá lâu và đã bị dừng — thử lại sau" {
		t.Fatalf("timeout copy: %q", done.Error.Message)
	}
}

// Editor errors keep their API code and message so the client sees exactly what
// a synchronous request would have returned.
func TestJobsPreserveEditorErrorCopy(t *testing.T) {
	q := NewJobQueue(1)
	job := q.Start("media.link", time.Minute, func(context.Context) (Object, error) {
		return nil, apiError("invalid_request", "Đoạn cắt tối đa 10 phút")
	})
	done := waitJob(t, q, job.ID)
	if done.State != JobFailed || done.Error == nil || done.Error.Code != "invalid_request" || done.Error.Message != "Đoạn cắt tối đa 10 phút" {
		t.Fatalf("failed job: %+v", done)
	}
	plain := q.Start("media.link", time.Minute, func(context.Context) (Object, error) {
		return nil, errors.New("yt-dlp exploded")
	})
	failed := waitJob(t, q, plain.ID)
	if failed.State != JobFailed || failed.Error == nil || failed.Error.Code != "internal_error" || failed.Error.Message != "Không xử lý được yêu cầu editor" {
		t.Fatalf("unexpected failure leaked: %+v", failed)
	}
}

// Concurrent pollers and finishers must not race on the registry.
func TestJobsConcurrentPollsAndFinishes(t *testing.T) {
	q := NewJobQueue(2)
	var wg sync.WaitGroup
	jobs := make([]string, 8)
	for i := range jobs {
		jobs[i] = q.Start("media.link", time.Minute, func(context.Context) (Object, error) {
			time.Sleep(2 * time.Millisecond)
			return Object{"ok": true}, nil
		}).ID
	}
	for _, id := range jobs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for range 20 {
				if _, e := q.Get(id); e != nil {
					t.Error(e)
					return
				}
			}
		}(id)
	}
	wg.Wait()
	for _, id := range jobs {
		if done := waitJob(t, q, id); done.State != JobSucceeded {
			t.Fatalf("job %s: %+v", id, done)
		}
	}
}
