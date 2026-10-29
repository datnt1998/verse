package audio

import (
	"archive/zip"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportOptionsAppliedAB(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	synth := &stoppedSynth{}
	s.synth = synth
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	source := []SourceChapter{{Chapter: 2, Title: "Hai"}, {Chapter: 4, Title: "Bốn"}}
	for _, chapter := range source {
		script := &Script{Volume: 1, Chapter: chapter.Chapter, Status: StatusLocked, Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Xin chào.", Rendered: &SegmentAudio{Key: "ready", DurationSec: .5}}}}
		if err := WriteScript(s.layout, script); err != nil {
			t.Fatal(err)
		}
		if err := WriteSegmentAudio(s.layout, 1, chapter.Chapter, "ready", testWAV(48000, 1, 24000, 8000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(s.layout.MusicDir(), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"intro.wav", "outro.wav"} {
		if err := os.WriteFile(filepath.Join(s.layout.MusicDir(), name), testWAV(48000, 1, 24000, 8000), 0600); err != nil {
			t.Fatal(err)
		}
	}
	full, err := s.Export(context.Background(), 1, "Truyện", source, ExportOptions{Format: "m4b", Intro: true, ChapterTitles: true, Loudness: true, GapSec: 2, ChapterMarks: true, Subtitles: true})
	if err != nil {
		t.Fatal(err)
	}
	// 2 x .5s source + 2 x .5s titles + 2 x .5s music + exactly one 2s gap.
	if full.DurationSec < 4.99 || full.DurationSec > 5.01 || synth.calls != 2 || len(full.ChapterMarks) != 3 {
		t.Fatalf("full options not applied: duration=%v titles=%d marks=%v", full.DurationSec, synth.calls, full.ChapterMarks)
	}
	hasSRT := false
	for _, file := range full.Files {
		if strings.HasSuffix(file.Path, ".srt") {
			hasSRT = true
		}
	}
	if !hasSRT || len(full.ChapterFiles) != 2 {
		t.Fatalf("missing subtitles/chapters: %+v", full)
	}
	split, err := s.Export(context.Background(), 1, "Truyện", source, ExportOptions{Format: "wav", Split: true, Pack: "zip", Chapters: []int{2, 4}, Intro: true, GapSec: 9, ChapterMarks: true, Subtitles: true})
	if err != nil {
		t.Fatal(err)
	}
	if split.DurationSec < .99 || split.DurationSec > 1.01 || len(split.ChapterMarks) != 0 || len(split.ChapterFiles) != 2 || !split.Split || split.ZipURL == "" {
		t.Fatalf("split ignored hidden-option rules: %+v", split)
	}
	for i, file := range split.ChapterFiles {
		if file.Chapter != source[i].Chapter || !strings.HasSuffix(file.Name, ".wav") || file.DurationSec != .5 {
			t.Fatalf("chapter file: %+v", file)
		}
	}
	archiveURL, err := url.Parse(split.ZipURL)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(filepath.Join(s.layout.ExportsDir, archiveURL.Query().Get("path")))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 4 {
		t.Fatalf("zip entries=%d, expected 2 WAV + 2 SRT", len(archive.File))
	}
	for _, file := range archive.File {
		if !strings.Contains(file.Name, "Chương 02") && !strings.Contains(file.Name, "Chương 04") {
			t.Fatalf("unselected zip entry: %s", file.Name)
		}
	}
}
