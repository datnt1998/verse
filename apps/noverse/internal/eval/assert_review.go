package eval

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// 期望严重度的顺序是 warning < error < critical。它是「作者判定这是不是一个
// 必须报告的缺陷」的口径，与 diag 的 info/warning/critical 不是同一套分类：
// diag 的 info 是优化建议，case 断言只认 warning 以上的问题。
var expectSeverityRanks = map[string]int{"critical": 0, "error": 1, "warning": 2}

func validExpectSeverity(s string) bool {
	_, ok := expectSeverityRanks[s]
	return ok
}

// expectSeverityRank 越小越严重；空值按默认 error 处理。
func expectSeverityRank(s string) int {
	if strings.TrimSpace(s) == "" {
		s = defaultExpectSeverity
	}
	if rank, ok := expectSeverityRanks[s]; ok {
		return rank
	}
	return 99
}

// normalizeQuoteText 是引文比对的唯一口径：小写、弯引号/直引号合一、空白压缩。
// 不做标点剥离——引文是证据片段，标点也是它的一部分。
func normalizeQuoteText(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range s {
		switch r {
		case '“', '”', '„', '‟', '«', '»', '「', '」', '『', '』':
			r = '"'
		case '‘', '’', '‚', '‛':
			r = '\''
		}
		r = unicode.ToLower(r)
		if unicode.IsSpace(r) {
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteRune(' ')
		}
		pendingSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// quoteMatches 判断引文是否落在给定文本里。空引文视为不设条件。
func quoteMatches(haystack, needle string) bool {
	normalizedNeedle := normalizeQuoteText(needle)
	if normalizedNeedle == "" {
		return true
	}
	return strings.Contains(normalizeQuoteText(haystack), normalizedNeedle)
}

// issueEvidence 汇总一条 issue 里所有可引用文本，供 quote_contains 使用。
func issueEvidence(issue domain.ConsistencyIssue) string {
	return strings.Join([]string{issue.Quote, issue.Evidence, issue.Description}, "\n")
}

// reviewSortKey 让评审的排列稳定：先按章，再按 chapter < arc < global。
func reviewSortKey(r domain.ReviewEntry) (int, int) {
	scope := 0
	switch r.Scope {
	case "arc":
		scope = 1
	case "global":
		scope = 2
	}
	return r.Chapter, scope
}

func sortReviews(reviews []domain.ReviewEntry) {
	sort.SliceStable(reviews, func(i, j int) bool {
		ci, si := reviewSortKey(reviews[i])
		cj, sj := reviewSortKey(reviews[j])
		if ci != cj {
			return ci < cj
		}
		return si < sj
	})
}

// gradeReviewExpect 断言运行时保存的 ReviewEntry。没有 review 就是 FAIL 并计入
// mechanical_invalid：Editor 没落盘评审时，任何"没报错"的结论都是假的。
func gradeReviewExpect(c Case, col Collected, res *Result) {
	e := c.Expect.Review
	if e == nil {
		return
	}
	res.ReviewChecked = true

	reviews := slices.Clone(col.Reviews)
	sortReviews(reviews)
	if len(reviews) == 0 {
		res.addHardFail("review", "không lưu được review nào để chấm")
	} else {
		primary := reviews[len(reviews)-1]
		if strings.TrimSpace(primary.Verdict) == "" {
			res.addHardFail("review", "review đã lưu nhưng thiếu verdict nên không dùng được")
		} else {
			res.MechanicalValid = true
			res.addPass("review", "đã lưu review (chapter="+fmt.Sprint(primary.Chapter)+", scope="+primary.Scope+")")
		}
		if len(e.VerdictIn) > 0 {
			switch {
			case primary.Verdict == "":
			case !slices.Contains(e.VerdictIn, primary.Verdict):
				res.addHardFail("review:verdict", fmt.Sprintf("verdict=%s không nằm trong %v", primary.Verdict, e.VerdictIn))
			default:
				res.addPass("review:verdict", "verdict="+primary.Verdict)
			}
		}
	}

	for _, want := range e.MustFind {
		res.MustFindTotal++
		minRank := expectSeverityRank(want.SeverityMin)
		if found, ok := findIssue(reviews, want, minRank); ok {
			res.MustFindHit++
			res.addPass("review:must_find", describeFindExpect(want)+" → "+found)
			continue
		}
		res.addHardFail("review:must_find", "không bắt được: "+describeFindExpect(want))
	}

	violations := map[string]bool{}
	for _, reject := range e.MustNotFind {
		minRank := expectSeverityRank(reject.SeverityMin)
		for ri, review := range reviews {
			for ii, issue := range review.Issues {
				if expectSeverityRank(issue.Severity) > minRank {
					continue
				}
				if !quoteMatches(issueEvidence(issue), reject.QuoteContains) {
					continue
				}
				if !quoteMatches(issue.Type, reject.TypeContains) {
					continue
				}
				key := fmt.Sprintf("%d:%d", ri, ii)
				if violations[key] {
					continue
				}
				violations[key] = true
				res.FalsePositives++
				res.addHardFail("review:must_not_find", "báo nhầm: "+describeIssue(issue))
			}
		}
	}

	// 对照 case 的正确答案是「除 must_find 声明的缺陷外什么也没发现」：其余 error 以上的
	// issue 都是假阳性（例如正文里夹带指令的对照 case 仍要求抓到真实的时间线错误）。
	// 与 must_not_find 命中的同一条 issue 只算一次。
	if c.Control {
		for ri, review := range reviews {
			for ii, issue := range review.Issues {
				if expectSeverityRank(issue.Severity) > expectSeverityRanks["error"] {
					continue
				}
				if matchesAnyFind(review, issue, e.MustFind) {
					continue
				}
				key := fmt.Sprintf("%d:%d", ri, ii)
				if violations[key] {
					continue
				}
				violations[key] = true
				res.FalsePositives++
				res.addHardFail("review:control", "case đối chứng nhưng vẫn báo lỗi: "+describeIssue(issue))
			}
		}
	}

	// Match goal IDs against the latest chapter-scoped review.
	if len(e.GoalStatus) > 0 {
		var primary *domain.ReviewEntry
		for i := len(reviews) - 1; i >= 0; i-- {
			if reviews[i].Scope == "chapter" {
				primary = &reviews[i]
				break
			}
		}
		got := map[string]string{}
		if primary != nil {
			for _, c := range primary.GoalChecks {
				got[c.ID] = c.Status
			}
		}
		ids := make([]string, 0, len(e.GoalStatus))
		for id := range e.GoalStatus {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			want := e.GoalStatus[id]
			if want == "unknown" {
				res.UnknownExpected++
			}
			status, ok := got[id]
			switch {
			case !ok:
				res.addHardFail("review:goal_status", fmt.Sprintf("%s: thiếu trong goal_checks, mong đợi %s", id, want))
			case status != want:
				res.addHardFail("review:goal_status", fmt.Sprintf("%s: status=%s, mong đợi %s", id, status, want))
			default:
				if want == "unknown" {
					res.UnknownHit++
				}
				res.addPass("review:goal_status", fmt.Sprintf("%s=%s", id, status))
			}
		}
	}
	if len(e.RecheckStatus) > 0 {
		// Gold keys may be stable IDs or quotes from the archived source round.
		got := make(map[string]string)
		for _, review := range reviews {
			if !review.Recheck || review.Scope != "chapter" {
				continue
			}
			for _, result := range review.RecheckResults {
				got[result.FindingID] = result.Status
				for _, source := range col.ReviewRounds {
					if source.Chapter != review.Chapter || source.Scope != "chapter" || source.Recheck {
						continue
					}
					ids := domain.ReviewEntryItemIDs(source)
					for i, issue := range source.Issues {
						if ids[i] != result.FindingID {
							continue
						}
						quote := issue.Quote
						if strings.TrimSpace(quote) == "" {
							quote = issue.Evidence
						}
						got[normalizeQuoteText(quote)] = result.Status
					}
				}
			}
		}
		keys := make([]string, 0, len(e.RecheckStatus))
		for key := range e.RecheckStatus {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			want := e.RecheckStatus[key]
			status := got[key]
			if status == "" {
				status = got[normalizeQuoteText(key)]
			}
			if status != want {
				res.addHardFail("review:recheck_status", fmt.Sprintf("%s: status=%s, mong đợi %s", key, status, want))
			} else {
				res.addPass("review:recheck_status", fmt.Sprintf("%s=%s", key, status))
			}
		}
	}
}

// findIssue 返回第一条满足期望的 issue 摘要。"必须抓到"只要求存在，不要求排序。
func findIssue(reviews []domain.ReviewEntry, want MustFindExpect, minRank int) (string, bool) {
	for _, review := range reviews {
		for _, issue := range review.Issues {
			if issueMatchesFind(review, issue, want, minRank) {
				return describeIssue(issue), true
			}
		}
	}
	return "", false
}

func issueMatchesFind(review domain.ReviewEntry, issue domain.ConsistencyIssue, want MustFindExpect, minRank int) bool {
	if expectSeverityRank(issue.Severity) > minRank {
		return false
	}
	if want.RuleRef != "" && !strings.EqualFold(strings.TrimSpace(issue.RuleRef), strings.TrimSpace(want.RuleRef)) {
		return false
	}
	if want.Rule != "" && !strings.EqualFold(strings.TrimSpace(issue.Rule), strings.TrimSpace(want.Rule)) {
		return false
	}
	if want.Chapter > 0 && review.Chapter != want.Chapter && !slices.Contains(issue.Chapters, want.Chapter) {
		return false
	}
	return quoteMatches(issueEvidence(issue), want.QuoteContains)
}

func matchesAnyFind(review domain.ReviewEntry, issue domain.ConsistencyIssue, wants []MustFindExpect) bool {
	for _, want := range wants {
		if issueMatchesFind(review, issue, want, expectSeverityRank(want.SeverityMin)) {
			return true
		}
	}
	return false
}

func describeFindExpect(want MustFindExpect) string {
	parts := []string{"severity≥" + defaultOr(want.SeverityMin, defaultExpectSeverity)}
	if want.RuleRef != "" {
		parts = append(parts, "rule_ref="+want.RuleRef)
	}
	if want.Rule != "" {
		parts = append(parts, "rule="+want.Rule)
	}
	if want.Chapter > 0 {
		parts = append(parts, fmt.Sprintf("chapter=%d", want.Chapter))
	}
	if want.QuoteContains != "" {
		parts = append(parts, "quote≈"+want.QuoteContains)
	}
	return strings.Join(parts, " ")
}

func describeIssue(issue domain.ConsistencyIssue) string {
	line := "[" + issue.Severity + "] " + issue.Type
	if issue.Quote != "" {
		line += " «" + issue.Quote + "»"
	}
	return line
}

func defaultOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func mapKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
