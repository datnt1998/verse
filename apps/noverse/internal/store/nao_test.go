package store

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

func naoCharacters(t *testing.T, s *Store) {
	t.Helper()
	if err := s.Characters.Save([]domain.Character{
		{Name: "Lâm Uyển", Aliases: []string{"Uyển Uyển"}, Role: "Nữ chính", Description: "Biên kịch."},
		{Name: "Kỷ Dương", Aliases: []string{"Dương"}, Role: "Nam chính"},
	}); err != nil {
		t.Fatal(err)
	}
}

func naoRead(t *testing.T, s *Store, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Dir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNaoCharacterLegacyIdentityAndResolution(t *testing.T) {
	s := newTestStore(t)
	raw := []byte(`[{"name":"Đặng Ánh"},{"name":"Dang Anh"},{"name":"Lâm Uyển","aliases":["Uyển Uyển"]}]`)
	if err := os.WriteFile(filepath.Join(s.Dir(), "characters.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	chars, err := s.Characters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if chars[0].ID != "nv:dang-anh" || chars[1].ID != "nv:dang-anh-2" || chars[2].ID != "nv:lam-uyen" {
		t.Fatalf("legacy ids/collisions = %+v", chars)
	}
	if !bytes.Equal(raw, naoRead(t, s, "characters.json")) {
		t.Fatal("read must not migrate on disk")
	}
	if err := s.Characters.Save(chars); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(naoRead(t, s, "characters.json"), []byte(`"id": "nv:dang-anh"`)) {
		t.Fatal("next save must persist assigned ids")
	}
	chars[0].Name = "Đặng Bình"
	if err := s.Characters.Save(chars); err != nil {
		t.Fatal(err)
	}
	loaded, _ := s.Characters.Load()
	if loaded[0].ID != "nv:dang-anh" {
		t.Fatalf("rename changed identity: %+v", loaded[0])
	}
	ids, err := s.Characters.ResolveIDs("Uyển Uyển", "LÂM UYỂN", "nv:lam-uyen")
	if err != nil || !reflect.DeepEqual(ids, []string{"nv:lam-uyen", "nv:lam-uyen", "nv:lam-uyen"}) {
		t.Fatalf("name/alias resolution = %v, %v", ids, err)
	}
	if _, err := s.Characters.ResolveIDs("Lam Uyen"); err == nil || !strings.Contains(err.Error(), "Tên đã biết: Đặng Bình, Dang Anh, Lâm Uyển") {
		t.Fatalf("unknown name must list known names without folding diacritics: %v", err)
	}
	if err := s.Characters.Save([]domain.Character{
		{Name: "Khác", Aliases: []string{"UYỂN"}}, {Name: "Uyển"},
	}); err != nil {
		t.Fatal(err)
	}
	ids, err = s.Characters.ResolveIDs("Uyển")
	if err != nil || ids[0] != "nv:uyen" {
		t.Fatalf("exact match must beat case-insensitive alias: %v, %v", ids, err)
	}
}

func TestNaoRelationshipHistoryAndLegacySeed(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "append", true: "legacy_seed"}[legacy], func(t *testing.T) {
			s := newTestStore(t)
			naoCharacters(t, s)
			initial := []domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "đồng nghiệp", Chapter: 1}}
			if legacy {
				data, _ := json.Marshal(initial)
				if err := os.WriteFile(filepath.Join(s.Dir(), "relationship_state.json"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := s.World.UpdateRelationships(initial); err != nil {
				t.Fatal(err)
			}
			change := []domain.RelationshipEntry{{CharacterA: "Dương", CharacterB: "Uyển Uyển", Relation: "đối thủ", Chapter: 5}}
			if err := s.World.UpdateRelationships(change); err != nil {
				t.Fatal(err)
			}
			log, err := s.World.LoadRelationshipLog()
			if err != nil || len(log) != 2 {
				t.Fatalf("want two historical facts, got %+v, %v", log, err)
			}
			if log[0].From != "nv:lam-uyen" || log[1].From != "nv:ky-duong" || log[1].Scope != "truyen" || log[1].Source != "chương 5" {
				t.Fatalf("local ids/provenance = %+v", log)
			}
			if legacy && log[0].Source != "trạng thái cũ" {
				t.Fatalf("legacy seed provenance = %+v", log[0])
			}
			state, err := s.World.LoadRelationships()
			if err != nil || len(state) != 1 || state[0] != (domain.RelationshipEntry{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "đối thủ", Chapter: 5}) {
				t.Fatalf("unordered state/orientation recompute = %+v, %v", state, err)
			}
			before := naoRead(t, s, relationshipLogFile)
			if err := s.World.UpdateRelationships(change); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, naoRead(t, s, relationshipLogFile)) {
				t.Fatal("replay duplicated history")
			}
			if err := s.World.SaveRelationships([]domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "bạn", Chapter: 5}}); err != nil {
				t.Fatal(err)
			}
			after := naoRead(t, s, relationshipLogFile)
			if !bytes.HasPrefix(after, before) || !bytes.Contains(after, []byte("viết lại chương 5")) {
				t.Fatalf("revision must append diff without destroying history: %s", after)
			}
		})
	}
}

func TestNaoDirectedAddressHistory(t *testing.T) {
	s := newTestStore(t)
	naoCharacters(t, s)
	inputs := []domain.AddressTermInput{
		{From: "Dương", To: "Uyển Uyển", Calls: "chị", Self: "em"},
		{From: "Lâm Uyển", To: "Kỷ Dương", Calls: "em", Self: "chị"},
	}
	if err := s.World.UpdateAddressTerms(1, inputs); err != nil {
		t.Fatal(err)
	}
	terms, _ := s.World.AddressAt(1)
	if len(terms) != 2 || terms[0].From != "nv:ky-duong" || terms[1].From != "nv:lam-uyen" || terms[0].Source != "chương 1" || terms[0].Scope != "truyen" {
		t.Fatalf("directed initial terms = %+v", terms)
	}
	before := naoRead(t, s, addressTermsFile)
	change := []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "cô", Self: "tôi"}}
	if err := s.World.UpdateAddressTerms(5, change); err == nil || !strings.Contains(err.Error(), "phải điền reason") {
		t.Fatalf("change without actionable reason error = %v", err)
	}
	if !bytes.Equal(before, naoRead(t, s, addressTermsFile)) {
		t.Fatal("rejected change modified data")
	}
	change[0].Reason = "lật mặt ở chương 5"
	if err := s.World.UpdateAddressTerms(5, change); err != nil {
		t.Fatal(err)
	}
	all, _ := s.World.LoadAddressTerms()
	if len(all) != 3 || all[0].UntilChapter != 4 || all[1].UntilChapter != 0 || all[2].FromChapter != 5 || all[2].UntilChapter != 0 {
		t.Fatalf("close/open and reverse untouched = %+v", all)
	}
	for _, chapter := range []int{4, 5} {
		at, err := s.World.AddressAt(chapter)
		want := "chị"
		if chapter == 5 {
			want = "cô"
		}
		var calls string
		for _, term := range at {
			if term.From == "nv:ky-duong" {
				calls = term.Calls
			}
		}
		if err != nil || len(at) != 2 || calls != want {
			t.Fatalf("AddressAt(%d) = %+v, %v", chapter, at, err)
		}
	}
	before = naoRead(t, s, addressTermsFile)
	change[0].Reason = ""
	if err := s.World.UpdateAddressTerms(6, change); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, naoRead(t, s, addressTermsFile)) {
		t.Fatal("identical restatement must be no-op")
	}
}

func TestNaoVaultDeterministicAndStaleRemoval(t *testing.T) {
	s := newTestStore(t)
	naoCharacters(t, s)
	if err := s.World.UpdateRelationships([]domain.RelationshipEntry{{CharacterA: "Lâm Uyển", CharacterB: "Kỷ Dương", Relation: "đồng nghiệp", Chapter: 1}}); err != nil {
		t.Fatal(err)
	}
	inputs := []domain.AddressTermInput{{From: "Kỷ Dương", To: "Lâm Uyển", Calls: "chị", Self: "em"}}
	if err := s.World.UpdateAddressTerms(1, inputs); err != nil {
		t.Fatal(err)
	}
	page := string(naoRead(t, s, "nao/nhan-vat/ky-duong.md"))
	for _, want := range []string{"id: \"nv:ky-duong\"", "loai: nhan-vat", "ten: \"Kỷ Dương\"", "biet-danh: [\"Dương\"]", "vai-tro: \"Nam chính\"", naoGeneratedNotice, "[[nhan-vat/lam-uyen", "## Xưng hô", "## Quan hệ theo chương", "đồng nghiệp"} {
		if !strings.Contains(page, want) {
			t.Fatalf("vault page missing %q:\n%s", want, page)
		}
	}
	if !strings.Contains(string(naoRead(t, s, "nao/index.md")), "[[nhan-vat/ky-duong|Kỷ Dương]]") {
		t.Fatal("index must link from vault root")
	}
	files := func() map[string]string {
		result := map[string]string{}
		err := filepath.WalkDir(filepath.Join(s.Dir(), "nao"), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				result[path] = string(data)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := files()
	if err := s.World.UpdateAddressTerms(1, inputs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, files()) {
		t.Fatal("regenerating vault twice must produce identical bytes")
	}
	chars, _ := s.Characters.Load()
	if err := s.Characters.Save(chars[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "nao/nhan-vat/ky-duong.md")); !os.IsNotExist(err) {
		t.Fatalf("stale generated page remains: %v", err)
	}
}
