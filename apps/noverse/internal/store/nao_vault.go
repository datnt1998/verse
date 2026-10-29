package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

const naoGeneratedNotice = "Tệp sinh tự động từ Noverse — sửa trong Noverse, không sửa ở đây."

func naoSlug(c domain.Character) string {
	// Chỉ tên tệp từ mã an toàn; không cho id nhập tay thoát khỏi thư mục kho ghi chú.
	return characterSlug(strings.TrimPrefix(c.ID, "nv:"))
}

func naoLink(chars []domain.Character, ref string) string {
	if c, ok := matchCharacter(chars, ref); ok {
		return fmt.Sprintf("[[nhan-vat/%s|%s]]", naoSlug(c), strings.ReplaceAll(c.Name, "]", ""))
	}
	return ref
}

func yamlValue(value any) string {
	data, _ := json.Marshal(value) // JSON strings/arrays are valid YAML values.
	return string(data)
}

func naoCell(value string) string {
	return strings.NewReplacer("|", "\\|", "\r", " ", "\n", " ").Replace(value)
}

// regenerateNaoVaultUnlocked chỉ sinh bản đọc. Cùng khóa với ba nguồn dữ kiện;
// từng tệp ghi tmp+rename, sau đó xóa trang hết hạn. Không đọc ngược từ vault.
func regenerateNaoVaultUnlocked(io *IO) error {
	chars, err := loadNaoCharactersUnlocked(io)
	if err != nil {
		return err
	}
	terms, err := loadAddressTermsUnlocked(io)
	if err != nil {
		return err
	}
	log, _, err := loadRelationshipLogUnlocked(io)
	if err != nil {
		return err
	}
	slices.SortFunc(chars, func(a, b domain.Character) int { return strings.Compare(naoSlug(a), naoSlug(b)) })
	var index strings.Builder
	fmt.Fprintf(&index, "# Não truyện\n\n%s\n\n## Nhân vật\n\n", naoGeneratedNotice)
	wanted := make(map[string]bool, len(chars))
	for _, c := range chars {
		name := naoSlug(c) + ".md"
		wanted[name] = true
		fmt.Fprintf(&index, "- %s\n", naoLink(chars, c.ID))
		if err := io.WriteMarkdownUnlocked(filepath.Join("nao", "nhan-vat", name), renderNaoCharacter(c, chars, terms, log)); err != nil {
			return err
		}
	}
	if err := io.WriteMarkdownUnlocked("nao/index.md", index.String()); err != nil {
		return err
	}
	files, err := io.ListDirUnlocked("nao/nhan-vat")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, file := range files {
		if !file.IsDir && strings.HasSuffix(file.Name, ".md") && !wanted[file.Name] {
			if err := io.RemoveFileUnlocked(filepath.Join("nao", "nhan-vat", file.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderNaoCharacter(c domain.Character, chars []domain.Character, terms []domain.AddressTerm, log []domain.RelationshipChange) string {
	aliases := c.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %s\nloai: nhan-vat\nten: %s\nbiet-danh: %s\nvai-tro: %s\n---\n\n%s\n\n# %s\n\n%s\n\n## Xưng hô\n\n", yamlValue(c.ID), yamlValue(c.Name), yamlValue(aliases), yamlValue(c.Role), naoGeneratedNotice, c.Name, c.Description)
	b.WriteString("| Đối phương | Gọi | Tự xưng | Từ chương | Đến chương | Lý do |\n| --- | --- | --- | --- | --- | --- |\n")
	var outgoing []domain.AddressTerm
	for _, term := range terms {
		if term.From == c.ID {
			outgoing = append(outgoing, term)
		}
	}
	sort.SliceStable(outgoing, func(i, j int) bool {
		if outgoing[i].To != outgoing[j].To {
			return outgoing[i].To < outgoing[j].To
		}
		return outgoing[i].FromChapter < outgoing[j].FromChapter
	})
	for _, term := range outgoing {
		until := "—"
		if term.UntilChapter > 0 {
			until = fmt.Sprint(term.UntilChapter)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %s |\n", naoCell(naoLink(chars, term.To)), naoCell(term.Calls), naoCell(term.Self), term.FromChapter, until, naoCell(term.Reason))
	}
	b.WriteString("\n## Quan hệ theo chương\n\n")
	var history []domain.RelationshipChange
	for _, e := range log {
		if e.From == c.ID || e.To == c.ID {
			history = append(history, e)
		}
	}
	slices.SortStableFunc(history, func(a, b domain.RelationshipChange) int { return a.Chapter - b.Chapter })
	for _, e := range history {
		other := e.To
		if other == c.ID {
			other = e.From
		}
		fmt.Fprintf(&b, "- Chương %d — %s: %s (%s).\n", e.Chapter, naoLink(chars, other), e.Relation, e.Source)
	}
	return b.String()
}
