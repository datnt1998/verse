package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// Check 对章节正文按结构化规则进行机械检查，返回违规事实列表。
//
// 设计契约：
//   - 仅返事实，不下指令（铁律一）
//   - 不阻断任何调用方流程
//   - severity 按规则类型固定映射（参见 types.go 注释表）
//
// 参数：
//   - text：章节正文（终稿或草稿都可）
//   - s：合并后的结构化规则；IsEmpty 时直接返回 nil。
func Check(text string, s Structured, openings ...Opening) []Violation {
	if s.IsEmpty() {
		return nil
	}

	var violations []Violation
	violations = appendForbiddenChars(violations, text, s.ForbiddenChars)
	violations = appendForbiddenPhrases(violations, text, s.ForbiddenPhrases)
	violations = appendFatigueWords(violations, text, s.FatigueWords)
	return appendMeasurements(violations, text, s.Measurements, openings)
}

// forbidden_chars：出现 ≥1 次即 error。
// 同一条规则只产生一条 violation，actual 是出现次数。
func appendForbiddenChars(vs []Violation, text string, list []string) []Violation {
	for _, ch := range list {
		if ch == "" {
			continue
		}
		n := strings.Count(text, ch)
		if n == 0 {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "forbidden_chars",
			Target:   ch,
			Actual:   n,
			Severity: SeverityError,
		})
	}
	return vs
}

// forbidden_phrases：出现 ≥1 次即 error；行为与 forbidden_chars 一致，仅 rule 名区分。
func appendForbiddenPhrases(vs []Violation, text string, list []string) []Violation {
	for _, ph := range list {
		if ph == "" {
			continue
		}
		n := strings.Count(text, ph)
		if n == 0 {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "forbidden_phrases",
			Target:   ph,
			Actual:   n,
			Severity: SeverityError,
		})
	}
	return vs
}

// fatigue_words：按密度判定——出现次数超过 per_1000×tiếng/1000 才违规（tiếng 是
// 章内音节数，用 domain.WordCount 数：空白分隔、至少含一个字母/数字的 token 算 1 tiếng）。
// 无论阈值多低都至少允许 1 次，短章不至于一出现就报警。不跨章累计——跨章问题后续交诊断。
func appendFatigueWords(vs []Violation, text string, m map[string]FatiguePer1000) []Violation {
	tieng := domain.WordCount(text)
	for word, limit := range m {
		per1000 := limit.Per1000()
		if word == "" || per1000 <= 0 {
			continue
		}
		n := strings.Count(text, word)
		if n == 0 {
			continue
		}
		allowedF := per1000 * float64(tieng) / 1000
		if allowedF < 1 {
			allowedF = 1 // chương ngắn: luôn cho phép ít nhất 1 lần
		}
		if float64(n) <= allowedF {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "fatigue_words",
			Target:   word,
			Limit:    int(allowedF), // số lần được phép ở chương này — UI cũ vẫn đọc "tối đa N"
			Per1000:  per1000,
			Actual:   n,
			Severity: SeverityWarning,
			Message: fmt.Sprintf("“%s” %d lần/%s tiếng, vượt %s lần/1.000 tiếng",
				word, n, viInt(tieng), viDensity(per1000)),
		})
	}
	return vs
}

// viInt 按越南语习惯给整数加千位点：1400 → "1.400"。
func viInt(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}

// viDensity 按越南语习惯格式化密度（2 位小数、逗号小数点）：1.33 → "1,33"。
func viDensity(f float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(f, 'f', 2, 64), ".", ",")
}
