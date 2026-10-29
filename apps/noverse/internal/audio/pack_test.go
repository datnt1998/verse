package audio_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

func TestPackAudioDefaultsAndPronunciations(t *testing.T) {
	load := func(t *testing.T, manifest string, dictionary string) (*audio.PackDefaults, *assets.Pack) {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, "packs", "audio-test")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "goi.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if dictionary != "" {
			if err := os.WriteFile(filepath.Join(dir, "cach-doc.json"), []byte(dictionary), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		bundle := assets.Load("audio-test", assets.LoadOptions{BookStyleDir: root})
		if bundle.Err != nil || bundle.Pack == nil {
			t.Fatalf("load test pack: %v", bundle.Err)
		}
		defaults, err := audio.ReadPackDefaults(bundle.Pack, bundle.Pack.NguoiNghe)
		if err != nil {
			t.Fatal(err)
		}
		return defaults, bundle.Pack
	}
	service := func(t *testing.T, pack *audio.PackDefaults) *audio.Service {
		t.Helper()
		root := t.TempDir()
		layout := audio.Layout{
			StoryDir: root, AudioDir: filepath.Join(root, "audio"),
			AssetsDir: filepath.Join(root, "tai-san"), ExportsDir: filepath.Join(root, "xuat-ban"),
			PronouncePath: filepath.Join(root, ".ainovel", "cach-doc.json"), Pack: pack,
		}
		// Both catalogue spellings are real engine/project metadata conventions.
		voices := []audio.ProjectVoice{
			{ID: "a-male", Name: "Nam", Gender: "male"},
			{ID: "z-female", Name: "Nữ Z", Gender: "nu"},
			{ID: "b-female", Name: "Nữ B", Gender: "female"},
		}
		if err := audio.WriteProjectVoices(layout, voices); err != nil {
			t.Fatal(err)
		}
		svc, err := audio.NewService(layout, audio.NullSynth{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(svc.Close)
		return svc
	}
	bundle := assets.Load("nu-tan-bao-thu", assets.LoadOptions{})
	if bundle.Err != nil || bundle.Pack == nil {
		t.Fatalf("load pack A: %v", bundle.Err)
	}
	packA, err := audio.ReadPackDefaults(bundle.Pack, bundle.Pack.NguoiNghe)
	if err != nil {
		t.Fatal(err)
	}
	ref := audio.CharacterRef{Characters: []audio.Character{{Name: "Lâm Tri Hạ"}}}
	t.Run("pack A new cast", func(t *testing.T) {
		svc := service(t, packA)
		cast, err := svc.Cast(ref)
		if err != nil {
			t.Fatal(err)
		}
		if cast.Mode != audio.ModeSingle || cast.Single.VoiceID != "b-female" {
			t.Fatalf("pack A new cast: mode=%q voice=%q, want single and b-female", cast.Mode, cast.Single.VoiceID)
		}
		narrator, ok := cast.SpeakerByID(audio.NarratorID)
		if !ok || narrator.VoiceID == nil || *narrator.VoiceID != "b-female" {
			t.Fatalf("narrator voice = %+v", narrator)
		}
	})
	t.Run("existing vai.json untouched", func(t *testing.T) {
		svc := service(t, packA)
		current := audio.DefaultCast(ref.Characters, ref.Support)
		current.Single.VoiceID = "user-voice"
		current.Single.DialogueShift = false
		if err := audio.WriteCast(svc.Layout(), &current); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(svc.Layout().CastPath())
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.Cast(ref)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(svc.Layout().CastPath())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || !reflect.DeepEqual(*got, current) {
			t.Fatalf("existing vai.json changed: %+v", got)
		}
	})
	t.Run("no pack unchanged", func(t *testing.T) {
		svc := service(t, nil)
		got, err := svc.Cast(ref)
		if err != nil {
			t.Fatal(err)
		}
		if want := audio.DefaultCast(ref.Characters, ref.Support); !reflect.DeepEqual(*got, want) {
			t.Fatalf("no pack cast = %+v, want %+v", got, want)
		}
		dict, err := svc.Pronunciations()
		if err != nil || len(dict) != 0 {
			t.Fatalf("no pack dictionary = %+v, %v", dict, err)
		}
		if got, _ := audio.Normalize("NXB", audio.DefaultNormalizeOptions(), dict); got != "nhà xuất bản" {
			t.Fatalf("built-in default = %q", got)
		}
	})
	t.Run("dictionary precedence", func(t *testing.T) {
		pack, _ := load(t, `{"ten":"Audio test","audio":{"che_do":"cast","giong":"nam"}}`,
			`{"entries":[{"written":"NXB","spoken":"nhà in"},{"written":"XYZ","spoken":"chữ thử"}]}`)
		svc := service(t, pack)
		dict, err := svc.Pronunciations()
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := audio.Normalize("NXB XYZ", audio.DefaultNormalizeOptions(), dict); got != "nhà in chữ thử" {
			t.Fatalf("pack dictionary over built-ins = %q", got)
		}
		if _, err := svc.SavePronunciations([]audio.Pronunciation{{Written: "nxb", Spoken: "ban biên tập"}}); err != nil {
			t.Fatal(err)
		}
		dict, err = svc.Pronunciations()
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := audio.Normalize("NXB XYZ", audio.DefaultNormalizeOptions(), dict); got != "ban biên tập chữ thử" {
			t.Fatalf("user dictionary over pack = %q", got)
		}
		stored, err := audio.ReadPronunciations(svc.Layout())
		if err != nil || len(stored) != 1 || stored[0].Spoken != "ban biên tập" {
			t.Fatalf("pack entries leaked into user dictionary: %+v, %v", stored, err)
		}
		cast, err := svc.Cast(ref)
		if err != nil || cast.Mode != audio.ModeCast || cast.Single.VoiceID != "a-male" {
			t.Fatalf("male cast default = %+v, %v", cast, err)
		}
	})
	t.Run("listener defaults", func(t *testing.T) {
		for _, tc := range []struct{ listener, gender string }{
			{"Phụ nữ trung niên, nghe khi làm việc nhà", "nu"},
			{"Nữ 18–35", "nu"}, {"Nam trung niên", "nam"}, {"Nam trẻ", "nam"},
			{"Người cao tuổi", ""}, {"Nam trung niên (C-Nam) / nữ 18–35 (C-Nữ)", ""},
		} {
			t.Run(tc.listener, func(t *testing.T) {
				manifest, err := json.Marshal(map[string]any{"ten": "Test", "nguoi_nghe": tc.listener, "audio": map[string]string{"che_do": "single"}})
				if err != nil {
					t.Fatal(err)
				}
				pack, _ := load(t, string(manifest), "")
				if pack.Gender != tc.gender {
					t.Fatalf("listener %q gender = %q, want %q", tc.listener, pack.Gender, tc.gender)
				}
				svc := service(t, pack)
				cast, err := svc.Cast(ref)
				if err != nil {
					t.Fatal(err)
				}
				wantVoice := map[string]string{"nu": "b-female", "nam": "a-male"}[tc.gender]
				if cast.Single.VoiceID != wantVoice {
					t.Fatalf("listener %q voice = %q, want %q", tc.listener, cast.Single.VoiceID, wantVoice)
				}
			})
		}
	})
}
