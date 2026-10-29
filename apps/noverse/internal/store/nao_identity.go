package store

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// characterSlug chỉ dùng khi cấp mã lần đầu; đổi tên không đổi mã đã cấp.
func characterSlug(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		switch {
		case strings.ContainsRune("àáạảãâầấậẩẫăằắặẳẵ", r):
			r = 'a'
		case strings.ContainsRune("èéẹẻẽêềếệểễ", r):
			r = 'e'
		case strings.ContainsRune("ìíịỉĩ", r):
			r = 'i'
		case strings.ContainsRune("òóọỏõôồốộổỗơờớợởỡ", r):
			r = 'o'
		case strings.ContainsRune("ùúụủũưừứựửữ", r):
			r = 'u'
		case strings.ContainsRune("ỳýỵỷỹ", r):
			r = 'y'
		case r == 'đ':
			r = 'd'
		}
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			separator = false
			b.WriteRune(r)
		} else {
			separator = true
		}
	}
	if b.Len() == 0 {
		return "nhan-vat"
	}
	return b.String()
}

func assignCharacterIDs(chars []domain.Character) {
	used := make(map[string]bool, len(chars))
	for _, c := range chars {
		if c.ID != "" {
			used[c.ID] = true
		}
	}
	for i := range chars {
		if chars[i].ID != "" {
			continue
		}
		base := "nv:" + characterSlug(chars[i].Name)
		id := base
		for n := 2; used[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		chars[i].ID = id
		used[id] = true
	}
}

// migrateCharacterKinds (Gói S · 06-data-contract › Bổ sung §S): dữ liệu cũ thiếu `kind`
// được tính một lần từ `role` — chính → lead; phản diện, đồng phạm → antagonist;
// đồng minh, bạn → ally; còn lại → support. Load điền trong bộ nhớ (không ghi file,
// TestNaoCharacterLegacyIdentityAndResolution bắt buộc vậy); Save lưu lại kèm lần ghi sau.
func migrateCharacterKinds(chars []domain.Character) {
	for i := range chars {
		if chars[i].Kind == "" || !domain.ValidCharacterKind(chars[i].Kind) {
			chars[i].Kind = CharacterKindOf(chars[i].Role)
		}
	}
}

// CharacterKindOf suy loại vai từ dòng vai chữ (cả tiếng Việt lẫn tiếng Anh).
// Kiểm "phản diện chính"/"đồng phạm" trước "chính" để "phản diện chính" không thành lead.
func CharacterKindOf(role string) domain.CharacterKind {
	r := strings.ToLower(strings.TrimSpace(role))
	if r == "" {
		return domain.CharacterKindSupport
	}
	if strings.Contains(r, "phản diện") || strings.Contains(r, "đồng phạm") ||
		strings.Contains(r, "antagonist") || strings.Contains(r, "villain") {
		return domain.CharacterKindAntagonist
	}
	if strings.Contains(r, "đồng minh") || strings.HasPrefix(r, "bạn ") || r == "bạn" ||
		strings.Contains(r, "ally") || strings.Contains(r, "friend") || strings.Contains(r, "sidekick") {
		return domain.CharacterKindAlly
	}
	if strings.Contains(r, "chính") || strings.Contains(r, "chủ") ||
		strings.Contains(r, "protagonist") || strings.Contains(r, "main") {
		return domain.CharacterKindLead
	}
	return domain.CharacterKindSupport
}

func loadNaoCharactersUnlocked(io *IO) ([]domain.Character, error) {
	var chars []domain.Character
	if err := io.ReadJSONUnlocked("characters.json", &chars); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	assignCharacterIDs(chars)
	migrateCharacterKinds(chars)
	return chars, nil
}

// FindCharacter tra nhân vật theo mã, tên hoặc biệt danh — cùng quy tắc với
// resolveCharacter, dùng cho các endpoint đọc/ghi của web (GAP-38) khi chỉ cần tra cứu.
func FindCharacter(chars []domain.Character, ref string) (domain.Character, bool) {
	return matchCharacter(chars, ref)
}

func matchCharacter(chars []domain.Character, ref string) (domain.Character, bool) {
	// Exact names/aliases win over any case-insensitive match.
	for _, c := range chars {
		if c.ID == ref || c.Name == ref {
			return c, true
		}
		for _, alias := range c.Aliases {
			if alias == ref {
				return c, true
			}
		}
	}
	for _, c := range chars {
		if strings.EqualFold(c.Name, ref) {
			return c, true
		}
		for _, alias := range c.Aliases {
			if strings.EqualFold(alias, ref) {
				return c, true
			}
		}
	}
	return domain.Character{}, false
}

func resolveCharacter(chars []domain.Character, ref string) (string, error) {
	if c, ok := matchCharacter(chars, ref); ok {
		return c.ID, nil
	}
	names := make([]string, len(chars))
	for i, c := range chars {
		names[i] = c.Name
	}
	return "", fmt.Errorf("không tìm thấy nhân vật %q; hãy dùng tên hoặc biệt danh trong characters.json. Tên đã biết: %s", ref, strings.Join(names, ", "))
}

// ResolveIDs không ghi dữ liệu; công cụ dùng trước khi đóng băng bản nộp.
func (s *fileCharacterStore) ResolveIDs(refs ...string) ([]string, error) {
	chars, err := s.Load()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i], err = resolveCharacter(chars, ref)
		if err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func characterName(chars []domain.Character, ref string) string {
	if c, ok := matchCharacter(chars, ref); ok {
		return c.Name
	}
	return ref
}
