package userrules

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// FileRules đọc quy tắc cấp dự án — không gắn với truyện nào, nên không có
// snapshot đã lưu, luật runtime hay mốc chương. Dùng cùng bộ chuẩn hoá và bộ nhớ đệm theo băm
// nội dung như Service, chỉ khác nơi lưu đệm (một tệp JSON thay cho store của truyện).
type FileRules struct {
	norm *Normalizer
	opts rules.LoadOptions
	sink *fileNormCache
	pack rules.Candidate
}

// NewFileRules dựng bộ đọc cho opts (ProjectRulesDir).
// cachePath rỗng = chỉ đệm trong bộ nhớ của lần gọi. model có thể nil: dòng chưa có trong đệm hạ cấp.
func NewFileRules(model llmtypes.ChatModel, opts rules.LoadOptions, cachePath string, pack ...rules.Candidate) *FileRules {
	f := &FileRules{norm: NewNormalizer(model), opts: opts, sink: &fileNormCache{path: cachePath}}
	if len(pack) > 0 {
		f.pack = pack[0]
	}
	return f
}

// FileEffective là hình dạng của GET S/rules/effective bỏ `runtime` và `timing`:
// Snapshot = system_defaults + mọi tệp .md của các cấp trong opts (tính tại chỗ, không lưu);
// Items = từng dòng của quy-tac.md ở các cấp đó.
type FileEffective struct {
	Snapshot *rules.Snapshot `json:"snapshot"`
	Items    []RuleItem      `json:"items"`
	FatigueWords      []rules.FatigueWord `json:"fatigueWords"`
	ForbiddenDefaults []string            `json:"forbiddenDefaults"`
}

// Effective chuẩn hoá (có đệm) các tệp quy tắc và trả snapshot + từng quy tắc kèm chip.
func (f *FileRules) Effective(ctx context.Context) (*FileEffective, error) {
	b := newBatch(f.norm, f.opts, f.sink)
	cands := []rules.Candidate{rules.SystemDefaults()}
	if f.pack.Source == "genre_pack" {
		cands = append(cands, f.pack)
	}
	items := []RuleItem{}
	for _, rs := range rules.RawFileSources(f.opts) {
		cand, fileItems := b.fileCandidate(ctx, rs)
		cands = append(cands, cand)
		items = append(items, fileItems...)
	}
	if f.opts.HomeRulesDir != "" {
		learned, err := lessons.NewStore(f.opts.HomeRulesDir).Rules("")
		if err != nil {
			return nil, err
		}
		cands = append(cands, defaultLessonCandidates(learned)...)
	}
	b.flush()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snap := rules.BuildSnapshot(cands)
	return &FileEffective{
		Snapshot: &snap, Items: items, FatigueWords: rules.EffectiveFatigue(&snap, f.pack, nil),
		ForbiddenDefaults: rules.SystemDefaults().Structured.ForbiddenPhrases,
	}, nil
}

// fileNormCache lưu bộ nhớ đệm chuẩn hoá ở một tệp JSON cùng định dạng với store.UserRules
// (khoá là băm nội dung nên dùng chung cho mọi cấp). Lỗi đọc/ghi chỉ làm mất đệm, không làm hỏng kết quả.
type fileNormCache struct {
	path string
	mu   sync.Mutex
	mem  map[string]store.NormCacheEntry // khi path rỗng
}

type fileNormCacheDoc struct {
	Entries map[string]store.NormCacheEntry `json:"entries"`
}

func (c *fileNormCache) NormCache() map[string]store.NormCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadLocked()
}

func (c *fileNormCache) loadLocked() map[string]store.NormCacheEntry {
	if c.path == "" {
		if c.mem == nil {
			c.mem = map[string]store.NormCacheEntry{}
		}
		return c.mem
	}
	var doc fileNormCacheDoc
	if data, err := os.ReadFile(c.path); err != nil || json.Unmarshal(data, &doc) != nil || doc.Entries == nil {
		return map[string]store.NormCacheEntry{}
	}
	return doc.Entries
}

func (c *fileNormCache) PutNormCache(entries map[string]store.NormCacheEntry) error {
	if len(entries) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	all := c.loadLocked()
	for k, v := range entries {
		all[k] = v
	}
	store.TrimNormCache(all)
	if c.path == "" {
		return nil
	}
	data, err := json.Marshal(fileNormCacheDoc{Entries: all})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(c.path, data)
}
