package lessons

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// schemaVersion is the current lessons.json format. Version 2 folds the legacy
// severity-split ids into the merged group ids and drops ids that belong to no
// current or legacy group.
const schemaVersion = 2

// legacyID is the id written before levels stopped splitting a lesson. The key
// shape lives here alone: nothing else in the package may hash it.
func legacyID(packID, ref, level string) string {
	sum := sha256.Sum256([]byte(packID + "\x00" + ref + "\x00" + level))
	return "lesson-" + hex.EncodeToString(sum[:12])
}

func stateRank(state string) int {
	switch state {
	case "kept":
		return 2
	case "dropped":
		return 1
	default:
		return 0
	}
}

// earlier returns the earlier non-zero timestamp, so merged lessons keep the
// moment the pattern first appeared.
func earlier(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

// mergeLessons folds two entries of the same group: kept > dropped > pending, the
// decision of the winning entry survives, and createdAt is the earliest of both.
func mergeLessons(a, b Lesson) Lesson {
	winner, loser := a, b
	if stateRank(b.State) > stateRank(a.State) || (stateRank(b.State) == stateRank(a.State) && b.CreatedAt.Before(a.CreatedAt)) {
		winner, loser = b, a
	}
	winner.CreatedAt = earlier(winner.CreatedAt, loser.CreatedAt)
	return winner
}

// migrateLessons rewrites a legacy catalog in place and reports whether it must be
// written back. Files already at schemaVersion are left untouched, so a second run
// is a no-op.
func migrateLessons(c *catalog) bool {
	if c.Schema >= schemaVersion {
		return false
	}
	merged := make([]Lesson, 0, len(c.Lessons))
	index := map[string]int{}
	for _, l := range c.Lessons {
		if len(l.Refs) == 0 {
			continue
		}
		ref := l.Refs[0]
		target := ID(GroupKey(l.PackID, ref))
		if l.ID != target {
			if l.ID != legacyID(l.PackID, ref, l.Evidence.legacyKind) {
				continue
			}
			l.ID = target
		}
		// Mục dùng chung đang chờ duyệt về phạm vi mặc định “Mọi gói” (câu 15); quyết định đã
		// có giữ đúng phạm vi người dùng chọn, mục `Bộ …` không bị chạm.
		if !IsPackRef(ref) && l.State == "pending" {
			l.Scope = "all"
		}
		if i, ok := index[target]; ok {
			merged[i] = mergeLessons(merged[i], l)
			continue
		}
		index[target] = len(merged)
		merged = append(merged, l)
	}
	c.Lessons = merged
	c.Schema = schemaVersion
	return true
}
