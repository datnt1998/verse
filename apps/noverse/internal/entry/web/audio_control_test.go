package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

func waitAudioExportState(t *testing.T, svc *audio.Service, state string) audio.ExportRequest {
	t.Helper()
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, request := range svc.ExportRequests() {
				if request.State == state {
					return request
				}
			}
		case <-deadline:
			t.Fatalf("export never reached %s: %+v", state, svc.ExportRequests())
			return audio.ExportRequest{}
		}
	}
}

func TestAudioExportProviderCancelsWaitingRequest(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	response := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","whenReady":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("schedule: %d %s", response.Code, response.Body.String())
	}
	// 同卷已有等待中的导出 → 再 POST 必须 §AE 409 + jobId。
	conflict := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","whenReady":true}`)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("concurrent export must be 409: %d %s", conflict.Code, conflict.Body.String())
	}
	conflictBody := decodeJSONBody(t, conflict)
	if conflictBody["jobId"] != "proj/01-tra-xanh/audio/1/export" || conflictBody["error"] == "" {
		t.Fatalf("409 must carry jobId+message: %s", conflict.Body.String())
	}
	if err := newAudioJobsProvider(nil).Control(context.Background(), "proj/01-tra-xanh/audio/1/export", "cancel"); err != nil {
		t.Fatalf("export provider cancel: %v", err)
	}
	waitAudioExportState(t, f.svc, audio.JobCanceled)
	for _, row := range audioRows(audioEntry{svc: f.svc, project: "proj", story: "01-tra-xanh"}, time.Now(), true) {
		if strings.HasSuffix(row.ID, "/export") {
			t.Fatalf("canceled export still visible: %+v", row)
		}
	}
	data, err := os.ReadFile(filepath.Join(f.svc.Layout().AudioDir, "export-requests.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[int]audio.ExportRequest
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted[1].State != audio.JobCanceled {
		t.Fatalf("cancel not persisted: %s", data)
	}
}

func TestAudioExportProviderRetriesFailedSelection(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.seedScript(t, 1, audio.StatusLocked)
	if _, err := f.svc.Enqueue(1, 1); err != nil {
		t.Fatal(err)
	}
	if j := f.waitJob(t, 1, audio.JobDone, audio.JobError); j.State != audio.JobDone {
		t.Fatal(j.Error)
	}
	response := f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[1]}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("initial export: %d %s", response.Code, response.Body.String())
	}
	waitAudioExportState(t, f.svc, audio.JobDone)
	info, err := f.svc.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	var existing string
	for _, file := range info.Files {
		if strings.HasSuffix(file.Name, ".m4b") {
			existing = filepath.Join(f.svc.Layout().ExportsDir, file.Path)
		}
	}
	response = f.do("POST", "/audio/volumes/1/exports", `{"format":"m4b","chapters":[1],"whenReady":true,"onExists":"fail"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("schedule collision: %d %s", response.Code, response.Body.String())
	}
	waitAudioExportState(t, f.svc, audio.JobError)
	if err := os.Remove(existing); err != nil {
		t.Fatal(err)
	}
	if err := newAudioJobsProvider(nil).Control(context.Background(), "proj/01-tra-xanh/audio/1/export", "retry"); err != nil {
		t.Fatalf("export provider retry: %v", err)
	}
	request := waitAudioExportState(t, f.svc, audio.JobDone)
	if len(request.Options.Chapters) != 1 || request.Options.Chapters[0] != 1 {
		t.Fatalf("retry lost selected chapters: %+v", request.Options)
	}
	if _, err := os.Stat(existing); err != nil {
		t.Fatalf("retry did not recreate output: %v", err)
	}
}

// §AE "Ghép không nhạc": lỗi intro_missing → POST /api/jobs/<id>/retry với
// {"intro":false} phải qua HTTP xuống ControlBody và chạy xong không cần nhạc.
func TestAudioExportRetryWithoutMusicViaHTTP(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.seedScript(t, 1, audio.StatusLocked)
	if _, err := f.svc.Enqueue(1, 1); err != nil {
		t.Fatal(err)
	}
	if j := f.waitJob(t, 1, audio.JobDone, audio.JobError); j.State != audio.JobDone {
		t.Fatal(j.Error)
	}
	// Không có nhạc trong thư mục nhạc → intro:true 必然 intro_missing.
	if w := f.do("POST", "/audio/volumes/1/exports", `{"format":"mp3","chapters":[1],"intro":true,"chapterTitles":false}`); w.Code != http.StatusAccepted {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	failed := waitAudioExportState(t, f.svc, audio.JobError)
	if failed.ErrorCause != "intro_missing" || !failed.Retryable {
		t.Fatalf("want intro_missing: %+v", failed)
	}
	if w := f.tg.do("POST", "/api/jobs/proj/01-tra-xanh/audio/1/export/retry", f.admin, `{"intro":false}`); w.Code != http.StatusOK {
		t.Fatalf("retry without music: %d %s", w.Code, w.Body.String())
	}
	done := waitAudioExportState(t, f.svc, audio.JobDone)
	if done.Options.Intro {
		t.Fatalf("retry kept intro music: %+v", done)
	}
}
