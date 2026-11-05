package main

import (
	"context"

	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/posted"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postedMarker đánh dấu đã đăng qua kho clone (cùng đường với thao tác hàng loạt).
type postedMarker struct{ store *postgres.CloneStore }

func (m postedMarker) MarkPosted(ctx context.Context, id string, posted bool) error {
	return m.store.MarkPosted(ctx, id, posted)
}

// mountPosted gắn đường mark-posted (§21) lên gateway: đánh dấu đã đăng, chốt
// facts metadata và lưu link YouTube. V38 đã bỏ reader số liệu sau đăng nên đây
// là pure handler — không goroutine, không reader.
func mountPosted(g *web.Gateway, pool *pgxpool.Pool, store *postgres.CloneStore, jobs *pipeline.Service, metadata *clonemetadata.Service, publish func(string, any)) {
	svc := &posted.Service{
		Store:   postgres.NewPostedStore(pool),
		Jobs:    jobs,
		Marker:  postedMarker{store: store},
		Publish: publish,
		Title: func(ctx context.Context, jobID string) (string, error) {
			view, err := metadata.Read(ctx, jobID)
			if err != nil || view.Metadata == nil {
				return "", err
			}
			return view.Metadata.Title, nil
		},
	}
	web.MountPosted(g, svc)
}
