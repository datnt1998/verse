package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

func TestAudioPartialExportSelectedChaptersOnly(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.seedScript(t, 2, audio.StatusLocked)
	script, err := audio.ReadScript(f.svc.Layout(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	script.Segments = script.Segments[:1]
	script.Segments[0].Text = "Chỉ có chương hai."
	if err := audio.WriteScript(f.svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Enqueue(1, 2); err != nil {
		t.Fatal(err)
	}
	if job := f.waitJob(t, 2, audio.JobDone, audio.JobError); job.State != audio.JobDone {
		t.Fatal(job.Error)
	}
	response := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[2,2],"chapterMarks":true,"subtitles":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("selected ready chapter must export without unready chapter 1: %d %s", response.Code, response.Body.String())
	}
	if req := waitAudioExportState(t, f.svc, audio.JobDone); req.State != audio.JobDone {
		t.Fatalf("partial export failed: %+v", req)
	}
	info, err := f.svc.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.ChapterMarks) != 1 || !strings.Contains(info.ChapterMarks[0].Label, "Chương 2") {
		t.Fatalf("partial marks include unselected chapter: %+v", info.ChapterMarks)
	}
	foundSubtitle := false
	for _, file := range info.Files {
		if strings.HasSuffix(file.Name, ".m4b") && !strings.Contains(file.Name, "-ch2.") {
			t.Fatalf("partial filename missing selection suffix: %s", file.Name)
		}
		if strings.HasSuffix(file.Name, ".m4b") {
			output, err := exec.Command("ffprobe", "-v", "error", "-show_chapters", "-of", "json", filepath.Join(f.svc.Layout().ExportsDir, file.Path)).Output()
			if err != nil {
				t.Fatal(err)
			}
			var probe struct {
				Chapters []struct {
					Tags map[string]string `json:"tags"`
				} `json:"chapters"`
			}
			if err := json.Unmarshal(output, &probe); err != nil {
				t.Fatal(err)
			}
			if len(probe.Chapters) != 1 || !strings.Contains(probe.Chapters[0].Tags["title"], "Chương 2") {
				t.Fatalf("embedded partial chapters: %s", output)
			}
		}
		if strings.HasSuffix(file.Name, ".srt") {
			foundSubtitle = true
			data, err := os.ReadFile(filepath.Join(f.svc.Layout().ExportsDir, file.Path))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "Chỉ có chương hai.") || strings.Contains(string(data), "Trên tivi") {
				t.Fatalf("partial subtitles: %s", data)
			}
		}
	}
	if !foundSubtitle {
		t.Fatal("missing partial subtitles")
	}
	for _, body := range []string{`{"format":"m4b","chapters":[1]}`, `{"format":"m4b","chapters":[99]}`, `{"format":"m4b","chapters":[]}`} {
		bad := f.do("POST", "/audio/volumes/1/exports", body)
		if bad.Code != 409 {
			t.Fatalf("invalid/unvoiced selection must conflict: %s: %d %s", body, bad.Code, bad.Body.String())
		}
		if strings.Contains(body, "[99]") && !strings.Contains(bad.Body.String(), "Chương 99") {
			t.Fatalf("unknown chapter error must name it: %s", bad.Body.String())
		}
		if strings.Contains(body, "[1]") && !strings.Contains(bad.Body.String(), "Chương 1") {
			t.Fatalf("missing chapter error must name it: %s", bad.Body.String())
		}
	}
}

func TestAudioPartialExportKeepsFullVolumeAndKeepVariant(t *testing.T) {
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
	full := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","subtitles":true}`)
	if full.Code != http.StatusAccepted {
		t.Fatalf("full export: %d %s", full.Code, full.Body.String())
	}
	waitAudioExportState(t, f.svc, audio.JobDone)
	var originalPath string
	for _, file := range waitAudioExportState(t, f.svc, audio.JobDone).Result.Files {
		if strings.HasSuffix(file.Name, ".m4b") {
			originalPath = filepath.Join(f.svc.Layout().ExportsDir, file.Path)
		}
	}
	before, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	var partialName string
	for attempt := range 2 {
		partial := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[1],"onExists":"keep","subtitles":true}`)
		if partial.Code != http.StatusAccepted {
			t.Fatalf("partial keep: %d %s", partial.Code, partial.Body.String())
		}
		result := waitAudioExportState(t, f.svc, audio.JobDone)
		var name string
		for _, file := range result.Result.Files {
			if strings.HasSuffix(file.Name, ".m4b") {
				if name != "" {
					t.Fatal("partial response must list only files from this export")
				}
				name = file.Name
			}
		}
		if !strings.Contains(name, "-ch1") {
			t.Fatalf("partial file collides with full volume: %q", name)
		}
		if attempt == 1 && name == partialName {
			t.Fatal("onExists keep replaced partial export")
		}
		partialName = name
	}
	after, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("partial export overwrote full-volume audio")
	}
	if _, err := os.Stat(filepath.Join(f.svc.Layout().VolumeExportDir(1), "Tập 1.srt")); err != nil {
		t.Fatalf("partial export removed full-volume subtitles: %v", err)
	}
	info, err := f.svc.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range info.Files {
		if strings.HasSuffix(file.Name, ".m4b") && file.Name != partialName {
			t.Fatalf("latest export info includes a different volume artifact: %s", file.Name)
		}
	}
	fail := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[1],"onExists":"fail"}`)
	if fail.Code != 409 {
		t.Fatalf("existing partial with onExists fail: %d %s", fail.Code, fail.Body.String())
	}
}

func TestAudioPartialWhenReadyWaitsOnlySelectedChapters(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.seedScript(t, 1, audio.StatusLocked)
	response := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[1],"whenReady":true,"chapterMarks":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("schedule partial export: %d %s", response.Code, response.Body.String())
	}
	if _, err := f.svc.Enqueue(1, 1); err != nil {
		t.Fatal(err)
	}
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, request := range f.svc.ExportRequests() {
				if request.State == audio.JobError {
					t.Fatal(request.Error)
				}
				if request.State == audio.JobDone {
					info, err := f.svc.ExportInfo(1)
					if err != nil {
						t.Fatal(err)
					}
					if info.ChapterCount != 1 {
						t.Fatalf("pending partial exported %d chapters", info.ChapterCount)
					}
					return
				}
			}
		case <-timeout:
			t.Fatal("partial whenReady waited for unselected unvoiced chapter 2")
		}
	}
}
