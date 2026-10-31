package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/bundle"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	stepbackdrop "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cloneOwnerEdits(store *postgres.CloneStore, files *media.Service, publish pipeline.Publisher, wake func()) *jobedit.Service {
	return &jobedit.Service{Store: store, Media: files, Publish: publish, Wake: wake}
}
func cloneReadImageManifest(edits *jobedit.Service) func(context.Context, domain.CloneJob) (*images.Manifest, error) {
	return func(ctx context.Context, job domain.CloneJob) (*images.Manifest, error) {
		snapshot, err := edits.Read(ctx, job.ID, domain.CloneImages)
		if err != nil {
			return nil, err
		}
		if len(snapshot.Manifest) == 0 {
			return nil, nil
		}
		var manifest images.Manifest
		if err = json.Unmarshal(snapshot.Manifest, &manifest); err != nil {
			return nil, err
		}
		return &manifest, nil
	}
}
func mountCloneOwnerOperations(g *web.Gateway, pool *pgxpool.Pool, edits *jobedit.Service, jobs *pipeline.Service, ai *aiWiring, librarySvc *library.Service, backdropSvc *backdrop.Service, toolKit *tools.Tools, toolDeps tools.Deps, editorService *editor.Service, ownerName func(context.Context, string) (string, error)) func(context.Context) error {
	images := &cloneimages.Service{Edits: edits, Prompts: librarySvc, Ffmpeg: toolKit.Ffmpeg, Runner: toolDeps.Runner, QuotaLeft: func(ctx context.Context) (int, error) {
		artist, err := ai.Service.ResolveArtist(ctx)
		if err != nil {
			return 0, err
		}
		providers, _ := ai.Executors.imageProviders(ctx, artist.DrawChain)
		now := time.Now()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		seen := map[string]bool{}
		left := 0
		for _, provider := range providers {
			if seen[provider.Name()] || !provider.Paid() {
				continue
			}
			seen[provider.Name()] = true
			if err := provider.Ready(ctx); err != nil {
				continue
			}
			charged, err := ai.Executors.journal.Charged(ctx, provider.Name(), day)
			if err != nil {
				return 0, err
			}
			left += max(0, ai.Executors.limit-int(charged))
		}
		return left, nil
	}}
	backdrops := &clonebackdrop.Service{Edits: edits, Deps: stepbackdrop.ProductionDeps(postgres.NewBackdropStore(pool), backdropSvc)}
	bundles := &bundle.Service{Edits: edits, Exports: editorService, Ffmpeg: toolKit.Ffmpeg, Runner: toolDeps.Runner, OwnerName: ownerName}
	store := edits.Store.(*postgres.CloneStore)
	bulkOps := &bulk.Service{Jobs: jobs, Images: images, Store: store, Exports: editorService, Publish: edits.Publish, Approvals: store, ApproveProject: store.PrepareCloneExport}
	web.MountCloneOperations(g, web.CloneOperations{Images: images, Backdrops: backdrops, Edits: edits, Bundles: bundles, Bulk: bulkOps})
	return bulkOps.DispatchReviews
}
