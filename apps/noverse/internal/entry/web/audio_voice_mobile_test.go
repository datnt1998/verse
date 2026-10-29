package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datnt1998/noverse/internal/audio"
)

func TestMobileVoiceRoleQueuesOnlyOldRole(t *testing.T) {
	f := newAudioFixture(t)
	f.svc.Close()
	f.seedScript(t, 1, audio.StatusLocked)
	script, err := audio.ReadScript(f.svc.Layout(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range script.Segments {
		script.Segments[i].Rendered = &audio.SegmentAudio{Key: "old", VoiceID: "old", DurationSec: 2}
	}
	for i := range 6 {
		seg := script.Segments[1]
		seg.ID = fmt.Sprintf("role-%d", i)
		script.Segments = append(script.Segments, seg)
	}
	if err := audio.WriteScript(f.svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
	w := f.do("PUT", "/audio/cast/lth", `{"voiceId":"giong-nu","rerenderOld":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("role change: want 200, got %d %s", w.Code, w.Body.String())
	}
	var result struct {
		QueuedSegments int `json:"queuedSegments"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.QueuedSegments != 7 {
		t.Fatalf("queued old role segments = %d, want 7", result.QueuedSegments)
	}
	jobs := f.svc.Queue()
	if len(jobs) != 1 {
		t.Fatalf("queued jobs = %+v", jobs)
	}
	if jobs[0].Segments != 7 {
		t.Fatalf("role queue segments = %d, want 7", jobs[0].Segments)
	}
	encoded, _ := json.Marshal(jobs[0])
	var job struct {
		Roles []string `json:"rerenderRoles"`
	}
	_ = json.Unmarshal(encoded, &job)
	if len(job.Roles) != 1 || job.Roles[0] != "lth" {
		t.Fatalf("selective role job = %s", encoded)
	}
	cast, _ := audio.ReadCast(f.svc.Layout())
	speaker, _ := cast.SpeakerByID("lth")
	if speaker.VoiceID == nil || *speaker.VoiceID != "giong-nu" {
		t.Fatalf("saved role = %+v", speaker)
	}
}

func TestMobileVoiceUploadValidation(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("cần bộ xử lý âm thanh")
	}
	f := newAudioFixture(t)
	for _, tc := range []struct {
		name    string
		seconds float64
		status  int
	}{{"clip.ogg", 12, 400}, {"clip.wav", 9, 400}, {"clip.wav", 31, 400}, {"clip.wav", 10, 200}, {"clip.wav", 30, 200}, {"clip.m4a", 12, 200}, {"clip.mp3", 12, 200}} {
		t.Run(fmt.Sprintf("%s-%gs", tc.name, tc.seconds), func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			part, _ := form.CreateFormFile("file", tc.name)
			_, _ = part.Write(sineWAV(8000, tc.seconds, 440))
			_ = form.WriteField("name", "Mẫu của tôi")
			_ = form.Close()
			req := httptest.NewRequest("POST", "/api/projects/proj/voices", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.admin})
			w := httptest.NewRecorder()
			f.tg.g.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("upload %s %.0fs: want %d, got %d %s", tc.name, tc.seconds, tc.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestMobileVoicesIncludeShortSamples(t *testing.T) {
	f := newAudioFixture(t)
	w := f.do("GET", "/audio/voices", "")
	if w.Code != 200 {
		t.Fatalf("voices: want 200, got %d %s", w.Code, w.Body.String())
	}
	var voices []struct {
		ID        string `json:"id"`
		SampleURL string `json:"sampleUrl"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &voices); err != nil {
		t.Fatal(err)
	}
	if len(voices) == 0 || voices[0].SampleURL == "" {
		t.Fatalf("missing samples: %s", w.Body.String())
	}
	if audio.FFmpegAvailable() {
		sample := f.tg.do("GET", voices[0].SampleURL, f.admin, "")
		if sample.Code != 200 {
			t.Fatalf("sample: %d %s", sample.Code, sample.Body.String())
		}
		if duration := audio.WAVDurationSec(sample.Body.Bytes()); duration <= 0 || duration > 5.01 {
			t.Fatalf("sample duration = %g", duration)
		}
	}
}
