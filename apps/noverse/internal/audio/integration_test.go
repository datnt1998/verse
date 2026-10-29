package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Enable against a running VieNeu OpenAI Speech endpoint. HOME and all story
// output stay in a temporary directory; this never touches the user's library.
type smokeVieNeuSynth struct {
	NullSynth
	endpoint string
}

func (s smokeVieNeuSynth) Status(context.Context) Status {
	return Status{Ready: true, State: "ready", Engine: "vieneu", ModelID: "vieneu-v3-turbo"}
}
func (s smokeVieNeuSynth) Voices(context.Context) ([]Voice, error) {
	return []Voice{{ID: "smoke", Name: "Hải Đăng", Source: "builtin"}}, nil
}
func (s smokeVieNeuSynth) Synthesize(ctx context.Context, request SynthRequest) (Audio, error) {
	data, err := json.Marshal(map[string]any{"input": request.Text, "voice": request.VoiceID, "model": "vieneu-v3-turbo", "response_format": "wav"})
	if err != nil {
		return Audio{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return Audio{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return Audio{}, err
	}
	defer response.Body.Close()
	wav, err := io.ReadAll(response.Body)
	if err != nil {
		return Audio{}, err
	}
	if response.StatusCode != 200 {
		return Audio{}, fmt.Errorf("VieNeu HTTP %d: %s", response.StatusCode, wav)
	}
	return Audio{WAV: wav, SampleRate: SampleRate, DurationSec: WAVDurationSec(wav)}, nil
}

func TestRealVieNeuVolumeOneM4B(t *testing.T) {
	endpoint := os.Getenv("AINOVEL_AUDIO_SMOKE_ENDPOINT")
	if endpoint == "" {
		t.Skip("set AINOVEL_AUDIO_SMOKE_ENDPOINT to a real VieNeu /v1/audio/speech endpoint")
	}
	if !FFmpegAvailable() {
		t.Fatal("cần ffmpeg và ffprobe")
	}
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	layout := Layout{StoryDir: dir, AudioDir: filepath.Join(dir, "audio"), AssetsDir: filepath.Join(dir, "assets"), ExportsDir: filepath.Join(dir, "exports")}
	svc, err := NewService(layout, smokeVieNeuSynth{endpoint: endpoint})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	id := "smoke"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &id}}}); err != nil {
		t.Fatal(err)
	}
	source := []SourceChapter{{Chapter: 1, Title: "Buổi sáng", Text: "Buổi sáng, ánh nắng nhẹ chiếu qua cửa sổ. Tôi mở trang sách và bắt đầu một ngày mới.", Completed: true}, {Chapter: 2, Title: "Gặp lại", Text: "Chiều hôm ấy, chúng tôi gặp lại nhau bên bờ sông. Gió thổi qua những hàng cây, mang theo mùi hương mùa thu.", Completed: true}}
	for _, chapter := range source {
		if err := WriteScript(layout, &Script{Volume: 1, Chapter: chapter.Chapter, Status: StatusLocked, SourceHash: SourceHash(chapter.Text), Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: chapter.Text}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Enqueue(1, chapter.Chapter); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		done := 0
		for _, job := range svc.Queue() {
			if job.State == JobError {
				t.Fatal(job.Error)
			}
			if job.State == JobDone {
				done++
			}
		}
		if done == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	result, err := svc.Export(ctx, 1, "Kiểm tra VieNeu", source, ExportOptions{Format: "m4b", ChapterMarks: true, Subtitles: true, Loudness: true, GapSec: .5})
	if err != nil {
		t.Fatal(err)
	}
	var file string
	for _, output := range result.Files {
		if filepath.Ext(output.Path) == ".m4b" {
			file = filepath.Join(layout.ExportsDir, output.Path)
		}
	}
	if file == "" {
		t.Fatal("missing .m4b")
	}
	probe, err := exec.CommandContext(ctx, ffprobeBin, "-v", "error", "-show_entries", "stream=codec_name,sample_rate,channels:format=duration:chapter", "-of", "json", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe: %v %s", err, probe)
	}
	t.Logf("durationSec=%.3f\nffprobe:\n%s", result.DurationSec, probe)
	var metadata struct {
		Chapters []json.RawMessage `json:"chapters"`
	}
	if err := json.Unmarshal(probe, &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Chapters) != 2 {
		t.Fatalf("expected two chapters: %s", probe)
	}
	measurement, err := exec.CommandContext(ctx, ffmpegBin, "-hide_banner", "-nostdin", "-i", file, "-af", "ebur128=peak=true", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ebur128: %v %s", err, measurement)
	}
	t.Logf("ebur128:\n%s", measurement)
}
