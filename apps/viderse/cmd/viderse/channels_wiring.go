package main

// Ráp mảng Kênh của bạn: store Postgres, service nghiệp vụ, route HTTP, và
// đường AI (Đạo diễn/Hoạ sĩ) cho nhận diện. serve.go gọi mountChannels một
// lần; CLI gọi openChannelService theo cùng cách (không có hub SSE).
//
// Adapter ở đây giữ mảng kênh không import aiconn/imagegen/activity/library:
// business chỉ khai interface, composition root mới biết lớp thật.

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
)

// channelAI buộc Executors của Kết nối AI vào interface của mảng kênh. Gọi lại
// mỗi lần chạy: cấu hình trợ lý có thể vừa đổi.
type channelAI struct{ e *aiExecutors }

func (a channelAI) Director() []channels.TextModel {
	models := a.e.Director()
	out := make([]channels.TextModel, 0, len(models))
	for _, m := range models {
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

func (a channelAI) Drawer() channels.Drawer {
	svc := a.e.Drawer()
	if svc == nil {
		return nil
	}
	a.e.mu.RLock()
	names := make([]string, 0, len(a.e.chain))
	for _, ref := range a.e.chain {
		names = append(names, ref.Provider)
	}
	a.e.mu.RUnlock()
	return channelDrawer{svc: svc, providers: names}
}

// channelDrawer giữ tên các provider đã cấu hình để hỏi mốc tự thử lại khi một
// lần vẽ báo hạn mức (imagegen.QuotaReset cần đúng tên provider).
type channelDrawer struct {
	svc       *imagegen.Service
	providers []string
}

func (d channelDrawer) Draw(ctx context.Context, req imagegen.Request) (imagegen.Receipt, error) {
	return d.svc.Draw(ctx, req)
}

func (d channelDrawer) QuotaReset() (time.Time, bool) {
	for _, provider := range d.providers {
		if at, ok := d.svc.QuotaReset(provider); ok {
			return at, true
		}
	}
	return time.Time{}, false
}

// channelTasks bọc activity.Service thành interface của mảng kênh.
type channelTasks struct{ svc *activity.Service }

func (t channelTasks) Start(ctx context.Context, spec domain.TaskSpec) (string, error) {
	return t.svc.Start(ctx, spec)
}

func (t channelTasks) Update(ctx context.Context, id string, u domain.TaskUpdate) error {
	return t.svc.Update(ctx, id, u)
}

func (t channelTasks) Finish(ctx context.Context, id string) error { return t.svc.Finish(ctx, id) }

func (t channelTasks) FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*channels.TaskInfo, error) {
	task, err := t.svc.FindTaskByObject(ctx, kind, object)
	if err != nil || task == nil {
		return nil, err
	}
	return &channels.TaskInfo{ID: task.ID}, nil
}

func (t channelTasks) Notify(ctx context.Context, kind, text, href, actionLabel string) (string, error) {
	return t.svc.Notify(ctx, kind, text, href, actionLabel)
}

// mountChannels dựng service kênh và gắn route /api/owner-channels*. Trả
// service để serve dừng các run nhận diện khi tắt.
func mountChannels(
	g *web.Gateway, pool *pgxpool.Pool, logStore *logs.Store,
	activities *activity.Service, ai *aiExecutors, storageSvc *storage.Service,
	hub *events.Hub, mediaSvc *media.Service,
) (*channels.Service, error) {
	// Một library.Service tối giản (không route, không runner) chỉ để tạo/xoá
	// "Mẫu chuẩn"; vẫn phát SSE thư viện để trang Thư viện thấy template mới.
	notifyTemplates := web.Notifier(hub, library.TopicTemplate)
	templates := library.New(library.Deps{
		Store: postgres.NewLibraryStore(pool),
		Notify: func(topic, ref string) {
			if topic == library.TopicTemplate {
				notifyTemplates(ref)
			}
		},
	})
	svc := newChannelService(pool, activities, ai, storageSvc, templates, func(msg string) {
		logStore.Append("channels", logs.LevelError, logStore.Redact(msg))
	}, mediaSvc)
	svc.SetRunInBackground(true)
	svc.SetNotifier(web.Notifier(hub, "channels"))
	web.MountChannelsDiag(g, svc, func(msg string) {
		logStore.Append("channels", logs.LevelError, logStore.Redact(msg))
	})
	if err := svc.Start(context.Background()); err != nil {
		return nil, err
	}
	return svc, nil
}

// newChannelService dựng service trên đúng một đường ráp cho serve và CLI.
// diag nhận nguyên nhân kỹ thuật của lỗi nhận diện (nil ở CLI).
func newChannelService(pool *pgxpool.Pool, activities *activity.Service, ai *aiExecutors, storageSvc channels.Storage, templates channels.StandardTemplates, diag func(string), mediaSvc *media.Service) *channels.Service {
	opts := channels.Options{Diag: diag,
		Store:     postgres.NewChannelStore(pool),
		Templates: templates,
		Tasks:     channelTasks{svc: activities},
		Storage:   storageSvc,
		Media:     mediaSvc,
	}
	if ai != nil {
		opts.AI = channelAI{e: ai}
	}
	return channels.New(opts)
}

// openChannelService mở pool + activity cho các lệnh `viderse channel`; chỉ
// dựng AI khi lệnh cần vẽ (identity). CLI không có hub SSE và không chạy nền.
func openChannelService(ctx context.Context, needAI bool) (*channels.Service, func(), error) {
	pool, err := openRivalPool()
	if err != nil {
		return nil, nil, err
	}
	settings := postgres.NewSettingsStore(pool)
	activities := activity.New(postgres.NewActivityStore(pool), nil)
	storageSvc, err := openCLIStorage(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	manager, err := openCLIMedia(ctx, pool, storageSvc.Root)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	var ai *aiExecutors
	aiStop := func() {}
	if needAI {
		ai, aiStop, err = openCLIAI(ctx, pool, settings)
		if err != nil {
			_ = manager.Close(context.Background())
			pool.Close()
			return nil, nil, err
		}
	}
	// CLI: library.Service tối giản, không SSE.
	templates := library.New(library.Deps{Store: postgres.NewLibraryStore(pool)})
	svc := newChannelService(pool, activities, ai, storageSvc, templates, nil, manager.Media())
	if err := svc.Start(ctx); err != nil {
		aiStop()
		_ = manager.Close(context.Background())
		pool.Close()
		return nil, nil, err
	}
	stop := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = svc.Close(closeCtx)
		aiStop()
		_ = manager.Close(closeCtx)
		pool.Close()
	}
	return svc, stop, nil
}

// cliStorage shares validated root resolution across CLI entrypoints.
// All durable byte access goes through the media coordinator.
type cliStorage struct{ root string }

func (c cliStorage) Root() string { return c.root }

func (c cliStorage) Resolve(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("đường dẫn trong DB phải là tương đối: %q", rel)
	}
	root := c.Root()
	joined := filepath.Join(root, rel)
	if joined != root && !strings.HasPrefix(joined, root+string(filepath.Separator)) {
		return "", fmt.Errorf("đường dẫn thoát khỏi thư mục dự án: %q", rel)
	}
	return joined, nil
}

// openCLIStorage trả bộ giải đường dẫn cho CLI.
func openCLIStorage(ctx context.Context, pool *pgxpool.Pool) (channels.Storage, error) {
	root, err := cliProjectRoot(ctx, pool)
	if err != nil {
		return nil, err
	}
	return cliStorage{root: root}, nil
}

// openCLIAI dựng chuỗi Đạo diễn/Hoạ sĩ cho `viderse channel identity`. Dùng
// đúng wireAI của serve với hub/status tạm — không mở cổng, không phục vụ gì.
func openCLIAI(ctx context.Context, pool *pgxpool.Pool, settings *postgres.SettingsStore) (*aiExecutors, func(), error) {
	identityDSN := bootstrap.IdentityDSNFromEnv()
	if identityDSN == "" {
		return nil, nil, fmt.Errorf("thiếu VERSE_IDENTITY_DATABASE_URL (kho tài khoản chung, xem apps/viderse/deploy/README.md)")
	}
	accounts, closeAccounts, err := openAccounts(ctx, identityDSN)
	if err != nil {
		return nil, nil, err
	}
	hub := web.NewEventHub()
	statusSvc := status.New(settings, web.PublishStatus(hub))
	gateway := web.NewGateway(accounts, hub, statusSvc)
	cfgDir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		hub.Close()
		closeAccounts()
		return nil, nil, err
	}
	toolRegistry := tools.NewRegistry(cfgDir, nil)
	runner := tools.Runner(&tools.ExecRunner{Registry: toolRegistry})
	toolKit := tools.New(tools.Deps{
		Runner:   runner,
		Paths:    &tools.FilePathStore{Dir: filepath.Join(cfgDir, "tools")},
		BaseDir:  cfgDir,
		Client:   &http.Client{Timeout: 15 * time.Minute},
		Registry: toolRegistry,
	})
	aiCtx, cancel := context.WithCancel(ctx)
	wiring, err := wireAI(aiCtx, gateway, pool, settings, hub, statusSvc, cfgDir, toolKit, runner)
	if err != nil {
		cancel()
		hub.Close()
		closeAccounts()
		return nil, nil, err
	}
	return wiring.Executors, func() {
		cancel()
		wiring.Service.Close()
		hub.Close()
		closeAccounts()
	}, nil
}
