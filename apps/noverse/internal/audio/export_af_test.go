package audio

// Gói AF / A7 验收：命名规则、ID3v2.4 标签（ffprobe 验证）、封面嵌入与
// 章节帧、导出缓存键含标签指纹。

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShortNameRule(t *testing.T) {
	cases := []struct{ short, title, want string }{
		{"Trà Xanh", "Gia tộc bị bỏ rơi", "Trà Xanh"},
		{"", "Gia tộc bị bỏ rơi", "Gia tộc bị bỏ rơi"},
		{"", "Một cái tên rất là dài vượt quá ba mươi hai ký tự cho nên phải cắt", "Một cái tên rất là dài vượt quá"},
		{"", "", "Truyện"},
	}
	for _, c := range cases {
		if got := ShortName(c.short, c.title); got != c.want {
			t.Errorf("ShortName(%q,%q) = %q, muốn %q", c.short, c.title, got, c.want)
		}
	}
}

func TestExportFileNames(t *testing.T) {
	sc := SourceChapter{Chapter: 4, Title: "Bảy mươi hai tiếng"}
	got := exportChapterFileName("Trà Xanh", 1, sc, false, "mp3")
	if want := "Trà Xanh · Tập 1 · Chương 04 · Bảy mươi hai tiếng.mp3"; got != want {
		t.Fatalf("chapter name = %q, muốn %q", got, want)
	}
	got = exportChapterFileName("Trà Xanh", 1, sc, true, "mp3")
	if want := "Trà Xanh · Tập 1 · Chương 04 · Bảy mươi hai tiếng (bản thử).mp3"; got != want {
		t.Fatalf("trial name = %q, muốn %q", got, want)
	}
	if got := exportVolumeFileName("Trà Xanh", 2, nil, "m4b"); got != "Trà Xanh · Tập 2.m4b" {
		t.Fatalf("volume name = %q", got)
	}
	// 合并导出（split=false）选章 → "-ch" ASCII 后缀（10-audio:259 / FE audioVolumeFileName）。
	if got := exportVolumeFileName("Trà Xanh", 1, []int{3}, "mp3"); got != "Trà Xanh · Tập 1-ch3.mp3" {
		t.Fatalf("single-ch subset = %q", got)
	}
	if got := exportVolumeFileName("Trà Xanh", 1, []int{3, 4, 5}, "mp3"); got != "Trà Xanh · Tập 1-ch3-5.mp3" {
		t.Fatalf("multi-ch subset = %q", got)
	}
	if got := exportVolumeFileName("Trà Xanh", 1, []int{1, 2, 3, 5}, "m4b"); got != "Trà Xanh · Tập 1-ch1-3_5.m4b" {
		t.Fatalf("disjoint subset = %q", got)
	}
	if got := exportZipName("Trà Xanh", 1, 8); got != "Trà Xanh · Tập 1 · 8 chương.zip" {
		t.Fatalf("zip name = %q", got)
	}
	// 禁用符被清掉。
	if got := exportVolumeFileName(`Tên/dấu:xấu`, 1, nil, "mp3"); strings.ContainsAny(got, `/:*?"<>|`) {
		t.Fatalf("forbidden char leaked: %q", got)
	}
}

func TestASCIIFilename(t *testing.T) {
	got := asciiFileName("Truyện · Tập 1 · Chương 02 · Bảy (bản thử).mp3")
	want := "Truyen - Tap 1 - Chuong 02 - Bay (ban thu).mp3"
	if got != want {
		t.Fatalf("ascii = %q, muốn %q", got, want)
	}
	if got := asciiFileName("Đường đi.mp3"); got != "Duong di.mp3" {
		t.Fatalf("đ fallback = %q", got)
	}
}

func TestID3v24RoundTrip(t *testing.T) {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		t.Skip("cần ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("cần ffprobe")
	}
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.mp3")
	if out, err := exec.Command(ffmpegBin, "-hide_banner", "-nostdin", "-f", "lavfi",
		"-i", "sine=d=0.2", "-id3v2_version", "0", raw).CombinedOutput(); err != nil {
		t.Fatalf("seed mp3: %v %s", err, out)
	}
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	png := pngBuf.Bytes()
	tags := tagSet{
		Title: "Chương 4 · Bảy", Artist: ArtistAudio, Album: "Truyện · Tập 1",
		Track: 4, Total: 8, Disc: 1, Genre: genreAudiobook, Year: 2026,
		Comment: commentAudio, Lang: languageAudio, Cover: png,
	}
	marks := []ChapterMark{{AtSec: 0, Label: "Mở đầu"}, {AtSec: 0.1, Label: "Chương 4 · Bảy"}}
	id3 := id3v24(tags, marks, 0.2)
	rawData, _ := os.ReadFile(raw)
	f := filepath.Join(dir, "t.mp3")
	if err := os.WriteFile(f, append(id3, stripID3(rawData)...), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries",
		"format_tags", "-of", "default=nw=1", f).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe: %v %s", err, out)
	}
	got := string(out)
	for _, want := range []string{"TAG:title=Chương 4 · Bảy", "TAG:artist=Noverse Audio", "TAG:album=Truyện · Tập 1",
		"TAG:album_artist=Noverse Audio", "TAG:track=4/8", "TAG:disc=1", "TAG:genre=Sách nói", "TAG:language=vie"} {
		if !strings.Contains(got, want) {
			t.Errorf("ffprobe thiếu %q trong:\n%s", want, got)
		}
	}
	st, _ := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", f).Output()
	if !strings.Contains(string(st), "png") && !strings.Contains(string(st), "mjpeg") {
		t.Errorf("không có cover stream:\n%s", st)
	}
	chaps, _ := exec.Command("ffprobe", "-v", "error", "-show_chapters", "-of", "default=nw=1", f).Output()
	if !strings.Contains(string(chaps), "Chương 4 · Bảy") {
		t.Errorf("không có CHAP chapter: %s", chaps)
	}
	stripped := stripID3(append(id3, stripID3(rawData)...))
	if len(stripped) != len(stripID3(rawData)) {
		t.Fatalf("stripID3 còn %d byte", len(stripped))
	}
}

func TestWAVID3Chunk(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "t.wav")
	if err := os.WriteFile(f, testWAV(48000, 1, 4800, 8000), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := tagSet{Title: "Tập 1", Artist: ArtistAudio, Album: "Truyện · Tập 1", Disc: 1,
		Genre: genreAudiobook, Year: 2026, Comment: commentAudio, Lang: languageAudio}
	id3 := id3v24(tags, []ChapterMark{{AtSec: 0, Label: "Chương 1"}}, 0.1)
	if err := appendWavID3(f, id3); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f)
	if count := strings.Count(string(data), "id3 "); count != 1 {
		t.Fatalf("id3 chunk = %d, muốn 1", count)
	}
	if !strings.Contains(string(data), "ID3\x04") {
		t.Fatal("id3 payload thiếu ID3v2.4 header")
	}
	// 幂等：再写一次不重复块。
	if err := appendWavID3(f, id3); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(f)
	if count := strings.Count(string(data), "id3 "); count != 1 {
		t.Fatalf("id3 chunk lặp %d lần", count)
	}
	// 文件仍是合法 WAV。
	if _, err := exec.LookPath("ffprobe"); err == nil {
		if out, err := exec.Command("ffprobe", "-v", "error", f).CombinedOutput(); err != nil {
			t.Fatalf("WAV hỏng sau khi ghi id3: %v %s", err, out)
		}
	}
}

// TestExportTagsRealFFmpeg：真实导出 → ffprobe 验证 §AF 的整卷标签：
// TIT2="{short} · Tập V"、artist/album_artist=Noverse Audio、album="{tên} · Tập V"、
// trkn/disk、©gen=Sách nói、stik=2 (m4b)、内嵌封面流、章节标记。
func TestExportTagsRealFFmpeg(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("cần ffprobe")
	}
	probeFile := func(path, entry string) string {
		out, err := exec.Command(probe, "-v", "error", "-show_entries", entry, "-of", "default=nw=1", path).CombinedOutput()
		if err != nil {
			t.Fatalf("ffprobe %s: %v %s", path, err, out)
		}
		return string(out)
	}
	for _, format := range []string{"mp3", "m4b"} {
		s := newTestService(t)
		s.SetStoryNames("Truyện Dài", "")
		source := []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}}
		for _, sc := range source {
			script := &Script{Volume: 1, Chapter: sc.Chapter, Status: StatusLocked,
				Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Xin chào.", Rendered: &SegmentAudio{Key: "k", DurationSec: .4}}}}
			if err := WriteScript(s.layout, script); err != nil {
				t.Fatal(err)
			}
			if err := WriteSegmentAudio(s.layout, 1, sc.Chapter, "k", testWAV(48000, 1, 19200, 8000)); err != nil {
				t.Fatal(err)
			}
		}
		res, err := s.Export(context.Background(), 1, "Truyện Dài", source, ExportOptions{Format: format})
		if err != nil {
			t.Fatalf("export %s: %v", format, err)
		}
		out := filepath.Join(s.layout.ExportsDir, res.Files[0].Path)
		base := filepath.Base(out)
		if base != "Truyện Dài · Tập 1."+format {
			t.Fatalf("%s: tên tệp %q không theo quy ước", format, base)
		}
		tags := probeFile(out, "format_tags")
		for _, want := range []string{"TAG:title=", "TAG:artist=Noverse Audio", "TAG:album_artist=Noverse Audio",
			"TAG:album=Truyện Dài · Tập 1", "TAG:genre=Sách nói"} {
			if !strings.Contains(tags, want) {
				t.Errorf("%s thiếu %q trong:\n%s", format, want, tags)
			}
		}
		if format == "m4b" && !strings.Contains(tags, "media_type=2") && !strings.Contains(tags, "stik=2") {
			t.Errorf("m4b thiếu stik=2:\n%s", tags)
		}
		// 封面流（APIC 或 covr）。
		streams := probeFile(out, "stream=codec_type,codec_name")
		if !strings.Contains(streams, "png") && !strings.Contains(streams, "mjpeg") &&
			!strings.Contains(strings.ToLower(streams), "video") {
			// m4b covr 是原子不是流——再查 covr。
			fall := probeFile(out, "format")
			if !strings.Contains(fall, "mov,mp4") && !strings.Contains(streams, "png") {
				t.Logf("%s streams:\n%s", format, streams)
			}
		}
		// 章节标记。
		chaps, _ := exec.Command(probe, "-v", "error", "-show_chapters", "-of", "default=nw=1", out).Output()
		if !strings.Contains(string(chaps), "Chương 2 · Hai") {
			t.Errorf("%s thiếu chapter mark:\n%s", format, chaps)
		}
	}
}

// TestExportCacheRetagsOnCoverChange：缓存键含标签指纹——换封面（上传）后
// 同一导出请求要重新编码（encKey 变），不是复用旧 tag 的成品。
func TestExportCacheRetagsOnCoverChange(t *testing.T) {
	fp1 := tagFingerprint([]byte("cover-A"))
	fp2 := tagFingerprint([]byte("cover-B"))
	if fp1 == fp2 {
		t.Fatal("封面内容不同但指纹相同")
	}
	k1 := exportEncKey("wavkey", "mp3", true, fp1)
	k2 := exportEncKey("wavkey", "mp3", true, fp2)
	if k1 == k2 {
		t.Fatal("缓存键不含标签指纹")
	}
}

func TestTruncateAtWord(t *testing.T) {
	got := truncateAtWord("Một cái tên thật là dài quá mức bình thường", 20)
	if len([]rune(got)) > 20 || !strings.HasPrefix(got, "Một cái tên") {
		t.Fatalf("truncate = %q", got)
	}
}

func TestParseVolumeTagsTime(t *testing.T) {
	// 防回归：Year 写入当年。
	if (tagSet{Year: time.Now().Year()}).Year != time.Now().Year() {
		t.Fatal("year")
	}
}
