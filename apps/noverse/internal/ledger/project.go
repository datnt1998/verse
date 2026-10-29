package ledger

import (
	"path/filepath"
	"slices"

	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/library"
)

// ProjectHistory reloads sibling ledger facts for the Architect's save gate.
// Standalone books have no project-wide ledger.
func ProjectHistory(output string) ([]Story, error) {
	var err error
	output, err = filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	project := library.FindProjectDir(output)
	if project == "" {
		return nil, nil
	}
	root := filepath.Dir(project)
	lib, err := library.Open(root)
	if err != nil {
		return nil, err
	}
	p := filepath.Base(project)
	dirs, err := lib.Stories(p)
	if err != nil {
		return nil, err
	}
	prior := []Story{}
	for _, d := range dirs {
		s, ok := lib.ResolveStory(p, d.Name)
		if !ok {
			continue
		}
		cfg, err := bootstrap.LoadConfigAt(s.Dir)
		if err != nil {
			return nil, err
		}
		cfg.FillDefaults()
		out := cfg.OutputDir
		if !filepath.IsAbs(out) {
			out = filepath.Join(s.Dir, out)
		}
		if filepath.Clean(out) == filepath.Clean(output) {
			continue
		}
		r, err := Load(out)
		if err != nil {
			return nil, err
		}
		var values Values
		if r != nil {
			values = r.Values
		}
		title := s.Story
		if f, err := library.LoadStoryFile(s.Dir); err != nil {
			return nil, err
		} else if f != nil && f.Title != "" {
			title = f.Title
		}
		prior = append(prior, Story{ID: s.ID, Title: title, Values: values, Opening: values[8]})
	}
	slices.Reverse(prior)
	return prior, nil
}
