package audio

import (
	"context"
	"testing"
)

func TestRegenBrokenUnknownProvenanceIsNotEvidenceOfChange(t *testing.T) {
	script := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{ID: "s1", Text: "Chữ hiện tại."}}, RenderedFrom: &RenderSource{}}
	chapterChanged, scriptChanged, reason := script.freshness(SourceHash("Chữ hiện tại."))
	if chapterChanged || scriptChanged || reason != "" || script.Stale(SourceHash("Chữ hiện tại.")) {
		t.Fatalf("unknown fingerprints falsely claim changes: chapter=%v script=%v reason=%q", chapterChanged, scriptChanged, reason)
	}
}

func TestRegenBrokenUnknownSourceStillRecomposesOnRequest(t *testing.T) {
	s := newTestService(t)
	old := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, Segments: []Segment{{ID: "s1", Text: "Chữ cũ."}}}
	if err := WriteScript(s.layout, old); err != nil {
		t.Fatal(err)
	}
	sc := SourceChapter{Chapter: 1, Completed: true, Text: "Chữ hiện tại."}
	composer := &fakeComposer{result: ComposeResult{Segments: []Segment{{ID: "s1", Text: sc.Text}}}}
	s.queuedComposer = func(int, int) (SourceChapter, Composer, CharacterRef, error) {
		return sc, composer, CharacterRef{}, nil
	}
	id, _ := s.queue.Enqueue(1, 1)
	got, err := s.composeQueuedScript(Job{ID: id, Volume: 1, Chapter: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceHash != SourceHash(sc.Text) || got.Segments[0].Text != sc.Text {
		t.Fatalf("unknown source reused old script: %+v", got)
	}
}

func TestRegenBrokenUntouchedLegacyChapterDoesNotClaimChanges(t *testing.T) {
	s := newTestService(t)
	s.synth = &stoppedSynth{}
	source := []SourceChapter{{Chapter: 1, Completed: true, Text: "Không sửa."}}
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash(source[0].Text), Segments: []Segment{{ID: "s1", Text: source[0].Text, Rendered: &SegmentAudio{Key: "legacy", DurationSec: 1}}}}); err != nil {
		t.Fatal(err)
	}
	view, err := s.Volumes(context.Background(), source, 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.Chapters[0].ChapterChanged || view.Chapters[0].ScriptChanged {
		t.Fatalf("untouched legacy chapter flagged: %+v", view.Chapters[0])
	}
}

func TestRegenBrokenMergeAssignsUniqueSegmentIDs(t *testing.T) {
	old := &Script{Segments: []Segment{{ID: "s0001", Text: "Giữ một."}, {ID: "s0002", Text: "Giữ hai."}, {ID: "s0003", Text: "Giữ ba."}}}
	fresh := &Script{Segments: []Segment{{ID: "s0001", Text: "Giữ hai."}, {ID: "s0002", Text: "Thêm mới."}, {ID: "s0003", Text: "Giữ ba."}}}
	got, _ := mergeScript(old, fresh)
	seen := map[string]bool{}
	for _, seg := range got.Segments {
		if seen[seg.ID] {
			t.Fatalf("regenerated segments share playback id %s: %+v", seg.ID, got.Segments)
		}
		seen[seg.ID] = true
	}
}

func TestRegenBrokenUnavailableChapterIsNotAnEdit(t *testing.T) {
	s := newTestService(t)
	s.synth = &stoppedSynth{}
	if err := WriteScript(s.layout, &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash("Chữ đã soạn."), Segments: []Segment{{ID: "s1", Text: "Chữ đã soạn."}}}); err != nil {
		t.Fatal(err)
	}
	view, err := s.Volumes(context.Background(), []SourceChapter{{Chapter: 1}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.Chapters[0].ChapterChanged || view.Chapters[0].Status == StatusStale {
		t.Fatalf("unavailable chapter falsely flagged as edited: %+v", view.Chapters[0])
	}
}

func TestRegenBrokenPunctuationFoldPreservesLegacyAudioFingerprint(t *testing.T) {
	script := &Script{Volume: 1, Chapter: 1, Status: StatusLocked, SourceHash: SourceHash("Xin chào."), Segments: []Segment{{ID: "s1", Text: "Xin chào", Rendered: &SegmentAudio{Key: "spoken", DurationSec: 1}}, {ID: "s2", Text: ".", Rendered: &SegmentAudio{Key: "punctuation", DurationSec: .1}}}}
	if err := foldScriptUnspeakable(script); err != nil {
		t.Fatal(err)
	}
	chapterChanged, scriptChanged, _ := script.freshness(script.SourceHash)
	if chapterChanged || !scriptChanged {
		t.Fatalf("fold must invalidate only the script audio: chapter=%v script=%v", chapterChanged, scriptChanged)
	}
}

func TestRegenBrokenRecomposeLegacyRetainsAudioOrigin(t *testing.T) {
	old := &Script{Status: StatusLocked, SourceHash: SourceHash("Cũ."), Segments: []Segment{{ID: "s1", Text: "Cũ.", Rendered: &SegmentAudio{Key: "old-audio", DurationSec: 1}}}}
	oldHash := old.ScriptHash()
	fresh := &Script{Status: StatusReview, SourceHash: SourceHash("Mới."), Segments: []Segment{{ID: "s1", Text: "Mới."}}}
	got, _ := mergeScript(old, fresh)
	if got.RenderedFrom == nil || got.RenderedFrom.ScriptHash != oldHash {
		t.Fatalf("recomposition forgot the legacy chapter audio origin: %+v", got.RenderedFrom)
	}
	chapterChanged, scriptChanged, _ := got.freshness(fresh.SourceHash)
	if chapterChanged || !scriptChanged {
		t.Fatalf("new script must remain audio-stale until rendering succeeds: chapter=%v script=%v", chapterChanged, scriptChanged)
	}
}
