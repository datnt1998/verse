package web

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/pgtest"
)

// provisionHostedTTS builds a hosted gateway (pgAudio set) over a migrated
// schema, which is the only configuration where /api/tts/* reads the worker.
func provisionHostedTTS(t *testing.T) (*testGateway, *postgres.Jobs, *postgres.Pool) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	jobs := postgres.NewJobs(pool)
	tg := newTestGateway(t)
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs})
	return tg, jobs, pool
}

func hostedWorker(t *testing.T, jobs *postgres.Jobs, name string) postgres.Worker {
	t.Helper()
	sum := sha256.Sum256([]byte("token-" + name))
	w, err := jobs.RegisterWorker(context.Background(), name, sum[:], []string{"tts"})
	if err != nil {
		t.Fatalf("register worker: %v", err)
	}
	return w
}

func setWorkerState(t *testing.T, jobs *postgres.Jobs, id pgtype.UUID, state string) {
	t.Helper()
	if err := jobs.SetWorkerState(context.Background(), id, []byte(state)); err != nil {
		t.Fatalf("set worker state: %v", err)
	}
}

// hostedStatus reads GET /api/tts/status as an admin.
func hostedStatus(t *testing.T, tg *testGateway, cookie string) map[string]any {
	t.Helper()
	w := tg.do("GET", "/api/tts/status", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

func TestHostedExportReadinessUsesServerFFmpeg(t *testing.T) {
	if !ffmpegOnPath() {
		t.Skip("ffmpeg required")
	}
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	if got := hostedStatus(t, tg, admin); got["ffmpeg"] != true || got["ready"] != false {
		t.Fatalf("server can export without an online reader: ffmpeg=%v ready=%v", got["ffmpeg"], got["ready"])
	}
	w := hostedWorker(t, jobs, "synthesis-only")
	setWorkerState(t, jobs, w.ID, `{"state":"ready","ffmpeg":false}`)
	if got := hostedStatus(t, tg, admin); got["ffmpeg"] != true {
		t.Fatalf("reader without ffmpeg must not disable server exports: %v", got["ffmpeg"])
	}
	t.Setenv("PATH", t.TempDir())
	setWorkerState(t, jobs, w.ID, `{"state":"ready","ffmpeg":true}`)
	if got := hostedStatus(t, tg, admin); got["ffmpeg"] != false {
		t.Fatalf("reader ffmpeg must not enable exports on a server without it: %v", got["ffmpeg"])
	}
}

// In hosted mode the status endpoint reports what the Mac published — percent,
// error text, ready numbers — and derives `offline` itself.
func TestHostedTTSStatusFromWorkerState(t *testing.T) {
	tg, jobs, pool := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	w := hostedWorker(t, jobs, "mac-mini")

	// Never published: no reader yet, but the machine is connected.
	got := hostedStatus(t, tg, admin)
	if got["engine"] != "remote" {
		t.Fatalf("engine = %v, want remote", got["engine"])
	}
	if got["state"] != voice.StateMissing || got["installed"] != false {
		t.Fatalf("empty state = %v/%v, want missing/false", got["state"], got["installed"])
	}

	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"deps","percent":42,"message":"Đang cài thư viện…"}`)
	got = hostedStatus(t, tg, admin)
	if got["state"] != voice.ProvisionStateProvisioning {
		t.Fatalf("state = %v, want provisioning", got["state"])
	}
	if got["installed"] != true {
		t.Fatalf("installed = %v, want true while provisioning", got["installed"])
	}
	if got["ready"] != false {
		t.Fatalf("ready = %v, want false", got["ready"])
	}

	// Install progress is the same source, in the shape the UI already reads.
	rr := tg.do("GET", "/api/tts/install", admin, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("install: %d %s", rr.Code, rr.Body.String())
	}
	progress := decodeJSONBody(t, rr)
	if progress["state"] != "running" || progress["percent"].(float64) != 42 || progress["step"] != "deps" {
		t.Fatalf("install progress = %v", progress)
	}

	setWorkerState(t, jobs, w.ID, `{"state":"error","percent":-1,"error":{"code":"download_failed","message":"Tải mô hình hỏng"},"message":"Tải mô hình hỏng"}`)
	got = hostedStatus(t, tg, admin)
	errBody, _ := got["error"].(map[string]any)
	if got["state"] != voice.StateError || errBody == nil ||
		errBody["code"] != voice.ProvisionErrDownloadFailed || errBody["message"] != provisionReason(voice.ProvisionErrDownloadFailed) {
		t.Fatalf("error state = %v", got)
	}
	if got["percent"] != nil {
		t.Fatalf("error percent = %v, want null", got["percent"])
	}
	rr = tg.do("GET", "/api/tts/install", admin, "")
	if decodeJSONBody(t, rr)["state"] != "error" {
		t.Fatalf("install state after failure = %s", rr.Body.String())
	}

	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100,"model":"vieneu-1","rtf":0.35,"diskBytes":1073741824}`)
	got = hostedStatus(t, tg, admin)
	if got["state"] != voice.StateReady || got["ready"] != true {
		t.Fatalf("ready state = %v", got)
	}
	if got["model"] != "vieneu-1" || got["rtf"].(float64) != 0.35 || got["installedBytes"].(float64) != 1073741824 {
		t.Fatalf("ready numbers = %v", got)
	}

	// A machine that stopped talking is `offline`, whatever it last published.
	ageLastSeen(t, pool, w.ID)
	got = hostedStatus(t, tg, admin)
	if got["state"] != voice.ProvisionStateOffline {
		t.Fatalf("state = %v, want offline", got["state"])
	}
	if got["ready"] != false {
		t.Fatalf("offline reader reported ready: %v", got)
	}
}

// ageLastSeen pushes the credential's last_seen_at past lease + two heartbeats,
// which is how the server decides a machine is offline.
func ageLastSeen(t *testing.T, pool *postgres.Pool, id pgtype.UUID) {
	t.Helper()
	if _, err := pool.Pool().Exec(context.Background(),
		`UPDATE workers SET last_seen_at = now() - interval '1 hour',
		        updated_at = clock_timestamp(), version = version + 1
		 WHERE id = $1`, id); err != nil {
		t.Fatalf("age last_seen: %v", err)
	}
}

// One click = one queued command; a ready machine answers done without any
// command at all.
func TestHostedInstallPostBumpsWorkerCommand(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	w := hostedWorker(t, jobs, "mac-install")

	setWorkerState(t, jobs, w.ID, `{"state":"missing","percent":-1}`)
	rr := tg.do("POST", "/api/tts/install", admin, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("install = %d %s, want 202", rr.Code, rr.Body.String())
	}
	row, err := jobs.WorkerByID(context.Background(), w.ID)
	if err != nil {
		t.Fatalf("worker by id: %v", err)
	}
	if row.CommandSeq != 1 {
		t.Fatalf("command_seq = %d, want 1", row.CommandSeq)
	}
	var cmd map[string]string
	if err := json.Unmarshal(row.Command, &cmd); err != nil || cmd["op"] != "install" {
		t.Fatalf("command = %s (%v)", row.Command, err)
	}

	// A second click queues a second command (the worker runs only new seqs).
	if rr := tg.do("POST", "/api/tts/install", admin, ""); rr.Code != http.StatusAccepted {
		t.Fatalf("second install = %d", rr.Code)
	}
	row, _ = jobs.WorkerByID(context.Background(), w.ID)
	if row.CommandSeq != 2 {
		t.Fatalf("command_seq = %d, want 2", row.CommandSeq)
	}

	// Already installed and alive: done, no new command.
	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100}`)
	rr = tg.do("POST", "/api/tts/install", admin, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("install when ready = %d %s, want 200", rr.Code, rr.Body.String())
	}
	if decodeJSONBody(t, rr)["state"] != "done" {
		t.Fatalf("body = %s", rr.Body.String())
	}
	row, _ = jobs.WorkerByID(context.Background(), w.ID)
	if row.CommandSeq != 2 {
		t.Fatalf("ready machine got another command: seq = %d", row.CommandSeq)
	}

	// Non-admin cannot queue machine work.
	if rr := tg.do("POST", "/api/tts/install", writer, ""); rr.Code != http.StatusForbidden {
		t.Fatalf("writer install = %d, want 403", rr.Code)
	}
}

// Without any registered machine the button cannot do anything.
func TestHostedInstallWithoutWorkerConflicts(t *testing.T) {
	tg, _, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	if rr := tg.do("POST", "/api/tts/install", admin, ""); rr.Code != http.StatusConflict {
		t.Fatalf("install = %d, want 409", rr.Code)
	}
}

// The SSE stream mirrors the worker state and closes on a terminal state.
func TestHostedInstallEventsStream(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	w := hostedWorker(t, jobs, "mac-stream")
	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"model","percent":10,"message":"Đang tải mô hình…"}`)

	srv := httptest.NewServer(tg.g)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/tts/install/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: tg.g.cookiePolicy().Name, Value: admin})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}

	events := make(chan map[string]any, 4)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var p map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &p) == nil {
				events <- p
			}
		}
		close(events)
	}()

	select {
	case first := <-events:
		if first["state"] != "running" || first["percent"].(float64) != 10 {
			t.Fatalf("first event = %v", first)
		}
	case <-ctx.Done():
		t.Fatal("no event arrived")
	}

	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100,"model":"vieneu-1"}`)
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("stream closed before reporting done")
			}
			if ev["state"] == "done" {
				return
			}
		case <-deadline:
			t.Fatal("stream never reported done")
		}
	}
}

// The dock row is sourced from the worker state in hosted mode.
func TestHostedDockRowFromWorkerState(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	w := hostedWorker(t, jobs, "mac-dock")
	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"model","percent":55,"message":"Đang tải mô hình…"}`)

	rows := voiceJobsProvider{g: tg.g}.Jobs()
	if len(rows) != 1 {
		t.Fatalf("dock rows = %d, want 1", len(rows))
	}
	if rows[0].ID != voiceInstallJobID || rows[0].Status != JobRunning {
		t.Fatalf("row = %+v", rows[0])
	}
	if rows[0].Progress == nil || *rows[0].Progress != 0.55 {
		t.Fatalf("progress = %v, want 0.55", rows[0].Progress)
	}
}

// File/desktop mode keeps reading the local service: the hosted branch must not
// leak into it.
func TestFileModeTTSStatusStaysLocal(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	got := hostedStatus(t, tg, admin)
	if got["engine"] == "remote" {
		t.Fatal("file mode reported the hosted remote engine")
	}
	if got["state"] == voice.ProvisionStateOffline {
		t.Fatal("file mode derived an offline worker state")
	}
	engines := decodeJSONBody(t, tg.do("GET", "/api/tts/engines", admin, ""))
	active, _ := engines["active"].(string)
	if active == "remote" {
		t.Fatal("file mode advertised the hosted engine list")
	}
}

// enqueueQueuedTTSJob puts one waiting voice job in the durable queue: that is
// what `queueWaiting` counts for the machine card.
func enqueueQueuedTTSJob(t *testing.T, pool *postgres.Pool, key string) {
	t.Helper()
	ctx := context.Background()
	projectSlug := "wait-" + key
	if _, err := gen.New(pool.Pool()).EnsureProject(ctx, gen.EnsureProjectParams{
		ID: randomTestUUID(t), Slug: projectSlug, Name: "Wait",
	}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, projectSlug, nil, postgres.StoryInfo{Slug: "story", Title: "Chờ", Status: "writing"}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, projectSlug, "story", nil, false)
	if err != nil {
		t.Fatalf("resolve scope: %v", err)
	}
	if _, err := pool.Pool().Exec(ctx, `
		INSERT INTO jobs (project_id, story_id, id, kind, state, idempotency_key, input)
		VALUES ($1, $2, $3, 'tts', 'queued', $4, '{}'::jsonb)`,
		scope.ProjectID, scope.StoryID, randomTestUUID(t), key); err != nil {
		t.Fatalf("enqueue tts job: %v", err)
	}
}

// A step that cannot measure its progress reports `percent: null`; the UI draws
// a spinner instead of a made-up 0 %. A measured step keeps its number.
func TestHostedStatusUnmeasurableStepHasNoPercent(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	w := hostedWorker(t, jobs, "mac-percent")

	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"tools","percent":-1,"message":"Đang chuẩn bị công cụ…"}`)
	got := hostedStatus(t, tg, admin)
	if got["state"] != voice.ProvisionStateProvisioning || got["step"] != voice.ProvisionStepTools {
		t.Fatalf("state = %v/%v, want provisioning/tools", got["state"], got["step"])
	}
	if got["percent"] != nil {
		t.Fatalf("percent = %v, want null for an unmeasurable step", got["percent"])
	}

	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"model","percent":42,"bytesDone":4200,"bytesTotal":10000,"etaSec":9}`)
	got = hostedStatus(t, tg, admin)
	if got["percent"] == nil || got["percent"].(float64) != 42 {
		t.Fatalf("percent = %v, want 42", got["percent"])
	}
	if got["bytesDone"].(float64) != 4200 || got["bytesTotal"].(float64) != 10000 || got["etaSec"].(float64) != 9 {
		t.Fatalf("model numbers = %v", got)
	}
}

// `queueWaiting` is the number of chapters waiting for a reader: zero while the
// machine is ready, and the queued count whenever it is not.
func TestHostedStatusCountsChaptersWaitingForMachine(t *testing.T) {
	tg, jobs, pool := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	w := hostedWorker(t, jobs, "mac-waiting")
	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100}`)

	got := hostedStatus(t, tg, admin)
	if got["queueWaiting"].(float64) != 0 {
		t.Fatalf("queueWaiting = %v with a ready machine, want 0", got["queueWaiting"])
	}

	enqueueQueuedTTSJob(t, pool, "ch1")
	enqueueQueuedTTSJob(t, pool, "ch2")
	got = hostedStatus(t, tg, admin)
	if got["queueWaiting"].(float64) != 0 {
		t.Fatalf("queueWaiting = %v while the machine is ready, want 0", got["queueWaiting"])
	}

	// Reader gone: the two chapters now wait for it.
	ageLastSeen(t, pool, w.ID)
	got = hostedStatus(t, tg, admin)
	if got["state"] != voice.ProvisionStateOffline {
		t.Fatalf("state = %v, want offline", got["state"])
	}
	if got["queueWaiting"].(float64) != 2 {
		t.Fatalf("queueWaiting = %v, want 2", got["queueWaiting"])
	}
}

// Revoking is one admin step; the status then says `token_rejected` so Nâng cao
// can offer to grant the machine again. Members cannot revoke.
func TestRevokeMachineIsAdminOnlyAndReportsTokenRejected(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	w := hostedWorker(t, jobs, "mac-revoke")
	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100}`)

	if rr := tg.do("POST", "/api/tts/machine/revoke", writer, ""); rr.Code != http.StatusForbidden {
		t.Fatalf("writer revoke = %d, want 403", rr.Code)
	}
	rr := tg.do("POST", "/api/tts/machine/revoke", admin, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke = %d %s", rr.Code, rr.Body.String())
	}
	if body := decodeJSONBody(t, rr); body["machine"] != "mac-revoke" {
		t.Fatalf("revoke body = %v", body)
	}

	got := hostedStatus(t, tg, admin)
	if got["state"] != voice.ProvisionStateError || got["ready"] != false {
		t.Fatalf("revoked status = %v, want error/not ready", got)
	}
	errBody, _ := got["error"].(map[string]any)
	if errBody == nil || errBody["code"] != voice.ProvisionErrTokenRejected {
		t.Fatalf("error = %v, want token_rejected", got["error"])
	}
	machine, _ := got["machine"].(map[string]any)
	if machine == nil || machine["revokedAt"] == nil {
		t.Fatalf("machine = %v, want revokedAt", got["machine"])
	}

	// Revoking again is harmless: the machine is already cut off.
	if rr := tg.do("POST", "/api/tts/machine/revoke", admin, ""); rr.Code != http.StatusOK {
		t.Fatalf("second revoke = %d, want 200", rr.Code)
	}
}

// Channel and log tail belong to Nâng cao: admins get them, members never do.
func TestHostedStatusHidesTechnicalDetailFromMembers(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	w := hostedWorker(t, jobs, "mac-detail")
	setWorkerState(t, jobs, w.ID, `{"state":"error","percent":-1,"channel":"https://api.example.test",
		"error":{"code":"deps_failed","message":"Thiếu thư viện","logTail":"dòng 1\ndòng 2\ndòng 3\n"}}`)

	adminView := hostedStatus(t, tg, admin)
	if adminView["channel"] != "https://api.example.test" {
		t.Fatalf("admin channel = %v", adminView["channel"])
	}
	tail, _ := adminView["logTail"].([]any)
	if len(tail) != 4 || tail[0] != "Thiếu thư viện" || tail[3] != "dòng 3" {
		t.Fatalf("admin logTail = %v, want the cause then 3 log lines", adminView["logTail"])
	}

	memberView := hostedStatus(t, tg, writer)
	if _, ok := memberView["channel"]; ok {
		t.Fatalf("member saw the channel: %v", memberView["channel"])
	}
	if _, ok := memberView["logTail"]; ok {
		t.Fatalf("member saw the log tail: %v", memberView["logTail"])
	}
	if errBody, _ := memberView["error"].(map[string]any); errBody == nil ||
		errBody["message"] != provisionReason(voice.ProvisionErrDepsFailed) {
		t.Fatalf("member error = %v, want the 8.2 sentence for deps_failed", memberView["error"])
	}
}

// On-demand preview exists only when the machine advertises it: without the
// capability the endpoint answers 503 instead of hanging on a call the reader
// will never take (10-audio.md §8.4 #6).
func TestHostedPreviewNeedsWorkerCapability(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	body := `{"text":"Xin chào","voiceId":"vieneu-nu-bac"}`

	rr := tg.do("POST", "/api/tts/preview", admin, body)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("preview without machine = %d, want 503", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "chưa hỗ trợ nghe thử") {
		t.Fatalf("preview body = %s", rr.Body.String())
	}

	w := hostedWorker(t, jobs, "mac-preview")
	setWorkerState(t, jobs, w.ID, `{"state":"ready","percent":100,"capabilities":{"preview":true}}`)
	// With the capability the endpoint moves past the gate; an empty text is
	// rejected by validation, so the test never waits on a real synthesis.
	rr = tg.do("POST", "/api/tts/preview", admin, `{"text":""}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("preview with capability = %d %s, want 400 from validation", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "chưa hỗ trợ nghe thử") {
		t.Fatalf("preview with capability was refused: %s", rr.Body.String())
	}
}
