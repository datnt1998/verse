package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgtest"
)

// TestWorkerAPIPostgresPublish is the phase-07 worker gate over real Postgres:
// a TTS worker claims a job, uploads a synthesized segment, ACKs it with the
// hash + fence, and the render row/job completion land together. A worker
// whose lease was reclaimed cannot upload or publish afterwards, and its
// output never becomes the selected render.
func TestWorkerAPIPostgresPublish(t *testing.T) {
	ctx := context.Background()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration db: %v", err)
	}
	defer db.Close()
	if _, err := migrate.Up(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	q := gen.New(pool.Pool())
	projectID := randomTestUUID(t)
	if _, err := q.EnsureProject(ctx, gen.EnsureProjectParams{ID: projectID, Slug: "wg-proj", Name: "WG"}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, "wg-proj", nil, postgres.StoryInfo{Slug: "wg-story", Title: "WG", Status: "writing"}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, "wg-proj", "wg-story", nil, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated, "Một đoạn văn để đọc.",
		domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	var chapterID pgtype.UUID
	if err := pool.Pool().QueryRow(ctx, `SELECT id FROM chapters WHERE project_id=$1 AND story_id=$2 AND number=1`,
		scope.ProjectID, scope.StoryID).Scan(&chapterID); err != nil {
		t.Fatalf("chapter id: %v", err)
	}

	audioRepo := postgres.NewAudio(pool)
	if _, err := audioRepo.SaveScript(ctx, scope, postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: 1,
		SourceSHA256: strings.Repeat("c", 64), Status: "draft", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("save script: %v", err)
	}
	if _, err := audioRepo.SaveSegment(ctx, scope.ProjectID, chapterID, 1, postgres.Segment{
		SegmentID: "seg-1", Ordinal: 0, Speaker: "nguoi-ke", Text: "Một đoạn văn để đọc.", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("save segment: %v", err)
	}

	jobs := postgres.NewJobs(pool)
	tokenA := testWorkerToken
	sumA := sha256.Sum256([]byte(tokenA))
	if _, err := jobs.RegisterWorker(ctx, "mac-a", sumA[:], []string{"tts"}); err != nil {
		t.Fatalf("register worker A: %v", err)
	}
	tokenB := testWorkerToken + "-b"
	sumB := sha256.Sum256([]byte(tokenB))
	if _, err := jobs.RegisterWorker(ctx, "mac-b", sumB[:], []string{"tts"}); err != nil {
		t.Fatalf("register worker B: %v", err)
	}

	enqueue := func(key string) postgres.Job {
		job, err := jobs.Enqueue(ctx, scope, postgres.JobTTS, key, map[string]any{
			"kind":           "create",
			"chapterId":      postgres.UUIDString(chapterID),
			"scriptRevision": 1,
			"sourceRevision": 1,
			"segments": []map[string]any{{
				"id": "seg-1", "text": "Một đoạn văn để đọc.", "speaker": "nguoi-ke",
				"voice": "", "tone": "", "speed": 1, "pitch": 1, "quality": "standard",
				"engine": "remote", "model": "remote",
				"synthKey": (audio.SynthKey{Text: "Một đoạn văn để đọc.", Speed: 1, Pitch: 1, Quality: "standard", Engine: "remote", Model: "remote"}).Hash(),
			}},
		}, nil, pgtype.UUID{})
		if err != nil {
			t.Fatalf("enqueue %s: %v", key, err)
		}
		return job
	}

	blobs, err := blobstore.NewLocal(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	api := &WorkerAPI{
		Queue:      &PGWorkerQueue{Jobs: jobs, Audio: audioRepo},
		Blobs:      blobs,
		Registry:   &PGAudioBlobs{Audio: audioRepo},
		BlobDriver: "local",
		Lease:      2 * time.Second,
	}
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	call := func(token, method, path string, body []byte, headers map[string]string) *http.Response {
		req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	// ── claim ────────────────────────────────────────────────────────────────
	job := enqueue("ch1-create")
	resp := call(tokenA, http.MethodPost, "/internal/worker/claim", []byte(`{}`), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("claim = %d", resp.StatusCode)
	}
	var claimed struct {
		Job WorkerJob `json:"job"`
	}
	decodeWorkerResp(t, resp, &claimed)
	if claimed.Job.ID != postgres.UUIDString(job.ID) || claimed.Job.ProjectID != postgres.UUIDString(scope.ProjectID) {
		t.Fatalf("claimed = %+v", claimed.Job)
	}
	if !bytes.Contains(claimed.Job.Input, []byte("chapterId")) {
		t.Fatalf("claim snapshot missing chapterId: %s", claimed.Job.Input)
	}

	// ── upload ──────────────────────────────────────────────────────────────
	segmentBytes := []byte("ID3 synthesized segment bytes")
	segSum := sha256.Sum256(segmentBytes)
	segSHA := hex.EncodeToString(segSum[:])
	uploadPath := "/internal/worker/jobs/" + claimed.Job.ID + "/blob"
	resp = call(tokenA, http.MethodPost, uploadPath, segmentBytes, map[string]string{
		uploadFenceHeader: strconv.FormatInt(claimed.Job.Fence, 10),
		uploadSHAHeader:   segSHA,
		"Content-Type":    "audio/mpeg",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d", resp.StatusCode)
	}
	var uploaded struct {
		Key    string `json:"key"`
		SHA256 string `json:"sha256"`
	}
	decodeWorkerResp(t, resp, &uploaded)
	if uploaded.SHA256 != segSHA {
		t.Fatalf("upload hash = %q", uploaded.SHA256)
	}

	// ── complete with attachment ────────────────────────────────────────────
	synthKey := audio.SynthKey{
		Text: "Một đoạn văn để đọc.", Speed: 1, Pitch: 1,
		Quality: "standard", Engine: "remote", Model: "remote",
	}.Hash()
	ack := map[string]any{
		"fence":        claimed.Job.Fence,
		"progressDone": 1,
		"result":       map[string]any{"segments": 1},
		"attachments": []map[string]any{{
			"segmentId": "seg-1", "synthKey": synthKey, "key": uploaded.Key, "sha256": segSHA,
			"durationSec": 2.5, "sampleRate": 24000, "chapterId": postgres.UUIDString(chapterID),
			"scriptRevision": 1, "select": true, "parameters": map[string]any{"speed": 1},
		}},
	}
	ackBody, _ := json.Marshal(ack)
	resp = call(tokenA, http.MethodPost, "/internal/worker/jobs/"+claimed.Job.ID+"/complete", ackBody, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("complete = %d: %s", resp.StatusCode, body)
	}

	done, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: job.ID})
	if err != nil {
		t.Fatalf("get completed job: %v", err)
	}
	if done.State != postgres.JobCompleted || done.FinishedAt == nil {
		t.Fatalf("job = %q finished=%v", done.State, done.FinishedAt)
	}
	render, err := audioRepo.SelectedRender(ctx, scope.ProjectID, chapterID, 1, "seg-1")
	if err != nil {
		t.Fatalf("selected render: %v", err)
	}
	blob, err := audioRepo.BlobByKey(ctx, scope.ProjectID, uploaded.Key)
	if err != nil {
		t.Fatalf("blob by key: %v", err)
	}
	if render.BlobID != blob.ID || render.SynthKey != synthKey || !render.Selected {
		t.Fatalf("render = %+v blob=%v", render, blob.ID)
	}

	// ── stale worker after reclaim ──────────────────────────────────────────
	second := enqueue("ch1-update")
	resp = call(tokenA, http.MethodPost, "/internal/worker/claim", []byte(`{}`), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second claim = %d", resp.StatusCode)
	}
	decodeWorkerResp(t, resp, &claimed)
	if claimed.Job.ID != postgres.UUIDString(second.ID) {
		t.Fatalf("second claim = %+v", claimed.Job)
	}
	// Worker A's lease lapses; worker B reclaims the same job with a higher fence.
	if _, err := pool.Pool().Exec(ctx, `
		UPDATE jobs SET lease_until = now() - interval '1 minute',
			updated_at = clock_timestamp(), version = version + 1
		WHERE project_id = $1 AND id = $2`, scope.ProjectID, second.ID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	resp = call(tokenB, http.MethodPost, "/internal/worker/claim", []byte(`{}`), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reclaim = %d", resp.StatusCode)
	}
	var reclaimed struct {
		Job WorkerJob `json:"job"`
	}
	decodeWorkerResp(t, resp, &reclaimed)
	if reclaimed.Job.Fence <= claimed.Job.Fence {
		t.Fatalf("fence did not move: %d -> %d", claimed.Job.Fence, reclaimed.Job.Fence)
	}

	staleBytes := []byte("stale worker output")
	staleSum := sha256.Sum256(staleBytes)
	resp = call(tokenA, http.MethodPost, "/internal/worker/jobs/"+claimed.Job.ID+"/blob", staleBytes, map[string]string{
		uploadFenceHeader: strconv.FormatInt(claimed.Job.Fence, 10),
		uploadSHAHeader:   hex.EncodeToString(staleSum[:]),
		"Content-Type":    "audio/mpeg",
	})
	if resp.StatusCode < 400 {
		t.Fatalf("stale worker upload = %d, want rejection", resp.StatusCode)
	}
	staleAck, _ := json.Marshal(map[string]any{
		"fence": claimed.Job.Fence, "progressDone": 1,
		"attachments": []map[string]any{{
			"segmentId": "seg-1", "synthKey": synthKey, "key": uploaded.Key, "sha256": segSHA,
			"durationSec": 2.5, "sampleRate": 24000, "chapterId": postgres.UUIDString(chapterID),
			"scriptRevision": 1, "select": false,
		}},
	})
	resp = call(tokenA, http.MethodPost, "/internal/worker/jobs/"+claimed.Job.ID+"/complete", staleAck, nil)
	if resp.StatusCode < 400 {
		t.Fatalf("stale worker complete = %d, want rejection", resp.StatusCode)
	}
	if state, err := jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: second.ID}); err != nil {
		t.Fatalf("get reclaimed: %v", err)
	} else if state.State != postgres.JobRunning {
		t.Fatalf("stale ACK changed job state to %q", state.State)
	}

	// The new owner finishes the job.
	resp = call(tokenB, http.MethodPost, "/internal/worker/jobs/"+reclaimed.Job.ID+"/complete",
		[]byte(`{"fence":`+strconv.FormatInt(reclaimed.Job.Fence, 10)+`,"progressDone":1,"result":{"segments":1}}`), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("new owner complete = %d", resp.StatusCode)
	}

	// A revoked credential is cut off at the door: no claim, no state publish.
	workers, err := jobs.ListWorkers(ctx)
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	for _, wk := range workers {
		if wk.Name != "mac-b" {
			continue
		}
		if err := jobs.RevokeWorker(ctx, wk.ID); err != nil {
			t.Fatalf("revoke worker b: %v", err)
		}
	}
	resp = call(tokenB, http.MethodPost, "/internal/worker/claim", []byte(`{}`), nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked claim = %d, want 401", resp.StatusCode)
	}
}

func decodeWorkerResp(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func randomTestUUID(t *testing.T) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		t.Fatalf("uuid: %v", err)
	}
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}
