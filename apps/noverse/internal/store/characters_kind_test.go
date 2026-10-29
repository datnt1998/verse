package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// Gói S · 06-data-contract › Bổ sung §S: dữ liệu cũ chưa có `kind` được tính một lần từ `role`.
func TestCharacterKindFromLegacyRole(t *testing.T) {
	cases := []struct {
		role string
		want domain.CharacterKind
	}{
		{"Nữ chính", domain.CharacterKindLead},
		{"Phản diện chính", domain.CharacterKindAntagonist},
		{"Đồng phạm", domain.CharacterKindAntagonist},
		{"Đồng minh hài", domain.CharacterKindAlly},
		{"bạn đồng hành", domain.CharacterKindAlly},
		{"Phụ chức năng", domain.CharacterKindSupport},
		{"", domain.CharacterKindSupport},
		{"main", domain.CharacterKindLead},
		{"villain", domain.CharacterKindAntagonist},
	}
	for _, c := range cases {
		if got := CharacterKindOf(c.role); got != c.want {
			t.Fatalf("CharacterKindOf(%q) = %q, muốn %q", c.role, got, c.want)
		}
	}
}

// Load điền kind trong bộ nhớ nhưng không ghi đè characters.json (cùng luật với mã nhân vật).
func TestCharacterLoadFillsKindWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	legacy := `[{"name":"Lâm Tri Hạ","role":"Nữ chính","description":"","arc":"","traits":[]},` +
		`{"name":"Phùng Chấn","role":"Đồng phạm","description":"","arc":"","traits":[]}]`
	path := filepath.Join(dir, "characters.json")
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	chars, err := st.Characters.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(chars) != 2 || chars[0].Kind != domain.CharacterKindLead || chars[1].Kind != domain.CharacterKindAntagonist {
		t.Fatalf("kind suy từ role sai: %+v", chars)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != legacy {
		t.Fatalf("Load không được ghi file; file đã đổi: %s", onDisk)
	}
	// Save lần sau mới ghi kind xuống đĩa.
	if err := st.Characters.Save(chars); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := st.Characters.Load()
	if err != nil {
		t.Fatalf("Load sau Save: %v", err)
	}
	if saved[0].Kind != domain.CharacterKindLead {
		t.Fatalf("kind chưa được lưu: %+v", saved[0])
	}
}
