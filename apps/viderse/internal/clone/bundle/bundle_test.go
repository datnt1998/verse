package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

func TestSanitizePartAndUploadInfo(t *testing.T) {
	if got := SanitizePart(" ../A\\B/:*?\x00  . ", "video"); strings.ContainsAny(got, "/\\:*?\x00") || strings.Contains(got, "..") {
		t.Fatalf("unsafe name: %q", got)
	}
	if got := SanitizePart("   ", "video"); got != "video" {
		t.Fatal(got)
	}
	if got := UploadInfo("Title", "Description", []string{"a", "b"}); got != "TIÊU ĐỀ\nTitle\n\nMÔ TẢ\nDescription\n\nTAGS\na, b\n" {
		t.Fatal(got)
	}
}

type fixtureStore struct{ snapshot jobedit.Snapshot }

func (f *fixtureStore) Read(_ context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	s := f.snapshot
	s.Step = domain.CloneStepRecord{Step: step}
	for _, record := range s.Steps {
		if record.Step == step {
			s.Step = record
		}
	}
	return s, nil
}
func (*fixtureStore) Commit(context.Context, jobedit.Snapshot, jobedit.Change) error { return nil }

type fixtureMedia struct{ root string }

func (m fixtureMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	file := filepath.Join(m.root, path)
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}
	return &media.Handle{Path: file}, nil
}
func (m fixtureMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return nil
}

type fixtureExports struct{ file string }

func (e fixtureExports) AcquireExport(context.Context, string, string) (*media.Handle, editor.ExportRecord, error) {
	return &media.Handle{Path: e.file}, editor.ExportRecord{Status: "ready", FileName: "video.mp4"}, nil
}
func TestBundleLimitsAndStreamsOwnedCommittedExport(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "video.mp4")
	if err := os.WriteFile(file, []byte("real-video-content"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := jobedit.Snapshot{Job: domain.CloneJob{ID: "job", ChannelID: "channel", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Owner title"}}, Steps: []domain.CloneStepRecord{{Step: domain.CloneBuild, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}}, {Step: domain.CloneExport, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project", "exportId": "export"}}}}}
	service := Service{Edits: &jobedit.Service{Store: &fixtureStore{snapshot: snapshot}, Media: fixtureMedia{root}}, Exports: fixtureExports{file}}
	for _, ids := range [][]string{nil, make([]string, 51)} {
		if _, err := service.Prepare(context.Background(), ids); err == nil {
			t.Fatalf("accepted %d IDs", len(ids))
		}
	}
	prepared, err := service.Prepare(context.Background(), []string{"job"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := prepared.Write(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 || !strings.HasSuffix(archive.File[0].Name, "/Owner title.mp4") {
		t.Fatalf("entries: %+v", archive.File)
	}
	stream, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(stream)
	stream.Close()
	if err != nil || string(data) != "real-video-content" {
		t.Fatalf("export bytes: %q %v", data, err)
	}
}
func TestBundleRejectsUnfinishedOrRemovedExport(t *testing.T) {
	snapshot := jobedit.Snapshot{Job: domain.CloneJob{ID: "job", Status: domain.CloneWaiting}}
	store := &fixtureStore{snapshot: snapshot}
	service := Service{Edits: &jobedit.Service{Store: store}}
	if _, err := service.Prepare(context.Background(), []string{"job"}); err == nil {
		t.Fatal("unfinished export accepted")
	}
	store.snapshot.Job.Status = domain.CloneSucceeded
	store.snapshot.Steps = []domain.CloneStepRecord{{Step: domain.CloneExport, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Data: map[string]any{"removedAt": "now", "exportId": "export", "projectId": "project"}}}}
	if _, err := service.Prepare(context.Background(), []string{"job"}); err == nil {
		t.Fatal("removed export accepted")
	}
}
