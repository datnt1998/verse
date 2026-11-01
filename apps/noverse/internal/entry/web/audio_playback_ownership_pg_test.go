//go:build !integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type playbackOwnershipFixture struct {
	db     gen.DBTX
	ctx    context.Context
	t      *testing.T
	nar    *postgres.Narrative
	scope  postgres.ScopeRef
	repo   *postgres.Audio
	jobs   *postgres.Jobs
	store  blobstore.BlobStore
	svc    *audio.PGService
	deps   audio.PGDeps
	worker postgres.Worker
	cast   *audio.Cast
	source []audio.SourceChapter
}

func newPlaybackOwnershipFixture(t *testing.T, twoSegments ...bool) *playbackOwnershipFixture {
	t.Helper()
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "proof-proj", "proof-story")
	repo, jobs := postgres.NewAudio(pool), postgres.NewJobs(pool)
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deps := audio.PGDeps{Audio: repo, Jobs: jobs, Cache: postgres.NewExportCache(pool), Nar: nar, Blobs: store, Project: "proof-proj", Story: "proof-story"}
	svc := audio.NewPGService(deps)
	t.Cleanup(svc.Close)
	f := &playbackOwnershipFixture{ctx: ctx, db: pool.Pool(), t: t, nar: nar, scope: scope, repo: repo, jobs: jobs, store: store, svc: svc, deps: deps,
		cast: &audio.Cast{Mode: audio.ModeSingle, Single: audio.SingleCast{VoiceID: "voice-a", DialogueShift: true, KeepTones: true, DialoguePauseMs: 50}}}
	if _, err := svc.SaveCast(f.cast); err != nil {
		t.Fatal(err)
	}
	for chapter := 1; chapter <= 2; chapter++ {
		text := "Một đoạn văn để đọc."
		sourceText := text
		if len(twoSegments) > 0 && twoSegments[0] {
			sourceText += " " + text
		}
		if _, err := nar.AcceptRecord(ctx, scope, chapter, domain.ChapterOriginGenerated, sourceText, domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
			t.Fatal(err)
		}
		ref, err := nar.ChapterRefByNumber(ctx, scope, chapter)
		if err != nil {
			t.Fatal(err)
		}
		seg := audio.Segment{ID: "s0001", Speaker: "hero", Text: text, PauseMs: 100}
		script := audio.Script{Volume: 1, Chapter: chapter, Status: audio.StatusLocked, SourceHash: audio.SourceHash(sourceText), Segments: []audio.Segment{seg}}
		if len(twoSegments) > 0 && twoSegments[0] {
			second := seg
			second.ID = "s0002"
			script.Segments = append(script.Segments, second)
		}
		body, err := json.Marshal(script)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.SaveScript(ctx, scope, postgres.Script{ChapterID: ref.ID, RevisionNo: 1, SourceRevision: 1, SourceSHA256: script.SourceHash, Status: script.Status, Payload: body}); err != nil {
			t.Fatal(err)
		}
		for i, seg := range script.Segments {
			segmentBody, err := json.Marshal(seg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{SegmentID: seg.ID, Ordinal: int32(i), Speaker: seg.Speaker, Text: seg.Text, Payload: segmentBody}); err != nil {
				t.Fatal(err)
			}
		}
		f.source = append(f.source, audio.SourceChapter{Chapter: chapter, Text: sourceText, Completed: true})
	}
	hash := sha256.Sum256([]byte(testWorkerToken))
	worker, err := jobs.RegisterWorker(ctx, "ownership-native", hash[:], []string{"tts"})
	if err != nil {
		t.Fatal(err)
	}
	f.worker = worker
	return f
}

func (f *playbackOwnershipFixture) enqueue(chapter int, legacy bool) postgres.Job {
	f.t.Helper()
	if _, err := f.svc.QueueChapters(f.ctx, f.source, 1, audio.QueueInput{Redo: []int{chapter}}); err != nil {
		f.t.Fatal(err)
	}
	rows, err := f.jobs.List(f.ctx, f.scope)
	if err != nil {
		f.t.Fatal(err)
	}
	var job postgres.Job
	for _, row := range rows {
		if row.Kind == postgres.JobTTS && row.State == postgres.JobQueued {
			job = row
		}
	}
	if !job.ID.Valid {
		f.t.Fatal("missing queued TTS job")
	}
	if !legacy {
		return job
	}
	if _, err := f.jobs.Cancel(f.ctx, postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID}); err != nil {
		f.t.Fatal(err)
	}
	var frozen map[string]json.RawMessage
	if err := json.Unmarshal(job.Input, &frozen); err != nil {
		f.t.Fatal(err)
	}
	delete(frozen, "ownership")
	job, err = f.jobs.Enqueue(f.ctx, f.scope, postgres.JobTTS, "legacy-"+postgres.UUIDString(job.ID), frozen, job.ProgressTotal, pgtype.UUID{})
	if err != nil {
		f.t.Fatal(err)
	}
	return job
}

func (f *playbackOwnershipFixture) claim(job postgres.Job) postgres.Job {
	f.t.Helper()
	claimed, err := f.jobs.Claim(f.ctx, postgres.JobTTS, f.worker.ID, postgres.DefaultLease, f.scope.ProjectID)
	if err != nil || claimed.ID != job.ID {
		f.t.Fatalf("claim expected job: %v", err)
	}
	return claimed
}

func (f *playbackOwnershipFixture) complete(claimed postgres.Job, oldServer bool) {
	f.t.Helper()
	var frozen struct {
		ChapterID      string `json:"chapterId"`
		ScriptRevision int32  `json:"scriptRevision"`
		Segments       []struct {
			ID       string `json:"id"`
			SynthKey string `json:"synthKey"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(claimed.Input, &frozen); err != nil {
		f.t.Fatal(err)
	}
	if len(frozen.Segments) != 1 {
		f.t.Fatal("expected one actual frozen segment")
	}
	wav := testWAV(f.t, 8000, 0.2)
	sum := sha256.Sum256(wav)
	sha := hex.EncodeToString(sum[:])
	key, err := (blobkey.Keys{}).AudioSegment(postgres.UUIDString(f.scope.ProjectID), postgres.UUIDString(f.scope.StoryID), sha, "wav")
	if err != nil {
		f.t.Fatal(err)
	}
	blob, err := postgres.UploadAndPublishBlob(f.ctx, f.repo, f.store, postgres.PublishInput{ProjectID: f.scope.ProjectID, JobID: claimed.ID, Key: key, Driver: "local", ContentType: "audio/wav", SHA256: sha, Size: int64(len(wav))}, bytes.NewReader(wav))
	if err != nil {
		f.t.Fatal(err)
	}
	seg := frozen.Segments[0]
	if !oldServer {
		queue := PGWorkerQueue{Jobs: f.jobs, Audio: f.repo}
		if _, err := queue.Complete(f.ctx, postgres.UUIDString(f.worker.ID), postgres.UUIDString(claimed.ID), claimed.FencingToken, 1, map[string]any{}, []WorkerAttachment{{SegmentID: seg.ID, SynthKey: seg.SynthKey, Key: key, SHA256: sha, DurationSec: .2, SampleRate: 8000, Select: true}}); err != nil {
			f.t.Fatal(err)
		}
		return
	}
	// Reproduce a pre-upgrade completion transaction: selected real WAVs and
	// completed TTS existed, but the old server dispatched no chapter assembly.
	chapter, err := postgres.ParseUUID(frozen.ChapterID)
	if err != nil {
		f.t.Fatal(err)
	}
	_, err = f.jobs.CompleteWith(f.ctx, postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: claimed.ProjectID, ID: claimed.ID}, WorkerID: f.worker.ID, Fence: claimed.FencingToken}, map[string]any{}, 1, func(ctx context.Context, tx pgx.Tx, _ postgres.Job) error {
		if _, err := f.repo.AddRenderTx(ctx, tx, f.scope.ProjectID, chapter, frozen.ScriptRevision, postgres.Render{SegmentID: seg.ID, SynthKey: seg.SynthKey, BlobID: blob.ID, DurationSec: .2, SampleRate: 8000}); err != nil {
			return err
		}
		_, err := f.repo.SelectRenderTx(ctx, tx, f.scope.ProjectID, chapter, frozen.ScriptRevision, seg.ID, seg.SynthKey)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *playbackOwnershipFixture) run() {
	f.t.Helper()
	runner, err := audio.NewPGExportRunner(f.ctx, f.deps)
	if err != nil {
		f.t.Fatal(err)
	}
	if ran, err := runner.RunOnce(f.ctx); err != nil || !ran {
		f.t.Fatalf("run assembly: ran=%v err=%v", ran, err)
	}
}

func TestPGLegacyPlaybackOwnership(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required")
	}
	for _, stage := range []string{"ack", "recovery"} {
		for _, change := range []string{"unchanged", "voice", "settings"} {
			t.Run(stage+"/"+change, func(t *testing.T) {
				f := newPlaybackOwnershipFixture(t)
				job := f.claim(f.enqueue(1, true))
				if stage == "recovery" {
					f.complete(job, true)
				}
				switch change {
				case "voice":
					f.cast.Single.VoiceID = "voice-b"
					if _, err := f.svc.SaveCast(f.cast); err != nil {
						t.Fatal(err)
					}
				case "settings":
					settings := f.svc.Settings()
					settings.Quality = "high"
					if _, err := f.svc.SaveSettings(settings); err != nil {
						t.Fatal(err)
					}
				}
				if stage == "ack" {
					f.complete(job, false)
				}
				f.run()
				obj, err := f.svc.OpenChapterAudio(f.ctx, 1, 1)
				if change == "unchanged" {
					if err != nil || obj.Size == 0 {
						t.Fatalf("unchanged legacy chapter unavailable: %+v %v", obj, err)
					}
				} else {
					if !errors.Is(err, audio.ErrNotFound) {
						t.Fatalf("stale legacy chapter playable: %+v %v", obj, err)
					}
					ref, err := f.nar.ChapterRefByNumber(f.ctx, f.scope, 1)
					if err != nil {
						t.Fatal(err)
					}
					if stage == "ack" {
						if _, err := f.repo.SelectedRender(f.ctx, f.scope.ProjectID, ref.ID, 1, "s0001"); !errors.Is(err, postgres.ErrJobNotFound) {
							t.Fatalf("stale legacy ACK selected a render: %v", err)
						}
					}
				}
				var unknown map[string]json.RawMessage
				if err := json.Unmarshal(job.Input, &unknown); err != nil {
					t.Fatal(err)
				}
				delete(unknown, "segments")
				job.Input, err = json.Marshal(unknown)
				if err != nil {
					t.Fatal(err)
				}
				if current, err := (audio.PlaybackProof{}).LegacyCurrent(f.ctx, f.db, job); err != nil || current {
					t.Fatalf("unknown legacy provenance accepted: %v %v", current, err)
				}
			})
		}
	}
}

func TestPGPlaybackAudibleOwnershipStable(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required")
	}
	f := newPlaybackOwnershipFixture(t)
	f.complete(f.claim(f.enqueue(1, false)), false)
	f.run()
	ref, err := f.nar.ChapterRefByNumber(f.ctx, f.scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	proof := audio.PlaybackProof{}
	revision := func() postgres.PlaybackRevision {
		t.Helper()
		r, err := f.repo.PlaybackRevision(f.ctx, f.scope, ref.ID, proof)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	assertPlayable := func() {
		t.Helper()
		if obj, err := f.svc.OpenChapterAudio(f.ctx, 1, 1); err != nil || obj.Size == 0 {
			t.Fatalf("unrelated cast update lost playable chapter: %v", err)
		}
	}
	before := revision()
	for range 2 {
		if _, err := f.svc.SaveCast(f.cast); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.repo.UpsertVoice(f.ctx, f.scope.ProjectID, "unused", audio.ProjectVoice{ID: "unused", Name: "Unrelated voice", Note: "display only"}, pgtype.UUID{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.UpsertVoice(f.ctx, f.scope.ProjectID, "voice-a", audio.ProjectVoice{ID: "voice-a", Name: "Renamed built-in", Note: "display only"}, pgtype.UUID{}); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before != after {
		t.Fatalf("same audible inputs changed fingerprint: before=%+v after=%+v", before, after)
	}
	assertPlayable()
	// Queueing chapter two with the same voiceMode exercises the production
	// cast upsert path, including its metadata/version increments.
	if _, err := f.svc.QueueChapters(f.ctx, f.source, 1, audio.QueueInput{Chapters: []int{2}, VoiceMode: audio.ModeSingle}); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before != after {
		t.Fatalf("other chapter enqueue changed chapter one: %+v %+v", before, after)
	}
	assertPlayable()
	rows, err := f.jobs.List(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range rows {
		if job.Kind == postgres.JobTTS && job.State == postgres.JobQueued {
			if _, err := f.jobs.Cancel(f.ctx, postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID}); err != nil {
				t.Fatal(err)
			}
		}
	}
	pending := f.enqueue(1, false)
	before = revision()
	if _, err := f.svc.SaveCast(f.cast); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.QueueChapters(f.ctx, f.source, 1, audio.QueueInput{Chapters: []int{2}, VoiceMode: audio.ModeSingle}); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before != after {
		t.Fatalf("same cast invalidated pending completion: %+v %+v", before, after)
	}
	f.complete(f.claim(pending), false)
	f.run()
	assertPlayable()
	before = revision()
	f.cast.Single.DialoguePauseMs += 100
	if _, err := f.svc.SaveCast(f.cast); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before.Ownership == after.Ownership {
		t.Fatal("real audible pause change did not supersede ownership")
	}
	before = revision()
	f.cast.Single.VoiceID = "voice-b"
	if _, err := f.svc.SaveCast(f.cast); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before.Ownership == after.Ownership {
		t.Fatal("real audible voice change did not supersede ownership")
	}
	// Referenced sample identity is audible provenance; its display metadata
	// is not. Use a real registered WAV as the sample fixture.
	selected, err := f.repo.SelectedRender(f.ctx, f.scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil {
		t.Fatal(err)
	}
	before = revision()
	if _, err := f.repo.UpsertVoice(f.ctx, f.scope.ProjectID, "voice-b", audio.ProjectVoice{ID: "voice-b", Name: "Sample voice"}, selected.BlobID); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before.Ownership == after.Ownership {
		t.Fatal("referenced voice sample change did not supersede ownership")
	}
	before = revision()
	if _, err := f.repo.UpsertVoice(f.ctx, f.scope.ProjectID, "voice-b", audio.ProjectVoice{ID: "voice-b", Name: "Renamed sample", Note: "display only"}, selected.BlobID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.UpsertVoice(f.ctx, f.scope.ProjectID, "unused", audio.ProjectVoice{ID: "unused", Name: "Unrelated sample"}, selected.BlobID); err != nil {
		t.Fatal(err)
	}
	if after := revision(); before != after {
		t.Fatal("voice display/unrelated sample metadata changed audible ownership")
	}
}

func TestPGLegacyIncrementalPlaybackCoverage(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required")
	}
	for _, stage := range []string{"ack", "recovery"} {
		for _, coverage := range []string{"current", "missing", "stale"} {
			t.Run(stage+"/"+coverage, func(t *testing.T) {
				f := newPlaybackOwnershipFixture(t, true)
				get := func(id string) postgres.Job {
					t.Helper()
					uuid, err := postgres.ParseUUID(id)
					if err != nil {
						t.Fatal(err)
					}
					job, err := f.jobs.Get(f.ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: uuid})
					if err != nil {
						t.Fatal(err)
					}
					return job
				}
				cancel := func(job postgres.Job) {
					t.Helper()
					if _, err := f.jobs.Cancel(f.ctx, postgres.JobKey{ProjectID: job.ProjectID, ID: job.ID}); err != nil {
						t.Fatal(err)
					}
				}
				sourceHash := audio.SourceHash(f.source[0].Text)
				id, err := f.svc.Enqueue(f.ctx, 1, 1, sourceHash)
				if err != nil {
					t.Fatal(err)
				}
				full := get(id)
				var all struct {
					Segments []struct {
						ID       string `json:"id"`
						SynthKey string `json:"synthKey"`
					} `json:"segments"`
				}
				if err := json.Unmarshal(full.Input, &all); err != nil {
					t.Fatal(err)
				}
				if len(all.Segments) != 2 {
					t.Fatal("expected full two-segment chapter")
				}
				cancel(full)
				ref, err := f.nar.ChapterRefByNumber(f.ctx, f.scope, 1)
				if err != nil {
					t.Fatal(err)
				}
				wav := testWAV(t, 8000, .2)
				sum := sha256.Sum256(wav)
				sha := hex.EncodeToString(sum[:])
				key, err := (blobkey.Keys{}).AudioSegment(postgres.UUIDString(f.scope.ProjectID), postgres.UUIDString(f.scope.StoryID), sha, "wav")
				if err != nil {
					t.Fatal(err)
				}
				blob, err := postgres.UploadAndPublishBlob(f.ctx, f.repo, f.store, postgres.PublishInput{ProjectID: f.scope.ProjectID, Key: key, Driver: "local", ContentType: "audio/wav", SHA256: sha, Size: int64(len(wav))}, bytes.NewReader(wav))
				if err != nil {
					t.Fatal(err)
				}
				selectFirst := func(synthKey string) {
					t.Helper()
					if _, err := f.repo.AddRender(f.ctx, f.scope.ProjectID, ref.ID, 1, postgres.Render{SegmentID: "s0001", SynthKey: synthKey, BlobID: blob.ID, DurationSec: .2, SampleRate: 8000}); err != nil {
						t.Fatal(err)
					}
					if _, err := f.repo.SelectRender(f.ctx, f.scope.ProjectID, ref.ID, 1, "s0001", synthKey); err != nil {
						t.Fatal(err)
					}
				}
				selectFirst(all.Segments[0].SynthKey)
				// This is the actual ordinary producer, not Redo or a hand-built
				// partial job: the ready selected first take is intentionally omitted.
				id, err = f.svc.Enqueue(f.ctx, 1, 1, sourceHash)
				if err != nil {
					t.Fatal(err)
				}
				incremental := get(id)
				var frozen struct {
					Segments []struct {
						ID string `json:"id"`
					} `json:"segments"`
				}
				if err := json.Unmarshal(incremental.Input, &frozen); err != nil {
					t.Fatal(err)
				}
				if len(frozen.Segments) != 1 || frozen.Segments[0].ID != "s0002" ||
					incremental.ProgressTotal == nil || *incremental.ProgressTotal != 1 {
					t.Fatalf("ordinary enqueue did not freeze one needed segment: %+v", incremental)
				}
				cancel(incremental)
				var input map[string]json.RawMessage
				if err := json.Unmarshal(incremental.Input, &input); err != nil {
					t.Fatal(err)
				}
				delete(input, "ownership")
				legacy, err := f.jobs.Enqueue(f.ctx, f.scope, postgres.JobTTS, "incremental-legacy", input, incremental.ProgressTotal, pgtype.UUID{})
				if err != nil {
					t.Fatal(err)
				}
				claimed := f.claim(legacy)
				if stage == "recovery" {
					f.complete(claimed, true)
				}
				switch coverage {
				case "missing":
					if err := f.repo.DeleteRender(f.ctx, f.scope.ProjectID, ref.ID, 1, "s0001", all.Segments[0].SynthKey); err != nil {
						t.Fatal(err)
					}
				case "stale":
					selectFirst((audio.SynthKey{Text: "Old voice take", VoiceID: "voice-b", Speed: 1, Pitch: 1, Quality: "standard", Engine: "remote", Model: "remote"}).Hash())
				}
				if stage == "ack" {
					f.complete(claimed, false)
				}
				f.run()
				obj, err := f.svc.OpenChapterAudio(f.ctx, 1, 1)
				if coverage == "current" {
					if err != nil || obj.Size == 0 {
						t.Fatalf("unchanged incremental legacy unavailable: %+v %v", obj, err)
					}
				} else {
					if !errors.Is(err, audio.ErrNotFound) {
						t.Fatalf("uncovered incremental chapter playable: %+v %v", obj, err)
					}
					if stage == "ack" {
						if _, err := f.repo.SelectedRender(f.ctx, f.scope.ProjectID, ref.ID, 1, "s0002"); !errors.Is(err, postgres.ErrJobNotFound) {
							t.Fatalf("uncovered legacy ACK selected its output: %v", err)
						}
					}
				}
				unknown := claimed
				unknown.ProgressTotal = nil
				if current, err := (audio.PlaybackProof{}).LegacyCurrent(f.ctx, f.db, unknown); err != nil || current {
					t.Fatalf("unknown frozen work total accepted: %v %v", current, err)
				}
				wrongTotal := int32(2)
				unknown.ProgressTotal = &wrongTotal
				if current, err := (audio.PlaybackProof{}).LegacyCurrent(f.ctx, f.db, unknown); err != nil || current {
					t.Fatalf("inconsistent frozen work total accepted: %v %v", current, err)
				}
			})
		}
	}
}
