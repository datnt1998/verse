package exp

import (
	"regexp"
	"strings"

	"github.com/datnt1998/noverse/internal/audio"
)

// narration 导出（Master v3 §15.5）：只保留"要朗读的文本"。
// 无书名、无章标题/卷分隔；Markdown、表情符号、装饰符号全部去掉；
// 章与章之间恰好两个空行，段与段之间一个空行，UTF-8 + LF + 末尾换行。
// readNumbers=true 时再经 audio 的确定性规范化把数字/日期/金额/符号读成越南语字。

// mdHeadingLine 章内 Markdown 标题行（"# …"）。narration 没有标题概念，
// 且标题行剥掉 "#" 后会变成裸的章名文本（"Chương 1: …"），正好命中
// kiem_tra_truyen.py 的章标题检查，所以整行丢弃。
var mdHeadingLine = regexp.MustCompile(`^#{1,6}\s+`)

// mdTableSep 表格分隔行（|---|---|），丢整行。
var mdTableSep = regexp.MustCompile(`^[\s|:\-—]+$`)

// mdThematic 水平线行（---、***、___），丢整行。
var mdThematic = regexp.MustCompile(`^\s*([-*_]\s*){3,}$`)

// mdBlockquote/列表/有序列表 行首标记。
var mdLinePrefix = regexp.MustCompile(`^\s*(?:>\s*|[-+*]\s+|\d+[.)]\s+)`)

// mdImage / mdLink：图片丢整段（![alt](src) 只剩 alt 文本没意义），链接保留可见文本。
var mdImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// mdBareBracket 剩下的成对中括号保留内部文字（可能是人物代号等正文），只去括号。
// 真正的朗读控制标签由 audio 规范化的 DropBrackets 兜底处理（readNumbers 开时）。
var mdBareBracket = regexp.MustCompile(`\[([^\]]*)\]`)

// 行内标记字符：*（强调）、_（斜体/下划线）、`（代码）、~（删除线）、#（残头）、|（表格）。
var mdInlineChars = regexp.MustCompile(`[*_` + "`" + `~#]`)

// decorativeRe 命中 kiem_tra_truyen.py --narration 的表情/装饰检查：
// U+1F300–1FAFF、U+2600–27BF（☀–➿）、U+2B00–2BFF（⭐ 等杂项符号），
// 外加分隔线用的 box-drawing 字符与 emoji 连接符。
var decorativeRe = regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{2500}-\x{257F}\x{FE0F}\x{200D}]`)

var multiSpace = regexp.MustCompile(`[ \t\x{3000}]{2,}`)

// cleanNarrationLine 把一行正文剥成纯文本；返回空串表示整行丢弃。
func cleanNarrationLine(line string) string {
	s := strings.TrimSpace(line)
	if s == "" {
		return ""
	}
	if mdHeadingLine.MatchString(s) || mdThematic.MatchString(s) || mdTableSep.MatchString(s) {
		return ""
	}
	s = mdLinePrefix.ReplaceAllString(s, "")
	s = mdImage.ReplaceAllString(s, "")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdBareBracket.ReplaceAllString(s, "$1")
	s = mdInlineChars.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "|", " ")
	s = decorativeRe.ReplaceAllString(s, "")
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// paragraphBlockRe 切章节为段块：一个或多个空行是段边界。
var paragraphBlockRe = regexp.MustCompile(`\n\s*\n`)

// narrationNormalizeOpts 用项目默认的全量规范化（数字/日期/金额/符号/缩写/中括号注记），
// 正好覆盖 §15.5 "không chứa chữ số" 与 "thẻ điều khiển giọng"。
// 项目级读音词典在 <project>/.ainovel/cach-doc.json，store 无法定位，传 nil。
func narrationNormalizeOpts() audio.NormalizeOptions {
	return audio.DefaultNormalizeOptions()
}

// renderNarration 拼出 narration 纯文本。bodies 已按章号顺序传入；
// 每章内部按空行切段，段内软换行并成一个空格。
func renderNarration(chapters []int, bodies map[int]string, readNumbers bool) string {
	norm := narrationNormalizeOpts()
	var blocks []string
	for _, ch := range chapters {
		body := strings.ReplaceAll(bodies[ch], "\r\n", "\n")
		var paras []string
		for _, block := range paragraphBlockRe.Split(body, -1) {
			var lines []string
			for _, raw := range strings.Split(block, "\n") {
				if line := cleanNarrationLine(raw); line != "" {
					lines = append(lines, line)
				}
			}
			if len(lines) == 0 {
				continue
			}
			para := strings.Join(lines, " ")
			if readNumbers {
				para, _ = audio.Normalize(para, norm, nil)
			}
			if para != "" {
				paras = append(paras, para)
			}
		}
		if len(paras) == 0 {
			continue
		}
		blocks = append(blocks, strings.Join(paras, "\n\n"))
	}
	return strings.Join(blocks, "\n\n\n") + "\n"
}
