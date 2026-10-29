package postgres

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// pgChapterRecords implements store.ChapterRecordStore on the immutable
// chapter_versions table: acceptance appends an attributed revision and moves
// the pointer in one transaction. Idempotence keeps the file backend's rule —
// same digest/origin/facts/style returns the existing record without a write.
type pgChapterRecords struct {
	nar    *Narrative
	ctx    context.Context
	scope  ScopeRef
	author ChapterAuthor
}

func (p *pgChapterRecords) Load(chapter int) (*domain.ChapterRecord, error) {
	return p.nar.LoadRecord(p.ctx, p.scope, chapter)
}

// LoadCompleted requires every requested chapter to have an accepted record,
// the same strict read the file backend performs.
func (p *pgChapterRecords) LoadCompleted(chapters []int) ([]domain.ChapterRecord, error) {
	numbers := slices.Clone(chapters)
	slices.Sort(numbers)
	records := make([]domain.ChapterRecord, 0, len(numbers))
	for _, chapter := range numbers {
		record, err := p.Load(chapter)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return nil, fmt.Errorf("Chương %d thiếu bản ghi nghiệm thu", chapter)
		}
		records = append(records, *record)
	}
	return records, nil
}

// Save imports a record as the next immutable revision. An identical current
// record is a no-op; a record whose revision is not the next one is a
// conflict — history is append-only, never overwritten.
func (p *pgChapterRecords) Save(record domain.ChapterRecord) error {
	existing, err := p.Load(record.Chapter)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.Revision == record.Revision {
			if existing.ContentSHA256 == record.ContentSHA256 &&
				existing.Origin == record.Origin &&
				reflect.DeepEqual(existing.Facts, record.Facts) &&
				reflect.DeepEqual(existing.StyleDelta, record.StyleDelta) {
				return nil
			}
			return fmt.Errorf("ghi đè bản ghi nghiệm thu chương %d: %w", record.Chapter, errs.ErrStorageConflict)
		}
		if record.Revision != existing.Revision+1 {
			return fmt.Errorf("bản ghi chương %d revision %d, muốn %d: %w",
				record.Chapter, existing.Revision, record.Revision, errs.ErrStorageConflict)
		}
	}
	// Save writes a record produced outside this process (migration/import);
	// attributing it to the importing actor is the accurate answer.
	_, err = p.nar.AcceptRecord(p.ctx, p.scope, record.Chapter, record.Origin,
		record.Content, record.Facts, record.StyleDelta, ChapterAuthor{Kind: "import"})
	return err
}

func (p *pgChapterRecords) Prepare(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error) {
	existing, err := p.Load(chapter)
	if err != nil {
		return nil, err
	}
	record, _ := store.PrepareChapterRecord(existing, chapter, origin, content, facts, style)
	return record, nil
}

func (p *pgChapterRecords) Accept(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error) {
	return p.nar.AcceptRecord(p.ctx, p.scope, chapter, origin, content, facts, style, p.author)
}
