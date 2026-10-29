package audio

import (
	"errors"
	"testing"
	"time"
)

// TestExportPersistenceDoesNotBlockProgressReaders pins the invariant behind
// "export stuck at 85 %, step Đang nối cả tập": persistence uses fsync, which
// can stall for seconds under disk pressure (full-suite runs: Postgres, MinIO,
// real ffmpeg, hundreds of temp files). If that stall happens while s.mu is
// held, ExportRequests() — used by the HTTP progress endpoint, SSE snapshots
// and the tests' poll loop — blocks with it, so progress looks frozen and the
// poll loop cannot even reach its own deadline.
//
// Here the file write blocks until released; readers must still answer.
func TestExportPersistenceDoesNotBlockProgressReaders(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 8)
	prevWrite := exportWriteFile
	exportWriteFile = func(path string, data []byte) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return prevWrite(path, data)
	}
	t.Cleanup(func() { exportWriteFile = prevWrite })

	fakeFFmpeg(t)
	svc := newTestService(t)
	svc.synth = &stoppedSynth{}
	seedExportableChapters(t, svc, 1, []int{1, 2}, 1)

	done := make(chan error, 1)
	go func() {
		done <- svc.StartExport(1, "Truyện", []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}},
			ExportOptions{Format: "mp3"})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("export never began persisting")
	}

	// The writer is parked mid-fsync. Readers must not inherit that stall.
	read := make(chan []ExportRequest, 1)
	go func() { read <- svc.ExportRequests() }()
	select {
	case reqs := <-read:
		if len(reqs) != 1 || reqs[0].State != JobQueued {
			t.Fatalf("unexpected requests while persisting: %+v", reqs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExportRequests blocked while the export was persisting to disk (s.mu held across fsync)")
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("start export: %v", err)
	}
	req := awaitExport(t, svc, 1)
	if req.State != JobDone {
		t.Fatalf("export failed: %s", req.Error)
	}
}

// TestExportPersistFailureKeepsMemoryState guards the rollback path moved out
// of the lock: a failed write must not leave the service thinking it persisted.
func TestExportPersistFailureKeepsMemoryState(t *testing.T) {
	prevWrite := exportWriteFile
	exportWriteFile = func(string, []byte) error { return errors.New("disk full") }
	t.Cleanup(func() { exportWriteFile = prevWrite })

	svc := newTestService(t)
	err := svc.StartExport(1, "Truyện", []SourceChapter{{Chapter: 1, Title: "Một"}}, ExportOptions{Format: "mp3"})
	if err == nil {
		t.Fatal("StartExport must surface a persistence failure")
	}
	if reqs := svc.ExportRequests(); len(reqs) != 0 {
		t.Fatalf("failed start left a queued request behind: %+v", reqs)
	}
}
