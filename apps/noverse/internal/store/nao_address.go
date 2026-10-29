package store

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

const addressTermsFile = "address_terms.json"

func loadAddressTermsUnlocked(io *IO) ([]domain.AddressTerm, error) {
	var terms []domain.AddressTerm
	if err := io.ReadJSONUnlocked(addressTermsFile, &terms); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return terms, nil
}

// LoadAddressTerms trả cả những bản đã đóng để giữ lịch sử xưng hô.
func (s *fileWorldStore) LoadAddressTerms() ([]domain.AddressTerm, error) {
	var terms []domain.AddressTerm
	if err := s.io.ReadJSON(addressTermsFile, &terms); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return terms, nil
}

// AddressAt trả những bản có hiệu lực trong chương N, giữ chiều From → To.
func (s *fileWorldStore) AddressAt(chapter int) ([]domain.AddressTerm, error) {
	terms, err := s.LoadAddressTerms()
	if err != nil {
		return nil, err
	}
	var active []domain.AddressTerm
	for _, term := range terms {
		if term.FromChapter <= chapter && (term.UntilChapter == 0 || term.UntilChapter >= chapter) {
			active = append(active, term)
		}
	}
	return active, nil
}

func planAddressTerms(chapter int, inputs []domain.AddressTermInput, terms []domain.AddressTerm, chars []domain.Character) ([]domain.AddressTerm, error) {
	if chapter <= 0 {
		return nil, fmt.Errorf("chương bắt đầu xưng hô phải lớn hơn 0")
	}
	for i, input := range inputs {
		if strings.TrimSpace(input.From) == "" || strings.TrimSpace(input.To) == "" || strings.TrimSpace(input.Calls) == "" || strings.TrimSpace(input.Self) == "" {
			return nil, fmt.Errorf("address_terms[%d] cần đủ from, to, calls và self", i)
		}
		from, err := resolveCharacter(chars, input.From)
		if err != nil {
			return nil, err
		}
		to, err := resolveCharacter(chars, input.To)
		if err != nil {
			return nil, err
		}
		if from == to {
			return nil, fmt.Errorf("address_terms[%d]: nhân vật không thể xưng hô với chính mình", i)
		}
		active := -1
		for j, term := range terms {
			if term.From == from && term.To == to && term.UntilChapter == 0 {
				active = j
			}
		}
		if active >= 0 {
			old := terms[active]
			if old.Calls == input.Calls && old.Self == input.Self {
				continue
			}
			if strings.TrimSpace(input.Reason) == "" {
				return nil, fmt.Errorf("xưng hô %s → %s đã là gọi %q, tự xưng %q; muốn đổi thành %q/%q phải điền reason giải thích sự kiện trong chương %d", characterName(chars, from), characterName(chars, to), old.Calls, old.Self, input.Calls, input.Self, chapter)
			}
			if chapter <= old.FromChapter {
				return nil, fmt.Errorf("không thể đổi xưng hô %s → %s ở chương %d: bản đang dùng bắt đầu chương %d; hãy ghi thay đổi ở chương sau", characterName(chars, from), characterName(chars, to), chapter, old.FromChapter)
			}
			terms[active].UntilChapter = chapter - 1
		}
		terms = append(terms, domain.AddressTerm{From: from, To: to, Calls: input.Calls,
			Self: input.Self, FromChapter: chapter, Reason: input.Reason,
			Scope: "truyen", Source: fmt.Sprintf("chương %d", chapter)})
	}
	return terms, nil
}

// ValidateAddressTerms dùng cùng quy tắc với ghi, trước khi đóng băng commit.
func (s *fileWorldStore) ValidateAddressTerms(chapter int, inputs []domain.AddressTermInput) error {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	chars, err := loadNaoCharactersUnlocked(s.io)
	if err != nil {
		return err
	}
	terms, err := loadAddressTermsUnlocked(s.io)
	if err != nil {
		return err
	}
	_, err = planAddressTerms(chapter, inputs, terms, chars)
	return err
}

// planAddressTerm tính danh sách xưng hô sau khi người dùng sửa một chiều (chưa ghi).
// changed=false khi dòng đang dùng đã đúng giá trị. Dùng chung cho kiểm trước và ghi,
// để lỗi dữ liệu không bao giờ để lại nửa thay đổi.
func planAddressTerm(chapter int, fromRef, toRef, calls, self, reason string, terms []domain.AddressTerm, chars []domain.Character) ([]domain.AddressTerm, bool, error) {
	calls, self, reason = strings.TrimSpace(calls), strings.TrimSpace(self), strings.TrimSpace(reason)
	if chapter <= 0 {
		return nil, false, fmt.Errorf("từ chương phải lớn hơn 0")
	}
	if calls == "" || self == "" {
		return nil, false, fmt.Errorf("cần đủ cách gọi và cách tự xưng")
	}
	from, err := resolveCharacter(chars, fromRef)
	if err != nil {
		return nil, false, err
	}
	to, err := resolveCharacter(chars, toRef)
	if err != nil {
		return nil, false, err
	}
	if from == to {
		return nil, false, fmt.Errorf("nhân vật không thể xưng hô với chính mình")
	}
	active := -1
	latestFrom := 0
	for j, term := range terms {
		if term.From != from || term.To != to {
			continue
		}
		if term.FromChapter > latestFrom {
			latestFrom = term.FromChapter
		}
		if term.UntilChapter == 0 {
			active = j
		}
	}
	if active >= 0 {
		old := terms[active]
		if old.Calls == calls && old.Self == self {
			return terms, false, nil
		}
	}
	if chapter <= latestFrom {
		return nil, false, fmt.Errorf("từ chương phải sau chương %d của dòng xưng hô đã có", latestFrom)
	}
	next := slices.Clone(terms)
	if active >= 0 {
		next[active].UntilChapter = chapter - 1
	}
	next = append(next, domain.AddressTerm{
		From: from, To: to, Calls: calls, Self: self,
		FromChapter: chapter, Reason: reason, Scope: "truyen", Source: "người dùng",
	})
	return next, true, nil
}

// ValidateSetAddressTerm kiểm trước khi ghi để hàng đợi viết lại và dòng xưng hô cùng
// thành công hoặc cùng không đổi.
func (s *fileWorldStore) ValidateSetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) error {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	chars, err := loadNaoCharactersUnlocked(s.io)
	if err != nil {
		return err
	}
	terms, err := loadAddressTermsUnlocked(s.io)
	if err != nil {
		return err
	}
	_, _, err = planAddressTerm(chapter, fromRef, toRef, calls, self, reason, terms, chars)
	return err
}

// SetAddressTerm là đường ghi của người dùng trên web (GAP-38): đóng dòng đang dùng ở
// chương ngay trước FromChapter rồi mở dòng mới, giữ nguyên lịch sử đã đóng. Khác
// UpdateAddressTerms (engine, số chương lấy từ bản nộp), ở đây chương do người dùng chọn
// và nguồn ghi là "người dùng" để Não truyện phân biệt dữ kiện máy với dữ kiện người.
func (s *fileWorldStore) SetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) ([]domain.AddressTerm, error) {
	var out []domain.AddressTerm
	err := s.io.WithWriteLock(func() error {
		chars, err := loadNaoCharactersUnlocked(s.io)
		if err != nil {
			return err
		}
		terms, err := loadAddressTermsUnlocked(s.io)
		if err != nil {
			return err
		}
		next, changed, err := planAddressTerm(chapter, fromRef, toRef, calls, self, reason, terms, chars)
		if err != nil {
			return err
		}
		if !changed {
			out = terms
			return nil
		}
		if err := s.io.WriteJSONUnlocked(addressTermsFile, next); err != nil {
			return err
		}
		out = next
		return regenerateNaoVaultUnlocked(s.io)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateAddressTerms ghi cặp có chiều, đóng bản cũ và mở bản mới trong cùng khóa ghi.
func (s *fileWorldStore) UpdateAddressTerms(chapter int, inputs []domain.AddressTermInput) error {
	return s.io.WithWriteLock(func() error {
		chars, err := loadNaoCharactersUnlocked(s.io)
		if err != nil {
			return err
		}
		terms, err := loadAddressTermsUnlocked(s.io)
		if err != nil {
			return err
		}
		next, err := planAddressTerms(chapter, inputs, terms, chars)
		if err != nil {
			return err
		}
		if err := s.io.WriteJSONUnlocked(addressTermsFile, next); err != nil {
			return err
		}
		return regenerateNaoVaultUnlocked(s.io)
	})
}
