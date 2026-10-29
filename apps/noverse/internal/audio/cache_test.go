package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// contextWithCancel 让测试里少写一行。
func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithCancel(context.Background())
}

// testWAV 生成一段 s16 PCM：每 sample 一个方波峰值。
func testWAV(sampleRate, channels, frames int, amp int16) []byte {
	var buf bytes.Buffer
	dataLen := frames * channels * 2
	writeWAVHeader(&buf, dataLen, sampleRate, channels, 16)
	for i := range frames {
		var v int16
		if i%2 == 0 {
			v = amp
		} else {
			v = -amp
		}
		for range channels {
			_ = binary.Write(&buf, binary.LittleEndian, v)
		}
	}
	return buf.Bytes()
}

func TestWAVDurationAndPeaks(t *testing.T) {
	data := testWAV(48000, 1, 48000, 16000) // 1 秒
	info, err := ParseWAV(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.SampleRate != 48000 || info.Channels != 1 || info.BitsPerSample != 16 {
		t.Fatalf("info = %+v", info)
	}
	if got := info.DurationSec(); math.Abs(got-1) > 0.001 {
		t.Fatalf("duration = %v", got)
	}
	peaks, err := WAVPeaks(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(peaks) != 2 {
		t.Fatalf("2 điểm/giây: got %d peaks", len(peaks))
	}
	if math.Abs(peaks[0]-16000.0/32768.0) > 0.001 {
		t.Fatalf("peak = %v", peaks[0])
	}
	// 尾部不满一个窗口也要算一个点，否则波形比音频短。
	tail, err := WAVPeaks(testWAV(48000, 1, 60000, 16000), 2) // 1.25 秒
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 3 {
		t.Fatalf("1.25 s ở 2 điểm/giây: got %d peaks, want 3", len(tail))
	}
}

func TestWAVPeaksRejectsNon16Bit(t *testing.T) {
	var buf bytes.Buffer
	writeWAVHeader(&buf, 8, 8000, 1, 8)
	buf.Write(make([]byte, 8))
	if _, err := WAVPeaks(buf.Bytes(), 2); err == nil {
		t.Fatal("8-bit wav must be rejected")
	}
}

func TestSilenceWAV(t *testing.T) {
	data := silenceWAV(500, 48000)
	if got := WAVDurationSec(data); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("silence duration = %v", got)
	}
	if data := silenceWAV(0, 48000); data != nil {
		t.Fatal("0ms silence must be nil")
	}
}

func TestSegmentAudioCacheReadWrite(t *testing.T) {
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = filepath.Join(layout.StoryDir, "audio")
	data := testWAV(48000, 1, 24000, 8000)
	if err := WriteSegmentAudio(layout, 1, 3, "abc", data); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSegmentAudio(layout, 1, 3, "abc")
	if err != nil || got == nil {
		t.Fatalf("cache miss: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("cache content changed")
	}
	// 已存在的缓存不被覆盖（同一 key 同一内容）。
	if err := WriteSegmentAudio(layout, 1, 3, "abc", []byte("other")); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadSegmentAudio(layout, 1, 3, "abc"); !bytes.Equal(got, data) {
		t.Fatal("existing cache must not be overwritten")
	}
	if got, _ := ReadSegmentAudio(layout, 1, 3, "nope"); got != nil {
		t.Fatal("missing cache should be nil")
	}
	if d := cachedDuration(layout, 1, 3, "abc"); math.Abs(d-0.5) > 0.001 {
		t.Fatalf("cachedDuration = %v", d)
	}
	if err := removeSegmentAudio(layout, 1, 3, "abc"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadSegmentAudio(layout, 1, 3, "abc"); got != nil {
		t.Fatal("cache not removed")
	}
}

func TestParseWAVRejectsGarbage(t *testing.T) {
	if _, err := ParseWAV([]byte("not a wav file at all, definitely too short")); err == nil {
		t.Fatal("garbage must fail")
	}
	if _, err := ParseWAV(make([]byte, 100)); err == nil {
		t.Fatal("zero header must fail")
	}
}

func TestChapterHashTracksSegments(t *testing.T) {
	cast := &Cast{Mode: ModeCast}
	script := &Script{Segments: []Segment{
		{ID: "s0001", Speaker: "dan", PauseMs: 100, Rendered: &SegmentAudio{Key: "k1", DurationSec: 1}},
	}}
	h1 := chapterHash(script, cast)
	script.Segments[0].Rendered.Key = "k2"
	if chapterHash(script, cast) == h1 {
		t.Fatal("changing a segment key must change the chapter hash")
	}
	script.Segments[0].Rendered.Key = "k1"
	script.Segments[0].PauseMs = 900
	if chapterHash(script, cast) == h1 {
		t.Fatal("changing a pause must change the chapter hash")
	}
}

func TestEffectivePauseSingleMode(t *testing.T) {
	cast := &Cast{Mode: ModeSingle, Single: SingleCast{DialoguePauseMs: 300}}
	if got := effectivePauseMs(cast, Segment{Speaker: "lth", PauseMs: 200}); got != 500 {
		t.Fatalf("dialogue pause = %d, want 500", got)
	}
	if got := effectivePauseMs(cast, Segment{Speaker: NarratorID, PauseMs: 200}); got != 200 {
		t.Fatalf("narrator pause = %d, want 200", got)
	}
	cast.Mode = ModeCast
	if got := effectivePauseMs(cast, Segment{Speaker: "lth", PauseMs: 200}); got != 200 {
		t.Fatalf("cast mode pause = %d, want 200", got)
	}
}

func TestWriteSilenceWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sil.wav")
	if err := writeSilenceWAV(path, 250); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := WAVDurationSec(data); math.Abs(d-0.25) > 0.001 {
		t.Fatalf("silence = %v", d)
	}
}

func TestAudioFilters(t *testing.T) {
	if got := audioFilters(1, 1); got != "" {
		t.Fatalf("no change should produce no filters, got %q", got)
	}
	got := audioFilters(1.25, 1.05)
	if !bytes.Contains([]byte(got), []byte("asetrate=50400")) || !bytes.Contains([]byte(got), []byte("atempo=1.250000")) {
		t.Fatalf("filters = %q", got)
	}
	// 极端语速要拆成多段 atempo（ffmpeg 单段只支持 0.5–2）。
	if got := audioFilters(3, 1); got != "atempo=2.0,atempo=1.500000" {
		t.Fatalf("3x speed filters = %q", got)
	}
}
