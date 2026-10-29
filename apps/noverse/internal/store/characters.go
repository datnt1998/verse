package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// fileCharacterStore 管理角色档案和状态快照（file backend）。
type fileCharacterStore struct {
	io      *IO
	outline *fileOutlineStore // 只读依赖，用于快照遍历
}

func newFileCharacterStore(io *IO, outline *fileOutlineStore) *fileCharacterStore {
	return &fileCharacterStore{io: io, outline: outline}
}

// Save 同时保存 characters.json 和 characters.md（原子写入）。
func (s *fileCharacterStore) Save(chars []domain.Character) error {
	return s.io.WithWriteLock(func() error {
		previous, err := loadNaoCharactersUnlocked(s.io)
		if err != nil {
			return err
		}
		for i := range chars {
			if chars[i].ID == "" {
				if c, ok := matchCharacter(previous, chars[i].Name); ok {
					chars[i].ID = c.ID
				}
			}
		}
		assignCharacterIDs(chars)
		migrateCharacterKinds(chars)
		if err := s.io.WriteJSONUnlocked("characters.json", chars); err != nil {
			return err
		}
		if err := s.io.WriteMarkdownUnlocked("characters.md", renderCharacters(chars, s.io.labels())); err != nil {
			return err
		}
		return regenerateNaoVaultUnlocked(s.io)
	})
}

// Load 从 characters.json 读取角色档案。
func (s *fileCharacterStore) Load() ([]domain.Character, error) {
	var chars []domain.Character
	if err := s.io.ReadJSON("characters.json", &chars); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	assignCharacterIDs(chars)
	migrateCharacterKinds(chars)
	return chars, nil
}

// SaveSnapshots 保存角色状态快照到 meta/snapshots/v{vol}a{arc}.json。
func (s *fileCharacterStore) SaveSnapshots(volume, arc int, snapshots []domain.CharacterSnapshot) error {
	return s.io.WriteJSON(fmt.Sprintf("meta/snapshots/v%02da%02d.json", volume, arc), snapshots)
}

// LoadSnapshots 读取指定卷弧的角色快照。
func (s *fileCharacterStore) LoadSnapshots(volume, arc int) ([]domain.CharacterSnapshot, error) {
	var snapshots []domain.CharacterSnapshot
	if err := s.io.ReadJSON(fmt.Sprintf("meta/snapshots/v%02da%02d.json", volume, arc), &snapshots); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return snapshots, nil
}

// LoadLatestSnapshots 加载最近一次角色快照（按卷弧倒序查找）。
func (s *fileCharacterStore) LoadLatestSnapshots() ([]domain.CharacterSnapshot, error) { // 大纲缺失→(nil,nil) 已在 LoadLayeredOutline 内消化;损坏必须上抛，否则角色快照
	// 会随分层大纲损坏一起静默消失，写作丢失角色连续性却无人知晓。
	volumes, err := s.outline.LoadLayeredOutline()
	if err != nil {
		return nil, err
	}
	if len(volumes) == 0 {
		return nil, nil
	}
	for vi := len(volumes) - 1; vi >= 0; vi-- {
		v := volumes[vi]
		for ai := len(v.Arcs) - 1; ai >= 0; ai-- {
			snaps, err := s.LoadSnapshots(v.Index, v.Arcs[ai].Index)
			if err != nil {
				return nil, err
			}
			if len(snaps) > 0 {
				return snaps, nil
			}
		}
	}
	return nil, nil
}

func renderCharacters(chars []domain.Character, l mdLabels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.charProfiles)
	for _, c := range chars {
		fmt.Fprintf(&b, "## %s%s%s%s\n\n", c.Name, l.openParen, c.Role, l.closeParen)
		fmt.Fprintf(&b, "%s\n\n", c.Description)
		if c.Arc != "" {
			fmt.Fprintf(&b, "**%s**%s%s\n\n", l.charArc, l.colon, c.Arc)
		}
		if len(c.Traits) > 0 {
			fmt.Fprintf(&b, "**%s**%s%s\n\n", l.traits, l.colon, strings.Join(c.Traits, l.listSep))
		}
	}
	return b.String()
}
