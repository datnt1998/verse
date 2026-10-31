package web

import (
	"context"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

func hostedDocumentSources(ctx context.Context, pool *postgres.Pool, project, story string) (host.NewOption, error) {
	docs := postgres.NewTextDocuments(pool)
	opts, err := docs.RuleOptions(ctx, nil, project, story)
	if err != nil {
		return nil, err
	}
	return host.WithDocumentSources(opts, func(ctx context.Context) ([]sim.Source, error) {
		if story == "" {
			return []sim.Source{}, nil
		}
		rows, err := docs.List(ctx, nil, project, story, "style_sample")
		if err != nil {
			return nil, err
		}
		out := make([]sim.Source, 0, len(rows))
		for _, row := range rows {
			out = append(out, sim.Source{Name: row.Name, Data: []byte(row.Text), UpdatedAt: row.UpdatedAt})
		}
		return out, nil
	}), nil
}

func (g *Gateway) rulesModelTarget(ctx context.Context, t configTarget) (llmtypes.ChatModel, error) {
	if !t.hosted {
		return rulesModel(t.chainTo)
	}
	docs, err := g.hostedConfigDocuments()
	if err != nil {
		return nil, err
	}
	cfg, err := bootstrap.LoadHostedConfig(ctx, docs, t.project, ruleStory(t))
	if err != nil {
		return nil, err
	}
	return rulesModelForConfig(cfg)
}
