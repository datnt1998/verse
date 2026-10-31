package web

import (
	"context"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/platform/identity"
)

// providerValidationConfig resolves partial project/story entries against their
// parent layers, without retaining fields removed from the replaced target.
func (g *Gateway) providerValidationConfig(ctx context.Context, t configTarget, user identity.User, cfg bootstrap.Config) (bootstrap.Config, error) {
	if t.scope == "global" || len(cfg.Providers) == 0 {
		return cfg, nil
	}
	var levels []bootstrap.ConfigLevel
	var err error
	if t.hosted {
		docs, docsErr := g.hostedConfigDocuments()
		if docsErr != nil {
			return bootstrap.Config{}, docsErr
		}
		levels, _, err = bootstrap.LoadHostedConfigChain(ctx, docs, configActor(user), t.project, t.story.Story)
	} else {
		levels, _, err = bootstrap.LoadConfigChain(t.chainTo)
	}
	if err != nil {
		return bootstrap.Config{}, err
	}
	var parent bootstrap.Config
	for _, level := range levels {
		if level.Scope == t.scope {
			break
		}
		parent = bootstrap.MergeConfig(parent, level.Config)
	}
	return bootstrap.MergeConfig(parent, cfg), nil
}
