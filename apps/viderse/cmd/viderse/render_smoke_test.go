package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	stepbackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/build"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/datnt1998/verse/platform/toolchain"
)

// TestRenderPhase05SmokeFixture is an explicitly requested, provider-free input
// fixture, not a substitute for the browser/export acceptance test. The source,
// images, extraction, stereo-reference loudness and waveform are real FFmpeg
// outputs. Only the authored transcript/scene/image inputs replace provider work.
// All clone identities, artifacts and review state go through the real runner.
//
// Run from apps/viderse, without printing the sourced credentials:
//
//	set -a; . /tmp/vs3.env; set +a
//	VIDERSE_RENDER_SMOKE=1 GOWORK=off go test -p 1 -count=1 -run '^TestRenderPhase05SmokeFixture$' -timeout 0 -v ./cmd/viderse
//
// Once fixture.json appears, source /tmp/p06-editor-smoke.env in the serving
// process. The consumer must stop its server before creating the release marker
// /tmp/p06-fixture.release. Cleanup drops the isolated schema and files only
// after that explicit release or SIGTERM; browser consumers own their lifetime.
func TestRenderPhase05SmokeFixture(t *testing.T) {
	if os.Getenv("VIDERSE_RENDER_SMOKE") != "1" {
		t.Skip("set VIDERSE_RENDER_SMOKE=1 to provision the real editor/export fixture")
	}
	fixtureCtx, stopFixture := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopFixture()
	// Use the same capability-checked selection as serve: PATH can contain a
	// valid H.264 encoder that lacks WebP, while the managed binary is ready.
	originalConfig, err := bootstrap.DefaultConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	goResult, err := (&tools.ExecRunner{}).Run(context.Background(), "go", []string{"env", "-json", "GOMODCACHE", "GOCACHE"}, tools.Spec{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal("resolve the consumer's existing Go caches before isolating HOME")
	}
	var goEnv struct {
		ModuleCache string `json:"GOMODCACHE"`
		BuildCache  string `json:"GOCACHE"`
	}
	if err := json.Unmarshal([]byte(goResult.Stdout), &goEnv); err != nil || !filepath.IsAbs(goEnv.ModuleCache) || !filepath.IsAbs(goEnv.BuildCache) {
		t.Fatal("Go did not return absolute module and build cache paths")
	}
	originalTools := tools.New(tools.Deps{Runner: &tools.ExecRunner{}, BaseDir: originalConfig, Paths: &tools.FilePathStore{Dir: filepath.Join(originalConfig, "tools")}})
	resolved := originalTools.Ffmpeg.Resolve(context.Background())
	if resolved == nil {
		t.Fatal("a capability-checked FFmpeg/ffprobe with libwebp is required")
	}
	ffmpeg, ffprobe := resolved.Ffmpeg, resolved.Ffprobe
	chrome := originalTools.Chrome.Resolve() // Resolution does not launch Chrome.

	const root = "/tmp/p06-editor-smoke"
	const envFile = "/tmp/p06-editor-smoke.env"
	const releaseFile = "/tmp/p06-fixture.release"
	// Refuse stale or concurrent consumers rather than deleting someone else's
	// fixture or reusing a schema whose owner might clean it up underneath us.
	for _, path := range []string{root, envFile, releaseFile} {
		if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
			t.Fatalf("smoke path already exists or is inaccessible: %s", path)
		}
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, path := range []string{envFile, releaseFile} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Errorf("remove smoke handoff: %v", err)
			}
		}
		// Go creates read-only module directories if a consumer overrides its
		// cache settings. They are still fixture-owned and must be removable.
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return os.Chmod(path, 0700)
			}
			return nil
		}); err != nil && !os.IsNotExist(err) {
			t.Errorf("make fixture directories removable: %v", err)
		}
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove smoke fixture: %v", err)
		}
	})
	home := filepath.Join(root, "home")
	cfgDir := filepath.Join(home, ".viderse")
	projects := filepath.Join(cfgDir, "projects")
	inputs := filepath.Join(root, "inputs")
	for _, dir := range []string{projects, inputs} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home) // Bootstrap deliberately uses HOME/.viderse.

	dsn := pgtest.Provision(t)
	ctx, cancel := context.WithTimeout(fixtureCtx, 2*time.Minute)
	defer cancel()
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatal("open isolated migration connection")
	}
	_, migrateErr := pgmigrate.Up(ctx, db, migrations.FS)
	closeErr := db.Close()
	if migrateErr != nil {
		t.Fatalf("migrate isolated smoke schema: %v", migrateErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal("open isolated smoke application pool")
	}
	t.Cleanup(pool.Close)
	config := blobstore.Config{Driver: "local", Delivery: "proxy", LocalRoot: projects}
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: media.NamespaceID(config), Config: config}); err != nil {
		t.Fatal(err)
	}
	cache, err := media.NewCache(filepath.Join(cfgDir, "cache", "blobstore"), 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, cache, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.Open(d.Config) })
	runner := &tools.ExecRunner{}
	paths := &tools.FilePathStore{Dir: filepath.Join(cfgDir, "tools")}
	if err := paths.Set(tools.FfmpegSettingKey, ffmpeg); err != nil {
		t.Fatal(err)
	}
	if chrome != nil {
		if err := paths.Set(tools.ChromeSettingKey, chrome.Path); err != nil {
			t.Fatal(err)
		}
	}
	toolDeps := tools.Deps{Runner: runner, Paths: paths, BaseDir: cfgDir}

	// A real library template owns the snapshot binding (including pinned
	// standard assets); never insert a synthetic template row or forge hashes.
	lib := library.New(library.Deps{Store: postgres.NewLibraryStore(pool), Media: files})
	template, err := lib.Create(ctx, "Mẫu kiểm tra xuất video")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Patch(ctx, template.ID, library.Patch{Revision: template.Revision, Recipe: []byte(`{"scenes":{"images":{"mode":"fixed","count":2}}}`)}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := lib.SnapshotTemplate(ctx, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	channelID := editor.ID()
	if _, err := postgres.NewChannelStore(pool).CreateChannel(ctx, channels.CreateInput{ID: channelID, Name: "Kênh kiểm tra xuất video", Language: domain.LangVI, PerDay: 1, DefaultTemplateID: template.ID}); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	duration := 6.0
	source := domain.CloneSourceSnapshot{ID: "p06-smoke-source", URL: "https://example.invalid/p06-smoke-source.mp4", Title: "Kiểm tra bản dựng sáu giây", DurationSec: &duration, ScannedAt: time.Now().UTC()}
	started, err := store.StartJob(ctx, pipeline.StartJobInput{ChannelID: channelID, SourceKey: domain.CloneSourceKey(channelID, source.ID), Source: source, TemplateID: template.ID, TemplateSnapshot: renderSmokeJSON(t, snapshot), ModelRefs: json.RawMessage(`{}`), Origin: domain.OriginManual})
	if err != nil {
		t.Fatal(err)
	}
	job := started.Job
	artifacts := renderSmokeInputs(t, ctx, runner, ffmpeg, inputs, source, *snapshot)
	registry := pipeline.Registry{
		domain.CloneBackdrop: stepbackdrop.New(stepbackdrop.Deps{Store: postgres.NewBackdropStore(pool)}),
		domain.CloneBuild:    build.Handler{Deps: build.Deps{Ffmpeg: ffmpeg, FFprobe: ffprobe}},
	}
	for step, inputs := range artifacts {
		// These are deliberately authored provider-free fixture inputs, not
		// mock media: the production runner publishes every file, computes its
		// content identity and commits it together with the fenced output.
		registry[step] = pipeline.HandlerFunc(func(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
			if err := ctx.Err(); err != nil {
				return pipeline.StepResult{}, err
			}
			data := map[string]any{"fixture": "six-second-real-media"}
			if s.Step.Step == domain.CloneDownload {
				data["durationSec"], data["audioCodec"] = 6.0, "aac"
			}
			return pipeline.StepResult{Artifacts: inputs, Data: data}, nil
		})
	}
	cloneRunner := &pipeline.Runner{Store: store, Media: files, Runner: runner, Handlers: registry, StagingRoot: filepath.Join(cfgDir, "staging", "clone")}
	status, err := cloneRunner.RunJob(ctx, job.ID, "render-smoke-fixture")
	if err != nil {
		t.Fatal(err)
	}
	built, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	if status != domain.CloneWaiting || built == nil || built.Output == nil || built.Error == nil || built.Error.Code != domain.CloneErrAwaitingReview {
		steps, readErr := store.ListSteps(ctx, job.ID)
		t.Fatalf("real build did not reach awaiting_review: status=%s steps=%+v readError=%v", status, steps, readErr)
	}
	projectID, ok := built.Output.Data["projectId"].(string)
	if !ok || projectID == "" {
		t.Fatal("build output omitted projectId")
	}
	edits := newEditorService(pool, files, cfgDir, tools.New(toolDeps), toolDeps)
	project, revision, err := edits.LoadProject(ctx, projectID)
	edits.Store.Templates = lib
	if err != nil {
		t.Fatal(err)
	}
	if project["schemaVersion"] != float64(15) || project["duration"] != 6.0 || revision == "" {
		t.Fatalf("invalid persistent editor project: schema=%v duration=%v revision=%q", project["schemaVersion"], project["duration"], revision)
	}
	queueStore := postgres.NewQueueStore(pool)
	runtime := &render.Service{Store: &render.Store{Root: filepath.Join(cfgDir, "render")}, Editor: edits, Jobs: store, Media: files, ChannelName: func(ctx context.Context, id string) (string, error) {
		owner, err := queueStore.Owner(ctx, id)
		if err != nil {
			return "", err
		}
		if owner == nil {
			return "", fmt.Errorf("fixture owner channel is missing: %s", id)
		}
		return owner.Name, nil
	}}
	quality, err := runtime.Quality(ctx, projectID)
	if err != nil {
		t.Fatalf("real render.Service.Quality before fixture readiness: %v", err)
	}
	if quality["jobId"] != job.ID || quality["channelName"] != "Kênh kiểm tra xuất video" {
		t.Fatalf("quality did not resolve the real fixture job and owner: %+v", quality)
	}
	var manifest build.Manifest
	foundManifest := false
	for _, artifact := range built.Output.Artifacts {
		if !strings.HasPrefix(artifact.Path, "clone/"+job.ID+"/"+artifact.SHA256+"/") || artifact.Bytes <= 0 {
			t.Fatalf("artifact did not use production clone content identity: %s", artifact.Path)
		}
		if strings.HasSuffix(artifact.Path, "/build/manifest.json") {
			handle, err := files.Acquire(ctx, artifact.Path)
			if err != nil {
				t.Fatal(err)
			}
			manifest, err = steps.DecodeJSONFile[build.Manifest](handle.Path, "real build manifest")
			handle.Release()
			if err != nil {
				t.Fatal(err)
			}
			foundManifest = true
		}
	}
	if !foundManifest || manifest.ProjectID != projectID || manifest.Audio.MediaID == "" || manifest.Audio.ReferenceVersion != 1 || math.IsNaN(manifest.Audio.SourceLUFS) || math.IsInf(manifest.Audio.SourceLUFS, 0) || manifest.Audio.SourceLUFS >= 0 || manifest.Envelope["sourceHash"] == "" || len(manifest.Scenes) != 2 {
		t.Fatalf("missing real build/audio/envelope evidence: %+v", manifest)
	}
	// The waveform must have nonzero energy, not the all-zero test double used
	// by the unit fixture this acceptance harness is based on.
	envelope := project["audioEnvelope"]
	if envelope == nil {
		// Filled projects attach the waveform to the source audio item.
		envelope = renderSmokeFindEnvelope(project)
	}
	encoded, ok := envelope.(map[string]any)
	if !ok {
		t.Fatal("real build omitted the source waveform envelope")
	}
	bins, err := base64.StdEncoding.DecodeString(fmt.Sprint(encoded["envelopeBase64"]))
	if err != nil || len(bins) < 290 {
		t.Fatalf("invalid six-second real waveform: bins=%d error=%v", len(bins), err)
	}
	nonzero := false
	for _, bin := range bins {
		nonzero = nonzero || bin > 0
	}
	if !nonzero {
		t.Fatal("real sine-wave extraction produced a silent envelope")
	}
	renderSmokePublishGoldenAnchors(t, ctx, files, store, edits, job.ID, projectID, cfgDir)
	renderSmokeInstallChrome(t, cfgDir)

	// Create private handoff exclusively: an existing file, even a symlink,
	// must never receive database credentials. No DSN is written to test logs.
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	env := "set -a\n. /tmp/vs3.env\nset +a\n"
	for _, entry := range [][2]string{{"GOMODCACHE", goEnv.ModuleCache}, {"GOCACHE", goEnv.BuildCache}, {"HOME", home}, {"P06_SMOKE_CONFIG_DIR", cfgDir}, {"VIDERSE_DATABASE_URL", dsn}, {"VIDERSE_MIGRATION_DATABASE_URL", dsn}, {"VIDERSE_ADDR", "127.0.0.1:8823"}, {"STORAGE_DRIVER", "local"}, {"BLOB_LOCAL_ROOT", projects}, {"BLOB_DELIVERY", "proxy"}, {"BLOB_CACHE_DIR", filepath.Join(cfgDir, "cache", "blobstore")}} {
		env += "export " + entry[0] + "=" + quote(entry[1]) + "\n"
	}
	f, err := os.OpenFile(envFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.WriteString(env)
	closeErr = f.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	fixture := struct {
		ProjectID   string `json:"projectId"`
		JobID       string `json:"jobId"`
		ChannelID   string `json:"channelId"`
		TemplateID  string `json:"templateId"`
		ConfigDir   string `json:"cfgDir"`
		EnvFile     string `json:"envFile"`
		ReleaseFile string `json:"releaseFile"`
	}{projectID, job.ID, channelID, template.ID, cfgDir, envFile, releaseFile}
	if err := os.WriteFile(filepath.Join(root, "fixture.json"), renderSmokeJSON(t, fixture), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("SMOKE_READY projectId=%s jobId=%s env=%s fixture=%s release=%s", projectID, job.ID, envFile, filepath.Join(root, "fixture.json"), releaseFile)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-fixtureCtx.Done():
			return
		case <-tick.C:
			if info, err := os.Lstat(releaseFile); err == nil {
				if !info.Mode().IsRegular() {
					t.Fatal("smoke release marker must be a regular file")
				}
				return
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
}

// TestRenderSmokeInstallChrome installs the pinned, checksum-verified Chrome for
// Testing into the live fixture without borrowing a logged-in browser profile.
// It is separate so a consumer can prepare export while the fixture test holds.
func TestRenderSmokeInstallChrome(t *testing.T) {
	if os.Getenv("P06_SMOKE_INSTALL_CHROME") != "1" {
		t.Skip("set P06_SMOKE_INSTALL_CHROME=1 to install pinned Chrome for Testing")
	}
	const cfgDir = "/tmp/p06-editor-smoke/home/.viderse"
	if os.Getenv("P06_SMOKE_CONFIG_DIR") != cfgDir {
		t.Fatal("source the live smoke environment before installing Chrome")
	}
	if _, err := os.Stat("/tmp/p06-editor-smoke/fixture.json"); err != nil {
		t.Fatal("the persistent smoke fixture is not active")
	}
	renderSmokeInstallChrome(t, cfgDir)
}

// TestRenderSmokePublishGoldenAnchors upgrades a live fixture through the public
// owner-edit publication transaction, without changing its editor timeline.
func TestRenderSmokePublishGoldenAnchors(t *testing.T) {
	if os.Getenv("P06_SMOKE_PUBLISH_ANCHORS") != "1" {
		t.Skip("set P06_SMOKE_PUBLISH_ANCHORS=1 to publish independent golden timing")
	}
	raw, err := os.ReadFile("/tmp/p06-editor-smoke/fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		JobID     string `json:"jobId"`
		ProjectID string `json:"projectId"`
		ConfigDir string `json:"cfgDir"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.ConfigDir != "/tmp/p06-editor-smoke/home/.viderse" || os.Getenv("P06_SMOKE_CONFIG_DIR") != fixture.ConfigDir || editor.ValidateID(fixture.JobID) != nil || editor.ValidateID(fixture.ProjectID) != nil {
		t.Fatal("source the active smoke environment before publishing golden timing")
	}
	dsn, _ := bootstrap.DSNFromEnv()
	if !strings.HasPrefix(pgtest.SchemaOf(dsn), "pgtest_testrenderphase05smokefixture_") {
		t.Fatal("golden timing can only be published to the isolated smoke schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal("open isolated smoke publication pool")
	}
	defer pool.Close()
	store := postgres.NewCloneStore(pool)
	if os.Getenv("P06_SMOKE_RECOVER_INTERRUPTED") == "1" {
		// Operator-only recovery after stopping the fixture server. Use the
		// worker's public expired-lease selection and fenced reconciliation;
		// never rewrite running statuses or bypass owner-edit admission.
		interrupted, err := store.ListInterruptedJobs(ctx, 1000)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, candidate := range interrupted {
			found = found || candidate.ID == fixture.JobID
		}
		if !found {
			t.Fatal("fixture is not an interrupted job with an expired lease; refusing recovery")
		}
		recovery := &pipeline.Runner{Store: store}
		if err := recovery.ReconcileInterrupted(ctx, fixture.JobID); err != nil {
			t.Fatal(err)
		}
	}
	job, err := store.GetJob(ctx, fixture.JobID)
	if err != nil {
		t.Fatal(err)
	}
	records, err := store.ListSteps(ctx, fixture.JobID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SMOKE_JOB_STATE status=%s fence=%d paused=%t cancelRequested=%t", job.Status, job.Fence, job.PausedAt != nil, job.CancelRequestedAt != nil)
	for _, record := range records {
		t.Logf("SMOKE_STEP_STATE step=%s status=%s fence=%d attempt=%d", record.Step, record.Status, record.Fence, record.Attempt)
	}
	// Local artifact acquisition does not use a working cache. The persisted
	// descriptor remains authoritative; this helper never initializes a store.
	files := media.New(postgres.NewBlobRepository(pool), nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.Open(d.Config) })
	renderSmokePublishGoldenAnchors(t, ctx, files, store, newEditorService(pool, files, fixture.ConfigDir, nil, tools.Deps{}), fixture.JobID, fixture.ProjectID, fixture.ConfigDir)
}

func renderSmokePublishGoldenAnchors(t *testing.T, ctx context.Context, files *media.Service, store *postgres.CloneStore, editorService *editor.Service, jobID, projectID, cfgDir string) {
	t.Helper()
	edits := &jobedit.Service{Store: store, Media: files}
	built, err := edits.Read(ctx, jobID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := edits.Acquire(ctx, built, "build/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := steps.DecodeJSONFile[build.Manifest](handle.Path, "build evidence")
	handle.Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Audio.SHA256) != 64 {
		t.Fatal("built source audio has no content digest")
	}
	before, err := edits.Read(ctx, jobID, domain.CloneTranscript)
	if err != nil {
		t.Fatal(err)
	}
	handle, err = edits.Acquire(ctx, before, "transcript/captions.json")
	if err != nil {
		t.Fatal(err)
	}
	captions, err := steps.DecodeJSONFile[transcript.CaptionManifest](handle.Path, "authored fixture captions")
	handle.Release()
	if err != nil {
		t.Fatal(err)
	}
	// Same raw-anchor schema and independent-onset mechanism as the scorer's
	// golden alignment fixture. These are AUTHOR-DEFINED reference timings,
	// not ASR output: a 440 Hz sine contains no speech. In particular, never
	// derive these timings from the mutable editor subtitle cue being measured.
	anchors := transcript.AsrAnchors{
		Version: 1, SourceAudioSHA256: manifest.Audio.SHA256,
		Spans: []transcript.Interval{{StartMs: 0, EndMs: 6000}},
		Words: []transcript.AnchorWord{
			{Text: "Xin", StartMs: 0, EndMs: 1000},
			{Text: "chào", StartMs: 1000, EndMs: 2500},
			{Text: "Cảnh", StartMs: 3000, EndMs: 4000},
			{Text: "kết", StartMs: 4000, EndMs: 5000},
			{Text: "thúc", StartMs: 5000, EndMs: 6000},
		},
		Model:       transcript.AnchorModel{Name: "authored-golden-not-asr", Version: "1", Size: "synthetic-fixture", ComputeType: "none", Device: "none"},
		GeneratedAt: time.Unix(1, 0).UTC().Format(time.RFC3339),
	}
	file := filepath.Join(cfgDir, "staging", "smoke-golden-anchors.json")
	if err := steps.WriteJSONAtomic(file, anchors); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)
	hash, err := steps.SHA256File(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := edits.Commit(ctx, before, captions, "", false, []media.Artifact{{Path: "transcript/asr-anchors.json", File: file, ContentType: "application/json", Filename: "asr-anchors.json"}}); err != nil {
		t.Fatalf("publish independent golden timing through owner-edit transaction: %v", err)
	}
	resolved, err := editorService.Anchors(ctx, projectID)
	if err != nil {
		t.Fatalf("resolve published golden timing through the real editor repository: %v", err)
	}
	resolvedBytes := renderSmokeJSON(t, resolved)
	var checked transcript.AsrAnchors
	if err := json.Unmarshal(resolvedBytes, &checked); err != nil || checked.Model.Name != "authored-golden-not-asr" || len(checked.Words) != 5 || checked.SourceAudioSHA256 != manifest.Audio.SHA256 {
		t.Fatal("editor anchor resolution did not select the independent golden timing artifact")
	}
	published, err := edits.Read(ctx, jobID, domain.CloneTranscript)
	if err != nil {
		t.Fatal(err)
	}
	handle, err = edits.Acquire(ctx, published, "transcript/asr-anchors.json")
	if err != nil {
		t.Fatal(err)
	}
	path := handle.Ref.Path
	handle.Release()
	if path != "clone/"+jobID+"/"+hash+"/transcript/asr-anchors.json" {
		t.Fatalf("golden anchor publication lost content identity: %s", path)
	}
	t.Logf("SMOKE_GOLDEN_ANCHORS_READY path=%s sha256=%s sourceAudioSha256=%s provenance=authored-golden-not-asr jobId=%s projectId=%s", path, hash, manifest.Audio.SHA256, jobID, projectID)
}

func renderSmokeInstallChrome(t *testing.T, cfgDir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	kit := tools.New(tools.Deps{BaseDir: cfgDir, Runner: &tools.ExecRunner{}})
	installer := &toolchain.Installer{BaseDir: cfgDir, DownloadTimeout: 3 * time.Minute}
	if _, err := installer.Install(ctx, kit.Chrome.InstallSpec()); err != nil {
		t.Fatalf("install real pinned Chrome for Testing: %v", err)
	}
	if kit.Chrome.ManagedPath() == "" {
		t.Fatal("installer did not activate a managed Chrome for Testing executable")
	}
	t.Logf("SMOKE_CHROME_READY path=%s", kit.Chrome.ManagedPath())
}

func renderSmokeJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

// Find the engine-owned waveform without depending on a particular track ID.
func renderSmokeFindEnvelope(value any) any {
	switch value := value.(type) {
	case editor.Project:
		return renderSmokeFindEnvelope(map[string]any(value))
	case map[string]any:
		if _, ok := value["envelopeBase64"].(string); ok {
			return value
		}
		for _, child := range value {
			if found := renderSmokeFindEnvelope(child); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range value {
			if found := renderSmokeFindEnvelope(child); found != nil {
				return found
			}
		}
	}
	return nil
}

func renderSmokeInputs(t *testing.T, ctx context.Context, runner tools.Runner, ffmpeg, dir string, source domain.CloneSourceSnapshot, snapshot library.Snapshot) map[domain.CloneStep][]media.Artifact {
	t.Helper()
	outputs := map[domain.CloneStep][]media.Artifact{}
	file := func(path string) string { return filepath.Join(dir, filepath.FromSlash(path)) }
	add := func(step domain.CloneStep, path, mime string) {
		outputs[step] = append(outputs[step], media.Artifact{Path: path, File: file(path), ContentType: mime, Filename: filepath.Base(path)})
	}
	writeJSON := func(step domain.CloneStep, path string, value any) {
		if err := steps.WriteJSONAtomic(file(path), value); err != nil {
			t.Fatal(err)
		}
		add(step, path, "application/json")
	}
	run := func(args ...string) {
		result, err := runner.Run(ctx, ffmpeg, append([]string{"-hide_banner", "-nostdin", "-y", "-threads", "1", "-filter_threads", "1"}, args...), tools.Spec{Timeout: time.Minute, LogLimit: 16384})
		if err != nil {
			t.Fatalf("generate real fixture media: %v\n%s", err, result.Stderr)
		}
	}
	for _, sub := range []string{"download", "scenes", "images"} {
		if err := os.MkdirAll(file(sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	run("-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=30:duration=6", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=6", "-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "28", "-pix_fmt", "yuv420p", "-threads:v", "1", "-c:a", "aac", "-ac", "1", "-ar", "48000", "-t", "6", "-movflags", "+faststart", file("download/source.mp4"))
	add(domain.CloneDownload, "download/source.mp4", "video/mp4")
	sourceHash, err := steps.SHA256File(file("download/source.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(domain.CloneDownload, "download/source.info.json", editor.Object{"id": source.ID, "title": source.Title, "description": "Nguồn kiểm tra tổng hợp: testsrc và âm đơn 440 Hz.", "duration": 6, "audioCodec": "aac", "tags": []string{}})
	// Cue text and scene boundaries are authored inputs for a synthetic source;
	// there is intentionally no claim that a speech model transcribed a sine.
	writeJSON(domain.CloneTranscript, "transcript/captions.json", editor.Object{"version": 1, "revision": 2, "cues": []editor.Object{{"id": "c1", "startMs": 0, "endMs": 2500, "text": "Xin chào", "source": "captions", "words": []editor.Object{{"startMs": 0, "endMs": 1000, "text": "Xin"}, {"startMs": 1000, "endMs": 2500, "text": "chào"}}}, {"id": "c2", "startMs": 3000, "endMs": 6000, "text": "Cảnh kết thúc", "source": "captions", "words": []editor.Object{{"startMs": 3000, "endMs": 4000, "text": "Cảnh"}, {"startMs": 4000, "endMs": 5000, "text": "kết"}, {"startMs": 5000, "endMs": 6000, "text": "thúc"}}}}, "speechIntervals": []any{}, "unresolvedGaps": []any{}, "needsReview": []any{}, "warnings": []any{}})
	writeJSON(domain.CloneMetadata, "metadata/metadata.json", metadata.Output{Version: 1, Title: source.Title, Description: "Bản dựng kiểm tra thực tế dài sáu giây, không gọi nhà cung cấp AI.", Tags: []string{"kiểm tra"}})
	recipe, err := scenes.ResolveRecipe(renderSmokeJSON(t, snapshot), false, 6)
	if err != nil {
		t.Fatal(err)
	}
	sceneDoc := scenes.Manifest{Version: 1, Revision: 3, FPS: 30, TotalFrames: 180, DurationSec: 6, Source: scenes.FileRef{Path: "download/source.mp4", SHA256: sourceHash}, Recipe: recipe, Scenes: []scenes.Scene{{SceneID: "sc-f0", StartFrame: 0, EndFrame: 90, KeyframeFrame: 45, KeyframePTS: 1.5, Text: "Xin chào", CaptionCueIDs: []string{"c1"}}, {SceneID: "sc-f90", StartFrame: 90, EndFrame: 180, KeyframeFrame: 135, KeyframePTS: 4.5, Text: "Cảnh kết thúc", CaptionCueIDs: []string{"c2"}}}}
	for n, timestamp := range []string{"1.5", "4.5"} {
		path := fmt.Sprintf("scenes/keyframe-%d.webp", n)
		run("-ss", timestamp, "-i", file("download/source.mp4"), "-frames:v", "1", "-c:v", "libwebp", file(path))
		add(domain.CloneScenes, path, "image/webp")
		hash, err := steps.SHA256File(file(path))
		if err != nil {
			t.Fatal(err)
		}
		sceneDoc.Scenes[n].Keyframe = scenes.FileRef{Path: path, SHA256: hash}
	}
	writeJSON(domain.CloneScenes, "scenes/manifest.json", sceneDoc)
	review := images.CreateManifest(sceneDoc, sceneDoc.Scenes[0].Keyframe, source.Title)
	for n := range review.Items {
		item := &review.Items[n]
		path, color := fmt.Sprintf("images/scene-%d.webp", n), "blue"
		if n == 1 {
			color = "green"
		}
		if item.Kind == "thumbnail" {
			path = "images/thumbnail.webp"
		}
		run("-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", color, item.Width, item.Height), "-frames:v", "1", "-c:v", "libwebp", file(path))
		add(domain.CloneImages, path, "image/webp")
		hash, err := steps.SHA256File(file(path))
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(file(path))
		if err != nil {
			t.Fatal(err)
		}
		one := 1
		item.Status, item.ReviewVersion = "review", &one
		item.Versions = []images.Version{{Version: 1, GenerationID: "fixture-" + item.ID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Path: path, SHA256: hash, RefHash: item.Reference.SHA256, Width: item.Width, Height: item.Height, Bytes: info.Size(), Mime: "image/webp", Source: "upload", SourceFingerprint: images.SourceFingerprint(*item)}}
	}
	// Explicit owner approval through the domain transition makes this fixture
	// usable for an actual export, while build itself still parks for review.
	for _, item := range review.Items {
		review, err = images.Review(review, item.ID, "approve", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	writeJSON(domain.CloneImages, "images/manifest.json", review)
	return outputs
}
