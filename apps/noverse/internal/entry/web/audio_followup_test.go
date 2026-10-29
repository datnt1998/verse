package web

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

func TestAudioProviderLiveWatch(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	sub, _, _ := f.tg.g.events().subscribe(webauth.User{Role: webauth.RoleAdmin}, 0, nil)
	defer f.tg.g.events().unsubscribe(sub)
	f.seedScript(t, 1, audio.StatusLocked)
	if _, err := f.svc.Enqueue(1, 1); err != nil {
		t.Fatal(err)
	}
	sawProgress := false
	timeout := time.After(20 * time.Second)
	for {
		select {
		case event := <-sub.ch:
			if event.Type == "job.progress" {
				if job, ok := event.Data.(Job); ok && job.ID == "proj/01-tra-xanh/audio/1" {
					sawProgress = true
				}
			}
			if event.Type == "job.done" {
				if job, ok := event.Data.(Job); ok && job.ID == "proj/01-tra-xanh/audio/1" {
					if !sawProgress {
						t.Fatal("live Watch missed job.progress before job.done")
					}
					return
				}
			}
		case <-timeout:
			t.Fatal("live Watch never published job.done")
		}
	}
}

func TestAudioExportM4BChaptersAndSubtitlesToggle(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	for ch := 1; ch <= 2; ch++ {
		f.seedScript(t, ch, audio.StatusLocked)
		if _, err := f.svc.Enqueue(1, ch); err != nil {
			t.Fatal(err)
		}
		if j := f.waitJob(t, ch, audio.JobDone, audio.JobError); j.State != audio.JobDone {
			t.Fatal(j.Error)
		}
	}
	w := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapterMarks":true,"subtitles":true,"loudness":true,"gapSec":0.25}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("m4b export: %d %s", w.Code, w.Body.String())
	}
	request := waitAudioExportState(t, f.svc, audio.JobDone)
	info, err := f.svc.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationSec <= 1 || len(info.ChapterMarks) != 2 {
		t.Fatalf("missing duration/marks: %+v", info)
	}
	var volumeFile string
	for _, fout := range request.Result.Files {
		if strings.HasSuffix(fout.Path, ".m4b") {
			volumeFile = filepath.Join(f.svc.Layout().ExportsDir, fout.Path)
		}
	}
	if volumeFile == "" {
		t.Fatal("no .m4b output")
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_chapters", "-show_streams", "-of", "json", volumeFile).Output()
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Chapters []struct {
			Start string            `json:"start_time"`
			Tags  map[string]string `json:"tags"`
		} `json:"chapters"`
		Streams []struct {
			Codec string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Chapters) != 2 || !strings.Contains(probe.Chapters[1].Tags["title"], "Chương 2") {
		t.Fatalf("ffprobe chapters: %s", out)
	}
	if len(probe.Streams) == 0 || probe.Streams[0].Codec != "aac" {
		t.Fatalf("m4b must contain AAC: %s", out)
	}
	if _, err := os.Stat(filepath.Join(f.svc.Layout().VolumeExportDir(1), "Tập 1.srt")); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"mp3", "wav"} {
		w = f.do("POST", "/audio/volumes/1/exports", `{"format":"`+format+`","subtitles":false,"chapterMarks":false}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("%s export: %d %s", format, w.Code, w.Body.String())
		}
		waitAudioExportState(t, f.svc, audio.JobDone)
	}
	if _, err := os.Stat(filepath.Join(f.svc.Layout().VolumeExportDir(1), "Tập 1.srt")); !os.IsNotExist(err) {
		t.Fatalf("subtitles:false left .srt: %v", err)
	}
}

func TestAudioGatewayResumesPersistedQueueAndPendingExport(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	for ch := 1; ch <= 2; ch++ {
		f.seedScript(t, ch, audio.StatusLocked)
	}
	w := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapterMarks":true,"subtitles":false,"whenReady":true}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("schedule export: %d %s", w.Code, w.Body.String())
	}
	if decodeJSONBody(t, w)["jobId"] == "" {
		t.Fatal("whenReady must return a jobId")
	}
	if len(f.svc.ExportRequests()) != 1 {
		t.Fatal("whenReady must persist a pending request before chapters finish")
	}
	f.svc.Close()
	audioServices.Delete(f.st.Dir)
	now := time.Now()
	queue := map[string]any{"jobs": []audio.Job{{ID: "resume-1", Volume: 1, Chapter: 1, State: audio.JobRendering, CreatedAt: now, UpdatedAt: now}, {ID: "resume-2", Volume: 1, Chapter: 2, State: audio.JobQueued, CreatedAt: now, UpdatedAt: now}}}
	data, _ := json.Marshal(queue)
	if err := os.WriteFile(filepath.Join(f.svc.Layout().AudioDir, "queue.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	g := NewGateway(f.tg.root, f.tg.g.accounts, version.Info{})
	defer g.Close()
	cached, ok := audioServices.Load(f.st.Dir)
	if !ok {
		t.Fatal("gateway startup did not reopen persisted audio queue")
	}
	svc := cached.(audioEntry).svc
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			requests := svc.ExportRequests()
			if len(requests) == 1 && requests[0].State == audio.JobError {
				t.Fatal(requests[0].Error)
			}
			if len(requests) == 1 && requests[0].State == audio.JobDone {
				if requests[0].DurationSec <= 0 {
					t.Fatal("automatic export lost duration")
				}
				info, err := svc.ExportInfo(1)
				if err != nil {
					t.Fatal(err)
				}
				if info.Format != "m4b" || math.Abs(info.DurationSec-requests[0].DurationSec) > .01 {
					t.Fatalf("export info=%+v request=%+v", info, requests[0])
				}
				// Await the provider, not just the audio worker's final write.
				if job, ok := g.jobsHub().get("proj/01-tra-xanh/audio/1/export"); ok && job.Status == JobDoneS {
					return
				}
			}
		case <-deadline:
			t.Fatalf("persisted work did not resume: queue=%+v exports=%+v", svc.Queue(), svc.ExportRequests())
		}
	}
}

func TestAudioSettingsVersionConflictReturnsCurrent(t *testing.T) {
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	get := f.do("GET", "/audio/settings", "")
	var settings map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["version"] == "" || get.Header().Get("ETag") == "" {
		t.Fatal("settings GET missing version/ETag")
	}
	settings["quality"] = "high"
	data, _ := json.Marshal(settings)
	put := f.do("PUT", "/audio/settings", string(data))
	if put.Code != 200 {
		t.Fatalf("save settings: %d %s", put.Code, put.Body.String())
	}
	stale := f.do("PUT", "/audio/settings", string(data))
	if stale.Code != 409 {
		t.Fatalf("stale settings: %d %s", stale.Code, stale.Body.String())
	}
	var conflict struct {
		Current map[string]any `json:"current"`
		Version string         `json:"version"`
	}
	if err := json.Unmarshal(stale.Body.Bytes(), &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Current["quality"] != "high" || conflict.Current["version"] != conflict.Version {
		t.Fatalf("conflict must return current GET resource: %s", stale.Body.String())
	}
}
