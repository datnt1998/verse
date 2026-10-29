package audio

import (
	"context"
	"os"
	"path/filepath"
)

// VoiceSample returns at most five seconds from the real project clip or the
// selected engine voice. The samples never modify a story's rendered segments.
func VoiceSample(ctx context.Context, layout Layout, synth Synthesizer, id string) ([]byte, error) {
	voices, err := ListVoices(ctx, layout, synth)
	if err != nil {
		return nil, err
	}
	var selected *ProjectVoice
	for i := range voices {
		if voices[i].ID == id {
			selected = &voices[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrNotFound
	}
	dir, err := os.MkdirTemp("", "ainovel-voice-sample-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "source.wav")
	if !selected.BuiltIn {
		src = filepath.Join(filepath.Dir(layout.AssetsDir), filepath.FromSlash(selected.File))
	} else {
		clip, err := synth.Synthesize(ctx, SynthRequest{VoiceID: id, Text: "Giọng đọc của câu chuyện.", Quality: "fast", Speed: 1, Pitch: 1})
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(src, clip.WAV, 0600); err != nil {
			return nil, err
		}
	}
	dest := filepath.Join(dir, "sample.wav")
	if err := runFFmpeg(ctx, "-i", src, "-t", "5", "-ar", "48000", "-ac", "1", "-c:a", "pcm_s16le", dest); err != nil {
		return nil, err
	}
	return os.ReadFile(dest)
}
