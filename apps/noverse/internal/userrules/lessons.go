package userrules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// CaptureLessons freezes system defaults and applicable pack lessons before
// the first Architect prompt. Neither creates a user-editable global scope.
func CaptureLessons(st *store.Store, packID string) ([]rules.Candidate, error) {
	if candidates, found, err := st.UserRules.LessonRules(); err != nil || found {
		return defaultLessonCandidates(candidates), err
	}
	book, err := st.Book.Load()
	if err != nil {
		return nil, err
	}
	snapshot, err := st.UserRules.LoadCurrent()
	if err != nil {
		return nil, err
	}
	if book != nil || snapshot != nil {
		return st.UserRules.CaptureLessonRules(nil)
	}
	s, err := lessons.Open()
	if err != nil {
		return nil, err
	}
	candidates, err := s.Rules(packID)
	if err != nil {
		return nil, err
	}
	return st.UserRules.CaptureLessonRules(defaultLessonCandidates(candidates))
}

// LessonContext is shared by both Architect workers and rule snapshot assembly.
func LessonContext(st *store.Store, packID string) (string, error) {
	candidates, err := CaptureLessons(st, packID)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("\n\n## Bài học mặc định\nChỉ áp dụng các bài học đã giữ khi bắt đầu truyện này:\n")
	for _, candidate := range candidates {
		fmt.Fprintf(&b, "- %s\n", candidate.Preferences)
	}
	return b.String(), nil
}

func defaultLessonCandidates(candidates []rules.Candidate) []rules.Candidate {
	for _, candidate := range candidates {
		if candidate.Source == "global:lessons" {
			out := slices.Clone(candidates)
			for i := range out {
				if out[i].Source == "global:lessons" {
					out[i].Source = "default:lessons"
				}
			}
			return out
		}
	}
	return candidates
}

// Relabel historical sources after global rules have migrated into projects.
// Thresholds, approved lesson content and chapter pins remain unchanged.
func migratedRuleSources(snapshot *rules.Snapshot) *rules.Snapshot {
	if snapshot == nil || !slices.ContainsFunc(snapshot.Sources, func(source string) bool {
		return strings.HasPrefix(source, "global:")
	}) {
		return snapshot
	}
	out := *snapshot
	out.Sources = slices.Clone(snapshot.Sources)
	for i, source := range out.Sources {
		replacement := migratedRuleSource(source)
		out.Sources[i] = replacement
		if replacement != source {
			out.Preferences = strings.ReplaceAll(out.Preferences, "## ["+source+"]\n\n", "## ["+replacement+"]\n\n")
		}
	}
	return &out
}

func migratedRuleSource(source string) string {
	if source == "global:lessons" {
		return "default:lessons"
	}
	if name, ok := strings.CutPrefix(source, "global:"); ok {
		return "project:" + name
	}
	return source
}
