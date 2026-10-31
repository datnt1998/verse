package backdrop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	pipelineBackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/platform/blobstore"
)

type editStore struct {
	snapshot     jobedit.Snapshot
	changes      []jobedit.Change
	beforeCommit func()
}

func (s *editStore) Read(_ context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	if id != s.snapshot.Job.ID || step != domain.CloneBackdrop {
		return jobedit.Snapshot{}, &domain.CloneError{Code: domain.CloneErrNotFound}
	}
	return s.snapshot, nil
}
func (s *editStore) Commit(_ context.Context, old jobedit.Snapshot, change jobedit.Change) error {
	if s.beforeCommit != nil {
		s.beforeCommit()
		s.beforeCommit = nil
	}
	if s.snapshot.Job.SupersededAt != nil || s.snapshot.Job.Generation != old.Job.Generation {
		return &domain.CloneError{Code: domain.CloneErrNotFound}
	}
	if !bytes.Equal(old.Step.Checkpoint, s.snapshot.Step.Checkpoint) || jobedit.Busy(s.snapshot, domain.CloneBackdrop) {
		return &domain.CloneError{Code: domain.CloneErrConflict}
	}
	s.snapshot.Step.Checkpoint = append(json.RawMessage(nil), change.Checkpoint...)
	s.snapshot.Artifacts = append(s.snapshot.Artifacts, change.Artifacts...)
	s.changes = append(s.changes, change)
	return nil
}

type catalogueStore struct {
	pipelineBackdrop.Store
	videos []domain.BackdropChannelVideo
}

func (s *catalogueStore) Channel(_ context.Context, id string) (domain.BackdropChannel, bool, error) {
	return domain.BackdropChannel{ID: id, Platform: domain.PlatformTwitch, Handle: "alpha", Title: new("Alpha")}, id == "twitch:alpha", nil
}
func (s *catalogueStore) ChannelVideos(context.Context, string) ([]domain.BackdropChannelVideo, error) {
	return s.videos, nil
}
func (s *catalogueStore) ChannelVideo(_ context.Context, channel, id string) (domain.BackdropChannelVideo, bool, error) {
	for _, video := range s.videos {
		if video.ChannelID == channel && video.VideoID == id {
			return video, true, nil
		}
	}
	return domain.BackdropChannelVideo{}, false, nil
}
func (s *catalogueStore) Video(context.Context, string) (domain.BackdropVideo, bool, error) {
	return domain.BackdropVideo{}, false, nil
}

type mediaRepo struct {
	media.Repository
	descriptor media.Descriptor
	refs       map[string]media.Ref
}

func (r *mediaRepo) State(context.Context) (media.State, error) {
	return media.State{Active: "local", Generation: 1}, nil
}
func (r *mediaRepo) Descriptor(context.Context, string) (media.Descriptor, error) {
	return r.descriptor, nil
}
func (r *mediaRepo) Publication(ctx context.Context, commit func(context.Context, media.State) error) error {
	before := make(map[string]media.Ref, len(r.refs))
	for path, ref := range r.refs {
		before[path] = ref
	}
	if err := commit(ctx, media.State{Active: "local", Generation: 1}); err != nil {
		r.refs = before
		return err
	}
	return nil
}
func (r *mediaRepo) Lock(context.Context, string, string) error  { return nil }
func (r *mediaRepo) Bind(_ context.Context, ref media.Ref) error { r.refs[ref.Path] = ref; return nil }
func (r *mediaRepo) Resolve(_ context.Context, _, path string) (media.Ref, error) {
	if ref, ok := r.refs[path]; ok {
		return ref, nil
	}
	return media.Ref{}, blobstore.ErrNotFound
}
func (r *mediaRepo) ReadLease(context.Context, string, string) (func(), error) { return func() {}, nil }

type fixture struct {
	service   *Service
	store     *editStore
	catalogue *catalogueStore
	manifest  pipelineBackdrop.Manifest
	wakes     int
	events    []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{}
	f.manifest = pipelineBackdrop.Seal(pipelineBackdrop.Manifest{WantedCount: 2, Fallback: "none", Items: []pipelineBackdrop.Item{
		{ID: "b1", ChannelID: "twitch:alpha", VideoID: "v1", Kind: "clip", URL: "https://example.test/v1", StartSec: 10, EndSec: 40, Source: "random", Status: "ready", MediaID: new("media-1")},
		{ID: "b2", ChannelID: "twitch:alpha", VideoID: "v2", Kind: "clip", URL: "https://example.test/v2", StartSec: 20, EndSec: 50, Source: "owner", Status: "ready", MediaID: new("media-2")},
	}})
	raw, err := json.Marshal(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	template, err := json.Marshal(library.Snapshot{Backdrop: &domain.BackdropRecipe{Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:alpha", Kinds: []string{"clip"}}}, ClipCount: new(2), ClipSec: 30}, Appearance: domain.BackgroundAppearance{Speed: 2}})
	if err != nil {
		t.Fatal(err)
	}
	f.store = &editStore{snapshot: jobedit.Snapshot{Job: domain.CloneJob{ID: "job-1", Generation: 1, TemplateSnapshot: template}, Step: domain.CloneStepRecord{JobID: "job-1", Step: domain.CloneBackdrop, Status: domain.CloneSucceeded, Checkpoint: raw}, Steps: []domain.CloneStepRecord{{Step: domain.CloneDownload, Status: domain.CloneSucceeded, Output: &domain.CloneStepOutput{Data: map[string]any{"durationSec": 130.0}}}}}}
	f.catalogue = &catalogueStore{}
	for _, id := range []string{"v1", "v2", "v3", "v4"} {
		f.catalogue.videos = append(f.catalogue.videos, domain.BackdropChannelVideo{ChannelID: "twitch:alpha", VideoID: id, Kind: "clip", URL: "https://example.test/" + id, Title: id, DurationSec: new(300.0), ThumbnailURL: new("https://example.test/thumb.jpg")})
	}
	root := t.TempDir()
	local, err := blobstore.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	repo := &mediaRepo{descriptor: media.Descriptor{ID: "local", Config: blobstore.Config{Driver: "local", LocalRoot: root}}, refs: map[string]media.Ref{}}
	edits := &jobedit.Service{Store: f.store, Media: media.New(repo, nil, func(media.Descriptor) (blobstore.BlobStore, error) { return local, nil }), Wake: func() { f.wakes++ }, Publish: func(event string, _ any) { f.events = append(f.events, event) }}
	f.service = &Service{Edits: edits, Deps: pipelineBackdrop.Deps{Store: f.catalogue, RefreshChannel: func(context.Context, string, time.Duration) error { return nil }, ProbeVideo: func(context.Context, string, string) (domain.BackdropChannelVideo, error) {
		return domain.BackdropChannelVideo{}, errors.New("unexpected probe")
	}}}
	return f
}
func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var cloneError *domain.CloneError
	if !errors.As(err, &cloneError) || cloneError.Code != code {
		t.Fatalf("error=%v; want %s", err, code)
	}
}
func storedManifest(t *testing.T, f *fixture) pipelineBackdrop.Manifest {
	t.Helper()
	checkpoint, err := pipelineBackdrop.ParseCheckpoint(f.store.snapshot.Step.Checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	return checkpoint.Manifest
}

func TestRemovePersistsAndPublishesManifestWithoutRequeueingBackdrop(t *testing.T) {
	f := newFixture(t)
	result, err := f.service.Edit(context.Background(), "job-1", EditRequest{Op: "remove", ItemID: "b1", Revision: f.manifest.Revision})
	if err != nil {
		t.Fatal(err)
	}
	manifest := storedManifest(t, f)
	if !result.Accepted || result.JobID != "job-1" || result.Revision != manifest.Revision || result.RunID != nil || !reflect.DeepEqual(result.Invalidated, []domain.CloneStep{domain.CloneBuild, domain.CloneScore, domain.CloneExport}) {
		t.Fatal(result)
	}
	if manifest.WantedCount != 1 || len(manifest.Items) != 1 || manifest.Items[0].ID != "b2" || len(f.store.changes) != 1 || f.store.changes[0].Rerun || f.store.changes[0].InvalidateFrom != domain.CloneBuild {
		t.Fatal(manifest, f.store.changes)
	}
	if f.wakes != 1 || !reflect.DeepEqual(f.events, []string{domain.CloneEventUpdated}) {
		t.Fatal(f.wakes, f.events)
	}
	snapshot, err := f.service.Edits.Read(context.Background(), "job-1", domain.CloneBackdrop)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := f.service.Edits.Acquire(context.Background(), snapshot, "backdrop/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()
	published, err := os.ReadFile(handle.Path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := pipelineBackdrop.Parse(published)
	if err != nil || !reflect.DeepEqual(parsed, manifest) {
		t.Fatal(parsed, err)
	}
	// A successful worker clears its checkpoint: subsequent reads must still see the edit.
	f.store.snapshot.Step.Checkpoint = nil
	view, err := f.service.Read(context.Background(), "job-1")
	if err != nil || view == nil || view.Revision != manifest.Revision || len(view.Items) != 1 || view.Items[0].ID != "b2" {
		t.Fatal(view, err)
	}
}

func TestReadUsesSnapshotCatalogueAndPlaybackCoverageWithoutNetwork(t *testing.T) {
	f := newFixture(t)
	f.service.Deps.RefreshChannel = func(context.Context, string, time.Duration) error { t.Fatal("read refreshed catalogue"); return nil }
	view, err := f.service.Read(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.Coverage.Mode != "count" || view.Coverage.VideoSec == nil || *view.Coverage.VideoSec != 130 || view.Coverage.ChannelSec != 30 || view.ReadyCount != 2 || !view.RandomPicks || view.Busy || *view.Items[0].ChannelTitle != "Alpha" || *view.Items[0].VideoDurationSec != 300 || view.Items[0].StartSec != 10 {
		t.Fatal(view)
	}
}
func TestRevisionRunningAndSupersededRefuseWithoutMutation(t *testing.T) {
	for _, name := range []string{"revision", "backdrop", "build", "score", "export", "superseded"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			input := EditRequest{Op: "remove", ItemID: "b1", Revision: f.manifest.Revision}
			code := domain.CloneErrConflict
			switch name {
			case "revision":
				input.Revision = "old"
			case "superseded":
				f.store.snapshot.Job.SupersededAt = new(time.Now())
				code = domain.CloneErrNotFound
			default:
				f.store.snapshot.Steps = append(f.store.snapshot.Steps, domain.CloneStepRecord{Step: domain.CloneStep(name), Status: domain.CloneRunning})
			}
			_, err := f.service.Edit(context.Background(), "job-1", input)
			assertCode(t, err, code)
			if len(f.store.changes) != 0 || f.wakes != 0 || storedManifest(t, f).Revision != f.manifest.Revision {
				t.Fatal("rejected edit mutated state")
			}
		})
	}
}
func TestInterveningMutationIsFencedAfterCatalogueCompute(t *testing.T) {
	for _, mutation := range []string{"checkpoint", "generation", "superseded", "running"} {
		t.Run(mutation, func(t *testing.T) {
			f := newFixture(t)
			f.store.beforeCommit = func() {
				switch mutation {
				case "checkpoint":
					f.store.snapshot.Step.Checkpoint = json.RawMessage(`{"intervening":true}`)
				case "generation":
					f.store.snapshot.Job.Generation++
				case "superseded":
					f.store.snapshot.Job.SupersededAt = new(time.Now())
				case "running":
					f.store.snapshot.Steps = append(f.store.snapshot.Steps, domain.CloneStepRecord{Step: domain.CloneBuild, Status: domain.CloneRunning})
				}
			}
			_, err := f.service.Edit(context.Background(), "job-1", EditRequest{Op: "add", VideoChoice: pipelineBackdrop.VideoChoice{ChannelID: "twitch:alpha", VideoID: "v3", StartSec: new(10.0), EndSec: new(40.0)}})
			code := domain.CloneErrConflict
			if mutation == "generation" || mutation == "superseded" {
				code = domain.CloneErrNotFound
			}
			assertCode(t, err, code)
			if len(f.store.changes) != 0 || f.wakes != 0 || len(f.events) != 0 {
				t.Fatal("fenced edit committed")
			}
		})
	}
}
func TestChosenSwapAndRandomRerollPersistPendingClips(t *testing.T) {
	for _, op := range []string{"swap", "reroll", "add"} {
		t.Run(op, func(t *testing.T) {
			f := newFixture(t)
			input := EditRequest{Op: op, ItemID: "b1"}
			if op == "swap" {
				input.VideoChoice = pipelineBackdrop.VideoChoice{ChannelID: "twitch:alpha", VideoID: "v3", StartSec: new(20.0), EndSec: new(50.0)}
			}
			result, err := f.service.Edit(context.Background(), "job-1", input)
			if err != nil {
				t.Fatal(err)
			}
			manifest := storedManifest(t, f)
			if !f.store.changes[0].Rerun || f.store.changes[0].InvalidateFrom != domain.CloneBackdrop || len(result.Invalidated) != 4 || result.Invalidated[0] != domain.CloneBackdrop {
				t.Fatal(result, f.store.changes)
			}
			if !reflect.DeepEqual(manifest.Items[1], f.manifest.Items[1]) {
				t.Fatal("owner clip changed")
			}
			pending := manifest.Items[0]
			if op == "add" {
				pending = manifest.Items[2]
			}
			if pending.Status != "pending" || pending.MediaID != nil || pending.EndSec-pending.StartSec != 30 || !strings.HasPrefix(pending.URL, "https://example.test/") {
				t.Fatal(pending)
			}
			if op == "swap" && (pending.VideoID != "v3" || pending.Source != "owner" || pending.StartSec != 20) {
				t.Fatal(pending)
			}
			if op == "reroll" && (pending.VideoID == "v1" || pending.Source != "random") {
				t.Fatal(pending)
			}
		})
	}
}
func TestInitialFillReadRevisionMatchesFirstEditAndDownloadedDuration(t *testing.T) {
	f := newFixture(t)
	f.store.snapshot.Step.Checkpoint = nil
	var template library.Snapshot
	if err := json.Unmarshal(f.store.snapshot.Job.TemplateSnapshot, &template); err != nil {
		t.Fatal(err)
	}
	template.Backdrop.ClipCount = nil
	raw, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	f.store.snapshot.Job.TemplateSnapshot = raw
	view, err := f.service.Read(context.Background(), "job-1")
	if err != nil || view == nil || view.Coverage.Mode != "fill" {
		t.Fatal(view, err)
	}
	_, err = f.service.Edit(context.Background(), "job-1", EditRequest{Op: "add", Revision: view.Revision, VideoChoice: pipelineBackdrop.VideoChoice{ChannelID: "twitch:alpha", VideoID: "v3", StartSec: new(0.0), EndSec: new(30.0)}})
	if err != nil {
		t.Fatal(err)
	}
	manifest := storedManifest(t, f)
	if manifest.TargetSec == nil || *manifest.TargetSec != 290 {
		t.Fatal(manifest)
	}
}
func TestCorruptCheckpointAndUnknownOperationDoNotCommit(t *testing.T) {
	f := newFixture(t)
	_, err := f.service.Edit(context.Background(), "job-1", EditRequest{Op: "unknown"})
	assertCode(t, err, domain.CloneErrInvalidRequest)
	f.store.snapshot.Step.Checkpoint = json.RawMessage(`{"manifest":null}`)
	_, err = f.service.Read(context.Background(), "job-1")
	assertCode(t, err, domain.CloneErrInternal)
	if len(f.store.changes) != 0 {
		t.Fatal("corrupt edit committed")
	}
}

func TestWrappedCheckpointKeepsStagedClipsAndPublishesBareManifest(t *testing.T) {
	f := newFixture(t)
	artifacts := []media.Artifact{{Path: "backdrop/ready.mp4", File: "/staged/ready.mp4", ContentType: "video/mp4"}}
	raw, err := json.Marshal(pipelineBackdrop.Checkpoint{Manifest: f.manifest, Artifacts: artifacts})
	if err != nil {
		t.Fatal(err)
	}
	f.store.snapshot.Step.Checkpoint = raw
	_, err = f.service.Edit(context.Background(), "job-1", EditRequest{Op: "remove", ItemID: "b1"})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := pipelineBackdrop.ParseCheckpoint(f.store.snapshot.Step.Checkpoint)
	if err != nil || !reflect.DeepEqual(checkpoint.Artifacts, artifacts) {
		t.Fatal(checkpoint, err)
	}
	snapshot, err := f.service.Edits.Read(context.Background(), "job-1", domain.CloneBackdrop)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := f.service.Edits.Acquire(context.Background(), snapshot, "backdrop/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()
	published, err := os.ReadFile(handle.Path)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := pipelineBackdrop.Parse(published)
	if err != nil || manifest.Revision != checkpoint.Manifest.Revision {
		t.Fatal(manifest, err)
	}
}

func TestUnknownJobAndCancellationDoNotCommit(t *testing.T) {
	f := newFixture(t)
	_, err := f.service.Read(context.Background(), "missing")
	assertCode(t, err, domain.CloneErrNotFound)
	_, err = f.service.Edit(context.Background(), "missing", EditRequest{Op: "remove", ItemID: "b1"})
	assertCode(t, err, domain.CloneErrNotFound)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = f.service.Edit(ctx, "job-1", EditRequest{Op: "remove", ItemID: "b1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(f.store.changes) != 0 {
		t.Fatal("invalid request committed")
	}
}
