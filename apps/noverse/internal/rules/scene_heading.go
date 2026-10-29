package rules

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// sceneHeadingPrefixes 是剧本场景标题的行首标记（场号、内/外景）。大纲由剧本
// 导入时 core_event 常带"CẢNH 1 — NỘI. PHÒNG…"，writer 会把它原样抄成独立
// 一行；小说正文里这种行没有文学用途，TTS 还会照读。只认大写形式，避免误伤
// 叙述里的"cảnh 3 của vở kịch"。
var sceneHeadingPrefixes = []string{"CẢNH ", "NỘI.", "NGOẠI.", "INT.", "EXT."}

// IsSceneHeading 判断一行（已去掉 markdown 装饰）是否以剧本场景标题标记开头。
// 只认大写标记：叙述里的 "cảnh 3 của vở kịch" 不是标题。
func IsSceneHeading(line string) bool { return isSceneHeading(line) }

func isSceneHeading(line string) bool {
	for _, prefix := range sceneHeadingPrefixes {
		rest, ok := strings.CutPrefix(line, prefix)
		if !ok {
			continue
		}
		if prefix != "CẢNH " {
			return true
		}
		rest = strings.TrimSpace(rest)
		afterNumber := strings.TrimLeftFunc(rest, unicode.IsDigit)
		if len(afterNumber) == len(rest) {
			return false
		}
		suffix := strings.TrimSpace(afterNumber)
		return suffix == "" || strings.HasPrefix(suffix, "–") ||
			strings.HasPrefix(suffix, "—") || strings.HasPrefix(suffix, "-") ||
			strings.HasPrefix(suffix, ":")
	}
	return false
}

// appendSceneHeading 查独立成行的剧本场景标题：去掉 markdown 装饰（**、#）后
// 以场号"CẢNH <số>"或内外景前缀开头的行。只报第一处，消息带原行便于定位。
// 产品是"用来听"的散文：正文不该有剧本排版标注。draft_chapter 与 edit_chapter
// 落盘前先机械删掉这类整行，所以 writer 不会被要求重发整章。
func appendSceneHeading(vs []Violation, text string) []Violation {
	for line := range strings.Lines(text) {
		bare, ok := SceneHeadingLine(line)
		if !ok {
			continue
		}
		return append(vs, Violation{
			Rule:     "scene_heading",
			Target:   bare,
			Actual:   1,
			Severity: SeverityError,
			Message:  fmt.Sprintf("Văn có dòng tiêu đề cảnh kiểu kịch bản %q; chính văn tiểu thuyết không dùng nhãn CẢNH/NỘI./NGOẠI. — xóa dòng đó, đưa bối cảnh, thời gian vào câu kể rồi gửi lại", bare),
		})
	}
	return vs
}

// SceneHeadingLine 返回一行的裸文本（去掉 **、#、_ 装饰）与它是否独立成行的
// 剧本场景标题。所有调用方共用这一个判定，避免"哪里算标题"出现第二套口径。
func SceneHeadingLine(line string) (string, bool) {
	bare := trimHeadingDecoration(line)
	if bare == "" || !isSceneHeading(bare) {
		return "", false
	}
	return bare, true
}

func trimHeadingDecoration(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "*#_ "))
}

// StripStandaloneSceneHeadings 删掉正文里独立成行的剧本场景标题，返回清理后的
// 文本与被删的裸行（保持出现顺序）。只动"整行都是标题"的情况：夹在句子里的
// "CẢNH 9: …" 属于叙述，交给 SanitizeSceneHeadingText 处理。
func StripStandaloneSceneHeadings(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	var removed []string
	for _, line := range lines {
		if bare, ok := SceneHeadingLine(line); ok {
			removed = append(removed, bare)
			continue
		}
		kept = append(kept, line)
	}
	if len(removed) == 0 {
		return text, nil
	}
	return collapseBlankLines(strings.Join(kept, "\n")), removed
}

func collapseBlankLines(text string) string {
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// SanitizeSceneHeadingText 把大纲/计划里夹在句子中的剧本场景标题改写成普通
// 叙述："CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00" → "Bối cảnh: phòng trực
// hạ tầng, 02:00"。整段只有标记（例如单独的"CẢNH 9"）时直接删掉。全小写的
// "cảnh 3 của vở kịch" 不是标题，原样保留。
func SanitizeSceneHeadingText(text string) string {
	if !containsSceneHeadingMarker(text) {
		return text
	}
	runes := []rune(text)
	var b strings.Builder
	capitalizeNext := false
	trimNextSpace := false
	for start := 0; start < len(runes); {
		end := headingClauseEnd(runes, start)
		clause := string(runes[start:end])
		if trimNextSpace {
			clause = strings.TrimLeft(clause, " \t")
			trimNextSpace = false
		}
		if capitalizeNext {
			clause = capitalizeFirstLetter(clause)
			capitalizeNext = false
		}
		converted := sanitizeHeadingClause(clause)
		if converted != clause {
			if strings.HasSuffix(strings.TrimSpace(clause), ":") {
				// 标题段原本以冒号收尾（"CẢNH 8 — …: giao dịch thử…"）：改写成
				// "Bối cảnh: …." 后，后面的正文另起一句，首字母需大写。
				capitalizeNext = true
			}
			if strings.TrimSpace(converted) == "" {
				// 只剩标记被整段删掉：别把后面的正文顶着一个前导空格。
				trimNextSpace = true
			}
		}
		b.WriteString(converted)
		start = end
	}
	return b.String()
}

// capitalizeFirstLetter 把子句里第一个字母改成大写，其余原样返回。
func capitalizeFirstLetter(clause string) string {
	runes := []rune(clause)
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		runes[i] = unicode.ToUpper(r)
		return string(runes)
	}
	return clause
}

func containsSceneHeadingMarker(text string) bool {
	for _, prefix := range sceneHeadingPrefixes {
		if strings.Contains(text, prefix) {
			return true
		}
	}
	return strings.Contains(text, "NỘI/NGOẠI.") || strings.Contains(text, "INT./EXT.")
}

// headingClauseEnd 返回从 start 开始的子句结束位置（含终止符）。终止符是：
// 换行、句号（"NỘI." 这类内外景标记里的句号除外）、后面跟空白的冒号。
// 冒号后紧跟数字的是时间（02:00），不能切开。
func headingClauseEnd(runes []rune, start int) int {
	for i := start; i < len(runes); i++ {
		switch runes[i] {
		case '\n':
			return i + 1
		case '.':
			if isDirectionMarkerDot(runes, start, i) {
				continue
			}
			return i + 1
		case ':':
			if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
				return i + 1
			}
		}
	}
	return len(runes)
}

// isDirectionMarkerDot 判断 dot 是否属于 NỘI./NGOẠI./INT./EXT. 这类方向标记，
// 而不是句子结束。只看紧邻的那个词，所以 "PRINT." 里的点不算标记。
func isDirectionMarkerDot(runes []rune, start, dot int) bool {
	j := dot - 1
	for j >= start && unicode.IsLetter(runes[j]) {
		j--
	}
	word := strings.ToUpper(string(runes[j+1 : dot]))
	switch word {
	case "NỘI", "NGOẠI", "INT", "EXT":
		return true
	}
	return false
}

func sanitizeHeadingClause(clause string) string {
	left := strings.TrimLeftFunc(clause, unicode.IsSpace)
	lead := clause[:len(clause)-len(left)]
	core := strings.TrimRightFunc(left, unicode.IsSpace)
	trail := left[len(core):]

	bare := trimHeadingDecoration(core)
	if !startsSceneHeading(bare) {
		return clause
	}
	if hasLowercaseRune(bare) {
		// "NỘI./NGOẠI. Sau đó…": chỉ marker là kịch bản, phần sau là văn xuôi.
		if rest, ok := stripDirectionMarkerPrefix(core); ok {
			return lead + rest + trail
		}
		// "CẢNH 9: Triệu chặn Lâm" là văn kể, không phải slugline.
		return clause
	}
	setting := SceneSetting(bare)
	if setting == "" {
		return lead + trail
	}
	return lead + "Bối cảnh: " + setting + "." + trail
}

// stripDirectionMarkerPrefix 去掉子句开头的内外景标记（带句号），返回其后的
// 文本。只处理标记形态，所以 "NỘI THẤT"（室内）不会被切开。
func stripDirectionMarkerPrefix(clause string) (string, bool) {
	for _, marker := range []string{"NỘI./NGOẠI.", "NỘI/NGOẠI.", "INT./EXT.", "NỘI.", "NGOẠI.", "INT.", "EXT."} {
		if rest, ok := strings.CutPrefix(clause, marker); ok {
			return strings.TrimLeftFunc(rest, unicode.IsSpace), true
		}
	}
	return clause, false
}

// startsSceneHeading 判断子句是否以场号或内外景标记起头。只有标题的起头才算：
// "TỔN THẤT CẢNH 1: …" 虽然含 CẢNH 1，却不是标题。
func startsSceneHeading(bare string) bool {
	if rest, ok := strings.CutPrefix(bare, "CẢNH "); ok {
		rest = strings.TrimSpace(rest)
		first := []rune(rest)
		return len(first) > 0 && unicode.IsDigit(first[0])
	}
	for _, prefix := range []string{"NỘI VÀ NGOẠI", "NỘI/NGOẠI.", "INT./EXT.", "NỘI.", "NGOẠI.", "INT.", "EXT."} {
		if strings.HasPrefix(bare, prefix) {
			return true
		}
	}
	return false
}

func hasLowercaseRune(s string) bool {
	return strings.IndexFunc(s, unicode.IsLower) >= 0
}

// sceneDirectionLabels 是标题里只表示"内景/外景/剪辑方式"的段，去掉它们只留
// 地点与时间。只整段匹配，避免把 "NỘI THẤT"（室内）切一半。
var sceneDirectionLabels = []string{
	"NỘI VÀ NGOẠI", "NỘI/NGOẠI", "INT./EXT.", "INT/EXT", "NỘI", "NGOẠI", "INT", "EXT", "MONTAGE",
}

// SceneSetting 把标题正文收成地点与时间的普通短语（小写、逗号分隔）：
// "CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00 ĐÊM TRƯỚC BUỔI HỌP" →
// "phòng trực hạ tầng, 02:00 đêm trước buổi họp"。只剩标记时返回空串。
// 朗读脚本与上下文净化共用它，保证两边读出/写出同一句。
func SceneSetting(bare string) string {
	parts := splitSceneHeadingParts(bare)
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(part), ".,;:"))
		if part == "" || isSceneNumberLabel(part) || isSceneDirectionLabel(part) {
			continue
		}
		part = trimDirectionPrefix(part)
		if part == "" {
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.ToLower(strings.Join(kept, ", "))
}

// splitSceneHeadingParts 按"两侧带空格"的横线切段。剧本标题用空格横线分隔；
// 连字符词（INTERNET-CAFE）不算分隔符。
func splitSceneHeadingParts(bare string) []string {
	bare = strings.TrimSpace(bare)
	var parts []string
	start := 0
	runes := []rune(bare)
	for i := 1; i < len(runes)-1; i++ {
		if !isHeadingDash(runes[i]) {
			continue
		}
		if !unicode.IsSpace(runes[i-1]) || !unicode.IsSpace(runes[i+1]) {
			continue
		}
		parts = append(parts, string(runes[start:i]))
		start = i + 1
	}
	if start == 0 {
		return []string{bare}
	}
	return append(parts, string(runes[start:]))
}

func isHeadingDash(r rune) bool {
	return r == '-' || r == '–' || r == '—'
}

func isSceneNumberLabel(part string) bool {
	rest, ok := strings.CutPrefix(part, "CẢNH ")
	if !ok {
		return false
	}
	rest = strings.TrimSpace(rest)
	return rest != "" && len(strings.TrimLeftFunc(rest, unicode.IsDigit)) == 0
}

func isSceneDirectionLabel(part string) bool {
	upper := strings.ToUpper(part)
	for _, label := range sceneDirectionLabels {
		if upper == label {
			return true
		}
	}
	return false
}

// trimDirectionPrefix 去掉标题段开头的方向标记（"NỘI. PHÒNG TRỰC" →
// "PHÒNG TRỰC"）。标记必须带句号或以斜杠连接，避免误切 "NỘI THẤT"。
func trimDirectionPrefix(part string) string {
	if rest, ok := strings.CutPrefix(part, "MONTAGE "); ok {
		return strings.TrimSpace(rest)
	}
	for _, prefix := range []string{"NỘI/NGOẠI.", "INT./EXT.", "NỘI.", "NGOẠI.", "INT.", "EXT."} {
		if rest, ok := strings.CutPrefix(part, prefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return part
}

// SanitizeOutlineEntries 返回大纲条目的一份净化副本：core_event/scenes/hook 等
// 交给 writer 的文本里不再有剧本场景标题（保留下来的地点、时间改写成
// "Bối cảnh: …"）。只用于注入上下文，store 里的大纲文件不改。
func SanitizeOutlineEntries(entries []domain.OutlineEntry) []domain.OutlineEntry {
	if len(entries) == 0 {
		return entries
	}
	out := make([]domain.OutlineEntry, len(entries))
	for i, entry := range entries {
		out[i] = SanitizeOutlineEntry(entry)
	}
	return out
}

// SanitizeOutlineEntry 净化单条大纲条目。
func SanitizeOutlineEntry(entry domain.OutlineEntry) domain.OutlineEntry {
	entry.Title = SanitizeSceneHeadingText(entry.Title)
	entry.CoreEvent = SanitizeSceneHeadingText(entry.CoreEvent)
	entry.Hook = SanitizeSceneHeadingText(entry.Hook)
	entry.Promise = SanitizeSceneHeadingText(entry.Promise)
	if len(entry.Scenes) > 0 {
		scenes := make([]string, len(entry.Scenes))
		for i, scene := range entry.Scenes {
			scenes[i] = SanitizeSceneHeadingText(scene)
		}
		entry.Scenes = scenes
	}
	if len(entry.OpenQuestions) > 0 {
		questions := make([]string, len(entry.OpenQuestions))
		for i, question := range entry.OpenQuestions {
			questions[i] = SanitizeSceneHeadingText(question)
		}
		entry.OpenQuestions = questions
	}
	return entry
}

// SanitizeChapterPlan 返回章节计划的净化副本（writer 自己写的计划里也可能
// 带着从大纲抄下来的场景标题）。入参为 nil 时返回 nil。
func SanitizeChapterPlan(plan *domain.ChapterPlan) *domain.ChapterPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	out.Title = SanitizeSceneHeadingText(out.Title)
	out.Goal = SanitizeSceneHeadingText(out.Goal)
	out.Conflict = SanitizeSceneHeadingText(out.Conflict)
	out.Hook = SanitizeSceneHeadingText(out.Hook)
	out.EmotionArc = SanitizeSceneHeadingText(out.EmotionArc)
	out.OpeningState = SanitizeSceneHeadingText(out.OpeningState)
	out.Notes = SanitizeSceneHeadingText(out.Notes)
	out.Promise = SanitizeSceneHeadingText(out.Promise)
	out.Contract.RequiredBeats = sanitizeSceneTexts(out.Contract.RequiredBeats)
	out.Contract.ForbiddenMoves = sanitizeSceneTexts(out.Contract.ForbiddenMoves)
	out.Contract.ContinuityChecks = sanitizeSceneTexts(out.Contract.ContinuityChecks)
	out.Contract.EvaluationFocus = sanitizeSceneTexts(out.Contract.EvaluationFocus)
	out.Contract.PayoffPoints = sanitizeSceneTexts(out.Contract.PayoffPoints)
	out.Contract.EmotionTarget = SanitizeSceneHeadingText(out.Contract.EmotionTarget)
	out.Contract.HookGoal = SanitizeSceneHeadingText(out.Contract.HookGoal)
	out.OpenQuestions = sanitizeSceneTexts(out.OpenQuestions)
	return &out
}

func sanitizeSceneTexts(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = SanitizeSceneHeadingText(value)
	}
	return out
}
