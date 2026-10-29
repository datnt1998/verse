package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// quoteUnit 是规范化串里的一个 rune：记录它在规范化串中的起点，以及它
// 对应的原文区间。空白折叠、"…"→"..."、弯引号合并都只发生在规范化串上，
// 命中的偏移始终指回原文（模型读到的同一份文本，不持久化）。
type quoteUnit struct {
	normStart int
	origStart int
	origEnd   int
}

// quoteIndex 是文本的规范化视图，外加规范化 rune 到原文区间的映射。
type quoteIndex struct {
	norm  string
	units []quoteUnit
}

// MinQuoteRunes 是引文参与定位的最短长度：再短不具辨识度，提交时直接拒绝。
const MinQuoteRunes = 6

// MinPrefixRunes 是"最近匹配前缀"的最短长度：错误信息里提示原文片段给
// 模型照抄，至少要有这么多 rune 才比盲猜有帮助。
const MinPrefixRunes = 12

// normalizeQuoteRune 把一个 rune 折成规范化形式。返回空白时第二个返回值
// 为 true（由调用方折叠），其余单 rune 映射直接替换；返回 0 表示该 rune
// 由调用方原样保留。
func normalizeQuoteRune(r rune) (out rune, isSpace bool) {
	switch r {
	case '"', '“', '”', '„', '‟':
		return '"', false
	case '\'', '‘', '’', '‚', '‛':
		return '\'', false
	case '—', '–', '‐', '−':
		return '-', false
	}
	if unicode.IsSpace(r) {
		return ' ', true
	}
	return r, false
}

// buildQuoteIndex 生成规范化串与回指映射：空白折叠成单个空格；"..." 并成
// "…"；弯/直引号、三种连字符各归一。大小写与越南语附加符号保持原样——
// 引文是证据，改了字母就不是同一段话。
func buildQuoteIndex(text string) quoteIndex {
	var norm strings.Builder
	norm.Grow(len(text))
	var units []quoteUnit
	pendingSpace := false
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		mapped, isSpace := normalizeQuoteRune(r)
		if isSpace {
			// 记录这一段空白的起点，折叠出的空格覆盖整段空白。
			if !pendingSpace {
				units = append(units, quoteUnit{normStart: norm.Len(), origStart: i, origEnd: i + size})
			} else {
				units[len(units)-1].origEnd = i + size
			}
			pendingSpace = true
			i += size
			continue
		}
		if pendingSpace && norm.Len() > 0 {
			norm.WriteByte(' ')
		} else if pendingSpace {
			// 文本以空白开头：该单元不产生规范化字符。
			units = units[:len(units)-1]
		}
		pendingSpace = false
		if r == '.' && i+2 < len(text) && text[i+1] == '.' && text[i+2] == '.' {
			norm.WriteRune('…')
			units = append(units, quoteUnit{normStart: norm.Len() - 3, origStart: i, origEnd: i + 3})
			i += 3
			continue
		}
		norm.WriteRune(mapped)
		units = append(units, quoteUnit{normStart: norm.Len() - utf8.RuneLen(mapped), origStart: i, origEnd: i + size})
		i += size
	}
	if pendingSpace && len(units) > 0 {
		// 末尾空白不产生规范化字符。
		units = units[:len(units)-1]
	}
	return quoteIndex{norm: norm.String(), units: units}
}

// NormalizeQuote 是引文比对的唯一口径：空白折叠、弯/直双引号与单引号合一、
// "…" 与 "..." 合一、—/–/- 归一。不改大小写、不去越南语附加符号。首尾
// 空白先去掉再折叠，保证返回值可直接与规范化文本做 Contains 比较。
func NormalizeQuote(s string) string {
	return buildQuoteIndex(strings.TrimSpace(s)).norm
}

// origSpan 把规范化串的 rune 区间 [startUnit, endUnit) 映射回原文 byte 区间。
func (idx quoteIndex) origSpan(startUnit, endUnit int) (int, int, bool) {
	if startUnit < 0 || endUnit > len(idx.units) || startUnit >= endUnit {
		return 0, 0, false
	}
	return idx.units[startUnit].origStart, idx.units[endUnit-1].origEnd, true
}

// findUnits 在规范化文本里找 needle，返回其首末 rune 的下标（end 为独占）。
func (idx quoteIndex) findUnits(needle string) (int, int, bool) {
	at := strings.Index(idx.norm, needle)
	if at < 0 {
		return 0, 0, false
	}
	start := utf8.RuneCountInString(idx.norm[:at])
	return start, start + utf8.RuneCountInString(needle), true
}

// LocateQuote 在原文中定位引文：先各自规范化再匹配，命中的 start/end 是
// 原文 byte 偏移（[start, end)），可直接用于切片或回显。引文不足
// MinQuoteRunes 时拒绝匹配（ok=false），视为无效证据。
func LocateQuote(text, quote string) (start, end int, ok bool) {
	if utf8.RuneCountInString(strings.TrimSpace(quote)) < MinQuoteRunes {
		return 0, 0, false
	}
	nq := NormalizeQuote(quote)
	if nq == "" {
		return 0, 0, false
	}
	idx := buildQuoteIndex(text)
	first, last, found := idx.findUnits(nq)
	if !found {
		return 0, 0, false
	}
	origStart, origEnd, spanOK := idx.origSpan(first, last)
	if !spanOK {
		return 0, 0, false
	}
	return origStart, origEnd, true
}

// NearestQuotePrefix 返回原文中与引文开头对得上的最长片段：规范化后取引文的
// 前缀（≥ MinPrefixRunes），找在规范化文本里能命中的最长一段，回映射成原文。
// 错误信息用它给模型一个"能照抄的最近段落"，降低反复引错的成本。
// 找不到足够长的前缀时返回 false。
func NearestQuotePrefix(text, quote string) (snippet string, ok bool) {
	nq := NormalizeQuote(quote)
	runes := []rune(nq)
	if len(runes) < MinPrefixRunes {
		return "", false
	}
	idx := buildQuoteIndex(text)
	for n := len(runes); n >= MinPrefixRunes; n-- {
		first, last, found := idx.findUnits(string(runes[:n]))
		if !found {
			continue
		}
		origStart, origEnd, spanOK := idx.origSpan(first, last)
		if !spanOK {
			continue
		}
		return text[origStart:origEnd], true
	}
	return "", false
}
