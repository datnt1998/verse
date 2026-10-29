package audio

import (
	"testing"
	"time"
)

func TestSynthKeyChangesWithEveryInput(t *testing.T) {
	base := SynthKey{Text: "xin chào", VoiceID: "v1", Tone: "", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v3"}
	h := base.Hash()
	if len(h) != 64 {
		t.Fatalf("hash should be sha256 hex, got %q", h)
	}
	variants := map[string]SynthKey{
		"text":    {Text: "xin chào!", VoiceID: "v1", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v3"},
		"voice":   {Text: "xin chào", VoiceID: "v2", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v3"},
		"tone":    {Text: "xin chào", VoiceID: "v1", Tone: "Ngọt", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v3"},
		"speed":   {Text: "xin chào", VoiceID: "v1", Speed: 1.1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v3"},
		"pitch":   {Text: "xin chào", VoiceID: "v1", Speed: 1, Pitch: 1.05, Quality: QualityStandard, Engine: "vieneu", Model: "v3"},
		"quality": {Text: "xin chào", VoiceID: "v1", Speed: 1, Pitch: 1, Quality: QualityHigh, Engine: "vieneu", Model: "v3"},
		"engine":  {Text: "xin chào", VoiceID: "v1", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "other", Model: "v3"},
		"model":   {Text: "xin chào", VoiceID: "v1", Speed: 1, Pitch: 1, Quality: QualityStandard, Engine: "vieneu", Model: "v4"},
	}
	for name, v := range variants {
		if v.Hash() == h {
			t.Fatalf("%s must change the cache key", name)
		}
	}
	if base.Hash() != h {
		t.Fatalf("hash must be stable")
	}
}

func TestScriptStaleDetection(t *testing.T) {
	locked := time.Now()
	s := &Script{Status: StatusLocked, LockedAt: &locked, SourceHash: SourceHash("chương một")}
	if s.Stale(SourceHash("chương một")) {
		t.Fatal("same text must not be stale")
	}
	if !s.Stale(SourceHash("chương một đã sửa")) {
		t.Fatal("changed text must be stale")
	}
	s.Status = StatusReview
	if s.Stale(SourceHash("khác")) {
		t.Fatal("review scripts are never stale")
	}
	var nilScript *Script
	if nilScript.Stale("x") {
		t.Fatal("nil script is not stale")
	}
}

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name      string
		script    *Script
		composing bool
		hash      string
		want      string
	}{
		{"chưa soạn", nil, false, "h", StatusTodo},
		{"đang soạn", nil, true, "h", StatusDraft},
		{"chờ duyệt", &Script{Status: StatusReview, SourceHash: "h"}, false, "h", StatusReview},
		{"đã chốt", &Script{Status: StatusLocked, SourceHash: "h"}, false, "h", StatusLocked},
		{"sửa sau khi chốt", &Script{Status: StatusLocked, SourceHash: "old"}, false, "new", StatusStale},
		{"đang soạn lại", &Script{Status: StatusLocked, SourceHash: "old"}, true, "new", StatusDraft},
	}
	for _, tc := range cases {
		if got := DeriveStatus(tc.script, tc.composing, tc.hash); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestChangedSegmentsCountsOnlyRendered(t *testing.T) {
	s := &Script{Segments: []Segment{
		{ID: "s0001", Text: "a", Rendered: &SegmentAudio{Key: "k1"}},
		{ID: "s0002", Text: "b", Rendered: &SegmentAudio{Key: "old"}},
		{ID: "s0003", Text: "c"},
	}}
	got := s.ChangedSegments(func(seg Segment) string {
		if seg.Text == "b" {
			return "new"
		}
		return "k1"
	})
	if got != 1 {
		t.Fatalf("ChangedSegments = %d, want 1", got)
	}
}

func TestScriptRoundTrip(t *testing.T) {
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	now := time.Now().UTC().Truncate(time.Second)
	in := &Script{
		Chapter: 3, Volume: 1, Status: StatusLocked, LockedAt: &now, SourceHash: "h",
		Segments: []Segment{{ID: "s0001", Speaker: "dan", Text: "xin chào", PauseMs: 400, Tone: "Ngọt",
			Normalized: []Change{{From: "90", To: "chín mươi", Kind: "number"}},
			Rendered:   &SegmentAudio{Key: "k", VoiceID: "v", DurationSec: 1.5}}},
	}
	if err := WriteScript(layout, in); err != nil {
		t.Fatal(err)
	}
	out, err := ReadScript(layout, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Chapter != 3 || len(out.Segments) != 1 || out.Segments[0].Rendered == nil {
		t.Fatalf("round trip lost data: %+v", out)
	}
	if out.Segments[0].Normalized[0].Kind != "number" {
		t.Fatalf("normalized changes lost")
	}
	if missing, _ := ReadScript(layout, 1, 9); missing != nil {
		t.Fatalf("missing script should be nil")
	}
}

func TestReadScriptRecomputesEnglishFlag(t *testing.T) {
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	// Bản lưu cũ: cờ "english" gán nhầm vào câu thuần Việt, và thiếu trên câu có "email".
	in := &Script{Chapter: 1, Volume: 1, Segments: []Segment{
		{ID: "s0001", Speaker: "dan", Text: "Tôi gõ móng tay lên dòng chữ in đậm.", Flags: []string{FlagEnglish, FlagLongSentence}},
		{ID: "s0002", Speaker: "dan", Text: "Cô mở email rồi xoá file.", Flags: nil},
	}}
	if err := WriteScript(layout, in); err != nil {
		t.Fatal(err)
	}
	out, err := ReadScript(layout, 1, 1)
	if err != nil || out == nil {
		t.Fatal(err)
	}
	seg1 := out.Segments[0].Flags
	if hasFlag(out.Segments[0], FlagEnglish) || !hasFlag(out.Segments[0], FlagLongSentence) {
		t.Fatalf("câu Việt phải mất cờ english nhưng giữ long_sentence, flags = %v", seg1)
	}
	if !hasFlag(out.Segments[1], FlagEnglish) {
		t.Fatalf("câu có email/file phải được cờ english, flags = %v", out.Segments[1].Flags)
	}
}
