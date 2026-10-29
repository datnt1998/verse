package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/identity"
)

// charactersIn lưu sẵn hai nhân vật để PUT /characters có dữ liệu mà sửa.
// “Đồng phạm” không khai kind nên lần lưu đầu tự suy ra `antagonist`.
func charactersIn(t *testing.T, s *store.Store) {
	t.Helper()
	if err := s.Characters.Save([]domain.Character{
		{Name: "Lâm Tri Hạ", Role: "Nữ chính", Kind: domain.CharacterKindLead},
		{Name: "Phùng Chấn", Role: "Đồng phạm"},
	}); err != nil {
		t.Fatalf("chars: %v", err)
	}
}

// TestServeStoryRead_PutCharacters 钉死 Gói S › PUT /characters: sửa loại vai và
// chương lộ vai, `revealAt: null` xoá bí mật, và lần đọc sau thấy giá trị đã lưu.
func TestServeStoryRead_PutCharacters(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	charactersIn(t, s)

	body := `{"characters":[{"name":"Phùng Chấn","kind":"ally","revealAt":5},{"name":"Lâm Tri Hạ","revealAt":null}]}`
	rec := storyReq(t, g, "PUT", "/x/characters", adminUser(), st, "/characters", body)
	if rec.Code != 200 {
		t.Fatalf("PUT characters: %d %s", rec.Code, rec.Body)
	}
	var got struct {
		Characters []domain.Character `json:"characters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Characters) != 2 {
		t.Fatalf("characters = %+v", got.Characters)
	}
	if got.Characters[0].Kind != domain.CharacterKindLead || got.Characters[0].RevealAt != nil {
		t.Errorf("Lâm Tri Hạ = %+v, want lead + revealAt null", got.Characters[0])
	}
	if got.Characters[1].Kind != domain.CharacterKindAlly || got.Characters[1].RevealAt == nil || *got.Characters[1].RevealAt != 5 {
		t.Errorf("Phùng Chấn = %+v, want ally + revealAt 5", got.Characters[1])
	}
	// Đã ghi xuống đĩa: đọc lại từ store (không qua response) phải thấy cùng giá trị.
	saved, err := s.Characters.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if saved[1].Kind != domain.CharacterKindAlly || saved[1].RevealAt == nil || *saved[1].RevealAt != 5 {
		t.Errorf("saved Phùng Chấn = %+v", saved[1])
	}
}

// TestServeStoryRead_PutCharactersRejects 钉死 lỗi: kind ngoài bảng, revealAt < 1,
// nhân vật không có, lô có dòng sai thì không ghi dòng nào, và người chỉ đọc bị 404
// (không phân biệt được với truyện không tồn tại).
func TestServeStoryRead_PutCharactersRejects(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	charactersIn(t, s)

	cases := []struct {
		name string
		body string
		code int
	}{
		{"kind lạ", `{"characters":[{"name":"Phùng Chấn","kind":"villain"}]}`, 400},
		{"revealAt 0", `{"characters":[{"name":"Phùng Chấn","revealAt":0}]}`, 400},
		{"rỗng", `{"characters":[]}`, 400},
		{"không có nhân vật", `{"characters":[{"name":"Người Lạ"}]}`, 404},
	}
	for _, tc := range cases {
		rec := storyReq(t, g, "PUT", "/x/characters", adminUser(), st, "/characters", tc.body)
		if rec.Code != tc.code {
			t.Errorf("%s: %d %s, want %d", tc.name, rec.Code, rec.Body, tc.code)
		}
	}
	// Lô có một dòng sai thì không ghi dòng nào: Phùng Chấn vẫn là antagonist đã suy ra.
	body := `{"characters":[{"name":"Phùng Chấn","kind":"ally"},{"name":"Người Lạ","kind":"lead"}]}`
	if rec := storyReq(t, g, "PUT", "/x/characters", adminUser(), st, "/characters", body); rec.Code != 404 {
		t.Fatalf("mixed batch: %d %s", rec.Code, rec.Body)
	}
	saved, err := s.Characters.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if saved[1].Kind != domain.CharacterKindAntagonist {
		t.Errorf("batch must be atomic, got %+v", saved[1])
	}

	reader := identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{st.Project: identity.ProjectView}}
	rec := storyReq(t, g, "PUT", "/x/characters", reader, st, "/characters", `{"characters":[{"name":"Phùng Chấn","kind":"ally"}]}`)
	if rec.Code != 404 {
		t.Errorf("reader PUT = %d %s, want 404", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "Loại vai") {
		t.Errorf("reader response must not hint the endpoint exists: %s", rec.Body)
	}
}
