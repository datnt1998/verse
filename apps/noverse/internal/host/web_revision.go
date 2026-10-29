package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/revision"
)

// RevisionChange 描述一次外部章节修订检查结果（web 端 POST /sync {check:true}，GAP-29）。
// Kind: "edit" = 工作区正文被外部修改；"resume" = 存在未完成的修订恢复记录（上次同步中断）。
// Added/Removed 为相对已接受正文增删的词数（与 ChapterInfo.Words 同一计数口径）；
// ModifiedAt 为工作区章节文件的修改时间（读不到为 nil）。
type RevisionChange struct {
	Chapter    int        `json:"chapter"`
	Title      string     `json:"title"`
	Kind       string     `json:"kind"`
	Added      int        `json:"added"`
	Removed    int        `json:"removed"`
	ModifiedAt *time.Time `json:"modifiedAt"`
}

// RevisionDiff 是一章外部修订的前后正文（GET …/sync/diff/:chapter）。
type RevisionDiff struct {
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	Before  string `json:"before"`
	After   string `json:"after"`
}

// ErrNoRevision 表示该章没有外部修订。
var ErrNoRevision = errors.New("Chương này không có thay đổi bên ngoài")

// CheckRevisionChanges 返回发生外部修订的章节清单（含标题、来源类型与词数增删）。
// 与 CheckChapterRevisions 同一事实源：先看持久化的待恢复记录，否则现场扫描。
func (h *Host) CheckRevisionChanges() ([]RevisionChange, error) {
	texts, kind, err := h.revisionTexts()
	if err != nil {
		return nil, err
	}
	titles := h.chapterTitles()
	out := make([]RevisionChange, 0, len(texts))
	for _, t := range texts {
		added, removed := WordDelta(t.before, t.after)
		c := RevisionChange{Chapter: t.chapter, Title: titles[t.chapter], Kind: kind, Added: added, Removed: removed}
		if mt, ok := h.chapterModTime(t.chapter); ok {
			c.ModifiedAt = &mt
		}
		out = append(out, c)
	}
	return out, nil
}

// RevisionDiffFor 返回一章外部修订的前后正文；没有修订返回 ErrNoRevision。
func (h *Host) RevisionDiffFor(chapter int) (RevisionDiff, error) {
	texts, _, err := h.revisionTexts()
	if err != nil {
		return RevisionDiff{}, err
	}
	for _, t := range texts {
		if t.chapter == chapter {
			return RevisionDiff{Chapter: chapter, Title: h.chapterTitles()[chapter], Before: t.before, After: t.after}, nil
		}
	}
	return RevisionDiff{}, ErrNoRevision
}

type revisionText struct {
	chapter       int
	before, after string
}

// revisionTexts 列出外部修订章节的“已接受正文 / 工作区正文”。
// 待恢复记录存在时按其章节清单取：已接受正文取章节记录，工作区正文现读文件。
func (h *Host) revisionTexts() ([]revisionText, string, error) {
	pending, err := h.store.Revisions.LoadPending()
	if err != nil {
		return nil, "", err
	}
	if pending == nil {
		changes, err := revision.Scan(h.store)
		if err != nil {
			return nil, "", err
		}
		out := make([]revisionText, 0, len(changes))
		for _, c := range changes {
			out = append(out, revisionText{chapter: c.Chapter, before: c.Before, after: c.After})
		}
		return out, "edit", nil
	}
	out := make([]revisionText, 0, len(pending.Items))
	for _, item := range pending.Items {
		t := revisionText{chapter: item.Chapter}
		if rec, err := h.store.ChapterRecords.Load(item.Chapter); err == nil && rec != nil {
			t.before = rec.Content
		}
		if text, err := h.store.Drafts.LoadChapterText(item.Chapter); err == nil {
			t.after = domain.NormalizeChapterContent(text)
		}
		out = append(out, t)
	}
	return out, "resume", nil
}

// chapterModTime reports when the working chapter file last changed. Hosted
// stores keep chapter text in rows with no file to stat, so they report none.
func (h *Host) chapterModTime(chapter int) (time.Time, bool) {
	dir, ok := h.store.FileRoot()
	if !ok {
		return time.Time{}, false
	}
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(fmt.Sprintf("chapters/%02d.md", chapter))))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func (h *Host) chapterTitles() map[int]string {
	titles := map[int]string{}
	if infos, err := h.Chapters(); err == nil {
		for _, info := range infos {
			titles[info.Chapter] = info.Title
		}
	}
	return titles
}

// WordDelta 估算 after 相对 before 增加/删除的词数：按词（domain.WordCount 口径：
// 空白分隔且含字母或数字）做多重集差。替换一句 = 删旧词 + 加新词；纯调换顺序不计。
func WordDelta(before, after string) (added, removed int) {
	counts := map[string]int{}
	for _, w := range countedWords(before) {
		counts[w]++
	}
	for _, w := range countedWords(after) {
		counts[w]--
	}
	for _, n := range counts {
		switch {
		case n > 0:
			removed += n
		case n < 0:
			added -= n
		}
	}
	return added, removed
}

func countedWords(s string) []string {
	fields := strings.Fields(s)
	out := fields[:0]
	for _, f := range fields {
		if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			out = append(out, f)
		}
	}
	return out
}
