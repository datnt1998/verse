package rules

import (
	"fmt"
	"strings"
	"unicode"
)

// Stage 是 Admission 校验的阶段：草稿阶段（Draft，与 edit_chapter 的
// new_string 同级）只查机械残片；终稿阶段（Final，commit 新提交）再加
// 截断判断——草稿本来就允许半成品结尾。
type Stage int

const (
	Draft Stage = iota + 1
	Final
)

// Admission 在工具收稿时做准入检查，返回本次输入的机械事故事实。
// 与 Lint/Check 不同：这些 violation 供调用方硬阻断（ErrToolArgs），
// 只收"没有任何文学解释"的残留——思考块、代码栅栏、收尾废话、截断。
// 段落级句式层面的问题仍走 Check/Lint，不在这里判。
//
// 剧本场景标题（CẢNH 1 / NỘI. …）单独成行就是排版残留：产品是"用来听"的散文。
// 工具在落盘前先机械删掉这类整行（并给 writer 一句提示），所以这里的硬规则
// 只兜底漏网内容，不会让 writer 反复重发整章。
func Admission(text string, stage Stage) []Violation {
	var vs []Violation
	vs = appendThinkTag(vs, text)
	vs = appendCodeFence(vs, text)
	vs = appendMetaCommentary(vs, text)
	vs = appendSceneHeading(vs, text)
	if stage == Final {
		vs = appendTruncatedEnd(vs, text)
	}
	return vs
}

// thinkTagRe 覆盖思考块的四种开合形态（不区分大小写）：
// <think>、</think>、<thinking>、</thinking> 统一由 "<think"/"</think" 前缀命中。
func appendThinkTag(vs []Violation, text string) []Violation {
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "<think") && !strings.Contains(lower, "</think") {
		return vs
	}
	return append(vs, Violation{
		Rule:     "think_tag",
		Target:   "<think>",
		Actual:   1,
		Severity: SeverityError,
		Message:  "Văn còn thẻ suy nghĩ <think>/<thinking>; gỡ nguyên khối thẻ khỏi chính văn rồi gửi lại",
	})
}

// appendCodeFence 查代码栅栏行。顶格写 ``` 的绝对是机械残留——越南语/中文
// 小说正文没有合法用途（markdown_residue 只管 ** 与 #，不管栅栏）。
func appendCodeFence(vs []Violation, text string) []Violation {
	for line := range strings.Lines(text) {
		if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "```") {
			return append(vs, Violation{
				Rule:     "code_fence",
				Target:   "```",
				Actual:   1,
				Severity: SeverityError,
				Message:  "Văn có dòng code fence ```; chính văn không chứa đoạn mã, gỡ dòng fence rồi gửi lại",
			})
		}
	}
	return vs
}

// metaCommentaryPhrases 只收模型对读者说话的元叙述套话（越南语与英语）：
// 指向"本段文字/这个稿子"本身的开场白、收尾邀评或 AI 自指，正文中没有任何
// 正当用途的对话碎片（"hãy cho tôi biết"、"nếu bạn cần"、"let me know" 之类
// 角色也会说的家常话）一律不收。只匹配首段与末段，命中包含关系（不锚定开头）。
var metaCommentaryPhrases = []string{
	// 越南语
	"dưới đây là chương",
	"dưới đây là bản",
	"bản nháp chương",
	"đây là chương tiếp theo",
	"tôi đã viết xong chương",
	"đã được viết lại",
	"bản chỉnh sửa",
	"bản hoàn chỉnh",
	"hy vọng chương này",
	"hy vọng bạn thích",
	// 英语
	"here is chapter",
	"here's chapter",
	"here is the chapter",
	"here's the chapter",
	"here is the rewritten",
	"as an ai",
	"i hope you enjoy",
	"in this chapter",
}

// appendMetaCommentary 查首段或末段的元叙述。段落按空行分隔，与正文的
// 段落口径一致；首段同时是末段时（短文本）只报一次。
func appendMetaCommentary(vs []Violation, text string) []Violation {
	paras := splitParagraphs(text)
	if len(paras) == 0 {
		return vs
	}
	if target := metaCommentaryTarget(paras[0]); target != "" {
		vs = append(vs, metaCommentaryViolation(target, "đầu chương"))
	}
	if last := paras[len(paras)-1]; len(paras) > 1 {
		if target := metaCommentaryTarget(last); target != "" {
			vs = append(vs, metaCommentaryViolation(target, "cuối chương"))
		}
	}
	return vs
}

func metaCommentaryViolation(target, pos string) Violation {
	return Violation{
		Rule:     "meta_commentary",
		Target:   target,
		Actual:   1,
		Severity: SeverityError,
		Message:  fmt.Sprintf("Phát hiện lời bình luận meta %q ở %s; xóa đoạn đó, chỉ gửi chính văn truyện", target, pos),
	}
}

// metaCommentaryTarget 在去掉角色台词后匹配元叙述：引号对（“…”、"…"、«…»）内
// 的内容和以破折号开头（—/–，小说里常见的对白行标记）的整行都是角色在说话，
// 不是模型在正文外说话，不参与匹配。
func metaCommentaryTarget(para string) string {
	lower := strings.ToLower(paraWithoutDialogue(para))
	for _, phrase := range metaCommentaryPhrases {
		if strings.Contains(lower, phrase) {
			return phrase
		}
	}
	return ""
}

// paraWithoutDialogue 滤掉段落中的台词部分：破折号对白行整行剔除，其余行内的
// 引号对内容替换为空格。元叙述残片永远以裸文本形态出现，不会躲在引号里。
func paraWithoutDialogue(para string) string {
	var b strings.Builder
	for _, line := range strings.Split(para, "\n") {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if strings.HasPrefix(trimmed, "—") || strings.HasPrefix(trimmed, "–") {
			continue
		}
		runes := []rune(line)
		for i := 0; i < len(runes); i++ {
			end := quotedSegmentEnd(runes, i)
			if end > i {
				i = end
				b.WriteByte(' ')
				continue
			}
			b.WriteRune(runes[i])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// quotedSegmentEnd 返回从 i 开始的引号段最后一个 rune 下标；无闭合引号时
// 视为非引号段返回 i（保持原字符参与匹配）。
func quotedSegmentEnd(runes []rune, i int) int {
	var close rune
	switch r := runes[i]; {
	case r == '“':
		close = '”'
	case r == '«':
		close = '»'
	case r == '"':
		close = '"'
	default:
		return i
	}
	for j := i + 1; j < len(runes); j++ {
		if runes[j] == close {
			return j
		}
	}
	return i
}

// splitParagraphs 按空行切段（空白行内含缩进也算），返回去空白后的非空段落。
func splitParagraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(text, "\n\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// validEndRunes 是终稿允许的收尾字符：句读（ASCII 与全角）、引号、括号、
// 破折号、省略号。缺其中任意一个基本就是模型输出被截断（API 截断/超长截断
// 最常见）。全角 。！？ 一并放行：它们与 ASCII 对应物一样是明确的句末标点。
const validEndRunes = `.!?…"”’»)」』—~。！？`

// appendTruncatedEnd 只在 Final 阶段生效：草稿允许写到一半停下，
// 终稿最后一个非空白字符必须是收束标点。对话以 ” 结尾、句子以 — 或 …
// 结尾都算正常收束，不能误伤。
func appendTruncatedEnd(vs []Violation, text string) []Violation {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return vs
	}
	runes := []rune(trimmed)
	last := runes[len(runes)-1]
	if strings.ContainsRune(validEndRunes, last) {
		return vs
	}
	return append(vs, Violation{
		Rule:     "truncated_end",
		Target:   string(last),
		Actual:   1,
		Severity: SeverityError,
		Message:  fmt.Sprintf("Văn kết thúc bằng %q thay vì dấu câu kết thúc, có vẻ bị cắt cụt; viết tiếp phần còn thiếu hoặc kết lại câu cuối rồi nộp", string(last)),
	})
}
