package store

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

const chapterRecordsDir = "meta/chapter_records"

type fileChapterRecordStore struct{ io *IO }

func newFileChapterRecordStore(io *IO) *fileChapterRecordStore {
	return &fileChapterRecordStore{io: io}
}

func ChapterRecordPath(chapter int) string {
	return fmt.Sprintf("%s/%06d.json", chapterRecordsDir, chapter)
}

func (s *fileChapterRecordStore) Load(chapter int) (*domain.ChapterRecord, error) {
	var record domain.ChapterRecord
	if err := s.io.ReadJSON(ChapterRecordPath(chapter), &record); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if err := validateChapterRecord(record); err != nil {
		return nil, fmt.Errorf("Đọc bản ghi nghiệm thu chương %d: %w", chapter, err)
	}
	if record.Chapter != chapter {
		return nil, fmt.Errorf("Đọc bản ghi nghiệm thu chương %d: bản ghi ghi số chương là %d", chapter, record.Chapter)
	}
	return &record, nil
}

func (s *fileChapterRecordStore) Save(record domain.ChapterRecord) error {
	if err := validateChapterRecord(record); err != nil {
		return err
	}
	return s.io.WriteJSON(ChapterRecordPath(record.Chapter), record)
}

// PrepareChapterRecord 计算下一版章节记录但不落盘，供跨记录不变量在写入前
// 完成校验。两 backend 共用同一规则：content hash 相同且 origin/facts/style
// 不变 → changed=false 且原样返回既有记录。
func PrepareChapterRecord(existing *domain.ChapterRecord, chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, bool) {
	return prepareChapterRecord(existing, chapter, origin, content, facts, style)
}

// Prepare 构造下一版章节记录但不落盘，供跨记录不变量在写入前完成校验。
func (s *fileChapterRecordStore) Prepare(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error) {
	existing, err := s.Load(chapter)
	if err != nil {
		return nil, err
	}
	record, _ := prepareChapterRecord(existing, chapter, origin, content, facts, style)
	return record, nil
}

func (s *fileChapterRecordStore) Accept(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error) {
	existing, err := s.Load(chapter)
	if err != nil {
		return nil, err
	}
	record, changed := prepareChapterRecord(existing, chapter, origin, content, facts, style)
	if !changed {
		return record, nil
	}
	if err := s.Save(*record); err != nil {
		return nil, err
	}
	return record, nil
}

func prepareChapterRecord(existing *domain.ChapterRecord, chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, bool) {
	digest := domain.ChapterContentSHA256(content)
	revision := 1
	if existing != nil {
		// 覆盖旧记录时保住本章自己种下的伏笔：重写只换正文，不该抹掉这一章的埋设事实。
		facts.ForeshadowUpdates = domain.RestoreOwnPlants(existing.Facts.ForeshadowUpdates, facts.ForeshadowUpdates)
		if existing.ContentSHA256 == digest && existing.Origin == origin && reflect.DeepEqual(existing.Facts, facts) && reflect.DeepEqual(existing.StyleDelta, style) {
			return existing, false
		}
		revision = existing.Revision + 1
	}
	record := domain.ChapterRecord{
		Version:       domain.ChapterRecordVersion,
		Chapter:       chapter,
		Revision:      revision,
		Build:         version.Build(),
		Origin:        origin,
		Content:       domain.NormalizeChapterContent(content),
		ContentSHA256: digest,
		Facts:         facts,
		StyleDelta:    style,
		AcceptedAt:    time.Now(),
	}
	return &record, true
}

func (s *fileChapterRecordStore) LoadCompleted(chapters []int) ([]domain.ChapterRecord, error) {
	numbers := slices.Clone(chapters)
	slices.Sort(numbers)
	records := make([]domain.ChapterRecord, 0, len(numbers))
	for _, chapter := range numbers {
		record, err := s.Load(chapter)
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

func validateChapterRecord(record domain.ChapterRecord) error {
	// v1 记录没有 build 字段：Build 为空读作"不知道哪个构建写的"，照常受理。
	switch {
	case record.Version != 1 && record.Version != domain.ChapterRecordVersion:
		return fmt.Errorf("unsupported chapter record version %d", record.Version)
	case record.Chapter <= 0:
		return fmt.Errorf("chapter must be > 0")
	case record.Revision <= 0:
		return fmt.Errorf("revision must be > 0")
	case record.Origin != domain.ChapterOriginGenerated && record.Origin != domain.ChapterOriginUser:
		return fmt.Errorf("invalid chapter origin %q", record.Origin)
	case record.ContentSHA256 != domain.ChapterContentSHA256(record.Content):
		return fmt.Errorf("content digest mismatch")
	case record.AcceptedAt.IsZero():
		return fmt.Errorf("accepted_at is required")
	}
	return nil
}
