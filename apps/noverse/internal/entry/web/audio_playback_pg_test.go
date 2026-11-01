//go:build !integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/jackc/pgx/v5/pgtype"
)

// Real Postgres, worker upload/ACK HTTP, durable queue, filesystem blob store,
// real ffmpeg and public chapter GET. No synthesizer or echo transport stands
// in for assembly: the WAV fixtures are decoded from the resulting MP3.
func TestPGNativeCompletionPublishesChapterPlayback(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required")
	}
	t.Setenv("BLOB_CACHE_DIR", t.TempDir())
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "play-proj", "play-story")
	text := "Trời đã tối. Nàng nhìn ra cửa sổ."
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated, text,
		domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatal(err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	repo, jobs, cache := postgres.NewAudio(pool), postgres.NewJobs(pool), postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	script := audio.Script{Volume: 1, Chapter: 1, Status: audio.StatusLocked, SourceHash: audio.SourceHash(text), Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối.", PauseMs: 100},
		{ID: "s0002", Speaker: audio.NarratorID, Text: "Nàng nhìn ra cửa sổ."},
	}}
	payload, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveScript(ctx, scope, postgres.Script{ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision), SourceSHA256: script.SourceHash, Status: script.Status, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	for i, seg := range script.Segments {
		if _, err := repo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{SegmentID: seg.ID, Ordinal: int32(i), Speaker: seg.Speaker, Text: seg.Text, Payload: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	deps := audio.PGDeps{Jobs: jobs, Audio: repo, Cache: cache, Nar: nar, Blobs: blobs, Project: "play-proj", Story: "play-story"}
	svc := audio.NewPGService(deps)
	defer svc.Close()
	token := testWorkerToken
	hash := sha256.Sum256([]byte(token))
	worker, err := jobs.RegisterWorker(ctx, "native-fixture", hash[:], []string{"tts"})
	if err != nil {
		t.Fatal(err)
	}
	workerQueue := &PGWorkerQueue{Jobs: jobs, Audio: repo}
	api := &WorkerAPI{Queue: workerQueue, Blobs: blobs, Registry: &PGAudioBlobs{Audio: repo}, BlobDriver: "local"}
	server := httptest.NewServer(api.Handler())
	defer server.Close()
	callWorker := func(path string, body []byte, headers map[string]string, want ...int) []byte {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		valid := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated
		if len(want) > 0 {
			valid = resp.StatusCode == want[0]
		}
		if !valid {
			t.Fatalf("worker %s: %d %s", path, resp.StatusCode, out)
		}
		return out
	}
	complete := func(seconds float64) postgres.Job {
		t.Helper()
		var claim struct {
			Job WorkerJob `json:"job"`
		}
		if err := json.Unmarshal(callWorker("/internal/worker/claim", []byte(`{}`), nil), &claim); err != nil {
			t.Fatal(err)
		}
		var snapshot struct {
			Segments []struct {
				ID       string `json:"id"`
				SynthKey string `json:"synthKey"`
			} `json:"segments"`
		}
		if err := json.Unmarshal(claim.Job.Input, &snapshot); err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Segments) != 2 {
			t.Fatalf("native snapshot segments=%d", len(snapshot.Segments))
		}
		var attachments []WorkerAttachment
		for i, seg := range snapshot.Segments {
			wav := testWAV(t, 8000, seconds+float64(i)*0.1)
			sum := sha256.Sum256(wav)
			sha := hex.EncodeToString(sum[:])
			var uploaded struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(callWorker("/internal/worker/jobs/"+claim.Job.ID+"/blob", wav, map[string]string{
				uploadFenceHeader: strconv.FormatInt(claim.Job.Fence, 10), uploadSHAHeader: sha, "Content-Type": "audio/wav",
			}), &uploaded); err != nil {
				t.Fatal(err)
			}
			attachments = append(attachments, WorkerAttachment{SegmentID: seg.ID, SynthKey: seg.SynthKey, Key: uploaded.Key, SHA256: sha, DurationSec: seconds + float64(i)*0.1, SampleRate: 8000, Select: true})
		}
		ack, err := json.Marshal(map[string]any{"fence": claim.Job.Fence, "progressDone": 2, "result": map[string]any{"segments": 2}, "attachments": attachments})
		if err != nil {
			t.Fatal(err)
		}
		callWorker("/internal/worker/jobs/"+claim.Job.ID+"/complete", ack, nil)
		// A duplicate ACK is fenced out, never a second assembly dispatch.
		callWorker("/internal/worker/jobs/"+claim.Job.ID+"/complete", ack, nil, http.StatusConflict)
		id, err := postgres.ParseUUID(claim.Job.ID)
		if err != nil {
			t.Fatal(err)
		}
		job, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: id})
		if err != nil || job.State != postgres.JobCompleted {
			t.Fatalf("native completion: %s %v", job.State, err)
		}
		return job
	}
	tg := newTestGateway(t)
	story := library.Story{Project: "play-proj", Story: "play-story", ID: "play-proj/play-story"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, story.Project, story.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: repo, Cache: cache, Nar: nar, Blobs: blobs})
	get := func() *httptest.ResponseRecorder {
		t.Helper()
		path := "/audio/volumes/1/chapters/1.mp3"
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, httptest.NewRequest("GET", "http://x"+path, nil), identity.User{Username: "admin", Role: identity.RoleAdmin}, story, path) {
			t.Fatal("chapter route not handled")
		}
		return w
	}
	assertPlayable := func(minDuration float64) string {
		t.Helper()
		response := get()
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "audio/mpeg" {
			t.Fatalf("chapter GET: %d %s", response.Code, response.Body.String())
		}
		dir := t.TempDir()
		mp3, wav := filepath.Join(dir, "chapter.mp3"), filepath.Join(dir, "decoded.wav")
		if err := os.WriteFile(mp3, response.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("ffmpeg", "-v", "error", "-i", mp3, "-acodec", "pcm_s16le", wav).CombinedOutput(); err != nil {
			t.Fatalf("decode actual MP3: %v %s", err, out)
		}
		decoded, err := os.ReadFile(wav)
		if err != nil {
			t.Fatal(err)
		}
		if duration := audio.WAVDurationSec(decoded); duration < minDuration {
			t.Fatalf("assembled duration %.3f < %.3f; missing segment/pause", duration, minDuration)
		}
		entry, err := cache.Get(ctx, scope, audio.ChapterAudioCacheKey(1, 1), "mp3")
		if err != nil {
			t.Fatal(err)
		}
		return postgres.UUIDString(entry.BlobID)
	}
	run := func() {
		t.Helper()
		// Recreate the runner on each claim: no process-local callback state.
		runner, err := audio.NewPGExportRunner(ctx, deps)
		if err != nil {
			t.Fatal(err)
		}
		if ran, err := runner.RunOnce(ctx); err != nil || !ran {
			t.Fatalf("durable runner: ran=%v err=%v", ran, err)
		}
	}
	if _, err := svc.Enqueue(ctx, 1, 1, script.SourceHash); err != nil {
		t.Fatal(err)
	}
	initialRender := complete(0.2)
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("GET must not build: %d", response.Code)
	}
	run()
	first := assertPlayable(0.59) // .2 + .3 + .1 pause
	firstTake, err := repo.SelectedRender(ctx, scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := jobs.List(ctx, scope)
	if err != nil || len(rows) != 2 {
		t.Fatalf("ACK replay must yield exactly TTS + assembly: %d %v", len(rows), err)
	}

	source := []audio.SourceChapter{{Chapter: 1, Text: text, Completed: true}}
	redo := func() {
		t.Helper()
		if _, err := svc.QueueChapters(ctx, source, 1, audio.QueueInput{Redo: []int{1}}); err != nil {
			t.Fatal(err)
		}
	}
	// Old assembly is queued, then another selected take supersedes it.
	redo()
	complete(0.4)
	redo()
	complete(0.7)
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("stale cache served: %d", response.Code)
	}
	run() // obsolete assembly must not publish
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("superseded assembly published: %d", response.Code)
	}
	run()
	if next := assertPlayable(1.59); next == first {
		t.Fatal("reread reused stale bytes")
	}

	// Accepting a different candidate also dispatches a durable rebuild, with
	// no native rerender or manual export as a hidden prerequisite.
	currentTake, err := repo.SelectedRender(ctx, scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil {
		t.Fatal(err)
	}
	candidateKey := (audio.SynthKey{Text: "alternate take", Engine: "remote", Model: "remote"}).Hash()
	if _, err := repo.AddRender(ctx, scope.ProjectID, ref.ID, 1, postgres.Render{
		SegmentID: "s0001", SynthKey: candidateKey, BlobID: firstTake.BlobID,
		DurationSec: firstTake.DurationSec, SampleRate: firstTake.SampleRate,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptCandidate(ctx, 1, 1, "s0001", "1-s0001-"+candidateKey); err != nil {
		t.Fatal(err)
	}
	run()
	assertPlayable(1.09)
	if err := svc.AcceptCandidate(ctx, 1, 1, "s0001", "1-s0001-"+currentTake.SynthKey); err != nil {
		t.Fatal(err)
	}
	run()
	assertPlayable(1.59)
	// B -> A -> B revisits identical audible content, but requires a new
	// durable dispatch after B's earlier assembly has already completed.
	if err := svc.AcceptCandidate(ctx, 1, 1, "s0001", "1-s0001-"+candidateKey); err != nil {
		t.Fatal(err)
	}
	run()
	assertPlayable(1.09)
	// Restore A for the subsequent legacy-recovery/M4B assertions.
	if err := svc.AcceptCandidate(ctx, 1, 1, "s0001", "1-s0001-"+currentTake.SynthKey); err != nil {
		t.Fatal(err)
	}
	run()
	assertPlayable(1.59)

	// A pre-upgrade completed row has no automatic child yet. Recovery must
	// discover it without another render or an export request.
	var legacyInput map[string]json.RawMessage
	if err := json.Unmarshal(initialRender.Input, &legacyInput); err != nil {
		t.Fatal(err)
	}
	delete(legacyInput, "ownership")
	legacy, err := jobs.Enqueue(ctx, scope, postgres.JobTTS, "pre-upgrade", legacyInput, initialRender.ProgressTotal, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	for _, omitted := range []string{"segment", "voice", "tone", "pitch", "speaker", "sourceRevision", "nullTone"} {
		var input map[string]json.RawMessage
		if err := json.Unmarshal(legacy.Input, &input); err != nil {
			t.Fatal(err)
		}
		var segments []map[string]json.RawMessage
		if err := json.Unmarshal(input["segments"], &segments); err != nil {
			t.Fatal(err)
		}
		switch omitted {
		case "segment":
			segments = segments[:1]
		case "sourceRevision":
			delete(input, "sourceRevision")
		case "nullTone":
			segments[0]["tone"] = json.RawMessage(`null`)
		default:
			delete(segments[0], omitted)
		}
		input["segments"], err = json.Marshal(segments)
		if err != nil {
			t.Fatal(err)
		}
		incomplete := legacy
		incomplete.Input, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		if current, err := (audio.PlaybackProof{}).LegacyCurrent(ctx, pool.Pool(), incomplete); err != nil || current {
			t.Fatalf("legacy snapshot missing %s accepted: current=%v err=%v", omitted, current, err)
		}
	}
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, scope.ProjectID)
	if err != nil || claimed.ID != legacy.ID {
		t.Fatalf("legacy claim: %v", err)
	}
	if _, err := jobs.Complete(ctx, postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: scope.ProjectID, ID: legacy.ID}, WorkerID: worker.ID, Fence: claimed.FencingToken}, map[string]any{}, 0, nil); err != nil {
		t.Fatal(err)
	}
	run()
	assertPlayable(1.59)

	// The existing volume M4B path still runs after automatic chapter MP3.
	if err := svc.StartExport(ctx, 1, "Playback proof", source, audio.ExportOptions{Format: "m4b"}); err != nil {
		t.Fatal(err)
	}
	run()
	info, err := svc.ExportInfo(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range info.Files {
		if filepath.Ext(file.Name) == ".m4b" && file.SizeBytes > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("M4B not produced: %+v", info)
	}
	assertPlayable(1.59)

	// A cast/settings revision changed after enqueue: the late completion must
	// not replace the selected render or publish playback.
	before, err := repo.SelectedRender(ctx, scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil {
		t.Fatal(err)
	}
	redo()
	settings := svc.Settings()
	settings.Quality = "high"
	if _, err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	complete(1.1)
	after, err := repo.SelectedRender(ctx, scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil || before.BlobID != after.BlobID {
		t.Fatalf("stale completion changed selection: %v", err)
	}
	run() // recovery records an obsolete dispatch; it cannot publish
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("stale ownership playable: %d", response.Code)
	}

	redo()
	if _, err := svc.SaveCast(&audio.Cast{Mode: audio.ModeSingle, Single: audio.SingleCast{DialoguePauseMs: 300}}); err != nil {
		t.Fatal(err)
	}
	complete(0.5)
	after, err = repo.SelectedRender(ctx, scope.ProjectID, ref.ID, 1, "s0001")
	if err != nil || before.BlobID != after.BlobID {
		t.Fatalf("stale cast completion changed selection: %v", err)
	}
	run()
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("stale cast playable: %d", response.Code)
	}

	// Explicit cancellation is terminal; reconciliation must not recreate
	// a cancelled assembly job or publish its bytes.
	redo()
	cancelledRender := complete(0.9)
	pending, err := jobs.List(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	cancelled := false
	for _, job := range pending {
		if job.IdempotencyKey == "chapter-playback-"+postgres.UUIDString(cancelledRender.ID) {
			if _, err := jobs.Cancel(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: job.ID}); err != nil {
				t.Fatal(err)
			}
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("missing durable assembly to cancel")
	}
	runner, err := audio.NewPGExportRunner(ctx, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ran, err := runner.RunOnce(ctx); err != nil || ran {
		t.Fatalf("cancelled assembly recreated: ran=%v err=%v", ran, err)
	}
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("cancelled assembly playable: %d", response.Code)
	}

	// A newer accepted chapter revision also supersedes an already queued
	// assembly, even though the native job itself completed successfully.
	redo()
	complete(0.9)
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated, text+" Bản mới.",
		domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatal(err)
	}
	run()
	if response := get(); response.Code != http.StatusNotFound {
		t.Fatalf("superseded chapter revision playable: %d", response.Code)
	}
}
