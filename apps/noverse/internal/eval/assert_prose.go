package eval

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
)

// 开篇复读检查的取窗倍率。规整会去掉标点，原文窗口必须显著宽于目标 rune 数，
// 否则标点密集的开篇永远达不到阈值，检查会静默失效。
const (
	openingHeadFactor = 2
	prevTailFactor    = 4
)

// gradeProseExpect 断言目标章的终稿正文。正则只用于 unique 词法标记；开篇复读
// 用与 rules 同一套规整口径，不在这里另造一种"相似"定义。
func gradeProseExpect(c Case, col Collected, res *Result) {
	e := c.Expect.Prose
	if e == nil {
		return
	}
	chapter := e.Chapter
	text := col.ChapterTexts[chapter]
	if strings.TrimSpace(text) == "" {
		res.WriterViolations++
		res.addHardFail("prose", fmt.Sprintf("không đọc được văn bản chương %d", chapter))
		return
	}

	for _, pattern := range e.ForbidRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			res.addHardFail("prose:forbid", fmt.Sprintf("regex không hợp lệ %q: %v", pattern, err))
			continue
		}
		if re.MatchString(text) {
			res.WriterViolations++
			res.addHardFail("prose:forbid", fmt.Sprintf("chương %d khớp điều cấm %q", chapter, pattern))
			continue
		}
		res.addPass("prose:forbid", fmt.Sprintf("chương %d không khớp %q", chapter, pattern))
	}

	for _, pattern := range e.RequireRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			res.addHardFail("prose:require", fmt.Sprintf("regex không hợp lệ %q: %v", pattern, err))
			continue
		}
		if !re.MatchString(text) {
			res.WriterViolations++
			res.addHardFail("prose:require", fmt.Sprintf("chương %d thiếu %q", chapter, pattern))
			continue
		}
		res.addPass("prose:require", fmt.Sprintf("chương %d có %q", chapter, pattern))
	}

	if e.MaxOpeningOverlapRunes > 0 {
		gradeOpeningOverlap(col, chapter, text, e.MaxOpeningOverlapRunes, res)
	}
}

// gradeOpeningOverlap 用上一章结尾当母本，检查本章开篇是否复读了它。
func gradeOpeningOverlap(col Collected, chapter int, text string, maxRun int, res *Result) {
	if chapter <= 1 {
		res.addPass("prose:opening_overlap", "chương đầu không có chương trước để so")
		return
	}
	prev, ok := col.ChapterTexts[chapter-1]
	if !ok || strings.TrimSpace(prev) == "" {
		res.WriterViolations++
		res.addHardFail("prose:opening_overlap", fmt.Sprintf("thiếu văn bản chương %d để đo trùng mở đầu", chapter-1))
		return
	}
	head := firstRunes(text, maxRun*openingHeadFactor)
	overlap := rules.OpeningOverlapRunes(lastRunes(prev, maxRun*prevTailFactor), head)
	if overlap > maxRun {
		res.WriterViolations++
		res.addHardFail("prose:opening_overlap",
			fmt.Sprintf("chương %d mở đầu lặp %d rune của chương %d (giới hạn %d)", chapter, overlap, chapter-1, maxRun))
		return
	}
	res.addPass("prose:opening_overlap", fmt.Sprintf("chương %d mở đầu lặp %d rune (giới hạn %d)", chapter, overlap, maxRun))
}

// gradeFactsExpect 断言目标章落盘的结构化事实。事实读不到就是 FAIL——把"没读到"
// 当成"没有出现"，正是这类断言最容易产生的假通过。
func gradeFactsExpect(c Case, col Collected, res *Result) {
	e := c.Expect.Facts
	if e == nil {
		return
	}
	facts, ok := col.ChapterFacts[e.Chapter]
	if !ok {
		res.WriterViolations++
		res.addHardFail("facts", fmt.Sprintf("chương %d không có bản ghi sự kiện", e.Chapter))
		return
	}
	haystack := factsText(facts)
	for _, needle := range e.ForbidContains {
		if quoteMatches(haystack, needle) {
			res.WriterViolations++
			res.addHardFail("facts:forbid", fmt.Sprintf("sự kiện chương %d chứa điều cấm %q", e.Chapter, needle))
			continue
		}
		res.addPass("facts:forbid", fmt.Sprintf("sự kiện chương %d không chứa %q", e.Chapter, needle))
	}
}

// factsText 把一章事实摊平成可搜索文本：断言只读事实层，不重读正文。
func factsText(facts domain.ChapterFacts) string {
	parts := []string{facts.Title, facts.Summary, facts.HookType, facts.DominantStrand}
	parts = append(parts, facts.Characters...)
	parts = append(parts, facts.KeyEvents...)
	for _, ev := range facts.TimelineEvents {
		parts = append(parts, ev.Time, ev.Event)
	}
	for _, change := range facts.StateChanges {
		parts = append(parts, change.Entity, change.Field, change.OldValue, change.NewValue, change.Reason)
	}
	for _, item := range facts.ForeshadowUpdates {
		parts = append(parts, item.ID, item.Description)
	}
	for _, entity := range append(append([]domain.FactEntity{}, facts.Places...), facts.Objects...) {
		parts = append(parts, entity.Name, entity.Desc)
	}
	for _, rel := range facts.RelationshipChanges {
		parts = append(parts, rel.CharacterA, rel.CharacterB, rel.Relation, rel.Type)
	}
	return strings.Join(parts, "\n")
}

func firstRunes(s string, n int) string {
	runes := []rune(s)
	if n <= 0 || len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func lastRunes(s string, n int) string {
	runes := []rune(s)
	if n <= 0 || len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}
