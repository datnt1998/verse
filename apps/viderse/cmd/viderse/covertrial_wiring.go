package main

// Ráp "Vẽ thử ảnh" vào serve: dùng chung journal/chain của aiExecutors, pacer
// ảnh của hàng chờ, media.Service và nguồn video của bản xem thử. Không dựng
// chuỗi provider thứ hai, không đọc/ghi job nguồn.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/covertrial"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/preview"
)

type coverTrialDeps struct {
	Pool    *pgxpool.Pool
	Library *library.Service
	Jobs    interface {
		Get(ctx context.Context, id string) (*domain.CloneJobView, error)
	}
	Media   *media.Service
	Tasks   *activity.Service
	Preview *preview.Service
	AI      *aiWiring
	// Slot là pacer ảnh dùng chung với bước ảnh (queue.WithImageSlot).
	Slot func(ctx context.Context, run func(context.Context) error) error
}

// mountCoverTrials dựng service, gắn delegate vào Thư viện và trả hàm đóng.
func mountCoverTrials(deps coverTrialDeps) func() error {
	store := postgres.NewCoverTrialStore(deps.Pool)
	svc := covertrial.New(covertrial.Options{
		Templates: deps.Library,
		Prompts:   deps.Library,
		Jobs:      deps.Jobs,
		Sources:   deps.Preview.Sources,
		Store:     store,
		Media:     deps.Media,
		Tasks:     deps.Tasks,
		Slot:      deps.Slot,
		ResolveDrawer: func(ctx context.Context) (covertrial.DrawerResolution, error) {
			return resolveTrialDrawer(ctx, deps.AI)
		},
		Quota: func(ctx context.Context) (library.TrialQuota, error) {
			return trialQuota(ctx, deps.AI)
		},
	})
	recoverCtx, recoverCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := svc.Recover(recoverCtx); err != nil {
		fmt.Fprintln(os.Stderr, "Khôi phục lượt vẽ thử:", err)
	}
	recoverCancel()
	deps.Library.BindCoverTrial(svc)
	deps.Library.BindPromptSamples(promptSampleBuilder{svc})
	deps.Library.BindTrialFiles(trialFiles{store: store, media: deps.Media})
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return svc.Close(ctx)
	}
}

type promptSampleBuilder struct {
	*covertrial.Service
}

func (b promptSampleBuilder) Status(ctx context.Context, promptID string) (*library.PromptSampleStatus, error) {
	return b.PromptStatus(ctx, promptID)
}

func (b promptSampleBuilder) Start(ctx context.Context, promptID string, req library.PromptSampleRequest) (*library.PromptSampleStatus, error) {
	return b.StartPromptSample(ctx, promptID, req)
}

func (b promptSampleBuilder) Cancel(ctx context.Context, promptID, sampleID string) error {
	return b.CancelPromptSample(ctx, promptID, sampleID)
}

func (b promptSampleBuilder) ImagePath(ctx context.Context, promptID, sampleID string) (string, error) {
	return b.PromptSamplePath(ctx, promptID, sampleID)
}

// resolveTrialDrawer lấy đúng provider đứng đầu chuỗi đã lưu.
func resolveTrialDrawer(ctx context.Context, ai *aiWiring) (covertrial.DrawerResolution, error) {
	drawer := ai.Executors.Drawer()
	artist, err := ai.Service.ResolveArtist(ctx)
	if err != nil {
		return covertrial.DrawerResolution{}, noDrawerConfig()
	}
	providers, _ := ai.Executors.imageProviders(ctx, artist.DrawChain)
	if drawer == nil || len(providers) == 0 {
		return covertrial.DrawerResolution{}, noDrawerConfig()
	}
	primary := providers[0]
	if err := primary.Ready(ctx); err != nil {
		return covertrial.DrawerResolution{}, err
	}
	return covertrial.DrawerResolution{
		Drawer:   drawer,
		Provider: primary.Name(),
		Model:    primary.Model(),
		Label:    imagegen.DisplayLabel(primary),
		RefLimit: imagegen.RefLimit(primary.Name()),
	}, nil
}

// trialQuota đọc hạn mức ảnh của provider đứng đầu chuỗi.
func trialQuota(ctx context.Context, ai *aiWiring) (library.TrialQuota, error) {
	artist, err := ai.Service.ResolveArtist(ctx)
	if err != nil {
		return library.TrialQuota{}, err
	}
	providers, _ := ai.Executors.imageProviders(ctx, artist.DrawChain)
	if len(providers) == 0 {
		return library.TrialQuota{}, noDrawerConfig()
	}
	primary := providers[0]
	quota := library.TrialQuota{
		Provider: images.ProviderID(primary.Name()),
		Label:    imagegen.DisplayLabel(primary),
		Paid:     primary.Paid(),
		Total:    ai.Executors.limit,
	}
	if !primary.Paid() {
		return quota, nil
	}
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	charged, err := ai.Executors.journal.Charged(ctx, primary.Name(), day)
	if err != nil {
		return library.TrialQuota{}, err
	}
	quota.Left = max(0, ai.Executors.limit-int(charged))
	reset := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	quota.ResetAt = &reset
	return quota, nil
}

func noDrawerConfig() error {
	return imagegen.MarkUnpaid(&imagegen.DrawError{
		Code:     imagegen.ErrCodeToolMissing,
		Message:  "Chưa có nguồn vẽ ảnh",
		Recovery: "Mở Cài đặt → Kết nối AI và chọn nguồn vẽ ảnh",
	})
}

// trialFiles dọn media lịch sử vẽ thử khi template/prompt bị xoá.
type trialFiles struct {
	store *postgres.CoverTrialStore
	media *media.Service
}

func (t trialFiles) PathsForTemplate(ctx context.Context, templateID string) ([]string, error) {
	return t.store.CoverTrialMediaPaths(ctx, templateID)
}

func (t trialFiles) UnbindFiles(ctx context.Context, paths []string, commit func(context.Context) error) error {
	if len(paths) == 0 {
		return commit(ctx)
	}
	if t.media == nil {
		return errors.New("chưa có media")
	}
	return t.media.Unbind(ctx, paths, commit)
}

func (t trialFiles) DropPromptSamples(ctx context.Context, promptID string, commit func(context.Context) error) error {
	paths, err := t.store.PromptSampleMediaPaths(ctx, promptID)
	if err != nil {
		return err
	}
	drop := func(txctx context.Context) error {
		if err := t.store.DeletePromptSamplesByPrompt(txctx, promptID); err != nil {
			return err
		}
		return commit(txctx)
	}
	if t.media == nil {
		if len(paths) > 0 {
			return errors.New("chưa có media")
		}
		return drop(ctx)
	}
	return t.media.Unbind(ctx, paths, drop)
}
