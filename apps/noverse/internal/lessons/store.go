package lessons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/gofrs/flock"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrNotFound = errors.New("Không tìm thấy bài học")
	ErrAction   = errors.New("Thao tác bài học không hợp lệ")
)

// The additive file is the rule layer's source of truth: kept entries are rules,
// partitioned by scope (Mọi truyện) or packId. No pack asset is modified.
type catalog struct {
	Schema    int       `json:"schema,omitempty"`
	Lessons   []Lesson  `json:"lessons"`
	Summaries []Summary `json:"summaries,omitempty"`
}
type Store struct{ dir string }

type Summary struct {
	PackID   string `json:"packId"`
	PackName string `json:"packName"`
	Rows     []Row  `json:"rows"`
}

// equalRows so sánh theo giá trị: AlertStories là con trỏ nên `slices.Equal` chỉ so địa chỉ,
// và bản đọc lại từ JSON luôn có địa chỉ mới.
func equalRows(a, b []Row) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if (x.AlertStories == nil) != (y.AlertStories == nil) {
			return false
		}
		if x.AlertStories != nil {
			if *x.AlertStories != *y.AlertStories {
				return false
			}
			x.AlertStories, y.AlertStories = nil, nil
		}
		if x != y {
			return false
		}
	}
	return true
}

func NewStore(rulesDir string) *Store { return &Store{dir: rulesDir} }

func Open() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewStore(filepath.Join(home, ".ainovel", "rules")), nil
}

func (s *Store) load() (catalog, bool, error) {
	c := catalog{Lessons: []Lesson{}}
	err := store.NewIO(s.dir).ReadJSON("lessons.json", &c)
	if os.IsNotExist(err) {
		return c, false, nil
	}
	// catalog viết tay (hoặc bản cũ) có thể thiếu evidence.chips → marshal lại
	// ra "chips": null; contract API luôn là mảng.
	for i := range c.Lessons {
		if c.Lessons[i].Evidence.Chips == nil {
			c.Lessons[i].Evidence.Chips = []string{}
		}
	}
	return c, true, err
}

// read rewrites a legacy lessons.json once, under the store lock, before serving it.
func (s *Store) read() (catalog, error) {
	c, exists, err := s.load()
	if err != nil || !exists || c.Schema >= schemaVersion {
		return c, err
	}
	if err := s.transaction(func(*catalog) (bool, error) { return false, nil }); err != nil {
		return c, err
	}
	c, _, err = s.load()
	return c, err
}

func (s *Store) transaction(fn func(*catalog) (bool, error)) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(s.dir, "lessons.lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()
	io := store.NewIO(s.dir)
	c := catalog{Lessons: []Lesson{}}
	if err := io.ReadJSON("lessons.json", &c); err != nil && !os.IsNotExist(err) {
		return err
	}
	changed := migrateLessons(&c)
	// catalog viết tay (hoặc bản cũ) có thể thiếu evidence.chips → marshal lại
	// ra "chips": null; contract API luôn là mảng. Sửa nó như một phần migrate.
	for i := range c.Lessons {
		if c.Lessons[i].Evidence.Chips == nil {
			c.Lessons[i].Evidence.Chips = []string{}
			changed = true
		}
	}
	more, err := fn(&c)
	if err != nil {
		return err
	}
	if !changed && !more {
		return nil
	}
	return io.WriteJSON("lessons.json", c)
}

// Sync refreshes evidence without resurrecting dropped lessons or losing edits.
func (s *Store) Sync(candidates []Lesson, summaries ...Summary) ([]Lesson, error) {
	if len(candidates) == 0 && len(summaries) == 0 {
		c, err := s.read()
		return c.Lessons, err
	}
	out := []Lesson{}
	err := s.transaction(func(c *catalog) (bool, error) {
		changed := false
		sameSummaries := len(c.Summaries) == len(summaries)
		for i, summary := range summaries {
			if !sameSummaries {
				break
			}
			old := c.Summaries[i]
			sameSummaries = old.PackID == summary.PackID && old.PackName == summary.PackName && equalRows(old.Rows, summary.Rows)
		}
		if len(summaries) > 0 && !sameSummaries {
			c.Summaries = summaries
			changed = true
		}
		byID := map[string]int{}
		for i, l := range c.Lessons {
			byID[l.ID] = i
		}
		for _, candidate := range candidates {
			if i, ok := byID[candidate.ID]; ok {
				if !c.Lessons[i].Evidence.equal(candidate.Evidence) {
					c.Lessons[i].Evidence = candidate.Evidence
					changed = true
				}
			} else {
				if candidate.CreatedAt.IsZero() {
					candidate.CreatedAt = time.Now().UTC()
				}
				c.Lessons = append(c.Lessons, candidate)
				changed = true
			}
		}
		out = append(out, c.Lessons...)
		return changed, nil
	})
	return out, err
}

func (s *Store) Decide(id, action, text, scope string) (Lesson, error) {
	var out Lesson
	err := s.transaction(func(c *catalog) (bool, error) {
		for i := range c.Lessons {
			l := &c.Lessons[i]
			if l.ID != id {
				continue
			}
			switch action {
			case "keep", "drop":
				if l.State != "pending" {
					return false, ErrAction
				}
				if scope != "" && scope != "pack" && scope != "all" {
					return false, ErrAction
				}
				if scope != "" {
					l.Scope = scope
				}
				if action == "keep" {
					l.State = "kept"
				} else {
					l.State = "dropped"
				}
				now := time.Now().UTC()
				l.DecidedAt = &now
			case "edit":
				if l.State != "pending" || strings.TrimSpace(text) == "" {
					return false, ErrAction
				}
				l.Text = strings.TrimSpace(text)
			case "undo":
				if l.State == "pending" {
					return false, ErrAction
				}
				l.State = "pending"
				l.DecidedAt = nil
			default:
				return false, ErrAction
			}
			out = *l
			return true, nil
		}
		return false, ErrNotFound
	})
	return out, err
}

// Rules returns approved rules from exactly the applicable additive layers.
func (s *Store) Rules(packID string) ([]rules.Candidate, error) {
	c, err := s.read()
	if err != nil {
		return nil, err
	}
	out := []rules.Candidate{}
	for _, summary := range c.Summaries {
		if summary.PackID != packID {
			continue
		}
		for _, row := range summary.Rows {
			if row.Alert {
				out = append(out, rules.Candidate{Source: "pack:" + packID + ":lessons-preamble", Preferences: Preamble(summary.Rows, summary.PackName)})
				break
			}
		}
	}
	for _, l := range c.Lessons {
		if l.State != "kept" || (l.Scope != "all" && l.PackID != packID) {
			continue
		}
		source := "global:lessons"
		if l.Scope == "pack" {
			source = "pack:" + l.PackID + ":lessons"
		}
		out = append(out, rules.Candidate{Source: source, Preferences: l.Text})
	}
	return out, nil
}

// ValidateText rejects story identities and verbatim prose in an edited lesson.
// Names and passages are validation-only; neither is stored in the rule layer.
func ValidateText(text string, names, passages []string) error {
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 2000 {
		return fmt.Errorf("Câu bài học cần từ 1 đến 2.000 ký tự")
	}
	fold := func(s string) string {
		s = strings.Map(func(r rune) rune {
			if unicode.IsMark(r) {
				return -1
			}
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return ' '
		}, norm.NFD.String(s))
		return strings.Join(strings.Fields(s), " ")
	}
	candidate := " " + fold(text) + " "
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && strings.Contains(candidate, " "+fold(name)+" ") {
			return fmt.Errorf("Bài học không được chứa tên riêng của truyện")
		}
	}
	for _, p := range passages {
		words := strings.Fields(fold(p))
		for i := 0; i+6 <= len(words); i++ {
			if strings.Contains(candidate, strings.Join(words[i:i+6], " ")) {
				return fmt.Errorf("Bài học không được chép tình tiết của truyện")
			}
		}
	}
	return nil
}
