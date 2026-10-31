package render

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type supervisorRunner struct {
	calls  atomic.Int32
	active atomic.Int32
	max    atomic.Int32
	run    func(context.Context, int, []string, tools.Spec) error
}

func (f *supervisorRunner) Run(ctx context.Context, _ string, args []string, spec tools.Spec) (tools.Result, error) {
	n := int(f.calls.Add(1))
	active := f.active.Add(1)
	for old := f.max.Load(); active > old; old = f.max.Load() {
		if f.max.CompareAndSwap(old, active) {
			break
		}
	}
	defer f.active.Add(-1)
	return tools.Result{}, f.run(ctx, n, args, spec)
}

type supervisorOps struct {
	mu      sync.Mutex
	groups  map[int]string
	signals []int
}

func (o *supervisorOps) GroupAlive(id int) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.groups[id]
	return ok
}
func (o *supervisorOps) Processes(context.Context) ([]tools.ProcessRow, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	rows := []tools.ProcessRow{}
	for id, command := range o.groups {
		rows = append(rows, tools.ProcessRow{PID: id, PGID: id, Command: command})
	}
	return rows, nil
}
func (o *supervisorOps) SignalGroup(id int, sig tools.Signal) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.signals = append(o.signals, id)
	delete(o.groups, id)
	return nil
}
func fakeSupervisor(t *testing.T, mode string) (*Supervisor, *supervisorRunner, *supervisorOps) {
	t.Helper()
	store := &Store{Root: t.TempDir()}
	ops := &supervisorOps{groups: map[int]string{}}
	runner := &supervisorRunner{}
	runner.run = func(ctx context.Context, n int, args []string, spec tools.Spec) error {
		pid := 10000 + n
		ops.mu.Lock()
		ops.groups[pid] = strings.Join(args, " ")
		ops.mu.Unlock()
		spec.OnStart(pid)
		if mode == "exit" {
			return errors.New("early exit")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	return &Supervisor{Store: store, Runner: runner, ChromePath: func(context.Context) (string, error) { return "fake-chrome", nil }, Origin: "http://127.0.0.1:8999", ProcessOps: ops, PollInterval: time.Millisecond, ClaimDeadline: 4 * time.Millisecond, StallDeadline: 4 * time.Millisecond}, runner, ops
}
func TestSupervisorRestartsEarlyExitExactlyTwiceAndCleansGroup(t *testing.T) {
	s, r, ops := fakeSupervisor(t, "exit")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := s.LaunchExport(ctx, "project", "job", nil)
	if err == nil {
		t.Fatal("early exit succeeded")
	}
	if got := r.calls.Load(); got != 3 {
		t.Fatalf("Chrome starts=%d, want exactly3 (two restarts)", got)
	}
	ops.mu.Lock()
	defer ops.mu.Unlock()
	if len(ops.groups) != 0 || len(ops.signals) != 3 {
		t.Fatalf("groups=%v signals=%v", ops.groups, ops.signals)
	}
	requests, e := s.Store.ListExports()
	if e != nil {
		t.Fatal(e)
	}
	for _, request := range requests {
		if _, e = os.Stat(s.Store.ProfileDir(request.ID)); !os.IsNotExist(e) {
			t.Fatalf("profile survived cleanup: %v", e)
		}
	}
}
func TestSupervisorRestartsUnclaimedHangExactlyTwice(t *testing.T) {
	s, r, ops := fakeSupervisor(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := s.LaunchExport(ctx, "project", "job", nil)
	if err == nil {
		t.Fatal("hang succeeded")
	}
	if r.calls.Load() != 3 {
		t.Fatalf("starts=%d", r.calls.Load())
	}
	ops.mu.Lock()
	defer ops.mu.Unlock()
	if len(ops.groups) != 0 {
		t.Fatal("process group survived")
	}
}
func TestSupervisorOneSequentialExportLane(t *testing.T) {
	s, r, ops := fakeSupervisor(t, "hang")
	s.ClaimDeadline = time.Second
	s.StallDeadline = time.Second
	r.run = func(ctx context.Context, n int, args []string, spec tools.Spec) error {
		pid := 20000 + n
		ops.mu.Lock()
		ops.groups[pid] = strings.Join(args, " ")
		ops.mu.Unlock()
		spec.OnStart(pid)
		raw := args[len(args)-1]
		parsed, e := url.Parse(raw)
		if e != nil {
			return e
		}
		fragment := strings.Split(parsed.Fragment, "?")[0]
		id := strings.TrimPrefix(fragment, "/render/")
		request, e := s.Store.GetExport(id)
		if e != nil {
			return e
		}
		request, e = s.Store.ClaimExport(id, request.Nonce)
		if e != nil {
			return e
		}
		if _, e = s.Store.BindExport(id, request.Fence, fmt.Sprintf("export-%d", n)); e != nil {
			return e
		}
		if _, e = s.Store.CompleteExport(id, request.Fence, fmt.Sprintf("export-%d", n)); e != nil {
			return e
		}
		<-ctx.Done()
		return ctx.Err()
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.LaunchExport(context.Background(), "project", "job", nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.max.Load() != 1 {
		t.Fatalf("concurrent Chrome exports=%d", r.max.Load())
	}
}

func TestSupervisorKill9RecoversSameRequestAndNoOrphanChrome(t *testing.T) {
	if os.Getenv("VIDERSE_RENDER_OWNER_CHILD") == "1" {
		root := os.Getenv("VIDERSE_RENDER_TEST_ROOT")
		store := &Store{Root: root}
		request, err := store.CreateExport("project", "job")
		if err != nil {
			panic(err)
		}
		s := &Supervisor{Store: store, Runner: &tools.ExecRunner{}, ChromePath: func(context.Context) (string, error) { return os.Getenv("VIDERSE_RENDER_FAKE_CHROME"), nil }, Origin: "http://127.0.0.1:8999", PollInterval: time.Millisecond}
		s.defaults()
		_, _, _ = s.runExportAttempt(context.Background(), request, false, nil)
		os.Exit(7)
	}
	root := t.TempDir()
	script := filepath.Join(root, "chrome")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 300 &\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	owner := exec.Command(os.Args[0], "-test.run=^TestSupervisorKill9RecoversSameRequestAndNoOrphanChrome$")
	owner.Env = append(os.Environ(), "VIDERSE_RENDER_OWNER_CHILD=1", "VIDERSE_RENDER_TEST_ROOT="+root, "VIDERSE_RENDER_FAKE_CHROME="+script)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Process.Kill(); _ = owner.Wait() }()
	store := &Store{Root: root}
	var request ExportRequest
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		requests, err := store.ListExports()
		if err == nil && len(requests) > 0 && requests[0].Chrome != nil {
			request = requests[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if request.Chrome == nil {
		t.Fatal("owner did not record Chrome")
	}
	defer syscall.Kill(-request.Chrome.PGID, syscall.SIGKILL)
	if err := owner.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	if !((tools.SystemOps{}).GroupAlive(request.Chrome.PGID)) {
		t.Fatal("fixture failed to leave an orphan Chrome")
	}
	s := &Supervisor{Store: store}
	if err := s.Sweep(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.GetExport(request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ID != request.ID || recovered.Status != "queued" || recovered.Nonce == request.Nonce || recovered.Token == request.Token {
		t.Fatalf("not rerunnable: %+v", recovered)
	}
	if (tools.SystemOps{}).GroupAlive(request.Chrome.PGID) {
		t.Fatal("orphan Chrome group survived restart")
	}
	if _, err = os.Stat(store.ProfileDir(request.ID)); !os.IsNotExist(err) {
		t.Fatal("orphan profile survived restart")
	}
	fake, runner, _ := fakeSupervisor(t, "hang")
	fake.Store = store
	fake.ClaimDeadline = time.Second
	fake.StallDeadline = time.Second
	runner.run = func(ctx context.Context, n int, args []string, spec tools.Spec) error {
		spec.OnStart(30000)
		current, e := store.ClaimExport(recovered.ID, recovered.Nonce)
		if e != nil {
			return e
		}
		_, e = store.CompleteExport(current.ID, current.Fence, "rerun-export")
		if e != nil {
			return e
		}
		<-ctx.Done()
		return ctx.Err()
	}
	result, err := fake.LaunchExport(context.Background(), "project", "job", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequestID != request.ID || result.ExportID != "rerun-export" || runner.calls.Load() != 1 {
		t.Fatalf("restart did not rerun exact request: %+v", result)
	}
}

func TestSupervisorRestartsClaimedHeartbeatAndProgressHangs(t *testing.T) {
	for _, reason := range []string{"heartbeat", "đứng yên"} {
		t.Run(reason, func(t *testing.T) {
			s, runner, ops := fakeSupervisor(t, "hang")
			s.ClaimDeadline = time.Second
			s.StallDeadline = 15 * time.Millisecond
			runner.run = func(ctx context.Context, n int, args []string, spec tools.Spec) error {
				pid := 40000 + n
				ops.mu.Lock()
				ops.groups[pid] = strings.Join(args, " ")
				ops.mu.Unlock()
				spec.OnStart(pid)
				u, err := url.Parse(args[len(args)-1])
				if err != nil {
					return err
				}
				id := strings.TrimPrefix(strings.Split(u.Fragment, "?")[0], "/render/")
				request, err := s.Store.GetExport(id)
				if err != nil {
					return err
				}
				request, err = s.Store.ClaimExport(id, request.Nonce)
				if err != nil {
					return err
				}
				_, err = s.Store.mutateExport(id, func(r *ExportRequest) error {
					if reason == "heartbeat" {
						r.HeartbeatAt = timestamp(time.Now().Add(-46 * time.Second))
					} else {
						r.HeartbeatAt = timestamp(time.Now().Add(time.Hour))
						r.Progress = &ExportProgress{Phase: "exporting"}
					}
					return nil
				})
				if err != nil {
					return err
				}
				<-ctx.Done()
				return ctx.Err()
			}
			_, err := s.LaunchExport(context.Background(), "project", "job", nil)
			if err == nil || !strings.Contains(err.Error(), reason) {
				t.Fatalf("wrong hang detected: %v", err)
			}
			if runner.calls.Load() != 3 {
				t.Fatalf("Chrome starts=%d", runner.calls.Load())
			}
			ops.mu.Lock()
			defer ops.mu.Unlock()
			if len(ops.groups) != 0 {
				t.Fatal("hung group survived")
			}
		})
	}
}
