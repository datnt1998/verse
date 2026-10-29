package tools

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func nullableAddressTermsSchema() map[string]any {
	term := schema.Object(
		schema.Property("from", schema.String("Tên hoặc biệt danh nhân vật nói, đúng characters.json (không tự đặt mã)")).Required(),
		schema.Property("to", schema.String("Tên hoặc biệt danh người được gọi, đúng characters.json")).Required(),
		schema.Property("calls", schema.String("Cách from gọi to, ví dụ chị")).Required(),
		schema.Property("self", schema.String("Cách from tự xưng khi nói với to, ví dụ em")).Required(),
		schema.Property("reason", llmcontract.Nullable(schema.String("Lý do sự kiện dẫn đến đổi xưng hô; bắt buộc khi đổi cặp đã có, lần đầu có thể null"))).Required(),
	)
	return llmcontract.Nullable(schema.Array("Xưng hô có chiều: điền lần đầu một cặp nói chuyện trực tiếp, và mỗi khi cách gọi/tự xưng đổi (phải có reason). Dùng tên/biệt danh trong characters.json; A→B khác B→A. Không có dữ kiện mới thì null hoặc []. Số chương do công cụ lấy từ bản nộp (thiết lập nền dùng chương 1).", term))
}

func resolveAddressInputs(s *store.Store, inputs []domain.AddressTermInput) error {
	for i := range inputs {
		ids, err := s.Characters.ResolveIDs(inputs[i].From, inputs[i].To)
		if err != nil {
			return fmt.Errorf("address_terms[%d]: %v: %w", i, err, errs.ErrToolArgs)
		}
		inputs[i].From, inputs[i].To = ids[0], ids[1]
	}
	return nil
}

func (t *CommitChapterTool) validateNaoFacts(a commitArgs) error {
	for i := range a.RelationshipChanges {
		change := &a.RelationshipChanges[i]
		ids, err := t.store.Characters.ResolveIDs(change.CharacterA, change.CharacterB)
		if err != nil {
			return fmt.Errorf("relationship_changes[%d]: %v: %w", i, err, errs.ErrToolArgs)
		}
		if ids[0] == ids[1] {
			return fmt.Errorf("relationship_changes[%d]: hai tên trỏ tới cùng một nhân vật: %w", i, errs.ErrToolArgs)
		}
		change.CharacterA, change.CharacterB = ids[0], ids[1]
	}
	if err := resolveAddressInputs(t.store, a.AddressTerms); err != nil {
		return err
	}
	if len(a.AddressTerms) > 0 {
		if err := t.store.World.ValidateAddressTerms(a.Chapter, a.AddressTerms); err != nil {
			return fmt.Errorf("%v: %w", err, errs.ErrToolArgs)
		}
	}
	return nil
}
