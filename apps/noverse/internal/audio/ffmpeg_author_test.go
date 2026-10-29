package audio

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Tệp sách nói xuất ra phải ghi tác giả là Noverse ở mọi định dạng.
func TestEncodeAudioTagsNoverseAuthor(t *testing.T) {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		t.Skip("cần ffmpeg")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("cần ffprobe")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "in.wav")
	if out, err := exec.Command(ffmpegBin, "-hide_banner", "-nostdin", "-f", "lavfi", "-i", "sine=d=1", in).CombinedOutput(); err != nil {
		t.Fatalf("tạo wav: %v %s", err, out)
	}
	for _, format := range []string{"mp3", "m4b"} {
		out := filepath.Join(dir, "out."+format)
		// Gói AF: artist = "Noverse Audio"（不再写 "Noverse"）。
		spec := tagSpec{tags: tagSet{Title: "Chương 1 · Một", Artist: ArtistAudio,
			Album: "Truyện · Tập 1", Track: 1, Total: 4, Disc: 1,
			Genre: genreAudiobook, Year: 2026, Comment: commentAudio, Lang: languageAudio}}
		if err := encodeAudioTagged(context.Background(), in, out, format, false, spec, ""); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		tags, err := exec.Command(probe, "-v", "error", "-show_entries", "format_tags=artist,album_artist,title", "-of", "default=nw=1", out).Output()
		if err != nil {
			t.Fatalf("ffprobe %s: %v", format, err)
		}
		got := strings.ToLower(string(tags))
		for _, want := range []string{"artist=noverse audio", "album_artist=noverse audio", "title=chương 1"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s thiếu %q, có: %s", format, want, tags)
			}
		}
	}
}
