package audio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpeakerIDsAreStableAndUnique(t *testing.T) {
	if got := SpeakerID("Hứa Hoài Châu"); got != "hhc" {
		t.Fatalf("SpeakerID = %q", got)
	}
	if got := SpeakerID("Lâm Tri Hạ"); got != "lth" {
		t.Fatalf("SpeakerID = %q", got)
	}
	used := map[string]bool{}
	a := uniqueSpeakerID("Lâm Tri Hạ", used)
	b := uniqueSpeakerID("Lâm Thanh Hà", used)
	if a == b {
		t.Fatalf("collision not resolved: %q", a)
	}
	if !strings.HasPrefix(b, "lth") {
		t.Fatalf("collision id should keep base, got %q", b)
	}
}

func TestDefaultCastFromCharacters(t *testing.T) {
	cast := DefaultCast(
		[]Character{{Name: "Lâm Tri Hạ", Role: "nữ chính"}, {Name: "Tống Nhược Vy"}},
		[]SupportCharacter{{Name: "Phùng Chấn"}, {Name: "Lâm Tri Hạ"}},
	)
	if cast.Mode != ModeCast || !cast.Single.DialogueShift || !cast.Single.KeepTones {
		t.Fatalf("defaults wrong: %+v", cast)
	}
	if cast.Speakers[0].ID != NarratorID {
		t.Fatalf("narrator must be first, got %+v", cast.Speakers[0])
	}
	if len(cast.Speakers) != 4 {
		t.Fatalf("want narrator + 3 unique speakers, got %d: %+v", len(cast.Speakers), cast.Speakers)
	}
	if cast.Speakers[1].Role != RoleMain {
		t.Fatalf("first characters should be main, got %q", cast.Speakers[1].Role)
	}
}

func TestEnsureCastKeepsVoiceAssignments(t *testing.T) {
	voice := "giong-tri-ha"
	cur := &Cast{
		Mode: ModeSingle,
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator},
			{ID: "lth", Name: "Lâm Tri Hạ", Role: RoleMain, VoiceID: &voice},
		},
	}
	merged := EnsureCast(cur, []Character{{Name: "Lâm Tri Hạ"}, {Name: "Tống Nhược Vy"}}, nil)
	if merged.Mode != ModeSingle {
		t.Fatalf("mode must be preserved, got %q", merged.Mode)
	}
	sp, ok := merged.SpeakerByID("lth")
	if !ok || sp.VoiceID == nil || *sp.VoiceID != voice {
		t.Fatalf("voice assignment lost: %+v", sp)
	}
	if _, ok := merged.SpeakerByName("Tống Nhược Vy"); !ok {
		t.Fatalf("new character not added: %+v", merged.Speakers)
	}
}

func TestEnsureCastRefreshesSpeakerKind(t *testing.T) {
	voice := "giong-tri-ha"
	cur := &Cast{
		Mode: ModeCast,
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator},
			{ID: "lth", Name: "Lâm Tri Hạ", Role: RoleMain, VoiceID: &voice},
		},
	}
	merged := EnsureCast(cur, []Character{{Name: "Lâm Tri Hạ", Kind: "lead"}, {Name: "Tống Nhược Vy", Kind: "antagonist"}}, nil)
	sp, ok := merged.SpeakerByID("lth")
	if !ok || sp.Kind != "lead" {
		t.Fatalf("kind must be filled from the character file, got %+v", sp)
	}
	if sp.VoiceID == nil || *sp.VoiceID != voice || sp.Role != RoleMain {
		t.Fatalf("kind refresh must not touch role/voice: %+v", sp)
	}
	if narrator, _ := merged.SpeakerByID(NarratorID); narrator.Kind != "" {
		t.Errorf("narrator has no character kind, got %q", narrator.Kind)
	}
	if cur.Speakers[1].Kind != "" {
		t.Errorf("EnsureCast must not mutate the loaded cast, got %q", cur.Speakers[1].Kind)
	}
}

func TestResolveVoiceCastAndSingle(t *testing.T) {
	voices := map[string]ProjectVoice{
		"giong-tri-ha": {ID: "giong-tri-ha", File: "/p/tai-san/giong/tri-ha.wav"},
		"giong-nam":    {ID: "giong-nam", File: "/p/tai-san/giong/nam.wav"},
	}
	cast := &Cast{
		Mode: ModeCast,
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", VoiceID: ptr("giong-nam")},
			{ID: "lth", Name: "Lâm Tri Hạ", VoiceID: ptr("giong-tri-ha")},
			{ID: "hhc", Name: "Hứa Hoài Châu"},
		},
	}
	ref := ResolveVoice(cast, Segment{Speaker: "lth"}, voices)
	if !ref.OK || ref.VoiceID != "giong-tri-ha" || ref.RefAudio == "" || ref.Pitch != 1 {
		t.Fatalf("cast voice = %+v", ref)
	}
	// 没选声音的配角退回叙述者声音（不阻塞生成，但界面有 no_voice 标记）。
	ref = ResolveVoice(cast, Segment{Speaker: "hhc"}, voices)
	if !ref.OK || ref.VoiceID != "giong-nam" {
		t.Fatalf("fallback = %+v", ref)
	}
	// 模型编出不存在的说话人：退回叙述者声音。
	ref = ResolveVoice(cast, Segment{Speaker: "nguoi-la"}, voices)
	if !ref.OK || ref.VoiceID != "giong-nam" || ref.RefAudio != "/p/tai-san/giong/nam.wav" {
		t.Fatalf("unknown speaker = %+v", ref)
	}
	// 单声模式：对白升 5% 音高。
	cast.Mode = ModeSingle
	cast.Single = SingleCast{VoiceID: "giong-tri-ha", DialogueShift: true}
	ref = ResolveVoice(cast, Segment{Speaker: "lth"}, voices)
	if !ref.OK || ref.Pitch != 1.05 {
		t.Fatalf("dialogue pitch = %+v", ref)
	}
	ref = ResolveVoice(cast, Segment{Speaker: NarratorID}, voices)
	if ref.Pitch != 1 {
		t.Fatalf("narrator pitch = %+v", ref)
	}
	cast.Single.DialogueShift = false
	cast.Single.VoiceID = ""
	if ref := ResolveVoice(cast, Segment{Speaker: "lth"}, voices); ref.OK {
		t.Fatalf("no voice selected must not be OK: %+v", ref)
	}
}

func TestHasVoiceFlag(t *testing.T) {
	cast := &Cast{Mode: ModeCast, Speakers: []Speaker{
		{ID: NarratorID, Name: "Dẫn truyện", VoiceID: ptr("v")},
		{ID: "lth", Name: "Lâm Tri Hạ", VoiceID: ptr("v2")},
		{ID: "hhc", Name: "Hứa Hoài Châu"},
	}}
	if !hasVoice(cast, "lth") {
		t.Fatal("assigned speaker must have voice")
	}
	// 配角没选声音 → 退回叙述者，仍算"能生成"。
	if !hasVoice(cast, "hhc") {
		t.Fatal("speaker without voice falls back to narrator")
	}
	cast.Speakers[0].VoiceID = nil
	if hasVoice(cast, "hhc") {
		t.Fatal("no narrator voice and no own voice must be no_voice")
	}
}

func TestApplyVoiceFlags(t *testing.T) {
	cast := &Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện"}}}
	script := &Script{Segments: []Segment{{ID: "s1", Speaker: NarratorID, Flags: []string{FlagEnglish}}}}
	applyVoiceFlags(script, cast)
	joined := strings.Join(script.Segments[0].Flags, ",")
	if !strings.Contains(joined, FlagNoVoice) || !strings.Contains(joined, FlagEnglish) {
		t.Fatalf("flags = %v", script.Segments[0].Flags)
	}
	cast.Speakers[0].VoiceID = ptr("v")
	applyVoiceFlags(script, cast)
	if strings.Contains(strings.Join(script.Segments[0].Flags, ","), FlagNoVoice) {
		t.Fatalf("no_voice must be cleared: %v", script.Segments[0].Flags)
	}
}

func TestProjectVoicesAndClipValidation(t *testing.T) {
	layout := Layout{AssetsDir: filepath.Join(t.TempDir(), "tai-san")}
	if err := ValidateVoiceClip(5); err == nil || !strings.Contains(err.Error(), "10 đến 30") {
		t.Fatalf("short clip error = %v", err)
	}
	if err := ValidateVoiceClip(31); err == nil {
		t.Fatal("long clip must be rejected")
	}
	if err := ValidateVoiceClip(18); err != nil {
		t.Fatalf("18s clip must pass: %v", err)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "in.wav")
	if err := os.WriteFile(src, testWAV(48000, 1, 48000, 4000), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := StoreVoice(layout, "Giọng Tri Hạ", "nu", src, 18)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "giong-tri-ha" || v.File != "tai-san/giong/giong-tri-ha.wav" || v.BuiltIn {
		t.Fatalf("stored voice = %+v", v)
	}
	if _, err := os.Stat(filepath.Join(layout.AssetsDir, "giong", "giong-tri-ha.wav")); err != nil {
		t.Fatalf("voice file missing: %v", err)
	}
	list, err := ReadProjectVoices(layout)
	if err != nil || len(list) != 1 || list[0].Name != "Giọng Tri Hạ" {
		t.Fatalf("index = %+v (%v)", list, err)
	}
	// 同名再传 → id 不冲突。
	src2 := filepath.Join(dir, "in2.wav")
	if err := os.WriteFile(src2, testWAV(48000, 1, 48000, 4000), 0o644); err != nil {
		t.Fatal(err)
	}
	v2, err := StoreVoice(layout, "Giọng Tri Hạ", "", src2, 18)
	if err != nil {
		t.Fatal(err)
	}
	if v2.ID == v.ID {
		t.Fatalf("duplicate id: %q", v2.ID)
	}
	if err := DeleteVoice(layout, v.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := ReadProjectVoices(layout); len(list) != 1 {
		t.Fatalf("after delete = %+v", list)
	}
	if err := DeleteVoice(layout, "nope"); err == nil {
		t.Fatal("deleting unknown voice must fail")
	}
}

func TestSettingsDefaultsAndValidation(t *testing.T) {
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = filepath.Join(layout.StoryDir, "audio")
	s, err := ReadSettings(layout)
	if err != nil {
		t.Fatal(err)
	}
	if s.Quality != QualityStandard || !s.AutoOnLock || !s.Normalize.Numbers || !s.Normalize.DropBrackets {
		t.Fatalf("defaults = %+v", s)
	}
	s.Quality = QualityHigh
	s.AutoOnLock = false
	s.Normalize.Numbers = false
	if err := WriteSettings(layout, s); err != nil {
		t.Fatal(err)
	}
	back, err := ReadSettings(layout)
	if err != nil {
		t.Fatal(err)
	}
	if back.Quality != QualityHigh || back.AutoOnLock || back.Normalize.Numbers {
		t.Fatalf("round trip = %+v", back)
	}
	if err := WriteSettings(layout, Settings{Quality: "ultra"}); err == nil {
		t.Fatal("invalid quality must be rejected")
	}
}

func TestPronunciationsRoundTripAndValidation(t *testing.T) {
	layout := Layout{PronouncePath: filepath.Join(t.TempDir(), ".ainovel", "cach-doc.json")}
	entries := []Pronunciation{
		{Written: " live ", Spoken: "lai"},
		{Written: "LIVE", Spoken: "lai-two"},
		{Written: "", Spoken: ""},
	}
	if err := WritePronunciations(layout, entries); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPronunciations(layout)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Spoken != "lai-two" {
		t.Fatalf("dedupe last-wins failed: %+v", got)
	}
	if err := WritePronunciations(layout, []Pronunciation{{Written: "live"}}); err == nil {
		t.Fatal("entry without spoken must be rejected")
	}
}

func TestFlagsLifecycle(t *testing.T) {
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = filepath.Join(layout.StoryDir, "audio")
	f, err := AddFlag(layout, Flag{Chapter: 3, SegmentID: "s0007", AtSec: 12.5, Reason: "giọng bị rè", Speaker: "dan"})
	if err != nil {
		t.Fatal(err)
	}
	if f.ID == "" {
		t.Fatal("flag needs an id")
	}
	// 同段同理由重复标记 → 同一条。
	dup, err := AddFlag(layout, Flag{Chapter: 3, SegmentID: "s0007", AtSec: 12.5, Reason: "Giọng bị rè"})
	if err != nil || dup.ID != f.ID {
		t.Fatalf("duplicate flag = %+v (%v)", dup, err)
	}
	flags, err := ReadFlags(layout)
	if err != nil || len(flags) != 1 {
		t.Fatalf("flags = %+v (%v)", flags, err)
	}
	if err := DeleteFlag(layout, f.ID); err != nil {
		t.Fatal(err)
	}
	if flags, _ := ReadFlags(layout); len(flags) != 0 {
		t.Fatalf("after delete = %+v", flags)
	}
	if err := DeleteFlag(layout, "nope"); err == nil {
		t.Fatal("deleting unknown flag must fail")
	}
	if _, err := AddFlag(layout, Flag{Chapter: 0, Reason: "x"}); err == nil {
		t.Fatal("flag without chapter must fail")
	}
}

func ptr[T any](v T) *T { return &v }
