package web

import (
	"context"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// MigrateRules completes the one-time retirement of the global rules layer
// before requests or engine workers can observe the project-only rule chain.
func (g *Gateway) MigrateRules(ctx context.Context) error {
	if repo := g.hostedTextDocuments(); repo != nil {
		return repo.MigrateGlobalRules(ctx, rulesHomeDir())
	}
	return g.migrateFileRules(g.RootPath())
}

func (g *Gateway) migrateFileRules(root string) error {
	return rules.MigrateGlobalFiles(root, rulesHomeDir(), g.accounts)
}
