package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func TestRestartUnconfiguredWorkerWaitsForLeaseRelease(t *testing.T) {
	f := provisionHostedGateway(t, "retire-lease", "story")
	ctx := context.Background()
	nar := postgres.NewNarrative(f.pool)
	scope, err := nar.Resolve(ctx, f.project, f.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	oldLease, err := postgres.NewStoryLease(ctx, nar, scope, workerLeaseTTL)
	if err != nil {
		t.Fatal(err)
	}
	var held atomic.Bool
	held.Store(true)
	stopping, release, oldDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var stopOnce, releaseOnce sync.Once
	releaseOld := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseOld)
	go func() { <-release; _ = oldLease.Close(); held.Store(false); close(oldDone) }()
	oldWorker := &launchedWorker{noModel: true, done: oldDone, stdin: noModelInput(func() { stopOnce.Do(func() { close(stopping) }) }), kill: func() error { releaseOld(); return nil }}
	key := f.project + "/" + f.story
	slot := f.g.slot(key)
	slot.mu.Lock()
	slot.worker = oldWorker
	slot.mu.Unlock()
	launched := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"jobs": []any{}}) }))
	t.Cleanup(server.Close)
	f.g.launchHosted = func(key, project, story string) (*launchedWorker, error) {
		launched <- struct{}{}
		if held.Load() {
			t.Error("replacement attempted to claim the story while the old worker held its PG lease")
		}
		lease, err := postgres.NewStoryLease(ctx, nar, scope, workerLeaseTTL)
		if err != nil {
			return nil, err
		}
		done := make(chan struct{})
		var once sync.Once
		closeWorker := func() { once.Do(func() { _ = lease.Close(); close(done) }) }
		return &launchedWorker{addr: strings.TrimPrefix(server.URL, "http://"), done: done, stdin: noModelInput(closeWorker), kill: func() error { closeWorker(); return nil }}, nil
	}
	retired := make(chan struct{})
	go func() { f.g.restartUnconfiguredWorkers("global", "", ""); close(retired) }()
	select {
	case <-stopping:
	case <-time.After(5 * time.Second):
		t.Fatal("old worker was not told to stop")
	}
	slot.mu.Lock()
	gate := slot.starting
	slot.mu.Unlock()
	if gate == nil {
		t.Fatal("retiring slot has no startup gate while its PG lease is still held")
	}
	started := make(chan error, 1)
	go func() { _, err := f.g.start(key); started <- err }()
	select {
	case <-launched:
		t.Fatal("replacement launched before old lease release")
	case <-time.After(25 * time.Millisecond):
	}
	releaseOld()
	select {
	case <-retired:
	case <-time.After(5 * time.Second):
		t.Fatal("retirement did not finish")
	}
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("replacement lease claim: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replacement did not launch after old lease release")
	}
	f.g.closeBook(key)
}
