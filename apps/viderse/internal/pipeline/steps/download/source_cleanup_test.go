package download

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type cleanupStore struct {
	jobs      []domain.CloneJob
	steps     map[string][]domain.CloneStepRecord
	artifacts map[string][]domain.CloneArtifact
	removed   []string
	guarded   bool
	guardErr  error
}

func (s *cleanupStore) ListFinishedJobs(context.Context, int) ([]domain.CloneJob, error) {
	return s.jobs, nil
}
func (s *cleanupStore) GetJob(_ context.Context, id string) (*domain.CloneJob, error) {
	for _, j := range s.jobs {
		if j.ID == id {
			return &j, nil
		}
	}
	return nil, errors.New("missing")
}
func (s *cleanupStore) ListSteps(_ context.Context, id string) ([]domain.CloneStepRecord, error) {
	return s.steps[id], nil
}
func (s *cleanupStore) ListArtifacts(_ context.Context, id string) ([]domain.CloneArtifact, error) {
	return s.artifacts[id], nil
}
func (s *cleanupStore) GuardSourceCleanup(context.Context, string, int64) error {
	s.guarded = true
	return s.guardErr
}
func (s *cleanupStore) RemoveArtifactsByPath(_ context.Context, id string, paths []string) error {
	s.removed = append(s.removed, paths...)
	out := s.artifacts[id][:0]
	for _, a := range s.artifacts[id] {
		if a.Path != paths[0] {
			out = append(out, a)
		}
	}
	s.artifacts[id] = out
	return nil
}

type cleanupMedia struct {
	files   map[string]string
	unbound []string
}

func (m *cleanupMedia) Acquire(_ context.Context, p string) (*media.Handle, error) {
	f, ok := m.files[p]
	if !ok {
		return nil, errors.New("missing media")
	}
	return &media.Handle{Path: f}, nil
}
func (m *cleanupMedia) Unbind(ctx context.Context, p []string, commit func(context.Context) error) error {
	if err := commit(ctx); err != nil {
		return err
	}
	m.unbound = append(m.unbound, p...)
	for _, v := range p {
		delete(m.files, v)
	}
	return nil
}

func cleanupFixture(t *testing.T) (*SourceCleaner, *cleanupStore, *cleanupMedia) {
	t.Helper()
	s := &cleanupStore{jobs: []domain.CloneJob{{ID: "done", Status: domain.CloneSucceeded, Source: domain.CloneSourceSnapshot{Title: "Finished"}, Version: 1}}, steps: map[string][]domain.CloneStepRecord{}, artifacts: map[string][]domain.CloneArtifact{}}
	d := t.TempDir()
	m := &cleanupMedia{files: map[string]string{}}
	put := func(p string, b []byte) {
		f := filepath.Join(d, p)
		if err := os.MkdirAll(filepath.Dir(f), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, b, 0600); err != nil {
			t.Fatal(err)
		}
		m.files[p] = f
	}
	put("download/source.mp4", []byte("source"))
	put("download/source.info.json", []byte("{}"))
	put("build/source-audio.m4a", []byte("audio"))
	put("editor/media/audio/source-audio.m4a", []byte("audio"))
	put("editor/media/image/a.webp", []byte("image"))
	put("editor/media/audio/metadata.json", []byte(`{"id":"audio","sourceFile":"source-audio.m4a","contentHash":"sha256:6ed8919ce20490a5e3ad8630a4fab69475297abd07db73918dd5f36fcfaeb11b"}`))
	put("editor/media/image/metadata.json", []byte(`{"id":"image","sourceFile":"a.webp","contentHash":"sha256:6105d6cc76af400325e94d588ce511be5bfdbb73b437dc51eca43917d7a43e3d"}`))
	put("build/checkpoint.json", []byte(`{"version":1,"projectId":"done","audioSha256":"6ed8919ce20490a5e3ad8630a4fab69475297abd07db73918dd5f36fcfaeb11b"}`))
	project := map[string]any{"schemaVersion": 15, "timeline": map[string]any{"items": []any{map[string]any{"type": "audio", "mediaId": "audio"}, map[string]any{"type": "image", "mediaId": "image"}}}}
	raw, _ := json.Marshal(project)
	put("editor/projects/done/project.json", raw)
	s.artifacts["done"] = []domain.CloneArtifact{{Path: "download/source.mp4", Bytes: 6, Step: domain.CloneDownload}, {Path: "download/source.info.json", Step: domain.CloneDownload}, {Path: "editor/projects/done/project.json", Step: domain.CloneBuild}}
	s.steps["done"] = []domain.CloneStepRecord{
		{Step: domain.CloneBuild, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Data: map[string]any{"sourceIndependent": true, "projectPath": "editor/projects/done/project.json", "audioPath": "build/source-audio.m4a"}, Artifacts: []domain.CloneArtifact{{Path: "editor/projects/done/project.json"}}}},
		{Step: domain.CloneExport, Status: domain.CloneSucceeded},
	}
	return &SourceCleaner{Store: s, Media: m}, s, m
}

func TestSourceCleanupDryRunThenApplyKeepsSmallEvidence(t *testing.T) {
	c, s, m := cleanupFixture(t)
	preview, err := c.Reclaim(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || preview.FreedBytes != 6 || len(m.unbound) != 0 {
		t.Fatalf("preview=%+v", preview)
	}
	report, err := c.Reclaim(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 0 || report.FreedBytes != 6 || len(m.unbound) != 1 || !s.guarded {
		t.Fatalf("report=%+v unbound=%v guard=%v", report, m.unbound, s.guarded)
	}
	if _, ok := m.files["download/source.info.json"]; !ok {
		t.Fatal("small evidence removed")
	}
	again, err := c.Reclaim(context.Background(), true)
	if err != nil || len(again.Items) != 0 {
		t.Fatalf("second pass=%+v %v", again, err)
	}
}
func TestSourceCleanupKeepsRunnableOrSupersededJobs(t *testing.T) {
	for _, status := range []domain.CloneStatus{domain.CloneFailed, domain.CloneWaiting, domain.CloneRunning} {
		t.Run(string(status), func(t *testing.T) {
			c, s, m := cleanupFixture(t)
			s.jobs[0].Status = status
			r, e := c.Reclaim(context.Background(), false)
			if e != nil || len(r.Items) > 0 || len(m.unbound) > 0 {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
	c, s, _ := cleanupFixture(t)
	s.jobs[0].SupersededAt = new(time.Now())
	r, e := c.Reclaim(context.Background(), false)
	if e != nil || len(r.Items) > 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestSourceCleanupLeaseRaceRollsBackUnbind(t *testing.T) {
	c, s, m := cleanupFixture(t)
	s.guardErr = errors.New("worker acquired lease")
	r, e := c.Reclaim(context.Background(), false)
	if e != nil || len(r.Errors) != 1 || len(m.unbound) > 0 || len(r.Items) > 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestSourceCleanupKeepsTimelineReferencesAndMissingPlaybackAssets(t *testing.T) {
	for _, mode := range []string{"timeline", "missing-audio", "missing-project", "not-independent", "not-exported"} {
		t.Run(mode, func(t *testing.T) {
			c, s, m := cleanupFixture(t)
			switch mode {
			case "missing-audio":
				delete(m.files, "build/source-audio.m4a")
			case "missing-project":
				delete(m.files, "editor/projects/done/project.json")
			case "not-exported":
				s.steps["done"][1].Status = domain.CloneWaiting
			case "not-independent":
				s.steps["done"][0].Output.Data["sourceIndependent"] = false
			default:
				raw, _ := os.ReadFile(m.files["editor/projects/done/project.json"])
				var p map[string]any
				_ = json.Unmarshal(raw, &p)
				p["timeline"] = map[string]any{"src": "/api/media/download/source.mp4"}
				raw, _ = json.Marshal(p)
				_ = os.WriteFile(m.files["editor/projects/done/project.json"], raw, 0600)
			}
			r, e := c.Reclaim(context.Background(), false)
			if e != nil || len(r.Items) > 0 || len(m.unbound) > 0 {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}
