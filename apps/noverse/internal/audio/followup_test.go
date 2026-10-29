package audio

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type cueSynth struct {
	stoppedSynth
	enabled bool
	texts   []string
}

func (s *cueSynth) Capabilities() Capabilities { return Capabilities{EmotionCues: s.enabled} }
func (s *cueSynth) Synthesize(ctx context.Context, req SynthRequest) (Audio, error) {
	s.texts = append(s.texts, req.Text)
	return s.stoppedSynth.Synthesize(ctx, req)
}

func TestEmotionCuesOnlyReachSynthesisAndCacheKey(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	svc := newTestService(t)
	synth := &cueSynth{enabled: true}
	svc.synth = synth
	id := "Mai Anh"
	cast := &Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &id}}}
	if _, err := svc.SaveCast(cast); err != nil {
		t.Fatal(err)
	}
	for _, tone := range []string{"Cười", "Thở dài", "Hắng giọng"} {
		script := &Script{Volume: 1, Chapter: 1, Status: StatusReview, Segments: []Segment{{ID: "s0001", Speaker: NarratorID, Text: "Xin chào.", Tone: tone}}}
		if err := WriteScript(svc.layout, script); err != nil {
			t.Fatal(err)
		}
		with := svc.segmentKey(script.Segments[0], cast, nil, svc.Settings(), "vieneu", "m").Hash()
		synth.enabled = false
		without := svc.segmentKey(script.Segments[0], cast, nil, svc.Settings(), "vieneu", "m").Hash()
		synth.enabled = true
		if with == without {
			t.Fatalf("emotion cue capability did not affect cache key for %s", tone)
		}
		if _, err := svc.Preview(context.Background(), 1, 1, "s0001"); err != nil {
			t.Fatal(err)
		}
		want := "[" + strings.ToLower(tone) + "] Xin chào."
		if synth.texts[len(synth.texts)-1] != want {
			t.Fatalf("synthesis text=%q want=%q", synth.texts[len(synth.texts)-1], want)
		}
		stored, err := ReadScript(svc.layout, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Segments[0].Text != "Xin chào." {
			t.Fatal("emotion cue leaked into stored script")
		}
		transcript := renderScriptText(cast, []*Script{stored}, []SourceChapter{{Chapter: 1, Title: "Mở đầu"}})
		subtitle := BuildSRT(SegmentTimeline(0, []TimedSegment{{Text: stored.Segments[0].Text, DurationSec: 1}}))
		if strings.Contains(transcript, "[") || strings.Contains(subtitle, "[") {
			t.Fatal("emotion cue leaked into transcript/subtitles")
		}
	}
	synth.enabled = false
	if got := svc.synthesisText("Xin chào.", "Cười"); got != "Xin chào." {
		t.Fatal("unsupported engine received emotion cue")
	}
}

func TestExportTwoPassLoudnorm(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	ctx := context.Background()
	dir := t.TempDir()
	in := filepath.Join(dir, "input.wav")
	out := filepath.Join(dir, "output.wav")
	// A long quiet section followed by a loud one exposes one-pass dynamic normalization.
	if err := runFFmpeg(ctx, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=24", "-af", "volume='if(lt(t,12),0.1,2)':eval=frame", in); err != nil {
		t.Fatal(err)
	}
	filter, err := measuredLoudnorm(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"measured_I=", "measured_TP=", "measured_LRA=", "measured_thresh=", "offset=", "linear=true"} {
		if !strings.Contains(filter, field) {
			t.Fatalf("two-pass filter missing %s: %s", field, filter)
		}
	}
	if err := encodeAudio(ctx, in, out, "wav", true); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpegBin, "-hide_banner", "-nostdin", "-i", out, "-af", "loudnorm=I=-16:print_format=json", "-f", "null", "-")
	measured, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	text := string(measured)
	start, end := strings.LastIndex(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		t.Fatalf("no loudness measurement: %s", text)
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(text[start:end+1]), &values); err != nil {
		t.Fatal(err)
	}
	integrated, err := strconv.ParseFloat(values["input_i"], 64)
	if err != nil {
		t.Fatal(err)
	}
	if integrated < -17 || integrated > -15 {
		t.Fatalf("integrated loudness=%v LUFS, want -16 ±1", integrated)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}
