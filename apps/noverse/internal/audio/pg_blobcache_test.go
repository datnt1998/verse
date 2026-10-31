package audio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/blobstore/s3test"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgtest"
)

func audioMinIO(t *testing.T, endpoint string) *blobstore.S3 {
	t.Helper()
	if endpoint == "" {
		endpoint = os.Getenv("S3_TEST_ENDPOINT")
	}
	if endpoint == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("S3_TEST_ENDPOINT is required")
		}
		t.Skip("S3_TEST_ENDPOINT unset")
	}
	cfg := blobstore.S3Config{Endpoint: endpoint, Region: s3test.Env("S3_TEST_REGION", "garage"), Bucket: s3test.Env("S3_TEST_BUCKET", "audio-cache"), AccessKeyID: os.Getenv("S3_TEST_ACCESS_KEY"), SecretAccessKey: os.Getenv("S3_TEST_SECRET_KEY"), ForcePathStyle: true, AllowPlainHTTP: true}
	store, err := blobstore.NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Bucket = s3test.Bucket(t, store.Client(), cfg.AccessKeyID, cfg.Bucket)
	store, err = blobstore.NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cursor := ""
		for {
			page, err := store.List(ctx, "", cursor, 1000)
			if err != nil {
				return
			}
			for _, info := range page.Infos {
				_ = store.Delete(ctx, info.Key)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
	})
	return store
}

func audioCachePG(t *testing.T) (*postgres.Pool, *postgres.Narrative, postgres.ScopeRef) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.Up(context.Background(), db)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	id, err := postgres.ParseUUID("9c06da70-0f63-40ea-bf94-1ae17866c4ef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.New(pool.Pool()).EnsureProject(context.Background(), gen.EnsureProjectParams{ID: id, Slug: "cache-proj", Name: "Cache project"}); err != nil {
		t.Fatal(err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(context.Background(), "cache-proj", nil, postgres.StoryInfo{Slug: "cache-story", Title: "Truyện thử", Status: "writing"}); err != nil {
		t.Fatal(err)
	}
	scope, err := nar.Resolve(context.Background(), "cache-proj", "cache-story", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return pool, nar, scope
}

func TestPGMinIOExportFormatsReuseSourceAndPeaks(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("ffmpeg unavailable")
	}
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	pool, nar, scope := audioCachePG(t)
	store := &countingAudioStore{BlobStore: audioMinIO(t, "")}
	repo := postgres.NewAudio(pool)
	jobs := postgres.NewJobs(pool)
	deps := PGDeps{Jobs: jobs, Audio: repo, Cache: postgres.NewExportCache(pool), Nar: nar, Blobs: store, Project: "cache-proj", Story: "cache-story", Engine: "remote", Model: "remote"}
	ctx := context.Background()
	text := "Trời đã tối. Nàng nhìn ra cửa sổ."
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated, text, domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatal(err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	script := Script{Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash(text), Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: text}}}
	payload, _ := json.Marshal(script)
	if _, err := repo.SaveScript(ctx, scope, postgres.Script{ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision), SourceSHA256: script.SourceHash, Status: StatusLocked, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{SegmentID: "s0001", Ordinal: 0, Speaker: NarratorID, Text: text, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	wav := silenceWAV(200, 48000)
	sum := sha256.Sum256(wav)
	digest := hex.EncodeToString(sum[:])
	key, err := (blobkey.Keys{}).AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	blob, err := postgres.UploadAndPublishBlob(ctx, repo, store, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "s3", ContentType: "audio/wav", SHA256: digest, Size: int64(len(wav))}, bytes.NewReader(wav))
	if err != nil {
		t.Fatal(err)
	}
	synthKey := (SynthKey{Text: text, VoiceID: "", Speed: 1, Pitch: 0, Quality: "standard", Engine: "remote", Model: "remote"}).Hash()
	if _, err := repo.AddRender(ctx, scope.ProjectID, ref.ID, 1, postgres.Render{SegmentID: "s0001", SynthKey: synthKey, BlobID: blob.ID, DurationSec: .2, SampleRate: 48000, Selected: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SelectRender(ctx, scope.ProjectID, ref.ID, 1, "s0001", synthKey); err != nil {
		t.Fatal(err)
	}
	svc := NewPGService(deps)
	defer svc.Close()
	runner, err := NewPGExportRunner(ctx, deps)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"mp3", "m4b"} {
		before := store.gets.Load()
		if err := svc.StartExport(ctx, 1, "Truyện thử", []SourceChapter{{Chapter: 1, Title: "Mở đầu", Text: text}}, ExportOptions{Format: format}); err != nil {
			t.Fatal(err)
		}
		if ran, err := runner.RunOnce(ctx); err != nil || !ran {
			t.Fatalf("runner ran=%v err=%v", ran, err)
		}
		rows, err := jobs.List(ctx, scope)
		if err != nil {
			t.Fatal(err)
		}
		for _, job := range rows {
			if job.State != postgres.JobCompleted {
				t.Fatalf("export %s state=%s error=%s", format, job.State, job.Error)
			}
		}
		if format == "m4b" && store.gets.Load() != before {
			t.Fatalf("second format fetched %d source objects", store.gets.Load()-before)
		}
	}
	before := store.gets.Load()
	first, duration, err := svc.Peaks(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, duration2, err := svc.Peaks(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) || duration != duration2 {
		t.Fatal("peaks cache result changed")
	}
	if got := store.gets.Load() - before; got != 1 {
		t.Fatalf("two real peaks requests GETs=%d, want one", got)
	}
}

func TestPGExportHeartbeatLossPreventsAttachment(t *testing.T) {
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	pool, nar, scope := audioCachePG(t)
	jobs := postgres.NewJobs(pool)
	runner, err := NewPGExportRunner(context.Background(), PGDeps{Jobs: jobs, Nar: nar})
	if err != nil {
		t.Fatal(err)
	}
	job, err := jobs.Enqueue(context.Background(), scope, postgres.JobAudioExport, "fence-test", map[string]any{}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := jobs.Claim(context.Background(), postgres.JobAudioExport, runner.workerID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	lease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: scope.ProjectID, ID: job.ID}, WorkerID: runner.workerID, Fence: claimed.FencingToken}
	ticks := make(chan time.Time)
	renewed := make(chan struct{}, 1)
	ctx, stop := maintainExportLease(context.Background(), postgres.DefaultHeartbeat, func(ctx context.Context) error {
		_, err := jobs.Heartbeat(ctx, lease, postgres.DefaultLease)
		if err == nil {
			renewed <- struct{}{}
		}
		return err
	}, ticks)
	defer stop()
	select {
	case <-renewed:
	case <-ctx.Done():
		t.Fatalf("initial heartbeat failed: %v", context.Cause(ctx))
	case <-time.After(30 * time.Second):
		t.Fatal("initial heartbeat did not run")
	}
	// The real repository moves the fencing token on cancel. No mock SQL lease.
	if _, err := jobs.Cancel(context.Background(), lease.JobKey); err != nil {
		t.Fatal(err)
	}
	ticks <- time.Now()
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("repository fence loss did not cancel exporter")
	}
	if err := stop(); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("heartbeat err=%v", err)
	}
	attached := false
	_, err = jobs.CompleteWith(context.Background(), lease, map[string]any{}, 1, func(context.Context, pgx.Tx, postgres.Job) error { attached = true; return nil })
	if !errors.Is(err, postgres.ErrJobLeaseLost) || attached {
		t.Fatalf("stale export attached=%v err=%v", attached, err)
	}
}

// TestPGExportRunnerSurvivesJobControl proves a cancelled or paused claim is
// a per-job outcome: the runner abandons the fenced job without Fail/Complete
// and claims the next export — the singleton in-process runner keeps working
// where it used to exit permanently (review Bắt sửa #2).
func TestPGExportRunnerSurvivesJobControl(t *testing.T) {
	for _, control := range []string{"cancel", "pause"} {
		t.Run(control, func(t *testing.T) {
			t.Setenv("BLOB_CACHE_DIR", t.TempDir())
			pool, nar, scope := audioCachePG(t)
			jobs := postgres.NewJobs(pool)
			storeLocal, err := blobstore.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			store := &countingAudioStore{BlobStore: storeLocal}
			repo := postgres.NewAudio(pool)
			cacheRepo := postgres.NewExportCache(pool)
			ctx := context.Background()

			deps := PGDeps{Jobs: jobs, Audio: repo, Cache: cacheRepo, Nar: nar, Blobs: store}
			runner, err := NewPGExportRunner(ctx, deps)
			if err != nil {
				t.Fatal(err)
			}

			// Job A blocks inside its heartbeat so the control operation lands
			// deterministically mid-job instead of racing a 30-second ticker.
			beats := make(chan struct{}, 4)
			release := make(chan struct{})
			orig := exportHeartbeat
			exportHeartbeat = func(hbCtx context.Context, d PGDeps, l postgres.WorkerLease) error {
				beats <- struct{}{}
				select {
				case <-release:
					return orig(hbCtx, d, l)
				case <-hbCtx.Done():
					return hbCtx.Err()
				}
			}
			defer func() { exportHeartbeat = orig }()
			origInterval := exportHeartbeatInterval
			exportHeartbeatInterval = time.Second
			defer func() { exportHeartbeatInterval = origInterval }()

			jobA, err := jobs.Enqueue(ctx, scope, postgres.JobAudioExport, "controlled-job", map[string]any{}, nil, pgtype.UUID{})
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				ran, err := runner.RunOnce(ctx)
				if !ran {
					done <- fmt.Errorf("job A was not claimed")
					return
				}
				done <- err
			}()
			select {
			case <-beats:
			case <-time.After(30 * time.Second):
				t.Fatal("job A heartbeat never reached the barrier")
			}
			keyA := postgres.JobKey{ProjectID: scope.ProjectID, ID: jobA.ID}
			switch control {
			case "cancel":
				if _, err := jobs.Cancel(ctx, keyA); err != nil {
					t.Fatal(err)
				}
			case "pause":
				if _, err := jobs.Pause(ctx, keyA); err != nil {
					t.Fatal(err)
				}
			}
			close(release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("lease loss on job A returned runner error — runner would exit permanently: %v", err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("job A heartbeat was never released")
			}

			// Job B completes via the export_cache hit path — claim + finish
			// with no restart, materialization, or ffmpeg.
			data := []byte("chapter audio")
			sum := sha256.Sum256(data)
			digest := hex.EncodeToString(sum[:])
			key := testSegmentKey(t, digest)
			blob, err := postgres.UploadAndPublishBlob(ctx, repo, store, postgres.PublishInput{
				ProjectID: scope.ProjectID, Key: key, Driver: "local",
				ContentType: "audio/wav", SHA256: digest, Size: int64(len(data)),
			}, bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			manifest, _ := json.Marshal(pgExportManifest{ContentKey: "warm-key", Volume: 1})
			if _, err := cacheRepo.Put(ctx, scope, postgres.ExportCacheEntry{
				CacheKey: "warm-key", Kind: "mp3", BlobID: blob.ID, InputManifest: manifest,
			}, nil); err != nil {
				t.Fatal(err)
			}
			jobB, err := jobs.Enqueue(ctx, scope, postgres.JobAudioExport, "warm-export", map[string]any{
				"volume": 1, "options": map[string]any{"format": "mp3"}, "cacheKey": "warm-key", "storySlug": "cache-story",
			}, nil, pgtype.UUID{})
			if err != nil {
				t.Fatal(err)
			}
			ran, err := runner.RunOnce(ctx)
			if err != nil || !ran {
				t.Fatalf("job B after %s: ran=%v err=%v", control, ran, err)
			}
			got, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: jobB.ID})
			if err != nil {
				t.Fatal(err)
			}
			if got.State != postgres.JobCompleted {
				t.Fatalf("job B state = %q, want completed — runner did not keep working", got.State)
			}
			gotA, err := jobs.Get(ctx, keyA)
			if err != nil {
				t.Fatal(err)
			}
			// A cancelled job is terminal; a paused claim's next heartbeat is
			// fenced out and the job becomes claimable again by design, so it
			// can be re-claimed before we read its state.
			wantStates := map[postgres.JobState]bool{postgres.JobCancelled: true}
			if control == "pause" {
				wantStates = map[postgres.JobState]bool{postgres.JobPaused: true, postgres.JobQueued: true, postgres.JobRunning: true}
			}
			if !wantStates[gotA.State] {
				t.Fatalf("job A state = %q — stale claim must be abandoned untouched", gotA.State)
			}
		})
	}
}

// Explicit benchmark gate: real MinIO behind the audit's 30ms request + 30ms
// response-header proxy. It measures blob-to-scratch, not whole ffmpeg export.
func TestBlobCacheMaterializationLatency(t *testing.T) {
	if os.Getenv("S3_MATERIALIZE_BENCH") != "1" {
		t.Skip("S3_MATERIALIZE_BENCH unset")
	}
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	target, err := url.Parse(endpoint)
	if err != nil || target.Host == "" {
		t.Fatal("S3_TEST_ENDPOINT is required")
	}
	var gets, heads atomic.Int32
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		if req.Method == "GET" {
			gets.Add(1)
		}
		if req.Method == "HEAD" {
			heads.Add(1)
		}
		time.Sleep(30 * time.Millisecond)
		original(req)
	}
	proxy.ModifyResponse = func(*http.Response) error { time.Sleep(30 * time.Millisecond); return nil }
	listener, err := net.Listen("tcp", "127.0.0.1:63002")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(proxy)
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	store := audioMinIO(t, server.URL)
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	svc := NewPGService(PGDeps{Blobs: store})
	defer svc.Close()
	var items []blobMaterialization
	data := bytes.Repeat([]byte{37}, 5<<20)
	for i := range 30 {
		data[0] = byte(i)
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		key := testSegmentKey(t, digest)
		if _, err := store.Put(context.Background(), key, bytes.NewReader(data), blobstore.PutOptions{ExpectedSHA256: digest}); err != nil {
			t.Fatal(err)
		}
		items = append(items, blobMaterialization{postgres.Blob{ObjectKey: key, SHA256: digest, SizeBytes: int64(len(data)), StorageDriver: "s3", State: "ready"}, ""})
	}
	for _, phase := range []string{"serial-before", "cache-cold", "cache-warm"} {
		root := t.TempDir()
		for i := range items {
			items[i].dest = filepath.Join(root, fmt.Sprintf("%d.wav", i))
		}
		gets.Store(0)
		heads.Store(0)
		start := time.Now()
		if phase == "serial-before" {
			for _, item := range items {
				body, _, err := store.Open(context.Background(), item.blob.ObjectKey, blobstore.FullRange)
				if err != nil {
					t.Fatal(err)
				}
				out, err := os.Create(item.dest)
				if err != nil {
					body.Close()
					t.Fatal(err)
				}
				hash := sha256.New()
				_, copyErr := io.Copy(io.MultiWriter(out, hash), body)
				closeErr := out.Close()
				body.Close()
				if copyErr != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != item.blob.SHA256 {
					t.Fatalf("baseline copy=%v close=%v", copyErr, closeErr)
				}
			}
		} else if err := svc.materializeBlobs(context.Background(), items); err != nil {
			t.Fatal(err)
		}
		t.Logf("30 x 5MiB %s: duration=%s GET=%d HEAD=%d", phase, time.Since(start), gets.Load(), heads.Load())
		if phase == "cache-warm" && (gets.Load() != 0 || heads.Load() != 0) {
			t.Fatal("warm materialization used object storage")
		}
	}
}

func TestPGMinIOOnlySelectedAssetsAreFetched(t *testing.T) {
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	pool, _, scope := audioCachePG(t)
	store := &countingAudioStore{BlobStore: audioMinIO(t, "")}
	repo := postgres.NewAudio(pool)
	ctx := context.Background()
	for _, name := range []string{"bia/old.png", "bia/latest.png", "nhac/intro.wav", "nhac/middle.wav", "nhac/outro.wav"} {
		data := []byte(name)
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		key, err := (blobkey.Keys{}).Asset(postgres.UUIDString(scope.ProjectID), digest, strings.TrimPrefix(filepath.Ext(name), "."))
		if err != nil {
			t.Fatal(err)
		}
		blob, err := postgres.UploadAndPublishBlob(ctx, repo, store, postgres.PublishInput{
			ProjectID: scope.ProjectID, Key: key, Driver: "s3", ContentType: "application/octet-stream", SHA256: digest, Size: int64(len(data)),
		}, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.PutAssetDoc(ctx, scope.ProjectID, name, map[string]any{}, blob.ID); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewPGService(PGDeps{Audio: repo, Blobs: store})
	defer svc.Close()
	noMusic := Layout{AssetsDir: t.TempDir()}
	if err := svc.materializeAssets(ctx, scope, noMusic, ExportOptions{}); err != nil {
		t.Fatal(err)
	}
	if gets := store.gets.Load(); gets != 1 {
		t.Fatalf("no-music export fetched %d assets; want only the latest cover", gets)
	}
	if _, err := os.Stat(filepath.Join(noMusic.CoverDir(), "latest.png")); err != nil {
		t.Fatal("latest cover not materialized", err)
	}
	withMusic := Layout{AssetsDir: t.TempDir()}
	if err := svc.materializeAssets(ctx, scope, withMusic, ExportOptions{Intro: true}); err != nil {
		t.Fatal(err)
	}
	if gets := store.gets.Load(); gets != 3 {
		t.Fatalf("music export fetched %d total assets; want cached cover + intro/outro only", gets)
	}
	if _, err := os.Stat(filepath.Join(withMusic.MusicDir(), "middle.wav")); !os.IsNotExist(err) {
		t.Fatalf("unused music was materialized: %v", err)
	}
}
