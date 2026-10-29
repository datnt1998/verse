package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// Gói S: Kiến trúc sư điền `kind` khi lưu nhân vật; giá trị ngoài 4 loại phải bị từ chối,
// giá trị hợp lệ phải được lưu xuống characters.json.
func TestSaveFoundationCharactersKind(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	tool := NewSaveFoundationTool(st)

	bad, _ := json.Marshal(map[string]any{"type": "characters", "axes": nil,
		"content": `[{"name":"A","role":"x","kind":"protagonist","description":"","arc":"","traits":[]}]`})
	if _, err := tool.Execute(context.Background(), bad); err == nil {
		t.Fatal("kind ngoài lead|antagonist|ally|support phải bị từ chối")
	}

	good, _ := json.Marshal(map[string]any{"type": "characters", "axes": nil,
		"content": `[{"name":"Lâm Tri Hạ","role":"Nữ chính","kind":"lead","revealAt":5,"description":"","arc":"","traits":[]}]`})
	if _, err := tool.Execute(context.Background(), good); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	chars, err := st.Characters.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(chars) != 1 || chars[0].Kind != domain.CharacterKindLead {
		t.Fatalf("kind chưa lưu: %+v", chars)
	}
	if chars[0].RevealAt == nil || *chars[0].RevealAt != 5 {
		t.Fatalf("revealAt chưa lưu: %+v", chars[0].RevealAt)
	}
}
