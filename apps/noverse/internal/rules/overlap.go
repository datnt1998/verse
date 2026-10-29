package rules

import (
	"strings"
	"unicode"
)

// variantNormalized 是用于重叠比对的规整文本，origin[i] 是规整文本第 i 个 rune
// 在原文中的 rune 下标。比对口径：小写、引号拉直、非文字/数字/组合符替换成空格、
// 连续空白压成一个、去首尾空白。标点因此不参与比对，只留下一个分隔。
type variantNormalized struct {
	text   string
	origin []int
}

func normalizeOverlapText(s string) variantNormalized {
	var b []rune
	var origin []int
	pendingSpace := false
	for i, r := range []rune(s) {
		r = unicode.ToLower(foldOverlapQuote(r))
		if isOverlapWordRune(r) {
			if pendingSpace && len(b) > 0 {
				b = append(b, ' ')
				origin = append(origin, i)
			}
			pendingSpace = false
			b = append(b, r)
			origin = append(origin, i)
			continue
		}
		pendingSpace = true
	}
	return variantNormalized{text: string(b), origin: origin}
}

func isOverlapWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.Mn, r)
}

// foldOverlapQuote 把弯引号折成直引号，避免同一句话因排版引号不同而漏判。
func foldOverlapQuote(r rune) rune {
	switch r {
	case '“', '”', '„', '‟', '«', '»', '「', '」', '『', '』':
		return '"'
	case '‘', '’', '‚', '‛':
		return '\''
	}
	return r
}

// OpeningOverlapRunes 返回 prevTail 与 opening 在规整后最长的连续公共子串长度（rune）。
// 规整口径见 normalizeOverlapText：小写、引号拉直、标点与括号折叠为分隔、空白压缩。
// 纯函数，不读 store。
func OpeningOverlapRunes(prevTail, opening string) int {
	a := []rune(normalizeOverlapText(prevTail).text)
	b := []rune(normalizeOverlapText(opening).text)
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	// 经典 O(len(a)*len(b)) 动态规划。章节尾部与开篇各只有几百 rune，够用。
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	best := 0
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] != b[j-1] {
				cur[j] = 0
				continue
			}
			cur[j] = prev[j-1] + 1
			if cur[j] > best {
				best = cur[j]
			}
		}
		prev, cur = cur, prev
		clear(cur)
	}
	return best
}

// TrimLeadingOverlap 去掉 next 开头与 tail 结尾连续重复的部分：取规整后最长、且不短于
// minRunes 的前缀，要求它在 next 中收在词界上、且在 tail 中是从词界起一路贴到末尾的
// 后缀，才把它连同尾随标点一起从 next 原文里剪掉。返回剩余文本（已去掉开头空白）与
// 被剪掉的原文 rune 数。纯函数，不读 store。
func TrimLeadingOverlap(tail, next string, minRunes int) (string, int) {
	if minRunes <= 0 {
		minRunes = 1
	}
	tailNorm := normalizeOverlapText(tail)
	nextNorm := normalizeOverlapText(next)
	nextRunes := []rune(next)
	total := len(nextRunes)
	if tailNorm.text == "" || nextNorm.text == "" {
		return next, 0
	}
	limit := len([]rune(nextNorm.text))
	if tailLimit := len([]rune(tailNorm.text)); limit > tailLimit {
		limit = tailLimit
	}
	for length := limit; length >= minRunes; length-- {
		prefix := string([]rune(nextNorm.text)[:length])
		// 词边界守卫：前缀必须收在 next 的词界上，且在 tail 中必须是从词界起、
		// 一路贴到 tail 末尾的重复——否则会把"…trước bà."对"…trước bàn đá."
		// 这类只共享词干或碰巧同词的情形剪进半个词或剪掉新句的开词。
		if !endsAtOverlapBoundary(nextNorm.text, length) ||
			!hasBoundarySuffix(tailNorm.text, prefix) {
			continue
		}
		// 规整文本的每个 rune 都对应原文里一个文字 rune；折叠出来的分隔本身不占
		// 原文位置，所以剪到 origin[length] 就顺带剪掉了收尾的标点与空白。
		cut := total
		if length < len(nextNorm.origin) {
			cut = nextNorm.origin[length]
		}
		rest := strings.TrimLeftFunc(string(nextRunes[cut:]), unicode.IsSpace)
		return rest, total - len([]rune(rest))
	}
	return next, 0
}

// endsAtOverlapBoundary 检查 text 前 length 个 rune 恰好收在词界上：末尾是
// 串尾，或其后一个 rune 不是文字/数字（规整文本中即折叠出来的空格）。
func endsAtOverlapBoundary(text string, length int) bool {
	runes := []rune(text)
	return length >= len(runes) || !isOverlapWordRune(runes[length])
}

// hasBoundarySuffix 只接受 prefix 恰好是 text 的词界对齐后缀：它必须一路贴到
// text 末尾（重复的开篇必然是对着结尾重抄的），且其前一个 rune 是串头或非文字。
func hasBoundarySuffix(text, prefix string) bool {
	tr, pr := []rune(text), []rune(prefix)
	if len(pr) == 0 || len(pr) > len(tr) {
		return false
	}
	if string(tr[len(tr)-len(pr):]) != prefix {
		return false
	}
	return len(tr) == len(pr) || !isOverlapWordRune(tr[len(tr)-len(pr)-1])
}
