package editor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// This filesystem-backed seam exercises the actual editor service and shared
// JSON CAS helpers without a database or external ffmpeg binary.
type exportTestMedia struct {
	mu        sync.Mutex
	root      string
	paths     map[string]string
	repo      *exportTestRepository
	published [][]media.Artifact
}

func (m *exportTestMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	file, ok := m.paths[path]
	if !ok {
		return nil, blobstore.ErrNotFound
	}
	return &media.Handle{Path: file, Ref: media.Ref{Path: path}}, nil
}
func (m *exportTestMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	paths := make(map[string]string, len(m.paths))
	for path, file := range m.paths {
		paths[path] = file
	}
	m.repo.mu.Lock()
	rows := make(map[string]Resource, len(m.repo.rows))
	for key, row := range m.repo.rows {
		rows[key] = row
	}
	m.repo.mu.Unlock()
	pending := []string{}
	committed := false
	defer func() {
		if !committed {
			m.paths = paths
			m.repo.mu.Lock()
			m.repo.rows = rows
			m.repo.mu.Unlock()
			for _, file := range pending {
				_ = os.Remove(file)
			}
		}
	}()
	for _, a := range artifacts {
		b, e := os.ReadFile(a.File)
		if e != nil {
			return e
		}
		f, e := os.CreateTemp(m.root, "published-")
		if e != nil {
			return e
		}
		pending = append(pending, f.Name())
		_, e = f.Write(b)
		e = errors.Join(e, f.Close())
		if e != nil {
			return e
		}
		m.paths[a.Path] = f.Name()
	}
	if commit != nil {
		if e := commit(ctx); e != nil {
			return e
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	committed = true
	m.published = append(m.published, append([]media.Artifact(nil), artifacts...))
	return nil
}
func (m *exportTestMedia) Unbind(ctx context.Context, paths []string, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repo.mu.Lock()
	for _, path := range paths {
		for _, row := range m.repo.rows {
			if row.Path == path {
				m.repo.mu.Unlock()
				return errors.New("cannot unbind a rooted resource")
			}
		}
	}
	m.repo.mu.Unlock()
	if commit != nil {
		if e := commit(ctx); e != nil {
			return e
		}
	}
	for _, path := range paths {
		delete(m.paths, path)
	}
	return nil
}

type exportTestRepository struct {
	mu            sync.Mutex
	rows          map[string]Resource
	media         *exportTestMedia
	failKey       string
	failDeleteKey string
	beforeCAS     func(string, Resource)
}

func (r *exportTestRepository) Get(_ context.Context, key string) (Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.rows[key]
	if !ok {
		return Resource{}, blobstore.ErrNotFound
	}
	return v, nil
}
func (r *exportTestRepository) List(_ context.Context, prefix string) ([]Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Resource{}
	for k, v := range r.rows {
		if strings.HasPrefix(k, prefix) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *exportTestRepository) CAS(_ context.Context, key, expected string, next Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.beforeCAS != nil {
		r.beforeCAS(key, next)
	}
	if r.failKey == key {
		return errors.New("injected metadata failure")
	}
	old, exists := r.rows[key]
	if exists && old.Revision != expected {
		return ErrConflict
	}
	if !exists && expected != "" {
		bytes, err := os.ReadFile(r.media.paths[next.PreviousPath])
		if err != nil || fmt.Sprintf("sha256:%x", sha256.Sum256(bytes)) != expected {
			return ErrConflict
		}
	}
	bytes, err := os.ReadFile(r.media.paths[next.Path])
	if err != nil {
		return err
	}
	if fmt.Sprintf("sha256:%x", sha256.Sum256(bytes)) != next.Revision {
		return errors.New("resource revision does not match published bytes")
	}
	next.Key = key
	next.Version = old.Version + 1
	r.rows[key] = next
	previous := old.Path
	if !exists {
		previous = next.PreviousPath
	}
	if previous != next.Path {
		r.releasePath(previous)
	}
	return nil
}
func (r *exportTestRepository) Delete(_ context.Context, key, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failDeleteKey == key {
		return errors.New("injected resource deletion failure")
	}
	old, ok := r.rows[key]
	if ok && old.Revision != expected {
		return ErrConflict
	}
	delete(r.rows, key)
	r.releasePath(old.Path)
	return nil
}

// Mutations run inside Publish, which owns the media lock and rolls back both
// rows and bindings if any later CAS/Delete in the callback fails.
func (r *exportTestRepository) releasePath(path string) {
	for _, row := range r.rows {
		if row.Path == path {
			return
		}
	}
	delete(r.media.paths, path)
}

func (r *exportTestRepository) Paths(_ context.Context, prefix string) ([]string, error) {
	r.media.mu.Lock()
	defer r.media.mu.Unlock()
	out := []string{}
	for path := range r.media.paths {
		if strings.HasPrefix(path, prefix) {
			out = append(out, path)
		}
	}
	return out, nil
}

type exportTestGate struct {
	begin  func(context.Context, string, string) (Object, error)
	commit func(context.Context, string, Object, string, Object) (Object, error)
}

func (g exportTestGate) Begin(ctx context.Context, pid, rev string) (Object, error) {
	return g.begin(ctx, pid, rev)
}
func (g exportTestGate) Commit(ctx context.Context, pid string, session Object, file string, metadata Object) (Object, error) {
	if g.commit != nil {
		return g.commit(ctx, pid, session, file, metadata)
	}
	return Object{"reasons": []string{}, "score": Object{"total": 100}}, nil
}

func newExportStorageFixture(t *testing.T, clone bool) (*Service, Project, string) {
	t.Helper()
	root := t.TempDir()
	m := &exportTestMedia{root: root, paths: map[string]string{}}
	repo := &exportTestRepository{rows: map[string]Resource{}, media: m}
	m.repo = repo
	s := &Service{Media: m, Repository: repo, StagingDir: filepath.Join(root, "staged"), Now: func() time.Time { return time.Unix(100, 0) }}
	s.Store = Store{Media: m, Repository: repo, StagingDir: s.StagingDir, Now: s.Now}
	p, err := New("project", "Export", 1920, 1080, 30, s.now())
	if err != nil {
		t.Fatal(err)
	}
	p["timeline"] = Object{"tracks": []any{}, "items": []any{}}
	revision, err := s.Store.Create(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if clone {
		s.Gate = exportTestGate{begin: func(_ context.Context, _ string, rendered string) (Object, error) {
			if rendered != revision {
				return nil, apiError("conflict", "revision mismatch")
			}
			return Object{"jobId": "job", "projectRevision": revision, "settingsHash": "settings", "artifactHash": "artifacts"}, nil
		}}
	}
	return s, p, revision
}
func beginExportStorage(t *testing.T, s *Service, p Project, revision string) string {
	t.Helper()
	opts := BeginExportOptions{FileName: "out.mp4", MIME: "video/mp4"}
	if s.Gate != nil {
		opts.Clone = &BeginCloneExportOptions{ProjectRevision: revision}
	}
	id, e := s.BeginExport(context.Background(), exportString(p["id"]), opts)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func exportReadBytes(t *testing.T, s *Service, eid string) []byte {
	t.Helper()
	h, _, e := s.AcquireExport(context.Background(), "project", eid)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Release()
	b, e := os.ReadFile(h.Path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func requireExportCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestExportStorageLifecyclePrivateUntilCommit(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, false)
	_, e := s.BeginExport(ctx, "project", BeginExportOptions{FileName: "huge.mp4", ExpectedSize: 1 << 60})
	requireExportCode(t, e, "insufficient_storage")
	eid := beginExportStorage(t, s, p, rev)
	_, e = s.CommitExport(ctx, "project", eid, nil)
	requireExportCode(t, e, "invalid_request")
	if e = s.UploadExport(ctx, "project", eid, strings.NewReader("original render"), nil); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.AcquireExport(ctx, "project", eid)
	requireExportCode(t, e, "not_found")
	paths, e := s.listPaths(ctx, "editor/exports/")
	if e != nil || len(paths) != 0 {
		t.Fatalf("staged bytes published: %v, %v", paths, e)
	}
	record, e := s.CommitExport(ctx, "project", eid, nil)
	if e != nil {
		t.Fatal(e)
	}
	if record.Status != "ready" || record.Size != 15 || record.CommittedAt == nil {
		t.Fatalf("record = %+v", record)
	}
	if string(exportReadBytes(t, s, eid)) != "original render" {
		t.Fatal("wrong committed bytes")
	}
	_, e = s.CommitExport(ctx, "project", eid, nil)
	requireExportCode(t, e, "conflict")
	e = s.UploadExport(ctx, "project", eid, strings.NewReader("changed"), nil)
	requireExportCode(t, e, "conflict")
	listed, e := s.ListExports(ctx, "project")
	if e != nil || len(listed) != 1 {
		t.Fatalf("list = %v, %v", listed, e)
	}
	evidence, e := s.ReadExportEvidence(ctx, "project", eid)
	if e != nil || len(exportString(evidence["stagedSha256"])) != 64 {
		t.Fatalf("evidence = %v, %v", evidence, e)
	}
	if e = s.DeleteExport(ctx, "project", eid); e != nil {
		t.Fatal(e)
	}
	listed, e = s.ListExports(ctx, "project")
	if e != nil || len(listed) != 0 {
		t.Fatalf("after delete = %v, %v", listed, e)
	}
}

func TestExportStorageChunkOffsetsRestartAndRollback(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, false)
	eid := beginExportStorage(t, s, p, rev)
	upload := func(text string, offset int64) error {
		return s.UploadExport(ctx, "project", eid, strings.NewReader(text), &offset)
	}
	requireExportCode(t, upload("second", 6), "conflict")
	if e := upload("first", 0); e != nil {
		t.Fatal(e)
	}
	if e := upload("-", 5); e != nil {
		t.Fatal(e)
	}
	requireExportCode(t, upload("lost", 99), "conflict")
	offset := int64(6)
	if e := s.UploadExport(ctx, "project", eid, &exportFailingReader{}, &offset); e == nil {
		t.Fatal("read failure accepted")
	}
	b, e := os.ReadFile(s.exportStagedFile("project", eid))
	if e != nil || string(b) != "first-" {
		t.Fatalf("failed chunk modified file: %q %v", b, e)
	}
	repo := s.Repository.(*exportTestRepository)
	repo.failKey = "exports/project/" + eid
	if e = upload("bad", 6); e == nil {
		t.Fatal("metadata failure accepted")
	}
	repo.failKey = ""
	if e = upload("tail", 6); e != nil {
		t.Fatal(e)
	}
	if _, e = s.CommitExport(ctx, "project", eid, nil); e != nil {
		t.Fatal(e)
	}
	if string(exportReadBytes(t, s, eid)) != "first-tail" {
		t.Fatal("wrong concatenation")
	}
	restart := beginExportStorage(t, s, p, rev)
	zero := int64(0)
	for _, text := range []string{"partial", "clean"} {
		if e = s.UploadExport(ctx, "project", restart, strings.NewReader(text), &zero); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.CommitExport(ctx, "project", restart, nil); e != nil {
		t.Fatal(e)
	}
	if string(exportReadBytes(t, s, restart)) != "clean" {
		t.Fatal("restart did not truncate")
	}
}

type exportFailingReader struct{ sent bool }

func (r *exportFailingReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "partial"), nil
	}
	return 0, io.ErrUnexpectedEOF
}

func TestExportStorageRequestSweepPreservesReadyAndOtherRequests(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newExportStorageFixture(t, false)
	begin := func(request string) string {
		id, e := s.BeginExport(ctx, "project", BeginExportOptions{FileName: "request.mp4", RequestID: request})
		if e != nil {
			t.Fatal(e)
		}
		if e = s.UploadExport(ctx, "project", id, strings.NewReader("bytes"), nil); e != nil {
			t.Fatal(e)
		}
		return id
	}
	ready := begin("request-a")
	if _, e := s.CommitExport(ctx, "project", ready, nil); e != nil {
		t.Fatal(e)
	}
	staged := begin("request-a")
	other := begin("request-b")
	deleted, e := s.DeleteStagedExportsForRequest(ctx, "project", "request-a")
	if e != nil || len(deleted) != 1 || deleted[0] != staged {
		t.Fatalf("deleted = %v, %v", deleted, e)
	}
	if string(exportReadBytes(t, s, ready)) != "bytes" {
		t.Fatal("committed export swept")
	}
	if _, _, e = s.exportMeta(ctx, "project", other); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(s.exportDir("project", staged)); !os.IsNotExist(e) {
		t.Fatalf("scratch retained: %v", e)
	}
}

func TestExportStorageCloneGateRequiredAndFailuresRemainPrivate(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, false)
	s.ExportOwner = func(context.Context, string) (string, error) { return "job", nil }
	_, e := s.BeginExport(ctx, "project", BeginExportOptions{FileName: "clone.mp4"})
	requireExportCode(t, e, "invalid_request")
	s.ExportOwner = nil
	s.Gate = exportTestGate{begin: func(context.Context, string, string) (Object, error) {
		return Object{"projectRevision": rev, "jobId": "job"}, nil
	}, commit: func(context.Context, string, Object, string, Object) (Object, error) {
		return Object{"reasons": []string{"low-resolution output"}}, apiError("invalid_request", "low-resolution output", Object{"cloneGate": "final_check_failed"})
	}}
	_, e = s.BeginExport(ctx, "project", BeginExportOptions{FileName: "clone.mp4"})
	requireExportCode(t, e, "conflict")
	eid := beginExportStorage(t, s, p, rev)
	if e = s.UploadExport(ctx, "project", eid, strings.NewReader("render"), nil); e != nil {
		t.Fatal(e)
	}
	_, e = s.CommitExport(ctx, "project", eid, nil)
	requireExportCode(t, e, "invalid_request")
	_, _, e = s.AcquireExport(ctx, "project", eid)
	requireExportCode(t, e, "not_found")
	evidence, e := s.ReadExportEvidence(ctx, "project", eid)
	if e != nil || exportObject(evidence["gateDiagnostics"]) == nil {
		t.Fatalf("diagnostics = %v, %v", evidence, e)
	}
}

func TestExportStorageChecksumFailureNeverPersistsCoverage(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	fixture, _, metadata := captionPolicyFixture()
	fixture["id"] = "project"
	nextRev, e := s.Store.Write(ctx, "project", fixture, rev)
	if e != nil {
		t.Fatal(e)
	}
	rev = nextRev
	s.Gate = exportTestGate{begin: func(context.Context, string, string) (Object, error) {
		return Object{"jobId": "job", "projectRevision": rev}, nil
	}}
	supplied := exportObject(metadata["captionCoverage"])
	supplied["projectRevision"] = rev
	eid := beginExportStorage(t, s, p, rev)
	if e = s.UploadExport(ctx, "project", eid, strings.NewReader("render"), nil); e != nil {
		t.Fatal(e)
	}
	before, revision, e := s.exportMeta(ctx, "project", eid)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.exportStagedFile("project", eid), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e = s.CommitExport(ctx, "project", eid, metadata)
	requireExportCode(t, e, "invalid_request")
	after, afterRevision, e := s.exportMeta(ctx, "project", eid)
	if e != nil {
		t.Fatal(e)
	}
	if revision != afterRevision || exportObject(after["clone"])["captionCoverage"] != nil {
		t.Fatalf("failed commit changed metadata: %v => %v", before, after)
	}
	if e = s.UploadExport(ctx, "project", eid, strings.NewReader("render"), nil); e != nil {
		t.Fatal(e)
	}
	if _, e = s.CommitExport(ctx, "project", eid, metadata); e != nil {
		t.Fatal(e)
	}
	record := exportPublic(after)
	b, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "clone") || strings.Contains(string(b), "captionCoverage") {
		t.Fatal("private evidence leaked")
	}
}

func TestExportStorageReplacementAtomicCancellationAndDetachedSnapshot(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	eid := beginExportStorage(t, s, p, rev)
	if e := s.UploadExport(ctx, "project", eid, strings.NewReader("original committed render"), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := s.CommitExport(ctx, "project", eid, nil); e != nil {
		t.Fatal(e)
	}
	original := string(exportReadBytes(t, s, eid))
	failed := errors.New("encoder crashed")
	_, e := s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, _, output string, _ Project, _ Object) error {
		if e := os.WriteFile(output, []byte("partial"), 0600); e != nil {
			return e
		}
		return failed
	})
	if !errors.Is(e, failed) {
		t.Fatal(e)
	}
	if string(exportReadBytes(t, s, eid)) != original {
		t.Fatal("failed replacement overwrote committed bytes")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	_, e = s.ReplaceCloneExportFile(cancelCtx, "project", eid, func(_ context.Context, _, output string, _ Project, _ Object) error {
		cancel()
		return os.WriteFile(output, []byte("partial"), 0600)
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if string(exportReadBytes(t, s, eid)) != original {
		t.Fatal("cancel overwrote committed bytes")
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, _, output string, snapshot Project, _ Object) error {
			close(entered)
			<-release
			if snapshot["name"] != "Export" {
				return errors.New("snapshot changed during encoding")
			}
			return os.WriteFile(output, []byte("compressed"), 0600)
		})
		result <- err
	}()
	<-entered
	p["name"] = "Edited during encoding"
	if _, e = s.Store.Write(ctx, "project", p, rev); e != nil {
		close(release)
		t.Fatal(e)
	}
	close(release)
	if e = <-result; e != nil {
		t.Fatal(e)
	}
	if string(exportReadBytes(t, s, eid)) != "compressed" {
		t.Fatal("successful replacement not published")
	}
	invoked := false
	_, e = s.ReplaceCloneExportFile(ctx, "project", eid, func(context.Context, string, string, Project, Object) error { invoked = true; return nil })
	requireExportCode(t, e, "conflict")
	if invoked {
		t.Fatal("stale geometry reached encoder")
	}
}

func TestExportStorageReplacementMetadataFailureRetainsOldFile(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	eid := beginExportStorage(t, s, p, rev)
	if e := s.UploadExport(ctx, "project", eid, strings.NewReader("original"), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := s.CommitExport(ctx, "project", eid, nil); e != nil {
		t.Fatal(e)
	}
	repo := s.Repository.(*exportTestRepository)
	repo.failKey = "exports/project/" + eid
	_, e := s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, _, out string, _ Project, _ Object) error {
		return os.WriteFile(out, []byte("new"), 0600)
	})
	if e == nil {
		t.Fatal("failed CAS accepted")
	}
	repo.failKey = ""
	if string(exportReadBytes(t, s, eid)) != "original" {
		t.Fatal("failed CAS changed downloadable bytes")
	}
	paths, e := s.listPaths(ctx, "editor/exports/project/"+eid+"/files/")
	if e != nil || len(paths) != 1 {
		t.Fatalf("replacement leaked binding: %v, %v", paths, e)
	}
}

func TestExportStorageCoveragePrivateDetachedAndOrdinaryRejected(t *testing.T) {
	ctx := context.Background()
	s, _, revision := newExportStorageFixture(t, true)
	p, _, metadata := captionPolicyFixture()
	p["id"] = "project"
	revision, e := s.Store.Write(ctx, "project", p, revision)
	if e != nil {
		t.Fatal(e)
	}
	exportObject(metadata["captionCoverage"])["projectRevision"] = revision
	s.Gate = exportTestGate{begin: func(context.Context, string, string) (Object, error) {
		return Object{"jobId": "job", "projectRevision": revision}, nil
	}}
	eid := beginExportStorage(t, s, p, revision)
	if e = s.UploadExport(ctx, "project", eid, strings.NewReader("caption render"), nil); e != nil {
		t.Fatal(e)
	}
	if _, e = s.CommitExport(ctx, "project", eid, metadata); e != nil {
		t.Fatal(e)
	}
	expected, e := json.Marshal(exportCoverage(metadata))
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		_, e = s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, input, output string, snapshot Project, coverage Object) error {
			actual, e := json.Marshal(coverage)
			if e != nil {
				return e
			}
			if string(actual) != string(expected) {
				return errors.New("persisted caption coverage mutated")
			}
			if exportNumber(snapshot.Metadata()["width"]) != 3840 {
				return errors.New("wrong snapshot geometry")
			}
			exportObject(exportObject(exportArray(coverage["entries"])[0])["bounds"])["width"] = 1
			bytes, e := os.ReadFile(input)
			if e != nil {
				return e
			}
			return os.WriteFile(output, bytes, 0600)
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	record, e := s.ListExports(ctx, "project")
	if e != nil {
		t.Fatal(e)
	}
	public, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(public), "clone") || strings.Contains(string(public), "captionCoverage") {
		t.Fatal("private coverage leaked")
	}
	ordinary, ordinaryProject, ordinaryRevision := newExportStorageFixture(t, false)
	unbound := beginExportStorage(t, ordinary, ordinaryProject, ordinaryRevision)
	if e = ordinary.UploadExport(ctx, "project", unbound, strings.NewReader("ordinary"), nil); e != nil {
		t.Fatal(e)
	}
	_, e = ordinary.CommitExport(ctx, "project", unbound, metadata)
	requireExportCode(t, e, "invalid_request")
	if _, e = ordinary.CommitExport(ctx, "project", unbound, nil); e != nil {
		t.Fatal(e)
	}
	invoked := false
	_, e = ordinary.ReplaceCloneExportFile(ctx, "project", unbound, func(context.Context, string, string, Project, Object) error { invoked = true; return nil })
	requireExportCode(t, e, "conflict")
	if invoked {
		t.Fatal("ordinary export reached clone encoder")
	}
}

func TestExportStorageCommitSerializesUploadWithoutBlockingProjectEdits(t *testing.T) {
	ctx := context.Background()
	s, p, revision := newExportStorageFixture(t, true)
	entered := make(chan struct{})
	release := make(chan struct{})
	s.Gate = exportTestGate{begin: func(context.Context, string, string) (Object, error) {
		return Object{"jobId": "job", "projectRevision": revision}, nil
	}, commit: func(context.Context, string, Object, string, Object) (Object, error) {
		close(entered)
		<-release
		return Object{"reasons": []string{"output too small"}}, apiError("invalid_request", "output too small")
	}}
	eid := beginExportStorage(t, s, p, revision)
	if e := s.UploadExport(ctx, "project", eid, strings.NewReader("render"), nil); e != nil {
		t.Fatal(e)
	}
	committed := make(chan error, 1)
	go func() { _, e := s.CommitExport(ctx, "project", eid, nil); committed <- e }()
	<-entered
	uploaded := make(chan error, 1)
	go func() { uploaded <- s.UploadExport(ctx, "project", eid, strings.NewReader("retry"), nil) }()
	p["name"] = "Edited while probing"
	if _, e := s.Store.Write(ctx, "project", p, revision); e != nil {
		close(release)
		t.Fatal(e)
	}
	select {
	case e := <-uploaded:
		close(release)
		t.Fatalf("upload bypassed commit lock: %v", e)
	default:
	}
	close(release)
	requireExportCode(t, <-committed, "invalid_request")
	if e := <-uploaded; e != nil {
		t.Fatal(e)
	}
	_, _, e := s.AcquireExport(ctx, "project", eid)
	requireExportCode(t, e, "not_found")
}

func TestExportStoragePublicationRootsBinaryAndMetadataAtomically(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, false)
	eid := beginExportStorage(t, s, p, rev)
	if err := s.UploadExport(ctx, "project", eid, strings.NewReader("durable render"), nil); err != nil {
		t.Fatal(err)
	}
	m := s.Media.(*exportTestMedia)
	before := len(m.published)
	if _, err := s.CommitExport(ctx, "project", eid, nil); err != nil {
		t.Fatal(err)
	}
	repo := s.Repository.(*exportTestRepository)
	key := "exports/project/" + eid
	binary, err := repo.Get(ctx, key+"/file")
	if err != nil {
		t.Fatalf("binary has no durable resource: %v", err)
	}
	meta, _, err := s.exportMeta(ctx, "project", eid)
	if err != nil {
		t.Fatal(err)
	}
	if binary.Path != meta["filePath"] || binary.Revision != "sha256:"+exportString(meta["stagedSha256"]) {
		t.Fatalf("binary root does not match metadata: %+v, %v", binary, meta)
	}
	jsonHead, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.published) != before+1 || len(m.published[before]) != 2 {
		t.Fatalf("binary and JSON did not share one publication: %+v", m.published[before:])
	}
	paths := map[string]bool{}
	for _, artifact := range m.published[before] {
		paths[artifact.Path] = true
	}
	if !paths[binary.Path] || !paths[jsonHead.Path] {
		t.Fatalf("publication omitted a durable head: %v", paths)
	}
}

func TestExportStorageBinaryRootRollsBackOnMetadataFailure(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	eid := beginExportStorage(t, s, p, rev)
	if err := s.UploadExport(ctx, "project", eid, strings.NewReader("old durable render"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitExport(ctx, "project", eid, nil); err != nil {
		t.Fatal(err)
	}
	repo := s.Repository.(*exportTestRepository)
	key := "exports/project/" + eid
	oldBinary, err := repo.Get(ctx, key+"/file")
	if err != nil {
		t.Fatal(err)
	}
	oldMeta, oldRevision, err := s.exportMeta(ctx, "project", eid)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Media.(*exportTestMedia)
	oldFile := m.paths[oldBinary.Path]
	attempted := Resource{}
	repo.beforeCAS = func(key string, next Resource) {
		if key == oldBinary.Key {
			attempted = next
		}
	}
	repo.failKey = key
	_, err = s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, _, output string, _ Project, _ Object) error {
		return os.WriteFile(output, []byte("new durable render"), 0600)
	})
	if err == nil {
		t.Fatal("metadata failure accepted")
	}
	repo.failKey = ""
	repo.beforeCAS = nil
	if attempted.Path == "" || attempted.Path == oldBinary.Path {
		t.Fatal("replacement did not attempt binary CAS before metadata failure")
	}
	current, err := repo.Get(ctx, key+"/file")
	if err != nil || current != oldBinary {
		t.Fatalf("failed metadata CAS changed binary root: %+v, %v", current, err)
	}
	meta, revision, err := s.exportMeta(ctx, "project", eid)
	if err != nil || revision != oldRevision || meta["filePath"] != oldMeta["filePath"] {
		t.Fatalf("failed publication changed JSON head: %v, %v", meta, err)
	}
	if _, exists := m.paths[attempted.Path]; exists {
		t.Fatal("failed publication retained the new binary binding")
	}
	if bytes, err := os.ReadFile(oldFile); err != nil || string(bytes) != "old durable render" {
		t.Fatalf("old bytes lost on rollback: %q, %v", bytes, err)
	}
	if string(exportReadBytes(t, s, eid)) != "old durable render" {
		t.Fatal("rollback changed downloadable bytes")
	}
}

func TestExportStorageLegacyReplacementUsesPersistedOldHash(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	eid := beginExportStorage(t, s, p, rev)
	if err := s.UploadExport(ctx, "project", eid, strings.NewReader("legacy bytes"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitExport(ctx, "project", eid, nil); err != nil {
		t.Fatal(err)
	}
	repo := s.Repository.(*exportTestRepository)
	key := "exports/project/" + eid
	// Model a pre-binary-root ready export without releasing its legacy binding.
	repo.mu.Lock()
	delete(repo.rows, key+"/file")
	repo.mu.Unlock()
	oldMeta, _, err := s.exportMeta(ctx, "project", eid)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := exportString(oldMeta["filePath"])
	m := s.Media.(*exportTestMedia)
	oldFile := m.paths[oldPath]
	observed := false
	repo.beforeCAS = func(casKey string, next Resource) {
		if casKey == key+"/file" {
			observed = true
			if next.PreviousPath != oldPath {
				t.Errorf("legacy adoption lost prior path: %+v", next)
			}
		}
		if casKey == key {
			if bytes, err := os.ReadFile(oldFile); err != nil || string(bytes) != "legacy bytes" {
				t.Errorf("old bytes reclaimed before commit: %q, %v", bytes, err)
			}
		}
	}
	if _, err := s.ReplaceCloneExportFile(ctx, "project", eid, func(_ context.Context, _, output string, _ Project, _ Object) error {
		return os.WriteFile(output, []byte("different replacement bytes"), 0600)
	}); err != nil {
		t.Fatal(err)
	}
	repo.beforeCAS = nil
	if !observed {
		t.Fatal("legacy replacement did not publish a binary root")
	}
	if _, exists := m.paths[oldPath]; exists {
		t.Fatal("successful replacement retained obsolete binary binding")
	}
	if string(exportReadBytes(t, s, eid)) != "different replacement bytes" {
		t.Fatal("replacement did not expose new bytes")
	}
}

func TestExportStorageDeleteFailurePreservesBinaryRootAndBytes(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, false)
	eid := beginExportStorage(t, s, p, rev)
	if err := s.UploadExport(ctx, "project", eid, strings.NewReader("preserved export"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitExport(ctx, "project", eid, nil); err != nil {
		t.Fatal(err)
	}
	repo := s.Repository.(*exportTestRepository)
	key := "exports/project/" + eid
	binary, err := repo.Get(ctx, key+"/file")
	if err != nil {
		t.Fatal(err)
	}
	_, revision, err := s.exportMeta(ctx, "project", eid)
	if err != nil {
		t.Fatal(err)
	}
	repo.failDeleteKey = key
	if err := s.DeleteExport(ctx, "project", eid); err == nil {
		t.Fatal("failed resource deletion accepted")
	}
	repo.failDeleteKey = ""
	current, err := repo.Get(ctx, key+"/file")
	if err != nil || current != binary {
		t.Fatalf("failed deletion changed binary root: %+v, %v", current, err)
	}
	if _, after, err := s.exportMeta(ctx, "project", eid); err != nil || after != revision {
		t.Fatalf("failed deletion changed metadata: %s, %v", after, err)
	}
	if string(exportReadBytes(t, s, eid)) != "preserved export" {
		t.Fatal("failed deletion lost valid bytes")
	}
	if err := s.DeleteExport(ctx, "project", eid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, key+"/file"); !exportMissing(err) {
		t.Fatalf("successful deletion retained binary root: %v", err)
	}
	if _, exists := s.Media.(*exportTestMedia).paths[binary.Path]; exists {
		t.Fatal("successful deletion retained binary binding")
	}
}

func TestExportStoragePublicationCancellationRetainsOldBinaryRoot(t *testing.T) {
	ctx := context.Background()
	s, p, rev := newExportStorageFixture(t, true)
	eid := beginExportStorage(t, s, p, rev)
	if err := s.UploadExport(ctx, "project", eid, strings.NewReader("old committed bytes"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitExport(ctx, "project", eid, nil); err != nil {
		t.Fatal(err)
	}
	repo := s.Repository.(*exportTestRepository)
	key := "exports/project/" + eid
	binary, err := repo.Get(ctx, key+"/file")
	if err != nil {
		t.Fatal(err)
	}
	_, revision, err := s.exportMeta(ctx, "project", eid)
	if err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	attempted := Resource{}
	repo.beforeCAS = func(casKey string, next Resource) {
		if casKey == key+"/file" {
			attempted = next
		}
		if casKey == key {
			cancel()
		}
	}
	_, err = s.ReplaceCloneExportFile(cancelCtx, "project", eid, func(_ context.Context, _, output string, _ Project, _ Object) error {
		return os.WriteFile(output, []byte("uncommitted bytes"), 0600)
	})
	repo.beforeCAS = nil
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publication cancellation = %v", err)
	}
	current, err := repo.Get(ctx, key+"/file")
	if err != nil || current != binary {
		t.Fatalf("cancellation changed binary root: %+v, %v", current, err)
	}
	if _, after, err := s.exportMeta(ctx, "project", eid); err != nil || after != revision {
		t.Fatalf("cancellation changed metadata: %s, %v", after, err)
	}
	if attempted.Path == "" {
		t.Fatal("cancellation did not reach binary publication")
	}
	if _, exists := s.Media.(*exportTestMedia).paths[attempted.Path]; exists {
		t.Fatal("cancellation retained new binary binding")
	}
	if string(exportReadBytes(t, s, eid)) != "old committed bytes" {
		t.Fatal("cancellation lost committed bytes")
	}
}
