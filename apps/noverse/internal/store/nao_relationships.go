package store

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

const relationshipLogFile = "relationship_log.jsonl"

func loadRelationshipLogUnlocked(io *IO) ([]domain.RelationshipChange, bool, error) {
	data, err := committedJSONLinesUnlocked(io, relationshipLogFile)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	entries, err := decodeJSONLines[domain.RelationshipChange](relationshipLogFile, data)
	return entries, true, err
}

// LoadRelationshipLog đọc toàn bộ lịch sử, không đọc ngược từ kho ghi chú.
func (s *fileWorldStore) LoadRelationshipLog() ([]domain.RelationshipChange, error) {
	data, err := s.io.ReadFile(relationshipLogFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeJSONLines[domain.RelationshipChange](relationshipLogFile, data)
}

// Dữ liệu cũ có thể chứa vai phụ chưa có hồ sơ. Giữ tên hiển thị trong state;
// mã dự phòng tất định không đụng mã của hồ sơ đã biết. Công cụ mới kiểm tra tên trước ghi.
func legacyRelationshipCharacters(chars []domain.Character, entries []domain.RelationshipEntry) []domain.Character {
	for _, e := range entries {
		for _, ref := range []string{e.CharacterA, e.CharacterB} {
			if _, ok := matchCharacter(chars, ref); ok || strings.HasPrefix(ref, "nv:") {
				continue
			}
			hash := sha256.Sum256([]byte(ref))
			chars = append(chars, domain.Character{ID: fmt.Sprintf("nv:%s-%x", characterSlug(ref), hash[:4]), Name: ref})
		}
	}
	return chars
}

func relationshipRef(chars []domain.Character, ref string) string {
	if c, ok := matchCharacter(chars, ref); ok {
		return c.ID
	}
	return ref // Frozen local ids need no name lookup.
}

func relationshipState(log []domain.RelationshipChange, chars []domain.Character) []domain.RelationshipEntry {
	state := make([]domain.RelationshipEntry, 0)
	idx := make(map[string]int)
	for _, e := range log {
		key := pairKey(e.From, e.To)
		if i, ok := idx[key]; ok {
			// Preserve the orientation of the first appearance, as the old merge did.
			state[i].Relation = e.Relation
			state[i].Chapter = e.Chapter
		} else {
			idx[key] = len(state)
			state = append(state, domain.RelationshipEntry{
				CharacterA: characterName(chars, e.From), CharacterB: characterName(chars, e.To),
				Relation: e.Relation, Chapter: e.Chapter,
			})
		}
	}
	return state
}

// planRelationshipChange dựng dữ kiện quan hệ từ tham chiếu người dùng (chưa ghi).
// Type rỗng đọc là "rel"; giá trị lạ là lỗi để web trả 400 thay vì lưu bừa.
func planRelationshipChange(chapter int, aRef, bRef, relation, reason, typ string, chars []domain.Character) (domain.RelationshipChange, error) {
	relation, reason, typ = strings.TrimSpace(relation), strings.TrimSpace(reason), strings.TrimSpace(typ)
	if chapter <= 0 {
		return domain.RelationshipChange{}, fmt.Errorf("từ chương phải lớn hơn 0")
	}
	if relation == "" {
		return domain.RelationshipChange{}, fmt.Errorf("quan hệ mới không được để trống")
	}
	if typ != "" && !domain.ValidRelationshipType(typ) {
		return domain.RelationshipChange{}, fmt.Errorf("kiểu quan hệ không hợp lệ: %q", typ)
	}
	a, err := resolveCharacter(chars, aRef)
	if err != nil {
		return domain.RelationshipChange{}, err
	}
	b, err := resolveCharacter(chars, bRef)
	if err != nil {
		return domain.RelationshipChange{}, err
	}
	if a == b {
		return domain.RelationshipChange{}, fmt.Errorf("quan hệ cần hai nhân vật khác nhau")
	}
	return domain.RelationshipChange{Chapter: chapter, From: a, To: b,
		Relation: relation, Type: domain.NormalizeRelationshipType(typ), Reason: reason,
		Scope: "truyen", Source: "người dùng"}, nil
}

// ValidateRelationshipChange kiểm trước để hàng đợi viết lại và dữ kiện quan hệ cùng
// thành công hoặc cùng không đổi.
func (s *fileWorldStore) ValidateRelationshipChange(chapter int, aRef, bRef, relation, typ string) error {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	chars, err := loadNaoCharactersUnlocked(s.io)
	if err != nil {
		return err
	}
	_, err = planRelationshipChange(chapter, aRef, bRef, relation, "", typ, chars)
	return err
}

// AppendRelationshipChange là đường ghi quan hệ của người dùng trên web (GAP-38):
// thêm một dữ kiện vào relationship_log.jsonl (append-only) rồi dựng lại trạng thái
// chiếu và kho ghi chú trong cùng khóa ghi. Reason chỉ có ở dữ kiện người dùng.
func (s *fileWorldStore) AppendRelationshipChange(chapter int, aRef, bRef, relation, reason, typ string) error {
	return s.io.WithWriteLock(func() error {
		chars, err := loadNaoCharactersUnlocked(s.io)
		if err != nil {
			return err
		}
		change, err := planRelationshipChange(chapter, aRef, bRef, relation, reason, typ, chars)
		if err != nil {
			return err
		}
		log, _, err := loadRelationshipLogUnlocked(s.io)
		if err != nil {
			return err
		}
		data, err := encodeJSONLines([]domain.RelationshipChange{change})
		if err != nil {
			return err
		}
		if err := s.io.AppendLineUnlocked(relationshipLogFile, data); err != nil {
			return err
		}
		log = append(log, change)
		state := relationshipState(log, chars)
		if err := s.io.WriteJSONUnlocked("relationship_state.json", state); err != nil {
			return err
		}
		if err := s.io.WriteMarkdownUnlocked("relationship_state.md", renderRelationships(state, s.io.labels())); err != nil {
			return err
		}
		return regenerateNaoVaultUnlocked(s.io)
	})
}

func (s *fileWorldStore) updateRelationships(changes []domain.RelationshipEntry, rewrite bool) error {
	return s.io.WithWriteLock(func() error {
		chars, err := loadNaoCharactersUnlocked(s.io)
		if err != nil {
			return err
		}
		var previous []domain.RelationshipEntry
		if err := s.io.ReadJSONUnlocked("relationship_state.json", &previous); err != nil && !os.IsNotExist(err) {
			return err
		}
		chars = legacyRelationshipCharacters(chars, previous)
		chars = legacyRelationshipCharacters(chars, changes)
		log, exists, err := loadRelationshipLogUnlocked(s.io)
		if err != nil {
			return err
		}
		if !exists {
			seed := make([]domain.RelationshipChange, 0, len(previous))
			for _, e := range previous {
				seed = append(seed, domain.RelationshipChange{Chapter: e.Chapter,
					From: relationshipRef(chars, e.CharacterA), To: relationshipRef(chars, e.CharacterB),
					Relation: e.Relation, Type: domain.NormalizeRelationshipType(e.Type),
					Scope: "truyen", Source: "trạng thái cũ"})
			}
			data, err := encodeJSONLines(seed)
			if err != nil {
				return err
			}
			if err := s.io.WriteFileUnlocked(relationshipLogFile, data); err != nil {
				return err
			}
			log = seed
		}
		latest := make(map[string]domain.RelationshipChange)
		seen := make(map[string]bool, len(log))
		for _, e := range log {
			latest[pairKey(e.From, e.To)] = e
			seen[stableRecordKey(e.Chapter, pairKey(e.From, e.To), e.Relation)] = true
		}
		var added []domain.RelationshipChange
		for _, c := range changes {
			from, to := relationshipRef(chars, c.CharacterA), relationshipRef(chars, c.CharacterB)
			key := pairKey(from, to)
			typ := domain.NormalizeRelationshipType(c.Type)
			if e, ok := latest[key]; ok && e.Relation == c.Relation && domain.NormalizeRelationshipType(e.Type) == typ {
				continue
			}
			// A frozen ordinary commit may be replayed after its append succeeded.
			if !rewrite && seen[stableRecordKey(c.Chapter, key, c.Relation)] {
				continue
			}
			source := fmt.Sprintf("chương %d", c.Chapter)
			if rewrite {
				source = fmt.Sprintf("viết lại chương %d", c.Chapter)
			}
			e := domain.RelationshipChange{Chapter: c.Chapter, From: from, To: to,
				Relation: c.Relation, Type: typ, Scope: "truyen", Source: source}
			added = append(added, e)
			latest[key] = e
			seen[stableRecordKey(c.Chapter, key, c.Relation)] = true
		}
		if len(added) > 0 {
			data, err := encodeJSONLines(added)
			if err != nil {
				return err
			}
			if err := s.io.AppendLineUnlocked(relationshipLogFile, data); err != nil {
				return err
			}
		}
		if len(added) == 0 {
			if err := s.io.syncFileUnlocked(relationshipLogFile); err != nil {
				return err
			}
		}
		log = append(log, added...)
		state := relationshipState(log, chars)
		if err := s.io.WriteJSONUnlocked("relationship_state.json", state); err != nil {
			return err
		}
		if err := s.io.WriteMarkdownUnlocked("relationship_state.md", renderRelationships(state, s.io.labels())); err != nil {
			return err
		}
		return regenerateNaoVaultUnlocked(s.io)
	})
}
