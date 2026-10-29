package audio

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportNamingRegression(t *testing.T) {
	for _, tc := range []struct{ name, short, title, want string }{
		{"real title", "", "Khi sếp học Vibecode", "Khi sếp học Vibecode"},
		{"import prefix", "", "RULES KỊCH BẢN - “Khi sếp học Vibecode”", "Khi sếp học Vibecode"},
		{"import colon", "", "RULES KỊCH BẢN: “KHI SẾP HỌC VIBECODE”", "KHI SẾP HỌC VIBECODE"},
		{"unicode whitespace and dash", "", "RULES KỊCH\u00a0BẢN — ‘Khi sếp học Vibecode’", "Khi sếp học Vibecode"},
		{"saved short wins", "“Tên riêng”", "Tên truyện", "Tên riêng"},
		{"quotes", "", "\"Khi sếp học Vibecode\"", "Khi sếp học Vibecode"},
		{"dashes", "", "Khi sếp – học Vibecode", "Khi sếp – học Vibecode"},
		{"long", "", "Khi sếp học Vibecode và cả công ty cùng học lại từ đầu", "Khi sếp học Vibecode và cả công"},
		{"exact boundary", "", "12345678901234567890123456789 ab tiếp", "12345678901234567890123456789 ab"},
		{"no half quote", "", "Một truyện “có tên dài hơn ba mươi hai ký tự”", "Một truyện"},
		{"no half word", "", strings.Repeat("a", 40), "Truyện"},
		{"empty", "", "", "Truyện"},
		{"illegal", "", "Khi/sếp:học Vibecode", "Khi sếp học Vibecode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ShortName(tc.short, tc.title)
			if got != tc.want {
				t.Fatalf("shortTitle = %q, want %q", got, tc.want)
			}
			for _, file := range []struct{ got, want string }{
				{exportVolumeFileName(got, 1, nil, "mp3"), tc.want + " · Tập 1.mp3"},
				{exportVolumeFileName(got, 1, []int{1, 2, 3, 4, 5, 6, 7, 8}, "mp3"), tc.want + " · Tập 1-ch1-8.mp3"},
				{exportChapterFileName(got, 1, SourceChapter{Chapter: 4, Title: "Ai/nói: thế?"}, false, "mp3"), tc.want + " · Tập 1 · Chương 04 · Ai nói thế.mp3"},
			} {
				if file.got != file.want {
					t.Errorf("filename = %q, want %q", file.got, file.want)
				}
			}
		})
	}
}

func TestExportCanonicalTitlePriority(t *testing.T) {
	s := newTestService(t)
	s.SetStoryNames("RULES KỊCH BẢN: “KHI SẾP HỌC VIBECODE”", "")
	title, short := s.storyMeta("Khi sếp học Vibecode")
	if title != "Khi sếp học Vibecode" || short != title {
		t.Fatalf("storyMeta = (%q, %q), want canonical book title", title, short)
	}
	s.SetStoryNames("RULES KỊCH BẢN - “Khi sếp học Vibecode”", "Tên riêng")
	title, short = s.storyMeta("")
	if title != "Khi sếp học Vibecode" || short != "Tên riêng" {
		t.Fatalf("fallback storyMeta = (%q, %q)", title, short)
	}
}

func TestExportSelectedChapterTagsKeepVolumeNumbering(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("cần ffprobe")
	}
	s := newTestService(t)
	s.SetStoryNames("RULES KỊCH BẢN: “Khi sếp học Vibecode”", "Tên ngắn")
	source := []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}, {Chapter: 3, Title: "Ba"}}
	seedExportableChapters(t, s, 1, []int{1, 2, 3}, 1)
	opts := ExportOptions{Format: "mp3", Chapters: []int{2}, Split: true}
	probe := func(path string) string {
		t.Helper()
		out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags", "-of", "default=nw=1", path).CombinedOutput()
		if err != nil {
			t.Fatalf("ffprobe: %v %s", err, out)
		}
		return string(out)
	}
	for _, title := range []string{"Khi sếp học Vibecode", "Tên truyện mới"} {
		result, err := s.Export(context.Background(), 1, title, source, opts)
		if err != nil {
			t.Fatal(err)
		}
		tags := probe(filepath.Join(s.layout.ExportsDir, result.ChapterFiles[0].Path))
		for _, want := range []string{"TAG:title=Chương 2 · Hai", "TAG:album=" + title + " · Tập 1", "TAG:artist=Noverse Audio", "TAG:album_artist=Noverse Audio", "TAG:track=2/3", "TAG:disc=1"} {
			if !strings.Contains(tags, want) {
				t.Errorf("missing %q:\n%s", want, tags)
			}
		}
	}
}
