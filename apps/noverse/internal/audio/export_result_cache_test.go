package audio

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportReusesFinalResultAcrossRestartAndMetadataChanges(t *testing.T) {
	log := fakeFFmpeg(t)
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1, 2}, 2)
	source := []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}}
	opts := ExportOptions{Format: "mp3", Chapters: []int{1, 2}, Subtitles: true}
	run := func(title string) ExportResult {
		t.Helper()
		result, err := svc.Export(context.Background(), 1, title, source, opts)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := run("Khi sếp học Vibecode")
	mark := len(fakeFFmpegLog(t, log))
	second := run("Khi sếp học Vibecode")
	if calls := fakeFFmpegLog(t, log)[mark:]; len(calls) != 0 {
		t.Errorf("unchanged export reran ffmpeg (%d calls):\n%s", len(calls), strings.Join(calls, "\n"))
	}
	if first.Files[0].Path != second.Files[0].Path {
		t.Fatal("cache changed result path")
	}
	layout := svc.layout
	svc.Close()
	var err error
	svc, err = NewService(layout, NullSynth{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	mark = len(fakeFFmpegLog(t, log))
	run("Khi sếp học Vibecode")
	if calls := fakeFFmpegLog(t, log)[mark:]; len(calls) != 0 {
		t.Errorf("restart lost final result cache (%d calls)", len(calls))
	}
	mark = len(fakeFFmpegLog(t, log))
	changed := run("Tên truyện mới")
	if calls := fakeFFmpegLog(t, log)[mark:]; len(calls) != 0 {
		t.Errorf("metadata-only change rebuilt audio (%d calls):\n%s", len(calls), strings.Join(calls, "\n"))
	}
	if !strings.Contains(changed.Files[0].Name, "Tên truyện mới") {
		t.Fatalf("cached name not refreshed: %+v", changed.Files)
	}
}

func TestRetagExportPreservesEncodedAudio(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("cần ffprobe")
	}
	for _, format := range []string{"mp3", "m4b", "wav"} {
		t.Run(format, func(t *testing.T) {
			s := newTestService(t)
			s.SetStoryNames("", "Tên ngắn")
			seedExportableChapters(t, s, 1, []int{1, 2}, 1)
			source := []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}}
			opts := ExportOptions{Format: format}
			result, err := s.Export(context.Background(), 1, "Tên cũ", source, opts)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.layout.ExportsDir, result.Files[0].Path)
			probe := func(entries string) string {
				t.Helper()
				args := []string{"-v", "error", "-show_entries", entries, "-show_data_hash", "sha256", "-of", "json"}
				if entries == "packet=data_hash" {
					args = append(args, "-select_streams", "a")
				}
				out, err := exec.Command("ffprobe", append(args, path)...).CombinedOutput()
				if err != nil {
					t.Fatalf("ffprobe: %v %s", err, out)
				}
				return string(out)
			}
			before := probe("packet=data_hash")
			result, err = s.Export(context.Background(), 1, "Khi sếp học Vibecode", source, opts)
			if err != nil {
				t.Fatal(err)
			}
			for i, chapter := range result.ChapterFiles {
				if chapter.Chapter != i+1 || chapter.DurationSec <= 0 {
					t.Fatalf("metadata-only update lost chapter identity: %+v", result.ChapterFiles)
				}
			}
			if after := probe("packet=data_hash"); before != after {
				t.Fatal("metadata-only export changed encoded audio packets")
			}
			tags := probe("format_tags:chapter_tags:stream=codec_name,width,height:stream_disposition=attached_pic")
			if strings.Contains(tags, `"composer"`) {
				t.Errorf("export invents an author/composer absent from the A7 contract: %s", tags)
			}
			for _, want := range []string{"Khi sếp học Vibecode · Tập 1", "Noverse Audio", "Chương 2 · Hai"} {
				if !strings.Contains(tags, want) {
					t.Errorf("missing %q: %s", want, tags)
				}
			}
			// Deleting a result must invalidate it, even when input has not changed.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Export(context.Background(), 1, "Khi sếp học Vibecode", source, opts); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExportCaseOnlyTitleChangeKeepsAudio(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	seedExportableChapters(t, s, 1, []int{1}, 1)
	source := []SourceChapter{{Chapter: 1, Title: "Một"}}
	opts := ExportOptions{Format: "mp3"}
	first, err := s.Export(context.Background(), 1, "KHI SẾP HỌC VIBECODE", source, opts)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(s.layout.ExportsDir, first.Files[0].Path)
	newPath := filepath.Join(filepath.Dir(oldPath), "Khi sếp học Vibecode · Tập 1.mp3")
	before, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	// On case-sensitive CI create the same inode alias that APFS supplies for a
	// case-only destination. This exercises the safety branch on both systems.
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		if err := os.Link(oldPath, newPath); err != nil {
			t.Fatal(err)
		}
	}
	second, err := s.Export(context.Background(), 1, "Khi sếp học Vibecode", source, opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.Files[0].Name != "Khi sếp học Vibecode · Tập 1.mp3" {
		t.Fatalf("name = %q", second.Files[0].Name)
	}
	after, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(stripID3(before)) == 0 || !bytes.Equal(stripID3(before), stripID3(after)) {
		t.Fatal("case-only metadata update truncated or changed audio")
	}
	entries, err := os.ReadDir(filepath.Dir(newPath))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("left rename temporary: %s", entry.Name())
		}
		if entry.Name() == filepath.Base(newPath) {
			found = true
		}
	}
	if !found {
		t.Fatal("physical filename casing was not updated")
	}
	info, err := s.ExportInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Files) == 0 || info.Files[0].Name != filepath.Base(newPath) {
		t.Fatalf("download listing lost renamed audio: %+v", info.Files)
	}
}

func TestTagFailureKeepsPriorPublishedExport(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := os.WriteFile(source, testWAV(48000, 1, 48000, 8000), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "existing.mp3")
	original := []byte("previously published export")
	if err := os.WriteFile(destination, original, 0o644); err != nil {
		t.Fatal(err)
	}
	err := encodeAudioTagged(context.Background(), source, destination, "mp3", false, tagSpec{tags: tagSet{Title: "Title"}, coverPNG: filepath.Join(dir, "missing.png")}, "")
	if err == nil {
		t.Fatal("missing cover did not fail tag write")
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("failed metadata write replaced the published export")
	}
}

func TestCaseOnlyFilenameChangeWhileReencoding(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	seedExportableChapters(t, s, 1, []int{1}, 1)
	source := []SourceChapter{{Chapter: 1, Title: "Một"}}
	opts := ExportOptions{Format: "mp3"}
	if _, err := s.Export(context.Background(), 1, "KHI SẾP HỌC VIBECODE", source, opts); err != nil {
		t.Fatal(err)
	}
	settings := s.Settings()
	settings.Quality = QualityHigh
	if _, err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	result, err := s.Export(context.Background(), 1, "Khi sếp học Vibecode", source, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := "Khi sếp học Vibecode · Tập 1.mp3"
	found := false
	for _, file := range result.Files {
		if file.Name == want && file.Format == "audio" {
			found = true
		}
	}
	if !found {
		t.Fatalf("case-only replacement disappeared from listing: %+v", result.Files)
	}
	entries, err := os.ReadDir(s.layout.VolumeExportDir(1))
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, entry := range entries {
		if entry.Name() == want {
			found = true
		}
	}
	if !found {
		t.Fatal("atomic replacement retained old physical filename casing")
	}
}
