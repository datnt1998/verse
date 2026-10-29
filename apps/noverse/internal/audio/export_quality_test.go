package audio

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportQualitySelectsEncodingAndInvalidatesResult(t *testing.T) {
	log := fakeFFmpeg(t)
	s := newTestService(t)
	seedExportableChapters(t, s, 1, []int{1}, 1)
	source := []SourceChapter{{Chapter: 1, Title: "Một"}}
	opts := ExportOptions{Format: "mp3"}
	if _, err := s.Export(context.Background(), 1, "Truyện", source, opts); err != nil {
		t.Fatal(err)
	}
	mark := len(fakeFFmpegLog(t, log))
	settings := s.Settings()
	settings.Quality = QualityHigh
	if _, err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Export(context.Background(), 1, "Truyện", source, opts); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(fakeFFmpegLog(t, log)[mark:], "\n")
	if !strings.Contains(calls, "-ar 48000 -c:a libmp3lame -b:a 192k") {
		t.Fatalf("high quality must not return a standard cached encode; ffmpeg calls:\n%s", calls)
	}
	if strings.Contains(calls, "-s000") {
		t.Fatalf("encoding quality rebuilt source PCM:\n%s", calls)
	}
}

func TestHighQualityContainerParameters(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("cần ffprobe")
	}
	for _, format := range []string{"mp3", "m4b", "wav"} {
		t.Run(format, func(t *testing.T) {
			s := newTestService(t)
			seedExportableChapters(t, s, 1, []int{1}, 1)
			settings := s.Settings()
			settings.Quality = QualityHigh
			if _, err := s.SaveSettings(settings); err != nil {
				t.Fatal(err)
			}
			result, err := s.Export(context.Background(), 1, "Truyện", []SourceChapter{{Chapter: 1}}, ExportOptions{Format: format})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.layout.ExportsDir, result.Files[0].Path)
			out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a", "-show_entries", "stream=sample_rate,bit_rate,channels", "-of", "default=nw=1", path).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "sample_rate=48000") || !strings.Contains(string(out), "channels=1") {
				t.Fatalf("high source fidelity not preserved: %s", out)
			}
			if format == "mp3" && !strings.Contains(string(out), "bit_rate=192000") {
				t.Fatalf("high MP3 bitrate: %s", out)
			}
		})
	}
}
