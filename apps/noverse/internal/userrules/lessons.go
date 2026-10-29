package userrules

import (
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// CaptureLessons freezes the applicable Mọi truyện/pack layers before the first
// Architect prompt. Legacy stories deliberately capture an empty layer.
func CaptureLessons(st *store.Store, packID string) ([]rules.Candidate, error) {
	if candidates, found, err := st.UserRules.LessonRules(); err != nil || found {
		return candidates, err
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
	return st.UserRules.CaptureLessonRules(candidates)
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
	b.WriteString("\n\n## Bài học từ mọi truyện\nChỉ áp dụng các bài học đã giữ khi bắt đầu truyện này:\n")
	for _, candidate := range candidates {
		fmt.Fprintf(&b, "- %s\n", candidate.Preferences)
	}
	return b.String(), nil
}
