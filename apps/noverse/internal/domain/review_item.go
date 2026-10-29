package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var reviewPackRefPattern = regexp.MustCompile(`^(.+?)\s*§\s*(\d+(?:\.\d+)*)$`)

// ReviewIssueRef normalizes the identity reference independently of pack titles.
func ReviewIssueRef(issue ConsistencyIssue) string {
	raw := strings.TrimSpace(issue.RuleRef)
	switch raw {
	case "Xưng hô", "Quy tắc của bạn":
		return raw
	}
	if m := reviewPackRefPattern.FindStringSubmatch(raw); m != nil {
		return strings.TrimSpace(m[1]) + " §" + m[2]
	}
	return "Quy tắc của bạn"
}

// ReviewItemID preserves persisted web decisions, including duplicate suffixes.
func ReviewItemID(chapter int, used map[string]int, ref, quote string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{ref, fmt.Sprint(chapter), strings.TrimSpace(quote)}, "\x00")))
	base := fmt.Sprintf("ch%d-%s", chapter, hex.EncodeToString(sum[:])[:8])
	used[base]++
	if n := used[base]; n > 1 {
		return fmt.Sprintf("%s-%d", base, n)
	}
	return base
}

// ReviewEntryItemIDs uses a chapter-local duplicate counter, never a mixed payload's.
func ReviewEntryItemIDs(entry ReviewEntry) []string {
	ids := make([]string, 0, len(entry.Issues))
	used := make(map[string]int)
	for _, issue := range entry.Issues {
		quote := issue.Quote
		if strings.TrimSpace(quote) == "" {
			quote = issue.Evidence
		}
		ids = append(ids, ReviewItemID(entry.Chapter, used, ReviewIssueRef(issue), quote))
	}
	return ids
}

// ReviewIssueTargets restricts a finding to its explicit chapter or chapter scope.
func ReviewIssueTargets(entry ReviewEntry, issue ConsistencyIssue, chapter int) bool {
	if len(issue.Chapters) == 0 {
		return entry.Scope == "chapter" && entry.Chapter == chapter
	}
	for _, ch := range issue.Chapters {
		if ch == chapter {
			return true
		}
	}
	return false
}
