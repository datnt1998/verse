//go:build !integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/identity"
)

// TestPGAudioQueuePlaybackHTTP exercises the hosted audio backend over the
// real HTTP handlers and the real worker listener: enqueue writes a durable
// jobs row with the Gói AK snapshot (kind/segments/position/updateEtaSec), a
// fake worker claims/uploads/ACKs over /internal/worker/*, and playback
// streams the attached blob through the segment endpoint. Without
// SetAudioStore the same requests can only hit the file backend, which has no
// story directory here — the test fails, so a regression that drops the PG
// seam is caught.
func TestPGAudioQueuePlaybackHTTP(t *testing.T) {
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "story-proj", "novel-b")

	jobs := postgres.NewJobs(pool)
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}

	// Committed chapter so the source/lock checks and chapterId resolve.
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		"Trời đã tối. Nàng nhìn ra cửa sổ.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil || !ref.ID.Valid {
		t.Fatalf("chapter ref: %v %v", ref.ID.Valid, err)
	}
	// Volume ranges derive from the progress index; an accepted chapter makes
	// volume 1 cover chapter 1.
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	// Locked script + one segment, straight into the tables (compose is a
	// separate hosted capability; the queue path is what is under test).
	script := audio.Script{Chapter: 1, Volume: 1, Status: audio.StatusLocked,
		SourceHash: audio.SourceHash("Trời đã tối. Nàng nhìn ra cửa sổ."),
		Segments:   []audio.Segment{{ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối. Nàng nhìn ra cửa sổ."}}}

	payload, _ := json.Marshal(script)
	if _, err := audioRepo.SaveScript(ctx, scope, postgres.Script{
		ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256: script.SourceHash, Status: audio.StatusLocked, Payload: payload}); err != nil {
		t.Fatalf("save script: %v", err)
	}
	if _, err := audioRepo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{
		SegmentID: "s0001", Ordinal: 0, Speaker: audio.NarratorID,
		Text: script.Segments[0].Text, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("save segment: %v", err)
	}

	// Gateway with the hosted seam; the story has no directory anywhere.
	tg := newTestGateway(t)
	st := library.Story{Project: "story-proj", Story: "novel-b", ID: "story-proj/novel-b"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}

	call := func(method, path, query string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var r *http.Request
		if body != nil {
			buf, _ := json.Marshal(body)
			r = httptest.NewRequest(method, "http://x"+path+query, bytes.NewReader(buf))
		} else {
			r = httptest.NewRequest(method, "http://x"+path+query, nil)
		}
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, r, admin, st, path) {
			t.Fatalf("%s %s not handled", method, path)
		}
		return w
	}

	// Settings version is the real row token: before any save the row is
	// missing, afterwards it must move with each write (flag saves included).
	res := call("GET", "/audio/settings", "", nil)
	if res.Code != 200 {
		t.Fatalf("settings: %d %s", res.Code, res.Body.String())
	}
	var settingsBody struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&settingsBody); err != nil {
		t.Fatalf("settings decode: %v", err)
	}
	versionBefore := settingsBody.Version

	// Enqueue via the queue endpoint.
	res = call("POST", "/audio/queue", "?volume=1", map[string]any{"chapters": []int{1}, "quality": "standard"})
	if res.Code != 200 {
		t.Fatalf("queue: %d %s", res.Code, res.Body.String())
	}
	var queued audio.QueueResult
	if err := json.NewDecoder(res.Body).Decode(&queued); err != nil || len(queued.Chapters) != 1 {
		t.Fatalf("queue body: %v %v", queued, err)
	}

	// The durable job carries the Gói AK snapshot, verbatim for the worker.
	jobRows, err := jobs.List(ctx, scope)
	if err != nil || len(jobRows) != 1 {
		t.Fatalf("jobs: %d %v", len(jobRows), err)
	}
	job := jobRows[0]
	if job.Kind != postgres.JobTTS {
		t.Fatalf("job kind = %q", job.Kind)
	}
	var input map[string]any
	if err := json.Unmarshal(job.Input, &input); err != nil {
		t.Fatalf("job input: %v", err)
	}
	if input["kind"] != "create" || input["chapterId"] != postgres.UUIDString(ref.ID) ||
		input["scriptRevision"].(float64) != 1 || input["volume"].(float64) != 1 {
		t.Fatalf("bad input: %v", input)
	}
	if _, hasEta := input["updateEtaSec"]; hasEta {
		t.Fatalf("create job must not carry updateEtaSec: %v", input)
	}
	segs, _ := input["segments"].([]any)
	if len(segs) != 1 {
		t.Fatalf("segments: %v", segs)
	}
	seg := segs[0].(map[string]any)
	synthKey, _ := seg["synthKey"].(string)
	if synthKey == "" || seg["id"] != "s0001" || seg["text"] != script.Segments[0].Text {
		t.Fatalf("segment input: %v", seg)
	}

	// Queue snapshot shows the queued position (Gói AK).
	res = call("GET", "/audio/queue", "?volume=1", nil)
	if res.Code != 200 {
		t.Fatalf("timeline: %d %s", res.Code, res.Body.String())
	}
	var view audio.QueueView
	if err := json.NewDecoder(res.Body).Decode(&view); err != nil || len(view.Jobs) != 1 {
		t.Fatalf("timeline: %v %v", view, err)
	}
	if view.Jobs[0].State != audio.JobQueued || view.Jobs[0].Position != 1 || view.Jobs[0].Kind != "create" {
		t.Fatalf("queue view: %+v", view.Jobs[0])
	}

	// Fake worker over the real worker listener: register, claim, upload, ACK.
	workerToken := "wrt_test_token_0123456789abcdef0123456789"
	hash := sha256.Sum256([]byte(workerToken))
	if _, err := jobs.RegisterWorker(ctx, "mac-test", hash[:], []string{"tts"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	workerAPI := &WorkerAPI{
		Queue:      &PGWorkerQueue{Jobs: jobs, Audio: audioRepo},
		Blobs:      blobs,
		Registry:   &PGAudioBlobs{Audio: audioRepo},
		BlobDriver: "local",
	}
	srv := httptest.NewServer(workerAPI.Handler())
	defer srv.Close()
	workerCall := func(method, path string, body []byte, headers map[string]string) *http.Response {
		req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("worker request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+workerToken)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	resp := workerCall(http.MethodPost, "/internal/worker/claim", []byte(`{}`), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("claim = %d", resp.StatusCode)
	}
	var claimed struct {
		Job WorkerJob `json:"job"`
	}
	decodeWorkerResp(t, resp, &claimed)
	if claimed.Job.ID != postgres.UUIDString(job.ID) {
		t.Fatalf("claimed %s want %s", claimed.Job.ID, job.ID)
	}

	wav := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 256)
	sha := sha256.Sum256(wav)
	shaHex := hex.EncodeToString(sha[:])
	resp = workerCall(http.MethodPost, "/internal/worker/jobs/"+claimed.Job.ID+"/blob", wav, map[string]string{
		uploadFenceHeader: strconv.FormatInt(claimed.Job.Fence, 10),
		uploadSHAHeader:   shaHex,
		"Content-Type":    "audio/wav",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d", resp.StatusCode)
	}
	var uploaded struct {
		Key    string `json:"key"`
		SHA256 string `json:"sha256"`
	}
	decodeWorkerResp(t, resp, &uploaded)
	if uploaded.SHA256 != shaHex {
		t.Fatalf("upload hash = %q", uploaded.SHA256)
	}
	var uploadJob pgtype.UUID
	if err := pool.Pool().QueryRow(ctx, `SELECT upload_job_id FROM blobs WHERE object_key=$1`, uploaded.Key).Scan(&uploadJob); err != nil || uploadJob != job.ID {
		t.Fatalf("worker upload lost its owning job: got=%v want=%v err=%v", uploadJob, job.ID, err)
	}

	ack, _ := json.Marshal(map[string]any{
		"fence": claimed.Job.Fence, "progressDone": 1,
		"result": map[string]any{"segments": 1},
		"attachments": []map[string]any{{
			"segmentId": "s0001", "synthKey": synthKey, "key": uploaded.Key, "sha256": shaHex,
			"durationSec": 1.5, "sampleRate": 22050, "chapterId": postgres.UUIDString(ref.ID),
			"scriptRevision": 1, "select": true,
		}},
	})
	resp = workerCall(http.MethodPost, "/internal/worker/jobs/"+claimed.Job.ID+"/complete", ack, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("complete = %d: %s", resp.StatusCode, body)
	}
	if done, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: job.ID}); err != nil || done.State != postgres.JobCompleted {
		t.Fatalf("job = %q %v", done.State, err)
	}

	// Playback streams the blob through the segment endpoint.
	res = call("GET", "/audio/segments/1-s0001/audio", "?volume=1", nil)
	if res.Code != 200 || res.Body.Len() != len(wav) {
		t.Fatalf("segment audio: %d len=%d", res.Code, res.Body.Len())
	}
	if !bytes.Equal(res.Body.Bytes(), wav) {
		t.Fatal("segment audio bytes mismatch")
	}
	if res.Header().Get("X-Content-SHA256") != shaHex {
		t.Fatalf("sha header: %q", res.Header().Get("X-Content-SHA256"))
	}

	// Range requests work against the blob (seek support for the player).
	r := httptest.NewRequest(http.MethodGet, "http://x/audio/segments/1-s0001/audio?volume=1", nil)
	r.Header.Set("Range", "bytes=4-11")
	w := httptest.NewRecorder()
	tg.g.serveStoryAudio(w, r, admin, st, "/audio/segments/1-s0001/audio")
	if w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), wav[4:12]) {
		t.Fatalf("range = %d %q", w.Code, w.Body.Bytes())
	}

	// Timeline now reports the chapter done.
	res = call("GET", "/audio/queue", "?volume=1", nil)
	if err := json.NewDecoder(res.Body).Decode(&view); err != nil {
		t.Fatalf("timeline decode: %v", err)
	}
	if len(view.Jobs) != 1 || view.Jobs[0].State != audio.JobDone || len(view.Chapters) != 1 || view.Chapters[0].State != "done" {
		t.Fatalf("timeline after: %+v %+v", view.Jobs, view.Chapters)
	}

	// Flags round-trip through the settings document; the settings version
	// moved (a save happened for the flag), so ETag-based clients refresh.
	res = call("POST", "/audio/flags", "", map[string]any{"chapter": 1, "reason": "nghe lạ"})
	if res.Code != 200 {
		t.Fatalf("add flag: %d %s", res.Code, res.Body.String())
	}
	var flag audio.Flag
	if err := json.NewDecoder(res.Body).Decode(&flag); err != nil || flag.ID == "" {
		t.Fatalf("flag: %v %v", flag, err)
	}
	res = call("GET", "/audio/flags", "", nil)
	var flagsBody struct {
		Flags []audio.Flag `json:"flags"`
	}
	if err := json.NewDecoder(res.Body).Decode(&flagsBody); err != nil || len(flagsBody.Flags) != 1 {
		t.Fatalf("flags list: %v %v", flagsBody, err)
	}
	res = call("GET", "/audio/settings", "", nil)
	if err := json.NewDecoder(res.Body).Decode(&settingsBody); err != nil {
		t.Fatalf("settings decode: %v", err)
	}
	if settingsBody.Version == "" || settingsBody.Version == versionBefore {
		t.Fatalf("settings version did not move after a save: %q → %q", versionBefore, settingsBody.Version)
	}
	res = call("DELETE", "/audio/flags/"+flag.ID, "", nil)
	if res.Code != 200 {
		t.Fatalf("delete flag: %d", res.Code)
	}

	// Hosted capability gaps answer 501, never a fake success.
	res = call("GET", "/audio/volumes/1/chapters/1/peaks", "", nil)
	if res.Code != http.StatusNotImplemented {
		t.Fatalf("peaks: %d %s", res.Code, res.Body.String())
	}
}

// TestPGAudioExportCacheHTTP covers the hosted ffmpeg export: POST enqueues a
// durable audio_export job, the in-process runner materializes inputs and
// reuses the export pipeline, outputs land as blobs + an export_cache
// manifest inside the completion transaction — and an identical second
// request hits the cache without running ffmpeg.
func TestPGAudioExportCacheHTTP(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required for the hosted export pipeline")
	}
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "exp-proj", "exp-story")

	jobs := postgres.NewJobs(pool)
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}

	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		"Trời đã tối. Nàng nhìn ra cửa sổ.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil || !ref.ID.Valid {
		t.Fatalf("chapter ref: %v %v", ref.ID.Valid, err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	script := audio.Script{Chapter: 1, Volume: 1, Status: audio.StatusLocked,
		SourceHash: audio.SourceHash("Trời đã tối. Nàng nhìn ra cửa sổ."),
		Segments:   []audio.Segment{{ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối. Nàng nhìn ra cửa sổ."}}}
	payload, _ := json.Marshal(script)
	if _, err := audioRepo.SaveScript(ctx, scope, postgres.Script{
		ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256: script.SourceHash, Status: audio.StatusLocked, Payload: payload}); err != nil {
		t.Fatalf("save script: %v", err)
	}
	if _, err := audioRepo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{
		SegmentID: "s0001", Ordinal: 0, Speaker: audio.NarratorID,
		Text: script.Segments[0].Text, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("save segment: %v", err)
	}

	// A selected render for the segment: real WAV bytes in the blob store.
	wav := testWAV(t, 8000, 0.2)
	wavSHA := sha256.Sum256(wav)
	wavHex := hex.EncodeToString(wavSHA[:])
	wavKey, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), wavHex, "wav")
	if err != nil {
		t.Fatalf("build wav key: %v", err)
	}
	if _, err := blobs.Put(ctx, wavKey, bytes.NewReader(wav), blobstore.PutOptions{ContentType: "audio/wav"}); err != nil {
		t.Fatalf("put wav: %v", err)
	}
	staged, err := audioRepo.StageBlob(ctx, scope.ProjectID, wavKey, "local", "audio/wav", pgtype.UUID{}, pgtype.UUID{})
	if err != nil {
		t.Fatalf("stage blob: %v", err)
	}
	blobRow, err := audioRepo.MarkBlobReady(ctx, scope.ProjectID, staged.ID, wavHex, int64(len(wav)))
	if err != nil {
		t.Fatalf("mark blob: %v", err)
	}
	synthKey := audio.SynthKey{Text: script.Segments[0].Text, VoiceID: "", Speed: 1,
		Pitch: 0, Quality: "standard", Engine: "remote", Model: "remote"}.Hash()
	if _, err := audioRepo.AddRender(ctx, scope.ProjectID, ref.ID, 1, postgres.Render{
		SegmentID: "s0001", SynthKey: synthKey, BlobID: blobRow.ID,
		DurationSec: 0.2, SampleRate: 8000, Selected: true,
	}); err != nil {
		t.Fatalf("add render: %v", err)
	}
	if _, err := audioRepo.SelectRender(ctx, scope.ProjectID, ref.ID, 1, "s0001", synthKey); err != nil {
		t.Fatalf("select render: %v", err)
	}

	tg := newTestGateway(t)
	st := library.Story{Project: "exp-proj", Story: "exp-story", ID: "exp-proj/exp-story"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	call := func(method, path, query string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var r *http.Request
		if body != nil {
			buf, _ := json.Marshal(body)
			r = httptest.NewRequest(method, "http://x"+path+query, bytes.NewReader(buf))
		} else {
			r = httptest.NewRequest(method, "http://x"+path+query, nil)
		}
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, r, admin, st, path) {
			t.Fatalf("%s %s not handled", method, path)
		}
		return w
	}

	// Enqueue the export over HTTP.
	res := call("POST", "/audio/volumes/1/exports", "", map[string]any{"format": "mp3"})
	if res.Code != http.StatusAccepted {
		t.Fatalf("export enqueue: %d %s", res.Code, res.Body.String())
	}
	rows, err := jobs.List(ctx, scope)
	if err != nil || len(rows) != 1 || rows[0].Kind != postgres.JobAudioExport {
		t.Fatalf("export jobs: %v %v", rows, err)
	}

	runner, err := audio.NewPGExportRunner(ctx, audio.PGDeps{
		Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	ran, err := runner.RunOnce(ctx)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	if !ran {
		t.Fatal("runner claimed nothing")
	}
	done, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: rows[0].ID})
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if done.State != postgres.JobCompleted {
		errBody, _ := io.ReadAll(bytes.NewReader(done.Error))
		t.Fatalf("job = %q error=%s", done.State, errBody)
	}
	var result map[string]any
	if err := json.Unmarshal(done.Result, &result); err != nil || result["cacheHit"] != false {
		t.Fatalf("result: %v %v", result, err)
	}

	// Manifest row + output blobs exist; the produced file is playable bytes.
	entry, err := cache.Get(ctx, scope, exportJobCacheKey(done), "mp3")
	if err != nil {
		t.Fatalf("cache manifest: %v", err)
	}
	var manifest struct {
		Files []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(entry.InputManifest, &manifest); err != nil || len(manifest.Files) == 0 {
		t.Fatalf("manifest: %v %v", manifest, err)
	}
	rc, info, err := blobs.Open(ctx, manifest.Files[0].Path, blobstore.FullRange)
	if err != nil {
		t.Fatalf("open exported blob: %v", err)
	}
	if info.Size == 0 {
		t.Fatal("exported blob is empty")
	}
	rc.Close()

	// ExportInfo lists the produced files with blob URLs.
	res = call("GET", "/audio/volumes/1/exports", "", nil)
	if res.Code != 200 {
		t.Fatalf("export info: %d %s", res.Code, res.Body.String())
	}
	var infoOut audio.ExportInfo
	if err := json.NewDecoder(res.Body).Decode(&infoOut); err != nil || len(infoOut.Files) == 0 {
		t.Fatalf("export info: %v %v", infoOut, err)
	}
	if !strings.HasPrefix(infoOut.Files[0].URL, "/api/blobs/stream/") {
		t.Fatalf("file url: %q", infoOut.Files[0].URL)
	}

	// Identical request → a new job that completes on the cache manifest
	// without touching ffmpeg.
	res = call("POST", "/audio/volumes/1/exports", "", map[string]any{"format": "mp3"})
	if res.Code != http.StatusAccepted {
		t.Fatalf("second enqueue: %d %s", res.Code, res.Body.String())
	}
	ran, err = runner.RunOnce(ctx)
	if err != nil || !ran {
		t.Fatalf("second run: %v ran=%v", err, ran)
	}
	rows, err = jobs.List(ctx, scope)
	if err != nil || len(rows) != 2 {
		t.Fatalf("jobs after second: %d %v", len(rows), err)
	}
	var second postgres.Job
	for _, j := range rows {
		if j.ID != done.ID {
			second = j
		}
	}
	second, err = jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: second.ID})
	if err != nil || second.State != postgres.JobCompleted {
		t.Fatalf("second job = %q %v", second.State, err)
	}
	var result2 map[string]any
	if err := json.Unmarshal(second.Result, &result2); err != nil || result2["cacheHit"] != true {
		t.Fatalf("second result must be a cache hit: %v %v", result2, err)
	}

	// The serving principal invalidates by UPDATE only. Rows and bytes remain
	// available for the maintenance GC, never deleted inline by the app.
	var before, after int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM export_cache`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if n, err := cache.Expire(ctx, scope); err != nil || n == 0 {
		t.Fatalf("app cache expiry marking: n=%d err=%v", n, err)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM export_cache`).Scan(&after); err != nil || after != before {
		t.Fatalf("app expiry deleted cache rows: before=%d after=%d err=%v", before, after, err)
	}
}

// exportJobCacheKey pulls the frozen content key out of a job's input.
func exportJobCacheKey(job postgres.Job) string {
	var in struct {
		CacheKey string `json:"cacheKey"`
	}
	_ = json.Unmarshal(job.Input, &in)
	return in.CacheKey
}

// testWAV builds a small valid PCM16 WAV for render fixtures.
func testWAV(t *testing.T, rate int, seconds float64) []byte {
	t.Helper()
	n := int(float64(rate) * seconds)
	dataLen := n * 2
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	_ = binary.Write(buf, binary.LittleEndian, uint32(rate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(rate*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataLen))
	for i := 0; i < n; i++ {
		v := int16(3000 * math.Sin(2*math.Pi*440*float64(i)/float64(rate)))
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}
