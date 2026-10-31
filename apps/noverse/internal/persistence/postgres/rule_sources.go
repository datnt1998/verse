package postgres

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// RuleOptions loads the same project → story chain as file mode.
// Sources keep their layer and filename so RawFileSources applies the existing
// nearer-wins rule (with the managed file's additive exception) unchanged.
func (r *TextDocuments) RuleOptions(ctx context.Context, actor *Actor, project, story string) (rules.LoadOptions, error) {
	opts := rules.LoadOptions{HomeRulesDir: rules.DefaultHomeRulesDir()}
	for _, layer := range []struct {
		story string
		kind  rules.SourceKind
	}{{"", rules.SourceProject}, {story, rules.SourceStory}} {
		if layer.kind == rules.SourceStory && story == "" {
			continue
		}
		docs, err := r.List(ctx, actor, project, layer.story, "rules")
		if err != nil {
			return opts, err
		}
		for _, doc := range docs {
			if strings.HasPrefix(doc.Name, ".") || filepath.Base(doc.Name) != doc.Name || !strings.EqualFold(filepath.Ext(doc.Name), ".md") || strings.TrimSpace(doc.Text) == "" {
				continue
			}
			opts.Sources = append(opts.Sources, rules.RawSource{Label: layer.kind.String() + ":" + doc.Name, Name: doc.Name, Kind: layer.kind, Text: strings.TrimSpace(doc.Text)})
		}
	}
	return opts, nil
}
