package audio

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"
)

type textSynth struct{ stoppedSynth }

func (s *textSynth) Synthesize(_ context.Context, req SynthRequest) (Audio, error) {
	s.calls++
	hash := SourceHash(req.Text)
	return Audio{WAV: testWAV(48000, 1, 24000, int16(hash[0])*100), SampleRate: 48000, DurationSec: .5}, nil
}

func TestStaleAudioFlagsAfterScriptEdit(t *testing.T) {
	s := newTestService(t)
	s.synth = &stoppedSynth{}
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	source := []SourceChapter{{Chapter: 1, Text: "Bản gốc.", Completed: true}}
	script := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash(source[0].Text), Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: source[0].Text}}}
	if err := WriteScript(s.layout, script); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(context.Background(), 1, 1, "s1"); err != nil {
		t.Fatal(err)
	}
	script, _ = s.Script(1, 1)
	script.Segments[0].Text = "Bản đọc mới."
	if _, err := s.SaveScript(1, 1, ScriptEdit{Segments: script.Segments}); err != nil {
		t.Fatal(err)
	}
	view, err := s.Volumes(context.Background(), source, 1)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(view.Chapters[0])
	var row map[string]any
	_ = json.Unmarshal(data, &row)
	if row["chapterChanged"] != false || row["scriptChanged"] != true || row["staleReason"] == "" {
		t.Fatalf("script edit missing stale warning: %s", data)
	}
	source[0].Text = "Chính văn mới."
	view, err = s.Volumes(context.Background(), source, 1)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(view.Chapters[0])
	_ = json.Unmarshal(data, &row)
	if row["chapterChanged"] != true {
		t.Fatalf("chapter edit missing stale warning: %s", data)
	}
}

func TestStaleAudioRerenderUsesCurrentChapterAndReusesSegments(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	for _, entry := range []string{"render", "queue", "role"} {
		t.Run(entry, func(t *testing.T) {
			s := newTestService(t)
			synth := &textSynth{}
			s.synth = synth
			voice := "Mai Anh"
			if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", VoiceID: &voice}}}); err != nil {
				t.Fatal(err)
			}
			oldText := "Đoạn cũ.\nĐoạn giữ nguyên."
			script := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash(oldText), Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Đoạn cũ."}, {ID: "s2", Speaker: NarratorID, Text: "Đoạn giữ nguyên."}}}
			if err := WriteScript(s.layout, script); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Enqueue(1, 1); err != nil {
				t.Fatal(err)
			}
			s.runWG.Wait()
			before, _ := s.Script(1, 1)
			key := before.Segments[1].Rendered.Key
			path := s.layout.CachePath(1, 1, key)
			stat, _ := os.Stat(path)
			calls := synth.calls
			oldWAV, err := ReadSegmentAudio(s.layout, 1, 1, before.Segments[0].Rendered.Key)
			if err != nil {
				t.Fatal(err)
			}
			oldMP3, err := os.ReadFile(s.layout.ChapterAudioPath(1, 1, "mp3"))
			if err != nil {
				t.Fatal(err)
			}
			sc := SourceChapter{Chapter: 1, Text: "Đoạn mới.\nĐoạn giữ nguyên.", Completed: true}
			composer := &fakeComposer{result: ComposeResult{Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Đoạn mới."}, {ID: "s2", Speaker: NarratorID, Text: "Đoạn giữ nguyên."}}}}
			s.queuedComposer = func(volume, chapter int) (SourceChapter, Composer, CharacterRef, error) {
				return sc, composer, CharacterRef{}, nil
			}
			switch entry {
			case "render":
				if _, err := s.Enqueue(1, 1); err != nil {
					t.Fatal(err)
				}
			case "queue":
				if _, err := s.QueueChapters([]SourceChapter{sc}, 1, QueueInput{Chapters: []int{1}, Lock: true}); err != nil {
					t.Fatal(err)
				}
			case "role":
				if _, _, err := s.ChangeRoleVoice(NarratorID, voice, "", true); err != nil {
					t.Fatal(err)
				}
			}
			s.runWG.Wait()
			after, _ := s.Script(1, 1)
			if after.Segments[0].Text != "Đoạn mới." || composer.lastInput().Text != sc.Text {
				t.Fatalf("rerender used old chapter: text=%q compose=%q", after.Segments[0].Text, composer.lastInput().Text)
			}
			if after.SourceHash != SourceHash(sc.Text) || after.Status != StatusLocked {
				t.Fatalf("source not refreshed: %+v", after)
			}
			reused, _ := os.Stat(path)
			if after.Segments[1].Rendered.Key != key || !reused.ModTime().Equal(stat.ModTime()) || synth.calls != calls+1 {
				t.Fatalf("unchanged segment not reused: calls=%d want=%d", synth.calls, calls+1)
			}
			if after.Segments[0].Rendered.Key == before.Segments[0].Rendered.Key {
				t.Fatal("changed segment retained old cache key")
			}
			newWAV, err := ReadSegmentAudio(s.layout, 1, 1, after.Segments[0].Rendered.Key)
			if err != nil {
				t.Fatal(err)
			}
			newMP3, err := os.ReadFile(s.layout.ChapterAudioPath(1, 1, "mp3"))
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(oldWAV) == sha256.Sum256(newWAV) || sha256.Sum256(oldMP3) == sha256.Sum256(newMP3) {
				t.Fatal("regenerated files retained old hashes")
			}
			if after.RenderedFrom == nil || after.RenderedFrom.ScriptHash != after.ScriptHash() || after.RenderedFrom.Quality != "standard" || after.RenderedFrom.CastHash == "" {
				t.Fatalf("missing render input fingerprints: %+v", after.RenderedFrom)
			}
			view, err := s.Volumes(context.Background(), []SourceChapter{sc}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if view.Chapters[0].ChapterChanged || view.Chapters[0].ScriptChanged {
				t.Fatalf("fresh audio marked stale: %+v", view.Chapters[0])
			}
		})
	}
}

func TestStaleAudioLockPreservesCompositionSource(t *testing.T) {
	s := newTestService(t)
	script := &Script{Volume: 1, Chapter: 1, Status: StatusReview, SourceHash: SourceHash("Cũ."), Segments: []Segment{{ID: "s1", Text: "Cũ."}}}
	if err := WriteScript(s.layout, script); err != nil {
		t.Fatal(err)
	}
	settings := s.Settings()
	settings.AutoOnLock = false
	if _, err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	locked, err := s.Lock(context.Background(), 1, 1, SourceHash("Mới."))
	if err != nil {
		t.Fatal(err)
	}
	if locked.SourceHash != script.SourceHash || !locked.Stale(SourceHash("Mới.")) {
		t.Fatal("locking erased chapter change since composition")
	}
	script.SourceHash = ""
	if err := WriteScript(s.layout, script); err != nil {
		t.Fatal(err)
	}
	locked, err = s.Lock(context.Background(), 1, 1, SourceHash("Mới."))
	if err != nil {
		t.Fatal(err)
	}
	if locked.SourceHash != "" || locked.Stale(SourceHash("Mới.")) || !locked.needsComposition(SourceHash("Mới.")) {
		t.Fatal("locking must preserve unknown provenance and require composition on regeneration")
	}
}

func TestStaleAudioRoleRecastReplacesAcceptedVoice(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg")
	}
	s := newTestService(t)
	s.synth = &stoppedSynth{}
	voice := "Mai Anh"
	if _, err := s.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "Lời đọc hiện tại."}}}); err != nil {
		t.Fatal(err)
	}
	candidate, err := s.Rerender(context.Background(), 1, 1, "s1", RerenderOptions{VoiceID: "Giọng chọn riêng"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptCandidate(1, 1, "s1", candidate.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ChangeRoleVoice(NarratorID, voice, "", true); err != nil {
		t.Fatal(err)
	}
	s.runWG.Wait()
	script, err := s.Script(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if script.Segments[0].Rendered.VoiceID != voice {
		t.Fatalf("role recast kept stale accepted voice: %s", script.Segments[0].Rendered.VoiceID)
	}
}

func TestStaleAudioRefreshPreservesLegacyAudioProvenance(t *testing.T) {
	old := &Script{Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash("Cũ."), Segments: []Segment{{ID: "s1", Text: "Cũ.", Rendered: &SegmentAudio{Key: "old-key", VoiceID: "Mai Anh"}}}}
	fresh := &Script{Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash("Mới."), Segments: []Segment{{ID: "s1", Text: "Mới."}}}
	merged, _ := mergeScript(old, fresh)
	chapterChanged, scriptChanged, _ := merged.freshness(SourceHash("Mới."))
	if chapterChanged || !scriptChanged {
		t.Fatalf("refresh hid legacy stale audio: chapterChanged=%v scriptChanged=%v", chapterChanged, scriptChanged)
	}
}
