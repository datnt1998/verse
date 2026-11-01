package web

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// gateRunner holds the first tool call until the test releases it, which is how
// these tests prove the HTTP request no longer waits for the download/decode.
type gateRunner struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
	info    editor.LinkInfo
	write   func(args []string) error
}

func newGateRunner(info editor.LinkInfo, write func(args []string) error) *gateRunner {
	return &gateRunner{started: make(chan struct{}), release: make(chan struct{}), info: info, write: write}
}

func (r *gateRunner) Run(ctx context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
	case <-ctx.Done():
		return tools.Result{}, ctx.Err()
	}
	if len(args) > 0 && args[0] == "-J" {
		payload, e := json.Marshal(r.info)
		return tools.Result{Stdout: string(payload)}, e
	}
	return tools.Result{}, r.write(args)
}

type gateProber struct{ probe editor.ProbeResult }

func (p gateProber) Probe(context.Context, string) (editor.ProbeResult, error) { return p.probe, nil }

func writeLinkOutput(args []string) error {
	output := ""
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) {
			output = strings.ReplaceAll(args[i+1], "%(ext)s", "mp4")
		}
	}
	if output == "" {
		return os.ErrInvalid
	}
	payload, e := os.ReadFile(filepath.Join("..", "editor", "testdata", "__fixtures__", "mp4-h264.mp4"))
	if e != nil {
		return e
	}
	return os.WriteFile(output, payload, 0o600)
}

func writeWaveformOutput(args []string) error {
	pcm := make([]byte, 960*8)
	for i := range 960 {
		binary.LittleEndian.PutUint32(pcm[i*8:], math.Float32bits(.5))
		binary.LittleEndian.PutUint32(pcm[i*8+4:], math.Float32bits(-.5))
	}
	return os.WriteFile(args[len(args)-1], pcm, 0o600)
}

func editorJob(t *testing.T, h editorHTTP, id string) map[string]any {
	t.Helper()
	w := editorRequest(h, "GET", "/api/editor/jobs/"+id, "", "")
	if w.Code != 200 {
		t.Fatalf("job %s: %d %s", id, w.Code, w.Body.String())
	}
	job, ok := editorObject(t, w)["job"].(map[string]any)
	if !ok {
		t.Fatalf("job %s: %s", id, w.Body.String())
	}
	return job
}

func waitEditorJob(t *testing.T, h editorHTTP, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		job := editorJob(t, h, id)
		switch job["state"] {
		case "succeeded", "failed", "cancelled":
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s stuck in %v", id, job["state"])
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// postWhileBlocked issues the POST from another goroutine so a handler that
// still waits for the download fails the test instead of hanging it.
func postWhileBlocked(t *testing.T, h editorHTTP, path, body string, release func()) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", path, strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		h.serve(recorder, request)
		done <- recorder
	}()
	select {
	case recorder := <-done:
		cancel()
		return recorder
	case <-time.After(2 * time.Second):
		// Let the blocked tool finish so the test can report cleanly.
		release()
		t.Fatalf("POST %s did not answer within 2 s while the fetcher was blocked", path)
		return nil
	}
}

// Cloudflare cuts a request that sends no byte for 100 s (HTTP 524), so the
// link import answers 202 with a job, the download runs detached from the
// request, and the finished media is polled and then listed in the project.
func TestEditorMediaLinkJobReturnsAcceptedAndSurvivesDisconnect(t *testing.T) {
	h := editorHTTPFixture(t)
	runner := newGateRunner(editor.LinkInfo{Title: "Clip biên tập", Duration: new(float64(30)), Channel: new("streamer")}, writeLinkOutput)
	h.service.FFmpeg = "/managed/ffmpeg"
	h.service.YTDLP = "/managed/yt-dlp"
	h.service.Runner = runner
	h.service.Prober = gateProber{probe: editor.ProbeResult{Kind: "video", MIME: "video/mp4", Width: new(float64(160)), Height: new(float64(90)), DurationSec: new(float64(1))}}

	create := editorRequest(h, "POST", "/api/editor/projects", `{"name":"Link job","template":"youtube-1080p"}`, "")
	if create.Code != 201 {
		t.Fatalf("project: %d %s", create.Code, create.Body.String())
	}
	projectID := editorObject(t, create)["project"].(map[string]any)["id"].(string)

	accepted := postWhileBlocked(t, h, "/api/editor/media/link", `{"url":"https://www.twitch.tv/videos/2899740602","projectId":"`+projectID+`"}`, func() { close(runner.release) })
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("link: %d %s", accepted.Code, accepted.Body.String())
	}
	started := editorObject(t, accepted)["job"].(map[string]any)
	jobID, _ := started["id"].(string)
	if jobID == "" || started["kind"] != "media.link" {
		t.Fatalf("accepted job: %v", started)
	}
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the download never started")
	}
	// The request is gone; the job must still be running and then finish.
	if running := editorJob(t, h, jobID); running["state"] != "running" {
		t.Fatalf("blocked job: %v", running)
	}
	close(runner.release)
	done := waitEditorJob(t, h, jobID)
	if done["state"] != "succeeded" {
		t.Fatalf("job: %v", done)
	}
	result, _ := done["result"].(map[string]any)
	media, _ := result["media"].(map[string]any)
	mediaID, _ := media["id"].(string)
	if mediaID == "" || result["reused"] != false {
		t.Fatalf("result: %v", done)
	}
	listed := editorRequest(h, "GET", "/api/editor/media?projectId="+projectID, "", "")
	records, _ := editorObject(t, listed)["media"].([]any)
	found := false
	for _, record := range records {
		if entry, ok := record.(map[string]any); ok && entry["id"] == mediaID {
			found = true
		}
	}
	if !found {
		t.Fatalf("finished media missing from the project: %s", listed.Body.String())
	}
	// A second request reuses the imported file instead of downloading again.
	again := postWhileBlocked(t, h, "/api/editor/media/link", `{"url":"https://www.twitch.tv/videos/2899740602","projectId":"`+projectID+`"}`, func() {})
	if again.Code != http.StatusAccepted {
		t.Fatalf("second link: %d %s", again.Code, again.Body.String())
	}
	reused := waitEditorJob(t, h, editorObject(t, again)["job"].(map[string]any)["id"].(string))
	if reused["state"] != "succeeded" || reused["result"].(map[string]any)["reused"] != true {
		t.Fatalf("reuse: %v", reused)
	}
}

// A bad link or project still fails the POST itself: validation is not
// deferred to the job.
func TestEditorMediaLinkRejectsBadRequestsBeforeQueueing(t *testing.T) {
	h := editorHTTPFixture(t)
	for _, body := range []string{
		`{"url":"https://www.twitch.tv/shroud","projectId":"missing"}`,
		`{"url":"https://www.youtube.com/watch?v=x","projectId":"missing"}`,
		`{"url":"https://www.twitch.tv/videos/1","projectId":"missing","from":"90"}`,
	} {
		response := editorRequest(h, "POST", "/api/editor/media/link", body, "")
		if response.Code != 400 {
			t.Fatalf("%s: %d %s", body, response.Code, response.Body.String())
		}
	}
	if response := editorRequest(h, "POST", "/api/editor/media/link", `{"url":"https://www.twitch.tv/videos/1","projectId":"missing"}`, ""); response.Code != 404 {
		t.Fatalf("missing project: %d %s", response.Code, response.Body.String())
	}
	if response := editorRequest(h, "GET", "/api/editor/jobs/00000000-0000-4000-8000-000000000000", "", ""); response.Code != 404 {
		t.Fatalf("unknown job: %d %s", response.Code, response.Body.String())
	}
}

// The waveform decode is the same detached pattern: a long voice source would
// otherwise hold the request past the proxy limit.
func TestEditorMediaWaveformJobReturnsAcceptedWhileDecoding(t *testing.T) {
	h := editorHTTPFixture(t)
	voice, e := os.Open(filepath.Join("..", "editor", "testdata", "__fixtures__", "audio.wav"))
	if e != nil {
		t.Fatal(e)
	}
	imported := httptest.NewRequest("POST", "/api/editor/media?fileName=voice.wav", voice)
	imported.Header.Set("Content-Type", "audio/wav")
	recorder := httptest.NewRecorder()
	h.serve(recorder, imported)
	_ = voice.Close()
	if recorder.Code != 201 {
		t.Fatalf("import: %d %s", recorder.Code, recorder.Body.String())
	}
	media := editorObject(t, recorder)["media"].(map[string]any)
	mediaID := media["id"].(string)

	runner := newGateRunner(editor.LinkInfo{}, writeWaveformOutput)
	h.service.FFmpeg = "/managed/ffmpeg"
	h.service.Runner = runner
	accepted := postWhileBlocked(t, h, "/api/editor/media/"+mediaID+"/waveform", `{}`, func() { close(runner.release) })
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("waveform: %d %s", accepted.Code, accepted.Body.String())
	}
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the decode never started")
	}
	close(runner.release)
	done := waitEditorJob(t, h, editorObject(t, accepted)["job"].(map[string]any)["id"].(string))
	if done["state"] != "succeeded" {
		t.Fatalf("job: %v", done)
	}
	envelope, _ := done["result"].(map[string]any)["envelope"].(map[string]any)
	if envelope["sourceHash"] != media["contentHash"] || envelope["samplesPerSecond"] != float64(50) {
		t.Fatalf("envelope: %v", done)
	}

	// A non-audio source is rejected by the POST, not by a queued job.
	image, e := os.Open(filepath.Join("..", "editor", "testdata", "__fixtures__", "image.png"))
	if e != nil {
		t.Fatal(e)
	}
	picture := httptest.NewRequest("POST", "/api/editor/media?fileName=anh.png", image)
	picture.Header.Set("Content-Type", "image/png")
	recorder = httptest.NewRecorder()
	h.serve(recorder, picture)
	_ = image.Close()
	if recorder.Code != 201 {
		t.Fatalf("image import: %d %s", recorder.Code, recorder.Body.String())
	}
	pictureID := editorObject(t, recorder)["media"].(map[string]any)["id"].(string)
	rejected := editorRequest(h, "POST", "/api/editor/media/"+pictureID+"/waveform", `{}`, "")
	if rejected.Code != 400 || editorObject(t, rejected)["code"] != "invalid_request" {
		t.Fatalf("image waveform: %d %s", rejected.Code, rejected.Body.String())
	}
}
