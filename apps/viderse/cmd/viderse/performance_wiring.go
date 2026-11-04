package main

import (
	"context"
	"sync"

	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/performance"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sharedYouTubePacer là bộ điều tốc YouTube của cả tiến trình: quét đối thủ,
// kiểm tra link và đọc số liệu sau đăng cùng xếp hàng qua nó.
var sharedYouTubePacer = youtube.NewPacer()

// performanceMarker đánh dấu đã đăng qua kho clone (cùng đường với thao tác hàng loạt).
type performanceMarker struct{ store *postgres.CloneStore }

func (m performanceMarker) MarkPosted(ctx context.Context, id string, posted bool) error {
	return m.store.MarkPosted(ctx, id, posted)
}

func mountPerformance(ctx context.Context, g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, cfgDir string, jobs *pipeline.Service, store *postgres.CloneStore, metadata *clonemetadata.Service, publish func(string, any)) (*performance.Service, func()) {
	keys := &performance.KeyStore{Settings: settings, Box: secretbox.New(cfgDir)}
	reader := performance.AutoReader{
		Key:    keys.Key,
		Data:   &performance.DataAPIReader{},
		Public: performance.PublicPageReader{Client: youtube.NewClient(youtube.Options{Pacer: sharedYouTubePacer})},
	}
	svc := &performance.Service{
		Store:  postgres.NewPerformanceStore(pool),
		Reader: reader,
		Jobs:   jobs,
		Marker: performanceMarker{store: store},
		Title: func(ctx context.Context, jobID string) (string, error) {
			view, err := metadata.Read(ctx, jobID)
			if err != nil || view.Metadata == nil {
				return "", err
			}
			return view.Metadata.Title, nil
		},
		Configured: reader.Configured,
		Publish:    publish,
	}
	web.MountPerformance(g, svc)
	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); svc.Run(runCtx) }()
	return svc, func() { cancel(); wg.Wait() }
}
