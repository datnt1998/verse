package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/bundle"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
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

// withVideoDrawSource đặt nguồn vẽ riêng của video (lệnh /provider) lên đầu
// chuỗi vẽ; chuỗi chung ở Cài đặt vẫn đứng sau làm dự phòng. Override nằm trong
// manifest của chính video đó nên ảnh của video khác không đổi.
func withVideoDrawSource(ctx context.Context, edits *jobedit.Service, job domain.CloneJob, chain []domain.ModelRef) []domain.ModelRef {
	snapshot, err := edits.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		return chain
	}
	raw := snapshot.Manifest
	if len(raw) == 0 {
		raw = snapshot.Step.Checkpoint
	}
	if len(raw) == 0 {
		return chain
	}
	var manifest images.Manifest
	if json.Unmarshal(raw, &manifest) != nil || manifest.Overrides == nil || manifest.Overrides.DrawProvider == "" {
		return chain
	}
	override := domain.ModelRef{Provider: manifest.Overrides.DrawProvider, Model: manifest.Overrides.DrawModel}
	out := []domain.ModelRef{override}
	for _, ref := range chain {
		if ref.Provider == override.Provider && ref.Model == override.Model {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func mountCloneOwnerOperations(g *web.Gateway, pool *pgxpool.Pool, edits *jobedit.Service, jobs *pipeline.Service, ai *aiWiring, librarySvc *library.Service, backdropSvc *backdrop.Service, toolKit *tools.Tools, toolDeps tools.Deps, editorService *editor.Service, ownerName func(context.Context, string) (string, error), metadata *clonemetadata.Service, autonomySvc *autonomy.Service, seams *assistantSeams) func(context.Context) error {
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
	// Mức tự chủ duyệt ảnh theo điểm qua đúng đường "Duyệt hết" của chủ kênh.
	images.Autonomy = autonomySvc.Store
	autonomySvc.Reviewer = cloneimages.AutoReviewer{Service: images}
	images.Learner = imagesLearner{rt: seams.Learn}
	backdrops := &clonebackdrop.Service{Edits: edits, Deps: stepbackdrop.ProductionDeps(postgres.NewBackdropStore(pool), backdropSvc)}
	bundles := &bundle.Service{Edits: edits, Exports: editorService, Ffmpeg: toolKit.Ffmpeg, Runner: toolDeps.Runner, OwnerName: ownerName}
	store := edits.Store.(*postgres.CloneStore)
	bulkOps := &bulk.Service{Jobs: jobs, Images: images, Store: store, Exports: editorService, Publish: edits.Publish, Approvals: store, ApproveProject: store.PrepareCloneExport}
	imageOps := &queuedPromptOperations{Service: images, queue: postgres.NewQueueStore(pool), publish: edits.Publish}
	seams.Images, seams.Backdrops, seams.Metadata, seams.ImagesLeft = imageOps, backdrops, metadata, images.QuotaLeft
	web.MountCloneOperations(g, web.CloneOperations{Images: imageOps, Backdrops: backdrops, Edits: edits, Bundles: bundles, Bulk: bulkOps, Metadata: metadata})
	return func(ctx context.Context) error {
		err := bulkOps.DispatchReviews(ctx)
		if autoErr := autonomySvc.DispatchReviews(ctx); err == nil {
			err = autoErr
		}
		return err
	}
}
