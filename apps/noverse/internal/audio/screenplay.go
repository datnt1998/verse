package audio

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// maxSluglineRunes 是"整行大写、以句号收尾"的行仍可能被当成剧本外景标记的
// 长度上限。一句正常叙述很少整行大写；短标题（PHÒNG DỰ ÁN AI.）才像 slugline。
const maxSluglineRunes = 80

// maxSpeakerCueRunes 是"整行只有人名"的说话人提示行长度上限（ch05 的 LÂM、
// TUYẾT NHI）。超过这个长度的全大写行更可能是句子，不是提示。
const maxSpeakerCueRunes = 40

// transitionPauseMs 是转场处的停顿：明显长于普通句间（0–300ms），仍在
// clampPause 的 1500ms 以内。
const transitionPauseMs = 900

// DialogueHint 记录一行被剥掉说话人标签后的台词，供 Compose 把对应朗读段
// 的 speaker 直接定到分演名单里的角色。
type DialogueHint struct {
	Speaker string
	Text    string
}

// ReadingPlan 是"原文 → 可朗读文本"的确定性计划：改写后的正文、被改写的原行、
// 台词提示，以及每个转场之后那一块的起始文本（用来在朗读段上补长停顿）。
type ReadingPlan struct {
	Text        string
	Changed     []string
	Dialogue    []DialogueHint
	PauseBlocks []string
}

// PrepareReadingText 只返回文本与被改写的行（旧签名，测试与调用方沿用）。
func PrepareReadingText(text string) (string, []string) {
	plan := PlanReadingText(text)
	return plan.Text, plan.Changed
}

// PlanReadingText 把剧本排版改写成能自然朗读的正文：
//
//  1. 场景标题（CẢNH 7 – MONTAGE…，含 **…# 变体）：丢掉场号与 NỘI./NGOẠI./
//     INT./EXT./MONTAGE，只把地点与时间改写成 "Bối cảnh: …" 一句。
//  2. 紧跟标题/另一条 slugline 的整行大写短地点行（PHÒNG DỰ ÁN AI.）同上。
//  3. 转场行（CẮT ĐEN. / CHUYỂN CẢNH / MỜ DẦN / FADE OUT / CUT TO…）整行丢掉，
//     并记下下一块的起始文本，供朗读段补一段长停顿。
//  4. 说话人标签（TRIỆU: …、TRIỆU (nhìn chiếc điều khiển): …，或整行只有
//     LÂM / TUYẾT NHI 的提示行）：标签不读，台词保留，并记下是谁说的；
//     括号里的动作若有意义，改成一句短叙述（"Triệu nhìn chiếc điều khiển."）。
//  5. 其余整行大写按小写朗读。
//
// 正文内容与叙述（包括"cảnh 3 của vở kịch"）原样保留。
func PlanReadingText(text string) ReadingPlan {
	plan := ReadingPlan{}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	headingContext := false
	pendingPause := false
	atBlockStart := true
	dialogueSpeaker := ""

	emit := func(line string) {
		kept = append(kept, line)
		if pendingPause && strings.TrimSpace(line) != "" {
			plan.PauseBlocks = append(plan.PauseBlocks, line)
			pendingPause = false
		}
	}
	hint := func(speaker, dialogue string) {
		if strings.TrimSpace(dialogue) == "" {
			return
		}
		plan.Dialogue = append(plan.Dialogue, DialogueHint{Speaker: speaker, Text: dialogue})
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			emit(line)
			atBlockStart = true
			dialogueSpeaker = ""
			continue
		}
		if isTransitionLine(trimmed) {
			plan.Changed = append(plan.Changed, trimmed)
			pendingPause = true
			atBlockStart = true
			dialogueSpeaker = ""
			continue
		}
		if bare, ok := rules.SceneHeadingLine(line); ok {
			plan.Changed = append(plan.Changed, bare)
			headingContext = true
			atBlockStart = true
			dialogueSpeaker = ""
			if setting := rules.SceneSetting(bare); setting != "" {
				emit("Bối cảnh: " + setting + ".")
			}
			continue
		}
		if headingContext && isSlugline(trimmed) {
			plan.Changed = append(plan.Changed, trimmed)
			atBlockStart = false
			dialogueSpeaker = ""
			emit("Bối cảnh: " + lowerSlugline(trimmed) + ".")
			continue
		}
		if isMarkdownHeading(trimmed) {
			headingContext = true
			atBlockStart = true
			dialogueSpeaker = ""
			emit(line)
			continue
		}
		headingContext = false

		if atBlockStart {
			if speaker, direction, rest, ok := parseSpeakerLabel(trimmed); ok {
				plan.Changed = append(plan.Changed, trimmed)
				if sentence := directionSentence(speaker, direction); sentence != "" {
					emit(sentence)
				}
				dialogueSpeaker = speaker
				atBlockStart = false
				if rest != "" {
					out := lowerAllCapsLine(rest)
					emit(out)
					hint(speaker, out)
				}
				continue
			}
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
				if speaker, ok := bareSpeakerCue(trimmed); ok {
					plan.Changed = append(plan.Changed, trimmed)
					dialogueSpeaker = speaker
					atBlockStart = false
					continue
				}
			}
		}

		out := lowerAllCapsLine(line)
		emit(out)
		if dialogueSpeaker != "" {
			hint(dialogueSpeaker, out)
		}
		atBlockStart = false
	}

	if len(plan.Changed) == 0 {
		return ReadingPlan{Text: text}
	}
	if pendingPause {
		// 转场在章末：没有"下一块"可挂停顿，标记成结尾，让最后一段拖长停顿。
		plan.PauseBlocks = append(plan.PauseBlocks, pauseAtEnd)
	}
	plan.Text = collapseBlankLines(strings.Join(kept, "\n"))
	return plan
}

// pauseAtEnd 标记"转场在章末"：朗读段落地时把最后一段的停顿拉长。
const pauseAtEnd = "\x00end"

// transitionRe 是独立成行的转场标注。整行匹配，避免把正文里的"cắt 50%…"当转场。
var transitionRe = regexp.MustCompile(`(?i)^(?:cắt đen|cắt sang|cắt cảnh|cắt máy|chuyển cảnh|chuyển sang|mờ dần|mờ dần sang đen|đen dần|fade in|fade out|fade to black|cut to|smash cut|hết cảnh|hết phân cảnh)\s*[.:!…]*$`)

func isTransitionLine(line string) bool {
	bare := trimLineDecoration(line)
	if bare == "" {
		return false
	}
	return transitionRe.MatchString(strings.ToLower(bare))
}

// speakerLabelRe 匹配"IN HOA (chỉ dẫn): lời thoại"。名字必须全大写，避免把
// 普通叙述里的"A: …"当成标签。
var speakerLabelRe = regexp.MustCompile(`^([\p{Lu}\p{M}][\p{Lu}\p{M}\s.'’\-]*?)\s*(?:\(([^)]*)\))?\s*:\s*(.*)$`)

func parseSpeakerLabel(line string) (speaker, direction, rest string, ok bool) {
	m := speakerLabelRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", false
	}
	speaker = strings.TrimSpace(m[1])
	if speaker == "" || strings.IndexFunc(speaker, unicode.IsLower) >= 0 {
		return "", "", "", false
	}
	return speaker, strings.TrimSpace(m[2]), strings.TrimSpace(m[3]), true
}

// bareSpeakerCueRe 匹配整行只有人名（ch05 的 "LÂM"、"TUYẾT NHI"）：全大写、
// 无数字、没有冒号。
var bareSpeakerCueRe = regexp.MustCompile(`^[\p{Lu}\p{M}][\p{Lu}\p{M}\s.'’\-]*$`)

func bareSpeakerCue(line string) (string, bool) {
	bare := trimLineDecoration(line)
	if bare == "" || utf8.RuneCountInString(bare) > maxSpeakerCueRunes {
		return "", false
	}
	if strings.IndexFunc(bare, unicode.IsDigit) >= 0 || strings.Contains(bare, ":") {
		return "", false
	}
	if !bareSpeakerCueRe.MatchString(bare) {
		return "", false
	}
	return strings.Join(strings.Fields(bare), " "), true
}

// directionJunk 是括号里没有可演内容的提示，直接丢掉。
var directionJunk = map[string]bool{
	"im lặng": true, "lặng": true, "ngừng": true, "ngưng": true,
	"beat": true, "pause": true, "v.o.": true, "o.s.": true, "vo": true, "os": true,
}

// directionSentence 把括号动作变成一句短叙述："TRIỆU (nhìn chiếc điều khiển)"
// → "Triệu nhìn chiếc điều khiển."。没有可演内容时返回空串。
func directionSentence(speaker, direction string) string {
	action := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(direction), ".,;:"))
	if action == "" || !hasLetters(action) {
		return ""
	}
	if directionJunk[strings.ToLower(action)] {
		return ""
	}
	name := titleCaseName(speaker)
	if name == "" {
		return ""
	}
	return name + " " + action + "."
}

// titleCaseName 把全大写人名还原成书名式大小写（TRIỆU → Triệu,
// TUYẾT NHI → Tuyết Nhi）。
func titleCaseName(name string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(name)))
	for i, f := range fields {
		runes := []rune(f)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		fields[i] = string(runes)
	}
	return strings.Join(fields, " ")
}

// isSlugline 是保守的地点行判定：整行大写、以句号收尾、长度有限。只应在场景
// 开头调用——正文中间的全大写句子不在此列。
func isSlugline(line string) bool {
	bare := trimLineDecoration(line)
	if bare == "" || !strings.HasSuffix(bare, ".") {
		return false
	}
	if len([]rune(bare)) > maxSluglineRunes {
		return false
	}
	return hasLetters(bare) && allLettersUpper(bare)
}

// lowerAllCapsLine 把"整行都是大写"的行改成小写；混合大小写的正文不动。
func lowerAllCapsLine(line string) string {
	bare := trimLineDecoration(line)
	if bare == "" || !hasLetters(bare) || !allLettersUpper(bare) {
		return line
	}
	return strings.ToLower(line)
}

func lowerSlugline(line string) string {
	bare := strings.ToLower(strings.TrimSuffix(trimLineDecoration(line), "."))
	for _, prefix := range []string{"nội và ngoại ", "nội/ngoại ", "int./ext. ", "nội. ", "ngoại. ", "int. ", "ext. "} {
		if rest, ok := strings.CutPrefix(bare, prefix); ok {
			bare = rest
		}
	}
	bare = strings.TrimSpace(bare)
	bare = strings.TrimLeft(bare, "-–— ")
	return strings.TrimSpace(bare)
}

func hasLetters(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

func allLettersUpper(s string) bool {
	return strings.IndexFunc(s, unicode.IsLower) < 0
}

func isMarkdownHeading(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "#")
}

func trimLineDecoration(line string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "*#_ "))
}

// applyReadingPlan 把计划落到朗读段上：
//   - 台词提示把段的 speaker 定到分演名单里匹配的角色（名字/别名，忽略大小写），
//     并清掉"说话人不认识"的标志；
//   - 每个转场后的第一段，把**前一段**的停顿拉到 transitionPauseMs。
//
// 匹配用 Source（规范化前的原文），按空白/大小写归一后做等值或包含匹配；
// 匹配不到就保持模型的原判断，绝不猜测。
func applyReadingPlan(segs []Segment, plan ReadingPlan, in ComposeInput) []Segment {
	byName := map[string]string{}
	for _, sp := range in.Characters {
		if sp.ID == "" || strings.TrimSpace(sp.Name) == "" {
			continue
		}
		byName[strings.ToLower(strings.TrimSpace(sp.Name))] = sp.ID
		for _, alias := range sp.Aliases {
			if alias = strings.TrimSpace(alias); alias != "" {
				byName[strings.ToLower(alias)] = sp.ID
			}
		}
	}
	for i := range segs {
		seg := &segs[i]
		src := matchKey(seg.Source)
		best := -1
		bestLen := 0
		for j, hint := range plan.Dialogue {
			key := matchKey(hint.Text)
			if key == "" || !(src == key || (len(key) >= 20 && strings.Contains(src, key))) {
				continue
			}
			if len(key) > bestLen {
				best, bestLen = j, len(key)
			}
		}
		if best < 0 {
			continue
		}
		id, ok := byName[strings.ToLower(strings.TrimSpace(plan.Dialogue[best].Speaker))]
		if !ok {
			continue
		}
		seg.Speaker = id
		seg.SpeakerGuess = nil
		seg.Flags = removeFlag(seg.Flags, FlagUnknownSpeaker)
	}

	for _, block := range plan.PauseBlocks {
		if block == pauseAtEnd {
			if n := len(segs); n > 0 {
				segs[n-1].PauseMs = clampPause(max(segs[n-1].PauseMs, transitionPauseMs))
			}
			continue
		}
		key := matchKey(block)
		if len(key) < 16 {
			continue
		}
		for i := range segs {
			if !strings.Contains(matchKey(segs[i].Source), key) {
				continue
			}
			if i > 0 {
				segs[i-1].PauseMs = clampPause(max(segs[i-1].PauseMs, transitionPauseMs))
			}
			break
		}
	}
	return segs
}

// matchKey 把文本折成比对键：小写、去强调标记、压空白、去首尾标点。
func matchKey(s string) string {
	s = strings.ToLower(stripMarkdown(s))
	s = strings.Join(strings.Fields(s), " ")
	return strings.Trim(s, " .,;:!?…–—-\"'“”‘’")
}

func removeFlag(flags []string, want string) []string {
	out := flags[:0]
	for _, f := range flags {
		if f != want {
			out = append(out, f)
		}
	}
	return out
}

// collapseBlankLines 收掉被删除行留下的三连空行。
func collapseBlankLines(text string) string {
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}
